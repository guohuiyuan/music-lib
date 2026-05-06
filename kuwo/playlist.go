package kuwo

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
	return defaultKuwo.SearchPlaylist(keyword)
}

func GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := defaultKuwo.fetchPlaylistDetail(id)
	return songs, err
}

func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultKuwo.ParsePlaylist(link)
}

// GetRecommendedPlaylists 获取推荐歌单 (新增)
func GetRecommendedPlaylists() ([]model.Playlist, error) {
	return defaultKuwo.GetRecommendedPlaylists()
}

func GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return defaultKuwo.GetPlaylistCategories()
}

func GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return defaultKuwo.GetCategoryPlaylists(categoryID, page, limit)
}

func (k *Kuwo) GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return nil, model.ErrPlaylistCategoriesUnsupported
}

func (k *Kuwo) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return nil, model.ErrPlaylistCategoriesUnsupported
}

func (k *Kuwo) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	var resp struct {
		AbsList []struct {
			PlaylistID string `json:"playlistid"`
			Name       string `json:"name"`
			Pic        string `json:"pic"`
			SongNum    string `json:"songnum"`
			Intro      string `json:"intro"`
			NickName   string `json:"nickname"`
		} `json:"abslist"`
	}

	if err := k.searchCollection(keyword, "playlist", &resp); err != nil {
		return nil, err
	}

	var playlists []model.Playlist
	for _, item := range resp.AbsList {
		count, _ := strconv.Atoi(item.SongNum)
		cover := item.Pic
		if cover != "" {
			cover = strings.Replace(cover, "_150.", "_700.", 1)
			if !strings.HasPrefix(cover, "http") {
				cover = "http://" + cover
			}
		}

		playlists = append(playlists, model.Playlist{
			Source:      "kuwo",
			ID:          item.PlaylistID,
			Name:        item.Name,
			Cover:       cover,
			TrackCount:  count,
			Creator:     item.NickName,
			Description: item.Intro,
			// [修改] 填充 Link
			Link: fmt.Sprintf("http://www.kuwo.cn/playlist_detail/%s", item.PlaylistID),
		})
	}
	return playlists, nil
}

func (k *Kuwo) GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := k.fetchPlaylistDetail(id)
	return songs, err
}

// ParsePlaylist 解析歌单链接
func (k *Kuwo) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	// 链接格式: http://www.kuwo.cn/playlist_detail/1082685103
	re := regexp.MustCompile(`playlist_detail/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, nil, errors.New("invalid kuwo playlist link")
	}
	playlistID := matches[1]

	return k.fetchPlaylistDetail(playlistID)
}

// GetRecommendedPlaylists 获取推荐歌单 (酷我热门歌单)
func (k *Kuwo) GetRecommendedPlaylists() ([]model.Playlist, error) {
	// 使用 wapi 接口获取热门推荐歌单，不需要复杂 Token
	params := url.Values{}
	params.Set("pn", "0")
	params.Set("rn", "30")
	params.Set("order", "hot")

	apiURL := "http://wapi.kuwo.cn/api/pc/classify/playlist/getRcmPlayList?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Data []struct {
				ID        string      `json:"id"`
				Name      string      `json:"name"`
				Img       string      `json:"img"`
				ListenCnt interface{} `json:"listencnt"` // 可能是 string 或 int
				SongNum   interface{} `json:"songnum"`   // 歌曲数量 (部分接口)
				Total     interface{} `json:"total"`     // 歌曲数量 (备用字段)
				Count     interface{} `json:"count"`     // 歌曲数量 (备用字段)
				MusicNum  interface{} `json:"musicnum"`  // 歌曲数量 (备用字段)
				UserName  string      `json:"uname"`
				Desc      string      `json:"desc"`
			} `json:"data"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kuwo recommend json parse error: %w", err)
	}

	if resp.Code != 200 {
		return nil, fmt.Errorf("kuwo api error code: %d", resp.Code)
	}

	var playlists []model.Playlist
	for _, item := range resp.Data.Data {
		cover := item.Img
		if cover != "" && !strings.HasPrefix(cover, "http") {
			cover = "http://" + cover
		}

		parseAnyInt := func(val interface{}) int {
			switch v := val.(type) {
			case float64:
				return int(v)
			case string:
				if v != "" {
					if n, err := strconv.Atoi(v); err == nil {
						return n
					}
				}
			}
			return 0
		}

		// 处理 ListenCnt 多态类型
		playCount := parseAnyInt(item.ListenCnt)
		trackCount := parseAnyInt(item.SongNum)
		if trackCount == 0 {
			trackCount = parseAnyInt(item.Total)
		}
		if trackCount == 0 {
			trackCount = parseAnyInt(item.Count)
		}
		if trackCount == 0 {
			trackCount = parseAnyInt(item.MusicNum)
		}

		playlists = append(playlists, model.Playlist{
			Source:      "kuwo",
			ID:          item.ID,
			Name:        item.Name,
			Cover:       cover,
			PlayCount:   playCount,
			TrackCount:  trackCount,
			Creator:     item.UserName,
			Description: item.Desc,
			Link:        fmt.Sprintf("http://www.kuwo.cn/playlist_detail/%s", item.ID),
		})
	}

	if len(playlists) == 0 {
		return nil, errors.New("no recommended playlists found")
	}

	return playlists, nil
}
