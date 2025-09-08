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
		// Return empty result instead of error for empty query
		subsonicRespond(c, newSubsonicResponse(&SubsonicSearchResult2{}))
		return
	}

	artistCount, _ := strconv.Atoi(c.DefaultQuery("artistCount", "20"))
	artistOffset, _ := strconv.Atoi(c.DefaultQuery("artistOffset", "0"))
	albumCount, _ := strconv.Atoi(c.DefaultQuery("albumCount", "20"))
	albumOffset, _ := strconv.Atoi(c.DefaultQuery("albumOffset", "0"))
	songCount, _ := strconv.Atoi(c.DefaultQuery("songCount", "50"))
	songOffset, _ := strconv.Atoi(c.DefaultQuery("songOffset", "0"))

	result := SubsonicSearchResult2{}
	searchWords := strings.Fields(query)

	// --- Enhanced Artist Search Logic ---
	if artistCount > 0 {
		var artistConditions []string
		var artistArgs []interface{}
		for _, word := range searchWords {
			artistConditions = append(artistConditions, "artist LIKE ?")
			artistArgs = append(artistArgs, "%"+word+"%")
		}
		artistArgs = append(artistArgs, artistCount, artistOffset)
		artistQuery := "SELECT DISTINCT artist FROM songs WHERE " + strings.Join(artistConditions, " AND ") + " ORDER BY artist LIMIT ? OFFSET ?"
		artistRows, err := db.Query(artistQuery, artistArgs...)
		if err != nil {
			log.Printf("[ERROR] subsonicSearch2: Artist query failed: %v", err)
		} else {
			defer artistRows.Close()
			for artistRows.Next() {
				var artistName string
				if err := artistRows.Scan(&artistName); err == nil {
					result.Artists = append(result.Artists, SubsonicArtist{ID: artistName, Name: artistName})
				}
			}
		}
	}

	// --- Enhanced Album Search Logic ---
	if albumCount > 0 {
		var albumConditions []string
		var albumArgs []interface{}
		for _, word := range searchWords {
			albumConditions = append(albumConditions, "(album LIKE ? OR artist LIKE ?)")
			likeWord := "%" + word + "%"
			albumArgs = append(albumArgs, likeWord, likeWord)
		}
		albumArgs = append(albumArgs, albumCount, albumOffset)
		albumQuery := "SELECT album, artist, MIN(id) as albumId FROM songs WHERE " + strings.Join(albumConditions, " AND ") + " GROUP BY album, artist ORDER BY album LIMIT ? OFFSET ?"
		albumRows, err := db.Query(albumQuery, albumArgs...)
		if err != nil {
			log.Printf("[ERROR] subsonicSearch2: Album query failed: %v", err)
		} else {
			defer albumRows.Close()
			for albumRows.Next() {
				var albumName, artistName string
				var albumID int
				if err := albumRows.Scan(&albumName, &artistName, &albumID); err == nil {
					albumIDStr := strconv.Itoa(albumID)
					result.Albums = append(result.Albums, SubsonicAlbum{ID: albumIDStr, Name: albumName, Artist: artistName, CoverArt: albumIDStr})
				}
			}
		}
	}

	// --- Enhanced Song Search Logic ---
	if songCount > 0 {
		var songConditions []string
		var songArgs []interface{}
		for _, word := range searchWords {
			songConditions = append(songConditions, "(title LIKE ? OR artist LIKE ?)")
			likeWord := "%" + word + "%"
			songArgs = append(songArgs, likeWord, likeWord)
		}
		songArgs = append(songArgs, songCount, songOffset)
		songQuery := "SELECT id, title, artist, album, path, play_count, last_played FROM songs WHERE " + strings.Join(songConditions, " AND ") + " ORDER BY artist, title LIMIT ? OFFSET ?"
		songRows, err := db.Query(songQuery, songArgs...)
		if err != nil {
			log.Printf("[ERROR] subsonicSearch2: Song query failed: %v", err)
		} else {
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
		}
	}

	response := newSubsonicResponse(nil)
	c.JSON(http.StatusOK, gin.H{"subsonic-response": gin.H{
		"status":        response.Status,
		"version":       response.Version,
		"searchResult2": result,
	}})
}

