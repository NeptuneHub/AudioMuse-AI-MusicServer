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
func getAlbumDisplayArtist(db *sql.DB, albumName, albumPath string) (string, error) {
	// Simplified logic per request:
	// 1) album_artist for songs matching album + album_path
	rows, err := db.Query("SELECT DISTINCT TRIM(album_artist) FROM songs WHERE album = ? AND album_path = ? AND album_artist != '' AND cancelled = 0 ORDER BY album_artist COLLATE NOCASE", albumName, albumPath)
	if err == nil {
		defer rows.Close()
		parts := []string{}
		seen := make(map[string]bool)
		for rows.Next() {
			var a string
			if err := rows.Scan(&a); err != nil {
				continue
			}
			a = strings.TrimSpace(a)
			if a == "" {
				continue
			}
			// Ignore placeholder/unknown values that may have been written previously
			nk := normalizeKey(a)
			if nk == "unknown artist" || nk == "unknown" {
				continue
			}
			if seen[nk] {
				continue
			}
			seen[nk] = true
			parts = append(parts, a)
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; "), nil
		}
	}

	// 2) track artists for songs matching album + album_path
	rows2, err2 := db.Query("SELECT DISTINCT TRIM(artist) FROM songs WHERE album = ? AND album_path = ? AND artist != '' AND cancelled = 0 ORDER BY artist COLLATE NOCASE", albumName, albumPath)
	if err2 == nil {
		defer rows2.Close()
		parts := []string{}
		seen := make(map[string]bool)
		for rows2.Next() {
			var a string
			if err := rows2.Scan(&a); err != nil {
				continue
			}
			a = strings.TrimSpace(a)
			if a == "" {
				continue
			}
			// Ignore placeholder/unknown values when considering track artists
			nk := normalizeKey(a)
			if nk == "unknown artist" || nk == "unknown" {
				continue
			}
			if seen[nk] {
				continue
			}
			seen[nk] = true
			parts = append(parts, a)
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; "), nil
		}
	}

	return "Unknown Artist", nil
}