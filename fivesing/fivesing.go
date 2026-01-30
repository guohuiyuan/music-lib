package fivesing

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
)

type Fivesing struct {
	cookie string
}

func New(cookie string) *Fivesing {
	return &Fivesing{cookie: cookie}
}

var defaultFivesing = New("")

func Search(keyword string) ([]model.Song, error) { return defaultFivesing.Search(keyword) }
func GetDownloadURL(s *model.Song) (string, error) { return defaultFivesing.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error) { return defaultFivesing.GetLyrics(s) }
func Parse(link string) (*model.Song, error) { return defaultFivesing.Parse(link) }

// Search 搜索歌曲
func (f *Fivesing) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("sort", "1")
	params.Set("page", "1")
	params.Set("filter", "0")
	params.Set("type", "0")

	apiURL := "http://search.5sing.kugou.com/home/json?" + params.Encode()
	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", f.cookie))
	if err != nil {
		return nil, err
	}

	var resp struct {
		List []struct {
			SongID    int64  `json:"songId"`
			SongName  string `json:"songName"`
			Singer    string `json:"singer"`
			SongSize  int64  `json:"songSize"`
			TypeEname string `json:"typeEname"`
		} `json:"list"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("fivesing json parse error: %w", err)
	}

	var songs []model.Song
	for _, item := range resp.List {
		name := removeEmTags(html.UnescapeString(item.SongName))
		artist := removeEmTags(html.UnescapeString(item.Singer))
		
		duration := 0
		if item.SongSize > 0 {
			duration = int((item.SongSize * 8) / 320000)
		}

		songs = append(songs, model.Song{
			Source:   "fivesing",
			ID:       fmt.Sprintf("%d|%s", item.SongID, item.TypeEname),
			Name:     name,
			Artist:   artist,
			Duration: duration,
			Size:     item.SongSize,
			Link:     fmt.Sprintf("http://5sing.kugou.com/%s/%d.html", item.TypeEname, item.SongID),
			Extra: map[string]string{
				"songid":   strconv.FormatInt(item.SongID, 10),
				"songtype": item.TypeEname,
			},
		})
	}
	return songs, nil
}

// Parse 解析链接并获取完整信息
func (f *Fivesing) Parse(link string) (*model.Song, error) {
	// 1. 正则提取 Type 和 ID
	re := regexp.MustCompile(`5sing\.kugou\.com/(\w+)/(\d+)\.html`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 3 {
		return nil, errors.New("invalid 5sing link")
	}
	songType := matches[1]
	songID := matches[2]

	// 2. 调用内部方法获取详情（包含URL和Metadata）
	return f.fetchSongInfo(songID, songType)
}

// GetDownloadURL 获取下载链接
func (f *Fivesing) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "fivesing" {
		return "", errors.New("source mismatch")
	}
	if s.URL != "" {
		return s.URL, nil
	}

	var songID, songType string
	if s.Extra != nil {
		songID = s.Extra["songid"]
		songType = s.Extra["songtype"]
	}

	if songID == "" || songType == "" {
		parts := strings.Split(s.ID, "|")
		if len(parts) == 2 {
			songID = parts[0]
			songType = parts[1]
		} else {
			return "", errors.New("invalid id structure")
		}
	}

	// 复用 fetchSongInfo 获取 URL，或者仅调用 getAudioLink 节省开销
	// 为了高效，这里仅调用 audio 逻辑
	return f.fetchAudioLink(songID, songType)
}

// fetchSongInfo 获取完整的歌曲信息（Metadata + URL）
func (f *Fivesing) fetchSongInfo(songID, songType string) (*model.Song, error) {
	// A. 获取下载链接
	audioURL, err := f.fetchAudioLink(songID, songType)
	if err != nil {
		return nil, err
	}

	// B. 获取元数据 (使用 newget 接口，和 GetLyrics 类似，它包含标题信息)
	params := url.Values{}
	params.Set("songid", songID)
	params.Set("songtype", songType)
	metaURL := "http://mobileapi.5sing.kugou.com/song/newget?" + params.Encode()
	
	metaBody, _ := utils.Get(metaURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", f.cookie))
	
	var name, artist, cover string
	var duration int
	
	// 尝试解析元数据
	if metaBody != nil {
		var metaResp struct {
			Data struct {
				SongName string `json:"songName"`
				Singer   string `json:"singer"`
				Img      string `json:"img"` // 封面
			} `json:"data"`
		}
		if json.Unmarshal(metaBody, &metaResp) == nil {
			name = metaResp.Data.SongName
			artist = metaResp.Data.Singer
			cover = metaResp.Data.Img
		}
	}
	
	// 兜底 Name
	if name == "" {
		name = fmt.Sprintf("5sing_%s_%s", songType, songID)
	}

	return &model.Song{
		Source:   "fivesing",
		ID:       fmt.Sprintf("%s|%s", songID, songType),
		Name:     name,
		Artist:   artist,
		Cover:    cover,
		Duration: duration,
		URL:      audioURL, // 已填充
		Link:     fmt.Sprintf("http://5sing.kugou.com/%s/%s.html", songType, songID),
		Extra: map[string]string{
			"songid":   songID,
			"songtype": songType,
		},
	}, nil
}

// fetchAudioLink 仅获取音频链接
func (f *Fivesing) fetchAudioLink(songID, songType string) (string, error) {
	params := url.Values{}
	params.Set("songid", songID)
	params.Set("songtype", songType)

	apiURL := "http://mobileapi.5sing.kugou.com/song/getSongUrl?" + params.Encode()
	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", f.cookie))
	if err != nil {
		return "", err
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			SQUrl       string `json:"squrl"`
			SQUrlBackup string `json:"squrl_backup"`
			HQUrl       string `json:"hqurl"`
			HQUrlBackup string `json:"hqurl_backup"`
			LQUrl       string `json:"lqurl"`
			LQUrlBackup string `json:"lqurl_backup"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("json parse error: %w", err)
	}

	if resp.Code != 1000 {
		return "", errors.New("api returned error code")
	}

	if url := getFirstValid(resp.Data.SQUrl, resp.Data.SQUrlBackup); url != "" {
		return url, nil
	}
	if url := getFirstValid(resp.Data.HQUrl, resp.Data.HQUrlBackup); url != "" {
		return url, nil
	}
	if url := getFirstValid(resp.Data.LQUrl, resp.Data.LQUrlBackup); url != "" {
		return url, nil
	}

	return "", errors.New("no valid download url found")
}

