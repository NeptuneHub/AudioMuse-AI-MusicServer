package main

import (
	"bytes"
	"database/sql"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// CleaningStartHandler proxies a POST /api/cleaning/start request to the
// configured AudioMuse-AI instance. It prefers the configuration key
// 'audiomuse_ai_core_url' from the database and falls back to the
// AUDIO_MUSE_AI_URL environment variable. If the configured URL has no
// path, it will append /api/cleaning/start to avoid posting to root.
func CleaningStartHandler(c *gin.Context) {
	// Read incoming body (may be empty) so we can forward it to AudioMuse-AI
	body, _ := io.ReadAll(c.Request.Body)

	// Determine AudioMuse-AI URL: prefer configuration in DB, then env var.
	aiURL, err := GetConfig(db, "audiomuse_ai_core_url")
	if err == sql.ErrNoRows {
		// not set in DB, try env var
		aiURL = os.Getenv("AUDIO_MUSE_AI_URL")
		if aiURL == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "AudioMuse-AI endpoint not configured. Set configuration key 'audiomuse_ai_core_url' or AUDIO_MUSE_AI_URL env var."})
			return
		}
	} else if err != nil {
		// DB error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read configuration for AudioMuse-AI endpoint"})
		return
	}

	// If the configured URL has no path (or only "/"), assume API path and append it.
	if parsed, perr := url.Parse(aiURL); perr == nil {
		if parsed.Path == "" || parsed.Path == "/" {
			parsed.Path = "/api/cleaning/start"
			aiURL = parsed.String()
		}
	}

	// Use a client with timeout to avoid hanging requests
	client := &http.Client{Timeout: 20 * time.Second}

	// Respect context cancellation from the incoming request
	ctx := c.Request.Context()
	reqOut, err := http.NewRequestWithContext(ctx, "POST", aiURL, bytes.NewReader(body))
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
	respBody, _ := io.ReadAll(resp.Body)

	// Pass through status code and body from AudioMuse-AI
	c.Data(resp.StatusCode, "application/json", respBody)
}
