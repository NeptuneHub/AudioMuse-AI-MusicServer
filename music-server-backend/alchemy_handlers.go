package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

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

	payload, _ := json.Marshal(req)

	// Log what we're sending to AudioMuse-AI
	println("=== ALCHEMY HANDLER: Sending to AudioMuse-AI ===")
	println("Payload:", string(payload))

	body, statusCode, err := audioMuseClient.Alchemy(c.Request.Context(), bytes.NewReader(payload))
	if err == ErrAudioMuse401 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AudioMuse-AI authentication failed. Please configure API token in Admin settings."})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to contact AudioMuse-AI API", "details": err.Error()})
		return
	}

	// Log response from AudioMuse-AI
	println("=== ALCHEMY HANDLER: Received from AudioMuse-AI ===")
	println("Status Code:", statusCode)
	println("Response Length:", len(body), "bytes")
	if len(body) < 5000 {
		println("Response Body:", string(body))
	} else {
		println("Response Body: (too large, showing first 1000 chars)")
		println(string(body[:1000]))
	}

	// Pass through status code and body from AudioMuse-AI
	c.Data(statusCode, "application/json", body)
}

// SearchArtistsHandler searches for artists by name for alchemy autocomplete
func SearchArtistsHandler(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusOK, []gin.H{})
		return
	}

	// Search artists by name from the derived artists table.
	searchPattern := "%" + strings.ToLower(query) + "%"
	rows, err := db.Query(`
		SELECT name as artist, song_count as track_count
		FROM artists
		WHERE LOWER(name) LIKE ?
		ORDER BY track_count DESC, name COLLATE NOCASE
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
