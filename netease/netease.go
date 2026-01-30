package netease

import (
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
	Referer     = "http://music.163.com/"
	SearchAPI   = "http://music.163.com/api/linux/forward"
	DownloadAPI = "http://music.163.com/weapi/song/enhance/player/url"
)

type Netease struct {
	cookie string
}

func New(cookie string) *Netease { return &Netease{cookie: cookie} }
var defaultNetease = New("")
func Search(keyword string) ([]model.Song, error) { return defaultNetease.Search(keyword) }
func GetDownloadURL(s *model.Song) (string, error) { return defaultNetease.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error) { return defaultNetease.GetLyrics(s) }

// Search 搜索歌曲
func (n *Netease) Search(keyword string) ([]model.Song, error) {
	// ... (参数构造和请求发送保持不变)
	eparams := map[string]interface{}{
		"method": "POST",
		"url":    "http://music.163.com/api/cloudsearch/pc",
		"params": map[string]interface{}{"s": keyword, "type": 1, "offset": 0, "limit": 10},
	}
	eparamsJSON, _ := json.Marshal(eparams)
	encryptedParam := EncryptLinux(string(eparamsJSON))
	form := url.Values{}
	form.Set("eparams", encryptedParam)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
	}

	body, err := utils.Post(SearchAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil { return nil, err }

	var resp struct {
		Result struct {
			Songs []struct {
				ID        int `json:"id"`
				Name      string `json:"name"`
				Ar        []struct { Name string `json:"name"` } `json:"ar"`
				Al        struct { Name string `json:"name"`; PicURL string `json:"picUrl"` } `json:"al"`
				Dt        int `json:"dt"`
				Privilege struct { Fl int `json:"fl"`; Pl int `json:"pl"` } `json:"privilege"`
				H         struct { Size int64 `json:"size"` } `json:"h"`
				M         struct { Size int64 `json:"size"` } `json:"m"`
				L         struct { Size int64 `json:"size"` } `json:"l"`
			} `json:"songs"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease json parse error: %w", err)
	}

	var songs []model.Song
	for _, item := range resp.Result.Songs {
		if item.Privilege.Fl == 0 { continue }

		var size int64
		if item.Privilege.Fl >= 320000 && item.H.Size > 0 { size = item.H.Size
		} else if item.Privilege.Fl >= 192000 && item.M.Size > 0 { size = item.M.Size
		} else { size = item.L.Size }

		duration := item.Dt / 1000
		bitrate := 128
		if duration > 0 && size > 0 { bitrate = int(size * 8 / 1000 / int64(duration)) }

		var artistNames []string
		for _, ar := range item.Ar { artistNames = append(artistNames, ar.Name) }

		songs = append(songs, model.Song{
			Source:   "netease",
			ID:       strconv.Itoa(item.ID),
			Name:     item.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    item.Al.Name,
			Duration: duration,
			Size:     size,
			Bitrate:  bitrate,
			Cover:    item.Al.PicURL,
			// 核心修改：存入 Extra
			Extra: map[string]string{
				"song_id": strconv.Itoa(item.ID),
			},
		})
	}
	return songs, nil
}

// GetDownloadURL 获取下载链接
func (n *Netease) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "netease" { return "", errors.New("source mismatch") }

	// 核心修改：优先从 Extra 获取
	songID := s.ID
	if s.Extra != nil && s.Extra["song_id"] != "" {
		songID = s.Extra["song_id"]
	}

	reqData := map[string]interface{}{
		"ids": []string{songID}, 
		"br":  320000,
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
	}

	body, err := utils.Post(DownloadAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil { return "", err }

	var resp struct {
		Data []struct { URL string `json:"url"`; Code int `json:"code"`; Br int `json:"br"` } `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil { return "", fmt.Errorf("json parse error: %w", err) }
	if len(resp.Data) == 0 || resp.Data[0].URL == "" {
		return "", errors.New("download url not found (might be vip or copyright restricted)")
	}
	return resp.Data[0].URL, nil
}

// GetLyrics 获取歌词
func (n *Netease) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "netease" { return "", errors.New("source mismatch") }

	// 核心修改：优先从 Extra 获取
	songID := s.ID
	if s.Extra != nil && s.Extra["song_id"] != "" {
		songID = s.Extra["song_id"]
	}

	reqData := map[string]interface{}{
		"csrf_token": "",
		"id":         songID,
		"lv":         -1,
		"tv":         -1,
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
	}

	lyricAPI := "https://music.163.com/weapi/song/lyric"
	body, err := utils.Post(lyricAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil { return "", err }

	var resp struct {
		Code int `json:"code"`
		Lrc  struct { Lyric string `json:"lyric"` } `json:"lrc"`
	}
	if err := json.Unmarshal(body, &resp); err != nil { return "", fmt.Errorf("json parse error: %w", err) }
	if resp.Code != 200 { return "", fmt.Errorf("netease api error code: %d", resp.Code) }
	return resp.Lrc.Lyric, nil
}