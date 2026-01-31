package migu

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent   = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"
	Referer     = "http://music.migu.cn/"
	MagicUserID = "15548614588710179085069"
)

type Migu struct {
	cookie string
}

func New(cookie string) *Migu { return &Migu{cookie: cookie} }

var defaultMigu = New("")

func Search(keyword string) ([]model.Song, error) { return defaultMigu.Search(keyword) }
func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultMigu.SearchPlaylist(keyword)
}                                                      // [新增]
func GetPlaylistSongs(id string) ([]model.Song, error) { return defaultMigu.GetPlaylistSongs(id) } // [新增]
func GetDownloadURL(s *model.Song) (string, error)     { return defaultMigu.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error)          { return defaultMigu.GetLyrics(s) }
func Parse(link string) (*model.Song, error)           { return defaultMigu.Parse(link) }

// Search 搜索歌曲
func (m *Migu) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("ua", "Android_migu")
	params.Set("version", "5.0.1")
	params.Set("text", keyword)
	params.Set("pageNo", "1")
	params.Set("pageSize", "10")
	params.Set("searchSwitch", `{"song":1,"album":0,"singer":0,"tagSong":0,"mvSong":0,"songlist":0,"bestShow":1}`)

	apiURL := "http://pd.musicapp.migu.cn/MIGUM2.0/v1.0/content/search_all.do?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", m.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		SongResultData struct {
			Result []MiguSongItem `json:"result"`
		} `json:"songResultData"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("migu json parse error: %w", err)
	}

	var songs []model.Song
	for _, item := range resp.SongResultData.Result {
		song := m.convertItemToSong(item)
		if song != nil {
			songs = append(songs, *song)
		}
	}
	return songs, nil
}

// SearchPlaylist 搜索歌单
func (m *Migu) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("ua", "Android_migu")
	params.Set("version", "5.0.1")
	params.Set("text", keyword)
	params.Set("pageNo", "1")
	params.Set("pageSize", "10")
	// 切换开关：songlist:1
	params.Set("searchSwitch", `{"song":0,"album":0,"singer":0,"tagSong":0,"mvSong":0,"songlist":1,"bestShow":1}`)

	apiURL := "http://pd.musicapp.migu.cn/MIGUM2.0/v1.0/content/search_all.do?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", m.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		SongListResultData struct {
			Result []struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				MusicNum string `json:"musicNum"`
				UserName string `json:"userName"`
				ImgItems []struct {
					Img string `json:"img"`
				} `json:"imgItems"`
			} `json:"result"`
		} `json:"songListResultData"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("migu playlist json parse error: %w", err)
	}

	var playlists []model.Playlist
	for _, item := range resp.SongListResultData.Result {
		trackCount, _ := strconv.Atoi(item.MusicNum)
		cover := ""
		if len(item.ImgItems) > 0 {
			cover = item.ImgItems[0].Img
		}

		playlists = append(playlists, model.Playlist{
			Source:     "migu",
			ID:         item.ID,
			Name:       item.Name,
			Cover:      cover,
			TrackCount: trackCount,
			Creator:    item.UserName,
		})
	}
	return playlists, nil
}

