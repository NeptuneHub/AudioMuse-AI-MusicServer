// Suggested path: music-server-backend/subsonic_system_handlers.go
package main

import (
	"database/sql"
	"log"

	"github.com/gin-gonic/gin"
)

// --- Public Endpoints ---

func subsonicPing(c *gin.Context) {
	response := newSubsonicResponse(nil)
	response.Type = "AudioMuse-AI"
	response.ServerVersion = "0.1.0" // Replace with your app version
	response.OpenSubsonic = true
	subsonicRespond(c, response)
}

func subsonicGetOpenSubsonicExtensions(c *gin.Context) {
	extensions := []OpenSubsonicExtension{
		{Name: "apiKeyAuthentication", Versions: []int{1}},
		// Add other supported extensions here
	}
	response := newSubsonicResponse(&OpenSubsonicExtensions{Extensions: extensions})
	response.OpenSubsonic = true
	subsonicRespond(c, response)
}

// --- Authenticated Endpoints ---

func subsonicGetLicense(c *gin.Context) {
	_ = c.MustGet("user").(User) // Authentication is handled by middleware
	subsonicRespond(c, newSubsonicResponse(&SubsonicLicense{Valid: true}))
}

// --- API Key Management ---

func subsonicGetApiKey(c *gin.Context) {
	user := c.MustGet("user").(User)

	var apiKey sql.NullString
	err := db.QueryRow("SELECT api_key FROM users WHERE id = ?", user.ID).Scan(&apiKey)
	if err != nil {
		log.Printf("Error fetching API key for user %d: %v", user.ID, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error fetching API key."))
		return
	}

	if apiKey.Valid && apiKey.String != "" {
		subsonicRespond(c, newSubsonicResponse(&ApiKeyResponse{Key: apiKey.String}))
		return
	}

	// If no key exists, generate, save, and return it.
	newKey, err := generateSecureApiKey()
	if err != nil {
		log.Printf("CRITICAL: Failed to generate secure API key: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to generate new API key."))
		return
	}

	_, err = db.Exec("UPDATE users SET api_key = ? WHERE id = ?", newKey, user.ID)
	if err != nil {
		log.Printf("Error saving new API key for user %d: %v", user.ID, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error saving new API key."))
		return
	}

	log.Printf("Generated new API key for user '%s'", user.Username)
	subsonicRespond(c, newSubsonicResponse(&ApiKeyResponse{Key: newKey}))
}

func subsonicRevokeApiKey(c *gin.Context) {
	user := c.MustGet("user").(User)

	_, err := db.Exec("UPDATE users SET api_key = NULL WHERE id = ?", user.ID)
	if err != nil {
		log.Printf("Error revoking API key for user %d: %v", user.ID, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error revoking API key."))
		return
	}
	log.Printf("Revoked API key for user '%s'", user.Username)
	subsonicRespond(c, newSubsonicResponse(nil)) // Success
}

