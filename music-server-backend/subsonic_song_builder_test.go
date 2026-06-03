package main

import "testing"

func TestBuildSubsonicSong_PopulatesSpecFields(t *testing.T) {
	tg, tp := -7.5, 0.98
	r := SongResult{
		ID:          "song1",
		Title:       "Title",
		Artist:      "The Artist",
		Album:       "The Album",
		AlbumArtist: "The Album Artist",
		Path:        "/music/The Artist/The Album/01 Title.flac",
		Duration:    240,
		PlayCount:   3,
		LastPlayed:  "2026-01-02T03:04:05Z",
		Created:     "2025-12-01T00:00:00Z",
		Genre:       "Jazz",
		AlbumID:     "albumRep",
		Track:       5,
		Year:        2001,
		DiscNumber:  2,
		Starred:     true,
		ReplayGain:  &SubsonicReplayGain{TrackGain: &tg, TrackPeak: &tp},
	}

	s := buildSubsonicSong(r)

	if s.Track != 5 || s.Year != 2001 || s.DiscNumber != 2 {
		t.Errorf("track/year/disc = %d/%d/%d, want 5/2001/2", s.Track, s.Year, s.DiscNumber)
	}

	if s.IsDir {
		t.Errorf("isDir should be false for a song")
	}
	if s.Type != "music" {
		t.Errorf("type = %q, want music", s.Type)
	}
	if s.MediaType != "song" {
		t.Errorf("mediaType = %q, want song", s.MediaType)
	}
	if s.Suffix != "flac" {
		t.Errorf("suffix = %q, want flac", s.Suffix)
	}
	if s.ContentType != "audio/flac" {
		t.Errorf("contentType = %q, want audio/flac", s.ContentType)
	}
	if s.Created != r.Created {
		t.Errorf("created = %q, want %q", s.Created, r.Created)
	}
	if s.CoverArt != "song1" {
		t.Errorf("coverArt = %q, want song1", s.CoverArt)
	}
	if s.ArtistID != GenerateArtistID("The Artist") {
		t.Errorf("artistId mismatch")
	}
	if s.AlbumArtistID != GenerateArtistID("The Album Artist") {
		t.Errorf("albumArtistId mismatch")
	}
	if s.DisplayArtist != "The Artist" {
		t.Errorf("displayArtist = %q", s.DisplayArtist)
	}
	if s.AlbumID != "albumRep" || s.Parent != "albumRep" {
		t.Errorf("albumId/parent = %q/%q, want albumRep", s.AlbumID, s.Parent)
	}
	if len(s.Genres) != 1 || s.Genres[0].Name != "Jazz" {
		t.Errorf("genres = %+v, want [{Jazz}]", s.Genres)
	}
	if s.ReplayGain == nil || s.ReplayGain.TrackGain == nil || *s.ReplayGain.TrackGain != -7.5 {
		t.Errorf("replayGain not populated: %+v", s.ReplayGain)
	}
}

func TestBuildSubsonicSong_OmitsEmptyDerived(t *testing.T) {
	s := buildSubsonicSong(SongResult{ID: "x", Title: "t"})
	if s.Suffix != "" || s.ContentType != "" {
		t.Errorf("expected empty suffix/contentType for empty path")
	}
	if len(s.Genres) != 0 {
		t.Errorf("expected no genres for empty genre")
	}
	if s.ReplayGain != nil {
		t.Errorf("expected nil replayGain when no data")
	}
	if s.ArtistID != "" {
		t.Errorf("expected empty artistId for empty artist")
	}
}

// TestQuerySongs_RoundTripSpecFields verifies that the spec-aligned columns
// (track, year, disc_number, replay gain, album_artist, date_added) flow from
// the DB through QuerySongs and into a fully populated Child object.
func TestQuerySongs_RoundTripSpecFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`INSERT INTO songs
		(id, title, artist, album, album_artist, path, album_path, genre, duration, date_added,
		 replaygain_track_gain, track, year, disc_number, size, bitrate, sample_rate, channels, bit_depth, comment, cancelled)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0)`,
		"rt1", "RT Title", "RT Artist", "RT Album", "RT AlbumArtist",
		"/m/RT Artist/RT Album/03 RT Title.mp3", "/m/RT Artist/RT Album", "Rock", 200,
		"2025-11-11T11:11:11Z", -6.5, 3, 1999, 1, 8388608, 320, 44100, 2, 16, "Great track")
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	res, err := QuerySongs(db, SongQueryOptions{IDs: []string{"rt1"}, IncludeGenre: true, Limit: 1})
	if err != nil {
		t.Fatalf("QuerySongs failed: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}

	s := buildSubsonicSong(res[0])
	if s.Track != 3 || s.Year != 1999 || s.DiscNumber != 1 {
		t.Errorf("track/year/disc = %d/%d/%d, want 3/1999/1", s.Track, s.Year, s.DiscNumber)
	}
	if s.Suffix != "mp3" || s.ContentType != "audio/mpeg" {
		t.Errorf("suffix/contentType = %q/%q", s.Suffix, s.ContentType)
	}
	if s.Created != "2025-11-11T11:11:11Z" {
		t.Errorf("created = %q", s.Created)
	}
	if s.AlbumArtist != "RT AlbumArtist" || s.AlbumArtistID == "" {
		t.Errorf("albumArtist/Id = %q/%q", s.AlbumArtist, s.AlbumArtistID)
	}
	if s.AlbumID != "rt1" || s.Parent != "rt1" {
		t.Errorf("albumId/parent = %q/%q, want rt1 (MIN id over album_path)", s.AlbumID, s.Parent)
	}
	if s.ReplayGain == nil || s.ReplayGain.TrackGain == nil || *s.ReplayGain.TrackGain != -6.5 {
		t.Errorf("replayGain not wired: %+v", s.ReplayGain)
	}
	if s.IsDir { // isDir must always be present and false for songs
		t.Errorf("isDir should be false")
	}
	if s.Size != 8388608 || s.BitRate != 320 || s.SamplingRate != 44100 || s.ChannelCount != 2 || s.BitDepth != 16 {
		t.Errorf("audio props: size=%d bitRate=%d sampling=%d channels=%d bitDepth=%d",
			s.Size, s.BitRate, s.SamplingRate, s.ChannelCount, s.BitDepth)
	}
	if s.Comment != "Great track" {
		t.Errorf("comment = %q", s.Comment)
	}
}

func TestAudioContentType(t *testing.T) {
	cases := map[string]string{
		"mp3": "audio/mpeg", "flac": "audio/flac", "m4a": "audio/mp4",
		"ogg": "audio/ogg", "opus": "audio/opus", "wav": "audio/x-wav",
		"xyz": "application/octet-stream", "": "",
	}
	for suf, want := range cases {
		if got := audioContentType(suf); got != want {
			t.Errorf("audioContentType(%q) = %q, want %q", suf, got, want)
		}
	}
}
