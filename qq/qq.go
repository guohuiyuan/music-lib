package qq

import (
	"bytes"
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
	// 对应 Python config.get("ios_useragent")
	UserAgent = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"
	
	// 搜索 Referer
	SearchReferer = "http://m.y.qq.com"
	// 下载 Referer
	DownloadReferer = "http://y.qq.com"
)

// Search 搜索歌曲
func Search(keyword string) ([]model.Song, error) {
	// 1. 构造参数
	// Python: params = {"w": keyword, "format": "json", "p": 1, "n": number}
	params := url.Values{}
	params.Set("w", keyword)
	params.Set("format", "json")
	params.Set("p", "1")
	params.Set("n", "10")

	apiURL := "http://c.y.qq.com/soso/fcgi-bin/search_for_qq_cp?" + params.Encode()

	// 2. 发送请求
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", SearchReferer),
	)
	if err != nil {
		return nil, err
	}

	// 3. 解析 JSON (增加 Pay 对象)
	var resp struct {
		Data struct {
			Song struct {
				List []struct {
					SongID    int64  `json:"songid"`
					SongName  string `json:"songname"`
					SongMID   string `json:"songmid"` // 关键字段
					AlbumName string `json:"albumname"`
					Interval  int    `json:"interval"` // 时长
					Size128   int64  `json:"size128"`
					Singer    []struct {
						Name string `json:"name"`
					} `json:"singer"`
					// 支付信息
					Pay struct {
						PayDown    int `json:"pay_down"`    // 1表示付费下载
						PayPlay    int `json:"pay_play"`    // 1表示付费播放
						PriceTrack int `json:"price_track"` // 价格
					} `json:"pay"`
				} `json:"list"`
			} `json:"song"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq json parse error: %w", err)
	}

	// 4. 转换模型
	var songs []model.Song
	for _, item := range resp.Data.Song.List {
		// --- 核心过滤逻辑 ---
		// 过滤 VIP 和 付费歌曲
		if item.Pay.PayDown == 1 || item.Pay.PriceTrack > 0 {
			continue
		}

		// 拼接歌手名
		var artistNames []string
		for _, s := range item.Singer {
			artistNames = append(artistNames, s.Name)
		}

		songs = append(songs, model.Song{
			Source:   "qq",
			ID:       item.SongMID, // QQ 使用 SongMID 作为下载凭证，而非 SongID
			Name:     item.SongName,
			Artist:   strings.Join(artistNames, "、"),
			Album:    item.AlbumName,
			Duration: item.Interval,
			Size:     item.Size128,
		})
	}

	return songs, nil
}

// GetDownloadURL 获取下载链接
// 参考 Python 代码中的 _search 方法里 "non-vip / vip users using endpoint" 部分
func GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "qq" {
		return "", errors.New("source mismatch")
	}

	// 1. 生成随机 GUID
	rand.Seed(time.Now().UnixNano())
	guid := fmt.Sprintf("%d", rand.Int63n(9000000000)+1000000000)

	// 2. 定义音质列表
	// Python 代码逻辑：尝试不同前缀拼接文件名
	// M500: 128kbps mp3 (最常用)
	// C400: m4a
	type Rate struct {
		Prefix string
		Ext    string
	}
	rates := []Rate{
		{"M500", "mp3"},
		{"C400", "m4a"},
	}

	// 3. 循环尝试获取播放地址
	for _, r := range rates {
		// 构造文件名: 前缀 + SongMID + SongMID + 后缀
		// 对应 Python: f"{quality[0]}{search_result['mid']}{search_result['mid']}{quality[1]}"
		// 注意：Python代码里这里用了两次 mid，这是 UrlGetVkey 接口的一种特征
		filename := fmt.Sprintf("%s%s%s.%s", r.Prefix, s.ID, s.ID, r.Ext)

		// 构造 musicu.fcg 的请求体
		// 对应 Python: QQMusicClientUtils.buildrequestdata(module="music.vkey.GetVkey", method="UrlGetVkey", ...)
		reqData := map[string]interface{}{
			"comm": map[string]interface{}{
				"cv":                4747474,
				"ct":                24,
				"format":            "json",
				"inCharset":         "utf-8",
				"outCharset":        "utf-8",
				"notice":            0,
				"platform":          "yqq.json",
				"needNewCode":       1,
				"uin":               0,
				"g_tk_new_20200303": 5381,
				"g_tk":              5381,
			},
			"req_1": map[string]interface{}{
				"module": "music.vkey.GetVkey",
				"method": "UrlGetVkey",
				"param": map[string]interface{}{
					"guid":      guid,
					"songmid":   []string{s.ID},
					"songtype":  []int{0},
					"uin":       "0",
					"loginflag": 1,
					"platform":  "20",
					"filename":  []string{filename},
				},
			},
		}

		jsonData, err := json.Marshal(reqData)
		if err != nil {
			continue
		}

		// 发送 POST 请求到统一接口
		// 使用 utils.Post 方法
		headers := []utils.RequestOption{
			utils.WithHeader("User-Agent", UserAgent),
			utils.WithHeader("Referer", DownloadReferer),
			utils.WithHeader("Content-Type", "application/json"),
		}
		
		body, err := utils.Post("https://u.y.qq.com/cgi-bin/musicu.fcg", bytes.NewReader(jsonData), headers...)
		if err != nil {
			continue
		}

		// 解析响应
		// 路径: req_1 -> data -> midurlinfo -> [0] -> purl
		var result struct {
			Req1 struct {
				Data struct {
					MidUrlInfo []struct {
						Purl    string `json:"purl"`
						WifiUrl string `json:"wifiurl"`
					} `json:"midurlinfo"`
				} `json:"data"`
			} `json:"req_1"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		// 检查是否获取到有效链接
		if len(result.Req1.Data.MidUrlInfo) > 0 {
			info := result.Req1.Data.MidUrlInfo[0]
			purl := info.Purl
			if purl == "" {
				purl = info.WifiUrl
			}

			if purl != "" {
				// 拼接最终域名
				// 对应 Python: QQMusicClientUtils.music_domain + download_url
				// 常用域名: http://ws.stream.qqmusic.qq.com/
				return "http://ws.stream.qqmusic.qq.com/" + purl, nil
			}
		}
	}

	return "", errors.New("download url not found (copyright restricted or vip required)")
}
