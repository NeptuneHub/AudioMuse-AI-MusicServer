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
	// Count total artists (search both artist and album_artist; count deduplicated effective artist after normalization)
	if artistCount > 0 {
		seenArtists := make(map[string]bool)
		var rows *sql.Rows
		var err error
		if isShortQuery || query == "" || query == "*" {
			rows, err = db.Query("SELECT DISTINCT CASE WHEN album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist') THEN album_artist ELSE artist END AS artist FROM songs WHERE ((album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')) OR artist != '') AND cancelled = 0")
		} else {
			var conditions []string
			var args []interface{}
			for _, word := range searchWords {
				conditions = append(conditions, "(artist LIKE ? OR album_artist LIKE ?)")
				like := "%" + word + "%"
				args = append(args, like, like)
			}
			q := "SELECT DISTINCT CASE WHEN album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist') THEN album_artist ELSE artist END AS artist FROM songs WHERE " + strings.Join(conditions, " AND ") + " AND ((album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')) OR artist != '') AND cancelled = 0"
			rows, err = db.Query(q, args...)
		}
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name string
				if err := rows.Scan(&name); err != nil {
					continue
				}
				name = normalizeArtistName(name)
				key := normalizeKey(name)
				seenArtists[key] = true
			}
		}
		result.ArtistCount = len(seenArtists)
	}

	// Count total albums (deduplicate by album + album_path)
	if albumCount > 0 {
		seenAlbums := make(map[string]bool)
		var rows *sql.Rows
		var err error
		if isShortQuery {
			rows, err = db.Query("SELECT album, album_path FROM songs WHERE album != '' AND cancelled = 0")
		} else {
			// For counting, iterate distinct album groups and apply display-artist filter per-album
			rows, err = db.Query("SELECT album, MIN(NULLIF(album_path, '')) as album_path FROM songs WHERE album != '' AND cancelled = 0 GROUP BY CASE WHEN album_path IS NOT NULL AND album_path != '' THEN album_path || '|||' || album ELSE album END ORDER BY album")
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var albumName, albumPath string
					if err := rows.Scan(&albumName, &albumPath); err != nil {
						continue
					}
					albumName = strings.TrimSpace(albumName)
					albumPath = strings.TrimSpace(albumPath)
					if albumName == "" && albumPath == "" {
						continue
					}
					// Compute display artist and only count album if display artist or album name match all search words
					displayArtist, _ := getAlbumDisplayArtist(db, albumName, albumPath)
					match := true
					for _, word := range searchWords {
						lw := strings.ToLower(word)
						if !strings.Contains(strings.ToLower(albumName), lw) && !strings.Contains(strings.ToLower(displayArtist), lw) {
							match = false
							break
						}
					}
					if match {
						key := normalizeKey(albumPath + "|||" + albumName)
						seenAlbums[key] = true
					}
				}
				// 'rows' already deferred closed above
			}
		}
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var albumName, albumPath string
				if err := rows.Scan(&albumName, &albumPath); err != nil {
					continue
				}
				albumName = strings.TrimSpace(albumName)
				albumPath = strings.TrimSpace(albumPath)
				if albumName == "" && albumPath == "" {
					continue
				}
				key := normalizeKey(albumPath + "|||" + albumName)
				seenAlbums[key] = true
			}
		}
		result.AlbumCount = len(seenAlbums)
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
				conditions = append(conditions, "(title LIKE ? OR artist LIKE ?)")
				likeWord := "%" + word + "%"
				countArgs = append(countArgs, likeWord, likeWord)
			}
			countQuery = "SELECT COUNT(*) FROM songs WHERE " + strings.Join(conditions, " AND ") + " AND cancelled = 0"
		}
		db.QueryRow(countQuery, countArgs...).Scan(&result.SongCount)
	}

	// --- Enhanced Artist Search Logic ---
	if artistCount > 0 {
		var artistQuery string
		var artistArgs []interface{}

		// OPTIMIZED: Changed to aggregate album count in the main query instead of N+1 queries
		if isShortQuery || query == "" || query == "*" {
			artistQuery = `
				SELECT
					CASE WHEN album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')
					THEN album_artist ELSE artist END AS artist,
					COUNT(*) as song_count,
					COUNT(DISTINCT CASE
						WHEN album != '' AND album_path != '' THEN album_path || '|||' || album
						WHEN album != '' THEN album
						ELSE NULL
					END) as album_count
				FROM songs
				WHERE ((album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')) OR artist != '') AND cancelled = 0
				GROUP BY CASE WHEN album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist') THEN album_artist ELSE artist END
				ORDER BY artist COLLATE NOCASE
				LIMIT ? OFFSET ?`
			artistArgs = []interface{}{artistCount, artistOffset}
		} else {
			var artistConditions []string
			for _, word := range searchWords {
				artistConditions = append(artistConditions, "(CASE WHEN album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist') THEN album_artist ELSE artist END) LIKE ?")
				like := "%" + word + "%"
				artistArgs = append(artistArgs, like)
			}
			artistArgs = append(artistArgs, artistCount, artistOffset)
			artistQuery = `
				SELECT
					CASE WHEN album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')
					THEN album_artist ELSE artist END AS artist,
					COUNT(*) as song_count,
					COUNT(DISTINCT CASE
						WHEN album != '' AND album_path != '' THEN album_path || '|||' || album
						WHEN album != '' THEN album
						ELSE NULL
					END) as album_count
				FROM songs
				WHERE ` + strings.Join(artistConditions, " AND ") + ` AND ((album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')) OR artist != '') AND cancelled = 0
				GROUP BY CASE WHEN album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist') THEN album_artist ELSE artist END
				ORDER BY artist COLLATE NOCASE
				LIMIT ? OFFSET ?`
		}

		artistRows, err := db.Query(artistQuery, artistArgs...)
		if err != nil {
			log.Printf("[ERROR] subsonicSearch2: Artist query failed: %v", err)
		} else {
			defer artistRows.Close()
			seenArtists := make(map[string]bool)
			for artistRows.Next() {
				var artistName string
				var songCount int
				var albumCount int
				if err := artistRows.Scan(&artistName, &songCount, &albumCount); err == nil {
					key := normalizeKey(artistName)
					if seenArtists[key] {
						continue
					}
					seenArtists[key] = true
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
	// OPTIMIZED: Use display artist computation with proper song count
	if albumCount > 0 {
		var albumQuery string
		var albumArgs []interface{}

		if isShortQuery {
			// Show all albums when query is short
			albumQuery = `
				SELECT
					album,
					MIN(NULLIF(album_path, '')) as album_path,
					COALESCE(genre, '') as genre,
					MIN(id) as albumId,
					COUNT(*) as song_count
				FROM songs
				WHERE album != '' AND cancelled = 0
				GROUP BY CASE
					WHEN album_path IS NOT NULL AND album_path != '' THEN album_path || '|||' || album
					ELSE album
				END
				ORDER BY album LIMIT ? OFFSET ?`
			albumArgs = append(albumArgs, albumCount, albumOffset)
		} else {
			// For search terms, fetch distinct album groups then filter per-album in Go using displayArtist
			albumQuery = `
				SELECT
					album,
					MIN(NULLIF(album_path, '')) as albumPath,
					COALESCE(genre, '') as genre,
					MIN(id) as albumId,
					COUNT(*) as song_count
				FROM songs
				WHERE album != '' AND cancelled = 0
				GROUP BY CASE
					WHEN album_path IS NOT NULL AND album_path != '' THEN album_path || '|||' || album
					ELSE album
				END
				ORDER BY album`
		}
		albumRows, err := db.Query(albumQuery, albumArgs...)
		if err != nil {
			log.Printf("[ERROR] subsonicSearch2: Album query failed: %v", err)
		} else {
			defer albumRows.Close()
			// Deduplicate albums preferring entries backed by album_artist when duplicates exist
			seen := make(map[string]SubsonicAlbum)
			order := []string{}
			for albumRows.Next() {
				var albumName, albumPath, genre string
				var albumID string
				var songCount int
				if err := albumRows.Scan(&albumName, &albumPath, &genre, &albumID, &songCount); err == nil {
					albumName = strings.TrimSpace(albumName)
					albumPath = strings.TrimSpace(albumPath)
					if albumName == "" && albumPath == "" {
						continue
					}
					// Compute display artist for this album
					displayArtist, _ := getAlbumDisplayArtist(db, albumName, albumPath)
					// Apply search word matching against album name and display artist
					match := true
					for _, word := range searchWords {
						lw := strings.ToLower(word)
						if !strings.Contains(strings.ToLower(albumName), lw) && !strings.Contains(strings.ToLower(displayArtist), lw) {
							match = false
							break
						}
					}
					if !match {
						continue
					}
					key := normalizeKey(albumName)
					candidate := SubsonicAlbum{
						ID:        albumID,
						Name:      albumName,
						Artist:    displayArtist,
						ArtistID:  GenerateArtistID(displayArtist),
						Genre:     genre,
						CoverArt:  albumID,
						SongCount: songCount,
					}
					if existing, ok := seen[key]; ok {
						// If candidate is backed by album_artist and existing is not, replace existing
						var candIsAlbumArtist int
						if albumPath != "" {
							db.QueryRow("SELECT EXISTS(SELECT 1 FROM songs WHERE album = ? AND album_path = ? AND album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist') AND cancelled = 0)", albumName, albumPath).Scan(&candIsAlbumArtist)
						} else {
							db.QueryRow("SELECT EXISTS(SELECT 1 FROM songs WHERE album = ? AND (album_path IS NULL OR album_path = '') AND album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist') AND cancelled = 0)", albumName).Scan(&candIsAlbumArtist)
						}
						var existIsAlbumArtist int
						db.QueryRow("SELECT EXISTS(SELECT 1 FROM songs WHERE album = ? AND album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist') AND cancelled = 0)", existing.Name).Scan(&existIsAlbumArtist)
						if candIsAlbumArtist == 1 && existIsAlbumArtist == 0 {
							seen[key] = candidate
						}
					} else {
						seen[key] = candidate
						order = append(order, key)
					}
				}
			}
			// Apply pagination to the ordered results
			start := albumOffset
			end := start + albumCount
			if start < 0 {
				start = 0
			}
			if start > len(order) {
				start = len(order)
			}
			if end > len(order) {
				end = len(order)
			}
			for _, k := range order[start:end] {
				result.Albums = append(result.Albums, seen[k])
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
				songConditions = append(songConditions, "(s.title LIKE ? OR s.artist LIKE ?)")
				likeWord := "%" + word + "%"
				songArgs = append(songArgs, likeWord, likeWord)
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
	// Count total artists (use the artist tag only)
	if artistCount > 0 {
		seenArtists := make(map[string]bool)
		var rows *sql.Rows
		var err error
		if isShortQuery || query == "" || query == "*" {
			rows, err = db.Query("SELECT artist FROM songs WHERE artist != '' AND cancelled = 0")
		} else {
			var conditions []string
			var args []interface{}
			for _, word := range searchWords {
				conditions = append(conditions, "artist LIKE ?")
				like := "%" + word + "%"
				args = append(args, like)
			}
			q := "SELECT artist FROM songs WHERE " + strings.Join(conditions, " AND ") + " AND artist != '' AND cancelled = 0"
			rows, err = db.Query(q, args...)
		}
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var name string
				if err := rows.Scan(&name); err != nil {
					continue
				}
				name = normalizeArtistName(name)
				key := normalizeKey(name)
				seenArtists[key] = true
			}
		}
		result.ArtistCount = len(seenArtists)
	}

	// Count total albums (deduplicate by normalized album name)
	if albumCount > 0 {
		seenAlbums := make(map[string]bool)
		var rows *sql.Rows
		var err error
		if isShortQuery {
			rows, err = db.Query("SELECT album FROM songs WHERE album != '' AND cancelled = 0")
		} else {
			var conditions []string
			var args []interface{}
			for _, word := range searchWords {
				// Match album name or effective artist (ignore 'unknown' album_artist)
				conditions = append(conditions, "(album LIKE ? OR (CASE WHEN album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist') THEN album_artist ELSE artist END) LIKE ?)")
				likeWord := "%" + word + "%"
				args = append(args, likeWord, likeWord)
			}
			q := "SELECT album FROM songs WHERE " + strings.Join(conditions, " AND ") + " AND cancelled = 0"
			rows, err = db.Query(q, args...)
		}
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var albumName string
				if err := rows.Scan(&albumName); err != nil {
					continue
				}
				albumName = strings.TrimSpace(albumName)
				if albumName == "" {
					continue
				}
				seenAlbums[normalizeKey(albumName)] = true
			}
		}
		result.AlbumCount = len(seenAlbums)
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
	// OPTIMIZED: Single query with album count aggregation (no N+1 queries)
	if artistCount > 0 {
		var artistQuery string
		var artistArgs []interface{}

		if isShortQuery {
			// Show all artists when query is short
			artistQuery = `
				SELECT
					artist,
					COUNT(*) as song_count,
					COUNT(DISTINCT CASE
						WHEN album != '' AND album_path != '' THEN album_path || '|||' || album
						WHEN album != '' THEN album
						ELSE NULL
					END) as album_count
				FROM songs
				WHERE artist != '' AND cancelled = 0
				GROUP BY artist
				ORDER BY artist COLLATE NOCASE
				LIMIT ? OFFSET ?`
			artistArgs = append(artistArgs, artistCount, artistOffset)
		} else {
			// Filter by search terms
			var artistConditions []string
			for _, word := range searchWords {
				artistConditions = append(artistConditions, "artist LIKE ?")
				artistArgs = append(artistArgs, "%"+word+"%")
			}
			artistArgs = append(artistArgs, artistCount, artistOffset)
			artistQuery = `
				SELECT
					artist,
					COUNT(*) as song_count,
					COUNT(DISTINCT CASE
						WHEN album != '' AND album_path != '' THEN album_path || '|||' || album
						WHEN album != '' THEN album
						ELSE NULL
					END) as album_count
				FROM songs
				WHERE ` + strings.Join(artistConditions, " AND ") + ` AND artist != '' AND cancelled = 0
				GROUP BY artist
				ORDER BY artist COLLATE NOCASE
				LIMIT ? OFFSET ?`
		}

		artistRows, err := db.Query(artistQuery, artistArgs...)
		if err != nil {
			log.Printf("[ERROR] subsonicSearch3: Artist query failed: %v", err)
		} else {
			defer artistRows.Close()
			for artistRows.Next() {
				var artistName string
				var songCount int
				var albumCount int
				if err := artistRows.Scan(&artistName, &songCount, &albumCount); err == nil {
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
	// OPTIMIZED: Include song count in query
	if albumCount > 0 {
		var albumQuery string
		var albumArgs []interface{}

		if isShortQuery {
			// Show all albums when query is short
			albumQuery = `
				SELECT
					album,
					MIN(NULLIF(album_path, '')) as albumPath,
					COALESCE(genre, '') as genre,
					MIN(id) as albumId,
					COUNT(*) as song_count
				FROM songs
				WHERE album != '' AND cancelled = 0
				GROUP BY CASE
					WHEN album_path IS NOT NULL AND album_path != '' THEN album_path || '|||' || album
					ELSE album
				END
				ORDER BY album COLLATE NOCASE
				LIMIT ? OFFSET ?`
			albumArgs = append(albumArgs, albumCount, albumOffset)
		} else {
			// Filter by search terms (match album name or effective album artist)
			var albumConditions []string
			for _, word := range searchWords {
				// Match album name or effective artist (ignore 'unknown' album_artist)
				albumConditions = append(albumConditions, "(album LIKE ? OR (CASE WHEN album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist') THEN album_artist ELSE artist END) LIKE ?)")
				likeWord := "%" + word + "%"
				albumArgs = append(albumArgs, likeWord, likeWord)
			}
			// Fetch all candidates and apply display-artist filtering + pagination in Go
			albumQuery = `
				SELECT
					album,
					MIN(NULLIF(album_path, '')) as albumPath,
					COALESCE(genre, '') as genre,
					MIN(id) as albumId,
					COUNT(*) as song_count
				FROM songs
				WHERE ` + strings.Join(albumConditions, " AND ") + ` AND cancelled = 0
				GROUP BY CASE
					WHEN album_path IS NOT NULL AND album_path != '' THEN album_path || '|||' || album
					ELSE album
				END
				ORDER BY album COLLATE NOCASE`
		}

		albumRows, err := db.Query(albumQuery, albumArgs...)
		if err != nil {
			log.Printf("[ERROR] subsonicSearch3: Album query failed: %v", err)
		} else {
			defer albumRows.Close()
			// Deduplicate albums preferring entries backed by album_artist when duplicates exist
			seenAlbums := make(map[string]SubsonicAlbum)
			orderAlbums := []string{}
			candidates := []SubsonicAlbum{}
			for albumRows.Next() {
				var albumName, genre, albumPath string
				var albumID string
				var songCount int
				if err := albumRows.Scan(&albumName, &albumPath, &genre, &albumID, &songCount); err == nil {
					// Compute display artist for this album
					displayArtist, _ := getAlbumDisplayArtist(db, albumName, strings.TrimSpace(albumPath))
					// Ensure album matches search words by album name or display artist (case-insensitive)
					match := true
					for _, word := range searchWords {
						lw := strings.ToLower(word)
						if !strings.Contains(strings.ToLower(albumName), lw) && !strings.Contains(strings.ToLower(displayArtist), lw) {
							match = false
							break
						}
					}
					// Debug: log when searching for 'unknown' so we can inspect why albums are matching
					if strings.Contains(strings.ToLower(query), "unknown") {
						log.Printf("DEBUG search: query='%s' searchWords=%v album='%s' displayArtist='%s' match=%t", query, searchWords, albumName, displayArtist, match)
					}
					if !match {
						continue
					}
					candidate := SubsonicAlbum{
						ID:        albumID,
						Name:      albumName,
						Artist:    displayArtist,
						ArtistID:  GenerateArtistID(displayArtist),
						Genre:     genre,
						CoverArt:  albumID,
						SongCount: songCount,
					}
					candidates = append(candidates, candidate)
				}
			}
			// Deduplicate candidates by album name (preserve first occurrence), preferring album_artist-backed entries
			for _, candidate := range candidates {
				key := normalizeKey(candidate.Name)
				if existing, ok := seenAlbums[key]; ok {
					var candIsAlbumArtist int
					db.QueryRow("SELECT EXISTS(SELECT 1 FROM songs WHERE album = ? AND album_artist = ? AND cancelled = 0)", candidate.Name, candidate.Artist).Scan(&candIsAlbumArtist)
					var existIsAlbumArtist int
					db.QueryRow("SELECT EXISTS(SELECT 1 FROM songs WHERE album = ? AND album_artist = ? AND cancelled = 0)", existing.Name, existing.Artist).Scan(&existIsAlbumArtist)
					if candIsAlbumArtist == 1 && existIsAlbumArtist == 0 {
						seenAlbums[key] = candidate
					}
				} else {
					seenAlbums[key] = candidate
					orderAlbums = append(orderAlbums, key)
				}
			}
			// Apply pagination
			start := albumOffset
			if start < 0 {
				start = 0
			}
			end := start + albumCount
			if end > len(orderAlbums) {
				end = len(orderAlbums)
			}
			for _, k := range orderAlbums[start:end] {
				result.Albums = append(result.Albums, seenAlbums[k])
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
				songConditions = append(songConditions, "(s.title LIKE ? OR s.artist LIKE ?)")
				likeWord := "%" + word + "%"
				songArgs = append(songArgs, likeWord, likeWord)
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
