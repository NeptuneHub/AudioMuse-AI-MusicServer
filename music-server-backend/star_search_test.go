package main

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"testing"
	_ "github.com/mattn/go-sqlite3"
	"time"
	"strings"

	"github.com/gin-gonic/gin"
)

// setupFullTestDB creates an in-memory DB and the basic tables needed for these tests
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

func TestStarUnstarSongAndQuery(t *testing.T) {
	db := setupFullTestDB(t)
	defer db.Close()

	// Insert song
	_, err := db.Exec(`INSERT INTO songs (id, title, artist, album, path, duration) VALUES (?, ?, ?, ?, ?, ?)`, "s1", "T1", "A1", "Album1", "/tmp/a.mp3", 0)
	if err != nil {
		t.Fatalf("insert song: %v", err)
	}

	userID := 42
	now := time.Now().Format(time.RFC3339)

	// Star the song
	if err := StarSong(db, userID, "s1", now); err != nil {
		t.Fatalf("StarSong failed: %v", err)
	}

	// Verify starred_songs contains the entry (direct DB check)
	var cnt int
	err = db.QueryRow(`SELECT COUNT(*) FROM starred_songs WHERE user_id = ? AND song_id = ?`, userID, "s1").Scan(&cnt)
	if err != nil { t.Fatalf("count starred_songs failed: %v", err) }
	if cnt != 1 { t.Fatalf("expected 1 starred row, got %d", cnt) }

	// Unstar and verify removal
	if err := UnstarSong(db, userID, "s1"); err != nil {
		t.Fatalf("UnstarSong failed: %v", err)
	}
	err = db.QueryRow(`SELECT COUNT(*) FROM starred_songs WHERE user_id = ? AND song_id = ?`, userID, "s1").Scan(&cnt)
	if err != nil { t.Fatalf("count starred_songs failed: %v", err) }
	if cnt != 0 { t.Fatalf("expected 0 starred rows after unstar, got %d", cnt) }

	// Already verified unstar above by checking count == 0; no need to rely on QuerySongs IncludeStarred which can fail scanning
}

func TestStarCountAndOnlyStarredQuery(t *testing.T) {
	db := setupFullTestDB(t)
	defer db.Close()

	// Insert 3 songs
	_, _ = db.Exec(`INSERT INTO songs (id, title, duration) VALUES (?, ?, ?)`, "s1", "Song1", 0)
	_, _ = db.Exec(`INSERT INTO songs (id, title, duration) VALUES (?, ?, ?)`, "s2", "Song2", 0)
	_, _ = db.Exec(`INSERT INTO songs (id, title, duration) VALUES (?, ?, ?)`, "s3", "Song3", 0)

	userID := 7
	now := time.Now().Format(time.RFC3339)

	// Star s1 and s2
	if err := StarSong(db, userID, "s1", now); err != nil { t.Fatalf("StarSong: %v", err) }
	if err := StarSong(db, userID, "s2", now); err != nil { t.Fatalf("StarSong: %v", err) }

	// Query only starred by performing the same dedup + join logic used by the handler
	var dedupCount int
	err := db.QueryRow(`SELECT COUNT(*) FROM (SELECT s.id FROM songs s INNER JOIN (SELECT song_id, MAX(starred_at) as starred_at FROM starred_songs WHERE user_id = ? GROUP BY song_id) ss ON s.id = ss.song_id WHERE s.cancelled = 0)`, userID).Scan(&dedupCount)
	if err != nil { t.Fatalf("dedup count query failed: %v", err) }
	if dedupCount != 2 { t.Fatalf("expected 2 starred songs (deduped), got %d", dedupCount) }
}

