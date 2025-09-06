// Suggested path: music-server-backend/main.go
package main

import (
	"database/sql"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func main() {
	var err error
	// Enabled WAL mode for better concurrency and to prevent locking issues.
	db, err = sql.Open("sqlite3", "./music.db?_journal_mode=WAL")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	initDB()

	r := gin.Default()

	// Subsonic API routes
	rest := r.Group("/rest")
	{
		rest.GET("/ping.view", subsonicPing)
		rest.GET("/getLicense.view", subsonicGetLicense)
		rest.GET("/stream.view", subsonicStream)
		rest.GET("/getArtists.view", subsonicGetArtists)
		rest.GET("/getAlbumList2.view", subsonicGetAlbumList2)
		rest.GET("/getPlaylists.view", subsonicGetPlaylists)
		// Add stubs for other common endpoints to avoid 404s
		rest.GET("/getAlbum.view", subsonicGetAlbum) // To get songs in an album
		rest.GET("/search2.view", subsonicSearch)
		rest.GET("/search3.view", subsonicSearch)
		rest.GET("/getRandomSongs.view", subsonicGetRandomSongs)
		rest.GET("/getCoverArt.view", subsonicGetCoverArt)
		rest.GET("/tokenInfo.view", subsonicTokenInfo)
		rest.GET("/startScan.view", subsonicStartScan)
		rest.GET("/getScanStatus.view", subsonicGetScanStatus)
	}

	// JSON API routes for the web UI
	v1 := r.Group("/api/v1")
	{
		// This route is a bridge for the web UI to get a JWT after a successful
		// Subsonic password authentication. It's not used by external clients.
		userRoutes := v1.Group("/user")
		{
			userRoutes.POST("/login", loginUser)
		}
		// Admin routes
		adminRoutes := v1.Group("/admin")
		adminRoutes.Use(AuthMiddleware(), adminOnly())
		{
			// Filesystem browsing for library selection
			adminRoutes.GET("/browse", browseFiles)
			// Library Scanning for Web UI
			adminRoutes.POST("/scan/start", startAdminScan)
			adminRoutes.GET("/scan/status", getAdminScanStatus)
			// User Management Routes
			adminRoutes.GET("/users", getUsers)
			adminRoutes.POST("/users", createUser)
			adminRoutes.PUT("/users/:id/password", updateUserPassword)
			adminRoutes.DELETE("/users/:id", deleteUser)
		}
	}

	r.Run(":8080") // listen and serve on 0.0.0.0:8080
}

func adminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get("isAdmin")
		if !exists || !isAdmin.(bool) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func initDB() {
	// Create users table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE,
			password_hash TEXT,
			password_plain TEXT, -- WARNING: Storing plaintext passwords is a security risk. Required for Subsonic token auth.
			is_admin BOOLEAN 
		);
	`)
	if err != nil {
		log.Fatal("Failed to create users table:", err)
	}

	// Add password_plain column for Subsonic token auth if it doesn't exist.
	// This is a simple migration for existing databases.
	_, err = db.Exec("ALTER TABLE users ADD COLUMN password_plain TEXT")
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		log.Printf("Warning: Could not add password_plain column, token auth may fail if not already present: %v", err)
	}

	// Create scan_status table to track library scans
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS scan_status (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			is_scanning BOOLEAN NOT NULL DEFAULT 0,
			songs_added INTEGER NOT NULL DEFAULT 0,
			last_update_time TEXT
		);
	`)
	if err != nil {
		log.Fatal("Failed to create scan_status table:", err)
	}
	// Ensure the single row exists for tracking status
	db.Exec(`INSERT OR IGNORE INTO scan_status (id) VALUES (1);`)

	// Create songs table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS songs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT,
			artist TEXT,
			album TEXT,
			path TEXT UNIQUE
		);
	`)
	if err != nil {
		log.Fatal("Failed to create songs table:", err)
	}

	// Create playlists table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS playlists (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			user_id INTEGER,
			FOREIGN KEY(user_id) REFERENCES users(id)
		);
	`)
	if err != nil {
		log.Fatal("Failed to create playlists table:", err)
	}

	// Create playlist_songs table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS playlist_songs (
			playlist_id INTEGER,
			song_id INTEGER,
			FOREIGN KEY(playlist_id) REFERENCES playlists(id),
			FOREIGN KEY(song_id) REFERENCES songs(id),
			PRIMARY KEY (playlist_id, song_id)
		);
	`)
	if err != nil {
		log.Fatal("Failed to create playlist_songs table:", err)
	}

	// Create initial admin user if not exists
	var count int
	row := db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'admin'")
	if err := row.Scan(&count); err == nil && count == 0 {
		hashedPassword, _ := hashPassword("admin")
		_, err := db.Exec("INSERT INTO users (username, password_hash, password_plain, is_admin) VALUES (?, ?, ?, ?)", "admin", hashedPassword, "admin", true)
		if err != nil {
			log.Println("Could not create default admin user:", err)
		} else {
			log.Println("Default admin user created with password 'admin'")
		}
	}
}
