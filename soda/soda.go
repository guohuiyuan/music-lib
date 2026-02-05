package soda

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	// PC端 UserAgent
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/134.0.0.0 Safari/537.36"
)

type Soda struct {
	cookie string
}

func New(cookie string) *Soda { return &Soda{cookie: cookie} }

var defaultSoda = New("")

func Search(keyword string) ([]model.Song, error) { return defaultSoda.Search(keyword) }
func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultSoda.SearchPlaylist(keyword)
}
func GetPlaylistSongs(id string) ([]model.Song, error) {
	// 复用 fetchPlaylistDetail，只返回歌曲列表
	_, songs, err := defaultSoda.fetchPlaylistDetail(id)
	return songs, err
}
func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultSoda.ParsePlaylist(link)
}
func GetDownloadInfo(s *model.Song) (*DownloadInfo, error) { return defaultSoda.GetDownloadInfo(s) }
func GetDownloadURL(s *model.Song) (string, error)         { return defaultSoda.GetDownloadURL(s) }
func Download(s *model.Song, outputPath string) error      { return defaultSoda.Download(s, outputPath) }
func GetLyrics(s *model.Song) (string, error)              { return defaultSoda.GetLyrics(s) }
func Parse(link string) (*model.Song, error)               { return defaultSoda.Parse(link) }

// GetRecommendedPlaylists [新增] 获取推荐歌单 (空实现)
func GetRecommendedPlaylists() ([]model.Playlist, error) { return defaultSoda.GetRecommendedPlaylists() }

// Search 搜索歌曲 (PC API)
func (s *Soda) Search(keyword string) ([]model.Song, error) {
	params := url.Values{}
	params.Set("q", keyword)
	params.Set("cursor", "0")
	params.Set("search_method", "input")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	apiURL := "https://api.qishui.com/luna/pc/search/track?" + params.Encode()
	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		ResultGroups []struct {
			Data []struct {
				Entity struct {
					Track struct {
						ID       string `json:"id"`
						Name     string `json:"name"`
						Duration int    `json:"duration"`
						Artists  []struct {
							Name string `json:"name"`
						} `json:"artists"`
						Album struct {
							Name     string `json:"name"`
							UrlCover struct {
								Urls []string `json:"urls"`
								Uri  string   `json:"uri"`
							} `json:"url_cover"`
						} `json:"album"`
						BitRates []struct {
							Size    int64  `json:"size"`
							Quality string `json:"quality"`
						} `json:"bit_rates"`
					} `json:"track"`
				} `json:"entity"`
			} `json:"data"`
		} `json:"result_groups"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda search json parse error: %w", err)
	}
	if len(resp.ResultGroups) == 0 {
		return nil, nil
	}

	var songs []model.Song
	for _, item := range resp.ResultGroups[0].Data {
		track := item.Entity.Track
		if track.ID == "" {
			continue
		}

		// 计算最大文件大小
		var displaySize int64
		for _, br := range track.BitRates {
			if br.Size > displaySize {
				displaySize = br.Size
			}
		}

		var artistNames []string
		for _, ar := range track.Artists {
			artistNames = append(artistNames, ar.Name)
		}

		var cover string
		if len(track.Album.UrlCover.Urls) > 0 {
			domain := track.Album.UrlCover.Urls[0]
			uri := track.Album.UrlCover.Uri
			if domain != "" && uri != "" {
				cover = domain + uri + "~c5_375x375.jpg"
			}
		}

		bitrate := 0
		seconds := track.Duration / 1000
		if seconds > 0 && displaySize > 0 {
			bitrate = int(displaySize * 8 / 1000 / int64(seconds))
		}

		songs = append(songs, model.Song{
			Source:   "soda",
			ID:       track.ID,
			Name:     track.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    track.Album.Name,
			Duration: track.Duration / 1000,
			Size:     displaySize,
			Bitrate:  bitrate,
			Cover:    cover,
			Link:     fmt.Sprintf("https://www.qishui.com/track/%s", track.ID),
			Extra: map[string]string{
				"track_id": track.ID,
			},
		})
	}
	return songs, nil
}

// SearchPlaylist 搜索歌单 (PC API)
func (s *Soda) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	params := url.Values{}
	params.Set("q", keyword)
	params.Set("cursor", "0")
	params.Set("search_method", "input")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	apiURL := "https://api.qishui.com/luna/pc/search/playlist?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		ResultGroups []struct {
			Data []struct {
				Entity struct {
					Playlist struct {
						ID    string `json:"id"`
						Title string `json:"title"`
						Desc  string `json:"desc"`
						Owner struct {
							Nickname   string `json:"nickname"`
							PublicName string `json:"public_name"`
						} `json:"owner"`
						CountTracks int `json:"count_tracks"`
						UrlCover    struct {
							Urls []string `json:"urls"`
							Uri  string   `json:"uri"`
						} `json:"url_cover"`
					} `json:"playlist"`
				} `json:"entity"`
			} `json:"data"`
		} `json:"result_groups"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda playlist json parse error: %w", err)
	}

	var playlists []model.Playlist
	if len(resp.ResultGroups) == 0 || len(resp.ResultGroups[0].Data) == 0 {
		return nil, nil
	}

	for _, item := range resp.ResultGroups[0].Data {
		pl := item.Entity.Playlist
		if pl.ID == "" {
			continue
		}

		cover := ""
		if len(pl.UrlCover.Urls) > 0 {
			domain := pl.UrlCover.Urls[0]
			if pl.UrlCover.Uri != "" && !strings.Contains(domain, pl.UrlCover.Uri) {
				cover = domain + pl.UrlCover.Uri
			} else {
				cover = domain
			}
			if cover != "" && !strings.Contains(cover, "~") {
				cover += "~c5_300x300.jpg"
			}
		}

		creator := pl.Owner.PublicName
		if creator == "" {
			creator = pl.Owner.Nickname
		}

		playlists = append(playlists, model.Playlist{
			Source:      "soda",
			ID:          pl.ID,
			Name:        pl.Title,
			Cover:       cover,
			TrackCount:  pl.CountTracks,
			Creator:     creator,
			Description: pl.Desc,
			Link:        fmt.Sprintf("https://www.qishui.com/playlist/%s", pl.ID),
		})
	}
	return playlists, nil
}

