// Suggested path: music-server-backend/models.go
package main

import (
	"database/sql"
	"encoding/xml"
)

// --- Data Structures ---

type User struct {
	ID       int            `json:"id"`
	Username string         `json:"username"`
	Password string         `json:"password,omitempty"`
	IsAdmin  bool           `json:"is_admin"`
	APIKey   sql.NullString `json:"-"` // Use sql.NullString for the nullable api_key field
}

type Song struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	Path        string `json:"-"` // Don't expose path in JSON
	PlayCount   int    `json:"playCount"`
	LastPlayed  string `json:"lastPlayed"`
	DateAdded   string `json:"dateAdded"`
	DateUpdated string `json:"dateUpdated"`
	Starred     bool   `json:"starred"`
	Genre       string `json:"genre"`
}

type Album struct {
	Name   string `json:"name"`
	Artist string `json:"artist"`
	Genre  string `json:"genre"`
}

type Playlist struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type FileItem struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type LibraryPath struct {
	ID            int    `json:"id"`
	Path          string `json:"path"`
	SongCount     int    `json:"song_count"`
	LastScanEnded string `json:"last_scan_ended"`
}

// --- Subsonic Data Structures ---

type SubsonicResponse struct {
	XMLName       xml.Name `xml:"subsonic-response" json:"-"`
	Status        string   `xml:"status,attr" json:"status"`
	Version       string   `xml:"version,attr" json:"version"`
	Xmlns         string   `xml:"xmlns,attr" json:"xmlns,omitempty"`
	Type          string   `xml:"type,attr,omitempty" json:"type,omitempty"`
	ServerVersion string   `xml:"serverVersion,attr,omitempty" json:"serverVersion,omitempty"`
	OpenSubsonic  bool     `xml:"openSubsonic,attr,omitempty" json:"openSubsonic,omitempty"`
	Body          interface{}
}

// ... (Rest of the Subsonic structs are unchanged)
type SubsonicError struct {
	XMLName xml.Name `xml:"error" json:"-"`
	Code    int      `xml:"code,attr" json:"code"`
	Message string   `xml:"message,attr" json:"message"`
}

type SubsonicLicense struct {
	XMLName xml.Name `xml:"license" json:"-"`
	Valid   bool     `xml:"valid,attr" json:"valid"`
}

type SubsonicDirectory struct {
	XMLName   xml.Name       `xml:"directory" json:"-"`
	ID        string         `xml:"id,attr,omitempty" json:"id,omitempty"`
	Name      string         `xml:"name,attr,omitempty" json:"name,omitempty"`
	CoverArt  string         `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	SongCount int            `xml:"songCount,attr" json:"songCount"`
	Songs     []SubsonicSong `xml:"song" json:"song"`
}

type SubsonicAlbumWithSongs struct {
	XMLName   xml.Name       `xml:"album" json:"-"`
	ID        string         `xml:"id,attr,omitempty" json:"id,omitempty"`
	Name      string         `xml:"name,attr,omitempty" json:"name,omitempty"`
	CoverArt  string         `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	SongCount int            `xml:"songCount,attr" json:"songCount"`
	Songs     []SubsonicSong `xml:"song" json:"song"`
}

type SubsonicSong struct {
	XMLName    xml.Name `xml:"song" json:"-"`
	ID         string   `xml:"id,attr" json:"id"`
	CoverArt   string   `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Title      string   `xml:"title,attr" json:"title"`
	Artist     string   `xml:"artist,attr" json:"artist"`
	Album      string   `xml:"album,attr" json:"album"`
	Path       string   `xml:"path,attr,omitempty" json:"path,omitempty"`
	PlayCount  int      `xml:"playCount,attr,omitempty" json:"playCount,omitempty"`
	LastPlayed string   `xml:"lastPlayed,attr,omitempty" json:"lastPlayed,omitempty"`
	Starred    bool     `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	Genre      string   `xml:"genre,attr,omitempty" json:"genre,omitempty"`
}

type SubsonicArtistIndex struct {
	XMLName xml.Name         `xml:"index" json:"-"`
	Name    string           `xml:"name,attr" json:"name"`
	Artists []SubsonicArtist `xml:"artist" json:"artist"`
}

type SubsonicArtists struct {
	XMLName xml.Name              `xml:"artists" json:"-"`
	Index   []SubsonicArtistIndex `xml:"index" json:"index"`
}

