package qq

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

func GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	return defaultQQ.GetUserPlaylists(page, limit)
}

func (q *QQ) GetUserPlaylists(page, limit int) ([]model.Playlist, error) {
	if strings.TrimSpace(q.cookie) == "" {
		return nil, fmt.Errorf("qq user playlists require cookie")
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	uin := qqCookieValue(q.cookie, "uin")
	if uin == "" {
		uin = qqCookieValue(q.cookie, "ptui_loginuin")
	}
	if uin == "" {
		uin = qqCookieValue(q.cookie, "luin")
	}
	uin = strings.TrimLeft(strings.TrimPrefix(uin, "o"), "0")
	if uin == "" {
		return nil, fmt.Errorf("qq user playlists require uin cookie")
	}

	params := url.Values{}
	params.Set("hostuin", uin)
	params.Set("sin", strconv.Itoa((page-1)*limit))
	params.Set("size", strconv.Itoa(limit))
	params.Set("format", "json")
	params.Set("inCharset", "utf8")
	params.Set("outCharset", "utf-8")
	apiURL := "https://c.y.qq.com/rsc/fcgi-bin/fcg_user_created_diss?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		utils.WithHeader("Referer", "https://y.qq.com/"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			DissList []struct {
				DirID      int64  `json:"dirid"`
				DissID     int64  `json:"dissid"`
				DissName   string `json:"diss_name"`
				Title      string `json:"title"`
				DissCover  string `json:"diss_cover"`
				Cover      string `json:"cover"`
				SongNum    int    `json:"song_num"`
				SongCount  int    `json:"song_count"`
				ListenNum  int    `json:"listen_num"`
				VisitNum   int    `json:"visitnum"`
				DissDesc   string `json:"diss_desc"`
				Desc       string `json:"desc"`
				CommitTime string `json:"commit_time"`
			} `json:"disslist"`
			List []struct {
				DissID       string `json:"dissid"`
				DissName     string `json:"dissname"`
				ImgURL       string `json:"imgurl"`
				SongCount    int    `json:"song_count"`
				SongNum      int    `json:"song_num"`
				ListenNum    int    `json:"listennum"`
				Introduction string `json:"introduction"`
			} `json:"list"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq user playlist json parse error: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("qq user playlist api error: %s (code %d)", resp.Message, resp.Code)
	}

	playlists := make([]model.Playlist, 0, len(resp.Data.DissList)+len(resp.Data.List))
	for _, item := range resp.Data.DissList {
		playlistID := ""
		if item.DissID > 0 {
			playlistID = strconv.FormatInt(item.DissID, 10)
		} else if item.DirID > 0 {
			playlistID = strconv.FormatInt(item.DirID, 10)
		}
		name := firstNonEmptyQQ(item.DissName, item.Title)
		if playlistID == "" || name == "" {
			continue
		}
		trackCount := item.SongCount
		if trackCount == 0 {
			trackCount = item.SongNum
		}
		playCount := item.ListenNum
		if playCount == 0 {
			playCount = item.VisitNum
		}
		playlists = append(playlists, model.Playlist{
			Source:      "qq",
			ID:          playlistID,
			Name:        name,
			Cover:       normalizeQQCover(firstNonEmptyQQ(item.DissCover, item.Cover)),
			TrackCount:  trackCount,
			PlayCount:   playCount,
			Creator:     uin,
			Description: firstNonEmptyQQ(item.DissDesc, item.Desc),
			Link:        fmt.Sprintf("https://y.qq.com/n/ryqq/playlist/%s", playlistID),
			Extra: map[string]string{
				"uin":         uin,
				"commit_time": item.CommitTime,
			},
		})
	}
	for _, item := range resp.Data.List {
		playlistID := strings.TrimSpace(item.DissID)
		name := strings.TrimSpace(item.DissName)
		if playlistID == "" || name == "" {
			continue
		}
		trackCount := item.SongCount
		if trackCount == 0 {
			trackCount = item.SongNum
		}
		playlists = append(playlists, model.Playlist{
			Source:      "qq",
			ID:          playlistID,
			Name:        name,
			Cover:       normalizeQQCover(item.ImgURL),
			TrackCount:  trackCount,
			PlayCount:   item.ListenNum,
			Creator:     uin,
			Description: item.Introduction,
			Link:        fmt.Sprintf("https://y.qq.com/n/ryqq/playlist/%s", playlistID),
			Extra: map[string]string{
				"uin": uin,
			},
		})
	}
	return playlists, nil
}

func qqCookieValue(cookie, key string) string {
	for _, part := range strings.Split(cookie, ";") {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[0]) == key {
			return strings.TrimSpace(kv[1])
		}
	}
	return ""
}

func normalizeQQCover(cover string) string {
	cover = strings.TrimSpace(cover)
	if strings.HasPrefix(cover, "//") {
		return "https:" + cover
	}
	if strings.HasPrefix(cover, "http://") {
		return strings.Replace(cover, "http://", "https://", 1)
	}
	return cover
}
