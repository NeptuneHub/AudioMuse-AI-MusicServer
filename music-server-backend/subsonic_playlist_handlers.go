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

	query := `
		SELECT p.id, p.name, COUNT(ps.song_id)
		FROM playlists p
		LEFT JOIN playlist_songs ps ON p.id = ps.playlist_id
		WHERE p.user_id = ?
		GROUP BY p.id, p.name
		ORDER BY p.name
	`
	rows, err := db.Query(query, user.ID)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error fetching playlists."))
		return
	}
	defer rows.Close()

	var playlists []SubsonicPlaylist
	for rows.Next() {
		var p SubsonicPlaylist
		if err := rows.Scan(&p.ID, &p.Name, &p.SongCount); err != nil {
			log.Printf("Error scanning playlist row: %v", err)
			continue
		}
		p.Owner = user.Username
		p.Public = false
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
		subsonicRespond(c, newSubsonicErrorResponse(10, "Missing required parameter 'id'"))
		return
	}

	var playlistName string
	err := db.QueryRow("SELECT name FROM playlists WHERE id = ? AND user_id = ?", playlistID, user.ID).Scan(&playlistName)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found."))
		return
	}

	query := `
		SELECT s.id, s.title, s.artist, s.album, s.play_count, s.last_played
		FROM songs s
		JOIN playlist_songs ps ON s.id = ps.song_id
		WHERE ps.playlist_id = ?
	`
	rows, err := db.Query(query, playlistID)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database error fetching playlist songs."))
		return
	}
	defer rows.Close()

	var songs []SubsonicSong
	for rows.Next() {
		var s SubsonicSong
		var songId int
		var lastPlayed sql.NullString
		if err := rows.Scan(&songId, &s.Title, &s.Artist, &s.Album, &s.PlayCount, &lastPlayed); err != nil {
			log.Printf("Error scanning playlist song row: %v", err)
			continue
		}
		s.ID = strconv.Itoa(songId)
		s.CoverArt = s.ID
		if lastPlayed.Valid {
			s.LastPlayed = lastPlayed.String
		}
		songs = append(songs, s)
	}

	responseBody := &SubsonicDirectory{
		ID:        playlistID,
		Name:      playlistName,
		SongCount: len(songs),
		Songs:     songs,
	}
	subsonicRespond(c, newSubsonicResponse(responseBody))
}

func subsonicCreatePlaylist(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	playlistName := c.Query("name")
	if playlistName == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Missing required parameter 'name'"))
		return
	}

	songIds := c.QueryArray("songId")

	tx, err := db.Begin()
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database transaction error."))
		return
	}

	res, err := tx.Exec("INSERT INTO playlists (name, user_id) VALUES (?, ?)", playlistName, user.ID)
	if err != nil {
		tx.Rollback()
		subsonicRespond(c, newSubsonicErrorResponse(0, "Error creating playlist entry."))
		return
	}
	newID, _ := res.LastInsertId()

	if len(songIds) > 0 {
		stmt, err := tx.Prepare("INSERT OR IGNORE INTO playlist_songs (playlist_id, song_id) VALUES (?, ?)")
		if err != nil {
			tx.Rollback()
			subsonicRespond(c, newSubsonicErrorResponse(0, "Error preparing to add songs."))
			return
		}
		defer stmt.Close()

		for _, songID := range songIds {
			if _, err := stmt.Exec(newID, songID); err != nil {
				tx.Rollback()
				log.Printf("Error adding song %s to new playlist %d: %v", songID, newID, err)
				subsonicRespond(c, newSubsonicErrorResponse(0, "Error adding a song to the playlist."))
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Error finalizing playlist creation."))
		return
	}

	createdPlaylist := SubsonicPlaylist{
		ID:        int(newID),
		Name:      playlistName,
		Owner:     user.Username,
		Public:    false,
		SongCount: len(songIds),
	}

	// Respond with the created playlist object, which subsonicRespond will correctly wrap under a "playlist" key for JSON.
	response := newSubsonicResponse(&createdPlaylist)
	subsonicRespond(c, response)
}

func subsonicUpdatePlaylist(c *gin.Context) {
	user, ok := subsonicAuthenticate(c)
	if !ok {
		subsonicRespond(c, newSubsonicErrorResponse(40, subsonicAuthErrorMsg))
		return
	}

	playlistID := c.Query("playlistId")
	songIdsToAdd := c.QueryArray("songIdToAdd")
	// songIndicesToRemove := c.QueryArray("songIndexToRemove") // Not implemented

	if playlistID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Missing required parameter 'playlistId'"))
		return
	}

	var ownerId int
	err := db.QueryRow("SELECT user_id FROM playlists WHERE id = ?", playlistID).Scan(&ownerId)
	if err != nil || ownerId != user.ID {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found or permission denied."))
		return
	}

	tx, err := db.Begin()
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database transaction error."))
		return
	}

	for _, songID := range songIdsToAdd {
		_, err := tx.Exec("INSERT OR IGNORE INTO playlist_songs (playlist_id, song_id) VALUES (?, ?)", playlistID, songID)
		if err != nil {
			tx.Rollback()
			subsonicRespond(c, newSubsonicErrorResponse(0, "Error adding song to playlist."))
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Error committing playlist changes."))
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
		subsonicRespond(c, newSubsonicErrorResponse(10, "Missing required parameter 'id'"))
		return
	}

	tx, err := db.Begin()
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database transaction error."))
		return
	}

	_, err = tx.Exec("DELETE FROM playlist_songs WHERE playlist_id = ?", playlistID)
	if err != nil {
		tx.Rollback()
		subsonicRespond(c, newSubsonicErrorResponse(0, "Error clearing playlist songs."))
		return
	}

	res, err := tx.Exec("DELETE FROM playlists WHERE id = ? AND user_id = ?", playlistID, user.ID)
	if err != nil {
		tx.Rollback()
		subsonicRespond(c, newSubsonicErrorResponse(0, "Error deleting playlist."))
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		tx.Rollback()
		subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found or permission denied."))
		return
	}

	err = tx.Commit()
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Error committing playlist deletion."))
		return
	}

	subsonicRespond(c, newSubsonicResponse(nil))
}

