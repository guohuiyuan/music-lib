package soda

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func GetDownloadInfo(s *model.Song) (*DownloadInfo, error) { return defaultSoda.GetDownloadInfo(s) }

func GetDownloadURL(s *model.Song) (string, error) { return defaultSoda.GetDownloadURL(s) }

func Download(s *model.Song, outputPath string) error { return defaultSoda.Download(s, outputPath) }

func (s *Soda) GetDownloadInfo(song *model.Song) (*DownloadInfo, error) {
	if strings.Contains(song.URL, "#auth=") {
		parts := strings.Split(song.URL, "#auth=")
		if len(parts) == 2 {
			auth, _ := url.QueryUnescape(parts[1])
			return &DownloadInfo{
				URL:      parts[0],
				PlayAuth: auth,
				Format:   song.Ext,
				Size:     song.Size,
			}, nil
		}
	}

	if song.Source != "soda" {
		return nil, errors.New("source mismatch")
	}

	trackID := song.ID
	if song.Extra != nil && song.Extra["track_id"] != "" {
		trackID = song.Extra["track_id"]
	}

	params := url.Values{}
	params.Set("track_id", trackID)
	params.Set("media_type", "track")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	v2URL := "https://api.qishui.com/luna/pc/track_v2?" + params.Encode()
	v2Body, err := utils.Get(v2URL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, err
	}

	var v2Resp struct {
		TrackPlayer struct {
			URLPlayerInfo string `json:"url_player_info"`
		} `json:"track_player"`
	}
	if err := json.Unmarshal(v2Body, &v2Resp); err != nil {
		return nil, fmt.Errorf("parse track_v2 response error: %w", err)
	}
	if v2Resp.TrackPlayer.URLPlayerInfo == "" {
		return nil, errors.New("player info url not found")
	}

	return s.fetchPlayerInfo(v2Resp.TrackPlayer.URLPlayerInfo)
}

// GetDownloadURL 返回下载链接
func (s *Soda) GetDownloadURL(song *model.Song) (string, error) {
	info, err := s.GetDownloadInfo(song)
	if err != nil {
		return "", err
	}
	return info.URL + "#auth=" + url.QueryEscape(info.PlayAuth), nil
}

// Download 下载并解密歌曲
func (s *Soda) Download(song *model.Song, outputPath string) error {
	info, err := s.GetDownloadInfo(song)
	if err != nil {
		return fmt.Errorf("get download info failed: %w", err)
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", info.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download status: %d", resp.StatusCode)
	}

	encryptedData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	decryptedData, err := DecryptAudio(encryptedData, info.PlayAuth)
	if err != nil {
		return fmt.Errorf("decrypt failed: %w", err)
	}

	err = os.WriteFile(outputPath, decryptedData, 0644)
	if err != nil {
		return err
	}
	return nil
}
