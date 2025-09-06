// Suggested path: music-server-backend/admin_handlers.go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
	"github.com/gin-gonic/gin"
)


// --- Admin Handlers (JSON API) ---

func scanLibrary(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid path provided"})
		return
	}

	if req.Path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path cannot be empty"})
		return
	}

	log.Printf("Starting library scan for path: %s", req.Path)

	var songsAdded, filesScanned, errorsEncountered int

	err := filepath.WalkDir(req.Path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			log.Printf("Error accessing path %q: %v\n", path, err)
			errorsEncountered++
			return nil
		}

		if !d.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			supportedExts := map[string]bool{".mp3": true, ".flac": true, ".m4a": true, ".ogg": true}

			if supportedExts[ext] {
				filesScanned++
				log.Printf("-> Found audio file: %s", path)

				file, err := os.Open(path)
				if err != nil {
					log.Printf("Error opening file %s: %v", path, err)
					errorsEncountered++
					return nil
				}
				defer file.Close()

				meta, err := tag.ReadFrom(file)
				if err != nil {
					log.Printf("Error reading tags from %s: %v", path, err)
					errorsEncountered++
					return nil
				}

				res, err := db.Exec("INSERT OR IGNORE INTO songs (title, artist, album, path) VALUES (?, ?, ?, ?)",
					meta.Title(), meta.Artist(), meta.Album(), path)
				if err != nil {
					log.Printf("Error inserting song from %s into DB: %v", path, err)
					errorsEncountered++
					return nil
				}

				rowsAffected, _ := res.RowsAffected()
				if rowsAffected > 0 {
					songsAdded++
					log.Printf("   Added to DB: %s - %s", meta.Artist(), meta.Title())
				}
			}
		}
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An unexpected error occurred during the scan."})
		return
	}

	responseMessage := fmt.Sprintf("Scan complete. Scanned %d audio files, added %d new songs. Encountered %d errors.", filesScanned, songsAdded, errorsEncountered)
	log.Println(responseMessage)
	c.JSON(http.StatusOK, gin.H{"message": responseMessage})
}

func browseFiles(c *gin.Context) {
	browseRoot := "/"
	userPath := c.Query("path")

	absPath, err := filepath.Abs(filepath.Join(browseRoot, userPath))
	if err != nil || !filepath.HasPrefix(absPath, browseRoot) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or forbidden path"})
		return
	}

	dirEntries, err := os.ReadDir(absPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not read directory"})
		return
	}

	var items []FileItem
	for _, entry := range dirEntries {
		if entry.IsDir() {
			items = append(items, FileItem{Name: entry.Name(), Type: "dir"})
		}
	}

	c.JSON(http.StatusOK, gin.H{"path": absPath, "items": items})
}

func getUsers(c *gin.Context) {
	rows, err := db.Query("SELECT id, username, is_admin FROM users ORDER BY username")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query users"})
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.IsAdmin); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan user row"})
			return
		}
		users = append(users, user)
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

	_, err = db.Exec("INSERT INTO users (username, password_hash, is_admin) VALUES (?, ?, ?)", user.Username, hashedPassword, user.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create user, username might already exist"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User created successfully"})
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

	res, err := db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", hashedPassword, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
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

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}
