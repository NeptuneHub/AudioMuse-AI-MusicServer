// hls_transcoding.go - HLS transcoding session management
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// TranscodingSession represents an active HLS transcoding session
type TranscodingSession struct {
	SessionID      string
	SongID         string
	Format         string
	Bitrate        string
	FilePath       string
	SegmentDir     string
	CreatedAt      time.Time
	LastAccessedAt time.Time
	Duration       int // Total duration in seconds
	mu             sync.Mutex
}

// SessionManager manages all active transcoding sessions
type SessionManager struct {
	sessions sync.Map // map[sessionID]*TranscodingSession
}

var hlsSessionManager = &SessionManager{}

// Constants for HLS configuration
const (
	HLS_SEGMENT_DURATION = 10          // seconds per segment
	HLS_SESSION_TIMEOUT  = 5 * 60      // 5 minutes
	HLS_CLEANUP_INTERVAL = 1 * 60      // Check every 1 minute
	HLS_TEMP_DIR         = "hls_cache" // Directory for HLS segments
)

// StartSessionCleanup starts a background goroutine to clean up stale sessions
func StartSessionCleanup() {
	// Clean up any orphaned cache directories from previous server runs
	cleanupOrphanedCache()

	go func() {
		ticker := time.NewTicker(HLS_CLEANUP_INTERVAL * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			cleanupStaleSessions()
		}
	}()
	log.Println("🧹 HLS session cleanup started")
}

// cleanupOrphanedCache removes all cache directories on startup (since sessions are in-memory only)
func cleanupOrphanedCache() {
	if _, err := os.Stat(HLS_TEMP_DIR); os.IsNotExist(err) {
		// Cache directory doesn't exist, nothing to clean
		return
	}

	log.Println("🧹 Cleaning up orphaned HLS cache from previous server run...")

	entries, err := os.ReadDir(HLS_TEMP_DIR)
	if err != nil {
		log.Printf("⚠️  Failed to read HLS cache directory: %v", err)
		return
	}

	cleaned := 0
	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(HLS_TEMP_DIR, entry.Name())
			if err := os.RemoveAll(dirPath); err != nil {
				log.Printf("⚠️  Failed to remove cache directory %s: %v", dirPath, err)
			} else {
				cleaned++
			}
		}
	}

	if cleaned > 0 {
		log.Printf("🧹 Removed %d orphaned HLS cache directories", cleaned)
	}
}

// cleanupStaleSessions removes sessions that haven't been accessed recently
func cleanupStaleSessions() {
	now := time.Now()
	var toDelete []string

	hlsSessionManager.sessions.Range(func(key, value interface{}) bool {
		sessionID := key.(string)
		session := value.(*TranscodingSession)

		session.mu.Lock()
		if now.Sub(session.LastAccessedAt).Seconds() > HLS_SESSION_TIMEOUT {
			toDelete = append(toDelete, sessionID)
		}
		session.mu.Unlock()
		return true
	})

	for _, sessionID := range toDelete {
		if sessionVal, ok := hlsSessionManager.sessions.Load(sessionID); ok {
			session := sessionVal.(*TranscodingSession)
			log.Printf("🧹 Cleaning up stale HLS session: %s", sessionID)
			cleanupSession(session)
			hlsSessionManager.sessions.Delete(sessionID)
		}
	}

	if len(toDelete) > 0 {
		log.Printf("🧹 Cleaned up %d stale HLS sessions", len(toDelete))
	}
}

// cleanupSession removes all files and directories for a session
func cleanupSession(session *TranscodingSession) {
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.SegmentDir != "" {
		if err := os.RemoveAll(session.SegmentDir); err != nil {
			log.Printf("⚠️  Failed to remove HLS segment directory %s: %v", session.SegmentDir, err)
		}
	}
}

