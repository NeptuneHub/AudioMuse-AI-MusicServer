// Suggested path: music-server-backend/main.go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/robfig/cron/v3"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var db *sql.DB
var isScanCancelled atomic.Bool // Global flag to signal scan cancellation.
var scheduler *cron.Cron
var isAnalysisRunning atomic.Bool
var isClusteringRunning atomic.Bool

// backupMu ensures only one backup runs at a time
var backupMu sync.Mutex

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

// subsonicCompatibilityHandler creates a wrapper that registers both .view and non-.view versions
func subsonicCompatibilityHandler(router gin.IRouter, method string, path string, handler gin.HandlerFunc) {
	// Register with .view suffix (standard)
	switch method {
	case "GET":
		router.GET(path+".view", handler)
		router.GET(path, handler) // Also register without .view
	case "POST":
		router.POST(path+".view", handler)
		router.POST(path, handler)
	case "ANY":
		router.Any(path+".view", handler)
		router.Any(path, handler)
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
	// NOTE: Do not defer db.Close() here. DB will be closed during graceful shutdown or if a restore is performed.

	// Strengthen SQLite durability and timeouts
	if _, err := db.Exec("PRAGMA synchronous = FULL"); err != nil {
		log.Printf("Warning: could not set PRAGMA synchronous: %v", err)
	}
	if _, err := db.Exec("PRAGMA wal_autocheckpoint = 1000"); err != nil {
		log.Printf("Warning: could not set PRAGMA wal_autocheckpoint: %v", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		log.Printf("Warning: could not set PRAGMA busy_timeout: %v", err)
	}

	// Verify DB integrity on startup and attempt restore from single rotating backup if corrupted
	if err := checkAndRestoreDB(dbPath); err != nil {
		log.Printf("ERROR: %v", err)
		log.Fatal("Database is corrupted and no valid backup available.")
	}

	initDB()
	// Run idempotent migrations to cover older DBs that may be missing new tables/keys
	if err := migrateDB(); err != nil {
		log.Printf("Database migration warnings/errors: %v", err)
	}
	startScheduler()
	StartSessionCleanup() // Start HLS session cleanup

	// Start periodic DB maintenance (checkpoint, integrity checks, optional backups)
	startDBMaintenance(db, dbPath)

	if _, err := db.Exec("UPDATE scan_status SET is_scanning = 0 WHERE id = 1"); err != nil {
		log.Fatalf("Failed to reset scan status on startup: %v", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())
	r.Use(loggingMiddleware())

	// Public Subsonic routes (no auth required) - register both with and without .view
	subsonicCompatibilityHandler(r, "GET", "/rest/ping", subsonicPing)
	subsonicCompatibilityHandler(r, "GET", "/rest/getOpenSubsonicExtensions", subsonicGetOpenSubsonicExtensions)

	// Authenticated Subsonic API routes
	subsonic := r.Group("/rest")
	subsonic.Use(SubsonicAuthMiddleware())
	{
		// Core endpoints - register both with and without .view suffix
		subsonicCompatibilityHandler(subsonic, "GET", "/getLicense", subsonicGetLicense)
		subsonicCompatibilityHandler(subsonic, "GET", "/stream", subsonicStream)
		subsonicCompatibilityHandler(subsonic, "GET", "/waveform", subsonicGetWaveform)    // NEW: Fast waveform data
		subsonicCompatibilityHandler(subsonic, "GET", "/hlsPlaylist", subsonicHLSPlaylist) // NEW: HLS playlist
		subsonicCompatibilityHandler(subsonic, "GET", "/hlsSegment", subsonicHLSSegment)   // NEW: HLS segments
		subsonicCompatibilityHandler(subsonic, "GET", "/scrobble", subsonicScrobble)

		// Browsing endpoints
		subsonicCompatibilityHandler(subsonic, "GET", "/getMusicFolders", subsonicGetMusicFolders)
		subsonicCompatibilityHandler(subsonic, "GET", "/getIndexes", subsonicGetIndexes)
		subsonicCompatibilityHandler(subsonic, "GET", "/getMusicDirectory", subsonicGetMusicDirectory)
		subsonicCompatibilityHandler(subsonic, "GET", "/getArtist", subsonicGetArtist)
		subsonicCompatibilityHandler(subsonic, "GET", "/getArtists", subsonicGetArtists)
		subsonicCompatibilityHandler(subsonic, "GET", "/getAlbumList2", subsonicGetAlbumList2)
		subsonicCompatibilityHandler(subsonic, "GET", "/getPlaylists", subsonicGetPlaylists)
		subsonicCompatibilityHandler(subsonic, "GET", "/getPlaylist", subsonicGetPlaylist)
		subsonicCompatibilityHandler(subsonic, "ANY", "/createPlaylist", subsonicCreatePlaylist)
		subsonicCompatibilityHandler(subsonic, "ANY", "/updatePlaylist", subsonicUpdatePlaylist)
		subsonicCompatibilityHandler(subsonic, "ANY", "/deletePlaylist", subsonicDeletePlaylist)
		subsonicCompatibilityHandler(subsonic, "GET", "/getAlbum", subsonicGetAlbum)
		subsonicCompatibilityHandler(subsonic, "GET", "/search2", subsonicSearch2)
		subsonicCompatibilityHandler(subsonic, "GET", "/search3", subsonicSearch3)
		subsonicCompatibilityHandler(subsonic, "GET", "/getSong", subsonicGetSong)
		subsonicCompatibilityHandler(subsonic, "GET", "/getRandomSongs", subsonicGetRandomSongs)
		subsonicCompatibilityHandler(subsonic, "GET", "/getSongsByGenre", subsonicGetSongsByGenre)
		subsonicCompatibilityHandler(subsonic, "GET", "/getCoverArt", subsonicGetCoverArt)

		// Media info endpoints
		subsonicCompatibilityHandler(subsonic, "GET", "/getTopSongs", subsonicGetTopSongs)
		subsonicCompatibilityHandler(subsonic, "GET", "/getSimilarSongs2", subsonicGetSimilarSongs2)
		subsonicCompatibilityHandler(subsonic, "GET", "/getSimilarArtists2", subsonicGetSimilarArtists2)
		subsonicCompatibilityHandler(subsonic, "GET", "/getAlbumInfo", subsonicGetAlbumInfo)
		subsonicCompatibilityHandler(subsonic, "GET", "/getAlbumInfo2", subsonicGetAlbumInfo)
		subsonicCompatibilityHandler(subsonic, "GET", "/download", subsonicDownload)

		subsonicCompatibilityHandler(subsonic, "ANY", "/startScan", subsonicStartScan)
		subsonicCompatibilityHandler(subsonic, "GET", "/getScanStatus", subsonicGetScanStatus)
		subsonicCompatibilityHandler(subsonic, "GET", "/getLibraryPaths", subsonicGetLibraryPaths)
		subsonicCompatibilityHandler(subsonic, "POST", "/addLibraryPath", subsonicAddLibraryPath)
		subsonicCompatibilityHandler(subsonic, "POST", "/updateLibraryPath", subsonicUpdateLibraryPath)
		subsonicCompatibilityHandler(subsonic, "POST", "/deleteLibraryPath", subsonicDeleteLibraryPath)
		subsonicCompatibilityHandler(subsonic, "GET", "/getUsers", subsonicGetUsers)
		subsonicCompatibilityHandler(subsonic, "GET", "/createUser", subsonicCreateUser)
		subsonicCompatibilityHandler(subsonic, "GET", "/updateUser", subsonicUpdateUser)
		subsonicCompatibilityHandler(subsonic, "GET", "/deleteUser", subsonicDeleteUser)
		subsonicCompatibilityHandler(subsonic, "GET", "/changePassword", subsonicChangePassword)
		subsonicCompatibilityHandler(subsonic, "GET", "/getConfiguration", subsonicGetConfiguration)
		subsonicCompatibilityHandler(subsonic, "GET", "/setConfiguration", subsonicSetConfiguration)
		subsonicCompatibilityHandler(subsonic, "GET", "/getSimilarSongs", subsonicGetSimilarSongs)
		subsonicCompatibilityHandler(subsonic, "GET", "/getSongPath", subsonicGetSongPath)
		subsonicCompatibilityHandler(subsonic, "GET", "/getSonicFingerprint", subsonicGetSonicFingerprint)

		// Star/Unstar functionality
		subsonicCompatibilityHandler(subsonic, "GET", "/star", subsonicStar)
		subsonicCompatibilityHandler(subsonic, "GET", "/unstar", subsonicUnstar)
		subsonicCompatibilityHandler(subsonic, "GET", "/getStarred", subsonicGetStarred)
		subsonicCompatibilityHandler(subsonic, "GET", "/getGenres", subsonicGetGenres)

		// API Key Management
		subsonicCompatibilityHandler(subsonic, "GET", "/getApiKey", subsonicGetApiKey)
		subsonicCompatibilityHandler(subsonic, "POST", "/revokeApiKey", subsonicRevokeApiKey)

		// AudioMuse-AI Subsonic routes
		subsonicCompatibilityHandler(subsonic, "ANY", "/startSonicAnalysis", subsonicStartSonicAnalysis)
		subsonicCompatibilityHandler(subsonic, "GET", "/getSonicAnalysisStatus", subsonicGetSonicAnalysisStatus)
		subsonicCompatibilityHandler(subsonic, "ANY", "/cancelSonicAnalysis", subsonicCancelSonicAnalysis)
		subsonicCompatibilityHandler(subsonic, "ANY", "/startSonicClustering", subsonicStartClusteringAnalysis)
	}

	// Separate JSON API for Web UI
	v1 := r.Group("/api/v1")
	{
		userRoutes := v1.Group("/user")
		{
			userRoutes.POST("/login", loginUser)
			// Return info about the logged-in user (JWT required)
			userRoutes.GET("/me", AuthMiddleware(), userInfo)
			// User transcoding settings
			userRoutes.GET("/settings/transcoding", AuthMiddleware(), getUserTranscodingSettings)
			userRoutes.POST("/settings/transcoding", AuthMiddleware(), updateUserTranscodingSettings)
		}
		adminRoutes := v1.Group("/admin")
		adminRoutes.Use(AuthMiddleware(), adminOnly())
		{
			adminRoutes.GET("/browse", browseFiles)
			adminRoutes.POST("/scan/cancel", cancelAdminScan)
			adminRoutes.POST("/scan/rescan", rescanAllLibraries)
		}
		// Discovery views (authenticated)
		v1.GET("/counts", AuthMiddleware(), getMusicCounts)
		v1.GET("/recently-added", AuthMiddleware(), getRecentlyAdded)
		v1.GET("/most-played", AuthMiddleware(), getMostPlayed)
		v1.GET("/recently-played", AuthMiddleware(), getRecentlyPlayed)
		v1.GET("/debug/songs", AuthMiddleware(), debugSongsHandler)
	}

	// Admin-protected cleaning endpoint that proxies to AudioMuse-AI
	r.POST("/api/cleaning/start", AuthMiddleware(), adminOnly(), CleaningStartHandler)

	// Alchemy endpoints — require authentication (available to any authenticated user)
	r.POST("/api/alchemy", AuthMiddleware(), AlchemyHandler)
	r.GET("/api/alchemy/search_artists", AuthMiddleware(), SearchArtistsHandler)

	// Radio endpoints (authenticated)
	r.POST("/api/radios", AuthMiddleware(), createRadioHandler)
	r.GET("/api/radios", AuthMiddleware(), getRadiosHandler)
	r.GET("/api/radios/:id/seed", AuthMiddleware(), getRadioSeedHandler)
	r.DELETE("/api/radios/:id", AuthMiddleware(), deleteRadioHandler)
	r.PUT("/api/radios/:id/name", AuthMiddleware(), updateRadioNameHandler)

	// Map and voyager proxy endpoints (authenticated)
	r.GET("/api/map", AuthMiddleware(), MapHandler)
	r.GET("/api/voyager/search_tracks", AuthMiddleware(), VoyagerSearchTracksHandler)
	r.POST("/api/map/create_playlist", AuthMiddleware(), MapCreatePlaylistHandler)

	// CLAP search endpoints (authenticated)
	r.POST("/api/clap/search", AuthMiddleware(), clapSearchHandler)
	r.GET("/api/clap/top_queries", AuthMiddleware(), clapTopQueriesHandler)

	// Serve static files from React build
	buildDir := getEnv("FRONTEND_BUILD_DIR", "/app/music-server-frontend/build")
	// If the absolute path used in containers doesn't exist locally, try
	// to fall back to the repo's build or public folders for local dev.
	if _, err := os.Stat(buildDir); os.IsNotExist(err) {
		localBuild := filepath.Join(".", "music-server-frontend", "build")
		if _, err := os.Stat(localBuild); err == nil {
			buildDir = localBuild
		} else {
			localPublic := filepath.Join(".", "music-server-frontend", "public")
			if _, err := os.Stat(localPublic); err == nil {
				buildDir = localPublic
			}
		}
	}

	// Log which build directory we're using so it's easy to verify in dev
	log.Printf("Using frontend build directory: %s", buildDir)

	// Serve static subdirectory (may be absent in some dev setups)
	r.Static("/static", filepath.Join(buildDir, "static"))
	// Favicon endpoint: serve favicon.ico if present, otherwise fall back to audiomuseai.png
	// This handles cases where browsers request /favicon.ico even if the HTML links to a PNG favicon.
	r.GET("/favicon.ico", func(c *gin.Context) {
		fav := filepath.Join(buildDir, "favicon.ico")
		if stat, err := os.Stat(fav); err == nil && !stat.IsDir() {
			// Set explicit MIME and caching headers
			if t := mime.TypeByExtension(filepath.Ext(fav)); t != "" {
				c.Header("Content-Type", t)
			} else {
				c.Header("Content-Type", "application/octet-stream")
			}
			c.Header("Cache-Control", "public, max-age=86400")
			log.Printf("Serving favicon file: %s to %s", fav, c.ClientIP())
			c.File(fav)
			return
		}
		png := filepath.Join(buildDir, "audiomuseai.png")
		if stat, err := os.Stat(png); err == nil && !stat.IsDir() {
			if t := mime.TypeByExtension(filepath.Ext(png)); t != "" {
				c.Header("Content-Type", t)
			} else {
				c.Header("Content-Type", "application/octet-stream")
			}
			c.Header("Cache-Control", "public, max-age=86400")
			log.Printf("Serving favicon fallback (PNG): %s to %s", png, c.ClientIP())
			c.File(png)
			return
		}
		log.Printf("Favicon not found in buildDir: %s", buildDir)
		c.Status(http.StatusNotFound)
	})
	// Ensure HEAD requests return the same headers for tools like curl -I
	r.HEAD("/favicon.ico", func(c *gin.Context) {
		fav := filepath.Join(buildDir, "favicon.ico")
		if stat, err := os.Stat(fav); err == nil && !stat.IsDir() {
			if t := mime.TypeByExtension(filepath.Ext(fav)); t != "" {
				c.Header("Content-Type", t)
			}
			c.Header("Cache-Control", "public, max-age=86400")
			c.Status(http.StatusOK)
			return
		}
		png := filepath.Join(buildDir, "audiomuseai.png")
		if stat, err := os.Stat(png); err == nil && !stat.IsDir() {
			if t := mime.TypeByExtension(filepath.Ext(png)); t != "" {
				c.Header("Content-Type", t)
			}
			c.Header("Cache-Control", "public, max-age=86400")
			c.Status(http.StatusOK)
			return
		}
		c.Status(http.StatusNotFound)
	})
	// Manifest may exist as a file in the build dir
	r.StaticFile("/manifest.json", filepath.Join(buildDir, "manifest.json"))

	// Explicit route for the logo file so we always return the correct MIME
	r.GET("/audiomuseai.png", func(c *gin.Context) {
		png := filepath.Join(buildDir, "audiomuseai.png")
		log.Printf("Logo handler: checking for file at %s", png)
		if stat, err := os.Stat(png); err == nil && !stat.IsDir() {
			c.Header("Content-Type", "image/png")
			c.Header("Cache-Control", "public, max-age=86400")
			log.Printf("Serving logo file: %s to %s", png, c.ClientIP())
			c.File(png)
			return
		}
		log.Printf("Logo handler: file not found: %s", png)
		c.Status(http.StatusNotFound)
	})
	// Ensure HEAD requests return the same headers for tools like curl -I
	r.HEAD("/audiomuseai.png", func(c *gin.Context) {
		png := filepath.Join(buildDir, "audiomuseai.png")
		if stat, err := os.Stat(png); err == nil && !stat.IsDir() {
			c.Header("Content-Type", "image/png")
			c.Header("Cache-Control", "public, max-age=86400")
			c.Status(http.StatusOK)
			return
		}
		c.Status(http.StatusNotFound)
	})

	// SPA fallback: serve existing files from the chosen buildDir if present
	// otherwise return index.html for client-side routing. Avoids registering
	// a catch-all route that would conflict with existing API prefixes (/rest, /api).
	r.NoRoute(func(c *gin.Context) {
		// Don't serve index.html for API or Subsonic routes
		if strings.HasPrefix(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/rest/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
			return
		}

		// If requesting root, serve index.html
		if c.Request.URL.Path == "/" {
			c.File(filepath.Join(buildDir, "index.html"))
			return
		}

		// Try to serve a static file from the build dir if it exists
		fpath := filepath.Join(buildDir, c.Request.URL.Path)
		if stat, err := os.Stat(fpath); err == nil && !stat.IsDir() {
			// Set explicit MIME and caching headers for static assets
			if t := mime.TypeByExtension(filepath.Ext(fpath)); t != "" {
				c.Header("Content-Type", t)
			}
			c.Header("Cache-Control", "public, max-age=86400")
			log.Printf("Serving static file: %s to %s", fpath, c.ClientIP())
			c.File(fpath)
			return
		}

		// Fallback to index.html for SPA routes
		c.File(filepath.Join(buildDir, "index.html"))
	})

	// Configure server with HTTP/2 h2c (cleartext) support for multiplexing
	h2s := &http2.Server{}

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           h2c.NewHandler(r, h2s), // Wrap handler with h2c for HTTP/2 without TLS
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1MB
	}

	log.Println("[GIN-debug] Listening and serving HTTP on :8080")
	log.Println("[HTTP/2] h2c (HTTP/2 Cleartext) enabled - multiplexing active for parallel cover art requests")
	log.Println("[HTTP/2] Multiple cover art images can now be downloaded simultaneously over a single connection")

	// Setup graceful shutdown on SIGINT/SIGTERM
	stopSig := make(chan os.Signal, 1)
	signal.Notify(stopSig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-stopSig
		log.Printf("Signal %s received, initiating graceful shutdown...", sig)
		isScanCancelled.Store(true)
		if scheduler != nil {
			scheduler.Stop()
		}
		// Give background tasks a short grace period to stop
		grace := 30 * time.Second
		log.Printf("Waiting up to %s for background tasks to stop...", grace)
		time.Sleep(grace)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
		// Final DB checkpoint and close
		if _, err := db.Exec("PRAGMA wal_checkpoint(FULL)"); err != nil {
			log.Printf("Final WAL checkpoint failed: %v", err)
		}
		if err := db.Close(); err != nil {
			log.Printf("Error closing DB: %v", err)
		}
		os.Exit(0)
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
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

// startDBMaintenance runs periodic WAL checkpoint and integrity checks and optionally writes backups.
// Config via env: DB_MAINTENANCE_INTERVAL_MIN (default 10), DB_BACKUP_DIR (optional)
func startDBMaintenance(db *sql.DB, dbPath string) {
	intervalStr := getEnv("DB_MAINTENANCE_INTERVAL_MIN", "10")
	intervalMin, err := strconv.Atoi(intervalStr)
	if err != nil || intervalMin <= 0 {
		intervalMin = 10
	}
	backupDir := getEnv("DB_BACKUP_DIR", "")
	// Enable periodic backup writes only when DB_PERIODIC_BACKUP_ENABLED=true (default: disabled)
	backupEnabled := (getEnv("DB_PERIODIC_BACKUP_ENABLED", "false") == "true")
	ticker := time.NewTicker(time.Duration(intervalMin) * time.Minute)
	go func() {
		for range ticker.C {
			log.Println("DB maintenance: running WAL checkpoint and integrity_check")
			if _, err := db.Exec("PRAGMA wal_checkpoint(FULL)"); err != nil {
				log.Printf("WAL checkpoint failed: %v", err)
			}
			var integrity string
			if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
				log.Printf("Integrity check failed: %v", err)
			} else {
				if integrity != "ok" {
					log.Printf("Integrity check reported issues: %s", integrity)
				} else {
					log.Println("Integrity check OK")
				}
			}
			if !backupEnabled {
				// Periodic backup disabled by default; skip writing backup this tick
				continue
			}
			// Single rotating backup strategy: write one backup file and overwrite it on each run.
			// If DB_BACKUP_DIR is set we write backup there, otherwise use the same directory as DB.
			var backupPath string
			if backupDir != "" {
				if err := os.MkdirAll(backupDir, 0755); err != nil {
					log.Printf("Could not create backup dir: %v", err)
					continue
				}
				backupPath = filepath.Join(backupDir, filepath.Base(dbPath)+"_backup")
			} else {
				backupPath = filepath.Join(filepath.Dir(dbPath), filepath.Base(dbPath)+"_backup")
			}
			// ensure checkpoint before copy
			if _, err := db.Exec("PRAGMA wal_checkpoint(FULL)"); err != nil {
				log.Printf("WAL checkpoint before backup failed: %v", err)
				// still try to copy; checkpoint failure is not fatal here
			}
			tmpBackup := backupPath + ".tmp"
			if err := copyFile(dbPath, tmpBackup); err != nil {
				log.Printf("DB backup failed: %v", err)
				_ = os.Remove(tmpBackup)
			} else {
				if err := os.Rename(tmpBackup, backupPath); err != nil {
					log.Printf("Failed to rename temp backup to final: %v", err)
				} else {
					log.Printf("DB backup saved to %s (rotated single file)", backupPath)
				}
			}
		}
	}()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// checkAndRestoreDB verifies DB readability on startup and, if the DB is not readable, attempts to restore
// from a single rotating backup file after validating the backup's integrity. The backup file is named
// <basename>_backup in the DB directory (or in DB_BACKUP_DIR if set).
func checkAndRestoreDB(dbPath string) error {
	// First try a lightweight read to determine if the DB is readable.
	tempDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		log.Printf("Could not open DB file for read test: %v", err)
		// If we cannot even open the DB, attempt restore below
	} else {
		defer tempDB.Close()

		// Try a simple query that requires the schema to be readable
		var tblName string
		err = tempDB.QueryRow("SELECT name FROM sqlite_master WHERE type='table' LIMIT 1").Scan(&tblName)
		if err == nil {
			// DB is readable; run integrity_check but do not auto-restore on integrity failures (just warn)
			var integrity string
			if err := tempDB.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
				log.Printf("Integrity check query failed: %v", err)
			} else {
				if integrity == "ok" {
					log.Println("DB integrity check OK")
					return nil
				}
				// DB is readable but integrity_check reports issues. Log and continue without restoring.
				log.Printf("DB integrity check reported issues at startup: %s (will not auto-restore)", integrity)
				return nil
			}
		} else {
			// If the DB is empty (no tables), treat this as a fresh install and allow initialization
			if err == sql.ErrNoRows {
				log.Println("DB appears to be empty (no tables); treating as fresh DB and will initialize.")
				return nil
			}
			log.Printf("DB read test failed: %v", err)
			// fall through to attempt restore from backup
		}
	}

	// At this point the DB is not readable -> attempt restore from backup
	backupDir := getEnv("DB_BACKUP_DIR", "")
	var backupPath string
	if backupDir != "" {
		backupPath = filepath.Join(backupDir, filepath.Base(dbPath)+"_backup")
	} else {
		backupPath = filepath.Join(filepath.Dir(dbPath), filepath.Base(dbPath)+"_backup")
	}

	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("DB is not readable and no backup found at %s", backupPath)
	}

	// Sanity-check the backup before restoring
	backupDB, err := sql.Open("sqlite3", backupPath+"?_journal_mode=WAL")
	if err != nil {
		return fmt.Errorf("could not open backup for sanity check: %v", err)
	}
	defer backupDB.Close()

	var backupIntegrity string
	if err := backupDB.QueryRow("PRAGMA integrity_check").Scan(&backupIntegrity); err != nil {
		return fmt.Errorf("backup integrity_check failed: %v", err)
	}
	if backupIntegrity != "ok" {
		return fmt.Errorf("backup integrity_check not ok: %s", backupIntegrity)
	}

	log.Printf("Backup sanity check OK; attempting to restore database from backup: %s", backupPath)

	tmpRestore := dbPath + ".restore_tmp"
	if err := copyFile(backupPath, tmpRestore); err != nil {
		_ = os.Remove(tmpRestore)
		return fmt.Errorf("failed to copy backup to temp: %v", err)
	}
	if err := os.Rename(tmpRestore, dbPath); err != nil {
		_ = os.Remove(tmpRestore)
		return fmt.Errorf("failed to rename restored file: %v", err)
	}

	// Re-open global DB replacing previous handle
	if db != nil {
		_ = db.Close()
	}
	newDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return fmt.Errorf("failed to reopen DB after restore: %v", err)
	}
	db = newDB

	// Re-apply PRAGMAs
	if _, err := db.Exec("PRAGMA synchronous = FULL"); err != nil {
		log.Printf("Warning: could not set PRAGMA synchronous after restore: %v", err)
	}
	if _, err := db.Exec("PRAGMA wal_autocheckpoint = 1000"); err != nil {
		log.Printf("Warning: could not set PRAGMA wal_autocheckpoint after restore: %v", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		log.Printf("Warning: could not set PRAGMA busy_timeout after restore: %v", err)
	}

	// Verify restored DB integrity
	var integrity2 string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&integrity2); err != nil {
		return fmt.Errorf("integrity_check after restore failed: %v", err)
	}
	if integrity2 != "ok" {
		return fmt.Errorf("integrity_check after restore not ok: %s", integrity2)
	}

	log.Println("Database successfully restored from backup and verified OK")
	return nil
}

