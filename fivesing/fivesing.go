package fivesing

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
)

type Fivesing struct {
	cookie string
}

func New(cookie string) *Fivesing {
	return &Fivesing{cookie: cookie}
}

var defaultFivesing = New("")

func Search(keyword string) ([]model.Song, error) { return defaultFivesing.Search(keyword) }
func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultFivesing.SearchPlaylist(keyword)
}                                                      // [新增]
func GetPlaylistSongs(id string) ([]model.Song, error) { return defaultFivesing.GetPlaylistSongs(id) } // [新增]
func GetDownloadURL(s *model.Song) (string, error)     { return defaultFivesing.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error)          { return defaultFivesing.GetLyrics(s) }
func Parse(link string) (*model.Song, error)           { return defaultFivesing.Parse(link) }

// Search 搜索歌曲
func (f *Fivesing) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("sort", "1")
	params.Set("page", "1")
	params.Set("filter", "0")
	params.Set("type", "0")

	apiURL := "http://search.5sing.kugou.com/home/json?" + params.Encode()
	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", f.cookie))
	if err != nil {
		return nil, err
	}

	var resp struct {
		List []struct {
			SongID    int64  `json:"songId"`
			SongName  string `json:"songName"`
			Singer    string `json:"singer"`
			SongSize  int64  `json:"songSize"`
			TypeEname string `json:"typeEname"`
		} `json:"list"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("fivesing json parse error: %w", err)
	}

	var songs []model.Song
	for _, item := range resp.List {
		name := removeEmTags(html.UnescapeString(item.SongName))
		artist := removeEmTags(html.UnescapeString(item.Singer))

		duration := 0
		if item.SongSize > 0 {
			duration = int((item.SongSize * 8) / 320000)
		}

		songs = append(songs, model.Song{
			Source:   "fivesing",
			ID:       fmt.Sprintf("%d|%s", item.SongID, item.TypeEname),
			Name:     name,
			Artist:   artist,
			Duration: duration,
			Size:     item.SongSize,
			Link:     fmt.Sprintf("http://5sing.kugou.com/%s/%d.html", item.TypeEname, item.SongID),
			Extra: map[string]string{
				"songid":   strconv.FormatInt(item.SongID, 10),
				"songtype": item.TypeEname,
			},
		})
	}
	return songs, nil
}

// SearchPlaylist 搜索歌单
func (f *Fivesing) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("sort", "1")
	params.Set("page", "1")
	params.Set("filter", "0")
	params.Set("type", "1")

	apiURL := "http://search.5sing.kugou.com/home/json?" + params.Encode()
	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", f.cookie))
	if err != nil {
		return nil, err
	}

	var resp struct {
		List []struct {
			SongListId string `json:"songListId"`
			Title      string `json:"title"`
			Picture    string `json:"pictureUrl"`
			PlayCount  int    `json:"playCount"`
			UserName   string `json:"userName"`
			SongCnt    int    `json:"songCnt"`
			Content    string `json:"content"`
			UserId     string `json:"userId"`
		} `json:"list"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("fivesing playlist json parse error: %w", err)
	}

	var playlists []model.Playlist
	for _, item := range resp.List {
		title := removeEmTags(html.UnescapeString(item.Title))
		desc := removeEmTags(html.UnescapeString(item.Content))
		if desc == "0" {
			desc = ""
		}

		playlists = append(playlists, model.Playlist{
			Source:      "fivesing",
			ID:          item.SongListId,
			Name:        title,
			Cover:       item.Picture,
			TrackCount:  item.SongCnt,
			PlayCount:   item.PlayCount,
			Creator:     item.UserName,
			Description: desc,
			// [关键] 将 UserId 存入 Extra，供 GetPlaylistSongs 使用
			Extra: map[string]string{
				"user_id": item.UserId,
			},
		})
	}
	return playlists, nil
}

