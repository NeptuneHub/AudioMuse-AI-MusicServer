// Suggested path: music-server-backend/subsonic_admin_handlers.go
package main

import (
	"database/sql"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func subsonicStartScan(c *gin.Context) {
	user := c.MustGet("user").(User)
	if !user.IsAdmin {
		subsonicRespond(c, newSubsonicErrorResponse(40, "Admin rights required."))
		return
	}

	var isScanning bool
	err := db.QueryRow("SELECT is_scanning FROM scan_status WHERE id = 1").Scan(&isScanning)
	if err != nil && err != sql.ErrNoRows {
		subsonicRespond(c, newSubsonicErrorResponse(0, "DB error checking scan status."))
		return
	}

	if isScanning {
		log.Println("Scan requested, but a scan is already in progress.")
		subsonicGetScanStatus(c)
		return
	}

	_, err = db.Exec("UPDATE scan_status SET is_scanning = 1, songs_added = 0, last_update_time = ? WHERE id = 1", time.Now().Format(time.RFC3339))
	if err != nil {
		log.Printf("Error starting scan in DB: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "DB error starting scan."))
		return
	}

	pathIdStr := c.Query("pathId")
	if pathIdStr != "" {
		pathId, err := strconv.Atoi(pathIdStr)
		if err != nil {
			subsonicRespond(c, newSubsonicErrorResponse(10, "Invalid pathId provided."))
			db.Exec("UPDATE scan_status SET is_scanning = 0 WHERE id = 1")
			return
		}
		go scanSingleLibrary(pathId)
	} else {
		go scanAllLibraries()
	}

	subsonicGetScanStatus(c)
}

func subsonicGetScanStatus(c *gin.Context) {
	_ = c.MustGet("user") // Auth is handled by middleware
	var isScanning bool
	var songsAdded int64
	err := db.QueryRow("SELECT is_scanning, songs_added FROM scan_status WHERE id = 1").Scan(&isScanning, &songsAdded)
	if err != nil {
		subsonicRespond(c, newSubsonicResponse(&SubsonicScanStatus{Scanning: false, Count: 0}))
		return
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicScanStatus{Scanning: isScanning, Count: songsAdded}))
}

func subsonicGetLibraryPaths(c *gin.Context) {
	user := c.MustGet("user").(User)
	_ = user // Auth is handled by middleware
	rows, err := db.Query("SELECT id, path, song_count, last_scan_ended FROM library_paths ORDER BY path")
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "DB error fetching library paths."))
		return
	}
	defer rows.Close()

	var paths []SubsonicLibraryPath
	for rows.Next() {
		var p LibraryPath
		var lastScan sql.NullString
		if err := rows.Scan(&p.ID, &p.Path, &p.SongCount, &lastScan); err != nil {
			log.Printf("Error scanning library path row: %v", err)
			continue
		}
		paths = append(paths, SubsonicLibraryPath{
			ID: p.ID, Path: p.Path, SongCount: p.SongCount, LastScanEnded: lastScan.String,
		})
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicLibraryPaths{Paths: paths}))
}

func subsonicAddLibraryPath(c *gin.Context) {
	user := c.MustGet("user").(User)
	_ = user // Auth is handled by middleware
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "A valid path is required."))
		return
	}

	_, err := db.Exec("INSERT INTO library_paths (path) VALUES (?)", req.Path)
	if err != nil {
		log.Printf("Database error adding library path '%s': %v", req.Path, err)
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			subsonicRespond(c, newSubsonicErrorResponse(0, "This library path already exists."))
		} else {
			subsonicRespond(c, newSubsonicErrorResponse(0, "A database error occurred."))
		}
		return
	}
	subsonicGetLibraryPaths(c)
}

func subsonicUpdateLibraryPath(c *gin.Context) {
	user := c.MustGet("user").(User)
	_ = user // Auth is handled by middleware
	var req struct {
		ID   int    `json:"id"`
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Path == "" || req.ID == 0 {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Valid ID and path are required."))
		return
	}
	_, err := db.Exec("UPDATE library_paths SET path = ? WHERE id = ?", req.Path, req.ID)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to update library path."))
		return
	}
	subsonicGetLibraryPaths(c)
}

