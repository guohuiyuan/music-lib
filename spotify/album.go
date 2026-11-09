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
	if !s.hasCatalogAuthentication() {
		albums, err := s.searchAlbumsFromWebPlayer(
			context.Background(),
			keyword,
			spotifyAlbumSearchLimit,
		)
		if err != nil {
			return nil, fmt.Errorf("spotify album search anonymous fallback: %w", err)
		}
		return albums, nil
	}

	query := url.Values{}
	query.Set("q", keyword)
	query.Set("type", "album")
	query.Set("limit", strconv.Itoa(spotifyAlbumSearchLimit))
	query.Set("market", s.marketOrDefault())

	var response spotifySearchResponse
	if err := s.getJSON(context.Background(), "/search", query, false, &response); err != nil {
		if spotifyAllowsAnonymousSearchFallback(err) {
			albums, fallbackErr := s.searchAlbumsFromWebPlayer(
				context.Background(),
				keyword,
				spotifyAlbumSearchLimit,
			)
			if fallbackErr == nil {
				return albums, nil
			}
			return nil, fmt.Errorf(
				"spotify album search: official API: %v; anonymous fallback: %w",
				err,
				fallbackErr,
			)
		}
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
	if !s.hasCatalogAuthentication() {
		_, songs, fallbackErr := s.fetchAlbumFromPublic(albumID)
		if fallbackErr != nil {
			return nil, fmt.Errorf(
				"spotify album %s public fallback: %w",
				albumID,
				fallbackErr,
			)
		}
		return songs, nil
	}
	album, err := s.fetchAlbum(albumID)
	if err != nil {
		if spotifyAllowsPublicFallback(err) {
			_, songs, fallbackErr := s.fetchAlbumFromPublic(albumID)
			if fallbackErr == nil {
				return songs, nil
			}
			return nil, fmt.Errorf(
				"spotify album %s: official API: %v; public fallback: %w",
				albumID,
				err,
				fallbackErr,
			)
		}
		return nil, err
	}
	songs, err := s.fetchAlbumTracks(*album)
	if err != nil && spotifyAllowsPublicFallback(err) {
		_, fallbackSongs, fallbackErr := s.fetchAlbumFromPublic(albumID)
		if fallbackErr == nil {
			return fallbackSongs, nil
		}
		return nil, fmt.Errorf(
			"spotify album tracks %s: official API: %v; public fallback: %w",
			albumID,
			err,
			fallbackErr,
		)
	}
	return songs, err
}

func (s *Spotify) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	albumID, err := extractSpotifyID(link, "album")
	if err != nil {
		return nil, nil, err
	}
	if !s.hasCatalogAuthentication() {
		playlist, songs, fallbackErr := s.fetchAlbumFromPublic(albumID)
		if fallbackErr != nil {
			return nil, nil, fmt.Errorf(
				"spotify album %s public fallback: %w",
				albumID,
				fallbackErr,
			)
		}
		return playlist, songs, nil
	}
	album, err := s.fetchAlbum(albumID)
	if err != nil {
		if spotifyAllowsPublicFallback(err) {
			playlist, songs, fallbackErr := s.fetchAlbumFromPublic(albumID)
			if fallbackErr == nil {
				return playlist, songs, nil
			}
			return nil, nil, fmt.Errorf(
				"spotify album %s: official API: %v; public fallback: %w",
				albumID,
				err,
				fallbackErr,
			)
		}
		return nil, nil, err
	}
	songs, err := s.fetchAlbumTracks(*album)
	if err != nil {
		if spotifyAllowsPublicFallback(err) {
			playlist, fallbackSongs, fallbackErr := s.fetchAlbumFromPublic(
				albumID,
			)
			if fallbackErr == nil {
				return playlist, fallbackSongs, nil
			}
			return nil, nil, fmt.Errorf(
				"spotify album tracks %s: official API: %v; public fallback: %w",
				albumID,
				err,
				fallbackErr,
			)
		}
		return nil, nil, err
	}

	playlist := spotifyAlbumFromAPI(*album)
	if playlist.TrackCount <= 0 || playlist.TrackCount > len(songs) && len(songs) > 0 {
		playlist.TrackCount = len(songs)
	}
	return &playlist, songs, nil
}

func (s *Spotify) fetchAlbumFromPublic(
	id string,
) (*model.Playlist, []model.Song, error) {
	entity, err := s.fetchSpotifyEmbedEntityContextWithTimeout(
		context.Background(),
		"album",
		id,
		defaultRequestTimeout,
	)
	if err != nil {
		return nil, nil, err
	}

	albumID := firstSpotifyNonEmpty(entity.ID, id)
	name := firstSpotifyNonEmpty(
		cleanSpotifyEmbedText(entity.Name),
		cleanSpotifyEmbedText(entity.Title),
	)
	if albumID == "" || name == "" {
		return nil, nil, errors.New(
			"spotify public album metadata did not contain an id and name",
		)
	}
	cover := spotifyCover(entity.VisualIdentity.Image)
	if cover == "" {
		cover = spotifyCover(entity.CoverArt.Sources)
	}
	creator := spotifyArtists(entity.Artists)
	if creator == "" {
		creator = cleanSpotifyEmbedText(entity.Subtitle)
	}
	releaseDate := strings.TrimSpace(entity.ReleaseDate)
	playlist := model.Playlist{
		Source:      "spotify",
		ID:          albumID,
		Name:        name,
		Cover:       cover,
		TrackCount:  len(entity.TrackList),
		Creator:     creator,
		Description: releaseDate,
		Link:        spotifyAlbumLink(albumID),
		Extra: map[string]string{
			"type":            "album",
			"spotify_id":      albumID,
			"release_date":    releaseDate,
			"metadata_source": "embed",
			"auth_source":     "anonymous_public_embed",
		},
	}

	songs := spotifySongsFromEmbed(*entity)
	for index := range songs {
		if songs[index].Album == "" {
			songs[index].Album = name
		}
		if songs[index].AlbumID == "" {
			songs[index].AlbumID = albumID
		}
		if songs[index].Cover == "" {
			songs[index].Cover = cover
		}
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
