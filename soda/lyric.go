package soda

import (
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
)

func GetLyrics(s *model.Song) (string, error) { return defaultSoda.GetLyrics(s) }

// GetLyrics 获取歌词
func (s *Soda) GetLyrics(song *model.Song) (string, error) {
	if song.Source != "soda" {
		return "", errors.New("source mismatch")
	}

	trackID := song.ID
	if song.Extra != nil && song.Extra["track_id"] != "" {
		trackID = song.Extra["track_id"]
	}

	// PC track_v2 已下线，歌词同样走 SEO seo_track 接口（无需签名）。
	resp, err := s.fetchWebTrackV2(trackID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch lyric API: %w", err)
	}
	if resp.Lyric.Content == "" {
		return "", nil
	}

	return parseSodaLyric(resp.Lyric.Content), nil
}
