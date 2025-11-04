// Suggested path: music-server-backend/admin_handlers.go
package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/gin-gonic/gin"
)

func scanSingleLibrary(pathId int) {
	defer func() {
		db.Exec("UPDATE scan_status SET is_scanning = 0, last_update_time = ? WHERE id = 1", time.Now().Format(time.RFC3339))
		log.Println("Single library scan process finished, final status updated.")
	}()

	var path string
	err := db.QueryRow("SELECT path FROM library_paths WHERE id = ?", pathId).Scan(&path)
	if err != nil {
		log.Printf("Cannot start single scan, path not found for id %d: %v", pathId, err)
		return
	}

	log.Printf("Background scan started for single path: %s", path)
	isScanCancelled.Store(false)

	// Initialize the scan counter for single path scan
	db.Exec("UPDATE scan_status SET songs_added = 0, last_update_time = ? WHERE id = 1", time.Now().Format(time.RFC3339))

	scannedPaths := make(map[string]bool)
	songsAdded := processPathWithTracking(path, &scannedPaths)

	// Remove songs that are in this library path but weren't found during scan
	if !isScanCancelled.Load() {
		removeMissingSongsFromPath(path, scannedPaths)
	}

	updateSongCountForPath(path, pathId)
	db.Exec("UPDATE library_paths SET last_scan_ended = ? WHERE id = ?", time.Now().Format(time.RFC3339), pathId)

	db.Exec("UPDATE scan_status SET songs_added = ? WHERE id = 1", songsAdded)

	if isScanCancelled.Load() {
		log.Printf("Scan was cancelled for path %s. Songs added before stop: %d.", path, songsAdded)
	} else {
		log.Printf("Scan finished for path %s. Total songs added: %d.", path, songsAdded)
	}
}

func scanAllLibraries() {
	defer func() {
		db.Exec("UPDATE scan_status SET is_scanning = 0, last_update_time = ? WHERE id = 1", time.Now().Format(time.RFC3339))
		log.Println("Finished scanning all libraries, final status updated.")
	}()

	log.Println("Background scan started for ALL library paths.")
	isScanCancelled.Store(false)

	rows, err := db.Query("SELECT id, path FROM library_paths")
	if err != nil {
		log.Printf("Error fetching library paths for scanning: %v", err)
		return
	}
	defer rows.Close()

	var pathsToScan []LibraryPath
	for rows.Next() {
		var p LibraryPath
		if err := rows.Scan(&p.ID, &p.Path); err != nil {
			log.Printf("Error scanning library path row for scan job: %v", err)
			continue
		}
		pathsToScan = append(pathsToScan, p)
	}

	// Initialize the scan counter for "Scan All"
	db.Exec("UPDATE scan_status SET songs_added = 0, last_update_time = ? WHERE id = 1", time.Now().Format(time.RFC3339))

	var totalSongsAdded int64
	for _, p := range pathsToScan {
		if isScanCancelled.Load() {
			log.Println("Scan All was cancelled, stopping further processing.")
			break
		}
		scannedPaths := make(map[string]bool)
		processPathWithRunningTotalAndTracking(p.Path, &totalSongsAdded, &scannedPaths)

		// Remove songs that are in this library path but weren't found during scan
		if !isScanCancelled.Load() {
			removeMissingSongsFromPath(p.Path, scannedPaths)
		}

		updateSongCountForPath(p.Path, p.ID)
		db.Exec("UPDATE library_paths SET last_scan_ended = ? WHERE id = ?", time.Now().Format(time.RFC3339), p.ID)
	}

	// After scanning all paths, remove orphaned songs (songs that don't belong to any current library path)
	if !isScanCancelled.Load() {
		removeOrphanedSongs(pathsToScan)
	}

	log.Printf("Total new songs added in this run across all paths: %d.", totalSongsAdded)
	db.Exec("UPDATE scan_status SET songs_added = ? WHERE id = 1", totalSongsAdded)
}