// GetPlaylistSongs 获取歌单详情（解析歌曲列表）
func (m *Migu) GetPlaylistSongs(id string) ([]model.Song, error) {
	// 参考 Search 和 GetDownloadURL 的参数构造方式
	params := url.Values{}
	params.Set("playListId", id)
	params.Set("resourceType", "2") // 2 代表歌单
	params.Set("pageNo", "1")
	params.Set("pageSize", "100") // 获取前100首

	// [修改] 使用与 Search/GetDownloadURL 相同的域名 c.musicapp.migu.cn
	// 接口路径为 /MIGUM2.0/v1.0/content/queryPlaylistById.do
	apiURL := "http://c.musicapp.migu.cn/MIGUM2.0/v1.0/content/queryPlaylistById.do?" + params.Encode()

	body, err := utils.Get(apiURL,
		// 使用文件顶部定义的常量 Header
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", m.cookie),
	)
	if err != nil {
		return nil, err
	}

	// 调试用：如果还报错，取消注释查看返回内容
	// os.WriteFile("migu_playlist_debug.json", body, 0644)

	// 定义响应结构 (针对 c.musicapp.migu.cn 的返回格式)
	var resp struct {
		Code string `json:"code"`
		Info string `json:"info"`
		// 注意：这里是 ContentList 数组
		ContentList []struct {
			SongList []struct {
				ContentId   string `json:"contentId"`   // 歌曲ID (下载用)
				SongId      string `json:"songId"`      // 歌曲ID (备用)
				SongName    string `json:"songName"`
				SingerName  string `json:"singerName"`
				AlbumName   string `json:"albumName"`
				PicM        string `json:"picM"`        // 封面
				PicL        string `json:"picL"`        // 大封面
				CopyrightId string `json:"copyrightId"` // 版权ID (网页链接用)
			} `json:"songList"`
		} `json:"contentList"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("migu playlist json parse error: %w", err)
	}

	// 检查返回码
	if resp.Code != "000000" {
		return nil, fmt.Errorf("migu api error: %s (code %s)", resp.Info, resp.Code)
	}

	if len(resp.ContentList) == 0 || len(resp.ContentList[0].SongList) == 0 {
		return nil, errors.New("playlist is empty or invalid")
	}

	var songs []model.Song
	for _, item := range resp.ContentList[0].SongList {
		// ID 选取逻辑：优先 contentId
		id := item.ContentId
		if id == "" {
			id = item.SongId
		}

		// 封面选取逻辑
		cover := item.PicL
		if cover == "" {
			cover = item.PicM
		}
		if cover != "" && !strings.HasPrefix(cover, "http") {
			cover = "http:" + cover
		}

		songs = append(songs, model.Song{
			Source: "migu",
			ID:     id,
			Name:   item.SongName,
			Artist: item.SingerName,
			Album:  item.AlbumName,
			Cover:  cover,
			// 构造网页链接
			Link: fmt.Sprintf("https://music.migu.cn/v3/music/song/%s", item.CopyrightId),
			Extra: map[string]string{
				"content_id":   id,
				"copyright_id": item.CopyrightId,
			},
		})
	}
	return songs, nil
}

// Parse 解析链接并获取完整信息
func (m *Migu) Parse(link string) (*model.Song, error) {
	// 1. 提取 ContentID
	// 支持格式: https://music.migu.cn/v3/music/song/60054701934
	re := regexp.MustCompile(`music\.migu\.cn/v3/music/song/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, errors.New("invalid migu link")
	}
	contentID := matches[1]

	// 2. 获取歌曲详情 (为了拿到 resourceType 和 formatType)
	song, err := m.fetchSongDetail(contentID)
	if err != nil {
		return nil, err
	}

	// 3. 获取下载链接
	// 因为 convertItemToSong 已经填充了 Extra，所以可以直接调用 GetDownloadURL
	downloadURL, err := m.GetDownloadURL(song)
	if err == nil {
		song.URL = downloadURL
	}

	return song, nil
}

