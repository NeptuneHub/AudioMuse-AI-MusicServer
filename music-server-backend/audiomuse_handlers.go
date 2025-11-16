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

	// Convert string IDs to []interface{} for the query
	args := "'" + strings.Join(ids, `','`) + `'`
	query := fmt.Sprintf(`
		SELECT id, title, artist, album, path, play_count, last_played, duration
		FROM songs WHERE id IN (%s)
	`, args)

	log.Printf("Query for songs by IDs: %v", query)
	rows, err := db.Query(query)
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

func getItemIdByTitleAndArtist(title string, artist string) (songId string) {
	// Resolving local DB song IDs by title and artist. AudioMuse-AI Core maintains its
	// database and the item_id is a base62 UUID. By definition, this will be
	// different between the music server and the AI server.
	query := fmt.Sprintf(`SELECT id FROM songs WHERE title = "%s" AND artist = "%s" LIMIT 1`,
		title, artist)

	var songID string
	err := db.QueryRow(query, title, artist).Scan(&songID)
	if err != nil {
		log.Printf("Error getting song row by title and artist: %v", err)
		return
	}

	if songID == "" {
		log.Printf("songID not found for %s and %s. Confirm local database has track.",
			title, artist)
		return
	}

	return songID
}

func getSongByTitleAndArtist(title string, artist string) (SubsonicSong, error) {
	query := fmt.Sprintf(`SELECT id, title, artist, album, path, play_count, last_played, duration FROM songs WHERE title = "%s" AND artist = "%s" LIMIT 1`,
		title, artist)

	var song SubsonicSong
	var lastPlayed, path, playCount, duration interface{} // Use interface{} to handle NULLs gracefully
	// Set duration if it's a valid integer
	if dur, ok := duration.(int64); ok {
		song.Duration = int(dur)
	}
	err := db.QueryRow(query).Scan(&song.ID, &song.Title, &song.Artist, &song.Album, &path, &playCount, &lastPlayed, &duration)
	if err != nil {
		log.Printf("Error getting song row by title and artist: %v", err)
		return song, err
	}

	return song, nil
}

func subsonicGetSimilarSongs(c *gin.Context) {
	// Allow all authenticated users to request similar songs (Instant Mix).
	_ = c.MustGet("user").(User)

	songTitle := c.Query("title")
	songArtist := c.Query("artist")
	count := c.DefaultQuery("count", "20")

	if songTitle == "" || songArtist == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Parameters 'artist' and 'title' are required."))
		return
	}

	var coreURL string
	err := db.QueryRow("SELECT value FROM configuration WHERE key = 'audiomuse_ai_core_url'").Scan(&coreURL)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(50, "AudioMuse-AI Core URL not configured."))
		return
	}

	// Forward the request
	resp, err := http.Get(fmt.Sprintf("%s/api/similar_tracks?title=%s&artist=%s&n=%s", coreURL, songTitle, songArtist, count))
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
		Title  string `json:"title"`
		Artist string `json:"author"`
	}
	if err := json.Unmarshal(body, &similarTracks); err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to parse similar tracks from AudioMuse-AI Core."))
		return
	}

	var songs []SubsonicSong
	for _, track := range similarTracks {
		var song, err = getSongByTitleAndArtist(track.Title, track.Artist)
		if err != nil {
			log.Printf("Failed to find Title: %s by Artist: %s, is it in your library?", track.Title, track.Artist)
			log.Printf("You can check by conencting to your local DB and running the query:")
			log.Printf("SELECT id, title, artist, album, path, play_count, last_played, duration FROM songs WHERE title = \"%s\" AND artist = \"%s\" LIMIT 1;", track.Title, track.Artist)
		} else {
			songs = append(songs, song)
		}
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
