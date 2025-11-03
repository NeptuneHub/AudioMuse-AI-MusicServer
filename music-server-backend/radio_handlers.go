package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// createRadioHandler creates a new radio station from alchemy seed
func createRadioHandler(c *gin.Context) {
	userID := c.GetInt("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Name             string  `json:"name"`
		SeedSongs        string  `json:"seed_songs"` // JSON string: [{"id":"123","op":"ADD"},...]
		Temperature      float64 `json:"temperature"`
		SubtractDistance float64 `json:"subtract_distance"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Radio name is required"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)

	result, err := db.Exec(`
		INSERT INTO radio_stations (user_id, name, seed_songs, temperature, subtract_distance, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, userID, req.Name, req.SeedSongs, req.Temperature, req.SubtractDistance, now, now)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create radio station"})
		return
	}

	id, _ := result.LastInsertId()
	c.JSON(http.StatusOK, gin.H{
		"id":                int(id),
		"name":              req.Name,
		"seed_songs":        req.SeedSongs,
		"temperature":       req.Temperature,
		"subtract_distance": req.SubtractDistance,
		"created_at":        now,
		"updated_at":        now,
	})
}

// getRadiosHandler returns all radio stations for the current user
func getRadiosHandler(c *gin.Context) {
	userID := c.GetInt("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	rows, err := db.Query(`
		SELECT id, name, seed_songs, temperature, subtract_distance, created_at, updated_at
		FROM radio_stations
		WHERE user_id = ?
		ORDER BY updated_at DESC
	`, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch radio stations"})
		return
	}
	defer rows.Close()

	var radios []RadioStation
	for rows.Next() {
		var r RadioStation
		if err := rows.Scan(&r.ID, &r.Name, &r.SeedSongs, &r.Temperature, &r.SubtractDistance, &r.CreatedAt, &r.UpdatedAt); err != nil {
			continue
		}
		r.UserID = userID
		radios = append(radios, r)
	}

	if radios == nil {
		radios = []RadioStation{}
	}

	c.JSON(http.StatusOK, gin.H{"radios": radios})
}

// deleteRadioHandler deletes a radio station
func deleteRadioHandler(c *gin.Context) {
	userID := c.GetInt("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	radioID := c.Param("id")

	result, err := db.Exec(`DELETE FROM radio_stations WHERE id = ? AND user_id = ?`, radioID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete radio station"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Radio station not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// updateRadioNameHandler renames a radio station
func updateRadioNameHandler(c *gin.Context) {
	userID := c.GetInt("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	radioID := c.Param("id")

	var req struct {
		Name string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)

	result, err := db.Exec(`
		UPDATE radio_stations 
		SET name = ?, updated_at = ?
		WHERE id = ? AND user_id = ?
	`, req.Name, now, radioID, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update radio station"})
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Radio station not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "name": req.Name})
}

// getRadioSeedHandler returns the radio station seed data for running alchemy
func getRadioSeedHandler(c *gin.Context) {
	userID := c.GetInt("userID")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	radioID := c.Param("id")

	var radio RadioStation
	err := db.QueryRow(`
		SELECT id, name, seed_songs, temperature, subtract_distance
		FROM radio_stations
		WHERE id = ? AND user_id = ?
	`, radioID, userID).Scan(&radio.ID, &radio.Name, &radio.SeedSongs, &radio.Temperature, &radio.SubtractDistance)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Radio station not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                radio.ID,
		"name":              radio.Name,
		"seed_songs":        radio.SeedSongs,
		"temperature":       radio.Temperature,
		"subtract_distance": radio.SubtractDistance,
	})
}
