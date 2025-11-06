// Suggested path: music-server-backend/subsonic_media_info_handlers.go
package main

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
)

// subsonicGetTopSongs returns the most played songs for an artist
func subsonicGetTopSongs(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	artistName := c.Query("artist")
	if artistName == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter artist is missing."))
		return
	}

	count, _ := strconv.Atoi(c.DefaultQuery("count", "50"))
	if count > 500 {
		count = 500
	}

	log.Printf("getTopSongs called for artist: %s, count: %d", artistName, count)

	query := `
		SELECT id, title, artist, album, play_count, last_played, COALESCE(genre, '')
		FROM songs
		WHERE artist = ?
		ORDER BY play_count DESC, title COLLATE NOCASE
		LIMIT ?
	`

	rows, err := db.Query(query, artistName, count)
	if err != nil {
		log.Printf("Error querying top songs for artist %s: %v", artistName, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var songID string
		var title, artist, album, genre string
		var playCount int
		var lastPlayed sql.NullString

		if err := rows.Scan(&songID, &title, &artist, &album, &playCount, &lastPlayed, &genre); err != nil {
			log.Printf("Error scanning top song: %v", err)
			continue
		}

		song := SubsonicSong{
			ID:        songID,
			Title:     title,
			Artist:    artist,
			Album:     album,
			Genre:     genre,
			CoverArt:  songID,
			PlayCount: playCount,
		}

		if lastPlayed.Valid {
			song.LastPlayed = lastPlayed.String
		}

		songs = append(songs, song)
	}

	// Ensure songs is never nil for JSON marshaling
	if songs == nil {
		songs = []SubsonicSong{}
	}

	response := newSubsonicResponse(&SubsonicTopSongs{Songs: songs})
	subsonicRespond(c, response)
}

// subsonicGetSimilarSongs2 returns songs similar to a given song (based on artist and genre)
func subsonicGetSimilarSongs2(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	songID := c.Query("id")
	if songID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter id is missing."))
		return
	}

	count, _ := strconv.Atoi(c.DefaultQuery("count", "50"))
	if count > 500 {
		count = 500
	}

	// Get the reference song's artist and genre
	var refArtist, refGenre string
	err := db.QueryRow("SELECT artist, COALESCE(genre, '') FROM songs WHERE id = ?", songID).Scan(&refArtist, &refGenre)
	if err != nil {
		log.Printf("Reference song not found for getSimilarSongs2: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(70, "Song not found."))
		return
	}

	log.Printf("getSimilarSongs2 called for song ID: %s (artist: %s, genre: %s), count: %d", songID, refArtist, refGenre, count)

	// Find similar songs by artist or genre, excluding the original song
	query := `
		SELECT id, title, artist, album, play_count, last_played, COALESCE(genre, ''), duration
		FROM songs
		WHERE id != ?
		  AND (artist = ? OR (genre = ? AND genre != ''))
		ORDER BY 
			CASE 
				WHEN artist = ? THEN 0 
				WHEN genre = ? THEN 1 
				ELSE 2 
			END,
			RANDOM()
		LIMIT ?
	`

	rows, err := db.Query(query, songID, refArtist, refGenre, refArtist, refGenre, count)
	if err != nil {
		log.Printf("Error querying similar songs: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var id string
		var title, artist, album, genre string
		var playCount int
		var duration int
		var lastPlayed sql.NullString

		if err := rows.Scan(&id, &title, &artist, &album, &playCount, &lastPlayed, &genre, &duration); err != nil {
			log.Printf("Error scanning similar song: %v", err)
			continue
		}

		song := SubsonicSong{
			ID:        id,
			Title:     title,
			Artist:    artist,
			Album:     album,
			Genre:     genre,
			CoverArt:  id,
			PlayCount: playCount,
			Duration:  duration,
		}

		if lastPlayed.Valid {
			song.LastPlayed = lastPlayed.String
		}

		songs = append(songs, song)
	}

	// Ensure songs is never nil for JSON marshaling
	if songs == nil {
		songs = []SubsonicSong{}
	}

	response := newSubsonicResponse(&SubsonicSimilarSongs{Songs: songs})
	subsonicRespond(c, response)
}

// subsonicDownload downloads a song or creates a zip archive of an album
func subsonicDownload(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	id := c.Query("id")
	if id == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter id is missing."))
		return
	}

	log.Printf("download called for ID: %s", id)

	// Check if this is a single song or an album reference
	var path, albumName, artistName string
	err := db.QueryRow("SELECT path, album, artist FROM songs WHERE id = ?", id).Scan(&path, &albumName, &artistName)
	if err != nil {
		log.Printf("Song not found for download: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(70, "Song not found."))
		return
	}

	// Check if request wants the whole album by checking for multiple songs
	var albumSongCount int
	err = db.QueryRow("SELECT COUNT(*) FROM songs WHERE album = ? AND artist = ?", albumName, artistName).Scan(&albumSongCount)
	if err != nil || albumSongCount <= 1 {
		// Single song download
		downloadSingleFile(c, path)
		return
	}

	// Multiple songs - create zip archive of the album
	downloadAlbumAsZip(c, albumName, artistName)
}

// downloadSingleFile serves a single file for download
func downloadSingleFile(c *gin.Context, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Error opening file for download: %v", err)
		c.Status(500)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("Error getting file info: %v", err)
		c.Status(500)
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(filePath)))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	io.Copy(c.Writer, file)
}

