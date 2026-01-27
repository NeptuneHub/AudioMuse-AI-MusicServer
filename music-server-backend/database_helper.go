package main

import (
	"database/sql"
	"fmt"
	"strings"
)

// ============================================================================
// QUERY OPTIONS STRUCTURES
// ============================================================================

// ArtistQueryOptions defines options for artist queries
type ArtistQueryOptions struct {
	UseEffectiveArtist bool   // Use album_artist with fallback to artist
	IncludeCounts      bool   // Include album_count and song_count
	SearchTerm         string // Optional search filter (LIKE)
	Limit              int    // Limit results (0 = no limit)
	Offset             int    // Offset for pagination
	OrderBy            string // Order clause (default: "artist COLLATE NOCASE")
}

// AlbumQueryOptions defines options for album queries
type AlbumQueryOptions struct {
	Artist          string // Filter by artist
	SearchTerm      string // Optional search filter (LIKE)
	IncludeCounts   bool   // Include song_count
	GroupByPath     bool   // Group by album_path + album
	Limit           int    // Limit results (0 = no limit)
	Offset          int    // Offset for pagination
	OrderBy         string // Order clause (default: "album COLLATE NOCASE")
	IncludeAlbumID  bool   // Include MIN(id) as albumId
	IncludeGenre    bool   // Include genre
	IncludeArtist   bool   // Include effective artist
}

// SongQueryOptions defines options for song queries
type SongQueryOptions struct {
	Artist           string   // Filter by artist
	Album            string   // Filter by album
	AlbumPath        string   // Filter by album_path
	Genre            string   // Filter by genre
	SearchTerm       string   // Optional search filter (title, artist, album)
	IDs              []string // Filter by specific IDs
	IncludeStarred   bool     // Include starred status (requires UserID)
	UserID           int      // User ID for starred status
	IncludeGenre     bool     // Include genre field
	Random           bool     // Order by RANDOM()
	Limit            int      // Limit results (0 = no limit)
	Offset           int      // Offset for pagination
	OrderBy          string   // Order clause (default: "artist, album, title")
	IncludeTranscode bool     // Include transcoding settings
	OnlyStarred      bool     // Only return starred songs
}

// ArtistResult represents an artist query result
type ArtistResult struct {
	Name       string
	AlbumCount int
	SongCount  int
}

// AlbumResult represents an album query result
type AlbumResult struct {
	Name      string
	AlbumPath string
	Artist    string
	Genre     string
	AlbumID   string
	SongCount int
}

// SongResult represents a song query result
type SongResult struct {
	ID                  string
	Title               string
	Artist              string
	Album               string
	Path                string
	Duration            int
	PlayCount           int
	LastPlayed          string
	Genre               string
	Starred             bool
	TranscodingEnabled  bool
}

// ============================================================================
// ARTIST QUERIES
// ============================================================================

