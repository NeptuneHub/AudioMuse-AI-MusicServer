// Suggested path: music-server-backend/subsonic_music_handlers.go
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/dhowden/tag"
	"github.com/disintegration/imaging"
	"github.com/gin-gonic/gin"
)

// --- Subsonic Music Handlers ---

// AudioInfo represents detected audio file information
type AudioInfo struct {
	Format   string
	Bitrate  int
	Codec    string
	Duration int // Duration in seconds
}

// getDuration extracts the duration of an audio file using ffprobe
func getDuration(filePath string) int {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath)

	output, err := cmd.Output()
	if err != nil {
		log.Printf("⚠️  FFprobe duration failed for %s: %v", filepath.Base(filePath), err)
		return 0
	}

	// Parse duration (in seconds, may have decimal)
	durationStr := strings.TrimSpace(string(output))
	durationFloat, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		log.Printf("⚠️  Could not parse duration '%s' for %s", durationStr, filepath.Base(filePath))
		return 0
	}

	// Round to nearest integer
	return int(durationFloat + 0.5)
}

// audioProperties holds the OpenSubsonic Child audio metadata captured during a
// scan. All numeric fields are 0 when unknown (omitted from responses).
type audioProperties struct {
	Duration     int   // seconds
	Size         int64 // bytes
	BitRate      int   // kbps
	SamplingRate int   // Hz
	ChannelCount int
	BitDepth     int
}

// getAudioProperties probes a file's audio properties with a single ffprobe call
// (plus an os.Stat for size), replacing the bare getDuration probe during scans
// so we capture bitRate/samplingRate/channelCount/bitDepth without a second pass.
func getAudioProperties(filePath string) audioProperties {
	props := audioProperties{}
	if fi, err := os.Stat(filePath); err == nil {
		props.Size = fi.Size()
	}

	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "format=duration,bit_rate:stream=sample_rate,channels,bits_per_raw_sample",
		"-of", "default=noprint_wrappers=1",
		filePath)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("⚠️  FFprobe properties failed for %s: %v", filepath.Base(filePath), err)
		return props
	}

	for _, line := range strings.Split(string(output), "\n") {
		kv := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(kv) != 2 {
			continue
		}
		val := strings.TrimSpace(kv[1])
		switch kv[0] {
		case "duration":
			if f, e := strconv.ParseFloat(val, 64); e == nil {
				props.Duration = int(f + 0.5)
			}
		case "bit_rate":
			if n, e := strconv.Atoi(val); e == nil {
				props.BitRate = n / 1000 // bps -> kbps
			}
		case "sample_rate":
			if n, e := strconv.Atoi(val); e == nil {
				props.SamplingRate = n
			}
		case "channels":
			if n, e := strconv.Atoi(val); e == nil {
				props.ChannelCount = n
			}
		case "bits_per_raw_sample":
			if n, e := strconv.Atoi(val); e == nil {
				props.BitDepth = n
			}
		}
	}
	return props
}

// extractTitleFromFilename extracts title from filename with proper priority
// Priority: 1. Metadata, 2. Filename parsing, 3. Folder structure
func extractTitleFromFilename(filePath string) string {
	filename := filepath.Base(filePath)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	// Remove track number patterns: "01 - ", "01. ", "01 ", etc.
	trackNumRegex := regexp.MustCompile(`^(\d{1,3})[\s.\-_]+`)
	nameWithoutExt = trackNumRegex.ReplaceAllString(nameWithoutExt, "")

	// Priority 2a: Try "Artist - Title" pattern in filename (with " - " separator)
	if parts := strings.SplitN(nameWithoutExt, " - ", 2); len(parts) == 2 {
		title := cleanMetadataString(parts[1])
		if title != "" {
			return title
		}
	}

	// Priority 2b: Try "Artist_Title" pattern (underscore separator)
	if parts := strings.SplitN(nameWithoutExt, "_", 2); len(parts) == 2 {
		title := cleanMetadataString(parts[1])
		if title != "" && !isCommonFolderName(parts[0]) {
			return title
		}
	}

	// Priority 2c: Use entire filename as title (after removing track number)
	title := cleanMetadataString(nameWithoutExt)
	if title != "" {
		return title
	}

	return "Unknown Title"
}

// extractArtistFromPath extracts artist with proper priority
// Priority: 1. Metadata, 2. Filename "Artist - Title", 3. Folder structure
// NOTE: skip the configured library root folder when deriving artist from folders
func extractArtistFromPath(filePath string) string {
	filename := filepath.Base(filePath)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	// Remove track numbers first
	trackNumRegex := regexp.MustCompile(`^(\d{1,3})[\s.\-_]+`)
	nameWithoutExt = trackNumRegex.ReplaceAllString(nameWithoutExt, "")

	// Priority 2a: Try "Artist - Title" pattern in FILENAME FIRST (with " - " separator)
	if parts := strings.SplitN(nameWithoutExt, " - ", 2); len(parts) == 2 {
		artist := cleanMetadataString(parts[0])
		if artist != "" && !isCommonFolderName(artist) {
			return artist
		}
	}

	// Priority 2b: Try "Artist_Title" pattern (underscore separator)
	if parts := strings.SplitN(nameWithoutExt, "_", 2); len(parts) == 2 {
		artist := cleanMetadataString(parts[0])
		if artist != "" && !isCommonFolderName(artist) {
			return artist
		}
	}

	// Priority 3: Fall back to folder structure using the configured library root
	dir := filepath.Dir(filePath)

	// If a library root is configured and contains this file, derive artist as the
	// first path component under the root (if present). This ensures we skip the
	// library root itself and correctly handle both root/artist/album/file and
	// root/artist/file structures.
	if libRoot, ok := findLibraryRootForFile(filePath); ok {
		rel, rErr := filepath.Rel(libRoot, filePath)
		if rErr == nil {
			relParts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
			if len(relParts) >= 2 {
				artist := cleanMetadataString(relParts[0])
				if artist != "" && !isCommonFolderName(artist) {
					return artist
				}
			}
			// If there's no component under root aside from the filename, treat as unknown
			return "Unknown Artist"
		}
	}

	// Fallback to previous behavior: use grandparent folder
	pathParts := strings.Split(filepath.Clean(dir), string(filepath.Separator))
	if len(pathParts) >= 2 {
		artist := cleanMetadataString(pathParts[len(pathParts)-2])
		if artist != "" && !isCommonFolderName(artist) {
			return artist
		}
	}

	return "Unknown Artist"
}

// extractAlbumFromPath extracts album name with proper priority
// Priority: 1. Metadata, 2. Filename patterns, 3. Parent folder name
// artistName parameter is used to remove redundant "Artist - Album" patterns
// NOTE: If the parent folder is actually the artist folder (i.e., parent is directly under library root)
// then treat as no album (Unknown Album).
func extractAlbumFromPath(filePath string, artistName string) string {
	// Priority 2: Could check filename for "Artist - Album - Title" patterns
	// but this is extremely rare, so we skip directly to folder-based extraction

	// Priority 3: Use parent folder as album name (most common pattern)
	dir := filepath.Dir(filePath)

	// If the file is under a configured library root, use the relative path to
	// determine whether an album exists: root/artist/album/file -> album=component[1]
	if libRoot, ok := findLibraryRootForFile(filePath); ok {
		rel, rErr := filepath.Rel(libRoot, filePath)
		if rErr == nil {
			relParts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
			// If we have at least 3 components (artist/album/file), use the 2nd as album
			if len(relParts) >= 3 {
				albumName := cleanMetadataString(relParts[1])
				if albumName != "" && !isCommonFolderName(albumName) {
					return albumName
				}
			}
			// Otherwise, there's no album component under this library layout
			return "Unknown Album"
		}
	}

	albumName := filepath.Base(dir)

	// Remove artist prefix if present: "SUPERARE - Rich Party People" -> "Rich Party People"
	if artistName != "" && artistName != "Unknown Artist" {
		// Try exact match with " - " separator
		dashPrefix := artistName + " - "
		if strings.HasPrefix(albumName, dashPrefix) {
			albumName = strings.TrimPrefix(albumName, dashPrefix)
		} else {
			// Try case-insensitive match with " - "
			dashPrefixLower := strings.ToLower(dashPrefix)
			if strings.HasPrefix(strings.ToLower(albumName), dashPrefixLower) {
				albumName = albumName[len(dashPrefix):]
			}
		}

		// Also try underscore separator: "SUPERARE_Rich Party People" -> "Rich Party People"
		underscorePrefix := artistName + "_"
		if strings.HasPrefix(albumName, underscorePrefix) {
			albumName = strings.TrimPrefix(albumName, underscorePrefix)
		} else {
			// Try case-insensitive match with "_"
			underscorePrefixLower := strings.ToLower(underscorePrefix)
			if strings.HasPrefix(strings.ToLower(albumName), underscorePrefixLower) {
				albumName = albumName[len(underscorePrefix):]
			}
		}
	}

	// Clean up common patterns in album name
	albumName = cleanMetadataString(albumName)

	if albumName != "" && !isCommonFolderName(albumName) {
		return albumName
	}

	return "Unknown Album"
}

