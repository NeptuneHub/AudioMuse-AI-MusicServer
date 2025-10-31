package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

type AlchemyRequest struct {
	Items []struct {
		ID string `json:"id"`
		Op string `json:"op"`
	} `json:"items"`
	N                int     `json:"n"`
	Temperature      float64 `json:"temperature"`
	SubtractDistance float64 `json:"subtract_distance"`
}

func AlchemyHandler(c *gin.Context) {
	var req AlchemyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Determine AudioMuse-AI URL: prefer configuration in DB, then env var.
	var aiURL string
	err := db.QueryRow("SELECT value FROM configuration WHERE key = ?", "audiomuse_ai_core_url").Scan(&aiURL)
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
	// Pass through status code and body from AudioMuse-AI
	c.Data(resp.StatusCode, "application/json", body)
}
