package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// buildLargeDB creates a temp-file SQLite DB populated with n synthetic songs,
// mirroring the production schema + FTS5 index + triggers. It is used by the
// search performance benchmarks. Vocabulary is chosen so a search for a common
// word ("love") matches roughly 1/len(vocab) of all songs, exercising the
// realistic "broad result set" case that makes naive LIKE scans slow.
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
		artist := fmt.Sprintf("Artist %d", i%50000)        // ~50k artists
		album := fmt.Sprintf("%s Album %d", w, i%200000)   // ~200k albums
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
	// Rebuild FTS from content (triggers are not installed here; we bulk-load then rebuild)
	if _, err := d.Exec(`INSERT INTO songs_fts(songs_fts) VALUES('rebuild')`); err != nil {
		tb.Fatalf("fts rebuild: %v", err)
	}
	// Apply the same secondary indexes the production migration creates.
	ensureSongSearchIndexes(d)
	return d
}

// TestSearchPerfBaseline times the CURRENT search3 query shapes (raw LIKE scans
// + per-album N+1 display-artist subqueries) against the FTS5 index equivalents,
// at 1M songs. Run with:
//
//	go test -tags fts5 -run TestSearchPerfBaseline -v -timeout 600s
func TestSearchPerfBaseline(t *testing.T) {
	if os.Getenv("AUDIOMUSE_PERF_TEST") == "" {
		t.Skip("set AUDIOMUSE_PERF_TEST=1 to run the 1M-song search perf test")
	}
	const n = 1_000_000
	d := buildLargeDB(t, n)
	defer d.Close()

	query := "love"
	words := strings.Fields(query)

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

	// ============================================================
	// POST-FIX search3 path: FTS-backed counts + indexed album work
	// ============================================================
	var total time.Duration

	total += timeit("counts: CountArtists (FTS)", func() (int, error) {
		return CountArtists(d, query, false)
	})
	total += timeit("counts: CountAlbums (FTS)", func() (int, error) {
		return CountAlbums(d, query)
	})
	total += timeit("counts: CountSongs (FTS)", func() (int, error) {
		return CountSongs(d, query)
	})

	total += timeit("results: QueryArtists (FTS, limit 20)", func() (int, error) {
		a, err := QueryArtists(d, ArtistQueryOptions{SearchTerm: query, IncludeCounts: true, Limit: 20})
		return len(a), err
	})

	// search3 album search as it stands today: LIKE candidate scan grouped by
	// album, then a per-candidate getAlbumDisplayArtist. The fix is the index on
	// (album, album_path) which this DB now has — this measures the real
	// post-fix cost of that N+1 over all matching albums.
	total += timeit("results: album-search (N+1 displayArtist, indexed)", func() (int, error) {
		var conds []string
		var args []interface{}
		for _, w := range words {
			conds = append(conds, "(album LIKE ? OR artist LIKE ? OR album_artist LIKE ?)")
			p := "%" + w + "%"
			args = append(args, p, p, p)
		}
		q := `SELECT album, MIN(NULLIF(album_path, '')) as albumPath, COALESCE(genre,'') as genre, MIN(id) as albumId, COUNT(*) as song_count
			FROM songs WHERE (` + strings.Join(conds, " AND ") + `) AND cancelled = 0
			GROUP BY CASE WHEN album_path IS NOT NULL AND album_path != '' THEN album_path || '|||' || album ELSE album END
			ORDER BY album COLLATE NOCASE`
		rows, err := d.Query(q, args...)
		if err != nil {
			return 0, err
		}
		type cand struct{ album, albumPath string }
		var cands []cand
		for rows.Next() {
			var album, albumPath, genre, albumID string
			var sc int
			_ = rows.Scan(&album, &albumPath, &genre, &albumID, &sc)
			cands = append(cands, cand{album, strings.TrimSpace(albumPath)})
		}
		rows.Close()
		for _, cd := range cands {
			_, _ = getAlbumDisplayArtist(d, cd.album, cd.albumPath)
		}
		return len(cands), nil
	})

	total += timeit("results: QuerySongs (FTS, limit 50)", func() (int, error) {
		s, err := QuerySongs(d, SongQueryOptions{SearchTerm: query, IncludeGenre: true, Limit: 50, OrderBy: "s.artist, s.album, s.title"})
		return len(s), err
	})

	t.Logf("==> TOTAL post-fix search3 work for query %q: %.3fs", query, total.Seconds())
	if total > 2*time.Second {
		t.Errorf("post-fix search exceeded 2s target: %.3fs", total.Seconds())
	}
}
