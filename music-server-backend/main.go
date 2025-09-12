// Suggested path: music-server-backend/main.go
package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron/v3"
)

var db *sql.DB
var isScanCancelled atomic.Bool // Global flag to signal scan cancellation.
var scheduler *cron.Cron

func loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		log.Printf(
			"[GIN] | %d | %13v | %15s | %-7s | %s",
			c.Writer.Status(),
			latency,
			c.ClientIP(),
			c.Request.Method,
			c.Request.URL.Path,
		)
		if c.Request.URL.RawQuery != "" {
			log.Printf("[GIN-QUERY] %s", c.Request.URL.RawQuery)
		}
	}
}

// getEnv gets an environment variable or returns a default value.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	var err error
	defaultDbPath := "/config/music.db"
	dbPath := getEnv("DATABASE_PATH", defaultDbPath)

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatalf("Failed to create database directory '%s': %v", filepath.Dir(dbPath), err)
	}
	db, err = sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	initDB()
	startScheduler()

	if _, err := db.Exec("UPDATE scan_status SET is_scanning = 0 WHERE id = 1"); err != nil {
		log.Fatalf("Failed to reset scan status on startup: %v", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(loggingMiddleware())

	// Public Subsonic routes (no auth required)
	r.GET("/rest/ping.view", subsonicPing)
	r.GET("/rest/getOpenSubsonicExtensions.view", subsonicGetOpenSubsonicExtensions)

	// Authenticated Subsonic API routes
	subsonic := r.Group("/rest")
	subsonic.Use(SubsonicAuthMiddleware())
	{
		subsonic.GET("/getLicense.view", subsonicGetLicense)
		subsonic.GET("/stream.view", subsonicStream)
		subsonic.GET("/scrobble.view", subsonicScrobble)
		subsonic.GET("/getArtists.view", subsonicGetArtists)
		subsonic.GET("/getAlbumList2.view", subsonicGetAlbumList2)
		subsonic.GET("/getPlaylists.view", subsonicGetPlaylists)
		subsonic.GET("/getPlaylist.view", subsonicGetPlaylist)
		subsonic.Any("/createPlaylist.view", subsonicCreatePlaylist)
		subsonic.GET("/updatePlaylist.view", subsonicUpdatePlaylist)
		subsonic.GET("/deletePlaylist.view", subsonicDeletePlaylist)
		subsonic.GET("/getAlbum.view", subsonicGetAlbum)
		subsonic.GET("/search2.view", subsonicSearch2)
		subsonic.GET("/search3.view", subsonicSearch2)
		subsonic.GET("/getSong.view", subsonicGetSong)
		subsonic.GET("/getRandomSongs.view", subsonicGetRandomSongs)
		subsonic.GET("/getCoverArt.view", subsonicGetCoverArt)
		subsonic.Any("/startScan.view", subsonicStartScan)
		subsonic.GET("/getScanStatus.view", subsonicGetScanStatus)
		subsonic.GET("/getLibraryPaths.view", subsonicGetLibraryPaths)
		subsonic.POST("/addLibraryPath.view", subsonicAddLibraryPath)
		subsonic.POST("/updateLibraryPath.view", subsonicUpdateLibraryPath)
		subsonic.POST("/deleteLibraryPath.view", subsonicDeleteLibraryPath)
		subsonic.GET("/getUsers.view", subsonicGetUsers)
		subsonic.GET("/createUser.view", subsonicCreateUser)
		subsonic.GET("/updateUser.view", subsonicUpdateUser)
		subsonic.GET("/deleteUser.view", subsonicDeleteUser)
		subsonic.GET("/changePassword.view", subsonicChangePassword)
		subsonic.GET("/getConfiguration.view", subsonicGetConfiguration)
		subsonic.GET("/setConfiguration.view", subsonicSetConfiguration)
		subsonic.GET("/getSimilarSongs.view", subsonicGetSimilarSongs)
		subsonic.GET("/getSongPath.view", subsonicGetSongPath)
		subsonic.GET("/getSonicFingerprint.view", subsonicGetSonicFingerprint)

		// API Key Management
		subsonic.GET("/getApiKey.view", subsonicGetApiKey)
		subsonic.POST("/revokeApiKey.view", subsonicRevokeApiKey)

		// AudioMuse-AI Subsonic routes
		subsonic.Any("/startSonicAnalysis.view", subsonicStartSonicAnalysis)
		subsonic.GET("/getSonicAnalysisStatus.view", subsonicGetSonicAnalysisStatus)
		subsonic.Any("/cancelSonicAnalysis.view", subsonicCancelSonicAnalysis)
		subsonic.Any("/startSonicClustering.view", subsonicStartClusteringAnalysis)
	}

	// Separate JSON API for Web UI
	v1 := r.Group("/api/v1")
	{
		userRoutes := v1.Group("/user")
		{
			userRoutes.POST("/login", loginUser)
		}
		adminRoutes := v1.Group("/admin")
		adminRoutes.Use(AuthMiddleware(), adminOnly())
		{
			adminRoutes.GET("/browse", browseFiles)
			adminRoutes.POST("/scan/cancel", cancelAdminScan)
		}
	}

	log.Println("[GIN-debug] Listening and serving HTTP on :8080")
	r.Run(":8080")
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
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		password_plain TEXT NOT NULL,
		is_admin BOOLEAN NOT NULL DEFAULT 0,
		api_key TEXT UNIQUE
	);`)
	if err != nil {
		log.Fatalf("Failed to create/update users table: %v", err)
	}
	// ... rest of initDB is unchanged
	// ... (make sure to copy the rest of your initDB function here if it's not present)

	// Scan status table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS scan_status (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		is_scanning BOOLEAN NOT NULL DEFAULT 0,
		songs_added INTEGER NOT NULL DEFAULT 0,
		last_update_time TEXT
	);`)
	if err != nil {
		log.Fatalf("Failed to create scan_status table: %v", err)
	}
	db.Exec(`INSERT OR IGNORE INTO scan_status (id, is_scanning, songs_added) VALUES (1, 0, 0);`)

	// Songs table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS songs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT,
		artist TEXT,
		album TEXT,
		path TEXT UNIQUE NOT NULL,
		play_count INTEGER NOT NULL DEFAULT 0,
		last_played TEXT,
		date_added TEXT,
		date_updated TEXT
	);`)
	if err != nil {
		log.Fatalf("Failed to create songs table: %v", err)
	}

	// Playlists table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS playlists (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		user_id INTEGER,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`)
	if err != nil {
		log.Fatalf("Failed to create playlists table: %v", err)
	}

	// Playlist songs table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS playlist_songs (
		playlist_id INTEGER NOT NULL,
		song_id INTEGER NOT NULL,
		position INTEGER NOT NULL,
		FOREIGN KEY(playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
		FOREIGN KEY(song_id) REFERENCES songs(id) ON DELETE CASCADE
	);`)
	if err != nil {
		log.Fatalf("Failed to create playlist_songs table: %v", err)
	}

	// Create index for performance
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_playlist_songs_order ON playlist_songs (playlist_id, position);`)
	if err != nil {
		log.Fatalf("Failed to create index on playlist_songs: %v", err)
	}

	// Configuration table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS configuration (
		key TEXT PRIMARY KEY NOT NULL,
		value TEXT
	);`)
	if err != nil {
		log.Fatalf("Failed to create configuration table: %v", err)
	}
	db.Exec(`INSERT OR IGNORE INTO configuration (key, value) VALUES ('scan_enabled', 'true');`)
	db.Exec(`INSERT OR IGNORE INTO configuration (key, value) VALUES ('scan_schedule', '0 2 * * *');`)

	// Library paths table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS library_paths (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT UNIQUE NOT NULL,
		song_count INTEGER NOT NULL DEFAULT 0,
		last_scan_ended TEXT
	);`)
	if err != nil {
		log.Fatalf("Failed to create library_paths table: %v", err)
	}

	// Default admin user
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

