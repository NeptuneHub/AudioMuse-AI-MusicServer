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
	// Count total artists (search artist field only)
	if artistCount > 0 {
		seenArtists := make(map[string]bool)
		var rows *sql.Rows
		var err error
		if isShortQuery || query == "" || query == "*" {
			rows, err = db.Query("SELECT DISTINCT artist FROM songs WHERE artist != '' AND cancelled = 0")
		} else {
			var conditions []string
			var args []interface{}
			for _, word := range searchWords {
				conditions = append(conditions, "artist LIKE ?")
				like := "%" + word + "%"
				args = append(args, like)
			}
			q := "SELECT DISTINCT artist FROM songs WHERE " + strings.Join(conditions, " AND ") + " AND artist != '' AND cancelled = 0"
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
		searchTerm := ""
		if !isShortQuery && query != "" && query != "*" {
			// Build search term with AND logic for multiple words
			var conditions []string
			for _, word := range searchWords {
				conditions = append(conditions, "%"+word+"%")
			}
			// For multi-word queries, we'll need to filter in Go
			// For now, use first word as primary filter
			if len(searchWords) > 0 {
				searchTerm = searchWords[0]
			}
		}

		artists, err := QueryArtists(db, ArtistQueryOptions{
			SearchTerm:    searchTerm,
			IncludeCounts: true,
			Limit:         artistCount,
			Offset:        artistOffset,
		})
		if err != nil {
			log.Printf("[ERROR] subsonicSearch2: Artist query failed: %v", err)
		} else {
			seenArtists := make(map[string]bool)
			for _, artist := range artists {
				// Apply multi-word filter if needed
				if !isShortQuery && len(searchWords) > 1 {
					match := true
					for _, word := range searchWords {
						if !strings.Contains(strings.ToLower(artist.Name), strings.ToLower(word)) {
							match = false
							break
						}
					}
					if !match {
						continue
					}
				}

				key := normalizeKey(artist.Name)
				if seenArtists[key] {
					continue
				}
				seenArtists[key] = true
				artistID := GenerateArtistID(artist.Name)
				result.Artists = append(result.Artists, SubsonicArtist{
					ID:         artistID,
					Name:       artist.Name,
					CoverArt:   artistID,
					AlbumCount: artist.AlbumCount,
					SongCount:  artist.SongCount,
				})
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
			// For search terms, filter by album name, artist, or album_artist
			var albumConditions []string
			albumConditions = append(albumConditions, "album != ''", "cancelled = 0")
			for _, word := range searchWords {
				// Match on album name, artist, OR album_artist to show albums where the artist appears in ANY song
				albumConditions = append(albumConditions, "(album LIKE ? OR artist LIKE ? OR album_artist LIKE ?)")
				likeWord := "%" + word + "%"
				albumArgs = append(albumArgs, likeWord, likeWord, likeWord)
			}
			albumQuery = `
				SELECT
					album,
					MIN(NULLIF(album_path, '')) as albumPath,
					COALESCE(genre, '') as genre,
					MIN(id) as albumId,
					COUNT(*) as song_count
				FROM songs
				WHERE ` + strings.Join(albumConditions, " AND ") + `
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
						candIsAlbumArtist, _ := CheckAlbumHasAlbumArtist(db, albumName, albumPath)
						existIsAlbumArtist, _ := CheckAlbumHasAlbumArtist(db, existing.Name, "")
						if candIsAlbumArtist && !existIsAlbumArtist {
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
		searchTerm := ""
		if !isShortQuery && query != "" {
			// For multi-word queries, use first word as primary filter
			if len(searchWords) > 0 {
				searchTerm = searchWords[0]
			}
		}

		songs, err := QuerySongs(db, SongQueryOptions{
			SearchTerm:     searchTerm,
			IncludeStarred: true,
			IncludeGenre:   true,
			UserID:         user.ID,
			Limit:          songCount,
			Offset:         songOffset,
			OrderBy:        "s.artist, s.title",
		})
		if err != nil {
			log.Printf("[ERROR] subsonicSearch2: Song query failed: %v", err)
		} else {
			for _, songFromDb := range songs {
				// Apply multi-word filter if needed
				if !isShortQuery && len(searchWords) > 1 {
					match := true
					for _, word := range searchWords {
						lw := strings.ToLower(word)
						if !strings.Contains(strings.ToLower(songFromDb.Title), lw) &&
							!strings.Contains(strings.ToLower(songFromDb.Artist), lw) {
							match = false
							break
						}
					}
					if !match {
						continue
					}
				}

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
					Starred:   songFromDb.Starred,
					LastPlayed: songFromDb.LastPlayed,
				}
				result.Songs = append(result.Songs, song)
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
		searchTerm := ""
		if !isShortQuery && query != "" && query != "*" {
			// For multi-word queries, use first word as primary filter
			if len(searchWords) > 0 {
				searchTerm = searchWords[0]
			}
		}

		artists, err := QueryArtists(db, ArtistQueryOptions{
			SearchTerm:    searchTerm,
			IncludeCounts: true,
			Limit:         artistCount,
			Offset:        artistOffset,
		})
		if err != nil {
			log.Printf("[ERROR] subsonicSearch3: Artist query failed: %v", err)
		} else {
			for _, artist := range artists {
				// Apply multi-word filter if needed
				if !isShortQuery && len(searchWords) > 1 {
					match := true
					for _, word := range searchWords {
						if !strings.Contains(strings.ToLower(artist.Name), strings.ToLower(word)) {
							match = false
							break
						}
					}
					if !match {
						continue
					}
				}

				artistID := GenerateArtistID(artist.Name)
				result.Artists = append(result.Artists, SubsonicArtist{
					ID:         artistID,
					Name:       artist.Name,
					CoverArt:   artistID,
					AlbumCount: artist.AlbumCount,
					SongCount:  artist.SongCount,
				})
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
			// Filter by search terms (match album name, artist, or album_artist)
			var albumConditions []string
			for _, word := range searchWords {
				// Match on album name, artist, OR album_artist to show albums where the artist appears in ANY song
				albumConditions = append(albumConditions, "(album LIKE ? OR artist LIKE ? OR album_artist LIKE ?)")
				likeWord := "%" + word + "%"
				albumArgs = append(albumArgs, likeWord, likeWord, likeWord)
			}
			// Fetch candidates filtered by album name, artist, or album_artist
			albumQuery = `
				SELECT
					album,
					MIN(NULLIF(album_path, '')) as albumPath,
					COALESCE(genre, '') as genre,
					MIN(id) as albumId,
					COUNT(*) as song_count
				FROM songs
				WHERE (` + strings.Join(albumConditions, " AND ") + `) AND cancelled = 0
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
					candIsAlbumArtist, _ := AlbumExists(db, candidate.Name, candidate.Artist)
					existIsAlbumArtist, _ := AlbumExists(db, existing.Name, existing.Artist)
					if candIsAlbumArtist && !existIsAlbumArtist {
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
		searchTerm := ""
		if !isShortQuery && query != "" {
			// For multi-word queries, use first word as primary filter
			if len(searchWords) > 0 {
				searchTerm = searchWords[0]
			}
		}

		songs, err := QuerySongs(db, SongQueryOptions{
			SearchTerm:     searchTerm,
			IncludeStarred: true,
			IncludeGenre:   true,
			UserID:         user.ID,
			Limit:          songCount,
			Offset:         songOffset,
			OrderBy:        "s.artist, s.album, s.title COLLATE NOCASE",
		})
		if err != nil {
			log.Printf("[ERROR] subsonicSearch3: Song query failed: %v", err)
		} else {
			for _, songResult := range songs {
				// Apply multi-word filter if needed
				if !isShortQuery && len(searchWords) > 1 {
					match := true
					for _, word := range searchWords {
						lw := strings.ToLower(word)
						if !strings.Contains(strings.ToLower(songResult.Title), lw) &&
							!strings.Contains(strings.ToLower(songResult.Artist), lw) {
							match = false
							break
						}
					}
					if !match {
						continue
					}
				}

				song := SubsonicSong{
					ID:        songResult.ID,
					Title:     songResult.Title,
					Artist:    songResult.Artist,
					ArtistID:  GenerateArtistID(songResult.Artist),
					Album:     songResult.Album,
					Genre:     songResult.Genre,
					Duration:  songResult.Duration,
					PlayCount: songResult.PlayCount,
					CoverArt:  songResult.ID,
					Starred:   songResult.Starred,
					LastPlayed: songResult.LastPlayed,
				}
				result.Songs = append(result.Songs, song)
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
