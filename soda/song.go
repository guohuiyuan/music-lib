package soda

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"regexp"
	"strings"
)

func Search(keyword string) ([]model.Song, error) { return defaultSoda.Search(keyword) }

func Parse(link string) (*model.Song, error) { return defaultSoda.Parse(link) }

// Search 搜索歌曲 (PC API)
func (s *Soda) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("q", keyword)
	params.Set("cursor", "0")
	params.Set("search_method", "input")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	apiURL := "https://api.qishui.com/luna/pc/search/track?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		ResultGroups []struct {
			Data []struct {
				Entity struct {
					Track struct {
						ID       string `json:"id"`
						Name     string `json:"name"`
						Duration int    `json:"duration"`
						Artists  []struct {
							Name string `json:"name"`
						} `json:"artists"`
						Album struct {
							Name     string `json:"name"`
							UrlCover struct {
								Urls []string `json:"urls"`
								Uri  string   `json:"uri"`
							} `json:"url_cover"`
						} `json:"album"`
						BitRates []struct {
							Size    int64  `json:"size"`
							Quality string `json:"quality"`
						} `json:"bit_rates"`
					} `json:"track"`
				} `json:"entity"`
			} `json:"data"`
		} `json:"result_groups"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda search json parse error: %w", err)
	}
	if len(resp.ResultGroups) == 0 {
		return nil, nil
	}

	var songs []model.Song
	for _, item := range resp.ResultGroups[0].Data {
		track := item.Entity.Track
		if track.ID == "" {
			continue
		}

		// 计算最大文件大小
		var displaySize int64
		for _, br := range track.BitRates {
			if br.Size > displaySize {
				displaySize = br.Size
			}
		}

		var artistNames []string
		for _, ar := range track.Artists {
			artistNames = append(artistNames, ar.Name)
		}

		var cover string
		if len(track.Album.UrlCover.Urls) > 0 {
			domain := track.Album.UrlCover.Urls[0]
			uri := track.Album.UrlCover.Uri
			if domain != "" && uri != "" {
				cover = domain + uri + "~c5_375x375.jpg"
			}
		}

		bitrate := 0
		seconds := track.Duration / 1000
		if seconds > 0 && displaySize > 0 {
			bitrate = int(displaySize * 8 / 1000 / int64(seconds))
		}

		songs = append(songs, model.Song{
			Source:   "soda",
			ID:       track.ID,
			Name:     track.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    track.Album.Name,
			Duration: track.Duration / 1000,
			Size:     displaySize,
			Bitrate:  bitrate,
			Cover:    cover,
			Link:     fmt.Sprintf("https://www.qishui.com/track/%s", track.ID),
			Extra: map[string]string{
				"track_id": track.ID,
			},
		})
	}
	return songs, nil
}

// Parse 解析链接并获取完整信息
func (s *Soda) Parse(link string) (*model.Song, error) {
	re := regexp.MustCompile(`track/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, errors.New("invalid soda link")
	}
	trackID := matches[1]
	return s.fetchSongDetail(trackID)
}
