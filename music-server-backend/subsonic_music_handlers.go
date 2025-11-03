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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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

func subsonicStream(c *gin.Context) {
	user := c.MustGet("user").(User)

	songID := c.Query("id")
	var path string
	err := db.QueryRow("SELECT path FROM songs WHERE id = ?", songID).Scan(&path)
	if err != nil {
		if err == sql.ErrNoRows {
			subsonicRespond(c, newSubsonicErrorResponse(70, "Song not found."))
			return
		}
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}

	// Check if user has transcoding enabled
	var transcodingEnabled int
	var format string
	var bitrate int
	err = db.QueryRow("SELECT enabled, format, bitrate FROM transcoding_settings WHERE user_id = ?", user.ID).
		Scan(&transcodingEnabled, &format, &bitrate)

	useTranscoding := err == nil && transcodingEnabled == 1

	log.Printf("🎧 Stream request: user=%s, song=%s, transcoding_enabled=%v, format=%s, bitrate=%d",
		user.Username, filepath.Base(path), useTranscoding, format, bitrate)

	if useTranscoding {
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
	http.ServeContent(c.Writer, c.Request, fileInfo.Name(), fileInfo.ModTime(), file)
}

func streamWithTranscoding(c *gin.Context, inputPath string, format string, bitrate int) {
	log.Printf("🎵 TRANSCODING ACTIVE: format=%s, bitrate=%dkbps, file=%s", format, bitrate, filepath.Base(inputPath))

	// Map format to FFmpeg codec and file extension
	codecMap := map[string]string{
		"mp3":  "libmp3lame",
		"ogg":  "libvorbis",
		"aac":  "aac",
		"opus": "libopus",
	}
	extMap := map[string]string{
		"mp3":  "mp3",
		"ogg":  "ogg",
		"aac":  "m4a",
		"opus": "opus",
	}

	codec, ok := codecMap[format]
	if !ok {
		log.Printf("❌ Unsupported transcoding format: %s - falling back to direct stream", format)
		streamDirect(c, inputPath)
		return
	}

	ext := extMap[format]
	bitrateStr := strconv.Itoa(bitrate) + "k"

	// Build FFmpeg command
	args := []string{
		"-i", inputPath,
		"-vn", // No video
		"-acodec", codec,
		"-b:a", bitrateStr,
		"-f", ext,
		"pipe:1", // Output to stdout
	}

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
		log.Printf("❌ Failed to create FFmpeg stdout pipe: %v - falling back to direct stream", err)
		streamDirect(c, inputPath)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("❌ Failed to start FFmpeg: %v - falling back to direct stream", err)
		streamDirect(c, inputPath)
		return
	}

	// Set appropriate content type
	contentTypes := map[string]string{
		"mp3":  "audio/mpeg",
		"ogg":  "audio/ogg",
		"aac":  "audio/aac",
		"opus": "audio/opus",
	}
	contentType := contentTypes[format]
	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "none") // Transcoding doesn't support range requests
	c.Header("X-Transcoded", "true")  // Custom header to indicate transcoding
	c.Header("X-Transcode-Format", format)
	c.Header("X-Transcode-Bitrate", bitrateStr)

	log.Printf("✅ Streaming transcoded audio: Content-Type=%s, Bitrate=%s", contentType, bitrateStr)

	// Stream transcoded output to client
	bytesWritten, _ := io.Copy(c.Writer, stdout)

	cmd.Wait()

	log.Printf("✅ Transcoding complete: %d bytes sent", bytesWritten)
}

