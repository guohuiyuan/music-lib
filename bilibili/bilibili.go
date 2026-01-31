package bilibili

import (
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

var defaultBilibili = New("buvid3=2E109C72-251F-3827-FA8E-921FA0D7EC5291319infoc; SESSDATA=your_sessdata;")

func Search(keyword string) ([]model.Song, error) { return defaultBilibili.Search(keyword) }
func GetDownloadURL(s *model.Song) (string, error) { return defaultBilibili.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error) { return defaultBilibili.GetLyrics(s) }
func Parse(link string) (*model.Song, error) { return defaultBilibili.Parse(link) }

// Search 搜索歌曲
func (b *Bilibili) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("search_type", "video")
	params.Set("keyword", keyword)
	params.Set("page", "1")
	params.Set("page_size", "50")

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

		// 获取视频详情以得到 CID
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
		if strings.HasPrefix(cover, "//") {
			cover = "https:" + cover
		}

		for i, page := range viewResp.Data.Pages {
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
				Duration: page.Duration,
				Cover:    cover,
				Link:     fmt.Sprintf("https://www.bilibili.com/video/%s?p=%d", item.BVID, i+1),
				Extra: map[string]string{
					"bvid": item.BVID,
					"cid":  strconv.FormatInt(page.CID, 10),
				},
			})
		}
	}
	return songs, nil
}

// Parse 解析链接并获取完整信息（包括下载链接）
func (b *Bilibili) Parse(link string) (*model.Song, error) {
	// 1. 提取 BVID
	bvidRe := regexp.MustCompile(`(BV\w+)`)
	bvidMatches := bvidRe.FindStringSubmatch(link)
	if len(bvidMatches) < 2 {
		return nil, errors.New("invalid bilibili link: bvid not found")
	}
	bvid := bvidMatches[1]

	// 2. 提取 Page (p=X), 默认为 1
	page := 1
	pageRe := regexp.MustCompile(`[?&]p=(\d+)`)
	pageMatches := pageRe.FindStringSubmatch(link)
	if len(pageMatches) >= 2 {
		if p, err := strconv.Atoi(pageMatches[1]); err == nil && p > 0 {
			page = p
		}
	}

	// 3. 调用 View 接口获取元数据
	viewURL := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?bvid=%s", bvid)
	viewBody, err := utils.Get(viewURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", b.cookie))
	if err != nil {
		return nil, err
	}

	var viewResp struct {
		Data struct {
			BVID   string `json:"bvid"`
			Title  string `json:"title"`
			Pic    string `json:"pic"`
			Owner  struct {
				Name string `json:"name"`
			} `json:"owner"`
			Pages []struct {
				CID      int64  `json:"cid"`
				Part     string `json:"part"`
				Duration int    `json:"duration"`
			} `json:"pages"`
		} `json:"data"`
	}

	if err := json.Unmarshal(viewBody, &viewResp); err != nil {
		return nil, fmt.Errorf("bilibili view json error: %w", err)
	}

	if len(viewResp.Data.Pages) == 0 {
		return nil, errors.New("no video pages found")
	}

	if page > len(viewResp.Data.Pages) {
		page = 1
	}
	targetPage := viewResp.Data.Pages[page-1]

	displayTitle := targetPage.Part
	if len(viewResp.Data.Pages) == 1 && displayTitle == "" {
		displayTitle = viewResp.Data.Title
	} else if displayTitle != viewResp.Data.Title {
		displayTitle = fmt.Sprintf("%s - %s", viewResp.Data.Title, displayTitle)
	}

	cover := viewResp.Data.Pic
	if strings.HasPrefix(cover, "//") {
		cover = "https:" + cover
	}

	cidStr := strconv.FormatInt(targetPage.CID, 10)

	// 4. 立即获取下载链接
	audioURL, _ := b.fetchAudioURL(bvid, cidStr) // 忽略错误，尽可能返回元数据

	return &model.Song{
		Source:   "bilibili",
		ID:       fmt.Sprintf("%s|%d", viewResp.Data.BVID, targetPage.CID),
		Name:     displayTitle,
		Artist:   viewResp.Data.Owner.Name,
		Album:    viewResp.Data.BVID,
		Duration: targetPage.Duration,
		Cover:    cover,
		URL:      audioURL, // 已填充
		Link:     fmt.Sprintf("https://www.bilibili.com/video/%s?p=%d", viewResp.Data.BVID, page),
		Extra: map[string]string{
			"bvid": viewResp.Data.BVID,
			"cid":  cidStr,
		},
	}, nil
}

// GetDownloadURL 获取下载链接
func (b *Bilibili) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "bilibili" {
		return "", errors.New("source mismatch")
	}

	if s.URL != "" {
		return s.URL, nil
	}

	var bvid, cid string
	if s.Extra != nil {
		bvid = s.Extra["bvid"]
		cid = s.Extra["cid"]
	}

	if bvid == "" || cid == "" {
		parts := strings.Split(s.ID, "|")
		if len(parts) == 2 {
			bvid = parts[0]
			cid = parts[1]
		} else {
			return "", errors.New("invalid id structure")
		}
	}

	return b.fetchAudioURL(bvid, cid)
}

// fetchAudioURL 内部逻辑提取
func (b *Bilibili) fetchAudioURL(bvid, cid string) (string, error) {
	apiURL := fmt.Sprintf("https://api.bilibili.com/x/player/playurl?fnval=80&qn=127&bvid=%s&cid=%s", bvid, cid)
	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
	if err != nil {
		return "", err
	}

	var resp struct {
		Data struct {
			Durl []struct {
				URL string `json:"url"`
			} `json:"durl"`
			Dash struct {
				Audio []struct {
					BaseURL string `json:"baseUrl"`
				} `json:"audio"`
				Flac struct {
					Audio []struct {
						BaseURL string `json:"baseUrl"`
					} `json:"audio"`
				} `json:"flac"`
			} `json:"dash"`
		} `json:"data"`
	}
	json.Unmarshal(body, &resp)

	if len(resp.Data.Dash.Flac.Audio) > 0 {
		return resp.Data.Dash.Flac.Audio[0].BaseURL, nil
	}
	if len(resp.Data.Dash.Audio) > 0 {
		return resp.Data.Dash.Audio[0].BaseURL, nil
	}
	if len(resp.Data.Durl) > 0 {
		return resp.Data.Durl[0].URL, nil
	}

	return "", errors.New("no audio found")
}

func (b *Bilibili) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "bilibili" {
		return "", errors.New("source mismatch")
	}
	return "", nil
}