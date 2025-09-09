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

// scanSingleLibrary is a robust wrapper to scan one library and manage global status.
func scanSingleLibrary(pathId int) {
	// This function now handles the entire lifecycle of a single scan.
	defer func() {
		// This deferred block ensures the scanning status is always reset.
		log.Println("Single library scan finished, updating final status.")
		db.Exec("UPDATE scan_status SET is_scanning = 0, last_update_time = ? WHERE id = 1", time.Now().Format(time.RFC3339))
	}()

	var path string
	err := db.QueryRow("SELECT path FROM library_paths WHERE id = ?", pathId).Scan(&path)
	if err != nil {
		log.Printf("Cannot start single scan, path not found for id %d: %v", pathId, err)
		return // The defer will handle resetting the scan status.
	}

	log.Printf("Background scan started for single path: %s", path)
	isScanCancelled.Store(false)

	songsAdded := processPath(path)
	updateSongCountForPath(path, pathId) // Update count for this path.

	// Update the global count of songs added *in this specific run*.
	db.Exec("UPDATE scan_status SET songs_added = ? WHERE id = 1", songsAdded)

	if isScanCancelled.Load() {
		log.Printf("Scan was cancelled for path %s. Songs added before stop: %d.", path, songsAdded)
	} else {
		log.Printf("Scan finished for path %s. Total songs added: %d.", path, songsAdded)
	}
}

// scanAllLibraries is a background worker function that scans all configured library paths.
func scanAllLibraries() {
	defer func() {
		// This deferred block ensures the scanning status is always reset.
		log.Println("Finished scanning all libraries, updating final status.")
		db.Exec("UPDATE scan_status SET is_scanning = 0, last_update_time = ? WHERE id = 1", time.Now().Format(time.RFC3339))
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

	var totalSongsAdded int64
	for _, p := range pathsToScan {
		if isScanCancelled.Load() {
			log.Println("Scan All was cancelled, stopping further processing.")
			break
		}
		songsAdded := processPath(p.Path)
		totalSongsAdded += songsAdded
		updateSongCountForPath(p.Path, p.ID) // Update count for each path as it completes.
	}

	log.Printf("Total new songs added in this run across all paths: %d.", totalSongsAdded)
	db.Exec("UPDATE scan_status SET songs_added = ? WHERE id = 1", totalSongsAdded)
}

// processPath walks a single directory path and adds songs to the database.
// It returns the number of new songs added from that path.
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
				res, err := db.Exec("INSERT OR IGNORE INTO songs (title, artist, album, path, date_added, date_updated) VALUES (?, ?, ?, ?, ?, ?)",
					meta.Title(), meta.Artist(), meta.Album(), path, currentTime, currentTime)
				if err != nil {
					log.Printf("Error inserting song from %s into DB: %v", path, err)
					return nil
				}

				rowsAffected, _ := res.RowsAffected()
				if rowsAffected > 0 {
					songsAdded++
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

// updateSongCountForPath calculates and updates the total number of songs for a specific library path.
func updateSongCountForPath(path string, pathId int) {
	var count int
	likePath := filepath.Join(path, "%")
	err := db.QueryRow("SELECT COUNT(*) FROM songs WHERE path LIKE ?", likePath).Scan(&count)
	if err != nil {
		log.Printf("Error counting songs for path %s: %v", path, err)
		return
	}

	_, err = db.Exec("UPDATE library_paths SET song_count = ? WHERE id = ?", count, pathId)
	if err != nil {
		log.Printf("Error updating song count for path ID %d: %v", pathId, err)
	} else {
		log.Printf("Updated song count for path '%s' to %d", path, count)
	}
}

// browseFiles is a UI helper not part of the Subsonic API standard.
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

// cancelAdminScan is a UI helper to signal a running scan to stop.
func cancelAdminScan(c *gin.Context) {
	log.Println("Received request to cancel library scan.")
	isScanCancelled.Store(true)
	// The running scan functions will now handle updating the DB status upon cancellation.
	c.JSON(http.StatusOK, gin.H{"message": "Scan cancellation signal sent."})
}