func subsonicScrobble(c *gin.Context) {
	user := c.MustGet("user").(User)

	songID := c.Query("id")
	if songID == "" {
		subsonicRespond(c, newSubsonicResponse(nil))
		return
	}

	now := time.Now().Format(time.RFC3339)

	_, err := db.Exec(`
		UPDATE songs
		SET play_count = play_count + 1, last_played = ?
		WHERE id = ?
	`, now, songID)

	if err != nil {
		log.Printf("Error updating play count for user '%s' on song '%s': %v", user.Username, songID, err)
	}

	// Track play history for this user
	_, err = db.Exec(`
		INSERT INTO play_history (user_id, song_id, played_at)
		VALUES (?, ?, ?)
	`, user.ID, songID, now)

	if err != nil {
		log.Printf("Error inserting play history for user '%s' on song '%s': %v", user.Username, songID, err)
	}

	log.Printf("Scrobbled song '%s' for user '%s'", songID, user.Username)
	subsonicRespond(c, newSubsonicResponse(nil))
}

func subsonicGetArtists(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	query := `
		SELECT
			s.artist,
			COUNT(DISTINCT s.album)
		FROM songs s
		WHERE s.artist != ''
		GROUP BY s.artist
		ORDER BY s.artist COLLATE NOCASE
	`
	rows, err := db.Query(query)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error querying artists."))
		return
	}
	defer rows.Close()

	artistIndex := make(map[string][]SubsonicArtist)
	for rows.Next() {
		var artist SubsonicArtist
		if err := rows.Scan(&artist.Name, &artist.AlbumCount); err != nil {
			log.Printf("Error scanning artist for subsonicGetArtists: %v", err)
			continue
		}
		artist.ID = artist.Name
		artist.CoverArt = artist.Name

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

func subsonicGetAlbumList2(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware
	// Respect size/offset params and return empty list when offset exceeds total (Navidrome-like behavior)
	sizeParam := c.DefaultQuery("size", "500")
	offsetParam := c.DefaultQuery("offset", "0")
	genreParam := c.Query("genre")

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

	// Build query with optional genre filter
	whereClause := "WHERE album != ''"
	var args []interface{}
	if genreParam != "" {
		whereClause += " AND genre = ?"
		args = append(args, genreParam)
	}

	// Count distinct albums
	var totalAlbums int
	countQuery := fmt.Sprintf("SELECT COUNT(DISTINCT album || '~~' || artist) FROM songs %s", whereClause)
	err = db.QueryRow(countQuery, args...).Scan(&totalAlbums)
	if err != nil {
		log.Printf("Error counting albums for pagination: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error querying albums."))
		return
	}

	// If offset is beyond total, return empty result (like Navidrome)
	if offset >= totalAlbums {
		responseBody := &SubsonicAlbumList2{Albums: []SubsonicAlbum{}}
		response := newSubsonicResponse(responseBody)
		subsonicRespond(c, response)
		return
	}

	// Query with LIMIT/OFFSET for pagination
	query := fmt.Sprintf("SELECT album, artist, COALESCE(genre, ''), MIN(id) FROM songs %s GROUP BY album, artist ORDER BY artist, album LIMIT ? OFFSET ?", whereClause)
	args = append(args, size, offset)
	rows, err := db.Query(query, args...)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error querying albums."))
		return
	}
	defer rows.Close()

	var albums []SubsonicAlbum
	for rows.Next() {
		var album SubsonicAlbum
		var albumId int
		if err := rows.Scan(&album.Name, &album.Artist, &album.Genre, &albumId); err != nil {
			log.Printf("Error scanning album row for subsonicGetAlbumList2: %v", err)
			continue
		}
		album.ID = strconv.Itoa(albumId)
		album.CoverArt = album.ID
		albums = append(albums, album)
	}

	responseBody := &SubsonicAlbumList2{Albums: albums}
	response := newSubsonicResponse(responseBody)
	subsonicRespond(c, response)
}

