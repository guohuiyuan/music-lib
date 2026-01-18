package bilibili

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 Edg/121.0.0.0"
	Referer   = "https://www.bilibili.com/"
	Origin    = "https://www.bilibili.com"
	Accept    = "application/json, text/plain, */*"
)

// Search 搜索 B 站视频并展开为分 P 音频
func Search(keyword string) ([]model.Song, error) {
	// 1. 构造搜索请求
	params := url.Values{}
	params.Set("search_type", "video")
	params.Set("keyword", keyword)
	params.Set("page", "1")
	params.Set("page_size", "10")

	searchURL := "https://api.bilibili.com/x/web-interface/search/type?" + params.Encode()

	// 2. 发送搜索请求
	// 注意：B站 API 对 Cookie 有一定依赖，这里使用基础 Header
	// 如果需要更高权限（如 1080P/无损音频搜索），需要在 utils.Get 中注入 Cookie
	body, err := utils.Get(searchURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Origin", Origin),
		utils.WithHeader("Accept", Accept),
	)
	if err != nil {
		return nil, err
	}

	// 3. 解析搜索结果
	var searchResp struct {
		Data struct {
			Result []struct {
				BVID   string `json:"bvid"`
				Title  string `json:"title"` // 往往包含 HTML 标签
				Author string `json:"author"`
				Pic    string `json:"pic"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("bilibili search json error: %w", err)
	}

	var songs []model.Song

	// 4. [核心逻辑] 遍历每个视频，获取分 P 详情 (View API)
	// Python 代码是串行请求 view 接口，这里保持逻辑一致
	for _, item := range searchResp.Data.Result {
		// 清洗标题 (去除 <em> 标签)
		rootTitle := strings.ReplaceAll(strings.ReplaceAll(item.Title, "<em class=\"keyword\">", ""), "</em>", "")

		// 请求 View 接口获取分 P
		viewURL := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?bvid=%s", item.BVID)
		viewBody, err := utils.Get(viewURL,
			utils.WithHeader("User-Agent", UserAgent),
			utils.WithHeader("Referer", Referer),
			utils.WithHeader("Origin", Origin),
			utils.WithHeader("Accept", Accept),
		)
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

		// 5. 展开分 P
		for _, page := range viewResp.Data.Pages {
			// 构造复合 ID: BVID|CID
			// 因为下载接口同时需要这两个参数
			compoundID := fmt.Sprintf("%s|%d", item.BVID, page.CID)

			// 构造显示标题
			displayTitle := page.Part
			if len(viewResp.Data.Pages) == 1 && displayTitle == "" {
				displayTitle = rootTitle // 如果只有 1 P，直接用视频标题
			} else if displayTitle != rootTitle {
				displayTitle = fmt.Sprintf("%s - %s", rootTitle, displayTitle)
			}

			// 处理封面图 (B站 API 返回的图片通常缺协议头)
			cover := item.Pic
			if strings.HasPrefix(cover, "//") {
				cover = "https:" + cover
			}

			songs = append(songs, model.Song{
				Source:   "bilibili",
				ID:       compoundID,
				Name:     displayTitle,
				Artist:   item.Author,
				Album:    item.BVID, // 用 BVID 做专辑名
				Duration: page.Duration,
				// URL 和 Size 需要在 GetDownloadURL 中获取
			})
		}
	}

	return songs, nil
}

// GetDownloadURL 获取音频流链接
// 对应 Python 中的 playurl 调用及 audios 排序逻辑
func GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "bilibili" {
		return "", errors.New("source mismatch")
	}

	// 1. 解析复合 ID
	parts := strings.Split(s.ID, "|")
	if len(parts) != 2 {
		return "", errors.New("invalid bilibili id format")
	}
	bvid := parts[0]
	cid := parts[1]

	// 2. 构造 PlayURL 请求
	// fnval=16 代表请求 DASH 格式 (音视频分离)，这样能单独提取音频
	apiURL := fmt.Sprintf("https://api.bilibili.com/x/player/playurl?fnval=16&bvid=%s&cid=%s", bvid, cid)

	// 3. 发送请求
	// Python 代码中使用了大量的 Cookie，如果不传 Cookie，通常只能拿到 128k/64k 音频
	// 如果需要高音质，需要在 utils.Get 中通过 WithHeader("Cookie", "SESSDATA=...") 传入
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Origin", Origin),
		utils.WithHeader("Accept", Accept),
	)
	if err != nil {
		return "", err
	}

	// 4. 解析 DASH 数据
	// 结构比较复杂，需要提取不同层级的 audio
	var resp struct {
		Data struct {
			Dash struct {
				// 普通音频
				Audio []DashStream `json:"audio"`
				// 无损音频
				Flac struct {
					Audio []DashStream `json:"audio"`
				} `json:"flac"`
				// 杜比音频
				Dolby struct {
					Audio []DashStream `json:"audio"`
				} `json:"dolby"`
			} `json:"dash"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("playurl json error: %w", err)
	}

	// 5. [核心逻辑] 优先级选择 (Flac > Dolby > Audio)
	// Python: audios = flac or dolby or audio
	var candidates []DashStream
	
	if len(resp.Data.Dash.Flac.Audio) > 0 {
		candidates = resp.Data.Dash.Flac.Audio
	} else if len(resp.Data.Dash.Dolby.Audio) > 0 {
		candidates = resp.Data.Dash.Dolby.Audio
	} else if len(resp.Data.Dash.Audio) > 0 {
		candidates = resp.Data.Dash.Audio
	}

	if len(candidates) == 0 {
		return "", errors.New("no audio stream found")
	}

	// 6. 排序取最佳
	// Python: sorted(audios, key=lambda x: x.get("bandwidth", 0)... reverse=True)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Bandwidth > candidates[j].Bandwidth
	})

	bestStream := candidates[0]
	
	// 优先取 BaseURL，没有则取 BackupURL
	if bestStream.BaseURL != "" {
		return bestStream.BaseURL, nil
	}
	if len(bestStream.BackupURL) > 0 {
		return bestStream.BackupURL[0], nil
	}

	return "", errors.New("no valid url in stream")
}

// DashStream 辅助结构体
type DashStream struct {
	ID        int      `json:"id"`
	BaseURL   string   `json:"baseUrl"` // 注意 json tag 大小写
	BackupURL []string `json:"backupUrl"`
	Bandwidth int      `json:"bandwidth"`
	MimeType  string   `json:"mimeType"`
}
