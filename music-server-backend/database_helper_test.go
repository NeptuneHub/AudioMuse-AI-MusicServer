package main

import (
	"database/sql"
	"testing"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}

	create := `
	CREATE TABLE songs (
		id TEXT PRIMARY KEY,
		title TEXT,
		artist TEXT,
		album TEXT,
		album_artist TEXT,
		album_path TEXT,
		genre TEXT,
		path TEXT,
		duration INTEGER,
		play_count INTEGER,
		last_played TEXT,
		cancelled INTEGER DEFAULT 0
	);
	`
	if _, err := db.Exec(create); err != nil {
		db.Close()
		t.Fatalf("failed to create songs table: %v", err)
	}
	return db
}

func TestCountAlbumsIncludesAlbumArtist(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create albums with album_artist present in one
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "s1", "t1", "ArtistA", "AlbumX", "AlbumArtist", "")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "s2", "t2", "ArtistB", "AlbumY", "", "")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "s3", "t3", "ArtistC", "AlbumZ", "AlbumArtist", "somepath")

	// Query albums using QueryAlbums
	albums, err := QueryAlbums(db, AlbumQueryOptions{SearchTerm: "AlbumArtist", GroupByPath: true, IncludeArtist: true})
	if err != nil {
		t.Fatalf("QueryAlbums failed: %v", err)
	}

	count, err := CountAlbums(db, "AlbumArtist")
	if err != nil {
		t.Fatalf("CountAlbums failed: %v", err)
	}

	if len(albums) != count {
		t.Fatalf("expected CountAlbums (%d) to match QueryAlbums length (%d)", count, len(albums))
	}
}

func TestMultiWordArtistSearchAND(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, _ = db.Exec(`INSERT INTO songs (id, title, artist) VALUES (?, ?, ?)`, "a1", "x", "The Beatles")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist) VALUES (?, ?, ?)`, "a2", "y", "Beatles")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist) VALUES (?, ?, ?)`, "a3", "z", "The Rolling Stones")

	// Search for both words 'The Beatles' should only match 'The Beatles', not 'Beatles' alone
	artists, err := QueryArtists(db, ArtistQueryOptions{SearchTerm: "The Beatles"})
	if err != nil {
		t.Fatalf("QueryArtists failed: %v", err)
	}

	if len(artists) != 1 || artists[0].Name != "The Beatles" {
		t.Fatalf("expected only 'The Beatles' match for 'The Beatles', got: %v", artists)
	}

	// Search for single word 'Beatles' should match both 'The Beatles' and 'Beatles'
	artists2, err := QueryArtists(db, ArtistQueryOptions{SearchTerm: "Beatles"})
	if err != nil {
		t.Fatalf("QueryArtists failed: %v", err)
	}

	if len(artists2) < 2 {
		t.Fatalf("expected at least 2 matches for 'Beatles', got %d", len(artists2))
	}
}