func subsonicGetAlbum(c *gin.Context) {
	user := c.MustGet("user").(User)

	albumSongId := c.Query("id")
	if albumSongId == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Missing required parameter 'id'"))
		return
	}

	var albumName, artistName, albumGenre string
	err := db.QueryRow("SELECT album, artist, COALESCE(genre, '') FROM songs WHERE id = ?", albumSongId).Scan(&albumName, &artistName, &albumGenre)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Album not found."))
		return
	}

	query := `
		SELECT s.id, s.title, s.artist, s.album, s.play_count, s.last_played, COALESCE(s.genre, ''), 
		       CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred
		FROM songs s
		LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ?
		WHERE s.album = ? AND s.artist = ? 
		ORDER BY s.title
	`

	rows, err := db.Query(query, user.ID, albumName, artistName)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Error querying for songs in album."))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var s SubsonicSong
		var lastPlayed sql.NullString
		var songId int
		var starred int
		if err := rows.Scan(&songId, &s.Title, &s.Artist, &s.Album, &s.PlayCount, &lastPlayed, &s.Genre, &starred); err != nil {
			log.Printf("Error scanning song in getAlbum: %v", err)
			continue
		}
		s.ID = strconv.Itoa(songId)
		s.CoverArt = albumSongId
		s.Starred = starred == 1
		if lastPlayed.Valid {
			s.LastPlayed = lastPlayed.String
		}
		songs = append(songs, s)
	}

	responseBody := &SubsonicAlbumWithSongs{
		ID:        albumSongId,
		Name:      albumName,
		CoverArt:  albumSongId,
		SongCount: len(songs),
		Songs:     songs,
	}

	subsonicRespond(c, newSubsonicResponse(responseBody))
}

func subsonicGetSong(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	songID := c.Query("id")
	if songID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Missing required parameter 'id'"))
		return
	}

	var s SubsonicSong
	var lastPlayed sql.NullString
	var songId int

	// Get the song details from the database
	err := db.QueryRow(`
		SELECT id, title, artist, album, play_count, last_played
		FROM songs WHERE id = ?`, songID).Scan(&songId, &s.Title, &s.Artist, &s.Album, &s.PlayCount, &lastPlayed)

	if err != nil {
		if err == sql.ErrNoRows {
			subsonicRespond(c, newSubsonicErrorResponse(70, "Song not found."))
		} else {
			log.Printf("Error querying for song in getSong: %v", err)
			subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		}
		return
	}

	s.ID = strconv.Itoa(songId)
	if lastPlayed.Valid {
		s.LastPlayed = lastPlayed.String
	}
	s.CoverArt = s.ID // A song can serve as its own cover art reference

	subsonicRespond(c, newSubsonicResponse(&SubsonicSongWrapper{Song: s}))
}

func subsonicGetRandomSongs(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	if size > 500 {
		size = 500
	}

	rows, err := db.Query("SELECT id, title, artist, album, play_count, last_played FROM songs ORDER BY RANDOM() LIMIT ?", size)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error fetching random songs."))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var s SubsonicSong
		var songId int
		var lastPlayed sql.NullString
		if err := rows.Scan(&songId, &s.Title, &s.Artist, &s.Album, &s.PlayCount, &lastPlayed); err != nil {
			log.Printf("Error scanning random song: %v", err)
			continue
		}
		s.ID = strconv.Itoa(songId)
		s.CoverArt = s.ID
		if lastPlayed.Valid {
			s.LastPlayed = lastPlayed.String
		}
		songs = append(songs, s)
	}

	responseBody := &SubsonicDirectory{
		Name:      "Random Songs",
		SongCount: len(songs),
		Songs:     songs,
	}
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

	if _, err := strconv.Atoi(id); err == nil {
		handleAlbumArt(c, id, size)
	} else {
		handleArtistArt(c, id, size)
	}
}

