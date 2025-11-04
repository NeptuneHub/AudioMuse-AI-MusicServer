// Suggested path: music-server-backend/subsonic_browsing_handlers.go
package main

import (
	"database/sql"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"
)

// subsonicGetMusicFolders returns the list of music folders (libraries)
// In AudioMuse-AI, we'll return a single default music folder since we manage paths differently
func subsonicGetMusicFolders(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	// Return a single default music folder - this is required by many Subsonic clients
	// Clients expect at least one music folder to be present
	folders := []SubsonicMusicFolder{
		{
			ID:   1,
			Name: "Music Library",
		},
	}

	response := newSubsonicResponse(&SubsonicMusicFolders{Folders: folders})
	subsonicRespond(c, response)
}

// subsonicGetIndexes returns an indexed structure of all artists
// This is the old Subsonic API format (pre-ID3)
func subsonicGetIndexes(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	// Get last scan time for the lastModified attribute
	var lastScanStr sql.NullString
	err := db.QueryRow("SELECT last_scan_ended FROM library_paths ORDER BY last_scan_ended DESC LIMIT 1").Scan(&lastScanStr)

	lastModified := int64(0)
	if err == nil && lastScanStr.Valid {
		if t, err := time.Parse(time.RFC3339, lastScanStr.String); err == nil {
			lastModified = t.UnixMilli()
		}
	}

	// Query all artists with album counts
	query := `
		SELECT
			s.artist,
			COUNT(DISTINCT s.album)
		FROM songs s
		WHERE s.artist != ''
		GROUP BY s.artist
		ORDER BY s.artist COLLATE NOCASE
	`

	rows, err := db.Query(query)
	if err != nil {
		log.Printf("Error querying artists for getIndexes: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error querying artists."))
		return
	}
	defer rows.Close()

	// Build artist index map
	artistIndex := make(map[string][]SubsonicIndexArtist)
	for rows.Next() {
		var artist SubsonicIndexArtist
		if err := rows.Scan(&artist.Name, &artist.AlbumCount); err != nil {
			log.Printf("Error scanning artist for getIndexes: %v", err)
			continue
		}
		artist.ID = artist.Name // Use artist name as ID for compatibility

		// Determine index character
		var indexChar string
		for _, r := range artist.Name {
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				indexChar = strings.ToUpper(string(r))
				break
			}
		}
		if indexChar == "" {
			indexChar = "#"
		}

		artistIndex[indexChar] = append(artistIndex[indexChar], artist)
	}

	// Convert map to slice
	var indices []SubsonicIndex
	for name, artists := range artistIndex {
		indices = append(indices, SubsonicIndex{
			Name:    name,
			Artists: artists,
		})
	}

	// Sort indices
	sortIndices(indices)

	response := newSubsonicResponse(&SubsonicIndexes{
		LastModified:    lastModified,
		IgnoredArticles: "The El La Los Las Le Les",
		Indices:         indices,
	})
	subsonicRespond(c, response)
}

// subsonicGetMusicDirectory returns the contents of a music directory
// ID can be either an artist name (returns albums) or an album ID (returns songs)
func subsonicGetMusicDirectory(c *gin.Context) {
	user := c.MustGet("user").(User)

	id := c.Query("id")
	if id == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter id is missing."))
		return
	}

	log.Printf("getMusicDirectory called with ID: %s", id)

	// Try to parse ID as integer (album/song ID)
	if songID, err := strconv.Atoi(id); err == nil {
		// Numeric ID - could be a song ID representing an album
		var albumName, artistName string
		err := db.QueryRow("SELECT album, artist FROM songs WHERE id = ?", songID).Scan(&albumName, &artistName)
		if err != nil {
			log.Printf("Song/Album not found for ID %d: %v", songID, err)
			subsonicRespond(c, newSubsonicErrorResponse(70, "Directory not found."))
			return
		}

		// Return songs in this album
		getAlbumDirectory(c, user, songID, albumName, artistName)
		return
	}

	// Non-numeric ID - treat as artist name
	getArtistDirectory(c, id)
}

