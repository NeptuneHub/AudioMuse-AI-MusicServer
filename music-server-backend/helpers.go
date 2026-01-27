package main

import (
	"database/sql"
	"strings"
)

// normalizeKey trims, folds case, and collapses whitespace for stable comparisons.
func normalizeKey(s string) string {
	s = strings.TrimSpace(s)
	// Collapse consecutive whitespace and normalize internal spacing
	s = strings.Join(strings.Fields(s), " ")
	// Lowercase for case-insensitive matching
	s = strings.ToLower(s)
	return s
}

// normalizeArtistName returns a canonical artist label (preferred AlbumArtist fallback and Unknown normalization)
func normalizeArtistName(s string) string {
	if s == "" || strings.ToLower(strings.TrimSpace(s)) == "unknown" {
		return "Unknown Artist"
	}
	return s
}

// getArtistAlbumCounts returns a map of artist names to their album counts
// OPTIMIZED: Single query instead of N queries (critical for 100k songs)
func getArtistAlbumCounts(db *sql.DB, artistNames []string) (map[string]int, error) {
	if len(artistNames) == 0 {
		return make(map[string]int), nil
	}

	// Build a query that counts albums per artist in a single pass
	// Use album_path + album for grouping (not just album name)
	query := `
		SELECT
			artist,
			COUNT(DISTINCT CASE
				WHEN album_path IS NOT NULL AND album_path != ''
				THEN album_path || '|||' || album
				ELSE album
			END) as album_count
		FROM songs
		WHERE artist IN (` + strings.Repeat("?,", len(artistNames)-1) + `?)
			AND cancelled = 0
			AND album != ''
		GROUP BY artist
	`

	args := make([]interface{}, len(artistNames))
	for i, name := range artistNames {
		args[i] = name
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var artist string
		var count int
		if err := rows.Scan(&artist, &count); err != nil {
			continue
		}
		counts[artist] = count
	}

	return counts, nil
}

// fetchEffectiveArtists fetches and returns a deduplicated list of effective artists (album_artist fallback)
func fetchEffectiveArtists(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT CASE WHEN album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist') THEN album_artist ELSE artist END AS artist FROM songs WHERE ((album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')) OR artist != '') AND cancelled = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var artists []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		name = normalizeArtistName(name)
		key := normalizeKey(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		artists = append(artists, name)
	}
	return artists, nil
}

// fetchArtists returns a deduplicated list of artists using the artist field only
func fetchArtists(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT artist FROM songs WHERE artist != '' AND cancelled = 0`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := make(map[string]bool)
	var artists []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		key := normalizeKey(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		artists = append(artists, name)
	}
	return artists, nil
}

// getAlbumDisplayArtist returns a display string for an album's artist(s).
// Priority: distinct non-empty album_artist values (sorted) concatenated with "; ",
// otherwise distinct non-empty artist values concatenated with "; ". Returns "Unknown Artist" if none found.
// OPTIMIZED: Single query for better performance with 100k+ songs
func getAlbumDisplayArtist(db *sql.DB, albumName, albumPath string) (string, error) {
	// Use a single query with CASE to prioritize album_artist, fallback to artist
	// This is MUCH faster than two separate queries
	query := `
		SELECT DISTINCT
			CASE
				WHEN album_artist IS NOT NULL
					AND TRIM(album_artist) != ''
					AND LOWER(TRIM(album_artist)) NOT IN ('unknown', 'unknown artist')
				THEN TRIM(album_artist)
				WHEN artist IS NOT NULL
					AND TRIM(artist) != ''
				THEN TRIM(artist)
				ELSE NULL
			END as display_artist
		FROM songs
		WHERE album = ? AND (album_path = ? OR (album_path IS NULL AND ? = '')) AND cancelled = 0
		ORDER BY display_artist COLLATE NOCASE
	`

	rows, err := db.Query(query, albumName, albumPath, albumPath)
	if err != nil {
		return "Unknown Artist", err
	}
	defer rows.Close()

	parts := []string{}
	seen := make(map[string]bool)

	for rows.Next() {
		var displayArtist sql.NullString
		if err := rows.Scan(&displayArtist); err != nil {
			continue
		}

		if !displayArtist.Valid || displayArtist.String == "" {
			continue
		}

		// Normalize for deduplication
		nk := normalizeKey(displayArtist.String)
		if seen[nk] {
			continue
		}
		seen[nk] = true
		parts = append(parts, displayArtist.String)
	}

	if len(parts) > 0 {
		return strings.Join(parts, "; "), nil
	}

	return "Unknown Artist", nil
}