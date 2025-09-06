// Suggested path: music-server-backend/main.go
package main

import (
	"database/sql"
	"log"
	"net/http"

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
		rest.GET("/getSongs.view", subsonicGetSongs)
		rest.GET("/stream.view", subsonicStream)
		// Add other subsonic routes here as needed
	}

	// JSON API routes for the web UI
	v1 := r.Group("/api/v1")
	{
		userRoutes := v1.Group("/user")
		{
			// userRoutes.POST("/register", registerUser) // Should we allow public registration?
			userRoutes.POST("/login", loginUser)
		}

		// Authenticated routes
		musicRoutes := v1.Group("/music")
		musicRoutes.Use(AuthMiddleware())
		{
			musicRoutes.GET("/artists", getArtists)
			musicRoutes.GET("/albums", getAlbums)
			musicRoutes.GET("/songs", getSongs)
			musicRoutes.GET("/stream/:songID", streamSong)
		}

		playlistRoutes := v1.Group("/playlists")
		playlistRoutes.Use(AuthMiddleware())
		{
			playlistRoutes.GET("", getPlaylists)
			playlistRoutes.POST("", createPlaylist)
			playlistRoutes.POST("/:id/songs", addSongToPlaylist)
		}

		// Admin routes
		adminRoutes := v1.Group("/admin")
		adminRoutes.Use(AuthMiddleware(), adminOnly())
		{
			adminRoutes.POST("/library/scan", scanLibrary)
			adminRoutes.GET("/browse", browseFiles)

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
			is_admin BOOLEAN
		);
	`)
	if err != nil {
		log.Fatal("Failed to create users table:", err)
	}

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
		_, err := db.Exec("INSERT INTO users (username, password_hash, is_admin) VALUES (?, ?, ?)", "admin", hashedPassword, true)
		if err != nil {
			log.Println("Could not create default admin user:", err)
		} else {
			log.Println("Default admin user created with password 'admin'")
		}
	}
}
