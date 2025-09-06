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
	XMLName xml.Name `xml:"subsonic-response" json:"-"`
	Status  string   `xml:"status,attr" json:"status"`
	Version string   `xml:"version,attr" json:"version"`
	Xmlns   string   `xml:"xmlns,attr" json:"xmlns,omitempty"`
	Body    interface{}
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

// SubsonicDirectory represents a container for songs.
type SubsonicDirectory struct {
	XMLName   xml.Name       `xml:"directory" json:"-"`
	SongCount int            `xml:"songCount,attr" json:"songCount"`
	Songs     []SubsonicSong `xml:"song" json:"song"`
}

// SubsonicSong represents a single song.
type SubsonicSong struct {
	XMLName xml.Name `xml:"song" json:"-"`
	ID      int      `xml:"id,attr" json:"id"`
	Title   string   `xml:"title,attr" json:"title"`
	Artist  string   `xml:"artist,attr" json:"artist"`
	Album   string   `xml:"album,attr" json:"album"`
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
	XMLName xml.Name `xml:"album" json:"-"`
	ID      string   `xml:"id,attr" json:"id"`
	Name    string   `xml:"name,attr" json:"name"`
	Artist  string   `xml:"artist,attr" json:"artist"`
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
