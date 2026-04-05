package qq

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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
	UserAgent       = "Mozilla/5.0 (iPhone; CPU iPhone OS 9_1 like Mac OS X) AppleWebKit/601.1.46 (KHTML, like Gecko) Version/9.0 Mobile/13B143 Safari/601.1"
	SearchReferer   = "http://m.y.qq.com"
	DownloadReferer = "http://y.qq.com"
	LyricReferer    = "https://y.qq.com/portal/player.html"
)

type QQ struct {
	cookie     string
	isVipCache *bool
}

func New(cookie string) *QQ { return &QQ{cookie: cookie} }

var defaultQQ = New("")

func Search(keyword string) ([]model.Song, error) { return defaultQQ.Search(keyword) }
func SearchAlbum(keyword string) ([]model.Playlist, error) {
	return defaultQQ.SearchAlbum(keyword)
}
func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultQQ.SearchPlaylist(keyword)
}
func GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := defaultQQ.fetchAlbumDetail(id)
	return songs, err
}
func ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	return defaultQQ.ParseAlbum(link)
}
func GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := defaultQQ.fetchPlaylistDetail(id)
	return songs, err
}
func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultQQ.ParsePlaylist(link)
}
func GetDownloadURL(s *model.Song) (string, error) { return defaultQQ.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error)      { return defaultQQ.GetLyrics(s) }
func Parse(link string) (*model.Song, error)       { return defaultQQ.Parse(link) }

// GetRecommendedPlaylists returns recommended playlists.
func GetRecommendedPlaylists() ([]model.Playlist, error) { return defaultQQ.GetRecommendedPlaylists() }