// cleanMetadataString removes unwanted characters and patterns from extracted metadata
func cleanMetadataString(s string) string {
	// Remove year patterns: [2020], (2020), - 2020, etc.
	yearRegex := regexp.MustCompile(`[\[\(]?\d{4}[\]\)]?`)
	s = yearRegex.ReplaceAllString(s, "")

	// Remove common tags: [320kbps], (Remastered), etc.
	tagRegex := regexp.MustCompile(`[\[\(][^\]\)]+[\]\)]`)
	s = tagRegex.ReplaceAllString(s, "")

	// Clean up underscores, hyphens at edges, multiple spaces
	s = strings.ReplaceAll(s, "_", " ")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	s = strings.Trim(s, " -")

	return strings.TrimSpace(s)
}

// isCommonFolderName checks if a folder name is too generic to be useful metadata
func isCommonFolderName(name string) bool {
	commonNames := []string{
		"music", "audio", "songs", "tracks", "albums", "downloads",
		"mp3", "flac", "m4a", "ogg", "media", "library", "collection",
		"unknown", "various", "compilation",
	}

	lowerName := strings.ToLower(name)
	for _, common := range commonNames {
		if lowerName == common {
			return true
		}
	}

	return false
}

// findLibraryRootForFile returns the configured library root path that contains filePath, if any
func findLibraryRootForFile(filePath string) (string, bool) {
	var libPath string
	// Find the longest matching prefix path from library_paths
	// Use LIKE with path||'%' to match files under a library path
	err := db.QueryRow("SELECT path FROM library_paths WHERE ? LIKE path || '%' ORDER BY LENGTH(path) DESC LIMIT 1", filePath).Scan(&libPath)
	if err != nil {
		return "", false
	}
	return filepath.Clean(libPath), true
}

// detectAudioFormat detects the format and bitrate of an audio file using FFprobe
func detectAudioFormat(filePath string) (*AudioInfo, error) {
	info := &AudioInfo{}

	// Detect format from file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".mp3":
		info.Format = "mp3"
		info.Codec = "mp3"
	case ".flac":
		info.Format = "flac"
		info.Codec = "flac"
	case ".m4a", ".aac":
		info.Format = "aac"
		info.Codec = "aac"
	case ".ogg":
		info.Format = "ogg"
		info.Codec = "vorbis"
	case ".opus":
		info.Format = "opus"
		info.Codec = "opus"
	default:
		info.Format = "unknown"
	}

	// Use ffprobe to get accurate bitrate
	// ffprobe -v error -show_entries format=bit_rate -of default=noprint_wrappers=1:nokey=1 file.mp3
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=bit_rate",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filePath)

	output, err := cmd.Output()
	if err != nil {
		log.Printf("⚠️  FFprobe failed for %s: %v (will transcode)", filepath.Base(filePath), err)
		info.Bitrate = 0 // Unknown bitrate, will transcode
		return info, nil
	}

	// Parse bitrate (in bits per second)
	bitrateStr := strings.TrimSpace(string(output))
	bitrateBps, err := strconv.Atoi(bitrateStr)
	if err != nil || bitrateBps == 0 {
		log.Printf("⚠️  Could not parse bitrate '%s' for %s", bitrateStr, filepath.Base(filePath))
		info.Bitrate = 0
		return info, nil
	}

	// Convert bps to kbps
	info.Bitrate = bitrateBps / 1000
	log.Printf("🔍 Detected: %s, %dkbps, codec=%s", info.Format, info.Bitrate, info.Codec)

	return info, nil
}

// shouldTranscode determines if transcoding is necessary
func shouldTranscode(sourceInfo *AudioInfo, targetFormat string, targetBitrate int) bool {
	// Always transcode lossless formats (FLAC) to save bandwidth
	if sourceInfo.Format == "flac" {
		log.Printf("🔄 Transcoding needed: source is lossless FLAC")
		return true
	}

	// If source format matches target format
	if sourceInfo.Format == targetFormat {
		// If we can't determine source bitrate, assume transcoding needed
		if sourceInfo.Bitrate == 0 {
			log.Printf("🔄 Transcoding: source format matches but bitrate unknown")
			return true
		}

		// If source bitrate is lower or equal to target, no need to transcode
		if sourceInfo.Bitrate <= targetBitrate {
			log.Printf("✨ Skipping transcode: source %s %dkbps <= target %dkbps",
				sourceInfo.Format, sourceInfo.Bitrate, targetBitrate)
			return false
		}
	}

	log.Printf("🔄 Transcoding needed: %s → %s", sourceInfo.Format, targetFormat)
	return true
}

// getTranscodingProfile returns optimized FFmpeg parameters based on quality
func getTranscodingProfile(format string, bitrate int) []string {
	// Base arguments common to all formats with ULTRA low-latency streaming optimizations
	baseArgs := []string{
		"-map", "0:a:0", // Map only first audio stream (skip embedded images/video)
		"-vn",           // No video processing
		"-sn",           // No subtitles
		"-threads", "0", // Auto threads for faster encoding
		"-v", "error", // Only show errors (reduce log spam)
		"-fflags", "+flush_packets+nobuffer", // Force immediate packet flushing
		"-flags", "low_delay", // Low delay mode
		"-max_delay", "0", // Zero output delay
		"-probesize", "32", // Minimal probe size for faster start
		"-analyzeduration", "0", // Skip analysis, start immediately
		"-avoid_negative_ts", "make_zero", // Handle timestamp issues
	}

	// Format-specific optimizations
	// Note: Some encoders like libmp3lame don't support preset parameter
	// Instead we use compression_level and quality settings for speed optimization
	switch format {
	case "mp3":
		return append(baseArgs,
			"-acodec", "libmp3lame",
			"-b:a", fmt.Sprintf("%dk", bitrate),
			"-compression_level", "0", // FASTEST encoding
			"-reservoir", "0", // Disable bit reservoir for instant start
			"-write_xing", "0", // Skip Xing header for immediate streaming
			"-q:a", "9", // Lowest quality for maximum speed (still acceptable at 192k)
		)
	case "ogg":
		return append(baseArgs,
			"-acodec", "libvorbis",
			"-b:a", fmt.Sprintf("%dk", bitrate),
		)
	case "aac":
		return append(baseArgs,
			"-acodec", "aac",
			"-b:a", fmt.Sprintf("%dk", bitrate),
			"-cutoff", "18000", // Frequency cutoff for AAC
			"-profile:a", "aac_low", // AAC-LC profile for best compatibility
		)
	case "opus":
		return append(baseArgs,
			"-acodec", "libopus",
			"-b:a", fmt.Sprintf("%dk", bitrate),
			"-vbr", "on", // Variable bitrate
			"-compression_level", "10", // Opus compression level (0-10, higher = better)
			"-frame_duration", "20", // Lower frame duration for faster start
		)
	default:
		// Fallback to MP3 for unknown formats
		return append(baseArgs,
			"-acodec", "libmp3lame",
			"-b:a", fmt.Sprintf("%dk", bitrate),
			"-compression_level", "0",
			"-reservoir", "0",
			"-write_xing", "0",
		)
	}
}

