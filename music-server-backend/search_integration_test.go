package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// fileSearchTestDB creates a temp file-backed SQLite DB with the production-like
// songs schema, songs_fts (+triggers), starred_songs and the derived tables.
// A file DB (unlike :memory:) shares data across pooled connections and allows
// the handlers to run nested queries without deadlocking.
func fileSearchTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite3", t.TempDir()+"/s.db")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	stmts := []string{
		`CREATE TABLE songs (id TEXT PRIMARY KEY, title TEXT, artist TEXT, album TEXT, album_artist TEXT DEFAULT '', path TEXT, album_path TEXT DEFAULT '', genre TEXT DEFAULT '', duration INTEGER DEFAULT 0, play_count INTEGER DEFAULT 0, last_played TEXT, date_added TEXT, cancelled INTEGER NOT NULL DEFAULT 0)`,
		`CREATE VIRTUAL TABLE songs_fts USING fts5(title, artist, album, album_artist, content='songs', content_rowid='rowid', tokenize='unicode61 remove_diacritics 2')`,
		`CREATE TRIGGER songs_ai AFTER INSERT ON songs BEGIN INSERT INTO songs_fts(rowid,title,artist,album,album_artist) VALUES (new.rowid,new.title,new.artist,new.album,new.album_artist); END;`,
		`CREATE TABLE starred_songs (user_id INTEGER, song_id TEXT, starred_at TEXT)`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			t.Fatalf("schema (%s): %v", s, err)
		}
	}
	ensureLibraryDerivedTables(d)
	return d
}

// callSearch drives a search handler with the given query string and returns the
// parsed JSON "subsonic-response" body.
func callSearch(t *testing.T, handler gin.HandlerFunc, rawQuery string) map[string]interface{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/rest/x?"+rawQuery, nil)
	c.Set("user", User{ID: 1, Username: "test"})
	handler(c)

	var parsed map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON response (%d): %s", w.Code, w.Body.String())
	}
	resp, _ := parsed["subsonic-response"].(map[string]interface{})
	if resp == nil {
		t.Fatalf("no subsonic-response in %s", w.Body.String())
	}
	if status, _ := resp["status"].(string); status != "ok" {
		t.Fatalf("search failed: %s", w.Body.String())
	}
	return resp
}

func songTitles(result map[string]interface{}, key string) []string {
	var titles []string
	r, _ := result[key].(map[string]interface{})
	if r == nil {
		return titles
	}
	songs, _ := r["song"].([]interface{})
	for _, s := range songs {
		if m, ok := s.(map[string]interface{}); ok {
			if t, ok := m["title"].(string); ok {
				titles = append(titles, t)
			}
		}
	}
	return titles
}

func TestSearch2And3FindSong(t *testing.T) {
	testDB := fileSearchTestDB(t)
	defer testDB.Close()

	// Swap in the global db the handlers use.
	old := db
	db = testDB
	defer func() { db = old }()

	db.Exec(`INSERT INTO songs (id, title, artist, album, album_path, duration) VALUES (?,?,?,?,?,?)`,
		"s1", "Dublin Daisies", "'Gene Green", "Antique Phonograph Music Program", "/m/antique", 120)
	db.Exec(`INSERT INTO songs (id, title, artist, album, album_path, duration) VALUES (?,?,?,?,?,?)`,
		"s2", "Other Track", "Someone Else", "Other Album", "/m/other", 100)
	if err := RebuildLibraryIndex(db); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	// The user's exact query.
	q := "query=dub&songCount=200&songOffset=0&artistCount=0&albumCount=0&f=json"

	res2 := callSearch(t, subsonicSearch2, q)
	titles2 := songTitles(res2, "searchResult2")
	if !contains(titles2, "Dublin Daisies") {
		t.Errorf("search2 'dub' did not return 'Dublin Daisies'; got %v", titles2)
	}

	res3 := callSearch(t, subsonicSearch3, q)
	titles3 := songTitles(res3, "searchResult3")
	if !contains(titles3, "Dublin Daisies") {
		t.Errorf("search3 'dub' did not return 'Dublin Daisies'; got %v", titles3)
	}

	// A full (artist+album+song) search should also find it via the song list.
	full := "query=dublin&songCount=50&artistCount=20&albumCount=20&f=json"
	resFull := callSearch(t, subsonicSearch3, full)
	if !contains(songTitles(resFull, "searchResult3"), "Dublin Daisies") {
		t.Errorf("search3 'dublin' (full) did not return the song")
	}

	// Artist search: "gene" should return the artist '"Gene Green" (search2 + search3).
	for _, h := range []struct {
		name string
		fn   gin.HandlerFunc
		key  string
	}{{"search2", subsonicSearch2, "searchResult2"}, {"search3", subsonicSearch3, "searchResult3"}} {
		res := callSearch(t, h.fn, "query=gene&artistCount=20&albumCount=20&songCount=50&f=json")
		if !hasNamed(res, h.key, "artist", "name", "'Gene Green") {
			t.Errorf("%s 'gene' did not return artist 'Gene Green", h.name)
		}
		// Album search: "antique" should return the album.
		res2 := callSearch(t, h.fn, "query=antique&artistCount=20&albumCount=20&songCount=50&f=json")
		if !hasNamed(res2, h.key, "album", "name", "Antique Phonograph Music Program") {
			t.Errorf("%s 'antique' did not return the album", h.name)
		}
	}
}

// hasNamed checks result[key][child][].field == want.
func hasNamed(result map[string]interface{}, key, child, field, want string) bool {
	r, _ := result[key].(map[string]interface{})
	if r == nil {
		return false
	}
	items, _ := r[child].([]interface{})
	for _, it := range items {
		if m, ok := it.(map[string]interface{}); ok {
			if v, ok := m[field].(string); ok && v == want {
				return true
			}
		}
	}
	return false
}

// TestRebuildLibraryIndexRepairsSongsFTS verifies that a rescan (which calls
// RebuildLibraryIndex) repairs an empty songs_fts index, not just the derived
// tables — so the user does not need a separate restart to fix search.
func TestRebuildLibraryIndexRepairsSongsFTS(t *testing.T) {
	testDB := fileSearchTestDB(t)
	defer testDB.Close()
	old := db
	db = testDB
	defer func() { db = old }()

	// Insert songs without firing the sync trigger -> songs_fts is empty,
	// exactly like existing songs after the index was dropped/recreated.
	db.Exec(`DROP TRIGGER IF EXISTS songs_ai`)
	db.Exec(`INSERT INTO songs (id,title,artist,album,album_path,duration) VALUES (?,?,?,?,?,?)`, "s1", "Dublin Daisies", "'Gene Green", "Antique Phonograph", "/m/a", 100)
	db.Exec(`CREATE TRIGGER songs_ai AFTER INSERT ON songs BEGIN INSERT INTO songs_fts(rowid,title,artist,album,album_artist) VALUES (new.rowid,new.title,new.artist,new.album,new.album_artist); END;`)

	if !songsFTSIndexEmpty(db) {
		t.Fatalf("precondition: songs_fts should be empty")
	}

	if err := RebuildLibraryIndex(db); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	if songsFTSIndexEmpty(db) {
		t.Fatalf("songs_fts still empty after RebuildLibraryIndex")
	}
	res := callSearch(t, subsonicSearch3, "query=dub&songCount=200&artistCount=0&albumCount=0&f=json")
	if !contains(songTitles(res, "searchResult3"), "Dublin Daisies") {
		t.Errorf("search after rescan did not find the song")
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