// QueryArtists fetches artists based on provided options
func QueryArtists(db *sql.DB, opts ArtistQueryOptions) ([]ArtistResult, error) {
	var query strings.Builder
	var args []interface{}

	// Build SELECT clause
	if opts.IncludeCounts {
		query.WriteString(`
			SELECT
				artist AS name,
				COUNT(*) as song_count,
				COUNT(DISTINCT CASE
					WHEN album != '' AND album_path != ''
					THEN album_path || '|||' || album
					WHEN album != '' THEN album
					ELSE NULL
				END) as album_count
			FROM songs
		`)
	} else {
		// Select distinct artist field based on option
		if opts.UseEffectiveArtist {
			query.WriteString(`
				SELECT DISTINCT
					CASE
						WHEN album_artist IS NOT NULL AND TRIM(album_artist) != ''
							AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')
						THEN album_artist
						ELSE artist
					END AS artist
				FROM songs
			`)
		} else {
			query.WriteString(`SELECT DISTINCT artist FROM songs`)
		}
	}

	// Build WHERE clause
	whereClauses := []string{"cancelled = 0"}

	// Artist not empty condition
	if opts.UseEffectiveArtist {
		whereClauses = append(whereClauses,
			`((album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')) OR artist != '')`)
	} else {
		whereClauses = append(whereClauses, "artist != ''")
	}

	// Search filter (support multi-word AND semantics)
	if opts.SearchTerm != "" {
		words := strings.Fields(opts.SearchTerm)
		var termClauses []string
		if opts.UseEffectiveArtist {
			for _, w := range words {
				termClauses = append(termClauses, "(artist LIKE ? OR album_artist LIKE ?)")
				p := "%" + w + "%"
				args = append(args, p, p)
			}
		} else {
			for _, w := range words {
				termClauses = append(termClauses, "artist LIKE ?")
				args = append(args, "%"+w+"%")
			}
		}
		whereClauses = append(whereClauses, strings.Join(termClauses, " AND "))
	}

	query.WriteString(" WHERE " + strings.Join(whereClauses, " AND "))

	// GROUP BY for aggregation
	if opts.IncludeCounts {
		query.WriteString(" GROUP BY artist")
	}

	// ORDER BY
	orderBy := opts.OrderBy
	if orderBy == "" {
		orderBy = "artist COLLATE NOCASE"
	}
	query.WriteString(" ORDER BY " + orderBy)

	// LIMIT and OFFSET
	if opts.Limit > 0 {
		query.WriteString(" LIMIT ?")
		args = append(args, opts.Limit)
		if opts.Offset > 0 {
			query.WriteString(" OFFSET ?")
			args = append(args, opts.Offset)
		}
	}

	// Execute query
	rows, err := db.Query(query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Parse results
	var results []ArtistResult
	seen := make(map[string]bool)

	for rows.Next() {
		var result ArtistResult

		if opts.IncludeCounts {
			if err := rows.Scan(&result.Name, &result.SongCount, &result.AlbumCount); err != nil {
				continue
			}
		} else {
			if err := rows.Scan(&result.Name); err != nil {
				continue
			}
		}

		// Deduplicate if not using aggregation
		if !opts.IncludeCounts {
			key := normalizeKey(result.Name)
			if seen[key] {
				continue
			}
			seen[key] = true
		}

		results = append(results, result)
	}

	return results, nil
}

// QueryArtistPath returns the path of a song by the given artist (for cover art)
func QueryArtistPath(db *sql.DB, artistName string) (string, error) {
	var path string
	err := db.QueryRow(`SELECT path FROM songs WHERE artist = ? AND cancelled = 0 LIMIT 1`, artistName).Scan(&path)
	return path, err
}

// ============================================================================
// ALBUM QUERIES
// ============================================================================

// QueryAlbums fetches albums based on provided options
func QueryAlbums(db *sql.DB, opts AlbumQueryOptions) ([]AlbumResult, error) {
	var query strings.Builder
	var args []interface{}

	// Build SELECT clause
	query.WriteString(`SELECT `)

	selectFields := []string{"album"}

	if opts.GroupByPath {
		selectFields = append(selectFields, "MIN(NULLIF(album_path, '')) as album_path")
	} else {
		selectFields = append(selectFields, "album_path")
	}

	if opts.IncludeArtist {
		selectFields = append(selectFields,
			`COALESCE(NULLIF(album_artist, ''), artist) as effective_artist`)
	}

	if opts.IncludeGenre {
		selectFields = append(selectFields, "COALESCE(genre, '') as genre")
	}

	if opts.IncludeAlbumID {
		selectFields = append(selectFields, "MIN(id) as albumId")
	}

	if opts.IncludeCounts {
		selectFields = append(selectFields, "COUNT(*) as song_count")
	}

	query.WriteString(strings.Join(selectFields, ", "))
	query.WriteString(" FROM songs")

	// Build WHERE clause
	whereClauses := []string{"album != ''", "cancelled = 0"}

	if opts.Artist != "" {
		whereClauses = append(whereClauses,
			`(artist = ? OR album_artist = ?)`)
		args = append(args, opts.Artist, opts.Artist)
	}

	if opts.SearchTerm != "" {
		words := strings.Fields(opts.SearchTerm)
		var termClauses []string
		for _, w := range words {
			termClauses = append(termClauses, "(album LIKE ? OR artist LIKE ? OR album_artist LIKE ?)")
			p := "%" + w + "%"
			args = append(args, p, p, p)
		}
		whereClauses = append(whereClauses, strings.Join(termClauses, " AND "))
	}

	query.WriteString(" WHERE " + strings.Join(whereClauses, " AND "))

	// GROUP BY for aggregation or path grouping
	if opts.GroupByPath {
		query.WriteString(` GROUP BY CASE
			WHEN album_path IS NOT NULL AND album_path != ''
			THEN album_path || '|||' || album
			ELSE album
		END`)
	}

	// ORDER BY
	orderBy := opts.OrderBy
	if orderBy == "" {
		if opts.IncludeArtist {
			orderBy = "effective_artist COLLATE NOCASE, album COLLATE NOCASE"
		} else {
			orderBy = "album COLLATE NOCASE"
		}
	}
	query.WriteString(" ORDER BY " + orderBy)

	// LIMIT and OFFSET
	if opts.Limit > 0 {
		query.WriteString(" LIMIT ?")
		args = append(args, opts.Limit)
		if opts.Offset > 0 {
			query.WriteString(" OFFSET ?")
			args = append(args, opts.Offset)
		}
	}

	// Execute query
	rows, err := db.Query(query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Parse results
	var results []AlbumResult
	for rows.Next() {
		var result AlbumResult
		var albumPath sql.NullString
		var genre sql.NullString
		var albumID sql.NullString

		scanArgs := []interface{}{&result.Name}

		scanArgs = append(scanArgs, &albumPath)

		if opts.IncludeArtist {
			scanArgs = append(scanArgs, &result.Artist)
		}

		if opts.IncludeGenre {
			scanArgs = append(scanArgs, &genre)
		}

		if opts.IncludeAlbumID {
			scanArgs = append(scanArgs, &albumID)
		}

		if opts.IncludeCounts {
			scanArgs = append(scanArgs, &result.SongCount)
		}

		if err := rows.Scan(scanArgs...); err != nil {
			continue
		}

		if albumPath.Valid {
			result.AlbumPath = albumPath.String
		}
		if genre.Valid {
			result.Genre = genre.String
		}
		if albumID.Valid {
			result.AlbumID = albumID.String
		}

		results = append(results, result)
	}

	return results, nil
}

// QueryAlbumDetails returns album details from a song ID
func QueryAlbumDetails(db *sql.DB, songID string) (album, artist, genre, path string, err error) {
	err = db.QueryRow(`
		SELECT album, artist, COALESCE(genre, ''), path
		FROM songs
		WHERE id = ? AND cancelled = 0`,
		songID).Scan(&album, &artist, &genre, &path)
	return
}

// QueryAlbumSongCount returns the count of songs in an album
func QueryAlbumSongCount(db *sql.DB, album, artist string) (int, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM songs
		WHERE album = ? AND artist = ? AND cancelled = 0`,
		album, artist).Scan(&count)
	return count, err
}

// CheckAlbumHasAlbumArtist checks if an album has any songs with album_artist field set
func CheckAlbumHasAlbumArtist(db *sql.DB, album, albumPath string) (bool, error) {
	var exists bool
	var query string
	var args []interface{}

	if albumPath != "" {
		query = `SELECT EXISTS(
			SELECT 1 FROM songs
			WHERE album = ? AND album_path = ?
				AND album_artist IS NOT NULL
				AND TRIM(album_artist) != ''
				AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')
				AND cancelled = 0
		)`
		args = []interface{}{album, albumPath}
	} else {
		query = `SELECT EXISTS(
			SELECT 1 FROM songs
			WHERE album = ?
				AND (album_path IS NULL OR album_path = '')
				AND album_artist IS NOT NULL
				AND TRIM(album_artist) != ''
				AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')
				AND cancelled = 0
		)`
		args = []interface{}{album}
	}

	err := db.QueryRow(query, args...).Scan(&exists)
	return exists, err
}

// ============================================================================
// SONG QUERIES
// ============================================================================

// QuerySongs fetches songs based on provided options
func QuerySongs(db *sql.DB, opts SongQueryOptions) ([]SongResult, error) {
	var query strings.Builder
	var args []interface{}

	// Build SELECT clause
	query.WriteString(`SELECT s.id, s.title, s.artist, s.album, s.path, s.duration, s.play_count, s.last_played`)

	if opts.IncludeGenre {
		query.WriteString(`, COALESCE(s.genre, '') as genre`)
	}

	if opts.IncludeStarred {
		query.WriteString(`, CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred`)
	}

	if opts.IncludeTranscode {
		query.WriteString(`, CASE WHEN pref.enabled THEN 1 ELSE 0 END as transcoding_enabled`)
	}

	query.WriteString(` FROM songs s`)

	// JOINs
	if opts.IncludeStarred {
		query.WriteString(` LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ?`)
		args = append(args, opts.UserID)
	}

	if opts.IncludeTranscode {
		query.WriteString(` LEFT JOIN transcoding_settings pref ON s.id = pref.song_id AND pref.user_id = ?`)
		args = append(args, opts.UserID)
	}

	// Build WHERE clause
	whereClauses := []string{"s.cancelled = 0"}

	if opts.Artist != "" {
		whereClauses = append(whereClauses, "s.artist = ?")
		args = append(args, opts.Artist)
	}

	if opts.Album != "" {
		whereClauses = append(whereClauses, "s.album = ?")
		args = append(args, opts.Album)
	}

	if opts.AlbumPath != "" {
		whereClauses = append(whereClauses, "s.album_path = ?")
		args = append(args, opts.AlbumPath)
	}

	if opts.Genre != "" {
		whereClauses = append(whereClauses, "s.genre = ?")
		args = append(args, opts.Genre)
	}

	if opts.SearchTerm != "" {
		words := strings.Fields(opts.SearchTerm)
		var termClauses []string
		for _, w := range words {
			termClauses = append(termClauses, "(s.title LIKE ? OR s.artist LIKE ? OR s.album LIKE ?)")
			p := "%" + w + "%"
			args = append(args, p, p, p)
		}
		whereClauses = append(whereClauses, strings.Join(termClauses, " AND "))
	}

	if len(opts.IDs) > 0 {
		placeholders := strings.Repeat("?,", len(opts.IDs)-1) + "?"
		whereClauses = append(whereClauses, "s.id IN ("+placeholders+")")
		for _, id := range opts.IDs {
			args = append(args, id)
		}
	}

	if opts.OnlyStarred {
		whereClauses = append(whereClauses, "ss.song_id IS NOT NULL")
	}

	query.WriteString(" WHERE " + strings.Join(whereClauses, " AND "))

	// ORDER BY
	orderBy := opts.OrderBy
	if orderBy == "" {
		if opts.Random {
			orderBy = "RANDOM()"
		} else {
			orderBy = "s.artist, s.album, s.title"
		}
	}
	query.WriteString(" ORDER BY " + orderBy)

	// LIMIT and OFFSET
	if opts.Limit > 0 {
		query.WriteString(" LIMIT ?")
		args = append(args, opts.Limit)
		if opts.Offset > 0 {
			query.WriteString(" OFFSET ?")
			args = append(args, opts.Offset)
		}
	}

	// Execute query
	rows, err := db.Query(query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Parse results
	var results []SongResult
	for rows.Next() {
		var result SongResult
		var lastPlayed sql.NullString
		var genre sql.NullString

		scanArgs := []interface{}{
			&result.ID, &result.Title, &result.Artist, &result.Album,
			&result.Path, &result.Duration, &result.PlayCount, &lastPlayed,
		}

		if opts.IncludeGenre {
			scanArgs = append(scanArgs, &genre)
		}

		if opts.IncludeStarred {
			scanArgs = append(scanArgs, &result.Starred)
		}

		if opts.IncludeTranscode {
			scanArgs = append(scanArgs, &result.TranscodingEnabled)
		}

		if err := rows.Scan(scanArgs...); err != nil {
			continue
		}

		if lastPlayed.Valid {
			result.LastPlayed = lastPlayed.String
		}
		if genre.Valid {
			result.Genre = genre.String
		}

		results = append(results, result)
	}

	return results, nil
}

// QuerySongByID fetches a single song by ID
func QuerySongByID(db *sql.DB, songID string) (*SongResult, error) {
	results, err := QuerySongs(db, SongQueryOptions{
		IDs:          []string{songID},
		IncludeGenre: true,
		Limit:        1,
	})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, sql.ErrNoRows
	}
	return &results[0], nil
}

// QuerySongPath returns only the path for a song ID
func QuerySongPath(db *sql.DB, songID string) (string, error) {
	var path string
	err := db.QueryRow(`SELECT path FROM songs WHERE id = ? AND cancelled = 0`, songID).Scan(&path)
	return path, err
}

// QuerySongPathAndDuration returns path and duration for a song ID
func QuerySongPathAndDuration(db *sql.DB, songID string) (path string, duration int, err error) {
	err = db.QueryRow(`SELECT path, duration FROM songs WHERE id = ? AND cancelled = 0`, songID).Scan(&path, &duration)
	return
}

// QuerySongWaveform returns path, duration, and waveform peaks for a song ID
func QuerySongWaveform(db *sql.DB, songID string) (path string, duration int, waveformPeaks string, err error) {
	var peaks sql.NullString
	err = db.QueryRow(`SELECT path, duration, waveform_peaks FROM songs WHERE id = ? AND cancelled = 0`,
		songID).Scan(&path, &duration, &peaks)
	if peaks.Valid {
		waveformPeaks = peaks.String
	}
	return
}

// QuerySimilarSongs finds similar songs based on artist and genre
func QuerySimilarSongs(db *sql.DB, songID string, limit int) ([]SongResult, error) {
	// First, get the artist and genre of the reference song
	var artist, genre string
	err := db.QueryRow(`SELECT artist, COALESCE(genre, '') FROM songs WHERE id = ? AND cancelled = 0`,
		songID).Scan(&artist, &genre)
	if err != nil {
		return nil, err
	}

	// Build query for similar songs
	query := `
		SELECT id, title, artist, album, path, play_count, last_played, genre, duration
		FROM songs
		WHERE cancelled = 0 AND id != ?
			AND (artist = ? OR genre = ?)
		ORDER BY
			CASE WHEN artist = ? AND genre = ? THEN 0
				 WHEN artist = ? THEN 1
				 WHEN genre = ? THEN 2
				 ELSE 3 END,
			RANDOM()
		LIMIT ?
	`

	rows, err := db.Query(query, songID, artist, genre, artist, genre, artist, genre, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SongResult
	for rows.Next() {
		var result SongResult
		var lastPlayed sql.NullString
		var genreVal sql.NullString

		if err := rows.Scan(&result.ID, &result.Title, &result.Artist, &result.Album,
			&result.Path, &result.PlayCount, &lastPlayed, &genreVal, &result.Duration); err != nil {
			continue
		}

		if lastPlayed.Valid {
			result.LastPlayed = lastPlayed.String
		}
		if genreVal.Valid {
			result.Genre = genreVal.String
		}

		results = append(results, result)
	}

	return results, nil
}

// ============================================================================
// COUNT QUERIES
// ============================================================================

// CountSongs returns the total number of non-cancelled songs
func CountSongs(db *sql.DB, searchTerm string) (int, error) {
	var count int
	var query string
	var args []interface{}

	if searchTerm != "" {
		query = `SELECT COUNT(*) FROM songs WHERE (title LIKE ? OR artist LIKE ? OR album LIKE ?) AND cancelled = 0`
		searchPattern := "%" + searchTerm + "%"
		args = []interface{}{searchPattern, searchPattern, searchPattern}
	} else {
		query = `SELECT COUNT(*) FROM songs WHERE cancelled = 0`
	}

	err := db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// CountArtists returns the total number of distinct artists
func CountArtists(db *sql.DB, searchTerm string, useEffective bool) (int, error) {
	var count int
	var query string
	var args []interface{}

	if useEffective {
		if searchTerm != "" {
			query = `SELECT COUNT(DISTINCT CASE
				WHEN album_artist IS NOT NULL AND TRIM(album_artist) != ''
					AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')
				THEN album_artist ELSE artist END)
			FROM songs
			WHERE (artist LIKE ? OR album_artist LIKE ?) AND cancelled = 0`
			searchPattern := "%" + searchTerm + "%"
			args = []interface{}{searchPattern, searchPattern}
		} else {
			query = `SELECT COUNT(DISTINCT CASE
				WHEN album_artist IS NOT NULL AND TRIM(album_artist) != ''
					AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')
				THEN album_artist ELSE artist END)
			FROM songs WHERE cancelled = 0`
		}
	} else {
		if searchTerm != "" {
			query = `SELECT COUNT(DISTINCT artist) FROM songs WHERE artist LIKE ? AND artist != '' AND cancelled = 0`
			args = []interface{}{"%" + searchTerm + "%"}
		} else {
			query = `SELECT COUNT(DISTINCT artist) FROM songs WHERE artist != '' AND cancelled = 0`
		}
	}

	err := db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// CountAlbums returns the total number of distinct albums
func CountAlbums(db *sql.DB, searchTerm string) (int, error) {
	var count int
	var query string
	var args []interface{}

	if searchTerm != "" {
		query = `SELECT COUNT(DISTINCT CASE
			WHEN album_path IS NOT NULL AND album_path != ''
			THEN album_path || '|||' || album ELSE album END)
		FROM songs
		WHERE (album LIKE ? OR artist LIKE ? OR album_artist LIKE ?) AND album != '' AND cancelled = 0`
		searchPattern := "%" + searchTerm + "%"
		args = []interface{}{searchPattern, searchPattern, searchPattern}
	} else {
		query = `SELECT COUNT(DISTINCT CASE
			WHEN album_path IS NOT NULL AND album_path != ''
			THEN album_path || '|||' || album ELSE album END)
		FROM songs WHERE album != '' AND cancelled = 0`
	}

	err := db.QueryRow(query, args...).Scan(&count)
	return count, err
}

// ============================================================================
// EXISTENCE CHECKS
// ============================================================================

// SongExists checks if a song exists by ID
func SongExists(db *sql.DB, songID string) (bool, error) {
	var exists bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM songs WHERE id = ? AND cancelled = 0)`,
		songID).Scan(&exists)
	return exists, err
}

// ArtistExists checks if an artist exists
func ArtistExists(db *sql.DB, artistName string) (bool, error) {
	var exists bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM songs WHERE (album_artist = ? OR artist = ?) AND cancelled = 0)`,
		artistName, artistName).Scan(&exists)
	return exists, err
}

// AlbumExists checks if an album exists with album_artist backing
func AlbumExists(db *sql.DB, album, albumArtist string) (bool, error) {
	var exists bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM songs WHERE album = ? AND album_artist = ? AND cancelled = 0)`,
		album, albumArtist).Scan(&exists)
	return exists, err
}