// getOrCreateSession gets an existing session or creates a new one
func getOrCreateSession(songID, format, bitrate, filePath string, duration int) (*TranscodingSession, error) {
	sessionID := fmt.Sprintf("%s_%s_%s", songID, format, bitrate)

	// Check if session exists and is still valid
	if sessionVal, ok := hlsSessionManager.sessions.Load(sessionID); ok {
		session := sessionVal.(*TranscodingSession)
		session.mu.Lock()
		session.LastAccessedAt = time.Now()
		session.mu.Unlock()
		return session, nil
	}

	// Create new session
	segmentDir := filepath.Join(HLS_TEMP_DIR, sessionID)
	if err := os.MkdirAll(segmentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create segment directory: %v", err)
	}

	session := &TranscodingSession{
		SessionID:      sessionID,
		SongID:         songID,
		Format:         format,
		Bitrate:        bitrate,
		FilePath:       filePath,
		SegmentDir:     segmentDir,
		CreatedAt:      time.Now(),
		LastAccessedAt: time.Now(),
		Duration:       duration,
	}

	hlsSessionManager.sessions.Store(sessionID, session)
	log.Printf("📺 Created new HLS session: %s (format=%s, bitrate=%s)", sessionID, format, bitrate)

	// HYBRID APPROACH: Instant playback + gapless quality
	// 1. Pre-encode first 3 segments immediately (instant start, ~2-3 seconds)
	// 2. Pre-encode remaining segments in background (gapless playback)
	log.Printf("🚀 Quick-encoding first 3 segments for instant playback...")
	if err := preEncodeFirstSegments(session, 3); err != nil {
		log.Printf("⚠️  Failed to quick-encode first segments: %v", err)
	}

	// Start background encoding for remaining segments (non-blocking)
	go func() {
		log.Printf("🎬 Background pre-encoding remaining segments for session %s", session.SessionID)
		if err := preEncodeHLSSegments(session); err != nil {
			log.Printf("⚠️  Background pre-encoding failed: %v (on-demand fallback active)", err)
		} else {
			log.Printf("✅ Background pre-encoding complete for session %s", session.SessionID)
		}
	}()

	return session, nil
}

// generateHLSPlaylist generates an M3U8 playlist for the session
func generateHLSPlaylist(c *gin.Context, session *TranscodingSession) {
	session.mu.Lock()
	session.LastAccessedAt = time.Now()
	session.mu.Unlock()

	// Calculate total number of segments
	totalSegments := (session.Duration + HLS_SEGMENT_DURATION - 1) / HLS_SEGMENT_DURATION

	// Build M3U8 playlist
	playlist := "#EXTM3U\n"
	playlist += "#EXT-X-VERSION:3\n"
	playlist += fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", HLS_SEGMENT_DURATION)
	playlist += "#EXT-X-MEDIA-SEQUENCE:0\n"
	playlist += "#EXT-X-PLAYLIST-TYPE:VOD\n"

	// Add segments
	for i := 0; i < totalSegments; i++ {
		segmentDuration := HLS_SEGMENT_DURATION
		if i == totalSegments-1 {
			// Last segment may be shorter
			segmentDuration = session.Duration - (i * HLS_SEGMENT_DURATION)
		}
		playlist += fmt.Sprintf("#EXTINF:%.3f,\n", float64(segmentDuration))

		// Construct segment URL with JWT authentication (same as playlist request)
		jwt := c.Query("jwt")
		segmentURL := fmt.Sprintf("/rest/hlsSegment.view?sessionId=%s&segment=%d&jwt=%s&v=%s&c=%s",
			session.SessionID,
			i,
			jwt,
			c.Query("v"),
			c.Query("c"))
		playlist += segmentURL + "\n"
	}

	playlist += "#EXT-X-ENDLIST\n"

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.String(200, playlist)
}

// preEncodeFirstSegments quickly encodes first N segments for instant playback
// Uses on-demand encoding for speed, background process will replace with gapless versions
func preEncodeFirstSegments(session *TranscodingSession, count int) error {
	totalSegments := (session.Duration + HLS_SEGMENT_DURATION - 1) / HLS_SEGMENT_DURATION
	if count > totalSegments {
		count = totalSegments
	}

	log.Printf("⚡ Quick-encoding first %d segments for instant playback", count)

	for i := 0; i < count; i++ {
		startTime := i * HLS_SEGMENT_DURATION
		segmentPath := filepath.Join(session.SegmentDir, fmt.Sprintf("segment_%d.ts", i))

		// Check if already exists (from previous session or background encoding)
		if _, err := os.Stat(segmentPath); err == nil {
			log.Printf("✅ Segment %d already exists, skipping", i)
			continue
		}

		if err := generateSegment(session, i, segmentPath, startTime); err != nil {
			return fmt.Errorf("failed to encode segment %d: %v", i, err)
		}
		log.Printf("✅ Quick-encoded segment %d/%d", i+1, count)
	}

	return nil
}

