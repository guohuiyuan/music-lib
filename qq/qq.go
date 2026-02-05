package qq

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent       = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"
	SearchReferer   = "http://m.y.qq.com"
	DownloadReferer = "http://y.qq.com"
	LyricReferer    = "https://y.qq.com/portal/player.html"
)

type QQ struct {
	cookie string
}

func New(cookie string) *QQ { return &QQ{cookie: cookie} }

var defaultQQ = New("")

func Search(keyword string) ([]model.Song, error) { return defaultQQ.Search(keyword) }
func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultQQ.SearchPlaylist(keyword)
}
func GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := defaultQQ.fetchPlaylistDetail(id)
	return songs, err
}
func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultQQ.ParsePlaylist(link)
}
func GetDownloadURL(s *model.Song) (string, error) { return defaultQQ.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error)      { return defaultQQ.GetLyrics(s) }
func Parse(link string) (*model.Song, error)       { return defaultQQ.Parse(link) }

// GetRecommendedPlaylists 获取推荐歌单
func GetRecommendedPlaylists() ([]model.Playlist, error) { return defaultQQ.GetRecommendedPlaylists() }

// Search 搜索歌曲
func (q *QQ) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("w", keyword)
	params.Set("format", "json")
	params.Set("p", "1")
	params.Set("n", "10")
	apiURL := "http://c.y.qq.com/soso/fcgi-bin/search_for_qq_cp?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", SearchReferer),
		utils.WithHeader("Cookie", q.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Song struct {
				List []struct {
					SongID    int64  `json:"songid"`
					SongName  string `json:"songname"`
					SongMID   string `json:"songmid"`
					AlbumName string `json:"albumname"`
					AlbumMID  string `json:"albummid"`
					Interval  int    `json:"interval"`
					Size128   int64  `json:"size128"`
					Size320   int64  `json:"size320"`
					SizeFlac  int64  `json:"sizeflac"`
					Singer    []struct {
						Name string `json:"name"`
					} `json:"singer"`
					Pay struct {
						PayDownload   int `json:"paydownload"`
						PayPlay       int `json:"payplay"`
						PayTrackPrice int `json:"paytrackprice"`
					} `json:"pay"`
				} `json:"list"`
			} `json:"song"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq json parse error: %w", err)
	}

	var songs []model.Song
	for _, item := range resp.Data.Song.List {
		if item.Pay.PayPlay == 1 {
			continue
		}

		var artistNames []string
		for _, s := range item.Singer {
			artistNames = append(artistNames, s.Name)
		}

		var coverURL string
		if item.AlbumMID != "" {
			coverURL = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", item.AlbumMID)
		}

		fileSize := item.Size128
		bitrate := 128
		if item.SizeFlac > 0 {
			fileSize = item.SizeFlac
			if item.Interval > 0 {
				bitrate = int(fileSize * 8 / 1000 / int64(item.Interval))
			} else {
				bitrate = 800
			}
		} else if item.Size320 > 0 {
			fileSize = item.Size320
			bitrate = 320
		}

		songs = append(songs, model.Song{
			Source:   "qq",
			ID:       item.SongMID,
			Name:     item.SongName,
			Artist:   strings.Join(artistNames, "、"),
			Album:    item.AlbumName,
			Duration: item.Interval,
			Size:     fileSize,
			Bitrate:  bitrate,
			Cover:    coverURL,
			Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", item.SongMID),
			Extra: map[string]string{
				"songmid": item.SongMID,
			},
		})
	}
	return songs, nil
}

// SearchPlaylist 搜索歌单
func (q *QQ) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("query", keyword)
	params.Set("page_no", "0")
	params.Set("num_per_page", "20")
	params.Set("format", "json")
	params.Set("remoteplace", "txt.yqq.playlist")
	params.Set("flag_qc", "0")

	apiURL := "http://c.y.qq.com/soso/fcgi-bin/client_music_search_songlist?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"),
		utils.WithHeader("Referer", "https://y.qq.com/portal/search.html"),
		utils.WithHeader("Cookie", q.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			List []struct {
				DissID    string `json:"dissid"`
				DissName  string `json:"dissname"`
				ImgUrl    string `json:"imgurl"`
				SongCount int    `json:"song_count"`
				ListenNum int    `json:"listennum"`
				Creator   struct {
					Name string `json:"name"`
				} `json:"creator"`
			} `json:"list"`
		} `json:"data"`
		Message string `json:"message"`
	}

	sBody := string(body)
	if idx := strings.Index(sBody, "("); idx >= 0 {
		if idx2 := strings.LastIndex(sBody, ")"); idx2 >= 0 {
			body = []byte(sBody[idx+1 : idx2])
		}
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq playlist json parse error: %w", err)
	}

	var playlists []model.Playlist
	for _, item := range resp.Data.List {
		cover := item.ImgUrl
		if cover != "" {
			if strings.HasPrefix(cover, "http://") {
				cover = strings.Replace(cover, "http://", "https://", 1)
			}
		}

		playlists = append(playlists, model.Playlist{
			Source:      "qq",
			ID:          item.DissID,
			Name:        item.DissName,
			Cover:       cover,
			TrackCount:  item.SongCount,
			PlayCount:   item.ListenNum,
			Creator:     item.Creator.Name,
			Description: "",
			Link:        fmt.Sprintf("https://y.qq.com/n/ryqq/playlist/%s", item.DissID),
		})
	}

	if len(playlists) == 0 {
		return nil, errors.New("no playlists found")
	}

	return playlists, nil
}

// GetPlaylistSongs 获取歌单详情（仅返回歌曲列表）
func (q *QQ) GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := q.fetchPlaylistDetail(id)
	return songs, err
}

// ParsePlaylist 解析歌单链接并返回详情
func (q *QQ) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	// 链接格式如: https://y.qq.com/n/ryqq/playlist/8825279434
	re := regexp.MustCompile(`playlist/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, nil, errors.New("invalid qq playlist link")
	}
	dissid := matches[1]

	return q.fetchPlaylistDetail(dissid)
}