// GetRecommendedPlaylists [新增] 获取推荐歌单 (空实现)
func (s *Soda) GetRecommendedPlaylists() ([]model.Playlist, error) {
	// 汽水音乐目前没有公开的每日推荐歌单 PC 接口
	return nil, errors.New("soda daily recommendation not supported")
}

// GetPlaylistSongs 获取歌单所有歌曲
func (s *Soda) GetPlaylistSongs(id string) ([]model.Song, error) {
	_, songs, err := s.fetchPlaylistDetail(id)
	return songs, err
}

// ParsePlaylist 解析歌单链接
func (s *Soda) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	re := regexp.MustCompile(`playlist/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, nil, errors.New("invalid soda playlist link")
	}
	playlistID := matches[1]
	return s.fetchPlaylistDetail(playlistID)
}

// fetchPlaylistDetail [内部通用] 获取歌单详情
func (s *Soda) fetchPlaylistDetail(id string) (*model.Playlist, []model.Song, error) {
	params := url.Values{}
	params.Set("playlist_id", id)
	params.Set("cursor", "0")
	params.Set("cnt", "20")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	apiURL := "https://api.qishui.com/luna/pc/playlist/detail?" + params.Encode()

	body, err := utils.Get(apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		Playlist struct {
			ID    string `json:"id"`
			Title string `json:"title"`
			Desc  string `json:"desc"`
			Owner struct {
				Nickname string `json:"nickname"`
			} `json:"owner"`
			CountTracks int `json:"count_tracks"`
			UrlCover    struct {
				Urls []string `json:"urls"`
				Uri  string   `json:"uri"`
			} `json:"url_cover"`
		} `json:"playlist"`

		MediaResources []struct {
			Type   string `json:"type"`
			Entity struct {
				TrackWrapper struct {
					Track struct {
						ID       string `json:"id"`
						Name     string `json:"name"`
						Duration int    `json:"duration"`
						Artists  []struct {
							Name string `json:"name"`
						} `json:"artists"`
						Album struct {
							Name     string `json:"name"`
							UrlCover struct {
								Urls []string `json:"urls"`
								Uri  string   `json:"uri"`
							} `json:"url_cover"`
						} `json:"album"`
						BitRates []struct {
							Size    int64  `json:"size"`
							Quality string `json:"quality"`
						} `json:"bit_rates"`
						AudioInfo struct {
							PlayInfoList []struct {
								MainPlayUrl string `json:"main_play_url"`
								PlayAuth    string `json:"play_auth"`
								Size        int64  `json:"size"`
								Format      string `json:"format"`
								Bitrate     int    `json:"bitrate"`
							} `json:"play_info_list"`
						} `json:"audio_info"`
					} `json:"track"`
				} `json:"track_wrapper"`
			} `json:"entity"`
		} `json:"media_resources"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("soda playlist detail json error: %w", err)
	}

	pl := &model.Playlist{
		Source:      "soda",
		ID:          id,
		Name:        resp.Playlist.Title,
		Creator:     resp.Playlist.Owner.Nickname,
		Description: resp.Playlist.Desc,
		TrackCount:  resp.Playlist.CountTracks,
		Link:        fmt.Sprintf("https://www.qishui.com/playlist/%s", id),
	}
	if len(resp.Playlist.UrlCover.Urls) > 0 {
		cover := resp.Playlist.UrlCover.Urls[0]
		if resp.Playlist.UrlCover.Uri != "" && !strings.Contains(cover, resp.Playlist.UrlCover.Uri) {
			cover += resp.Playlist.UrlCover.Uri
		}
		if !strings.Contains(cover, "~") {
			cover += "~c5_300x300.jpg"
		}
		pl.Cover = cover
	}

	var songs []model.Song
	for _, item := range resp.MediaResources {
		if item.Type != "track" {
			continue
		}
		track := item.Entity.TrackWrapper.Track
		if track.ID == "" {
			continue
		}

		var displaySize int64
		for _, br := range track.BitRates {
			if br.Size > displaySize {
				displaySize = br.Size
			}
		}
		for _, pi := range track.AudioInfo.PlayInfoList {
			if pi.Size > displaySize {
				displaySize = pi.Size
			}
		}

		var artistNames []string
		for _, ar := range track.Artists {
			artistNames = append(artistNames, ar.Name)
		}

		var cover string
		if len(track.Album.UrlCover.Urls) > 0 {
			domain := track.Album.UrlCover.Urls[0]
			uri := track.Album.UrlCover.Uri
			if domain != "" && uri != "" && !strings.Contains(domain, uri) {
				cover = domain + uri + "~c5_375x375.jpg"
			} else if domain != "" {
				cover = domain + "~c5_375x375.jpg"
			}
		}

		bitrate := 0
		seconds := track.Duration / 1000
		if seconds > 0 && displaySize > 0 {
			bitrate = int(displaySize * 8 / 1000 / int64(seconds))
		}

		song := model.Song{
			Source:   "soda",
			ID:       track.ID,
			Name:     track.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    track.Album.Name,
			Duration: track.Duration / 1000,
			Size:     displaySize,
			Bitrate:  bitrate,
			Cover:    cover,
			Link:     fmt.Sprintf("https://www.qishui.com/track/%s", track.ID),
			Extra: map[string]string{
				"track_id": track.ID,
			},
		}

		if len(track.AudioInfo.PlayInfoList) > 0 {
			best := track.AudioInfo.PlayInfoList[0]
			for _, info := range track.AudioInfo.PlayInfoList {
				if info.Size > best.Size {
					best = info
				}
			}
			if best.MainPlayUrl != "" {
				song.URL = best.MainPlayUrl + "#auth=" + url.QueryEscape(best.PlayAuth)
				if song.Size == 0 {
					song.Size = best.Size
				}
				song.Ext = best.Format
				song.Bitrate = best.Bitrate
			}
		}

		songs = append(songs, song)
	}
	return pl, songs, nil
}

