// subsonic_helpers.go
package main

import (
	"crypto/md5"
	"encoding/hex"
	"log"
	"net/http"
	"strings"

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
	if response.Status == "failed" {
		// Default to a server error, then refine based on the code
		httpStatus = http.StatusInternalServerError
		if errBody, ok := response.Body.(*SubsonicError); ok {
			switch errBody.Code {
			case 10: // Missing parameter
				httpStatus = http.StatusBadRequest
			case 40: // Wrong username or password
				httpStatus = http.StatusUnauthorized
			case 70: // Data not found
				httpStatus = http.StatusNotFound
			}
		}
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
		case *SubsonicAlbumWithSongs:
			inner["album"] = body
		case *SubsonicTokenInfo:
			inner["tokenInfo"] = body
		case *SubsonicScanStatus:
			inner["scanStatus"] = body
		case *SubsonicUsers:
			inner["users"] = body
		case *SubsonicConfigurations: // <<< FIX: Added this case
			inner["configurations"] = body
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
