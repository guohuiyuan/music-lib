package kugou

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

// ... (常量和结构体保持不变)
const (
	MobileUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1"
	MobileReferer   = "http://m.kugou.com"
)

type Kugou struct {
	cookie string
}

func New(cookie string) *Kugou { return &Kugou{cookie: cookie} }
var defaultKugou = New("")
func Search(keyword string) ([]model.Song, error) { return defaultKugou.Search(keyword) }
func GetDownloadURL(s *model.Song) (string, error) { return defaultKugou.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error) { return defaultKugou.GetLyrics(s) }

// Search 搜索歌曲
func (k *Kugou) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("platform", "WebFilter")
	params.Set("format", "json")
	params.Set("page", "1")
	params.Set("pagesize", "10")

	apiURL := "http://songsearch.kugou.com/song_search_v2?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Cookie", k.cookie),
	)
	if err != nil { return nil, err }

	var resp struct {
		Data struct {
			Lists []struct {
				Scid       interface{} `json:"Scid"`
				SongName   string      `json:"SongName"`
				SingerName string      `json:"SingerName"`
				AlbumName  string      `json:"AlbumName"`
				Duration   int         `json:"Duration"`
				FileHash   string      `json:"FileHash"`
				SQFileHash string      `json:"SQFileHash"`
				HQFileHash string      `json:"HQFileHash"`
				FileSize   interface{} `json:"FileSize"`
				Image      string      `json:"Image"`
				PayType    int         `json:"PayType"`
				Privilege  int         `json:"Privilege"`
			} `json:"lists"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}

	var songs []model.Song
	for _, item := range resp.Data.Lists {
		if item.Privilege == 10 { continue }
		if item.FileHash == "" && item.SQFileHash == "" && item.HQFileHash == "" { continue }

		finalHash := item.FileHash
		if isValidHash(item.SQFileHash) {
			finalHash = item.SQFileHash
		} else if isValidHash(item.HQFileHash) {
			finalHash = item.HQFileHash
		}

		var size int64
		switch v := item.FileSize.(type) {
		case float64: size = int64(v)
		case int: size = int64(v)
		case string:
			if i, err := strconv.ParseInt(v, 10, 64); err == nil { size = i }
		}

		bitrate := 0
		if item.Duration > 0 && size > 0 {
			bitrate = int(size * 8 / 1000 / int64(item.Duration))
		}

		coverURL := strings.Replace(item.Image, "{size}", "240", 1)

		songs = append(songs, model.Song{
			Source:   "kugou",
			ID:       finalHash,
			Name:     item.SongName,
			Artist:   item.SingerName,
			Album:    item.AlbumName,
			Duration: item.Duration,
			Size:     size,
			Bitrate:  bitrate,
			Cover:    coverURL,
			// 核心修改：存入 Extra
			Extra: map[string]string{
				"hash": finalHash,
			},
		})
	}
	return songs, nil
}

// GetDownloadURL 获取下载链接
func (k *Kugou) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "kugou" { return "", errors.New("source mismatch") }

	// 核心修改：优先从 Extra 获取
	hash := s.ID
	if s.Extra != nil && s.Extra["hash"] != "" {
		hash = s.Extra["hash"]
	}

	params := url.Values{}
	params.Set("cmd", "playInfo")
	params.Set("hash", hash)

	apiURL := "http://m.kugou.com/app/i/getSongInfo.php?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
	)
	if err != nil { return "", err }

	var resp struct {
		URL      string      `json:"url"`
		BitRate  int         `json:"bitRate"`
		ExtName  string      `json:"extName"`
		AlbumImg string      `json:"album_img"`
		Error    interface{} `json:"error"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("json parse error: %w", err)
	}

	if resp.URL == "" {
		return "", errors.New("download url not found (might be paid song)")
	}

	return resp.URL, nil
}

// GetLyrics 获取歌词
func (k *Kugou) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "kugou" { return "", errors.New("source mismatch") }

	// 核心修改：优先从 Extra 获取
	hash := s.ID
	if s.Extra != nil && s.Extra["hash"] != "" {
		hash = s.Extra["hash"]
	}

	searchURL := fmt.Sprintf("http://krcs.kugou.com/search?ver=1&client=mobi&duration=&hash=%s&album_audio_id=", hash)

	body, err := utils.Get(searchURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
	)
	if err != nil { return "", err }

	var searchResp struct {
		Status     int `json:"status"`
		Candidates []struct {
			ID        interface{} `json:"id"`
			AccessKey string      `json:"accesskey"`
			Song      string      `json:"song"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(body, &searchResp); err != nil {
		return "", fmt.Errorf("search lyrics json parse error: %w", err)
	}

	if len(searchResp.Candidates) == 0 { return "", errors.New("lyrics not found") }

	candidate := searchResp.Candidates[0]
	downloadURL := fmt.Sprintf("http://lyrics.kugou.com/download?ver=1&client=pc&id=%v&accesskey=%s&fmt=lrc&charset=utf8", candidate.ID, candidate.AccessKey)

	lrcBody, err := utils.Get(downloadURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
	)
	if err != nil { return "", err }

	var downloadResp struct {
		Status  int    `json:"status"`
		Content string `json:"content"`
		Fmt     string `json:"fmt"`
	}
	if err := json.Unmarshal(lrcBody, &downloadResp); err != nil {
		return "", fmt.Errorf("download lyrics json parse error: %w", err)
	}
	if downloadResp.Content == "" { return "", errors.New("lyrics content is empty") }

	decodedBytes, err := base64.StdEncoding.DecodeString(downloadResp.Content)
	if err != nil { return "", fmt.Errorf("base64 decode error: %w", err) }

	return string(decodedBytes), nil
}

// ... (辅助函数保持不变)
func isValidHash(h string) bool {
	return h != "" && h != "00000000000000000000000000000000"
}