// Suggested path: music-server-backend/subsonic_search_handlers.go
package main

import (
	"database/sql"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// SubsonicSearchResult2 represents the structure for search2 and search3 responses.
type SubsonicSearchResult2 struct {
	Artists []SubsonicArtist `json:"artist,omitempty"`
	Albums  []SubsonicAlbum  `json:"album,omitempty"`
	Songs   []SubsonicSong   `json:"song,omitempty"`
}

// subsonicSearch2 handles the search2 and search3 API endpoints.
func subsonicSearch2(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	query := c.Query("query")
	if query == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter 'query' is missing."))
		return
	}
	likeQuery := "%" + query + "%"

	artistCount, _ := strconv.Atoi(c.DefaultQuery("artistCount", "20"))
	artistOffset, _ := strconv.Atoi(c.DefaultQuery("artistOffset", "0"))
	albumCount, _ := strconv.Atoi(c.DefaultQuery("albumCount", "20"))
	albumOffset, _ := strconv.Atoi(c.DefaultQuery("albumOffset", "0"))
	songCount, _ := strconv.Atoi(c.DefaultQuery("songCount", "50"))
	songOffset, _ := strconv.Atoi(c.DefaultQuery("songOffset", "0"))

	result := SubsonicSearchResult2{}

	// Search Artists
	artistRows, err := db.Query("SELECT DISTINCT artist FROM songs WHERE artist LIKE ? ORDER BY artist LIMIT ? OFFSET ?", likeQuery, artistCount, artistOffset)
	if err != nil {
		log.Printf("[ERROR] subsonicSearch2: Artist query failed: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error during artist search."))
		return
	}
	defer artistRows.Close()
	for artistRows.Next() {
		var artistName string
		if err := artistRows.Scan(&artistName); err == nil {
			result.Artists = append(result.Artists, SubsonicArtist{ID: artistName, Name: artistName})
		}
	}

	// Search Albums
	albumRows, err := db.Query("SELECT album, artist, MIN(id) as albumId FROM songs WHERE album LIKE ? GROUP BY album, artist ORDER BY album LIMIT ? OFFSET ?", likeQuery, albumCount, albumOffset)
	if err != nil {
		log.Printf("[ERROR] subsonicSearch2: Album query failed: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error during album search."))
		return
	}
	defer albumRows.Close()
	for albumRows.Next() {
		var albumName, artistName string
		var albumID int
		if err := albumRows.Scan(&albumName, &artistName, &albumID); err == nil {
			albumIDStr := strconv.Itoa(albumID)
			result.Albums = append(result.Albums, SubsonicAlbum{ID: albumIDStr, Name: albumName, Artist: artistName, CoverArt: albumIDStr})
		}
	}

	// --- Enhanced Song Search Logic ---
	searchWords := strings.Fields(query)
	var conditions []string
	var args []interface{}

	for _, word := range searchWords {
		conditions = append(conditions, "(title LIKE ? OR artist LIKE ?)")
		likeWord := "%" + word + "%"
		args = append(args, likeWord, likeWord)
	}

	// Add limit and offset to the arguments list
	args = append(args, songCount, songOffset)

	songQuery := "SELECT id, title, artist, album, path, play_count, last_played FROM songs WHERE " + strings.Join(conditions, " AND ") + " ORDER BY artist, title LIMIT ? OFFSET ?"

	songRows, err := db.Query(songQuery, args...)
	if err != nil {
		log.Printf("[ERROR] subsonicSearch2: Song query failed: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error during song search."))
		return
	}
	defer songRows.Close()
	for songRows.Next() {
		var songFromDb Song
		var lastPlayed sql.NullString
		if err := songRows.Scan(&songFromDb.ID, &songFromDb.Title, &songFromDb.Artist, &songFromDb.Album, &songFromDb.Path, &songFromDb.PlayCount, &lastPlayed); err == nil {
			song := SubsonicSong{
				ID:        strconv.Itoa(songFromDb.ID),
				Title:     songFromDb.Title,
				Artist:    songFromDb.Artist,
				Album:     songFromDb.Album,
				PlayCount: songFromDb.PlayCount,
			}
			if lastPlayed.Valid {
				song.LastPlayed = lastPlayed.String
			}
			result.Songs = append(result.Songs, song)
		}
	}

	// Create a base response to get standard fields like version and status
	response := newSubsonicResponse(nil)

	// Manually construct the JSON body to match the expected structure
	c.JSON(http.StatusOK, gin.H{"subsonic-response": gin.H{
		"status":        response.Status,
		"version":       response.Version,
		"searchResult2": result,
	}})
}

