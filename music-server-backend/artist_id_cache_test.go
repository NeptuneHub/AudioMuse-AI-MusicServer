package main

import "testing"

func TestResolveArtistIDToName(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	invalidateArtistIDCache()

	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album_artist) VALUES (?,?,?,?)`, "s1", "t1", "ArtistA", "")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album_artist) VALUES (?,?,?,?)`, "s2", "t2", "RealArtist", "unknown")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album_artist) VALUES (?,?,?,?)`, "s3", "t3", "Track Artist", "Album Artist")

	cases := map[string]string{
		"ArtistA":      GenerateArtistID("ArtistA"),
		"RealArtist":   GenerateArtistID("RealArtist"),
		"Album Artist": GenerateArtistID("Album Artist"),
	}
	for wantName, id := range cases {
		got, ok := resolveArtistIDToName(db, id)
		if !ok || got != wantName {
			t.Errorf("resolveArtistIDToName(%q) = %q, %v; want %q, true", id, got, ok, wantName)
		}
	}

	if _, ok := resolveArtistIDToName(db, "deadbeefdeadbeefdeadbeefdeadbeef"); ok {
		t.Errorf("expected unknown ID to resolve to ok=false")
	}
	if _, ok := resolveArtistIDToName(db, ""); ok {
		t.Errorf("expected empty ID to resolve to ok=false")
	}

	invalidateArtistIDCache()
}