// getArtistDirectory returns all albums by an artist
func getArtistDirectory(c *gin.Context, artistName string) {
	query := `
		SELECT album, MIN(id) as album_id, COUNT(*) as song_count, COALESCE(genre, '') as genre
		FROM songs
		WHERE artist = ?
		GROUP BY album
		ORDER BY album COLLATE NOCASE
	`

	rows, err := db.Query(query, artistName)
	if err != nil {
		log.Printf("Error querying albums for artist %s: %v", artistName, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}
	defer rows.Close()

	var children []SubsonicDirectoryChild
	for rows.Next() {
		var albumName string
		var albumID, songCount int
		var genre string

		if err := rows.Scan(&albumName, &albumID, &songCount, &genre); err != nil {
			log.Printf("Error scanning album: %v", err)
			continue
		}

		child := SubsonicDirectoryChild{
			ID:       strconv.Itoa(albumID),
			Title:    albumName,
			Album:    albumName,
			Artist:   artistName,
			IsDir:    true,
			CoverArt: strconv.Itoa(albumID),
			Genre:    genre,
		}
		children = append(children, child)
	}

	dir := &SubsonicMusicDirectory{
		ID:       artistName,
		Name:     artistName,
		Children: children,
	}

	response := newSubsonicResponse(dir)
	subsonicRespond(c, response)
}

// getAlbumDirectory returns all songs in an album
func getAlbumDirectory(c *gin.Context, user User, albumID int, albumName, artistName string) {
	query := `
		SELECT s.id, s.title, s.artist, s.album, s.duration, s.play_count, s.last_played, COALESCE(s.genre, ''),
		       CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred
		FROM songs s
		LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ?
		WHERE s.album = ? AND s.artist = ?
		ORDER BY s.title COLLATE NOCASE
	`

	rows, err := db.Query(query, user.ID, albumName, artistName)
	if err != nil {
		log.Printf("Error querying songs for album %s: %v", albumName, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}
	defer rows.Close()

	var children []SubsonicDirectoryChild
	for rows.Next() {
		var songID int
		var title, artist, album, genre string
		var duration, playCount int
		var lastPlayed sql.NullString
		var starred int

		if err := rows.Scan(&songID, &title, &artist, &album, &duration, &playCount, &lastPlayed, &genre, &starred); err != nil {
			log.Printf("Error scanning song: %v", err)
			continue
		}

		child := SubsonicDirectoryChild{
			ID:        strconv.Itoa(songID),
			Title:     title,
			Artist:    artist,
			Album:     album,
			IsDir:     false,
			CoverArt:  strconv.Itoa(albumID),
			Duration:  duration,
			Genre:     genre,
			PlayCount: playCount,
			Starred:   starred == 1,
		}

		if lastPlayed.Valid {
			child.LastPlayed = lastPlayed.String
		}

		children = append(children, child)
	}

	dir := &SubsonicMusicDirectory{
		ID:       strconv.Itoa(albumID),
		Parent:   artistName,
		Name:     albumName,
		Children: children,
	}

	response := newSubsonicResponse(dir)
	subsonicRespond(c, response)
}

// subsonicGetArtist returns an artist with their albums (ID3 format)
func subsonicGetArtist(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware

	artistName := c.Query("id")
	if artistName == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter id is missing."))
		return
	}

	log.Printf("getArtist called with ID: %s", artistName)

	// Get albums by this artist
	query := `
		SELECT album, MIN(id) as album_id, COUNT(*) as song_count, COALESCE(genre, '') as genre
		FROM songs
		WHERE artist = ?
		GROUP BY album
		ORDER BY album COLLATE NOCASE
	`

	rows, err := db.Query(query, artistName)
	if err != nil {
		log.Printf("Error querying albums for artist %s: %v", artistName, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}
	defer rows.Close()

	var albums []SubsonicAlbum
	for rows.Next() {
		var albumName string
		var albumID, songCount int
		var genre string

		if err := rows.Scan(&albumName, &albumID, &songCount, &genre); err != nil {
			log.Printf("Error scanning album: %v", err)
			continue
		}

		album := SubsonicAlbum{
			ID:       strconv.Itoa(albumID),
			Name:     albumName,
			Artist:   artistName,
			CoverArt: strconv.Itoa(albumID),
			Genre:    genre,
		}
		albums = append(albums, album)
	}

	artistWithAlbums := &SubsonicArtistWithAlbums{
		ID:         artistName,
		Name:       artistName,
		CoverArt:   artistName,
		AlbumCount: len(albums),
		Albums:     albums,
	}

	response := newSubsonicResponse(artistWithAlbums)
	subsonicRespond(c, response)
}

// Helper function to sort indices alphabetically
func sortIndices(indices []SubsonicIndex) {
	// Simple bubble sort - good enough for small index lists
	for i := 0; i < len(indices); i++ {
		for j := i + 1; j < len(indices); j++ {
			if indices[i].Name > indices[j].Name {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}
}
