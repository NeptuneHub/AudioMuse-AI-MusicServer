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

	// Read artists from the derived artists table (album counts precomputed).
	rows, err := db.Query(`SELECT id, name, album_count FROM artists ORDER BY name COLLATE NOCASE`)
	if err != nil {
		log.Printf("Error querying artists for getIndexes: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error querying artists."))
		return
	}
	defer rows.Close()

	// Build artist index map
	artistIndex := make(map[string][]SubsonicIndexArtist)
	seenArtists := make(map[string]bool)
	for rows.Next() {
		var artist SubsonicIndexArtist
		if err := rows.Scan(&artist.ID, &artist.Name, &artist.AlbumCount); err != nil {
			log.Printf("Error scanning artist for getIndexes: %v", err)
			continue
		}
		// Deduplicate artists by normalized name
		key := normalizeKey(artist.Name)
		if seenArtists[key] {
			continue
		}
		seenArtists[key] = true

		artist.CoverArt = artist.ID // Set cover art ID for artist images

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
	err := db.QueryRow("SELECT album, artist FROM songs WHERE id = ? AND cancelled = 0", id).Scan(&albumName, &artistName)
	if err == nil {
		// Found song - return songs in this album
		getAlbumDirectory(c, user, id, albumName, artistName)
		return
	}

	// Not a song ID - it might be an artist ID (MD5 hash). Resolve via the cache.
	if actualArtistName, ok := resolveArtistIDToName(db, id); ok {
		getArtistDirectory(c, actualArtistName)
	} else {
		// ID doesn't match any song or artist
		subsonicRespond(c, newSubsonicErrorResponse(70, "Item not found."))
	}
}