func subsonicStream(c *gin.Context) {
	user := c.MustGet("user").(User)

	songID := c.Query("id")
	path, duration, err := QuerySongPathAndDuration(db, songID)
	if err != nil {
		if err == sql.ErrNoRows {
			subsonicRespond(c, newSubsonicErrorResponse(70, "Song not found."))
			return
		}
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}

	// Set X-Content-Duration header (like Navidrome does) so browser knows duration immediately
	// This is critical for HTML5 audio controls to show correct timeline
	if duration > 0 {
		c.Header("X-Content-Duration", strconv.Itoa(duration))
	}

	// Check if user has transcoding enabled
	var transcodingEnabled int
	var format string
	var bitrate int
	err = db.QueryRow("SELECT enabled, format, bitrate FROM transcoding_settings WHERE user_id = ?", user.ID).
		Scan(&transcodingEnabled, &format, &bitrate)

	useTranscoding := err == nil && transcodingEnabled == 1

	log.Printf("🎧 Stream request: user=%s, song=%s, duration=%ds, transcoding_enabled=%v, format=%s, bitrate=%d",
		user.Username, filepath.Base(path), duration, useTranscoding, format, bitrate)

	if useTranscoding {
		// Smart codec detection: check if transcoding is actually needed
		sourceInfo, err := detectAudioFormat(path)
		if err == nil && !shouldTranscode(sourceInfo, format, bitrate) {
			log.Printf("✨ Smart skip: source already optimal, direct streaming")
			streamDirect(c, path)
			return
		}

		streamWithTranscoding(c, path, format, bitrate)
	} else {
		log.Printf("📀 Direct stream (no transcoding): %s", filepath.Base(path))
		streamDirect(c, path)
	}
}

func streamDirect(c *gin.Context, path string) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("Could not open file for streaming %s: %v", path, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Could not open file."))
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("Could not get file info for streaming %s: %v", path, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Could not read file info."))
		return
	}

	// Explicitly set Content-Length to help browser determine duration faster
	// http.ServeContent should do this, but let's be explicit
	c.Header("Content-Length", strconv.FormatInt(fileInfo.Size(), 10))
	c.Header("Accept-Ranges", "bytes")

	http.ServeContent(c.Writer, c.Request, fileInfo.Name(), fileInfo.ModTime(), file)
}

func streamWithTranscoding(c *gin.Context, inputPath string, format string, bitrate int) {
	startTime := time.Now()
	songID := c.Query("id")

	log.Printf("🎵 TRANSCODING REQUEST: format=%s, bitrate=%dkbps, file=%s, songID=%s",
		format, bitrate, filepath.Base(inputPath), songID)

	// Parse Range header if present
	rangeHeader := c.GetHeader("Range")
	var requestedStart int64 = 0
	var isRangeRequest bool = false

	if rangeHeader != "" {
		// Parse "bytes=12345-" or "bytes=12345-67890"
		re := regexp.MustCompile(`bytes=(\d+)-`)
		matches := re.FindStringSubmatch(rangeHeader)
		if len(matches) > 1 {
			requestedStart, _ = strconv.ParseInt(matches[1], 10, 64)
			// Only treat as range request if starting position is > 0
			// bytes=0- is just the browser asking for the whole file
			if requestedStart > 0 {
				isRangeRequest = true
				log.Printf("📍 RANGE REQUEST: bytes=%d- (seeking in transcoded stream)", requestedStart)
			} else {
				log.Printf("📀 Initial request (Range: bytes=0-) - treating as full stream")
			}
		}
	}

	// FFmpeg format mapping
	ffmpegFormatMap := map[string]string{
		"mp3":  "mp3",
		"ogg":  "ogg",
		"aac":  "adts",
		"opus": "opus",
	}

	ffmpegFormat, ok := ffmpegFormatMap[format]
	if !ok {
		log.Printf("❌ Unsupported transcoding format: %s - falling back to direct stream", format)
		streamDirect(c, inputPath)
		return
	}

	var seekSeconds float64 = 0

	if isRangeRequest && requestedStart > 0 {
		// Calculate approximate seek time from byte offset
		// Formula: bytes / (bitrate_kbps * 125) = seconds
		seekSeconds = float64(requestedStart) / float64(bitrate*125)
		log.Printf("🔍 Calculated seek position: %.2f seconds", seekSeconds)
	}

	// Get optimized transcoding profile
	profileArgs := getTranscodingProfile(format, bitrate)

	// Build FFmpeg command with seeking support
	args := []string{}

	// Add seek BEFORE input for fast seeking (only if actually seeking)
	if seekSeconds > 0 {
		args = append(args, "-ss", fmt.Sprintf("%.2f", seekSeconds))
		log.Printf("🔧 Adding FFmpeg seek flag: -ss %.2f", seekSeconds)
	}

	args = append(args, "-i", inputPath, "-vn")
	args = append(args, profileArgs...)
	args = append(args, "-f", ffmpegFormat, "pipe:1")

	log.Printf("🔧 FFmpeg command: ffmpeg %s", strings.Join(args, " "))

	cmd := exec.Command("ffmpeg", args...)

	// Capture stderr for debugging
	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Printf("❌ Failed to create FFmpeg stderr pipe: %v", err)
	} else {
		go func() {
			buf := make([]byte, 1024)
			for {
				n, err := stderr.Read(buf)
				if n > 0 {
					log.Printf("📹 FFmpeg: %s", string(buf[:n]))
				}
				if err != nil {
					break
				}
			}
		}()
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("❌ Failed to create FFmpeg stdout pipe: %v", err)
		streamDirect(c, inputPath)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("❌ Failed to start FFmpeg: %v", err)
		streamDirect(c, inputPath)
		return
	}

	// Set headers
	contentTypes := map[string]string{
		"mp3":  "audio/mpeg",
		"ogg":  "audio/ogg",
		"aac":  "audio/aac",
		"opus": "audio/opus",
	}
	contentType := contentTypes[format]
	bitrateStr := strconv.Itoa(bitrate) + "k"

	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes") // Support seeking
	c.Header("X-Transcoded", "true")
	c.Header("X-Transcode-Format", format)
	c.Header("X-Transcode-Bitrate", bitrateStr)
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	if isRangeRequest {
		c.Status(http.StatusPartialContent)
		log.Printf("📤 Sending 206 Partial Content response")
	} else {
		c.Status(http.StatusOK)
		log.Printf("📤 Sending 200 OK response")
	}

	// Flush headers immediately
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
		elapsed := time.Since(startTime).Milliseconds()
		log.Printf("⚡ Headers flushed at %dms", elapsed)
	}

	// Stream transcoded audio
	buf := make([]byte, 4096)
	bytesWritten := int64(0)
	chunkCount := 0

	for {
		n, err := stdout.Read(buf)
		if n > 0 {
			written, writeErr := c.Writer.Write(buf[:n])
			bytesWritten += int64(written)
			chunkCount++

			if flusher, ok := c.Writer.(http.Flusher); ok {
				flusher.Flush()
			}

			if chunkCount == 1 {
				elapsed := time.Since(startTime).Milliseconds()
				log.Printf("🚀 FIRST CHUNK SENT at %dms (%d bytes)", elapsed, written)
			}

			if writeErr != nil {
				log.Printf("⚠️  Client disconnected: %v", writeErr)
				cmd.Process.Kill()
				break
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("❌ Read error: %v", err)
			break
		}
	}

	cmd.Wait()
	log.Printf("✅ Transcoding complete: %d bytes sent", bytesWritten)
}

