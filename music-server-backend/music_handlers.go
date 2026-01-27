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
	results, err := QueryArtists(db, ArtistQueryOptions{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query artists"})
		return
	}
	type ArtistWithID struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	var artists []ArtistWithID
	for _, result := range results {
		artists = append(artists, ArtistWithID{
			ID:   GenerateArtistID(result.Name),
			Name: result.Name,
		})
	}
	c.JSON(http.StatusOK, artists)
}

func getAlbums(c *gin.Context) {
	artistFilter := c.Query("artist")

	results, err := QueryAlbums(db, AlbumQueryOptions{
		Artist:        artistFilter,
		GroupByPath:   true,
		IncludeArtist: true,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query albums"})
		return
	}

	var albums []Album
	seen := make(map[string]bool)
	for _, result := range results {
		// Compute display artist for this album
		displayArtist, _ := getAlbumDisplayArtist(db, result.Name, strings.TrimSpace(result.AlbumPath))

		album := Album{
			Name:   result.Name,
			Artist: displayArtist,
			Genre:  result.Genre,
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

	results, err := QuerySongs(db, SongQueryOptions{
		Album:        albumFilter,
		Artist:       artistFilter,
		IncludeGenre: true,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query songs"})
		return
	}

	var songs []Song
	for _, result := range results {
		// Note: We don't have date_added, date_updated in SongResult
		// If needed, we should extend SongResult or keep a custom query here
		songs = append(songs, Song{
			ID:         result.ID,
			Title:      result.Title,
			Artist:     result.Artist,
			Album:      result.Album,
			Duration:   result.Duration,
			PlayCount:  result.PlayCount,
			LastPlayed: result.LastPlayed,
			Genre:      result.Genre,
			Starred:    result.Starred,
		})
	}
	c.JSON(http.StatusOK, songs)
}

func streamSong(c *gin.Context) {
	songID := c.Param("songID")
	path, err := QuerySongPath(db, songID)
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