// GetDownloadURL 获取下载链接
func (m *Migu) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "migu" {
		return "", errors.New("source mismatch")
	}
	if s.URL != "" {
		return s.URL, nil
	}

	var contentID, resourceType, formatType string
	if s.Extra != nil {
		contentID = s.Extra["content_id"]
		resourceType = s.Extra["resource_type"]
		formatType = s.Extra["format_type"]
	}

	if contentID == "" || resourceType == "" || formatType == "" {
		parts := strings.Split(s.ID, "|")
		if len(parts) == 3 {
			contentID = parts[0]
			resourceType = parts[1]
			formatType = parts[2]
		} else {
			return "", errors.New("invalid id structure and missing extra data")
		}
	}

	params := url.Values{}
	params.Set("toneFlag", formatType)
	params.Set("netType", "00")
	params.Set("userId", MagicUserID)
	params.Set("ua", "Android_migu")
	params.Set("version", "5.1")
	params.Set("copyrightId", "0")
	params.Set("contentId", contentID)
	params.Set("resourceType", resourceType)
	params.Set("channel", "0")

	apiURL := "http://app.pd.nf.migu.cn/MIGUM2.0/v1.0/content/sub/listenSong.do?" + params.Encode()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Referer", Referer)
	req.Header.Set("Cookie", m.cookie)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 302 {
		location := resp.Header.Get("Location")
		if location != "" {
			return location, nil
		}
	}

	return apiURL, nil
}

// 内部结构体定义，用于 Search 和 Parse 复用
type MiguSongItem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Singers []struct {
		Name string `json:"name"`
	} `json:"singers"`
	Albums []struct {
		Name string `json:"name"`
	} `json:"albums"`
	ContentID       string `json:"contentId"`
	ChargeAuditions string `json:"chargeAuditions"`
	ImgItems        []struct {
		Img string `json:"img"`
	} `json:"imgItems"`
	RateFormats []struct {
		FormatType      string   `json:"formatType"`
		ResourceType    string   `json:"resourceType"`
		Size            string   `json:"size"`
		AndroidSize     string   `json:"androidSize"`
		FileType        string   `json:"fileType"`
		AndroidFileType string   `json:"androidFileType"`
		Price           string   `json:"price"`
		ShowTag         []string `json:"showTag"`
	} `json:"rateFormats"`
}

// fetchSongDetail 通过 contentId 获取歌曲详情
func (m *Migu) fetchSongDetail(contentID string) (*model.Song, error) {
	params := url.Values{}
	params.Set("resourceType", "2")
	params.Set("contentId", contentID)

	// 使用 queryById 接口获取详情，结构与 Search 结果类似
	apiURL := "http://c.musicapp.migu.cn/MIGUM2.0/v1.0/content/queryById.do?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", m.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Item MiguSongItem `json:"resource"` // 注意：虽然通常是数组，但此接口有时直接返回对象或由外层包裹
		} `json:"data"`
		// 容错：有些接口返回结构略有不同，这里简化处理，假设返回的是标准结构
		Resource []MiguSongItem `json:"resource"`
	}

	// 尝试解析
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	var item MiguSongItem
	if len(resp.Resource) > 0 {
		item = resp.Resource[0]
	} else if resp.Data.Item.ContentID != "" {
		item = resp.Data.Item
	} else {
		return nil, errors.New("song detail not found")
	}

	song := m.convertItemToSong(item)
	if song == nil {
		return nil, errors.New("no valid format found for this song")
	}
	return song, nil
}