// generateWaveformPeaks generates waveform peaks from an audio file using FFmpeg
// Returns JSON string of peaks array for database storage
func generateWaveformPeaks(filePath string) (string, error) {
	// Use FFmpeg to extract audio samples for waveform
	// Generate 1000 peaks (500 samples = 1000 values for min/max peaks)
	samplesCount := 500

	cmd := exec.Command("ffmpeg",
		"-i", filePath,
		"-ac", "1", // Mono
		"-ar", "8000", // Low sample rate for faster processing
		"-f", "s16le", // 16-bit PCM
		"-acodec", "pcm_s16le",
		"pipe:1")

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("FFmpeg waveform generation failed: %v", err)
	}

	// Convert PCM data to samples array
	samples := make([]int16, len(output)/2)
	for i := 0; i < len(samples); i++ {
		samples[i] = int16(output[i*2]) | int16(output[i*2+1])<<8
	}

	// Downsample to desired number of peaks
	peaks := make([]float32, samplesCount*2) // min/max pairs
	samplesPerPeak := len(samples) / samplesCount

	if samplesPerPeak == 0 {
		samplesPerPeak = 1
	}

	for i := 0; i < samplesCount; i++ {
		start := i * samplesPerPeak
		end := start + samplesPerPeak
		if end > len(samples) {
			end = len(samples)
		}

		if start >= len(samples) {
			break
		}

		var min, max int16 = 32767, -32768
		for j := start; j < end; j++ {
			if samples[j] < min {
				min = samples[j]
			}
			if samples[j] > max {
				max = samples[j]
			}
		}

		// Normalize to -1.0 to 1.0
		peaks[i*2] = float32(min) / 32768.0   // Min peak
		peaks[i*2+1] = float32(max) / 32768.0 // Max peak
	}

	// Convert to JSON string for database storage
	peaksJSON, err := json.Marshal(peaks)
	if err != nil {
		return "", fmt.Errorf("failed to marshal peaks to JSON: %v", err)
	}

	return string(peaksJSON), nil
}

// subsonicGetWaveform generates waveform peaks data for fast visualization
func subsonicGetWaveform(c *gin.Context) {
	user := c.MustGet("user").(User)
	songID := c.Query("id")

	if songID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing song ID"})
		return
	}

	// Get song file path and pre-computed waveform peaks
	path, duration, waveformPeaks, err := QuerySongWaveform(db, songID)
	if err != nil {
		log.Printf("Error fetching song for waveform %s: %v", songID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Song not found"})
		return
	}

	// If pre-computed waveform exists, use it (fast path)
	if waveformPeaks != "" {
		var peaks []float32
		if err := json.Unmarshal([]byte(waveformPeaks), &peaks); err == nil {
			log.Printf("🚀 Using pre-computed waveform for user=%s, song=%s (%d peaks)",
				user.Username, filepath.Base(path), len(peaks))

			c.Header("Content-Type", "application/json")
			c.Header("Cache-Control", "public, max-age=31536000") // Cache for 1 year (immutable)
			c.JSON(http.StatusOK, gin.H{
				"peaks":    peaks,
				"duration": duration,
			})
			return
		}
		log.Printf("⚠️  Failed to parse pre-computed waveform, falling back to on-demand generation")
	}

	// Fallback: generate waveform on-demand (slow path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Printf("File does not exist for waveform: %s", path)
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	log.Printf("🌊 Generating waveform on-demand for user=%s, song=%s, duration=%ds",
		user.Username, filepath.Base(path), duration)

	// Use FFmpeg to extract audio samples for waveform
	// Generate 1000 peaks (500 samples = 1000 values for min/max peaks)
	samplesCount := 500

	cmd := exec.Command("ffmpeg",
		"-i", path,
		"-ac", "1", // Mono
		"-ar", "8000", // Low sample rate for faster processing
		"-f", "s16le", // 16-bit PCM
		"-acodec", "pcm_s16le",
		"pipe:1")

	output, err := cmd.Output()
	if err != nil {
		log.Printf("❌ FFmpeg waveform generation failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Waveform generation failed"})
		return
	}

	// Convert PCM data to peaks array
	samples := make([]int16, len(output)/2)
	for i := 0; i < len(samples); i++ {
		samples[i] = int16(output[i*2]) | int16(output[i*2+1])<<8
	}

	// Downsample to desired number of peaks
	peaks := make([]float32, samplesCount*2) // min/max pairs
	samplesPerPeak := len(samples) / samplesCount

	if samplesPerPeak == 0 {
		samplesPerPeak = 1
	}

	for i := 0; i < samplesCount; i++ {
		start := i * samplesPerPeak
		end := start + samplesPerPeak
		if end > len(samples) {
			end = len(samples)
		}

		if start >= len(samples) {
			break
		}

		var min, max int16 = 32767, -32768
		for j := start; j < end; j++ {
			if samples[j] < min {
				min = samples[j]
			}
			if samples[j] > max {
				max = samples[j]
			}
		}

		// Normalize to -1.0 to 1.0
		peaks[i*2] = float32(min) / 32768.0   // Min peak
		peaks[i*2+1] = float32(max) / 32768.0 // Max peak
	}

	log.Printf("✅ Generated %d waveform peaks on-demand", len(peaks))

	c.Header("Content-Type", "application/json")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate") // NO CACHING for on-demand
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	c.JSON(http.StatusOK, gin.H{
		"peaks":    peaks,
		"duration": duration,
	})
}

func subsonicScrobble(c *gin.Context) {
	user := c.MustGet("user").(User)

	songID := c.Query("id")
	if songID == "" {
		subsonicRespond(c, newSubsonicResponse(nil))
		return
	}

	now := time.Now().Format(time.RFC3339)

	err := UpdateSongPlayCount(db, songID, now)
	if err != nil {
		log.Printf("Error updating play count for user '%s' on song '%s': %v", user.Username, songID, err)
	}

	// Track play history for this user
	err = InsertPlayHistory(db, user.ID, songID, now)
	if err != nil {
		log.Printf("Error inserting play history for user '%s' on song '%s': %v", user.Username, songID, err)
	}

	log.Printf("Scrobbled song '%s' for user '%s'", songID, user.Username)
	subsonicRespond(c, newSubsonicResponse(nil))
}

func subsonicGetArtists(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	// List artists from the derived artists table (counts precomputed).
	rows, err := db.Query(`SELECT name, song_count, album_count FROM artists ORDER BY name COLLATE NOCASE`)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error querying artists."))
		return
	}
	defer rows.Close()
	var results []ArtistResult
	for rows.Next() {
		var r ArtistResult
		if err := rows.Scan(&r.Name, &r.SongCount, &r.AlbumCount); err != nil {
			continue
		}
		results = append(results, r)
	}

	artistIndex := make(map[string][]SubsonicArtist)
	seenArtists := make(map[string]bool)
	for _, result := range results {
		// Deduplicate exact string matches by normalized name (same approach as search)
		key := normalizeKey(result.Name)
		if seenArtists[key] {
			continue
		}
		seenArtists[key] = true

		var artist SubsonicArtist
		artist.Name = result.Name
		artist.ID = GenerateArtistID(artist.Name)
		artist.CoverArt = artist.Name
		artist.AlbumCount = result.AlbumCount
		artist.SongCount = result.SongCount

		var indexChar string
		for _, r := range artist.Name {
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				indexChar = strings.ToUpper(string(r))
				break
			}
		}
		if indexChar == "" {
			indexChar = "#"
		}

		artistIndex[indexChar] = append(artistIndex[indexChar], artist)
	}

	var indices []SubsonicArtistIndex
	for name, artists := range artistIndex {
		indices = append(indices, SubsonicArtistIndex{
			Name:    name,
			Artists: artists,
		})
	}

	sort.Slice(indices, func(i, j int) bool {
		return indices[i].Name < indices[j].Name
	})

	responseBody := &SubsonicArtists{Index: indices}
	response := newSubsonicResponse(responseBody)
	subsonicRespond(c, response)
}

