// Suggested path: music-server-backend/subsonic_music_handlers.go
package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/dhowden/tag"
	"github.com/gin-gonic/gin"
)

func subsonicGetArtists(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	rows, err := db.Query("SELECT DISTINCT artist FROM songs WHERE artist != '' ORDER BY artist")
	if err != nil {
		log.Printf("[ERROR] subsonicGetArtists: Database query failed: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var artists []SubsonicArtist
	for rows.Next() {
		var artistName string
		if err := rows.Scan(&artistName); err != nil {
			log.Printf("Error scanning artist row for Subsonic: %v", err)
			continue
		}
		artists = append(artists, SubsonicArtist{ID: artistName, Name: artistName})
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicArtists{Artists: artists}))
}

func subsonicGetAlbumList2(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	listType := c.DefaultQuery("type", "alphabeticalByName")
	sizeStr := c.DefaultQuery("size", "500")
	offsetStr := c.DefaultQuery("offset", "0")

	size, _ := strconv.Atoi(sizeStr)
	offset, _ := strconv.Atoi(offsetStr)

	log.Printf("[DEBUG] subsonicGetAlbumList2: Running query with type '%s', size %d, offset %d", listType, size, offset)

	var query string
	switch listType {
	case "newest":
		// This query correctly gets the distinct albums ordered by the most recent date_added for any song in that album.
		query = `
			SELECT s.album, s.artist, MIN(s.id) as albumId, MAX(s.date_added) AS max_date
			FROM songs s
			WHERE s.album != ''
			GROUP BY s.album, s.artist
			ORDER BY max_date DESC
			LIMIT ? OFFSET ?`
	// Add other cases like 'random', 'frequent', 'recent' here if needed.
	default: // alphabeticalByName
		query = `
			SELECT album, artist, MIN(id) as albumId
			FROM songs
			WHERE album != ''
			GROUP BY album, artist
			ORDER BY album
			LIMIT ? OFFSET ?`
	}

	rows, err := db.Query(query, size, offset)
	if err != nil {
		log.Printf("[ERROR] subsonicGetAlbumList2: Database query failed: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var albums []SubsonicAlbum
	for rows.Next() {
		var albumName, artistName string
		var albumId int
		var dummyDate sql.NullString // To scan the max_date or be ignored
		if listType == "newest" {
			if err := rows.Scan(&albumName, &artistName, &albumId, &dummyDate); err != nil {
				log.Printf("Error scanning newest album row for Subsonic: %v", err)
				continue
			}
		} else {
			if err := rows.Scan(&albumName, &artistName, &albumId); err != nil {
				log.Printf("Error scanning alphabetical album row for Subsonic: %v", err)
				continue
			}
		}
		albumIDStr := strconv.Itoa(albumId)
		albums = append(albums, SubsonicAlbum{
			ID:       albumIDStr,
			Name:     albumName,
			Artist:   artistName,
			CoverArt: albumIDStr,
		})
	}
	log.Printf("[DEBUG] subsonicGetAlbumList2: Found %d albums.", len(albums))
	subsonicRespond(c, newSubsonicResponse(&SubsonicAlbumList2{Albums: albums}))
}

func subsonicGetAlbum(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	albumUIDStr := c.Query("id")
	albumUID, err := strconv.Atoi(albumUIDStr)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter 'id' must be a number."))
		return
	}

	log.Printf("[DEBUG] subsonicGetAlbum: Fetching songs for album UID: %d", albumUID)

	// First, find the album name associated with this UID.
	var albumName string
	err = db.QueryRow("SELECT album FROM songs WHERE id = ?", albumUID).Scan(&albumName)
	if err != nil {
		if err == sql.ErrNoRows {
			subsonicRespond(c, newSubsonicErrorResponse(70, "Album not found."))
		} else {
			log.Printf("[ERROR] subsonicGetAlbum: Database error finding album name for UID %d: %v", albumUID, err)
			subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		}
		return
	}

	rows, err := db.Query("SELECT id, title, artist, album, path, play_count, last_played FROM songs WHERE album = ? ORDER BY title", albumName)
	if err != nil {
		log.Printf("[ERROR] subsonicGetAlbum: Database query failed for songs in album '%s': %v", albumName, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var songFromDb Song
		var lastPlayed sql.NullString
		if err := rows.Scan(&songFromDb.ID, &songFromDb.Title, &songFromDb.Artist, &songFromDb.Album, &songFromDb.Path, &songFromDb.PlayCount, &lastPlayed); err != nil {
			log.Printf("Error scanning song row for Subsonic getAlbum: %v", err)
			continue
		}

		subsonicSong := SubsonicSong{
			ID:        strconv.Itoa(songFromDb.ID),
			Title:     songFromDb.Title,
			Artist:    songFromDb.Artist,
			Album:     songFromDb.Album,
			Path:      songFromDb.Path,
			PlayCount: songFromDb.PlayCount,
			CoverArt:  albumUIDStr,
		}
		if lastPlayed.Valid {
			subsonicSong.LastPlayed = lastPlayed.String
		}
		songs = append(songs, subsonicSong)
	}
	log.Printf("[DEBUG] subsonicGetAlbum: Found %d songs for album '%s'.", len(songs), albumName)

	album := SubsonicAlbumWithSongs{
		ID:        albumUIDStr,
		Name:      albumName,
		CoverArt:  albumUIDStr,
		SongCount: len(songs),
		Songs:     songs,
	}
	subsonicRespond(c, newSubsonicResponse(&album))
}

func subsonicGetRandomSongs(c *gin.Context) {
	if _, ok := subsonicAuthenticate(c); !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	sizeStr := c.DefaultQuery("size", "50")
	size, err := strconv.Atoi(sizeStr)
	if err != nil || size <= 0 {
		size = 50
	}

	rows, err := db.Query("SELECT id, title, artist, album, path, play_count, last_played FROM songs ORDER BY RANDOM() LIMIT ?", size)
	if err != nil {
		log.Printf("[ERROR] subsonicGetRandomSongs: Database query failed: %v", err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var songFromDb Song
		var lastPlayed sql.NullString
		if err := rows.Scan(&songFromDb.ID, &songFromDb.Title, &songFromDb.Artist, &songFromDb.Album, &songFromDb.Path, &songFromDb.PlayCount, &lastPlayed); err != nil {
			log.Printf("Error scanning song row for Subsonic getRandomSongs: %v", err)
			continue
		}

		subsonicSong := SubsonicSong{
			ID:        strconv.Itoa(songFromDb.ID),
			Title:     songFromDb.Title,
			Artist:    songFromDb.Artist,
			Album:     songFromDb.Album,
			Path:      songFromDb.Path,
			PlayCount: songFromDb.PlayCount,
			CoverArt:  songFromDb.Album, // This should ideally be a stable album ID
		}
		if lastPlayed.Valid {
			subsonicSong.LastPlayed = lastPlayed.String
		}
		songs = append(songs, subsonicSong)
	}

	directory := SubsonicDirectory{SongCount: len(songs), Songs: songs}
	subsonicRespond(c, newSubsonicResponse(&directory))
}

func subsonicGetCoverArt(c *gin.Context) {
	albumUID, err := strconv.Atoi(c.Query("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	var songPath string
	// Use the album UID, which is the ID of a song in that album, to find a path for the cover art.
	err = db.QueryRow("SELECT path FROM songs WHERE id = ?", albumUID).Scan(&songPath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	file, err := os.Open(songPath)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	defer file.Close()

	meta, err := tag.ReadFrom(file)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	pic := meta.Picture()
	if pic == nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Data(http.StatusOK, pic.MIMEType, pic.Data)
}

func subsonicStream(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}
	songID := c.Query("id")
	log.Printf("[DEBUG] subsonicStream: User '%s' requesting stream for song ID: %s", user.Username, songID)

	var path string
	err := db.QueryRow("SELECT path FROM songs WHERE id = ?", songID).Scan(&path)
	if err != nil {
		log.Printf("[ERROR] subsonicStream: Failed to find path for song ID %s. Error: %v", songID, err)
		subsonicRespond(c, newSubsonicErrorResponse(70, "The requested data was not found."))
		return
	}

	log.Printf("[DEBUG] subsonicStream: Found path '%s' for song ID %s. Attempting to serve file.", path, songID)

	// Update play count and last played time
	go func() {
		_, err := db.Exec("UPDATE songs SET play_count = play_count + 1, last_played = ? WHERE id = ?", time.Now().Format(time.RFC3339), songID)
		if err != nil {
			log.Printf("[ERROR] Failed to update play count for song ID %s: %v", songID, err)
		}
	}()

	file, err := os.Open(path)
	if err != nil {
		log.Printf("[ERROR] subsonicStream: Could not open file for streaming %s: %v", path, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error: could not open file"))
		return
	}
	defer file.Close()
	fileInfo, _ := file.Stat()
	http.ServeContent(c.Writer, c.Request, fileInfo.Name(), fileInfo.ModTime(), file)
}
