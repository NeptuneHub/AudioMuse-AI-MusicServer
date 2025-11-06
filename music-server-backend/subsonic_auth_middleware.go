// Suggested path: music-server-backend/subsonic_auth_middleware.go
package main

import (
	"crypto/md5"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log"
	"strings"

	"github.com/gin-gonic/gin"
)

// SubsonicAuthMiddleware creates a gin middleware for handling all subsonic authentication.
// It checks for API Key, user/pass, token/salt, or a valid JWT.
// If authentication is successful, it adds the User object to the context.
// If it fails, it aborts the request with a proper Subsonic error.
func SubsonicAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.Query("apiKey")
		username := c.Query("u")
		password := c.Query("p")
		token := c.Query("t")
		salt := c.Query("s")
		jwtToken := c.Query("jwt") // Support JWT token in query string for <audio> elements
		authHeader := c.GetHeader("Authorization")

		log.Printf("DEBUG: Subsonic auth attempt - URL: %s, apiKey: %t, username: %s, password: %t, token: %t, salt: %t, jwt: %t, authHeader: %s",
			c.Request.URL.Path, apiKey != "", username, password != "", token != "", salt != "", jwtToken != "", authHeader)

		authMethods := 0
		if apiKey != "" {
			authMethods++
		}
		if username != "" && (password != "" || token != "") {
			authMethods++
		}
		if strings.HasPrefix(authHeader, "Bearer ") || jwtToken != "" {
			authMethods++
		}

		// Check for conflicting authentication methods as per OpenSubsonic spec
		if authMethods > 1 {
			subsonicRespond(c, newSubsonicErrorResponse(43, "Multiple conflicting authentication mechanisms provided"))
			c.Abort()
			return
		}

		// 1. API Key Authentication (Highest Priority)
		if apiKey != "" {
			if username != "" {
				subsonicRespond(c, newSubsonicErrorResponse(43, "Username parameter (u) must not be provided when using an API key."))
				c.Abort()
				return
			}
			var user User
			err := db.QueryRow("SELECT id, username, is_admin FROM users WHERE api_key = ?", apiKey).Scan(&user.ID, &user.Username, &user.IsAdmin)
			if err != nil {
				subsonicRespond(c, newSubsonicErrorResponse(40, "Invalid API key."))
				c.Abort()
				return
			}
			c.Set("user", user)
			c.Next()
			return
		}

		// 2. JWT Bearer token (For Web UI integration)
		// Support both Authorization header and query parameter (for <audio> src)
		tokenString := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		} else if jwtToken != "" {
			tokenString = jwtToken
		}

		if tokenString != "" {
			claims, err := parseJWT(tokenString)
			if err == nil {
				user := User{ID: claims.UserID, Username: claims.Username, IsAdmin: claims.IsAdmin}
				c.Set("user", user)
				c.Next()
				return
			} else {
				tokenPreview := tokenString
				if len(tokenString) > 20 {
					tokenPreview = tokenString[:20] + "..."
				}
				log.Printf("ERROR: JWT parsing failed for token: %s, error: %v", tokenPreview, err)
			}
		}

		// 3. Legacy Subsonic Authentication (User/Pass & Token/Salt)
		if username != "" {
			// Plaintext or hex-encoded password check
			if password != "" {
				var storedUser User
				var passwordHash string
				var storedApiKey sql.NullString // Use sql.NullString for potentially NULL api_key

				// Fetch user details first, including the API key
				row := db.QueryRow("SELECT id, username, password_hash, is_admin, COALESCE(api_key, '') FROM users WHERE username = ?", username)
				if err := row.Scan(&storedUser.ID, &storedUser.Username, &passwordHash, &storedUser.IsAdmin, &storedApiKey); err == nil {
					log.Printf("DEBUG: Found user '%s' (ID: %d, IsAdmin: %t)", storedUser.Username, storedUser.ID, storedUser.IsAdmin)

					// Handle hex-encoded password `p=enc:HEXSTRING`
					if strings.HasPrefix(password, "enc:") {
						hexEncoded := strings.TrimPrefix(password, "enc:")
						decodedBytes, err := hex.DecodeString(hexEncoded)
						if err == nil {
							decodedString := string(decodedBytes)

							// NEW: Check if the decoded string is the user's API key
							if storedApiKey.Valid && storedApiKey.String != "" && decodedString == storedApiKey.String {
								log.Printf("DEBUG: Successfully authenticated user '%s' via API key in hex-encoded password", storedUser.Username)
								c.Set("user", storedUser)
								c.Next()
								return
							}

							// Fallback: Check if it's the user's password
							if checkPasswordHash(decodedString, passwordHash) {
								log.Printf("DEBUG: Successfully authenticated user '%s' via hex-encoded password", storedUser.Username)
								c.Set("user", storedUser)
								c.Next()
								return
							} else {
								log.Printf("DEBUG: Hex-encoded password check failed for user '%s'", storedUser.Username)
							}
						} else {
							log.Printf("DEBUG: Failed to decode hex string for user '%s': %v", username, err)
						}
					} else if checkPasswordHash(password, passwordHash) { // Plaintext password check
						log.Printf("DEBUG: Successfully authenticated user '%s' via plaintext password", storedUser.Username)
						c.Set("user", storedUser)
						c.Next()
						return
					} else {
						log.Printf("DEBUG: Plaintext password check failed for user '%s'", storedUser.Username)
					}
				} else {
					log.Printf("DEBUG: User '%s' not found in database or query failed: %v", username, err)
				}
			}

			// Token/Salt check
			if token != "" && salt != "" {
				var storedUser User
				var passwordPlain sql.NullString
				row := db.QueryRow("SELECT id, username, COALESCE(password_plain, ''), is_admin FROM users WHERE username = ?", username)
				if err := row.Scan(&storedUser.ID, &storedUser.Username, &passwordPlain, &storedUser.IsAdmin); err == nil && passwordPlain.Valid && passwordPlain.String != "" {
					hasher := md5.New()
					hasher.Write([]byte(passwordPlain.String + salt))
					expectedToken := hex.EncodeToString(hasher.Sum(nil))
					if token == expectedToken {
						log.Printf("DEBUG: Successfully authenticated user '%s' via token/salt", storedUser.Username)
						c.Set("user", storedUser)
						c.Next()
						return
					} else {
						log.Printf("DEBUG: Token/salt check failed for user '%s' (expected: %s, got: %s)", storedUser.Username, expectedToken, token)
					}
				} else {
					if err != nil {
						log.Printf("DEBUG: Token/salt auth query failed for user '%s': %v", username, err)
					} else if !passwordPlain.Valid || passwordPlain.String == "" {
						log.Printf("DEBUG: Token/salt auth failed for user '%s': no password_plain stored", username)
					}
				}
			}
		}

		// If no valid authentication was found
		log.Printf("DEBUG: Authentication failed for username='%s', password provided=%t, token provided=%t, salt provided=%t",
			username, password != "", token != "", salt != "")
		subsonicRespond(c, newSubsonicErrorResponse(40, "Authentication failed. Please provide valid credentials."))
		c.Abort()
	}
}

// generateSecureApiKey creates a cryptographically secure random hex string.
func generateSecureApiKey() (string, error) {
	bytes := make([]byte, 24) // 24 bytes = 48 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