// fetchAlbumList resolves the album list for getAlbumList/getAlbumList2 from the
// derived albums table (display artist + counts precomputed). On a DB error it
// responds with a Subsonic error and returns ok=false.
func fetchAlbumList(c *gin.Context) (resultAlbums []SubsonicAlbum, ok bool) {
	// Get parameters
	sizeParam := c.DefaultQuery("size", "500")
	offsetParam := c.DefaultQuery("offset", "0")
	genreParam := c.Query("genre")
	listType := c.DefaultQuery("type", "alphabeticalByArtist") // Required by Subsonic API spec

	size, err := strconv.Atoi(sizeParam)
	if err != nil || size <= 0 {
		size = 500
	}
	if size > 500 {
		size = 500
	}
	offset, err := strconv.Atoi(offsetParam)
	if err != nil || offset < 0 {
		offset = 0
	}

	log.Printf("getAlbumList2: type=%s, size=%d, offset=%d, genre=%s", listType, size, offset, genreParam)

	// Read from the derived albums table (display artist + counts precomputed).
	where := []string{}
	var args []interface{}
	if genreParam != "" {
		where = append(where, "(genres = ? OR genres LIKE ? OR genres LIKE ? OR genres LIKE ?)")
		args = append(args, genreParam, genreParam+";%", "%;"+genreParam+";%", "%;"+genreParam)
	}

	var orderByClause string
	switch listType {
	case "starred":
		user := c.MustGet("user").(User)
		where = append(where, "id IN (SELECT album_id FROM starred_albums WHERE user_id = ?)")
		args = append(args, user.ID)
		orderByClause = "ORDER BY name COLLATE NOCASE"
	case "newest":
		orderByClause = "ORDER BY max_date_added DESC, artist, name"
	case "recent":
		orderByClause = "ORDER BY max_last_played DESC, artist, name"
	case "frequent":
		orderByClause = "ORDER BY total_play_count DESC, artist, name"
	case "random":
		orderByClause = "ORDER BY RANDOM()"
	case "alphabeticalByName":
		orderByClause = "ORDER BY name COLLATE NOCASE, artist"
	case "alphabeticalByArtist":
		orderByClause = "ORDER BY artist COLLATE NOCASE, name COLLATE NOCASE"
	default:
		log.Printf("Warning: Unknown album list type '%s', defaulting to alphabeticalByArtist", listType)
		orderByClause = "ORDER BY artist COLLATE NOCASE, name COLLATE NOCASE"
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}

	var totalAlbums int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := db.QueryRow("SELECT COUNT(*) FROM albums "+whereSQL, countArgs...).Scan(&totalAlbums); err != nil {
		log.Printf("Error counting albums for pagination: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error querying albums."))
		return nil, false
	}

	if offset >= totalAlbums {
		return []SubsonicAlbum{}, true
	}

	query := fmt.Sprintf(`SELECT id, name, artist, artist_id, COALESCE(genre,''), song_count, total_duration, COALESCE(min_date_added,'')
		FROM albums %s %s LIMIT ? OFFSET ?`, whereSQL, orderByClause)
	args = append(args, size, offset)
	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Error querying albums: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error querying albums."))
		return nil, false
	}
	defer rows.Close()

	var albums []SubsonicAlbum
	seen := make(map[string]bool)
	for rows.Next() {
		var album SubsonicAlbum
		if err := rows.Scan(&album.ID, &album.Name, &album.Artist, &album.ArtistID, &album.Genre, &album.SongCount, &album.Duration, &album.Created); err != nil {
			log.Printf("Error scanning album row: %v", err)
			continue
		}
		if album.Name == "Unknown" {
			album.Name = "Unknown Album"
		}
		key := normalizeKey(album.Artist) + "|||" + normalizeKey(album.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		album.CoverArt = album.ID
		decorateAlbum(&album)
		albums = append(albums, album)
	}

	log.Printf("fetchAlbumList: Returning %d albums (total=%d)", len(albums), totalAlbums)
	return albums, true
}

// subsonicGetAlbumList2 returns albums in ID3 form (<albumList2> of AlbumID3).
func subsonicGetAlbumList2(c *gin.Context) {
	albums, ok := fetchAlbumList(c)
	if !ok {
		return
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicAlbumList2{Albums: albums}))
}

// subsonicGetAlbumList returns albums in the legacy directory form
// (<albumList> of Child objects with isDir=true).
func subsonicGetAlbumList(c *gin.Context) {
	albums, ok := fetchAlbumList(c)
	if !ok {
		return
	}
	children := make([]SubsonicDirectoryChild, 0, len(albums))
	for _, a := range albums {
		child := SubsonicDirectoryChild{
			ID:       a.ID,
			Parent:   a.ArtistID,
			Title:    a.Name,
			Album:    a.Name,
			Artist:   a.Artist,
			IsDir:    true,
			CoverArt: a.CoverArt,
			Duration: a.Duration,
			Genre:    a.Genre,
			Created:  a.Created,
		}
		children = append(children, child)
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicAlbumList{Albums: children}))
}

func subsonicGetAlbum(c *gin.Context) {
	user := c.MustGet("user").(User)

	albumSongId := c.Query("id")
	if albumSongId == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Missing required parameter 'id'"))
		return
	}

	var albumName, artistName, albumGenre, albumPath string
	err := db.QueryRow("SELECT album, artist, COALESCE(genre, ''), path FROM songs WHERE id = ? AND cancelled = 0", albumSongId).Scan(&albumName, &artistName, &albumGenre, &albumPath)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Album not found."))
		return
	}

	// Extract the album's directory path (parent directory of the song file)
	// This ensures we only return songs from the SAME physical album location
	albumDir := filepath.Dir(albumPath)
	log.Printf("getAlbum: Fetching songs for album='%s', artist='%s', albumId=%s, albumDir='%s'", albumName, artistName, albumSongId, albumDir)

	// Display album artist (precomputed in the derived albums table)
	displayArtist := albumDisplayArtist(db, albumName, albumDir)

	// Query songs that match the album title and are in the same directory
	// This ensures we return songs from the SAME physical album location regardless of individual track artist
	query := `
		SELECT s.id, s.title, s.artist, s.album, s.path, s.play_count, s.last_played, COALESCE(s.genre, ''), s.duration, COALESCE(s.date_added, ''),
		       s.replaygain_track_gain, s.replaygain_track_peak, s.replaygain_album_gain, s.replaygain_album_peak,
		       COALESCE(s.track, 0), COALESCE(s.year, 0), COALESCE(s.disc_number, 0),
		       COALESCE(s.size, 0), COALESCE(s.bitrate, 0), COALESCE(s.sample_rate, 0), COALESCE(s.channels, 0), COALESCE(s.bit_depth, 0), COALESCE(s.comment, ''),
		       CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred
		FROM songs s
		LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ?
		WHERE s.album = ? AND s.path LIKE ? AND s.cancelled = 0
		ORDER BY COALESCE(s.disc_number, 0), COALESCE(s.track, 0), s.title
	`

	// Use LIKE with the album directory to match all songs in the same folder
	pathPattern := albumDir + string(filepath.Separator) + "%"
	rows, err := db.Query(query, user.ID, albumName, pathPattern)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Error querying for songs in album."))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	var albumDuration int
	var albumCreated string
	for rows.Next() {
		var r SongResult
		var lastPlayed, genreVal, dateAdded sql.NullString
		var starred int
		var rgTrackGain, rgTrackPeak, rgAlbumGain, rgAlbumPeak sql.NullFloat64
		if err := rows.Scan(&r.ID, &r.Title, &r.Artist, &r.Album, &r.Path, &r.PlayCount, &lastPlayed, &genreVal, &r.Duration, &dateAdded,
			&rgTrackGain, &rgTrackPeak, &rgAlbumGain, &rgAlbumPeak, &r.Track, &r.Year, &r.DiscNumber,
			&r.Size, &r.BitRate, &r.SamplingRate, &r.ChannelCount, &r.BitDepth, &r.Comment, &starred); err != nil {
			log.Printf("Error scanning song in getAlbum: %v", err)
			continue
		}
		if lastPlayed.Valid {
			r.LastPlayed = lastPlayed.String
		}
		if genreVal.Valid {
			r.Genre = genreVal.String
		}
		if dateAdded.Valid {
			r.Created = dateAdded.String
		}
		r.Starred = starred == 1
		r.ReplayGain = newReplayGain(rgTrackGain, rgTrackPeak, rgAlbumGain, rgAlbumPeak)
		// The album's representative id and display artist override the
		// per-row derivations so all songs share a consistent album context.
		r.AlbumID = albumSongId
		r.AlbumArtist = displayArtist

		albumDuration += r.Duration
		if r.Created != "" && (albumCreated == "" || r.Created < albumCreated) {
			albumCreated = r.Created
		}

		s := buildSubsonicSong(r)
		s.CoverArt = albumSongId // Songs share the album cover
		songs = append(songs, s)
	}

	log.Printf("getAlbum: Returning %d songs for album '%s'", len(songs), albumName)

	responseBody := &SubsonicAlbumWithSongs{
		ID:            albumSongId,
		Name:          albumName,
		Artist:        displayArtist,
		ArtistID:      GenerateArtistID(displayArtist),
		CoverArt:      albumSongId,
		SongCount:     len(songs),
		Duration:      albumDuration,
		Created:       albumCreated,
		Genre:         albumGenre,
		DisplayArtist: displayArtist,
	}
	if albumGenre != "" {
		responseBody.Genres = []SubsonicItemGenre{{Name: albumGenre}}
	}
	responseBody.Songs = songs

	subsonicRespond(c, newSubsonicResponse(responseBody))
}

