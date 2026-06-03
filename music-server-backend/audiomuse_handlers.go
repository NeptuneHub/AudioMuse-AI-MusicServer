// Suggested path: music-server-backend/audiomuse_handlers.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// getSongsByIDs is a helper function to fetch song details from a list of IDs, preserving order.
func getSongsByIDs(ids []string) ([]SubsonicSong, error) {
	results, err := QuerySongsByIDs(db, ids)
	if err != nil {
		return nil, err
	}

	// Convert to spec-aligned SubsonicSong (Child) format
	var songs []SubsonicSong
	for _, result := range results {
		songs = append(songs, buildSubsonicSong(result))
	}

	return songs, nil
}

func subsonicGetSimilarSongs(c *gin.Context) {
	// Allow all authenticated users to request similar songs (Instant Mix).
	_ = c.MustGet("user").(User)

	songId := c.Query("id")
	count := c.DefaultQuery("count", "20")

	if songId == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Parameter 'id' is required."))
		return
	}

	body, statusCode, err := audioMuseClient.GetSimilarTracks(c.Request.Context(), songId, count)
	if err == ErrAudioMuse401 {
		subsonicRespond(c, newSubsonicErrorResponse(0, "AudioMuse-AI authentication failed."))
		return
	}
	if err != nil {
		log.Printf("Error calling AudioMuse-AI for similar tracks: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to connect to AudioMuse-AI Core service."))
		return
	}

	if statusCode != http.StatusOK {
		log.Printf("AudioMuse-AI returned non-OK status: %d - %s", statusCode, string(body))
		subsonicRespond(c, newSubsonicErrorResponse(0, fmt.Sprintf("AudioMuse-AI Core error: %s", string(body))))
		return
	}

	var similarTracks []struct {
		ItemID string `json:"item_id"`
	}
	if err := json.Unmarshal(body, &similarTracks); err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to parse similar tracks from AudioMuse-AI Core."))
		return
	}

	var songIDs []string
	for _, track := range similarTracks {
		songIDs = append(songIDs, track.ItemID)
	}

	songs, err := getSongsByIDs(songIDs)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error fetching song details."))
		return
	}

	response := newSubsonicResponse(&SubsonicDirectory{
		Name:      "Similar Songs",
		SongCount: len(songs),
		Songs:     songs,
	})
	subsonicRespond(c, response)
}

func subsonicGetSongPath(c *gin.Context) {
	// Allow all authenticated users to request a song path.
	_ = c.MustGet("user").(User)

	startId := c.Query("startId")
	endId := c.Query("endId")

	if startId == "" || endId == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Parameters 'startId' and 'endId' are required."))
		return
	}

	body, statusCode, err := audioMuseClient.GetSongPath(c.Request.Context(), startId, endId)
	if err == ErrAudioMuse401 {
		subsonicRespond(c, newSubsonicErrorResponse(0, "AudioMuse-AI authentication failed."))
		return
	}
	if err != nil {
		log.Printf("Error calling AudioMuse-AI for song path: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to connect to AudioMuse-AI Core service."))
		return
	}

	if statusCode != http.StatusOK {
		log.Printf("AudioMuse-AI returned non-OK status for pathfinding: %d - %s", statusCode, string(body))
		subsonicRespond(c, newSubsonicErrorResponse(0, fmt.Sprintf("AudioMuse-AI Core error: %s", string(body))))
		return
	}

	var pathResponse struct {
		Path []struct {
			ItemID string `json:"item_id"`
		} `json:"path"`
	}
	if err := json.Unmarshal(body, &pathResponse); err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to parse song path from AudioMuse-AI Core."))
		return
	}

	var songIDs []string
	for _, track := range pathResponse.Path {
		songIDs = append(songIDs, track.ItemID)
	}

	songs, err := getSongsByIDs(songIDs)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error fetching song details for path."))
		return
	}

	response := newSubsonicResponse(&SubsonicDirectory{
		Name:      "Song Path",
		SongCount: len(songs),
		Songs:     songs,
	})
	subsonicRespond(c, response)
}

func subsonicGetSonicFingerprint(c *gin.Context) {
	// Allow authenticated users to request sonic fingerprinting (heavy ops like clustering remain admin-only).
	_ = c.MustGet("user").(User)

	body, statusCode, err := audioMuseClient.GetSonicFingerprint(c.Request.Context())
	if err == ErrAudioMuse401 {
		subsonicRespond(c, newSubsonicErrorResponse(0, "AudioMuse-AI authentication failed."))
		return
	}
	if err != nil {
		log.Printf("Error calling AudioMuse-AI for sonic fingerprint: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to connect to AudioMuse-AI Core service."))
		return
	}

	if statusCode != http.StatusOK {
		log.Printf("AudioMuse-AI returned non-OK status: %d - %s", statusCode, string(body))
		subsonicRespond(c, newSubsonicErrorResponse(0, fmt.Sprintf("AudioMuse-AI Core error: %s", string(body))))
		return
	}

	// The python response is a JSON array of objects with "item_id".
	var fingerprintTracks []struct {
		ItemID string `json:"item_id"`
	}
	if err := json.Unmarshal(body, &fingerprintTracks); err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to parse sonic fingerprint from AudioMuse-AI Core."))
		return
	}

	var songIDs []string
	for _, track := range fingerprintTracks {
		songIDs = append(songIDs, track.ItemID)
	}

	songs, err := getSongsByIDs(songIDs)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error fetching song details for fingerprint."))
		return
	}

	response := newSubsonicResponse(&SubsonicDirectory{
		Name:      "Sonic Fingerprint",
		SongCount: len(songs),
		Songs:     songs,
	})
	subsonicRespond(c, response)
}

