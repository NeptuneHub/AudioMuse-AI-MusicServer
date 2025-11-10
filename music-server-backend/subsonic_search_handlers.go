// Suggested path: music-server-backend/subsonic_search_handlers.go
package main

import (
	"database/sql"
	"encoding/xml"
	"log"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// SubsonicSearchResult2 represents the structure for search2 responses.
type SubsonicSearchResult2 struct {
	XMLName xml.Name         `xml:"searchResult2" json:"-"`
	Artists []SubsonicArtist `xml:"artist" json:"artist,omitempty"`
	Albums  []SubsonicAlbum  `xml:"album" json:"album,omitempty"`
	Songs   []SubsonicSong   `xml:"song" json:"song,omitempty"`
}

// SubsonicSearchResult3 represents the structure for search3 responses (ID3 tags).
type SubsonicSearchResult3 struct {
	XMLName xml.Name         `xml:"searchResult3" json:"-"`
	Artists []SubsonicArtist `xml:"artist" json:"artist,omitempty"`
	Albums  []SubsonicAlbum  `xml:"album" json:"album,omitempty"`
	Songs   []SubsonicSong   `xml:"song" json:"song,omitempty"`
}

// subsonicSearch2 handles the search2 API endpoint (old tag format).
func subsonicSearch2(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	query := c.Query("query")

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
		var artistQuery string
		var artistArgs []interface{}

		if query == "" || query == "*" {
			// Return all artists with pagination when no search query or wildcard
			artistQuery = "SELECT artist, COUNT(DISTINCT album_path), COUNT(*) FROM songs WHERE artist != '' AND cancelled = 0 GROUP BY artist ORDER BY artist COLLATE NOCASE LIMIT ? OFFSET ?"
			artistArgs = append(artistArgs, artistCount, artistOffset)
		} else {
			// Search artists with query
			var artistConditions []string
			for _, word := range searchWords {
				artistConditions = append(artistConditions, "artist LIKE ?")
				artistArgs = append(artistArgs, "%"+word+"%")
			}
			artistArgs = append(artistArgs, artistCount, artistOffset)
			artistQuery = "SELECT artist, COUNT(DISTINCT album_path), COUNT(*) FROM songs WHERE " + strings.Join(artistConditions, " AND ") + " AND artist != '' AND cancelled = 0 GROUP BY artist ORDER BY artist COLLATE NOCASE LIMIT ? OFFSET ?"
		}

		artistRows, err := db.Query(artistQuery, artistArgs...)
		if err != nil {
			log.Printf("[ERROR] subsonicSearch2: Artist query failed: %v", err)
		} else {
			defer artistRows.Close()
			for artistRows.Next() {
				var artistName string
				var albumCount, songCount int
				if err := artistRows.Scan(&artistName, &albumCount, &songCount); err == nil {
					artistID := GenerateArtistID(artistName)
					result.Artists = append(result.Artists, SubsonicArtist{
						ID:         artistID, // Generate MD5 artist ID
						Name:       artistName,
						CoverArt:   artistID, // Use artist ID for getCoverArt (not artist name!)
						AlbumCount: albumCount,
						SongCount:  songCount,
					})
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
		// Group by album_path (directory) ONLY - 1 folder = 1 album
		albumQuery := "SELECT album, artist, COALESCE(genre, ''), MIN(id) as albumId FROM songs WHERE " + strings.Join(albumConditions, " AND ") + " AND cancelled = 0 GROUP BY album_path ORDER BY album LIMIT ? OFFSET ?"
		albumRows, err := db.Query(albumQuery, albumArgs...)
		if err != nil {
			log.Printf("[ERROR] subsonicSearch2: Album query failed: %v", err)
		} else {
			defer albumRows.Close()
			for albumRows.Next() {
				var albumName, artistName, genre string
				var albumID string
				if err := albumRows.Scan(&albumName, &artistName, &genre, &albumID); err == nil {
					result.Albums = append(result.Albums, SubsonicAlbum{
						ID:       albumID,
						Name:     albumName,
						Artist:   artistName,
						ArtistID: GenerateArtistID(artistName),
						Genre:    genre,
						CoverArt: albumID,
					})
				}
			}
		}
	}

	// --- Enhanced Song Search Logic ---
	if songCount > 0 {
		user := c.MustGet("user").(User)
		var songConditions []string
		var songArgs []interface{}
		songArgs = append(songArgs, user.ID) // First arg for JOIN

		for _, word := range searchWords {
			songConditions = append(songConditions, "(s.title LIKE ? OR s.artist LIKE ?)")
			likeWord := "%" + word + "%"
			songArgs = append(songArgs, likeWord, likeWord)
		}
		songArgs = append(songArgs, songCount, songOffset)

		songQuery := `
			SELECT s.id, s.title, s.artist, s.album, s.path, s.duration, s.play_count, s.last_played, COALESCE(s.genre, ''),
			       CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred
			FROM songs s
			LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ?
			WHERE ` + strings.Join(songConditions, " AND ") + ` AND s.cancelled = 0
			ORDER BY s.artist, s.title LIMIT ? OFFSET ?
		`

		songRows, err := db.Query(songQuery, songArgs...)
		if err != nil {
			log.Printf("[ERROR] subsonicSearch2: Song query failed: %v", err)
		} else {
			defer songRows.Close()
			for songRows.Next() {
				var songFromDb Song
				var lastPlayed sql.NullString
				var starred int
				if err := songRows.Scan(&songFromDb.ID, &songFromDb.Title, &songFromDb.Artist, &songFromDb.Album, &songFromDb.Path, &songFromDb.Duration, &songFromDb.PlayCount, &lastPlayed, &songFromDb.Genre, &starred); err == nil {
					song := SubsonicSong{
						ID:        songFromDb.ID,
						CoverArt:  songFromDb.ID,
						Title:     songFromDb.Title,
						Artist:    songFromDb.Artist,
						ArtistID:  GenerateArtistID(songFromDb.Artist),
						Album:     songFromDb.Album,
						Duration:  songFromDb.Duration,
						PlayCount: songFromDb.PlayCount,
						Genre:     songFromDb.Genre,
						Starred:   starred == 1,
					}
					if lastPlayed.Valid {
						song.LastPlayed = lastPlayed.String
					}
					result.Songs = append(result.Songs, song)
				}
			}
		}
	}

	// Ensure slices are non-nil so JSON response includes empty arrays (not omitted/null).
	if result.Artists == nil {
		result.Artists = []SubsonicArtist{}
	}
	if result.Albums == nil {
		result.Albums = []SubsonicAlbum{}
	}
	if result.Songs == nil {
		result.Songs = []SubsonicSong{}
	}

	response := newSubsonicResponse(&result)
	subsonicRespond(c, response)
}

// subsonicSearch3 handles the search3 API endpoint (ID3 tag format).
func subsonicSearch3(c *gin.Context) {
	user := c.MustGet("user").(User)

	query := c.Query("query")

	artistCount, _ := strconv.Atoi(c.DefaultQuery("artistCount", "20"))
	artistOffset, _ := strconv.Atoi(c.DefaultQuery("artistOffset", "0"))
	albumCount, _ := strconv.Atoi(c.DefaultQuery("albumCount", "20"))
	albumOffset, _ := strconv.Atoi(c.DefaultQuery("albumOffset", "0"))
	songCount, _ := strconv.Atoi(c.DefaultQuery("songCount", "50"))
	songOffset, _ := strconv.Atoi(c.DefaultQuery("songOffset", "0"))

	result := SubsonicSearchResult3{}

	// Empty query returns empty results
	if query == "" {
		result.Artists = []SubsonicArtist{}
		result.Albums = []SubsonicAlbum{}
		result.Songs = []SubsonicSong{}
		response := newSubsonicResponse(&result)
		subsonicRespond(c, response)
		return
	}

	searchWords := strings.Fields(query)

	// --- Artist Search ---
	if artistCount > 0 {
		var artistConditions []string
		var artistArgs []interface{}
		for _, word := range searchWords {
			artistConditions = append(artistConditions, "artist LIKE ?")
			artistArgs = append(artistArgs, "%"+word+"%")
		}
		artistArgs = append(artistArgs, artistCount, artistOffset)
		artistQuery := "SELECT artist, COUNT(DISTINCT album_path), COUNT(*) FROM songs WHERE " + strings.Join(artistConditions, " AND ") + " AND artist != '' AND cancelled = 0 GROUP BY artist ORDER BY artist COLLATE NOCASE LIMIT ? OFFSET ?"

		artistRows, err := db.Query(artistQuery, artistArgs...)
		if err != nil {
			log.Printf("[ERROR] subsonicSearch3: Artist query failed: %v", err)
		} else {
			defer artistRows.Close()
			for artistRows.Next() {
				var artistName string
				var albumCount, songCount int
				if err := artistRows.Scan(&artistName, &albumCount, &songCount); err == nil {
					artistID := GenerateArtistID(artistName)
					result.Artists = append(result.Artists, SubsonicArtist{
						ID:         artistID, // Generate MD5 artist ID
						Name:       artistName,
						CoverArt:   artistID, // Use artist ID for getCoverArt (not artist name!)
						AlbumCount: albumCount,
						SongCount:  songCount,
					})
				}
			}
		}
	}

	// --- Album Search ---
	if albumCount > 0 {
		var albumConditions []string
		var albumArgs []interface{}
		for _, word := range searchWords {
			albumConditions = append(albumConditions, "(album LIKE ? OR artist LIKE ?)")
			likeWord := "%" + word + "%"
			albumArgs = append(albumArgs, likeWord, likeWord)
		}
		albumArgs = append(albumArgs, albumCount, albumOffset)
		albumQuery := "SELECT album, artist, COALESCE(genre, ''), MIN(id) as albumId FROM songs WHERE " + strings.Join(albumConditions, " AND ") + " AND cancelled = 0 GROUP BY album_path ORDER BY album COLLATE NOCASE LIMIT ? OFFSET ?"

		albumRows, err := db.Query(albumQuery, albumArgs...)
		if err != nil {
			log.Printf("[ERROR] subsonicSearch3: Album query failed: %v", err)
		} else {
			defer albumRows.Close()
			for albumRows.Next() {
				var albumName, artistName, genre string
				var albumID string
				if err := albumRows.Scan(&albumName, &artistName, &genre, &albumID); err == nil {
					result.Albums = append(result.Albums, SubsonicAlbum{
						ID:       albumID,
						Name:     albumName,
						Artist:   artistName,
						ArtistID: GenerateArtistID(artistName),
						Genre:    genre,
						CoverArt: albumID,
					})
				}
			}
		}
	}

	// --- Song Search ---
	if songCount > 0 {
		var songConditions []string
		var songArgs []interface{}
		songArgs = append(songArgs, user.ID) // For starred check

		for _, word := range searchWords {
			songConditions = append(songConditions, "(s.title LIKE ? OR s.artist LIKE ? OR s.album LIKE ?)")
			likeWord := "%" + word + "%"
			songArgs = append(songArgs, likeWord, likeWord, likeWord)
		}
		songArgs = append(songArgs, songCount, songOffset)

		songQuery := `
			SELECT s.id, s.title, s.artist, s.album, s.duration, s.play_count, s.last_played, COALESCE(s.genre, ''),
			       CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred
			FROM songs s
			LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ?
			WHERE ` + strings.Join(songConditions, " AND ") + ` AND s.cancelled = 0
			ORDER BY s.artist, s.album, s.title COLLATE NOCASE LIMIT ? OFFSET ?
		`

		songRows, err := db.Query(songQuery, songArgs...)
		if err != nil {
			log.Printf("[ERROR] subsonicSearch3: Song query failed: %v", err)
		} else {
			defer songRows.Close()
			for songRows.Next() {
				var id, title, artist, album, genre string
				var duration, playCount, starred int
				var lastPlayed sql.NullString

				if err := songRows.Scan(&id, &title, &artist, &album, &duration, &playCount, &lastPlayed, &genre, &starred); err == nil {
					song := SubsonicSong{
						ID:        id,
						Title:     title,
						Artist:    artist,
						ArtistID:  GenerateArtistID(artist),
						Album:     album,
						Genre:     genre,
						Duration:  duration,
						PlayCount: playCount,
						CoverArt:  id,
						Starred:   starred == 1,
					}
					if lastPlayed.Valid {
						song.LastPlayed = lastPlayed.String
					}
					result.Songs = append(result.Songs, song)
				}
			}
		}
	}

	// Ensure slices are non-nil
	if result.Artists == nil {
		result.Artists = []SubsonicArtist{}
	}
	if result.Albums == nil {
		result.Albums = []SubsonicAlbum{}
	}
	if result.Songs == nil {
		result.Songs = []SubsonicSong{}
	}

	response := newSubsonicResponse(&result)
	subsonicRespond(c, response)
}
