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

	"github.com/gin-gonic/gin"
	"github.com/dhowden/tag"
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

// subsonicRespond sends a response in the format requested by the client (XML or JSON).
func subsonicRespond(c *gin.Context, response SubsonicResponse) {
	httpStatus := http.StatusOK

	// If the response is an authentication error, use a 401 status code.
	// While the spec often wraps errors in a 200 OK, modern clients respond better to standard HTTP codes.
	if errBody, ok := response.Body.(*SubsonicError); ok && (errBody.Code == 10 || errBody.Code == 40) { // 10: Required parameter missing, 40: Wrong credentials
		httpStatus = http.StatusUnauthorized
	}

	// Subsonic clients can request JSON or JSONP.
	if c.Query("f") == "json" || c.Query("f") == "jsonp" {
		// Build the inner response object that will be nested inside "subsonic-response"
		inner := gin.H{
			"status":  response.Status,
			"version": response.Version,
		}

		// Add optional ping fields if they are set in the response struct
		if response.Type != "" {
			inner["type"] = response.Type
		}
		if response.ServerVersion != "" {
			inner["serverVersion"] = response.ServerVersion
		}
		if response.OpenSubsonic {
			inner["openSubsonic"] = response.OpenSubsonic
		}

		// Add the body to the inner object, using the correct key for each type.
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
		case *SubsonicDirectory:
			inner["directory"] = body
		case *SubsonicTokenInfo:
			inner["tokenInfo"] = body
		case *SubsonicScanStatus:
			inner["scanStatus"] = body
		case nil:
			// No body, do nothing (e.g., for a successful ping).
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
		// Default to XML if format is not specified or is 'xml'.
		c.XML(httpStatus, response)
	}
}

// subsonicAuthenticate checks username and password from query params.
func subsonicAuthenticate(c *gin.Context) (User, bool) {
	username := c.Query("u")
	password := c.Query("p")
	token := c.Query("t")
	salt := c.Query("s")

	// This server supports both password-based and token-based authentication.
	// Password-based is for clients that support it ("Legacy Login").
	// Token-based is for clients that require it ("New Login").

	// Handle password-based authentication
	if password != "" {
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

	// Handle token-based authentication
	if token != "" && salt != "" {
		var storedUser User
		var passwordPlain string
		// WARNING: This requires fetching the plaintext password from the DB.
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

	// No valid authentication method provided
	return User{}, false
}

func subsonicPing(c *gin.Context) {
	// Per the Subsonic API spec, ping is used to test connectivity and does not
	// require authentication. It should always return a successful response.
	// This allows clients to verify the server URL and capabilities before authenticating.
	response := newSubsonicResponse(nil)
	response.Type = "AudioMuse-AI"
	response.ServerVersion = "0.1.0" // Using a placeholder version
	response.OpenSubsonic = true
	subsonicRespond(c, response)
}

func subsonicGetLicense(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}
	// We are a free, open-source server, so the license is always valid.
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
		// The ID can be the artist name itself for simplicity
		artists = append(artists, SubsonicArtist{
			ID:   artistName,
			Name: artistName,
		})
	}

	subsonicRespond(c, newSubsonicResponse(&SubsonicArtists{Artists: artists}))
}

func subsonicGetAlbumList2(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	// The 'type' parameter is required by the spec (e.g., 'alphabeticalByName')
	// We will return albums sorted by name regardless for this implementation.
	rows, err := db.Query("SELECT DISTINCT album, artist FROM songs WHERE album != '' ORDER BY album")
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
		// The ID can be a combination for uniqueness, though just the name is often fine
		albums = append(albums, SubsonicAlbum{
			ID:     albumName,
			Name:   albumName,
			Artist: artistName,
			CoverArt: albumName, // The ID for getCoverArt is the album name
		})
	}

	subsonicRespond(c, newSubsonicResponse(&SubsonicAlbumList2{Albums: albums}))
}

func subsonicGetPlaylists(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	// For now, this is a placeholder as playlist implementation is minimal.
	// In a full implementation, you would query the playlists for the authenticated user.
	log.Printf("User %s requested playlists. Returning empty list as placeholder.", user.Username)

	playlists := SubsonicPlaylists{
		Playlists: []SubsonicPlaylist{},
	}

	subsonicRespond(c, newSubsonicResponse(&playlists))
}