// generateHLSSegment generates a specific HLS segment on-demand
func generateHLSSegment(c *gin.Context, session *TranscodingSession, segmentNum int) {
	session.mu.Lock()
	session.LastAccessedAt = time.Now()
	session.mu.Unlock()

	// Calculate start time for this segment
	startTime := segmentNum * HLS_SEGMENT_DURATION

	// Segment file path
	segmentPath := filepath.Join(session.SegmentDir, fmt.Sprintf("segment_%d.ts", segmentNum))

	// Check if segment already exists
	if _, err := os.Stat(segmentPath); err == nil {
		// Segment exists, serve it immediately
		log.Printf("✅ Serving cached HLS segment %d for session %s", segmentNum, session.SessionID)
		c.File(segmentPath)
		return
	}

	// Generate segment using FFmpeg
	log.Printf("🎬 Generating HLS segment %d for session %s (start=%ds)", segmentNum, session.SessionID, startTime)

	// Generate this segment ON-DEMAND (backend has 10 seconds to generate each segment)
	if err := generateSegment(session, segmentNum, segmentPath, startTime); err != nil {
		log.Printf("❌ Segment generation failed: %v", err)
		c.String(500, "Segment generation failed")
		return
	}

	// Serve the generated segment
	c.File(segmentPath)
}

// preEncodeHLSSegments pre-encodes ALL segments using FFmpeg's HLS muxer
// This ensures seamless playback with no gaps by maintaining continuous encoder state
func preEncodeHLSSegments(session *TranscodingSession) error {
	// Check if already fully encoded
	playlistPath := filepath.Join(session.SegmentDir, "playlist.m3u8")
	if _, err := os.Stat(playlistPath); err == nil {
		log.Printf("✅ Session %s already pre-encoded, skipping", session.SessionID)
		return nil
	}

	log.Printf("🎬 Pre-encoding HLS segments for session %s", session.SessionID)

	// Convert bitrate string to int
	bitrateInt, err := strconv.Atoi(session.Bitrate)
	if err != nil {
		log.Printf("⚠️  Invalid bitrate: %s, using default 192", session.Bitrate)
		bitrateInt = 192
	}

	// Build FFmpeg command for HLS muxer (BEST PRACTICE for gapless audio)
	var ffmpegArgs []string

	// Input file
	ffmpegArgs = append(ffmpegArgs, "-i", session.FilePath)

	// Get base transcoding profile (audio codec settings)
	profileArgs := getTranscodingProfile(session.Format, bitrateInt)
	ffmpegArgs = append(ffmpegArgs, profileArgs...)

	// CRITICAL: HLS-specific settings for gapless audio playback
	ffmpegArgs = append(ffmpegArgs,
		// Timestamp handling - CRITICAL for seamless transitions
		"-copyts",                         // Copy timestamps from input
		"-start_at_zero",                  // Start timestamps at zero
		"-avoid_negative_ts", "make_zero", // Handle AAC encoder priming delay
		"-async", "1", // Audio sync correction

		// HLS muxer settings
		"-f", "hls", // HLS output format
		"-hls_time", fmt.Sprintf("%d", HLS_SEGMENT_DURATION), // Segment duration
		"-hls_list_size", "0", // Keep all segments in playlist
		"-hls_segment_type", "mpegts", // Use MPEG-TS for audio
		"-hls_flags", "split_by_time+independent_segments", // Accurate splitting + independent segments
		"-hls_segment_filename", filepath.Join(session.SegmentDir, "segment_%d.ts"), // Segment naming

		// Output playlist path
		filepath.Join(session.SegmentDir, "playlist.m3u8"),
	)

	// Run FFmpeg in background
	cmd := exec.Command("ffmpeg", ffmpegArgs...)

	// Capture output for debugging
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("FFmpeg HLS muxer failed: %v\nOutput: %s", err, string(output))
	}

	log.Printf("✅ Pre-encoded HLS segments for session %s", session.SessionID)
	return nil
}

