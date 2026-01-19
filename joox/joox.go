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
	params := url.Values{}
	params.Set("country", "sg")
	params.Set("lang", "zh_cn")
	params.Set("keyword", keyword)

	apiURL := "https://cache.api.joox.com/openjoox/v3/search?" + params.Encode()

	// 2. 发送请求
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", Cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	// 3. 解析 JSON (更新结构体以支持 Duration 和 Images)
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
						
						// [新增] 补充字段
						PlayDuration int `json:"play_duration"` // 时长 (秒)
						Images       []struct {
							Width int    `json:"width"`
							URL   string `json:"url"`
						} `json:"images"` // 封面列表
						VipFlag int `json:"vip_flag"`
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

				// [新增] 提取封面
				// 优先取 300x300 的图片，如果没有则取第一张
				var cover string
				for _, img := range info.Images {
					if img.Width == 300 {
						cover = img.URL
						break
					}
				}
				if cover == "" && len(info.Images) > 0 {
					cover = info.Images[0].URL
				}

				songs = append(songs, model.Song{
					Source:   "joox",
					ID:       info.ID,
					Name:     info.Name,
					Artist:   strings.Join(artistNames, "、"),
					Album:    info.AlbumName,
					Duration: info.PlayDuration, // [新增] 填充时长
					Cover:    cover,             // [新增] 填充封面
					Size:     0,                 // 搜索结果 JSON 中未提供文件大小
				})
			}
		}
	}

	return songs, nil
}

// GetDownloadURL 获取下载链接
func GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "joox" {
		return "", errors.New("source mismatch")
	}

	// 1. 构造参数
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
		KbpsMap   interface{} `json:"kbps_map"`
	}

	if err := json.Unmarshal([]byte(bodyStr), &resp); err != nil {
		return "", fmt.Errorf("joox detail json error: %w", err)
	}

	// 5. 解析 KbpsMap
	availableQualities := make(map[string]interface{})
	if kbpsMapStr, ok := resp.KbpsMap.(string); ok {
		json.Unmarshal([]byte(kbpsMapStr), &availableQualities)
	} else if kbpsMapObj, ok := resp.KbpsMap.(map[string]interface{}); ok {
		availableQualities = kbpsMapObj
	}

	// 6. 音质选择策略
	type Candidate struct {
		MapKey string
		URL    string
	}

	candidates := []Candidate{
		{"320", resp.R320Url},
		{"192", resp.R192Url},
		{"128", resp.Mp3Url},
		{"96", resp.M4aUrl},
	}

	for _, c := range candidates {
		if val, ok := availableQualities[c.MapKey]; ok {
			// 检查是否有大小信息
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
				return c.URL, nil
			}
		}
	}

	return "", errors.New("no valid download url found")
}