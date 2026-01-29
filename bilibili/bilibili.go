package bilibili

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
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 Edg/121.0.0.0"
	Referer   = "https://www.bilibili.com/"
)

// Bilibili 结构体
type Bilibili struct {
	cookie string
}

// New 初始化函数
func New(cookie string) *Bilibili {
	return &Bilibili{
		cookie: cookie,
	}
}

// 全局默认实例（向后兼容）
var defaultBilibili = New("buvid3=2E109C72-251F-3827-FA8E-921FA0D7EC5291319infoc; SESSDATA=your_sessdata;")

// Search 搜索歌曲（向后兼容）
func Search(keyword string) ([]model.Song, error) {
	return defaultBilibili.Search(keyword)
}

// GetDownloadURL 获取下载链接（向后兼容）
func GetDownloadURL(s *model.Song) (string, error) {
	return defaultBilibili.GetDownloadURL(s)
}

// GetLyrics 获取歌词（向后兼容）
func GetLyrics(s *model.Song) (string, error) {
	return defaultBilibili.GetLyrics(s)
}

// Search 搜索歌曲
func (b *Bilibili) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("search_type", "video")
	params.Set("keyword", keyword)
	params.Set("page", "1")
	params.Set("page_size", "10")

	searchURL := "https://api.bilibili.com/x/web-interface/search/type?" + params.Encode()
	body, err := utils.Get(searchURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
	if err != nil {
		return nil, err
	}

	var searchResp struct {
		Data struct {
			Result []struct {
				BVID   string `json:"bvid"`
				Title  string `json:"title"`
				Author string `json:"author"`
				Pic    string `json:"pic"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("bilibili search json error: %w", err)
	}

	var songs []model.Song
	for _, item := range searchResp.Data.Result {
		rootTitle := strings.ReplaceAll(strings.ReplaceAll(item.Title, "<em class=\"keyword\">", ""), "</em>", "")

		// 必须保留 view 接口调用，因为我们需要 CID 来生成唯一的 Song ID
		viewURL := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?bvid=%s", item.BVID)
		viewBody, err := utils.Get(viewURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", b.cookie))
		if err != nil {
			continue
		}

		var viewResp struct {
			Data struct {
				Pages []struct {
					CID      int64  `json:"cid"`
					Part     string `json:"part"`
					Duration int    `json:"duration"`
				} `json:"pages"`
			} `json:"data"`
		}
		if err := json.Unmarshal(viewBody, &viewResp); err != nil {
			continue
		}

		cover := item.Pic
		if strings.HasPrefix(cover, "//") { cover = "https:" + cover }

		for _, page := range viewResp.Data.Pages {
			// 已移除：在此处循环调用 playurl 获取大小的逻辑
			
			displayTitle := page.Part
			if len(viewResp.Data.Pages) == 1 && displayTitle == "" {
				displayTitle = rootTitle
			} else if displayTitle != rootTitle {
				displayTitle = fmt.Sprintf("%s - %s", rootTitle, displayTitle)
			}
			songs = append(songs, model.Song{
				Source:   "bilibili",
				ID:       fmt.Sprintf("%s|%d", item.BVID, page.CID),
				Name:     displayTitle,
				Artist:   item.Author,
				Album:    item.BVID,
				Duration: page.Duration, // 使用 view 接口提供的基础时长
				Size:     0,             // Search 不再请求 size，由 Inspect 或 Download 处理
				Bitrate:  0,
				Cover:    cover,
			})
		}
	}
	return songs, nil
}

// GetDownloadURL 获取下载链接
func (b *Bilibili) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "bilibili" {
		return "", errors.New("source mismatch")
	}
	
	parts := strings.Split(s.ID, "|")
	if len(parts) != 2 { return "", errors.New("invalid id") }
	apiURL := fmt.Sprintf("https://api.bilibili.com/x/player/playurl?fnval=16&bvid=%s&cid=%s", parts[0], parts[1])

	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
	if err != nil { return "", err }

	var resp struct {
		Data struct {
			Durl []struct { URL string `json:"url"` } `json:"durl"`
			Dash struct {
				Audio []DashStream `json:"audio"`
				Flac  struct { Audio []DashStream `json:"audio"` } `json:"flac"`
			} `json:"dash"`
		} `json:"data"`
	}
	json.Unmarshal(body, &resp)

	// 优先 DASH
	if len(resp.Data.Dash.Flac.Audio) > 0 { return resp.Data.Dash.Flac.Audio[0].BaseURL, nil }
	if len(resp.Data.Dash.Audio) > 0 { return resp.Data.Dash.Audio[0].BaseURL, nil }
	// 兜底 Durl
	if len(resp.Data.Durl) > 0 { return resp.Data.Durl[0].URL, nil }

	return "", errors.New("no audio found")
}

type DashStream struct {
	BaseURL   string `json:"baseUrl"`
	Bandwidth int    `json:"bandwidth"`
}

// GetLyrics 获取歌词 (B站暂不支持歌词接口)
func (b *Bilibili) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "bilibili" {
		return "", errors.New("source mismatch")
	}
	// B站歌词接口暂未实现
	return "", nil
}