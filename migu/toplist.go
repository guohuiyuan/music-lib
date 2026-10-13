package migu

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	// TopListAPI 咪咕排行榜目录接口（移动端 v2.0，含特色榜/尖叫榜/精选榜单）。
	TopListAPI = "https://app.u.nf.migu.cn/bmw/rank/rank-index/v2.0"
	// TopListFallbackAPI 咪咕 PC 端排行榜目录接口，作为 v2.0 失败时的兜底。
	TopListFallbackAPI = "https://app.c.nf.migu.cn/pc/bmw/rank/rank-index/v1.0"
	// TopListSongsAPI 咪咕榜单歌曲接口，每页固定返回 50 首，无需登录。
	TopListSongsAPI = "https://app.c.nf.migu.cn/bmw/rank/rank-info/v1.0"
	// HotSongTopListID 咪咕「全网热歌榜」ID，作为「热门单曲」来源。
	HotSongTopListID = "83048887"
	// DefaultHotSongLimit 热门单曲默认返回条数。
	DefaultHotSongLimit = 50
	// MaxHotSongLimit 热门单曲最大返回条数（榜单本身只有 50 首）。
	MaxHotSongLimit = 50
	// miguTopListPageSize 榜单歌曲接口单页条数（接口固定 50 条/页）。
	miguTopListPageSize = 50
	// miguTopListMaxPages 兜底分页上限，避免接口异常时无上限翻页。
	miguTopListMaxPages = 10
)

func GetTopLists() ([]model.TopList, error) {
	return defaultMigu.GetTopLists()
}

func GetTopListSongs(topListID string) ([]model.Song, error) {
	return defaultMigu.GetTopListSongs(topListID)
}

func GetHotSongs(limit int) ([]model.Song, error) {
	return defaultMigu.GetHotSongs(limit)
}

// miguTopListIndexAPIs 榜单目录接口，按顺序尝试，前一个失败时回退到下一个。
var miguTopListIndexAPIs = []string{TopListAPI, TopListFallbackAPI}

// miguTopListSongsAPI 榜单歌曲接口，测试时可替换为本地服务地址。
var miguTopListSongsAPI = TopListSongsAPI

// GetTopLists 获取咪咕排行榜目录（无需登录）。
// 优先使用移动端 v2.0 目录（含「精选榜单」），失败时回退到 PC 端目录。
func (m *Migu) GetTopLists() ([]model.TopList, error) {
	errs := make([]string, 0, len(miguTopListIndexAPIs))
	for _, apiURL := range miguTopListIndexAPIs {
		topLists, err := m.fetchTopLists(apiURL)
		if err == nil && len(topLists) > 0 {
			return topLists, nil
		}
		if err == nil {
			err = errors.New("migu toplist index is empty")
		}
		errs = append(errs, err.Error())
	}
	if len(errs) == 0 {
		errs = append(errs, "migu toplist index is unavailable")
	}
	return nil, fmt.Errorf("migu toplist failed: %s", strings.Join(errs, "; fallback: "))
}

// GetTopListSongs 获取指定榜单的全部歌曲，按榜单名次排序，名次写入 Song.Extra["rank"]。
func (m *Migu) GetTopListSongs(topListID string) ([]model.Song, error) {
	return m.fetchTopListSongs(topListID, 0)
}

// GetHotSongs 获取咪咕全网热歌榜（热门单曲），limit <= 0 时返回默认条数。
func (m *Migu) GetHotSongs(limit int) ([]model.Song, error) {
	return m.fetchTopListSongs(HotSongTopListID, normalizeMiguHotSongLimit(limit))
}

func (m *Migu) fetchTopLists(apiURL string) ([]model.TopList, error) {
	body, err := utils.Get(apiURL, miguTopListRequestOptions(m.cookie)...)
	if err != nil {
		return nil, err
	}
	return parseMiguTopLists(body)
}

func miguTopListRequestOptions(cookie string) []utils.RequestOption {
	opts := []utils.RequestOption{
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", Referer),
	}
	if strings.TrimSpace(cookie) != "" {
		opts = append(opts, utils.WithHeader("Cookie", cookie))
	}
	return opts
}

type miguRankGroup struct {
	View     string          `json:"view"`
	Style    string          `json:"style"`
	Contents []miguRankBrief `json:"contents"`
}

type miguRankBrief struct {
	View     string `json:"view"`
	RankID   string `json:"rankId"`
	RankName string `json:"rankName"`
	ImageURL string `json:"imageUrl"`
}

type miguRankIndexResponse struct {
	Code string `json:"code"`
	Info string `json:"info"`
	Data struct {
		Contents []miguRankGroup `json:"contents"`
	} `json:"data"`
}

func parseMiguTopLists(body []byte) ([]model.TopList, error) {
	var resp miguRankIndexResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("migu toplist json parse error: %w", err)
	}
	if resp.Code != "" && resp.Code != "000000" {
		return nil, fmt.Errorf("migu toplist api error: %s (code %s)", resp.Info, resp.Code)
	}

	seen := make(map[string]struct{})
	topLists := make([]model.TopList, 0, 24)
	for _, group := range resp.Data.Contents {
		groupName := strings.TrimSpace(group.Style)
		if groupName == "" {
			groupName = "咪咕榜单"
		}
		for _, item := range group.Contents {
			rankID := strings.TrimSpace(item.RankID)
			rankName := strings.TrimSpace(item.RankName)
			if rankID == "" || rankName == "" {
				continue
			}
			// 目录里「精选榜单」会出现重复分组，按榜单 ID 全局去重。
			if _, ok := seen[rankID]; ok {
				continue
			}
			seen[rankID] = struct{}{}

			topLists = append(topLists, model.TopList{
				ID:          rankID,
				Name:        rankName,
				Cover:       normalizeMiguImageURL(item.ImageURL),
				Description: "咪咕音乐 · " + groupName,
				Source:      "migu",
				Link:        miguTopListLink(rankID),
				Extra: map[string]string{
					"rank_id":      rankID,
					"group":        groupName,
					"toplist_type": "rank",
				},
			})
		}
	}
	if len(topLists) == 0 {
		return nil, errors.New("migu toplist index is empty")
	}
	return topLists, nil
}

