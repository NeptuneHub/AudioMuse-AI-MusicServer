package main

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// callHandler drives a Subsonic handler with a raw query and returns the parsed
// "subsonic-response" object, asserting the status is ok.
func callHandler(t *testing.T, handler gin.HandlerFunc, rawQuery string) map[string]interface{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/rest/x?"+rawQuery+"&f=json", nil)
	c.Set("user", User{ID: 1, Username: "test"})
	handler(c)

	var parsed map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON (%d): %s", w.Code, w.Body.String())
	}
	resp, _ := parsed["subsonic-response"].(map[string]interface{})
	if resp == nil {
		t.Fatalf("no subsonic-response: %s", w.Body.String())
	}
	if status, _ := resp["status"].(string); status != "ok" {
		t.Fatalf("handler returned non-ok: %s", w.Body.String())
	}
	return resp
}

func TestPlaylistAndRandomSongsShape(t *testing.T) {
	// getPlaylist -> <playlist> with songs as "entry"; getRandomSongs -> "randomSongs" with "song".
	pl := SubsonicPlaylistWithSongs{ID: "1", Name: "P", Owner: "u", Public: true, SongCount: 1, Duration: 200,
		Entries: []SubsonicSong{{ID: "s1", Title: "T"}}}
	pj, _ := json.Marshal(pl)
	var pm map[string]interface{}
	_ = json.Unmarshal(pj, &pm)
	if _, ok := pm["entry"]; !ok {
		t.Errorf("playlist JSON must use 'entry' key, got: %s", pj)
	}
	if pm["owner"] != "u" || pm["public"] != true || pm["songCount"].(float64) != 1 || pm["duration"].(float64) != 200 {
		t.Errorf("playlist attributes missing/incorrect: %s", pj)
	}

	px, _ := xml.Marshal(pl)
	if !strings.Contains(string(px), "<entry ") || strings.Contains(string(px), "<song ") {
		t.Errorf("playlist XML must use <entry> children, got: %s", px)
	}

	rs := SubsonicRandomSongs{Songs: []SubsonicSong{{ID: "s1", Title: "T"}}}
	rj, _ := json.Marshal(rs)
	var rm map[string]interface{}
	_ = json.Unmarshal(rj, &rm)
	if _, ok := rm["song"]; !ok {
		t.Errorf("randomSongs JSON must use 'song' key, got: %s", rj)
	}
}

func TestNewEndpointsReturnSpecShapedResponses(t *testing.T) {
	// Empty-but-valid endpoints (no DB needed).
	if _, ok := callHandler(t, subsonicGetNowPlaying, "")["nowPlaying"]; !ok {
		t.Errorf("getNowPlaying missing nowPlaying element")
	}
	if _, ok := callHandler(t, subsonicGetBookmarks, "")["bookmarks"]; !ok {
		t.Errorf("getBookmarks missing bookmarks element")
	}
	if _, ok := callHandler(t, subsonicGetVideos, "")["videos"]; !ok {
		t.Errorf("getVideos missing videos element")
	}
	if _, ok := callHandler(t, subsonicGetArtistInfo2, "id=abc")["artistInfo2"]; !ok {
		t.Errorf("getArtistInfo2 missing artistInfo2 element")
	}
	if _, ok := callHandler(t, subsonicGetArtistInfo, "id=abc")["artistInfo"]; !ok {
		t.Errorf("getArtistInfo missing artistInfo element")
	}
}

func TestStarred2AndAlbumListElements(t *testing.T) {
	db = setupFullTestDB(t)
	defer db.Close()
	ensureLibraryDerivedTables(db)

	// getStarred2 should always return a starred2 element (empty is fine).
	if _, ok := callHandler(t, subsonicGetStarred2, "")["starred2"]; !ok {
		t.Errorf("getStarred2 missing starred2 element")
	}
	// getStarred should return a starred element.
	if _, ok := callHandler(t, subsonicGetStarred, "")["starred"]; !ok {
		t.Errorf("getStarred missing starred element")
	}
	// getAlbumList should return an albumList element.
	if _, ok := callHandler(t, subsonicGetAlbumList, "type=newest")["albumList"]; !ok {
		t.Errorf("getAlbumList missing albumList element")
	}
}
