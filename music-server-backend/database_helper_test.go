package main

import (
	"database/sql"
	"testing"
	_ "github.com/mattn/go-sqlite3"
	"strconv"
	"time"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}

	create := `
	CREATE TABLE songs (
		id TEXT PRIMARY KEY,
		title TEXT,
		artist TEXT,
		album TEXT,
		album_artist TEXT,
		album_path TEXT,
		genre TEXT,
		path TEXT,
		duration INTEGER,
		play_count INTEGER,
		last_played TEXT,
		cancelled INTEGER DEFAULT 0
	);
	`
	if _, err := db.Exec(create); err != nil {
		db.Close()
		t.Fatalf("failed to create songs table: %v", err)
	}
	return db
}

func TestCountAlbumsIncludesAlbumArtist(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create albums with album_artist present in one
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "s1", "t1", "ArtistA", "AlbumX", "AlbumArtist", "")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "s2", "t2", "ArtistB", "AlbumY", "", "")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "s3", "t3", "ArtistC", "AlbumZ", "AlbumArtist", "somepath")

	// Query albums using QueryAlbums
	albums, err := QueryAlbums(db, AlbumQueryOptions{SearchTerm: "AlbumArtist", GroupByPath: true, IncludeArtist: true})
	if err != nil {
		t.Fatalf("QueryAlbums failed: %v", err)
	}

	count, err := CountAlbums(db, "AlbumArtist")
	if err != nil {
		t.Fatalf("CountAlbums failed: %v", err)
	}

	if len(albums) != count {
		t.Fatalf("expected CountAlbums (%d) to match QueryAlbums length (%d)", count, len(albums))
	}
}

func TestMultiWordArtistSearchAND(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, _ = db.Exec(`INSERT INTO songs (id, title, artist) VALUES (?, ?, ?)`, "a1", "x", "The Beatles")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist) VALUES (?, ?, ?)`, "a2", "y", "Beatles")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist) VALUES (?, ?, ?)`, "a3", "z", "The Rolling Stones")

	// Search for both words 'The Beatles' should only match 'The Beatles', not 'Beatles' alone
	artists, err := QueryArtists(db, ArtistQueryOptions{SearchTerm: "The Beatles"})
	if err != nil {
		t.Fatalf("QueryArtists failed: %v", err)
	}

	if len(artists) != 1 || artists[0].Name != "The Beatles" {
		t.Fatalf("expected only 'The Beatles' match for 'The Beatles', got: %v", artists)
	}

	// Search for single word 'Beatles' should match both 'The Beatles' and 'Beatles'
	artists2, err := QueryArtists(db, ArtistQueryOptions{SearchTerm: "Beatles"})
	if err != nil {
		t.Fatalf("QueryArtists failed: %v", err)
	}

	if len(artists2) < 2 {
		t.Fatalf("expected at least 2 matches for 'Beatles', got %d", len(artists2))
	}
}

