package qq

import (
	"bytes"
	"encoding/base64"
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

// ... (常量和结构体保持不变)
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
func GetDownloadURL(s *model.Song) (string, error) { return defaultQQ.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error) { return defaultQQ.GetLyrics(s) }

// Search 搜索歌曲
func (q *QQ) Search(keyword string) ([]model.Song, error) {
	// ... (参数构造保持不变)
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
	if err != nil { return nil, err }

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
					Size128  int64 `json:"size128"`
					Size320  int64 `json:"size320"`
					SizeFlac int64 `json:"sizeflac"`
					Singer []struct { Name string `json:"name"` } `json:"singer"`
					Pay struct { PayDownload int `json:"paydownload"`; PayPlay int `json:"payplay"`; PayTrackPrice int `json:"paytrackprice"` } `json:"pay"`
				} `json:"list"`
			} `json:"song"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq json parse error: %w", err)
	}

	var songs []model.Song
	for _, item := range resp.Data.Song.List {
		if item.Pay.PayPlay == 1 { continue }

		var artistNames []string
		for _, s := range item.Singer { artistNames = append(artistNames, s.Name) }

		var coverURL string
		if item.AlbumMID != "" {
			coverURL = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", item.AlbumMID)
		}

		fileSize := item.Size128
		bitrate := 128
		if item.SizeFlac > 0 {
			fileSize = item.SizeFlac
			if item.Interval > 0 { bitrate = int(fileSize * 8 / 1000 / int64(item.Interval)) } else { bitrate = 800 }
		} else if item.Size320 > 0 {
			fileSize = item.Size320; bitrate = 320
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
			// 核心修改：存入 Extra
			Extra: map[string]string{
				"songmid": item.SongMID,
			},
		})
	}
	return songs, nil
}

// GetDownloadURL 获取下载链接
func (q *QQ) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "qq" { return "", errors.New("source mismatch") }

	// 核心修改：优先从 Extra 获取
	songMID := s.ID
	if s.Extra != nil && s.Extra["songmid"] != "" {
		songMID = s.Extra["songmid"]
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	guid := fmt.Sprintf("%d", r.Int63n(9000000000)+1000000000)
	type Rate struct { Prefix string; Ext string }
	rates := []Rate{ {"M500", "mp3"}, {"C400", "m4a"} }
	var lastErr string

	for _, rate := range rates {
		filename := fmt.Sprintf("%s%s%s.%s", rate.Prefix, songMID, songMID, rate.Ext) // 使用 songMID

		reqData := map[string]interface{}{
			"comm": map[string]interface{}{ "cv": 4747474, "ct": 24, "format": "json", "inCharset": "utf-8", "outCharset": "utf-8", "notice": 0, "platform": "yqq.json", "needNewCode": 1, "uin": 0, "g_tk_new_20200303": 5381, "g_tk": 5381 },
			"req_1": map[string]interface{}{ "module": "music.vkey.GetVkey", "method": "UrlGetVkey", "param": map[string]interface{}{ "guid": guid, "songmid": []string{songMID}, "songtype": []int{0}, "uin": "0", "loginflag": 1, "platform": "20", "filename": []string{filename} } },
		}

		jsonData, _ := json.Marshal(reqData)
		headers := []utils.RequestOption{
			utils.WithHeader("User-Agent", UserAgent),
			utils.WithHeader("Referer", DownloadReferer),
			utils.WithHeader("Content-Type", "application/json"),
			utils.WithHeader("Cookie", q.cookie),
		}

		body, err := utils.Post("https://u.y.qq.com/cgi-bin/musicu.fcg", bytes.NewReader(jsonData), headers...)
		if err != nil { lastErr = err.Error(); continue }

		var result struct {
			Req1 struct {
				Data struct {
					MidUrlInfo []struct { Purl string `json:"purl"`; WifiUrl string `json:"wifiurl"`; Result int `json:"result"`; ErrMsg string `json:"errtype"` } `json:"midurlinfo"`
				} `json:"data"`
			} `json:"req_1"`
		}

		if err := json.Unmarshal(body, &result); err != nil { lastErr = "json parse error"; continue }
		if len(result.Req1.Data.MidUrlInfo) > 0 {
			info := result.Req1.Data.MidUrlInfo[0]
			if info.Purl == "" { lastErr = fmt.Sprintf("empty purl (result code: %d)", info.Result); continue }
			return "http://ws.stream.qqmusic.qq.com/" + info.Purl, nil
		}
	}
	return "", fmt.Errorf("download url not found: %s", lastErr)
}

// GetLyrics 获取歌词
func (q *QQ) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "qq" { return "", errors.New("source mismatch") }

	// 核心修改：优先从 Extra 获取
	songMID := s.ID
	if s.Extra != nil && s.Extra["songmid"] != "" {
		songMID = s.Extra["songmid"]
	}

	params := url.Values{}
	params.Set("songmid", songMID) // 使用 songMID
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
	if err != nil { return "", err }

	var resp struct { Retcode int `json:"retcode"`; Lyric string `json:"lyric"`; Trans string `json:"trans"` }
	sBody := string(body)
	if idx := strings.Index(sBody, "("); idx >= 0 {
		sBody = sBody[idx+1:]
		if idx2 := strings.LastIndex(sBody, ")"); idx2 >= 0 { sBody = sBody[:idx2] }
	}

	if err := json.Unmarshal([]byte(sBody), &resp); err != nil { return "", fmt.Errorf("qq lyric json parse error: %w", err) }
	if resp.Lyric == "" { return "", errors.New("lyric is empty or not found") }

	decodedBytes, err := base64.StdEncoding.DecodeString(resp.Lyric)
	if err != nil { return "", fmt.Errorf("base64 decode error: %w", err) }

	return string(decodedBytes), nil
}