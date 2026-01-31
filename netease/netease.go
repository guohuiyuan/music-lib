package netease

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	Referer     = "http://music.163.com/"
	SearchAPI   = "http://music.163.com/api/linux/forward"
	DownloadAPI = "http://music.163.com/weapi/song/enhance/player/url"
	DetailAPI   = "https://music.163.com/weapi/v3/song/detail"
	PlaylistAPI = "https://music.163.com/weapi/v3/playlist/detail" // [新增] 歌单详情接口
)

type Netease struct {
	cookie string
}

func New(cookie string) *Netease { return &Netease{cookie: cookie} }

var defaultNetease = New("")

func Search(keyword string) ([]model.Song, error)             { return defaultNetease.Search(keyword) }
func SearchPlaylist(keyword string) ([]model.Playlist, error) { return defaultNetease.SearchPlaylist(keyword) }
func GetPlaylistSongs(playlistID string) ([]model.Song, error) {
	return defaultNetease.GetPlaylistSongs(playlistID)
} // [新增]
func GetDownloadURL(s *model.Song) (string, error) { return defaultNetease.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error)      { return defaultNetease.GetLyrics(s) }
func Parse(link string) (*model.Song, error)       { return defaultNetease.Parse(link) }

// Search 搜索歌曲
func (n *Netease) Search(keyword string) ([]model.Song, error) {
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
	if err != nil {
		return nil, err
	}

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
		// 简单的过滤逻辑，可根据需要调整
		if item.Privilege.Fl == 0 {
			continue
		}

		var size int64
		if item.Privilege.Fl >= 320000 && item.H.Size > 0 {
			size = item.H.Size
		} else if item.Privilege.Fl >= 192000 && item.M.Size > 0 {
			size = item.M.Size
		} else {
			size = item.L.Size
		}

		duration := item.Dt / 1000
		bitrate := 128
		if duration > 0 && size > 0 {
			bitrate = int(size * 8 / 1000 / int64(duration))
		}

		var artistNames []string
		for _, ar := range item.Ar {
			artistNames = append(artistNames, ar.Name)
		}

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
			Link:     fmt.Sprintf("https://music.163.com/#/song?id=%d", item.ID),
			Extra: map[string]string{
				"song_id": strconv.Itoa(item.ID),
			},
		})
	}
	return songs, nil
}

// SearchPlaylist 搜索歌单
func (n *Netease) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	// type=1000 代表歌单
	eparams := map[string]interface{}{
		"method": "POST",
		"url":    "http://music.163.com/api/cloudsearch/pc",
		"params": map[string]interface{}{"s": keyword, "type": 1000, "offset": 0, "limit": 10},
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
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			Playlists []struct {
				ID          int    `json:"id"`
				Name        string `json:"name"`
				CoverImgURL string `json:"coverImgUrl"`
				Creator     struct {
					Nickname string `json:"nickname"`
				} `json:"creator"`
				TrackCount  int    `json:"trackCount"`
				PlayCount   int    `json:"playCount"`
				Description string `json:"description"`
			} `json:"playlists"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease playlist json parse error: %w", err)
	}

	var playlists []model.Playlist
	for _, item := range resp.Result.Playlists {
		playlists = append(playlists, model.Playlist{
			Source:      "netease",
			ID:          strconv.Itoa(item.ID),
			Name:        item.Name,
			Cover:       item.CoverImgURL,
			TrackCount:  item.TrackCount,
			PlayCount:   item.PlayCount,
			Creator:     item.Creator.Nickname,
			Description: item.Description,
		})
	}
	return playlists, nil
}

// GetPlaylistSongs [新增] 获取歌单详情（解析歌曲列表）
func (n *Netease) GetPlaylistSongs(playlistID string) ([]model.Song, error) {
	reqData := map[string]interface{}{
		"id":         playlistID,
		"n":          1000, // 限制获取歌曲数量，最大通常为 1000
		"csrf_token": "",
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

	body, err := utils.Post(PlaylistAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code int `json:"code"`
		Playlist struct {
			Tracks []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
				Ar   []struct { Name string `json:"name"` } `json:"ar"`
				Al   struct { Name string `json:"name"`; PicURL string `json:"picUrl"` } `json:"al"`
				Dt   int `json:"dt"`
			} `json:"tracks"`
		} `json:"playlist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease playlist detail json parse error: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("netease api error code: %d", resp.Code)
	}

	var songs []model.Song
	for _, item := range resp.Playlist.Tracks {
		var artistNames []string
		for _, ar := range item.Ar {
			artistNames = append(artistNames, ar.Name)
		}

		songs = append(songs, model.Song{
			Source:   "netease",
			ID:       strconv.Itoa(item.ID),
			Name:     item.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    item.Al.Name,
			Duration: item.Dt / 1000,
			Cover:    item.Al.PicURL,
			Link:     fmt.Sprintf("https://music.163.com/#/song?id=%d", item.ID),
			Extra: map[string]string{
				"song_id": strconv.Itoa(item.ID),
			},
		})
	}
	return songs, nil
}

