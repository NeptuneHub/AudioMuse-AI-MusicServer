package main

import (
	"database/sql"
	"log"
	"sort"
	"strings"
	"sync"
)

// This file implements Navidrome-style derived tables for artists and albums.
//
// The `songs` table remains the single source of truth (the scanner writes only
// to it). `artists` and `albums` are denormalised summary tables rebuilt from
// `songs` whenever the library changes. Listing, searching and counting albums
// and artists then read these small pre-aggregated tables instead of doing a
// GROUP BY over the whole `songs` table on every request, and the per-album
// "display artist" is computed once during the rebuild rather than with an
// N+1 query per album.
//
// Each derived table has its own FTS5 index (artists_fts / albums_fts) built
// with `remove_diacritics 2`, so searches are accent-insensitive (é == e).

const librarySchemaDDL = `
CREATE TABLE IF NOT EXISTS artists (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	song_count INTEGER NOT NULL DEFAULT 0,
	album_count INTEGER NOT NULL DEFAULT 0,
	search_text TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_artists_name ON artists (name COLLATE NOCASE);

CREATE TABLE IF NOT EXISTS albums (
	group_key TEXT PRIMARY KEY,
	id TEXT NOT NULL,
	name TEXT NOT NULL,
	album_path TEXT NOT NULL DEFAULT '',
	artist TEXT NOT NULL DEFAULT '',
	artist_id TEXT NOT NULL DEFAULT '',
	genre TEXT NOT NULL DEFAULT '',
	song_count INTEGER NOT NULL DEFAULT 0,
	has_album_artist INTEGER NOT NULL DEFAULT 0,
	max_date_added TEXT NOT NULL DEFAULT '',
	min_date_added TEXT NOT NULL DEFAULT '',
	max_last_played TEXT NOT NULL DEFAULT '',
	total_play_count INTEGER NOT NULL DEFAULT 0,
	total_duration INTEGER NOT NULL DEFAULT 0,
	genres TEXT NOT NULL DEFAULT '',
	search_text TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_albums_name ON albums (name COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_albums_artist ON albums (artist COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS idx_albums_id ON albums (id);
`

// libraryRebuildMu serialises rebuilds so two scans finishing close together do
// not interleave their DELETE/INSERT work.
var libraryRebuildMu sync.Mutex

// ensureLibraryDerivedTables creates the artists/albums tables, their indexes,
// and the FTS5 virtual tables. FTS creation is tolerant of builds without fts5.
func ensureLibraryDerivedTables(db *sql.DB) {
	if _, err := db.Exec(librarySchemaDDL); err != nil {
		log.Printf("ensureLibraryDerivedTables: failed to create tables: %v", err)
		return
	}
	// Defensively add any columns missing on a derived table created by an
	// earlier build (CREATE TABLE IF NOT EXISTS does not add new columns).
	albumCols := map[string]string{
		"artist_id": "TEXT NOT NULL DEFAULT ''", "genre": "TEXT NOT NULL DEFAULT ''",
		"has_album_artist": "INTEGER NOT NULL DEFAULT 0", "max_date_added": "TEXT NOT NULL DEFAULT ''",
		"min_date_added": "TEXT NOT NULL DEFAULT ''",
		"max_last_played": "TEXT NOT NULL DEFAULT ''", "total_play_count": "INTEGER NOT NULL DEFAULT 0",
		"total_duration": "INTEGER NOT NULL DEFAULT 0",
		"genres": "TEXT NOT NULL DEFAULT ''", "search_text": "TEXT NOT NULL DEFAULT ''",
	}
	// If total_duration is newly added, the albums table predates the aggregate
	// columns and its rows hold the defaults (0 / ''); flag a rebuild so
	// getAlbumList2 returns real duration/created right after an upgrade.
	needsAggregateRebuild := false
	for col, def := range albumCols {
		added, err := ensureColumnExists(db, "albums", col, def)
		if err != nil {
			log.Printf("ensureLibraryDerivedTables: albums.%s: %v", col, err)
			continue
		}
		if added && col == "total_duration" {
			needsAggregateRebuild = true
		}
	}
	for col, def := range map[string]string{"song_count": "INTEGER NOT NULL DEFAULT 0", "album_count": "INTEGER NOT NULL DEFAULT 0", "search_text": "TEXT NOT NULL DEFAULT ''"} {
		if _, err := ensureColumnExists(db, "artists", col, def); err != nil {
			log.Printf("ensureLibraryDerivedTables: artists.%s: %v", col, err)
		}
	}
	ensureDerivedFTS(db, "artists_fts", "artists")
	ensureDerivedFTS(db, "albums_fts", "albums")

	if needsAggregateRebuild {
		var songs int
		_ = db.QueryRow(`SELECT COUNT(*) FROM songs WHERE cancelled = 0`).Scan(&songs)
		if songs > 0 {
			log.Printf("ensureLibraryDerivedTables: backfilling album duration/created aggregates")
			if err := RebuildLibraryIndex(db); err != nil {
				log.Printf("ensureLibraryDerivedTables: aggregate backfill rebuild: %v", err)
			}
		}
	}
}

