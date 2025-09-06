// Suggested path: music-server-backend/subsonic_handlers.go
package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

const subsonicVersion = "1.16.1"

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