func handleAlbumArt(c *gin.Context, songID string, size int) {
	var path string
	err := db.QueryRow("SELECT path FROM songs WHERE id = ?", songID).Scan(&path)
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
	if err == nil && meta.Picture() != nil {
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
	if imageUrl, err := fetchArtistImageFromMusicBrainz(artistName, size); err == nil && imageUrl != "" {
		log.Printf("[ARTIST ART] Found image on MusicBrainz for '%s'. Proxying URL: %s", artistName, imageUrl)
		proxyImage(c, imageUrl)
		return
	} else if err != nil {
		log.Printf("[ARTIST ART] MusicBrainz lookup for '%s' failed: %v", artistName, err)
	} else {
		log.Printf("[ARTIST ART] No image found on MusicBrainz for '%s'", artistName)
	}

	var songPath string
	err := db.QueryRow("SELECT path FROM songs WHERE artist = ? LIMIT 1", artistName).Scan(&songPath)
	if err == nil {
		artistDir := filepath.Dir(songPath)
		if imagePath, ok := findLocalImage(artistDir); ok {
			localFile, err := os.Open(imagePath)
			if err == nil {
				defer localFile.Close()
				resizeAndServeImage(c, localFile, http.DetectContentType(nil), size)
				return
			}
		}
	}

	log.Printf("[ARTIST ART] All methods failed for '%s'. Returning 404.", artistName)
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
	img, err := imaging.Decode(reader)
	if err != nil {
		log.Printf("[RESIZE] Failed to decode image: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	resizedImg := imaging.Fit(img, size, size, imaging.Lanczos)

	var format imaging.Format
	switch contentType {
	case "image/jpeg":
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
		format = imaging.JPEG // Default to JPEG
	}

	c.Header("Content-Type", contentType)
	err = imaging.Encode(c.Writer, resizedImg, format)
	if err != nil {
		log.Printf("[RESIZE] Failed to encode resized image: %v", err)
	}
}

func fetchArtistImageFromMusicBrainz(artistName string, size int) (string, error) {
	searchURL := fmt.Sprintf("https://musicbrainz.org/ws/2/artist/?query=artist:%s&fmt=json", url.QueryEscape(artistName))
	log.Printf("[MBrainz] Constructed search URL: %s", searchURL)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create MusicBrainz request: %w", err)
	}
	req.Header.Set("User-Agent", "AudioMuse-AI/0.1.0 ( https://audiomuse.ai/ )")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute MusicBrainz request: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[MBrainz] Received status code %d from artist search API", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("musicbrainz artist search returned non-200 status: %d - %s", resp.StatusCode, string(bodyBytes))
	}

	var searchResult struct {
		Artists []struct {
			ID    string `json:"id"`
			Score int    `json:"score"`
		} `json:"artists"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResult); err != nil {
		return "", fmt.Errorf("failed to decode MusicBrainz artist search response: %w", err)
	}

	if len(searchResult.Artists) == 0 || searchResult.Artists[0].Score < 90 {
		log.Printf("[MBrainz] No artist found or score too low for '%s'", artistName)
		return "", nil
	}

	artistID := searchResult.Artists[0].ID
	log.Printf("[MBrainz] Found artist ID '%s' for '%s'", artistID, artistName)

	lookupURL := fmt.Sprintf("https://musicbrainz.org/ws/2/artist/%s?inc=url-rels&fmt=json", artistID)
	log.Printf("[MBrainz] Constructed lookup URL: %s", lookupURL)

	req, err = http.NewRequest("GET", lookupURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create MusicBrainz lookup request: %w", err)
	}
	req.Header.Set("User-Agent", "AudioMuse-AI/0.1.0 ( https://audiomuse.ai/ )")
	resp, err = client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute MusicBrainz lookup request: %w", err)
	}
	defer resp.Body.Close()

	log.Printf("[MBrainz] Received status code %d from artist lookup API", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("musicbrainz artist lookup returned non-200 status: %d", resp.StatusCode)
	}

	var lookupResult struct {
		Relations []struct {
			Type string `json:"type"`
			URL  struct {
				Resource string `json:"resource"`
			} `json:"url"`
		} `json:"relations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&lookupResult); err != nil {
		return "", fmt.Errorf("failed to decode MusicBrainz lookup response: %w", err)
	}

	for _, rel := range lookupResult.Relations {
		if rel.Type == "image" {
			imageUrl := rel.URL.Resource
			if strings.Contains(imageUrl, "commons.wikimedia.org/wiki/File:") {
				fileName := filepath.Base(imageUrl)
				finalUrl := fmt.Sprintf("https://commons.wikimedia.org/w/index.php?title=Special:Redirect/file/%s&width=%d", url.QueryEscape(fileName), size)
				log.Printf("[MBrainz] Found wikimedia image, transforming to direct link: %s", finalUrl)
				return finalUrl, nil
			}
		}
	}

	log.Printf("[MBrainz] No 'image' type relation found for artist ID %s", artistID)
	return "", nil
}

func proxyImage(c *gin.Context, imageUrl string) {
	log.Printf("[PROXY] Proxying image from URL: %s", imageUrl)
	resp, err := http.Get(imageUrl)
	if err != nil {
		log.Printf("[PROXY] Error fetching image for proxy: %v", err)
		c.Status(http.StatusNotFound)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[PROXY] Remote server returned non-200 status: %d for URL: %s", resp.StatusCode, imageUrl)
		c.Status(resp.StatusCode)
		return
	}

	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Header("Cache-Control", "public, max-age=86400")

	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		log.Printf("[PROXY] Error copying image stream to response: %v", err)
	}
}

