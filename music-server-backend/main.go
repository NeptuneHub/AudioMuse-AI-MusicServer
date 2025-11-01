// Suggested path: music-server-backend/main.go
package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron/v3"
)

var db *sql.DB
var isScanCancelled atomic.Bool // Global flag to signal scan cancellation.
var scheduler *cron.Cron
var isAnalysisRunning atomic.Bool
var isClusteringRunning atomic.Bool

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
	// Run idempotent migrations to cover older DBs that may be missing new tables/keys
	if err := migrateDB(); err != nil {
		log.Printf("Database migration warnings/errors: %v", err)
	}
	startScheduler()

	if _, err := db.Exec("UPDATE scan_status SET is_scanning = 0 WHERE id = 1"); err != nil {
		log.Fatalf("Failed to reset scan status on startup: %v", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
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

		// Browsing endpoints
		subsonic.GET("/getMusicFolders.view", subsonicGetMusicFolders)
		subsonic.GET("/getIndexes.view", subsonicGetIndexes)
		subsonic.GET("/getMusicDirectory.view", subsonicGetMusicDirectory)
		subsonic.GET("/getArtist.view", subsonicGetArtist)
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
		subsonic.GET("/getSongsByGenre.view", subsonicGetSongsByGenre)
		subsonic.GET("/getCoverArt.view", subsonicGetCoverArt)

		// Media info endpoints
		subsonic.GET("/getTopSongs.view", subsonicGetTopSongs)
		subsonic.GET("/getSimilarSongs2.view", subsonicGetSimilarSongs2)
		subsonic.GET("/getAlbumInfo.view", subsonicGetAlbumInfo)
		subsonic.GET("/getAlbumInfo2.view", subsonicGetAlbumInfo)
		subsonic.GET("/download.view", subsonicDownload)

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

		// Star/Unstar functionality
		subsonic.GET("/star.view", subsonicStar)
		subsonic.GET("/unstar.view", subsonicUnstar)
		subsonic.GET("/getStarred.view", subsonicGetStarred)
		subsonic.GET("/getGenres.view", subsonicGetGenres)

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
			// Return info about the logged-in user (JWT required)
			userRoutes.GET("/me", AuthMiddleware(), userInfo)
		}
		adminRoutes := v1.Group("/admin")
		adminRoutes.Use(AuthMiddleware(), adminOnly())
		{
			adminRoutes.GET("/browse", browseFiles)
			adminRoutes.POST("/scan/cancel", cancelAdminScan)
			adminRoutes.POST("/scan/rescan", rescanAllLibraries)
		}
	}

	// Admin-protected cleaning endpoint that proxies to AudioMuse-AI
	r.POST("/api/cleaning/start", AuthMiddleware(), adminOnly(), CleaningStartHandler)

	// Public endpoint used by the frontend to run Alchemy (may be allowed for non-admin flows)
	r.POST("/api/alchemy", AlchemyHandler)

	// Map and voyager proxy endpoints (authenticated)
	r.GET("/api/map", AuthMiddleware(), MapHandler)
	r.GET("/api/voyager/search_tracks", AuthMiddleware(), VoyagerSearchTracksHandler)
	r.POST("/api/map/create_playlist", AuthMiddleware(), MapCreatePlaylistHandler)

	// Serve static files from React build
	r.Static("/static", "/app/music-server-frontend/build/static")
	r.StaticFile("/favicon.ico", "/app/music-server-frontend/build/favicon.ico")
	r.StaticFile("/manifest.json", "/app/music-server-frontend/build/manifest.json")

	// Serve React app for all non-API routes
	r.NoRoute(func(c *gin.Context) {
		// Don't serve index.html for API routes
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/rest/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}
		c.File("/app/music-server-frontend/build/index.html")
	})

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

