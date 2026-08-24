package kuwo

import (
	"net/url"
	"strings"
	"testing"
)

func TestKuwoPlaylistDetailURLUsesPageAndPageSize(t *testing.T) {
	raw := kuwoPlaylistDetailURL("3704397303", 2, 100)
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse playlist detail url: %v", err)
	}
	query := parsed.Query()
	if query.Get("pid") != "3704397303" {
		t.Fatalf("pid = %q, want 3704397303", query.Get("pid"))
	}
	if query.Get("pn") != "2" {
		t.Fatalf("pn = %q, want 2", query.Get("pn"))
	}
	if query.Get("rn") != "100" {
		t.Fatalf("rn = %q, want 100", query.Get("rn"))
	}
}

func TestKuwoPlaylistSongFromMusicNormalizesFields(t *testing.T) {
	song := kuwoPlaylistSongFromMusic(kuwoPlaylistMusic{
		Id:         " 123 ",
		Name:       "Main",
		Artist:     "",
		SongName:   "Fallback",
		ArtistName: "Artist",
		Album:      "Album",
		AlbumPic:   "_100._150.",
		Duration:   "235",
	})
	if song.ID != "123" {
		t.Fatalf("ID = %q, want 123", song.ID)
	}
	if song.Name != "Main" {
		t.Fatalf("Name = %q, want Main", song.Name)
	}
	if song.Artist != "Artist" {
		t.Fatalf("Artist = %q, want Artist", song.Artist)
	}
	if song.Duration != 235 {
		t.Fatalf("Duration = %d, want 235", song.Duration)
	}
	if !strings.Contains(song.Cover, "_500.") {
		t.Fatalf("Cover should use high-res image: %q", song.Cover)
	}
	if song.Extra["rid"] != "123" {
		t.Fatalf("Extra rid = %q, want 123", song.Extra["rid"])
	}
}
