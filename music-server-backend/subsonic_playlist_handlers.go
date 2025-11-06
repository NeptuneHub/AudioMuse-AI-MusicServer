// Suggested path: music-server-backend/subsonic_playlist_handlers.go
package main

import (
	"database/sql"
	"log"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func subsonicGetPlaylists(c *gin.Context) {
	user := c.MustGet("user").(User)
	_ = user // Auth is handled by middleware

	// Return playlists owned by the user and also playlists created by admin users (visible to all)
	query := `
		SELECT p.id, p.name, COUNT(CASE WHEN s.cancelled = 0 THEN 1 END), u.username, u.is_admin
		FROM playlists p
		LEFT JOIN playlist_songs ps ON p.id = ps.playlist_id
		LEFT JOIN songs s ON ps.song_id = s.id
		JOIN users u ON u.id = p.user_id
		WHERE p.user_id = ? OR u.is_admin = 1
		GROUP BY p.id, p.name, u.username, u.is_admin
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
		var ownerUsername string
		var ownerIsAdmin bool
		if err := rows.Scan(&p.ID, &p.Name, &p.SongCount, &ownerUsername, &ownerIsAdmin); err != nil {
			log.Printf("Error scanning playlist row: %v", err)
			continue
		}
		p.Owner = ownerUsername
		// Mark playlists created by admin users as public/visible
		p.Public = ownerIsAdmin
		playlists = append(playlists, p)
	}

	subsonicRespond(c, newSubsonicResponse(&SubsonicPlaylists{Playlists: playlists}))
}

func subsonicGetPlaylist(c *gin.Context) {
	user := c.MustGet("user").(User)
	_ = user // Auth is handled by middleware

	playlistID := c.Query("id")
	if playlistID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Missing required parameter 'id'"))
		return
	}

	// Allow viewing the playlist if the requester is the owner OR the playlist was created by an admin
	var playlistName string
	var ownerUsername string
	var ownerIsAdmin bool
	err := db.QueryRow(
		"SELECT p.name, u.username, u.is_admin FROM playlists p JOIN users u ON p.user_id = u.id WHERE p.id = ? AND (p.user_id = ? OR u.is_admin = 1)",
		playlistID, user.ID,
	).Scan(&playlistName, &ownerUsername, &ownerIsAdmin)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found."))
		return
	}

	query := `
		SELECT s.id, s.title, s.artist, s.album, s.play_count, s.last_played, s.duration
		FROM songs s
		JOIN playlist_songs ps ON s.id = ps.song_id
		WHERE ps.playlist_id = ? AND s.cancelled = 0
		ORDER BY ps.position ASC
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
		var duration int
		var lastPlayed sql.NullString
		if err := rows.Scan(&s.ID, &s.Title, &s.Artist, &s.Album, &s.PlayCount, &lastPlayed, &duration); err != nil {
			log.Printf("Error scanning playlist song row: %v", err)
			continue
		}
		s.CoverArt = s.ID
		s.Duration = duration
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
	user := c.MustGet("user").(User)
	_ = user // Auth is handled by middleware

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
		stmt, err := tx.Prepare("INSERT INTO playlist_songs (playlist_id, song_id, position) VALUES (?, ?, ?)")
		if err != nil {
			tx.Rollback()
			subsonicRespond(c, newSubsonicErrorResponse(0, "Error preparing to add songs."))
			return
		}
		defer stmt.Close()

		for i, songID := range songIds {
			if _, err := stmt.Exec(newID, songID, i); err != nil {
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

	response := newSubsonicResponse(&createdPlaylist)
	subsonicRespond(c, response)
}

func subsonicUpdatePlaylist(c *gin.Context) {
	user := c.MustGet("user").(User)
	_ = user // Auth is handled by middleware

	playlistID := c.Query("playlistId")
	newName := c.Query("name")
	songIdsToAdd := c.QueryArray("songIdToAdd")
	songIndicesToRemoveStr := c.QueryArray("songIndexToRemove")

	// Correctly parse comma-separated list for full playlist updates
	songIdParam := c.Query("songId")
	var fullSongIdList []string
	if songIdParam != "" {
		fullSongIdList = strings.Split(songIdParam, ",")
	}

	if playlistID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Missing required parameter 'playlistId'"))
		return
	}

	// Fetch owner and whether the owner is an admin
	var ownerId int
	var ownerIsAdmin bool
	err := db.QueryRow("SELECT p.user_id, u.is_admin FROM playlists p JOIN users u ON p.user_id = u.id WHERE p.id = ?", playlistID).Scan(&ownerId, &ownerIsAdmin)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found or permission denied."))
		return
	}

	// Permission rules:
	// - The playlist owner can update their own playlists.
	// - If the playlist owner is an admin, only other admins may edit/delete it.
	if ownerId != user.ID {
		if ownerIsAdmin && user.IsAdmin {
			// allow: admin editing another admin's playlist
		} else {
			subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found or permission denied."))
			return
		}
	}

	tx, err := db.Begin()
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Database transaction error."))
		return
	}
	defer tx.Rollback()

	if newName != "" {
		_, err := tx.Exec("UPDATE playlists SET name = ? WHERE id = ?", newName, playlistID)
		if err != nil {
			log.Printf("Error renaming playlist %s: %v", playlistID, err)
			subsonicRespond(c, newSubsonicErrorResponse(0, "Error renaming playlist."))
			return
		}
	}

	// If no song modifications are requested, commit potential name change and exit
	if len(fullSongIdList) == 0 && len(songIdsToAdd) == 0 && len(songIndicesToRemoveStr) == 0 {
		if err := tx.Commit(); err != nil {
			subsonicRespond(c, newSubsonicErrorResponse(0, "Error committing playlist changes."))
		} else {
			subsonicRespond(c, newSubsonicResponse(nil))
		}
		return
	}

	var finalSongIds []string

	if len(fullSongIdList) > 0 {
		// This handles full replacement of playlist content (e.g., for reordering)
		finalSongIds = fullSongIdList
	} else {
		// This handles incremental additions/removals
		rows, err := tx.Query("SELECT song_id FROM playlist_songs WHERE playlist_id = ? ORDER BY position ASC", playlistID)
		if err != nil {
			subsonicRespond(c, newSubsonicErrorResponse(0, "Could not fetch current playlist state."))
			return
		}

		var currentSongIds []string
		for rows.Next() {
			var songId string
			if err := rows.Scan(&songId); err != nil {
				rows.Close()
				subsonicRespond(c, newSubsonicErrorResponse(0, "Error reading current playlist state."))
				return
			}
			currentSongIds = append(currentSongIds, songId)
		}
		rows.Close()

		if len(songIndicesToRemoveStr) > 0 {
			indicesToRemove := make(map[int]bool)
			for _, idxStr := range songIndicesToRemoveStr {
				if idx, err := strconv.Atoi(idxStr); err == nil {
					indicesToRemove[idx] = true
				}
			}

			var songsToKeep []string
			for i, songId := range currentSongIds {
				if !indicesToRemove[i] {
					songsToKeep = append(songsToKeep, songId)
				}
			}
			currentSongIds = songsToKeep
		}

		currentSongIds = append(currentSongIds, songIdsToAdd...)
		finalSongIds = currentSongIds
	}

	// Atomically update the playlist songs
	_, err = tx.Exec("DELETE FROM playlist_songs WHERE playlist_id = ?", playlistID)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Error clearing playlist for update."))
		return
	}

	if len(finalSongIds) > 0 {
		stmt, err := tx.Prepare("INSERT INTO playlist_songs (playlist_id, song_id, position) VALUES (?, ?, ?)")
		if err != nil {
			subsonicRespond(c, newSubsonicErrorResponse(0, "Error preparing to update playlist songs."))
			return
		}
		defer stmt.Close()

		for i, songID := range finalSongIds {
			// Ensure songID is not an empty string which can happen from trailing commas
			if songID != "" {
				if _, err := stmt.Exec(playlistID, songID, i); err != nil {
					log.Printf("Error inserting song %s into playlist %s at position %d: %v", songID, playlistID, i, err)
					subsonicRespond(c, newSubsonicErrorResponse(0, "Error inserting song into playlist."))
					return
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Error committing playlist changes."))
		return
	}

	subsonicRespond(c, newSubsonicResponse(nil))
}

func subsonicDeletePlaylist(c *gin.Context) {
	user := c.MustGet("user").(User)
	_ = user // Auth is handled by middleware

	playlistID := c.Query("id")
	if playlistID == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Missing required parameter 'id'"))
		return
	}

	// ON DELETE CASCADE in the schema handles deleting from playlist_songs
	// Check owner and admin status to decide if deletion is allowed
	var ownerId int
	var ownerIsAdmin bool
	err := db.QueryRow("SELECT p.user_id, u.is_admin FROM playlists p JOIN users u ON p.user_id = u.id WHERE p.id = ?", playlistID).Scan(&ownerId, &ownerIsAdmin)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found or permission denied."))
		return
	}

	if ownerId != user.ID {
		// If the playlist was created by an admin, only admins can delete it
		if ownerIsAdmin && user.IsAdmin {
			// allowed
		} else {
			subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found or permission denied."))
			return
		}
	}

	res, err := db.Exec("DELETE FROM playlists WHERE id = ?", playlistID)
	if err != nil {
		subsonicRespond(c, newSubsonicErrorResponse(0, "Error deleting playlist."))
		return
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		subsonicRespond(c, newSubsonicErrorResponse(70, "Playlist not found or permission denied."))
		return
	}

	subsonicRespond(c, newSubsonicResponse(nil))
}
