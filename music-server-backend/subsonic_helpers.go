// Suggested path: music-server-backend/subsonic_helpers.go
package main

import (
	"log"
	"net/http"

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
		httpStatus = http.StatusInternalServerError
		if errBody, ok := response.Body.(*SubsonicError); ok {
			switch errBody.Code {
			case 10: // Missing parameter
				httpStatus = http.StatusBadRequest
			case 40, 41, 42, 43, 44: // Auth errors
				httpStatus = http.StatusUnauthorized
			case 70: // Data not found
				httpStatus = http.StatusNotFound
			}
		}
	}

	if c.Query("f") == "json" || c.Query("f") == "jsonp" {
		// Simplified JSON response generation
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

		// Use a map to handle different body types dynamically
		bodyMap := map[string]interface{}{}
		switch body := response.Body.(type) {
		case *SubsonicError:
			inner["error"] = body
		case *SubsonicLicense:
			bodyMap["license"] = body
		case *SubsonicArtists:
			bodyMap["artists"] = body
		case *SubsonicAlbumList2:
			bodyMap["albumList2"] = body
		case *SubsonicPlaylists:
			bodyMap["playlists"] = body
		case *SubsonicPlaylist:
			bodyMap["playlist"] = body
		case *SubsonicDirectory:
			bodyMap["directory"] = body
		case *SubsonicAlbumWithSongs:
			bodyMap["album"] = body
		case *SubsonicScanStatus:
			bodyMap["scanStatus"] = body
		case *SubsonicUsers:
			bodyMap["users"] = body
		case *SubsonicConfigurations:
			bodyMap["configurations"] = body
		case *SubsonicLibraryPaths:
			bodyMap["libraryPaths"] = body
		case *OpenSubsonicExtensions:
			bodyMap["openSubsonicExtensions"] = body.Extensions // Directly embed the slice
		case *ApiKeyResponse:
			bodyMap["apiKey"] = body
		case *SubsonicSongWrapper:
			bodyMap["song"] = body.Song
		case *SubsonicSearchResult2:
			bodyMap["searchResult2"] = body
		case *SubsonicSearchResult3:
			bodyMap["searchResult3"] = body
		case *SubsonicStarred:
			bodyMap["starred"] = body
		case *SubsonicStarred2:
			bodyMap["starred2"] = body
		case *SubsonicSongsByGenre:
			bodyMap["songsByGenre"] = body
		case *SubsonicGenres:
			bodyMap["genres"] = body
		case nil:
			// No body
		default:
			log.Printf("Warning: Unhandled Subsonic body type for JSON response: %T", body)
		}

		// Add non-error body content to the response
		for key, val := range bodyMap {
			inner[key] = val
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
