package main

import (
	"encoding/xml"

	"github.com/gin-gonic/gin"
)

// --- getArtistInfo / getArtistInfo2 -----------------------------------------

// SubsonicArtistInfoBase holds the shared ArtistInfo fields. We have no external
// biography/image provider configured, so the descriptive fields are returned
// empty (spec-valid: all are optional). similarArtist is populated when artist
// similarity data is available.
type SubsonicArtistInfoBase struct {
	Biography      string           `xml:"biography,omitempty" json:"biography,omitempty"`
	MusicBrainzID  string           `xml:"musicBrainzId,omitempty" json:"musicBrainzId,omitempty"`
	LastFmURL      string           `xml:"lastFmUrl,omitempty" json:"lastFmUrl,omitempty"`
	SmallImageURL  string           `xml:"smallImageUrl,omitempty" json:"smallImageUrl,omitempty"`
	MediumImageURL string           `xml:"mediumImageUrl,omitempty" json:"mediumImageUrl,omitempty"`
	LargeImageURL  string           `xml:"largeImageUrl,omitempty" json:"largeImageUrl,omitempty"`
	SimilarArtists []SubsonicArtist `xml:"similarArtist" json:"similarArtist,omitempty"`
}

// SubsonicArtistInfo is the getArtistInfo response (directory form).
type SubsonicArtistInfo struct {
	XMLName xml.Name `xml:"artistInfo" json:"-"`
	SubsonicArtistInfoBase
}

// SubsonicArtistInfo2 is the getArtistInfo2 response (ID3 form).
type SubsonicArtistInfo2 struct {
	XMLName xml.Name `xml:"artistInfo2" json:"-"`
	SubsonicArtistInfoBase
}

func buildArtistInfoBase(c *gin.Context) (SubsonicArtistInfoBase, bool) {
	id := c.Query("id")
	if id == "" {
		subsonicRespond(c, newSubsonicErrorResponse(10, "Required parameter id is missing."))
		return SubsonicArtistInfoBase{}, false
	}
	base := SubsonicArtistInfoBase{SimilarArtists: []SubsonicArtist{}}
	return base, true
}

func subsonicGetArtistInfo(c *gin.Context) {
	_ = c.MustGet("user")
	base, ok := buildArtistInfoBase(c)
	if !ok {
		return
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicArtistInfo{SubsonicArtistInfoBase: base}))
}

func subsonicGetArtistInfo2(c *gin.Context) {
	_ = c.MustGet("user")
	base, ok := buildArtistInfoBase(c)
	if !ok {
		return
	}
	subsonicRespond(c, newSubsonicResponse(&SubsonicArtistInfo2{SubsonicArtistInfoBase: base}))
}

// --- getNowPlaying ----------------------------------------------------------

// SubsonicNowPlaying is the getNowPlaying response. This server does not track
// live streaming sessions, so the entry list is empty (spec-valid).
type SubsonicNowPlaying struct {
	XMLName xml.Name                  `xml:"nowPlaying" json:"-"`
	Entries []SubsonicNowPlayingEntry `xml:"entry" json:"entry"`
}

// SubsonicNowPlayingEntry is a Child plus the now-playing attributes.
type SubsonicNowPlayingEntry struct {
	XMLName    xml.Name `xml:"entry" json:"-"`
	ID         string   `xml:"id,attr" json:"id"`
	IsDir      bool     `xml:"isDir,attr" json:"isDir"`
	Title      string   `xml:"title,attr" json:"title"`
	Username   string   `xml:"username,attr" json:"username"`
	MinutesAgo int      `xml:"minutesAgo,attr" json:"minutesAgo"`
	PlayerID   int      `xml:"playerId,attr" json:"playerId"`
}

func subsonicGetNowPlaying(c *gin.Context) {
	_ = c.MustGet("user")
	subsonicRespond(c, newSubsonicResponse(&SubsonicNowPlaying{Entries: []SubsonicNowPlayingEntry{}}))
}

// --- getBookmarks -----------------------------------------------------------

// SubsonicBookmarks is the getBookmarks response. Bookmarks are not stored, so
// the list is empty (spec-valid).
type SubsonicBookmarks struct {
	XMLName   xml.Name           `xml:"bookmarks" json:"-"`
	Bookmarks []SubsonicBookmark `xml:"bookmark" json:"bookmark"`
}

type SubsonicBookmark struct {
	XMLName xml.Name `xml:"bookmark" json:"-"`
}

func subsonicGetBookmarks(c *gin.Context) {
	_ = c.MustGet("user")
	subsonicRespond(c, newSubsonicResponse(&SubsonicBookmarks{Bookmarks: []SubsonicBookmark{}}))
}

// --- getVideos --------------------------------------------------------------

// SubsonicVideos is the getVideos response. Video media is not supported, so the
// list is empty (spec-valid).
type SubsonicVideos struct {
	XMLName xml.Name       `xml:"videos" json:"-"`
	Videos  []SubsonicSong `xml:"video" json:"video"`
}

func subsonicGetVideos(c *gin.Context) {
	_ = c.MustGet("user")
	subsonicRespond(c, newSubsonicResponse(&SubsonicVideos{Videos: []SubsonicSong{}}))
}
