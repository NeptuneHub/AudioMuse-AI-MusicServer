package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// AudioMuseClient is the centralized client for all AudioMuse-AI API calls.
// It handles URL resolution, authentication token management, HTTP client lifecycle,
// and consistent 401 error handling across all endpoints.
type AudioMuseClient struct {
	db         *sql.DB
	httpClient *http.Client
}

// ErrAudioMuse401 is returned by Get and Post when AudioMuse-AI responds with 401.
// Callers should handle this by returning a 503 or appropriate error response.
var ErrAudioMuse401 = errors.New("audiomuse-ai: authentication failed (401)")

// NewAudioMuseClient creates a new AudioMuse-AI client with a single HTTP client
// (30-second timeout for all calls).
func NewAudioMuseClient(db *sql.DB) *AudioMuseClient {
	return &AudioMuseClient{
		db: db,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// baseURL resolves the AudioMuse-AI base URL from environment variables or database config.
// Priority: env AUDIOMUSE_AI_CORE_URL > env AUDIO_MUSE_AI_URL > DB key "audiomuse_ai_core_url"
// Always trims trailing slash. Returns error if no URL is configured.
func (cl *AudioMuseClient) baseURL() (string, error) {
	// Check first env var (from audiomuse_admin_handlers.go's getAudioMuseURL)
	if url, ok := os.LookupEnv("AUDIOMUSE_AI_CORE_URL"); ok && url != "" {
		log.Printf("Using AudioMuse-AI URL from AUDIOMUSE_AI_CORE_URL env var")
		return strings.TrimSuffix(url, "/"), nil
	}

	// Check second env var (from map_handlers.go, alchemy_handlers.go, cleaning_handlers.go)
	if url, ok := os.LookupEnv("AUDIO_MUSE_AI_URL"); ok && url != "" {
		log.Printf("Using AudioMuse-AI URL from AUDIO_MUSE_AI_URL env var")
		return strings.TrimSuffix(url, "/"), nil
	}

	// Fall back to database config
	url, err := GetConfig(cl.db, "audiomuse_ai_core_url")
	if err == sql.ErrNoRows {
		return "", errors.New("AudioMuse-AI Core URL not configured (set AUDIOMUSE_AI_CORE_URL or AUDIO_MUSE_AI_URL env var, or 'audiomuse_ai_core_url' in database config)")
	}
	if err != nil {
		return "", err
	}
	if url == "" {
		return "", errors.New("AudioMuse-AI Core URL is empty")
	}

	log.Printf("Using AudioMuse-AI URL from database config")
	return strings.TrimSuffix(url, "/"), nil
}

// token resolves the AudioMuse-AI API token from environment variables or database config.
// Priority: env AUDIO_MUSE_AI_TOKEN > DB key "audiomuse_ai_api_token"
// Returns ("", nil) if no token is configured — that is not an error.
func (cl *AudioMuseClient) token() (string, error) {
	// Check environment variable
	if token, ok := os.LookupEnv("AUDIO_MUSE_AI_TOKEN"); ok && token != "" {
		log.Printf("Using AudioMuse-AI API token from AUDIO_MUSE_AI_TOKEN env var")
		return token, nil
	}

	// Fall back to database config
	token, err := GetConfig(cl.db, "audiomuse_ai_api_token")
	if err == sql.ErrNoRows {
		// Not configured; return empty string, not an error
		return "", nil
	}
	if err != nil {
		return "", err
	}

	if token != "" {
		log.Printf("Using AudioMuse-AI API token from database config")
	}
	return token, nil
}

// buildRequest creates an HTTP request to AudioMuse-AI with the Authorization header attached.
func (cl *AudioMuseClient) buildRequest(ctx context.Context, method, targetURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
	if err != nil {
		return nil, err
	}

	// Attach authorization if a token is configured
	token, err := cl.token()
	if err != nil {
		log.Printf("Error retrieving AudioMuse-AI token: %v", err)
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return req, nil
}

// Get performs a GET request to the given path with optional query parameters.
// Returns (body, statusCode, error). On 401 from AudioMuse-AI, returns ErrAudioMuse401.
func (cl *AudioMuseClient) Get(ctx context.Context, path string, queryParams url.Values) ([]byte, int, error) {
	baseURL, err := cl.baseURL()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to resolve AudioMuse-AI base URL: %v", err)
	}

	// Build full URL with query parameters
	fullURL := baseURL + path
	if len(queryParams) > 0 {
		fullURL = fullURL + "?" + queryParams.Encode()
	}

	req, err := cl.buildRequest(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build request: %v", err)
	}

	resp, err := cl.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to contact AudioMuse-AI: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %v", err)
	}

	// Translate 401 to a sentinel error
	if resp.StatusCode == http.StatusUnauthorized {
		return body, resp.StatusCode, ErrAudioMuse401
	}

	return body, resp.StatusCode, nil
}

// Post performs a POST request with the given body.
// Returns (body, statusCode, error). On 401 from AudioMuse-AI, returns ErrAudioMuse401.
func (cl *AudioMuseClient) Post(ctx context.Context, path string, body io.Reader) ([]byte, int, error) {
	baseURL, err := cl.baseURL()
	if err != nil {
		return nil, 0, fmt.Errorf("failed to resolve AudioMuse-AI base URL: %v", err)
	}

	fullURL := baseURL + path

	req, err := cl.buildRequest(ctx, "POST", fullURL, body)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to build request: %v", err)
	}

	resp, err := cl.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to contact AudioMuse-AI: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response: %v", err)
	}

	// Translate 401 to a sentinel error
	if resp.StatusCode == http.StatusUnauthorized {
		return respBody, resp.StatusCode, ErrAudioMuse401
	}

	return respBody, resp.StatusCode, nil
}

// ProxyGin forwards the current gin request to AudioMuse-AI and writes the response back.
// This is a full pass-through proxy that copies Content-Type and Accept headers from the
// incoming request. On 401 from AudioMuse-AI, writes HTTP 503 with an error message and
// returns immediately (never proxies the 401 to the frontend).
func (cl *AudioMuseClient) ProxyGin(c *gin.Context, method, path string) {
	baseURL, err := cl.baseURL()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fullURL := baseURL + path

	// Preserve query string from original request
	if c.Request.URL.RawQuery != "" {
		fullURL = fullURL + "?" + c.Request.URL.RawQuery
	}

	req, err := cl.buildRequest(c.Request.Context(), method, fullURL, c.Request.Body)
	if err != nil {
		log.Printf("Error creating request to AudioMuse-AI: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Copy Content-Type and Accept headers from the incoming request
	req.Header.Set("Content-Type", c.GetHeader("Content-Type"))
	req.Header.Set("Accept", c.GetHeader("Accept"))

	resp, err := cl.httpClient.Do(req)
	if err != nil {
		log.Printf("Error calling AudioMuse-AI: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to contact AudioMuse-AI Core"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response from AudioMuse-AI: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response from AudioMuse-AI Core"})
		return
	}

	// CRITICAL: Do NOT proxy 401 errors from AudioMuse-AI back to frontend
	// A 401 from AudioMuse-AI (third-party service) does NOT mean the user's session is invalid
	if resp.StatusCode == http.StatusUnauthorized {
		log.Printf("AudioMuse-AI returned 401 - API token likely not configured or invalid")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AudioMuse-AI authentication failed. Please configure API token in Admin settings."})
		return
	}

	// Pass through the response with all original headers
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}