// GetRecommendedPlaylists 获取推荐歌单 (QQ音乐每日推荐/热门歌单)
func (q *QQ) GetRecommendedPlaylists() ([]model.Playlist, error) {
	// 构造 musicu.fcg 的请求体
	reqData := map[string]interface{}{
		"comm": map[string]interface{}{
			"ct": 24,
		},
		"recomPlaylist": map[string]interface{}{
			"method": "get_hot_recommend",
			"module": "playlist.HotRecommendServer",
			"param": map[string]interface{}{
				"async": 1,
				"cmd":   2,
			},
		},
	}

	jsonData, _ := json.Marshal(reqData)

	headers := []utils.RequestOption{
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36"),
		utils.WithHeader("Referer", "https://y.qq.com/"),
		utils.WithHeader("Content-Type", "application/json"),
		// 推荐接口通常不需要严格的 Cookie，但如果有则带上
		utils.WithHeader("Cookie", q.cookie),
	}

	body, err := utils.Post("https://u.y.qq.com/cgi-bin/musicu.fcg", bytes.NewReader(jsonData), headers...)
	if err != nil {
		return nil, err
	}

	// 响应结构
	var resp struct {
		Code          int `json:"code"`
		RecomPlaylist struct {
			Data struct {
				VHot []struct {
					ContentID int64  `json:"content_id"` // 歌单ID
					Title     string `json:"title"`      // 歌单名
					Cover     string `json:"cover"`      // 封面
					ListenNum int    `json:"listen_num"` // 播放量
					SongCnt   int    `json:"song_cnt"`   // 歌曲数量 (部分接口)
					SongCount int    `json:"song_count"` // 歌曲数量 (备用字段)
					Username  string `json:"username"`   // 创建者 (有时为空)
				} `json:"v_hot"`
			} `json:"data"`
		} `json:"recomPlaylist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq recommended playlist json parse error: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("qq api error code: %d", resp.Code)
	}

	var playlists []model.Playlist
	for _, item := range resp.RecomPlaylist.Data.VHot {
		cover := item.Cover
		if cover != "" && strings.HasPrefix(cover, "http://") {
			cover = strings.Replace(cover, "http://", "https://", 1)
		}

		playlistID := strconv.FormatInt(item.ContentID, 10)

		trackCount := item.SongCnt
		if trackCount == 0 {
			trackCount = item.SongCount
		}

		playlists = append(playlists, model.Playlist{
			Source:      "qq",
			ID:          playlistID,
			Name:        item.Title,
			Cover:       cover,
			PlayCount:   item.ListenNum,
			TrackCount:  trackCount,
			Creator:     item.Username,
			Description: "", // 列表页通常不返回描述
			Link:        fmt.Sprintf("https://y.qq.com/n/ryqq/playlist/%s", playlistID),
		})
	}

	if len(playlists) == 0 {
		return nil, errors.New("no recommended playlists found")
	}

	return playlists, nil
}

// fetchPlaylistDetail [内部复用] 获取歌单详情（元数据+歌曲）
func (q *QQ) fetchPlaylistDetail(id string) (*model.Playlist, []model.Song, error) {
	params := url.Values{}
	params.Set("type", "1")
	params.Set("json", "1")
	params.Set("utf8", "1")
	params.Set("onlysong", "0")
	params.Set("disstid", id)
	params.Set("format", "json")
	params.Set("g_tk", "5381")
	params.Set("loginUin", "0")
	params.Set("hostUin", "0")
	params.Set("inCharset", "utf8")
	params.Set("outCharset", "utf-8")
	params.Set("notice", "0")
	params.Set("platform", "yqq")
	params.Set("needNewCode", "0")

	apiURL := "http://c.y.qq.com/qzone/fcg-bin/fcg_ucc_getcdinfo_byids_cp.fcg?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		utils.WithHeader("Referer", "https://y.qq.com/"),
		utils.WithHeader("Cookie", q.cookie),
	)
	if err != nil {
		return nil, nil, err
	}

	// 处理 JSONP
	sBody := string(body)
	if idx := strings.Index(sBody, "("); idx >= 0 && strings.HasSuffix(strings.TrimSpace(sBody), ")") {
		sBody = sBody[idx+1 : len(sBody)-1]
		body = []byte(sBody)
	}

	var resp struct {
		Cdlist []struct {
			Dissname string `json:"dissname"`
			Logo     string `json:"logo"`
			Nickname string `json:"nickname"`
			Desc     string `json:"desc"`
			Visitnum int    `json:"visitnum"`
			Songnum  int    `json:"songnum"`
			Songlist []struct {
				SongID    int64  `json:"songid"`
				SongName  string `json:"songname"`
				SongMID   string `json:"songmid"`
				AlbumName string `json:"albumname"`
				AlbumMID  string `json:"albummid"`
				Interval  int    `json:"interval"`
				Size128   int64  `json:"size128"`
				Size320   int64  `json:"size320"`
				SizeFlac  int64  `json:"sizeflac"`
				Pay       struct {
					PayPlay int `json:"payplay"`
				} `json:"pay"`
				Singer []struct {
					Name string `json:"name"`
				} `json:"singer"`
			} `json:"songlist"`
		} `json:"cdlist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("qq playlist detail json error: %w", err)
	}

	if len(resp.Cdlist) == 0 {
		return nil, nil, errors.New("playlist not found (empty cdlist)")
	}

	info := resp.Cdlist[0]

	// 构造 Playlist 元数据
	playlist := &model.Playlist{
		Source:      "qq",
		ID:          id,
		Name:        info.Dissname,
		Cover:       info.Logo,
		Creator:     info.Nickname,
		Description: info.Desc,
		PlayCount:   info.Visitnum,
		TrackCount:  info.Songnum,
		Link:        fmt.Sprintf("https://y.qq.com/n/ryqq/playlist/%s", id),
	}

	var songs []model.Song
	for _, item := range info.Songlist {

		var artistNames []string
		for _, s := range item.Singer {
			artistNames = append(artistNames, s.Name)
		}

		var coverURL string
		if item.AlbumMID != "" {
			coverURL = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", item.AlbumMID)
		}

		fileSize := item.Size128
		bitrate := 128
		if item.SizeFlac > 0 {
			fileSize = item.SizeFlac
			if item.Interval > 0 {
				bitrate = int(fileSize * 8 / 1000 / int64(item.Interval))
			} else {
				bitrate = 800
			}
		} else if item.Size320 > 0 {
			fileSize = item.Size320
			bitrate = 320
		}

		songs = append(songs, model.Song{
			Source:   "qq",
			ID:       item.SongMID,
			Name:     item.SongName,
			Artist:   strings.Join(artistNames, "、"),
			Album:    item.AlbumName,
			Duration: item.Interval,
			Size:     fileSize,
			Bitrate:  bitrate,
			Cover:    coverURL,
			Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", item.SongMID),
			Extra: map[string]string{
				"songmid": item.SongMID,
			},
		})
	}
	return playlist, songs, nil
}

