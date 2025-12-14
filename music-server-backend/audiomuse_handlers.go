// Suggested path: music-server-backend/audiomuse_handlers.go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// getSongsByIDs is a helper function to fetch song details from a list of IDs, preserving order.
func getSongsByIDs(ids []string) ([]SubsonicSong, error) {
	if len(ids) == 0 {
		return []SubsonicSong{}, nil
	}

	// Create placeholders for the IN clause, e.g., (?, ?, ?)
	placeholders := strings.Repeat("?,", len(ids)-1) + "?"
	query := fmt.Sprintf(`
		SELECT id, title, artist, album, path, play_count, last_played, duration
		FROM songs WHERE id IN (%s)
	`, placeholders)

	// Convert string IDs to []interface{} for the query
	args := make([]interface{}, len(ids))
	for i, v := range ids {
		args[i] = v
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Map results for easy lookup to preserve order
	songMap := make(map[string]SubsonicSong)
	for rows.Next() {
		var song SubsonicSong
		var lastPlayed, path, playCount, duration interface{} // Use interface{} to handle NULLs gracefully
		if err := rows.Scan(&song.ID, &song.Title, &song.Artist, &song.Album, &path, &playCount, &lastPlayed, &duration); err != nil {
			log.Printf("Error scanning song row in getSongsByIDs: %v", err)
			continue
		}
		// Set duration if it's a valid integer
		if dur, ok := duration.(int64); ok {
			song.Duration = int(dur)
		}
		songMap[song.ID] = song
	}

	// Build the final slice in the original order of IDs
	var orderedSongs []SubsonicSong
	for _, id := range ids {
		if song, ok := songMap[id]; ok {
			orderedSongs = append(orderedSongs, song)
		}
	}

	return orderedSongs, nil
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

	var coreURL string
	err := db.QueryRow("SELECT value FROM configuration WHERE key = 'audiomuse_ai_core_url'").Scan(&coreURL)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(50, "AudioMuse-AI Core URL not configured."))
		return
	}

	// Forward the request
	resp, err := http.Get(fmt.Sprintf("%s/api/similar_tracks?item_id=%s&n=%s", coreURL, songId, count))
	if err != nil {
		log.Printf("Error calling AudioMuse-AI Core for similar tracks: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to connect to AudioMuse-AI Core service."))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to read response from AudioMuse-AI Core."))
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("AudioMuse-AI Core returned non-OK status: %d - %s", resp.StatusCode, string(body))
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

	var coreURL string
	err := db.QueryRow("SELECT value FROM configuration WHERE key = 'audiomuse_ai_core_url'").Scan(&coreURL)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(50, "AudioMuse-AI Core URL not configured."))
		return
	}

	resp, err := http.Get(fmt.Sprintf("%s/api/find_path?start_song_id=%s&end_song_id=%s", coreURL, startId, endId))
	if err != nil {
		log.Printf("Error calling AudioMuse-AI Core for song path: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to connect to AudioMuse-AI Core service."))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to read response from AudioMuse-AI Core."))
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("AudioMuse-AI Core returned non-OK status for pathfinding: %d - %s", resp.StatusCode, string(body))
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

	var coreURL string
	err := db.QueryRow("SELECT value FROM configuration WHERE key = 'audiomuse_ai_core_url'").Scan(&coreURL)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(50, "AudioMuse-AI Core URL not configured."))
		return
	}

	// Forward the request to python backend, which uses its own configured default user.
	resp, err := http.Get(fmt.Sprintf("%s/api/sonic_fingerprint/generate", coreURL))
	if err != nil {
		log.Printf("Error calling AudioMuse-AI Core for sonic fingerprint: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to connect to AudioMuse-AI Core service."))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to read response from AudioMuse-AI Core."))
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("AudioMuse-AI Core returned non-OK status: %d - %s", resp.StatusCode, string(body))
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

	var coreURL string
	err := db.QueryRow("SELECT value FROM configuration WHERE key = 'audiomuse_ai_core_url'").Scan(&coreURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AudioMuse-AI Core URL not configured"})
		return
	}

	// Prepare the request to the AudioMuse-AI Core
	reqBody := map[string]interface{}{
		"query": requestBody.Query,
		"limit": requestBody.Limit,
	}
	reqJSON, _ := json.Marshal(reqBody)

	resp, err := http.Post(
		fmt.Sprintf("%s/api/clap/search", coreURL),
		"application/json",
		strings.NewReader(string(reqJSON)),
	)
	if err != nil {
		log.Printf("Error calling AudioMuse-AI Core for CLAP search: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to AudioMuse-AI Core service"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response from AudioMuse-AI Core"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("AudioMuse-AI Core returned non-OK status for CLAP search: %d - %s", resp.StatusCode, string(body))
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("AudioMuse-AI Core error: %s", string(body))})
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

	if err := json.Unmarshal(body, &clapResponse); err != nil {
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

	var coreURL string
	err := db.QueryRow("SELECT value FROM configuration WHERE key = 'audiomuse_ai_core_url'").Scan(&coreURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AudioMuse-AI Core URL not configured"})
		return
	}

	resp, err := http.Get(fmt.Sprintf("%s/api/clap/top_queries", coreURL))
	if err != nil {
		log.Printf("Error calling AudioMuse-AI Core for CLAP top queries: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to AudioMuse-AI Core service"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response from AudioMuse-AI Core"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("AudioMuse-AI Core returned non-OK status for CLAP top queries: %d - %s", resp.StatusCode, string(body))
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
