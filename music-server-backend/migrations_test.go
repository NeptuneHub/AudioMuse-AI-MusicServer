package main

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigrateDB_IdempotentAndCreatesExpectedTables(t *testing.T) {
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	// set package-level db var for migrateDB
	prev := db
	db = conn
	defer func() { db = prev }()

	// Create songs table first — in production initDB() does this before migrateDB() runs.
	// Without it, FTS5 triggers that reference songs cannot be created.
	if _, err = conn.Exec(`CREATE TABLE IF NOT EXISTS songs (
		id TEXT PRIMARY KEY NOT NULL,
		title TEXT, artist TEXT, album TEXT, album_artist TEXT DEFAULT '',
		path TEXT UNIQUE NOT NULL DEFAULT '', cancelled INTEGER NOT NULL DEFAULT 0
	);`); err != nil {
		t.Fatalf("failed to create songs table for test: %v", err)
	}

	// Run migration twice
	if err := migrateDB(); err != nil {
		t.Fatalf("first migrate failed: %v", err)
	}
	if err := migrateDB(); err != nil {
		t.Fatalf("second migrate failed: %v", err)
	}

	// Check users table columns include api_key and password_plain
	rows, err := db.Query(`PRAGMA table_info(users)`)
	if err != nil {
		t.Fatalf("pragma failed: %v", err)
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			continue
		}
		cols[name] = true
	}
	if !cols["api_key"] || !cols["password_plain"] {
		t.Fatalf("expected users table to have api_key and password_plain columns, got cols=%v", cols)
	}

	// Check starred_songs exists and has starred_at
	rows2, err := db.Query(`PRAGMA table_info(starred_songs)`)
	if err != nil {
		t.Fatalf("pragma starred_songs failed: %v", err)
	}
	defer rows2.Close()
	cols2 := map[string]bool{}
	for rows2.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows2.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			continue
		}
		cols2[name] = true
	}
	if !cols2["starred_at"] {
		t.Fatalf("expected starred_songs to have starred_at column, got cols=%v", cols2)
	}

	// Confirm FTS virtual table exists — FTS5 must be compiled in (via -tags fts5)
	// to match the production Dockerfile. If this fails, add -tags "fts5" to the build.
	var cnt int
	if err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='songs_fts'`).Scan(&cnt); err != nil {
		t.Fatalf("could not query sqlite_master for songs_fts: %v", err)
	}
	if cnt == 0 {
		t.Fatal("songs_fts table not created: FTS5 is not compiled in — build with -tags fts5")
	}

	// Confirm the three sync triggers were created alongside songs_fts
	for _, trig := range []string{"songs_ai", "songs_au", "songs_ad"} {
		var trigCnt int
		if err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='trigger' AND name=?`, trig).Scan(&trigCnt); err != nil {
			t.Fatalf("could not query sqlite_master for trigger %s: %v", trig, err)
		}
		if trigCnt == 0 {
			t.Fatalf("trigger %s not created after songs_fts was created", trig)
		}
	}
}