// Parse 解析链接并获取完整信息
func (q *QQ) Parse(link string) (*model.Song, error) {
	re := regexp.MustCompile(`songDetail/(\w+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, errors.New("invalid qq music link")
	}
	songMID := matches[1]

	song, err := q.fetchSongDetail(songMID)
	if err != nil {
		return nil, err
	}

	downloadURL, err := q.GetDownloadURL(song)
	if err == nil {
		song.URL = downloadURL
	}

	return song, nil
}

// GetDownloadURL 获取下载链接
func (q *QQ) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "qq" {
		return "", errors.New("source mismatch")
	}

	songMID := s.ID
	if s.Extra != nil && s.Extra["songmid"] != "" {
		songMID = s.Extra["songmid"]
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	guid := fmt.Sprintf("%d", r.Int63n(9000000000)+1000000000)
	type Rate struct {
		Prefix string
		Ext    string
	}
	rates := []Rate{{"M500", "mp3"}, {"C400", "m4a"}}
	var lastErr string

	for _, rate := range rates {
		filename := fmt.Sprintf("%s%s%s.%s", rate.Prefix, songMID, songMID, rate.Ext)

		reqData := map[string]interface{}{
			"comm":  map[string]interface{}{"cv": 4747474, "ct": 24, "format": "json", "inCharset": "utf-8", "outCharset": "utf-8", "notice": 0, "platform": "yqq.json", "needNewCode": 1, "uin": 0, "g_tk_new_20200303": 5381, "g_tk": 5381},
			"req_1": map[string]interface{}{"module": "music.vkey.GetVkey", "method": "UrlGetVkey", "param": map[string]interface{}{"guid": guid, "songmid": []string{songMID}, "songtype": []int{0}, "uin": "0", "loginflag": 1, "platform": "20", "filename": []string{filename}}},
		}

		jsonData, _ := json.Marshal(reqData)
		headers := []utils.RequestOption{
			utils.WithHeader("User-Agent", UserAgent),
			utils.WithHeader("Referer", DownloadReferer),
			utils.WithHeader("Content-Type", "application/json"),
			utils.WithHeader("Cookie", q.cookie),
		}

		body, err := utils.Post("https://u.y.qq.com/cgi-bin/musicu.fcg", bytes.NewReader(jsonData), headers...)
		if err != nil {
			lastErr = err.Error()
			continue
		}

		var result struct {
			Req1 struct {
				Data struct {
					MidUrlInfo []struct {
						Purl    string `json:"purl"`
						WifiUrl string `json:"wifiurl"`
						Result  int    `json:"result"`
						ErrMsg  string `json:"errtype"`
					} `json:"midurlinfo"`
				} `json:"data"`
			} `json:"req_1"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = "json parse error"
			continue
		}
		if len(result.Req1.Data.MidUrlInfo) > 0 {
			info := result.Req1.Data.MidUrlInfo[0]
			if info.Purl == "" {
				lastErr = fmt.Sprintf("empty purl (result code: %d)", info.Result)
				continue
			}
			return "http://ws.stream.qqmusic.qq.com/" + info.Purl, nil
		}
	}
	return "", fmt.Errorf("download url not found: %s", lastErr)
}

