package joox

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	UserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36"
	Cookie        = "wmid=142420656; user_type=1; country=id; session_key=2a5d97d05dc8fe238150184eaf3519ad;"
	XForwardedFor = "36.73.34.109"
)

type Joox struct {
	cookie string
}

func New(cookie string) *Joox {
	if cookie == "" {
		cookie = Cookie
	}
	return &Joox{cookie: cookie}
}

var defaultJoox = New(Cookie)

func Search(keyword string) ([]model.Song, error) { return defaultJoox.Search(keyword) }
func SearchAlbum(keyword string) ([]model.Playlist, error) {
	return defaultJoox.SearchAlbum(keyword)
}
func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultJoox.SearchPlaylist(keyword)
}                                                      // [新增]
func GetPlaylistSongs(id string) ([]model.Song, error) { return defaultJoox.GetPlaylistSongs(id) } // [新增]
func GetAlbumSongs(id string) ([]model.Song, error)    { return defaultJoox.GetAlbumSongs(id) }
func GetDownloadURL(s *model.Song) (string, error)     { return defaultJoox.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error)          { return defaultJoox.GetLyrics(s) }
func ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	return defaultJoox.ParseAlbum(link)
}
func Parse(link string) (*model.Song, error) { return defaultJoox.Parse(link) }

// Search 搜索歌曲
func (j *Joox) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("country", "sg")
	params.Set("lang", "zh_cn")
	params.Set("keyword", keyword)
	apiURL := "https://cache.api.joox.com/openjoox/v3/search?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		SectionList []struct {
			ItemList []struct {
				Song []struct {
					SongInfo struct {
						ID         string `json:"id"`
						Name       string `json:"name"`
						AlbumName  string `json:"album_name"`
						ArtistList []struct {
							Name string `json:"name"`
						} `json:"artist_list"`
						PlayDuration int `json:"play_duration"`
						Images       []struct {
							Width int    `json:"width"`
							URL   string `json:"url"`
						} `json:"images"`
						VipFlag int `json:"vip_flag"`
					} `json:"song_info"`
				} `json:"song"`
			} `json:"item_list"`
		} `json:"section_list"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("joox search json error: %w", err)
	}

	var songs []model.Song
	for _, section := range resp.SectionList {
		for _, items := range section.ItemList {
			for _, songItem := range items.Song {
				info := songItem.SongInfo
				if info.ID == "" {
					continue
				}

				var artistNames []string
				for _, ar := range info.ArtistList {
					artistNames = append(artistNames, ar.Name)
				}

				var cover string
				for _, img := range info.Images {
					if img.Width == 300 {
						cover = img.URL
						break
					}
				}
				if cover == "" && len(info.Images) > 0 {
					cover = info.Images[0].URL
				}

				songs = append(songs, model.Song{
					Source:   "joox",
					ID:       info.ID,
					Name:     info.Name,
					Artist:   strings.Join(artistNames, " / "),
					Album:    info.AlbumName,
					Duration: info.PlayDuration,
					Cover:    cover,
					Link:     fmt.Sprintf("https://www.joox.com/hk/single/%s", info.ID),
					Extra: map[string]string{
						"songid": info.ID,
					},
				})
			}
		}
	}
	return songs, nil
}

// SearchPlaylist 搜索歌单
func (j *Joox) SearchAlbum(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("country", "sg")
	params.Set("lang", "zh_cn")
	params.Set("keyword", keyword)
	apiURL := "https://cache.api.joox.com/openjoox/v3/search?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		SectionList []struct {
			SectionType int `json:"section_type"`
			ItemList    []struct {
				Type  int `json:"type"`
				Album struct {
					ID          string       `json:"id"`
					Name        string       `json:"name"`
					Images      []jooxImage  `json:"images"`
					PublishDate string       `json:"publish_date"`
					ArtistList  []jooxArtist `json:"artist_list"`
				} `json:"album"`
			} `json:"item_list"`
		} `json:"section_list"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("joox album search json error: %w", err)
	}

	albums := make([]model.Playlist, 0)
	seen := make(map[string]struct{})
	for _, section := range resp.SectionList {
		if section.SectionType != 1 {
			continue
		}
		for _, item := range section.ItemList {
			if item.Type != 2 {
				continue
			}
			albumID := normalizeJooxID(item.Album.ID)
			if albumID == "" {
				continue
			}
			if _, ok := seen[albumID]; ok {
				continue
			}
			seen[albumID] = struct{}{}

			albums = append(albums, model.Playlist{
				Source:      "joox",
				ID:          albumID,
				Name:        item.Album.Name,
				Cover:       pickJooxImage(item.Album.Images),
				Creator:     joinJooxArtists(item.Album.ArtistList),
				Description: strings.TrimSpace(item.Album.PublishDate),
				Link:        jooxAlbumLink(albumID),
				Extra: map[string]string{
					"type":         "album",
					"album_id":     albumID,
					"publish_date": strings.TrimSpace(item.Album.PublishDate),
				},
			})
		}
	}

	if len(albums) == 0 {
		return nil, errors.New("no albums found")
	}

	return albums, nil
}

