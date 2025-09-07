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
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	Path        string `json:"-"` // Don't expose path in JSON
	PlayCount   int    `json:"playCount"`
	LastPlayed  string `json:"lastPlayed"`
	DateAdded   string `json:"dateAdded"`
	DateUpdated string `json:"dateUpdated"`
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
	XMLName       xml.Name `xml:"subsonic-response" json:"-"`
	Status        string   `xml:"status,attr" json:"status"`
	Version       string   `xml:"version,attr" json:"version"`
	Xmlns         string   `xml:"xmlns,attr" json:"xmlns,omitempty"`
	Type          string   `xml:"type,attr,omitempty" json:"type,omitempty"`
	ServerVersion string   `xml:"serverVersion,attr,omitempty" json:"serverVersion,omitempty"`
	OpenSubsonic  bool     `xml:"openSubsonic,attr,omitempty" json:"openSubsonic,omitempty"`
	Body          interface{}
}

// SubsonicError represents an error message in a Subsonic response.
type SubsonicError struct {
	XMLName xml.Name `xml:"error" json:"-"`
	Code    int      `xml:"code,attr" json:"code"`
	Message string   `xml:"message,attr" json:"message"`
}

// SubsonicLicense represents the license status.
type SubsonicLicense struct {
	XMLName xml.Name `xml:"license" json:"-"`
	Valid   bool     `xml:"valid,attr" json:"valid"`
}

// SubsonicDirectory represents a container for songs (e.g., a playlist).
type SubsonicDirectory struct {
	XMLName   xml.Name       `xml:"directory" json:"-"`
	ID        string         `xml:"id,attr,omitempty" json:"id,omitempty"`
	Name      string         `xml:"name,attr,omitempty" json:"name,omitempty"`
	CoverArt  string         `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	SongCount int            `xml:"songCount,attr" json:"songCount"`
	Songs     []SubsonicSong `xml:"song" json:"song"`
}

// SubsonicAlbumWithSongs represents an album and its songs for getAlbum responses.
type SubsonicAlbumWithSongs struct {
	XMLName   xml.Name       `xml:"album" json:"-"`
	ID        string         `xml:"id,attr,omitempty" json:"id,omitempty"`
	Name      string         `xml:"name,attr,omitempty" json:"name,omitempty"`
	CoverArt  string         `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	SongCount int            `xml:"songCount,attr" json:"songCount"`
	Songs     []SubsonicSong `xml:"song" json:"song"`
}

// SubsonicSong represents a single song.
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
}

// SubsonicArtists represents a list of artists.
type SubsonicArtists struct {
	XMLName xml.Name         `xml:"artists" json:"-"`
	Artists []SubsonicArtist `xml:"artist" json:"artist"`
}

// SubsonicArtist represents a single artist.
type SubsonicArtist struct {
	XMLName xml.Name `xml:"artist" json:"-"`
	ID      string   `xml:"id,attr" json:"id"`
	Name    string   `xml:"name,attr" json:"name"`
}

// SubsonicAlbumList2 represents a list of albums.
type SubsonicAlbumList2 struct {
	XMLName xml.Name        `xml:"albumList2" json:"-"`
	Albums  []SubsonicAlbum `xml:"album" json:"album"`
}

// SubsonicAlbum represents a single album in a list.
type SubsonicAlbum struct {
	XMLName  xml.Name `xml:"album" json:"-"`
	ID       string   `xml:"id,attr" json:"id"`
	Name     string   `xml:"name,attr" json:"name"`
	Artist   string   `xml:"artist,attr" json:"artist"`
	CoverArt string   `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
}

// SubsonicPlaylists represents a list of playlists.
type SubsonicPlaylists struct {
	XMLName   xml.Name           `xml:"playlists" json:"-"`
	Playlists []SubsonicPlaylist `xml:"playlist" json:"playlist"`
}

// SubsonicPlaylist represents a single playlist.
type SubsonicPlaylist struct {
	XMLName   xml.Name `xml:"playlist" json:"-"`
	ID        int      `xml:"id,attr" json:"id"`
	Name      string   `xml:"name,attr" json:"name"`
	Owner     string   `xml:"owner,attr" json:"owner"`
	Public    bool     `xml:"public,attr" json:"public"`
	SongCount int      `xml:"songCount,attr" json:"songCount"`
	Duration  int      `xml:"duration,attr" json:"duration"`
}

// SubsonicTokenInfo represents information about an API key.
type SubsonicTokenInfo struct {
	XMLName  xml.Name `xml:"tokenInfo" json:"-"`
	Username string   `xml:"username,attr" json:"username"`
}

// SubsonicScanStatus represents the status of a library scan.
type SubsonicScanStatus struct {
	XMLName  xml.Name `xml:"scanStatus" json:"-"`
	Scanning bool     `xml:"scanning,attr" json:"scanning"`
	Count    int64    `xml:"count,attr" json:"count"`
}

// SubsonicUsers represents a list of users for user management.
type SubsonicUsers struct {
	XMLName xml.Name       `xml:"users" json:"-"`
	Users   []SubsonicUser `xml:"user" json:"user"`
}

// SubsonicUser represents a single user.
type SubsonicUser struct {
	XMLName      xml.Name `xml:"user" json:"-"`
	Username     string   `xml:"username,attr" json:"username"`
	AdminRole    bool     `xml:"adminRole,attr" json:"adminRole"`
	SettingsRole bool     `xml:"settingsRole,attr" json:"settingsRole"` // Same as admin for us
}