func subsonicGetSong(c *gin.Context) {
	user := c.MustGet("user").(User)

	songID := c.Query("id")
	if songID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Missing required parameter 'id'"))
		return
	}

	// Use the central song query so the response carries every spec-aligned
	// Child field (suffix, contentType, created, genres, replayGain, etc.).
	results, err := QuerySongs(db, SongQueryOptions{
		IDs:            []string{songID},
		IncludeGenre:   true,
		IncludeStarred: true,
		UserID:         user.ID,
		Limit:          1,
	})
	if err != nil {
		log.Printf("Error querying for song in getSong: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}
	if len(results) == 0 {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Song not found."))
		return
	}

	s := buildSubsonicSong(results[0])

	subsonicRespond(c, newSubsonicResponse(&SubsonicSongWrapper{Song: s}))
}

func subsonicGetRandomSongs(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	if size > 500 {
		size = 500
	}

	results, err := QuerySongs(db, SongQueryOptions{
		Random: true,
		Limit:  size,
	})
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error fetching random songs."))
		return
	}

	var songs []SubsonicSong
	for _, result := range results {
		songs = append(songs, buildSubsonicSong(result))
	}

	if songs == nil {
		songs = []SubsonicSong{}
	}
	responseBody := &SubsonicRandomSongs{Songs: songs}
	subsonicRespond(c, newSubsonicResponse(responseBody))
}

func subsonicGetCoverArt(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	sizeStr := c.DefaultQuery("size", "512")
	size, err := strconv.Atoi(sizeStr)
	if err != nil {
		size = 512 // Default on parse error
	}

	// Check if ID exists in songs table (song/album ID)
	exists, err := SongExists(db, id)
	if err == nil && exists {
		handleAlbumArt(c, id, size)
		return
	}

	// Try to resolve as artist ID (MD5 hash) to artist name
	if name, ok := resolveArtistIDToName(db, id); ok {
		handleArtistArt(c, name, size)
		return
	}

	// If not found as artist ID, treat as artist name directly (backward compatibility)
	handleArtistArt(c, id, size)
}

func handleAlbumArt(c *gin.Context, songID string, size int) {
	path, err := QuerySongPath(db, songID)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	log.Printf("[COVER ART] Found path for song ID %s: %s", songID, path)

	file, err := os.Open(path)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	defer file.Close()

	meta, err := tag.ReadFrom(file)
	if err != nil {
		log.Printf("INFO: unable to read tags for cover art in %s: %v", path, err)
	} else if meta != nil && meta.Picture() != nil {
		pic := meta.Picture()
		log.Printf("[COVER ART] Found embedded picture in %s", path)
		resizeAndServeImage(c, bytes.NewReader(pic.Data), pic.MIMEType, size)
		return
	}

	albumDir := filepath.Dir(path)
	if imagePath, ok := findLocalImage(albumDir); ok {
		log.Printf("[COVER ART] Found local image file: %s", imagePath)
		localFile, err := os.Open(imagePath)
		if err == nil {
			defer localFile.Close()
			resizeAndServeImage(c, localFile, http.DetectContentType(nil), size)
			return
		}
	}

	log.Printf("[COVER ART] No cover art found for song ID %s", songID)
	c.Status(http.StatusNotFound)
}

func handleArtistArt(c *gin.Context, artistName string, size int) {
	// Only use local files in artist directory - no external API calls
	var songPath string
	err := db.QueryRow("SELECT path FROM songs WHERE artist = ? AND cancelled = 0 LIMIT 1", artistName).Scan(&songPath)
	if err == nil {
		artistDir := filepath.Dir(songPath)
		if imagePath, ok := findLocalImage(artistDir); ok {
			localFile, err := os.Open(imagePath)
			if err == nil {
				defer localFile.Close()
				log.Printf("[ARTIST ART] Found local image for '%s': %s", artistName, imagePath)
				resizeAndServeImage(c, localFile, http.DetectContentType(nil), size)
				return
			}
		}
	}

	log.Printf("[ARTIST ART] No local image found for '%s'. Returning 404.", artistName)
	c.Status(http.StatusNotFound)
}

func findLocalImage(dir string) (string, bool) {
	imageNames := []string{"cover.jpg", "cover.png", "folder.jpg", "front.jpg", "artist.jpg", "artist.png"}
	for _, name := range imageNames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}

func resizeAndServeImage(c *gin.Context, reader io.Reader, contentType string, size int) {
	// Read all data first so we can retry with different decoders
	data, err := io.ReadAll(reader)
	if err != nil {
		log.Printf("[RESIZE] Failed to read image data: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	// Try to decode image (supports JPEG, PNG, GIF, TIFF, BMP)
	img, err := imaging.Decode(bytes.NewReader(data))
	if err != nil {
		log.Printf("[RESIZE] Unsupported image format, serving original: %v", err)
		// Serve original image without resizing (better than 500 error!)
		c.Header("Content-Type", contentType)
		c.Data(http.StatusOK, contentType, data)
		return
	}

	// Resize image
	resizedImg := imaging.Fit(img, size, size, imaging.Lanczos)

	// Determine output format
	var format imaging.Format
	switch contentType {
	case "image/jpeg", "image/jpg":
		format = imaging.JPEG
	case "image/png":
		format = imaging.PNG
	case "image/gif":
		format = imaging.GIF
	case "image/tiff":
		format = imaging.TIFF
	case "image/bmp":
		format = imaging.BMP
	default:
		// Unknown format - convert to JPEG
		format = imaging.JPEG
		contentType = "image/jpeg"
	}

	c.Header("Content-Type", contentType)
	err = imaging.Encode(c.Writer, resizedImg, format)
	if err != nil {
		log.Printf("[RESIZE] Failed to encode resized image: %v", err)
		c.Status(http.StatusInternalServerError)
	}
}

// subsonicStar handles starring of songs, albums, and artists according to Open Subsonic API
func subsonicStar(c *gin.Context) {
	user := c.MustGet("user").(User)

	// Can star multiple items at once - use Query() to get all values for repeated parameters
	// Subsonic sends: id=1&id=2&albumId=3&artistId=4
	songIDs := c.Request.URL.Query()["id"]
	albumIDs := c.Request.URL.Query()["albumId"]
	artistIDs := c.Request.URL.Query()["artistId"]

	if len(songIDs) == 0 && len(albumIDs) == 0 && len(artistIDs) == 0 {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter is missing."))
		return
	}

	now := time.Now().Format(time.RFC3339)

	// Star songs
	for _, songID := range songIDs {
		// Check if song exists
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM songs WHERE id = ? AND cancelled = 0)", songID).Scan(&exists)
		if err != nil || !exists {
			log.Printf("Song %s not found for starring", songID)
			continue
		}

		_, err = db.Exec(`INSERT OR REPLACE INTO starred_songs (user_id, song_id, starred_at) VALUES (?, ?, ?)`,
			user.ID, songID, now)
		if err != nil {
			log.Printf("Error starring song %s for user %s: %v", songID, user.Username, err)
		} else {
			log.Printf("Song %s starred by user %s", songID, user.Username)
		}
	}

	// Star albums
	for _, albumID := range albumIDs {
		// Check if album exists (albumID is actually a song ID representing the album)
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM songs WHERE id = ? AND cancelled = 0)", albumID).Scan(&exists)
		if err != nil || !exists {
			log.Printf("Album %s not found for starring", albumID)
			continue
		}

		_, err = db.Exec(`INSERT OR REPLACE INTO starred_albums (user_id, album_id, starred_at) VALUES (?, ?, ?)`,
			user.ID, albumID, now)
		if err != nil {
			log.Printf("Error starring album %s for user %s: %v", albumID, user.Username, err)
		} else {
			log.Printf("Album %s starred by user %s", albumID, user.Username)
		}
	}

	// Star artists
	for _, artistID := range artistIDs {
		// artistID may be an artist name or an artist ID (generated by GenerateArtistID)
		artistName := artistID

		// First, check if artistID directly matches an artist name in songs (consider album_artist fallback)
		exists, err := ArtistExists(db, artistName)
		if err != nil {
			log.Printf("Artist check error for %s: %v", artistID, err)
			continue
		}

		// If direct match not found, try resolving by artist ID (MD5 hash)
		if !exists {
			name, resolved := resolveArtistIDToName(db, artistID)
			if !resolved {
				log.Printf("Artist %s not found for starring", artistID)
				continue
			}
			artistName = name
			log.Printf("Resolved artist ID %s to artist name %s", artistID, artistName)
		}

		err = StarArtist(db, user.ID, artistName, now)
		if err != nil {
			log.Printf("Error starring artist %s for user %s: %v", artistName, user.Username, err)
		} else {
			log.Printf("Artist %s starred by user %s", artistName, user.Username)
		}
	}

	subsonicRespond(c, newSubsonicResponse(nil))
}

