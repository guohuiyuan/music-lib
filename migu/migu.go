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
				ChargeAuditions string `json:"chargeAuditions"` // 0:免费 1:收费
				ImgItems        []struct {
					Img string `json:"img"`
				} `json:"imgItems"`
				
				RateFormats []struct {
					FormatType      string   `json:"formatType"`
					ResourceType    string   `json:"resourceType"`
					Size            string   `json:"size"`            // 通用大小
					AndroidSize     string   `json:"androidSize"`     // [重点] 安卓端实际大小
					FileType        string   `json:"fileType"`
					AndroidFileType string   `json:"androidFileType"` // [重点] 安卓端文件后缀
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

		// 定义候选结构
		type validFormat struct {
			index int
			size  int64
			ext   string
		}
		var candidates []validFormat
		var duration int64 = 0

		for i, fmtItem := range item.RateFormats {
			// --- [修改点] 优先使用 AndroidSize ---
			// 这里的逻辑是：如果 androidSize 存在且不为0，就用它；否则回退到通用 size
			sizeStr := fmtItem.AndroidSize
			if sizeStr == "" || sizeStr == "0" {
				sizeStr = fmtItem.Size
			}
			sizeVal, _ := strconv.ParseInt(sizeStr, 10, 64)

			// --- [修改点] 优先使用 AndroidFileType ---
			ext := fmtItem.AndroidFileType
			if ext == "" {
				ext = fmtItem.FileType
			}

			// 计算时长 (PQ 码率最准，但这只是估算，显示用)
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

			// --- 过滤逻辑 (保持之前的优化) ---
			priceVal, _ := strconv.Atoi(fmtItem.Price)
			
			// 1. VIP 标签过滤
			isVipTag := false
			for _, tag := range fmtItem.ShowTag {
				if tag == "vip" {
					isVipTag = true
					break
				}
			}

			// 2. 隐形收费过滤 (针对周杰伦等歌曲: 试听收费且价格>=200)
			isHiddenPaid := (item.ChargeAuditions == "1" && priceVal >= 200)

			if !isVipTag && !isHiddenPaid {
				candidates = append(candidates, validFormat{
					index: i, 
					size:  sizeVal, // 这里存入的就是 androidSize
					ext:   ext,
				})
			}
		}

		if len(candidates) == 0 {
			continue 
		}

		// 选择最佳音质 (Size 最大)
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].size > candidates[j].size
		})
		
		bestInfo := candidates[0]
		bestFormat := item.RateFormats[bestInfo.index]

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
			Size:     bestInfo.size,   // [确认] 这里使用的是优先取到的 AndroidSize
			Duration: int(duration),
			Cover:    coverURL,
			Ext:      bestInfo.ext,    // 记录正确后缀
		})
	}

	return songs, nil
}

// GetDownloadURL 保持带重定向解析的逻辑
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