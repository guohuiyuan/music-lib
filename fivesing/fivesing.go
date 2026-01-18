package fivesing

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	// 对应 Python default_search_headers
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
)

// Search 搜索歌曲
// 对应 Python: _search 方法前半部分
func Search(keyword string) ([]model.Song, error) {
	// 1. 构造搜索参数
	// Python: {'keyword': keyword, 'sort': 1, 'page': 1, 'filter': 0, 'type': 0}
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("sort", "1")
	params.Set("page", "1")
	params.Set("filter", "0")
	params.Set("type", "0")

	apiURL := "http://search.5sing.kugou.com/home/json?" + params.Encode()

	// 2. 发送请求
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
	)
	if err != nil {
		return nil, err
	}

	// 3. 解析 JSON
	var resp struct {
		List []struct {
			SongID    int64  `json:"songId"`
			SongName  string `json:"songName"`
			Singer    string `json:"singer"`
			TypeEname string `json:"typeEname"` // 关键字段：歌曲类型 (yc, fc 等)
		} `json:"list"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("fivesing json parse error: %w", err)
	}

	// 4. 转换模型
	var songs []model.Song
	for _, item := range resp.List {
		// 构造复合 ID: SongID|TypeEname
		// 因为下载接口同时需要这两个参数
		compoundID := fmt.Sprintf("%d|%s", item.SongID, item.TypeEname)

		// 解码HTML实体并移除<em>标签（包括带class属性的）
		name := html.UnescapeString(item.SongName)
		name = removeEmTags(name)
		artist := html.UnescapeString(item.Singer)
		artist = removeEmTags(artist)
		
		songs = append(songs, model.Song{
			Source: "fivesing",
			ID:     compoundID,
			Name:   name,
			Artist: artist,
			// Album 和 Duration 在搜索列表里没有，需要详情接口才有，为了速度这里暂留空
			Album:    "",
			Duration: 0,
		})
	}

	return songs, nil
}

// GetDownloadURL 获取下载链接
// 对应 Python: _search 方法循环内部的 getSongUrl 调用部分
func GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "fivesing" {
		return "", errors.New("source mismatch")
	}

	// 1. 解析复合 ID
	parts := strings.Split(s.ID, "|")
	if len(parts) != 2 {
		return "", errors.New("invalid fivesing id format")
	}
	songID := parts[0]
	songType := parts[1]

	// 2. 构造请求参数
	// Python: params = {'songid': str(search_result['songId']), 'songtype': search_result['typeEname']}
	params := url.Values{}
	params.Set("songid", songID)
	params.Set("songtype", songType)

	apiURL := "http://mobileapi.5sing.kugou.com/song/getSongUrl?" + params.Encode()

	// 3. 发送请求
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
	)
	if err != nil {
		return "", err
	}

	// 4. 解析 JSON
	// Python 代码检查了 ['sq', 'hq', 'lq'] 三种音质
	var resp struct {
		Code int `json:"code"`
		Data struct {
			SQUrl       string `json:"squrl"`
			SQUrlBackup string `json:"squrl_backup"`
			HQUrl       string `json:"hqurl"`
			HQUrlBackup string `json:"hqurl_backup"`
			LQUrl       string `json:"lqurl"`
			LQUrlBackup string `json:"lqurl_backup"`
			// 可以在这里补充获取 size, ext 等信息
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("json parse error: %w", err)
	}

	// Python: if str(download_result['code']) not in ('1000',): continue
	if resp.Code != 1000 {
		return "", errors.New("api returned error code")
	}

	// 5. 音质选择策略 (SQ > HQ > LQ)
	// Python: for quality in ['sq', 'hq', 'lq']: ...

	// 尝试 SQ
	if url := getFirstValid(resp.Data.SQUrl, resp.Data.SQUrlBackup); url != "" {
		return url, nil
	}
	// 尝试 HQ
	if url := getFirstValid(resp.Data.HQUrl, resp.Data.HQUrlBackup); url != "" {
		return url, nil
	}
	// 尝试 LQ
	if url := getFirstValid(resp.Data.LQUrl, resp.Data.LQUrlBackup); url != "" {
		return url, nil
	}

	return "", errors.New("no valid download url found")
}

// 辅助函数：返回第一个非空字符串
func getFirstValid(urls ...string) string {
	for _, u := range urls {
		if u != "" {
			return u
		}
	}
	return ""
}

// removeEmTags 移除所有<em>标签（包括带属性的）
func removeEmTags(s string) string {
	// 移除 <em class="keyword"> 和类似的变体
	s = strings.ReplaceAll(s, "<em class=\"keyword\">", "")
	s = strings.ReplaceAll(s, "<em class='keyword'>", "")
	s = strings.ReplaceAll(s, "<em class=keyword>", "")
	// 移除普通的 <em> 标签
	s = strings.ReplaceAll(s, "<em>", "")
	// 移除闭合标签
	s = strings.ReplaceAll(s, "</em>", "")
	return strings.TrimSpace(s)
}
