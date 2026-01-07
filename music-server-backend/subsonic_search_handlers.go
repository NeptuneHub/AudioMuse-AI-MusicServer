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
	XMLName      xml.Name         `xml:"searchResult2" json:"-"`
	Artists      []SubsonicArtist `xml:"artist" json:"artist,omitempty"`
	Albums       []SubsonicAlbum  `xml:"album" json:"album,omitempty"`
	Songs        []SubsonicSong   `xml:"song" json:"song,omitempty"`
	ArtistCount  int              `xml:"artistCount,attr,omitempty" json:"artistCount,omitempty"`
	AlbumCount   int              `xml:"albumCount,attr,omitempty" json:"albumCount,omitempty"`
	SongCount    int              `xml:"songCount,attr,omitempty" json:"songCount,omitempty"`
}

// SubsonicSearchResult3 represents the structure for search3 responses (ID3 tags).
type SubsonicSearchResult3 struct {
	XMLName      xml.Name         `xml:"searchResult3" json:"-"`
	Artists      []SubsonicArtist `xml:"artist" json:"artist,omitempty"`
	Albums       []SubsonicAlbum  `xml:"album" json:"album,omitempty"`
	Songs        []SubsonicSong   `xml:"song" json:"song,omitempty"`
	ArtistCount  int              `xml:"artistCount,attr,omitempty" json:"artistCount,omitempty"`
	AlbumCount   int              `xml:"albumCount,attr,omitempty" json:"albumCount,omitempty"`
	SongCount    int              `xml:"songCount,attr,omitempty" json:"songCount,omitempty"`
}

