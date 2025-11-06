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
        song_id TEXT NOT NULL,
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

	// Ensure songs table has 'cancelled' column for soft-delete functionality
	if err := ensureColumnExists(db, "songs", "cancelled", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		log.Printf("migrateDB: ensureColumnExists cancelled: %v", err)
	}

	// Migrate song IDs from INTEGER to TEXT (UUID in base62)
	// This is a complex migration that needs to be done carefully
	if err := migrateSongIDsToUUID(db); err != nil {
		log.Printf("migrateDB: migrateSongIDsToUUID: %v", err)
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

	// Ensure songs table has 'duration' column (in seconds)
	if err := ensureColumnExists(db, "songs", "duration", "INTEGER DEFAULT 0"); err != nil {
		log.Printf("migrateDB: ensureColumnExists duration: %v", err)
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
		song_id TEXT NOT NULL,
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

	// Create radio_stations table for Radio feature
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS radio_stations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		seed_songs TEXT NOT NULL,
		temperature REAL NOT NULL DEFAULT 1.0,
		subtract_distance REAL NOT NULL DEFAULT 0.3,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`)
	if err != nil {
		log.Printf("migrateDB: failed to create radio_stations table: %v", err)
	}

	// Create index for radio_stations user lookup
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_radio_stations_user ON radio_stations (user_id);`)
	if err != nil {
		log.Printf("migrateDB: failed to create radio_stations index: %v", err)
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

// migrateSongIDsToUUID migrates the songs table ID column from INTEGER to TEXT (UUID base62)
// This is idempotent and safe to run multiple times
func migrateSongIDsToUUID(db *sql.DB) error {
	// Check if migration has already been done by checking the type of the id column
	var columnType string
	err := db.QueryRow(`
		SELECT type FROM pragma_table_info('songs') WHERE name = 'id'
	`).Scan(&columnType)

	if err != nil {
		return fmt.Errorf("failed to check songs.id column type: %v", err)
	}

	// If already TEXT, migration is complete
	if strings.ToUpper(columnType) == "TEXT" {
		log.Println("migrateSongIDsToUUID: songs.id is already TEXT, migration complete")
		return nil
	}

	log.Println("migrateSongIDsToUUID: Starting migration of songs.id from INTEGER to TEXT (UUID base62)")

	// Begin transaction
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// Create new songs table with TEXT id
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS songs_new (
			id TEXT PRIMARY KEY NOT NULL,
			title TEXT,
			artist TEXT,
			album TEXT,
			path TEXT UNIQUE NOT NULL,
			play_count INTEGER NOT NULL DEFAULT 0,
			last_played TEXT,
			date_added TEXT,
			date_updated TEXT,
			starred INTEGER NOT NULL DEFAULT 0,
			genre TEXT DEFAULT '',
			album_path TEXT DEFAULT '',
			duration INTEGER DEFAULT 0,
			replaygain_track_gain REAL,
			replaygain_track_peak REAL,
			replaygain_album_gain REAL,
			replaygain_album_peak REAL,
			cancelled INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create songs_new table: %v", err)
	}

	// Migrate data from old table to new table with UUID generation
	rows, err := tx.Query("SELECT id, title, artist, album, path, play_count, last_played, date_added, date_updated, starred, genre, album_path, duration, replaygain_track_gain, replaygain_track_peak, replaygain_album_gain, replaygain_album_peak FROM songs")
	if err != nil {
		return fmt.Errorf("failed to query existing songs: %v", err)
	}
	defer rows.Close()

	insertStmt, err := tx.Prepare(`
		INSERT INTO songs_new (id, title, artist, album, path, play_count, last_played, date_added, date_updated, starred, genre, album_path, duration, replaygain_track_gain, replaygain_track_peak, replaygain_album_gain, replaygain_album_peak, cancelled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare insert statement: %v", err)
	}
	defer insertStmt.Close()

	// Map old integer IDs to new UUID IDs for foreign key updates
	idMapping := make(map[int]string)
	migratedCount := 0

	for rows.Next() {
		var oldID int
		var title, artist, album, path sql.NullString
		var playCount, starred, duration sql.NullInt64
		var lastPlayed, dateAdded, dateUpdated, genre, albumPath sql.NullString
		var replayGainTrackGain, replayGainTrackPeak, replayGainAlbumGain, replayGainAlbumPeak sql.NullFloat64

		err := rows.Scan(&oldID, &title, &artist, &album, &path, &playCount, &lastPlayed, &dateAdded, &dateUpdated, &starred, &genre, &albumPath, &duration, &replayGainTrackGain, &replayGainTrackPeak, &replayGainAlbumGain, &replayGainAlbumPeak)
		if err != nil {
			log.Printf("Error scanning song row: %v", err)
			continue
		}

		// Generate new UUID for this song
		newID := GenerateBase62UUID()
		idMapping[oldID] = newID

		_, err = insertStmt.Exec(
			newID,
			title.String,
			artist.String,
			album.String,
			path.String,
			playCount.Int64,
			lastPlayed.String,
			dateAdded.String,
			dateUpdated.String,
			starred.Int64,
			genre.String,
			albumPath.String,
			duration.Int64,
			nullFloat64ToInterface(replayGainTrackGain),
			nullFloat64ToInterface(replayGainTrackPeak),
			nullFloat64ToInterface(replayGainAlbumGain),
			nullFloat64ToInterface(replayGainAlbumPeak),
		)
		if err != nil {
			log.Printf("Error inserting song with new UUID: %v", err)
			continue
		}
		migratedCount++
	}

	log.Printf("migrateSongIDsToUUID: Migrated %d songs with new UUIDs", migratedCount)

	// Update foreign key references in other tables
	// Update playlist_songs
	if err := updatePlaylistSongsForeignKeys(tx, idMapping); err != nil {
		return fmt.Errorf("failed to update playlist_songs: %v", err)
	}

	// Update starred_songs
	if err := updateStarredSongsForeignKeys(tx, idMapping); err != nil {
		return fmt.Errorf("failed to update starred_songs: %v", err)
	}

	// Update play_history
	if err := updatePlayHistoryForeignKeys(tx, idMapping); err != nil {
		return fmt.Errorf("failed to update play_history: %v", err)
	}

	// Drop old songs table and rename new one
	_, err = tx.Exec("DROP TABLE songs")
	if err != nil {
		return fmt.Errorf("failed to drop old songs table: %v", err)
	}

	_, err = tx.Exec("ALTER TABLE songs_new RENAME TO songs")
	if err != nil {
		return fmt.Errorf("failed to rename songs_new to songs: %v", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	log.Println("migrateSongIDsToUUID: Successfully completed migration")
	return nil
}

func nullFloat64ToInterface(nf sql.NullFloat64) interface{} {
	if nf.Valid {
		return nf.Float64
	}
	return nil
}

func updatePlaylistSongsForeignKeys(tx *sql.Tx, idMapping map[int]string) error {
	// Create new playlist_songs table with TEXT song_id
	_, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS playlist_songs_new (
			playlist_id INTEGER NOT NULL,
			song_id TEXT NOT NULL,
			position INTEGER NOT NULL,
			FOREIGN KEY(playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
			FOREIGN KEY(song_id) REFERENCES songs(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	// Migrate data
	rows, err := tx.Query("SELECT playlist_id, song_id, position FROM playlist_songs")
	if err != nil {
		return err
	}
	defer rows.Close()

	insertStmt, err := tx.Prepare("INSERT INTO playlist_songs_new (playlist_id, song_id, position) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer insertStmt.Close()

	for rows.Next() {
		var playlistID, position int
		var oldSongID int
		if err := rows.Scan(&playlistID, &oldSongID, &position); err != nil {
			continue
		}

		if newSongID, exists := idMapping[oldSongID]; exists {
			insertStmt.Exec(playlistID, newSongID, position)
		}
	}

	_, err = tx.Exec("DROP TABLE playlist_songs")
	if err != nil {
		return err
	}

	_, err = tx.Exec("ALTER TABLE playlist_songs_new RENAME TO playlist_songs")
	if err != nil {
		return err
	}

	_, err = tx.Exec("CREATE INDEX IF NOT EXISTS idx_playlist_songs_order ON playlist_songs (playlist_id, position)")
	return err
}

func updateStarredSongsForeignKeys(tx *sql.Tx, idMapping map[int]string) error {
	// Create new starred_songs table with TEXT song_id
	_, err := tx.Exec(`
		CREATE TABLE IF NOT EXISTS starred_songs_new (
			user_id INTEGER NOT NULL,
			song_id TEXT NOT NULL,
			starred_at TEXT NOT NULL,
			PRIMARY KEY (user_id, song_id),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(song_id) REFERENCES songs(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	// Migrate data
	rows, err := tx.Query("SELECT user_id, song_id, starred_at FROM starred_songs")
	if err != nil {
		return err
	}
	defer rows.Close()

	insertStmt, err := tx.Prepare("INSERT INTO starred_songs_new (user_id, song_id, starred_at) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer insertStmt.Close()

	for rows.Next() {
		var userID int
		var oldSongID int
		var starredAt string
		if err := rows.Scan(&userID, &oldSongID, &starredAt); err != nil {
			continue
		}

		if newSongID, exists := idMapping[oldSongID]; exists {
			insertStmt.Exec(userID, newSongID, starredAt)
		}
	}

	_, err = tx.Exec("DROP TABLE starred_songs")
	if err != nil {
		return err
	}

	_, err = tx.Exec("ALTER TABLE starred_songs_new RENAME TO starred_songs")
	return err
}

func updatePlayHistoryForeignKeys(tx *sql.Tx, idMapping map[int]string) error {
	// Check if play_history table exists
	var tableName string
	err := tx.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='play_history'").Scan(&tableName)
	if err != nil {
		// Table doesn't exist, skip
		return nil
	}

	// Create new play_history table with TEXT song_id
	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS play_history_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			song_id TEXT NOT NULL,
			played_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(song_id) REFERENCES songs(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return err
	}

	// Migrate data
	rows, err := tx.Query("SELECT id, user_id, song_id, played_at FROM play_history")
	if err != nil {
		return err
	}
	defer rows.Close()

	insertStmt, err := tx.Prepare("INSERT INTO play_history_new (id, user_id, song_id, played_at) VALUES (?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer insertStmt.Close()

	for rows.Next() {
		var id, userID, oldSongID int
		var playedAt string
		if err := rows.Scan(&id, &userID, &oldSongID, &playedAt); err != nil {
			continue
		}

		if newSongID, exists := idMapping[oldSongID]; exists {
			insertStmt.Exec(id, userID, newSongID, playedAt)
		}
	}

	_, err = tx.Exec("DROP TABLE play_history")
	if err != nil {
		return err
	}

	_, err = tx.Exec("ALTER TABLE play_history_new RENAME TO play_history")
	if err != nil {
		return err
	}

	_, err = tx.Exec("CREATE INDEX IF NOT EXISTS idx_play_history_user_played ON play_history (user_id, played_at DESC)")
	return err
}