// GetDownloadInfo 获取下载信息
func (s *Soda) GetDownloadInfo(song *model.Song) (*DownloadInfo, error) {
	if strings.Contains(song.URL, "#auth=") {
		parts := strings.Split(song.URL, "#auth=")
		if len(parts) == 2 {
			auth, _ := url.QueryUnescape(parts[1])
			return &DownloadInfo{
				URL:      parts[0],
				PlayAuth: auth,
				Format:   song.Ext,
				Size:     song.Size,
			}, nil
		}
	}

	if song.Source != "soda" {
		return nil, errors.New("source mismatch")
	}

	trackID := song.ID
	if song.Extra != nil && song.Extra["track_id"] != "" {
		trackID = song.Extra["track_id"]
	}

	params := url.Values{}
	params.Set("track_id", trackID)
	params.Set("media_type", "track")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	v2URL := "https://api.qishui.com/luna/pc/track_v2?" + params.Encode()
	v2Body, err := utils.Get(v2URL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, err
	}

	var v2Resp struct {
		TrackPlayer struct {
			URLPlayerInfo string `json:"url_player_info"`
		} `json:"track_player"`
	}
	if err := json.Unmarshal(v2Body, &v2Resp); err != nil {
		return nil, fmt.Errorf("parse track_v2 response error: %w", err)
	}
	if v2Resp.TrackPlayer.URLPlayerInfo == "" {
		return nil, errors.New("player info url not found")
	}

	return s.fetchPlayerInfo(v2Resp.TrackPlayer.URLPlayerInfo)
}

