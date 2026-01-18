package migu

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	// 对应 Python config.get("ios_useragent")
	UserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"
	Referer   = "http://music.migu.cn/"

	// 硬编码的 UserID，来自 Python 源码
	MagicUserID = "15548614588710179085069"
)

// Search 搜索歌曲
func Search(keyword string) ([]model.Song, error) {
	// 1. 构造参数
	// Python: searchSwitch='{"song":1,"album":0,"singer":0,"tagSong":0,"mvSong":0,"songlist":0,"bestShow":1}'
	params := url.Values{}
	params.Set("ua", "Android_migu")
	params.Set("version", "5.0.1")
	params.Set("text", keyword)
	params.Set("pageNo", "1")
	params.Set("pageSize", "10")
	params.Set("searchSwitch", `{"song":1,"album":0,"singer":0,"tagSong":0,"mvSong":0,"songlist":0,"bestShow":1}`)

	apiURL := "http://pd.musicapp.migu.cn/MIGUM2.0/v1.0/content/search_all.do?" + params.Encode()

	// 2. 发送请求
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
	)
	if err != nil {
		return nil, err
	}

	// 3. 解析 JSON
	// 结构比较深，定义所需的字段
	var resp struct {
		SongResultData struct {
			Result []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Singers  []struct {
					Name string `json:"name"`
				} `json:"singers"`
				Albums []struct {
					Name string `json:"name"`
				} `json:"albums"`
				ContentID   string `json:"contentId"` // 核心ID
				ImgItems    []struct {
					Img string `json:"img"`
				} `json:"imgItems"`
				// 音质列表
				RateFormats []struct {
					FormatType   string `json:"formatType"`   // SQ, HQ...
					ResourceType string `json:"resourceType"` // E, Z...
					Size         string `json:"size"`         // 注意 API 返回的是字符串
					FileType     string `json:"fileType"`     // flac, mp3
				} `json:"rateFormats"`
			} `json:"result"`
		} `json:"songResultData"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("migu json parse error: %w", err)
	}

	// 4. 转换模型
	var songs []model.Song
	for _, item := range resp.SongResultData.Result {
		// 拼接歌手
		var artistNames []string
		for _, s := range item.Singers {
			artistNames = append(artistNames, s.Name)
		}
		
		// 获取专辑名
		albumName := ""
		if len(item.Albums) > 0 {
			albumName = item.Albums[0].Name
		}

		// --- 核心逻辑：选择最佳音质 ---
		// Python: sorted(rate_list, key=lambda x: int(x["size"]), reverse=True)
		// 如果没有音质列表，跳过
		if len(item.RateFormats) == 0 {
			continue
		}

		// 排序，找最大的文件
		sort.Slice(item.RateFormats, func(i, j int) bool {
			sizeI, _ := strconv.ParseInt(item.RateFormats[i].Size, 10, 64)
			sizeJ, _ := strconv.ParseInt(item.RateFormats[j].Size, 10, 64)
			return sizeI > sizeJ // 降序
		})

		bestFormat := item.RateFormats[0]
		
		// 技巧：我们将 contentId, resourceType, formatType 组合存入 ID
		// 这样 GetDownloadURL 就不需要再次请求，也不需要改动通用的 Song 结构体
		// 格式: ContentID|ResourceType|FormatType
		compoundID := fmt.Sprintf("%s|%s|%s", item.ContentID, bestFormat.ResourceType, bestFormat.FormatType)

		// 计算大小 (MB)
		sizeBytes, _ := strconv.ParseInt(bestFormat.Size, 10, 64)
		
		songs = append(songs, model.Song{
			Source:   "migu",
			ID:       compoundID, // 复合 ID
			Name:     item.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    albumName,
			Size:     sizeBytes, // 字节数
			Duration: 0,         // 搜索接口未直接返回时长，暂置0
		})
	}

	return songs, nil
}

// GetDownloadURL 获取真实下载链接
func GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "migu" {
		return "", errors.New("source mismatch")
	}

	// 1. 解包复合 ID
	// 格式: ContentID|ResourceType|FormatType
	parts := strings.Split(s.ID, "|")
	if len(parts) != 3 {
		return "", errors.New("invalid migu song id format")
	}
	contentID := parts[0]
	resourceType := parts[1]
	formatType := parts[2]

	// 2. 构造请求 URL
	// Python: http://app.pd.nf.migu.cn/MIGUM2.0/v1.0/content/sub/listenSong.do?...
	params := url.Values{}
	params.Set("toneFlag", formatType)
	params.Set("netType", "00")
	params.Set("userId", MagicUserID) // 必须带上这个
	params.Set("ua", "Android_migu")
	params.Set("version", "5.1")
	params.Set("copyrightId", "0")
	params.Set("contentId", contentID)
	params.Set("resourceType", resourceType)
	params.Set("channel", "0")

	// 注意：Migu 这个接口生成的 URL 往往就是最终重定向的地址
	// 或者本身就是 mp3/flac 流地址。
	// Python 代码是直接把它当作 song_url。
	// 我们这里直接返回构造好的 URL 即可，因为这是一个 API 接口，不是最终文件地址
	// 但通常客户端直接访问这个 API 会被重定向到 CDN 文件。
	
	finalAPI := "http://app.pd.nf.migu.cn/MIGUM2.0/v1.0/content/sub/listenSong.do?" + params.Encode()
	
	// 可选：为了确保链接有效，我们可以发一个 HEAD 请求，或者获取重定向后的真实 URL
	// 这里为了简单和速度，直接返回 API 地址 (许多播放器能处理重定向)
	// 如果需要真实 CDN 地址，可以使用 utils.Get 获取 Header Location，但 Migu 这个通常直接能用。
	
	return finalAPI, nil
}
