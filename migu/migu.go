package migu

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

// ... (常量和结构体保持不变)
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
func GetDownloadURL(s *model.Song) (string, error) { return defaultMigu.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error) { return defaultMigu.GetLyrics(s) }

// Search 搜索歌曲
func (m *Migu) Search(keyword string) ([]model.Song, error) {
	// ... (参数构造保持不变)
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
	if err != nil { return nil, err }

	var resp struct {
		SongResultData struct {
			Result []struct {
				ID              string `json:"id"`
				Name            string `json:"name"`
				Singers         []struct { Name string `json:"name"` } `json:"singers"`
				Albums          []struct { Name string `json:"name"` } `json:"albums"`
				ContentID       string `json:"contentId"`
				ChargeAuditions string `json:"chargeAuditions"`
				ImgItems        []struct { Img string `json:"img"` } `json:"imgItems"`
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
			} `json:"result"`
		} `json:"songResultData"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("migu json parse error: %w", err)
	}

	var songs []model.Song
	for _, item := range resp.SongResultData.Result {
		var artistNames []string
		for _, s := range item.Singers { artistNames = append(artistNames, s.Name) }
		
		albumName := ""
		if len(item.Albums) > 0 { albumName = item.Albums[0].Name }

		if len(item.RateFormats) == 0 { continue }

		type validFormat struct { index int; size int64; ext string }
		var candidates []validFormat
		var duration int64 = 0
		var pqSize int64 = 0 

		for i, fmtItem := range item.RateFormats {
			sizeStr := fmtItem.AndroidSize
			if sizeStr == "" || sizeStr == "0" { sizeStr = fmtItem.Size }
			sizeVal, _ := strconv.ParseInt(sizeStr, 10, 64)

			ext := fmtItem.AndroidFileType
			if ext == "" { ext = fmtItem.FileType }

			if fmtItem.FormatType == "PQ" { pqSize = sizeVal }

			if duration == 0 && sizeVal > 0 {
				var bitrate int64 = 0
				switch fmtItem.FormatType {
				case "PQ": bitrate = 128000
				case "HQ": bitrate = 320000
				case "LQ": bitrate = 64000
				}
				if bitrate > 0 { duration = (sizeVal * 8) / bitrate }
			}

			priceVal, _ := strconv.Atoi(fmtItem.Price)
			isVipTag := false
			for _, tag := range fmtItem.ShowTag { if tag == "vip" { isVipTag = true; break } }
			isHiddenPaid := (item.ChargeAuditions == "1" && priceVal >= 200)

			if !isVipTag && !isHiddenPaid {
				candidates = append(candidates, validFormat{index: i, size:  sizeVal, ext:   ext})
			}
		}

		if len(candidates) == 0 { continue }

		sort.Slice(candidates, func(i, j int) bool { return candidates[i].size > candidates[j].size })
		bestInfo := candidates[0]
		bestFormat := item.RateFormats[bestInfo.index]

		displaySize := bestInfo.size
		if pqSize > 0 { displaySize = pqSize }

		bitrate := 0
		if duration > 0 && bestInfo.size > 0 {
			bitrate = int(bestInfo.size * 8 / 1000 / duration)
		}

		compoundID := fmt.Sprintf("%s|%s|%s", item.ContentID, bestFormat.ResourceType, bestFormat.FormatType)
		
		var coverURL string
		if len(item.ImgItems) > 0 { coverURL = item.ImgItems[0].Img }

		songs = append(songs, model.Song{
			Source:   "migu",
			ID:       compoundID,
			Name:     item.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    albumName,
			Size:     displaySize,
			Duration: int(duration),
			Bitrate:  bitrate,
			Cover:    coverURL,
			Ext:      bestInfo.ext, 
			// 核心修改：存入 Extra
			Extra: map[string]string{
				"content_id":    item.ContentID,
				"resource_type": bestFormat.ResourceType,
				"format_type":   bestFormat.FormatType,
			},
		})
	}

	return songs, nil
}

// GetDownloadURL 获取下载链接
func (m *Migu) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "migu" { return "", errors.New("source mismatch") }

	// 核心修改：优先从 Extra 获取
	var contentID, resourceType, formatType string
	if s.Extra != nil {
		contentID = s.Extra["content_id"]
		resourceType = s.Extra["resource_type"]
		formatType = s.Extra["format_type"]
	}

	// 兼容旧数据
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
	if err != nil { return "", err }
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Referer", Referer)
	req.Header.Set("Cookie", m.cookie)

	resp, err := client.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()

	if resp.StatusCode == 302 {
		location := resp.Header.Get("Location")
		if location != "" { return location, nil }
	}

	return apiURL, nil
}

// GetLyrics 获取歌词
func (m *Migu) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "migu" { return "", errors.New("source mismatch") }

	// 核心修改：优先从 Extra 获取 ContentID
	contentID := ""
	if s.Extra != nil && s.Extra["content_id"] != "" {
		contentID = s.Extra["content_id"]
	} else {
		parts := strings.Split(s.ID, "|")
		if len(parts) >= 1 { contentID = parts[0] }
	}

	if contentID == "" { return "", errors.New("invalid migu song id") }

	params := url.Values{}
	params.Set("resourceId", contentID)
	params.Set("resourceType", "2") 

	apiURL := "http://c.musicapp.migu.cn/MIGUM2.0/v1.0/content/resourceinfo.do?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Cookie", m.cookie),
	)
	if err != nil { return "", err }

	var resp struct {
		Resource []struct {
			LrcUrl   string `json:"lrcUrl"`
			LyricUrl string `json:"lyricUrl"`
		} `json:"resource"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("migu resource info parse error: %w", err)
	}

	if len(resp.Resource) == 0 { return "", errors.New("resource info not found") }

	lyricUrl := resp.Resource[0].LrcUrl
	if lyricUrl == "" { lyricUrl = resp.Resource[0].LyricUrl }

	if lyricUrl == "" { return "", errors.New("lyric url not found") }

	lyricUrl = strings.Replace(lyricUrl, "http://", "https://", 1)

	lrcBody, err := utils.Get(lyricUrl,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"),
		utils.WithHeader("Referer", "https://y.migu.cn/"),
		utils.WithHeader("Cookie", m.cookie),
	)
	if err != nil { return "", fmt.Errorf("download lyric failed: %w", err) }

	return string(lrcBody), nil
}