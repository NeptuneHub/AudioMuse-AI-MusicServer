// Suggested path: music-server-backend/admin_handlers.go
package main

import (
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
	// 1. Set scan status to 'scanning' and reset counters
	_, err := db.Exec("UPDATE scan_status SET is_scanning = 1, songs_added = 0, last_update_time = ? WHERE id = 1", time.Now().Format(time.RFC3339))
	if err != nil {
		log.Printf("Error starting scan in DB: %v", err)
		return
	}

	log.Printf("Background scan started for path: %s", scanPath)
	var songsAdded int64

	err = filepath.WalkDir(scanPath, func(path string, d os.DirEntry, err error) error {
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

				res, err := db.Exec("INSERT OR IGNORE INTO songs (title, artist, album, path) VALUES (?, ?, ?, ?)",
					meta.Title(), meta.Artist(), meta.Album(), path)
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
	if err != nil {
		log.Printf("Error walking directory %s: %v", scanPath, err)
	}

	// Final update and status change
	db.Exec("UPDATE scan_status SET is_scanning = 0, songs_added = ?, last_update_time = ? WHERE id = 1", songsAdded, time.Now().Format(time.RFC3339))
	log.Printf("Background scan finished. Added %d new songs.", songsAdded)
}

// browseFiles is a UI helper not part of the Subsonic API standard.
// It allows an admin to browse the server filesystem to select a library path.
func browseFiles(c *gin.Context) {
	path := c.DefaultQuery("path", "/")
	if path == "" {
		path = "/"
	}

	// On Windows, a path like "C:" needs to be "C:\" to be read.
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

