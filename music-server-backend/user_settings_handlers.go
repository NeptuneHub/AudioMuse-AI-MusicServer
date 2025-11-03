package main

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// TranscodingSettings represents user transcoding preferences
type TranscodingSettings struct {
	UserID  int    `json:"userId"`
	Enabled bool   `json:"enabled"`
	Format  string `json:"format"`
	Bitrate int    `json:"bitrate"`
}

// getUserTranscodingSettings retrieves transcoding settings for the authenticated user
func getUserTranscodingSettings(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userID := userIDVal.(int)

	var settings TranscodingSettings
	var enabled int
	err := db.QueryRow("SELECT user_id, enabled, format, bitrate FROM transcoding_settings WHERE user_id = ?", userID).
		Scan(&settings.UserID, &enabled, &settings.Format, &settings.Bitrate)

	if err == sql.ErrNoRows {
		// Return default settings if none exist
		settings = TranscodingSettings{
			UserID:  userID,
			Enabled: false,
			Format:  "mp3",
			Bitrate: 128,
		}
		c.JSON(http.StatusOK, settings)
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve settings"})
		return
	}

	settings.Enabled = enabled == 1
	c.JSON(http.StatusOK, settings)
}

// updateUserTranscodingSettings updates transcoding settings for the authenticated user
func updateUserTranscodingSettings(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userID := userIDVal.(int)

	var settings TranscodingSettings
	if err := c.BindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate format
	validFormats := map[string]bool{"mp3": true, "ogg": true, "aac": true, "opus": true}
	if !validFormats[settings.Format] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format. Supported: mp3, ogg, aac, opus"})
		return
	}

	// Validate bitrate
	if settings.Bitrate < 64 || settings.Bitrate > 320 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bitrate must be between 64 and 320"})
		return
	}

	enabledInt := 0
	if settings.Enabled {
		enabledInt = 1
	}

	_, err := db.Exec(`INSERT INTO transcoding_settings (user_id, enabled, format, bitrate) 
		VALUES (?, ?, ?, ?) 
		ON CONFLICT(user_id) DO UPDATE SET enabled=excluded.enabled, format=excluded.format, bitrate=excluded.bitrate`,
		userID, enabledInt, settings.Format, settings.Bitrate)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Settings updated successfully"})
}

// Discovery view handlers

// CountsResponse represents total counts for music views
type CountsResponse struct {
	Artists int `json:"artists"`
	Albums  int `json:"albums"`
	Songs   int `json:"songs"`
}

// getMusicCounts returns total counts for artists, albums, and songs with optional genre filter
func getMusicCounts(c *gin.Context) {
	genre := c.Query("genre")

	var counts CountsResponse

	// Count artists
	artistQuery := "SELECT COUNT(DISTINCT artist) FROM songs WHERE artist != ''"
	args := []interface{}{}
	if genre != "" {
		artistQuery += " AND (genre = ? OR genre LIKE ? OR genre LIKE ? OR genre LIKE ?)"
		args = append(args, genre, genre+";%", "%;"+genre+";%", "%;"+genre)
	}
	db.QueryRow(artistQuery, args...).Scan(&counts.Artists)

	// Count albums
	albumQuery := "SELECT COUNT(DISTINCT artist || '|' || album) FROM songs WHERE album != ''"
	args = []interface{}{}
	if genre != "" {
		albumQuery += " AND (genre = ? OR genre LIKE ? OR genre LIKE ? OR genre LIKE ?)"
		args = append(args, genre, genre+";%", "%;"+genre+";%", "%;"+genre)
	}
	db.QueryRow(albumQuery, args...).Scan(&counts.Albums)

	// Count songs
	songQuery := "SELECT COUNT(*) FROM songs WHERE 1=1"
	args = []interface{}{}
	if genre != "" {
		songQuery += " AND (genre = ? OR genre LIKE ? OR genre LIKE ? OR genre LIKE ?)"
		args = append(args, genre, genre+";%", "%;"+genre+";%", "%;"+genre)
	}
	db.QueryRow(songQuery, args...).Scan(&counts.Songs)

	c.JSON(http.StatusOK, counts)
}

// debugSongsHandler returns raw song data for debugging
func debugSongsHandler(c *gin.Context) {
	rows, err := db.Query("SELECT id, title, date_added, date_updated FROM songs LIMIT 10")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	type DebugSong struct {
		ID          int     `json:"id"`
		Title       string  `json:"title"`
		DateAdded   *string `json:"dateAdded"`
		DateUpdated *string `json:"dateUpdated"`
	}

	var songs []DebugSong
	for rows.Next() {
		var s DebugSong
		if err := rows.Scan(&s.ID, &s.Title, &s.DateAdded, &s.DateUpdated); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		songs = append(songs, s)
	}

	var totalSongs, songsWithDate int
	db.QueryRow("SELECT COUNT(*) FROM songs").Scan(&totalSongs)
	db.QueryRow("SELECT COUNT(*) FROM songs WHERE date_added IS NOT NULL AND date_added != ''").Scan(&songsWithDate)

	c.JSON(http.StatusOK, gin.H{
		"totalSongs":    totalSongs,
		"songsWithDate": songsWithDate,
		"sampleSongs":   songs,
	})
}