// SongExistsByPath checks if a song exists by file path
func SongExistsByPath(db *sql.DB, path string) (bool, error) {
	var exists bool
	err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM songs WHERE path = ?)`, path).Scan(&exists)
	return exists, err
}

// ============================================================================
// UPDATE OPERATIONS
// ============================================================================

// UpdateSongPlayCount increments play count and updates last_played timestamp
func UpdateSongPlayCount(db *sql.DB, songID, timestamp string) error {
	_, err := db.Exec(`UPDATE songs SET play_count = play_count + 1, last_played = ? WHERE id = ?`,
		timestamp, songID)
	return err
}

// UpdateSongCancelled marks a song as cancelled
func UpdateSongCancelled(db *sql.DB, songID string, cancelled bool) error {
	cancelledInt := 0
	if cancelled {
		cancelledInt = 1
	}
	_, err := db.Exec(`UPDATE songs SET cancelled = ? WHERE id = ?`, cancelledInt, songID)
	return err
}

// UpsertSong inserts or updates a song in the database
func UpsertSong(db *sql.DB, song Song) error {
	_, err := db.Exec(`
		INSERT INTO songs (id, title, artist, album, album_artist, path, album_path, genre, duration, date_added, date_updated, cancelled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			title = excluded.title,
			artist = excluded.artist,
			album = excluded.album,
			album_artist = excluded.album_artist,
			genre = excluded.genre,
			duration = excluded.duration,
			date_updated = excluded.date_updated
	`, song.ID, song.Title, song.Artist, song.Album, song.AlbumArtist, song.Path,
	   "", song.Genre, song.Duration, song.DateAdded, song.DateUpdated, song.Cancelled)
	return err
}

// ============================================================================
// STARRING OPERATIONS
// ============================================================================

// StarSong adds a song to user's starred list
func StarSong(db *sql.DB, userID int, songID, timestamp string) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO starred_songs (user_id, song_id, starred_at) VALUES (?, ?, ?)`,
		userID, songID, timestamp)
	return err
}

