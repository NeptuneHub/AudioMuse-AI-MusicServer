// audiomuse_handlers.go
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
)

// Structs to decode JSON responses from the AudioMuse-AI Core API
type similarTrackResponse struct {
	ItemID   string  `json:"item_id"`
	Title    string  `json:"title"`
	Author   string  `json:"author"`
	Distance float64 `json:"distance"`
}

type songPathResponse struct {
	Path          []similarTrackResponse `json:"path"`
	TotalDistance float64                `json:"total_distance"`
}

func getAudioMuseAICoreURL() (string, error) {
	var audioMuseURL string
	err := db.QueryRow("SELECT value FROM configuration WHERE key = 'audiomuse_ai_core_url'").Scan(&audioMuseURL)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("AudioMuse-AI Core URL not configured. Please set 'audiomuse_ai_core_url' in the configuration")
		}
		return "", fmt.Errorf("database error fetching AudioMuse-AI Core URL: %w", err)
	}
	if audioMuseURL == "" {
		return "", fmt.Errorf("AudioMuse-AI Core URL is configured but empty")
	}
	return audioMuseURL, nil
}

func subsonicGetSimilarSongs(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	songID := c.Query("id")
	if songID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Parameter 'id' is required."))
		return
	}
	count := c.DefaultQuery("count", "10")

	audioMuseURL, err := getAudioMuseAICoreURL()
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(50, err.Error()))
		return
	}

	// Construct the request to the external API
	apiURL, _ := url.Parse(audioMuseURL)
	apiURL.Path += "/api/similar_tracks"
	q := apiURL.Query()
	q.Set("item_id", songID)
	q.Set("n", count)
	// Note: 'eliminate_duplicates' is not mapped for simplicity as requested, but could be added here.
	apiURL.RawQuery = q.Encode()

	resp, err := http.Get(apiURL.String())
	if err != nil {
		log.Printf("Error calling AudioMuse-AI similar_tracks API: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to connect to AudioMuse-AI Core service."))
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to read response from AudioMuse-AI Core service."))
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("AudioMuse-AI API returned non-200 status: %d. Body: %s", resp.StatusCode, string(body))
		subsonicRespond(c, newSubsonicErrorResponse(0, fmt.Sprintf("AudioMuse-AI Core service returned an error (status %d)", resp.StatusCode)))
		return
	}

	var similarTracks []similarTrackResponse
	if err := json.Unmarshal(body, &similarTracks); err != nil {
		log.Printf("Error unmarshalling similar_tracks response: %v. Body: %s", err, string(body))
		subsonicRespond(c, newSubsonicErrorResponse(0, "Invalid response from AudioMuse-AI Core service."))
		return
	}

	var subsonicSongs []SubsonicSong
	for _, track := range similarTracks {
		subsonicSongs = append(subsonicSongs, SubsonicSong{
			ID:       track.ItemID,
			Title:    track.Title,
			Artist:   track.Author,
			Distance: track.Distance,
		})
	}

	responseBody := SubsonicDirectory{
		Name:      "Similar Songs",
		SongCount: len(subsonicSongs),
		Songs:     subsonicSongs,
	}

	subsonicRespond(c, newSubsonicResponse(&responseBody))
}

func subsonicGetSongPath(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	fromID := c.Query("fromId")
	toID := c.Query("toId")
	maxSteps := c.DefaultQuery("maxSteps", "10")

	if fromID == "" || toID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Parameters 'fromId' and 'toId' are required."))
		return
	}

	audioMuseURL, err := getAudioMuseAICoreURL()
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(50, err.Error()))
		return
	}

	apiURL, _ := url.Parse(audioMuseURL)
	apiURL.Path += "/api/find_path"
	q := apiURL.Query()
	q.Set("start_song_id", fromID)
	q.Set("end_song_id", toID)
	q.Set("max_steps", maxSteps)
	apiURL.RawQuery = q.Encode()

	resp, err := http.Get(apiURL.String())
	if err != nil {
		log.Printf("Error calling AudioMuse-AI find_path API: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to connect to AudioMuse-AI Core service."))
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to read response from AudioMuse-AI Core service."))
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("AudioMuse-AI API returned non-200 status: %d. Body: %s", resp.StatusCode, string(body))
		subsonicRespond(c, newSubsonicErrorResponse(0, fmt.Sprintf("AudioMuse-AI Core service returned an error (status %d)", resp.StatusCode)))
		return
	}

	var pathResult songPathResponse
	if err := json.Unmarshal(body, &pathResult); err != nil {
		log.Printf("Error unmarshalling find_path response: %v. Body: %s", err, string(body))
		subsonicRespond(c, newSubsonicErrorResponse(0, "Invalid response from AudioMuse-AI Core service."))
		return
	}

	var subsonicSongs []SubsonicSong
	for _, track := range pathResult.Path {
		subsonicSongs = append(subsonicSongs, SubsonicSong{
			ID:     track.ItemID,
			Title:  track.Title,
			Artist: track.Author,
		})
	}

	responseBody := SubsonicDirectory{
		Name:          fmt.Sprintf("Path from %s to %s", fromID, toID),
		SongCount:     len(subsonicSongs),
		Songs:         subsonicSongs,
		TotalDistance: pathResult.TotalDistance,
	}

	subsonicRespond(c, newSubsonicResponse(&responseBody))
}
