package kugou

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultKugou.SearchPlaylist(keyword)
}

func GetPlaylistSongs(id string) ([]model.Song, error) {
	// 保持原接口兼容性，仅返回 Songs
	_, songs, err := defaultKugou.fetchPlaylistDetail(id)
	return songs, err
}

func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultKugou.ParsePlaylist(link)
}

// GetRecommendedPlaylists 获取推荐歌单
func GetRecommendedPlaylists() ([]model.Playlist, error) {
	return defaultKugou.GetRecommendedPlaylists()
}

func GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return defaultKugou.GetPlaylistCategories()
}

func GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return defaultKugou.GetCategoryPlaylists(categoryID, page, limit)
}

func (k *Kugou) GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return nil, model.ErrPlaylistCategoriesUnsupported
}

func (k *Kugou) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return nil, model.ErrPlaylistCategoriesUnsupported
}

func (k *Kugou) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("keyword", keyword)
	params.Set("platform", "WebFilter")
	params.Set("format", "json")
	params.Set("page", "1")
	params.Set("pagesize", "10")
	params.Set("filter", "0")
	apiURL := "http://mobilecdn.kugou.com/api/v3/search/special?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Info []struct {
				SpecialID   int    `json:"specialid"`
				SpecialName string `json:"specialname"`
				Intro       string `json:"intro"`
				ImgURL      string `json:"imgurl"`
				SongCount   int    `json:"songcount"`
				PlayCount   int    `json:"playcount"`
				NickName    string `json:"nickname"`
				PubTime     string `json:"publishtime"`
			} `json:"info"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou playlist search json error: %w", err)
	}

	var playlists []model.Playlist
	for _, item := range resp.Data.Info {
		cover := strings.Replace(item.ImgURL, "{size}", "240", 1)
		playlists = append(playlists, model.Playlist{
			Source:      "kugou",
			ID:          strconv.Itoa(item.SpecialID),
			Name:        item.SpecialName,
			Cover:       cover,
			TrackCount:  item.SongCount,
			PlayCount:   item.PlayCount,
			Creator:     item.NickName,
			Description: item.Intro,
			Link:        fmt.Sprintf("https://www.kugou.com/yy/special/single/%d.html", item.SpecialID),
		})
	}
	return playlists, nil
}

func (k *Kugou) GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := k.fetchPlaylistDetail(id)
	return songs, err
}

// ParsePlaylist 解析歌单链接
func (k *Kugou) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	// 链接格式: https://www.kugou.com/yy/special/single/546903.html
	switch {
	case strings.Contains(link, "/yy/special/single/"):
		re := regexp.MustCompile(`special/single/(\d+)\.html`)
		matches := re.FindStringSubmatch(link)
		if len(matches) < 2 {
			return nil, nil, errors.New("invalid kugou playlist link")
		}
		return k.fetchPlaylistDetail(matches[1])
	case strings.Contains(link, "/songlist/"):
		re := regexp.MustCompile(`songlist/(gcid_[a-zA-Z0-9]+)`)
		matches := re.FindStringSubmatch(link)
		if len(matches) < 2 {
			return nil, nil, errors.New("invalid kugou songlist link")
		}
		return k.fetchPlaylistDetail(matches[1])
	default:
		return nil, nil, errors.New("invalid kugou playlist link")
	}
}

// GetRecommendedPlaylists 获取推荐歌单
func (k *Kugou) GetRecommendedPlaylists() ([]model.Playlist, error) {
	// [修改] 使用 m.kugou.com 的 plist 接口，这个接口对 MobileUserAgent 更友好
	// json=true 返回 JSON 数据
	apiURL := "http://m.kugou.com/plist/index&json=true"

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	// 检查 Body 是否是 JSON 格式 (简单的开头检查)
	// 如果酷狗返回 HTML 错误页，这里可以拦截到
	if len(body) == 0 || body[0] != '{' {
		return nil, fmt.Errorf("kugou api returned invalid json: %s", string(body))
	}

	var resp struct {
		Plist struct {
			List struct {
				Info []struct {
					SpecialID   int    `json:"specialid"`
					SpecialName string `json:"specialname"`
					ImgURL      string `json:"imgurl"`
					PlayCount   int    `json:"playcount"`
					SongCount   int    `json:"songcount"`
					Username    string `json:"username"`
					Intro       string `json:"intro"`
				} `json:"info"`
			} `json:"list"`
		} `json:"plist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou recommended playlist json parse error: %w", err)
	}

	var playlists []model.Playlist
	for _, item := range resp.Plist.List.Info {
		cover := strings.Replace(item.ImgURL, "{size}", "240", 1)

		playlists = append(playlists, model.Playlist{
			Source:      "kugou",
			ID:          strconv.Itoa(item.SpecialID),
			Name:        item.SpecialName,
			Cover:       cover,
			TrackCount:  item.SongCount,
			PlayCount:   item.PlayCount,
			Creator:     item.Username,
			Description: item.Intro,
			Link:        fmt.Sprintf("https://www.kugou.com/yy/special/single/%d.html", item.SpecialID),
		})
	}

	if len(playlists) == 0 {
		return nil, errors.New("no recommended playlists found")
	}

	return playlists, nil
}