// corsMiddleware sets permissive CORS headers so browser-based frontends
// can call both the /rest (Subsonic) endpoints and the JSON /api/v1 endpoints.
// For production you may want to restrict the allowed origin via an env var.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Allow all origins by default. For stricter security set ALLOWED_ORIGIN env var.
		allowed := getEnv("ALLOWED_ORIGIN", "*")
		c.Writer.Header().Set("Access-Control-Allow-Origin", allowed)
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, Cache-Control, Pragma")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
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

	// Add starred column if it doesn't exist (backward compatibility)
	_, err = db.Exec(`ALTER TABLE songs ADD COLUMN starred INTEGER NOT NULL DEFAULT 0;`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		log.Printf("Note: Could not add starred column (may already exist): %v", err)
	}

	// Add genre column if it doesn't exist (backward compatibility)
	_, err = db.Exec(`ALTER TABLE songs ADD COLUMN genre TEXT DEFAULT '';`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		log.Printf("Note: Could not add genre column (may already exist): %v", err)
	}

	// Create starred_songs table for user-specific stars
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS starred_songs (
		user_id INTEGER NOT NULL,
		song_id INTEGER NOT NULL,
		starred_at TEXT NOT NULL,
		PRIMARY KEY (user_id, song_id),
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY(song_id) REFERENCES songs(id) ON DELETE CASCADE
	);`)
	if err != nil {
		log.Fatalf("Failed to create starred_songs table: %v", err)
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

	// Schedule Analysis and Clustering if configured
	// Analysis: read analysis_schedule and analysis_enabled
	var analysisSchedule string
	var analysisEnabledStr string
	if err := db.QueryRow("SELECT value FROM configuration WHERE key = 'analysis_schedule'").Scan(&analysisSchedule); err != nil {
		log.Printf("Analysis schedule not set in configuration, using default")
		analysisSchedule = "0 2 * * 0-5" // default: nightly at 2:00 except Saturday
	}
	if err := db.QueryRow("SELECT value FROM configuration WHERE key = 'analysis_enabled'").Scan(&analysisEnabledStr); err != nil {
		analysisEnabledStr = "false"
	}
	analysisEnabled := (analysisEnabledStr == "true")

	if analysisEnabled {
		_, err := scheduler.AddFunc(analysisSchedule, func() {
			if isAnalysisRunning.Load() {
				log.Println("Scheduled analysis skipped: analysis already running")
				return
			}
			isAnalysisRunning.Store(true)
			log.Println("Cron job triggered: starting scheduled analysis")
			go func() {
				defer isAnalysisRunning.Store(false)
				ctx := context.Background()
				if err := runAnalysisJob(ctx); err != nil {
					log.Printf("Scheduled analysis failed: %v", err)
				}
			}()
		})
		if err != nil {
			log.Fatalf("Error scheduling analysis cron job: %v", err)
		}
		log.Printf("Scheduled analysis started with schedule: '%s'", analysisSchedule)
	} else {
		log.Println("Scheduled analysis is disabled.")
	}

	// Clustering: read clustering_schedule and clustering_enabled
	var clusteringSchedule string
	var clusteringEnabledStr string
	if err := db.QueryRow("SELECT value FROM configuration WHERE key = 'clustering_schedule'").Scan(&clusteringSchedule); err != nil {
		log.Printf("Clustering schedule not set in configuration, using default")
		clusteringSchedule = "0 2 * * 6" // default: Saturday at 2:00
	}
	if err := db.QueryRow("SELECT value FROM configuration WHERE key = 'clustering_enabled'").Scan(&clusteringEnabledStr); err != nil {
		clusteringEnabledStr = "false"
	}
	clusteringEnabled := (clusteringEnabledStr == "true")

	if clusteringEnabled {
		_, err := scheduler.AddFunc(clusteringSchedule, func() {
			if isClusteringRunning.Load() {
				log.Println("Scheduled clustering skipped: clustering already running")
				return
			}
			isClusteringRunning.Store(true)
			log.Println("Cron job triggered: starting scheduled clustering")
			go func() {
				defer isClusteringRunning.Store(false)
				ctx := context.Background()
				if err := runClusteringJob(ctx); err != nil {
					log.Printf("Scheduled clustering failed: %v", err)
				}
			}()
		})
		if err != nil {
			log.Fatalf("Error scheduling clustering cron job: %v", err)
		}
		log.Printf("Scheduled clustering started with schedule: '%s'", clusteringSchedule)
	} else {
		log.Println("Scheduled clustering is disabled.")
	}
}
