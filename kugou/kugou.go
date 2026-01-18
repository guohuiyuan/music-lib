package kugou

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

// 酷狗移动端伪装 Header，对应 Python 代码中的 config.get("ios_headers")
const (
	MobileUserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.0.3 Mobile/15E148 Safari/604.1"
	MobileReferer   = "http://m.kugou.com"
)

// Search 搜索歌曲
// 对应 Python: kugou_search 函数
func Search(keyword string) ([]model.Song, error) {
	// 1. 构造请求参数
	// Python: params = dict(keyword=keyword, platform="WebFilter", format="json", page=1, pagesize=number)
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("platform", "WebFilter")
	params.Set("format", "json")
	params.Set("page", "1")
	params.Set("pagesize", "10")

	apiURL := "http://songsearch.kugou.com/song_search_v2?" + params.Encode()

	// 2. 发送请求
	body, err := utils.Get(apiURL, 
		utils.WithHeader("User-Agent", MobileUserAgent),
	)
	if err != nil {
		return nil, err
	}

	// 3. 解析响应结构
	var resp struct {
		Data struct {
			Lists []struct {
				Scid       interface{} `json:"Scid"`       // 可能是字符串或数字
				SongName   string      `json:"SongName"`
				SingerName string      `json:"SingerName"`
				AlbumName  string      `json:"AlbumName"`
				Duration   int         `json:"Duration"`
				FileHash   string      `json:"FileHash"`   // 普通音质 Hash
				SQFileHash string      `json:"SQFileHash"` // 无损音质 Hash
				HQFileHash string      `json:"HQFileHash"` // 高频音质 Hash
				FileSize   interface{} `json:"FileSize"`   // 可能是数字或字符串
				// 支付信息
				PayType   int `json:"PayType"`   // 关键字段
				Privilege int `json:"Privilege"` // 权限字段 (10通常是无版权)
			} `json:"lists"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}

	// 4. 转换数据模型
	var songs []model.Song
	for _, item := range resp.Data.Lists {
		// --- 核心过滤逻辑 ---
		// 1. 过滤付费 (PayType == 1 可能是VIP，但需要验证)
		// 暂时注释掉，因为 PayType=3 对于所有歌曲都相同，但有些可以下载
		// if item.PayType == 1 {
		// 	continue
		// }
		
		// 2. 过滤无版权 (Privilege == 10 经常出现在无版权歌曲中)
		// 暂时注释掉，因为所有歌曲都是 Privilege=10
		// if item.Privilege == 10 {
		// 	continue
		// }
		
		// 3. 确保有 Hash
		if item.FileHash == "" && item.SQFileHash == "" && item.HQFileHash == "" {
			continue
		}

		// --- 核心逻辑移植：音质 Hash 选择 ---
		// Python: keys_list = ["SQFileHash", "HQFileHash"] 优先选择高品质
		finalHash := item.FileHash
		if isValidHash(item.SQFileHash) {
			finalHash = item.SQFileHash
		} else if isValidHash(item.HQFileHash) {
			finalHash = item.HQFileHash
		}

		// 处理文件大小
		var size int64
		switch v := item.FileSize.(type) {
		case float64:
			size = int64(v)
		case int:
			size = int64(v)
		case string:
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				size = i
			}
		}

		songs = append(songs, model.Song{
			Source:   "kugou",
			ID:       finalHash, // 将计算出的最佳 Hash 作为 ID
			Name:     item.SongName,
			Artist:   item.SingerName,
			Album:    item.AlbumName,
			Duration: item.Duration,
			Size:     size,
		})
	}

	return songs, nil
}

// GetDownloadURL 获取下载链接
// 对应 Python: KugouSong.download 方法
func GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "kugou" {
		return "", errors.New("source mismatch")
	}

	// 1. 构造请求
	// Python: params = dict(cmd="playInfo", hash=self.hash)
	// API: http://m.kugou.com/app/i/getSongInfo.php
	params := url.Values{}
	params.Set("cmd", "playInfo")
	params.Set("hash", s.ID) // 这里的 ID 就是我们在 Search 中选定的最佳 Hash

	apiURL := "http://m.kugou.com/app/i/getSongInfo.php?" + params.Encode()

	// 2. 发送请求 (必须带上特定的 Header)
	// Python: session.headers.update({"referer": "http://m.kugou.com", "User-Agent": ...})
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
	)
	if err != nil {
		return "", err
	}

	// 3. 解析响应
	var resp struct {
		URL      string      `json:"url"`
		BitRate  int         `json:"bitRate"`
		ExtName  string      `json:"extName"`
		AlbumImg string      `json:"album_img"`
		Error    interface{} `json:"error"` // 有时会返回错误信息
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("json parse error: %w", err)
	}

	if resp.URL == "" {
		return "", errors.New("download url not found (might be paid song)")
	}

	return resp.URL, nil
}

// 辅助函数：判断 Hash 是否有效
// Python: if hash and hash != "00000000000000000000000000000000":
func isValidHash(h string) bool {
	return h != "" && h != "00000000000000000000000000000000"
}