func ensureDerivedFTS(db *sql.DB, ftsName, contentTable string) {
	stmt := "CREATE VIRTUAL TABLE IF NOT EXISTS " + ftsName +
		" USING fts5(search_text, content='" + contentTable +
		"', content_rowid='rowid', tokenize='unicode61 remove_diacritics 2')"
	if _, err := db.Exec(stmt); err != nil {
		log.Printf("ensureDerivedFTS: could not create %s (fts5 may be unavailable): %v", ftsName, err)
	}
}

func derivedFTSAvailable(db *sql.DB) bool {
	var n int
	_ = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name IN ('albums_fts','artists_fts')`).Scan(&n)
	return n == 2
}

// albumAccumulator gathers the per-group state needed to emit one albums row.
type albumAccumulator struct {
	groupKey       string
	name           string
	albumPath      string
	id             string
	genre          string
	songCount      int
	hasAlbumArtist bool
	maxDateAdded   string
	minDateAdded   string
	maxLastPlayed  string
	totalPlayCount int
	totalDuration  int
	displaySeen    map[string]string // normalizeKey -> original display token
	searchTokens   map[string]bool
	genreTokens    map[string]bool
}

// artistAccumulator gathers per (raw) artist state for one artists row.
type artistAccumulator struct {
	name       string
	songCount  int
	albumKeys  map[string]bool // distinct non-empty album groups for album_count
}

func effectiveArtist(albumArtist, artist string) string {
	aa := strings.TrimSpace(albumArtist)
	if aa != "" {
		lower := strings.ToLower(aa)
		if lower != "unknown" && lower != "unknown artist" {
			return aa
		}
	}
	return strings.TrimSpace(artist)
}

func albumGroupKey(album, albumPath string) string {
	if strings.TrimSpace(albumPath) != "" {
		return albumPath + "|||" + album
	}
	return album
}

// RebuildLibraryIndex repopulates the artists and albums tables (and their FTS
// indexes) from the current contents of the songs table. It performs a single
// streaming pass over songs and aggregates in memory, reproducing the exact
// grouping, effective-artist and display-artist semantics used by the legacy
// per-request GROUP BY queries.
func RebuildLibraryIndex(db *sql.DB) error {
	libraryRebuildMu.Lock()
	defer libraryRebuildMu.Unlock()

	rows, err := db.Query(`SELECT COALESCE(id,''), COALESCE(title,''), COALESCE(artist,''),
		COALESCE(album,''), COALESCE(album_artist,''), COALESCE(album_path,''), COALESCE(genre,''),
		COALESCE(date_added,''), COALESCE(last_played,''), COALESCE(play_count,0), COALESCE(duration,0)
		FROM songs WHERE cancelled = 0`)
	if err != nil {
		return err
	}

	albumsByKey := make(map[string]*albumAccumulator)
	artistsByName := make(map[string]*artistAccumulator)

	for rows.Next() {
		var id, title, artist, album, albumArtist, albumPath, genre, dateAdded, lastPlayed string
		var playCount int
		var duration int
		if err := rows.Scan(&id, &title, &artist, &album, &albumArtist, &albumPath, &genre, &dateAdded, &lastPlayed, &playCount, &duration); err != nil {
			continue
		}
		artist = strings.TrimSpace(artist)
		album = strings.TrimSpace(album)
		albumPath = strings.TrimSpace(albumPath)

		// --- artist aggregation (keyed by raw artist, matching the artist list) ---
		if artist != "" {
			a := artistsByName[artist]
			if a == nil {
				a = &artistAccumulator{name: artist, albumKeys: make(map[string]bool)}
				artistsByName[artist] = a
			}
			a.songCount++
			if album != "" {
				a.albumKeys[albumGroupKey(album, albumPath)] = true
			}
		}

		// --- album aggregation (albums require a non-empty album name) ---
		if album == "" {
			continue
		}
		key := albumGroupKey(album, albumPath)
		acc := albumsByKey[key]
		if acc == nil {
			acc = &albumAccumulator{
				groupKey:     key,
				name:         album,
				albumPath:    albumPath,
				id:           id,
				genre:        genre,
				displaySeen:  make(map[string]string),
				searchTokens: make(map[string]bool),
				genreTokens:  make(map[string]bool),
			}
			albumsByKey[key] = acc
		}
		acc.songCount++
		if id < acc.id {
			acc.id = id
		}
		if acc.genre == "" && genre != "" {
			acc.genre = genre
		}
		if dateAdded > acc.maxDateAdded {
			acc.maxDateAdded = dateAdded
		}
		// Earliest date_added is the album's "created" timestamp.
		if dateAdded != "" && (acc.minDateAdded == "" || dateAdded < acc.minDateAdded) {
			acc.minDateAdded = dateAdded
		}
		if lastPlayed > acc.maxLastPlayed {
			acc.maxLastPlayed = lastPlayed
		}
		acc.totalPlayCount += playCount
		acc.totalDuration += duration

		// display-artist candidate for this song (album_artist preferred, else artist)
		cand := effectiveArtist(albumArtist, artist)
		if cand != "" {
			acc.displaySeen[normalizeKey(cand)] = cand
		}
		aaTrim := strings.TrimSpace(albumArtist)
		if aaTrim != "" && strings.ToLower(aaTrim) != "unknown" && strings.ToLower(aaTrim) != "unknown artist" {
			acc.hasAlbumArtist = true
		}
		// search tokens: album name + every contributing raw artist / album_artist
		acc.searchTokens[album] = true
		if artist != "" {
			acc.searchTokens[artist] = true
		}
		if aaTrim != "" {
			acc.searchTokens[aaTrim] = true
		}
		// genre tokens: split each song's (possibly multi-valued) genre so album
		// genre filtering matches if any contributing song has the genre.
		for _, g := range strings.Split(genre, ";") {
			if g = strings.TrimSpace(g); g != "" {
				acc.genreTokens[g] = true
			}
		}
	}
	rows.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM artists`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM albums`); err != nil {
		return err
	}

	artStmt, err := tx.Prepare(`INSERT OR REPLACE INTO artists (id, name, song_count, album_count, search_text) VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	for _, a := range artistsByName {
		id := GenerateArtistID(a.name)
		if _, err := artStmt.Exec(id, a.name, a.songCount, len(a.albumKeys), a.name); err != nil {
			artStmt.Close()
			return err
		}
	}
	artStmt.Close()

	albStmt, err := tx.Prepare(`INSERT OR REPLACE INTO albums
		(group_key, id, name, album_path, artist, artist_id, genre, song_count, has_album_artist, max_date_added, min_date_added, max_last_played, total_play_count, total_duration, genres, search_text)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	for _, acc := range albumsByKey {
		display := buildDisplayArtist(acc.displaySeen)
		var hasAA int
		if acc.hasAlbumArtist {
			hasAA = 1
		}
		searchText := buildSearchText(acc.searchTokens)
		genres := joinTokens(acc.genreTokens, ";")
		if _, err := albStmt.Exec(acc.groupKey, acc.id, acc.name, acc.albumPath, display, GenerateArtistID(display),
			acc.genre, acc.songCount, hasAA, acc.maxDateAdded, acc.minDateAdded, acc.maxLastPlayed, acc.totalPlayCount, acc.totalDuration, genres, searchText); err != nil {
			albStmt.Close()
			return err
		}
	}
	albStmt.Close()

	if err := tx.Commit(); err != nil {
		return err
	}

	if derivedFTSAvailable(db) {
		if _, err := db.Exec(`INSERT INTO artists_fts(artists_fts) VALUES('rebuild')`); err != nil {
			log.Printf("RebuildLibraryIndex: artists_fts rebuild failed: %v", err)
		}
		if _, err := db.Exec(`INSERT INTO albums_fts(albums_fts) VALUES('rebuild')`); err != nil {
			log.Printf("RebuildLibraryIndex: albums_fts rebuild failed: %v", err)
		}
	}

	// Also rebuild the songs full-text index. A rescan does not re-insert songs
	// that already exist, so the sync triggers never fire for them; rebuilding
	// here guarantees the songs_fts index matches the songs table after any scan.
	if ftsAvailable(db) {
		if _, err := db.Exec(`INSERT INTO songs_fts(songs_fts) VALUES('rebuild')`); err != nil {
			log.Printf("RebuildLibraryIndex: songs_fts rebuild failed: %v", err)
		}
	}

	log.Printf("RebuildLibraryIndex: %d artists, %d albums", len(artistsByName), len(albumsByKey))
	return nil
}

// albumDisplayArtist returns the precomputed display artist for an album from
// the derived albums table (an O(1) primary-key lookup), falling back to the
// live per-song computation if the album is not yet in the table.
func albumDisplayArtist(db *sql.DB, albumName, albumPath string) string {
	key := albumGroupKey(albumName, strings.TrimSpace(albumPath))
	var artist string
	if err := db.QueryRow(`SELECT artist FROM albums WHERE group_key = ?`, key).Scan(&artist); err == nil && artist != "" {
		return artist
	}
	disp, _ := getAlbumDisplayArtist(db, albumName, albumPath)
	return disp
}

// buildDisplayArtist reproduces getAlbumDisplayArtist: distinct effective-artist
// tokens, sorted case-insensitively, joined with "; ", or "Unknown Artist".
func buildDisplayArtist(seen map[string]string) string {
	if len(seen) == 0 {
		return "Unknown Artist"
	}
	parts := make([]string, 0, len(seen))
	for _, v := range seen {
		parts = append(parts, v)
	}
	sort.Slice(parts, func(i, j int) bool {
		return strings.ToLower(parts[i]) < strings.ToLower(parts[j])
	})
	return strings.Join(parts, "; ")
}

func buildSearchText(tokens map[string]bool) string {
	return joinTokens(tokens, " ")
}

func joinTokens(tokens map[string]bool, sep string) string {
	parts := make([]string, 0, len(tokens))
	for t := range tokens {
		if t != "" {
			parts = append(parts, t)
		}
	}
	sort.Strings(parts)
	return strings.Join(parts, sep)
}

// rebuildLibraryIndexIfEmpty builds the derived tables on first run (e.g. after
// upgrading from a version without them) when songs exist but the tables are empty.
func rebuildLibraryIndexIfEmpty(db *sql.DB) {
	var songs, albums int
	_ = db.QueryRow(`SELECT COUNT(*) FROM songs WHERE cancelled = 0`).Scan(&songs)
	_ = db.QueryRow(`SELECT COUNT(*) FROM albums`).Scan(&albums)
	if songs > 0 && albums == 0 {
		if err := RebuildLibraryIndex(db); err != nil {
			log.Printf("rebuildLibraryIndexIfEmpty: %v", err)
		}
	}
}