// subsonicSearch2 handles the search2 API endpoint (old tag format).
func subsonicSearch2(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	query := c.Query("query")
	isShortQuery := len(query) < 3 // Show all items if query is less than 3 characters

	artistCount, _ := strconv.Atoi(c.DefaultQuery("artistCount", "20"))
	artistOffset, _ := strconv.Atoi(c.DefaultQuery("artistOffset", "0"))
	albumCount, _ := strconv.Atoi(c.DefaultQuery("albumCount", "20"))
	albumOffset, _ := strconv.Atoi(c.DefaultQuery("albumOffset", "0"))
	songCount, _ := strconv.Atoi(c.DefaultQuery("songCount", "50"))
	songOffset, _ := strconv.Atoi(c.DefaultQuery("songOffset", "0"))

	result := SubsonicSearchResult2{}
	searchWords := strings.Fields(query)

	// --- Count total results (not paginated) ---
	// Count total artists
	if artistCount > 0 {
		var countQuery string
		var countArgs []interface{}
		if isShortQuery || query == "" || query == "*" {
			countQuery = "SELECT COUNT(DISTINCT artist) FROM songs WHERE artist != '' AND cancelled = 0"
		} else {
			var conditions []string
			for _, word := range searchWords {
				conditions = append(conditions, "artist LIKE ?")
				countArgs = append(countArgs, "%"+word+"%")
			}
			countQuery = "SELECT COUNT(DISTINCT artist) FROM songs WHERE " + strings.Join(conditions, " AND ") + " AND artist != '' AND cancelled = 0"
		}
		db.QueryRow(countQuery, countArgs...).Scan(&result.ArtistCount)
	}

	// Count total albums
	if albumCount > 0 {
		var countQuery string
		var countArgs []interface{}
		if isShortQuery {
			countQuery = "SELECT COUNT(DISTINCT CASE WHEN album_artist IS NOT NULL AND album_artist != '' THEN album_artist || '|||' || album WHEN artist IS NOT NULL AND artist != '' THEN artist || '|||' || album ELSE album_path END) FROM songs WHERE album != '' AND cancelled = 0"
		} else {
			var conditions []string
			for _, word := range searchWords {
				conditions = append(conditions, "(album LIKE ? OR artist LIKE ? OR album_artist LIKE ?)")
				likeWord := "%" + word + "%"
				countArgs = append(countArgs, likeWord, likeWord, likeWord)
			}
			countQuery = "SELECT COUNT(DISTINCT CASE WHEN album_artist IS NOT NULL AND album_artist != '' THEN album_artist || '|||' || album WHEN artist IS NOT NULL AND artist != '' THEN artist || '|||' || album ELSE album_path END) FROM songs WHERE " + strings.Join(conditions, " AND ") + " AND cancelled = 0"
		}
		db.QueryRow(countQuery, countArgs...).Scan(&result.AlbumCount)
	}

	// Count total songs
	if songCount > 0 {
		var countQuery string
		var countArgs []interface{}
		if isShortQuery {
			countQuery = "SELECT COUNT(*) FROM songs WHERE cancelled = 0"
		} else {
			var conditions []string
			for _, word := range searchWords {
				conditions = append(conditions, "(title LIKE ? OR artist LIKE ? OR album LIKE ?)")
				likeWord := "%" + word + "%"
				countArgs = append(countArgs, likeWord, likeWord, likeWord)
			}
			countQuery = "SELECT COUNT(*) FROM songs WHERE " + strings.Join(conditions, " AND ") + " AND cancelled = 0"
		}
		db.QueryRow(countQuery, countArgs...).Scan(&result.SongCount)
	}

	// --- Enhanced Artist Search Logic ---
	if artistCount > 0 {
		var artistQuery string
		var artistArgs []interface{}

		if isShortQuery || query == "" || query == "*" {
			// Return all artists with pagination when query is short, empty, or wildcard
			artistQuery = "SELECT artist, COUNT(DISTINCT CASE WHEN album_artist IS NOT NULL AND album_artist != '' THEN album_artist || '|||' || album WHEN artist IS NOT NULL AND artist != '' THEN artist || '|||' || album ELSE album_path END), COUNT(*) FROM songs WHERE artist != '' AND cancelled = 0 GROUP BY artist ORDER BY artist COLLATE NOCASE LIMIT ? OFFSET ?"
			artistArgs = append(artistArgs, artistCount, artistOffset)
		} else {
			// Search artists with query
			var artistConditions []string
			for _, word := range searchWords {
				artistConditions = append(artistConditions, "artist LIKE ?")
				artistArgs = append(artistArgs, "%"+word+"%")
			}
			artistArgs = append(artistArgs, artistCount, artistOffset)
			artistQuery = "SELECT artist, COUNT(DISTINCT CASE WHEN album_artist IS NOT NULL AND album_artist != '' THEN album_artist || '|||' || album WHEN artist IS NOT NULL AND artist != '' THEN artist || '|||' || album ELSE album_path END), COUNT(*) FROM songs WHERE " + strings.Join(artistConditions, " AND ") + " AND artist != '' AND cancelled = 0 GROUP BY artist ORDER BY artist COLLATE NOCASE LIMIT ? OFFSET ?"
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
		var albumQuery string
		var albumArgs []interface{}
		
		if isShortQuery {
			// Show all albums when query is short
			albumQuery = "SELECT album, COALESCE(NULLIF(album_artist, ''), artist) as artist, COALESCE(genre, ''), MIN(id) as albumId FROM songs WHERE album != '' AND cancelled = 0 GROUP BY CASE WHEN album_artist IS NOT NULL AND album_artist != '' THEN album_artist || '|||' || album WHEN artist IS NOT NULL AND artist != '' THEN artist || '|||' || album ELSE album_path END ORDER BY album LIMIT ? OFFSET ?"
			albumArgs = append(albumArgs, albumCount, albumOffset)
		} else {
			// Filter by search terms
			var albumConditions []string
			for _, word := range searchWords {
				albumConditions = append(albumConditions, "(album LIKE ? OR artist LIKE ? OR album_artist LIKE ?)")
				likeWord := "%" + word + "%"
				albumArgs = append(albumArgs, likeWord, likeWord, likeWord)
			}
			albumArgs = append(albumArgs, albumCount, albumOffset)
			albumQuery = "SELECT album, COALESCE(NULLIF(album_artist, ''), artist) as artist, COALESCE(genre, ''), MIN(id) as albumId FROM songs WHERE " + strings.Join(albumConditions, " AND ") + " AND cancelled = 0 GROUP BY CASE WHEN album_artist IS NOT NULL AND album_artist != '' THEN album_artist || '|||' || album WHEN artist IS NOT NULL AND artist != '' THEN artist || '|||' || album ELSE album_path END ORDER BY album LIMIT ? OFFSET ?"
		}
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
		var songQuery string
		var songArgs []interface{}
		songArgs = append(songArgs, user.ID) // First arg for JOIN

		if isShortQuery {
			// Show all songs when query is short
			songQuery = `
				SELECT s.id, s.title, s.artist, s.album, s.path, s.duration, s.play_count, s.last_played, COALESCE(s.genre, ''),
				       CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred
				FROM songs s
				LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ?
				WHERE s.cancelled = 0
				ORDER BY s.artist, s.title LIMIT ? OFFSET ?
			`
			songArgs = append(songArgs, songCount, songOffset)
		} else {
			// Filter by search terms
			var songConditions []string
			for _, word := range searchWords {
				songConditions = append(songConditions, "(s.title LIKE ? OR s.artist LIKE ? OR s.album LIKE ?)")
				likeWord := "%" + word + "%"
				songArgs = append(songArgs, likeWord, likeWord, likeWord)
			}
			songArgs = append(songArgs, songCount, songOffset)
			songQuery = `
				SELECT s.id, s.title, s.artist, s.album, s.path, s.duration, s.play_count, s.last_played, COALESCE(s.genre, ''),
				       CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred
				FROM songs s
				LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ?
				WHERE ` + strings.Join(songConditions, " AND ") + ` AND s.cancelled = 0
				ORDER BY s.artist, s.title LIMIT ? OFFSET ?
			`
		}

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
	isShortQuery := len(query) < 3 // Show all items if query is less than 3 characters

	artistCount, _ := strconv.Atoi(c.DefaultQuery("artistCount", "20"))
	artistOffset, _ := strconv.Atoi(c.DefaultQuery("artistOffset", "0"))
	albumCount, _ := strconv.Atoi(c.DefaultQuery("albumCount", "20"))
	albumOffset, _ := strconv.Atoi(c.DefaultQuery("albumOffset", "0"))
	songCount, _ := strconv.Atoi(c.DefaultQuery("songCount", "50"))
	songOffset, _ := strconv.Atoi(c.DefaultQuery("songOffset", "0"))

	result := SubsonicSearchResult3{}

	searchWords := strings.Fields(query)

	// --- Count total results (not paginated) ---
	// Count total artists
	if artistCount > 0 {
		var countQuery string
		var countArgs []interface{}
		if isShortQuery {
			countQuery = "SELECT COUNT(DISTINCT artist) FROM songs WHERE artist != '' AND cancelled = 0"
		} else {
			var conditions []string
			for _, word := range searchWords {
				conditions = append(conditions, "artist LIKE ?")
				countArgs = append(countArgs, "%"+word+"%")
			}
			countQuery = "SELECT COUNT(DISTINCT artist) FROM songs WHERE " + strings.Join(conditions, " AND ") + " AND artist != '' AND cancelled = 0"
		}
		db.QueryRow(countQuery, countArgs...).Scan(&result.ArtistCount)
	}

	// Count total albums
	if albumCount > 0 {
		var countQuery string
		var countArgs []interface{}
		if isShortQuery {
			countQuery = "SELECT COUNT(DISTINCT CASE WHEN album_artist IS NOT NULL AND album_artist != '' THEN album_artist || '|||' || album WHEN artist IS NOT NULL AND artist != '' THEN artist || '|||' || album ELSE album_path END) FROM songs WHERE album != '' AND cancelled = 0"
		} else {
			var conditions []string
			for _, word := range searchWords {
				conditions = append(conditions, "(album LIKE ? OR artist LIKE ? OR album_artist LIKE ?)")
				likeWord := "%" + word + "%"
				countArgs = append(countArgs, likeWord, likeWord, likeWord)
			}
			countQuery = "SELECT COUNT(DISTINCT CASE WHEN album_artist IS NOT NULL AND album_artist != '' THEN album_artist || '|||' || album WHEN artist IS NOT NULL AND artist != '' THEN artist || '|||' || album ELSE album_path END) FROM songs WHERE " + strings.Join(conditions, " AND ") + " AND cancelled = 0"
		}
		db.QueryRow(countQuery, countArgs...).Scan(&result.AlbumCount)
	}

	// Count total songs
	if songCount > 0 {
		var countQuery string
		var countArgs []interface{}
		if isShortQuery {
			countQuery = "SELECT COUNT(*) FROM songs WHERE cancelled = 0"
		} else {
			var conditions []string
			for _, word := range searchWords {
				conditions = append(conditions, "(title LIKE ? OR artist LIKE ? OR album LIKE ?)")
				likeWord := "%" + word + "%"
				countArgs = append(countArgs, likeWord, likeWord, likeWord)
			}
			countQuery = "SELECT COUNT(*) FROM songs WHERE " + strings.Join(conditions, " AND ") + " AND cancelled = 0"
		}
		db.QueryRow(countQuery, countArgs...).Scan(&result.SongCount)
	}

	// --- Artist Search ---
	if artistCount > 0 {
		var artistQuery string
		var artistArgs []interface{}
		
		if isShortQuery {
			// Show all artists when query is short
			artistQuery = "SELECT artist, COUNT(DISTINCT CASE WHEN album_artist IS NOT NULL AND album_artist != '' THEN album_artist || '|||' || album WHEN artist IS NOT NULL AND artist != '' THEN artist || '|||' || album ELSE album_path END), COUNT(*) FROM songs WHERE artist != '' AND cancelled = 0 GROUP BY artist ORDER BY artist COLLATE NOCASE LIMIT ? OFFSET ?"
			artistArgs = append(artistArgs, artistCount, artistOffset)
		} else {
			// Filter by search terms
			var artistConditions []string
			for _, word := range searchWords {
				artistConditions = append(artistConditions, "artist LIKE ?")
				artistArgs = append(artistArgs, "%"+word+"%")
			}
			artistArgs = append(artistArgs, artistCount, artistOffset)
			artistQuery = "SELECT artist, COUNT(DISTINCT CASE WHEN album_artist IS NOT NULL AND album_artist != '' THEN album_artist || '|||' || album WHEN artist IS NOT NULL AND artist != '' THEN artist || '|||' || album ELSE album_path END), COUNT(*) FROM songs WHERE " + strings.Join(artistConditions, " AND ") + " AND artist != '' AND cancelled = 0 GROUP BY artist ORDER BY artist COLLATE NOCASE LIMIT ? OFFSET ?"
		}

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
		var albumQuery string
		var albumArgs []interface{}
		
		if isShortQuery {
			// Show all albums when query is short
			albumQuery = "SELECT album, COALESCE(NULLIF(album_artist, ''), artist) as artist, COALESCE(genre, ''), MIN(id) as albumId FROM songs WHERE album != '' AND cancelled = 0 GROUP BY CASE WHEN album_artist IS NOT NULL AND album_artist != '' THEN album_artist || '|||' || album WHEN artist IS NOT NULL AND artist != '' THEN artist || '|||' || album ELSE album_path END ORDER BY album COLLATE NOCASE LIMIT ? OFFSET ?"
			albumArgs = append(albumArgs, albumCount, albumOffset)
		} else {
			// Filter by search terms
			var albumConditions []string
			for _, word := range searchWords {
				albumConditions = append(albumConditions, "(album LIKE ? OR artist LIKE ? OR album_artist LIKE ?)")
				likeWord := "%" + word + "%"
				albumArgs = append(albumArgs, likeWord, likeWord, likeWord)
			}
			albumArgs = append(albumArgs, albumCount, albumOffset)
			albumQuery = "SELECT album, COALESCE(NULLIF(album_artist, ''), artist) as artist, COALESCE(genre, ''), MIN(id) as albumId FROM songs WHERE " + strings.Join(albumConditions, " AND ") + " AND cancelled = 0 GROUP BY CASE WHEN album_artist IS NOT NULL AND album_artist != '' THEN album_artist || '|||' || album WHEN artist IS NOT NULL AND artist != '' THEN artist || '|||' || album ELSE album_path END ORDER BY album COLLATE NOCASE LIMIT ? OFFSET ?"
		}

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
		var songQuery string
		var songArgs []interface{}
		songArgs = append(songArgs, user.ID) // For starred check

		if isShortQuery {
			// Show all songs when query is short
			songQuery = `
				SELECT s.id, s.title, s.artist, s.album, s.duration, s.play_count, s.last_played, COALESCE(s.genre, ''),
				       CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred
				FROM songs s
				LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ?
				WHERE s.cancelled = 0
				ORDER BY s.artist, s.album, s.title COLLATE NOCASE LIMIT ? OFFSET ?
			`
			songArgs = append(songArgs, songCount, songOffset)
		} else {
			// Filter by search terms
			var songConditions []string
			for _, word := range searchWords {
				songConditions = append(songConditions, "(s.title LIKE ? OR s.artist LIKE ? OR s.album LIKE ?)")
				likeWord := "%" + word + "%"
				songArgs = append(songArgs, likeWord, likeWord, likeWord)
			}
			songArgs = append(songArgs, songCount, songOffset)
			songQuery = `
				SELECT s.id, s.title, s.artist, s.album, s.duration, s.play_count, s.last_played, COALESCE(s.genre, ''),
				       CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred
				FROM songs s
				LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ?
				WHERE ` + strings.Join(songConditions, " AND ") + ` AND s.cancelled = 0
				ORDER BY s.artist, s.album, s.title COLLATE NOCASE LIMIT ? OFFSET ?
			`
		}

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
