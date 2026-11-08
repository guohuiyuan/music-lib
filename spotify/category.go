package spotify

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

const spotifyCategoryLimit = 50

type spotifyCategoriesResponse struct {
	Categories spotifyPaging[spotifyCategory] `json:"categories"`
}

type spotifyCategoryPlaylistsResponse struct {
	Playlists spotifyPaging[spotifyPlaylist] `json:"playlists"`
}

type spotifyFeaturedPlaylistsResponse struct {
	Message   string                         `json:"message"`
	Playlists spotifyPaging[spotifyPlaylist] `json:"playlists"`
}

var spotifyFallbackCategories = []model.PlaylistCategory{
	{ID: "pop", Name: "流行", Group: "曲风", Source: "spotify", Hot: true},
	{ID: "hiphop", Name: "嘻哈", Group: "曲风", Source: "spotify", Hot: true},
	{ID: "rock", Name: "摇滚", Group: "曲风", Source: "spotify", Hot: true},
	{ID: "indie", Name: "独立音乐", Group: "曲风", Source: "spotify"},
	{ID: "dance", Name: "舞曲", Group: "曲风", Source: "spotify", Hot: true},
	{ID: "rnb", Name: "R&B", Group: "曲风", Source: "spotify"},
	{ID: "chill", Name: "轻松", Group: "场景", Source: "spotify"},
	{ID: "workout", Name: "运动", Group: "场景", Source: "spotify"},
	{ID: "party", Name: "派对", Group: "场景", Source: "spotify"},
	{ID: "sleep", Name: "助眠", Group: "场景", Source: "spotify"},
	{ID: "country", Name: "乡村", Group: "曲风", Source: "spotify"},
	{ID: "latin", Name: "拉丁", Group: "曲风", Source: "spotify"},
}

func GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return defaultSpotify.GetPlaylistCategories()
}

func GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return defaultSpotify.GetCategoryPlaylists(categoryID, page, limit)
}

func GetRecommendedPlaylists() ([]model.Playlist, error) {
	return defaultSpotify.GetRecommendedPlaylists()
}

func (s *Spotify) GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	query := url.Values{}
	query.Set("limit", strconv.Itoa(spotifyCategoryLimit))
	query.Set("country", s.marketOrDefault())
	query.Set("locale", "zh_CN")

	var response spotifyCategoriesResponse
	err := s.getJSON(context.Background(), "/browse/categories", query, false, &response)
	if err != nil {
		if isSpotifyBrowseUnavailable(err) {
			return cloneSpotifyCategories(spotifyFallbackCategories), nil
		}
		return nil, fmt.Errorf("spotify playlist categories: %w", err)
	}

	categories := make([]model.PlaylistCategory, 0, len(response.Categories.Items))
	for _, category := range response.Categories.Items {
		id := strings.TrimSpace(category.ID)
		name := strings.TrimSpace(category.Name)
		if id == "" || name == "" {
			continue
		}
		categories = append(categories, model.PlaylistCategory{
			ID:     id,
			Name:   name,
			Group:  "Spotify",
			Source: "spotify",
			Extra:  map[string]string{"icon": spotifyCover(category.Icons)},
		})
	}
	if len(categories) == 0 {
		return cloneSpotifyCategories(spotifyFallbackCategories), nil
	}
	return categories, nil
}

func (s *Spotify) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" {
		categoryID = "pop"
	}
	offset, pageSize := normalizeSpotifyPage(page, limit, 20, 50)

	query := url.Values{}
	query.Set("country", s.marketOrDefault())
	query.Set("limit", strconv.Itoa(pageSize))
	query.Set("offset", strconv.Itoa(offset))

	var response spotifyCategoryPlaylistsResponse
	path := "/browse/categories/" + url.PathEscape(categoryID) + "/playlists"
	if err := s.getJSON(context.Background(), path, query, false, &response); err != nil {
		if !isSpotifyBrowseUnavailable(err) {
			return nil, fmt.Errorf("spotify category playlists: %w", err)
		}
		return s.searchCategoryFallback(categoryID, pageSize)
	}
	return spotifyPlaylistsFromPaging(response.Playlists, pageSize), nil
}

func (s *Spotify) GetRecommendedPlaylists() ([]model.Playlist, error) {
	query := url.Values{}
	query.Set("country", s.marketOrDefault())
	query.Set("locale", "zh_CN")
	query.Set("limit", strconv.Itoa(spotifyCategoryLimit))

	var response spotifyFeaturedPlaylistsResponse
	if err := s.getJSON(context.Background(), "/browse/featured-playlists", query, false, &response); err != nil {
		if !isSpotifyBrowseUnavailable(err) {
			return nil, fmt.Errorf("spotify recommended playlists: %w", err)
		}
		return spotifyTopListPlaylists(), nil
	}
	playlists := spotifyPlaylistsFromPaging(response.Playlists, spotifyCategoryLimit)
	if len(playlists) == 0 {
		return spotifyTopListPlaylists(), nil
	}
	return playlists, nil
}

func (s *Spotify) searchCategoryFallback(categoryID string, limit int) ([]model.Playlist, error) {
	name := categoryID
	for _, category := range spotifyFallbackCategories {
		if category.ID == categoryID {
			name = category.Name
			break
		}
	}

	query := url.Values{}
	query.Set("q", name)
	query.Set("type", "playlist")
	query.Set("limit", strconv.Itoa(limit))
	query.Set("market", s.marketOrDefault())

	var response spotifySearchResponse
	if err := s.getJSON(context.Background(), "/search", query, false, &response); err != nil {
		return nil, fmt.Errorf("spotify category playlist fallback: %w", err)
	}
	return spotifyPlaylistsFromPaging(response.Playlists, limit), nil
}

func spotifyPlaylistsFromPaging(page spotifyPaging[spotifyPlaylist], limit int) []model.Playlist {
	if limit <= 0 || limit > len(page.Items) {
		limit = len(page.Items)
	}
	playlists := make([]model.Playlist, 0, limit)
	for _, playlist := range page.Items[:limit] {
		if strings.TrimSpace(playlist.ID) == "" {
			continue
		}
		playlists = append(playlists, spotifyPlaylistFromAPI(playlist))
	}
	return playlists
}

func cloneSpotifyCategories(source []model.PlaylistCategory) []model.PlaylistCategory {
	result := make([]model.PlaylistCategory, len(source))
	copy(result, source)
	return result
}

func isSpotifyBrowseUnavailable(err error) bool {
	var apiErr *spotifyHTTPError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.StatusCode == http.StatusForbidden || apiErr.StatusCode == http.StatusNotFound
}

func isSpotifyEndpointUnavailable(err error) bool {
	var apiErr *spotifyHTTPError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}
