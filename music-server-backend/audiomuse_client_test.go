package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestAudioMuseClient_baseURL_EnvVar(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	client := NewAudioMuseClient(db)

	// Test environment variable takes priority
	os.Setenv("AUDIOMUSE_AI_CORE_URL", "http://env-core.local:8000")
	defer os.Unsetenv("AUDIOMUSE_AI_CORE_URL")

	baseURL, err := client.baseURL()
	if err != nil {
		t.Fatalf("baseURL error: %v", err)
	}
	if baseURL != "http://env-core.local:8000" {
		t.Fatalf("expected env var URL, got %s", baseURL)
	}
}

func TestAudioMuseClient_baseURL_DBFallback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create configuration table
	_, _ = db.Exec(`CREATE TABLE configuration (key TEXT PRIMARY KEY, value TEXT)`)

	// Ensure env var is not set
	os.Unsetenv("AUDIOMUSE_AI_CORE_URL")
	os.Unsetenv("AUDIO_MUSE_AI_URL")

	// Set in database
	_, _ = db.Exec(`INSERT INTO configuration (key, value) VALUES (?, ?)`, "audiomuse_ai_core_url", "http://db-core.local:9000")

	client := NewAudioMuseClient(db)
	baseURL, err := client.baseURL()
	if err != nil {
		t.Fatalf("baseURL error: %v", err)
	}
	if baseURL != "http://db-core.local:9000" {
		t.Fatalf("expected db URL, got %s", baseURL)
	}
}

func TestAudioMuseClient_baseURL_TrimsTrailingSlash(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	os.Setenv("AUDIOMUSE_AI_CORE_URL", "http://core.local:8000/")
	defer os.Unsetenv("AUDIOMUSE_AI_CORE_URL")

	client := NewAudioMuseClient(db)
	baseURL, err := client.baseURL()
	if err != nil {
		t.Fatalf("baseURL error: %v", err)
	}
	if baseURL != "http://core.local:8000" {
		t.Fatalf("expected trailing slash trimmed, got %s", baseURL)
	}
}

func TestAudioMuseClient_token_EnvVar(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	os.Setenv("AUDIO_MUSE_AI_TOKEN", "env-token-123")
	defer os.Unsetenv("AUDIO_MUSE_AI_TOKEN")

	client := NewAudioMuseClient(db)
	token, err := client.token()
	if err != nil {
		t.Fatalf("token error: %v", err)
	}
	if token != "env-token-123" {
		t.Fatalf("expected env token, got %s", token)
	}
}

func TestAudioMuseClient_token_DBFallback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create configuration table
	_, _ = db.Exec(`CREATE TABLE configuration (key TEXT PRIMARY KEY, value TEXT)`)

	os.Unsetenv("AUDIO_MUSE_AI_TOKEN")

	_, _ = db.Exec(`INSERT INTO configuration (key, value) VALUES (?, ?)`, "audiomuse_ai_api_token", "db-token-456")

	client := NewAudioMuseClient(db)
	token, err := client.token()
	if err != nil {
		t.Fatalf("token error: %v", err)
	}
	if token != "db-token-456" {
		t.Fatalf("expected db token, got %s", token)
	}
}

func TestAudioMuseClient_Get_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Fatalf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/test" {
			t.Fatalf("expected path /api/test, got %s", r.URL.Path)
		}
		// Check Authorization header
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-token" {
			t.Fatalf("expected Bearer test-token, got %s", auth)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result":"ok"}`))
	}))
	defer server.Close()

	os.Setenv("AUDIOMUSE_AI_CORE_URL", server.URL)
	os.Setenv("AUDIO_MUSE_AI_TOKEN", "test-token")
	defer os.Unsetenv("AUDIOMUSE_AI_CORE_URL")
	defer os.Unsetenv("AUDIO_MUSE_AI_TOKEN")

	client := NewAudioMuseClient(db)
	body, statusCode, err := client.Get(context.Background(), "/api/test", url.Values{})

	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}
	if string(body) != `{"result":"ok"}` {
		t.Fatalf("expected body {\"result\":\"ok\"}, got %s", string(body))
	}
}

func TestAudioMuseClient_Get_401Error(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create mock server that returns 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	os.Setenv("AUDIOMUSE_AI_CORE_URL", server.URL)
	os.Setenv("AUDIO_MUSE_AI_TOKEN", "invalid-token")
	defer os.Unsetenv("AUDIOMUSE_AI_CORE_URL")
	defer os.Unsetenv("AUDIO_MUSE_AI_TOKEN")

	client := NewAudioMuseClient(db)
	_, statusCode, err := client.Get(context.Background(), "/api/test", url.Values{})

	if err != ErrAudioMuse401 {
		t.Fatalf("expected ErrAudioMuse401, got %v", err)
	}
	if statusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", statusCode)
	}
}

func TestAudioMuseClient_Post_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != "test payload" {
			t.Fatalf("expected 'test payload', got %s", string(body))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"created"}`))
	}))
	defer server.Close()

	os.Setenv("AUDIOMUSE_AI_CORE_URL", server.URL)
	os.Setenv("AUDIO_MUSE_AI_TOKEN", "test-token")
	defer os.Unsetenv("AUDIOMUSE_AI_CORE_URL")
	defer os.Unsetenv("AUDIO_MUSE_AI_TOKEN")

	client := NewAudioMuseClient(db)
	body, statusCode, err := client.Post(context.Background(), "/api/test", bytes.NewReader([]byte("test payload")))

	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}
	if string(body) != `{"status":"created"}` {
		t.Fatalf("expected body {\"status\":\"created\"}, got %s", string(body))
	}
}

func TestAudioMuseClient_Post_WithoutToken(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create configuration table
	_, _ = db.Exec(`CREATE TABLE configuration (key TEXT PRIMARY KEY, value TEXT)`)

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check that no Authorization header is present
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Fatalf("expected no Authorization header, got %s", auth)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	os.Setenv("AUDIOMUSE_AI_CORE_URL", server.URL)
	os.Unsetenv("AUDIO_MUSE_AI_TOKEN")
	defer os.Unsetenv("AUDIOMUSE_AI_CORE_URL")

	client := NewAudioMuseClient(db)
	_, statusCode, err := client.Post(context.Background(), "/api/test", bytes.NewReader([]byte("{}")))

	if err != nil {
		t.Fatalf("Post error: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}
}

func TestAudioMuseClient_Get_QueryParams(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create configuration table
	_, _ = db.Exec(`CREATE TABLE configuration (key TEXT PRIMARY KEY, value TEXT)`)

	// Create mock server that checks query params
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("id") != "song123" {
			t.Fatalf("expected id=song123, got %s", r.URL.Query().Get("id"))
		}
		if r.URL.Query().Get("n") != "10" {
			t.Fatalf("expected n=10, got %s", r.URL.Query().Get("n"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	os.Setenv("AUDIOMUSE_AI_CORE_URL", server.URL)
	defer os.Unsetenv("AUDIOMUSE_AI_CORE_URL")

	client := NewAudioMuseClient(db)
	params := url.Values{
		"id": []string{"song123"},
		"n":  []string{"10"},
	}
	_, statusCode, err := client.Get(context.Background(), "/api/test", params)

	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}
}

func TestAudioMuseClient_baseURL_NotConfigured(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Ensure all sources are empty
	os.Unsetenv("AUDIOMUSE_AI_CORE_URL")
	os.Unsetenv("AUDIO_MUSE_AI_URL")

	client := NewAudioMuseClient(db)
	_, err := client.baseURL()

	if err == nil {
		t.Fatalf("expected error when baseURL not configured")
	}
}
