package main

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func buildLargeDB(tb testing.TB, n int) *sql.DB {
	tb.Helper()
	dir := tb.TempDir()
	dbp := dir + "/perf.db"
	d, err := sql.Open("sqlite3", dbp+"?_journal_mode=WAL")
	if err != nil {
		tb.Fatalf("open: %v", err)
	}
	_, _ = d.Exec("PRAGMA synchronous = OFF")
	_, _ = d.Exec("PRAGMA temp_store = MEMORY")

	schema := `CREATE TABLE songs (
		id TEXT PRIMARY KEY NOT NULL,
		title TEXT, artist TEXT, album TEXT, album_artist TEXT DEFAULT '',
		path TEXT UNIQUE NOT NULL, play_count INTEGER DEFAULT 0, last_played TEXT,
		date_added TEXT, date_updated TEXT, starred INTEGER DEFAULT 0,
		genre TEXT DEFAULT '', album_path TEXT DEFAULT '', duration INTEGER DEFAULT 0,
		cancelled INTEGER NOT NULL DEFAULT 0
	);`
	if _, err := d.Exec(schema); err != nil {
		tb.Fatalf("schema: %v", err)
	}
	if _, err := d.Exec(`CREATE VIRTUAL TABLE songs_fts USING fts5(title, artist, album, album_artist, content='songs', content_rowid='rowid');`); err != nil {
		tb.Fatalf("fts5 not available (build with -tags fts5): %v", err)
	}

	vocab := []string{"love", "night", "blue", "fire", "heart", "dream", "rain", "sun", "moon", "road",
		"river", "gold", "dark", "light", "wild", "free", "lost", "home", "time", "song"}

	tx, _ := d.Begin()
	stmt, err := tx.Prepare(`INSERT INTO songs (id, title, artist, album, album_artist, path, album_path, genre, duration) VALUES (?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		tb.Fatalf("prepare: %v", err)
	}
	for i := 0; i < n; i++ {
		w := vocab[i%len(vocab)]
		artist := fmt.Sprintf("Artist %d", i%50000)
		album := fmt.Sprintf("%s Album %d", w, i%200000)
		title := fmt.Sprintf("%s Song %d", w, i)
		albumPath := fmt.Sprintf("/music/%d", i%200000)
		id := fmt.Sprintf("id%d", i)
		path := fmt.Sprintf("/music/%d/%d.flac", i%200000, i)
		if _, err := stmt.Exec(id, title, artist, album, artist, path, albumPath, "Rock", 200); err != nil {
			tb.Fatalf("insert %d: %v", i, err)
		}
		if i%100000 == 0 && i > 0 {
			_ = tx.Commit()
			tx, _ = d.Begin()
			stmt, _ = tx.Prepare(`INSERT INTO songs (id, title, artist, album, album_artist, path, album_path, genre, duration) VALUES (?,?,?,?,?,?,?,?,?)`)
		}
	}
	_ = tx.Commit()
	if _, err := d.Exec(`INSERT INTO songs_fts(songs_fts) VALUES('rebuild')`); err != nil {
		tb.Fatalf("fts rebuild: %v", err)
	}
	ensureSongSearchIndexes(d)
	return d
}

func TestMusicCountsQueriesAreCorrect(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	rows := [][]string{
		{"s1", "t1", "ArtistA", "AlbumX", "/m/x"},
		{"s2", "t2", "ArtistA", "AlbumX", "/m/x"},
		{"s3", "t3", "ArtistB", "AlbumY", "/m/y"},
		{"s4", "t4", "ArtistC", "", "/m/z"},
	}
	for _, r := range rows {
		if _, err := db.Exec(`INSERT INTO songs (id, title, artist, album, album_path) VALUES (?,?,?,?,?)`, r[0], r[1], r[2], r[3], r[4]); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	var artists, albums, songs int
	if err := db.QueryRow("SELECT COUNT(DISTINCT artist) FROM songs WHERE artist != '' AND cancelled = 0").Scan(&artists); err != nil {
		t.Fatalf("artist count: %v", err)
	}
	albumQuery := `SELECT COUNT(DISTINCT CASE
		WHEN album IS NOT NULL AND TRIM(album) != '' THEN album
		ELSE album_path END)
		FROM songs WHERE cancelled = 0 AND (TRIM(album) != '' OR TRIM(album_path) != '')`
	if err := db.QueryRow(albumQuery).Scan(&albums); err != nil {
		t.Fatalf("album count: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM songs WHERE cancelled = 0").Scan(&songs); err != nil {
		t.Fatalf("song count: %v", err)
	}

	if artists != 3 {
		t.Errorf("artists = %d, want 3", artists)
	}
	if albums != 3 {
		t.Errorf("albums = %d, want 3", albums)
	}
	if songs != 4 {
		t.Errorf("songs = %d, want 4", songs)
	}
}

func TestSongOnlySearchReturnsSongs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	for i := 0; i < 5; i++ {
		_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album) VALUES (?,?,?,?)`,
			fmt.Sprintf("s%d", i), fmt.Sprintf("Love Song %d", i), "ArtistA", "Love Album")
	}
	songs, err := QuerySongs(db, SongQueryOptions{SearchTerm: "love", Limit: 200})
	if err != nil {
		t.Fatalf("QuerySongs: %v", err)
	}
	if len(songs) != 5 {
		t.Errorf("got %d songs, want 5", len(songs))
	}
}

func TestSearchPerfBaseline(t *testing.T) {
	if os.Getenv("AUDIOMUSE_PERF_TEST") == "" {
		t.Skip("set AUDIOMUSE_PERF_TEST=1 to run the 1M-song search perf test")
	}
	const n = 1_000_000
	d := buildLargeDB(t, n)
	defer d.Close()

	query := "love"

	timeit := func(label string, fn func() (int, error)) time.Duration {
		start := time.Now()
		cnt, err := fn()
		el := time.Since(start)
		if err != nil {
			t.Errorf("%-46s ERROR: %v", label, err)
			return el
		}
		t.Logf("%-46s %8.3fs  (result=%d)", label, el.Seconds(), cnt)
		return el
	}

	var total time.Duration
	total += timeit("counts: CountArtists (FTS)", func() (int, error) { return CountArtists(d, query, false) })
	total += timeit("counts: CountAlbums (FTS)", func() (int, error) { return CountAlbums(d, query) })
	total += timeit("counts: CountSongs (FTS)", func() (int, error) { return CountSongs(d, query) })
	total += timeit("results: QueryArtists (FTS, limit 20)", func() (int, error) {
		a, err := QueryArtists(d, ArtistQueryOptions{SearchTerm: query, IncludeCounts: true, Limit: 20})
		return len(a), err
	})
	total += timeit("results: QuerySongs (FTS, limit 200)", func() (int, error) {
		s, err := QuerySongs(d, SongQueryOptions{SearchTerm: query, IncludeGenre: true, Limit: 200, OrderBy: "s.artist, s.title"})
		return len(s), err
	})
	t.Logf("==> TOTAL song-only search work for query %q: %.3fs", query, total.Seconds())
	if total > 2*time.Second {
		t.Errorf("song-only search exceeded 2s target: %.3fs", total.Seconds())
	}
}
