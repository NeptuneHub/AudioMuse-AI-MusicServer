// Suggested path: music-server-backend/main.go
package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB
var jwtKey = []byte("your_secret_key_change_me") // In production, use an environment variable

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

// --- Auth Types and Functions ---

type Claims struct {
	UserID  int  `json:"user_id"`
	IsAdmin bool `json:"is_admin"`
	jwt.StandardClaims
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// --- Auth Middleware ---

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Bearer token required"})
			c.Abort()
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil {
			if err == jwt.ErrSignatureInvalid {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token signature"})
				c.Abort()
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		if !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("isAdmin", claims.IsAdmin)
		c.Next()
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

// --- Route Handlers ---

func loginUser(c *gin.Context) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&creds); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var id int
	var hashedPassword string
	var isAdmin bool
	err := db.QueryRow("SELECT id, password_hash, is_admin FROM users WHERE username = ?", creds.Username).Scan(&id, &hashedPassword, &isAdmin)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if !checkPasswordHash(creds.Password, hashedPassword) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID:  id,
		IsAdmin: isAdmin,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenString, "is_admin": isAdmin})
}

func scanLibrary(c *gin.Context) {
	var req struct {
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request, path is required"})
		return
	}

	log.Printf("Starting library scan for path: %s", req.Path)

	// This should be a background job in a real app, maybe with status reporting via WebSockets
	go func(libraryPath string) {
		err := filepath.WalkDir(libraryPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				ext := strings.ToLower(filepath.Ext(path))
				if ext == ".mp3" || ext == ".m4a" || ext == ".flac" || ext == ".ogg" {
					file, err := os.Open(path)
					if err != nil {
						log.Printf("Error opening file %s: %v", path, err)
						return nil // continue walking
					}
					defer file.Close()

					meta, err := tag.ReadFrom(file)
					if err != nil {
						log.Printf("Error reading tags from %s: %v", path, err)
						return nil // continue walking
					}

					_, err = db.Exec(`
						INSERT INTO songs (title, artist, album, path)
						VALUES (?, ?, ?, ?)
						ON CONFLICT(path) DO NOTHING;
					`, meta.Title(), meta.Artist(), meta.Album(), path)
					if err != nil {
						log.Printf("Error inserting song %s into DB: %v", path, err)
					}
				}
			}
			return nil
		})

		if err != nil {
			log.Printf("Error scanning library at %s: %v", libraryPath, err)
		} else {
			log.Printf("Finished library scan for path: %s", libraryPath)
		}
	}(req.Path)

	c.JSON(http.StatusAccepted, gin.H{"message": "Library scan started in the background."})
}

type FileItem struct {
	Name string `json:"name"`
	Type string `json:"type"` // "dir" or "file"
}

func browseFiles(c *gin.Context) {
	reqPath := c.DefaultQuery("path", "/")
	cleanPath := filepath.Clean(reqPath)

	if !filepath.IsAbs(cleanPath) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path must be absolute"})
		return
	}

	dirEntries, err := os.ReadDir(cleanPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read directory: %v", err)})
		return
	}

	var items []FileItem
	for _, entry := range dirEntries {
		if entry.IsDir() {
			items = append(items, FileItem{Name: entry.Name(), Type: "dir"})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"path":  cleanPath,
		"items": items,
	})
}

func getUsers(c *gin.Context) {
	rows, err := db.Query("SELECT id, username, is_admin FROM users")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer rows.Close()

	var users []gin.H
	for rows.Next() {
		var id int
		var username string
		var isAdmin bool
		if err := rows.Scan(&id, &username, &isAdmin); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan user row"})
			return
		}
		users = append(users, gin.H{"id": id, "username": username, "is_admin": isAdmin})
	}
	c.JSON(http.StatusOK, users)
}

type CreateUserPayload struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	IsAdmin  bool   `json:"is_admin"`
}

func createUser(c *gin.Context) {
	var payload CreateUserPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := hashPassword(payload.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	_, err = db.Exec("INSERT INTO users (username, password_hash, is_admin) VALUES (?, ?, ?)", payload.Username, hashedPassword, payload.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user, username might already exist"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User created successfully"})
}

type UpdatePasswordPayload struct {
	Password string `json:"password" binding:"required"`
}

func updateUserPassword(c *gin.Context) {
	userID := c.Param("id")
	var payload UpdatePasswordPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := hashPassword(payload.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	_, err = db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", hashedPassword, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated successfully"})
}

func deleteUser(c *gin.Context) {
	userIDstr := c.Param("id")

	// Security: prevent deleting the last admin user
	var adminCount int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE is_admin = 1").Scan(&adminCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error checking admin count"})
		return
	}

	if adminCount <= 1 {
		var isAdmin bool
		err := db.QueryRow("SELECT is_admin FROM users WHERE id = ?", userIDstr).Scan(&isAdmin)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error checking user role"})
			return
		}
		if isAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete the last admin user"})
			return
		}
	}

	// Prevent user from deleting themselves
	currentUserID, _ := c.Get("userID")
	if fmt.Sprintf("%v", currentUserID) == userIDstr {
		c.JSON(http.StatusForbidden, gin.H{"error": "You cannot delete yourself"})
		return
	}

	res, err := db.Exec("DELETE FROM users WHERE id = ?", userIDstr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check affected rows"})
		return
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

// --- Stub Handlers ---

func subsonicPing(c *gin.Context)       { c.XML(http.StatusOK, gin.H{"subsonic-response": gin.H{"status": "ok", "version": "1.16.1"}}) }
func subsonicGetLicense(c *gin.Context) { c.XML(http.StatusOK, gin.H{"subsonic-response": gin.H{"status": "ok", "version": "1.16.1", "license": gin.H{"valid": "true"}}}) }
func subsonicGetSongs(c *gin.Context)   { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
func subsonicStream(c *gin.Context)     { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
func getArtists(c *gin.Context)         { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
func getAlbums(c *gin.Context)          { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
func getSongs(c *gin.Context)           { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
func streamSong(c *gin.Context)         { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
func getPlaylists(c *gin.Context)       { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
func createPlaylist(c *gin.Context)     { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }
func addSongToPlaylist(c *gin.Context)  { c.JSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"}) }

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