// subsonicUnstar handles unstarring of songs, albums, and artists according to Open Subsonic API
func subsonicUnstar(c *gin.Context) {
	user := c.MustGet("user").(User)

	// Can unstar multiple items at once - use Query() to get all values for repeated parameters
	songIDs := c.Request.URL.Query()["id"]
	albumIDs := c.Request.URL.Query()["albumId"]
	artistIDs := c.Request.URL.Query()["artistId"]

	if len(songIDs) == 0 && len(albumIDs) == 0 && len(artistIDs) == 0 {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter is missing."))
		return
	}

	// Unstar songs
	for _, songID := range songIDs {
		err := UnstarSong(db, user.ID, songID)
		if err != nil {
			log.Printf("Error unstarring song %s for user %s: %v", songID, user.Username, err)
		} else {
			log.Printf("Song %s unstarred by user %s", songID, user.Username)
		}
	}

	// Unstar albums
	for _, albumID := range albumIDs {
		err := UnstarAlbum(db, user.ID, albumID)
		if err != nil {
			log.Printf("Error unstarring album %s for user %s: %v", albumID, user.Username, err)
		} else {
			log.Printf("Album %s unstarred by user %s", albumID, user.Username)
		}
	}

	// Unstar artists
	for _, artistID := range artistIDs {
		artistName := artistID
		exists, err := ArtistExists(db, artistName)
		if err != nil {
			log.Printf("Artist check error for %s: %v", artistID, err)
			continue
		}
		if !exists {
			name, resolved := resolveArtistIDToName(db, artistID)
			if !resolved {
				log.Printf("Artist %s not found for un-starring", artistID)
				continue
			}
			artistName = name
		}

		err = UnstarArtist(db, user.ID, artistName)
		if err != nil {
			log.Printf("Error unstarring artist %s for user %s: %v", artistName, user.Username, err)
		} else {
			log.Printf("Artist %s unstarred by user %s", artistName, user.Username)
		}
	}

	subsonicRespond(c, newSubsonicResponse(nil))
}

// collectStarred gathers the current user's starred songs, albums and artists.
// Shared by getStarred (<starred>) and getStarred2 (<starred2>); on a DB error
// it responds with a Subsonic error and returns ok=false.
func collectStarred(c *gin.Context, user User) (songsOut []SubsonicSong, albumsOut []SubsonicAlbum, artistsOut []SubsonicArtist, ok bool) {
	// Get starred songs (deduplicated by song_id in case of duplicate starred_songs entries)
	query := `
		SELECT s.id, s.title, s.artist, s.album, s.path, s.play_count, s.last_played, COALESCE(s.genre, '') as genre, COALESCE(s.duration, 0) as duration,
			COALESCE(s.album_artist, ''), COALESCE(s.date_added, ''),
			s.replaygain_track_gain, s.replaygain_track_peak, s.replaygain_album_gain, s.replaygain_album_peak,
			(SELECT MIN(s2.id) FROM songs s2 WHERE s2.album_path = s.album_path AND s2.cancelled = 0) AS album_id,
			COALESCE(s.track, 0), COALESCE(s.year, 0), COALESCE(s.disc_number, 0),
			COALESCE(s.size, 0), COALESCE(s.bitrate, 0), COALESCE(s.sample_rate, 0), COALESCE(s.channels, 0), COALESCE(s.bit_depth, 0), COALESCE(s.comment, '')
		FROM songs s
		INNER JOIN (
			SELECT song_id, MAX(starred_at) as starred_at
			FROM starred_songs
			WHERE user_id = ?
			GROUP BY song_id
		) ss ON s.id = ss.song_id
		WHERE s.cancelled = 0
		ORDER BY ss.starred_at DESC
	`

	rows, err := db.Query(query, user.ID)
	if err != nil {
		log.Printf("Starred songs query error: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return nil, nil, nil, false
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var r SongResult
		var lastPlayed, genreVal, albumArtist, created, albumID sql.NullString
		var rgTrackGain, rgTrackPeak, rgAlbumGain, rgAlbumPeak sql.NullFloat64
		var trackInt, yearInt, discInt sql.NullInt64
		err := rows.Scan(&r.ID, &r.Title, &r.Artist, &r.Album, &r.Path, &r.PlayCount, &lastPlayed, &genreVal, &r.Duration,
			&albumArtist, &created, &rgTrackGain, &rgTrackPeak, &rgAlbumGain, &rgAlbumPeak, &albumID,
			&trackInt, &yearInt, &discInt,
			&r.Size, &r.BitRate, &r.SamplingRate, &r.ChannelCount, &r.BitDepth, &r.Comment)
		if err != nil {
			log.Printf("Error scanning starred song: %v", err)
			continue
		}
		r.Track = int(trackInt.Int64)
		r.Year = int(yearInt.Int64)
		r.DiscNumber = int(discInt.Int64)
		if lastPlayed.Valid {
			r.LastPlayed = lastPlayed.String
		}
		if genreVal.Valid {
			r.Genre = genreVal.String
		}
		if albumArtist.Valid {
			r.AlbumArtist = albumArtist.String
		}
		if created.Valid {
			r.Created = created.String
		}
		if albumID.Valid {
			r.AlbumID = albumID.String
		}
		r.Starred = true
		r.ReplayGain = newReplayGain(rgTrackGain, rgTrackPeak, rgAlbumGain, rgAlbumPeak)
		songs = append(songs, buildSubsonicSong(r))
	}

	// Get starred albums
	albumQuery := `
		SELECT s.album, s.artist, COALESCE(s.genre, ''), sa.album_id
		FROM starred_albums sa
		INNER JOIN songs s ON sa.album_id = s.id
		WHERE sa.user_id = ?
		GROUP BY sa.album_id
		ORDER BY sa.starred_at DESC
	`

	albumRows, err := db.Query(albumQuery, user.ID)
	var albums []SubsonicAlbum
	if err == nil {
		defer albumRows.Close()
		for albumRows.Next() {
			var a SubsonicAlbum
			err := albumRows.Scan(&a.Name, &a.Artist, &a.Genre, &a.ID)
			if err == nil {
				a.ArtistID = GenerateArtistID(a.Artist)
				a.CoverArt = a.ID
				decorateAlbum(&a)
				albums = append(albums, a)
			}
		}
	}

	// Get starred artists
	artistQuery := `
		SELECT artist_name
		FROM starred_artists
		WHERE user_id = ?
		ORDER BY starred_at DESC
	`

	artistRows, err := db.Query(artistQuery, user.ID)
	var artists []SubsonicArtist
	if err == nil {
		defer artistRows.Close()
		for artistRows.Next() {
			var artistName string
			if err := artistRows.Scan(&artistName); err == nil {
				artistID := GenerateArtistID(artistName)
				artists = append(artists, SubsonicArtist{
					ID:       artistID,
					Name:     artistName,
					CoverArt: artistID, // Use artist ID for getCoverArt
				})
			}
		}
	}

	// Ensure slices are non-nil
	if songs == nil {
		songs = []SubsonicSong{}
	}
	if albums == nil {
		albums = []SubsonicAlbum{}
	}
	if artists == nil {
		artists = []SubsonicArtist{}
	}

	return songs, albums, artists, true
}

// subsonicGetStarred returns starred songs, albums and artists (<starred>).
func subsonicGetStarred(c *gin.Context) {
	user := c.MustGet("user").(User)
	songs, albums, artists, ok := collectStarred(c, user)
	if !ok {
		return
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicStarred{
		Artists: artists,
		Albums:  albums,
		Songs:   songs,
	}))
}

