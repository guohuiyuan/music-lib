package kugou

import (
	"encoding/base64"
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
	MobileUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1"
	MobileReferer   = "http://m.kugou.com"
)

type Kugou struct {
	cookie string
}

func New(cookie string) *Kugou { return &Kugou{cookie: cookie} }

var defaultKugou = New("")

func Search(keyword string) ([]model.Song, error) { return defaultKugou.Search(keyword) }
func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultKugou.SearchPlaylist(keyword)
}
func GetPlaylistSongs(id string) ([]model.Song, error) {
	// 保持原接口兼容性，仅返回 Songs
	_, songs, err := defaultKugou.fetchPlaylistDetail(id)
	return songs, err
}
func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultKugou.ParsePlaylist(link)
}
func GetDownloadURL(s *model.Song) (string, error) { return defaultKugou.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error)      { return defaultKugou.GetLyrics(s) }
func Parse(link string) (*model.Song, error)       { return defaultKugou.Parse(link) }

// GetRecommendedPlaylists 获取推荐歌单
func GetRecommendedPlaylists() ([]model.Playlist, error) { return defaultKugou.GetRecommendedPlaylists() }

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
	if err != nil {
		return nil, err
	}

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
		if item.Privilege == 10 {
			continue
		}
		if item.FileHash == "" && item.SQFileHash == "" && item.HQFileHash == "" {
			continue
		}

		finalHash := item.FileHash
		if isValidHash(item.SQFileHash) {
			finalHash = item.SQFileHash
		} else if isValidHash(item.HQFileHash) {
			finalHash = item.HQFileHash
		}

		var size int64
		switch v := item.FileSize.(type) {
		case float64:
			size = int64(v)
		case int:
			size = int64(v)
		case string:
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				size = i
			}
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
			Link:     fmt.Sprintf("https://www.kugou.com/song/#hash=%s", finalHash),
			Extra: map[string]string{
				"hash": finalHash,
			},
		})
	}
	return songs, nil
}

// SearchPlaylist 搜索歌单
func (k *Kugou) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("platform", "WebFilter")
	params.Set("format", "json")
	params.Set("page", "1")
	params.Set("pagesize", "10")
	params.Set("filter", "0")
	apiURL := "http://mobilecdn.kugou.com/api/v3/search/special?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Cookie", k.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Info []struct {
				SpecialID   int    `json:"specialid"`
				SpecialName string `json:"specialname"`
				Intro       string `json:"intro"`
				ImgURL      string `json:"imgurl"`
				SongCount   int    `json:"songcount"`
				PlayCount   int    `json:"playcount"`
				NickName    string `json:"nickname"`
				PubTime     string `json:"publishtime"`
			} `json:"info"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou playlist search json error: %w", err)
	}

	var playlists []model.Playlist
	for _, item := range resp.Data.Info {
		cover := strings.Replace(item.ImgURL, "{size}", "240", 1)
		playlists = append(playlists, model.Playlist{
			Source:      "kugou",
			ID:          strconv.Itoa(item.SpecialID),
			Name:        item.SpecialName,
			Cover:       cover,
			TrackCount:  item.SongCount,
			PlayCount:   item.PlayCount,
			Creator:     item.NickName,
			Description: item.Intro,
			Link:        fmt.Sprintf("https://www.kugou.com/yy/special/single/%d.html", item.SpecialID),
		})
	}
	return playlists, nil
}

// GetPlaylistSongs 获取歌单详情 (仅返回 Songs, 兼容旧接口)
func (k *Kugou) GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := k.fetchPlaylistDetail(id)
	return songs, err
}

// ParsePlaylist 解析歌单链接
func (k *Kugou) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	// 链接格式: https://www.kugou.com/yy/special/single/546903.html
	re := regexp.MustCompile(`special/single/(\d+)\.html`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, nil, errors.New("invalid kugou playlist link")
	}
	specialID := matches[1]

	return k.fetchPlaylistDetail(specialID)
}

// GetRecommendedPlaylists 获取推荐歌单
func (k *Kugou) GetRecommendedPlaylists() ([]model.Playlist, error) {
	// [修改] 使用 m.kugou.com 的 plist 接口，这个接口对 MobileUserAgent 更友好
	// json=true 返回 JSON 数据
	apiURL := "http://m.kugou.com/plist/index&json=true"

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
	)
	if err != nil {
		return nil, err
	}

	// 检查 Body 是否是 JSON 格式 (简单的开头检查)
	// 如果酷狗返回 HTML 错误页，这里可以拦截到
	if len(body) == 0 || body[0] != '{' {
		return nil, fmt.Errorf("kugou api returned invalid json: %s", string(body))
	}

	var resp struct {
		Plist struct {
			List struct {
				Info []struct {
					SpecialID   int    `json:"specialid"`
					SpecialName string `json:"specialname"`
					ImgURL      string `json:"imgurl"`
					PlayCount   int    `json:"playcount"`
					SongCount   int    `json:"songcount"`
					Username    string `json:"username"`
					Intro       string `json:"intro"`
				} `json:"info"`
			} `json:"list"`
		} `json:"plist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou recommended playlist json parse error: %w", err)
	}

	var playlists []model.Playlist
	for _, item := range resp.Plist.List.Info {
		cover := strings.Replace(item.ImgURL, "{size}", "240", 1)
		
		playlists = append(playlists, model.Playlist{
			Source:      "kugou",
			ID:          strconv.Itoa(item.SpecialID),
			Name:        item.SpecialName,
			Cover:       cover,
			TrackCount:  item.SongCount,
			PlayCount:   item.PlayCount,
			Creator:     item.Username,
			Description: item.Intro,
			Link:        fmt.Sprintf("https://www.kugou.com/yy/special/single/%d.html", item.SpecialID),
		})
	}

	if len(playlists) == 0 {
		return nil, errors.New("no recommended playlists found")
	}

	return playlists, nil
}

