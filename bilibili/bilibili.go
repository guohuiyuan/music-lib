package bilibili

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
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 Edg/121.0.0.0"
	Referer   = "https://www.bilibili.com/"
)

// Bilibili 结构体
type Bilibili struct {
	cookie     string
	isVipCache *bool
}

// New 初始化函数
func New(cookie string) *Bilibili {
	if cookie == "" {
		cookie = "buvid3=2E109C72-251F-3827-FA8E-921FA0D7EC5291319infoc; SESSDATA=your_sessdata;"
	}
	return &Bilibili{
		cookie: cookie,
	}
}

var defaultBilibili = New("buvid3=2E109C72-251F-3827-FA8E-921FA0D7EC5291319infoc; SESSDATA=your_sessdata;")

func Search(keyword string) ([]model.Song, error) { return defaultBilibili.Search(keyword) }
func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultBilibili.SearchPlaylist(keyword)
}
func GetPlaylistSongs(id string) ([]model.Song, error) { return defaultBilibili.GetPlaylistSongs(id) }
func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultBilibili.ParsePlaylist(link)
}
func GetDownloadURL(s *model.Song) (string, error) { return defaultBilibili.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error)      { return defaultBilibili.GetLyrics(s) }
func Parse(link string) (*model.Song, error)       { return defaultBilibili.Parse(link) }

