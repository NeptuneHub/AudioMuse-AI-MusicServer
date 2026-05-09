package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// MapHandler proxies GET /api/map to the configured AudioMuse-AI core /api/map
func MapHandler(c *gin.Context) {
	// Ensure the user is authenticated (AuthMiddleware will have run)
	if _, err := getUserFromContext(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	// Use the centralized AudioMuse-AI client to fetch map data
	body, statusCode, err := audioMuseClient.Get(c.Request.Context(), "/api/map", c.Request.URL.Query())
	if err == ErrAudioMuse401 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AudioMuse-AI authentication failed. Please configure API token in Admin settings."})
		return
	}
	if err != nil {
		log.Printf("Error calling AudioMuse-AI /api/map: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to contact AudioMuse-AI Core"})
		return
	}

	// Set explicit no-cache headers to prevent ANY caching
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// Return the response directly without caching
	c.Data(statusCode, "application/json", body)
}

// VoyagerSearchTracksHandler proxies search requests for the map UI's autocomplete
func VoyagerSearchTracksHandler(c *gin.Context) {
	if _, err := getUserFromContext(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	body, statusCode, err := audioMuseClient.Get(c.Request.Context(), "/api/voyager/search_tracks", c.Request.URL.Query())
	if err == ErrAudioMuse401 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AudioMuse-AI authentication failed. Please configure API token in Admin settings."})
		return
	}
	if err != nil {
		log.Printf("Error calling AudioMuse-AI voyager search: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to contact AudioMuse-AI Core"})
		return
	}

	c.Data(statusCode, "application/json", body)
}

// Create playlist from map selection
func MapCreatePlaylistHandler(c *gin.Context) {
	user, err := getUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var payload struct {
		Name    string   `json:"name"`
		ItemIDs []string `json:"item_ids"`
	}
	if err := c.BindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}
	if payload.Name == "" || len(payload.ItemIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name and item_ids are required"})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer tx.Rollback()

	res, err := tx.Exec("INSERT INTO playlists (name, user_id) VALUES (?, ?)", payload.Name, user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create playlist"})
		return
	}
	newID, _ := res.LastInsertId()

	stmt, err := tx.Prepare("INSERT INTO playlist_songs (playlist_id, song_id, position) VALUES (?, ?, ?)")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare insert"})
		return
	}
	defer stmt.Close()

	for i, sid := range payload.ItemIDs {
		if _, err := stmt.Exec(newID, sid, i); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add song to playlist"})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "playlist_id": newID})
}

// getUserFromContext attempts to build a User from the Gin context.
// It supports two styles used in the codebase:
//   - older middleware which sets a full User at key "user"
//   - JWT AuthMiddleware which sets "userID", "username", "isAdmin"
func getUserFromContext(c *gin.Context) (*User, error) {
	if v, ok := c.Get("user"); ok {
		if u, ok := v.(User); ok {
			return &u, nil
		}
	}
	// Fallback to JWT-style keys
	uidv, ok := c.Get("userID")
	if !ok {
		return nil, errors.New("no user in context")
	}
	uid, ok := uidv.(int)
	if !ok {
		return nil, errors.New("invalid userID type")
	}
	uname := ""
	if uv, ok := c.Get("username"); ok {
		if s, ok := uv.(string); ok {
			uname = s
		}
	}
	isAdmin := false
	if av, ok := c.Get("isAdmin"); ok {
		if b, ok := av.(bool); ok {
			isAdmin = b
		}
	}
	return &User{ID: uid, Username: uname, IsAdmin: isAdmin}, nil
}
