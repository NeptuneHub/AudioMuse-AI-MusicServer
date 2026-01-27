package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// MapHandler proxies GET /api/map to the configured AudioMuse-AI core /api/map
func MapHandler(c *gin.Context) {
	// Ensure the user is authenticated (AuthMiddleware will have run)
	if _, err := getUserFromContext(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	coreURL, err := GetConfig(db, "audiomuse_ai_core_url")
	if err != nil || coreURL == "" {
		coreURL = getEnv("AUDIO_MUSE_AI_URL", "")
	}
	if coreURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AudioMuse-AI Core URL not configured"})
		return
	}

	// Preserve query string
	proxyURL, err := url.Parse(coreURL)
	if err != nil {
		log.Printf("Invalid AudioMuse-AI Core URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid AudioMuse-AI Core URL"})
		return
	}
	// Ensure path to /api/map is present if coreURL has no path
	if proxyURL.Path == "" || proxyURL.Path == "/" {
		proxyURL.Path = "/api/map"
	} else {
		// append /api/map if not already present
		proxyURL.Path = strings.TrimRight(proxyURL.Path, "/") + "/api/map"
	}

	// Build final URL including query string
	final := proxyURL.String()
	if c.Request.URL.RawQuery != "" {
		final = final + "?" + c.Request.URL.RawQuery
	}

	// NO CACHE - Always fetch fresh data from AudioMuse-AI
	// The map is dynamically generated and must reflect the latest state
	log.Printf("🔄 Fetching FRESH map data from AudioMuse-AI: %s", final)

	// Create request with explicit no-cache headers
	req, err := http.NewRequest("GET", final, nil)
	if err != nil {
		log.Printf("❌ Error creating request to AudioMuse-AI /api/map: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Add aggressive no-cache headers to ensure AudioMuse-AI doesn't cache
	req.Header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	req.Header.Set("Pragma", "no-cache")
	req.Header.Set("Expires", "0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Error calling AudioMuse-AI /api/map: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to contact AudioMuse-AI Core"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ Error reading response from AudioMuse-AI /api/map: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response from AudioMuse-AI Core"})
		return
	}

	log.Printf("✅ Received map data from AudioMuse-AI: %d bytes (status %d)", len(body), resp.StatusCode)

	// Set explicit no-cache headers to prevent ANY caching
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")

	// Return the response directly without caching
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

// Voyager search proxy: proxies search requests for the map UI's autocomplete
func VoyagerSearchTracksHandler(c *gin.Context) {
	if _, err := getUserFromContext(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	coreURL, err := GetConfig(db, "audiomuse_ai_core_url")
	if err != nil || coreURL == "" {
		coreURL = getEnv("AUDIO_MUSE_AI_URL", "")
	}
	if coreURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "AudioMuse-AI Core URL not configured"})
		return
	}

	proxyURL, err := url.Parse(coreURL)
	if err != nil {
		log.Printf("Invalid AudioMuse-AI Core URL: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid AudioMuse-AI Core URL"})
		return
	}
	// Ensure path to voyager search endpoint
	if proxyURL.Path == "" || proxyURL.Path == "/" {
		proxyURL.Path = "/api/voyager/search_tracks"
	} else {
		proxyURL.Path = strings.TrimRight(proxyURL.Path, "/") + "/api/voyager/search_tracks"
	}

	final := proxyURL.String()
	if c.Request.URL.RawQuery != "" {
		final = final + "?" + c.Request.URL.RawQuery
	}

	resp, err := http.Get(final)
	if err != nil {
		log.Printf("Error calling AudioMuse-AI voyager search: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to contact AudioMuse-AI Core"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response from AudioMuse-AI Core"})
		return
	}

	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
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