func subsonicGetAlbum(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	albumID := c.Query("id") // In our case, this is the album name
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
		songs = append(songs, SubsonicSong{
			ID:     song.ID,
			Title:  song.Title,
			Artist: song.Artist,
			Album:  song.Album,
			CoverArt: albumID, // The cover art ID is the album name (albumID)
		})
	}

	// The response for getAlbum is a 'directory' element
	directory := SubsonicDirectory{
		ID:        albumID,
		Name:      albumID,
		CoverArt:  albumID,
		SongCount: len(songs),
		Songs:     songs,
	}

	subsonicRespond(c, newSubsonicResponse(&directory))
}

func subsonicSearch(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}
	// This is a stub. A full implementation would search based on the 'query' param.
	log.Printf("Subsonic search request received. Returning empty result as placeholder.")
	// The response for search3 is a 'searchResult3' element. For now, returning an empty directory is a safe fallback.
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
		songs = append(songs, SubsonicSong{
			ID:       song.ID,
			Title:    song.Title,
			Artist:   song.Artist,
			Album:    song.Album,
			CoverArt: song.Album, // The cover art ID is the album name
		})
	}

	directory := SubsonicDirectory{
		SongCount: len(songs),
		Songs:     songs,
	}

	subsonicRespond(c, newSubsonicResponse(&directory))
}

func subsonicGetCoverArt(c *gin.Context) {
	// No auth check on cover art is common practice and improves client performance.
	// To protect cover art, uncomment the authentication block below.
	/*
		if _, ok := subsonicAuthenticate(c); !ok {
			c.Status(http.StatusUnauthorized)
			return
		}
	*/

	albumID := c.Query("id")
	if albumID == "" || albumID == "undefined" {
		c.Status(http.StatusBadRequest)
		return
	}

	var songPath string
	// Find any song from the album to extract its embedded art.
	err := db.QueryRow("SELECT path FROM songs WHERE album = ? LIMIT 1", albumID).Scan(&songPath)
	if err != nil {
		if err == sql.ErrNoRows {
			c.Status(http.StatusNotFound)
			return
		}
		log.Printf("DB error getting path for cover art for album '%s': %v", albumID, err)
		c.Status(http.StatusInternalServerError)
		return
	}

	file, err := os.Open(songPath)
	if err != nil {
		log.Printf("File open error for cover art for album '%s': %v", albumID, err)
		c.Status(http.StatusInternalServerError)
		return
	}
	defer file.Close()

	meta, err := tag.ReadFrom(file)
	if err != nil {
		// If we can't read tags, we can't get art.
		c.Status(http.StatusNotFound)
		return
	}

	pic := meta.Picture()
	if pic == nil {
		// No embedded picture in this file.
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
		if err == sql.ErrNoRows {
			subsonicRespond(c, newSubsonicErrorResponse(70, "The requested data was not found."))
			return
		}
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}

	file, err := os.Open(path)
	if err != nil {
		log.Printf("Could not open file for streaming %s: %v", path, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error: could not open file"))
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		log.Printf("Could not get file info for streaming %s: %v", path, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error: could not stat file"))
		return
	}

	// http.ServeContent is superior to c.File as it properly handles Range requests,
	// which is critical for seeking and robust playback on many clients, especially mobile.
	http.ServeContent(c.Writer, c.Request, fileInfo.Name(), fileInfo.ModTime(), file)
}

func subsonicTokenInfo(c *gin.Context) {
	// This server does not support API Key authentication.
	// We provide a stub endpoint to prevent 404 errors from clients
	// that probe for this functionality.
	// We return error 44, which means "Invalid or expired token", as it's the closest fit.
	subsonicRespond(c, newSubsonicErrorResponse(44, "API Key authentication is not supported by this server."))
}

func subsonicStartScan(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	var isScanning bool
	err := db.QueryRow("SELECT is_scanning FROM scan_status WHERE id = 1").Scan(&isScanning)
	if err != nil && err != sql.ErrNoRows {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error checking scan status."))
		return
	}

	if !isScanning {
		// Subsonic spec for startScan has no path parameter. A server would use a pre-configured path.
		// We will log this and do nothing, as path selection is handled by the web UI.
		log.Println("Subsonic client requested a scan. This server requires path selection via the web admin panel.")
	} else {
		log.Println("Scan start requested, but a scan is already in progress.")
	}

	// Immediately return the current status
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
		if err == sql.ErrNoRows {
			// This should not happen if initDB is correct, but handle it gracefully.
			subsonicRespond(c, newSubsonicResponse(&SubsonicScanStatus{Scanning: false, Count: 0}))
			return
		}
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error getting scan status."))
		return
	}

	subsonicRespond(c, newSubsonicResponse(&SubsonicScanStatus{
		Scanning: isScanning,
		Count:    songsAdded,
	}))
}
