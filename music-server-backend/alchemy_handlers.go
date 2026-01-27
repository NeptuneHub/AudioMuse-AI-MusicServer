package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type AlchemyRequest struct {
	Items []struct {
		ID   string `json:"id"`
		Op   string `json:"op"`
		Type string `json:"type"` // "song" or "artist"
	} `json:"items"`
	N                int     `json:"n"`
	Temperature      float64 `json:"temperature"`
	SubtractDistance float64 `json:"subtract_distance"`
	Preview          bool    `json:"preview,omitempty"` // Added: preview mode flag
}

func AlchemyHandler(c *gin.Context) {
	var req AlchemyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Log what we received from frontend
	reqJSON, _ := json.MarshalIndent(req, "", "  ")
	println("=== ALCHEMY HANDLER: Received from frontend ===")
	println(string(reqJSON))
	println("Number of items:", len(req.Items))
	for i, item := range req.Items {
		println("  Item", i, "- ID:", item.ID, "Op:", item.Op, "Type:", item.Type)
	}

	// Determine AudioMuse-AI URL: prefer configuration in DB, then env var.
	var aiURL string
	aiURL, err := GetConfig(db, "audiomuse_ai_core_url")
	if err == nil {
		// got aiURL from DB
	} else if err == sql.ErrNoRows {
		// not set in DB, try env var
		aiURL = os.Getenv("AUDIO_MUSE_AI_URL")
		if aiURL == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "AudioMuse-AI endpoint not configured. Set configuration key 'audiomuse_ai_core_url' or AUDIO_MUSE_AI_URL env var."})
			return
		}
	} else {
		// DB error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read configuration for AudioMuse-AI endpoint"})
		return
	}

	// If the configured URL has no path (or only "/"), assume API path and append it.
	if parsed, perr := url.Parse(aiURL); perr == nil {
		if parsed.Path == "" || parsed.Path == "/" {
			parsed.Path = "/api/alchemy"
			aiURL = parsed.String()
		}
	}

	payload, _ := json.Marshal(req)

	// Log what we're sending to AudioMuse-AI
	println("=== ALCHEMY HANDLER: Sending to AudioMuse-AI ===")
	println("URL:", aiURL)
	println("Payload:", string(payload))

	// Use a client with timeout to avoid hanging requests
	client := &http.Client{Timeout: 20 * time.Second}

	// Respect context cancellation from the incoming request
	ctx := c.Request.Context()
	reqOut, err := http.NewRequestWithContext(ctx, "POST", aiURL, bytes.NewReader(payload))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build request to AudioMuse-AI"})
		return
	}
	reqOut.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(reqOut)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to contact AudioMuse-AI API", "details": err.Error()})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	// Log response from AudioMuse-AI
	println("=== ALCHEMY HANDLER: Received from AudioMuse-AI ===")
	println("Status Code:", resp.StatusCode)
	println("Response Length:", len(body), "bytes")
	if len(body) < 5000 {
		println("Response Body:", string(body))
	} else {
		println("Response Body: (too large, showing first 1000 chars)")
		println(string(body[:1000]))
	}

	// Pass through status code and body from AudioMuse-AI
	c.Data(resp.StatusCode, "application/json", body)
}

// SearchArtistsHandler searches for artists by name for alchemy autocomplete
func SearchArtistsHandler(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusOK, []gin.H{})
		return
	}

	// Search for artists matching the query (artist field only)
	searchPattern := "%" + strings.ToLower(query) + "%"
	rows, err := db.Query(`
		SELECT artist as artist, COUNT(DISTINCT id) as track_count
		FROM songs
		WHERE LOWER(artist) LIKE ? AND artist != '' AND cancelled = 0
		GROUP BY artist
		ORDER BY track_count DESC, artist COLLATE NOCASE
		LIMIT 20
	`, searchPattern)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var results []gin.H
	for rows.Next() {
		var artist string
		var trackCount int
		if err := rows.Scan(&artist, &trackCount); err != nil {
			continue
		}

		// Generate artist ID using the same method as the rest of the codebase
		artistID := GenerateArtistID(artist)

		results = append(results, gin.H{
			"artist":      artist,
			"artist_id":   artistID,
			"track_count": trackCount,
		})
	}

	c.JSON(http.StatusOK, results)
}
