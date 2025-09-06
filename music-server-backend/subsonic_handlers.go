// Suggested path: music-server-backend/subsonic_handlers.go
package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

const subsonicVersion = "1.16.1"
const subsonicAuthErrorMsg = "Wrong username or password. This server only supports password-based authentication."

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
	// Subsonic clients can request JSON or JSONP.
	if c.Query("f") == "json" || c.Query("f") == "jsonp" {
		// Build the inner response object that will be nested inside "subsonic-response"
		inner := gin.H{
			"status":  response.Status,
			"version": response.Version,
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
		case nil:
			// No body, do nothing (e.g., for a successful ping).
		default:
			log.Printf("Warning: Unhandled Subsonic body type for JSON response: %T", body)
		}

		finalResponse := gin.H{"subsonic-response": inner}

		if c.Query("f") == "jsonp" && c.Query("callback") != "" {
			c.JSONP(http.StatusOK, finalResponse)
		} else {
			c.JSON(http.StatusOK, finalResponse)
		}
	} else {
		// Default to XML if format is not specified or is 'xml'.
		c.XML(http.StatusOK, response)
	}
}

// subsonicAuthenticate checks username and password from query params.
func subsonicAuthenticate(c *gin.Context) (User, bool) {
	username := c.Query("u")
	password := c.Query("p")

	// The Subsonic API supports two authentication methods:
	// 1. Plain password: `p=password`
	// 2. Token-based: `t=token&s=salt` where token is md5(password + salt).
	//
	// This server uses bcrypt for password storage, which is a one-way hash.
	// We cannot recover the plain password to verify the md5-based token.
	// Therefore, we ONLY support plain password authentication.
	// Clients that attempt token auth will fail here and should fall back to password auth.
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
	// Ping is the first call a client makes. It must be authenticated.
	// By enforcing authentication here, we force clients that default to
	// unsupported token-based auth to fall back to password-based auth immediately.
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}
	subsonicRespond(c, newSubsonicResponse(nil))
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

	c.File(path)
}
