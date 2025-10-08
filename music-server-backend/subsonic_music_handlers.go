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
	_ = c.MustGet("user") // Auth is handled by middleware

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

func subsonicScrobble(c *gin.Context) {
	user := c.MustGet("user").(User)
	_ = user // Auth is handled by middleware

	songID := c.Query("id")
	if songID == "" {
		subsonicRespond(c, newSubsonicResponse(nil))
		return
	}

	_, err := db.Exec(`
		UPDATE songs
		SET play_count = play_count + 1, last_played = ?
		WHERE id = ?
	`, time.Now().Format(time.RFC3339), songID)

	if err != nil {
		log.Printf("Error updating play count for user '%s' on song '%s': %v", user.Username, songID, err)
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

	// Count distinct albums
	var totalAlbums int
	err = db.QueryRow("SELECT COUNT(DISTINCT album || '~~' || artist) FROM songs WHERE album != ''").Scan(&totalAlbums)
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
	query := "SELECT album, artist, MIN(id) FROM songs WHERE album != '' GROUP BY album, artist ORDER BY artist, album LIMIT ? OFFSET ?"
	rows, err := db.Query(query, size, offset)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error querying albums."))
		return
	}
	defer rows.Close()

	var albums []SubsonicAlbum
	for rows.Next() {
		var album SubsonicAlbum
		var albumId int
		if err := rows.Scan(&album.Name, &album.Artist, &albumId); err != nil {
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
	_ = c.MustGet("user") // Auth is handled by middleware

	albumSongId := c.Query("id")
	if albumSongId == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Missing required parameter 'id'"))
		return
	}

	var albumName, artistName string
	err := db.QueryRow("SELECT album, artist FROM songs WHERE id = ?", albumSongId).Scan(&albumName, &artistName)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Album not found."))
		return
	}

	rows, err := db.Query("SELECT id, title, artist, album, play_count, last_played FROM songs WHERE album = ? AND artist = ? ORDER BY title", albumName, artistName)
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
		if err := rows.Scan(&songId, &s.Title, &s.Artist, &s.Album, &s.PlayCount, &lastPlayed); err != nil {
			log.Printf("Error scanning song in getAlbum: %v", err)
			continue
		}
		s.ID = strconv.Itoa(songId)
		s.CoverArt = albumSongId
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
