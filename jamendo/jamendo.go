package jamendo

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"
	Referer       = "https://www.jamendo.com/search?q=musicdl"
	XJamVersion   = "4gvfvv"
	SearchAPI     = "https://www.jamendo.com/api/search"
	SearchApiPath = "/api/search" // 用于签名计算
	TrackAPI      = "https://www.jamendo.com/api/tracks" // 新增：单曲查询接口
	TrackApiPath  = "/api/tracks" // 新增：单曲接口路径用于签名
)

// Jamendo 结构体
type Jamendo struct {
	cookie string
}

// New 初始化函数
func New(cookie string) *Jamendo {
	return &Jamendo{
		cookie: cookie,
	}
}

// 全局默认实例（向后兼容）
var defaultJamendo = New("")

// Search 搜索歌曲（向后兼容）
func Search(keyword string) ([]model.Song, error) {
	return defaultJamendo.Search(keyword)
}

// GetDownloadURL 获取下载链接（向后兼容）
func GetDownloadURL(s *model.Song) (string, error) {
	return defaultJamendo.GetDownloadURL(s)
}

// GetLyrics 获取歌词（向后兼容）
func GetLyrics(s *model.Song) (string, error) {
	return defaultJamendo.GetLyrics(s)
}

// Search 搜索歌曲
// 对应 Python: _search 方法
func (j *Jamendo) Search(keyword string) ([]model.Song, error) {
	// 1. 构造搜索参数
	params := url.Values{}
	params.Set("query", keyword)
	params.Set("type", "track")
	params.Set("limit", "20")
	params.Set("identities", "www")

	apiURL := SearchAPI + "?" + params.Encode()

	// 2. 生成动态签名 Header
	xJamCall := makeXJamCall(SearchApiPath)

	// 3. 发送请求
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

	// 4. 解析 JSON
	var results []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Duration int    `json:"duration"`
		Artist   struct {
			Name string `json:"name"`
		} `json:"artist"`
		Album struct {
			Name string `json:"name"`
		} `json:"album"`
		// 封面结构
		Cover struct {
			Big struct {
				Size300 string `json:"size300"`
			} `json:"big"`
		} `json:"cover"`
		// 下载/流地址
		Download map[string]string `json:"download"`
		Stream   map[string]string `json:"stream"`
	}

	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("jamendo json parse error: %w", err)
	}

	// 5. 转换模型
	var songs []model.Song
	for _, item := range results {
		// 获取音频流字典 (优先 download，其次 stream)
		streams := item.Download
		if len(streams) == 0 {
			streams = item.Stream
		}
		if len(streams) == 0 {
			continue
		}

		// 挑选链接
		downloadURL, ext := pickBestQuality(streams)
		if downloadURL == "" {
			continue
		}

		songs = append(songs, model.Song{
			Source:   "jamendo",
			ID:       fmt.Sprintf("%d", item.ID),
			Name:     item.Name,
			Artist:   item.Artist.Name,
			Album:    item.Album.Name,
			Duration: item.Duration,
			Ext:      ext,
			Cover:    item.Cover.Big.Size300,
			URL:      downloadURL,
			Size:     0,
			Bitrate:  0,
		})
	}

	return songs, nil
}

// GetDownloadURL 获取下载链接
// 修改逻辑：如果 s.URL 为空，则主动调用 API 通过 ID 获取
func (j *Jamendo) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "jamendo" {
		return "", errors.New("source mismatch")
	}

	// 1. 如果 URL 已经存在，直接返回 (优化)
	if s.URL != "" {
		return s.URL, nil
	}

	// 2. URL 为空，需要通过 ID 重新获取
	if s.ID == "" {
		return "", errors.New("id missing")
	}

	// 构造针对 /api/tracks 的请求
	params := url.Values{}
	params.Set("id", s.ID) // 使用 ID 查询

	apiURL := TrackAPI + "?" + params.Encode()
	xJamCall := makeXJamCall(TrackApiPath) // 这里的 path 必须对应

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("x-jam-call", xJamCall),
		utils.WithHeader("x-jam-version", XJamVersion),
		utils.WithHeader("x-requested-with", "XMLHttpRequest"),
		utils.WithHeader("Cookie", j.cookie),
	)
	if err != nil {
		return "", err
	}

	// 解析结构 (与 Search 类似)
	var results []struct {
		Download map[string]string `json:"download"`
		Stream   map[string]string `json:"stream"`
	}

	if err := json.Unmarshal(body, &results); err != nil {
		return "", fmt.Errorf("jamendo track json error: %w", err)
	}

	if len(results) == 0 {
		return "", errors.New("track not found")
	}

	// 3. 提取链接
	item := results[0]
	streams := item.Download
	if len(streams) == 0 {
		streams = item.Stream
	}

	downloadURL, _ := pickBestQuality(streams)
	if downloadURL == "" {
		return "", errors.New("no valid stream found")
	}

	return downloadURL, nil
}

// 辅助函数：挑选最佳音质
func pickBestQuality(streams map[string]string) (string, string) {
	if url := streams["flac"]; url != "" {
		return url, "flac"
	}
	if url := streams["mp3"]; url != "" {
		return url, "mp3"
	}
	if url := streams["ogg"]; url != "" {
		return url, "ogg"
	}
	return "", ""
}

// makeXJamCall 生成动态签名
func makeXJamCall(path string) string {
	r := rand.Float64()
	randStr := fmt.Sprintf("%v", r)

	data := path + randStr
	hash := sha1.Sum([]byte(data))
	digest := hex.EncodeToString(hash[:])

	return fmt.Sprintf("$%s*%s~", digest, randStr)
}

// GetLyrics 获取歌词 (Jamendo暂不支持歌词接口)
func (j *Jamendo) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "jamendo" {
		return "", errors.New("source mismatch")
	}
	return "", nil
}