package main

import (
	"testing"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
)

type AlbumTestSuite struct {
	Name          string
	Keyword       string
	SearchAlbum   func(string) ([]model.Playlist, error)
	GetAlbumSongs func(string) ([]model.Song, error)
	ParseAlbum    func(string) (*model.Playlist, []model.Song, error)
}

func TestAlbumPlatforms(t *testing.T) {
	suites := []AlbumTestSuite{
		{
			Name:          "netease",
			Keyword:       "Taylor",
			SearchAlbum:   netease.SearchAlbum,
			GetAlbumSongs: netease.GetAlbumSongs,
			ParseAlbum:    netease.ParseAlbum,
		},
	}

	for _, suite := range suites {
		suite := suite
		t.Run(suite.Name, func(t *testing.T) {
			t.Parallel()

			albums, err := suite.SearchAlbum(suite.Keyword)
			if err != nil {
				t.Fatalf("[%s] SearchAlbum failed: %v", suite.Name, err)
			}
			if len(albums) == 0 {
				t.Fatalf("[%s] SearchAlbum returned no albums", suite.Name)
			}

			first := albums[0]
			if first.ID == "" {
				t.Fatalf("[%s] SearchAlbum returned empty album id", suite.Name)
			}

			songs, err := suite.GetAlbumSongs(first.ID)
			if err != nil {
				t.Fatalf("[%s] GetAlbumSongs failed: %v", suite.Name, err)
			}
			if len(songs) == 0 {
				t.Fatalf("[%s] GetAlbumSongs returned no songs", suite.Name)
			}

			if suite.ParseAlbum == nil || first.Link == "" {
				return
			}

			parsedAlbum, parsedSongs, err := suite.ParseAlbum(first.Link)
			if err != nil {
				t.Fatalf("[%s] ParseAlbum failed: %v", suite.Name, err)
			}
			if parsedAlbum == nil {
				t.Fatalf("[%s] ParseAlbum returned nil album", suite.Name)
			}
			if parsedAlbum.ID == "" {
				t.Fatalf("[%s] ParseAlbum returned empty album id", suite.Name)
			}
			if len(parsedSongs) == 0 {
				t.Fatalf("[%s] ParseAlbum returned no songs", suite.Name)
			}
		})
	}
}
