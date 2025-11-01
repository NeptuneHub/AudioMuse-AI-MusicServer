package main

import (
	"compress/gzip"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// MapHandler proxies GET /api/map to the configured AudioMuse-AI core /api/map
func MapHandler(c *gin.Context) {
	// Ensure the user is authenticated (AuthMiddleware will have run)
	if _, err := getUserFromContext(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var coreURL string
	if err := db.QueryRow("SELECT value FROM configuration WHERE key = 'audiomuse_ai_core_url'").Scan(&coreURL); err != nil || coreURL == "" {
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

	// Use simple in-memory cache with gzip; key by final URL
	cached, ok := mapCache.Get(final)
	if ok {
		c.Header("Content-Type", cached.contentType)
		c.Header("Content-Encoding", "gzip")
		c.Data(http.StatusOK, cached.contentType, cached.data)
		return
	}

	resp, err := http.Get(final)
	if err != nil {
		log.Printf("Error calling AudioMuse-AI /api/map: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to contact AudioMuse-AI Core"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response from AudioMuse-AI Core"})
		return
	}

	// gzip-compress and store in cache
	var gzbuf strings.Builder
	gw := gzip.NewWriter(&gzbuf)
	if _, err := gw.Write(body); err == nil {
		gw.Close()
		compressed := []byte(gzbuf.String())
		ct := resp.Header.Get("Content-Type")
		mapCache.Set(final, compressed, ct, 5*time.Minute)
		c.Header("Content-Type", ct)
		c.Header("Content-Encoding", "gzip")
		c.Data(resp.StatusCode, ct, compressed)
		return
	}

	// If gzip failed, return uncompressed
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

// Voyager search proxy: proxies search requests for the map UI's autocomplete
func VoyagerSearchTracksHandler(c *gin.Context) {
	if _, err := getUserFromContext(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var coreURL string
	if err := db.QueryRow("SELECT value FROM configuration WHERE key = 'audiomuse_ai_core_url'").Scan(&coreURL); err != nil || coreURL == "" {
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

// Map cache implementation
type cachedResponse struct {
	data        []byte
	contentType string
	expiry      time.Time
}

type simpleCache struct {
	mu    sync.RWMutex
	store map[string]cachedResponse
}

var mapCache = &simpleCache{store: make(map[string]cachedResponse)}

func (c *simpleCache) Get(key string) (cachedResponse, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.store[key]
	if !ok {
		return cachedResponse{}, false
	}
	if time.Now().After(v.expiry) {
		go c.delete(key)
		return cachedResponse{}, false
	}
	return v, true
}

func (c *simpleCache) Set(key string, data []byte, contentType string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = cachedResponse{data: data, contentType: contentType, expiry: time.Now().Add(ttl)}
}

func (c *simpleCache) delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, key)
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