// subsonicGetStarred2 returns starred songs, albums and artists in ID3 form (<starred2>).
func subsonicGetStarred2(c *gin.Context) {
	user := c.MustGet("user").(User)
	songs, albums, artists, ok := collectStarred(c, user)
	if !ok {
		return
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicStarred2{
		Songs:   songs,
		Albums:  albums,
		Artists: artists,
	}))
}

// subsonicGetGenres returns all genres in the library
func subsonicGetGenres(c *gin.Context) {
	user := c.MustGet("user").(User)
	log.Printf("subsonicGetGenres called by user: %s", user.Username)

	// First, let's check if we have any songs at all
	var totalSongs int
	err := db.QueryRow("SELECT COUNT(*) FROM songs WHERE cancelled = 0").Scan(&totalSongs)
	if err != nil {
		log.Printf("Error counting total songs: %v", err)
	} else {
		log.Printf("Total songs in database: %d", totalSongs)
	}

	// OPTIMIZED: Single query with proper album count (no N+1 queries)
	query := `
		SELECT
			COALESCE(genre, 'Unknown') as genre,
			COUNT(*) as song_count,
			COUNT(DISTINCT CASE
				WHEN album != '' AND album_path != '' THEN album_path || '|||' || album
				WHEN album != '' THEN album
				ELSE NULL
			END) as album_count
		FROM songs
		WHERE cancelled = 0
		GROUP BY COALESCE(genre, 'Unknown')
		ORDER BY genre COLLATE NOCASE
	`

	log.Printf("Executing genre query: %s", query)
	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Genre query error: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}
	defer rows.Close()

	var genres []SubsonicGenre
	for rows.Next() {
		var g SubsonicGenre
		err := rows.Scan(&g.Name, &g.SongCount, &g.AlbumCount)
		if err != nil {
			log.Printf("Error scanning genre: %v", err)
			continue
		}
		genres = append(genres, g)
	}

	// Ensure genres is never nil for JSON marshaling
	if genres == nil {
		genres = []SubsonicGenre{}
	}

	log.Printf("Found %d genres", len(genres))

	// Add a test genre to ensure response structure works
	if len(genres) == 0 {
		log.Printf("No genres found, adding test genre")
		genres = append(genres, SubsonicGenre{
			Name:       "Test",
			SongCount:  0,
			AlbumCount: 0,
		})
	}

	genreList := &SubsonicGenres{Genres: genres}
	log.Printf("About to respond with genres: %+v", genreList)
	subsonicRespond(c, newSubsonicResponse(genreList))
}

// subsonicGetSongsByGenre handles the getSongsByGenre.view API endpoint
func subsonicGetSongsByGenre(c *gin.Context) {
	user := c.MustGet("user").(User)

	genre := c.Query("genre")
	log.Printf("[DEBUG] *** getSongsByGenre ENDPOINT CALLED *** Genre: '%s', User: %d", genre, user.ID)

	if genre == "" {
		log.Printf("[DEBUG] *** NO GENRE PROVIDED ***")
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter genre is missing."))
		return
	}

	log.Printf("[DEBUG] getSongsByGenre: Looking for genre '%s' for user %d", genre, user.ID)

	// Debug: Check what genres actually exist
	var sampleGenres []string
	testRows, err := db.Query("SELECT DISTINCT genre FROM songs WHERE genre IS NOT NULL AND genre != '' AND cancelled = 0 LIMIT 10")
	if err == nil {
		for testRows.Next() {
			var g string
			if testRows.Scan(&g) == nil {
				sampleGenres = append(sampleGenres, g)
			}
		}
		testRows.Close()
		log.Printf("[DEBUG] Sample genres in database: %v", sampleGenres)
	}

	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	if size > 500 {
		size = 500
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if offset < 0 {
		offset = 0
	}

	// Simple test: just get any songs with genres first
	query := `
		SELECT s.id, s.title, s.artist, s.album, s.path, s.play_count, s.last_played, COALESCE(s.genre, ''), s.duration,
		       COALESCE(s.album_artist, ''), COALESCE(s.date_added, ''),
		       s.replaygain_track_gain, s.replaygain_track_peak, s.replaygain_album_gain, s.replaygain_album_peak,
		       (SELECT MIN(s2.id) FROM songs s2 WHERE s2.album_path = s.album_path AND s2.cancelled = 0) AS album_id,
		       COALESCE(s.track, 0), COALESCE(s.year, 0), COALESCE(s.disc_number, 0),
		       COALESCE(s.size, 0), COALESCE(s.bitrate, 0), COALESCE(s.sample_rate, 0), COALESCE(s.channels, 0), COALESCE(s.bit_depth, 0), COALESCE(s.comment, ''),
		       CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred
		FROM songs s
		LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ?
		WHERE s.genre IS NOT NULL AND s.genre != '' AND LOWER(s.genre) LIKE LOWER(?)
		ORDER BY s.artist, s.title
		LIMIT ? OFFSET ?
	`

	// Simple pattern - just check if genre contains the search term anywhere
	genrePattern := "%" + genre + "%"

	log.Printf("[DEBUG] getSongsByGenre: Simple query with pattern: '%s'", genrePattern)

	rows, err := db.Query(query, user.ID, genrePattern, size, offset)
	if err != nil {
		log.Printf("[ERROR] getSongsByGenre: Query failed: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error querying songs by genre."))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var r SongResult
		var lastPlayed, genreVal, albumArtist, created, albumID sql.NullString
		var rgTrackGain, rgTrackPeak, rgAlbumGain, rgAlbumPeak sql.NullFloat64
		var trackInt, yearInt, discInt sql.NullInt64
		var starred int

		if err := rows.Scan(&r.ID, &r.Title, &r.Artist, &r.Album,
			&r.Path, &r.PlayCount, &lastPlayed, &genreVal, &r.Duration,
			&albumArtist, &created, &rgTrackGain, &rgTrackPeak, &rgAlbumGain, &rgAlbumPeak, &albumID,
			&trackInt, &yearInt, &discInt,
			&r.Size, &r.BitRate, &r.SamplingRate, &r.ChannelCount, &r.BitDepth, &r.Comment, &starred); err != nil {
			log.Printf("[ERROR] getSongsByGenre: Scan failed: %v", err)
			continue
		}
		r.Track = int(trackInt.Int64)
		r.Year = int(yearInt.Int64)
		r.DiscNumber = int(discInt.Int64)
		if lastPlayed.Valid {
			r.LastPlayed = lastPlayed.String
		}
		if genreVal.Valid {
			r.Genre = genreVal.String
		}
		if albumArtist.Valid {
			r.AlbumArtist = albumArtist.String
		}
		if created.Valid {
			r.Created = created.String
		}
		if albumID.Valid {
			r.AlbumID = albumID.String
		}
		r.Starred = starred == 1
		r.ReplayGain = newReplayGain(rgTrackGain, rgTrackPeak, rgAlbumGain, rgAlbumPeak)

		songs = append(songs, buildSubsonicSong(r))
	}

	// Ensure songs is never nil for JSON marshaling
	if songs == nil {
		songs = []SubsonicSong{}
	}

	log.Printf("[DEBUG] getSongsByGenre: Found %d songs for genre '%s'", len(songs), genre)

	result := &SubsonicSongsByGenre{Songs: songs}
	subsonicRespond(c, newSubsonicResponse(result))
}