// IsVipAccount 检测 Bilibili 账号是否为大会员
func (b *Bilibili) IsVipAccount() (bool, error) {
	if b.isVipCache != nil {
		return *b.isVipCache, nil
	}

	if b.cookie == "" {
		isVip := false
		b.isVipCache = &isVip
		return false, nil
	}

	apiURL := "https://api.bilibili.com/x/web-interface/nav"
	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
	if err != nil {
		return false, err
	}

	var resp struct {
		Code int `json:"code"`
		Data struct {
			IsLogin   bool `json:"isLogin"`
			VipStatus int  `json:"vipStatus"`
			VipType   int  `json:"vipType"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return false, err
	}

	isVip := false
	if resp.Code == 0 && resp.Data.IsLogin && resp.Data.VipStatus == 1 && resp.Data.VipType > 0 {
		isVip = true
	}

	b.isVipCache = &isVip
	return isVip, nil
}

type bilibiliViewResponse struct {
	Data struct {
		BVID  string `json:"bvid"`
		Title string `json:"title"`
		Pic   string `json:"pic"`
		Owner struct {
			Name string `json:"name"`
			Mid  int64  `json:"mid"`
		} `json:"owner"`
		Pages     []bilibiliPage `json:"pages"`
		UgcSeason *struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
			Cover string `json:"cover"`
			Intro string `json:"intro"`
			Stat  struct {
				View int `json:"view"`
			} `json:"stat"`
			Sections []bilibiliSeasonSection `json:"sections"`
		} `json:"ugc_season"`
	} `json:"data"`
}

type bilibiliPage struct {
	CID      int64  `json:"cid"`
	Part     string `json:"part"`
	Duration int    `json:"duration"`
}

type bilibiliSeasonResp struct {
	Code int `json:"code"`
	Data struct {
		Season struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
			Cover string `json:"cover"`
			Intro string `json:"intro"`
		} `json:"season"`
		Page struct {
			Total int `json:"total"`
		} `json:"page"`
		Archives []struct {
			BVID     string `json:"bvid"`
			Title    string `json:"title"`
			Cover    string `json:"cover"`
			Duration int    `json:"duration"`
			CID      int64  `json:"cid"`
		} `json:"archives"`
	} `json:"data"`
}

type bilibiliPageListResp struct {
	Code int            `json:"code"`
	Data []bilibiliPage `json:"data"`
}

func cleanTitle(t string) string {
	if t == "" {
		return ""
	}
	t = strings.ReplaceAll(t, "<em class=\"keyword\">", "")
	t = strings.ReplaceAll(t, "</em>", "")
	return t
}

type bilibiliSeasonSection struct {
	Episodes []struct {
		BVID     string `json:"bvid"`
		CID      int64  `json:"cid"`
		Title    string `json:"title"`
		Cover    string `json:"cover"`
		Duration int    `json:"duration"`
		Arc      struct {
			Pic      string `json:"pic"`
			Title    string `json:"title"`
			Duration int    `json:"duration"`
		} `json:"arc"`
		Page struct {
			Part     string `json:"part"`
			Duration int    `json:"duration"`
		} `json:"page"`
	} `json:"episodes"`
}

type bilibiliSeasonArchiveMeta struct {
	Title    string
	Cover    string
	Duration int
}

func countSeasonEpisodes(sections []bilibiliSeasonSection) int {
	count := 0
	for _, sec := range sections {
		count += len(sec.Episodes)
	}
	return count
}

func (b *Bilibili) buildSongsFromSeasonSections(sections []bilibiliSeasonSection, seasonTitle, seasonCover, artistName string, archiveIndex map[string]bilibiliSeasonArchiveMeta) []model.Song {
	var songs []model.Song
	if artistName == "" {
		artistName = seasonTitle
	}
	for _, sec := range sections {
		for _, ep := range sec.Episodes {
			cover := ep.Cover
			meta, hasMeta := archiveIndex["bvid:"+ep.BVID]
			if !hasMeta && ep.CID != 0 {
				meta, hasMeta = archiveIndex["cid:"+strconv.FormatInt(ep.CID, 10)]
			}
			if cover == "" {
				cover = ep.Arc.Pic
			}
			if cover == "" {
				cover = seasonCover
				if cover == "" && hasMeta {
					cover = meta.Cover
				}
			}
			cover = normalizeCover(cover)
			duration := ep.Duration
			if duration == 0 {
				duration = ep.Page.Duration
			}
			if duration == 0 {
				duration = ep.Arc.Duration
			}
			if duration == 0 && hasMeta {
				duration = meta.Duration
			}
			name := ep.Title
			if name == "" {
				name = ep.Arc.Title
			}
			if name == "" {
				name = ep.Page.Part
			}
			if name == "" && hasMeta {
				name = meta.Title
			}
			songs = append(songs, model.Song{
				Source:   "bilibili",
				ID:       fmt.Sprintf("%s|%d", ep.BVID, ep.CID),
				Name:     name,
				Artist:   artistName,
				Album:    ep.BVID,
				Duration: duration,
				Cover:    cover,
				Link:     fmt.Sprintf("https://www.bilibili.com/video/%s", ep.BVID),
				Extra: map[string]string{
					"bvid": ep.BVID,
					"cid":  strconv.FormatInt(ep.CID, 10),
				},
			})
		}
	}
	return songs
}

func findPageDuration(pages []bilibiliPage, cid int64) int {
	if cid == 0 {
		return 0
	}
	for _, p := range pages {
		if p.CID == cid {
			return p.Duration
		}
	}
	return 0
}

func needsSeasonArchiveFallback(sections []bilibiliSeasonSection) bool {
	for _, sec := range sections {
		for _, ep := range sec.Episodes {
			if ep.Duration == 0 || ep.Cover == "" || ep.Title == "" {
				return true
			}
		}
	}
	return false
}

func (b *Bilibili) fetchSeasonArchiveIndex(mid, seasonID int64) (map[string]bilibiliSeasonArchiveMeta, string, string, error) {
	if seasonID == 0 || mid == 0 {
		return nil, "", "", errors.New("invalid season info")
	}

	index := make(map[string]bilibiliSeasonArchiveMeta)
	processedArchives := 0
	var seasonTitle string
	var seasonCover string
	pageNum := 1
	pageSize := 30
	for {
		apiURL := fmt.Sprintf("https://api.bilibili.com/x/space/ugc/season?mid=%d&season_id=%d&page_num=%d&page_size=%d", mid, seasonID, pageNum, pageSize)
		body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
		if err != nil {
			return nil, "", "", err
		}

		var resp bilibiliSeasonResp
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, "", "", err
		}
		if resp.Code != 0 {
			return nil, "", "", fmt.Errorf("bilibili season api error: %d", resp.Code)
		}
		if seasonTitle == "" {
			seasonTitle = resp.Data.Season.Title
		}
		if seasonCover == "" {
			seasonCover = resp.Data.Season.Cover
		}
		if len(resp.Data.Archives) == 0 {
			break
		}

		for _, arc := range resp.Data.Archives {
			meta := bilibiliSeasonArchiveMeta{
				Title:    arc.Title,
				Cover:    normalizeCover(arc.Cover),
				Duration: arc.Duration,
			}
			if arc.BVID != "" {
				index["bvid:"+arc.BVID] = meta
			}
			if arc.CID != 0 {
				index["cid:"+strconv.FormatInt(arc.CID, 10)] = meta
			}
		}
		processedArchives += len(resp.Data.Archives)

		total := resp.Data.Page.Total
		if total == 0 || processedArchives >= total {
			break
		}
		pageNum++
	}
	return index, seasonTitle, seasonCover, nil
}

func normalizeCover(cover string) string {
	if strings.HasPrefix(cover, "//") {
		return "https:" + cover
	}
	return cover
}

func (b *Bilibili) fetchView(bvid string) (*bilibiliViewResponse, error) {
	viewURL := fmt.Sprintf("https://api.bilibili.com/x/web-interface/view?bvid=%s", bvid)
	viewBody, err := utils.Get(viewURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Cookie", b.cookie))
	if err != nil {
		return nil, err
	}

	var viewResp bilibiliViewResponse
	if err := json.Unmarshal(viewBody, &viewResp); err != nil {
		return nil, err
	}
	return &viewResp, nil
}

func (b *Bilibili) fetchPageList(bvid string) ([]bilibiliPage, error) {
	pageURL := fmt.Sprintf("https://api.bilibili.com/x/player/pagelist?bvid=%s", bvid)
	body, err := utils.Get(pageURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
	if err != nil {
		return nil, err
	}

	var resp bilibiliPageListResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("bilibili pagelist api error: %d", resp.Code)
	}
	return resp.Data, nil
}

func (b *Bilibili) buildSongsFromPages(bvid, rootTitle, author, cover string, pages []bilibiliPage) []model.Song {
	var songs []model.Song
	cover = normalizeCover(cover)

	for i, page := range pages {
		displayTitle := page.Part
		if len(pages) == 1 && displayTitle == "" {
			displayTitle = rootTitle
		} else if displayTitle != rootTitle {
			displayTitle = fmt.Sprintf("%s - %s", rootTitle, displayTitle)
		}

		songs = append(songs, model.Song{
			Source:   "bilibili",
			ID:       fmt.Sprintf("%s|%d", bvid, page.CID),
			Name:     displayTitle,
			Artist:   author,
			Album:    bvid,
			Duration: page.Duration,
			Cover:    cover,
			Link:     fmt.Sprintf("https://www.bilibili.com/video/%s?p=%d", bvid, i+1),
			Extra: map[string]string{
				"bvid": bvid,
				"cid":  strconv.FormatInt(page.CID, 10),
			},
		})
	}
	return songs
}

// Search 搜索歌曲
func (b *Bilibili) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("search_type", "video")
	params.Set("keyword", keyword)
	params.Set("page", "1")
	params.Set("page_size", "20")

	searchURL := "https://api.bilibili.com/x/web-interface/search/type?" + params.Encode()
	body, err := utils.Get(searchURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
	if err != nil {
		return nil, err
	}

	var searchResp struct {
		Data struct {
			Result []struct {
				BVID   string `json:"bvid"`
				Title  string `json:"title"`
				Author string `json:"author"`
				Pic    string `json:"pic"`
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("bilibili search json error: %w", err)
	}

	var songs []model.Song
	for _, item := range searchResp.Data.Result {
		rootTitle := cleanTitle(item.Title)
		viewResp, err := b.fetchView(item.BVID)
		if err != nil || len(viewResp.Data.Pages) == 0 {
			continue
		}

		cover := normalizeCover(item.Pic)
		page := viewResp.Data.Pages[0]
		displayTitle := page.Part
		if displayTitle == "" {
			displayTitle = rootTitle
		} else if displayTitle != rootTitle {
			displayTitle = fmt.Sprintf("%s - %s", rootTitle, displayTitle)
		}

		songs = append(songs, model.Song{
			Source:   "bilibili",
			ID:       fmt.Sprintf("%s|%d", item.BVID, page.CID),
			Name:     displayTitle,
			Artist:   item.Author,
			Album:    item.BVID,
			Duration: page.Duration,
			Cover:    cover,
			Link:     fmt.Sprintf("https://www.bilibili.com/video/%s?p=1", item.BVID),
			Extra: map[string]string{
				"bvid": item.BVID,
				"cid":  strconv.FormatInt(page.CID, 10),
			},
		})
	}
	return songs, nil
}

// SearchPlaylist 搜索合集/分P
func (b *Bilibili) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("search_type", "video")
	params.Set("keyword", keyword)
	params.Set("page", "1")
	params.Set("page_size", "20")

	searchURL := "https://api.bilibili.com/x/web-interface/search/type?" + params.Encode()
	body, err := utils.Get(searchURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
	if err != nil {
		return nil, err
	}

	var searchResp struct {
		Data struct {
			Result []struct {
				BVID   string `json:"bvid"`
				Title  string `json:"title"`
				Author string `json:"author"`
				Pic    string `json:"pic"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &searchResp); err != nil {
		return nil, fmt.Errorf("bilibili search json error: %w", err)
	}

	plMap := make(map[string]bool)
	var playlists []model.Playlist
	for _, item := range searchResp.Data.Result {
		viewResp, err := b.fetchView(item.BVID)
		if err != nil {
			continue
		}

		cover := normalizeCover(item.Pic)
		if viewResp.Data.UgcSeason != nil {
			seasonID := viewResp.Data.UgcSeason.ID
			mid := viewResp.Data.Owner.Mid
			key := fmt.Sprintf("season:%d:%d:%s", seasonID, mid, item.BVID)
			if plMap[key] {
				continue
			}
			plMap[key] = true

			seasonName := viewResp.Data.UgcSeason.Title
			if seasonName == "" {
				seasonName = cleanTitle(item.Title)
			}
			seasonCover := viewResp.Data.UgcSeason.Cover
			if seasonCover == "" {
				seasonCover = cover
			}

			trackCount := countSeasonEpisodes(viewResp.Data.UgcSeason.Sections)
			if trackCount == 0 && seasonID != 0 && mid != 0 {
				seasonSongs, err := b.fetchSeasonSongs(mid, seasonID)
				if err == nil {
					trackCount = len(seasonSongs)
				}
			}

			playlists = append(playlists, model.Playlist{
				Source:      "bilibili",
				ID:          key,
				Name:        seasonName,
				Cover:       normalizeCover(seasonCover),
				TrackCount:  trackCount,
				PlayCount:   viewResp.Data.UgcSeason.Stat.View,
				Creator:     viewResp.Data.Owner.Name,
				Description: viewResp.Data.UgcSeason.Intro,
				Link:        fmt.Sprintf("https://www.bilibili.com/video/%s", item.BVID),
				Extra: map[string]string{
					"season_id": strconv.FormatInt(seasonID, 10),
					"mid":       strconv.FormatInt(mid, 10),
					"bvid":      item.BVID,
					"type":      "season",
				},
			})
			continue
		}

		if len(viewResp.Data.Pages) > 1 {
			plID := "bvid:" + item.BVID
			if plMap[plID] {
				continue
			}
			plMap[plID] = true
			playlists = append(playlists, model.Playlist{
				Source:     "bilibili",
				ID:         plID,
				Name:       cleanTitle(item.Title),
				Cover:      cover,
				TrackCount: len(viewResp.Data.Pages),
				Creator:    viewResp.Data.Owner.Name,
				Link:       fmt.Sprintf("https://www.bilibili.com/video/%s", item.BVID),
				Extra: map[string]string{
					"bvid": item.BVID,
					"type": "multipart",
				},
			})
		}
	}
	return playlists, nil
}

// GetPlaylistSongs 获取合集/分P所有歌曲
func (b *Bilibili) GetPlaylistSongs(id string) ([]model.Song, error) {
	if strings.HasPrefix(id, "season:") {
		parts := strings.Split(id, ":")
		if len(parts) < 3 {
			return nil, errors.New("invalid season id")
		}
		seasonID, _ := strconv.ParseInt(parts[1], 10, 64)
		mid, _ := strconv.ParseInt(parts[2], 10, 64)
		bvid := ""
		if len(parts) >= 4 {
			bvid = parts[3]
		}
		if bvid != "" {
			viewResp, err := b.fetchView(bvid)
			if err == nil && viewResp.Data.UgcSeason != nil {
				sections := viewResp.Data.UgcSeason.Sections
				archiveIndex := map[string]bilibiliSeasonArchiveMeta{}
				seasonTitle := viewResp.Data.UgcSeason.Title
				seasonCover := viewResp.Data.UgcSeason.Cover
				idx, sTitle, sCover, idxErr := b.fetchSeasonArchiveIndex(mid, seasonID)
				if idxErr == nil {
					archiveIndex = idx
					if seasonTitle == "" {
						seasonTitle = sTitle
					}
					if seasonCover == "" {
						seasonCover = sCover
					}
				}
				songs := b.buildSongsFromSeasonSections(sections, seasonTitle, seasonCover, viewResp.Data.Owner.Name, archiveIndex)
				if len(songs) > 0 {
					return songs, nil
				}
			}
		}
		return b.fetchSeasonSongs(mid, seasonID)
	}

	bvid := strings.TrimPrefix(id, "bvid:")
	if bvid == "" {
		return nil, errors.New("invalid playlist id")
	}
	viewResp, err := b.fetchView(bvid)
	if err != nil {
		return nil, err
	}
	rootTitle := viewResp.Data.Title
	pages := viewResp.Data.Pages
	if len(pages) <= 1 {
		if pageList, err := b.fetchPageList(bvid); err == nil && len(pageList) > 0 {
			pages = pageList
		}
	}
	if len(pages) == 0 {
		return nil, errors.New("no video pages found")
	}
	return b.buildSongsFromPages(bvid, rootTitle, viewResp.Data.Owner.Name, viewResp.Data.Pic, pages), nil
}

// ParsePlaylist 解析合集/分P链接
func (b *Bilibili) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	bvidRe := regexp.MustCompile(`(BV\w+)`)
	bvidMatches := bvidRe.FindStringSubmatch(link)
	if len(bvidMatches) >= 2 {
		bvid := bvidMatches[1]
		viewResp, err := b.fetchView(bvid)
		if err != nil {
			return nil, nil, err
		}
		if viewResp.Data.UgcSeason != nil {
			seasonID := viewResp.Data.UgcSeason.ID
			mid := viewResp.Data.Owner.Mid
			trackCount := countSeasonEpisodes(viewResp.Data.UgcSeason.Sections)
			playlist := &model.Playlist{
				Source:      "bilibili",
				ID:          fmt.Sprintf("season:%d:%d:%s", seasonID, mid, bvid),
				Name:        viewResp.Data.UgcSeason.Title,
				Cover:       normalizeCover(viewResp.Data.UgcSeason.Cover),
				TrackCount:  trackCount,
				PlayCount:   viewResp.Data.UgcSeason.Stat.View,
				Creator:     viewResp.Data.Owner.Name,
				Description: viewResp.Data.UgcSeason.Intro,
				Link:        fmt.Sprintf("https://www.bilibili.com/video/%s", bvid),
				Extra: map[string]string{
					"season_id": strconv.FormatInt(seasonID, 10),
					"mid":       strconv.FormatInt(mid, 10),
					"bvid":      bvid,
					"type":      "season",
				},
			}
			sections := viewResp.Data.UgcSeason.Sections
			archiveIndex := map[string]bilibiliSeasonArchiveMeta{}
			seasonTitle := viewResp.Data.UgcSeason.Title
			seasonCover := viewResp.Data.UgcSeason.Cover
			idx, sTitle, sCover, idxErr := b.fetchSeasonArchiveIndex(mid, seasonID)
			if idxErr == nil {
				archiveIndex = idx
				if seasonTitle == "" {
					seasonTitle = sTitle
				}
				if seasonCover == "" {
					seasonCover = sCover
				}
				if trackCount == 0 {
					trackCount = len(archiveIndex)
					playlist.TrackCount = trackCount
				}
			}
			songs := b.buildSongsFromSeasonSections(sections, seasonTitle, seasonCover, viewResp.Data.Owner.Name, archiveIndex)
			if len(songs) > 0 {
				return playlist, songs, nil
			}
			songs, err := b.fetchSeasonSongs(mid, seasonID)
			return playlist, songs, err
		}

		if len(viewResp.Data.Pages) > 1 {
			playlist := &model.Playlist{
				Source:     "bilibili",
				ID:         "bvid:" + bvid,
				Name:       viewResp.Data.Title,
				Cover:      normalizeCover(viewResp.Data.Pic),
				TrackCount: len(viewResp.Data.Pages),
				Creator:    viewResp.Data.Owner.Name,
				Link:       fmt.Sprintf("https://www.bilibili.com/video/%s", bvid),
				Extra: map[string]string{
					"bvid": bvid,
					"type": "multipart",
				},
			}
			songs := b.buildSongsFromPages(bvid, viewResp.Data.Title, viewResp.Data.Owner.Name, viewResp.Data.Pic, viewResp.Data.Pages)
			return playlist, songs, nil
		}

		if len(viewResp.Data.Pages) == 1 {
			playlist := &model.Playlist{
				Source:     "bilibili",
				ID:         "bvid:" + bvid,
				Name:       viewResp.Data.Title,
				Cover:      normalizeCover(viewResp.Data.Pic),
				TrackCount: 1,
				Creator:    viewResp.Data.Owner.Name,
				Link:       fmt.Sprintf("https://www.bilibili.com/video/%s", bvid),
				Extra: map[string]string{
					"bvid": bvid,
					"type": "single",
				},
			}
			songs := b.buildSongsFromPages(bvid, viewResp.Data.Title, viewResp.Data.Owner.Name, viewResp.Data.Pic, viewResp.Data.Pages)
			return playlist, songs, nil
		}
	}

	return nil, nil, errors.New("invalid bilibili playlist link")
}