type miguRankInfoItem struct {
	ResType  string `json:"resType"`
	ResID    string `json:"resId"`
	Txt      string `json:"txt"`
	Txt2     string `json:"txt2"`
	Txt3     string `json:"txt3"`
	Score    string `json:"score"`
	SongData string `json:"songData"`
}

type miguRankInfoResponse struct {
	Code string `json:"code"`
	Info string `json:"info"`
	Data struct {
		Title       string             `json:"title"`
		Desc        string             `json:"desc"`
		UpdateTime  string             `json:"updateTime"`
		TotalCount  int                `json:"totalCount"`
		HasNextPage bool               `json:"hasNextPage"`
		Contents    []miguRankInfoItem `json:"contents"`
	} `json:"data"`
}

func parseMiguRankInfoResponse(body []byte) (*miguRankInfoResponse, error) {
	var resp miguRankInfoResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("migu toplist songs json parse error: %w", err)
	}
	if resp.Code != "" && resp.Code != "000000" {
		return nil, fmt.Errorf("migu toplist songs api error: %s (code %s)", resp.Info, resp.Code)
	}
	if len(resp.Data.Contents) == 0 {
		return nil, errors.New("migu toplist songs is empty")
	}
	return &resp, nil
}

func (m *Migu) fetchTopListSongs(topListID string, limit int) ([]model.Song, error) {
	topListID = strings.TrimSpace(topListID)
	if topListID == "" {
		return nil, errors.New("migu toplist id is empty")
	}
	if !isMiguTopListID(topListID) {
		return nil, fmt.Errorf("invalid migu toplist id: %s", topListID)
	}

	songs := make([]model.Song, 0, miguTopListPageSize)
	seen := make(map[string]struct{})
	for pageNo := 1; pageNo <= miguTopListMaxPages; pageNo++ {
		pageSongs, hasNextPage, err := m.fetchTopListSongsPage(topListID, pageNo)
		if err != nil {
			return nil, err
		}
		if len(pageSongs) == 0 {
			break
		}

		for _, song := range pageSongs {
			key := firstNonEmpty(song.Extra["content_id"], song.ID, song.Name+"|"+song.Artist)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}

			if song.Extra == nil {
				song.Extra = make(map[string]string, 4)
			}
			song.Extra["rank"] = strconv.Itoa(len(songs) + 1)
			song.Extra["toplist_id"] = topListID
			songs = append(songs, song)
			if limit > 0 && len(songs) >= limit {
				break
			}
		}

		if limit > 0 && len(songs) >= limit {
			break
		}
		if !hasNextPage {
			break
		}
	}

	if len(songs) == 0 {
		return nil, fmt.Errorf("migu toplist %s has no songs", topListID)
	}
	return songs, nil
}

func (m *Migu) fetchTopListSongsPage(topListID string, pageNo int) ([]model.Song, bool, error) {
	params := url.Values{}
	params.Set("pageNo", strconv.Itoa(pageNo))
	params.Set("rankId", topListID)

	body, err := utils.Get(miguTopListSongsAPI+"?"+params.Encode(), miguTopListRequestOptions(m.cookie)...)
	if err != nil {
		return nil, false, err
	}

	resp, err := parseMiguRankInfoResponse(body)
	if err != nil {
		return nil, false, err
	}

	songs := make([]model.Song, 0, len(resp.Data.Contents))
	for _, item := range resp.Data.Contents {
		song := m.convertRankItemToSong(item)
		if song == nil {
			continue
		}
		if score := strings.TrimSpace(item.Score); score != "" {
			if song.Extra == nil {
				song.Extra = make(map[string]string, 2)
			}
			song.Extra["rank_score"] = score
		}
		songs = append(songs, *song)
	}
	return songs, resp.Data.HasNextPage, nil
}

// convertRankItemToSong 把榜单条目里的 songData 转成歌曲，songData 缺失或格式不全时跳过。
func (m *Migu) convertRankItemToSong(item miguRankInfoItem) *model.Song {
	songData := strings.TrimSpace(item.SongData)
	if songData == "" {
		return nil
	}

	var data MiguSongItem
	if err := json.Unmarshal([]byte(songData), &data); err != nil {
		return nil
	}
	return m.convertItemToSong(data)
}

func isMiguTopListID(topListID string) bool {
	if topListID == "" {
		return false
	}
	for _, r := range topListID {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func normalizeMiguHotSongLimit(limit int) int {
	if limit <= 0 {
		return DefaultHotSongLimit
	}
	if limit > MaxHotSongLimit {
		return MaxHotSongLimit
	}
	return limit
}

func miguTopListLink(rankID string) string {
	return fmt.Sprintf("https://music.migu.cn/v5/#/rankDetail?id=%s&playlistType=charts", rankID)
}
