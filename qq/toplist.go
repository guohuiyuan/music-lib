package qq

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
	// TopListAPI QQ音乐排行榜列表接口（musicu.fcg）
	TopListAPI = "https://u.y.qq.com/cgi-bin/musicu.fcg"
	// TopListSongsAPI QQ音乐排行榜歌曲接口
	TopListSongsAPI = "https://c.y.qq.com/v8/fcg-bin/fcg_v8_toplist_cp.fcg"
	// HotSongTopListID QQ音乐热歌榜 ID
	HotSongTopListID = "26"
	// DefaultHotSongLimit 热门单曲默认返回条数
	DefaultHotSongLimit = 100
	// MaxHotSongLimit 热门单曲最大返回条数
	MaxHotSongLimit = 200
	// topListPageSize 榜单歌曲接口单次请求最大条数
	topListPageSize = 50
	// recTypeMV 表示该榜单为 MV（视频）榜，无法作为歌曲榜单解析
	recTypeMV = 10010
)

func GetTopLists() ([]model.TopList, error) {
	return defaultQQ.GetTopLists()
}

func GetTopListSongs(topListID string) ([]model.Song, error) {
	return defaultQQ.GetTopListSongs(topListID)
}

func GetHotSongs(limit int) ([]model.Song, error) {
	return defaultQQ.GetHotSongs(limit)
}

