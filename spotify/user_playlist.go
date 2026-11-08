package spotify

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

const spotifyUserPlaylistPageSize = 50

func GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	return defaultSpotify.GetUserPlaylists(page, limit)
}

func (s *Spotify) GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	offset, pageSize := normalizeSpotifyPage(page, limit, 20, spotifyUserPlaylistPageSize)
	result := make([]model.Playlist, 0, pageSize)
	requestOffset := offset

	for len(result) < pageSize {
		requestSize := pageSize - len(result)
		if requestSize > spotifyUserPlaylistPageSize {
			requestSize = spotifyUserPlaylistPageSize
		}

		query := url.Values{}
		query.Set("limit", strconv.Itoa(requestSize))
		query.Set("offset", strconv.Itoa(requestOffset))

		var response spotifyPaging[spotifyPlaylist]
		if err := s.getJSON(context.Background(), "/me/playlists", query, true, &response); err != nil {
			return nil, fmt.Errorf("spotify user playlists: %w", err)
		}
		requestOffset += len(response.Items)
		for _, playlist := range response.Items {
			if strings.TrimSpace(playlist.ID) == "" {
				continue
			}
			result = append(result, spotifyPlaylistFromAPI(playlist))
			if len(result) == pageSize {
				break
			}
		}
		if len(response.Items) == 0 || len(response.Items) < requestSize {
			break
		}
		if response.Total > 0 && requestOffset >= response.Total {
			break
		}
	}
	return result, nil
}