func (b *Bilibili) fetchSeasonSongs(mid, seasonID int64) ([]model.Song, error) {
	if seasonID == 0 || mid == 0 {
		return nil, errors.New("invalid season info")
	}

	var allSongs []model.Song
	processedArchives := 0
	var seasonTitle string
	var seasonCover string
	pageNum := 1
	pageSize := 30
	for {
		apiURL := fmt.Sprintf("https://api.bilibili.com/x/space/ugc/season?mid=%d&season_id=%d&page_num=%d&page_size=%d", mid, seasonID, pageNum, pageSize)
		body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
		if err != nil {
			return nil, err
		}

		var resp bilibiliSeasonResp
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		if resp.Code != 0 {
			return nil, fmt.Errorf("bilibili season api error: %d", resp.Code)
		}
		if seasonTitle == "" {
			seasonTitle = resp.Data.Season.Title
		}
		if seasonCover == "" {
			seasonCover = resp.Data.Season.Cover
		}
		if len(resp.Data.Archives) == 0 {
			break
		}

		for _, arc := range resp.Data.Archives {
			if arc.CID != 0 {
				cover := arc.Cover
				if cover == "" {
					cover = seasonCover
				}
				allSongs = append(allSongs, model.Song{
					Source:   "bilibili",
					ID:       fmt.Sprintf("%s|%d", arc.BVID, arc.CID),
					Name:     arc.Title,
					Artist:   "",
					Album:    seasonTitle,
					Duration: arc.Duration,
					Cover:    normalizeCover(cover),
					Link:     fmt.Sprintf("https://www.bilibili.com/video/%s", arc.BVID),
					Extra: map[string]string{
						"bvid": arc.BVID,
						"cid":  strconv.FormatInt(arc.CID, 10),
					},
				})
				continue
			}
		}
		processedArchives += len(resp.Data.Archives)

		total := resp.Data.Page.Total
		if total == 0 || processedArchives >= total {
			break
		}
		pageNum++
	}
	return allSongs, nil
}

