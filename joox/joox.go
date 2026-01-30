package joox

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

// ... (常量定义保持不变)
const (
	UserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36"
	Cookie        = "wmid=142420656; user_type=1; country=id; session_key=2a5d97d05dc8fe238150184eaf3519ad;"
	XForwardedFor = "36.73.34.109" 
)

// ... (结构体和 New 方法保持不变)
type Joox struct {
	cookie string
}

func New(cookie string) *Joox {
	if cookie == "" { cookie = Cookie }
	return &Joox{cookie: cookie}
}

var defaultJoox = New(Cookie)

func Search(keyword string) ([]model.Song, error) { return defaultJoox.Search(keyword) }
func GetDownloadURL(s *model.Song) (string, error) { return defaultJoox.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error) { return defaultJoox.GetLyrics(s) }

// Search 搜索歌曲
func (j *Joox) Search(keyword string) ([]model.Song, error) {
	// ... (参数构造保持不变)
	params := url.Values{}
	params.Set("country", "sg")
	params.Set("lang", "zh_cn")
	params.Set("keyword", keyword)
	apiURL := "https://cache.api.joox.com/openjoox/v3/search?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil { return nil, err }

	// ... (JSON 解析保持不变)
	var resp struct {
		SectionList []struct {
			ItemList []struct {
				Song []struct {
					SongInfo struct {
						ID           string `json:"id"`
						Name         string `json:"name"`
						AlbumName    string `json:"album_name"`
						ArtistList   []struct { Name string `json:"name"` } `json:"artist_list"`
						PlayDuration int `json:"play_duration"` 
						Images       []struct { Width int `json:"width"`; URL string `json:"url"` } `json:"images"`
						VipFlag      int `json:"vip_flag"`
					} `json:"song_info"`
				} `json:"song"`
			} `json:"item_list"`
		} `json:"section_list"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("joox search json error: %w", err)
	}

	var songs []model.Song
	for _, section := range resp.SectionList {
		for _, items := range section.ItemList {
			for _, songItem := range items.Song {
				info := songItem.SongInfo
				if info.ID == "" { continue }

				var artistNames []string
				for _, ar := range info.ArtistList { artistNames = append(artistNames, ar.Name) }

				var cover string
				for _, img := range info.Images {
					if img.Width == 300 { cover = img.URL; break }
				}
				if cover == "" && len(info.Images) > 0 { cover = info.Images[0].URL }

				songs = append(songs, model.Song{
					Source:   "joox",
					ID:       info.ID,
					Name:     info.Name,
					Artist:   strings.Join(artistNames, "、"),
					Album:    info.AlbumName,
					Duration: info.PlayDuration,
					Cover:    cover,
					// 核心修改：存入 Extra
					Extra: map[string]string{
						"songid": info.ID,
					},
				})
			}
		}
	}
	return songs, nil
}

// GetDownloadURL 获取下载链接
func (j *Joox) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "joox" { return "", errors.New("source mismatch") }

	// 核心修改：优先从 Extra 获取
	songID := s.ID
	if s.Extra != nil && s.Extra["songid"] != "" {
		songID = s.Extra["songid"]
	}

	params := url.Values{}
	params.Set("songid", songID)
	params.Set("lang", "zh_cn")
	params.Set("country", "sg")

	apiURL := "https://api.joox.com/web-fcgi-bin/web_get_songinfo?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil { return "", err }

	// ... (JSONP 处理及解析保持不变)
	bodyStr := string(body)
	if strings.HasPrefix(bodyStr, "MusicInfoCallback(") {
		bodyStr = strings.TrimPrefix(bodyStr, "MusicInfoCallback(")
		bodyStr = strings.TrimSuffix(bodyStr, ")")
	}

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

	availableQualities := make(map[string]interface{})
	if kbpsMapStr, ok := resp.KbpsMap.(string); ok {
		json.Unmarshal([]byte(kbpsMapStr), &availableQualities)
	} else if kbpsMapObj, ok := resp.KbpsMap.(map[string]interface{}); ok {
		availableQualities = kbpsMapObj
	}

	type Candidate struct { MapKey string; URL string }
	candidates := []Candidate{
		{"320", resp.R320Url}, {"192", resp.R192Url}, {"128", resp.Mp3Url}, {"96", resp.M4aUrl},
	}

	for _, c := range candidates {
		if val, ok := availableQualities[c.MapKey]; ok {
			hasSize := false
			switch v := val.(type) {
			case string: hasSize = v != "0" && v != ""
			case float64: hasSize = v > 0
			case int: hasSize = v > 0
			}
			if hasSize && c.URL != "" { return c.URL, nil }
		}
	}
	return "", errors.New("no valid download url found")
}

// GetLyrics 获取歌词
func (j *Joox) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "joox" { return "", errors.New("source mismatch") }

	// 核心修改：优先从 Extra 获取
	songID := s.ID
	if s.Extra != nil && s.Extra["songid"] != "" {
		songID = s.Extra["songid"]
	}

	params := url.Values{}
	params.Set("musicid", songID)
	params.Set("country", "sg")
	params.Set("lang", "zh_cn")
	apiURL := "https://api.joox.com/web-fcgi-bin/web_lyric?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil { return "", err }

	bodyStr := string(body)
	if idx := strings.Index(bodyStr, "MusicJsonCallback("); idx >= 0 {
		bodyStr = strings.TrimPrefix(bodyStr[idx:], "MusicJsonCallback(")
		bodyStr = strings.TrimSuffix(bodyStr, ")")
	}

	var resp struct { Lyric string `json:"lyric"` }
	if err := json.Unmarshal([]byte(bodyStr), &resp); err != nil {
		return "", fmt.Errorf("joox lyric json parse error: %w", err)
	}
	if resp.Lyric == "" { return "", errors.New("lyric not found or empty") }

	decodedBytes, err := base64.StdEncoding.DecodeString(resp.Lyric)
	if err != nil { return "", fmt.Errorf("base64 decode error: %w", err) }

	return string(decodedBytes), nil
}