package qianqian

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	AppID     = "16073360"
	Secret    = "0b50b02fd0d73a9c4c8c3a781c30845f"
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"
	Referer   = "https://music.91q.com/player"
)

// Search 搜索歌曲
// 对应 Python: _search 方法
func Search(keyword string) ([]model.Song, error) {
	// 1. 构造基础参数
	params := url.Values{}
	params.Set("word", keyword)
	params.Set("type", "1")
	params.Set("pageNo", "1")
	params.Set("pageSize", "10")
	params.Set("appid", AppID)

	// 2. 签名
	signParams(params)

	apiURL := "https://music.91q.com/v1/search?" + params.Encode()

	// 3. 发送请求
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
	)
	if err != nil {
		return nil, err
	}

	// 4. 解析 JSON
	var resp struct {
		Data struct {
			TypeTrack []struct {
				TSID       string `json:"TSID"`
				Title      string `json:"title"`
				AlbumTitle string `json:"albumTitle"`
				Pic        string `json:"pic"`      // [新增] 封面
				Duration   int    `json:"duration"` // [新增] 时长
				Lyric      string `json:"lyric"`
				Artist     []struct {
					Name string `json:"name"`
				} `json:"artist"`
				// [新增] 音质信息，用于获取文件大小
				RateFileInfo map[string]struct {
					Size   int64  `json:"size"`
					Format string `json:"format"`
				} `json:"rateFileInfo"`
			} `json:"typeTrack"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qianqian json parse error: %w", err)
	}

	// 5. 转换模型
	var songs []model.Song
	for _, item := range resp.Data.TypeTrack {
		// 拼接歌手名
		var artistNames []string
		for _, ar := range item.Artist {
			artistNames = append(artistNames, ar.Name)
		}

		// [新增] 计算文件大小
		// 逻辑：遍历下载时尝试的音质顺序，找到第一个存在的大小
		// 顺序与 GetDownloadURL 保持一致：3000(无损) > 320(高品) > 128(标准) > 64
		var size int64
		rates := []string{"3000", "320", "128", "64"}
		for _, r := range rates {
			if info, ok := item.RateFileInfo[r]; ok && info.Size > 0 {
				size = info.Size
				break
			}
		}

		songs = append(songs, model.Song{
			Source:   "qianqian",
			ID:       item.TSID,
			Name:     item.Title,
			Artist:   strings.Join(artistNames, "、"),
			Album:    item.AlbumTitle,
			Duration: item.Duration, // [新增] 填充时长
			Size:     size,          // [新增] 填充大小
			Cover:    item.Pic,      // [新增] 填充封面
		})
	}

	return songs, nil
}

// GetDownloadURL 获取下载链接
// 对应 Python: 遍历 MUSIC_QUALITIES 调用 tracklink 接口
func GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "qianqian" {
		return "", errors.New("source mismatch")
	}

	// 定义音质列表 (从高到低尝试)
	// 3000=无损, 320=高品, 128=标准
	qualities := []string{"3000", "320", "128", "64"}

	for _, rate := range qualities {
		// 1. 构造参数
		params := url.Values{}
		params.Set("TSID", s.ID)
		params.Set("appid", AppID)
		params.Set("rate", rate)

		// 2. 签名
		signParams(params)

		apiURL := "https://music.91q.com/v1/song/tracklink?" + params.Encode()

		// 3. 发送请求
		body, err := utils.Get(apiURL,
			utils.WithHeader("User-Agent", UserAgent),
			utils.WithHeader("Referer", Referer),
		)
		if err != nil {
			continue
		}

		// 4. 解析响应
		var resp struct {
			Data struct {
				Path           string `json:"path"`
				Format         string `json:"format"`
				Size           int64  `json:"size"`
				Duration       int    `json:"duration"`
				TrailAudioInfo struct {
					Path string `json:"path"`
				} `json:"trail_audio_info"`
				IsVip int `json:"isVip"`
			} `json:"data"`
		}

		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}

		// 获取下载链接，优先使用 path，如果没有则尝试 trail_audio_info.path
		downloadURL := resp.Data.Path
		if downloadURL == "" {
			downloadURL = resp.Data.TrailAudioInfo.Path
		}

		if downloadURL != "" {
			return downloadURL, nil
		}
	}

	return "", errors.New("download url not found")
}

// 辅助函数：参数签名
// Python: _addsignandtstoparams
func signParams(v url.Values) {
	// 1. 添加时间戳
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	v.Set("timestamp", timestamp)

	// 2. 提取所有 Key 并排序
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 3. 拼接字符串 k=v&k=v...
	var buf strings.Builder
	for i, k := range keys {
		if i > 0 {
			buf.WriteString("&")
		}
		buf.WriteString(k)
		buf.WriteString("=")
		buf.WriteString(v.Get(k))
	}

	// 4. 追加 Secret
	buf.WriteString(Secret)

	// 5. 计算 MD5
	hash := md5.Sum([]byte(buf.String()))
	sign := hex.EncodeToString(hash[:])

	// 6. 设置 sign 参数
	v.Set("sign", sign)
}