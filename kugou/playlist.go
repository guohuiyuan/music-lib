package kugou

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	kugouRecommendedPlaylistURL         = "https://m.kugou.com/plist/index?json=true"
	kugouRecommendedPlaylistFallbackURL = "http://mobilecdn.kugou.com/api/v3/tag/specialList"
)

func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultKugou.SearchPlaylist(keyword)
}

func GetPlaylistSongs(id string) ([]model.Song, error) {
	// 保持原接口兼容性，仅返回 Songs
	return defaultKugou.GetPlaylistSongs(id)
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
	apiURL := "http://mobilecdnbj.kugou.com/api/v3/tag/list?pid=0&apiver=2&plat=0"
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Status  int    `json:"status"`
		Errcode int    `json:"errcode"`
		Error   string `json:"error"`
		Data    struct {
			Info []struct {
				ID       int    `json:"id"`
				Name     string `json:"name"`
				Children []struct {
					ID           int    `json:"id"`
					Name         string `json:"name"`
					SpecialTagID int    `json:"special_tag_id"`
					IsHot        int    `json:"is_hot"`
				} `json:"children"`
			} `json:"info"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou playlist category json parse error: %w", err)
	}
	if resp.Status != 1 || resp.Errcode != 0 {
		return nil, fmt.Errorf("kugou playlist category api error: %s (status %d errcode %d)", resp.Error, resp.Status, resp.Errcode)
	}

	categories := []model.PlaylistCategory{{
		Source: "kugou",
		ID:     "",
		Name:   "全部",
		Group:  "全部",
	}}
	for _, group := range resp.Data.Info {
		groupName := strings.TrimSpace(group.Name)
		if group.ID > 0 && groupName != "" {
			categoryID := kugouPlaylistCategoryID(group.ID, 0)
			categories = append(categories, model.PlaylistCategory{
				Source: "kugou",
				ID:     categoryID,
				Name:   groupName,
				Group:  groupName,
				Extra: map[string]string{
					"id":     strconv.Itoa(group.ID),
					"tag_id": "0",
				},
			})
		}
		for _, child := range group.Children {
			name := strings.TrimSpace(child.Name)
			if child.ID == 0 || name == "" {
				continue
			}
			categoryID := kugouPlaylistCategoryID(child.ID, child.SpecialTagID)
			categories = append(categories, model.PlaylistCategory{
				Source: "kugou",
				ID:     categoryID,
				Name:   name,
				Group:  groupName,
				Hot:    child.IsHot == 1,
				Extra: map[string]string{
					"id":     strconv.Itoa(child.ID),
					"tag_id": strconv.Itoa(child.SpecialTagID),
				},
			})
		}
	}
	return categories, nil
}

func (k *Kugou) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	categoryID = strings.TrimSpace(categoryID)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	if categoryID == "" {
		categories, err := k.GetPlaylistCategories()
		if err != nil {
			return nil, err
		}
		for _, category := range categories {
			if strings.TrimSpace(category.ID) != "" {
				categoryID = category.ID
				break
			}
		}
		if categoryID == "" {
			return nil, errors.New("no playlist categories found")
		}
	}
	id, tagID := parseKugouPlaylistCategoryID(categoryID)
	if id == "" {
		return nil, errors.New("invalid kugou playlist category id")
	}

	params := url.Values{}
	params.Set("plat", "0")
	params.Set("page", strconv.Itoa(page))
	params.Set("tagid", tagID)
	params.Set("pagesize", strconv.Itoa(limit))
	params.Set("ugc", "1")
	params.Set("id", id)
	params.Set("sort", "2")
	apiURL := "http://mobilecdnbj.kugou.com/api/v3/tag/specialList?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Status  int    `json:"status"`
		Errcode int    `json:"errcode"`
		Error   string `json:"error"`
		Data    struct {
			Info []struct {
				SpecialID       int    `json:"specialid"`
				GlobalSpecialID string `json:"global_specialid"`
				SpecialName     string `json:"specialname"`
				ImgURL          string `json:"imgurl"`
				Intro           string `json:"intro"`
				PlayCount       int    `json:"playcount"`
				SongCount       int    `json:"songcount"`
				Username        string `json:"username"`
				SingerName      string `json:"singername"`
				PubTime         string `json:"publishtime"`
			} `json:"info"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou category playlist json parse error: %w", err)
	}
	if resp.Status != 1 || resp.Errcode != 0 {
		return nil, fmt.Errorf("kugou category playlist api error: %s (status %d errcode %d)", resp.Error, resp.Status, resp.Errcode)
	}

	playlists := make([]model.Playlist, 0, len(resp.Data.Info))
	for _, item := range resp.Data.Info {
		playlistID := ""
		if item.SpecialID > 0 {
			playlistID = strconv.Itoa(item.SpecialID)
		} else {
			playlistID = strings.TrimSpace(item.GlobalSpecialID)
		}
		name := strings.TrimSpace(item.SpecialName)
		if playlistID == "" || name == "" {
			continue
		}
		cover := strings.Replace(item.ImgURL, "{size}", "240", 1)
		creator := strings.TrimSpace(item.Username)
		if creator == "" {
			creator = strings.TrimSpace(item.SingerName)
		}
		playlists = append(playlists, model.Playlist{
			Source:      "kugou",
			ID:          playlistID,
			Name:        name,
			Cover:       cover,
			TrackCount:  item.SongCount,
			PlayCount:   item.PlayCount,
			Creator:     creator,
			Description: item.Intro,
			Link:        fmt.Sprintf("https://www.kugou.com/yy/special/single/%s.html", playlistID),
			Extra: map[string]string{
				"category_id":      categoryID,
				"id":               id,
				"tag_id":           tagID,
				"global_specialid": item.GlobalSpecialID,
				"publish_time":     item.PubTime,
			},
		})
	}
	if len(playlists) == 0 {
		return nil, errors.New("no category playlists found")
	}
	return playlists, nil
}