// fetchPlaylistDetail [内部复用] 获取歌单详情 (Metadata + Songs)
func (k *Kugou) fetchPlaylistDetail(id string) (*model.Playlist, []model.Song, error) {
	apiURL := fmt.Sprintf("http://mobilecdn.kugou.com/api/v3/special/song?specialid=%s&page=1&pagesize=300&version=9108&area_code=1", id)

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Cookie", k.cookie),
	)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		Data struct {
			Info []struct {
				Hash       string `json:"hash"`
				FileName   string `json:"filename"`
				Duration   int    `json:"duration"`
				FileSize   int64  `json:"filesize"`
				AlbumName  string `json:"album_name"`
				SingerName string `json:"singername"`
				SongName   string `json:"songname"`
			} `json:"info"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("kugou playlist detail json error: %w", err)
	}

	playlist := &model.Playlist{
		Source: "kugou",
		ID:     id,
		Link:   fmt.Sprintf("https://www.kugou.com/yy/special/single/%s.html", id),
	}

	var songs []model.Song
	for _, item := range resp.Data.Info {
		name := item.SongName
		artist := item.SingerName
		if name == "" || artist == "" {
			parts := strings.Split(item.FileName, " - ")
			if len(parts) >= 2 {
				artist = strings.TrimSpace(parts[0])
				name = strings.TrimSpace(strings.Join(parts[1:], " - "))
			} else {
				name = item.FileName
			}
		}

		songs = append(songs, model.Song{
			Source:   "kugou",
			ID:       item.Hash,
			Name:     name,
			Artist:   artist,
			Album:    item.AlbumName,
			Duration: item.Duration,
			Size:     item.FileSize,
			Link:     fmt.Sprintf("https://www.kugou.com/song/#hash=%s", item.Hash),
			Extra: map[string]string{
				"hash": item.Hash,
			},
		})
	}
	
	playlist.TrackCount = len(songs)
	
	return playlist, songs, nil
}

// Parse 解析链接
func (k *Kugou) Parse(link string) (*model.Song, error) {
	re := regexp.MustCompile(`(?i)hash=([a-f0-9]{32})`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, errors.New("invalid kugou link or hash not found")
	}
	hash := matches[1]
	return k.fetchSongInfo(hash)
}

// GetDownloadURL 获取下载链接
func (k *Kugou) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "kugou" {
		return "", errors.New("source mismatch")
	}
	if s.URL != "" {
		return s.URL, nil
	}

	hash := s.ID
	if s.Extra != nil && s.Extra["hash"] != "" {
		hash = s.Extra["hash"]
	}

	info, err := k.fetchSongInfo(hash)
	if err != nil {
		return "", err
	}
	return info.URL, nil
}

// fetchSongInfo 内部核心逻辑：获取详情和 URL
func (k *Kugou) fetchSongInfo(hash string) (*model.Song, error) {
	params := url.Values{}
	params.Set("cmd", "playInfo")
	params.Set("hash", hash)

	apiURL := "http://m.kugou.com/app/i/getSongInfo.php?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		URL        string      `json:"url"`
		BitRate    int         `json:"bitRate"`
		ExtName    string      `json:"extName"`
		AlbumImg   string      `json:"album_img"`
		SongName   string      `json:"songName"`
		AuthorName string      `json:"author_name"`
		TimeLength int         `json:"timeLength"`
		FileSize   int64       `json:"fileSize"`
		Error      interface{} `json:"error"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}

	if resp.URL == "" {
		return nil, errors.New("download url not found (might be paid song)")
	}

	cover := strings.Replace(resp.AlbumImg, "{size}", "240", 1)

	return &model.Song{
		Source:   "kugou",
		ID:       hash,
		Name:     resp.SongName,
		Artist:   resp.AuthorName,
		Duration: resp.TimeLength,
		Size:     resp.FileSize,
		Bitrate:  resp.BitRate / 1000,
		Ext:      resp.ExtName,
		Cover:    cover,
		URL:      resp.URL,
		Link:     fmt.Sprintf("https://www.kugou.com/song/#hash=%s", hash),
		Extra: map[string]string{
			"hash": hash,
		},
	}, nil
}

// GetLyrics 获取歌词
func (k *Kugou) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "kugou" {
		return "", errors.New("source mismatch")
	}

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
	if err != nil {
		return "", err
	}

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

	if len(searchResp.Candidates) == 0 {
		return "", errors.New("lyrics not found")
	}

	candidate := searchResp.Candidates[0]
	downloadURL := fmt.Sprintf("http://lyrics.kugou.com/download?ver=1&client=pc&id=%v&accesskey=%s&fmt=lrc&charset=utf8", candidate.ID, candidate.AccessKey)

	lrcBody, err := utils.Get(downloadURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
	)
	if err != nil {
		return "", err
	}

	var downloadResp struct {
		Status  int    `json:"status"`
		Content string `json:"content"`
		Fmt     string `json:"fmt"`
	}
	if err := json.Unmarshal(lrcBody, &downloadResp); err != nil {
		return "", fmt.Errorf("download lyrics json parse error: %w", err)
	}
	if downloadResp.Content == "" {
		return "", errors.New("lyrics content is empty")
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(downloadResp.Content)
	if err != nil {
		return "", fmt.Errorf("base64 decode error: %w", err)
	}

	return string(decodedBytes), nil
}

func isValidHash(h string) bool {
	return h != "" && h != "00000000000000000000000000000000"
}