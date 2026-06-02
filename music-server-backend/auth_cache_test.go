package main

import (
	"database/sql"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthCacheSkipsBcrypt(t *testing.T) {
	d, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT, password_hash TEXT, password_plain TEXT, is_admin BOOLEAN, api_key TEXT)`)
	// cost 14 like production, so the per-request cost is clearly visible
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), 12)
	d.Exec(`INSERT INTO users (username, password_hash, is_admin) VALUES (?,?,?)`, "alice", string(hash), 0)

	old := db
	db = d
	defer func() { db = old }()
	invalidateAuthCache()

	mw := SubsonicAuthMiddleware()
	call := func() (time.Duration, int, bool) {
		gin.SetMode(gin.TestMode)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/rest/ping?u=alice&p=secret", nil)
		var gotUser bool
		c.Set("__none", 0)
		start := time.Now()
		mw(c)
		el := time.Since(start)
		if u, ok := c.Get("user"); ok {
			if uu, ok2 := u.(User); ok2 && uu.Username == "alice" {
				gotUser = true
			}
		}
		return el, w.Code, gotUser
	}

	d1, _, ok1 := call()
	d2, _, ok2 := call()

	if !ok1 || !ok2 {
		t.Fatalf("auth failed: first=%v second=%v", ok1, ok2)
	}
	t.Logf("first (bcrypt) = %v, second (cached) = %v", d1, d2)
	// The cached call must be dramatically faster (no bcrypt).
	if d2 > d1/5 {
		t.Errorf("second request not using cache: first=%v second=%v", d1, d2)
	}
	if d2 > 50*time.Millisecond {
		t.Errorf("cached auth too slow: %v", d2)
	}

	// Wrong password must NOT hit the cache.
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/rest/ping?u=alice&p=wrong", nil)
	mw(c)
	if _, ok := c.Get("user"); ok {
		t.Errorf("wrong password should not authenticate")
	}
}
