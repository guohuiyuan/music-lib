package qq

import (
	"bytes"
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
	return defaultQQ.SearchPlaylist(keyword)
}

func GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := defaultQQ.fetchPlaylistDetail(id)
	return songs, err
}

func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultQQ.ParsePlaylist(link)
}

// GetRecommendedPlaylists returns recommended playlists.
func GetRecommendedPlaylists() ([]model.Playlist, error) { return defaultQQ.GetRecommendedPlaylists() }

func GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return defaultQQ.GetPlaylistCategories()
}

func GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return defaultQQ.GetCategoryPlaylists(categoryID, page, limit)
}

func (q *QQ) GetPlaylistCategories() ([]model.PlaylistCategory, error) {
	return nil, model.ErrPlaylistCategoriesUnsupported
}

func (q *QQ) GetCategoryPlaylists(categoryID string, page, limit int) ([]model.Playlist, error) {
	return nil, model.ErrPlaylistCategoriesUnsupported
}

// SearchPlaylist searches playlists.
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
		utils.WithRandomIPHeader(),
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

// GetPlaylistSongs returns songs in a playlist.
func (q *QQ) GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := q.fetchPlaylistDetail(id)
	return songs, err
}

// ParsePlaylist parses a playlist link.
func (q *QQ) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	// Example: https://y.qq.com/n/ryqq/playlist/8825279434
	re := regexp.MustCompile(`playlist/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, nil, errors.New("invalid qq playlist link")
	}
	dissid := matches[1]

	return q.fetchPlaylistDetail(dissid)
}

// GetRecommendedPlaylists returns QQ Music recommended playlists.
func (q *QQ) GetRecommendedPlaylists() ([]model.Playlist, error) {
	// Build the musicu.fcg request body.
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
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post("https://u.y.qq.com/cgi-bin/musicu.fcg", bytes.NewReader(jsonData), headers...)
	if err != nil {
		return nil, err
	}

	// Response shape.
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
