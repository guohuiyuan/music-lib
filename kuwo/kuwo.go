package kuwo

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"math/rand"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
)

type Kuwo struct {
	cookie string
}

func New(cookie string) *Kuwo { return &Kuwo{cookie: cookie} }

var defaultKuwo = New("")

func Search(keyword string) ([]model.Song, error) { return defaultKuwo.Search(keyword) }
func SearchAlbum(keyword string) ([]model.Playlist, error) {
	return defaultKuwo.SearchAlbum(keyword)
}
func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultKuwo.SearchPlaylist(keyword)
}
func GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := defaultKuwo.fetchAlbumDetail(id)
	return songs, err
}
func ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	return defaultKuwo.ParseAlbum(link)
}
func GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := defaultKuwo.fetchPlaylistDetail(id)
	return songs, err
}
func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultKuwo.ParsePlaylist(link)
}
func GetDownloadURL(s *model.Song) (string, error) { return defaultKuwo.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error)      { return defaultKuwo.GetLyrics(s) }
func Parse(link string) (*model.Song, error)       { return defaultKuwo.Parse(link) }

// GetRecommendedPlaylists 获取推荐歌单 (新增)
func GetRecommendedPlaylists() ([]model.Playlist, error) {
	return defaultKuwo.GetRecommendedPlaylists()
}

// Search 搜索歌曲
func (k *Kuwo) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("vipver", "1")
	params.Set("client", "kt")
	params.Set("ft", "music")
	params.Set("cluster", "0")
	params.Set("strategy", "2012")
	params.Set("encoding", "utf8")
	params.Set("rformat", "json")
	params.Set("mobi", "1")
	params.Set("issubtitle", "1")
	params.Set("show_copyright_off", "1")
	params.Set("pn", "0")
	params.Set("rn", "10")
	params.Set("all", keyword)

	apiURL := "http://www.kuwo.cn/search/searchMusicBykeyWord?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		AbsList []struct {
			MusicRID  string `json:"MUSICRID"`
			SongName  string `json:"SONGNAME"`
			Artist    string `json:"ARTIST"`
			Album     string `json:"ALBUM"`
			Duration  string `json:"DURATION"`
			HtsMVPic  string `json:"hts_MVPIC"`
			MInfo     string `json:"MINFO"`
			PayInfo   string `json:"PAY"`
			BitSwitch int    `json:"bitSwitch"`
		} `json:"abslist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("kuwo json parse error: %w", err)
	}

	var songs []model.Song
	for _, item := range resp.AbsList {
		if item.BitSwitch == 0 {
			continue
		}

		cleanID := strings.TrimPrefix(item.MusicRID, "MUSIC_")
		duration, _ := strconv.Atoi(item.Duration)
		size := parseSizeFromMInfo(item.MInfo)
		bitrate := parseBitrateFromMInfo(item.MInfo)

		songs = append(songs, model.Song{
			Source:   "kuwo",
			ID:       cleanID,
			Name:     item.SongName,
			Artist:   item.Artist,
			Album:    item.Album,
			Duration: duration,
			Size:     size,
			Bitrate:  bitrate,
			Cover:    item.HtsMVPic,
			Link:     fmt.Sprintf("http://www.kuwo.cn/play_detail/%s", cleanID),
			Extra: map[string]string{
				"rid": cleanID,
			},
		})
	}

	return songs, nil
}

// 酷我的歌单和专辑搜索共用同一个 legacy 路由，仅通过 ft 参数区分类型。
func (k *Kuwo) searchCollection(keyword, ft string, out interface{}) error {
	params := url.Values{}
	params.Set("all", keyword)
	params.Set("ft", ft)
	params.Set("itemset", "web_2013")
	params.Set("client", "kt")
	params.Set("pcmp4", "1")
	params.Set("geo", "c")
	params.Set("vipver", "1")
	params.Set("pn", "0")
	params.Set("rn", "10")
	params.Set("rformat", "json")
	params.Set("encoding", "utf8")

	apiURL := "http://search.kuwo.cn/r.s?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return err
	}

	if err := parseKuwoLegacyJSON(body, out); err != nil {
		return fmt.Errorf("kuwo %s json parse error: %w", ft, err)
	}

	return nil
}

// SearchAlbum 搜索专辑
func (k *Kuwo) SearchAlbum(keyword string) ([]model.Playlist, error) {
	var resp struct {
		AlbumList []struct {
			AlbumID  string `json:"albumid"`
			ID       string `json:"id"`
			Name     string `json:"name"`
			Artist   string `json:"artist"`
			AArtist  string `json:"aartist"`
			HtsImg   string `json:"hts_img"`
			Img      string `json:"img"`
			MusicCnt string `json:"musiccnt"`
			Info     string `json:"info"`
			Company  string `json:"company"`
			Pub      string `json:"pub"`
			PlayCnt  string `json:"PLAYCNT"`
		} `json:"albumlist"`
	}

	if err := k.searchCollection(keyword, "album", &resp); err != nil {
		return nil, err
	}

	albums := make([]model.Playlist, 0, len(resp.AlbumList))
	for _, item := range resp.AlbumList {
		albumID := firstNonEmpty(item.AlbumID, item.ID)
		if albumID == "" {
			continue
		}

		albums = append(albums, model.Playlist{
			Source:      "kuwo",
			ID:          albumID,
			Name:        normalizeKuwoText(item.Name),
			Cover:       normalizeKuwoImageURL(firstNonEmpty(item.HtsImg, item.Img)),
			TrackCount:  parseKuwoStringInt(item.MusicCnt),
			PlayCount:   parseKuwoStringInt(item.PlayCnt),
			Creator:     normalizeKuwoText(firstNonEmpty(item.AArtist, item.Artist)),
			Description: normalizeKuwoText(item.Info),
			Link:        fmt.Sprintf("http://www.kuwo.cn/album_detail/%s", albumID),
			Extra: map[string]string{
				"type":         "album",
				"album_id":     albumID,
				"company":      normalizeKuwoText(item.Company),
				"publish_time": strings.TrimSpace(item.Pub),
			},
		})
	}

	if len(albums) == 0 {
		return nil, errors.New("no albums found")
	}

	return albums, nil
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

// GetPlaylistSongs 获取歌单详情（解析歌曲列表）
func (k *Kuwo) GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := k.fetchAlbumDetail(id)
	return songs, err
}

func (k *Kuwo) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`album_detail/(\d+)`),
		regexp.MustCompile(`album/(\d+)`),
		regexp.MustCompile(`albumid=(\d+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(link)
		if len(matches) >= 2 {
			return k.fetchAlbumDetail(matches[1])
		}
	}

	return nil, nil, errors.New("invalid kuwo album link")
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

