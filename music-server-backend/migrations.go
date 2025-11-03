package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// migrateDB performs lightweight, idempotent schema and configuration migrations
// to bring older databases up-to-date without destroying existing data.
func migrateDB() error {
	if db == nil {
		return nil
	}

	// Ensure playlists table exists (safe: CREATE IF NOT EXISTS)
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS playlists (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT NOT NULL,
        user_id INTEGER,
        FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
    );`)
	if err != nil {
		log.Printf("migrateDB: failed to ensure playlists table: %v", err)
		return err
	}

	// Ensure playlist_songs table exists
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS playlist_songs (
        playlist_id INTEGER NOT NULL,
        song_id INTEGER NOT NULL,
        position INTEGER NOT NULL,
        FOREIGN KEY(playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
        FOREIGN KEY(song_id) REFERENCES songs(id) ON DELETE CASCADE
    );`)
	if err != nil {
		log.Printf("migrateDB: failed to ensure playlist_songs table: %v", err)
		return err
	}

	// Ensure index for playlist order exists
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_playlist_songs_order ON playlist_songs (playlist_id, position);`)
	if err != nil {
		log.Printf("migrateDB: failed to ensure playlist_songs index: %v", err)
		return err
	}

	// Ensure configuration table exists (initDB normally creates it, but be defensive)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS configuration (
        key TEXT PRIMARY KEY NOT NULL,
        value TEXT
    );`)
	if err != nil {
		log.Printf("migrateDB: failed to ensure configuration table: %v", err)
		return err
	}

	// Ensure audiomuse_ai_core_url key exists (insert empty value if missing)
	if _, err = db.Exec(`INSERT OR IGNORE INTO configuration (key, value) VALUES ('audiomuse_ai_core_url', '')`); err != nil {
		log.Printf("migrateDB: failed to ensure audiomuse_ai_core_url config key: %v", err)
		return err
	}

	// Ensure songs table has 'genre' column (best-effort; ALTER will fail if column exists)
	if err := ensureColumnExists(db, "songs", "genre", "TEXT DEFAULT ''"); err != nil {
		log.Printf("migrateDB: ensureColumnExists genre: %v", err)
		// non-fatal; proceed
	}

	// Ensure songs table has 'starred' column
	if err := ensureColumnExists(db, "songs", "starred", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		log.Printf("migrateDB: ensureColumnExists starred: %v", err)
	}

	// Ensure songs table has 'date_added' column
	if err := ensureColumnExists(db, "songs", "date_added", "TEXT"); err != nil {
		log.Printf("migrateDB: ensureColumnExists date_added: %v", err)
	}

	// Ensure songs table has 'date_updated' column
	if err := ensureColumnExists(db, "songs", "date_updated", "TEXT"); err != nil {
		log.Printf("migrateDB: ensureColumnExists date_updated: %v", err)
	}

	// Backfill date_added and date_updated for existing songs that don't have them
	// This is a one-time migration to set current timestamp for older songs
	// Use strftime to match RFC3339 format used in application code
	_, err = db.Exec(`UPDATE songs SET date_added = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE date_added IS NULL OR date_added = ''`)
	if err != nil {
		log.Printf("migrateDB: failed to backfill date_added: %v", err)
	}
	_, err = db.Exec(`UPDATE songs SET date_updated = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE date_updated IS NULL OR date_updated = ''`)
	if err != nil {
		log.Printf("migrateDB: failed to backfill date_updated: %v", err)
	}

	// Add ReplayGain columns
	if err := ensureColumnExists(db, "songs", "replaygain_track_gain", "REAL"); err != nil {
		log.Printf("migrateDB: ensureColumnExists replaygain_track_gain: %v", err)
	}
	if err := ensureColumnExists(db, "songs", "replaygain_track_peak", "REAL"); err != nil {
		log.Printf("migrateDB: ensureColumnExists replaygain_track_peak: %v", err)
	}
	if err := ensureColumnExists(db, "songs", "replaygain_album_gain", "REAL"); err != nil {
		log.Printf("migrateDB: ensureColumnExists replaygain_album_gain: %v", err)
	}
	if err := ensureColumnExists(db, "songs", "replaygain_album_peak", "REAL"); err != nil {
		log.Printf("migrateDB: ensureColumnExists replaygain_album_peak: %v", err)
	}

	// Create play_history table for Recently Played tracking
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS play_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		song_id INTEGER NOT NULL,
		played_at TEXT NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY(song_id) REFERENCES songs(id) ON DELETE CASCADE
	);`)
	if err != nil {
		log.Printf("migrateDB: failed to create play_history table: %v", err)
	}

	// Create index for play_history queries
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_play_history_user_played ON play_history (user_id, played_at DESC);`)
	if err != nil {
		log.Printf("migrateDB: failed to create play_history index: %v", err)
	}

	// Create transcoding_settings table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS transcoding_settings (
		user_id INTEGER PRIMARY KEY NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 0,
		format TEXT NOT NULL DEFAULT 'mp3',
		bitrate INTEGER NOT NULL DEFAULT 128,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`)
	if err != nil {
		log.Printf("migrateDB: failed to create transcoding_settings table: %v", err)
	}

	log.Println("migrateDB: completed migrations (idempotent)")
	return nil
}

// ensureColumnExists will attempt to add a column to a table if it doesn't exist.
// For SQLite we attempt to ALTER TABLE ADD COLUMN and ignore duplicate column errors.
func ensureColumnExists(db *sql.DB, table, column, definition string) error {
	// Naive approach: try ALTER TABLE; if it errors with "duplicate column name" then ignore.
	_, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", table, column, definition))
	if err != nil {
		// Check error string for SQLite duplicate column message (best-effort)
		if strings.Contains(err.Error(), "duplicate column name") || strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return err
	}
	return nil
}
