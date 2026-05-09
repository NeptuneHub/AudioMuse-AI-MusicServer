// Suggested path: music-server-backend/audiomuse_admin_handlers.go
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
)

// subsonicStartSonicAnalysis handles the Subsonic API request to start an analysis.
func subsonicStartSonicAnalysis(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware
	audioMuseClient.ProxyGin(c, "POST", "/api/analysis/start")
}

// subsonicCancelSonicAnalysis handles the Subsonic API request to cancel an analysis.
func subsonicCancelSonicAnalysis(c *gin.Context) {
	_ = c.MustGet("user")       // Auth is handled by middleware
	taskID := c.Query("taskId") // Task ID from query parameter
	if taskID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Parameter 'taskId' is required."))
		return
	}
	audioMuseClient.ProxyGin(c, "POST", fmt.Sprintf("/api/cancel/%s", taskID))
}

// subsonicGetSonicAnalysisStatus handles the Subsonic API request to get analysis status.
func subsonicGetSonicAnalysisStatus(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware
	audioMuseClient.ProxyGin(c, "GET", "/api/last_task")
}

// subsonicStartClusteringAnalysis handles the Subsonic API request to start clustering.
func subsonicStartClusteringAnalysis(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware
	audioMuseClient.ProxyGin(c, "POST", "/api/clustering/start")
}

// runAnalysisJob performs a POST to the AudioMuse-AI /api/analysis/start endpoint
// without a gin context. It is safe to call from background goroutines.
func runAnalysisJob(ctx context.Context) error {
	log.Printf("INFO: runAnalysisJob: POST /api/analysis/start")

	body, statusCode, err := audioMuseClient.Post(ctx, "/api/analysis/start", bytes.NewReader([]byte("{}")))
	if err == ErrAudioMuse401 {
		log.Printf("❌ AudioMuse-AI returned 401 - API token likely not configured or invalid")
		return fmt.Errorf("audio muse-ai authentication failed")
	}
	if err != nil {
		log.Printf("ERROR: runAnalysisJob request failed: %v", err)
		return err
	}

	log.Printf("INFO: runAnalysisJob response: %s", string(body))
	if statusCode >= 300 {
		return fmt.Errorf("analysis start returned status %d", statusCode)
	}
	return nil
}

// runClusteringJob performs a POST to the AudioMuse-AI /api/clustering/start endpoint
func runClusteringJob(ctx context.Context) error {
	log.Printf("INFO: runClusteringJob: POST /api/clustering/start")

	body, statusCode, err := audioMuseClient.Post(ctx, "/api/clustering/start", bytes.NewReader([]byte("{}")))
	if err == ErrAudioMuse401 {
		log.Printf("❌ AudioMuse-AI returned 401 - API token likely not configured or invalid")
		return fmt.Errorf("audio muse-ai authentication failed")
	}
	if err != nil {
		log.Printf("ERROR: runClusteringJob request failed: %v", err)
		return err
	}

	log.Printf("INFO: runClusteringJob response: %s", string(body))
	if statusCode >= 300 {
		return fmt.Errorf("clustering start returned status %d", statusCode)
	}
	return nil
}