func processPath(scanPath string) int64 {
	var songsAdded int64
	log.Printf("Processing path: %s", scanPath)

	walkErr := filepath.WalkDir(scanPath, func(path string, d os.DirEntry, err error) error {
		if isScanCancelled.Load() {
			return errors.New("scan cancelled by user")
		}
		if err != nil {
			log.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}

		if !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			supportedExts := map[string]bool{".mp3": true, ".flac": true, ".m4a": true, ".ogg": true}

			if supportedExts[ext] {
				file, err := os.Open(path)
				if err != nil {
					log.Printf("Error opening file %s: %v", path, err)
					return nil
				}
				defer file.Close()

				meta, err := tag.ReadFrom(file)
				if err != nil {
					log.Printf("Error reading tags from %s: %v", path, err)
					return nil
				}

				currentTime := time.Now().Format(time.RFC3339)
				genre := meta.Genre()
				if genre == "" {
					genre = "Unknown"
				}
				// Get duration using ffprobe
				duration := getDuration(path)

				// Use INSERT ... ON CONFLICT to update existing songs or insert new ones
				// This ensures date_added is set for old songs missing it, and date_updated is always current
				res, err := db.Exec(`INSERT INTO songs (title, artist, album, path, genre, duration, date_added, date_updated) 
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT(path) DO UPDATE SET 
						title=excluded.title, 
						artist=excluded.artist, 
						album=excluded.album, 
						genre=excluded.genre,
						duration=excluded.duration,
						date_added=COALESCE(songs.date_added, excluded.date_added),
						date_updated=excluded.date_updated`,
					meta.Title(), meta.Artist(), meta.Album(), path, genre, duration, currentTime, currentTime)
				if err != nil {
					log.Printf("Error upserting song from %s into DB: %v", path, err)
					return nil
				}

				rowsAffected, _ := res.RowsAffected()
				if rowsAffected > 0 {
					songsAdded++
					// Update scan status in real-time every time a new song is added
					db.Exec("UPDATE scan_status SET songs_added = ?, last_update_time = ? WHERE id = 1",
						songsAdded, time.Now().Format(time.RFC3339))
				}
			}
		}
		return nil
	})

	if walkErr != nil {
		log.Printf("Stopped walking path %s due to error: %v", scanPath, walkErr)
	}
	return songsAdded
}

func processPathWithRunningTotal(scanPath string, totalSongsAdded *int64) {
	log.Printf("Processing path: %s", scanPath)

	walkErr := filepath.WalkDir(scanPath, func(path string, d os.DirEntry, err error) error {
		if isScanCancelled.Load() {
			return errors.New("scan cancelled by user")
		}
		if err != nil {
			log.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}

		if !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			supportedExts := map[string]bool{".mp3": true, ".flac": true, ".m4a": true, ".ogg": true}

			if supportedExts[ext] {
				file, err := os.Open(path)
				if err != nil {
					log.Printf("Error opening file %s: %v", path, err)
					return nil
				}
				defer file.Close()

				meta, err := tag.ReadFrom(file)
				if err != nil {
					log.Printf("Error reading tags from %s: %v", path, err)
					return nil
				}

				currentTime := time.Now().Format(time.RFC3339)
				genre := meta.Genre()
				if genre == "" {
					genre = "Unknown"
				}
				// Get duration using ffprobe
				duration := getDuration(path)

				// Use UPSERT to update existing songs or insert new ones
				res, err := db.Exec(`INSERT INTO songs (title, artist, album, path, genre, duration, date_added, date_updated) 
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT(path) DO UPDATE SET 
						title=excluded.title, 
						artist=excluded.artist, 
						album=excluded.album, 
						genre=excluded.genre,
						duration=excluded.duration,
						date_added=COALESCE(songs.date_added, excluded.date_added),
						date_updated=excluded.date_updated`,
					meta.Title(), meta.Artist(), meta.Album(), path, genre, duration, currentTime, currentTime)
				if err != nil {
					log.Printf("Error upserting song from %s into DB: %v", path, err)
					return nil
				}

				rowsAffected, _ := res.RowsAffected()
				if rowsAffected > 0 {
					*totalSongsAdded++
					// Update scan status in real-time with the cumulative total
					db.Exec("UPDATE scan status SET songs_added = ?, last_update_time = ? WHERE id = 1",
						*totalSongsAdded, time.Now().Format(time.RFC3339))
				}
			}
		}
		return nil
	})

	if walkErr != nil {
		log.Printf("Stopped walking path %s due to error: %v", scanPath, walkErr)
	}
}