type DownloadInfo struct {
	URL      string
	PlayAuth string
	Format   string
	Size     int64
}

func (s *Soda) fetchPlayerInfo(playerInfoURL string) (*DownloadInfo, error) {
	infoBody, err := utils.Get(playerInfoURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, err
	}

	var infoResp struct {
		Result struct {
			Data struct {
				PlayInfoList []struct {
					MainPlayUrl   string `json:"MainPlayUrl"`
					BackupPlayUrl string `json:"BackupPlayUrl"`
					PlayAuth      string `json:"PlayAuth"`
					Size          int64  `json:"Size"`
					Bitrate       int    `json:"Bitrate"`
					Format        string `json:"Format"`
				} `json:"PlayInfoList"`
			} `json:"Data"`
		} `json:"Result"`
	}
	if err := json.Unmarshal(infoBody, &infoResp); err != nil {
		return nil, fmt.Errorf("parse play info response error: %w", err)
	}

	list := infoResp.Result.Data.PlayInfoList
	if len(list) == 0 {
		return nil, errors.New("no audio stream found")
	}

	sort.Slice(list, func(i, j int) bool {
		if list[i].Size != list[j].Size {
			return list[i].Size > list[j].Size
		}
		return list[i].Bitrate > list[j].Bitrate
	})

	best := list[0]
	downloadURL := best.MainPlayUrl
	if downloadURL == "" {
		downloadURL = best.BackupPlayUrl
	}
	if downloadURL == "" {
		return nil, errors.New("invalid download url")
	}

	return &DownloadInfo{URL: downloadURL, PlayAuth: best.PlayAuth, Format: best.Format, Size: best.Size}, nil
}

// GetDownloadURL 返回下载链接
func (s *Soda) GetDownloadURL(song *model.Song) (string, error) {
	info, err := s.GetDownloadInfo(song)
	if err != nil {
		return "", err
	}
	return info.URL + "#auth=" + url.QueryEscape(info.PlayAuth), nil
}

// Download 下载并解密歌曲
func (s *Soda) Download(song *model.Song, outputPath string) error {
	info, err := s.GetDownloadInfo(song)
	if err != nil {
		return fmt.Errorf("get download info failed: %w", err)
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", info.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download status: %d", resp.StatusCode)
	}

	encryptedData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	decryptedData, err := DecryptAudio(encryptedData, info.PlayAuth)
	if err != nil {
		return fmt.Errorf("decrypt failed: %w", err)
	}

	err = os.WriteFile(outputPath, decryptedData, 0644)
	if err != nil {
		return err
	}
	return nil
}