// clapSearchHandler handles CLAP-based text search for songs.
func clapSearchHandler(c *gin.Context) {
	// Allow all authenticated users to search via CLAP.
	// JWT auth sets username in context
	_ = c.MustGet("username").(string)

	var requestBody struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if requestBody.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter is required"})
		return
	}

	if requestBody.Limit == 0 {
		requestBody.Limit = 50
	}

	// Prepare the request to the AudioMuse-AI Core
	reqBody := map[string]interface{}{
		"query": requestBody.Query,
		"limit": requestBody.Limit,
	}
	reqJSON, _ := json.Marshal(reqBody)

	respBody, statusCode, err := audioMuseClient.ClapSearch(c.Request.Context(), bytes.NewReader(reqJSON))
	if err == ErrAudioMuse401 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AudioMuse-AI authentication failed. Please configure API token in Admin settings."})
		return
	}
	if err != nil {
		log.Printf("Error calling AudioMuse-AI for CLAP search: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to AudioMuse-AI Core service"})
		return
	}

	if statusCode != http.StatusOK {
		log.Printf("AudioMuse-AI returned non-OK status for CLAP search: %d - %s", statusCode, string(respBody))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AudioMuse-AI Core error: %s", string(respBody))})
		return
	}

	var clapResponse struct {
		Query   string `json:"query"`
		Count   int    `json:"count"`
		Results []struct {
			ItemID     string  `json:"item_id"`
			Title      string  `json:"title"`
			Author     string  `json:"author"`
			Similarity float64 `json:"similarity"`
		} `json:"results"`
	}

	if err := json.Unmarshal(respBody, &clapResponse); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse CLAP search results"})
		return
	}

	// Extract song IDs and fetch full song details
	var songIDs []string
	for _, result := range clapResponse.Results {
		songIDs = append(songIDs, result.ItemID)
	}

	songs, err := getSongsByIDs(songIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error fetching song details"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query": clapResponse.Query,
		"count": clapResponse.Count,
		"songs": songs,
	})
}

// clapTopQueriesHandler retrieves the top CLAP queries.
func clapTopQueriesHandler(c *gin.Context) {
	// Allow all authenticated users to view top queries.
	// JWT auth sets username in context
	_ = c.MustGet("username").(string)

	body, statusCode, err := audioMuseClient.ClapTopQueries(c.Request.Context())
	if err == ErrAudioMuse401 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AudioMuse-AI authentication failed. Please configure API token in Admin settings."})
		return
	}
	if err != nil {
		log.Printf("Error calling AudioMuse-AI for CLAP top queries: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to AudioMuse-AI Core service"})
		return
	}

	if statusCode != http.StatusOK {
		log.Printf("AudioMuse-AI returned non-OK status for CLAP top queries: %d - %s", statusCode, string(body))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AudioMuse-AI Core error: %s", string(body))})
		return
	}

	var topQueriesResponse struct {
		Queries []string `json:"queries"`
		Ready   bool     `json:"ready"`
	}

	if err := json.Unmarshal(body, &topQueriesResponse); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse CLAP top queries"})
		return
	}

	c.JSON(http.StatusOK, topQueriesResponse)
}

// semanticSearchHandler handles semantic text search for songs.
func semanticSearchHandler(c *gin.Context) {
	// Allow all authenticated users to search via semantic search.
	_ = c.MustGet("username").(string)

	var requestBody struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if requestBody.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query parameter is required"})
		return
	}

	if requestBody.Limit == 0 {
		requestBody.Limit = 50
	}

	// Prepare the request to the AudioMuse-AI Core
	reqBody := map[string]interface{}{
		"query": requestBody.Query,
		"limit": requestBody.Limit,
	}
	reqJSON, _ := json.Marshal(reqBody)

	respBody, statusCode, err := audioMuseClient.SemanticSearch(c.Request.Context(), bytes.NewReader(reqJSON))
	if err == ErrAudioMuse401 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AudioMuse-AI authentication failed. Please configure API token in Admin settings."})
		return
	}
	if err != nil {
		log.Printf("Error calling AudioMuse-AI for semantic search: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to AudioMuse-AI Core service"})
		return
	}

	if statusCode != http.StatusOK {
		log.Printf("AudioMuse-AI returned non-OK status for semantic search: %d - %s", statusCode, string(respBody))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AudioMuse-AI Core error: %s", string(respBody))})
		return
	}

	var semanticResponse struct {
		Query   string `json:"query"`
		Count   int    `json:"count"`
		Results []struct {
			ItemID          string  `json:"item_id"`
			Title           string  `json:"title"`
			Author          string  `json:"author"`
			SimilarityScore float64 `json:"similarity_score"`
		} `json:"results"`
	}

	if err := json.Unmarshal(respBody, &semanticResponse); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse semantic search results"})
		return
	}

	// Extract song IDs and fetch full song details
	var songIDs []string
	for _, result := range semanticResponse.Results {
		songIDs = append(songIDs, result.ItemID)
	}

	songs, err := getSongsByIDs(songIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error fetching song details"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"query": semanticResponse.Query,
		"count": semanticResponse.Count,
		"songs": songs,
	})
}