func kugouPlaylistCategoryID(id, tagID int) string {
	return strconv.Itoa(id) + ":" + strconv.Itoa(tagID)
}

func parseKugouPlaylistCategoryID(categoryID string) (string, string) {
	parts := strings.SplitN(categoryID, ":", 2)
	id := strings.TrimSpace(parts[0])
	tagID := "0"
	if len(parts) == 2 {
		tagID = strings.TrimSpace(parts[1])
	}
	if tagID == "" {
		tagID = "0"
	}
	return id, tagID
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
	if listID, ok := parseKugouCloudlistID(id); ok {
		_, songs, err := k.fetchCloudlistDetail(listID)
		return songs, err
	}
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
	return k.getRecommendedPlaylists(kugouRecommendedPlaylistURL, kugouRecommendedPlaylistFallbackURL)
}

func (k *Kugou) getRecommendedPlaylists(primaryURL, fallbackURL string) ([]model.Playlist, error) {
	playlists, primaryErr := k.fetchHomepageRecommendedPlaylists(primaryURL)
	if len(playlists) > 0 {
		return playlists, nil
	}
	if primaryErr == nil {
		primaryErr = errors.New("empty recommended playlist response")
	}

	playlists, fallbackErr := k.fetchTagRecommendedPlaylists(fallbackURL)
	if len(playlists) > 0 {
		return playlists, nil
	}
	if fallbackErr == nil {
		fallbackErr = errors.New("empty recommended playlist fallback response")
	}
	return nil, fmt.Errorf("kugou recommended playlists unavailable: primary: %v; fallback: %w", primaryErr, fallbackErr)
}

type kugouRecommendedPlaylistItem struct {
	SpecialID       int    `json:"specialid"`
	GlobalSpecialID string `json:"global_specialid"`
	SpecialName     string `json:"specialname"`
	ImgURL          string `json:"imgurl"`
	PlayCount       int    `json:"playcount"`
	SongCount       int    `json:"songcount"`
	Username        string `json:"username"`
	SingerName      string `json:"singername"`
	Intro           string `json:"intro"`
	PubTime         string `json:"publishtime"`
}

func (k *Kugou) fetchHomepageRecommendedPlaylists(apiURL string) ([]model.Playlist, error) {
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Plist struct {
			Info []kugouRecommendedPlaylistItem `json:"info"`
			List struct {
				Info []kugouRecommendedPlaylistItem `json:"info"`
			} `json:"list"`
		} `json:"plist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou recommended playlist json parse error: %w", err)
	}

	items := resp.Plist.List.Info
	if len(items) == 0 {
		items = resp.Plist.Info
	}
	return buildKugouRecommendedPlaylists(items), nil
}

func (k *Kugou) fetchTagRecommendedPlaylists(apiURL string) ([]model.Playlist, error) {
	parsedURL, err := url.Parse(apiURL)
	if err != nil {
		return nil, fmt.Errorf("invalid kugou recommended playlist fallback url: %w", err)
	}
	params := parsedURL.Query()
	params.Set("plat", "0")
	params.Set("page", "1")
	params.Set("tagid", "0")
	params.Set("pagesize", "30")
	params.Set("ugc", "1")
	params.Set("id", "0")
	params.Set("sort", "2")
	parsedURL.RawQuery = params.Encode()

	body, err := utils.Get(parsedURL.String(),
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Status  int    `json:"status"`
		Errcode int    `json:"errcode"`
		Error   string `json:"error"`
		Data    struct {
			Info []kugouRecommendedPlaylistItem `json:"info"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou recommended playlist fallback json parse error: %w", err)
	}
	if resp.Error != "" && len(resp.Data.Info) == 0 {
		return nil, fmt.Errorf("kugou recommended playlist fallback api error: %s (status %d errcode %d)", resp.Error, resp.Status, resp.Errcode)
	}
	return buildKugouRecommendedPlaylists(resp.Data.Info), nil
}

func buildKugouRecommendedPlaylists(items []kugouRecommendedPlaylistItem) []model.Playlist {
	playlists := make([]model.Playlist, 0, len(items))
	for _, item := range items {
		playlistID := strconv.Itoa(item.SpecialID)
		if item.SpecialID <= 0 {
			playlistID = strings.TrimSpace(item.GlobalSpecialID)
		}
		name := strings.TrimSpace(item.SpecialName)
		if playlistID == "" || name == "" {
			continue
		}
		cover := strings.Replace(item.ImgURL, "{size}", "240", 1)
		creator := strings.TrimSpace(item.Username)
		if creator == "" {
			creator = strings.TrimSpace(item.SingerName)
		}

		playlists = append(playlists, model.Playlist{
			Source:      "kugou",
			ID:          playlistID,
			Name:        name,
			Cover:       cover,
			TrackCount:  item.SongCount,
			PlayCount:   item.PlayCount,
			Creator:     creator,
			Description: item.Intro,
			Link:        fmt.Sprintf("https://www.kugou.com/yy/special/single/%s.html", playlistID),
			Extra: map[string]string{
				"global_specialid": item.GlobalSpecialID,
				"publish_time":     item.PubTime,
			},
		})
	}
	return playlists
}