// fetchPlaylistDetail [内部复用] 获取歌单详情 (Metadata + Songs)
func (k *Kuwo) fetchPlaylistDetail(id string) (*model.Playlist, []model.Song, error) {
	params := url.Values{}
	params.Set("op", "getlistinfo")
	params.Set("pid", id)
	params.Set("pn", "0")
	params.Set("rn", "100")
	params.Set("encode", "utf8")
	params.Set("keyset", "pl2012")
	params.Set("identity", "kuwo")
	params.Set("pcmp4", "1")
	params.Set("vipver", "1")
	params.Set("newver", "1")

	apiURL := "http://nplserver.kuwo.cn/pl.svc?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		MusicList []struct {
			Id         string      `json:"id"`
			Name       string      `json:"name"`
			Artist     string      `json:"artist"`
			Album      string      `json:"album"`
			AlbumPic   string      `json:"albumpic"`
			Duration   interface{} `json:"duration"`
			SongName   string      `json:"song_name"`
			ArtistName string      `json:"artist_name"`
		} `json:"musiclist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("kuwo playlist detail json error: %w", err)
	}

	if len(resp.MusicList) == 0 {
		return nil, nil, errors.New("playlist is empty or id is invalid")
	}

	playlist := &model.Playlist{
		Source:     "kuwo",
		ID:         id,
		Link:       fmt.Sprintf("http://www.kuwo.cn/playlist_detail/%s", id),
		TrackCount: len(resp.MusicList),
	}

	var songs []model.Song
	for _, item := range resp.MusicList {
		name := item.Name
		if name == "" {
			name = item.SongName
		}
		artist := item.Artist
		if artist == "" {
			artist = item.ArtistName
		}

		var duration int
		switch v := item.Duration.(type) {
		case string:
			d, _ := strconv.Atoi(v)
			duration = d
		case float64:
			duration = int(v)
		}

		cover := item.AlbumPic
		if cover != "" {
			if !strings.HasPrefix(cover, "http") {
				cover = "http://" + cover
			}
			if strings.Contains(cover, "_100.") {
				cover = strings.Replace(cover, "_100.", "_500.", 1)
			} else if strings.Contains(cover, "_150.") {
				cover = strings.Replace(cover, "_150.", "_500.", 1)
			} else if strings.Contains(cover, "_120.") {
				cover = strings.Replace(cover, "_120.", "_500.", 1)
			}
		}

		songs = append(songs, model.Song{
			Source:   "kuwo",
			ID:       item.Id,
			Name:     name,
			Artist:   artist,
			Album:    item.Album,
			Duration: duration,
			Cover:    cover,
			Link:     fmt.Sprintf("http://www.kuwo.cn/play_detail/%s", item.Id),
			Extra: map[string]string{
				"rid": item.Id,
			},
		})
	}
	return playlist, songs, nil
}

// Parse 解析链接并获取完整信息
func (k *Kuwo) fetchAlbumDetail(id string) (*model.Playlist, []model.Song, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil, errors.New("album id is empty")
	}

	album, songs, err := k.fetchAlbumDetailFromLegacyAPI(id)
	if err == nil && len(songs) > 0 {
		return album, songs, nil
	}

	pageAlbum, pageSongs, pageErr := k.fetchAlbumDetailFromPage(id)
	if pageErr == nil && len(pageSongs) > 0 {
		return pageAlbum, pageSongs, nil
	}

	if err != nil && pageErr != nil {
		return nil, nil, fmt.Errorf("kuwo album detail failed: legacy api: %v; page parse: %w", err, pageErr)
	}
	if err != nil {
		return nil, nil, err
	}
	if pageErr != nil {
		return nil, nil, pageErr
	}

	return album, songs, nil
}

func (k *Kuwo) fetchAlbumDetailFromLegacyAPI(id string) (*model.Playlist, []model.Song, error) {
	const pageSize = 100

	var album *model.Playlist
	totalSongs := 0
	seen := make(map[string]struct{})
	songs := make([]model.Song, 0, pageSize)

	for page := 0; ; page++ {
		params := url.Values{}
		params.Set("pn", strconv.Itoa(page))
		params.Set("rn", strconv.Itoa(pageSize))
		params.Set("stype", "albuminfo")
		params.Set("albumid", id)
		params.Set("sortby", "0")
		params.Set("alflac", "1")
		params.Set("show_copyright_off", "1")
		params.Set("pcmp4", "1")
		params.Set("encoding", "utf8")

		apiURL := "http://search.kuwo.cn/r.s?" + params.Encode()

		body, err := utils.Get(apiURL,
			utils.WithHeader("User-Agent", UserAgent),
			utils.WithHeader("Cookie", k.cookie),
			utils.WithRandomIPHeader(),
		)
		if err != nil {
			return nil, nil, err
		}

		var resp map[string]interface{}
		if err := parseKuwoLegacyJSON(body, &resp); err != nil {
			return nil, nil, fmt.Errorf("kuwo album detail json error: %w", err)
		}

		if album == nil {
			albumID := firstNonEmpty(parseKuwoAnyString(resp["albumid"]), parseKuwoAnyString(resp["id"]), id)
			totalSongs = parseKuwoAnyInt(resp["songnum"])
			album = &model.Playlist{
				Source:      "kuwo",
				ID:          albumID,
				Name:        normalizeKuwoText(parseKuwoAnyString(resp["name"])),
				Cover:       normalizeKuwoImageURL(firstNonEmpty(parseKuwoAnyString(resp["hts_img"]), parseKuwoAnyString(resp["img"]))),
				TrackCount:  totalSongs,
				Creator:     normalizeKuwoText(firstNonEmpty(parseKuwoAnyString(resp["aartist"]), parseKuwoAnyString(resp["artist"]))),
				Description: normalizeKuwoText(parseKuwoAnyString(resp["info"])),
				Link:        fmt.Sprintf("http://www.kuwo.cn/album_detail/%s", albumID),
				Extra: map[string]string{
					"type":         "album",
					"album_id":     albumID,
					"company":      normalizeKuwoText(parseKuwoAnyString(resp["company"])),
					"publish_time": strings.TrimSpace(parseKuwoAnyString(resp["pub"])),
					"lang":         normalizeKuwoText(parseKuwoAnyString(resp["lang"])),
				},
			}
		}

		musicList := parseKuwoAnySlice(resp["musiclist"])
		if len(musicList) == 0 {
			if page == 0 {
				return nil, nil, fmt.Errorf("album %s detail api returned empty musiclist", id)
			}
			break
		}

		for _, rawItem := range musicList {
			item, ok := rawItem.(map[string]interface{})
			if !ok {
				continue
			}

			rid := firstNonEmpty(parseKuwoAnyString(item["id"]), parseKuwoAnyString(item["musicrid"]))
			if rid == "" {
				continue
			}
			if _, ok := seen[rid]; ok {
				continue
			}
			seen[rid] = struct{}{}

			songCover := normalizeKuwoImageURL(firstNonEmpty(parseKuwoAnyString(item["pic120"]), parseKuwoAnyString(item["web_albumpic_short"])))
			if songCover == "" && album != nil {
				songCover = album.Cover
			}

			song := model.Song{
				Source:   "kuwo",
				ID:       rid,
				Name:     normalizeKuwoText(firstNonEmpty(parseKuwoAnyString(item["name"]), parseKuwoAnyString(item["songname"]))),
				Artist:   normalizeKuwoText(firstNonEmpty(parseKuwoAnyString(item["aartist"]), parseKuwoAnyString(item["artist"]))),
				Album:    normalizeKuwoText(firstNonEmpty(parseKuwoAnyString(item["album"]), album.Name)),
				Duration: parseKuwoAnyInt(item["duration"]),
				Size:     parseSizeFromMInfo(parseKuwoAnyString(item["MINFO"])),
				Bitrate:  parseBitrateFromMInfo(parseKuwoAnyString(item["MINFO"])),
				Cover:    songCover,
				Link:     fmt.Sprintf("http://www.kuwo.cn/play_detail/%s", rid),
				Extra: map[string]string{
					"rid": rid,
				},
			}

			if album != nil {
				song.AlbumID = album.ID
				song.Extra["album_id"] = album.ID
			}
			if track := strings.TrimSpace(parseKuwoAnyString(item["track"])); track != "" {
				song.Extra["track"] = track
			}
			if subtitle := normalizeKuwoText(parseKuwoAnyString(item["subtitle"])); subtitle != "" {
				song.Extra["subtitle"] = subtitle
			}
			if bitSwitch := parseKuwoAnyInt(item["bitSwitch"]); bitSwitch > 0 {
				song.Extra["bit_switch"] = strconv.Itoa(bitSwitch)
			}

			songs = append(songs, song)
		}

		if len(musicList) < pageSize {
			break
		}
		if totalSongs > 0 && len(songs) >= totalSongs {
			break
		}
	}

	if album == nil {
		return nil, nil, errors.New("album not found")
	}
	if album.TrackCount == 0 {
		album.TrackCount = len(songs)
	}

	return album, songs, nil
}

func (k *Kuwo) fetchAlbumDetailFromPage(id string) (*model.Playlist, []model.Song, error) {
	pageURL := fmt.Sprintf("https://www.kuwo.cn/album_detail/%s", id)

	body, err := utils.Get(pageURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", k.cookie),
	)
	if err != nil {
		return nil, nil, err
	}

	htmlBody := string(body)
	albumName, artistName := parseKuwoAlbumPageTitle(findKuwoSubmatch(htmlBody, `(?is)<title>(.*?)</title>`))
	if albumName == "" {
		albumName = normalizeKuwoText(stripKuwoHTMLTags(findKuwoSubmatch(htmlBody, `(?is)<p class="song_name"[^>]*>(.*?)</p>`)))
	}
	if artistName == "" {
		artistName = normalizeKuwoText(stripKuwoHTMLTags(findKuwoSubmatch(htmlBody, `(?is)<p class="artist_name"[^>]*>(.*?)</p>`)))
	}

	infoBlock := findKuwoSubmatch(htmlBody, `(?is)<p class="song_info"[^>]*>(.*?)</p>`)
	infoTips := findKuwoSubmatches(infoBlock, `(?is)<span class="tip"[^>]*>(.*?)</span>`)
	lang := ""
	publishTime := ""
	if len(infoTips) > 0 {
		lang = normalizeKuwoText(stripKuwoHTMLTags(infoTips[0]))
	}
	if len(infoTips) > 1 {
		publishTime = normalizeKuwoText(stripKuwoHTMLTags(infoTips[1]))
	}

	description := normalizeKuwoText(stripKuwoHTMLTags(findKuwoSubmatch(htmlBody, `(?is)<p class="intr_txt"[^>]*>.*?<span[^>]*>(.*?)</span>`)))
	cover := normalizeKuwoImageURL(decodeKuwoEscapedString(findKuwoSubmatch(htmlBody, `hts_img:"([^"]+)"`)))
	if cover == "" {
		cover = normalizeKuwoImageURL(decodeKuwoEscapedString(findKuwoSubmatch(htmlBody, `img:"([^"]*albumcover[^"]+)"`)))
	}

	company := normalizeKuwoText(decodeKuwoEscapedString(findKuwoSubmatch(htmlBody, `company:"([^"]+)"`)))
	songBlocks := regexp.MustCompile(`(?is)<li class="song_item[^"]*"[^>]*>.*?</li>`).FindAllString(htmlBody, -1)
	if len(songBlocks) == 0 {
		return nil, nil, errors.New("album page returned no songs")
	}

	album := &model.Playlist{
		Source:      "kuwo",
		ID:          id,
		Name:        albumName,
		Cover:       cover,
		TrackCount:  len(songBlocks),
		Creator:     artistName,
		Description: description,
		Link:        pageURL,
		Extra: map[string]string{
			"type":         "album",
			"album_id":     id,
			"company":      company,
			"publish_time": publishTime,
			"lang":         lang,
		},
	}

	songs := make([]model.Song, 0, len(songBlocks))
	seen := make(map[string]struct{}, len(songBlocks))
	for _, block := range songBlocks {
		rid := findKuwoSubmatch(block, `href="/play_detail/(\d+)"`)
		if rid == "" {
			continue
		}
		if _, ok := seen[rid]; ok {
			continue
		}
		seen[rid] = struct{}{}

		name := normalizeKuwoText(stripKuwoHTMLTags(firstNonEmpty(
			findKuwoSubmatch(block, `title="([^"]+)"`),
			findKuwoSubmatch(block, `(?is)<a[^>]*class="name"[^>]*>(.*?)</a>`),
		)))
		artist := normalizeKuwoText(stripKuwoHTMLTags(firstNonEmpty(
			findKuwoSubmatch(block, `(?is)<div class="song_artist"[^>]*>.*?<span[^>]*title="([^"]+)"`),
			findKuwoSubmatch(block, `(?is)<div class="song_artist"[^>]*>.*?<span[^>]*>(.*?)</span>`),
			artistName,
		)))
		track := firstNonEmpty(
			findKuwoSubmatch(block, `(?is)<div class="rank_num"[^>]*>.*?<span style="display:;?"[^>]*>\s*(\d+)\s*</span>`),
			findKuwoSubmatch(block, `(?is)<div class="rank_num"[^>]*>.*?<span[^>]*>\s*(\d+)\s*</span>`),
		)

		song := model.Song{
			Source:   "kuwo",
			ID:       rid,
			Name:     name,
			Artist:   artist,
			Album:    album.Name,
			AlbumID:  album.ID,
			Cover:    album.Cover,
			Link:     fmt.Sprintf("http://www.kuwo.cn/play_detail/%s", rid),
			Duration: 0,
			Extra: map[string]string{
				"rid":      rid,
				"album_id": album.ID,
			},
		}
		if track != "" {
			song.Extra["track"] = track
		}

		songs = append(songs, song)
	}

	if len(songs) == 0 {
		return nil, nil, errors.New("album page parsed zero songs")
	}
	album.TrackCount = len(songs)

	return album, songs, nil
}

