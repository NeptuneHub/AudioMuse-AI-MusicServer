package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	_ "github.com/mattn/go-sqlite3"
	"github.com/gin-gonic/gin"
	"strings"
)

func TestUnknownArtist_SearchAndArtistDirectory_ReturnsAlbums(t *testing.T) {
	// Setup in-memory DB
	testDB := setupFullTestDB(t)
	defer testDB.Close()
	prev := db
	db = testDB
	defer func() { db = prev }()
	if err := migrateDB(); err != nil { t.Fatalf("migrateDB failed: %v", err) }

	// Insert songs where artist is literally 'Unknown Artist'
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path, path, duration) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, "u1", "UA Track 1", "Unknown Artist", "Unknown Album", "", "p1", "/tmp/u1.mp3", 120)
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path, path, duration) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, "u2", "UA Track 2", "Unknown Artist", "Royalty Free", "", "p1", "/tmp/u2.mp3", 90)
	// Add one song with a real album_artist on the same album to exercise album_artist preference
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path, path, duration) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, "u3", "VA Track", "Various Artist", "Royalty Free", "Kevin MacLeod", "p1", "/tmp/u3.mp3", 150)

	// Sanity: CountAlbums should include albums for Unknown Artist
	count, err := CountAlbums(db, "Unknown Artist")
	if err != nil { t.Fatalf("CountAlbums failed: %v", err) }
	if count == 0 { t.Fatalf("expected CountAlbums > 0 for 'Unknown Artist', got 0") }

	// --- Call search handler and assert albums returned ---
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	cCtx, _ := gin.CreateTestContext(w)
	r := httptest.NewRequest("GET", "/?query=Unknown+Artist&albumCount=10&f=json", nil)
	cCtx.Request = r
	cCtx.Set("user", User{ID: 1, Username: "tester"})

	subsonicSearch2(cCtx)
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse json: %v body=%s", err, w.Body.String())
	}
	searchResult := resp["subsonic-response"].(map[string]interface{})["searchResult2"].(map[string]interface{})
	// Expect album list present
	if _, ok := searchResult["album"]; !ok {
		t.Fatalf("expected album list in search response for 'Unknown Artist', got body=%s", w.Body.String())
	}

	// Verify QueryAlbums by artist returns albums (artist may be in artist or album_artist fields)
	albums, err := QueryAlbums(db, AlbumQueryOptions{Artist: "Unknown Artist", GroupByPath: true, IncludeCounts: true, Limit: 50})
	if err != nil { t.Fatalf("QueryAlbums failed: %v", err) }
	if len(albums) == 0 {
		t.Fatalf("expected QueryAlbums to return albums for 'Unknown Artist' (got none)")
	}
	// Ensure expected album names are present
	foundUnknown := false
	foundRoyalty := false
	for _, a := range albums {
		if strings.EqualFold(a.Name, "Unknown Album") {
			foundUnknown = true
		}
		if strings.EqualFold(a.Name, "Royalty Free") {
			foundRoyalty = true
		}
	}
	if !foundUnknown && !foundRoyalty {
		t.Fatalf("expected to find either 'Unknown Album' or 'Royalty Free' in QueryAlbums result, got: %+v", albums)
	}
}

func TestSearchAlbums_ListFromSongCandidates_Compilation(t *testing.T) {
	// setup DB and package-level db var so handler uses test DB
	testDB := setupFullTestDB(t)
	defer testDB.Close()
	prev := db
	db = testDB
	defer func() { db = prev }()
	if err := migrateDB(); err != nil { t.Fatalf("migrateDB failed: %v", err) }

	// Insert compilation songs (same album_path, different artists, no album_artist)
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path, path, duration, play_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "c1", "Track One", "CompArtistA", "Compilation VolX", "", "compx/p1", "/tmp/c1.mp3", 0, 0)
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path, path, duration, play_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "c2", "Track Two", "CompArtistB", "Compilation VolX", "", "compx/p1", "/tmp/c2.mp3", 0, 0)

	// sanity: CountAlbums should include the album when searching for CompArtistA
	count, err := CountAlbums(db, "CompArtistA")
	if err != nil { t.Fatalf("CountAlbums failed: %v", err) }
	if count == 0 { t.Fatalf("expected CountAlbums > 0 for CompArtistA, got 0") }

	// Call handler and ensure album appears
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	cCtx, _ := gin.CreateTestContext(w)
	r := httptest.NewRequest("GET", "/?query=CompArtistA&albumCount=10&f=json", nil)
	cCtx.Request = r
	cCtx.Set("user", User{ID: 1, Username: "tester"})

	subsonicSearch2(cCtx)
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON response: %v; body: %s", err, w.Body.String())
	}
	searchResult := resp["subsonic-response"].(map[string]interface{})["searchResult2"].(map[string]interface{})
	var albumsRes []interface{}
	if a, ok := searchResult["album"]; ok {
		switch v := a.(type) {
		case []interface{}:
			albumsRes = v
		case map[string]interface{}:
			albumsRes = []interface{}{v}
		}
	}
	found := false
	for _, a := range albumsRes {
		aMap := a.(map[string]interface{})
		if strings.EqualFold(aMap["name"].(string), "Compilation VolX") {
			found = true
			break
		}
	}
	if !found { t.Fatalf("expected 'Compilation VolX' in search albums, body: %s", w.Body.String()) }
}
