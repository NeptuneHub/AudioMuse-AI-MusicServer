// Suggested path: music-server-backend/handlers.go
package main

import (
	"database/sql"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// --- Data Structures ---

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	IsAdmin  bool   `json:"is_admin"`
}

type Song struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
	Path   string `json:"-"` // Don't expose path in JSON
}

type Album struct {
	Name   string `json:"name"`
	Artist string `json:"artist"`
}

type Playlist struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// --- User Handlers (JSON API) ---

func loginUser(c *gin.Context) {
	var user User
	var storedUser User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	row := db.QueryRow("SELECT id, username, password_hash, is_admin FROM users WHERE username = ?", user.Username)
	err := row.Scan(&storedUser.ID, &storedUser.Username, &storedUser.Password, &storedUser.IsAdmin)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		log.Printf("!!! DATABASE SCAN ERROR during login for user '%s': %v", user.Username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if !checkPasswordHash(user.Password, storedUser.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, err := GenerateJWT(storedUser.Username, storedUser.IsAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": token, "is_admin": storedUser.IsAdmin})
}

// --- Password Hashing ---

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// --- Music Library Handlers (JSON API) ---

func getArtists(c *gin.Context) {
	rows, err := db.Query("SELECT DISTINCT artist FROM songs WHERE artist != '' ORDER BY artist")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query artists"})
		return
	}
	defer rows.Close()

	var artists []string
	for rows.Next() {
		var artist string
		if err := rows.Scan(&artist); err != nil {
			log.Printf("Error scanning artist row: %v", err)
			continue
		}
		artists = append(artists, artist)
	}
	c.JSON(http.StatusOK, artists)
}

func getAlbums(c *gin.Context) {
	artistFilter := c.Query("artist")
	query := "SELECT DISTINCT album, artist FROM songs WHERE album != ''"
	args := []interface{}{}

	if artistFilter != "" {
		query += " AND artist = ?"
		args = append(args, artistFilter)
	}
	query += " ORDER BY artist, album"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query albums"})
		return
	}
	defer rows.Close()

	var albums []Album
	for rows.Next() {
		var album Album
		if err := rows.Scan(&album.Name, &album.Artist); err != nil {
			log.Printf("Error scanning album row: %v", err)
			continue
		}
		albums = append(albums, album)
	}
	c.JSON(http.StatusOK, albums)
}

func getSongs(c *gin.Context) {
	albumFilter := c.Query("album")
	artistFilter := c.Query("artist") // Used to disambiguate albums with the same name

	query := "SELECT id, title, artist, album FROM songs"
	conditions := []string{}
	args := []interface{}{}

	if albumFilter != "" {
		conditions = append(conditions, "album = ?")
		args = append(args, albumFilter)
	}
	if artistFilter != "" {
		conditions = append(conditions, "artist = ?")
		args = append(args, artistFilter)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY artist, album, title"

	rows, err := db.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query songs"})
		return
	}
	defer rows.Close()

	var songs []Song
	for rows.Next() {
		var song Song
		if err := rows.Scan(&song.ID, &song.Title, &song.Artist, &song.Album); err != nil {
			log.Printf("Error scanning song row: %v", err)
			continue
		}
		songs = append(songs, song)
	}
	c.JSON(http.StatusOK, songs)
}

func streamSong(c *gin.Context) {
	songID := c.Param("songID")
	var path string
	err := db.QueryRow("SELECT path FROM songs WHERE id = ?", songID).Scan(&path)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Song not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.File(path)
}

// --- Playlist Handlers (JSON API) ---

func getPlaylists(c *gin.Context) {
	// Placeholder
	c.JSON(http.StatusOK, []gin.H{})
}

func createPlaylist(c *gin.Context) {
	// Placeholder
	c.JSON(http.StatusCreated, gin.H{"message": "Playlist created"})
}

func addSongToPlaylist(c *gin.Context) {
	// Placeholder
	c.JSON(http.StatusOK, gin.H{"message": "Song added to playlist"})
}

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

type FileItem struct {
	Name string `json:"name"`
	Type string `json:"type"`
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

	if userID == 1 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot delete the primary admin user"})
		return
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

// --- Admin Middleware ---

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

// --- Subsonic API Implementation ---

const subsonicVersion = "1.16.1"

// SubsonicResponse is the top-level wrapper for all Subsonic API responses.
type SubsonicResponse struct {
	XMLName xml.Name `xml:"subsonic-response"`
	Status  string   `xml:"status,attr"`
	Version string   `xml:"version,attr"`
	Xmlns   string   `xml:"xmlns,attr"`
	Body    interface{}
}

// SubsonicError represents an error message in a Subsonic response.
type SubsonicError struct {
	XMLName xml.Name `xml:"error"`
	Code    int      `xml:"code,attr"`
	Message string   `xml:"message,attr"`
}

// SubsonicLicense represents the license status.
type SubsonicLicense struct {
	XMLName xml.Name `xml:"license"`
	Valid   bool     `xml:"valid,attr"`
}

// SubsonicDirectory represents a container for songs.
type SubsonicDirectory struct {
	XMLName   xml.Name       `xml:"directory"`
	SongCount int            `xml:"songCount,attr"`
	Songs     []SubsonicSong `xml:"song"`
}

// SubsonicSong represents a single song.
type SubsonicSong struct {
	XMLName xml.Name `xml:"song"`
	ID      int      `xml:"id,attr"`
	Title   string   `xml:"title,attr"`
	Artist  string   `xml:"artist,attr"`
	Album   string   `xml:"album,attr"`
}

func newSubsonicResponse(body interface{}) SubsonicResponse {
	return SubsonicResponse{
		Status:  "ok",
		Version: subsonicVersion,
		Xmlns:   "http://subsonic.org/restapi",
		Body:    body,
	}
}

func newSubsonicErrorResponse(code int, message string) SubsonicResponse {
	return SubsonicResponse{
		Status:  "failed",
		Version: subsonicVersion,
		Xmlns:   "http://subsonic.org/restapi",
		Body:    &SubsonicError{Code: code, Message: message},
	}
}

// subsonicAuthenticate checks username and password from query params.
func subsonicAuthenticate(c *gin.Context) (User, bool) {
	username := c.Query("u")
	password := c.Query("p")
	// Note: We are ignoring token-based auth (t & s params) for now
	// as it requires more complex handling with our bcrypt password storage.
	// Plain password auth is sufficient for most clients.

	if username == "" || password == "" {
		return User{}, false
	}

	var storedUser User
	var passwordHash string
	row := db.QueryRow("SELECT id, username, password_hash, is_admin FROM users WHERE username = ?", username)
	err := row.Scan(&storedUser.ID, &storedUser.Username, &passwordHash, &storedUser.IsAdmin)
	if err != nil {
		return User{}, false
	}

	if !checkPasswordHash(password, passwordHash) {
		return User{}, false
	}

	return storedUser, true
}

func subsonicPing(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		c.XML(http.StatusOK, newSubsonicErrorResponse(40, "Wrong username or password."))
		return
	}
	c.XML(http.StatusOK, newSubsonicResponse(nil))
}

func subsonicGetLicense(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		c.XML(http.StatusOK, newSubsonicErrorResponse(40, "Wrong username or password."))
		return
	}
	// We are a free, open-source server, so the license is always valid.
	c.XML(http.StatusOK, newSubsonicResponse(&SubsonicLicense{Valid: true}))
}