// Parse 解析链接并获取完整信息
func (s *Soda) Parse(link string) (*model.Song, error) {
	re := regexp.MustCompile(`track/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) < 2 {
		return nil, errors.New("invalid soda link")
	}
	trackID := matches[1]
	return s.fetchSongDetail(trackID)
}

func (s *Soda) fetchSongDetail(trackID string) (*model.Song, error) {
	params := url.Values{}
	params.Set("track_id", trackID)
	params.Set("media_type", "track")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	v2URL := "https://api.qishui.com/luna/pc/track_v2?" + params.Encode()
	v2Body, err := utils.Get(v2URL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return nil, err
	}

	var v2Resp struct {
		TrackInfo struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Duration int    `json:"duration"`
			Artists  []struct {
				Name string `json:"name"`
			} `json:"artists"`
			Album struct {
				Name     string `json:"name"`
				UrlCover struct {
					Urls []string `json:"urls"`
					Uri  string   `json:"uri"`
				} `json:"url_cover"`
			} `json:"album"`
		} `json:"track_info"`
		TrackPlayer struct {
			URLPlayerInfo string `json:"url_player_info"`
		} `json:"track_player"`
	}
	if err := json.Unmarshal(v2Body, &v2Resp); err != nil {
		return nil, fmt.Errorf("parse track_v2 response error: %w", err)
	}

	if v2Resp.TrackInfo.ID == "" {
		return nil, errors.New("track info not found")
	}

	info := v2Resp.TrackInfo
	var artistNames []string
	for _, ar := range info.Artists {
		artistNames = append(artistNames, ar.Name)
	}

	var cover string
	if len(info.Album.UrlCover.Urls) > 0 {
		domain := info.Album.UrlCover.Urls[0]
		uri := info.Album.UrlCover.Uri
		if domain != "" && uri != "" {
			cover = domain + uri + "~c5_375x375.jpg"
		}
	}

	song := &model.Song{
		Source:   "soda",
		ID:       info.ID,
		Name:     info.Name,
		Artist:   strings.Join(artistNames, "、"),
		Album:    info.Album.Name,
		Duration: info.Duration / 1000,
		Cover:    cover,
		Link:     fmt.Sprintf("https://www.qishui.com/track/%s", info.ID),
		Extra: map[string]string{
			"track_id": info.ID,
		},
	}

	if v2Resp.TrackPlayer.URLPlayerInfo != "" {
		dInfo, err := s.fetchPlayerInfo(v2Resp.TrackPlayer.URLPlayerInfo)
		if err == nil {
			song.URL = dInfo.URL + "#auth=" + url.QueryEscape(dInfo.PlayAuth)
			song.Size = dInfo.Size
			song.Ext = dInfo.Format
			if song.Duration > 0 && dInfo.Size > 0 {
				song.Bitrate = int(dInfo.Size * 8 / 1000 / int64(song.Duration))
			}
		}
	}

	return song, nil
}

// GetLyrics 获取歌词
func (s *Soda) GetLyrics(song *model.Song) (string, error) {
	if song.Source != "soda" {
		return "", errors.New("source mismatch")
	}

	trackID := song.ID
	if song.Extra != nil && song.Extra["track_id"] != "" {
		trackID = song.Extra["track_id"]
	}

	params := url.Values{}
	params.Set("track_id", trackID)
	params.Set("media_type", "track")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	v2URL := "https://api.qishui.com/luna/pc/track_v2?" + params.Encode()
	body, err := utils.Get(v2URL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", s.cookie),
	)
	if err != nil {
		return "", fmt.Errorf("failed to fetch lyric API: %w", err)
	}

	var resp struct {
		Lyric struct {
			Content string `json:"content"`
		} `json:"lyric"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("failed to parse lyric JSON: %w", err)
	}
	if resp.Lyric.Content == "" {
		return "", nil
	}

	return parseSodaLyric(resp.Lyric.Content), nil
}

func parseSodaLyric(raw string) string {
	var sb strings.Builder
	lineRegex := regexp.MustCompile(`^\[(\d+),(\d+)\](.*)$`)
	wordRegex := regexp.MustCompile(`<[^>]+>`)

	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := lineRegex.FindStringSubmatch(line)
		if len(matches) >= 4 {
			startTimeStr := matches[1]
			content := matches[3]
			cleanContent := wordRegex.ReplaceAllString(content, "")
			startTime, _ := strconv.Atoi(startTimeStr)
			minutes := startTime / 60000
			seconds := (startTime % 60000) / 1000
			millis := (startTime % 1000) / 10
			sb.WriteString(fmt.Sprintf("[%02d:%02d.%02d]%s\n", minutes, seconds, millis, cleanContent))
		}
	}
	return sb.String()
}