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

	songsAdded := processPath(path)
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
		processPathWithRunningTotal(p.Path, &totalSongsAdded)
		updateSongCountForPath(p.Path, p.ID)
		db.Exec("UPDATE library_paths SET last_scan_ended = ? WHERE id = ?", time.Now().Format(time.RFC3339), p.ID)
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
				res, err := db.Exec("INSERT OR IGNORE INTO songs (title, artist, album, path, genre, date_added, date_updated) VALUES (?, ?, ?, ?, ?, ?, ?)",
					meta.Title(), meta.Artist(), meta.Album(), path, genre, currentTime, currentTime)
				if err != nil {
					log.Printf("Error inserting song from %s into DB: %v", path, err)
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
				res, err := db.Exec("INSERT OR IGNORE INTO songs (title, artist, album, path, genre, date_added, date_updated) VALUES (?, ?, ?, ?, ?, ?, ?)",
					meta.Title(), meta.Artist(), meta.Album(), path, genre, currentTime, currentTime)
				if err != nil {
					log.Printf("Error inserting song from %s into DB: %v", path, err)
					return nil
				}

				rowsAffected, _ := res.RowsAffected()
				if rowsAffected > 0 {
					*totalSongsAdded++
					// Update scan status in real-time with the cumulative total
					db.Exec("UPDATE scan_status SET songs_added = ?, last_update_time = ? WHERE id = 1",
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
