package main

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// Subsonic clients send the username+password (or a bcrypt-hashed credential) on
// EVERY request. Verifying it with bcrypt (cost 14, ~0.5-2s) on every request
// makes each API call take seconds and serializes under load. This cache stores
// the result of a successful password verification for a short time, keyed by a
// SHA-256 of (username, presented-credential), so bcrypt runs at most once per
// credential per TTL instead of once per request — the same approach Navidrome
// uses. Only successful verifications are cached, and the key includes the exact
// presented credential, so a wrong password can never hit a cached entry.
var (
	authCacheMu  sync.RWMutex
	authCacheMap = map[string]authCacheEntry{}
)

type authCacheEntry struct {
	user    User
	expires time.Time
}

const authCacheTTL = 10 * time.Minute

func authCacheKey(username, credential string) string {
	h := sha256.Sum256([]byte(username + "\x00" + credential))
	return hex.EncodeToString(h[:])
}

// authCacheLookup returns the cached user for a previously-verified credential.
func authCacheLookup(username, credential string) (User, bool) {
	key := authCacheKey(username, credential)
	authCacheMu.RLock()
	e, ok := authCacheMap[key]
	authCacheMu.RUnlock()
	if !ok || time.Now().After(e.expires) {
		return User{}, false
	}
	return e.user, true
}

// authCacheStore records a successful verification.
func authCacheStore(username, credential string, u User) {
	key := authCacheKey(username, credential)
	authCacheMu.Lock()
	// Opportunistically evict expired entries to keep the map bounded.
	if len(authCacheMap) > 1024 {
		now := time.Now()
		for k, v := range authCacheMap {
			if now.After(v.expires) {
				delete(authCacheMap, k)
			}
		}
	}
	authCacheMap[key] = authCacheEntry{user: u, expires: time.Now().Add(authCacheTTL)}
	authCacheMu.Unlock()
}

// invalidateAuthCache clears all cached credentials (e.g. after a password change).
func invalidateAuthCache() {
	authCacheMu.Lock()
	authCacheMap = map[string]authCacheEntry{}
	authCacheMu.Unlock()
}