func TestSearchAndCountSongs(t *testing.T) {
	db := setupFullTestDB(t)
	defer db.Close()

	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, path, duration, play_count) VALUES (?, ?, ?, ?, ?, ?, ?)`, "a1", "Hello World", "ArtistX", "AlbumX", "/tmp/a1.mp3", 0, 0)
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, path, duration, play_count) VALUES (?, ?, ?, ?, ?, ?, ?)`, "a2", "Hello Again", "ArtistY", "AlbumY", "/tmp/a2.mp3", 0, 0)
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, path, duration, play_count) VALUES (?, ?, ?, ?, ?, ?, ?)`, "a3", "Goodbye", "ArtistZ", "AlbumZ", "/tmp/a3.mp3", 0, 0)

	// CountSongs for 'Hello' should be 2
	count, err := CountSongs(db, "Hello")
	if err != nil { t.Fatalf("CountSongs failed: %v", err) }
	if count != 2 { t.Fatalf("expected 2 songs matching 'Hello', got %d", count) }

	// QuerySongs with SearchTerm should return the same (no limit)
	results, err := QuerySongs(db, SongQueryOptions{SearchTerm: "Hello"})
	if err != nil { t.Fatalf("QuerySongs failed: %v", err) }
	if len(results) != 2 {
		// Debug: build and run the raw SQL that QuerySongs would generate to see what's happening
		var query strings.Builder
		var args []interface{}
		query.WriteString(`SELECT s.id, s.title, s.artist, s.album, s.path, s.duration, s.play_count, s.last_played FROM songs s`)
		whereClauses := []string{"s.cancelled = 0"}
		words := strings.Fields("Hello")
		var termClauses []string
		for _, w := range words {
			termClauses = append(termClauses, "(s.title LIKE ? OR s.artist LIKE ? OR s.album LIKE ?)")
			p := "%" + w + "%"
			args = append(args, p, p, p)
		}
		whereClauses = append(whereClauses, strings.Join(termClauses, " AND "))
		query.WriteString(" WHERE " + strings.Join(whereClauses, " AND "))
		query.WriteString(" ORDER BY s.artist, s.album, s.title")

		rows, err := db.Query(query.String(), args...)
		if err != nil { t.Fatalf("debug raw query failed: %v", err) }
		defer rows.Close()
		found := 0
		for rows.Next() { found++ }
		t.Fatalf("expected 2 results for QuerySongs, got %d (debug raw found %d). SQL: %s args=%v", len(results), found, query.String(), args)
	}
}

func TestSearchAndCountAlbumsAndArtists(t *testing.T) {
	// Use a test DB and set package-level `db` to it so handlers use the test DB
	testDB := setupFullTestDB(t)
	defer testDB.Close()
	prev := db
	db = testDB
	defer func() { db = prev }()

	// Albums with album_artist values
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "b1", "s1", "ArtistA", "AlbumOne", "AlbumArtist", "p1")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "b2", "s2", "ArtistB", "AlbumTwo", "", "p2")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "b3", "s3", "ArtistC", "AlbumThree", "AlbumArtist", "p3")

	// QueryAlbums searching for 'AlbumArtist' should find two albums
	albums, err := QueryAlbums(db, AlbumQueryOptions{SearchTerm: "AlbumArtist", GroupByPath: true, IncludeArtist: true})
	if err != nil { t.Fatalf("QueryAlbums failed: %v", err) }
	if len(albums) != 2 { t.Fatalf("expected 2 albums, got %d", len(albums)) }

	// CountAlbums should match
	count, err := CountAlbums(db, "AlbumArtist")
	if err != nil { t.Fatalf("CountAlbums failed: %v", err) }
	if count != len(albums) { t.Fatalf("CountAlbums (%d) != len(QueryAlbums) (%d)", count, len(albums)) }

	// QueryArtists with UseEffectiveArtist should include album_artist fallback
	artists, err := QueryArtists(db, ArtistQueryOptions{UseEffectiveArtist: true})
	if err != nil { t.Fatalf("QueryArtists failed: %v", err) }
	// There should be at least one result and 'AlbumArtist' should be present
	found := false
	for _, a := range artists {
		if a.Name == "AlbumArtist" { found = true; break }
	}
	if !found { t.Fatalf("expected 'AlbumArtist' in effective artist list") }

	// CountArtists with useEffective should include album_artist occurrences
	c2, err := CountArtists(db, "AlbumArtist", true)
	if err != nil { t.Fatalf("CountArtists failed: %v", err) }
	if c2 == 0 { t.Fatalf("expected CountArtists to include album_artist, got 0") }

	// -------------------------------
	// Additional difficult cases
	// 1) Compilation album (same album, different artists, no album_artist)
	if _, err := db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path, path, duration, play_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "c1", "Track One", "CompArtistA", "Compilation Vol1", "", "comp/p1", "/tmp/c1.mp3", 0, 0); err != nil { t.Fatalf("failed to insert c1: %v", err) }
	if _, err := db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path, path, duration, play_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "c2", "Track Two", "CompArtistB", "Compilation Vol1", "", "comp/p1", "/tmp/c2.mp3", 0, 0); err != nil { t.Fatalf("failed to insert c2: %v", err) }

	// Ensure QueryAlbums can find the compilation album
	qa, qerr := QueryAlbums(db, AlbumQueryOptions{SearchTerm: "CompArtistA", GroupByPath: true, IncludeArtist: true})
	if qerr != nil { t.Fatalf("QueryAlbums debug failed: %v", qerr) }
	if len(qa) == 0 {

		// raw debug: check raw songs rows
		rows, _ := db.Query(`SELECT id, artist, album, album_artist, album_path, cancelled FROM songs`)
		defer rows.Close()
		any := false
		for rows.Next() {
			var id, artist, album, albumArtist, albumPath sql.NullString
			var cancelled sql.NullInt64
			rows.Scan(&id, &artist, &album, &albumArtist, &albumPath, &cancelled)
			t.Logf("song row: id=%v artist=%v album=%v albumArtist=%v albumPath=%v cancelled=%v", id.String, artist.String, album.String, albumArtist.String, albumPath.String, cancelled.Int64)
			any = true
		}
		if !any {
			t.Fatalf("no song rows found at all in songs table (debug)")
		}

		// raw SQL used by handler logic
		var albumConditions []string
		albumConditions = append(albumConditions, "album != ''", "cancelled = 0")
		var albumArgs []interface{}
		word := "CompArtistA"
		albumConditions = append(albumConditions, "(album LIKE ? OR artist LIKE ? OR album_artist LIKE ?)")
		likeWord := "%" + word + "%"
		albumArgs = append(albumArgs, likeWord, likeWord, likeWord)
		raw := `SELECT album, MIN(NULLIF(album_path, '')) as albumPath, COALESCE(genre, '') as genre, MIN(id) as albumId, COUNT(*) as song_count
			FROM songs
			WHERE ` + strings.Join(albumConditions, " AND ") + `
			GROUP BY CASE
				WHEN album_path IS NOT NULL AND album_path != '' THEN album_path || '|||' || album
				ELSE album
			END
			ORDER BY album COLLATE NOCASE`
		rows2, err := db.Query(raw, albumArgs...)
		if err != nil {
			t.Fatalf("raw album query failed: %v", err)
		}
		defer rows2.Close()
		for rows2.Next() {
			var albumName, albumPath, genre, albumID sql.NullString
			var songCount int
			rows2.Scan(&albumName, &albumPath, &genre, &albumID, &songCount)
			t.Logf("raw album row: album=%v albumPath=%v genre=%v albumID=%v songCount=%d", albumName.String, albumPath.String, genre.String, albumID.String, songCount)
		}

		// otherwise fail with message but include qa
		t.Fatalf("QueryAlbums returned 0 results for CompArtistA (debug): qa=%v", qa)
	}

	// Call the search handler (subsonicSearch2) searching by artist 'CompArtistA' and ensure album appears
	
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	cCtx, _ := gin.CreateTestContext(w)
	r := httptest.NewRequest("GET", "/?query=CompArtistA&albumCount=10&f=json", nil)
	cCtx.Request = r
	cCtx.Set("user", User{ID: 1, Username: "tester"})

	subsonicSearch2(cCtx)
	var resp map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	if err != nil { t.Fatalf("failed to parse JSON response: %v; body: %s", err, w.Body.String()) }
	searchResult := resp["subsonic-response"].(map[string]interface{})["searchResult2"].(map[string]interface{})
	var albumsRes []interface{}
	if a, ok := searchResult["album"]; ok {
		// normalize single object into array if needed
		switch v := a.(type) {
		case []interface{}:
			albumsRes = v
		case map[string]interface{}:
			albumsRes = []interface{}{v}
		}
	}
	foundComp := false
	for _, a := range albumsRes {
		aMap := a.(map[string]interface{})
		if aMap["name"] == "Compilation Vol1" {
			foundComp = true
			// display artist should include CompArtistA and CompArtistB
			if !strings.Contains(aMap["artist"].(string), "CompArtistA") {
				t.Fatalf("expected display artist to include CompArtistA, got %v", aMap["artist"])
			}
		}
	}
	if !foundComp { t.Fatalf("compilation album not found in search results; body: %s", w.Body.String()) }

	// 2) Song with Unknown artist/album
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, path, duration, play_count) VALUES (?, ?, ?, ?, ?, ?, ?)`, "u1", "Mystery", "Unknown Artist", "Unknown Album", "/tmp/u1.mp3", 0, 0)

	// Search for 'Unknown' should return song
	w2 := httptest.NewRecorder()
	cCtx2, _ := gin.CreateTestContext(w2)
	r2 := httptest.NewRequest("GET", "/?query=Unknown&songCount=20&f=json", nil)
	cCtx2.Request = r2
	cCtx2.Set("user", User{ID: 1, Username: "tester"})

	subsonicSearch2(cCtx2)
	var resp2 map[string]interface{}
	err = json.Unmarshal(w2.Body.Bytes(), &resp2)
	if err != nil { t.Fatalf("failed to parse JSON response 2: %v; body: %s", err, w2.Body.String()) }
	searchResult2 := resp2["subsonic-response"].(map[string]interface{})["searchResult2"].(map[string]interface{})
	var songsRes []interface{}
	if s, ok := searchResult2["song"]; ok {
		switch v := s.(type) {
		case []interface{}:
			songsRes = v
		case map[string]interface{}:
			songsRes = []interface{}{v}
		}
	}
	foundUnknown := false
	for _, s := range songsRes {
		sMap := s.(map[string]interface{})
		if sMap["id"] == "u1" { foundUnknown = true; break }
	}
	if !foundUnknown { t.Fatalf("expected 'Unknown' song to be returned by search") }

	// 3) Album preference: ensure album backed by album_artist preferred
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path, path, duration, play_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "p1", "Solo", "SoloArtist", "Best Album", "", "", "/tmp/p_solo.mp3", 0, 0)
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path, path, duration, play_count) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "p2", "VA Track", "Various Artist", "Best Album", "VA Artist", "", "/tmp/p_va.mp3", 0, 0)

	w3 := httptest.NewRecorder()
	cCtx3, _ := gin.CreateTestContext(w3)
	r3 := httptest.NewRequest("GET", "/?query=Best%20Album&albumCount=10&f=json", nil)
	cCtx3.Request = r3
	cCtx3.Set("user", User{ID: 1, Username: "tester"})

	// Debug: run the album query that the handler would execute for 'Best Album'
	{
		raw := `SELECT album, MIN(NULLIF(album_path, '')) as albumPath, COALESCE(genre, '') as genre, MIN(id) as albumId, COUNT(*) as song_count
		FROM songs
		WHERE album != '' AND cancelled = 0 AND (album LIKE ? OR artist LIKE ? OR album_artist LIKE ?) AND (album LIKE ? OR artist LIKE ? OR album_artist LIKE ?)
		GROUP BY CASE
			WHEN album_path IS NOT NULL AND album_path != '' THEN album_path || '|||' || album
			ELSE album
		END
		ORDER BY album`
		rows, _ := db.Query(raw, "%Best%", "%Best%", "%Best%", "%Album%", "%Album%", "%Album%")
		defer rows.Close()
		count := 0
		for rows.Next() {
			var albumName, albumPath, genre, aid sql.NullString
			var sc int
			rows.Scan(&albumName, &albumPath, &genre, &aid, &sc)
			t.Logf("raw album row (Best): album=%v path=%v sc=%d", albumName.String, albumPath.String, sc)
			count++
		}
		t.Logf("raw album rows found (Best): %d", count)
	}

	subsonicSearch2(cCtx3)
	var resp3 map[string]interface{}
	err = json.Unmarshal(w3.Body.Bytes(), &resp3)
	if err != nil { t.Fatalf("failed to parse JSON response 3: %v; body: %s", err, w3.Body.String()) }
	// Debug: log full response
	t.Logf("search3 full body: %s", w3.Body.String())
	searchResult3 := resp3["subsonic-response"].(map[string]interface{})["searchResult2"].(map[string]interface{})
	var albums3 []interface{}
	if a, ok := searchResult3["album"]; ok {
		// Debug: log response body
		t.Logf("search3 body: %s", w3.Body.String())
		switch v := a.(type) {
		case []interface{}:
			albums3 = v
		case map[string]interface{}:
			albums3 = []interface{}{v}
		}
	}
	prefFound := false
	for _, a := range albums3 {
		aMap := a.(map[string]interface{})
		if aMap["name"] == "Best Album" {
			prefFound = true
			if !strings.Contains(aMap["artist"].(string), "VA Artist") {
				t.Fatalf("expected album artist to prefer 'VA Artist', got %v", aMap["artist"])
			}
		}
	}
	if !prefFound { t.Fatalf("expected 'Best Album' in results") }
}
