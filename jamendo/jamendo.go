package jamendo

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"
	Referer      = "https://www.jamendo.com/search?q=musicdl"
	XJamVersion  = "4gvfvv"
	SearchAPI    = "https://www.jamendo.com/api/search"
	SearchApiPath = "/api/search" // 用于签名计算
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// Search 搜索歌曲
// 对应 Python: _search 方法
func Search(keyword string) ([]model.Song, error) {
	// 1. 构造搜索参数
	// Python: {'query': keyword, 'type': 'track', 'limit': ..., 'identities': 'www'}
	params := url.Values{}
	params.Set("query", keyword)
	params.Set("type", "track")
	params.Set("limit", "20")
	params.Set("identities", "www")

	apiURL := SearchAPI + "?" + params.Encode()

	// 2. 生成动态签名 Header
	xJamCall := makeXJamCall(SearchApiPath)

	// 3. 发送请求
	// Python: headers['x-jam-call'] = self._makexjamcall()
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
	// Jamendo 返回的是一个对象列表
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
		// 下载/流地址可能在 download 或 stream 字段中
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

		// 音质选择策略: flac > ogg > mp3
		// Python: for quality in ['flac', 'ogg', 'mp3']: ...
		var downloadURL string
		if url := streams["flac"]; url != "" {
			downloadURL = url
		} else if url := streams["ogg"]; url != "" {
			downloadURL = url
		} else if url := streams["mp3"]; url != "" {
			downloadURL = url
		} else {
			continue // 没有有效链接
		}

		// 简单处理 ID 转 string
		idStr := fmt.Sprintf("%d", item.ID)

		// 填充 Song 模型
		// 注意：Jamendo 直接给出了 URL，我们将其存入 model.Song 的 URL 字段中
		// 如果 model.Song 没有 URL 字段，可以考虑存入 ID 字段做 Hack (如 "ID|URL")
		// 这里假设 model.Song 结构体有 URL 字段 (符合 music-lib 通用设计)
		songs = append(songs, model.Song{
			Source:   "jamendo",
			ID:       idStr,
			Name:     item.Name,
			Artist:   item.Artist.Name,
			Album:    item.Album.Name,
			Duration: item.Duration,
			URL:      downloadURL, // 直接存储 URL
			// Size 未知
		})
	}

	return songs, nil
}

// GetDownloadURL 获取下载链接
// 由于 Jamendo 搜索结果直接包含链接，这里直接从 Song 对象提取
func GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "jamendo" {
		return "", errors.New("source mismatch")
	}

	if s.URL != "" {
		return s.URL, nil
	}

	// 如果 URL 为空 (例如是通过纯 ID 构造的 Song 对象)，
	// 按照 Python 代码逻辑，它没有提供通过 ID 获取 Track 的独立 API 方法。
	// 为了健壮性，这里返回错误提示。在完整实现中，可以通过调用 Search 接口查询 ID 来获取。
	return "", errors.New("url not present in song object, please search first")
}

// makeXJamCall 生成动态签名
// 对应 Python: _makexjamcall
func makeXJamCall(path string) string {
	// 1. 生成随机数字符串 (模拟 Python random.random())
	// Python 的 random.random() 返回 [0.0, 1.0) 的浮点数
	r := rand.Float64()
	randStr := fmt.Sprintf("%v", r)

	// 2. 计算 SHA1 (path + rand)
	data := path + randStr
	hash := sha1.Sum([]byte(data))
	digest := hex.EncodeToString(hash[:])

	// 3. 拼接结果: ${digest}*{rand}~
	return fmt.Sprintf("$%s*%s~", digest, randStr)
}
