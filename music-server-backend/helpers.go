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

// fetchEffectiveArtists fetches and returns a deduplicated list of effective artists (album_artist fallback)
func fetchEffectiveArtists(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT COALESCE(NULLIF(album_artist, ''), artist) AS artist FROM songs WHERE COALESCE(NULLIF(album_artist, ''), artist) != '' AND cancelled = 0`)
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