// Parse 解析链接并获取完整信息
func (n *Netease) Parse(link string) (*model.Song, error) {
	// 1. 提取 ID
	re := regexp.MustCompile(`id=(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, errors.New("invalid netease link")
	}
	songID := matches[1]

	// 2. 获取 Metadata
	song, err := n.fetchSongDetail(songID)
	if err != nil {
		return nil, err
	}

	// 3. 获取下载链接
	downloadURL, err := n.GetDownloadURL(song)
	if err == nil {
		song.URL = downloadURL
	}

	return song, nil
}

// fetchSongDetail 内部方法：调用 detail 接口获取详情
func (n *Netease) fetchSongDetail(songID string) (*model.Song, error) {
	reqData := map[string]interface{}{
		"c":   fmt.Sprintf(`[{"id":%s}]`, songID),
		"ids": fmt.Sprintf(`[%s]`, songID),
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

	body, err := utils.Post(DetailAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Songs []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Ar   []struct { Name string `json:"name"` } `json:"ar"`
			Al   struct { Name string `json:"name"`; PicURL string `json:"picUrl"` } `json:"al"`
			Dt   int `json:"dt"`
		} `json:"songs"`
		Privileges []struct {
			ID int `json:"id"`
			Fl int `json:"fl"`
			Pl int `json:"pl"`
		} `json:"privileges"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease detail json error: %w", err)
	}

	if len(resp.Songs) == 0 {
		return nil, errors.New("song not found")
	}

	item := resp.Songs[0]
	var artistNames []string
	for _, ar := range item.Ar {
		artistNames = append(artistNames, ar.Name)
	}

	// 简单估算比特率/大小，因为 Detail 接口不像 Search 接口那样直接返回 Size 结构
	// 这里做保守处理
	duration := item.Dt / 1000

	return &model.Song{
		Source:   "netease",
		ID:       strconv.Itoa(item.ID),
		Name:     item.Name,
		Artist:   strings.Join(artistNames, "、"),
		Album:    item.Al.Name,
		Duration: duration,
		Cover:    item.Al.PicURL,
		Link:     fmt.Sprintf("https://music.163.com/#/song?id=%d", item.ID),
		Extra: map[string]string{
			"song_id": strconv.Itoa(item.ID),
		},
	}, nil
}

// GetDownloadURL 获取下载链接
func (n *Netease) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "netease" {
		return "", errors.New("source mismatch")
	}

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
	if err != nil {
		return "", err
	}

	var resp struct {
		Data []struct {
			URL  string `json:"url"`
			Code int    `json:"code"`
			Br   int    `json:"br"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("json parse error: %w", err)
	}
	if len(resp.Data) == 0 || resp.Data[0].URL == "" {
		return "", errors.New("download url not found (might be vip or copyright restricted)")
	}
	return resp.Data[0].URL, nil
}

// GetLyrics 获取歌词
func (n *Netease) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "netease" {
		return "", errors.New("source mismatch")
	}

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
	if err != nil {
		return "", err
	}

	var resp struct {
		Code int `json:"code"`
		Lrc  struct {
			Lyric string `json:"lyric"`
		} `json:"lrc"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("json parse error: %w", err)
	}
	if resp.Code != 200 {
		return "", fmt.Errorf("netease api error code: %d", resp.Code)
	}
	return resp.Lrc.Lyric, nil
}