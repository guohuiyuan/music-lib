package soda

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/guohuiyuan/music-lib/model"
)

func Search(keyword string) ([]model.Song, error) { return defaultSoda.Search(keyword) }

func Parse(link string) (*model.Song, error) { return defaultSoda.Parse(link) }

// Search 搜索歌曲 (PC API)
func (s *Soda) Search(keyword string) ([]model.Song, error) {
	body, err := s.fetchTrackSearch(keyword)
	if err != nil {
		return nil, err
	}

	var resp struct {
		StatusCode int `json:"status_code"`
		StatusInfo struct {
			StatusMsg string `json:"status_msg"`
		} `json:"status_info"`
		ResultGroups []struct {
			Data []struct {
				Entity struct {
					Track sodaTrack `json:"track"`
				} `json:"entity"`
			} `json:"data"`
		} `json:"result_groups"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		if len(bytes.TrimSpace(body)) == 0 {
			return nil, errors.New("soda search returned an empty response")
		}
		return nil, fmt.Errorf("soda search json parse error: %w", err)
	}
	if resp.StatusCode != 0 {
		msg := resp.StatusInfo.StatusMsg
		if msg == "" {
			msg = "unknown error"
		}
		return nil, fmt.Errorf("soda search api error: status_code=%d status_msg=%s", resp.StatusCode, msg)
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