// convertItemToSong 将 API 返回的 Item 转换为 Song 模型 (复用 Search 中的逻辑)
func (m *Migu) convertItemToSong(item MiguSongItem) *model.Song {
	var artistNames []string
	for _, s := range item.Singers {
		artistNames = append(artistNames, s.Name)
	}

	albumName := ""
	if len(item.Albums) > 0 {
		albumName = item.Albums[0].Name
	}

	if len(item.RateFormats) == 0 {
		return nil
	}

	type validFormat struct {
		index int
		size  int64
		ext   string
	}
	var candidates []validFormat
	var duration int64 = 0
	var pqSize int64 = 0

	for i, fmtItem := range item.RateFormats {
		sizeStr := fmtItem.AndroidSize
		if sizeStr == "" || sizeStr == "0" {
			sizeStr = fmtItem.Size
		}
		sizeVal, _ := strconv.ParseInt(sizeStr, 10, 64)

		ext := fmtItem.AndroidFileType
		if ext == "" {
			ext = fmtItem.FileType
		}

		if fmtItem.FormatType == "PQ" {
			pqSize = sizeVal
		}

		if duration == 0 && sizeVal > 0 {
			var bitrate int64 = 0
			switch fmtItem.FormatType {
			case "PQ":
				bitrate = 128000
			case "HQ":
				bitrate = 320000
			case "LQ":
				bitrate = 64000
			}
			if bitrate > 0 {
				duration = (sizeVal * 8) / bitrate
			}
		}

		priceVal, _ := strconv.Atoi(fmtItem.Price)
		isVipTag := false
		for _, tag := range fmtItem.ShowTag {
			if tag == "vip" {
				isVipTag = true
				break
			}
		}
		isHiddenPaid := (item.ChargeAuditions == "1" && priceVal >= 200)

		if !isVipTag && !isHiddenPaid {
			candidates = append(candidates, validFormat{index: i, size: sizeVal, ext: ext})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].size > candidates[j].size })
	bestInfo := candidates[0]
	bestFormat := item.RateFormats[bestInfo.index]

	displaySize := bestInfo.size
	if pqSize > 0 {
		displaySize = pqSize
	}

	bitrate := 0
	if duration > 0 && bestInfo.size > 0 {
		bitrate = int(bestInfo.size * 8 / 1000 / duration)
	}

	var coverURL string
	if len(item.ImgItems) > 0 {
		coverURL = item.ImgItems[0].Img
	}

	return &model.Song{
		Source:   "migu",
		ID:       fmt.Sprintf("%s|%s|%s", item.ContentID, bestFormat.ResourceType, bestFormat.FormatType),
		Name:     item.Name,
		Artist:   strings.Join(artistNames, "、"),
		Album:    albumName,
		Size:     displaySize,
		Duration: int(duration),
		Bitrate:  bitrate,
		Cover:    coverURL,
		Ext:      bestInfo.ext,
		Link:     fmt.Sprintf("https://music.migu.cn/v3/music/song/%s", item.ContentID),
		Extra: map[string]string{
			"content_id":    item.ContentID,
			"resource_type": bestFormat.ResourceType,
			"format_type":   bestFormat.FormatType,
		},
	}
}

// GetLyrics 获取歌词
func (m *Migu) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "migu" {
		return "", errors.New("source mismatch")
	}

	contentID := ""
	if s.Extra != nil && s.Extra["content_id"] != "" {
		contentID = s.Extra["content_id"]
	} else {
		parts := strings.Split(s.ID, "|")
		if len(parts) >= 1 {
			contentID = parts[0]
		}
	}

	if contentID == "" {
		return "", errors.New("invalid migu song id")
	}

	params := url.Values{}
	params.Set("resourceId", contentID)
	params.Set("resourceType", "2")

	apiURL := "http://c.musicapp.migu.cn/MIGUM2.0/v1.0/content/resourceinfo.do?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", m.cookie),
	)
	if err != nil {
		return "", err
	}

	var resp struct {
		Resource []struct {
			LrcUrl   string `json:"lrcUrl"`
			LyricUrl string `json:"lyricUrl"`
		} `json:"resource"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("migu resource info parse error: %w", err)
	}

	if len(resp.Resource) == 0 {
		return "", errors.New("resource info not found")
	}

	lyricUrl := resp.Resource[0].LrcUrl
	if lyricUrl == "" {
		lyricUrl = resp.Resource[0].LyricUrl
	}

	if lyricUrl == "" {
		return "", errors.New("lyric url not found")
	}

	lyricUrl = strings.Replace(lyricUrl, "http://", "https://", 1)

	lrcBody, err := utils.Get(lyricUrl,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"),
		utils.WithHeader("Referer", "https://y.migu.cn/"),
		utils.WithHeader("Cookie", m.cookie),
	)
	if err != nil {
		return "", fmt.Errorf("download lyric failed: %w", err)
	}

	return string(lrcBody), nil
}