func (j *Joox) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("country", "sg")
	params.Set("lang", "zh_cn")
	params.Set("keyword", keyword)
	apiURL := "https://cache.api.joox.com/openjoox/v3/search?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	// Update struct to match joox_playlist_search.json structure
	var resp struct {
		SectionList []struct {
			SectionTitle string `json:"section_title"`
			SectionType  int    `json:"section_type"`
			ItemList     []struct {
				Type           int `json:"type"` // 1: Editor Playlist, 2: Album, 5: Song
				EditorPlaylist struct {
					ID     string `json:"id"`
					Name   string `json:"name"`
					Images []struct {
						Width int    `json:"width"`
						URL   string `json:"url"`
					} `json:"images"`
				} `json:"editor_playlist"`
			} `json:"item_list"`
		} `json:"section_list"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("joox search playlist json error: %w", err)
	}

	var playlists []model.Playlist
	for _, section := range resp.SectionList {
		for _, item := range section.ItemList {
			// According to the JSON, Type 1 is a playlist (Editor Playlist)
			// We skip Albums (Type 2) or Songs (Type 5)
			if item.Type != 1 {
				continue
			}

			info := item.EditorPlaylist
			if info.ID == "" {
				continue
			}

			// Image selection logic: prioritize 300px, fallback to first available
			var cover string
			for _, img := range info.Images {
				if img.Width == 300 {
					cover = img.URL
					break
				}
			}
			if cover == "" && len(info.Images) > 0 {
				cover = info.Images[0].URL
			}

			// Generate the public link
			link := fmt.Sprintf("https://www.joox.com/hk/playlist/%s", info.ID)

			// Populate the Playlist model
			playlists = append(playlists, model.Playlist{
				Source: "joox", // Essential for universal player logic
				ID:     info.ID,
				Name:   info.Name,
				Cover:  cover,
				Link:   link,

				// Fields not provided in the Search API response (JSON):
				// TrackCount:  0,
				// PlayCount:   0,
				// Creator:     "",
				// Description: "",

				// Optional: Store raw ID in Extra if needed for specific logic later
				Extra: map[string]string{
					"playlist_id": info.ID,
				},
			})
		}
	}
	return playlists, nil
}

// GetPlaylistSongs 获取歌单详情 (Updated to use OpenJoox v3 API)
func (j *Joox) GetAlbumSongs(id string) ([]model.Song, error) {
	_, songs, err := j.fetchAlbumDetail(id)
	return songs, err
}

func (j *Joox) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`joox\.com/.*/album/([^/?#]+)`),
		regexp.MustCompile(`h_activity_id=([^&]+)`),
		regexp.MustCompile(`albumid=([^&]+)`),
	}

	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(link)
		if len(matches) >= 2 {
			return j.fetchAlbumDetail(matches[1])
		}
	}

	if len(link) > 10 && !strings.Contains(link, "/") {
		return j.fetchAlbumDetail(link)
	}

	return nil, nil, errors.New("invalid joox album link")
}

