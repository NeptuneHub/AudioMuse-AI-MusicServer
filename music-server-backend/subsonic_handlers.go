// Suggested path: music-server-backend/subsonic_handlers.go
package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/gin-gonic/gin"
)

const subsonicVersion = "1.16.1"
const subsonicAuthErrorMsg = "Wrong username or password."

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

func subsonicRespond(c *gin.Context, response SubsonicResponse) {
	httpStatus := http.StatusOK
	if errBody, ok := response.Body.(*SubsonicError); ok && (errBody.Code == 10 || errBody.Code == 40) {
		httpStatus = http.StatusUnauthorized
	}

	if c.Query("f") == "json" || c.Query("f") == "jsonp" {
		inner := gin.H{
			"status":  response.Status,
			"version": response.Version,
		}

		if response.Type != "" {
			inner["type"] = response.Type
		}
		if response.ServerVersion != "" {
			inner["serverVersion"] = response.ServerVersion
		}
		if response.OpenSubsonic {
			inner["openSubsonic"] = response.OpenSubsonic
		}

		switch body := response.Body.(type) {
		case *SubsonicError:
			inner["error"] = body
		case *SubsonicLicense:
			inner["license"] = body
		case *SubsonicArtists:
			inner["artists"] = body
		case *SubsonicAlbumList2:
			inner["albumList2"] = body
		case *SubsonicPlaylists:
			inner["playlists"] = body
		case *SubsonicPlaylist:
			inner["playlist"] = body
		case *SubsonicDirectory:
			inner["directory"] = body
		case *SubsonicTokenInfo:
			inner["tokenInfo"] = body
		case *SubsonicScanStatus:
			inner["scanStatus"] = body
		case *SubsonicUsers:
			inner["users"] = body
		case nil:
		default:
			log.Printf("Warning: Unhandled Subsonic body type for JSON response: %T", body)
		}

		finalResponse := gin.H{"subsonic-response": inner}
		if c.Query("f") == "jsonp" && c.Query("callback") != "" {
			c.JSONP(httpStatus, finalResponse)
		} else {
			c.JSON(httpStatus, finalResponse)
		}
	} else {
		c.XML(httpStatus, response)
	}
}

func subsonicAuthenticate(c *gin.Context) (User, bool) {
	// First, check for JWT Bearer token for web UI integration.
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := parseJWT(tokenString)
		if err == nil {
			user := User{ID: claims.UserID, Username: claims.Username, IsAdmin: claims.IsAdmin}
			return user, true
		}
	}

	// Fallback to standard Subsonic authentication methods.
	username := c.Query("u")
	password := c.Query("p")
	token := c.Query("t")
	salt := c.Query("s")

	if password != "" {
		// Handle Navidrome-style hex-encoded password: p=enc:....
		if strings.HasPrefix(password, "enc:") {
			hexEncodedPass := strings.TrimPrefix(password, "enc:")
			decodedPassBytes, err := hex.DecodeString(hexEncodedPass)
			if err != nil {
				return User{}, false // Invalid hex encoding
			}

			var storedUser User
			var storedPasswordPlain string
			// We need to compare against the stored plaintext password.
			row := db.QueryRow("SELECT id, username, password_plain, is_admin FROM users WHERE username = ?", username)
			err = row.Scan(&storedUser.ID, &storedUser.Username, &storedPasswordPlain, &storedUser.IsAdmin)
			if err != nil {
				return User{}, false // User not found or DB error.
			}
			// Compare the decoded password with the stored plaintext password.
			return storedUser, string(decodedPassBytes) == storedPasswordPlain
		}

		// Handle standard plaintext password: p=...
		var storedUser User
		var passwordHash string
		row := db.QueryRow("SELECT id, username, password_hash, is_admin FROM users WHERE username = ?", username)
		err := row.Scan(&storedUser.ID, &storedUser.Username, &passwordHash, &storedUser.IsAdmin)
		if err != nil {
			return User{}, false
		}
		return storedUser, checkPasswordHash(password, passwordHash)
	}

	if token != "" && salt != "" {
		var storedUser User
		var passwordPlain string
		row := db.QueryRow("SELECT id, username, password_plain, is_admin FROM users WHERE username = ?", username)
		err := row.Scan(&storedUser.ID, &storedUser.Username, &passwordPlain, &storedUser.IsAdmin)
		if err != nil || passwordPlain == "" {
			return User{}, false
		}
		hasher := md5.New()
		hasher.Write([]byte(passwordPlain + salt))
		expectedToken := hex.EncodeToString(hasher.Sum(nil))
		return storedUser, token == expectedToken
	}

	return User{}, false
}

func subsonicPing(c *gin.Context) {
	response := newSubsonicResponse(nil)
	response.Type = "AudioMuse-AI"
	response.ServerVersion = "0.1.0"
	response.OpenSubsonic = true
	subsonicRespond(c, response)
}

func subsonicGetLicense(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicLicense{Valid: true}))
}

