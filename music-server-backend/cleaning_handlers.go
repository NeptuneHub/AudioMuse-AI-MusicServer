package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// CleaningStartHandler proxies a POST /api/cleaning/start request to the
// configured AudioMuse-AI instance using the centralized client.
func CleaningStartHandler(c *gin.Context) {
	respBody, statusCode, err := audioMuseClient.Post(c.Request.Context(), "/api/cleaning/start", nil)
	if err == ErrAudioMuse401 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AudioMuse-AI authentication failed. Please configure API token in Admin settings."})
		return
	}
	if err != nil {
		log.Printf("Error calling AudioMuse-AI for cleaning: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to contact AudioMuse-AI API", "details": err.Error()})
		return
	}

	// Pass through status code and body from AudioMuse-AI
	c.Data(statusCode, "application/json", respBody)
}