func processPathWithTracking(scanPath string, scannedPaths *map[string]bool) int64 {
	var songsAdded int64
	log.Printf("Processing path with tracking: %s", scanPath)

	walkErr := filepath.WalkDir(scanPath, func(path string, d os.DirEntry, err error) error {
		if isScanCancelled.Load() {
			return errors.New("scan cancelled by user")
		}
		if err != nil {
			log.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}

		if !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			supportedExts := map[string]bool{".mp3": true, ".flac": true, ".m4a": true, ".ogg": true}

			if supportedExts[ext] {
				// Track this file path
				(*scannedPaths)[path] = true

				file, err := os.Open(path)
				if err != nil {
					log.Printf("Error opening file %s: %v", path, err)
					return nil
				}
				defer file.Close()

				meta, err := tag.ReadFrom(file)
				if err != nil {
					log.Printf("Error reading tags from %s: %v", path, err)
					return nil
				}

				currentTime := time.Now().Format(time.RFC3339)
				genre := meta.Genre()
				if genre == "" {
					genre = "Unknown"
				}

				title := meta.Title()
				artist := meta.Artist()
				album := meta.Album()

				// Get duration using ffprobe
				duration := getDuration(path)

				// DEBUG: Log the first few songs being inserted
				if songsAdded < 3 {
					log.Printf("DEBUG [processPathWithTracking]: Inserting song #%d: title='%s', duration=%ds, date_added='%s', date_updated='%s'",
						songsAdded+1, title, duration, currentTime, currentTime)
				}

				// Use UPSERT to update existing songs or insert new ones
				res, err := db.Exec(`INSERT INTO songs (title, artist, album, path, genre, duration, date_added, date_updated) 
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT(path) DO UPDATE SET 
						title=excluded.title, 
						artist=excluded.artist, 
						album=excluded.album, 
						genre=excluded.genre,
						duration=excluded.duration,
						date_added=COALESCE(songs.date_added, excluded.date_added),
						date_updated=excluded.date_updated`,
					title, artist, album, path, genre, duration, currentTime, currentTime)
				if err != nil {
					log.Printf("Error upserting song from %s into DB: %v", path, err)
					return nil
				}

				rowsAffected, _ := res.RowsAffected()
				if rowsAffected > 0 {
					songsAdded++
					db.Exec("UPDATE scan_status SET songs_added = ?, last_update_time = ? WHERE id = 1",
						songsAdded, time.Now().Format(time.RFC3339))

					// DEBUG: Verify the song was actually inserted with date_added
					if songsAdded <= 3 {
						var checkDateAdded string
						err := db.QueryRow("SELECT date_added FROM songs WHERE path = ?", path).Scan(&checkDateAdded)
						if err != nil {
							log.Printf("DEBUG [processPathWithTracking]: ERROR querying date_added after insert: %v", err)
						} else {
							log.Printf("DEBUG [processPathWithTracking]: Verified song #%d in DB: date_added='%s'", songsAdded, checkDateAdded)
						}
					}
				}
			}
		}
		return nil
	})

	if walkErr != nil {
		log.Printf("Stopped walking path %s due to error: %v", scanPath, walkErr)
	}
	return songsAdded
}

func processPathWithRunningTotalAndTracking(scanPath string, totalSongsAdded *int64, scannedPaths *map[string]bool) {
	log.Printf("Processing path with running total and tracking: %s", scanPath)

	walkErr := filepath.WalkDir(scanPath, func(path string, d os.DirEntry, err error) error {
		if isScanCancelled.Load() {
			return errors.New("scan cancelled by user")
		}
		if err != nil {
			log.Printf("Error accessing path %q: %v\n", path, err)
			return nil
		}

		if !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			supportedExts := map[string]bool{".mp3": true, ".flac": true, ".m4a": true, ".ogg": true}

			if supportedExts[ext] {
				// Track this file path
				(*scannedPaths)[path] = true

				file, err := os.Open(path)
				if err != nil {
					log.Printf("Error opening file %s: %v", path, err)
					return nil
				}
				defer file.Close()

				meta, err := tag.ReadFrom(file)
				if err != nil {
					log.Printf("Error reading tags from %s: %v", path, err)
					return nil
				}

				currentTime := time.Now().Format(time.RFC3339)
				genre := meta.Genre()
				if genre == "" {
					genre = "Unknown"
				}

				title := meta.Title()
				artist := meta.Artist()
				album := meta.Album()

				// Get duration using ffprobe
				duration := getDuration(path)

				// DEBUG: Log the first few songs being inserted
				if *totalSongsAdded < 3 {
					log.Printf("DEBUG [processPathWithRunningTotalAndTracking]: Inserting song #%d: title='%s', duration=%ds, date_added='%s'",
						*totalSongsAdded+1, title, duration, currentTime)
				}

				// Use UPSERT to update existing songs or insert new ones
				res, err := db.Exec(`INSERT INTO songs (title, artist, album, path, genre, duration, date_added, date_updated) 
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)
					ON CONFLICT(path) DO UPDATE SET 
						title=excluded.title, 
						artist=excluded.artist, 
						album=excluded.album, 
						genre=excluded.genre,
						duration=excluded.duration,
						date_added=COALESCE(songs.date_added, excluded.date_added),
						date_updated=excluded.date_updated`,
					title, artist, album, path, genre, duration, currentTime, currentTime)
				if err != nil {
					log.Printf("Error upserting song from %s into DB: %v", path, err)
					return nil
				}

				rowsAffected, _ := res.RowsAffected()
				if rowsAffected > 0 {
					(*totalSongsAdded)++
					db.Exec("UPDATE scan_status SET songs_added = ?, last_update_time = ? WHERE id = 1",
						*totalSongsAdded, time.Now().Format(time.RFC3339))

					// DEBUG: Verify first few songs were inserted with date_added
					if *totalSongsAdded <= 3 {
						var checkDateAdded string
						err := db.QueryRow("SELECT date_added FROM songs WHERE path = ?", path).Scan(&checkDateAdded)
						if err != nil {
							log.Printf("DEBUG [processPathWithRunningTotalAndTracking]: ERROR querying date_added: %v", err)
						} else {
							log.Printf("DEBUG [processPathWithRunningTotalAndTracking]: Verified song #%d in DB: date_added='%s'", *totalSongsAdded, checkDateAdded)
						}
					}
				}
			}
		}
		return nil
	})

	if walkErr != nil {
		log.Printf("Stopped walking path %s due to error: %v", scanPath, walkErr)
	}
}

