// Suggested path: music-server-backend/subsonic_playlist_handlers.go
package main

import (
	"database/sql"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
)

func subsonicGetPlaylists(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	rows, err := db.Query(`
		SELECT p.id, p.name, u.username, COUNT(ps.song_id)
		FROM playlists p
		JOIN users u ON p.user_id = u.id
		LEFT JOIN playlist_songs ps ON p.id = ps.playlist_id
		WHERE p.user_id = ?
		GROUP BY p.id, p.name, u.username
		ORDER BY p.name`, user.ID)

	if err != nil {
		log.Printf("Error querying playlists for user %d: %v", user.ID, err)
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error"))
		return
	}
	defer rows.Close()

	var playlists []SubsonicPlaylist
	for rows.Next() {
		var p SubsonicPlaylist
		if err := rows.Scan(&p.ID, &p.Name, &p.Owner, &p.SongCount); err != nil {
			log.Printf("Error scanning playlist row: %v", err)
			continue
		}
		playlists = append(playlists, p)
	}

	subsonicRespond(c, newSubsonicResponse(&SubsonicPlaylists{Playlists: playlists}))
}

func subsonicGetPlaylist(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	playlistID := c.Query("id")
	if playlistID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter 'id' is missing."))
		return
	}

	var playlistName string
	err := db.QueryRow("SELECT name FROM playlists WHERE id = ? AND user_id = ?", playlistID, user.ID).Scan(&playlistName)
	if err != nil {
		if err == sql.ErrNoRows {
			subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found or not owned by user."))
		} else {
			subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		}
		return
	}

	rows, err := db.Query(`
		SELECT s.id, s.title, s.artist, s.album, s.path, s.play_count, s.last_played
		FROM songs s
		JOIN playlist_songs ps ON s.id = ps.song_id
		WHERE ps.playlist_id = ?`, playlistID)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Internal server error querying songs."))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var songFromDb Song
		var lastPlayed sql.NullString
		if err := rows.Scan(&songFromDb.ID, &songFromDb.Title, &songFromDb.Artist, &songFromDb.Album, &songFromDb.Path, &songFromDb.PlayCount, &lastPlayed); err != nil {
			log.Printf("Error scanning song row for getPlaylist: %v", err)
			continue
		}

		subsonicSong := SubsonicSong{
			ID:        strconv.Itoa(songFromDb.ID),
			Title:     songFromDb.Title,
			Artist:    songFromDb.Artist,
			Album:     songFromDb.Album,
			Path:      songFromDb.Path,
			PlayCount: songFromDb.PlayCount,
		}
		if lastPlayed.Valid {
			subsonicSong.LastPlayed = lastPlayed.String
		}
		songs = append(songs, subsonicSong)
	}

	directory := SubsonicDirectory{ID: playlistID, Name: playlistName, SongCount: len(songs), Songs: songs}
	subsonicRespond(c, newSubsonicResponse(&directory))
}

func subsonicCreatePlaylist(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	playlistName := c.Query("name")
	if playlistName == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter 'name' is missing."))
		return
	}

	songIDs := c.QueryArray("songId")

	tx, err := db.Begin()
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error starting transaction."))
		return
	}

	res, err := tx.Exec("INSERT INTO playlists (name, user_id) VALUES (?, ?)", playlistName, user.ID)
	if err != nil {
		tx.Rollback()
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to create playlist."))
		return
	}

	newPlaylistID, err := res.LastInsertId()
	if err != nil {
		tx.Rollback()
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to get new playlist ID."))
		return
	}

	if len(songIDs) > 0 {
		stmt, err := tx.Prepare("INSERT INTO playlist_songs (playlist_id, song_id) VALUES (?, ?)")
		if err != nil {
			tx.Rollback()
			subsonicRespond(c, newSubsonicErrorResponse(0, "Database error preparing statement."))
			return
		}
		defer stmt.Close()
		for _, songID := range songIDs {
			stmt.Exec(newPlaylistID, songID)
		}
	}

	if err := tx.Commit(); err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error committing transaction."))
		return
	}

	createdPlaylist := &SubsonicPlaylist{
		ID:        int(newPlaylistID),
		Name:      playlistName,
		Owner:     user.Username,
		SongCount: len(songIDs),
	}

	subsonicRespond(c, newSubsonicResponse(createdPlaylist))
}

func subsonicUpdatePlaylist(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	playlistID := c.Query("playlistId")
	if playlistID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter 'playlistId' is missing."))
		return
	}

	var ownerID int
	err := db.QueryRow("SELECT user_id FROM playlists WHERE id = ?", playlistID).Scan(&ownerID)
	if err != nil || ownerID != user.ID {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found or you don't own it."))
		return
	}

	tx, err := db.Begin()
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}

	songsToAdd := c.QueryArray("songIdToAdd")
	if len(songsToAdd) > 0 {
		stmt, err := tx.Prepare("INSERT OR IGNORE INTO playlist_songs (playlist_id, song_id) VALUES (?, ?)")
		if err != nil {
			tx.Rollback()
			subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
			return
		}
		defer stmt.Close()
		for _, songID := range songsToAdd {
			stmt.Exec(playlistID, songID)
		}
	}

	songsToRemove := c.QueryArray("songIdToRemove")
	if len(songsToRemove) > 0 {
		stmt, err := tx.Prepare("DELETE FROM playlist_songs WHERE playlist_id = ? AND song_id = ?")
		if err != nil {
			tx.Rollback()
			subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
			return
		}
		defer stmt.Close()
		for _, songID := range songsToRemove {
			stmt.Exec(playlistID, songID)
		}
	}

	newName := c.Query("name")
	if newName != "" {
		_, err := tx.Exec("UPDATE playlists SET name = ? WHERE id = ?", newName, playlistID)
		if err != nil {
			tx.Rollback()
			subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to update playlist name."))
			return
		}
	}

	if err := tx.Commit(); err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}

	subsonicRespond(c, newSubsonicResponse(nil))
}

func subsonicDeletePlaylist(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	playlistID := c.Query("id")
	if playlistID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter 'id' is missing."))
		return
	}

	var ownerID int
	err := db.QueryRow("SELECT user_id FROM playlists WHERE id = ?", playlistID).Scan(&ownerID)
	if err != nil || ownerID != user.ID {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found or you don't own it."))
		return
	}

	tx, err := db.Begin()
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}

	_, err = tx.Exec("DELETE FROM playlist_songs WHERE playlist_id = ?", playlistID)
	if err != nil {
		tx.Rollback()
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to delete playlist songs."))
		return
	}

	_, err = tx.Exec("DELETE FROM playlists WHERE id = ?", playlistID)
	if err != nil {
		tx.Rollback()
		subsonicRespond(c, newSubsonicErrorResponse(0, "Failed to delete playlist."))
		return
	}

	if err := tx.Commit(); err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error."))
		return
	}

	subsonicRespond(c, newSubsonicResponse(nil))
}
