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

const (
	UserAgent   = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"
	Referer     = "http://music.migu.cn/"
	MagicUserID = "15548614588710179085069"
)

// Search 搜索歌曲
func Search(keyword string) ([]model.Song, error) {
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
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		SongResultData struct {
			Result []struct {
				ID              string `json:"id"`
				Name            string `json:"name"`
				Singers         []struct {
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
					AndroidSize     string   `json:"androidSize"` // 优先使用
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
		for _, s := range item.Singers {
			artistNames = append(artistNames, s.Name)
		}
		
		albumName := ""
		if len(item.Albums) > 0 {
			albumName = item.Albums[0].Name
		}

		if len(item.RateFormats) == 0 {
			continue
		}

		type validFormat struct {
			index int
			size  int64
			ext   string
		}
		var candidates []validFormat
		var duration int64 = 0
		var pqSize int64 = 0 // 记录标准音质大小

		for i, fmtItem := range item.RateFormats {
			// 解析大小：优先 AndroidSize
			sizeStr := fmtItem.AndroidSize
			if sizeStr == "" || sizeStr == "0" {
				sizeStr = fmtItem.Size
			}
			sizeVal, _ := strconv.ParseInt(sizeStr, 10, 64)

			// 解析后缀
			ext := fmtItem.AndroidFileType
			if ext == "" {
				ext = fmtItem.FileType
			}

			// 如果是标准音质(PQ)，记录其大小用于展示
			if fmtItem.FormatType == "PQ" {
				pqSize = sizeVal
			}

			// 估算时长 (优先用 PQ 码率 128k 估算)
			if duration == 0 && sizeVal > 0 {
				var bitrate int64 = 0
				switch fmtItem.FormatType {
				case "PQ": bitrate = 128000
				case "HQ": bitrate = 320000
				case "LQ": bitrate = 64000
				}
				if bitrate > 0 {
					duration = (sizeVal * 8) / bitrate
				}
			}

			// --- 过滤逻辑 ---
			priceVal, _ := strconv.Atoi(fmtItem.Price)
			isVipTag := false
			for _, tag := range fmtItem.ShowTag {
				if tag == "vip" {
					isVipTag = true
					break
				}
			}
			// 隐形收费过滤
			isHiddenPaid := (item.ChargeAuditions == "1" && priceVal >= 200)

			if !isVipTag && !isHiddenPaid {
				candidates = append(candidates, validFormat{
					index: i, 
					size:  sizeVal,
					ext:   ext,
				})
			}
		}

		if len(candidates) == 0 {
			continue 
		}

		// 2. 选择最佳音质 (用于下载)
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].size > candidates[j].size
		})
		
		bestInfo := candidates[0]
		bestFormat := item.RateFormats[bestInfo.index]

		// 3. 确定展示大小
		displaySize := bestInfo.size
		if pqSize > 0 {
			displaySize = pqSize
		}

		compoundID := fmt.Sprintf("%s|%s|%s", item.ContentID, bestFormat.ResourceType, bestFormat.FormatType)
		
		var coverURL string
		if len(item.ImgItems) > 0 {
			coverURL = item.ImgItems[0].Img
		}

		songs = append(songs, model.Song{
			Source:   "migu",
			ID:       compoundID,
			Name:     item.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    albumName,
			Size:     displaySize,
			Duration: int(duration),
			Cover:    coverURL,
			Ext:      bestInfo.ext, 
		})
	}

	return songs, nil
}

// GetDownloadURL 获取下载链接
func GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "migu" {
		return "", errors.New("source mismatch")
	}

	parts := strings.Split(s.ID, "|")
	if len(parts) != 3 {
		return "", errors.New("invalid migu song id format")
	}
	contentID := parts[0]
	resourceType := parts[1]
	formatType := parts[2]

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

	// 使用不自动跳转的 Client 获取 Location
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

// GetLyrics 获取歌词
// 模仿 Python 逻辑，通过 ContentID 获取详情后下载歌词
func GetLyrics(s *model.Song) (string, error) {
	if s.Source != "migu" {
		return "", errors.New("source mismatch")
	}

	parts := strings.Split(s.ID, "|")
	if len(parts) < 1 {
		return "", errors.New("invalid migu song id")
	}
	contentID := parts[0]

	// 1. 获取歌曲详情以拿到歌词 URL
	// API: http://c.musicapp.migu.cn/MIGUM2.0/v1.0/content/resourceinfo.do
	params := url.Values{}
	params.Set("resourceId", contentID)
	params.Set("resourceType", "2") // 2 代表歌曲

	apiURL := "http://c.musicapp.migu.cn/MIGUM2.0/v1.0/content/resourceinfo.do?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
	)
	if err != nil {
		return "", err
	}

	var resp struct {
		Resource []struct {
			LyricUrl string `json:"lyricUrl"`
		} `json:"resource"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("migu resource info parse error: %w", err)
	}

	if len(resp.Resource) == 0 || resp.Resource[0].LyricUrl == "" {
		return "", errors.New("lyric url not found")
	}

	lyricUrl := resp.Resource[0].LyricUrl

	// 2. 下载歌词
	// Python: lyric_url = ... .replace('http://', 'https://')
	// Python Headers: Referer: https://y.migu.cn/
	lyricUrl = strings.Replace(lyricUrl, "http://", "https://", 1)

	// 使用 Python 代码中针对歌词下载的 Header，确保成功率
	lrcBody, err := utils.Get(lyricUrl,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"),
		utils.WithHeader("Referer", "https://y.migu.cn/"),
	)
	if err != nil {
		return "", fmt.Errorf("download lyric failed: %w", err)
	}

	return string(lrcBody), nil
}