// GetPlaylistSongs 获取歌单详情 (HTML解析版 - 包含歌手提取)
func (f *Fivesing) GetPlaylistSongs(id string) ([]model.Song, error) {
	// 1. 获取 UserId (逻辑保持不变)
	infoURL := fmt.Sprintf("http://mobileapi.5sing.kugou.com/song/getsonglist?id=%s&songfields=ID,user", id)
	infoBody, err := utils.Get(infoURL, utils.WithHeader("User-Agent", UserAgent))
	if err != nil {
		return nil, fmt.Errorf("fetch info failed: %w", err)
	}

	var infoResp struct {
		Data struct {
			User struct {
				ID int64 `json:"ID"`
			} `json:"user"`
		} `json:"data"`
	}

	if err := json.Unmarshal(infoBody, &infoResp); err != nil {
		return nil, fmt.Errorf("fetch playlist info error: %w", err)
	}
	if infoResp.Data.User.ID == 0 {
		return nil, errors.New("playlist user not found or invalid id")
	}
	userId := strconv.FormatInt(infoResp.Data.User.ID, 10)

	// 2. 构造歌单页面 URL 并获取 HTML
	pageURL := fmt.Sprintf("http://5sing.kugou.com/%s/dj/%s.html", userId, id)
	htmlBodyBytes, err := utils.Get(pageURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", f.cookie),
	)
	if err != nil {
		return nil, err
	}
	htmlContent := string(htmlBodyBytes)

	// 3. 解析歌曲列表
	// 策略：先匹配每一个 <li> 块，再在块内分别提取信息，这样能确保歌名和歌手对应正确

	// A. 提取所有 <li class="p_rel">...</li> 块
	// [\s\S]*? 用于匹配包括换行符在内的所有字符 (非贪婪)
	blockRe := regexp.MustCompile(`<li class="p_rel">([\s\S]*?)</li>`)
	blocks := blockRe.FindAllStringSubmatch(htmlContent, -1)

	if len(blocks) == 0 {
		return nil, errors.New("no songs found in playlist html (structure mismatch)")
	}

	// B. 预编译块内提取正则
	// 提取歌曲: 匹配 href="/yc/123.html" 和 歌名
	songRe := regexp.MustCompile(`href="http://5sing\.kugou\.com/(yc|fc|bz)/(\d+)\.html"[^>]*>([^<]+)</a>`)
	// 提取歌手: 匹配 class="s_soner lt" 下的链接文本
	// 注意: 5sing HTML 中 class 名是 "s_soner" (拼写错误) 而不是 "s_singer"
	artistRe := regexp.MustCompile(`class="s_soner[^"]*".*?>([^<]+)</a>`)

	var songs []model.Song
	seen := make(map[string]bool)

	for _, match := range blocks {
		blockHTML := match[1]

		// 提取歌曲信息
		songMatch := songRe.FindStringSubmatch(blockHTML)
		if len(songMatch) < 4 {
			continue
		}
		kind := songMatch[1] // yc, fc, bz
		songID := songMatch[2]
		rawName := songMatch[3]

		// 提取歌手信息 (如果提取不到则默认为 Unknown)
		artist := "Unknown"
		artistMatch := artistRe.FindStringSubmatch(blockHTML)
		if len(artistMatch) >= 2 {
			artist = artistMatch[1]
		}

		// 去重
		uniqueKey := kind + "|" + songID
		if seen[uniqueKey] {
			continue
		}
		seen[uniqueKey] = true

		// 清理 HTML 转义字符 (如 &amp;) 和空白
		name := strings.TrimSpace(html.UnescapeString(rawName))
		artist = strings.TrimSpace(html.UnescapeString(artist))

		songs = append(songs, model.Song{
			Source: "fivesing",
			ID:     fmt.Sprintf("%s|%s", songID, kind),
			Name:   name,
			Artist: artist, // 现在可以正确获取到歌手名了
			Link:   fmt.Sprintf("http://5sing.kugou.com/%s/%s.html", kind, songID),
			Extra: map[string]string{
				"songid":   songID,
				"songtype": kind,
			},
		})
	}

	return songs, nil
}

// Parse 解析链接并获取完整信息
func (f *Fivesing) Parse(link string) (*model.Song, error) {
	// 1. 正则提取 Type 和 ID
	re := regexp.MustCompile(`5sing\.kugou\.com/(\w+)/(\d+)\.html`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 3 {
		return nil, errors.New("invalid 5sing link")
	}
	songType := matches[1]
	songID := matches[2]

	// 2. 调用内部方法获取详情（包含URL和Metadata）
	return f.fetchSongInfo(songID, songType)
}

// GetDownloadURL 获取下载链接
func (f *Fivesing) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "fivesing" {
		return "", errors.New("source mismatch")
	}
	if s.URL != "" {
		return s.URL, nil
	}

	var songID, songType string
	if s.Extra != nil {
		songID = s.Extra["songid"]
		songType = s.Extra["songtype"]
	}

	if songID == "" || songType == "" {
		parts := strings.Split(s.ID, "|")
		if len(parts) == 2 {
			songID = parts[0]
			songType = parts[1]
		} else {
			return "", errors.New("invalid id structure")
		}
	}

	// 复用 fetchSongInfo 获取 URL，或者仅调用 getAudioLink 节省开销
	// 为了高效，这里仅调用 audio 逻辑
	return f.fetchAudioLink(songID, songType)
}

