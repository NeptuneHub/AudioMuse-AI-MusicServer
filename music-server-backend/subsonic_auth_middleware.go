// Suggested path: music-server-backend/subsonic_auth_middleware.go
package main

import (
	"crypto/md5"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
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

		authMethods := 0
		if apiKey != "" {
			authMethods++
		}
		if username != "" && (password != "" || token != "") {
			authMethods++
		}
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
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
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := parseJWT(tokenString)
			if err == nil {
				user := User{ID: claims.UserID, Username: claims.Username, IsAdmin: claims.IsAdmin}
				c.Set("user", user)
				c.Next()
				return
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
				row := db.QueryRow("SELECT id, username, password_hash, is_admin, api_key FROM users WHERE username = ?", username)
				if err := row.Scan(&storedUser.ID, &storedUser.Username, &passwordHash, &storedUser.IsAdmin, &storedApiKey); err == nil {
					// Handle hex-encoded password `p=enc:HEXSTRING`
					if strings.HasPrefix(password, "enc:") {
						hexEncoded := strings.TrimPrefix(password, "enc:")
						decodedBytes, err := hex.DecodeString(hexEncoded)
						if err == nil {
							decodedString := string(decodedBytes)

							// NEW: Check if the decoded string is the user's API key
							if storedApiKey.Valid && decodedString == storedApiKey.String {
								c.Set("user", storedUser)
								c.Next()
								return
							}

							// Fallback: Check if it's the user's password
							if checkPasswordHash(decodedString, passwordHash) {
								c.Set("user", storedUser)
								c.Next()
								return
							}
						}
					} else if checkPasswordHash(password, passwordHash) { // Plaintext password check
						c.Set("user", storedUser)
						c.Next()
						return
					}
				}
			}

			// Token/Salt check
			if token != "" && salt != "" {
				var storedUser User
				var passwordPlain string
				row := db.QueryRow("SELECT id, username, password_plain, is_admin FROM users WHERE username = ?", username)
				if err := row.Scan(&storedUser.ID, &storedUser.Username, &passwordPlain, &storedUser.IsAdmin); err == nil && passwordPlain != "" {
					hasher := md5.New()
					hasher.Write([]byte(passwordPlain + salt))
					expectedToken := hex.EncodeToString(hasher.Sum(nil))
					if token == expectedToken {
						c.Set("user", storedUser)
						c.Next()
						return
					}
				}
			}
		}

		// If no valid authentication was found
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
