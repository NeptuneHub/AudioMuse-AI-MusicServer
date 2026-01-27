package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	_ "github.com/mattn/go-sqlite3"
	"github.com/gin-gonic/gin"
)

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
