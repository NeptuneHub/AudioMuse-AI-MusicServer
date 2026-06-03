// Suggested path: music-server-backend/models.go
package main

import (
	"database/sql"
	"encoding/xml"
	"strconv"
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
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	Artist              string   `json:"artist"`
	Album               string   `json:"album"`
	AlbumArtist         string   `json:"albumArtist"`
	Path                string   `json:"-"`        // Don't expose path in JSON
	Duration            int      `json:"duration"` // Duration in seconds
	PlayCount           int      `json:"playCount"`
	LastPlayed          string   `json:"lastPlayed"`
	DateAdded           string   `json:"dateAdded"`
	DateUpdated         string   `json:"dateUpdated"`
	Starred             bool     `json:"starred"`
	Genre               string   `json:"genre"`
	Cancelled           bool     `json:"cancelled"`
	ReplayGainTrackGain *float64 `json:"replaygainTrackGain,omitempty"`
	ReplayGainTrackPeak *float64 `json:"replaygainTrackPeak,omitempty"`
	ReplayGainAlbumGain *float64 `json:"replaygainAlbumGain,omitempty"`
	ReplayGainAlbumPeak *float64 `json:"replaygainAlbumPeak,omitempty"`
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

type RadioStation struct {
	ID               int     `json:"id"`
	UserID           int     `json:"user_id"`
	Name             string  `json:"name"`
	SeedSongs        string  `json:"seed_songs"` // JSON string: [{"id":"123","op":"ADD"},...]
	Temperature      float64 `json:"temperature"`
	SubtractDistance float64 `json:"subtract_distance"`
	CreatedAt        string  `json:"created_at"`
	UpdatedAt        string  `json:"updated_at"`
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

// SubsonicRandomSongs is the getRandomSongs response (<randomSongs> of Child).
type SubsonicRandomSongs struct {
	XMLName xml.Name       `xml:"randomSongs" json:"-"`
	Songs   []SubsonicSong `xml:"song" json:"song"`
}

// SubsonicPlaylistWithSongs is the getPlaylist response: a <playlist> with the
// playlist-level attributes and its songs as <entry> children (Child objects).
type SubsonicPlaylistWithSongs struct {
	XMLName   xml.Name       `xml:"playlist" json:"-"`
	ID        string         `xml:"id,attr" json:"id"`
	Name      string         `xml:"name,attr" json:"name"`
	Owner     string         `xml:"owner,attr,omitempty" json:"owner,omitempty"`
	Public    bool           `xml:"public,attr" json:"public"`
	SongCount int            `xml:"songCount,attr" json:"songCount"`
	Duration  int            `xml:"duration,attr" json:"duration"`
	Entries   []SubsonicSong `xml:"entry" json:"entry"`
}

// MarshalXML emits <playlist ...> with each song as an <entry> child. A custom
// marshaler is needed because SubsonicSong's XMLName would otherwise force the
// children to <song>; EncodeElement with an explicit start name overrides it.
func (p SubsonicPlaylistWithSongs) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "playlist"}
	start.Attr = []xml.Attr{
		{Name: xml.Name{Local: "id"}, Value: p.ID},
		{Name: xml.Name{Local: "name"}, Value: p.Name},
	}
	if p.Owner != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "owner"}, Value: p.Owner})
	}
	start.Attr = append(start.Attr,
		xml.Attr{Name: xml.Name{Local: "public"}, Value: strconv.FormatBool(p.Public)},
		xml.Attr{Name: xml.Name{Local: "songCount"}, Value: strconv.Itoa(p.SongCount)},
		xml.Attr{Name: xml.Name{Local: "duration"}, Value: strconv.Itoa(p.Duration)},
	)
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, s := range p.Entries {
		if err := e.EncodeElement(s, xml.StartElement{Name: xml.Name{Local: "entry"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

// SubsonicAlbumWithSongs is the AlbumID3 returned by getAlbum, including its
// song list. Mirrors the AlbumID3 fields so clients get artist/genre context.
type SubsonicAlbumWithSongs struct {
	XMLName       xml.Name            `xml:"album" json:"-"`
	ID            string              `xml:"id,attr" json:"id"`     // Required on AlbumID3
	Name          string              `xml:"name,attr" json:"name"` // Required on AlbumID3
	Artist        string              `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	ArtistID      string              `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	CoverArt      string              `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	SongCount     int                 `xml:"songCount,attr" json:"songCount"`
	Duration      int                 `xml:"duration,attr" json:"duration"`
	Created       string              `xml:"created,attr" json:"created"` // Required on AlbumID3
	Genre         string              `xml:"genre,attr,omitempty" json:"genre,omitempty"`
	DisplayArtist string              `xml:"displayArtist,attr,omitempty" json:"displayArtist,omitempty"`
	Genres        []SubsonicItemGenre `xml:"genres" json:"genres,omitempty"`
	Songs         []SubsonicSong      `xml:"song" json:"song"`
}

// SubsonicSong models the OpenSubsonic "Child" object. The required fields
// (id, isDir, title) are always emitted; the remaining standard and
// OpenSubsonic-extension fields are populated by buildSubsonicSong when the
// underlying data is available. See https://opensubsonic.netlify.app/docs/responses/child/
type SubsonicSong struct {
	XMLName       xml.Name `xml:"song" json:"-"`
	ID            string   `xml:"id,attr" json:"id"`
	Parent        string   `xml:"parent,attr,omitempty" json:"parent,omitempty"`
	IsDir         bool     `xml:"isDir,attr" json:"isDir"` // Required by spec; songs are always false
	CoverArt      string   `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Title         string   `xml:"title,attr" json:"title"`
	Artist        string   `xml:"artist,attr" json:"artist"`
	ArtistID      string   `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	Album         string   `xml:"album,attr" json:"album"`
	AlbumID       string   `xml:"albumId,attr,omitempty" json:"albumId,omitempty"`
	AlbumArtist   string   `xml:"albumArtist,attr,omitempty" json:"albumArtist,omitempty"`
	AlbumArtistID string   `xml:"albumArtistId,attr,omitempty" json:"albumArtistId,omitempty"`
	Path          string   `xml:"path,attr,omitempty" json:"path,omitempty"`
	Suffix        string   `xml:"suffix,attr,omitempty" json:"suffix,omitempty"`
	ContentType   string   `xml:"contentType,attr,omitempty" json:"contentType,omitempty"`
	Size          int64    `xml:"size,attr,omitempty" json:"size,omitempty"`
	BitRate       int      `xml:"bitRate,attr,omitempty" json:"bitRate,omitempty"`
	SamplingRate  int      `xml:"samplingRate,attr,omitempty" json:"samplingRate,omitempty"` // OpenSubsonic
	ChannelCount  int      `xml:"channelCount,attr,omitempty" json:"channelCount,omitempty"` // OpenSubsonic
	BitDepth      int      `xml:"bitDepth,attr,omitempty" json:"bitDepth,omitempty"`         // OpenSubsonic
	Track         int      `xml:"track,attr,omitempty" json:"track,omitempty"`
	Year          int      `xml:"year,attr,omitempty" json:"year,omitempty"`
	DiscNumber    int      `xml:"discNumber,attr,omitempty" json:"discNumber,omitempty"`
	Duration      int      `xml:"duration,attr,omitempty" json:"duration,omitempty"` // Duration in seconds
	PlayCount     int      `xml:"playCount,attr,omitempty" json:"playCount,omitempty"`
	LastPlayed    string   `xml:"lastPlayed,attr,omitempty" json:"lastPlayed,omitempty"`
	Created       string   `xml:"created,attr,omitempty" json:"created,omitempty"`
	Starred       bool     `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	Genre         string   `xml:"genre,attr,omitempty" json:"genre,omitempty"`
	Comment       string   `xml:"comment,attr,omitempty" json:"comment,omitempty"`     // OpenSubsonic
	Type          string   `xml:"type,attr,omitempty" json:"type,omitempty"`           // Always "music" for songs
	MediaType     string   `xml:"mediaType,attr,omitempty" json:"mediaType,omitempty"` // OpenSubsonic: "song"
	DisplayArtist string   `xml:"displayArtist,attr,omitempty" json:"displayArtist,omitempty"`
	// Nested OpenSubsonic-extension objects.
	Genres     []SubsonicItemGenre `xml:"genres" json:"genres,omitempty"`
	ReplayGain *SubsonicReplayGain `xml:"replayGain" json:"replayGain,omitempty"`
}

// SubsonicItemGenre is an OpenSubsonic ItemGenre entry (Child.genres / AlbumID3.genres).
type SubsonicItemGenre struct {
	XMLName xml.Name `xml:"genres" json:"-"`
	Name    string   `xml:"name,attr" json:"name"`
}

// SubsonicReplayGain is the OpenSubsonic ReplayGain object on a Child.
type SubsonicReplayGain struct {
	XMLName   xml.Name `xml:"replayGain" json:"-"`
	TrackGain *float64 `xml:"trackGain,attr,omitempty" json:"trackGain,omitempty"`
	AlbumGain *float64 `xml:"albumGain,attr,omitempty" json:"albumGain,omitempty"`
	TrackPeak *float64 `xml:"trackPeak,attr,omitempty" json:"trackPeak,omitempty"`
	AlbumPeak *float64 `xml:"albumPeak,attr,omitempty" json:"albumPeak,omitempty"`
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
	SongCount  int      `xml:"songCount,attr,omitempty" json:"songCount,omitempty"`
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
	ArtistID string   `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	CoverArt string   `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Genre    string   `xml:"genre,attr,omitempty" json:"genre,omitempty"`
	// songCount, duration and created are REQUIRED on AlbumID3, so they are
	// always emitted (even when 0/empty) for strict spec compliance.
	SongCount int    `xml:"songCount,attr" json:"songCount"`
	Duration  int    `xml:"duration,attr" json:"duration"`
	Created   string `xml:"created,attr" json:"created"`
	// OpenSubsonic-extension fields.
	DisplayArtist string              `xml:"displayArtist,attr,omitempty" json:"displayArtist,omitempty"`
	Genres        []SubsonicItemGenre `xml:"genres" json:"genres,omitempty"`
}

// decorateAlbum fills the OpenSubsonic-extension AlbumID3 fields (displayArtist,
// genres) that are pure derivations of the already-populated artist/genre, so
// every album construction site emits a spec-aligned object via one call.
func decorateAlbum(a *SubsonicAlbum) {
	if a.Artist != "" && a.DisplayArtist == "" {
		a.DisplayArtist = a.Artist
	}
	if a.Genre != "" && len(a.Genres) == 0 {
		a.Genres = []SubsonicItemGenre{{Name: a.Genre}}
	}
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
	Name       string   `xml:",chardata" json:"value"`
	SongCount  int      `xml:"songCount,attr" json:"songCount"`
	AlbumCount int      `xml:"albumCount,attr" json:"albumCount"`
}

type SubsonicStarred struct {
	XMLName xml.Name         `xml:"starred" json:"-"`
	Artists []SubsonicArtist `xml:"artist" json:"artist"`
	Albums  []SubsonicAlbum  `xml:"album" json:"album"`
	Songs   []SubsonicSong   `xml:"song" json:"song"`
}

type SubsonicStarred2 struct {
	XMLName xml.Name         `xml:"starred2" json:"-"`
	Artists []SubsonicArtist `xml:"artist" json:"artist"`
	Albums  []SubsonicAlbum  `xml:"album" json:"album"`
	Songs   []SubsonicSong   `xml:"song" json:"song"`
}

type SubsonicSongsByGenre struct {
	XMLName xml.Name       `xml:"songsByGenre" json:"-"`
	Songs   []SubsonicSong `xml:"song" json:"song"`
}

// Browsing API models

type SubsonicMusicFolder struct {
	XMLName xml.Name `xml:"musicFolder" json:"-"`
	ID      int      `xml:"id,attr" json:"id"`
	Name    string   `xml:"name,attr" json:"name"`
}

type SubsonicMusicFolders struct {
	XMLName xml.Name              `xml:"musicFolders" json:"-"`
	Folders []SubsonicMusicFolder `xml:"musicFolder" json:"musicFolder"`
}

type SubsonicIndexArtist struct {
	XMLName    xml.Name `xml:"artist" json:"-"`
	ID         string   `xml:"id,attr" json:"id"`
	Name       string   `xml:"name,attr" json:"name"`
	CoverArt   string   `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	AlbumCount int      `xml:"albumCount,attr,omitempty" json:"albumCount,omitempty"`
}

type SubsonicIndex struct {
	XMLName xml.Name              `xml:"index" json:"-"`
	Name    string                `xml:"name,attr" json:"name"`
	Artists []SubsonicIndexArtist `xml:"artist" json:"artist"`
}

type SubsonicIndexes struct {
	XMLName         xml.Name        `xml:"indexes" json:"-"`
	LastModified    int64           `xml:"lastModified,attr" json:"lastModified"`
	IgnoredArticles string          `xml:"ignoredArticles,attr,omitempty" json:"ignoredArticles,omitempty"`
	Indices         []SubsonicIndex `xml:"index" json:"index"`
}

// SubsonicDirectoryChild is the OpenSubsonic "Child" used by the legacy
// directory-browsing endpoints (getMusicDirectory, getAlbumList). It carries the
// same standard/extension fields as SubsonicSong so both formats stay aligned.
type SubsonicDirectoryChild struct {
	XMLName       xml.Name            `xml:"child" json:"-"`
	ID            string              `xml:"id,attr" json:"id"`
	Parent        string              `xml:"parent,attr,omitempty" json:"parent,omitempty"`
	Title         string              `xml:"title,attr" json:"title"`
	Album         string              `xml:"album,attr,omitempty" json:"album,omitempty"`
	Artist        string              `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	ArtistID      string              `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	AlbumID       string              `xml:"albumId,attr,omitempty" json:"albumId,omitempty"`
	IsDir         bool                `xml:"isDir,attr" json:"isDir"`
	CoverArt      string              `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Suffix        string              `xml:"suffix,attr,omitempty" json:"suffix,omitempty"`
	ContentType   string              `xml:"contentType,attr,omitempty" json:"contentType,omitempty"`
	Size          int64               `xml:"size,attr,omitempty" json:"size,omitempty"`
	BitRate       int                 `xml:"bitRate,attr,omitempty" json:"bitRate,omitempty"`
	SamplingRate  int                 `xml:"samplingRate,attr,omitempty" json:"samplingRate,omitempty"`
	ChannelCount  int                 `xml:"channelCount,attr,omitempty" json:"channelCount,omitempty"`
	BitDepth      int                 `xml:"bitDepth,attr,omitempty" json:"bitDepth,omitempty"`
	Track         int                 `xml:"track,attr,omitempty" json:"track,omitempty"`
	Year          int                 `xml:"year,attr,omitempty" json:"year,omitempty"`
	DiscNumber    int                 `xml:"discNumber,attr,omitempty" json:"discNumber,omitempty"`
	Duration      int                 `xml:"duration,attr,omitempty" json:"duration,omitempty"` // Duration in seconds
	Genre         string              `xml:"genre,attr,omitempty" json:"genre,omitempty"`
	PlayCount     int                 `xml:"playCount,attr,omitempty" json:"playCount,omitempty"`
	LastPlayed    string              `xml:"lastPlayed,attr,omitempty" json:"lastPlayed,omitempty"`
	Created       string              `xml:"created,attr,omitempty" json:"created,omitempty"`
	Starred       bool                `xml:"starred,attr,omitempty" json:"starred,omitempty"`
	Comment       string              `xml:"comment,attr,omitempty" json:"comment,omitempty"`
	Type          string              `xml:"type,attr,omitempty" json:"type,omitempty"`
	MediaType     string              `xml:"mediaType,attr,omitempty" json:"mediaType,omitempty"`
	DisplayArtist string              `xml:"displayArtist,attr,omitempty" json:"displayArtist,omitempty"`
	Genres        []SubsonicItemGenre `xml:"genres" json:"genres,omitempty"`
	ReplayGain    *SubsonicReplayGain `xml:"replayGain" json:"replayGain,omitempty"`
}

// SubsonicAlbumList is the legacy getAlbumList response (Child objects).
type SubsonicAlbumList struct {
	XMLName xml.Name                 `xml:"albumList" json:"-"`
	Albums  []SubsonicDirectoryChild `xml:"album" json:"album"`
}

type SubsonicMusicDirectory struct {
	XMLName  xml.Name                 `xml:"directory" json:"-"`
	ID       string                   `xml:"id,attr" json:"id"`
	Parent   string                   `xml:"parent,attr,omitempty" json:"parent,omitempty"`
	Name     string                   `xml:"name,attr" json:"name"`
	Children []SubsonicDirectoryChild `xml:"child" json:"child"`
}

type SubsonicArtistWithAlbums struct {
	XMLName    xml.Name        `xml:"artist" json:"-"`
	ID         string          `xml:"id,attr" json:"id"`
	Name       string          `xml:"name,attr" json:"name"`
	CoverArt   string          `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	AlbumCount int             `xml:"albumCount,attr" json:"albumCount"`
	Albums     []SubsonicAlbum `xml:"album" json:"album"`
}

// Media info API models

type SubsonicTopSongs struct {
	XMLName xml.Name       `xml:"topSongs" json:"-"`
	Songs   []SubsonicSong `xml:"song" json:"song"`
}

type SubsonicSimilarSongs struct {
	XMLName xml.Name       `xml:"similarSongs2" json:"-"`
	Songs   []SubsonicSong `xml:"song" json:"song"`
}

type SubsonicAlbumInfo struct {
	XMLName        xml.Name `xml:"albumInfo" json:"-"`
	Notes          string   `xml:"notes,omitempty" json:"notes,omitempty"`
	MusicBrainzID  string   `xml:"musicBrainzId,omitempty" json:"musicBrainzId,omitempty"`
	LastFmUrl      string   `xml:"lastFmUrl,omitempty" json:"lastFmUrl,omitempty"`
	SmallImageUrl  string   `xml:"smallImageUrl,omitempty" json:"smallImageUrl,omitempty"`
	MediumImageUrl string   `xml:"mediumImageUrl,omitempty" json:"mediumImageUrl,omitempty"`
	LargeImageUrl  string   `xml:"largeImageUrl,omitempty" json:"largeImageUrl,omitempty"`
}
