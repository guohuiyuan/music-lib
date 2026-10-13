package netease

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	// TopListAPI 网易云音乐排行榜列表接口
	TopListAPI = "https://music.163.com/api/toplist"
	// HotSongTopListID 网易云音乐热歌榜 ID
	HotSongTopListID = "3778678"
	// DefaultHotSongLimit 热门单曲默认返回条数
	DefaultHotSongLimit = 100
	// MaxHotSongLimit 热门单曲最大返回条数
	MaxHotSongLimit = 200
)

func GetTopLists() ([]model.TopList, error) {
	return defaultNetease.GetTopLists()
}

func GetTopListSongs(topListID string) ([]model.Song, error) {
	return defaultNetease.GetTopListSongs(topListID)
}

func GetHotSongs(limit int) ([]model.Song, error) {
	return defaultNetease.GetHotSongs(limit)
}

// GetTopLists 获取网易云音乐全部排行榜（榜单目录，无需登录）。
func (n *Netease) GetTopLists() ([]model.TopList, error) {
	body, err := utils.Post(TopListAPI, strings.NewReader(""),
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	return parseTopLists(body)
}

func parseTopLists(body []byte) ([]model.TopList, error) {
	var resp struct {
		Code int `json:"code"`
		List []struct {
			ID              int64  `json:"id"`
			Name            string `json:"name"`
			CoverImgURL     string `json:"coverImgUrl"`
			Description     string `json:"description"`
			UpdateFrequency string `json:"updateFrequency"`
			TrackCount      int    `json:"trackCount"`
			PlayCount       int64  `json:"playCount"`
			ToplistType     string `json:"ToplistType"`
			TrackUpdateTime int64  `json:"trackUpdateTime"`
		} `json:"list"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease toplist json parse error: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("netease api error code: %d", resp.Code)
	}

	topLists := make([]model.TopList, 0, len(resp.List))
	for _, item := range resp.List {
		name := strings.TrimSpace(item.Name)
		if item.ID <= 0 || name == "" {
			continue
		}

		id := strconv.FormatInt(item.ID, 10)
		topList := model.TopList{
			Source:          "netease",
			ID:              id,
			Name:            name,
			Cover:           item.CoverImgURL,
			Description:     item.Description,
			UpdateFrequency: item.UpdateFrequency,
			TrackCount:      item.TrackCount,
			PlayCount:       item.PlayCount,
			Link:            fmt.Sprintf("https://music.163.com/#/discover/toplist?id=%s", id),
			Extra:           map[string]string{},
		}
		if item.ToplistType != "" {
			topList.Extra["toplist_type"] = item.ToplistType
		}
		if item.TrackUpdateTime > 0 {
			topList.Extra["update_time"] = strconv.FormatInt(item.TrackUpdateTime, 10)
		}

		topLists = append(topLists, topList)
	}

	return topLists, nil
}

// GetTopListSongs 获取指定排行榜的歌曲，按榜单名次排序，名次写入歌曲 Extra["rank"]。
func (n *Netease) GetTopListSongs(topListID string) ([]model.Song, error) {
	return n.fetchTopListSongs(topListID, 0)
}

// GetHotSongs 获取网易云音乐热歌榜（热门单曲），limit <= 0 时返回默认条数。
func (n *Netease) GetHotSongs(limit int) ([]model.Song, error) {
	return n.fetchTopListSongs(HotSongTopListID, normalizeHotSongLimit(limit))
}

func (n *Netease) fetchTopListSongs(topListID string, limit int) ([]model.Song, error) {
	topListID = strings.TrimSpace(topListID)
	if topListID == "" {
		return nil, errors.New("netease toplist id is empty")
	}
	if !isDigits(topListID) {
		return nil, fmt.Errorf("invalid netease toplist id: %s", topListID)
	}

	_, songs, err := n.fetchPlaylistDetailLimited(topListID, limit)
	if err != nil {
		return nil, err
	}

	for i := range songs {
		if songs[i].Extra == nil {
			songs[i].Extra = make(map[string]string, 2)
		}
		songs[i].Extra["toplist_id"] = topListID
		songs[i].Extra["rank"] = strconv.Itoa(i + 1)
	}

	return songs, nil
}

func normalizeHotSongLimit(limit int) int {
	if limit <= 0 {
		return DefaultHotSongLimit
	}
	if limit > MaxHotSongLimit {
		return MaxHotSongLimit
	}
	return limit
}
