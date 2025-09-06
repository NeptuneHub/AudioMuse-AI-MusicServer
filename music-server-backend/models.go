// Suggested path: music-server-backend/models.go
package main

import "encoding/xml"

// --- Data Structures ---

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	IsAdmin  bool   `json:"is_admin"`
}

type Song struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	Album  string `json:"album"`
	Path   string `json:"-"` // Don't expose path in JSON
}

type Album struct {
	Name   string `json:"name"`
	Artist string `json:"artist"`
}

type Playlist struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type FileItem struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// --- Subsonic Data Structures ---

// SubsonicResponse is the top-level wrapper for all Subsonic API responses.
type SubsonicResponse struct {
	XMLName xml.Name `xml:"subsonic-response"`
	Status  string   `xml:"status,attr"`
	Version string   `xml:"version,attr"`
	Xmlns   string   `xml:"xmlns,attr"`
	Body    interface{}
}

// SubsonicError represents an error message in a Subsonic response.
type SubsonicError struct {
	XMLName xml.Name `xml:"error"`
	Code    int      `xml:"code,attr"`
	Message string   `xml:"message,attr"`
}

// SubsonicLicense represents the license status.
type SubsonicLicense struct {
	XMLName xml.Name `xml:"license"`
	Valid   bool     `xml:"valid,attr"`
}

// SubsonicDirectory represents a container for songs.
type SubsonicDirectory struct {
	XMLName   xml.Name       `xml:"directory"`
	SongCount int            `xml:"songCount,attr"`
	Songs     []SubsonicSong `xml:"song"`
}

// SubsonicSong represents a single song.
type SubsonicSong struct {
	XMLName xml.Name `xml:"song"`
	ID      int      `xml:"id,attr"`
	Title   string   `xml:"title,attr"`
	Artist  string   `xml:"artist,attr"`
	Album   string   `xml:"album,attr"`
}