func (j *Joox) GetPlaylistSongs(id string) ([]model.Song, error) {
	params := url.Values{}
	// The new v3 API uses "id" instead of "playlistid"
	params.Set("id", id)
	params.Set("country", "sg")
	params.Set("lang", "zh_cn")

	// Use the same host/path structure as Search
	// Guessing the endpoint is /playlist based on /search pattern
	apiURL := "https://cache.api.joox.com/openjoox/v3/playlist?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	// We reuse the generic "Section" structure from the Search API
	// because modern Joox APIs (v3) return "Pages" composed of "Sections".
	var resp struct {
		SectionList []struct {
			ItemList []struct {
				Type int `json:"type"` // We look for Type 5 (Song)
				Song []struct {
					SongInfo struct {
						ID         string `json:"id"`
						Name       string `json:"name"`
						AlbumName  string `json:"album_name"`
						AlbumID    string `json:"album_id"`
						ArtistList []struct {
							Name string `json:"name"`
						} `json:"artist_list"`
						PlayDuration int `json:"play_duration"`
						Images       []struct {
							Width int    `json:"width"`
							URL   string `json:"url"`
						} `json:"images"`
						VipFlag int `json:"vip_flag"`
					} `json:"song_info"`
				} `json:"song"`
			} `json:"item_list"`
		} `json:"section_list"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("joox playlist json error: %w", err)
	}

	var songs []model.Song
	foundSongs := false

	// Iterate through all sections to find songs
	for _, section := range resp.SectionList {
		for _, item := range section.ItemList {
			// Type 5 corresponds to Songs in the v3 API (as seen in your search json)
			if item.Type == 5 {
				for _, songItem := range item.Song {
					info := songItem.SongInfo
					if info.ID == "" {
						continue
					}

					var artistNames []string
					for _, ar := range info.ArtistList {
						artistNames = append(artistNames, ar.Name)
					}

					var cover string
					for _, img := range info.Images {
						if img.Width == 300 {
							cover = img.URL
							break
						}
					}
					if cover == "" && len(info.Images) > 0 {
						cover = info.Images[0].URL
					}

					// Fallback for missing cover using AlbumID if available
					if cover == "" && info.AlbumID != "" {
						// Standard Joox album cover pattern
						cover = fmt.Sprintf("https://imgcache.joox.com/music/joox/photo/mid_album_300/%s/%s/%s.jpg",
							info.AlbumID[len(info.AlbumID)-2:],
							info.AlbumID[len(info.AlbumID)-1:],
							info.AlbumID)
					}

					songs = append(songs, model.Song{
						Source:   "joox",
						ID:       info.ID,
						Name:     info.Name,
						Artist:   strings.Join(artistNames, " / "),
						Album:    info.AlbumName,
						Duration: info.PlayDuration,
						Cover:    cover,
						Link:     fmt.Sprintf("https://www.joox.com/hk/single/%s", info.ID),
						Extra: map[string]string{
							"songid": info.ID,
						},
					})
					foundSongs = true
				}
			}
		}
	}

	if !foundSongs {
		// If no songs found, the ID might be invalid or the playlist is empty
		return nil, errors.New("no songs found in playlist or invalid playlist ID")
	}

	return songs, nil
}

// Parse 解析链接并获取完整信息
func (j *Joox) fetchAlbumDetail(id string) (*model.Playlist, []model.Song, error) {
	albumData, err := j.fetchAlbumPageData(id)
	if err != nil {
		return nil, nil, err
	}

	albumID := normalizeJooxID(albumData.ID)
	if albumID == "" {
		return nil, nil, errors.New("album not found")
	}

	trackCount := albumData.TrackList.TotalCount
	if trackCount == 0 {
		if albumData.TrackList.ListCount > 0 {
			trackCount = albumData.TrackList.ListCount
		} else {
			trackCount = len(albumData.TrackList.Items)
		}
	}

	cover := strings.TrimSpace(albumData.ImgSrc)
	if cover == "" && len(albumData.TrackList.Items) > 0 {
		cover = pickJooxImage(albumData.TrackList.Items[0].Images)
	}

	album := &model.Playlist{
		Source:      "joox",
		ID:          albumID,
		Name:        albumData.Title,
		Cover:       cover,
		TrackCount:  trackCount,
		Creator:     joinJooxArtists(albumData.ArtistList),
		Description: strings.TrimSpace(albumData.Description),
		Link:        jooxAlbumLink(albumID),
		Extra: map[string]string{
			"type":         "album",
			"album_id":     albumID,
			"publish_date": strings.TrimSpace(albumData.PublishDate),
		},
	}

	songs := make([]model.Song, 0, len(albumData.TrackList.Items))
	for _, item := range albumData.TrackList.Items {
		songID := normalizeJooxID(item.ID)
		if songID == "" {
			continue
		}
		songs = append(songs, model.Song{
			Source:   "joox",
			ID:       songID,
			Name:     item.Name,
			Artist:   joinJooxArtists(item.ArtistList),
			Album:    firstNonEmpty(item.AlbumName, albumData.Title),
			Duration: item.PlayDuration,
			Cover:    firstNonEmpty(pickJooxImage(item.Images), strings.TrimSpace(albumData.ImgSrc)),
			Link:     fmt.Sprintf("https://www.joox.com/hk/single/%s", songID),
			Extra: map[string]string{
				"songid":   songID,
				"album_id": albumID,
			},
		})
	}

	if len(songs) == 0 {
		return nil, nil, errors.New("album has no songs")
	}

	return album, songs, nil
}

func (j *Joox) fetchAlbumPageData(id string) (*jooxAlbumPageData, error) {
	albumID := normalizeJooxID(id)
	if albumID == "" {
		return nil, errors.New("album id is empty")
	}

	pageURL := jooxAlbumLink(albumID)
	body, err := utils.Get(pageURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	matches := regexp.MustCompile(`(?s)<script[^>]*id="__NEXT_DATA__"[^>]*>(.*?)</script>`).FindSubmatch(body)
	if len(matches) < 2 {
		return nil, errors.New("joox album page data not found")
	}

	var nextData struct {
		Props struct {
			PageProps struct {
				AlbumData jooxAlbumPageData `json:"albumData"`
				Content   struct {
					Page struct {
						AlbumData jooxAlbumPageData `json:"albumData"`
					} `json:"page"`
				} `json:"content"`
			} `json:"pageProps"`
		} `json:"props"`
	}

	if err := json.Unmarshal(matches[1], &nextData); err != nil {
		return nil, fmt.Errorf("joox album page json error: %w", err)
	}

	albumData := nextData.Props.PageProps.AlbumData
	if normalizeJooxID(albumData.ID) == "" {
		albumData = nextData.Props.PageProps.Content.Page.AlbumData
	}
	if normalizeJooxID(albumData.ID) == "" {
		return nil, errors.New("album data not found")
	}

	return &albumData, nil
}

type jooxArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type jooxImage struct {
	Width int    `json:"width"`
	URL   string `json:"url"`
}

type jooxAlbumTrack struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	AlbumID      string       `json:"album_id"`
	AlbumName    string       `json:"album_name"`
	ArtistList   []jooxArtist `json:"artist_list"`
	PlayDuration int          `json:"play_duration"`
	Images       []jooxImage  `json:"images"`
}

type jooxAlbumPageData struct {
	ID          string       `json:"id"`
	ImgSrc      string       `json:"imgSrc"`
	Title       string       `json:"title"`
	ArtistList  []jooxArtist `json:"artistList"`
	PublishDate string       `json:"publishDate"`
	Description string       `json:"description"`
	TrackList   struct {
		Items      []jooxAlbumTrack `json:"items"`
		ListCount  int              `json:"list_count"`
		TotalCount int              `json:"total_count"`
	} `json:"trackList"`
}

func joinJooxArtists(artists []jooxArtist) string {
	names := make([]string, 0, len(artists))
	for _, artist := range artists {
		name := strings.TrimSpace(artist.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, " / ")
}

func pickJooxImage(images []jooxImage) string {
	for _, preferred := range []int{300, 1000, 100} {
		for _, image := range images {
			if image.Width == preferred && strings.TrimSpace(image.URL) != "" {
				return image.URL
			}
		}
	}
	for _, image := range images {
		if strings.TrimSpace(image.URL) != "" {
			return image.URL
		}
	}
	return ""
}

func normalizeJooxID(raw string) string {
	id := strings.TrimSpace(raw)
	if id == "" {
		return ""
	}
	if strings.Contains(id, "%") {
		if decoded, err := url.PathUnescape(id); err == nil {
			id = decoded
		}
		if decoded, err := url.QueryUnescape(id); err == nil {
			id = strings.ReplaceAll(decoded, " ", "+")
		}
	}
	return id
}

func jooxAlbumLink(id string) string {
	return fmt.Sprintf("https://www.joox.com/hk/album/%s", normalizeJooxID(id))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (j *Joox) Parse(link string) (*model.Song, error) {
	// 1. 提取 ID
	// 支持格式: https://www.joox.com/hk/single/C+Q0... 或纯 ID
	re := regexp.MustCompile(`joox\.com/.*/single/([a-zA-Z0-9]+)`)
	matches := re.FindStringSubmatch(link)
	var songID string
	if len(matches) >= 2 {
		songID = matches[1]
	} else {
		// 尝试直接匹配 ID (如果是纯 ID 字符串)
		if len(link) > 10 && !strings.Contains(link, "/") {
			songID = link
		} else {
			return nil, errors.New("invalid joox link")
		}
	}

	// 2. 调用核心逻辑获取详情
	return j.fetchSongInfo(songID)
}

// GetDownloadURL 获取下载链接
func (j *Joox) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "joox" {
		return "", errors.New("source mismatch")
	}
	if s.URL != "" {
		return s.URL, nil
	}

	songID := s.ID
	if s.Extra != nil && s.Extra["songid"] != "" {
		songID = s.Extra["songid"]
	}

	// 复用核心逻辑
	info, err := j.fetchSongInfo(songID)
	if err != nil {
		return "", err
	}
	return info.URL, nil
}

// fetchSongInfo 内部函数：获取歌曲详情和下载链接
func (j *Joox) fetchSongInfo(songID string) (*model.Song, error) {
	params := url.Values{}
	params.Set("songid", songID)
	params.Set("lang", "zh_cn")
	params.Set("country", "sg")

	apiURL := "https://api.joox.com/web-fcgi-bin/web_get_songinfo?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	bodyStr := string(body)
	if strings.HasPrefix(bodyStr, "MusicInfoCallback(") {
		bodyStr = strings.TrimPrefix(bodyStr, "MusicInfoCallback(")
		bodyStr = strings.TrimSuffix(bodyStr, ")")
	}

	var resp struct {
		Msong     string      `json:"msong"`   // 歌名
		Msinger   string      `json:"msinger"` // 歌手
		Malbum    string      `json:"malbum"`  // 专辑
		Img       string      `json:"img"`     // 封面
		MInterval int         `json:"minterval"`
		R320Url   string      `json:"r320Url"`
		R192Url   string      `json:"r192Url"`
		Mp3Url    string      `json:"mp3Url"`
		M4aUrl    string      `json:"m4aUrl"`
		KbpsMap   interface{} `json:"kbps_map"`
	}

	if err := json.Unmarshal([]byte(bodyStr), &resp); err != nil {
		return nil, fmt.Errorf("joox detail json error: %w", err)
	}

	// 解析下载链接
	availableQualities := make(map[string]interface{})
	if kbpsMapStr, ok := resp.KbpsMap.(string); ok {
		json.Unmarshal([]byte(kbpsMapStr), &availableQualities)
	} else if kbpsMapObj, ok := resp.KbpsMap.(map[string]interface{}); ok {
		availableQualities = kbpsMapObj
	}

	type Candidate struct {
		MapKey string
		URL    string
	}
	candidates := []Candidate{
		{"320", resp.R320Url}, {"192", resp.R192Url}, {"128", resp.Mp3Url}, {"96", resp.M4aUrl},
	}

	var downloadURL string
	for _, c := range candidates {
		if val, ok := availableQualities[c.MapKey]; ok {
			hasSize := false
			switch v := val.(type) {
			case string:
				hasSize = v != "0" && v != ""
			case float64:
				hasSize = v > 0
			case int:
				hasSize = v > 0
			}
			if hasSize && c.URL != "" {
				downloadURL = c.URL
				break
			}
		}
	}

	if downloadURL == "" {
		return nil, errors.New("no valid download url found")
	}

	return &model.Song{
		Source:   "joox",
		ID:       songID,
		Name:     resp.Msong,
		Artist:   resp.Msinger,
		Album:    resp.Malbum,
		Duration: resp.MInterval,
		Cover:    resp.Img,
		URL:      downloadURL,
		Link:     fmt.Sprintf("https://www.joox.com/hk/single/%s", songID),
		Extra: map[string]string{
			"songid": songID,
		},
	}, nil
}

// GetLyrics 获取歌词
func (j *Joox) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "joox" {
		return "", errors.New("source mismatch")
	}

	songID := s.ID
	if s.Extra != nil && s.Extra["songid"] != "" {
		songID = s.Extra["songid"]
	}

	params := url.Values{}
	params.Set("musicid", songID)
	params.Set("country", "sg")
	params.Set("lang", "zh_cn")
	apiURL := "https://api.joox.com/web-fcgi-bin/web_lyric?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return "", err
	}

	bodyStr := string(body)
	if idx := strings.Index(bodyStr, "MusicJsonCallback("); idx >= 0 {
		bodyStr = strings.TrimPrefix(bodyStr[idx:], "MusicJsonCallback(")
		bodyStr = strings.TrimSuffix(bodyStr, ")")
	}

	var resp struct {
		Lyric string `json:"lyric"`
	}
	if err := json.Unmarshal([]byte(bodyStr), &resp); err != nil {
		return "", fmt.Errorf("joox lyric json parse error: %w", err)
	}
	if resp.Lyric == "" {
		return "", errors.New("lyric not found or empty")
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(resp.Lyric)
	if err != nil {
		return "", fmt.Errorf("base64 decode error: %w", err)
	}

	return string(decodedBytes), nil
}