func (k *Kuwo) Parse(link string) (*model.Song, error) {
	re := regexp.MustCompile(`play_detail/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, errors.New("invalid kuwo link, rid not found")
	}
	rid := matches[1]

	return k.fetchFullSongInfo(rid)
}

// GetDownloadURL 获取下载链接
func (k *Kuwo) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "kuwo" {
		return "", errors.New("source mismatch")
	}
	if s.URL != "" {
		return s.URL, nil
	}

	rid := s.ID
	if s.Extra != nil && s.Extra["rid"] != "" {
		rid = s.Extra["rid"]
	}

	return k.fetchAudioURL(rid)
}

// fetchFullSongInfo 内部聚合：同时获取元数据和下载链接
func (k *Kuwo) fetchFullSongInfo(rid string) (*model.Song, error) {
	params := url.Values{}
	params.Set("musicId", rid)
	params.Set("httpsStatus", "1")
	metaURL := "http://m.kuwo.cn/newh5/singles/songinfoandlrc?" + params.Encode()

	var name, artist, cover string
	metaBody, err := utils.Get(metaURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)

	if err == nil {
		var metaResp struct {
			Data struct {
				SongInfo struct {
					SongName string `json:"songName"`
					Artist   string `json:"artist"`
					Pic      string `json:"pic"`
				} `json:"songinfo"`
			} `json:"data"`
		}
		if json.Unmarshal(metaBody, &metaResp) == nil {
			name = metaResp.Data.SongInfo.SongName
			artist = metaResp.Data.SongInfo.Artist
			cover = metaResp.Data.SongInfo.Pic
		}
	}

	if name == "" {
		name = fmt.Sprintf("Kuwo_Song_%s", rid)
	}

	audioURL, err := k.fetchAudioURL(rid)
	if err != nil {
		return nil, err
	}

	return &model.Song{
		Source: "kuwo",
		ID:     rid,
		Name:   name,
		Artist: artist,
		Cover:  cover,
		URL:    audioURL,
		Link:   fmt.Sprintf("http://www.kuwo.cn/play_detail/%s", rid),
		Extra: map[string]string{
			"rid": rid,
		},
	}, nil
}

// fetchAudioURL 内部核心：仅获取下载链接
func (k *Kuwo) fetchAudioURL(rid string) (string, error) {
	qualities := []string{"128kmp3", "320kmp3", "flac", "2000kflac"}
	randomID := fmt.Sprintf("C_APK_guanwang_%d%d", time.Now().UnixNano(), rand.Intn(1000000))

	for _, br := range qualities {
		params := url.Values{}
		params.Set("f", "web")
		params.Set("source", "kwplayercar_ar_6.0.0.9_B_jiakong_vh.apk")
		params.Set("from", "PC")
		params.Set("type", "convert_url_with_sign")
		params.Set("br", br)
		params.Set("rid", rid)
		params.Set("user", randomID)

		apiURL := "https://mobi.kuwo.cn/mobi.s?" + params.Encode()

		body, err := utils.Get(apiURL,
			utils.WithHeader("User-Agent", UserAgent),
			utils.WithHeader("Cookie", k.cookie),
			utils.WithRandomIPHeader(),
		)
		if err != nil {
			continue
		}

		var resp struct {
			Data struct {
				URL     string `json:"url"`
				Bitrate int    `json:"bitrate"`
				Format  string `json:"format"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}
		if resp.Data.URL != "" {
			return resp.Data.URL, nil
		}
	}

	// [降级策略] 尝试使用 www.kuwo.cn 的备用接口绕过防盗链
	fallbackURL := fmt.Sprintf("http://www.kuwo.cn/api/v1/www/music/playUrl?mid=%s&type=music&httpsStatus=1", rid)

	// 需要伪造 Secret 头部 (简单绕过)
	secret := "kuwo_web_secret"
	cookieWithSecret := k.cookie
	if !strings.Contains(cookieWithSecret, "kw_token") {
		cookieWithSecret += "; kw_token=secret_token"
	}

	fallbackBody, err := utils.Get(fallbackURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", cookieWithSecret),
		utils.WithHeader("Secret", secret), // Web API 需要的签名头
		utils.WithRandomIPHeader(),
	)
	if err == nil {
		var resp struct {
			Data struct {
				Url string `json:"url"`
			} `json:"data"`
		}
		if json.Unmarshal(fallbackBody, &resp) == nil && resp.Data.Url != "" {
			return resp.Data.Url, nil
		}
	}

	return "", errors.New("download url not found (copyright restricted)")
}

