package joox

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	// 关键 Headers，必须带上
	UserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36"
	Cookie        = "wmid=142420656; user_type=1; country=id; session_key=2a5d97d05dc8fe238150184eaf3519ad;"
	XForwardedFor = "36.73.34.109" // 模拟印尼 IP
)

// Search 搜索歌曲
// 对应 Python: _search 方法前半部分
func Search(keyword string) ([]model.Song, error) {
	// 1. 构造参数
	// Python: default_rule = {'country': 'sg', 'lang': 'zh_cn', 'keyword': keyword}
	params := url.Values{}
	params.Set("country", "sg")
	params.Set("lang", "zh_cn")
	params.Set("keyword", keyword)

	apiURL := "https://cache.api.joox.com/openjoox/v3/search?" + params.Encode()

	// 2. 发送请求
	// Joox 对 IP 和 Cookie 校验较严
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", Cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	// 3. 解析 JSON
	// 结构: section_list[] -> item_list[] -> song[] -> song_info{}
	var resp struct {
		SectionList []struct {
			ItemList []struct {
				Song []struct {
					SongInfo struct {
						ID         string `json:"id"`
						Name       string `json:"name"`
						AlbumName  string `json:"album_name"`
						ArtistList []struct {
							Name string `json:"name"`
						} `json:"artist_list"`
					} `json:"song_info"`
				} `json:"song"`
			} `json:"item_list"`
		} `json:"section_list"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("joox search json error: %w", err)
	}

	// 4. 转换模型
	var songs []model.Song
	// 遍历多层嵌套结构
	for _, section := range resp.SectionList {
		for _, items := range section.ItemList {
			for _, songItem := range items.Song {
				info := songItem.SongInfo
				if info.ID == "" {
					continue
				}

				// 拼接歌手
				var artistNames []string
				for _, ar := range info.ArtistList {
					artistNames = append(artistNames, ar.Name)
				}

				songs = append(songs, model.Song{
					Source:   "joox",
					ID:       info.ID,
					Name:     info.Name,
					Artist:   strings.Join(artistNames, "、"),
					Album:    info.AlbumName,
					Duration: 0, // 搜索列表未返回时长
				})
			}
		}
	}

	return songs, nil
}

// GetDownloadURL 获取下载链接
// 对应 Python: _search 方法内的 web_get_songinfo 调用逻辑
func GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "joox" {
		return "", errors.New("source mismatch")
	}

	// 1. 构造参数
	// Python: params = {'songid': ..., 'lang': lang, 'country': country}
	params := url.Values{}
	params.Set("songid", s.ID)
	params.Set("lang", "zh_cn")
	params.Set("country", "sg")

	apiURL := "https://api.joox.com/web-fcgi-bin/web_get_songinfo?" + params.Encode()

	// 2. 发送请求
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", Cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return "", err
	}

	// 3. 处理 JSONP
	// 响应格式: MusicInfoCallback({...})
	bodyStr := string(body)
	if strings.HasPrefix(bodyStr, "MusicInfoCallback(") {
		bodyStr = strings.TrimPrefix(bodyStr, "MusicInfoCallback(")
		bodyStr = strings.TrimSuffix(bodyStr, ")")
	}

	// 4. 解析 JSON
	var resp struct {
		R320Url   string      `json:"r320Url"`
		R192Url   string      `json:"r192Url"`
		Mp3Url    string      `json:"mp3Url"`
		M4aUrl    string      `json:"m4aUrl"`
		MInterval int         `json:"minterval"`
		KbpsMap   interface{} `json:"kbps_map"` // 注意：这通常是一个 JSON 字符串，需要二次解析
	}

	// Joox 的 JSON 有时不太标准，如果 json.Unmarshal 失败可能需要更宽松的解析器
	// 这里假设标准库能处理清洗后的 JSONP
	if err := json.Unmarshal([]byte(bodyStr), &resp); err != nil {
		return "", fmt.Errorf("joox detail json error: %w", err)
	}

	// 5. 解析 KbpsMap (用于判断音质是否存在)
	// Python: kbps_map = json_repair.loads(download_result['kbps_map'])
	availableQualities := make(map[string]interface{})
	if kbpsMapStr, ok := resp.KbpsMap.(string); ok {
		// 如果是字符串，需要二次反序列化
		json.Unmarshal([]byte(kbpsMapStr), &availableQualities)
	} else if kbpsMapObj, ok := resp.KbpsMap.(map[string]interface{}); ok {
		// 如果已经是对象
		availableQualities = kbpsMapObj
	}

	// 6. 音质选择策略
	// Python: [('r320Url', '320'), ('r192Url', '192'), ('mp3Url', '128'), ('m4aUrl', '96')]
	// 逻辑: 检查 kbps_map 是否有对应 key，且 url 是否非空

	type Candidate struct {
		URLField string
		MapKey   string
		URL      string
	}

	candidates := []Candidate{
		{"r320Url", "320", resp.R320Url},
		{"r192Url", "192", resp.R192Url},
		{"mp3Url", "128", resp.Mp3Url},
		{"m4aUrl", "96", resp.M4aUrl},
	}

	for _, c := range candidates {
		// 检查 KbpsMap 中是否有该码率 (值通常是文件大小，非0即存在)
		if val, ok := availableQualities[c.MapKey]; ok {
			// 简单的非空/非零检查
			hasSize := false
			switch v := val.(type) {
			case string:
				hasSize = v != "0" && v != ""
			case float64:
				hasSize = v > 0
			case int:
				hasSize = v > 0
			}

			if hasSize && c.URL != "" {
				// 可以在这里反向填充时长
				if s.Duration == 0 {
					s.Duration = resp.MInterval
				}
				return c.URL, nil
			}
		}
	}

	return "", errors.New("no valid download url found")
}
