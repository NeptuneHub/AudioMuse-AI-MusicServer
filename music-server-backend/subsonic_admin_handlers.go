// Suggested path: music-server-backend/subsonic_admin_handlers.go
package main

import (
	"database/sql"
	"log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func subsonicStartScan(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}
	if !user.IsAdmin {
		subsonicRespond(c, newSubsonicErrorResponse(40, "User is not authorized to start a scan."))
		return
	}

	var isScanning bool
	err := db.QueryRow("SELECT is_scanning FROM scan_status WHERE id = 1").Scan(&isScanning)
	if err != nil && err != sql.ErrNoRows {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error checking scan status."))
		return
	}

	if !isScanning {
		var libraryPath string
		if c.Request.Method == "POST" {
			var req struct {
				Path string `json:"path"`
			}
			if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" {
				subsonicRespond(c, newSubsonicErrorResponse(10, "A valid path is required for the initial scan."))
				return
			}
			libraryPath = req.Path
			_, err := db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", "library_path", libraryPath)
			if err != nil {
				log.Printf("Warning: Could not save library path to settings: %v", err)
			}
		} else {
			err := db.QueryRow("SELECT value FROM settings WHERE key = 'library_path'").Scan(&libraryPath)
			if err != nil || libraryPath == "" {
				msg := "Music library path not configured. Please perform an initial scan from the admin web panel first."
				subsonicRespond(c, newSubsonicErrorResponse(50, msg))
				return
			}
		}

		// Set status to scanning BEFORE starting the goroutine to avoid race condition.
		_, err := db.Exec("UPDATE scan_status SET is_scanning = 1, songs_added = 0, last_update_time = ? WHERE id = 1", time.Now().Format(time.RFC3339))
		if err != nil {
			log.Printf("Error starting scan in DB: %v", err)
			subsonicRespond(c, newSubsonicErrorResponse(0, "Database error starting scan."))
			return
		}

		log.Printf("Library scan triggered for path: %s", libraryPath)
		go scanLibrary(libraryPath)
	} else {
		log.Println("Scan requested, but a scan is already in progress.")
	}

	subsonicGetScanStatus(c)
}

func subsonicGetScanStatus(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}
	var isScanning bool
	var songsAdded int64
	err := db.QueryRow("SELECT is_scanning, songs_added FROM scan_status WHERE id = 1").Scan(&isScanning, &songsAdded)
	if err != nil {
		subsonicRespond(c, newSubsonicResponse(&SubsonicScanStatus{Scanning: false, Count: 0}))
		return
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicScanStatus{Scanning: isScanning, Count: songsAdded}))
}

// --- Subsonic User Management Handlers ---

func subsonicGetUsers(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok || !user.IsAdmin {
		subsonicRespond(c, newSubsonicErrorResponse(40, "Admin rights required for this operation."))
		return
	}
	rows, err := db.Query("SELECT username, is_admin FROM users ORDER BY username")
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error fetching users."))
		return
	}
	defer rows.Close()

	var users []SubsonicUser
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.Username, &u.IsAdmin); err != nil {
			log.Printf("Error scanning user row: %v", err)
			continue
		}
		users = append(users, SubsonicUser{Username: u.Username, AdminRole: u.IsAdmin, SettingsRole: u.IsAdmin})
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicUsers{Users: users}))
}

func subsonicCreateUser(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok || !user.IsAdmin {
		subsonicRespond(c, newSubsonicErrorResponse(40, "Admin rights required for this operation."))
		return
	}
	username := c.Query("username")
	password := c.Query("password")
	isAdmin, _ := strconv.ParseBool(c.Query("adminRole"))

	if password == "" || username == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Username and password are required."))
		return
	}
	hashedPassword, err := hashPassword(password)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to hash password."))
		return
	}

	_, err = db.Exec("INSERT INTO users (username, password_hash, password_plain, is_admin) VALUES (?, ?, ?, ?)", username, hashedPassword, password, isAdmin)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Could not create user; username might already exist."))
		return
	}
	subsonicRespond(c, newSubsonicResponse(nil))
}

func subsonicUpdateUser(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok || !user.IsAdmin {
		subsonicRespond(c, newSubsonicErrorResponse(40, "Admin rights required for this operation."))
		return
	}
	username := c.Query("username")
	password := c.Query("password")
	if username == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Username is required."))
		return
	}

	// Only updating password for now, as it's the main use case for the UI
	if password != "" {
		hashedPassword, err := hashPassword(password)
		if err != nil {
			subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to hash password."))
			return
		}
		_, err = db.Exec("UPDATE users SET password_hash = ?, password_plain = ? WHERE username = ?", hashedPassword, password, username)
		if err != nil {
			subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to update password."))
			return
		}
	}
	subsonicRespond(c, newSubsonicResponse(nil))
}

func subsonicDeleteUser(c *gin.Context) {
	requestingUser, ok := subsonicAuthenticate(c)
	if !ok || !requestingUser.IsAdmin {
		subsonicRespond(c, newSubsonicErrorResponse(40, "Admin rights required for this operation."))
		return
	}
	usernameToDelete := c.Query("username")
	if usernameToDelete == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Username is required."))
		return
	}
	if requestingUser.Username == usernameToDelete {
		subsonicRespond(c, newSubsonicErrorResponse(50, "You cannot delete your own account."))
		return
	}

	res, err := db.Exec("DELETE FROM users WHERE username = ?", usernameToDelete)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to delete user."))
		return
	}
	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		subsonicRespond(c, newSubsonicErrorResponse(70, "User not found."))
		return
	}
	subsonicRespond(c, newSubsonicResponse(nil))
}

func subsonicChangePassword(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}
	newPassword := c.Query("password")
	if newPassword == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "New password is required."))
		return
	}
	hashedPassword, err := hashPassword(newPassword)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to hash password."))
		return
	}
	_, err = db.Exec("UPDATE users SET password_hash = ?, password_plain = ? WHERE id = ?", hashedPassword, newPassword, user.ID)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to update password."))
		return
	}
	subsonicRespond(c, newSubsonicResponse(nil))
}
