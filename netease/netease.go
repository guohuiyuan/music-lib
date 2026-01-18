package netease

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
	Referer = "http://music.163.com/"
	// 搜索接口 (通过 linux forward 转发)
	SearchAPI = "http://music.163.com/api/linux/forward"
	// 下载接口 (WeApi)
	DownloadAPI = "http://music.163.com/weapi/song/enhance/player/url"
)

// Search 搜索歌曲
// Python: netease_search
func Search(keyword string) ([]model.Song, error) {
	// 1. 构造内部 eparams (将被 AES-ECB 加密)
	eparams := map[string]interface{}{
		"method": "POST",
		"url":    "http://music.163.com/api/cloudsearch/pc",
		"params": map[string]interface{}{
			"s":      keyword,
			"type":   1,
			"offset": 0,
			"limit":  10, // 默认 10 条
		},
	}
	eparamsJSON, err := json.Marshal(eparams)
	if err != nil {
		return nil, fmt.Errorf("json marshal error: %w", err)
	}

	// 2. 加密参数
	encryptedParam := EncryptLinux(string(eparamsJSON))
	
	// 3. 构造 POST 表单数据
	form := url.Values{}
	form.Set("eparams", encryptedParam)

	// 4. 发送请求
	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
	}
	
	body, err := utils.Post(SearchAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return nil, err
	}

	// 5. 解析 JSON (增加 Fee 和 Privilege 字段定义)
	var resp struct {
		Result struct {
			Songs []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
				Ar   []struct {
					Name string `json:"name"`
				} `json:"ar"` // 歌手
				Al struct {
					Name   string `json:"name"`
					PicURL string `json:"picUrl"`
				} `json:"al"` // 专辑
				Dt        int `json:"dt"` // 时长 (ms)
				Fee       int `json:"fee"` // 关键字段: 0:免费, 1:VIP, 8:部分免费, 4:付费
				Privilege struct {
					Fl int `json:"fl"` // 版权标记
					Pl int `json:"pl"` // 播放等级
				} `json:"privilege"`
			} `json:"songs"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease json parse error: %w", err)
	}

	// 6. 转换模型
	var songs []model.Song
	for _, item := range resp.Result.Songs {
		// --- 核心过滤逻辑 ---
		// 1. 过滤无版权 (fl == 0 通常表示无版权或下架)
		// Python源码中写的是: if item.get("privilege").get("fl") == 0: continue
		if item.Privilege.Fl == 0 {
			continue
		}

		// 2. 过滤 VIP 和 付费专辑
		// fee == 1: 纯 VIP 歌曲
		// fee == 4: 付费专辑购买
		if item.Fee == 1 || item.Fee == 4 {
			continue
		}
		// fee == 8 通常是可以试听或下载低音质的，这里暂且保留

		var artistNames []string
		for _, ar := range item.Ar {
			artistNames = append(artistNames, ar.Name)
		}

		songs = append(songs, model.Song{
			Source:   "netease",
			ID:       fmt.Sprintf("%d", item.ID),
			Name:     item.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    item.Al.Name,
			Duration: item.Dt / 1000,
			// Size 这里暂不处理，因为搜索结果里的 Size 字段非常混乱 (h, m, l 节点)
			Size:     0,
		})
	}

	return songs, nil
}

// GetDownloadURL 获取下载链接
// Python: NeteaseSong.download
func GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "netease" {
		return "", errors.New("source mismatch")
	}

	// 1. 构造原始参数
	reqData := map[string]interface{}{
		"ids": []string{s.ID}, // 注意 ID 要放在数组里
		"br":  320000,         // 320k 码率
	}
	reqJSON, err := json.Marshal(reqData)
	if err != nil {
		return "", fmt.Errorf("json marshal error: %w", err)
	}

	// 2. WeApi 加密 (AES-CBC + RSA)
	params, encSecKey := EncryptWeApi(string(reqJSON))

	// 3. 构造 POST 表单
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	// 4. 发送请求
	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
	}

	body, err := utils.Post(DownloadAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return "", err
	}

	// 5. 解析响应
	var resp struct {
		Data []struct {
			URL  string `json:"url"`
			Code int    `json:"code"`
			Br   int    `json:"br"` // 实际码率
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("json parse error: %w", err)
	}

	if len(resp.Data) == 0 || resp.Data[0].URL == "" {
		return "", errors.New("download url not found (might be vip or copyright restricted)")
	}

	return resp.Data[0].URL, nil
}
