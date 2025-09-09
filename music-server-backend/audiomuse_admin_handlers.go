// Suggested path: music-server-backend/audiomuse_admin_handlers.go
package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// getAudioMuseURL retrieves the configured URL for the AI core service.
func getAudioMuseURL() (string, error) {
	var coreURL string
	err := db.QueryRow("SELECT value FROM configuration WHERE key = 'audiomuse_ai_core_url'").Scan(&coreURL)
	if err != nil {
		log.Printf("ERROR: Could not retrieve 'audiomuse_ai_core_url' from database: %v", err)
		return "", fmt.Errorf("AudioMuse-AI Core URL not configured")
	}
	log.Printf("DEBUG: Retrieved AudioMuse AI Core URL from DB: %s", coreURL)
	return coreURL, nil
}

// proxyToAudioMuse is a generic helper to forward requests to the Python AI service.
func proxyToAudioMuse(c *gin.Context, method, path string) {
	coreURL, err := getAudioMuseURL()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	trimmedCoreURL := strings.TrimSuffix(coreURL, "/")
	targetURL := fmt.Sprintf("%s%s", trimmedCoreURL, path)

	log.Printf("INFO: Proxying request to AudioMuse AI Core. Method: %s, Target URL: %s", method, targetURL)

	req, err := http.NewRequest(method, targetURL, c.Request.Body)
	if err != nil {
		log.Printf("ERROR: Failed to create proxy request to %s: %v", targetURL, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create proxy request"})
		return
	}
	req.Header.Set("Content-Type", c.GetHeader("Content-Type"))
	req.Header.Set("Accept", c.GetHeader("Accept"))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ERROR: Failed forwarding request to AudioMuse AI Core at %s: %v", targetURL, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to connect to AudioMuse-AI Core service. Check logs for details."})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERROR: Failed to read response body from AudioMuse AI Core: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response from AudioMuse-AI Core"})
		return
	}

	log.Printf("INFO: Received response from AudioMuse AI Core. Status: %s, Body: %s", resp.Status, string(body))

	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

// subsonicStartSonicAnalysis handles the Subsonic API request to start an analysis.
func subsonicStartSonicAnalysis(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, "Authentication failed."))
		return
	}
	proxyToAudioMuse(c, "POST", "/api/analysis/start")
}

// subsonicCancelSonicAnalysis handles the Subsonic API request to cancel an analysis.
func subsonicCancelSonicAnalysis(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, "Authentication failed."))
		return
	}
	taskID := c.Query("taskId") // Task ID from query parameter
	if taskID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Parameter 'taskId' is required."))
		return
	}
	proxyToAudioMuse(c, "POST", fmt.Sprintf("/api/cancel/%s", taskID))
}

// subsonicGetSonicAnalysisStatus handles the Subsonic API request to get analysis status.
func subsonicGetSonicAnalysisStatus(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, "Authentication failed."))
		return
	}
	proxyToAudioMuse(c, "GET", "/api/last_task")
}

// subsonicStartClusteringAnalysis handles the Subsonic API request to start clustering.
func subsonicStartClusteringAnalysis(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, "Authentication failed."))
		return
	}
	proxyToAudioMuse(c, "POST", "/api/clustering/start")
}