// getRecentlyAdded returns recently added songs
func getRecentlyAdded(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	genre := c.Query("genre")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	// First, let's check how many songs have date_added set
	var totalSongs, songsWithDate int
	db.QueryRow("SELECT COUNT(*) FROM songs").Scan(&totalSongs)
	db.QueryRow("SELECT COUNT(*) FROM songs WHERE date_added IS NOT NULL AND date_added != ''").Scan(&songsWithDate)
	log.Printf("DEBUG [getRecentlyAdded]: Total songs=%d, Songs with date_added=%d", totalSongs, songsWithDate)

	query := "SELECT id, title, artist, album, play_count, last_played, date_added, date_updated, starred, genre FROM songs WHERE date_added IS NOT NULL AND date_added != ''"
	args := []interface{}{}

	if genre != "" {
		query += " AND (genre = ? OR genre LIKE ? OR genre LIKE ? OR genre LIKE ?)"
		args = append(args, genre, genre+";%", "%;"+genre+";%", "%;"+genre)
	}

	query += " ORDER BY date_added DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	log.Printf("DEBUG [getRecentlyAdded]: Executing query: %s with args: %v", query, args)

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("DEBUG [getRecentlyAdded]: Query error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query recently added songs"})
		return
	}
	defer rows.Close()

	songs := make([]Song, 0)
	for rows.Next() {
		var song Song
		var starred int
		var lastPlayed, dateAdded, dateUpdated sql.NullString

		err := rows.Scan(&song.ID, &song.Title, &song.Artist, &song.Album, &song.PlayCount,
			&lastPlayed, &dateAdded, &dateUpdated, &starred, &song.Genre)
		if err != nil {
			log.Printf("DEBUG [getRecentlyAdded]: Row scan error: %v", err)
			continue
		}

		song.LastPlayed = lastPlayed.String
		song.DateAdded = dateAdded.String
		song.DateUpdated = dateUpdated.String
		song.Starred = starred == 1
		songs = append(songs, song)
	}

	log.Printf("DEBUG [getRecentlyAdded]: Returning %d songs", len(songs))
	c.JSON(http.StatusOK, songs)
}

// getMostPlayed returns most played songs
func getMostPlayed(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	genre := c.Query("genre")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	query := "SELECT id, title, artist, album, play_count, last_played, date_added, date_updated, starred, genre FROM songs WHERE play_count > 0"
	args := []interface{}{}

	if genre != "" {
		query += " AND (genre = ? OR genre LIKE ? OR genre LIKE ? OR genre LIKE ?)"
		args = append(args, genre, genre+";%", "%;"+genre+";%", "%;"+genre)
	}

	query += " ORDER BY play_count DESC, last_played DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query most played songs"})
		return
	}
	defer rows.Close()

	songs := make([]Song, 0)
	for rows.Next() {
		var song Song
		var starred int
		var lastPlayed, dateAdded, dateUpdated sql.NullString

		err := rows.Scan(&song.ID, &song.Title, &song.Artist, &song.Album, &song.PlayCount,
			&lastPlayed, &dateAdded, &dateUpdated, &starred, &song.Genre)
		if err != nil {
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

// getRecentlyPlayed returns recently played songs for the authenticated user
func getRecentlyPlayed(c *gin.Context) {
	userIDVal, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	userID := userIDVal.(int)

	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")
	genre := c.Query("genre")

	limit, _ := strconv.Atoi(limitStr)
	offset, _ := strconv.Atoi(offsetStr)

	query := `SELECT DISTINCT s.id, s.title, s.artist, s.album, s.play_count, s.last_played, 
		s.date_added, s.date_updated, s.starred, s.genre, MAX(ph.played_at) as recent_play
		FROM songs s
		INNER JOIN play_history ph ON s.id = ph.song_id
		WHERE ph.user_id = ?`
	args := []interface{}{userID}

	if genre != "" {
		query += " AND (s.genre = ? OR s.genre LIKE ? OR s.genre LIKE ? OR s.genre LIKE ?)"
		args = append(args, genre, genre+";%", "%;"+genre+";%", "%;"+genre)
	}

	query += " GROUP BY s.id ORDER BY recent_play DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query recently played songs"})
		return
	}
	defer rows.Close()

	songs := make([]Song, 0)
	for rows.Next() {
		var song Song
		var starred int
		var lastPlayed, dateAdded, dateUpdated, recentPlay sql.NullString

		err := rows.Scan(&song.ID, &song.Title, &song.Artist, &song.Album, &song.PlayCount,
			&lastPlayed, &dateAdded, &dateUpdated, &starred, &song.Genre, &recentPlay)
		if err != nil {
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
