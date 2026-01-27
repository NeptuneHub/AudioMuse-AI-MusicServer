package main

import (
	"database/sql"
	"testing"
	_ "github.com/mattn/go-sqlite3"
)

func TestStarredDuplicatesDoNotDuplicateResults(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	create := `
	CREATE TABLE songs (
		id TEXT PRIMARY KEY,
		title TEXT,
		artist TEXT,
		album TEXT,
		genre TEXT,
		duration INTEGER,
		play_count INTEGER,
		last_played TEXT,
		cancelled INTEGER DEFAULT 0
	);
	CREATE TABLE starred_songs (
		user_id INTEGER,
		song_id TEXT,
		starred_at TEXT
	);
	`
	if _, err := db.Exec(create); err != nil {
		t.Fatalf("create tables: %v", err)
	}

	_, _ = db.Exec(`INSERT INTO songs (id, title) VALUES (?, ?)`, "s1", "one")
	// Insert duplicate starred entries for same user/song
	_, _ = db.Exec(`INSERT INTO starred_songs (user_id, song_id, starred_at) VALUES (?, ?, ?)`, 1, "s1", "2026-01-01T00:00:00Z")
	_, _ = db.Exec(`INSERT INTO starred_songs (user_id, song_id, starred_at) VALUES (?, ?, ?)`, 1, "s1", "2026-01-02T00:00:00Z")

	query := `
		SELECT s.id
		FROM songs s
		INNER JOIN (
			SELECT song_id, MAX(starred_at) as starred_at
			FROM starred_songs
			WHERE user_id = ?
			GROUP BY song_id
		) ss ON s.id = ss.song_id
		WHERE s.cancelled = 0
	`

	rows, err := db.Query(query, 1)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if count != 1 {
		t.Fatalf("expected 1 starred result after deduplication, got %d", count)
	}
}