// UnstarSong removes a song from user's starred list
func UnstarSong(db *sql.DB, userID int, songID string) error {
	_, err := db.Exec(`DELETE FROM starred_songs WHERE user_id = ? AND song_id = ?`, userID, songID)
	return err
}

// StarArtist adds an artist to user's starred list
func StarArtist(db *sql.DB, userID int, artistName, timestamp string) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO starred_artists (user_id, artist_name, starred_at) VALUES (?, ?, ?)`,
		userID, artistName, timestamp)
	return err
}

// UnstarArtist removes an artist from user's starred list
func UnstarArtist(db *sql.DB, userID int, artistName string) error {
	_, err := db.Exec(`DELETE FROM starred_artists WHERE user_id = ? AND artist_name = ?`, userID, artistName)
	return err
}

// StarAlbum adds an album to user's starred list
func StarAlbum(db *sql.DB, userID int, albumID, timestamp string) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO starred_albums (user_id, album_id, starred_at) VALUES (?, ?, ?)`,
		userID, albumID, timestamp)
	return err
}

// UnstarAlbum removes an album from user's starred list
func UnstarAlbum(db *sql.DB, userID int, albumID string) error {
	_, err := db.Exec(`DELETE FROM starred_albums WHERE user_id = ? AND album_id = ?`, userID, albumID)
	return err
}

