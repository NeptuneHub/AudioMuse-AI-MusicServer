// Suggested path: music-server-backend/music_handlers.go
package main

import (
	"database/sql"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)


// --- Music Library Handlers (JSON API) ---

func getArtists(c *gin.Context) {
	rows, err := db.Query("SELECT DISTINCT artist FROM songs WHERE artist != '' ORDER BY artist")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query artists"})
		return
	}
	defer rows.Close()

	var artists []string
	for rows.Next() {
		var artist string
		if err := rows.Scan(&artist); err != nil {
			log.Printf("Error scanning artist row: %v", err)
			continue
		}
		artists = append(artists, artist)
	}
	c.JSON(http.StatusOK, artists)
}

func getAlbums(c *gin.Context) {
	artistFilter := c.Query("artist")
	query := "SELECT DISTINCT album, artist FROM songs WHERE album != ''"
	args := []interface{}{}

	if artistFilter != "" {
		query += " AND artist = ?"
		args = append(args, artistFilter)
	}
	query += " ORDER BY artist, album"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query albums"})
		return
	}
	defer rows.Close()

	var albums []Album
	for rows.Next() {
		var album Album
		if err := rows.Scan(&album.Name, &album.Artist); err != nil {
			log.Printf("Error scanning album row: %v", err)
			continue
		}
		albums = append(albums, album)
	}
	c.JSON(http.StatusOK, albums)
}

func getSongs(c *gin.Context) {
	albumFilter := c.Query("album")
	artistFilter := c.Query("artist") // Used to disambiguate albums with the same name

	query := "SELECT id, title, artist, album FROM songs"
	conditions := []string{}
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
		if err := rows.Scan(&song.ID, &song.Title, &song.Artist, &song.Album); err != nil {
			log.Printf("Error scanning song row: %v", err)
			continue
		}
		songs = append(songs, song)
	}
	c.JSON(http.StatusOK, songs)
}

func streamSong(c *gin.Context) {
	songID := c.Param("songID")
	var path string
	err := db.QueryRow("SELECT path FROM songs WHERE id = ?", songID).Scan(&path)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Song not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.File(path)
}