func subsonicGetArtists(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	rows, err := db.Query("SELECT DISTINCT artist FROM songs WHERE artist != '' ORDER BY artist")
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var artists []SubsonicArtist
	for rows.Next() {
		var artistName string
		if err := rows.Scan(&artistName); err != nil {
			log.Printf("Error scanning artist row for Subsonic: %v", err)
			continue
		}
		artists = append(artists, SubsonicArtist{ID: artistName, Name: artistName})
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicArtists{Artists: artists}))
}

func subsonicGetAlbumList2(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	artistFilter := c.Query("id")
	query := "SELECT DISTINCT album, artist FROM songs WHERE album != ''"
	args := []interface{}{}

	if artistFilter != "" {
		query += " AND artist = ?"
		args = append(args, artistFilter)
	}
	query += " ORDER BY album"

	rows, err := db.Query(query, args...)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var albums []SubsonicAlbum
	for rows.Next() {
		var albumName, artistName string
		if err := rows.Scan(&albumName, &artistName); err != nil {
			log.Printf("Error scanning album row for Subsonic: %v", err)
			continue
		}
		albums = append(albums, SubsonicAlbum{ID: albumName, Name: albumName, Artist: artistName, CoverArt: albumName})
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicAlbumList2{Albums: albums}))
}

func subsonicGetPlaylists(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	rows, err := db.Query(`
		SELECT p.id, p.name, u.username, COUNT(ps.song_id)
		FROM playlists p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN playlist_songs ps ON p.id = ps.playlist_id
		WHERE p.user_id = ?
		GROUP BY p.id, p.name, u.username
		ORDER BY p.name`, user.ID)

	if err != nil {
		log.Printf("Error querying playlists for user %d: %v", user.ID, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var playlists []SubsonicPlaylist
	for rows.Next() {
		var p SubsonicPlaylist
		if err := rows.Scan(&p.ID, &p.Name, &p.Owner, &p.SongCount); err != nil {
			log.Printf("Error scanning playlist row: %v", err)
			continue
		}
		playlists = append(playlists, p)
	}

	subsonicRespond(c, newSubsonicResponse(&SubsonicPlaylists{Playlists: playlists}))
}

func subsonicGetPlaylist(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	playlistID := c.Query("id")
	if playlistID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter 'id' is missing."))
		return
	}

	var playlistName string
	err := db.QueryRow("SELECT name FROM playlists WHERE id = ? AND user_id = ?", playlistID, user.ID).Scan(&playlistName)
	if err != nil {
		if err == sql.ErrNoRows {
			subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found or not owned by user."))
		} else {
			subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		}
		return
	}

	rows, err := db.Query(`
		SELECT s.id, s.title, s.artist, s.album
		FROM songs s
		JOIN playlist_songs ps ON s.id = ps.song_id
		WHERE ps.playlist_id = ?`, playlistID)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error querying songs."))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var song SubsonicSong
		if err := rows.Scan(&song.ID, &song.Title, &song.Artist, &song.Album); err != nil {
			log.Printf("Error scanning song row for getPlaylist: %v", err)
			continue
		}
		songs = append(songs, song)
	}

	directory := SubsonicDirectory{ID: playlistID, Name: playlistName, SongCount: len(songs), Songs: songs}
	subsonicRespond(c, newSubsonicResponse(&directory))
}

func subsonicCreatePlaylist(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	playlistName := c.Query("name")
	if playlistName == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter 'name' is missing."))
		return
	}

	songIDs := c.QueryArray("songId")

	tx, err := db.Begin()
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error starting transaction."))
		return
	}

	res, err := tx.Exec("INSERT INTO playlists (name, user_id) VALUES (?, ?)", playlistName, user.ID)
	if err != nil {
		tx.Rollback()
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to create playlist."))
		return
	}

	newPlaylistID, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to get new playlist ID."))
		return
	}

	if len(songIDs) > 0 {
		stmt, err := tx.Prepare("INSERT INTO playlist_songs (playlist_id, song_id) VALUES (?, ?)")
		if err != nil {
			tx.Rollback()
			subsonicRespond(c, newSubsonicErrorResponse(0, "Database error preparing statement."))
			return
		}
		defer stmt.Close()
		for _, songID := range songIDs {
			stmt.Exec(newPlaylistID, songID)
		}
	}

	if err := tx.Commit(); err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error committing transaction."))
		return
	}

	createdPlaylist := &SubsonicPlaylist{
		ID:        int(newPlaylistID),
		Name:      playlistName,
		Owner:     user.Username,
		SongCount: len(songIDs),
	}

	subsonicRespond(c, newSubsonicResponse(createdPlaylist))
}