// GetLyrics 获取歌词
func (k *Kuwo) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "kuwo" {
		return "", errors.New("source mismatch")
	}

	rid := s.ID
	if s.Extra != nil && s.Extra["rid"] != "" {
		rid = s.Extra["rid"]
	}

	params := url.Values{}
	params.Set("musicId", rid)
	params.Set("httpsStatus", "1")

	apiURL := "http://m.kuwo.cn/newh5/singles/songinfoandlrc?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", k.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return "", fmt.Errorf("failed to fetch kuwo lyric API: %w", err)
	}

	var resp struct {
		Data struct {
			Lrclist []struct {
				Time      string `json:"time"`
				LineLyric string `json:"lineLyric"`
			} `json:"lrclist"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse kuwo lyric JSON: %w", err)
	}

	if len(resp.Data.Lrclist) == 0 {
		return "", nil
	}

	var sb strings.Builder
	for _, line := range resp.Data.Lrclist {
		secs, _ := strconv.ParseFloat(line.Time, 64)
		m := int(secs) / 60
		s := int(secs) % 60
		ms := int((secs - float64(int(secs))) * 100)
		sb.WriteString(fmt.Sprintf("[%02d:%02d.%02d]%s\n", m, s, ms, line.LineLyric))
	}
	return sb.String(), nil
}

func parseKuwoLegacyJSON(body []byte, out interface{}) error {
	jsonStr := strings.ReplaceAll(string(body), "'", "\"")
	return json.Unmarshal([]byte(jsonStr), out)
}

func findKuwoSubmatch(input, pattern string) string {
	matches := regexp.MustCompile(pattern).FindStringSubmatch(input)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func findKuwoSubmatches(input, pattern string) []string {
	allMatches := regexp.MustCompile(pattern).FindAllStringSubmatch(input, -1)
	values := make([]string, 0, len(allMatches))
	for _, match := range allMatches {
		if len(match) >= 2 {
			values = append(values, match[1])
		}
	}
	return values
}

func stripKuwoHTMLTags(raw string) string {
	if raw == "" {
		return ""
	}

	replacer := strings.NewReplacer(
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
		"</p>", "\n",
		"</div>", "\n",
	)
	raw = replacer.Replace(raw)
	raw = regexp.MustCompile(`(?is)<[^>]+>`).ReplaceAllString(raw, "")

	return normalizeKuwoText(raw)
}

func decodeKuwoEscapedString(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	decoded, err := strconv.Unquote(`"` + raw + `"`)
	if err != nil {
		raw = strings.ReplaceAll(raw, `\/`, `/`)
		return raw
	}

	return decoded
}

func parseKuwoAlbumPageTitle(title string) (string, string) {
	title = normalizeKuwoText(title)
	if title == "" {
		return "", ""
	}

	parts := strings.Split(title, "_")
	if len(parts) < 2 {
		return "", ""
	}

	name := normalizeKuwoText(strings.TrimSuffix(parts[0], "专辑"))
	artist := normalizeKuwoText(parts[1])

	return name, artist
}

func normalizeKuwoText(value string) string {
	if value == "" {
		return ""
	}

	value = html.UnescapeString(value)
	value = strings.ReplaceAll(value, "\u00a0", " ")
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\\n;", "\n")
	value = strings.ReplaceAll(value, "\\n", "\n")
	value = strings.ReplaceAll(value, "\n;", "\n")
	return strings.TrimSpace(value)
}

func normalizeKuwoImageURL(raw string) string {
	raw = normalizeKuwoText(raw)
	if raw == "" {
		return ""
	}

	if strings.HasPrefix(raw, "//") {
		raw = "http:" + raw
	} else if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		switch {
		case strings.HasPrefix(raw, "img"):
			raw = "http://" + raw
		default:
			raw = "http://img1.kuwo.cn/star/albumcover/" + strings.TrimPrefix(raw, "/")
		}
	}

	replacements := []struct {
		old string
		new string
	}{
		{"/120/", "/500/"},
		{"/150/", "/500/"},
		{"/240/", "/500/"},
		{"_100.", "_500."},
		{"_120.", "_500."},
		{"_150.", "_500."},
		{"_240.", "_500."},
	}
	for _, replacement := range replacements {
		if strings.Contains(raw, replacement.old) {
			raw = strings.Replace(raw, replacement.old, replacement.new, 1)
		}
	}

	return raw
}

func parseKuwoStringInt(value string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(value))
	return n
}

func parseKuwoAnyString(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatFloat(v, 'f', 0, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func parseKuwoAnyInt(value interface{}) int {
	switch v := value.(type) {
	case nil:
		return 0
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case string:
		return parseKuwoStringInt(v)
	default:
		return parseKuwoStringInt(fmt.Sprint(v))
	}
}

func parseKuwoAnySlice(value interface{}) []interface{} {
	switch v := value.(type) {
	case nil:
		return nil
	case []interface{}:
		return v
	default:
		return nil
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func parseSizeFromMInfo(minfo string) int64 {
	if minfo == "" {
		return 0
	}
	type FormatInfo struct {
		Format  string
		Bitrate string
		Size    int64
	}
	var formats []FormatInfo
	parts := strings.Split(minfo, ";")
	for _, part := range parts {
		kv := make(map[string]string)
		attrs := strings.Split(part, ",")
		for _, attr := range attrs {
			pair := strings.Split(attr, ":")
			if len(pair) == 2 {
				kv[pair[0]] = pair[1]
			}
		}
		sizeStr := kv["size"]
		if sizeStr == "" {
			continue
		}
		sizeStr = strings.TrimSuffix(strings.ToLower(sizeStr), "mb")
		sizeMb, _ := strconv.ParseFloat(sizeStr, 64)
		sizeBytes := int64(sizeMb * 1024 * 1024)
		formats = append(formats, FormatInfo{Format: kv["format"], Bitrate: kv["bitrate"], Size: sizeBytes})
	}
	for _, f := range formats {
		if f.Format == "mp3" && f.Bitrate == "128" {
			return f.Size
		}
	}
	for _, f := range formats {
		if f.Format == "mp3" && f.Bitrate == "320" {
			return f.Size
		}
	}
	for _, f := range formats {
		if f.Format == "flac" {
			return f.Size
		}
	}
	for _, f := range formats {
		if f.Format == "flac" && f.Bitrate == "2000" {
			return f.Size
		}
	}
	var maxSize int64
	for _, f := range formats {
		if f.Size > maxSize {
			maxSize = f.Size
		}
	}
	return maxSize
}

func parseBitrateFromMInfo(minfo string) int {
	if minfo == "" {
		return 128
	}
	type FormatInfo struct {
		Format  string
		Bitrate string
		Size    int64
	}
	var formats []FormatInfo
	parts := strings.Split(minfo, ";")
	for _, part := range parts {
		kv := make(map[string]string)
		attrs := strings.Split(part, ",")
		for _, attr := range attrs {
			pair := strings.Split(attr, ":")
			if len(pair) == 2 {
				kv[pair[0]] = pair[1]
			}
		}
		sizeStr := kv["size"]
		if sizeStr == "" {
			continue
		}
		sizeStr = strings.TrimSuffix(strings.ToLower(sizeStr), "mb")
		sizeMb, _ := strconv.ParseFloat(sizeStr, 64)
		sizeBytes := int64(sizeMb * 1024 * 1024)
		formats = append(formats, FormatInfo{Format: kv["format"], Bitrate: kv["bitrate"], Size: sizeBytes})
	}
	toInt := func(s string) int { v, _ := strconv.Atoi(s); return v }
	for _, f := range formats {
		if f.Format == "mp3" && f.Bitrate == "128" {
			return 128
		}
	}
	for _, f := range formats {
		if f.Format == "mp3" && f.Bitrate == "320" {
			return 320
		}
	}
	for _, f := range formats {
		if f.Format == "flac" && f.Bitrate == "2000" {
			if val := toInt(f.Bitrate); val > 0 {
				return val
			}
			return 2000
		}
	}
	for _, f := range formats {
		if f.Format == "flac" {
			if val := toInt(f.Bitrate); val > 0 {
				return val
			}
			return 800
		}
	}
	return 128
}
