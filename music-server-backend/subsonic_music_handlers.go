// Suggested path: music-server-backend/subsonic_music_handlers.go
package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/gin-gonic/gin"
)

// subsonicGetArtists handles the getArtists.view API endpoint.
func subsonicGetArtists(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	rows, err := db.Query("SELECT DISTINCT artist FROM songs WHERE artist != '' ORDER BY artist")
	if err != nil {
		log.Printf("[ERROR] subsonicGetArtists: Database query failed: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var artists []SubsonicArtist
	for rows.Next() {
		var artistName string
		if err := rows.Scan(&artistName); err != nil {
			log.Printf("Error scanning artist row for Subsonic: %v", err)
			continue
		}
		artists = append(artists, SubsonicArtist{ID: artistName, Name: artistName})
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicArtists{Artists: artists}))
}

// subsonicGetAlbumList2 handles the getAlbumList2.view API endpoint.
func subsonicGetAlbumList2(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	listType := c.DefaultQuery("type", "alphabeticalByName")
	sizeStr := c.DefaultQuery("size", "500")
	offsetStr := c.DefaultQuery("offset", "0")
	artistFilter := c.Query("id") // Used for filtering by artist name

	size, _ := strconv.Atoi(sizeStr)
	offset, _ := strconv.Atoi(offsetStr)

	log.Printf("[DEBUG] subsonicGetAlbumList2: Running query with type '%s', size %d, offset %d, artist '%s'", listType, size, offset, artistFilter)

	var query string
	args := []interface{}{}

	baseQuery := `
        SELECT
            s.album,
            s.artist,
            MIN(s.id) as representativeSongId
        FROM songs s
    `
	var conditions []string
	if artistFilter != "" {
		conditions = append(conditions, "s.artist = ?")
		args = append(args, artistFilter)
	}
	conditions = append(conditions, "s.album != ''")

	if len(conditions) > 0 {
		baseQuery += " WHERE " + strings.Join(conditions, " AND ")
	}

	baseQuery += " GROUP BY s.album, s.artist"

	switch listType {
	case "newest":
		query = baseQuery + " ORDER BY MAX(s.date_added) DESC LIMIT ? OFFSET ?"
	default: // alphabeticalByName
		query = baseQuery + " ORDER BY s.album LIMIT ? OFFSET ?"
	}
	args = append(args, size, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("[ERROR] subsonicGetAlbumList2: Database query failed: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var albums []SubsonicAlbum
	for rows.Next() {
		var albumName, artistName string
		var representativeSongID int
		if err := rows.Scan(&albumName, &artistName, &representativeSongID); err != nil {
			log.Printf("Error scanning album row for Subsonic: %v", err)
			continue
		}
		albumIDStr := strconv.Itoa(representativeSongID)
		albums = append(albums, SubsonicAlbum{ID: albumIDStr, Name: albumName, Artist: artistName, CoverArt: albumIDStr})
	}
	log.Printf("[DEBUG] subsonicGetAlbumList2: Found %d albums.", len(albums))
	subsonicRespond(c, newSubsonicResponse(&SubsonicAlbumList2{Albums: albums}))
}

// subsonicGetAlbum handles the getAlbum.view API endpoint.
func subsonicGetAlbum(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	albumID := c.Query("id")
	if albumID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter 'id' is missing."))
		return
	}

	// First, find the album name using the representative song ID
	var albumName string
	err := db.QueryRow("SELECT album FROM songs WHERE id = ?", albumID).Scan(&albumName)
	if err != nil {
		log.Printf("[ERROR] subsonicGetAlbum: Could not find album for ID %s: %v", albumID, err)
		subsonicRespond(c, newSubsonicErrorResponse(70, "Album not found."))
		return
	}

	log.Printf("[DEBUG] subsonicGetAlbum: Fetching songs for album '%s' (ID: %s)", albumName, albumID)

	rows, err := db.Query("SELECT id, title, artist, album, path, play_count, last_played FROM songs WHERE album = ? ORDER BY title", albumName)
	if err != nil {
		log.Printf("[ERROR] subsonicGetAlbum: Database query failed: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var songFromDb Song
		var lastPlayed sql.NullString
		if err := rows.Scan(&songFromDb.ID, &songFromDb.Title, &songFromDb.Artist, &songFromDb.Album, &songFromDb.Path, &songFromDb.PlayCount, &lastPlayed); err != nil {
			log.Printf("Error scanning song row for Subsonic getAlbum: %v", err)
			continue
		}
		song := SubsonicSong{
			ID:        strconv.Itoa(songFromDb.ID),
			Title:     songFromDb.Title,
			Artist:    songFromDb.Artist,
			Album:     songFromDb.Album,
			PlayCount: songFromDb.PlayCount,
			Path:      songFromDb.Path, // Include path for client use
		}
		if lastPlayed.Valid {
			song.LastPlayed = lastPlayed.String
		}
		song.CoverArt = albumID // Use the representative ID for cover art requests
		songs = append(songs, song)
	}
	log.Printf("[DEBUG] subsonicGetAlbum: Found %d songs for album '%s'.", len(songs), albumName)

	album := SubsonicAlbumWithSongs{ID: albumID, Name: albumName, CoverArt: albumID, SongCount: len(songs), Songs: songs}
	subsonicRespond(c, newSubsonicResponse(&album))
}

// subsonicGetRandomSongs handles the getRandomSongs.view API endpoint.
func subsonicGetRandomSongs(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	sizeStr := c.DefaultQuery("size", "50")
	size, err := strconv.Atoi(sizeStr)
	if err != nil || size <= 0 {
		size = 50
	}

	rows, err := db.Query("SELECT id, title, artist, album, path, play_count, last_played FROM songs ORDER BY RANDOM() LIMIT ?", size)
	if err != nil {
		log.Printf("[ERROR] subsonicGetRandomSongs: Database query failed: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var songFromDb Song
		var lastPlayed sql.NullString
		if err := rows.Scan(&songFromDb.ID, &songFromDb.Title, &songFromDb.Artist, &songFromDb.Album, &songFromDb.Path, &songFromDb.PlayCount, &lastPlayed); err != nil {
			log.Printf("Error scanning song row for Subsonic getRandomSongs: %v", err)
			continue
		}
		song := SubsonicSong{
			ID:        strconv.Itoa(songFromDb.ID),
			Title:     songFromDb.Title,
			Artist:    songFromDb.Artist,
			Album:     songFromDb.Album,
			PlayCount: songFromDb.PlayCount,
		}
		if lastPlayed.Valid {
			song.LastPlayed = lastPlayed.String
		}
		// Find a representative song ID for the album to use for cover art
		var albumSongId int
		db.QueryRow("SELECT id FROM songs WHERE album = ? LIMIT 1", song.Album).Scan(&albumSongId)
		song.CoverArt = strconv.Itoa(albumSongId)
		songs = append(songs, song)
	}

	directory := SubsonicDirectory{SongCount: len(songs), Songs: songs}
	subsonicRespond(c, newSubsonicResponse(&directory))
}

// subsonicGetCoverArt handles the getCoverArt.view API endpoint.
func subsonicGetCoverArt(c *gin.Context) {
	albumSongID := c.Query("id")
	if albumSongID == "" || albumSongID == "undefined" {
		c.Status(http.StatusBadRequest)
		return
	}

	var songPath, albumName string
	err := db.QueryRow("SELECT path, album FROM songs WHERE id = ? LIMIT 1", albumSongID).Scan(&songPath, &albumName)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	file, err := os.Open(songPath)
	if err != nil {
		// If file doesn't exist, redirect to placeholder
		redirectToPlaceholder(c, albumName)
		return
	}
	defer file.Close()

	meta, err := tag.ReadFrom(file)
	if err != nil || meta.Picture() == nil {
		// If no tags or no picture, redirect to placeholder
		redirectToPlaceholder(c, albumName)
		return
	}

	pic := meta.Picture()
	c.Data(http.StatusOK, pic.MIMEType, pic.Data)
}

// redirectToPlaceholder generates a placeholder image URL and redirects the client.
func redirectToPlaceholder(c *gin.Context, text string) {
	// Use an external service to generate placeholder images
	encodedText := url.QueryEscape(text)
	placeholderURL := fmt.Sprintf("https://placehold.co/300x300/2d3748/ffffff/png?text=%s", encodedText)
	c.Redirect(http.StatusFound, placeholderURL)
}

// subsonicStream handles the stream.view API endpoint.
func subsonicStream(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}
	songID := c.Query("id")
	log.Printf("[DEBUG] subsonicStream: User '%s' requesting stream for song ID: %s", user.Username, songID)

	var path string
	err := db.QueryRow("SELECT path FROM songs WHERE id = ?", songID).Scan(&path)
	if err != nil {
		log.Printf("[ERROR] subsonicStream: Failed to find path for song ID %s. Error: %v", songID, err)
		subsonicRespond(c, newSubsonicErrorResponse(70, "The requested data was not found."))
		return
	}

	log.Printf("[DEBUG] subsonicStream: Found path '%s' for song ID %s. Attempting to serve file.", path, songID)

	// Update play count and last played time
	go func() {
		_, err := db.Exec("UPDATE songs SET play_count = play_count + 1, last_played = ? WHERE id = ?", time.Now().Format(time.RFC3339), songID)
		if err != nil {
			log.Printf("[ERROR] Failed to update play count for song ID %s: %v", songID, err)
		}
	}()

	file, err := os.Open(path)
	if err != nil {
		log.Printf("[ERROR] subsonicStream: Could not open file for streaming %s: %v", path, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error: could not open file"))
		return
	}
	defer file.Close()
	fileInfo, _ := file.Stat()
	http.ServeContent(c.Writer, c.Request, fileInfo.Name(), fileInfo.ModTime(), file)
}