func removeMissingSongsFromPath(libraryPath string, scannedPaths map[string]bool) {
	// Normalize path for comparison
	searchPath := libraryPath
	if !strings.HasSuffix(searchPath, "/") && !strings.HasSuffix(searchPath, "\\") {
		searchPath += string(filepath.Separator)
	}
	likePath := searchPath + "%"

	log.Printf("Checking for missing songs in path: %s", libraryPath)

	// Get all songs from database that belong to this library path
	rows, err := db.Query("SELECT id, path FROM songs WHERE path LIKE ?", likePath)
	if err != nil {
		log.Printf("Error querying songs for cleanup: %v", err)
		return
	}
	defer rows.Close()

	var songsToDelete []int
	for rows.Next() {
		var songID int
		var songPath string
		if err := rows.Scan(&songID, &songPath); err != nil {
			log.Printf("Error scanning song row for cleanup: %v", err)
			continue
		}

		// If this song's path wasn't in our scanned paths, it no longer exists
		if !scannedPaths[songPath] {
			songsToDelete = append(songsToDelete, songID)
			log.Printf("Song file not found, will delete: %s (ID: %d)", songPath, songID)
		}
	}

	// Delete missing songs
	if len(songsToDelete) > 0 {
		log.Printf("Removing %d missing songs from database", len(songsToDelete))

		for _, songID := range songsToDelete {
			// Remove from playlists
			_, err := db.Exec("DELETE FROM playlist_songs WHERE song_id = ?", songID)
			if err != nil {
				log.Printf("Error removing song %d from playlists: %v", songID, err)
			}

			// Remove from starred songs
			_, err = db.Exec("DELETE FROM starred_songs WHERE song_id = ?", songID)
			if err != nil {
				log.Printf("Error removing song %d from starred: %v", songID, err)
			}

			// Remove the song itself
			_, err = db.Exec("DELETE FROM songs WHERE id = ?", songID)
			if err != nil {
				log.Printf("Error deleting song %d: %v", songID, err)
			}
		}

		log.Printf("Successfully removed %d missing songs", len(songsToDelete))
	} else {
		log.Printf("No missing songs found in path: %s", libraryPath)
	}
}

func removeOrphanedSongs(activePaths []LibraryPath) {
	log.Println("Checking for orphaned songs (songs not belonging to any current library path)...")

	// Get all songs from database
	rows, err := db.Query("SELECT id, path FROM songs")
	if err != nil {
		log.Printf("Error querying all songs for orphan cleanup: %v", err)
		return
	}
	defer rows.Close()

	var orphanedSongs []int
	for rows.Next() {
		var songID int
		var songPath string
		if err := rows.Scan(&songID, &songPath); err != nil {
			log.Printf("Error scanning song row for orphan cleanup: %v", err)
			continue
		}

		// Check if this song belongs to any active library path
		belongsToActiveLibrary := false
		for _, libraryPath := range activePaths {
			// Normalize paths for comparison (handle both / and \ separators)
			normalizedLibPath := filepath.Clean(libraryPath.Path)
			normalizedSongPath := filepath.Clean(songPath)

			// Check if song path starts with library path
			if strings.HasPrefix(normalizedSongPath, normalizedLibPath) {
				belongsToActiveLibrary = true
				break
			}
		}

		// If song doesn't belong to any active library, mark for deletion
		if !belongsToActiveLibrary {
			orphanedSongs = append(orphanedSongs, songID)
			log.Printf("Orphaned song found (no matching library path): %s (ID: %d)", songPath, songID)
		}
	}

	// Delete orphaned songs
	if len(orphanedSongs) > 0 {
		log.Printf("Removing %d orphaned songs from database", len(orphanedSongs))

		for _, songID := range orphanedSongs {
			// Remove from playlists
			_, err := db.Exec("DELETE FROM playlist_songs WHERE song_id = ?", songID)
			if err != nil {
				log.Printf("Error removing orphaned song %d from playlists: %v", songID, err)
			}

			// Remove from starred songs
			_, err = db.Exec("DELETE FROM starred_songs WHERE song_id = ?", songID)
			if err != nil {
				log.Printf("Error removing orphaned song %d from starred: %v", songID, err)
			}

			// Remove the song itself
			_, err = db.Exec("DELETE FROM songs WHERE id = ?", songID)
			if err != nil {
				log.Printf("Error deleting orphaned song %d: %v", songID, err)
			}
		}

		log.Printf("Successfully removed %d orphaned songs", len(orphanedSongs))
	} else {
		log.Println("No orphaned songs found")
	}
}

