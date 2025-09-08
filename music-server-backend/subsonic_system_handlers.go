// Suggested path: music-server-backend/subsonic_system_handlers.go
package main

import (
	"log"

	"github.com/gin-gonic/gin"
)

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

func subsonicSearch(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}
	log.Printf("Subsonic search request received. Returning empty result as placeholder.")
	subsonicRespond(c, newSubsonicResponse(&SubsonicDirectory{Songs: []SubsonicSong{}}))
}

func subsonicTokenInfo(c *gin.Context) {
	subsonicRespond(c, newSubsonicErrorResponse(44, "API Key authentication is not supported by this server."))
}
