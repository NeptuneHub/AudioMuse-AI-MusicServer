// Suggested path: music-server-backend/subsonic_similar_artists_handlers.go
package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// AudioMuseSimilarArtist represents the response from AudioMuse-AI core
type AudioMuseSimilarArtist struct {
	Artist     string  `json:"artist"`
	ArtistID   string  `json:"artist_id"`
	Divergence float64 `json:"divergence"`
}

// SubsonicSimilarArtists is the Subsonic API response for similar artists
type SubsonicSimilarArtists struct {
	XMLName xml.Name         `xml:"similarArtists2" json:"-"`
	Artists []SubsonicArtist `xml:"artist" json:"artist"`
}

// subsonicGetSimilarArtists2 returns artists similar to a given artist
// This is a custom extension inspired by OpenSubsonic format
func subsonicGetSimilarArtists2(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	artistID := c.Query("id")
	if artistID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter id is missing."))
		return
	}

	count := c.DefaultQuery("count", "10")

	log.Printf("getSimilarArtists2: artistID=%s, count=%s", artistID, count)

	// Get configured AudioMuse-AI Core URL
	coreURL, err := getAudioMuseURL()
	if err != nil {
		log.Printf("Error getting AudioMuse-AI Core URL: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "AudioMuse-AI Core URL not configured."))
		return
	}

	// Call AudioMuse-AI core container
	audioMuseURL := fmt.Sprintf("%s/api/similar_artists?artist=%s&n=%s",
		strings.TrimSuffix(coreURL, "/"), url.QueryEscape(artistID), url.QueryEscape(count))

	log.Printf("Calling AudioMuse-AI core: %s", audioMuseURL)

	resp, err := http.Get(audioMuseURL)
	if err != nil {
		log.Printf("Error calling AudioMuse-AI core: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to fetch similar artists from AI service."))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("AudioMuse-AI core returned status %d", resp.StatusCode)
		subsonicRespond(c, newSubsonicErrorResponse(0, "AI service returned an error."))
		return
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading AudioMuse-AI response: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to read AI service response."))
		return
	}

	log.Printf("AudioMuse-AI raw response: %s", string(body))

	// Parse AudioMuse-AI response
	var similarArtists []AudioMuseSimilarArtist
	if err := json.Unmarshal(body, &similarArtists); err != nil {
		log.Printf("Error parsing AudioMuse-AI response: %v", err)
		log.Printf("Raw response body: %s", string(body))
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to parse AI service response."))
		return
	}

	log.Printf("Found %d similar artists", len(similarArtists))

	// Convert to Subsonic format
	var subsonicArtists []SubsonicArtist
	for _, sa := range similarArtists {
		// Get album count for this artist from database
		var albumCount int
		err := db.QueryRow(`
			SELECT COUNT(DISTINCT album_path) 
			FROM songs 
			WHERE artist = ? AND cancelled = 0
		`, sa.Artist).Scan(&albumCount)

		if err != nil {
			log.Printf("Warning: Failed to get album count for artist '%s': %v", sa.Artist, err)
			albumCount = 0
		}

		log.Printf("Processing artist: %s (ID: %s, Albums: %d)", sa.Artist, sa.ArtistID, albumCount)

		subsonicArtists = append(subsonicArtists, SubsonicArtist{
			ID:         sa.ArtistID,
			Name:       sa.Artist,
			CoverArt:   sa.ArtistID, // Use artist ID for getCoverArt
			AlbumCount: albumCount,
		})
	}

	// Ensure artists is never nil for JSON marshaling
	if subsonicArtists == nil {
		subsonicArtists = []SubsonicArtist{}
	}

	log.Printf("Returning %d subsonic artists", len(subsonicArtists))

	response := newSubsonicResponse(&SubsonicSimilarArtists{Artists: subsonicArtists})
	subsonicRespond(c, response)
}