// generateSegment generates a single HLS segment on-demand (FALLBACK ONLY)
// NOTE: This approach can cause audio gaps - pre-encoding is preferred
func generateSegment(session *TranscodingSession, segmentNum int, segmentPath string, startTime int) error {
	var ffmpegArgs []string

	// Input file
	ffmpegArgs = append(ffmpegArgs, "-i", session.FilePath)

	// Seek to segment start (AFTER input to avoid cutting frames)
	if startTime > 0 {
		ffmpegArgs = append(ffmpegArgs, "-ss", fmt.Sprintf("%d", startTime))
	}

	// Segment duration (exact, no overlap - overlap causes artifacts!)
	ffmpegArgs = append(ffmpegArgs, "-t", fmt.Sprintf("%d", HLS_SEGMENT_DURATION))

	// Convert bitrate string to int for getTranscodingProfile
	bitrateInt, err := strconv.Atoi(session.Bitrate)
	if err != nil {
		log.Printf("⚠️  Invalid bitrate: %s, using default 192", session.Bitrate)
		bitrateInt = 192
	}

	// Use the same transcoding profile as your existing streaming system
	profileArgs := getTranscodingProfile(session.Format, bitrateInt)
	ffmpegArgs = append(ffmpegArgs, profileArgs...)

	// CRITICAL: Add timestamp handling to minimize gaps
	ffmpegArgs = append(ffmpegArgs,
		"-avoid_negative_ts", "make_zero", // Handle AAC encoder delay
		"-async", "1", // Audio sync correction
	)

	// Output format: MPEG-TS for HLS
	ffmpegArgs = append(ffmpegArgs,
		"-f", "mpegts",
		segmentPath,
	)

	cmd := exec.Command("ffmpeg", ffmpegArgs...)

	// Run FFmpeg
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("FFmpeg failed: %v\nOutput: %s", err, string(output))
	}

	return nil
}

// Subsonic HLS playlist handler
func subsonicHLSPlaylist(c *gin.Context) {
	songID := c.Query("id")
	if songID == "" {
		c.String(400, "Missing id parameter")
		return
	}

	log.Printf("📺 HLS Playlist request for song ID: %s", songID)

	// Get song from database (using correct table and column names)
	var filePath string
	var duration int
	err := db.QueryRow("SELECT path, duration FROM songs WHERE id = ?", songID).Scan(&filePath, &duration)
	if err != nil {
		log.Printf("❌ Song not found in database: %s (error: %v)", songID, err)
		c.String(404, "Song not found")
		return
	}

	log.Printf("📺 Found song: %s, duration: %d seconds", filePath, duration)

	// Get format and bitrate parameters
	format := c.DefaultQuery("format", "mp3")
	bitrate := c.DefaultQuery("maxBitRate", "192")

	// Validate format is supported by our transcoding system
	supportedFormats := map[string]bool{
		"mp3":  true,
		"ogg":  true,
		"aac":  true,
		"opus": true,
	}
	if !supportedFormats[format] {
		log.Printf("❌ Unsupported HLS transcoding format: %s, using mp3", format)
		format = "mp3"
	}

	// Get or create session
	session, err := getOrCreateSession(songID, format, bitrate, filePath, duration)
	if err != nil {
		log.Printf("❌ Failed to create HLS session: %v", err)
		c.String(500, "Failed to create transcoding session")
		return
	}

	// Generate and return playlist
	generateHLSPlaylist(c, session)
}

// Subsonic HLS segment handler
func subsonicHLSSegment(c *gin.Context) {
	sessionID := c.Query("sessionId")
	if sessionID == "" {
		c.String(400, "Missing sessionId parameter")
		return
	}

	segmentNumStr := c.Query("segment")
	if segmentNumStr == "" {
		c.String(400, "Missing segment parameter")
		return
	}

	segmentNum, err := strconv.Atoi(segmentNumStr)
	if err != nil {
		c.String(400, "Invalid segment number")
		return
	}

	// Get session
	sessionVal, ok := hlsSessionManager.sessions.Load(sessionID)
	if !ok {
		log.Printf("❌ HLS session not found: %s", sessionID)
		c.String(404, "Session not found or expired")
		return
	}

	session := sessionVal.(*TranscodingSession)

	// Generate and serve segment
	generateHLSSegment(c, session, segmentNum)
}
