package jamendo

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"
	Referer       = "https://www.jamendo.com/search?q=musicdl"
	XJamVersion   = "4gvfvv"
	SearchAPI     = "https://www.jamendo.com/api/search"
	SearchApiPath = "/api/search" 
	TrackAPI      = "https://www.jamendo.com/api/tracks" 
	TrackApiPath  = "/api/tracks" 
)

type Jamendo struct {
	cookie string
}

func New(cookie string) *Jamendo {
	return &Jamendo{cookie: cookie}
}

var defaultJamendo = New("")

func Search(keyword string) ([]model.Song, error) { return defaultJamendo.Search(keyword) }
func GetDownloadURL(s *model.Song) (string, error) { return defaultJamendo.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error) { return defaultJamendo.GetLyrics(s) }
func Parse(link string) (*model.Song, error) { return defaultJamendo.Parse(link) }

// Search 搜索歌曲
func (j *Jamendo) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("query", keyword)
	params.Set("type", "track")
	params.Set("limit", "20")
	params.Set("identities", "www")
	apiURL := SearchAPI + "?" + params.Encode()
	xJamCall := makeXJamCall(SearchApiPath)

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("x-jam-call", xJamCall),
		utils.WithHeader("x-jam-version", XJamVersion),
		utils.WithHeader("x-requested-with", "XMLHttpRequest"),
		utils.WithHeader("Cookie", j.cookie),
	)
	if err != nil {
		return nil, err
	}

	var results []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Duration int    `json:"duration"`
		Artist   struct { Name string `json:"name"` } `json:"artist"`
		Album    struct { Name string `json:"name"` } `json:"album"`
		Cover    struct { Big struct { Size300 string `json:"size300"` } `json:"big"` } `json:"cover"`
		Download map[string]string `json:"download"`
		Stream   map[string]string `json:"stream"`
	}
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("jamendo json parse error: %w", err)
	}

	var songs []model.Song
	for _, item := range results {
		streams := item.Download
		if len(streams) == 0 { streams = item.Stream }
		if len(streams) == 0 { continue }

		downloadURL, ext := pickBestQuality(streams)
		if downloadURL == "" { continue }

		songs = append(songs, model.Song{
			Source:   "jamendo",
			ID:       fmt.Sprintf("%d", item.ID),
			Name:     item.Name,
			Artist:   item.Artist.Name,
			Album:    item.Album.Name,
			Duration: item.Duration,
			Ext:      ext,
			Cover:    item.Cover.Big.Size300,
			URL:      downloadURL, // Search 结果也可以直接带上 URL
			Link:     fmt.Sprintf("https://www.jamendo.com/track/%d", item.ID),
			Extra: map[string]string{
				"track_id": strconv.Itoa(item.ID),
			},
		})
	}
	return songs, nil
}

// Parse 解析链接并获取完整 Song 详情
func (j *Jamendo) Parse(link string) (*model.Song, error) {
	re := regexp.MustCompile(`jamendo\.com/track/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, errors.New("invalid jamendo link")
	}
	trackID := matches[1]

	// 直接调用底层获取详情
	return j.getTrackByID(trackID)
}

// GetDownloadURL 获取下载链接
func (j *Jamendo) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "jamendo" {
		return "", errors.New("source mismatch")
	}
	if s.URL != "" {
		return s.URL, nil
	}

	trackID := s.ID
	if s.Extra != nil && s.Extra["track_id"] != "" {
		trackID = s.Extra["track_id"]
	}
	if trackID == "" {
		return "", errors.New("id missing")
	}

	// 复用底层逻辑
	info, err := j.getTrackByID(trackID)
	if err != nil {
		return "", err
	}
	return info.URL, nil
}

// getTrackByID 底层核心逻辑：通过 ID 获取完整 Song 对象
func (j *Jamendo) getTrackByID(id string) (*model.Song, error) {
	params := url.Values{}
	params.Set("id", id)

	apiURL := TrackAPI + "?" + params.Encode()
	xJamCall := makeXJamCall(TrackApiPath)

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("x-jam-call", xJamCall),
		utils.WithHeader("x-jam-version", XJamVersion),
		utils.WithHeader("x-requested-with", "XMLHttpRequest"),
		utils.WithHeader("Cookie", j.cookie),
	)
	if err != nil {
		return nil, err
	}

	var results []struct {
		ID       int               `json:"id"`
		Name     string            `json:"name"`
		Duration int               `json:"duration"`
		Artist   struct{ Name string `json:"name"` } `json:"artist"`
		Album    struct{ Name string `json:"name"` } `json:"album"`
		Cover    struct{ Big struct{ Size300 string `json:"size300"` } `json:"big"` } `json:"cover"`
		Download map[string]string `json:"download"`
		Stream   map[string]string `json:"stream"`
	}
	
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("jamendo track json error: %w", err)
	}
	if len(results) == 0 {
		return nil, errors.New("track not found")
	}

	item := results[0]
	streams := item.Download
	if len(streams) == 0 {
		streams = item.Stream
	}

	downloadURL, ext := pickBestQuality(streams)
	if downloadURL == "" {
		return nil, errors.New("no valid stream found")
	}

	return &model.Song{
		Source:   "jamendo",
		ID:       strconv.Itoa(item.ID),
		Name:     item.Name,
		Artist:   item.Artist.Name,
		Album:    item.Album.Name,
		Duration: item.Duration,
		Ext:      ext,
		Cover:    item.Cover.Big.Size300,
		URL:      downloadURL, // 已填充
		Link:     fmt.Sprintf("https://www.jamendo.com/track/%d", item.ID),
		Extra: map[string]string{
			"track_id": strconv.Itoa(item.ID),
		},
	}, nil
}

func pickBestQuality(streams map[string]string) (string, string) {
	if url := streams["flac"]; url != "" { return url, "flac" }
	if url := streams["mp3"]; url != "" { return url, "mp3" }
	if url := streams["ogg"]; url != "" { return url, "ogg" }
	return "", ""
}

func makeXJamCall(path string) string {
	r := rand.Float64()
	randStr := fmt.Sprintf("%v", r)
	data := path + randStr
	hash := sha1.Sum([]byte(data))
	digest := hex.EncodeToString(hash[:])
	return fmt.Sprintf("$%s*%s~", digest, randStr)
}

func (j *Jamendo) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "jamendo" {
		return "", errors.New("source mismatch")
	}
	return "", nil
}