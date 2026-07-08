package main

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// albumSplitTestDB builds a library where one album's files are spread across
// several directories (the layout behind AudioMuse-AI issue #726) plus one
// conventional one-folder album, then populates the derived albums table the
// same way a real scan does.
func albumSplitTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := d.Exec(`CREATE TABLE songs (id TEXT PRIMARY KEY, title TEXT, artist TEXT, album TEXT, album_artist TEXT DEFAULT '', album_path TEXT DEFAULT '', genre TEXT DEFAULT '', path TEXT, duration INTEGER DEFAULT 0, play_count INTEGER DEFAULT 0, last_played TEXT, date_added TEXT, date_updated TEXT, replaygain_track_gain REAL, replaygain_track_peak REAL, replaygain_album_gain REAL, replaygain_album_peak REAL, track INTEGER DEFAULT 0, year INTEGER DEFAULT 0, disc_number INTEGER DEFAULT 0, size INTEGER DEFAULT 0, bitrate INTEGER DEFAULT 0, sample_rate INTEGER DEFAULT 0, channels INTEGER DEFAULT 0, bit_depth INTEGER DEFAULT 0, comment TEXT DEFAULT '', cancelled INTEGER DEFAULT 0)`); err != nil {
		t.Fatalf("create songs: %v", err)
	}
	if _, err := d.Exec(`CREATE TABLE starred_songs (song_id TEXT, user_id INTEGER)`); err != nil {
		t.Fatalf("create starred_songs: %v", err)
	}

	type row struct{ id, title, artist, album, dir, added string }
	rows := []row{
		// 'Siamese Dream' split across four directories (downloader layout).
		{"s01", "Cherub Rock", "Smashing Pumpkins", "Siamese Dream", "/music/dl1", "2026-07-01T00:00:01Z"},
		{"s02", "Quiet", "Smashing Pumpkins", "Siamese Dream", "/music/dl2", "2026-07-01T00:00:02Z"},
		{"s03", "Today", "Smashing Pumpkins", "Siamese Dream", "/music/dl3", "2026-07-01T00:00:03Z"},
		{"s04", "Disarm", "Smashing Pumpkins", "Siamese Dream", "/music/dl4", "2026-07-01T00:00:04Z"},
		// A conventional one-folder album.
		{"s05", "Track A", "Some Band", "OK Album", "/music/Some Band/OK Album", "2026-07-01T00:00:05Z"},
		{"s06", "Track B", "Some Band", "OK Album", "/music/Some Band/OK Album", "2026-07-01T00:00:06Z"},
		{"s07", "Track C", "Some Band", "OK Album", "/music/Some Band/OK Album", "2026-07-01T00:00:07Z"},
	}
	for _, r := range rows {
		if _, err := d.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path, path, date_added) VALUES (?,?,?,?,?,?,?,?)`,
			r.id, r.title, r.artist, r.album, r.artist, r.dir, r.dir+"/"+r.title+".mp3", r.added); err != nil {
			t.Fatalf("insert %s: %v", r.id, err)
		}
	}

	ensureLibraryDerivedTables(d)
	if err := RebuildLibraryIndex(d); err != nil {
		t.Fatalf("RebuildLibraryIndex: %v", err)
	}
	return d
}

// TestAlbumListSplitAcrossFoldersFullyReachable asserts the invariant broken in
// AudioMuse-AI issue #726: paging getAlbumList2 and calling getAlbum on every
// returned entry must reach EVERY song exactly once, even when an album's
// files live in different directories.
func TestAlbumListSplitAcrossFoldersFullyReachable(t *testing.T) {
	testDB := albumSplitTestDB(t)
	defer testDB.Close()
	old := db
	db = testDB
	defer func() { db = old }()

	resp := callHandler(t, subsonicGetAlbumList2, "type=newest&size=500")
	list, _ := resp["albumList2"].(map[string]interface{})
	if list == nil {
		t.Fatalf("missing albumList2: %v", resp)
	}
	entries, _ := list["album"].([]interface{})
	if len(entries) == 0 {
		t.Fatalf("no albums returned")
	}

	reached := map[string]int{}
	for _, e := range entries {
		entry, _ := e.(map[string]interface{})
		id, _ := entry["id"].(string)
		if id == "" {
			t.Fatalf("album entry without id: %v", entry)
		}
		albumResp := callHandler(t, subsonicGetAlbum, "id="+id)
		album, _ := albumResp["album"].(map[string]interface{})
		if album == nil {
			t.Fatalf("getAlbum(%s): missing album element", id)
		}
		songs, _ := album["song"].([]interface{})
		for _, s := range songs {
			song, _ := s.(map[string]interface{})
			sid, _ := song["id"].(string)
			reached[sid]++
		}
	}

	for _, want := range []string{"s01", "s02", "s03", "s04", "s05", "s06", "s07"} {
		switch reached[want] {
		case 0:
			t.Errorf("song %s is unreachable via getAlbumList2 -> getAlbum (issue #726)", want)
		case 1:
		default:
			t.Errorf("song %s served %d times (duplicate album coverage)", want, reached[want])
		}
	}
}