// getArtistDirectory returns all albums by an artist
// IMPORTANT: Show albums where the artist appears in EITHER artist OR album_artist fields
// This ensures all albums are shown where the artist contributed to ANY song
func getArtistDirectory(c *gin.Context, artistName string) {
	// Group by album_path + album for deterministic filesystem-based grouping
	// Match on BOTH artist and album_artist to show all albums where this artist appears in ANY song
	// Albums where this artist contributes (as track artist or album_artist),
	// joined to the derived albums table so the display artist is precomputed.
	query := `
		SELECT a.id, a.name, a.artist, COALESCE(a.genre, '')
		FROM albums a
		WHERE a.group_key IN (
			SELECT CASE
				WHEN album_path IS NOT NULL AND album_path != '' THEN album_path || '|||' || album
				ELSE album
			END
			FROM songs
			WHERE (artist = ? OR album_artist = ?) AND album != '' AND cancelled = 0
		)
		ORDER BY a.name COLLATE NOCASE
	`

	rows, err := db.Query(query, artistName, artistName)
	if err != nil {
		log.Printf("Error querying albums for artist %s: %v", artistName, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}
	defer rows.Close()

	var children []SubsonicDirectoryChild
	for rows.Next() {
		var albumName, albumID, displayArtist, genre string
		if err := rows.Scan(&albumID, &albumName, &displayArtist, &genre); err != nil {
			log.Printf("Error scanning album: %v", err)
			continue
		}

		child := SubsonicDirectoryChild{
			ID:       albumID,
			Title:    albumName,
			Album:    albumName,
			Artist:   displayArtist,
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
	err := db.QueryRow("SELECT path FROM songs WHERE id = ? AND cancelled = 0", albumID).Scan(&albumPath)
	if err != nil {
		log.Printf("Error getting album path for ID %s: %v", albumID, err)
		subsonicRespond(c, newSubsonicErrorResponse(70, "Album not found."))
		return
	}

	// Extract the album's directory to prevent mixing duplicate albums from different paths
	albumDir := filepath.Dir(albumPath)
	// Display album artist (precomputed in the derived albums table)
	displayArtist := albumDisplayArtist(db, albumName, albumDir)

	query := `
		SELECT s.id, s.title, s.artist, s.album, s.path, s.duration, s.play_count, s.last_played, COALESCE(s.genre, ''),
		       COALESCE(s.album_artist, ''), COALESCE(s.date_added, ''),
		       s.replaygain_track_gain, s.replaygain_track_peak, s.replaygain_album_gain, s.replaygain_album_peak,
		       COALESCE(s.track, 0), COALESCE(s.year, 0), COALESCE(s.disc_number, 0),
		       COALESCE(s.size, 0), COALESCE(s.bitrate, 0), COALESCE(s.sample_rate, 0), COALESCE(s.channels, 0), COALESCE(s.bit_depth, 0), COALESCE(s.comment, ''),
		       CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred
		FROM songs s
		LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ?
		WHERE s.album = ? AND s.path LIKE ? AND s.cancelled = 0
		ORDER BY COALESCE(s.disc_number, 0), COALESCE(s.track, 0), s.title COLLATE NOCASE
	`

	pathPattern := albumDir + string(filepath.Separator) + "%"
	rows, err := db.Query(query, user.ID, albumName, pathPattern)
	if err != nil {
		log.Printf("Error querying songs for album %s: %v", albumName, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}
	defer rows.Close()

	var children []SubsonicDirectoryChild
	for rows.Next() {
		var r SongResult
		var lastPlayed, genreVal, albumArtist, created sql.NullString
		var rgTrackGain, rgTrackPeak, rgAlbumGain, rgAlbumPeak sql.NullFloat64
		var starred int

		if err := rows.Scan(&r.ID, &r.Title, &r.Artist, &r.Album, &r.Path, &r.Duration, &r.PlayCount, &lastPlayed, &genreVal,
			&albumArtist, &created, &rgTrackGain, &rgTrackPeak, &rgAlbumGain, &rgAlbumPeak, &r.Track, &r.Year, &r.DiscNumber,
			&r.Size, &r.BitRate, &r.SamplingRate, &r.ChannelCount, &r.BitDepth, &r.Comment, &starred); err != nil {
			log.Printf("Error scanning song: %v", err)
			continue
		}
		if lastPlayed.Valid {
			r.LastPlayed = lastPlayed.String
		}
		if genreVal.Valid {
			r.Genre = genreVal.String
		}
		if albumArtist.Valid {
			r.AlbumArtist = albumArtist.String
		}
		if created.Valid {
			r.Created = created.String
		}
		r.Starred = starred == 1
		r.AlbumID = albumID
		r.ReplayGain = newReplayGain(rgTrackGain, rgTrackPeak, rgAlbumGain, rgAlbumPeak)

		child := directoryChildFromSong(buildSubsonicSong(r))
		child.CoverArt = albumID // Songs share the album cover
		children = append(children, child)
	}

	dir := &SubsonicMusicDirectory{
		ID:       albumID,
		Parent:   displayArtist,
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

	// Resolve artist ID (MD5 hash) to artist name via the cached ID->name map.
	artistName, found := resolveArtistIDToName(db, artistID)
	if !found {
		log.Printf("Artist not found for ID: %s", artistID)
		subsonicRespond(c, newSubsonicErrorResponse(70, "Artist not found."))
		return
	}
	log.Printf("Resolved artist ID %s to name: %s", artistID, artistName)

	// Get albums by this artist
	// Match on BOTH artist and album_artist fields to show all albums where this artist appears in ANY song
	query := `
		SELECT album, MIN(id) as album_id, COUNT(*) as song_count, COALESCE(genre, '') as genre, MIN(album_path) as album_path, COALESCE(SUM(duration), 0) as total_duration, MIN(date_added) as created
		FROM songs
		WHERE (artist = ? OR album_artist = ?) AND cancelled = 0
		GROUP BY CASE
			WHEN album_path IS NOT NULL AND album_path != '' THEN album_path || '|||' || album
			ELSE album
		END
		ORDER BY album COLLATE NOCASE
	`

	rows, err := db.Query(query, artistName, artistName)
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
		var songCount, totalDuration int
		var genre string
		var albumPath string
		var created sql.NullString

		if err := rows.Scan(&albumName, &albumID, &songCount, &genre, &albumPath, &totalDuration, &created); err != nil {
			log.Printf("Error scanning album: %v", err)
			continue
		}

		// Display artist for this album (precomputed in the derived albums table)
		displayArtist := albumDisplayArtist(db, albumName, strings.TrimSpace(albumPath))

		album := SubsonicAlbum{
			ID:        albumID,
			Name:      albumName,
			Artist:    displayArtist,
			ArtistID:  GenerateArtistID(displayArtist),
			CoverArt:  albumID,
			Genre:     genre,
			SongCount: songCount,
			Duration:  totalDuration,
			Created:   created.String,
		}
		decorateAlbum(&album)
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
