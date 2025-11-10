// Suggested path: music-server-backend/subsonic_browsing_handlers.go
package main

import (
	"database/sql"
	"log"
	"path/filepath"
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

	// Query all artists with album counts (by folder path)
	query := `
		SELECT
			s.artist,
			COUNT(DISTINCT s.album_path)
		FROM songs s
		WHERE s.artist != '' AND s.cancelled = 0
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

	// Check if ID exists as a song ID (representing an album)
	var albumName, artistName string
	err := db.QueryRow("SELECT album, artist FROM songs WHERE id = ?", id).Scan(&albumName, &artistName)
	if err == nil {
		// Found song - return songs in this album
		getAlbumDirectory(c, user, id, albumName, artistName)
		return
	}

	// Not a song ID - treat as artist name
	getArtistDirectory(c, id)
}

// getArtistDirectory returns all albums by an artist
func getArtistDirectory(c *gin.Context, artistName string) {
	// Group by album_path (directory) ONLY - 1 folder = 1 album
	query := `
		SELECT album, MIN(id) as album_id, COUNT(*) as song_count, COALESCE(genre, '') as genre
		FROM songs
		WHERE artist = ?
		GROUP BY album_path
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
		var albumID string
		var songCount int
		var genre string

		if err := rows.Scan(&albumName, &albumID, &songCount, &genre); err != nil {
			log.Printf("Error scanning album: %v", err)
			continue
		}

		child := SubsonicDirectoryChild{
			ID:       albumID,
			Title:    albumName,
			Album:    albumName,
			Artist:   artistName,
			IsDir:    true,
			CoverArt: albumID,
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
func getAlbumDirectory(c *gin.Context, user User, albumID string, albumName, artistName string) {
	// Get the album's directory path from the albumID song
	var albumPath string
	err := db.QueryRow("SELECT path FROM songs WHERE id = ?", albumID).Scan(&albumPath)
	if err != nil {
		log.Printf("Error getting album path for ID %s: %v", albumID, err)
		subsonicRespond(c, newSubsonicErrorResponse(70, "Album not found."))
		return
	}

	// Extract the album's directory to prevent mixing duplicate albums from different paths
	albumDir := filepath.Dir(albumPath)

	query := `
		SELECT s.id, s.title, s.artist, s.album, s.duration, s.play_count, s.last_played, COALESCE(s.genre, ''),
		       CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred
		FROM songs s
		LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ?
		WHERE s.album = ? AND s.artist = ? AND s.path LIKE ? AND s.cancelled = 0
		ORDER BY s.title COLLATE NOCASE
	`

	pathPattern := albumDir + string(filepath.Separator) + "%"
	rows, err := db.Query(query, user.ID, albumName, artistName, pathPattern)
	if err != nil {
		log.Printf("Error querying songs for album %s: %v", albumName, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}
	defer rows.Close()

	var children []SubsonicDirectoryChild
	for rows.Next() {
		var songID string
		var title, artist, album, genre string
		var duration, playCount int
		var lastPlayed sql.NullString
		var starred int

		if err := rows.Scan(&songID, &title, &artist, &album, &duration, &playCount, &lastPlayed, &genre, &starred); err != nil {
			log.Printf("Error scanning song: %v", err)
			continue
		}

		child := SubsonicDirectoryChild{
			ID:        songID,
			Title:     title,
			Artist:    artist,
			Album:     album,
			IsDir:     false,
			CoverArt:  albumID,
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
		ID:       albumID,
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

	artistID := c.Query("id")
	if artistID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter id is missing."))
		return
	}

	log.Printf("getArtist called with ID: %s", artistID)

	// Resolve artist ID to artist name by finding any song with matching artist ID
	var artistName string
	err := db.QueryRow(`
		SELECT DISTINCT artist 
		FROM songs 
		WHERE artist != '' 
		AND cancelled = 0
		LIMIT 1000
	`).Scan(&artistName)

	// Scan through artists to find matching ID (since we generate IDs dynamically)
	var foundArtist string
	artistRows, err := db.Query(`SELECT DISTINCT artist FROM songs WHERE artist != '' AND cancelled = 0`)
	if err != nil {
		log.Printf("Error querying artists: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}
	defer artistRows.Close()

	for artistRows.Next() {
		var name string
		if err := artistRows.Scan(&name); err != nil {
			continue
		}
		if GenerateArtistID(name) == artistID {
			foundArtist = name
			break
		}
	}

	if foundArtist == "" {
		log.Printf("Artist not found for ID: %s", artistID)
		subsonicRespond(c, newSubsonicErrorResponse(70, "Artist not found."))
		return
	}

	artistName = foundArtist
	log.Printf("Resolved artist ID %s to name: %s", artistID, artistName)

	// Get albums by this artist
	// Group by album_path (directory) ONLY - 1 folder = 1 album
	query := `
		SELECT album, MIN(id) as album_id, COUNT(*) as song_count, COALESCE(genre, '') as genre
		FROM songs
		WHERE artist = ?
		GROUP BY album_path
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
		var albumID string
		var songCount int
		var genre string

		if err := rows.Scan(&albumName, &albumID, &songCount, &genre); err != nil {
			log.Printf("Error scanning album: %v", err)
			continue
		}

		album := SubsonicAlbum{
			ID:       albumID,
			Name:     albumName,
			Artist:   artistName,
			ArtistID: GenerateArtistID(artistName),
			CoverArt: albumID,
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