func subsonicGetSongs(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		c.XML(http.StatusOK, newSubsonicErrorResponse(40, "Wrong username or password."))
		return
	}

	rows, err := db.Query("SELECT id, title, artist, album FROM songs ORDER BY artist, album, title")
	if err != nil {
		c.XML(http.StatusOK, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var song Song
		if err := rows.Scan(&song.ID, &song.Title, &song.Artist, &song.Album); err != nil {
			log.Printf("Error scanning song row for Subsonic: %v", err)
			continue
		}
		songs = append(songs, SubsonicSong{
			ID:     song.ID,
			Title:  song.Title,
			Artist: song.Artist,
			Album:  song.Album,
		})
	}

	directory := SubsonicDirectory{
		SongCount: len(songs),
		Songs:     songs,
	}

	c.XML(http.StatusOK, newSubsonicResponse(&directory))
}

func subsonicStream(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		c.XML(http.StatusOK, newSubsonicErrorResponse(40, "Wrong username or password."))
		return
	}
	songID := c.Query("id")
	var path string
	err := db.QueryRow("SELECT path FROM songs WHERE id = ?", songID).Scan(&path)
	if err != nil {
		if err == sql.ErrNoRows {
			c.XML(http.StatusOK, newSubsonicErrorResponse(70, "The requested data was not found."))
			return
		}
		c.XML(http.StatusOK, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}

	c.File(path)
}

