package main

import (
	"database/sql"
	"path/filepath"
	"strings"
)

// newReplayGain builds a *SubsonicReplayGain from nullable DB columns. Returns
// nil when every value is NULL so the replayGain field is omitted entirely.
func newReplayGain(trackGain, trackPeak, albumGain, albumPeak sql.NullFloat64) *SubsonicReplayGain {
	if !trackGain.Valid && !trackPeak.Valid && !albumGain.Valid && !albumPeak.Valid {
		return nil
	}
	rg := &SubsonicReplayGain{}
	if trackGain.Valid {
		v := trackGain.Float64
		rg.TrackGain = &v
	}
	if trackPeak.Valid {
		v := trackPeak.Float64
		rg.TrackPeak = &v
	}
	if albumGain.Valid {
		v := albumGain.Float64
		rg.AlbumGain = &v
	}
	if albumPeak.Valid {
		v := albumPeak.Float64
		rg.AlbumPeak = &v
	}
	return rg
}

// audioContentTypes maps a lower-cased file suffix (without dot) to its MIME
// content type. Used to populate the OpenSubsonic Child.contentType/suffix
// fields without touching the filesystem (the path is already in the DB row).
var audioContentTypes = map[string]string{
	"mp3":  "audio/mpeg",
	"flac": "audio/flac",
	"ogg":  "audio/ogg",
	"oga":  "audio/ogg",
	"opus": "audio/opus",
	"aac":  "audio/aac",
	"m4a":  "audio/mp4",
	"m4b":  "audio/mp4",
	"mp4":  "audio/mp4",
	"alac": "audio/mp4",
	"wav":  "audio/x-wav",
	"wma":  "audio/x-ms-wma",
	"aiff": "audio/x-aiff",
	"aif":  "audio/x-aiff",
	"ape":  "audio/x-ape",
	"wv":   "audio/x-wavpack",
	"mpc":  "audio/x-musepack",
	"dsf":  "audio/x-dsf",
}

// suffixFromPath returns the lower-cased file extension (without the dot).
func suffixFromPath(path string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
}

// audioContentType returns the MIME content type for a given suffix, falling
// back to a generic octet-stream type for unknown suffixes.
func audioContentType(suffix string) string {
	if ct, ok := audioContentTypes[suffix]; ok {
		return ct
	}
	if suffix == "" {
		return ""
	}
	return "application/octet-stream"
}

// buildReplayGain assembles the OpenSubsonic ReplayGain object from a SongResult.
// Returns nil when no replay gain data is present so the field is omitted.
func buildReplayGain(r SongResult) *SubsonicReplayGain {
	return r.ReplayGain
}

// directoryChildFromSong maps a fully-built SubsonicSong (Child) onto the legacy
// directory-browsing SubsonicDirectoryChild so getMusicDirectory emits the same
// spec-aligned fields as the ID3 endpoints.
func directoryChildFromSong(s SubsonicSong) SubsonicDirectoryChild {
	return SubsonicDirectoryChild{
		ID:            s.ID,
		Parent:        s.Parent,
		Title:         s.Title,
		Album:         s.Album,
		Artist:        s.Artist,
		ArtistID:      s.ArtistID,
		AlbumID:       s.AlbumID,
		IsDir:         false,
		CoverArt:      s.CoverArt,
		Suffix:        s.Suffix,
		ContentType:   s.ContentType,
		Size:          s.Size,
		BitRate:       s.BitRate,
		SamplingRate:  s.SamplingRate,
		ChannelCount:  s.ChannelCount,
		BitDepth:      s.BitDepth,
		Track:         s.Track,
		Year:          s.Year,
		DiscNumber:    s.DiscNumber,
		Duration:      s.Duration,
		Genre:         s.Genre,
		PlayCount:     s.PlayCount,
		LastPlayed:    s.LastPlayed,
		Created:       s.Created,
		Starred:       s.Starred,
		Comment:       s.Comment,
		Type:          s.Type,
		MediaType:     s.MediaType,
		DisplayArtist: s.DisplayArtist,
		Genres:        s.Genres,
		ReplayGain:    s.ReplayGain,
	}
}

// buildSubsonicSong converts a SongResult into a fully populated, OpenSubsonic
// spec-aligned Child object. Every derivable field (isDir, type, mediaType,
// suffix, contentType, created, genres, displayArtist, replayGain, parent,
// album/artist ids) is filled in centrally so all endpoints emit identical,
// compliant song objects. Fields with no underlying data are omitted.
func buildSubsonicSong(r SongResult) SubsonicSong {
	s := SubsonicSong{
		ID:           r.ID,
		IsDir:        false,
		CoverArt:     r.ID,
		Title:        r.Title,
		Artist:       r.Artist,
		Album:        r.Album,
		Duration:     r.Duration,
		PlayCount:    r.PlayCount,
		LastPlayed:   r.LastPlayed,
		Created:      r.Created,
		Starred:      r.Starred,
		Genre:        r.Genre,
		Track:        r.Track,
		Year:         r.Year,
		DiscNumber:   r.DiscNumber,
		Size:         r.Size,
		BitRate:      r.BitRate,
		SamplingRate: r.SamplingRate,
		ChannelCount: r.ChannelCount,
		BitDepth:     r.BitDepth,
		Comment:      r.Comment,
		Type:         "music",
		MediaType:    "song",
	}

	if r.Artist != "" {
		s.ArtistID = GenerateArtistID(r.Artist)
		s.DisplayArtist = r.Artist
	}
	if r.AlbumArtist != "" {
		s.AlbumArtist = r.AlbumArtist
		s.AlbumArtistID = GenerateArtistID(r.AlbumArtist)
	}
	if r.AlbumID != "" {
		s.AlbumID = r.AlbumID
		s.Parent = r.AlbumID
	}
	if suf := suffixFromPath(r.Path); suf != "" {
		s.Suffix = suf
		s.ContentType = audioContentType(suf)
	}
	if r.Genre != "" {
		s.Genres = []SubsonicItemGenre{{Name: r.Genre}}
	}
	s.ReplayGain = buildReplayGain(r)

	return s
}
