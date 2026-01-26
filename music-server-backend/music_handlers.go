// Suggested path: music-server-backend/music_handlers.go
package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// --- Music Library Handlers (JSON API) ---

func getArtists(c *gin.Context) {
	artistNames, err := fetchEffectiveArtists(db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query artists"})
		return
	}
	type ArtistWithID struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	var artists []ArtistWithID
	for _, artistName := range artistNames {
		artists = append(artists, ArtistWithID{
			ID:   GenerateArtistID(artistName),
			Name: artistName,
		})
	}
	c.JSON(http.StatusOK, artists)
}

func getAlbums(c *gin.Context) {
	artistFilter := c.Query("artist")
	// Group by album_path + album (filesystem grouping). If artistFilter is provided we filter songs by artist before grouping.
	query := `SELECT 
		album, 
		COALESCE(NULLIF(album_artist, ''), artist) as effective_artist, 
		MIN(id) 
	FROM songs 
	WHERE album != '' AND cancelled = 0`
	args := []interface{}{}

	if artistFilter != "" {
		query += " AND artist = ?"
		args = append(args, artistFilter)
	}
	query += ` GROUP BY 
		CASE
			WHEN album_path IS NOT NULL AND album_path != '' THEN album_path || '|||' || album
			ELSE album
		END
	ORDER BY effective_artist COLLATE NOCASE, album COLLATE NOCASE`

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query albums"})
		return
	}
	defer rows.Close()

	var albums []Album
	seen := make(map[string]bool)
	for rows.Next() {
		var album Album
		var minID string
		if err := rows.Scan(&album.Name, &album.Artist, &minID); err != nil {
			log.Printf("Error scanning album row: %v", err)
			continue
		}
		// Normalize legacy 'Unknown' album label
		if album.Name == "Unknown" {
			album.Name = "Unknown Album"
		}
		key := normalizeKey(album.Artist) + "|||" + normalizeKey(album.Name)
		if seen[key] {
			continue
		}
		seen[key] = true
		albums = append(albums, album)
	}
	c.JSON(http.StatusOK, albums)
}

func getSongs(c *gin.Context) {
	albumFilter := c.Query("album")
	artistFilter := c.Query("artist") // Used to disambiguate albums with the same name

	query := "SELECT id, title, artist, album, duration, play_count, last_played, date_added, date_updated, starred, genre FROM songs"
	conditions := []string{"cancelled = 0"}
	args := []interface{}{}

	if albumFilter != "" {
		conditions = append(conditions, "album = ?")
		args = append(args, albumFilter)
	}
	if artistFilter != "" {
		conditions = append(conditions, "artist = ?")
		args = append(args, artistFilter)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY artist, album, title"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query songs"})
		return
	}
	defer rows.Close()

	var songs []Song
	for rows.Next() {
		var song Song
		var starred int
		var lastPlayed, dateAdded, dateUpdated sql.NullString

		if err := rows.Scan(&song.ID, &song.Title, &song.Artist, &song.Album, &song.Duration,
			&song.PlayCount, &lastPlayed, &dateAdded, &dateUpdated, &starred, &song.Genre); err != nil {
			log.Printf("Error scanning song row: %v", err)
			continue
		}

		song.LastPlayed = lastPlayed.String
		song.DateAdded = dateAdded.String
		song.DateUpdated = dateUpdated.String
		song.Starred = starred == 1
		songs = append(songs, song)
	}
	c.JSON(http.StatusOK, songs)
}

func streamSong(c *gin.Context) {
	songID := c.Param("songID")
	var path string
	err := db.QueryRow("SELECT path FROM songs WHERE id = ? AND cancelled = 0", songID).Scan(&path)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Song not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	file, err := os.Open(path)
	if err != nil {
		log.Printf("Could not open file for streaming %s: %v", path, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error: could not open file"})
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("Could not get file info for streaming %s: %v", path, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error: could not stat file"})
		return
	}

	// http.ServeContent properly handles Range requests, which is crucial for seeking in the audio player.
	http.ServeContent(c.Writer, c.Request, fileInfo.Name(), fileInfo.ModTime(), file)
}
