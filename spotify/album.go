package spotify

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

const (
	spotifyAlbumSearchLimit = 20
	spotifyAlbumPageSize    = 50
)

func SearchAlbum(keyword string) ([]model.Playlist, error) {
	return defaultSpotify.SearchAlbum(keyword)
}

func GetAlbumSongs(id string) ([]model.Song, error) {
	return defaultSpotify.GetAlbumSongs(id)
}

func ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	return defaultSpotify.ParseAlbum(link)
}

func (s *Spotify) SearchAlbum(keyword string) ([]model.Playlist, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, errors.New("spotify album search keyword is empty")
	}

	query := url.Values{}
	query.Set("q", keyword)
	query.Set("type", "album")
	query.Set("limit", strconv.Itoa(spotifyAlbumSearchLimit))
	query.Set("market", s.marketOrDefault())

	var response spotifySearchResponse
	if err := s.getJSON(context.Background(), "/search", query, false, &response); err != nil {
		return nil, fmt.Errorf("spotify album search: %w", err)
	}

	albums := make([]model.Playlist, 0, len(response.Albums.Items))
	for _, album := range response.Albums.Items {
		if strings.TrimSpace(album.ID) == "" {
			continue
		}
		albums = append(albums, spotifyAlbumFromAPI(album))
	}
	return albums, nil
}

func (s *Spotify) GetAlbumSongs(id string) ([]model.Song, error) {
	albumID, err := extractSpotifyID(id, "album")
	if err != nil {
		return nil, err
	}
	album, err := s.fetchAlbum(albumID)
	if err != nil {
		return nil, err
	}
	return s.fetchAlbumTracks(*album)
}

func (s *Spotify) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	albumID, err := extractSpotifyID(link, "album")
	if err != nil {
		return nil, nil, err
	}
	album, err := s.fetchAlbum(albumID)
	if err != nil {
		return nil, nil, err
	}
	songs, err := s.fetchAlbumTracks(*album)
	if err != nil {
		return nil, nil, err
	}

	playlist := spotifyAlbumFromAPI(*album)
	if playlist.TrackCount <= 0 || playlist.TrackCount > len(songs) && len(songs) > 0 {
		playlist.TrackCount = len(songs)
	}
	return &playlist, songs, nil
}

func (s *Spotify) fetchAlbum(id string) (*spotifyAlbum, error) {
	query := url.Values{}
	query.Set("market", s.marketOrDefault())

	var album spotifyAlbum
	if err := s.getJSON(context.Background(), "/albums/"+url.PathEscape(id), query, false, &album); err != nil {
		return nil, fmt.Errorf("spotify album %s: %w", id, err)
	}
	if strings.TrimSpace(album.ID) == "" {
		return nil, fmt.Errorf("spotify album %s was not found", id)
	}
	return &album, nil
}

func (s *Spotify) fetchAlbumTracks(album spotifyAlbum) ([]model.Song, error) {
	songs := make([]model.Song, 0, album.TotalTracks)
	for offset := 0; ; {
		query := url.Values{}
		query.Set("limit", strconv.Itoa(spotifyAlbumPageSize))
		query.Set("offset", strconv.Itoa(offset))
		query.Set("market", s.marketOrDefault())

		var page spotifyPaging[spotifyTrack]
		path := "/albums/" + url.PathEscape(album.ID) + "/tracks"
		if err := s.getJSON(context.Background(), path, query, false, &page); err != nil {
			return nil, fmt.Errorf("spotify album tracks %s: %w", album.ID, err)
		}
		for _, track := range page.Items {
			if strings.TrimSpace(track.ID) == "" {
				continue
			}
			songs = append(songs, spotifySongFromTrack(track, &album))
		}
		if len(page.Items) == 0 || len(songs) >= page.Total || page.Next == "" {
			break
		}
		offset += len(page.Items)
	}
	return songs, nil
}