// subsonicStar handles starring of songs according to Open Subsonic API
func subsonicStar(c *gin.Context) {
	user := c.MustGet("user").(User)

	songID := c.Query("id")
	if songID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter is missing."))
		return
	}

	// Check if song exists
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM songs WHERE id = ?)", songID).Scan(&exists)
	if err != nil || !exists {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Song not found."))
		return
	}

	// Add star (ignore if already exists)
	_, err = db.Exec(`INSERT OR REPLACE INTO starred_songs (user_id, song_id, starred_at) VALUES (?, ?, ?)`,
		user.ID, songID, time.Now().Format(time.RFC3339))
	if err != nil {
		log.Printf("Error starring song %s for user %s: %v", songID, user.Username, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}

	log.Printf("Song %s starred by user %s", songID, user.Username)
	subsonicRespond(c, newSubsonicResponse(nil))
}

// subsonicUnstar handles unstarring of songs according to Open Subsonic API
func subsonicUnstar(c *gin.Context) {
	user := c.MustGet("user").(User)

	songID := c.Query("id")
	if songID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter is missing."))
		return
	}

	// Remove star
	_, err := db.Exec(`DELETE FROM starred_songs WHERE user_id = ? AND song_id = ?`, user.ID, songID)
	if err != nil {
		log.Printf("Error unstarring song %s for user %s: %v", songID, user.Username, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}

	log.Printf("Song %s unstarred by user %s", songID, user.Username)
	subsonicRespond(c, newSubsonicResponse(nil))
}

