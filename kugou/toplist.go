package kugou

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
	// TopListAPI 酷狗排行榜列表接口
	TopListAPI = "http://mobilecdn.kugou.com/api/v3/rank/list"
	// TopListSongsAPI 酷狗排行榜歌曲接口
	TopListSongsAPI = "http://mobilecdn.kugou.com/api/v3/rank/song"
	// HotSongTopListID 酷狗 TOP500 榜单 ID，作为「热门单曲」来源
	HotSongTopListID = "8888"
	// DefaultHotSongLimit 热门单曲默认返回条数
	DefaultHotSongLimit = 100
	// MaxHotSongLimit 热门单曲最大返回条数
	MaxHotSongLimit = 200
	// topListPageSize 榜单歌曲接口单次请求最大条数（实测 pagesize=300 可用）
	topListPageSize = 300
)

func GetTopLists() ([]model.TopList, error) {
	return defaultKugou.GetTopLists()
}

func GetTopListSongs(topListID string) ([]model.Song, error) {
	return defaultKugou.GetTopListSongs(topListID)
}

func GetHotSongs(limit int) ([]model.Song, error) {
	return defaultKugou.GetHotSongs(limit)
}

// GetTopLists 获取酷狗全部排行榜（榜单目录，无需登录）。
func (k *Kugou) GetTopLists() ([]model.TopList, error) {
	params := url.Values{}
	params.Set("version", "9108")
	params.Set("plat", "0")
	params.Set("showtype", "2")
	params.Set("parentid", "0")
	params.Set("apiver", "2")
	params.Set("area_code", "1")
	// withsong=1 时接口会在每条榜单的 extra.resp.all_total 里带上曲目总数
	params.Set("withsong", "1")
	params.Set("with_res_tag", "0")
	params.Set("page", "1")
	params.Set("pagesize", "100")

	body, err := utils.Get(TopListAPI+"?"+params.Encode(),
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	return parseTopLists(body)
}

type kugouTopListItem struct {
	RankID        int         `json:"rankid"`
	RankName      string      `json:"rankname"`
	RankType      int         `json:"ranktype"`
	Classify      int         `json:"classify"`
	Frequency     string      `json:"update_frequency"`
	FrequencyType int         `json:"update_frequency_type"`
	PlayTimes     int64       `json:"play_times"`
	Intro         string      `json:"intro"`
	ImgURL        string      `json:"imgurl"`
	Img9          string      `json:"img_9"`
	ImgCover      string      `json:"img_cover"`
	AlbumImg9     string      `json:"album_img_9"`
	PublishDate   string      `json:"rank_id_publish_date"`
	Issue         interface{} `json:"issue"`
	RankCID       int         `json:"rank_cid"`
	Extra         struct {
		Resp struct {
			AllTotal int64 `json:"all_total"`
		} `json:"resp"`
	} `json:"extra"`
}

func parseTopLists(body []byte) ([]model.TopList, error) {
	var resp struct {
		Status  int    `json:"status"`
		Errcode int    `json:"errcode"`
		Error   string `json:"error"`
		Data    struct {
			Total int                `json:"total"`
			Info  []kugouTopListItem `json:"info"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kugou toplist json parse error: %w", err)
	}
	if resp.Status != 1 || resp.Errcode != 0 {
		return nil, fmt.Errorf("kugou toplist api error: %s (status %d errcode %d)", resp.Error, resp.Status, resp.Errcode)
	}

	topLists := make([]model.TopList, 0, len(resp.Data.Info))
	for _, item := range resp.Data.Info {
		name := strings.TrimSpace(item.RankName)
		if item.RankID <= 0 || name == "" {
			continue
		}

		id := strconv.Itoa(item.RankID)
		topList := model.TopList{
			ID:              id,
			Name:            name,
			Cover:           kugouTopListCover(item.ImgURL, item.Img9, item.AlbumImg9, item.ImgCover),
			Description:     cleanKugouTopListIntro(item.Intro),
			UpdateFrequency: strings.TrimSpace(item.Frequency),
			// 曲目总数来自 extra.resp.all_total（withsong=1 时返回），缺失时为 0。
			TrackCount: int(item.Extra.Resp.AllTotal),
			PlayCount:  item.PlayTimes,
			Source:     "kugou",
			Link:       fmt.Sprintf("https://www.kugou.com/yy/rank/home/1-%s.html", id),
			Extra: map[string]string{
				"group":     kugouTopListGroupName(item.Classify),
				"classify":  strconv.Itoa(item.Classify),
				"rank_type": strconv.Itoa(item.RankType),
				"freq_type": strconv.Itoa(item.FrequencyType),
				"rank_cid":  strconv.Itoa(item.RankCID),
			},
		}
		if item.PublishDate != "" {
			topList.Extra["publish_time"] = item.PublishDate
		}
		if issue := formatKugouNumericString(item.Issue); issue != "" {
			topList.Extra["issue"] = issue
		}

		topLists = append(topLists, topList)
	}

	return topLists, nil
}

// kugouTopListGroupName 把酷狗榜单分类编号转换为分组名称。
func kugouTopListGroupName(classify int) string {
	switch classify {
	case 1:
		return "热门榜"
	case 2:
		return "地区榜"
	case 3:
		return "特色榜"
	case 4:
		return "全球榜"
	case 5:
		return "曲风榜"
	default:
		return "其他"
	}
}

// kugouTopListCover 返回第一个可用的封面地址，替换尺寸占位符并统一为 https。
func kugouTopListCover(urls ...string) string {
	for _, raw := range urls {
		cover := strings.TrimSpace(raw)
		if cover == "" {
			continue
		}
		cover = strings.Replace(cover, "{size}", "400", 1)
		if strings.HasPrefix(cover, "http://") {
			cover = "https://" + strings.TrimPrefix(cover, "http://")
		}
		return cover
	}
	return ""
}

// cleanKugouTopListIntro 去掉榜单简介中的换行，便于在界面直接展示。
func cleanKugouTopListIntro(intro string) string {
	intro = strings.ReplaceAll(intro, "\r\n", " ")
	intro = strings.ReplaceAll(intro, "\n", " ")
	intro = strings.ReplaceAll(intro, "\r", " ")
	return strings.Join(strings.Fields(intro), " ")
}

// GetTopListSongs 获取指定排行榜的全部歌曲，按榜单名次排序，名次写入 Song.Extra["rank"]。
func (k *Kugou) GetTopListSongs(topListID string) ([]model.Song, error) {
	return k.fetchTopListSongs(topListID, 0)
}

// GetHotSongs 获取酷狗 TOP500（热门单曲），limit <= 0 时返回默认条数。
func (k *Kugou) GetHotSongs(limit int) ([]model.Song, error) {
	return k.fetchTopListSongs(HotSongTopListID, normalizeHotSongLimit(limit))
}

func (k *Kugou) fetchTopListSongs(topListID string, limit int) ([]model.Song, error) {
	topListID = strings.TrimSpace(topListID)
	if topListID == "" {
		return nil, errors.New("kugou toplist id is empty")
	}
	if !isKugouTopListID(topListID) {
		return nil, fmt.Errorf("invalid kugou toplist id: %s", topListID)
	}

	pageSize := topListPageSize
	if limit > 0 && limit < pageSize {
		pageSize = limit
	}

	songs := make([]model.Song, 0, pageSize)
	for page := 1; ; page++ {
		pageSongs, total, err := k.fetchTopListSongsPage(topListID, page, pageSize)
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
		if total > 0 && len(songs) >= total {
			break
		}
		if len(pageSongs) < pageSize {
			break
		}
	}

	if len(songs) == 0 {
		return nil, fmt.Errorf("kugou toplist %s has no songs", topListID)
	}

	return songs, nil
}

// fetchTopListSongsPage 拉取榜单第 page 页歌曲，同时返回接口给出的曲目总数。
func (k *Kugou) fetchTopListSongsPage(topListID string, page, pageSize int) ([]model.Song, int, error) {
	params := url.Values{}
	params.Set("version", "9108")
	params.Set("plat", "0")
	params.Set("ranktype", "0")
	params.Set("area_code", "1")
	params.Set("rankid", topListID)
	params.Set("page", strconv.Itoa(page))
	params.Set("pagesize", strconv.Itoa(pageSize))
	params.Set("with_res_tag", "0")

	body, err := utils.Get(TopListSongsAPI+"?"+params.Encode(),
		utils.WithHeader("User-Agent", MobileUserAgent),
		utils.WithHeader("Referer", MobileReferer),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, 0, err
	}

	return parseTopListSongs(body, topListID, (page-1)*pageSize)
}

type kugouTopListSong struct {
	Hash         string `json:"hash"`
	HashHigh     string `json:"hash_high"`
	HashSuper    string `json:"hash_super"`
	HQHash       string `json:"320hash"`
	SQHash       string `json:"sqhash"`
	FileSize     int64  `json:"filesize"`
	HQFileSize   int64  `json:"320filesize"`
	SQFileSize   int64  `json:"sqfilesize"`
	Duration     int    `json:"duration"`
	Bitrate      int    `json:"bitrate"`
	SongName     string `json:"songname"`
	FileName     string `json:"filename"`
	Remark       string `json:"remark"`
	AlbumID      string `json:"album_id"`
	AudioID      int64  `json:"audio_id"`
	AlbumAudioID int64  `json:"album_audio_id"`
	MvHash       string `json:"mvhash"`
	Privilege    int    `json:"privilege"`
	Cover        string `json:"album_sizable_cover"`
	Authors      []struct {
		AuthorName string `json:"author_name"`
	} `json:"authors"`
	TransParam struct {
		UnionCover string `json:"union_cover"`
		Ogg320Hash string `json:"ogg_320_hash"`
		Ogg128Hash string `json:"ogg_128_hash"`
	} `json:"trans_param"`
}

func parseTopListSongs(body []byte, topListID string, offset int) ([]model.Song, int, error) {
	var resp struct {
		Status  int    `json:"status"`
		Errcode int    `json:"errcode"`
		Error   string `json:"error"`
		Data    struct {
			Total int                `json:"total"`
			Info  []kugouTopListSong `json:"info"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("kugou toplist songs json parse error: %w", err)
	}
	if resp.Status != 1 || resp.Errcode != 0 {
		return nil, 0, fmt.Errorf("kugou toplist songs api error: %s (status %d errcode %d)", resp.Error, resp.Status, resp.Errcode)
	}

	songs := make([]model.Song, 0, len(resp.Data.Info))
	for i, item := range resp.Data.Info {
		finalHash := firstNonEmpty(item.Hash, item.SQHash, item.HQHash, item.HashHigh, item.TransParam.Ogg320Hash, item.TransParam.Ogg128Hash)
		if !isValidHash(finalHash) {
			continue
		}

		name := strings.TrimSpace(item.SongName)
		artist := kugouTopListArtist(item.Authors)
		if name == "" || artist == "" {
			parts := strings.Split(item.FileName, " - ")
			if len(parts) >= 2 {
				if artist == "" {
					artist = strings.TrimSpace(parts[0])
				}
				if name == "" {
					name = strings.TrimSpace(strings.Join(parts[1:], " - "))
				}
			} else if name == "" {
				name = strings.TrimSpace(item.FileName)
			}
		}
		if name == "" {
			continue
		}

		size := item.FileSize
		switch finalHash {
		case item.SQHash:
			if item.SQFileSize > 0 {
				size = item.SQFileSize
			}
		case item.HQHash:
			if item.HQFileSize > 0 {
				size = item.HQFileSize
			}
		}

		bitrate := item.Bitrate
		if bitrate <= 0 && size > 0 && item.Duration > 0 {
			bitrate = int(size * 8 / 1000 / int64(item.Duration))
		}

		songs = append(songs, model.Song{
			Source:   "kugou",
			ID:       finalHash,
			Name:     cleanKugouSearchText(name),
			Artist:   cleanKugouSearchText(artist),
			Album:    cleanKugouSearchText(item.Remark),
			AlbumID:  item.AlbumID,
			Duration: item.Duration,
			Size:     size,
			Bitrate:  bitrate,
			Cover:    kugouTopListSongCover(item.Cover, item.TransParam.UnionCover),
			Link:     fmt.Sprintf("https://www.kugou.com/song/#hash=%s", finalHash),
			Extra: map[string]string{
				"hash":           finalHash,
				"file_hash":      item.Hash,
				"sq_hash":        item.SQHash,
				"hq_hash":        item.HQHash,
				"res_hash":       item.HashSuper,
				"high_hash":      item.HashHigh,
				"ogg_320_hash":   item.TransParam.Ogg320Hash,
				"ogg_128_hash":   item.TransParam.Ogg128Hash,
				"mv_hash":        item.MvHash,
				"audio_id":       formatKugouNumericString(item.AudioID),
				"album_audio_id": formatKugouNumericString(item.AlbumAudioID),
				"album_id":       item.AlbumID,
				"privilege":      strconv.Itoa(item.Privilege),
				"toplist_id":     topListID,
				"rank":           strconv.Itoa(offset + i + 1),
			},
		})
	}

	return songs, resp.Data.Total, nil
}

// kugouTopListArtist 拼接榜单歌曲的歌手名。
func kugouTopListArtist(authors []struct {
	AuthorName string `json:"author_name"`
}) string {
	names := make([]string, 0, len(authors))
	for _, author := range authors {
		if name := strings.TrimSpace(author.AuthorName); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

// kugouTopListSongCover 返回歌曲封面地址，替换尺寸占位符并统一为 https。
func kugouTopListSongCover(urls ...string) string {
	for _, raw := range urls {
		cover := strings.TrimSpace(raw)
		if cover == "" {
			continue
		}
		cover = strings.Replace(cover, "{size}", "240", 1)
		if strings.HasPrefix(cover, "http://") {
			cover = "https://" + strings.TrimPrefix(cover, "http://")
		}
		return cover
	}
	return ""
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

func isKugouTopListID(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