func (q *QQ) IsVipAccount() (bool, error) {
	if q.isVipCache != nil {
		return *q.isVipCache, nil
	}

	if q.cookie == "" {
		isVip := false
		q.isVipCache = &isVip
		return false, nil
	}

	// Use a random GUID to reduce the chance of rate limiting.
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	guid := fmt.Sprintf("%d", r.Int63n(9000000000)+1000000000)

	// Probe a VIP-only song to detect account capability.
	songMID := "004YZbkL2MNHoY"
	// Prefer M500 here because standard VIP accounts may not have FLAC access.
	filename := fmt.Sprintf("M500%s%s.mp3", songMID, songMID)

	reqData := map[string]interface{}{
		"comm": map[string]interface{}{
			"cv":          4747474,
			"ct":          24,
			"format":      "json",
			"inCharset":   "utf-8",
			"outCharset":  "utf-8",
			"notice":      0,
			"platform":    "yqq.json",
			"needNewCode": 1,
			"uin":         0,
		},
		"req_1": map[string]interface{}{
			"module": "music.vkey.GetVkey",
			"method": "UrlGetVkey",
			"param": map[string]interface{}{
				"guid":      guid,
				"songmid":   []string{songMID},
				"songtype":  []int{0},
				"uin":       "0",
				"loginflag": 1,
				"platform":  "20",
				"filename":  []string{filename},
			},
		},
	}

	jsonData, _ := json.Marshal(reqData)
	headers := []utils.RequestOption{
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", DownloadReferer),
		utils.WithHeader("Content-Type", "application/json"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post("https://u.y.qq.com/cgi-bin/musicu.fcg", bytes.NewReader(jsonData), headers...)
	if err != nil {
		return false, err
	}

	var result struct {
		Req1 struct {
			Code int `json:"code"`
			Data struct {
				MidUrlInfo []struct {
					Purl string `json:"purl"`
				} `json:"midurlinfo"`
			} `json:"data"`
		} `json:"req_1"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return false, err
	}

	// Cache only when the probe result is conclusive.
	isVip := false
	if len(result.Req1.Data.MidUrlInfo) > 0 && result.Req1.Data.MidUrlInfo[0].Purl != "" {
		isVip = true
	} else if result.Req1.Code != 0 {
		return false, fmt.Errorf("api returned error code: %d", result.Req1.Code)
	}

	q.isVipCache = &isVip
	return isVip, nil
}

// Search searches songs.
func (q *QQ) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("w", keyword)
	params.Set("format", "json")
	params.Set("p", "1")
	params.Set("n", "10")
	apiURL := "http://c.y.qq.com/soso/fcgi-bin/search_for_qq_cp?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", SearchReferer),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Song struct {
				List []struct {
					SongID    int64  `json:"songid"`
					SongName  string `json:"songname"`
					SongMID   string `json:"songmid"`
					AlbumName string `json:"albumname"`
					AlbumMID  string `json:"albummid"`
					Interval  int    `json:"interval"`
					Size128   int64  `json:"size128"`
					Size320   int64  `json:"size320"`
					SizeFlac  int64  `json:"sizeflac"`
					Singer    []struct {
						Name string `json:"name"`
					} `json:"singer"`
					Pay struct {
						PayDownload   int `json:"paydownload"`
						PayPlay       int `json:"payplay"`
						PayTrackPrice int `json:"paytrackprice"`
					} `json:"pay"`
				} `json:"list"`
			} `json:"song"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq json parse error: %w", err)
	}

	isVip, _ := q.IsVipAccount()

	var songs []model.Song
	for _, item := range resp.Data.Song.List {
		// Hide VIP-only songs for non-VIP accounts.
		if !isVip && item.Pay.PayPlay == 1 {
			continue
		}

		var artistNames []string
		for _, s := range item.Singer {
			artistNames = append(artistNames, s.Name)
		}

		var coverURL string
		if item.AlbumMID != "" {
			coverURL = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", item.AlbumMID)
		}

		fileSize := item.Size128
		bitrate := 128
		if item.SizeFlac > 0 {
			fileSize = item.SizeFlac
			if item.Interval > 0 {
				bitrate = int(fileSize * 8 / 1000 / int64(item.Interval))
			} else {
				bitrate = 800
			}
		} else if item.Size320 > 0 {
			fileSize = item.Size320
			bitrate = 320
		}

		songs = append(songs, model.Song{
			Source:   "qq",
			ID:       item.SongMID,
			Name:     item.SongName,
			Artist:   strings.Join(artistNames, "、"),
			Album:    item.AlbumName,
			Duration: item.Interval,
			Size:     fileSize,
			Bitrate:  bitrate,
			Cover:    coverURL,
			Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", item.SongMID),
			Extra: map[string]string{
				"songmid": item.SongMID,
			},
		})
	}
	return songs, nil
}

// joinQQNames joins artist names for display.
func joinQQNames(names []string) string {
	return strings.Join(names, ", ")
}

// SearchAlbum searches albums.
func (q *QQ) SearchAlbum(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("format", "json")
	params.Set("p", "1")
	params.Set("n", "10")
	params.Set("w", keyword)
	params.Set("t", "8")
	apiURL := "http://c.y.qq.com/soso/fcgi-bin/search_for_qq_cp?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		utils.WithHeader("Referer", "https://y.qq.com/portal/search.html"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Album struct {
				List []struct {
					AlbumID    int64  `json:"albumID"`
					AlbumMID   string `json:"albumMID"`
					AlbumName  string `json:"albumName"`
					PublicTime string `json:"publicTime"`
					SingerName string `json:"singerName"`
				} `json:"list"`
			} `json:"album"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq album json parse error: %w", err)
	}

	var albums []model.Playlist
	for _, item := range resp.Data.Album.List {
		if item.AlbumMID == "" {
			continue
		}

		albums = append(albums, model.Playlist{
			Source:      "qq",
			ID:          item.AlbumMID,
			Name:        item.AlbumName,
			Cover:       fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", item.AlbumMID),
			Creator:     item.SingerName,
			Description: "",
			Link:        fmt.Sprintf("https://y.qq.com/n/ryqq/albumDetail/%s", item.AlbumMID),
			Extra: map[string]string{
				"type":         "album",
				"album_id":     strconv.FormatInt(item.AlbumID, 10),
				"album_mid":    item.AlbumMID,
				"publish_time": item.PublicTime,
			},
		})
	}

	if len(albums) == 0 {
		return nil, errors.New("no albums found")
	}

	return albums, nil
}

func (q *QQ) GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := q.fetchAlbumDetail(id)
	return songs, err
}

func (q *QQ) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`albumDetail/([A-Za-z0-9]+)`),
		regexp.MustCompile(`album/([A-Za-z0-9]+)`),
		regexp.MustCompile(`albummid=([A-Za-z0-9]+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(link)
		if len(matches) >= 2 {
			return q.fetchAlbumDetail(matches[1])
		}
	}

	return nil, nil, errors.New("invalid qq album link")
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

// fetchAlbumDetail returns album metadata and songs.
func (q *QQ) fetchAlbumDetail(id string) (*model.Playlist, []model.Song, error) {
	albumMID := strings.TrimSpace(id)
	if albumMID == "" {
		return nil, nil, errors.New("album id is empty")
	}

	headers := []utils.RequestOption{
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		utils.WithHeader("Referer", "https://y.qq.com/"),
		utils.WithHeader("Content-Type", "application/json"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	}

	detailReq := map[string]interface{}{
		"comm": map[string]interface{}{
			"ct": 24,
			"cv": 0,
		},
		"album": map[string]interface{}{
			"module": "music.musichallAlbum.AlbumInfoServer",
			"method": "GetAlbumDetail",
			"param": map[string]interface{}{
				"albumMid": albumMID,
			},
		},
	}
	detailJSON, _ := json.Marshal(detailReq)

	detailBody, err := utils.Post("https://u.y.qq.com/cgi-bin/musicu.fcg", bytes.NewReader(detailJSON), headers...)
	if err != nil {
		return nil, nil, err
	}

	var detailResp struct {
		Code  int `json:"code"`
		Album struct {
			Code int `json:"code"`
			Data struct {
				BasicInfo struct {
					AlbumID     int64  `json:"albumID"`
					AlbumMid    string `json:"albumMid"`
					AlbumName   string `json:"albumName"`
					PublishDate string `json:"publishDate"`
					Desc        string `json:"desc"`
				} `json:"basicInfo"`
				Company struct {
					Name string `json:"name"`
				} `json:"company"`
				Singer struct {
					SingerList []struct {
						Name string `json:"name"`
					} `json:"singerList"`
				} `json:"singer"`
			} `json:"data"`
		} `json:"album"`
	}

	if err := json.Unmarshal(detailBody, &detailResp); err != nil {
		return nil, nil, fmt.Errorf("qq album detail json parse error: %w", err)
	}
	if detailResp.Album.Code != 0 {
		return nil, nil, fmt.Errorf("qq album detail api error code: %d", detailResp.Album.Code)
	}

	info := detailResp.Album.Data.BasicInfo
	if info.AlbumMid != "" {
		albumMID = info.AlbumMid
	}
	if info.AlbumName == "" {
		return nil, nil, errors.New("album not found")
	}

	artistNames := make([]string, 0, len(detailResp.Album.Data.Singer.SingerList))
	for _, singer := range detailResp.Album.Data.Singer.SingerList {
		if singer.Name != "" {
			artistNames = append(artistNames, singer.Name)
		}
	}

	const batchSize = 100
	totalNum := 0
	songs := make([]model.Song, 0)

	for begin := 0; ; begin += batchSize {
		songReq := map[string]interface{}{
			"comm": map[string]interface{}{
				"ct": 24,
				"cv": 0,
			},
			"album": map[string]interface{}{
				"module": "music.musichallAlbum.AlbumSongList",
				"method": "GetAlbumSongList",
				"param": map[string]interface{}{
					"albumMid": albumMID,
					"begin":    begin,
					"num":      batchSize,
					"order":    2,
				},
			},
		}
		songJSON, _ := json.Marshal(songReq)

		songBody, err := utils.Post("https://u.y.qq.com/cgi-bin/musicu.fcg", bytes.NewReader(songJSON), headers...)
		if err != nil {
			return nil, nil, err
		}

		var songResp struct {
			Code  int `json:"code"`
			Album struct {
				Code int `json:"code"`
				Data struct {
					TotalNum int `json:"totalNum"`
					SongList []struct {
						SongInfo struct {
							Mid      string `json:"mid"`
							Name     string `json:"name"`
							Interval int    `json:"interval"`
							Singer   []struct {
								Name string `json:"name"`
							} `json:"singer"`
							Album struct {
								ID   int64  `json:"id"`
								Mid  string `json:"mid"`
								Name string `json:"name"`
							} `json:"album"`
							File struct {
								Size128MP3 int64 `json:"size_128mp3"`
								Size320MP3 int64 `json:"size_320mp3"`
								SizeFlac   int64 `json:"size_flac"`
							} `json:"file"`
							Pay struct {
								PayPlay int `json:"pay_play"`
							} `json:"pay"`
						} `json:"songInfo"`
					} `json:"songList"`
				} `json:"data"`
			} `json:"album"`
		}

		if err := json.Unmarshal(songBody, &songResp); err != nil {
			return nil, nil, fmt.Errorf("qq album songs json parse error: %w", err)
		}
		if songResp.Album.Code != 0 {
			return nil, nil, fmt.Errorf("qq album songs api error code: %d", songResp.Album.Code)
		}

		if totalNum == 0 {
			totalNum = songResp.Album.Data.TotalNum
		}

		pageSongs := songResp.Album.Data.SongList
		if len(pageSongs) == 0 {
			break
		}

		for _, item := range pageSongs {
			songInfo := item.SongInfo
			if songInfo.Mid == "" {
				continue
			}

			pageArtistNames := make([]string, 0, len(songInfo.Singer))
			for _, singer := range songInfo.Singer {
				if singer.Name != "" {
					pageArtistNames = append(pageArtistNames, singer.Name)
				}
			}

			fileSize := songInfo.File.Size128MP3
			bitrate := 128
			if songInfo.File.SizeFlac > 0 {
				fileSize = songInfo.File.SizeFlac
				if songInfo.Interval > 0 {
					bitrate = int(fileSize * 8 / 1000 / int64(songInfo.Interval))
				} else {
					bitrate = 800
				}
			} else if songInfo.File.Size320MP3 > 0 {
				fileSize = songInfo.File.Size320MP3
				bitrate = 320
			}

			cover := ""
			if songInfo.Album.Mid != "" {
				cover = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", songInfo.Album.Mid)
			}

			songs = append(songs, model.Song{
				Source:   "qq",
				ID:       songInfo.Mid,
				Name:     songInfo.Name,
				Artist:   joinQQNames(pageArtistNames),
				Album:    songInfo.Album.Name,
				AlbumID:  songInfo.Album.Mid,
				Duration: songInfo.Interval,
				Size:     fileSize,
				Bitrate:  bitrate,
				Cover:    cover,
				Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", songInfo.Mid),
				Extra: map[string]string{
					"songmid":   songInfo.Mid,
					"album_mid": songInfo.Album.Mid,
					"album_id":  strconv.FormatInt(songInfo.Album.ID, 10),
				},
			})
		}

		if len(pageSongs) < batchSize {
			break
		}
		if totalNum > 0 && begin+len(pageSongs) >= totalNum {
			break
		}
	}

	trackCount := totalNum
	if trackCount == 0 {
		trackCount = len(songs)
	}

	album := &model.Playlist{
		Source:      "qq",
		ID:          albumMID,
		Name:        info.AlbumName,
		Cover:       fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", albumMID),
		TrackCount:  trackCount,
		Creator:     joinQQNames(artistNames),
		Description: info.Desc,
		Link:        fmt.Sprintf("https://y.qq.com/n/ryqq/albumDetail/%s", albumMID),
		Extra: map[string]string{
			"type":         "album",
			"album_id":     strconv.FormatInt(info.AlbumID, 10),
			"album_mid":    albumMID,
			"company":      detailResp.Album.Data.Company.Name,
			"publish_time": info.PublishDate,
		},
	}

	return album, songs, nil
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

// fetchPlaylistDetail returns playlist metadata and songs.
func (q *QQ) fetchPlaylistDetail(id string) (*model.Playlist, []model.Song, error) {
	params := url.Values{}
	params.Set("type", "1")
	params.Set("json", "1")
	params.Set("utf8", "1")
	params.Set("onlysong", "0")
	params.Set("disstid", id)
	params.Set("format", "json")
	params.Set("g_tk", "5381")
	params.Set("loginUin", "0")
	params.Set("hostUin", "0")
	params.Set("inCharset", "utf8")
	params.Set("outCharset", "utf-8")
	params.Set("notice", "0")
	params.Set("platform", "yqq")
	params.Set("needNewCode", "0")

	apiURL := "http://c.y.qq.com/qzone/fcg-bin/fcg_ucc_getcdinfo_byids_cp.fcg?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"),
		utils.WithHeader("Referer", "https://y.qq.com/"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, nil, err
	}

	// Unwrap JSONP when needed.
	sBody := string(body)
	if idx := strings.Index(sBody, "("); idx >= 0 && strings.HasSuffix(strings.TrimSpace(sBody), ")") {
		sBody = sBody[idx+1 : len(sBody)-1]
		body = []byte(sBody)
	}

	var resp struct {
		Cdlist []struct {
			Dissname string `json:"dissname"`
			Logo     string `json:"logo"`
			Nickname string `json:"nickname"`
			Desc     string `json:"desc"`
			Visitnum int    `json:"visitnum"`
			Songnum  int    `json:"songnum"`
			Songlist []struct {
				SongID    int64  `json:"songid"`
				SongName  string `json:"songname"`
				SongMID   string `json:"songmid"`
				AlbumName string `json:"albumname"`
				AlbumMID  string `json:"albummid"`
				Interval  int    `json:"interval"`
				Size128   int64  `json:"size128"`
				Size320   int64  `json:"size320"`
				SizeFlac  int64  `json:"sizeflac"`
				Pay       struct {
					PayPlay int `json:"payplay"`
				} `json:"pay"`
				Singer []struct {
					Name string `json:"name"`
				} `json:"singer"`
			} `json:"songlist"`
		} `json:"cdlist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("qq playlist detail json error: %w", err)
	}

	if len(resp.Cdlist) == 0 {
		return nil, nil, errors.New("playlist not found (empty cdlist)")
	}

	info := resp.Cdlist[0]

	// Build playlist metadata.
	playlist := &model.Playlist{
		Source:      "qq",
		ID:          id,
		Name:        info.Dissname,
		Cover:       info.Logo,
		Creator:     info.Nickname,
		Description: info.Desc,
		PlayCount:   info.Visitnum,
		TrackCount:  info.Songnum,
		Link:        fmt.Sprintf("https://y.qq.com/n/ryqq/playlist/%s", id),
	}

	var songs []model.Song
	for _, item := range info.Songlist {

		var artistNames []string
		for _, s := range item.Singer {
			artistNames = append(artistNames, s.Name)
		}

		var coverURL string
		if item.AlbumMID != "" {
			coverURL = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", item.AlbumMID)
		}

		fileSize := item.Size128
		bitrate := 128
		if item.SizeFlac > 0 {
			fileSize = item.SizeFlac
			if item.Interval > 0 {
				bitrate = int(fileSize * 8 / 1000 / int64(item.Interval))
			} else {
				bitrate = 800
			}
		} else if item.Size320 > 0 {
			fileSize = item.Size320
			bitrate = 320
		}

		songs = append(songs, model.Song{
			Source:   "qq",
			ID:       item.SongMID,
			Name:     item.SongName,
			Artist:   strings.Join(artistNames, "、"),
			Album:    item.AlbumName,
			Duration: item.Interval,
			Size:     fileSize,
			Bitrate:  bitrate,
			Cover:    coverURL,
			Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", item.SongMID),
			Extra: map[string]string{
				"songmid": item.SongMID,
			},
		})
	}
	return playlist, songs, nil
}

// Parse parses a song link and enriches it with download info when possible.
func (q *QQ) Parse(link string) (*model.Song, error) {
	re := regexp.MustCompile(`songDetail/(\w+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, errors.New("invalid qq music link")
	}
	songMID := matches[1]

	song, err := q.fetchSongDetail(songMID)
	if err != nil {
		return nil, err
	}

	downloadURL, err := q.GetDownloadURL(song)
	if err == nil {
		song.URL = downloadURL
	}

	return song, nil
}

// GetDownloadURL returns a download URL.
func (q *QQ) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "qq" {
		return "", errors.New("source mismatch")
	}

	songMID := s.ID
	if s.Extra != nil && s.Extra["songmid"] != "" {
		songMID = s.Extra["songmid"]
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	guid := fmt.Sprintf("%d", r.Int63n(9000000000)+1000000000)

	// Request qualities from best to worst and use the first successful one.
	var prefixes []string
	var exts []string

	isVip, _ := q.IsVipAccount()
	if isVip {
		prefixes = []string{"AI00", "Q001", "Q000", "F000", "O801", "M800", "M500"} // Master, Atmos5.1, Atmos2.0, FLAC, 640k, 320k, 128k
		exts = []string{"flac", "flac", "flac", "flac", "ogg", "mp3", "mp3"}
	} else {
		prefixes = []string{"M800", "M500"} // Non-VIPs typically only reach 128kbps natively unless the track is free 320k
		exts = []string{"mp3", "mp3"}
	}

	var filenames []string
	var songmids []string
	var songtypes []int

	for i := range prefixes {
		filename := fmt.Sprintf("%s%s%s.%s", prefixes[i], songMID, songMID, exts[i])
		filenames = append(filenames, filename)
		songmids = append(songmids, songMID)
		songtypes = append(songtypes, 0)
	}

	reqData := map[string]interface{}{
		"comm": map[string]interface{}{
			"cv":          4747474,
			"ct":          24,
			"format":      "json",
			"inCharset":   "utf-8",
			"outCharset":  "utf-8",
			"notice":      0,
			"platform":    "yqq.json",
			"needNewCode": 1,
			"uin":         0,
		},
		"req_1": map[string]interface{}{
			"module": "music.vkey.GetVkey",
			"method": "UrlGetVkey",
			"param": map[string]interface{}{
				"guid":      guid,
				"songmid":   songmids,
				"songtype":  songtypes,
				"uin":       "0",
				"loginflag": 1,
				"platform":  "20",
				"filename":  filenames,
			},
		},
	}

	jsonData, _ := json.Marshal(reqData)
	headers := []utils.RequestOption{
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", DownloadReferer),
		utils.WithHeader("Content-Type", "application/json"),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post("https://u.y.qq.com/cgi-bin/musicu.fcg", bytes.NewReader(jsonData), headers...)
	if err != nil {
		return "", err
	}

	var result struct {
		Req1 struct {
			Data struct {
				MidUrlInfo []struct {
					Filename string `json:"filename"`
					Purl     string `json:"purl"`
				} `json:"midurlinfo"`
			} `json:"data"`
		} `json:"req_1"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("qq geturl json parse error: %w", err)
	}

	// Because we passed the filenames cleanly prioritized down from best to worst, the mapped return array technically aligns 1:1.
	// We'll iterate the initial array order we asked for and grab the first `Filename` that successfully gave a `Purl`.
	for _, expectedFilename := range filenames {
		for _, info := range result.Req1.Data.MidUrlInfo {
			if info.Filename == expectedFilename && info.Purl != "" {
				return "https://ws.stream.qqmusic.qq.com/" + info.Purl, nil
			}
		}
	}

	return "", errors.New("no valid download url found or vip required")
}

// fetchSongDetail loads song metadata by songmid.
func (q *QQ) fetchSongDetail(songMID string) (*model.Song, error) {
	params := url.Values{}
	params.Set("songmid", songMID)
	params.Set("format", "json")

	apiURL := "https://c.y.qq.com/v8/fcg-bin/fcg_play_single_song.fcg?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Referer", SearchReferer),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data []struct {
			ID    int64  `json:"id"`
			Name  string `json:"name"`
			Mid   string `json:"mid"`
			Album struct {
				Name string `json:"name"`
				Mid  string `json:"mid"`
			} `json:"album"`
			Singer []struct {
				Name string `json:"name"`
			} `json:"singer"`
			Interval int `json:"interval"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("qq detail json parse error: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, errors.New("song detail not found")
	}

	item := resp.Data[0]
	var artistNames []string
	for _, s := range item.Singer {
		artistNames = append(artistNames, s.Name)
	}

	var coverURL string
	if item.Album.Mid != "" {
		coverURL = fmt.Sprintf("https://y.gtimg.cn/music/photo_new/T002R300x300M000%s.jpg", item.Album.Mid)
	}

	return &model.Song{
		Source:   "qq",
		ID:       item.Mid,
		Name:     item.Name,
		Artist:   strings.Join(artistNames, "、"),
		Album:    item.Album.Name,
		Duration: item.Interval,
		Cover:    coverURL,
		Link:     fmt.Sprintf("https://y.qq.com/n/ryqq/songDetail/%s", item.Mid),
		Extra: map[string]string{
			"songmid": item.Mid,
		},
	}, nil
}

// GetLyrics fetches lyrics.
func (q *QQ) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "qq" {
		return "", errors.New("source mismatch")
	}

	songMID := s.ID
	if s.Extra != nil && s.Extra["songmid"] != "" {
		songMID = s.Extra["songmid"]
	}

	params := url.Values{}
	params.Set("songmid", songMID)
	params.Set("loginUin", "0")
	params.Set("hostUin", "0")
	params.Set("format", "json")
	params.Set("inCharset", "utf8")
	params.Set("outCharset", "utf-8")
	params.Set("notice", "0")
	params.Set("platform", "yqq.json")
	params.Set("needNewCode", "0")

	apiURL := "https://c.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg?" + params.Encode()
	headers := []utils.RequestOption{
		utils.WithHeader("Referer", LyricReferer),
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", q.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Get(apiURL, headers...)
	if err != nil {
		return "", err
	}

	var resp struct {
		Retcode int    `json:"retcode"`
		Lyric   string `json:"lyric"`
		Trans   string `json:"trans"`
	}
	sBody := string(body)
	if idx := strings.Index(sBody, "("); idx >= 0 {
		sBody = sBody[idx+1:]
		if idx2 := strings.LastIndex(sBody, ")"); idx2 >= 0 {
			sBody = sBody[:idx2]
		}
	}

	if err := json.Unmarshal([]byte(sBody), &resp); err != nil {
		return "", fmt.Errorf("qq lyric json parse error: %w", err)
	}
	if resp.Lyric == "" {
		return "", errors.New("lyric is empty or not found")
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(resp.Lyric)
	if err != nil {
		return "", fmt.Errorf("base64 decode error: %w", err)
	}

	return string(decodedBytes), nil
}
