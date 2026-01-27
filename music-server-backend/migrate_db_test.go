package main

import (
	"database/sql"
	"testing"
	_ "github.com/mattn/go-sqlite3"
)

func TestMigrateDB_IdempotentAndCreatesExpectedTables(t *testing.T) {
	conn, err := sql.Open("sqlite3", ":memory:")
	if err != nil { t.Fatalf("open db: %v", err) }
	defer conn.Close()

	// set package-level db var for migrateDB
	prev := db
	db = conn
	defer func() { db = prev }()

	// Run migration twice
	if err := migrateDB(); err != nil { t.Fatalf("first migrate failed: %v", err) }
	if err := migrateDB(); err != nil { t.Fatalf("second migrate failed: %v", err) }

	// Check users table columns include api_key and password_plain
	rows, err := db.Query(`PRAGMA table_info(users)`) 
	if err != nil { t.Fatalf("pragma failed: %v", err) }
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil { continue }
		cols[name] = true
	}
	if !cols["api_key"] || !cols["password_plain"] {
		t.Fatalf("expected users table to have api_key and password_plain columns, got cols=%v", cols)
	}

	// Check starred_songs exists and has starred_at
	rows2, err := db.Query(`PRAGMA table_info(starred_songs)`)
	if err != nil { t.Fatalf("pragma starred_songs failed: %v", err) }
	defer rows2.Close()
	cols2 := map[string]bool{}
	for rows2.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt interface{}
		if err := rows2.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil { continue }
		cols2[name] = true
	}
	if !cols2["starred_at"] {
		t.Fatalf("expected starred_songs to have starred_at column, got cols=%v", cols2)
	}
}