func subsonicDeleteLibraryPath(c *gin.Context) {
	user := c.MustGet("user").(User)
	_ = user // Auth is handled by middleware
	var req struct {
		ID int `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		subsonicRespond(c, newSubsonicErrorResponse(10, "A valid ID is required."))
		return
	}
	_, err := db.Exec("DELETE FROM library_paths WHERE id = ?", req.ID)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to delete library path."))
		return
	}
	subsonicGetLibraryPaths(c)
}

func subsonicGetConfiguration(c *gin.Context) {
	user := c.MustGet("user").(User)
	// Admins get full configuration. Non-admins may read only the audiomuse URL key.
	if !user.IsAdmin {
		// Return only the audiomuse_ai_core_url key (if present) so normal users can use AudioMuse features when configured.
		var value sql.NullString
		err := db.QueryRow("SELECT value FROM configuration WHERE key = ?", "audiomuse_ai_core_url").Scan(&value)
		if err != nil && err != sql.ErrNoRows {
			subsonicRespond(c, newSubsonicErrorResponse(0, "DB error fetching configuration."))
			return
		}
		var configs []SubsonicConfiguration
		if value.Valid {
			configs = append(configs, SubsonicConfiguration{Name: "audiomuse_ai_core_url", Value: value.String})
		}
		subsonicRespond(c, newSubsonicResponse(&SubsonicConfigurations{Configurations: configs}))
		return
	}

	rows, err := db.Query("SELECT key, value FROM configuration")
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "DB error fetching configuration."))
		return
	}
	defer rows.Close()
	var configs []SubsonicConfiguration
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			log.Printf("Error scanning configuration row: %v", err)
			continue
		}
		configs = append(configs, SubsonicConfiguration{Name: key, Value: value})
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicConfigurations{Configurations: configs}))
}

func subsonicSetConfiguration(c *gin.Context) {
	user := c.MustGet("user").(User)
	if !user.IsAdmin {
		subsonicRespond(c, newSubsonicErrorResponse(40, "Admin rights required."))
		return
	}
	key := c.Query("key")
	value := c.Query("value")
	if key == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Parameter 'key' is required."))
		return
	}
	_, err := db.Exec("INSERT OR REPLACE INTO configuration (key, value) VALUES (?, ?)", key, value)
	if err != nil {
		log.Printf("Error saving configuration key '%s': %v", key, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to save configuration."))
		return
	}

	// Restart scheduler if any schedule-related config changed
	if key == "scan_schedule" || key == "scan_enabled" ||
		key == "analysis_schedule" || key == "analysis_enabled" ||
		key == "clustering_schedule" || key == "clustering_enabled" {
		log.Println("Scheduler configuration changed, restarting scheduler...")
		if scheduler != nil {
			scheduler.Stop()
		}
		startScheduler()
	}

	subsonicGetConfiguration(c)
}

func subsonicGetUsers(c *gin.Context) {
	user := c.MustGet("user").(User)
	if !user.IsAdmin {
		subsonicRespond(c, newSubsonicErrorResponse(40, "Admin rights required."))
		return
	}
	rows, err := db.Query("SELECT username, is_admin FROM users ORDER BY username")
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "DB error fetching users."))
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
	user := c.MustGet("user").(User)
	if !user.IsAdmin {
		subsonicRespond(c, newSubsonicErrorResponse(40, "Admin rights required."))
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
		subsonicRespond(c, newSubsonicErrorResponse(0, "Could not create user."))
		return
	}
	subsonicRespond(c, newSubsonicResponse(nil))
}

func subsonicUpdateUser(c *gin.Context) {
	user := c.MustGet("user").(User)
	if !user.IsAdmin {
		subsonicRespond(c, newSubsonicErrorResponse(40, "Admin rights required."))
		return
	}
	username := c.Query("username")
	password := c.Query("password")
	if username == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Username is required."))
		return
	}

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
	requestingUser := c.MustGet("user").(User)
	if !requestingUser.IsAdmin {
		subsonicRespond(c, newSubsonicErrorResponse(40, "Admin rights required."))
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
	user, ok := c.Get("user")
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
	_, err = db.Exec("UPDATE users SET password_hash = ?, password_plain = ? WHERE id = ?", hashedPassword, newPassword, user.(User).ID)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to update password."))
		return
	}
	subsonicRespond(c, newSubsonicResponse(nil))
}