// performBackup creates a single rotating backup file (basename + _backup).
// It performs a WAL checkpoint, copies the DB to a tmp file and renames it atomically.
func performBackup(db *sql.DB, dbPath string) error {
	// Ensure only one backup runs at once
	backupMu.Lock()
	defer backupMu.Unlock()

	backupDir := getEnv("DB_BACKUP_DIR", "")
	var backupPath string
	if backupDir != "" {
		backupPath = filepath.Join(backupDir, filepath.Base(dbPath)+"_backup")
	} else {
		backupPath = filepath.Join(filepath.Dir(dbPath), filepath.Base(dbPath)+"_backup")
	}
	log.Printf("DB backup: writing rotating backup to %s", backupPath)
	// Try to checkpoint the WAL to ensure DB file is as up-to-date as possible
	if _, err := db.Exec("PRAGMA wal_checkpoint(FULL)"); err != nil {
		log.Printf("WAL checkpoint before backup failed: %v", err)
	}
	tmp := backupPath + ".tmp"
	if err := copyFile(dbPath, tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("DB backup copy failed: %v", err)
	}
	if err := os.Rename(tmp, backupPath); err != nil {
		return fmt.Errorf("DB backup rename failed: %v", err)
	}
	log.Printf("DB backup saved to %s (rotated single file)", backupPath)
	return nil
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

	// Songs table - use TEXT for id to support UUID base62
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS songs (
		id TEXT PRIMARY KEY NOT NULL,
		title TEXT,
		artist TEXT,
		album TEXT,
		album_artist TEXT DEFAULT '',
		path TEXT UNIQUE NOT NULL,
		play_count INTEGER NOT NULL DEFAULT 0,
		last_played TEXT,
		date_added TEXT,
		date_updated TEXT,
		starred INTEGER NOT NULL DEFAULT 0,
		genre TEXT DEFAULT '',
		album_path TEXT DEFAULT '',
		duration INTEGER DEFAULT 0,
		replaygain_track_gain REAL,
		replaygain_track_peak REAL,
		replaygain_album_gain REAL,
		replaygain_album_peak REAL,
		waveform_peaks TEXT,
		cancelled INTEGER NOT NULL DEFAULT 0
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

	// Add duration column if it doesn't exist (backward compatibility)
	_, err = db.Exec(`ALTER TABLE songs ADD COLUMN duration INTEGER NOT NULL DEFAULT 0;`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		log.Printf("Note: Could not add duration column (may already exist): %v", err)
	}

	// Add album_path column if it doesn't exist - stores directory path for grouping
	_, err = db.Exec(`ALTER TABLE songs ADD COLUMN album_path TEXT DEFAULT '';`)
	if err != nil && !strings.Contains(err.Error(), "duplicate column name") {
		log.Printf("Note: Could not add album_path column (may already exist): %v", err)
	}

	// Populate album_path for existing songs that don't have it set
	log.Println("Checking/Updating album_path for existing songs...")
	rows, err := db.Query("SELECT id, path FROM songs WHERE album_path = '' OR album_path IS NULL")
	if err == nil {
		defer rows.Close()
		updateStmt, _ := db.Prepare("UPDATE songs SET album_path = ? WHERE id = ?")
		if updateStmt != nil {
			defer updateStmt.Close()
			updateCount := 0
			for rows.Next() {
				var id string
				var path string
				if err := rows.Scan(&id, &path); err == nil {
					albumPath := filepath.Dir(path)
					updateStmt.Exec(albumPath, id)
					updateCount++
				}
			}
			log.Printf("Checked album_path for existing songs: updated %d rows", updateCount)
		}
	}

	// Create starred_songs table for user-specific stars
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS starred_songs (
		user_id INTEGER NOT NULL,
		song_id TEXT NOT NULL,
		starred_at TEXT NOT NULL,
		PRIMARY KEY (user_id, song_id),
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY(song_id) REFERENCES songs(id) ON DELETE CASCADE
	);`)
	if err != nil {
		log.Fatalf("Failed to create starred_songs table: %v", err)
	}

	// Create starred_albums table for user-specific album stars
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS starred_albums (
		user_id INTEGER NOT NULL,
		album_id TEXT NOT NULL,
		starred_at TEXT NOT NULL,
		PRIMARY KEY (user_id, album_id),
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`)
	if err != nil {
		log.Fatalf("Failed to create starred_albums table: %v", err)
	}

	// Create starred_artists table for user-specific artist stars
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS starred_artists (
		user_id INTEGER NOT NULL,
		artist_name TEXT NOT NULL,
		starred_at TEXT NOT NULL,
		PRIMARY KEY (user_id, artist_name),
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`)
	if err != nil {
		log.Fatalf("Failed to create starred_artists table: %v", err)
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
		song_id TEXT NOT NULL,
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

	// Create play_history table for Recently Played tracking (matches migration)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS play_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		song_id TEXT NOT NULL,
		played_at TEXT NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY(song_id) REFERENCES songs(id) ON DELETE CASCADE
	);`)
	if err != nil {
		log.Fatalf("Failed to create play_history table: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_play_history_user_played ON play_history (user_id, played_at DESC);`)
	if err != nil {
		log.Fatalf("Failed to create play_history index: %v", err)
	}

	// Create transcoding_settings table (matches migration)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS transcoding_settings (
		user_id INTEGER PRIMARY KEY NOT NULL,
		enabled INTEGER NOT NULL DEFAULT 0,
		format TEXT NOT NULL DEFAULT 'mp3',
		bitrate INTEGER NOT NULL DEFAULT 128,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`)
	if err != nil {
		log.Fatalf("Failed to create transcoding_settings table: %v", err)
	}

	// Create radio_stations table for Radio feature (matches migration)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS radio_stations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		seed_songs TEXT NOT NULL,
		temperature REAL NOT NULL DEFAULT 1.0,
		subtract_distance REAL NOT NULL DEFAULT 0.3,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);`)
	if err != nil {
		log.Fatalf("Failed to create radio_stations table: %v", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_radio_stations_user ON radio_stations (user_id);`)
	if err != nil {
		log.Fatalf("Failed to create radio_stations index: %v", err)
	}

	// Default admin user - only create on fresh DB (no users present)
	var userCount int
	row := db.QueryRow("SELECT COUNT(*) FROM users")
	if err := row.Scan(&userCount); err == nil && userCount == 0 {
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
				// Perform pre-scan backup synchronously; skip scan on failure
				dbPath := getEnv("DATABASE_PATH", "/config/music.db")
				if err := performBackup(db, dbPath); err != nil {
					log.Printf("Scheduled pre-scan backup failed: %v - skipping scheduled scan", err)
					return
				}
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
