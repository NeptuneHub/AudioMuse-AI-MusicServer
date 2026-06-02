package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// timeHandler runs a gin handler with the given raw query string and returns the
// wall-clock duration and HTTP status.
func timeHandler(h gin.HandlerFunc, rawQuery string) (time.Duration, int) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/rest/x?"+rawQuery, nil)
	c.Set("user", User{ID: 1, Username: "test"})
	start := time.Now()
	h(c)
	return time.Since(start), w.Code
}

func TestScaleBenchmark(t *testing.T) {
	if os.Getenv("AUDIOMUSE_PERF_TEST") == "" {
		t.Skip("set AUDIOMUSE_PERF_TEST=1 to run the scale benchmark")
	}
	n := 90000
	if v := os.Getenv("PERF_N"); v != "" {
		fmtSscan(v, &n)
	}

	testDB := buildLargeDB(t, n)
	defer testDB.Close()
	ensureLibraryDerivedTables(testDB)
	t.Logf("building derived tables for %d songs...", n)
	if err := RebuildLibraryIndex(testDB); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	// also create starred_songs etc. so handlers that join them work
	testDB.Exec(`CREATE TABLE IF NOT EXISTS starred_songs (user_id INTEGER, song_id TEXT, starred_at TEXT)`)
	testDB.Exec(`CREATE TABLE IF NOT EXISTS starred_albums (user_id INTEGER, album_id TEXT, starred_at TEXT)`)
	testDB.Exec(`CREATE TABLE IF NOT EXISTS transcoding_settings (user_id INTEGER, song_id TEXT, enabled INTEGER)`)

	old := db
	db = testDB
	defer func() { db = old }()

	cases := []struct {
		name  string
		h     gin.HandlerFunc
		query string
	}{
		{"search2 FULL (a=20,al=20,s=50) love", subsonicSearch2, "query=love&artistCount=20&albumCount=20&songCount=50&f=json"},
		{"search3 FULL (a=20,al=20,s=50) love", subsonicSearch3, "query=love&artistCount=20&albumCount=20&songCount=50&f=json"},
		{"search3 SONG-ONLY love (frontend)", subsonicSearch3, "query=love&artistCount=0&albumCount=0&songCount=200&f=json"},
		{"search3 SONG-ONLY love 2-char broad", subsonicSearch3, "query=lo&artistCount=0&albumCount=0&songCount=200&f=json"},
		{"getAlbumList2 alphabeticalByArtist", subsonicGetAlbumList2, "type=alphabeticalByArtist&size=50&f=json"},
		{"getAlbumList2 newest", subsonicGetAlbumList2, "type=newest&size=50&f=json"},
		{"getAlbumList2 random", subsonicGetAlbumList2, "type=random&size=50&f=json"},
		{"getArtists", subsonicGetArtists, "f=json"},
		{"getIndexes", subsonicGetIndexes, "f=json"},
		{"getMusicCounts", getMusicCounts, "f=json"},
		{"getRandomSongs", subsonicGetRandomSongs, "size=50&f=json"},
		{"getSongsByGenre Rock", subsonicGetSongsByGenre, "genre=Rock&count=50&f=json"},
	}

	for _, tc := range cases {
		// warm + measure best of 2
		var best time.Duration = time.Hour
		var status int
		for i := 0; i < 2; i++ {
			d, s := timeHandler(tc.h, tc.query)
			status = s
			if d < best {
				best = d
			}
		}
		flag := ""
		if best > 1*time.Second {
			flag = "   <<<<< SLOW"
		}
		t.Logf("%-42s %8.1f ms  (http %d)%s", tc.name, float64(best.Microseconds())/1000.0, status, flag)
	}
}

func fmtSscan(s string, out *int) {
	v := 0
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return
		}
		v = v*10 + int(ch-'0')
	}
	if v > 0 {
		*out = v
	}
}