// Parse 解析链接并获取完整信息（包括下载链接）
func (b *Bilibili) Parse(link string) (*model.Song, error) {
	// 1. 提取 BVID
	bvidRe := regexp.MustCompile(`(BV\w+)`)
	bvidMatches := bvidRe.FindStringSubmatch(link)
	if len(bvidMatches) < 2 {
		return nil, errors.New("invalid bilibili link: bvid not found")
	}
	bvid := bvidMatches[1]

	// 2. 提取 Page (p=X), 默认为 1
	page := 1
	pageRe := regexp.MustCompile(`[?&]p=(\d+)`)
	pageMatches := pageRe.FindStringSubmatch(link)
	if len(pageMatches) >= 2 {
		if p, err := strconv.Atoi(pageMatches[1]); err == nil && p > 0 {
			page = p
		}
	}

	// 3. 调用 View 接口获取元数据
	viewResp, err := b.fetchView(bvid)
	if err != nil {
		return nil, err
	}
	if viewResp.Data.UgcSeason != nil || len(viewResp.Data.Pages) > 1 {
		return nil, errors.New("playlist link detected")
	}
	if len(viewResp.Data.Pages) <= 1 {
		if pages, err := b.fetchPageList(bvid); err == nil && len(pages) > 1 {
			return nil, errors.New("playlist link detected")
		}
	}
	if len(viewResp.Data.Pages) == 0 {
		return nil, errors.New("no video pages found")
	}

	if page > len(viewResp.Data.Pages) {
		page = 1
	}
	targetPage := viewResp.Data.Pages[page-1]

	displayTitle := targetPage.Part
	if len(viewResp.Data.Pages) == 1 && displayTitle == "" {
		displayTitle = viewResp.Data.Title
	} else if displayTitle != viewResp.Data.Title {
		displayTitle = fmt.Sprintf("%s - %s", viewResp.Data.Title, displayTitle)
	}

	cover := viewResp.Data.Pic
	if strings.HasPrefix(cover, "//") {
		cover = "https:" + cover
	}

	cidStr := strconv.FormatInt(targetPage.CID, 10)

	// 4. 立即获取下载链接
	isVip, _ := b.IsVipAccount()
	audioURL, _ := b.fetchAudioURL(bvid, cidStr, isVip) // 忽略错误，尽可能返回元数据

	return &model.Song{
		Source:   "bilibili",
		ID:       fmt.Sprintf("%s|%d", viewResp.Data.BVID, targetPage.CID),
		Name:     displayTitle,
		Artist:   viewResp.Data.Owner.Name,
		Album:    viewResp.Data.BVID,
		Duration: targetPage.Duration,
		Cover:    cover,
		URL:      audioURL, // 已填充
		Link:     fmt.Sprintf("https://www.bilibili.com/video/%s?p=%d", viewResp.Data.BVID, page),
		Extra: map[string]string{
			"bvid": viewResp.Data.BVID,
			"cid":  cidStr,
		},
	}, nil
}

