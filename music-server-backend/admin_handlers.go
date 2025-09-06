// Suggested path: music-server-backend/admin_handlers.go
package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/gin-gonic/gin"
)


// --- Admin Handlers (JSON API) ---

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

				res, err := db.Exec("INSERT OR IGNORE INTO songs (title, artist, album, path) VALUES (?, ?, ?, ?)",
					meta.Title(), meta.Artist(), meta.Album(), path)
				if err != nil {
					log.Printf("Error inserting song from %s into DB: %v", path, err)
					return nil
				}

				rowsAffected, _ := res.RowsAffected()
				if rowsAffected > 0 {
					songsAdded++
					// Update DB with new count
					db.Exec("UPDATE scan_status SET songs_added = ? WHERE id = 1", songsAdded)
				}
			}
		}
		return nil
	})

	// 3. Set scan status to 'finished'
	db.Exec("UPDATE scan_status SET is_scanning = 0, last_update_time = ? WHERE id = 1", time.Now().Format(time.RFC3339))
	log.Printf("Background scan finished. Added %d new songs.", songsAdded)
}

func startAdminScan(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A valid path is required."})
		return
	}

	var isScanning bool
	err := db.QueryRow("SELECT is_scanning FROM scan_status WHERE id = 1").Scan(&isScanning)
	if err != nil && err != sql.ErrNoRows {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error checking scan status."})
		return
	}

	if !isScanning {
		log.Println("Starting new library scan in background from Admin Panel.")
		go scanLibrary(req.Path)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Scan initiated."})
}

func getAdminScanStatus(c *gin.Context) {
	var isScanning bool
	var songsAdded int64
	err := db.QueryRow("SELECT is_scanning, songs_added FROM scan_status WHERE id = 1").Scan(&isScanning, &songsAdded)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not retrieve scan status."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"scanning": isScanning, "count": songsAdded})
}

func browseFiles(c *gin.Context) {
	// Security: In a real multi-user or internet-facing app, this should be heavily restricted.
	// For this self-hosted app, we allow browsing from the filesystem root.
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

func getUsers(c *gin.Context) {
	rows, err := db.Query("SELECT id, username, is_admin FROM users ORDER BY username")
	if err != nil {
		log.Printf("Error querying users: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error while fetching users."})
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		// We explicitly scan into fields, ensuring password hashes are never sent.
		if err := rows.Scan(&user.ID, &user.Username, &user.IsAdmin); err != nil {
			log.Printf("Error scanning user row: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process user data."})
			return
		}
		users = append(users, user)
	}
	if err = rows.Err(); err != nil {
		log.Printf("Error iterating user rows: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error while processing users."})
		return
	}
	c.JSON(http.StatusOK, users)
}

func createUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if user.Password == "" || user.Username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username and password are required"})
		return
	}

	hashedPassword, err := hashPassword(user.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	_, err = db.Exec("INSERT INTO users (username, password_hash, password_plain, is_admin) VALUES (?, ?, ?, ?)", user.Username, hashedPassword, user.Password, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create user, username might already exist"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User created successfully."})
}

func updateUserPassword(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input, new password required"})
		return
	}

	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	res, err := db.Exec("UPDATE users SET password_hash = ?, password_plain = ? WHERE id = ?", hashedPassword, req.Password, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully."})
}

func deleteUser(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Prevent user from deleting themselves
	currentUserID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not identify current user from token"})
		return
	}
	if currentUserID.(int) == userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "You cannot delete your own account."})
		return
	}

	// Prevent deleting the last admin
	var adminCount int
	err = db.QueryRow("SELECT COUNT(*) FROM users WHERE is_admin = 1").Scan(&adminCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error checking admin count"})
		return
	}

	if adminCount <= 1 {
		var isTargetAdmin bool
		err := db.QueryRow("SELECT is_admin FROM users WHERE id = ?", userID).Scan(&isTargetAdmin)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error checking user role"})
			return
		}
		if isTargetAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete the last remaining admin user."})
			return
		}
	}

	res, err := db.Exec("DELETE FROM users WHERE id = ?", userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully."})
}