// ============================================================================
// PLAY HISTORY
// ============================================================================

// InsertPlayHistory records a song play in history
func InsertPlayHistory(db *sql.DB, userID int, songID, playedAt string) error {
	_, err := db.Exec(`INSERT INTO play_history (user_id, song_id, played_at) VALUES (?, ?, ?)`,
		userID, songID, playedAt)
	return err
}

// ============================================================================
// CONFIGURATION HELPERS
// ============================================================================

// GetConfig retrieves a configuration value by key
func GetConfig(db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRow(`SELECT value FROM configuration WHERE key = ?`, key).Scan(&value)
	return value, err
}

// SetConfig sets a configuration value
func SetConfig(db *sql.DB, key, value string) error {
	_, err := db.Exec(`INSERT OR REPLACE INTO configuration (key, value) VALUES (?, ?)`, key, value)
	return err
}

// GetAllConfig retrieves all configuration key-value pairs
func GetAllConfig(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query(`SELECT key, value FROM configuration`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	config := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		config[key] = value
	}

	return config, nil
}

// ============================================================================
// GENRE QUERIES
// ============================================================================

// QueryGenres returns all genres with song and album counts
func QueryGenres(db *sql.DB) (map[string]struct{ SongCount, AlbumCount int }, error) {
	query := `
		SELECT
			COALESCE(genre, 'Unknown') as genre,
			COUNT(*) as song_count,
			COUNT(DISTINCT CASE
				WHEN album != '' AND album_path != ''
				THEN album_path || '|||' || album
				WHEN album != '' THEN album
				ELSE NULL
			END) as album_count
		FROM songs
		WHERE cancelled = 0
		GROUP BY genre
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	genres := make(map[string]struct{ SongCount, AlbumCount int })
	for rows.Next() {
		var genre string
		var songCount, albumCount int
		if err := rows.Scan(&genre, &songCount, &albumCount); err != nil {
			continue
		}
		genres[genre] = struct{ SongCount, AlbumCount int }{songCount, albumCount}
	}

	return genres, nil
}

// ============================================================================
// PLAYLIST HELPERS
// ============================================================================

// GetPlaylistSongs returns songs in a playlist ordered by position
func GetPlaylistSongs(db *sql.DB, playlistID, userID int) ([]SongResult, error) {
	query := `
		SELECT s.id, s.title, s.artist, s.album, s.path, s.duration, s.play_count, s.last_played,
			COALESCE(s.genre, '') as genre,
			CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred
		FROM playlist_songs ps
		JOIN songs s ON ps.song_id = s.id
		LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ?
		WHERE ps.playlist_id = ? AND s.cancelled = 0
		ORDER BY ps.position
	`

	rows, err := db.Query(query, userID, playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SongResult
	for rows.Next() {
		var result SongResult
		var lastPlayed sql.NullString

		if err := rows.Scan(&result.ID, &result.Title, &result.Artist, &result.Album,
			&result.Path, &result.Duration, &result.PlayCount, &lastPlayed,
			&result.Genre, &result.Starred); err != nil {
			continue
		}

		if lastPlayed.Valid {
			result.LastPlayed = lastPlayed.String
		}

		results = append(results, result)
	}

	return results, nil
}

// AddSongsToPlaylist adds songs to a playlist
func AddSongsToPlaylist(db *sql.DB, playlistID int, songIDs []string) error {
	// Get current max position
	var maxPos int
	err := db.QueryRow(`SELECT COALESCE(MAX(position), 0) FROM playlist_songs WHERE playlist_id = ?`,
		playlistID).Scan(&maxPos)
	if err != nil {
		return err
	}

	// Insert songs
	stmt, err := db.Prepare(`INSERT INTO playlist_songs (playlist_id, song_id, position) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i, songID := range songIDs {
		_, err := stmt.Exec(playlistID, songID, maxPos+i+1)
		if err != nil {
			return err
		}
	}

	return nil
}