// downloadAlbumAsZip creates a zip archive of all songs in an album
func downloadAlbumAsZip(c *gin.Context, albumName, artistName string) {
	// Query all songs in the album
	query := `
		SELECT id, title, path
		FROM songs
		WHERE album = ? AND artist = ?
		ORDER BY title COLLATE NOCASE
	`

	rows, err := db.Query(query, albumName, artistName)
	if err != nil {
		log.Printf("Error querying songs for album zip: %v", err)
		c.Status(500)
		return
	}
	defer rows.Close()

	// Collect song paths
	var songs []struct {
		ID    int
		Title string
		Path  string
	}

	for rows.Next() {
		var song struct {
			ID    int
			Title string
			Path  string
		}
		if err := rows.Scan(&song.ID, &song.Title, &song.Path); err != nil {
			log.Printf("Error scanning song for zip: %v", err)
			continue
		}
		songs = append(songs, song)
	}

	if len(songs) == 0 {
		c.Status(404)
		return
	}

	// Set headers for zip download
	zipFilename := fmt.Sprintf("%s - %s.zip", artistName, albumName)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", zipFilename))
	c.Header("Content-Type", "application/zip")

	// Create zip writer
	zipWriter := zip.NewWriter(c.Writer)
	defer zipWriter.Close()

	// Add each song to the zip
	for _, song := range songs {
		file, err := os.Open(song.Path)
		if err != nil {
			log.Printf("Error opening song file for zip: %v", err)
			continue
		}

		// Create entry in zip
		zipEntry, err := zipWriter.Create(filepath.Base(song.Path))
		if err != nil {
			log.Printf("Error creating zip entry: %v", err)
			file.Close()
			continue
		}

		// Copy file content to zip
		_, err = io.Copy(zipEntry, file)
		file.Close()

		if err != nil {
			log.Printf("Error copying file to zip: %v", err)
			continue
		}
	}

	log.Printf("Created zip archive with %d songs for album: %s", len(songs), albumName)
}

// subsonicGetAlbumInfo returns metadata about an album
// For now, we return basic info from our database
func subsonicGetAlbumInfo(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	id := c.Query("id")
	if id == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter id is missing."))
		return
	}

	log.Printf("getAlbumInfo called for ID: %s", id)

	// Get album info from a song in the album
	var albumName, artistName, genre string
	err := db.QueryRow("SELECT album, artist, COALESCE(genre, '') FROM songs WHERE id = ?", id).Scan(&albumName, &artistName, &genre)
	if err != nil {
		log.Printf("Album not found for getAlbumInfo: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(70, "Album not found."))
		return
	}

	// Count songs in album
	var songCount int
	err = db.QueryRow("SELECT COUNT(*) FROM songs WHERE album = ? AND artist = ?", albumName, artistName).Scan(&songCount)
	if err != nil {
		songCount = 0
	}

	// Build album info response
	albumInfo := &SubsonicAlbumInfo{
		Notes:          fmt.Sprintf("Album: %s by %s", albumName, artistName),
		MusicBrainzID:  "",
		LastFmUrl:      "",
		SmallImageUrl:  "",
		MediumImageUrl: "",
		LargeImageUrl:  "",
	}

	response := newSubsonicResponse(albumInfo)
	subsonicRespond(c, response)
}
