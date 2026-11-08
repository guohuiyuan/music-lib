package spotify

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
)

type spotifyTopListDefinition struct {
	ID           string
	PlaylistID   string
	Name         string
	Description  string
	ChartCountry string
}

var spotifyTopListDefinitions = []spotifyTopListDefinition{
	{
		ID:           "global-top-50",
		PlaylistID:   "37i9dQZEVXbMDoHDwVN2tF",
		Name:         "Global Top 50",
		Description:  "Spotify 全球热门歌曲 Top 50",
		ChartCountry: "global",
	},
	{
		ID:           "us-top-50",
		PlaylistID:   "37i9dQZEVXbLRQDuF5jeBp",
		Name:         "United States Top 50",
		Description:  "Spotify 美国热门歌曲 Top 50",
		ChartCountry: "us",
	},
	{
		ID:           "uk-top-50",
		PlaylistID:   "37i9dQZEVXbLnolsZ8PSNw",
		Name:         "United Kingdom Top 50",
		Description:  "Spotify 英国热门歌曲 Top 50",
		ChartCountry: "gb",
	},
	{
		ID:           "jp-top-50",
		PlaylistID:   "37i9dQZEVXbKXQ4mDTEBXq",
		Name:         "Japan Top 50",
		Description:  "Spotify 日本热门歌曲 Top 50",
		ChartCountry: "jp",
	},
	{
		ID:           "au-top-50",
		PlaylistID:   "37i9dQZEVXbJPcfkRz0wJ0",
		Name:         "Australia Top 50",
		Description:  "Spotify 澳大利亚热门歌曲 Top 50",
		ChartCountry: "au",
	},
	{
		ID:           "ca-top-50",
		PlaylistID:   "37i9dQZEVXbKj23U1GF4IR",
		Name:         "Canada Top 50",
		Description:  "Spotify 加拿大热门歌曲 Top 50",
		ChartCountry: "ca",
	},
}

func GetTopLists() ([]model.TopList, error) {
	return defaultSpotify.GetTopLists()
}

func GetTopListSongs(topListID string) ([]model.Song, error) {
	return defaultSpotify.GetTopListSongs(topListID)
}

func GetHotSongs(limit int) ([]model.Song, error) {
	return defaultSpotify.GetHotSongs(limit)
}

func (s *Spotify) GetTopLists() ([]model.TopList, error) {
	result := make([]model.TopList, 0, len(spotifyTopListDefinitions))
	for _, definition := range spotifyTopListDefinitions {
		result = append(result, model.TopList{
			ID:              definition.ID,
			Name:            definition.Name,
			Description:     definition.Description,
			UpdateFrequency: "每日更新",
			TrackCount:      50,
			Source:          "spotify",
			Link:            spotifyPlaylistLink(definition.PlaylistID),
			Extra: map[string]string{
				"playlist_id":  definition.PlaylistID,
				"toplist_type": "official_playlist",
			},
		})
	}
	return result, nil
}

func (s *Spotify) GetTopListSongs(topListID string) ([]model.Song, error) {
	definition, err := resolveSpotifyTopList(topListID)
	if err != nil {
		return nil, err
	}
	songs, err := s.fetchPlaylistTracks(definition.PlaylistID)
	if err != nil {
		return nil, err
	}
	for index := range songs {
		if songs[index].Extra == nil {
			songs[index].Extra = make(map[string]string, 3)
		}
		songs[index].Extra["rank"] = strconv.Itoa(index + 1)
		songs[index].Extra["toplist_id"] = definition.ID
	}
	return songs, nil
}

func (s *Spotify) GetHotSongs(limit int) ([]model.Song, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 50 {
		limit = 50
	}
	songs, err := s.fetchPlaylistTracks(spotifyTopListDefinitions[0].PlaylistID)
	if err != nil {
		return nil, err
	}
	if len(songs) > limit {
		songs = songs[:limit]
	}
	for index := range songs {
		if songs[index].Extra == nil {
			songs[index].Extra = make(map[string]string, 3)
		}
		songs[index].Extra["rank"] = strconv.Itoa(index + 1)
		songs[index].Extra["toplist_id"] = spotifyTopListDefinitions[0].ID
	}
	return songs, nil
}

func spotifyTopListPlaylists() []model.Playlist {
	result := make([]model.Playlist, 0, len(spotifyTopListDefinitions))
	for _, definition := range spotifyTopListDefinitions {
		result = append(result, model.Playlist{
			ID:          definition.PlaylistID,
			Name:        definition.Name,
			Creator:     "Spotify",
			Description: definition.Description,
			Source:      "spotify",
			Link:        spotifyPlaylistLink(definition.PlaylistID),
			Extra: map[string]string{
				"type":        "playlist",
				"spotify_id":  definition.PlaylistID,
				"recommended": "true",
			},
		})
	}
	return result
}

func resolveSpotifyTopList(topListID string) (spotifyTopListDefinition, error) {
	raw := strings.TrimSpace(topListID)
	if raw == "" {
		return spotifyTopListDefinition{}, errors.New("spotify toplist id is empty")
	}
	for _, definition := range spotifyTopListDefinitions {
		if raw == definition.ID || raw == definition.PlaylistID || raw == "playlist:"+definition.PlaylistID {
			return definition, nil
		}
	}
	if strings.HasPrefix(raw, "playlist:") {
		playlistID, err := extractSpotifyID(strings.TrimPrefix(raw, "playlist:"), "playlist")
		if err == nil {
			return spotifyTopListDefinition{
				ID:          raw,
				PlaylistID:  playlistID,
				Name:        "Spotify Playlist",
				Description: "Spotify editorial playlist",
			}, nil
		}
	}
	if strings.Contains(raw, "open.spotify.com") {
		playlistID, err := extractSpotifyID(raw, "playlist")
		if err == nil {
			return spotifyTopListDefinition{
				ID:          "playlist:" + playlistID,
				PlaylistID:  playlistID,
				Name:        "Spotify Playlist",
				Description: "Spotify editorial playlist",
			}, nil
		}
	}
	return spotifyTopListDefinition{}, fmt.Errorf("unsupported spotify toplist id: %s", topListID)
}