// GetDownloadURL 获取下载链接
func (b *Bilibili) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "bilibili" {
		return "", errors.New("source mismatch")
	}

	if s.URL != "" {
		return s.URL, nil
	}

	var bvid, cid string
	if s.Extra != nil {
		bvid = s.Extra["bvid"]
		cid = s.Extra["cid"]
	}

	if bvid == "" || cid == "" {
		parts := strings.Split(s.ID, "|")
		if len(parts) == 2 {
			bvid = parts[0]
			cid = parts[1]
		} else {
			return "", errors.New("invalid id structure")
		}
	}

	isVip, _ := b.IsVipAccount()
	return b.fetchAudioURL(bvid, cid, isVip)
}

// fetchAudioURL 内部逻辑提取
func (b *Bilibili) fetchAudioURL(bvid, cid string, isVip bool) (string, error) {
	fnval := 80
	if isVip {
		// 4048 allows requesting FLAC/Hi-Res/Dolby formats instead of default standard 80
		fnval = 4048
	}

	apiURL := fmt.Sprintf("https://api.bilibili.com/x/player/playurl?fnval=%d&qn=127&bvid=%s&cid=%s", fnval, bvid, cid)
	body, err := utils.Get(apiURL, utils.WithHeader("User-Agent", UserAgent), utils.WithHeader("Referer", Referer), utils.WithHeader("Cookie", b.cookie))
	if err != nil {
		return "", err
	}

	var resp struct {
		Data struct {
			Durl []struct {
				URL string `json:"url"`
			} `json:"durl"`
			Dash struct {
				Audio []struct {
					ID      int    `json:"id"`
					BaseURL string `json:"baseUrl"`
				} `json:"audio"`
				Flac struct {
					Audio struct {
						ID      int    `json:"id"`
						BaseURL string `json:"baseUrl"`
					} `json:"audio"`
				} `json:"flac"`
				Dolby struct {
					Audio []struct {
						ID      int    `json:"id"`
						BaseURL string `json:"baseUrl"`
					} `json:"audio"`
				} `json:"dolby"`
			} `json:"dash"`
		} `json:"data"`
	}
	json.Unmarshal(body, &resp)

	// 1. Highest Priority: Hi-Res FLAC format specifically marked by id 30251
	if resp.Data.Dash.Flac.Audio.ID == 30251 && resp.Data.Dash.Flac.Audio.BaseURL != "" {
		return resp.Data.Dash.Flac.Audio.BaseURL, nil
	}

	// 2. Secondary Priority: Dolby Atmos format specifically marked by id 30250
	for _, a := range resp.Data.Dash.Dolby.Audio {
		if a.ID == 30250 && a.BaseURL != "" {
			return a.BaseURL, nil
		}
	}

	// 3. Loop through standard DASH audio evaluating the highest ID quality natively (e.g. 30280 for 192kbps vs 30232 for 132kbps)
	var bestURL string
	var highestID = -1
	for _, a := range resp.Data.Dash.Audio {
		if a.ID > highestID && a.BaseURL != "" {
			highestID = a.ID
			bestURL = a.BaseURL
		}
	}
	if bestURL != "" {
		return bestURL, nil
	}

	// 4. Fallback to generic unsegmented DURL format if no Dash
	if len(resp.Data.Durl) > 0 {
		return resp.Data.Durl[0].URL, nil
	}

	return "", errors.New("no audio found")
}

func (b *Bilibili) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "bilibili" {
		return "", errors.New("source mismatch")
	}
	return "", nil
}