// fetchSongInfo 获取完整的歌曲信息（Metadata + URL）
func (f *Fivesing) fetchSongInfo(songID, songType string) (*model.Song, error) {
	// A. 获取下载链接
	audioURL, err := f.fetchAudioLink(songID, songType)
	if err != nil {
		return nil, err
	}

	// B. 获取元数据 (使用 newget 接口，和 GetLyrics 类似，它包含标题信息)
	params := url.Values{}
	params.Set("songid", songID)
	params.Set("songtype", songType)
	metaURL := "http://mobileapi.5sing.kugou.com/song/newget?" + params.Encode()

	metaBody, _ := utils.Get(metaURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", f.cookie))

	var name, artist, cover string
	var duration int

	// 尝试解析元数据
	if metaBody != nil {
		var metaResp struct {
			Data struct {
				SongName string `json:"songName"`
				Singer   string `json:"singer"`
				Img      string `json:"img"` // 封面
			} `json:"data"`
		}
		if json.Unmarshal(metaBody, &metaResp) == nil {
			name = metaResp.Data.SongName
			artist = metaResp.Data.Singer
			cover = metaResp.Data.Img
		}
	}

	// 兜底 Name
	if name == "" {
		name = fmt.Sprintf("5sing_%s_%s", songType, songID)
	}

	return &model.Song{
		Source:   "fivesing",
		ID:       fmt.Sprintf("%s|%s", songID, songType),
		Name:     name,
		Artist:   artist,
		Cover:    cover,
		Duration: duration,
		URL:      audioURL, // 已填充
		Link:     fmt.Sprintf("http://5sing.kugou.com/%s/%s.html", songType, songID),
		Extra: map[string]string{
			"songid":   songID,
			"songtype": songType,
		},
	}, nil
}

// fetchAudioLink 仅获取音频链接
func (f *Fivesing) fetchAudioLink(songID, songType string) (string, error) {
	params := url.Values{}
	params.Set("songid", songID)
	params.Set("songtype", songType)

	apiURL := "http://mobileapi.5sing.kugou.com/song/getSongUrl?" + params.Encode()
	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", f.cookie))
	if err != nil {
		return "", err
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			SQUrl       string `json:"squrl"`
			SQUrlBackup string `json:"squrl_backup"`
			HQUrl       string `json:"hqurl"`
			HQUrlBackup string `json:"hqurl_backup"`
			LQUrl       string `json:"lqurl"`
			LQUrlBackup string `json:"lqurl_backup"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("json parse error: %w", err)
	}

	if resp.Code != 1000 {
		return "", errors.New("api returned error code")
	}

	if url := getFirstValid(resp.Data.SQUrl, resp.Data.SQUrlBackup); url != "" {
		return url, nil
	}
	if url := getFirstValid(resp.Data.HQUrl, resp.Data.HQUrlBackup); url != "" {
		return url, nil
	}
	if url := getFirstValid(resp.Data.LQUrl, resp.Data.LQUrlBackup); url != "" {
		return url, nil
	}

	return "", errors.New("no valid download url found")
}

func (f *Fivesing) GetLyrics(s *model.Song) (string, error) {
	// Added source check to satisfy TestLyricsSourceMismatch
	if s.Source != "fivesing" {
		return "", errors.New("source mismatch")
	}

	var songID, songType string
	if s.Extra != nil {
		songID = s.Extra["songid"]
		songType = s.Extra["songtype"]
	} else {
		parts := strings.Split(s.ID, "|")
		if len(parts) == 2 {
			songID = parts[0]
			songType = parts[1]
		}
	}

	if songID == "" {
		return "", errors.New("invalid id")
	}

	params := url.Values{}
	params.Set("songid", songID)
	params.Set("songtype", songType)
	apiURL := "http://mobileapi.5sing.kugou.com/song/newget?" + params.Encode()

	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", f.cookie))
	if err != nil {
		return "", err
	}

	var resp struct {
		Data struct {
			DynamicWords string `json:"dynamicWords"`
		} `json:"data"`
	}
	json.Unmarshal(body, &resp)
	if resp.Data.DynamicWords == "" {
		return "", errors.New("lyrics not found")
	}
	return resp.Data.DynamicWords, nil
}

func getFirstValid(urls ...string) string {
	for _, u := range urls {
		if u != "" {
			return u
		}
	}
	return ""
}

func removeEmTags(s string) string {
	s = strings.ReplaceAll(s, "<em class=\"keyword\">", "")
	s = strings.ReplaceAll(s, "</em>", "")
	return strings.TrimSpace(s)
}