// GetTopLists 获取QQ音乐全部排行榜（榜单目录，无需登录）。
func (q *QQ) GetTopLists() ([]model.TopList, error) {
	payload := map[string]interface{}{
		"comm": map[string]interface{}{
			"cv":                4747474,
			"ct":                24,
			"format":            "json",
			"inCharset":         "utf-8",
			"outCharset":        "utf-8",
			"notice":            0,
			"platform":          "yqq.json",
			"needNewCode":       1,
			"uin":               "0",
			"g_tk_new_20200303": 5381,
			"g_tk":              5381,
		},
		"topList": map[string]interface{}{
			"module": "musicToplist.ToplistInfoServer",
			"method": "GetAll",
			"param":  map[string]interface{}{},
		},
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("qq toplist request marshal error: %w", err)
	}

	apiURL := TopListAPI + "?format=json&data=" + url.QueryEscape(string(payloadJSON))
	body, err := utils.Get(apiURL,
		utils.WithHeader("Referer", "https://y.qq.com/"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	return parseTopLists(body)
}

func parseTopLists(body []byte) ([]model.TopList, error) {
	var resp struct {
		Code    int `json:"code"`
		TopList struct {
			Code int `json:"code"`
			Data struct {
				Group []struct {
					GroupID   int    `json:"groupId"`
					GroupName string `json:"groupName"`
					Toplist   []struct {
						TopID       int    `json:"topId"`
						Title       string `json:"title"`
						TitleDetail string `json:"titleDetail"`
						Intro       string `json:"intro"`
						Period      string `json:"period"`
						UpdateTime  string `json:"updateTime"`
						UpdateTips  string `json:"updateTips"`
						ListenNum   int64  `json:"listenNum"`
						TotalNum    int    `json:"totalNum"`
						TopType     int    `json:"topType"`
						UpdateType  int    `json:"updateType"`
						GroupType   int    `json:"groupType"`
						RecType     int    `json:"recType"`
						FrontPicURL string `json:"frontPicUrl"`
						MbFrontPic  string `json:"mbFrontPicUrl"`
						HeadPicURL  string `json:"headPicUrl"`
						MbHeadPic   string `json:"mbHeadPicUrl"`
						MusicHall   string `json:"musichallPicUrl"`
					} `json:"toplist"`
				} `json:"group"`
			} `json:"data"`
		} `json:"topList"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq toplist json parse error: %w", err)
	}
	if resp.Code != 0 || resp.TopList.Code != 0 {
		return nil, fmt.Errorf("qq toplist api error code: %d/%d", resp.Code, resp.TopList.Code)
	}

	topLists := make([]model.TopList, 0, 32)
	for _, group := range resp.TopList.Data.Group {
		for _, item := range group.Toplist {
			name := strings.TrimSpace(item.Title)
			// MV 榜属于视频内容，没有歌曲 MID，无法作为歌曲榜单使用，这里直接跳过。
			if item.TopID <= 0 || name == "" || item.RecType == recTypeMV {
				continue
			}

			id := strconv.Itoa(item.TopID)
			topList := model.TopList{
				ID:              id,
				Name:            name,
				Cover:           qqTopListCover(item.FrontPicURL, item.MbFrontPic, item.HeadPicURL, item.MbHeadPic, item.MusicHall),
				Description:     cleanTopListIntro(item.Intro),
				UpdateFrequency: strings.TrimSpace(item.UpdateTips),
				TrackCount:      item.TotalNum,
				PlayCount:       item.ListenNum,
				Source:          "qq",
				Link:            fmt.Sprintf("https://y.qq.com/n/ryqq/toplist/%s", id),
				Extra: map[string]string{
					"group":     group.GroupName,
					"group_id":  strconv.Itoa(group.GroupID),
					"grouptype": strconv.Itoa(item.GroupType),
					"top_type":  strconv.Itoa(item.TopType),
				},
			}
			if detail := strings.TrimSpace(item.TitleDetail); detail != "" {
				topList.Extra["title_detail"] = detail
			}
			if item.Period != "" {
				topList.Extra["period"] = item.Period
			}
			if item.UpdateTime != "" {
				topList.Extra["update_time"] = item.UpdateTime
			}

			topLists = append(topLists, topList)
		}
	}

	return topLists, nil
}

// qqTopListCover 返回第一个可用的榜单封面地址，并统一为 https。
func qqTopListCover(urls ...string) string {
	for _, raw := range urls {
		cover := strings.TrimSpace(raw)
		if cover == "" {
			continue
		}
		if strings.HasPrefix(cover, "http://") {
			cover = "https://" + strings.TrimPrefix(cover, "http://")
		}
		return cover
	}
	return ""
}

// cleanTopListIntro 去掉榜单简介里的换行标签，便于在界面直接展示。
func cleanTopListIntro(intro string) string {
	intro = strings.ReplaceAll(intro, "<br>", " ")
	intro = strings.ReplaceAll(intro, "<br/>", " ")
	intro = strings.ReplaceAll(intro, "<br />", " ")
	return strings.TrimSpace(intro)
}

// GetTopListSongs 获取指定排行榜的全部歌曲，按榜单名次排序，名次写入 Song.Extra["rank"]。
func (q *QQ) GetTopListSongs(topListID string) ([]model.Song, error) {
	return q.fetchTopListSongs(topListID, 0)
}

// GetHotSongs 获取QQ音乐热歌榜（热门单曲），limit <= 0 时返回默认条数。
func (q *QQ) GetHotSongs(limit int) ([]model.Song, error) {
	return q.fetchTopListSongs(HotSongTopListID, normalizeHotSongLimit(limit))
}

func (q *QQ) fetchTopListSongs(topListID string, limit int) ([]model.Song, error) {
	topListID = strings.TrimSpace(topListID)
	if topListID == "" {
		return nil, errors.New("qq toplist id is empty")
	}
	if !isTopListDigits(topListID) {
		return nil, fmt.Errorf("invalid qq toplist id: %s", topListID)
	}

	songs := make([]model.Song, 0, topListPageSize)
	for begin := 0; ; begin += topListPageSize {
		pageSongs, err := q.fetchTopListSongsPage(topListID, begin, topListPageSize)
		if err != nil {
			return nil, err
		}
		if len(pageSongs) == 0 {
			break
		}

		songs = append(songs, pageSongs...)
		if limit > 0 && len(songs) >= limit {
			songs = songs[:limit]
			break
		}
		if len(pageSongs) < topListPageSize {
			break
		}
	}

	if len(songs) == 0 {
		return nil, fmt.Errorf("qq toplist %s has no songs", topListID)
	}

	return songs, nil
}

// fetchTopListSongsPage 拉取榜单中 [begin, begin+num) 区间的歌曲。
func (q *QQ) fetchTopListSongsPage(topListID string, begin, num int) ([]model.Song, error) {
	params := url.Values{}
	params.Set("topid", topListID)
	params.Set("song_begin", strconv.Itoa(begin))
	params.Set("song_num", strconv.Itoa(num))
	params.Set("format", "json")
	params.Set("inCharset", "utf8")
	params.Set("outCharset", "utf-8")
	params.Set("notice", "0")
	params.Set("platform", "h5")
	params.Set("needNewCode", "1")
	params.Set("tpl", "3")
	params.Set("page", "detail")
	params.Set("type", "top")

	body, err := utils.Get(TopListSongsAPI+"?"+params.Encode(),
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", "https://y.qq.com/"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	return parseTopListSongs(body, topListID, begin)
}

func parseTopListSongs(body []byte, topListID string, begin int) ([]model.Song, error) {
	var resp struct {
		Code     int `json:"code"`
		SongList []struct {
			CurCount string `json:"cur_count"`
			Data     struct {
				SongID      int64  `json:"songid"`
				SongMID     string `json:"songmid"`
				SongName    string `json:"songname"`
				AlbumName   string `json:"albumname"`
				AlbumMID    string `json:"albummid"`
				Interval    int    `json:"interval"`
				Size128     int64  `json:"size128"`
				Size320     int64  `json:"size320"`
				SizeFlac    int64  `json:"sizeflac"`
				StrMediaMid string `json:"strMediaMid"`
				Pay         struct {
					PayPlay int `json:"payplay"`
				} `json:"pay"`
				Singer []struct {
					Name string `json:"name"`
					Mid  string `json:"mid"`
				} `json:"singer"`
			} `json:"data"`
		} `json:"songlist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq toplist songs json parse error: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("qq toplist songs api error code: %d", resp.Code)
	}

	songs := make([]model.Song, 0, len(resp.SongList))
	for i, item := range resp.SongList {
		info := item.Data
		if info.SongMID == "" || info.SongName == "" {
			continue
		}

		artistNames := make([]string, 0, len(info.Singer))
		for _, singer := range info.Singer {
			if name := strings.TrimSpace(singer.Name); name != "" {
				artistNames = append(artistNames, name)
			}
		}

		fileSize := info.Size128
		bitrate := 128
		if info.SizeFlac > 0 {
			fileSize = info.SizeFlac
			if info.Interval > 0 {
				bitrate = int(fileSize * 8 / 1000 / int64(info.Interval))
			} else {
				bitrate = 800
			}
		} else if info.Size320 > 0 {
			fileSize = info.Size320
			bitrate = 320
		}

		cover := ""
		if info.AlbumMID != "" {
			cover = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", info.AlbumMID)
		}

		rank := strings.TrimSpace(item.CurCount)
		if rank == "" {
			rank = strconv.Itoa(begin + i + 1)
		}

		song := model.Song{
			Source:   "qq",
			ID:       info.SongMID,
			Name:     info.SongName,
			Artist:   joinQQNames(artistNames),
			Album:    info.AlbumName,
			AlbumID:  info.AlbumMID,
			Duration: info.Interval,
			Size:     fileSize,
			Bitrate:  bitrate,
			Cover:    cover,
			Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", info.SongMID),
			Extra: map[string]string{
				"songmid":   info.SongMID,
				"song_id":   strconv.FormatInt(info.SongID, 10),
				"album_mid": info.AlbumMID,
			},
		}
		if info.StrMediaMid != "" {
			song.Extra["media_mid"] = info.StrMediaMid
		}
		song.Extra["toplist_id"] = topListID
		song.Extra["rank"] = rank

		songs = append(songs, song)
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

func isTopListDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
