package main

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// ftsTestDB creates a songs table with an OLD (no-tokenizer) songs_fts and
// triggers, mimicking a pre-upgrade install.
func ftsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	db.Exec(`CREATE TABLE songs (id TEXT PRIMARY KEY, title TEXT, artist TEXT, album TEXT, album_artist TEXT DEFAULT '', album_path TEXT DEFAULT '', genre TEXT DEFAULT '', path TEXT, duration INTEGER, play_count INTEGER, last_played TEXT, date_added TEXT, cancelled INTEGER DEFAULT 0)`)
	db.Exec(`CREATE VIRTUAL TABLE songs_fts USING fts5(title, artist, album, album_artist, content='songs', content_rowid='rowid')`)
	db.Exec(`CREATE TRIGGER songs_ai AFTER INSERT ON songs BEGIN INSERT INTO songs_fts(rowid,title,artist,album,album_artist) VALUES (new.rowid,new.title,new.artist,new.album,new.album_artist); END;`)
	return db
}

// recreateSongsFTSLikeMigration replicates the tokenizer-upgrade portion of migrateDB.
func recreateSongsFTSLikeMigration(t *testing.T, db *sql.DB) {
	t.Helper()
	var existing string
	_ = db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='songs_fts'`).Scan(&existing)
	if existing != "" && !strings.Contains(existing, "remove_diacritics") {
		for _, trig := range []string{"songs_ai", "songs_au", "songs_ad"} {
			db.Exec(`DROP TRIGGER IF EXISTS ` + trig)
		}
		db.Exec(`DROP TABLE IF EXISTS songs_fts`)
	}
	db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS songs_fts USING fts5(title, artist, album, album_artist, content='songs', content_rowid='rowid', tokenize='unicode61 remove_diacritics 2')`)
	db.Exec(`CREATE TRIGGER songs_ai AFTER INSERT ON songs BEGIN INSERT INTO songs_fts(rowid,title,artist,album,album_artist) VALUES (new.rowid,new.title,new.artist,new.album,new.album_artist); END;`)

	var songsCount int
	_ = db.QueryRow(`SELECT COUNT(*) FROM songs WHERE cancelled = 0`).Scan(&songsCount)
	if songsCount > 0 && songsFTSIndexEmpty(db) {
		if _, err := db.Exec(`INSERT INTO songs_fts(songs_fts) VALUES('rebuild')`); err != nil {
			t.Fatalf("rebuild: %v", err)
		}
	}
}

func TestSongsFTSRebuildAfterTokenizerMigration(t *testing.T) {
	db := ftsTestDB(t)
	defer db.Close()

	db.Exec(`INSERT INTO songs (id,title,artist,album) VALUES (?,?,?,?)`, "s1", "Dublin Daisies", "'Gene Green", "Antique Phonograph")
	db.Exec(`INSERT INTO songs (id,title,artist,album) VALUES (?,?,?,?)`, "s2", "Café del Mar", "Énigma", "Best Of")

	// Before migration the index is populated (old tokenizer).
	if songsFTSIndexEmpty(db) {
		t.Fatalf("index should be populated before migration")
	}

	recreateSongsFTSLikeMigration(t, db)

	// After migration the index must be repopulated.
	if songsFTSIndexEmpty(db) {
		t.Fatalf("index is EMPTY after migration — rebuild did not run")
	}

	// The reported bug: prefix search for "dub" must find "Dublin Daisies".
	cases := map[string]string{"dub": "Dublin Daisies", "dublin": "Dublin Daisies", "daisies": "Dublin Daisies"}
	for q, wantTitle := range cases {
		songs, err := QuerySongs(db, SongQueryOptions{SearchTerm: q, Limit: 200})
		if err != nil {
			t.Fatalf("QuerySongs(%q): %v", q, err)
		}
		found := false
		for _, s := range songs {
			if s.Title == wantTitle {
				found = true
			}
		}
		if !found {
			t.Errorf("search %q did not return %q (got %d songs)", q, wantTitle, len(songs))
		}
	}

	// Accent folding: "cafe" should match "Café del Mar".
	songs, _ := QuerySongs(db, SongQueryOptions{SearchTerm: "cafe", Limit: 200})
	found := false
	for _, s := range songs {
		if s.Title == "Café del Mar" {
			found = true
		}
	}
	if !found {
		t.Errorf("accent-insensitive search 'cafe' did not match 'Café del Mar'")
	}
}

func TestSongsFTSIndexEmptyOnFreshTable(t *testing.T) {
	db := ftsTestDB(t)
	defer db.Close()
	db.Exec(`INSERT INTO songs (id,title) VALUES (?,?)`, "s1", "Hello World")
	// Drop the populated index, recreate empty without rebuilding.
	db.Exec(`DROP TRIGGER IF EXISTS songs_ai`)
	db.Exec(`DROP TABLE songs_fts`)
	db.Exec(`CREATE VIRTUAL TABLE songs_fts USING fts5(title, artist, album, album_artist, content='songs', content_rowid='rowid', tokenize='unicode61 remove_diacritics 2')`)
	if !songsFTSIndexEmpty(db) {
		t.Fatalf("freshly recreated index should be detected as empty")
	}
}
