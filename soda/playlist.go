package soda

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
)

func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultSoda.SearchPlaylist(keyword)
}

func GetPlaylistSongs(id string) ([]model.Song, error) {
	// 复用 fetchPlaylistDetail，只返回歌曲列表
	_, songs, err := defaultSoda.fetchPlaylistDetail(id)
	return songs, err
}

func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultSoda.ParsePlaylist(link)
}

// GetRecommendedPlaylists [新增] 获取推荐歌单 (空实现)
func GetRecommendedPlaylists() ([]model.Playlist, error) {
	return defaultSoda.GetRecommendedPlaylists()
}

func GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return defaultSoda.GetPlaylistCategories()
}

func GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return defaultSoda.GetCategoryPlaylists(categoryID, page, limit)
}

func (s *Soda) GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return nil, model.ErrPlaylistCategoriesUnsupported
}

func (s *Soda) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return nil, model.ErrPlaylistCategoriesUnsupported
}

func (s *Soda) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	body, err := s.fetchAndroidSearch("playlist", keyword, 1, sodaAndroidSearchPageSize)
	if err != nil {
		return nil, err
	}

	var resp struct {
		ResultGroups []struct {
			Data []struct {
				Entity struct {
					Playlist sodaUserPlaylistItem `json:"playlist"`
				} `json:"entity"`
			} `json:"data"`
		} `json:"result_groups"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda playlist json parse error: %w", err)
	}

	var playlists []model.Playlist
	for _, group := range resp.ResultGroups {
		for _, item := range group.Data {
			pl := item.Entity.Playlist
			if pl.ID == "" {
				continue
			}

			creator := pl.Owner.PublicName
			if creator == "" {
				creator = pl.Owner.Nickname
			}

			playlists = append(playlists, model.Playlist{
				Source:      "soda",
				ID:          pl.ID,
				Name:        pl.Title,
				Cover:       sodaBuildImageURL(pl.URLCover, "~c5_300x300.jpg"),
				TrackCount:  pl.CountTracks,
				Creator:     creator,
				Description: pl.Desc,
				Link:        fmt.Sprintf("https://www.qishui.com/playlist/%s", pl.ID),
			})
		}
	}
	return playlists, nil
}

func (s *Soda) GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := s.fetchPlaylistDetail(id)
	return songs, err
}

func (s *Soda) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	playlistID, err := s.extractPlaylistID(link)
	if err != nil || playlistID == "" {
		return nil, nil, errors.New("invalid soda playlist link")
	}
	return s.fetchPlaylistDetail(playlistID)
}

// GetRecommendedPlaylists [新增] 获取推荐歌单 (空实现)
func (s *Soda) GetRecommendedPlaylists() ([]model.Playlist, error) {
	// 汽水音乐目前没有公开的每日推荐歌单 PC 接口
	return nil, errors.New("soda daily recommendation not supported")
}