func (f *Fivesing) GetLyrics(s *model.Song) (string, error) {
	// Added source check to satisfy TestLyricsSourceMismatch
	if s.Source != "fivesing" {
		return "", errors.New("source mismatch")
	}

	var songID, songType string
	if s.Extra != nil {
		songID = s.Extra["songid"]
		songType = s.Extra["songtype"]
	} else {
		parts := strings.Split(s.ID, "|")
		if len(parts) == 2 {
			songID = parts[0]
			songType = parts[1]
		}
	}
	
	if songID == "" {
		return "", errors.New("invalid id")
	}

	params := url.Values{}
	params.Set("songid", songID)
	params.Set("songtype", songType)
	apiURL := "http://mobileapi.5sing.kugou.com/song/newget?" + params.Encode()

	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", f.cookie))
	if err != nil { return "", err }

	var resp struct {
		Data struct { DynamicWords string `json:"dynamicWords"` } `json:"data"`
	}
	json.Unmarshal(body, &resp)
	if resp.Data.DynamicWords == "" { return "", errors.New("lyrics not found") }
	return resp.Data.DynamicWords, nil
}

func getFirstValid(urls ...string) string {
	for _, u := range urls {
		if u != "" { return u }
	}
	return ""
}

func removeEmTags(s string) string {
	s = strings.ReplaceAll(s, "<em class=\"keyword\">", "")
	s = strings.ReplaceAll(s, "</em>", "")
	return strings.TrimSpace(s)
}