func startScheduler() {
	scheduler = cron.New()
	var schedule, enabledStr string
	var isEnabled bool

	err := db.QueryRow("SELECT value FROM configuration WHERE key = 'scan_schedule'").Scan(&schedule)
	if err != nil {
		log.Printf("Could not read scan_schedule from config, using default. Error: %v", err)
		schedule = "0 2 * * *" // Default: 2 AM daily
	}

	err = db.QueryRow("SELECT value FROM configuration WHERE key = 'scan_enabled'").Scan(&enabledStr)
	if err != nil {
		log.Printf("Could not read scan_enabled from config, defaulting to true. Error: %v", err)
		isEnabled = true
	} else {
		isEnabled = (enabledStr == "true")
	}

	if isEnabled {
		_, err := scheduler.AddFunc(schedule, func() {
			log.Println("Cron job triggered: starting scheduled scan of all libraries.")
			var isScanning bool
			db.QueryRow("SELECT is_scanning FROM scan_status WHERE id = 1").Scan(&isScanning)
			if !isScanning {
				db.Exec("UPDATE scan_status SET is_scanning = 1, songs_added = 0, last_update_time = ? WHERE id = 1", time.Now().Format(time.RFC3339))
				scanAllLibraries()
			} else {
				log.Println("Scheduled scan skipped: a scan is already in progress.")
			}
		})
		if err != nil {
			log.Fatalf("Error scheduling library scan cron job: %v", err)
		}
		scheduler.Start()
		log.Printf("Scheduled library scan started with schedule: '%s'", schedule)
	} else {
		log.Println("Scheduled library scan is disabled.")
	}
}