// subsonicGetStarred returns starred songs for the current user
func subsonicGetStarred(c *gin.Context) {
	user := c.MustGet("user").(User)
	log.Printf("subsonicGetStarred called by user: %s (ID: %d)", user.Username, user.ID)

	// First check if there are any starred songs for this user
	var starredCount int
	err := db.QueryRow("SELECT COUNT(*) FROM starred_songs WHERE user_id = ?", user.ID).Scan(&starredCount)
	if err != nil {
		log.Printf("Error counting starred songs: %v", err)
	} else {
		log.Printf("Total starred songs for user %d: %d", user.ID, starredCount)
	}

	query := `
		SELECT s.id, s.title, s.artist, s.album, s.play_count, s.last_played, COALESCE(s.genre, '') as genre
		FROM songs s
		INNER JOIN starred_songs ss ON s.id = ss.song_id
		WHERE ss.user_id = ?
		ORDER BY ss.starred_at DESC
	`

	log.Printf("Executing starred songs query for user ID: %d", user.ID)
	rows, err := db.Query(query, user.ID)
	if err != nil {
		log.Printf("Starred songs query error: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var s SubsonicSong
		var lastPlayed sql.NullString
		err := rows.Scan(&s.ID, &s.Title, &s.Artist, &s.Album, &s.PlayCount, &lastPlayed, &s.Genre)
		if err != nil {
			log.Printf("Error scanning starred song: %v", err)
			continue
		}

		s.Starred = true
		if lastPlayed.Valid {
			s.LastPlayed = lastPlayed.String
		}
		songs = append(songs, s)
	}

	// Ensure songs is never nil for JSON marshaling
	if songs == nil {
		songs = []SubsonicSong{}
	}

	log.Printf("Found %d starred songs for user %d", len(songs), user.ID)

	// Add a test song if no starred songs found
	if len(songs) == 0 {
		log.Printf("No starred songs found, adding test song")
		songs = append(songs, SubsonicSong{
			ID:      "test",
			Title:   "Test Song",
			Artist:  "Test Artist",
			Album:   "Test Album",
			Genre:   "Test",
			Starred: true,
		})
	}

	starred := &SubsonicStarred{Songs: songs}
	log.Printf("About to respond with starred songs: %+v", starred)
	subsonicRespond(c, newSubsonicResponse(starred))
}

// subsonicGetGenres returns all genres in the library
func subsonicGetGenres(c *gin.Context) {
	user := c.MustGet("user").(User)
	log.Printf("subsonicGetGenres called by user: %s", user.Username)

	// First, let's check if we have any songs at all
	var totalSongs int
	err := db.QueryRow("SELECT COUNT(*) FROM songs").Scan(&totalSongs)
	if err != nil {
		log.Printf("Error counting total songs: %v", err)
	} else {
		log.Printf("Total songs in database: %d", totalSongs)
	}

	query := `
		SELECT COALESCE(genre, 'Unknown') as genre, COUNT(*) as song_count, COUNT(DISTINCT album) as album_count
		FROM songs 
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
	testRows, err := db.Query("SELECT DISTINCT genre FROM songs WHERE genre IS NOT NULL AND genre != '' LIMIT 10")
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
		SELECT s.id, s.title, s.artist, s.album, s.path, s.play_count, s.last_played, COALESCE(s.genre, ''),
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
		var songFromDb Song
		var lastPlayed sql.NullString
		var starred int

		if err := rows.Scan(&songFromDb.ID, &songFromDb.Title, &songFromDb.Artist, &songFromDb.Album,
			&songFromDb.Path, &songFromDb.PlayCount, &lastPlayed, &songFromDb.Genre, &starred); err != nil {
			log.Printf("[ERROR] getSongsByGenre: Scan failed: %v", err)
			continue
		}

		subsonicSong := SubsonicSong{
			ID:        strconv.Itoa(songFromDb.ID),
			Title:     songFromDb.Title,
			Artist:    songFromDb.Artist,
			Album:     songFromDb.Album,
			Genre:     songFromDb.Genre,
			CoverArt:  strconv.Itoa(songFromDb.ID),
			PlayCount: songFromDb.PlayCount,
			Starred:   starred == 1,
		}

		log.Printf("[DEBUG] Found song: ID=%d, Title='%s', Genre='%s'", songFromDb.ID, songFromDb.Title, songFromDb.Genre)

		if lastPlayed.Valid {
			subsonicSong.LastPlayed = lastPlayed.String
		}

		songs = append(songs, subsonicSong)
	}

	// Ensure songs is never nil for JSON marshaling
	if songs == nil {
		songs = []SubsonicSong{}
	}

	log.Printf("[DEBUG] getSongsByGenre: Found %d songs for genre '%s'", len(songs), genre)

	result := &SubsonicSongsByGenre{Songs: songs}
	subsonicRespond(c, newSubsonicResponse(result))
}
