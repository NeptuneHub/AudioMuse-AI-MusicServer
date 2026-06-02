package main

import (
	"database/sql"
	"testing"
)

func setupDerivedTestDB(t *testing.T) *sql.DB {
	db := setupTestDB(t)
	ensureLibraryDerivedTables(db)
	return db
}

func TestRebuildLibraryIndexParity(t *testing.T) {
	db := setupDerivedTestDB(t)
	defer db.Close()

	rows := [][]string{
		// id, artist, album, album_artist, album_path
		{"s1", "ArtistA", "AlbumX", "", "/m/x"},
		{"s2", "ArtistB", "AlbumX", "", "/m/x"},
		{"s3", "ArtistC", "AlbumY", "VA", "/m/y"},
		{"s4", "ArtistD", "AlbumZ", "unknown", "/m/z"},
		{"s5", "ArtistA", "", "", "/m/none"},
		{"s6", "", "AlbumW", "", "/m/w"},
	}
	for _, r := range rows {
		if _, err := db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?,?,?,?,?,?)`,
			r[0], "t"+r[0], r[1], r[2], r[3], r[4]); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	if err := RebuildLibraryIndex(db); err != nil {
		t.Fatalf("RebuildLibraryIndex: %v", err)
	}

	// Album count parity with legacy CountAlbums.
	wantAlbums, _ := CountAlbums(db, "")
	var gotAlbums int
	_ = db.QueryRow(`SELECT COUNT(*) FROM albums`).Scan(&gotAlbums)
	if gotAlbums != wantAlbums {
		t.Errorf("albums table count = %d, legacy CountAlbums = %d", gotAlbums, wantAlbums)
	}

	// Artist count parity with legacy CountArtists (raw artist).
	wantArtists, _ := CountArtists(db, "", false)
	var gotArtists int
	_ = db.QueryRow(`SELECT COUNT(*) FROM artists`).Scan(&gotArtists)
	if gotArtists != wantArtists {
		t.Errorf("artists table count = %d, legacy CountArtists = %d", gotArtists, wantArtists)
	}

	// Display-artist parity: each albums.artist must equal getAlbumDisplayArtist.
	// Collect rows fully before issuing nested queries (the default :memory:
	// connection pool serves nested queries from a separate empty database).
	type albumRow struct {
		name, albumPath, artist string
		songCount               int
	}
	var albumRows []albumRow
	arows, err := db.Query(`SELECT name, album_path, artist, song_count FROM albums`)
	if err != nil {
		t.Fatalf("query albums: %v", err)
	}
	for arows.Next() {
		var r albumRow
		if err := arows.Scan(&r.name, &r.albumPath, &r.artist, &r.songCount); err != nil {
			t.Fatalf("scan album: %v", err)
		}
		albumRows = append(albumRows, r)
	}
	arows.Close()
	if len(albumRows) != 4 {
		t.Errorf("expected 4 albums, got %d", len(albumRows))
	}
	for _, r := range albumRows {
		wantDisplay, _ := getAlbumDisplayArtist(db, r.name, r.albumPath)
		if r.artist != wantDisplay {
			t.Errorf("album %q (%s) display artist = %q, want %q", r.name, r.albumPath, r.artist, wantDisplay)
		}
		var wantCount int
		_ = db.QueryRow(`SELECT COUNT(*) FROM songs WHERE album = ? AND album_path = ? AND cancelled = 0`, r.name, r.albumPath).Scan(&wantCount)
		if r.songCount != wantCount {
			t.Errorf("album %q song_count = %d, want %d", r.name, r.songCount, wantCount)
		}
	}

	// Spot-check specific display artists.
	checkAlbumArtist(t, db, "AlbumX", "ArtistA; ArtistB")
	checkAlbumArtist(t, db, "AlbumY", "VA")
	checkAlbumArtist(t, db, "AlbumZ", "ArtistD")
	checkAlbumArtist(t, db, "AlbumW", "Unknown Artist")

	// Artist song/album counts parity with legacy QueryArtists(IncludeCounts).
	legacy, err := QueryArtists(db, ArtistQueryOptions{IncludeCounts: true})
	if err != nil {
		t.Fatalf("QueryArtists: %v", err)
	}
	for _, a := range legacy {
		var sc, ac int
		if err := db.QueryRow(`SELECT song_count, album_count FROM artists WHERE name = ?`, a.Name).Scan(&sc, &ac); err != nil {
			t.Errorf("artist %q missing from table: %v", a.Name, err)
			continue
		}
		if sc != a.SongCount || ac != a.AlbumCount {
			t.Errorf("artist %q counts = (%d songs, %d albums), legacy = (%d, %d)", a.Name, sc, ac, a.SongCount, a.AlbumCount)
		}
	}
}

func checkAlbumArtist(t *testing.T, db *sql.DB, album, want string) {
	t.Helper()
	var got string
	if err := db.QueryRow(`SELECT artist FROM albums WHERE name = ?`, album).Scan(&got); err != nil {
		t.Errorf("album %q not found: %v", album, err)
		return
	}
	if got != want {
		t.Errorf("album %q artist = %q, want %q", album, got, want)
	}
}

func TestAlbumDisplayArtistAndGenres(t *testing.T) {
	db := setupDerivedTestDB(t)
	defer db.Close()

	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path, genre) VALUES (?,?,?,?,?,?,?)`,
		"s1", "t1", "ArtistA", "AlbumX", "", "/m/x", "Rock;Pop")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path, genre) VALUES (?,?,?,?,?,?,?)`,
		"s2", "t2", "ArtistB", "AlbumX", "", "/m/x", "Jazz")
	if err := RebuildLibraryIndex(db); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	// Helper reads precomputed display artist from the albums table.
	if got := albumDisplayArtist(db, "AlbumX", "/m/x"); got != "ArtistA; ArtistB" {
		t.Errorf("albumDisplayArtist = %q, want %q", got, "ArtistA; ArtistB")
	}
	// Fallback path when the album is not in the table.
	if got := albumDisplayArtist(db, "Nope", "/no/where"); got != "Unknown Artist" {
		t.Errorf("fallback albumDisplayArtist = %q, want Unknown Artist", got)
	}

	// genres column holds the union of all contributing song genre tokens, so
	// album genre filtering matches if any song has the genre.
	for _, g := range []string{"Rock", "Pop", "Jazz"} {
		var n int
		_ = db.QueryRow(`SELECT COUNT(*) FROM albums WHERE (genres = ? OR genres LIKE ? OR genres LIKE ? OR genres LIKE ?)`,
			g, g+";%", "%;"+g+";%", "%;"+g).Scan(&n)
		if n != 1 {
			t.Errorf("genre %q matched %d albums, want 1", g, n)
		}
	}
}

func TestBuildDisplayArtistSorting(t *testing.T) {
	in := map[string]string{"b": "Beta", "a": "alpha", "c": "Gamma"}
	got := buildDisplayArtist(in)
	parts := []string{"alpha", "Beta", "Gamma"}
	want := parts[0] + "; " + parts[1] + "; " + parts[2]
	if got != want {
		t.Errorf("buildDisplayArtist = %q, want %q", got, want)
	}
	if buildDisplayArtist(map[string]string{}) != "Unknown Artist" {
		t.Errorf("empty display artist should be 'Unknown Artist'")
	}
}