func subsonicUpdatePlaylist(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	playlistID := c.Query("playlistId")
	if playlistID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter 'playlistId' is missing."))
		return
	}

	var ownerID int
	err := db.QueryRow("SELECT user_id FROM playlists WHERE id = ?", playlistID).Scan(&ownerID)
	if err != nil || ownerID != user.ID {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found or you don't own it."))
		return
	}

	tx, err := db.Begin()
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}

	songsToAdd := c.QueryArray("songIdToAdd")
	if len(songsToAdd) > 0 {
		stmt, err := tx.Prepare("INSERT OR IGNORE INTO playlist_songs (playlist_id, song_id) VALUES (?, ?)")
		if err != nil {
			tx.Rollback()
			subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
			return
		}
		defer stmt.Close()
		for _, songID := range songsToAdd {
			stmt.Exec(playlistID, songID)
		}
	}

	songsToRemove := c.QueryArray("songIdToRemove")
	if len(songsToRemove) > 0 {
		stmt, err := tx.Prepare("DELETE FROM playlist_songs WHERE playlist_id = ? AND song_id = ?")
		if err != nil {
			tx.Rollback()
			subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
			return
		}
		defer stmt.Close()
		for _, songID := range songsToRemove {
			stmt.Exec(playlistID, songID)
		}
	}

	newName := c.Query("name")
	if newName != "" {
		_, err := tx.Exec("UPDATE playlists SET name = ? WHERE id = ?", newName, playlistID)
		if err != nil {
			tx.Rollback()
			subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to update playlist name."))
			return
		}
	}

	if err := tx.Commit(); err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}

	subsonicRespond(c, newSubsonicResponse(nil))
}

func subsonicDeletePlaylist(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	playlistID := c.Query("id")
	if playlistID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter 'id' is missing."))
		return
	}

	var ownerID int
	err := db.QueryRow("SELECT user_id FROM playlists WHERE id = ?", playlistID).Scan(&ownerID)
	if err != nil || ownerID != user.ID {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found or you don't own it."))
		return
	}

	tx, err := db.Begin()
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}

	_, err = tx.Exec("DELETE FROM playlist_songs WHERE playlist_id = ?", playlistID)
	if err != nil {
		tx.Rollback()
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to delete playlist songs."))
		return
	}

	_, err = tx.Exec("DELETE FROM playlists WHERE id = ?", playlistID)
	if err != nil {
		tx.Rollback()
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to delete playlist."))
		return
	}

	if err := tx.Commit(); err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}

	subsonicRespond(c, newSubsonicResponse(nil))
}

func subsonicGetAlbum(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	albumID := c.Query("id")
	if albumID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter 'id' is missing."))
		return
	}

	rows, err := db.Query("SELECT id, title, artist, album FROM songs WHERE album = ? ORDER BY title", albumID)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var song Song
		if err := rows.Scan(&song.ID, &song.Title, &song.Artist, &song.Album); err != nil {
			log.Printf("Error scanning song row for Subsonic getAlbum: %v", err)
			continue
		}
		songs = append(songs, SubsonicSong{ID: song.ID, Title: song.Title, Artist: song.Artist, Album: song.Album, CoverArt: albumID})
	}

	directory := SubsonicDirectory{ID: albumID, Name: albumID, CoverArt: albumID, SongCount: len(songs), Songs: songs}
	subsonicRespond(c, newSubsonicResponse(&directory))
}

func subsonicSearch(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}
	log.Printf("Subsonic search request received. Returning empty result as placeholder.")
	subsonicRespond(c, newSubsonicResponse(&SubsonicDirectory{Songs: []SubsonicSong{}}))
}

func subsonicGetRandomSongs(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	sizeStr := c.DefaultQuery("size", "50")
	size, err := strconv.Atoi(sizeStr)
	if err != nil || size <= 0 {
		size = 50
	}

	rows, err := db.Query("SELECT id, title, artist, album FROM songs ORDER BY RANDOM() LIMIT ?", size)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var song Song
		if err := rows.Scan(&song.ID, &song.Title, &song.Artist, &song.Album); err != nil {
			log.Printf("Error scanning song row for Subsonic getRandomSongs: %v", err)
			continue
		}
		songs = append(songs, SubsonicSong{ID: song.ID, Title: song.Title, Artist: song.Artist, Album: song.Album, CoverArt: song.Album})
	}

	directory := SubsonicDirectory{SongCount: len(songs), Songs: songs}
	subsonicRespond(c, newSubsonicResponse(&directory))
}

func subsonicGetCoverArt(c *gin.Context) {
	albumID := c.Query("id")
	if albumID == "" || albumID == "undefined" {
		c.Status(http.StatusBadRequest)
		return
	}
	var songPath string
	err := db.QueryRow("SELECT path FROM songs WHERE album = ? LIMIT 1", albumID).Scan(&songPath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	file, err := os.Open(songPath)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	defer file.Close()

	meta, err := tag.ReadFrom(file)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	pic := meta.Picture()
	if pic == nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, pic.MIMEType, pic.Data)
}

func subsonicStream(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}
	songID := c.Query("id")
	var path string
	err := db.QueryRow("SELECT path FROM songs WHERE id = ?", songID).Scan(&path)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(70, "The requested data was not found."))
		return
	}

	file, err := os.Open(path)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error: could not open file"))
		return
	}
	defer file.Close()
	fileInfo, _ := file.Stat()
	http.ServeContent(c.Writer, c.Request, fileInfo.Name(), fileInfo.ModTime(), file)
}

func subsonicTokenInfo(c *gin.Context) {
	subsonicRespond(c, newSubsonicErrorResponse(44, "API Key authentication is not supported by this server."))
}

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

