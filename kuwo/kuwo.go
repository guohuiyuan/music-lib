package kuwo

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	// 对应 Python default_search_headers
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
)

// Search 搜索歌曲
// 对应 Python: _search 方法
func Search(keyword string) ([]model.Song, error) {
	// 1. 构造参数
	// Python: default_rule = { "vipver": "1", "client": "kt", "ft": "music", ... }
	params := url.Values{}
	params.Set("vipver", "1")
	params.Set("client", "kt")
	params.Set("ft", "music")
	params.Set("cluster", "0")
	params.Set("strategy", "2012")
	params.Set("encoding", "utf8")
	params.Set("rformat", "json")
	params.Set("mobi", "1")
	params.Set("issubtitle", "1")
	params.Set("show_copyright_off", "1")
	params.Set("pn", "0")  // 页码
	params.Set("rn", "10") // 每页数量
	params.Set("all", keyword)

	apiURL := "http://www.kuwo.cn/search/searchMusicBykeyWord?" + params.Encode()

	// 2. 发送请求
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
	)
	if err != nil {
		return nil, err
	}

	// 3. 解析 JSON
	var resp struct {
		AbsList []struct {
			MusicRID string `json:"MUSICRID"` // 例如 "MUSIC_26319889"
			SongName string `json:"SONGNAME"`
			Artist   string `json:"ARTIST"`
			Album    string `json:"ALBUM"`
			Duration string `json:"DURATION"`  // 秒 (API返回字符串)
			HtsMVPic string `json:"hts_MVPIC"` // 封面
			// 支付信息
			PayInfo string `json:"pay"` // 例如 "pay_down" 或 具体价格
		} `json:"abslist"`
	}

	// 酷我搜索有时返回非标准 JSON，但 utils.Get 拿到的是 bytes，通常 Unmarshal 能处理
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kuwo json parse error: %w", err)
	}

	// 4. 转换模型
	var songs []model.Song
	for _, item := range resp.AbsList {
		// --- 核心过滤逻辑 ---
		// 如果 pay 字段包含具体付费标识
		// 酷我的 pay 字段比较乱，有时是 "0"，有时是 "pay_download"
		// 简单的过滤策略:
		pay := strings.ToLower(item.PayInfo)
		if strings.Contains(pay, "pay") && pay != "0" {
			// 包含 "pay" 且不是 "0"，很可能是付费
			continue
		}
		// 空字符串或 "0" 或没有 "pay" 字符串的通过

		// 处理 ID: 去掉 "MUSIC_" 前缀
		// Python: search_result['MUSICRID'].removeprefix('MUSIC_')
		cleanID := strings.TrimPrefix(item.MusicRID, "MUSIC_")

		// 将字符串类型的 Duration 转换为整数
		duration, _ := strconv.Atoi(item.Duration)

		songs = append(songs, model.Song{
			Source:   "kuwo",
			ID:       cleanID,
			Name:     item.SongName,
			Artist:   item.Artist,
			Album:    item.Album,
			Duration: duration,
			// Size 未知，需下载时获取
		})
	}

	return songs, nil
}

// GetDownloadURL 获取下载链接
// 对应 Python: 遍历 MUSIC_QUALITIES 调用 mobi.kuwo.cn 接口
func GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "kuwo" {
		return "", errors.New("source mismatch")
	}

	// 定义音质列表 (从高到低尝试)
	// Python: [(4000, '4000kflac'), (2000, '2000kflac'), (1000, 'flac'), (320, '320kmp3'), (192, '192kmp3'), (128, '128kmp3')][1:]
	// 注意：4000kflac 可能是加密格式，Python 代码跳过了它 ([1:])。我们也从 2000k 开始。
	qualities := []string{
		"2000kflac", // Hi-Res
		"flac",      // 无损
		"320kmp3",   // 高品
		"192kmp3",
		"128kmp3",   // 标准
	}

	for _, br := range qualities {
		// 构造 API URL
		// Python: https://mobi.kuwo.cn/mobi.s?f=web&source=kwplayercar...
		params := url.Values{}
		params.Set("f", "web")
		params.Set("source", "kwplayercar_ar_6.0.0.9_B_jiakong_vh.apk") // 关键：模拟车载客户端
		params.Set("from", "PC")
		params.Set("type", "convert_url_with_sign")
		params.Set("br", br)
		params.Set("rid", s.ID) // 注意这里要是纯数字 ID
		// Python: user=C_APK_guanwang_...
		params.Set("user", "C_APK_guanwang_12609069939969033731")

		apiURL := "https://mobi.kuwo.cn/mobi.s?" + params.Encode()

		// 发送请求
		body, err := utils.Get(apiURL,
			utils.WithHeader("User-Agent", UserAgent),
		)
		if err != nil {
			continue // 网络错误尝试下一个音质
		}

		// 解析响应
		var resp struct {
			Data struct {
				URL     string `json:"url"`
				Bitrate int    `json:"bitrate"`
				Format  string `json:"format"`
			} `json:"data"`
		}

		// 酷我这个接口如果失败可能返回 HTML 或不含 data 的 JSON，忽略错误继续尝试
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}

		if resp.Data.URL != "" {
			// 成功获取链接
			// 可以在这里反写文件大小估算值，或者更新扩展名
			// 例如 s.Size = ...
			return resp.Data.URL, nil
		}
	}

	return "", errors.New("download url not found (copyright restricted)")
}

// GetLyrics 获取歌词 (补充功能，参考 Python 代码实现)
func GetLyrics(s *model.Song) (string, error) {
	// Python: http://m.kuwo.cn/newh5/singles/songinfoandlrc?musicId=...
	params := url.Values{}
	params.Set("musicId", s.ID)
	params.Set("httpsStatus", "1")
	
	apiURL := "http://m.kuwo.cn/newh5/singles/songinfoandlrc?" + params.Encode()
	
	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent))
	if err != nil {
		return "", err
	}
	
	var resp struct {
		Data struct {
			LrcList []struct {
				Time string `json:"time"` // float string like "12.34"
				Line string `json:"lineLyric"`
			} `json:"lrclist"`
		} `json:"data"`
	}
	
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	
	// 拼接 LRC 格式
	var sb strings.Builder
	for _, line := range resp.Data.LrcList {
		// 需要将 "12.34" 转换为 [00:12.34] 格式，这里简化处理，直接拼接
		t, _ := strconv.ParseFloat(line.Time, 64)
		minutes := int(t) / 60
		seconds := float64(int(t)%60) + (t - float64(int(t)))
		sb.WriteString(fmt.Sprintf("[%02d:%05.2f]%s\n", minutes, seconds, line.Line))
	}
	
	return sb.String(), nil
}
