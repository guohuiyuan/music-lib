package soda

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
)

func Search(keyword string) ([]model.Song, error) { return defaultSoda.Search(keyword) }

func Parse(link string) (*model.Song, error) { return defaultSoda.Parse(link) }

// Search 搜索歌曲 (PC API)
func (s *Soda) Search(keyword string) ([]model.Song, error) {
	body, err := s.fetchAndroidSearch("track", keyword, 1, sodaAndroidSearchPageSize)
	if err != nil {
		return nil, err
	}

	var resp struct {
		ResultGroups []struct {
			Data []struct {
				Entity struct {
					Track sodaTrack `json:"track"`
				} `json:"entity"`
			} `json:"data"`
		} `json:"result_groups"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda search json parse error: %w", err)
	}

	var songs []model.Song
	seen := make(map[string]bool)
	for _, group := range resp.ResultGroups {
		for _, item := range group.Data {
			track := item.Entity.Track
			if track.ID == "" || seen[track.ID] {
				continue
			}
			seen[track.ID] = true
			songs = append(songs, sodaBuildSongFromTrack(track))
		}
	}
	return songs, nil
}

// Parse 解析链接并获取完整信息
func (s *Soda) Parse(link string) (*model.Song, error) {
	trackID, err := s.extractTrackID(link)
	if err != nil || trackID == "" {
		return nil, errors.New("invalid soda link")
	}
	return s.fetchSongDetail(trackID)
}