type SubsonicArtist struct {
	XMLName    xml.Name `xml:"artist" json:"-"`
	ID         string   `xml:"id,attr" json:"id"`
	Name       string   `xml:"name,attr" json:"name"`
	CoverArt   string   `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	AlbumCount int      `xml:"albumCount,attr" json:"albumCount"`
}

type SubsonicAlbumList2 struct {
	XMLName xml.Name        `xml:"albumList2" json:"-"`
	Albums  []SubsonicAlbum `xml:"album" json:"album"`
}

type SubsonicAlbum struct {
	XMLName  xml.Name `xml:"album" json:"-"`
	ID       string   `xml:"id,attr" json:"id"`
	Name     string   `xml:"name,attr" json:"name"`
	Artist   string   `xml:"artist,attr" json:"artist"`
	CoverArt string   `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Genre    string   `xml:"genre,attr,omitempty" json:"genre,omitempty"`
}

type SubsonicPlaylists struct {
	XMLName   xml.Name           `xml:"playlists" json:"-"`
	Playlists []SubsonicPlaylist `xml:"playlist" json:"playlist"`
}

type SubsonicPlaylist struct {
	XMLName   xml.Name `xml:"playlist" json:"-"`
	ID        int      `xml:"id,attr" json:"id"`
	Name      string   `xml:"name,attr" json:"name"`
	Owner     string   `xml:"owner,attr" json:"owner"`
	Public    bool     `xml:"public,attr" json:"public"`
	SongCount int      `xml:"songCount,attr" json:"songCount"`
	Duration  int      `xml:"duration,attr" json:"duration"`
}

type SubsonicScanStatus struct {
	XMLName  xml.Name `xml:"scanStatus" json:"-"`
	Scanning bool     `xml:"scanning,attr" json:"scanning"`
	Count    int64    `xml:"count,attr" json:"count"`
}

type SubsonicUsers struct {
	XMLName xml.Name       `xml:"users" json:"-"`
	Users   []SubsonicUser `xml:"user" json:"user"`
}

type SubsonicUser struct {
	XMLName      xml.Name `xml:"user" json:"-"`
	Username     string   `xml:"username,attr" json:"username"`
	AdminRole    bool     `xml:"adminRole,attr" json:"adminRole"`
	SettingsRole bool     `xml:"settingsRole,attr" json:"settingsRole"`
}

type SubsonicConfigurations struct {
	XMLName        xml.Name                `xml:"configurations" json:"-"`
	Configurations []SubsonicConfiguration `xml:"configuration" json:"configuration"`
}

type SubsonicConfiguration struct {
	XMLName xml.Name `xml:"configuration" json:"-"`
	Name    string   `xml:"name,attr" json:"name"`
	Value   string   `xml:"value,attr" json:"value"`
}

type SubsonicLibraryPaths struct {
	XMLName xml.Name              `xml:"libraryPaths" json:"-"`
	Paths   []SubsonicLibraryPath `xml:"path" json:"path"`
}

type SubsonicLibraryPath struct {
	XMLName       xml.Name `xml:"path" json:"-"`
	ID            int      `xml:"id,attr" json:"id"`
	Path          string   `xml:"path,attr" json:"path"`
	SongCount     int      `xml:"songCount,attr" json:"songCount"`
	LastScanEnded string   `xml:"lastScanEnded,attr,omitempty" json:"lastScanEnded"`
}

// --- OpenSubsonic Extension Structs ---

type OpenSubsonicExtension struct {
	Name     string `json:"name" xml:"name,attr"`
	Versions []int  `json:"versions" xml:"versions>version"`
}

type OpenSubsonicExtensions struct {
	XMLName    xml.Name                `xml:"openSubsonicExtensions" json:"-"`
	Extensions []OpenSubsonicExtension `xml:"extension" json:"extension"`
}

type ApiKeyResponse struct {
	XMLName xml.Name `xml:"apiKey" json:"-"`
	Key     string   `xml:"key,attr" json:"key"`
}

type SubsonicSongWrapper struct {
	XMLName xml.Name `xml:"song" json:"-"`
	Song    SubsonicSong
}

type SubsonicGenres struct {
	XMLName xml.Name        `xml:"genres" json:"-"`
	Genres  []SubsonicGenre `xml:"genre" json:"genre"`
}

type SubsonicGenre struct {
	XMLName    xml.Name `xml:"genre" json:"-"`
	Name       string   `xml:",chardata" json:"name"`
	SongCount  int      `xml:"songCount,attr" json:"songCount"`
	AlbumCount int      `xml:"albumCount,attr" json:"albumCount"`
}

type SubsonicStarred struct {
	XMLName xml.Name       `xml:"starred" json:"-"`
	Songs   []SubsonicSong `xml:"song" json:"song"`
}

type SubsonicSongsByGenre struct {
	XMLName xml.Name       `xml:"songsByGenre" json:"-"`
	Songs   []SubsonicSong `xml:"song" json:"song"`
}
