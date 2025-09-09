// Suggested path: music-server-backend/audiomuse_admin_handlers.go
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// getAudioMuseURL retrieves the configured URL for the AI core service.
func getAudioMuseURL() (string, error) {
	var coreURL string
	err := db.QueryRow("SELECT value FROM configuration WHERE key = 'audiomuse_ai_core_url'").Scan(&coreURL)
	if err != nil {
		return "", fmt.Errorf("AudioMuse-AI Core URL not configured")
	}
	return coreURL, nil
}

// proxyToAudioMuse is a generic helper to forward requests to the Python AI service.
func proxyToAudioMuse(c *gin.Context, method, path string) {
	coreURL, err := getAudioMuseURL()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	targetURL := fmt.Sprintf("%s%s", coreURL, path)

	req, err := http.NewRequest(method, targetURL, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create proxy request"})
		return
	}
	// The AI core doesn't need our server's auth token, so we don't forward it.
	req.Header.Set("Content-Type", c.GetHeader("Content-Type"))
	req.Header.Set("Accept", c.GetHeader("Accept"))


	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error forwarding request to AudioMuse AI Core: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to AudioMuse-AI Core service"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response from AudioMuse-AI Core"})
		return
	}

	// Forward the exact status code and body from the downstream service.
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}


// startSonicAnalysis proxies the request to start the analysis.
func startSonicAnalysis(c *gin.Context) {
    proxyToAudioMuse(c, "POST", "/api/analysis/start")
}

// cancelSonicAnalysis proxies the request to cancel a running analysis.
func cancelSonicAnalysis(c *gin.Context) {
    taskID := c.Param("taskID")
    if taskID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Task ID is required"})
        return
    }
    proxyToAudioMuse(c, "POST", fmt.Sprintf("/api/cancel/%s", taskID))
}

// getSonicAnalysisStatus proxies the request to get the status of the last task.
func getSonicAnalysisStatus(c *gin.Context) {
    proxyToAudioMuse(c, "GET", "/api/last_task")
}