func updateSongCountForPath(path string, pathId int) {
	var count int
	// Ensure path ends with / for proper pattern matching
	searchPath := path
	if !strings.HasSuffix(searchPath, "/") {
		searchPath += "/"
	}
	likePath := searchPath + "%"

	log.Printf("DEBUG: Counting songs for path '%s' using pattern '%s'", path, likePath)
	err := db.QueryRow("SELECT COUNT(*) FROM songs WHERE path LIKE ?", likePath).Scan(&count)
	if err != nil {
		log.Printf("Error counting songs for path %s: %v", path, err)
		return
	}

	_, err = db.Exec("UPDATE library_paths SET song_count = ? WHERE id = ?", count, pathId)
	if err != nil {
		log.Printf("Error updating song count for path ID %d: %v", pathId, err)
	} else {
		log.Printf("Updated song count for path '%s' (ID: %d) to %d using pattern '%s'", path, pathId, count, likePath)
	}
}

func browseFiles(c *gin.Context) {
	path := c.DefaultQuery("path", "/")
	if path == "" {
		path = "/"
	}

	if len(path) == 2 && path[1] == ':' {
		path += "\\"
	}

	dirEntries, err := os.ReadDir(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read directory: " + err.Error()})
		return
	}

	var items []FileItem
	for _, entry := range dirEntries {
		if entry.IsDir() {
			items = append(items, FileItem{Name: entry.Name(), Type: "dir"})
		}
	}
	c.JSON(http.StatusOK, gin.H{"path": path, "items": items})
}

func cancelAdminScan(c *gin.Context) {
	log.Println("Received request to cancel library scan.")
	isScanCancelled.Store(true)
	c.JSON(http.StatusOK, gin.H{"message": "Scan cancellation signal sent."})
}

func rescanAllLibraries(c *gin.Context) {
	// Check if scan is already running
	var isScanning bool
	err := db.QueryRow("SELECT is_scanning FROM scan_status WHERE id = 1").Scan(&isScanning)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error checking scan status"})
		return
	}

	if isScanning {
		c.JSON(http.StatusConflict, gin.H{"error": "A scan is already running"})
		return
	}

	// Clear the database first
	log.Println("Starting full library rescan - clearing existing data...")

	// Delete all songs and related data
	_, err = db.Exec("DELETE FROM playlist_songs")
	if err != nil {
		log.Printf("Warning: Could not clear playlist_songs: %v", err)
	}

	_, err = db.Exec("DELETE FROM starred_songs")
	if err != nil {
		log.Printf("Warning: Could not clear starred_songs: %v", err)
	}

	_, err = db.Exec("DELETE FROM songs")
	if err != nil {
		log.Printf("Error clearing songs table: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear songs database"})
		return
	}

	// Reset library path song counts
	_, err = db.Exec("UPDATE library_paths SET song_count = 0, last_scan_ended = NULL")
	if err != nil {
		log.Printf("Warning: Could not reset library_paths: %v", err)
	}

	log.Println("Database cleared. Starting fresh scan...")

	// Mark scan as started
	_, err = db.Exec("UPDATE scan_status SET is_scanning = 1, songs_added = 0, last_update_time = ? WHERE id = 1",
		time.Now().Format(time.RFC3339))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update scan status"})
		return
	}

	// Start the scan in background
	go scanAllLibraries()

	c.JSON(http.StatusOK, gin.H{"message": "Full library rescan started successfully"})
}
