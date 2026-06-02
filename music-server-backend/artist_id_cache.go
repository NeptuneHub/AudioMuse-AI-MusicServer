package main

import (
	"database/sql"
	"sync"
	"time"
)

// Artist IDs are MD5 hashes of the (effective) artist name, so the only way to
// turn an artist ID back into a name is to enumerate the distinct artist names
// and hash each one. Doing that with a fresh `SELECT DISTINCT ... FROM songs`
// on every getArtist / getCoverArt / star / unstar request is O(library) and
// becomes very slow as the collection grows. This cache builds the ID->name map
// once and reuses it, rebuilding lazily when an unknown ID arrives and the cache
// has gone stale (so newly-scanned artists still resolve within the TTL).
var (
	artistIDCacheMu      sync.RWMutex
	artistIDCacheMap     map[string]string
	artistIDCacheBuiltAt time.Time
)

const artistIDCacheTTL = 30 * time.Second

// resolveArtistIDToName returns the artist name for a generated artist ID.
// The boolean is false when the ID does not match any known artist.
func resolveArtistIDToName(db *sql.DB, id string) (string, bool) {
	if id == "" {
		return "", false
	}

	artistIDCacheMu.RLock()
	if artistIDCacheMap != nil {
		if name, ok := artistIDCacheMap[id]; ok {
			artistIDCacheMu.RUnlock()
			return name, true
		}
	}
	fresh := artistIDCacheMap != nil && time.Since(artistIDCacheBuiltAt) <= artistIDCacheTTL
	artistIDCacheMu.RUnlock()

	// Hit nothing and the cache is still fresh: the ID genuinely isn't a known
	// artist, so avoid a pointless rebuild.
	if fresh {
		return "", false
	}

	return rebuildArtistIDCacheAndLookup(db, id)
}

func rebuildArtistIDCacheAndLookup(db *sql.DB, id string) (string, bool) {
	artistIDCacheMu.Lock()
	defer artistIDCacheMu.Unlock()

	// Another goroutine may have rebuilt while we waited for the lock.
	if artistIDCacheMap != nil && time.Since(artistIDCacheBuiltAt) <= artistIDCacheTTL {
		name, ok := artistIDCacheMap[id]
		return name, ok
	}

	rows, err := db.Query(`SELECT DISTINCT CASE
		WHEN album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')
		THEN album_artist ELSE artist END AS artist
		FROM songs
		WHERE ((album_artist IS NOT NULL AND TRIM(album_artist) != '' AND LOWER(TRIM(album_artist)) NOT IN ('unknown','unknown artist')) OR artist != '') AND cancelled = 0`)
	if err != nil {
		if artistIDCacheMap != nil {
			name, ok := artistIDCacheMap[id]
			return name, ok
		}
		return "", false
	}
	defer rows.Close()

	m := make(map[string]string)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		m[GenerateArtistID(name)] = name
	}

	artistIDCacheMap = m
	artistIDCacheBuiltAt = time.Now()

	name, ok := m[id]
	return name, ok
}

// invalidateArtistIDCache forces the next lookup to rebuild from the database.
// Call after a library scan changes the set of artists.
func invalidateArtistIDCache() {
	artistIDCacheMu.Lock()
	artistIDCacheMap = nil
	artistIDCacheMu.Unlock()
}