// fetchSongDetail 内部方法：通过 songmid 获取详情
func (q *QQ) fetchSongDetail(songMID string) (*model.Song, error) {
	params := url.Values{}
	params.Set("songmid", songMID)
	params.Set("format", "json")

	apiURL := "https://c.y.qq.com/v8/fcg-bin/fcg_play_single_song.fcg?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", SearchReferer),
		utils.WithHeader("Cookie", q.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			ID    int64  `json:"id"`
			Name  string `json:"name"`
			Mid   string `json:"mid"`
			Album struct {
				Name string `json:"name"`
				Mid  string `json:"mid"`
			} `json:"album"`
			Singer []struct {
				Name string `json:"name"`
			} `json:"singer"`
			Interval int `json:"interval"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq detail json parse error: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, errors.New("song detail not found")
	}

	item := resp.Data[0]
	var artistNames []string
	for _, s := range item.Singer {
		artistNames = append(artistNames, s.Name)
	}

	var coverURL string
	if item.Album.Mid != "" {
		coverURL = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", item.Album.Mid)
	}

	return &model.Song{
		Source:   "qq",
		ID:       item.Mid,
		Name:     item.Name,
		Artist:   strings.Join(artistNames, "、"),
		Album:    item.Album.Name,
		Duration: item.Interval,
		Cover:    coverURL,
		Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", item.Mid),
		Extra: map[string]string{
			"songmid": item.Mid,
		},
	}, nil
}

// GetLyrics 获取歌词
func (q *QQ) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "qq" {
		return "", errors.New("source mismatch")
	}

	songMID := s.ID
	if s.Extra != nil && s.Extra["songmid"] != "" {
		songMID = s.Extra["songmid"]
	}

	params := url.Values{}
	params.Set("songmid", songMID)
	params.Set("loginUin", "0")
	params.Set("hostUin", "0")
	params.Set("format", "json")
	params.Set("inCharset", "utf8")
	params.Set("outCharset", "utf-8")
	params.Set("notice", "0")
	params.Set("platform", "yqq.json")
	params.Set("needNewCode", "0")

	apiURL := "https://c.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg?" + params.Encode()
	headers := []utils.RequestOption{
		utils.WithHeader("Referer", LyricReferer),
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", q.cookie),
	}

	body, err := utils.Get(apiURL, headers...)
	if err != nil {
		return "", err
	}

	var resp struct {
		Retcode int    `json:"retcode"`
		Lyric   string `json:"lyric"`
		Trans   string `json:"trans"`
	}
	sBody := string(body)
	if idx := strings.Index(sBody, "("); idx >= 0 {
		sBody = sBody[idx+1:]
		if idx2 := strings.LastIndex(sBody, ")"); idx2 >= 0 {
			sBody = sBody[:idx2]
		}
	}

	if err := json.Unmarshal([]byte(sBody), &resp); err != nil {
		return "", fmt.Errorf("qq lyric json parse error: %w", err)
	}
	if resp.Lyric == "" {
		return "", errors.New("lyric is empty or not found")
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(resp.Lyric)
	if err != nil {
		return "", fmt.Errorf("base64 decode error: %w", err)
	}

	return string(decodedBytes), nil
}
