package jamendo

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"
	Referer       = "https://www.jamendo.com/search?q=musicdl"
	XJamVersion   = "4gvfvv"
	SearchAPI     = "https://www.jamendo.com/api/search"
	SearchApiPath = "/api/search" // 用于签名计算
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Search 搜索歌曲
// 对应 Python: _search 方法
func Search(keyword string) ([]model.Song, error) {
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

		// 音质选择策略: flac > mp3 > ogg
		// 这里的顺序决定了下载的优先格式
		var downloadURL string
		var ext string

		if url := streams["flac"]; url != "" {
			downloadURL = url
			ext = "flac"
		} else if url := streams["mp3"]; url != "" {
			downloadURL = url
			ext = "mp3"
		} else if url := streams["ogg"]; url != "" {
			downloadURL = url
			ext = "ogg"
		} else {
			continue // 没有有效链接
		}

		// [核心修改] 将下载链接拼接到 ID 中，确保 GetDownloadURL 能获取到
		// 格式: ID|DownloadURL
		compoundID := fmt.Sprintf("%d|%s", item.ID, downloadURL)

		songs = append(songs, model.Song{
			Source:   "jamendo",
			ID:       compoundID,             // 存储复合ID
			Name:     item.Name,
			Artist:   item.Artist.Name,
			Album:    item.Album.Name,
			Duration: item.Duration,
			Ext:      ext,                    // 明确指定后缀
			Cover:    item.Cover.Big.Size300, // 填充封面
			// Size:  0,                      // Jamendo 搜索结果不包含大小
		})
	}

	return songs, nil
}

// GetDownloadURL 获取下载链接
func GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "jamendo" {
		return "", errors.New("source mismatch")
	}

	// [核心修改] 从复合 ID 中提取下载链接
	parts := strings.SplitN(s.ID, "|", 2)
	if len(parts) == 2 {
		return parts[1], nil
	}

	// 兼容旧逻辑：如果 ID 中没有 URL，尝试读取 s.URL (如果 struct 支持)
	if s.URL != "" {
		return s.URL, nil
	}

	return "", errors.New("download url missing in song ID")
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