// ============================================================================
// BATCH OPERATIONS
// ============================================================================

// QuerySongsByIDs efficiently fetches multiple songs by their IDs
func QuerySongsByIDs(db *sql.DB, songIDs []string) ([]SongResult, error) {
	if len(songIDs) == 0 {
		return []SongResult{}, nil
	}

	placeholders := strings.Repeat("?,", len(songIDs)-1) + "?"
	query := fmt.Sprintf(`
		SELECT id, title, artist, album, path, play_count, last_played, duration
		FROM songs
		WHERE id IN (%s)
	`, placeholders)

	args := make([]interface{}, len(songIDs))
	for i, id := range songIDs {
		args[i] = id
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SongResult
	for rows.Next() {
		var result SongResult
		var lastPlayed sql.NullString

		if err := rows.Scan(&result.ID, &result.Title, &result.Artist, &result.Album,
			&result.Path, &result.PlayCount, &lastPlayed, &result.Duration); err != nil {
			continue
		}

		if lastPlayed.Valid {
			result.LastPlayed = lastPlayed.String
		}

		results = append(results, result)
	}

	return results, nil
}

// GetSongIDByPath returns the song ID for a given file path
func GetSongIDByPath(db *sql.DB, path string) (string, error) {
	var id string
	err := db.QueryRow(`SELECT id FROM songs WHERE path = ?`, path).Scan(&id)
	return id, err
}
