package main

import (
	"testing"
	_ "github.com/mattn/go-sqlite3"
)

func TestGetAlbumDisplayArtist_AlbumArtistPreferred(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Insert songs with same album and album_artist set
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "a1", "T1", "ArtistA", "BestAlbum", "AlbumArtist", "p1")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "a2", "T2", "ArtistB", "BestAlbum", "AlbumArtist", "p1")

	disp, err := getAlbumDisplayArtist(db, "BestAlbum", "p1")
	if err != nil { t.Fatalf("getAlbumDisplayArtist error: %v", err) }
	if disp != "AlbumArtist" {
		t.Fatalf("expected AlbumArtist, got %v", disp)
	}
}

func TestGetAlbumDisplayArtist_CompilationSortedUnique(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Compilation: same album_path, different artists
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "c1", "T1", "CompArtistB", "Compilation Vol1", "", "comp/p1")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "c2", "T2", "CompArtistA", "Compilation Vol1", "", "comp/p1")

	disp, err := getAlbumDisplayArtist(db, "Compilation Vol1", "comp/p1")
	if err != nil { t.Fatalf("error: %v", err) }
	// Expect artists sorted: A then B, joined by "; "
	if disp != "CompArtistA; CompArtistB" {
		t.Fatalf("expected 'CompArtistA; CompArtistB', got %v", disp)
	}
}

func TestGetAlbumDisplayArtist_UnknownFallback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Artists are unknown/empty
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "u1", "T1", "", "Mystery", "", "p2")
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_artist, album_path) VALUES (?, ?, ?, ?, ?, ?)`, "u2", "T2", "", "Mystery", "", "p2")

	disp, err := getAlbumDisplayArtist(db, "Mystery", "p2")
	if err != nil { t.Fatalf("error: %v", err) }
	if disp != "Unknown Artist" {
		t.Fatalf("expected 'Unknown Artist', got %v", disp)
	}
}

func TestGetAlbumDisplayArtist_AlbumPathNullOrEmpty(t *testing.T) {
	// Ensure function matches when album_path is NULL or empty string
	db := setupTestDB(t)
	defer db.Close()

	// Insert song with NULL album_path
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album) VALUES (?, ?, ?, ?)`, "n1", "T1", "NArtist", "SoloAlbum")
	// Insert song with empty album_path
	_, _ = db.Exec(`INSERT INTO songs (id, title, artist, album, album_path) VALUES (?, ?, ?, ?, ?)`, "n2", "T2", "NArtist", "SoloAlbum", "")

	d1, err := getAlbumDisplayArtist(db, "SoloAlbum", "")
	if err != nil { t.Fatalf("error: %v", err) }
	if d1 != "NArtist" {
		t.Fatalf("expected 'NArtist' for empty path, got %v", d1)
	}

	d2, err := getAlbumDisplayArtist(db, "SoloAlbum", "")
	if err != nil { t.Fatalf("error: %v", err) }
	if d2 != "NArtist" {
		t.Fatalf("expected 'NArtist' for null/empty path case, got %v", d2)
	}
}