// --- QuerySongs tests moved/merged here ---
func TestQuerySongs_OnlyStarred_PaginationAndIncludeStarred(t *testing.T) {
	db := setupFullTestDB(t)
	defer db.Close()

	// Insert songs
	for i := 1; i <= 5; i++ {
		id := "s" + strconv.Itoa(i)
		title := "Song" + strconv.Itoa(i)
		_, _ = db.Exec(`INSERT INTO songs (id, title, duration) VALUES (?, ?, ?)`, id, title, 0)
	}
	// Sanity check: ensure songs were inserted
	var total int
	err := db.QueryRow(`SELECT COUNT(*) FROM songs`).Scan(&total)
	if err != nil { t.Fatalf("count songs failed: %v", err) }
	if total != 5 { t.Fatalf("expected 5 songs inserted, got %d", total) }

	// star s2 and s4 for user 1
	_, _ = db.Exec(`INSERT INTO starred_songs (user_id, song_id, starred_at) VALUES (?, ?, ?)`, 1, "s2", time.Now().Format(time.RFC3339))
	_, _ = db.Exec(`INSERT INTO starred_songs (user_id, song_id, starred_at) VALUES (?, ?, ?)`, 1, "s4", time.Now().Format(time.RFC3339))
	// Sanity: ensure starred rows exist
	var starCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM starred_songs WHERE user_id = ?`, 1).Scan(&starCount)
	if err != nil { t.Fatalf("count starred failed: %v", err) }
	if starCount != 2 { t.Fatalf("expected 2 starred rows, got %d", starCount) }
	// Page 1
	res, err := QuerySongs(db, SongQueryOptions{OnlyStarred: true, IncludeStarred: true, UserID: 1, Limit: 1, Offset: 0, OrderBy: "s.id"})
	if err != nil { t.Fatalf("QuerySongs failed: %v", err) }
	if len(res) != 1 {
		// debug raw query (simple) to see if DB has expected rows
		rows, _ := db.Query(`SELECT s.id FROM songs s LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ? WHERE s.cancelled = 0 AND ss.song_id IS NOT NULL ORDER BY s.id LIMIT ? OFFSET ?`, 1, 1, 0)
		defer rows.Close()
		cnt := 0
		for rows.Next() { cnt++ }

		// debug full QuerySongs-style select and scan to see if scan succeeds
		full := `SELECT s.id, s.title, s.artist, s.album, s.path, s.duration, s.play_count, s.last_played, CASE WHEN ss.song_id IS NOT NULL THEN 1 ELSE 0 END as starred FROM songs s LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ? WHERE s.cancelled = 0 AND ss.song_id IS NOT NULL ORDER BY s.id LIMIT ?`
		r2, err := db.Query(full, 1, 1)
		if err != nil { t.Fatalf("QuerySongs returned 0 but raw full query failed: %v (simple found %d)", err, cnt) }
		defer r2.Close()
		scanCnt := 0
		for r2.Next() {
		var id string
		var title, artist, album, path sql.NullString
		var duration, playCount sql.NullInt64
			var last sql.NullString
			var starredInt sql.NullInt64
			if err := r2.Scan(&id, &title, &artist, &album, &path, &duration, &playCount, &last, &starredInt); err != nil {
				t.Fatalf("raw full scan failed: %v", err)
			}
			scanCnt++
		}
		t.Fatalf("expected 1 result for limit=1, got %d (simple found %d full-scan found %d)", len(res), cnt, scanCnt)
	}
	if !res[0].Starred { t.Fatalf("expected first result to be starred") }

	// Page 2
	res2, err := QuerySongs(db, SongQueryOptions{OnlyStarred: true, IncludeStarred: true, UserID: 1, Limit: 1, Offset: 1, OrderBy: "s.id"})
	if err != nil { t.Fatalf("QuerySongs failed: %v", err) }
	if len(res2) != 1 {
		rows, _ := db.Query(`SELECT s.id FROM songs s LEFT JOIN starred_songs ss ON s.id = ss.song_id AND ss.user_id = ? WHERE s.cancelled = 0 AND ss.song_id IS NOT NULL ORDER BY s.id LIMIT ? OFFSET ?`, 1, 1, 1)
		defer rows.Close()
		cnt := 0
		for rows.Next() { cnt++ }
		t.Fatalf("expected 1 result for second page, got %d (raw found %d)", len(res2), cnt)
	}
	if !res2[0].Starred { t.Fatalf("expected second page result to be starred") }
}

func TestQuerySongs_OrderLimitOffsetDeterminism(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert songs with titles out of order
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album) VALUES (?, ?, ?, ?)`, "a1", "C", "A", "X")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album) VALUES (?, ?, ?, ?)`, "a2", "A", "A", "X")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album) VALUES (?, ?, ?, ?)`, "a3", "B", "A", "X")

	// Default ordering (artist, album, title) should sort by title resulting A,B,C
	res, err := QuerySongs(db, SongQueryOptions{OrderBy: "s.title"})
	if err != nil { t.Fatalf("QuerySongs failed: %v", err) }
	if len(res) < 3 {
		// debug raw query
		rows, _ := db.Query(`SELECT id,title FROM songs WHERE cancelled = 0 ORDER BY title`)
		defer rows.Close()
		cnt := 0
		for rows.Next() { cnt++ }
		t.Fatalf("expected 3 results, got %d (raw found %d)", len(res), cnt)
	}
	if res[0].Title != "A" || res[1].Title != "B" || res[2].Title != "C" {
		t.Fatalf("expected titles ordered A,B,C got %v", []string{res[0].Title, res[1].Title, res[2].Title})
	}
	// Limit + Offset
	res2, err := QuerySongs(db, SongQueryOptions{OrderBy: "s.title", Limit: 2, Offset: 1})
	if err != nil { t.Fatalf("QuerySongs failed: %v", err) }
	if len(res2) != 2 { t.Fatalf("expected 2 results for limit=2 offset=1, got %d", len(res2)) }
	if res2[0].Title != "B" || res2[1].Title != "C" {
		t.Fatalf("expected page to contain B,C got %v", []string{res2[0].Title, res2[1].Title})
	}
}

// setupFullTestDB creates an in-memory DB and the basic tables needed for starred tests
func setupFullTestDB(t *testing.T) *sql.DB {
	db := setupTestDB(t)

	// Create starred_songs and starred_artists tables used by Star functions
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS starred_songs (
			user_id INTEGER,
			song_id TEXT,
			starred_at TEXT
		)
	`)
	if err != nil {
		t.Fatalf("failed to create starred_songs: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS starred_artists (
			user_id INTEGER,
			artist_name TEXT,
			starred_at TEXT
		)
	`)
	if err != nil {
		t.Fatalf("failed to create starred_artists: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS starred_albums (
			user_id INTEGER,
			album_id TEXT,
			starred_at TEXT
		)
	`)
	if err != nil {
		t.Fatalf("failed to create starred_albums: %v", err)
	}

	return db
}
