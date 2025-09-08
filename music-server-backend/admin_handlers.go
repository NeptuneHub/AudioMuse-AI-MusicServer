// admin_handlers.go
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

// scanLibrary is a background worker function that scans the library path.
func scanLibrary(scanPath string) {
	isScanCancelled.Store(false) // Reset cancellation flag for this new scan.

	// The status is now set to 'scanning' by the handler that calls this function.
	log.Printf("Background scan started for path: %s", scanPath)
	var songsAdded int64

	walkErr := filepath.WalkDir(scanPath, func(path string, d os.DirEntry, err error) error {
		// Check for cancellation signal at the start of each file/dir operation.
		if isScanCancelled.Load() {
			return errors.New("scan cancelled by user")
		}

		if err != nil {
			log.Printf("Error accessing path %q: %v\n", path, err)
			return nil // Don't stop the walk on a single file error
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
					// Update DB with new count periodically for better UI feedback
					if songsAdded%20 == 0 {
						db.Exec("UPDATE scan_status SET songs_added = ? WHERE id = 1", songsAdded)
					}
				}
			}
		}
		return nil
	})

	if walkErr != nil && walkErr.Error() == "scan cancelled by user" {
		log.Println("Scan was cancelled by user.")
	} else if walkErr != nil {
		log.Printf("Error walking directory %s: %v", scanPath, walkErr)
	}

	// Final update and status change, but only if the scan was not cancelled.
	// If it was cancelled, the cancel handler has already updated the status.
	if !isScanCancelled.Load() {
		db.Exec("UPDATE scan_status SET is_scanning = 0, songs_added = ?, last_update_time = ? WHERE id = 1", songsAdded, time.Now().Format(time.RFC3339))
		log.Printf("Scan finished. Total songs added in this run: %d.", songsAdded)
	} else {
		log.Printf("Scan was cancelled. Total songs added before cancellation: %d.", songsAdded)
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

// cancelAdminScan is a UI helper to signal a running scan to stop and update the DB.
func cancelAdminScan(c *gin.Context) {
	log.Println("Received request to cancel library scan.")
	isScanCancelled.Store(true)

	// Immediately update the database to reflect the cancellation.
	// This prevents a stuck state if the server restarts.
	_, err := db.Exec("UPDATE scan_status SET is_scanning = 0 WHERE id = 1")
	if err != nil {
		log.Printf("Error updating scan status to cancelled in DB: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to update scan status in database."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scan cancellation signal sent and status updated."})
}

