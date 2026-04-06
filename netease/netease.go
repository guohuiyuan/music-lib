package netease

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
	Referer                = "http://music.163.com/"
	SearchAPI              = "http://music.163.com/api/linux/forward"
	DownloadAPI            = "http://music.163.com/weapi/song/enhance/player/url"
	DownloadEAPI           = "https://interface3.music.163.com/eapi/song/enhance/player/url/v1"
	DetailAPI              = "https://music.163.com/weapi/v3/song/detail"
	PlaylistAPI            = "https://music.163.com/weapi/v3/playlist/detail"
	AlbumAPI               = "https://music.163.com/weapi/v1/album/%s"
	UserAccountAPI         = "https://music.163.com/weapi/nuser/account/get"
	RecommendedPlaylistAPI = "https://music.163.com/weapi/personalized/playlist"
)

type Netease struct {
	cookie     string
	isVipCache *bool
}

type neteaseLinkKind string

const (
	neteaseLinkUnknown  neteaseLinkKind = ""
	neteaseLinkSong     neteaseLinkKind = "song"
	neteaseLinkAlbum    neteaseLinkKind = "album"
	neteaseLinkPlaylist neteaseLinkKind = "playlist"
)

var (
	errInvalidNeteaseLink      = errors.New("invalid netease link")
	errNeteasePlaylistLink     = errors.New("netease playlist link detected, use ParsePlaylist")
	errNeteaseAlbumLink        = errors.New("netease album link detected, use ParseAlbum")
	errNeteaseInvalidAlbumLink = errors.New("invalid netease album link")
	errNeteaseInvalidListLink  = errors.New("invalid netease playlist link")
	errNeteaseSongNotFound     = errors.New("netease song not found")
)

func New(cookie string) *Netease { return &Netease{cookie: cookie} }

var defaultNetease = New("")

func Search(keyword string) ([]model.Song, error) { return defaultNetease.Search(keyword) }
func SearchAlbum(keyword string) ([]model.Playlist, error) {
	return defaultNetease.SearchAlbum(keyword)
}
func SearchPlaylist(keyword string) ([]model.Playlist, error) {
	return defaultNetease.SearchPlaylist(keyword)
}
func GetAlbumSongs(albumID string) ([]model.Song, error) {
	return defaultNetease.GetAlbumSongs(albumID)
}
func ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	return defaultNetease.ParseAlbum(link)
}
func GetPlaylistSongs(playlistID string) ([]model.Song, error) {
	return defaultNetease.GetPlaylistSongs(playlistID)
}
func ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	return defaultNetease.ParsePlaylist(link)
}
func GetDownloadURL(s *model.Song) (string, error) { return defaultNetease.GetDownloadURL(s) }
func GetLyrics(s *model.Song) (string, error)      { return defaultNetease.GetLyrics(s) }
func Parse(link string) (*model.Song, error)       { return defaultNetease.Parse(link) }

// GetRecommendedPlaylists returns recommended playlists without login.
func GetRecommendedPlaylists() ([]model.Playlist, error) {
	return defaultNetease.GetRecommendedPlaylists()
}

// IsVipAccount reports whether the current account is VIP.
func (n *Netease) IsVipAccount() (bool, error) {
	if n.isVipCache != nil {
		return *n.isVipCache, nil
	}

	if n.cookie == "" {
		isVip := false
		n.isVipCache = &isVip
		return false, nil
	}

	reqData := map[string]interface{}{
		"csrf_token": "",
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post(UserAccountAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return false, fmt.Errorf("failed to fetch user account info: %w", err)
	}

	var resp struct {
		Code    int `json:"code"`
		Profile struct {
			VipType int `json:"vipType"`
		} `json:"profile"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return false, fmt.Errorf("netease user account json parse error: %w", err)
	}

	isVip := resp.Code == 200 && resp.Profile.VipType != 0
	n.isVipCache = &isVip
	return isVip, nil
}

// cloudSearch calls the shared Netease cloud search route.
func (n *Netease) cloudSearch(keyword string, searchType int, limit int) ([]byte, error) {
	eparams := map[string]interface{}{
		"method": "POST",
		"url":    "http://music.163.com/api/cloudsearch/pc",
		"params": map[string]interface{}{
			"s":      keyword,
			"type":   searchType,
			"offset": 0,
			"limit":  limit,
		},
	}
	eparamsJSON, _ := json.Marshal(eparams)
	encryptedParam := EncryptLinux(string(eparamsJSON))
	form := url.Values{}
	form.Set("eparams", encryptedParam)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	return utils.Post(SearchAPI, strings.NewReader(form.Encode()), headers...)
}

// Search searches songs.
func (n *Netease) Search(keyword string) ([]model.Song, error) {
	body, err := n.cloudSearch(keyword, 1, 10)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			Songs []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
				Ar   []struct {
					Name string `json:"name"`
				} `json:"ar"`
				Al struct {
					Name   string `json:"name"`
					PicURL string `json:"picUrl"`
				} `json:"al"`
				Dt        int `json:"dt"`
				Privilege struct {
					Fl  int `json:"fl"`
					Pl  int `json:"pl"`
					Fee int `json:"fee"`
				} `json:"privilege"`
				H struct {
					Size int64 `json:"size"`
				} `json:"h"`
				M struct {
					Size int64 `json:"size"`
				} `json:"m"`
				L struct {
					Size int64 `json:"size"`
				} `json:"l"`
			} `json:"songs"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease json parse error: %w", err)
	}

	var songs []model.Song
	isVip, _ := n.IsVipAccount()

	for _, item := range resp.Result.Songs {
		// Skip unavailable songs for non-VIP accounts.
		if !isVip && item.Privilege.Fl == 0 {
			continue
		}

		var size int64
		// Prefer the highest available bitrate.
		if item.Privilege.Fl >= 320000 && item.H.Size > 0 {
			size = item.H.Size
		} else if item.Privilege.Fl >= 192000 && item.M.Size > 0 {
			size = item.M.Size
		} else {
			size = item.L.Size
		}

		duration := item.Dt / 1000
		bitrate := 128
		if duration > 0 && size > 0 {
			bitrate = int(size * 8 / 1000 / int64(duration))
		}

		var artistNames []string
		for _, ar := range item.Ar {
			artistNames = append(artistNames, ar.Name)
		}

		songs = append(songs, model.Song{
			Source:   "netease",
			ID:       strconv.Itoa(item.ID),
			Name:     item.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    item.Al.Name,
			Duration: duration,
			Size:     size,
			Bitrate:  bitrate,
			Cover:    item.Al.PicURL,
			Link:     fmt.Sprintf("https://music.163.com/#/song?id=%d", item.ID),
			Extra: map[string]string{
				"song_id": strconv.Itoa(item.ID),
			},
		})
	}
	return songs, nil
}

// joinArtistNames joins artist names for display.
func joinArtistNames(names []string) string {
	return strings.Join(names, ", ")
}

// SearchAlbum searches albums.
func (n *Netease) SearchAlbum(keyword string) ([]model.Playlist, error) {
	body, err := n.cloudSearch(keyword, 10, 10)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code   int `json:"code"`
		Result struct {
			Albums []struct {
				ID          int    `json:"id"`
				Name        string `json:"name"`
				PicURL      string `json:"picUrl"`
				Size        int    `json:"size"`
				Company     string `json:"company"`
				Description string `json:"description"`
				BriefDesc   string `json:"briefDesc"`
				PublishTime int64  `json:"publishTime"`
				Artist      struct {
					Name string `json:"name"`
				} `json:"artist"`
				Artists []struct {
					Name string `json:"name"`
				} `json:"artists"`
			} `json:"albums"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease album json parse error: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("netease api error code: %d", resp.Code)
	}

	albums := make([]model.Playlist, 0, len(resp.Result.Albums))
	for _, item := range resp.Result.Albums {
		artistName := item.Artist.Name
		if artistName == "" && len(item.Artists) > 0 {
			names := make([]string, 0, len(item.Artists))
			for _, artist := range item.Artists {
				if artist.Name != "" {
					names = append(names, artist.Name)
				}
			}
			artistName = joinArtistNames(names)
		}

		description := item.Description
		if description == "" {
			description = item.BriefDesc
		}

		albums = append(albums, model.Playlist{
			Source:      "netease",
			ID:          strconv.Itoa(item.ID),
			Name:        item.Name,
			Cover:       item.PicURL,
			TrackCount:  item.Size,
			Creator:     artistName,
			Description: description,
			Link:        fmt.Sprintf("https://music.163.com/#/album?id=%d", item.ID),
			Extra: map[string]string{
				"type":         "album",
				"company":      item.Company,
				"publish_time": strconv.FormatInt(item.PublishTime, 10),
			},
		})
	}

	return albums, nil
}

// SearchPlaylist searches playlists.
func (n *Netease) SearchPlaylist(keyword string) ([]model.Playlist, error) {
	body, err := n.cloudSearch(keyword, 1000, 10)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			Playlists []struct {
				ID          int    `json:"id"`
				Name        string `json:"name"`
				CoverImgURL string `json:"coverImgUrl"`
				Creator     struct {
					Nickname string `json:"nickname"`
				} `json:"creator"`
				TrackCount  int    `json:"trackCount"`
				PlayCount   int    `json:"playCount"`
				Description string `json:"description"`
			} `json:"playlists"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease playlist json parse error: %w", err)
	}

	var playlists []model.Playlist
	for _, item := range resp.Result.Playlists {
		playlists = append(playlists, model.Playlist{
			Source:      "netease",
			ID:          strconv.Itoa(item.ID),
			Name:        item.Name,
			Cover:       item.CoverImgURL,
			TrackCount:  item.TrackCount,
			PlayCount:   item.PlayCount,
			Creator:     item.Creator.Nickname,
			Description: item.Description,
			Link:        fmt.Sprintf("https://music.163.com/#/playlist?id=%d", item.ID),
		})
	}
	return playlists, nil
}

// GetAlbumSongs returns songs in an album.
func (n *Netease) GetAlbumSongs(albumID string) ([]model.Song, error) {
	_, songs, err := n.fetchAlbumDetail(albumID)
	return songs, err
}

// ParseAlbum parses an album link.
func (n *Netease) ParseAlbum(link string) (*model.Playlist, []model.Song, error) {
	kind, albumID, err := parseNeteaseLink(link)
	if err != nil || kind != neteaseLinkAlbum {
		return nil, nil, errNeteaseInvalidAlbumLink
	}
	return n.fetchAlbumDetail(albumID)
}

// GetPlaylistSongs returns songs in a playlist.
func (n *Netease) GetPlaylistSongs(playlistID string) ([]model.Song, error) {
	_, songs, err := n.fetchPlaylistDetail(playlistID)
	return songs, err
}

// ParsePlaylist parses a playlist link.
func (n *Netease) ParsePlaylist(link string) (*model.Playlist, []model.Song, error) {
	kind, playlistID, err := parseNeteaseLink(link)
	if err != nil || kind != neteaseLinkPlaylist {
		return nil, nil, errNeteaseInvalidListLink
	}
	return n.fetchPlaylistDetail(playlistID)
}

// fetchAlbumDetail returns album metadata and songs.
func (n *Netease) fetchAlbumDetail(albumID string) (*model.Playlist, []model.Song, error) {
	reqData := map[string]interface{}{
		"csrf_token": "",
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post(fmt.Sprintf(AlbumAPI, albumID), strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		Code  int `json:"code"`
		Album struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			PicURL      string `json:"picUrl"`
			Size        int    `json:"size"`
			Company     string `json:"company"`
			Description string `json:"description"`
			BriefDesc   string `json:"briefDesc"`
			PublishTime int64  `json:"publishTime"`
			Artist      struct {
				Name string `json:"name"`
			} `json:"artist"`
			Artists []struct {
				Name string `json:"name"`
			} `json:"artists"`
		} `json:"album"`
		Songs []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Ar   []struct {
				Name string `json:"name"`
			} `json:"ar"`
			Al struct {
				ID     int    `json:"id"`
				Name   string `json:"name"`
				PicURL string `json:"picUrl"`
			} `json:"al"`
			Dt        int `json:"dt"`
			Privilege struct {
				Fl int `json:"fl"`
			} `json:"privilege"`
			H struct {
				Size int64 `json:"size"`
			} `json:"h"`
			M struct {
				Size int64 `json:"size"`
			} `json:"m"`
			L struct {
				Size int64 `json:"size"`
			} `json:"l"`
		} `json:"songs"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("netease album detail json parse error: %w", err)
	}
	if resp.Code != 200 {
		return nil, nil, fmt.Errorf("netease api error code: %d", resp.Code)
	}

	artistName := resp.Album.Artist.Name
	if artistName == "" && len(resp.Album.Artists) > 0 {
		names := make([]string, 0, len(resp.Album.Artists))
		for _, artist := range resp.Album.Artists {
			if artist.Name != "" {
				names = append(names, artist.Name)
			}
		}
		artistName = joinArtistNames(names)
	}

	description := resp.Album.Description
	if description == "" {
		description = resp.Album.BriefDesc
	}

	album := &model.Playlist{
		Source:      "netease",
		ID:          strconv.Itoa(resp.Album.ID),
		Name:        resp.Album.Name,
		Cover:       resp.Album.PicURL,
		TrackCount:  resp.Album.Size,
		Creator:     artistName,
		Description: description,
		Link:        fmt.Sprintf("https://music.163.com/#/album?id=%d", resp.Album.ID),
		Extra: map[string]string{
			"type":         "album",
			"company":      resp.Album.Company,
			"publish_time": strconv.FormatInt(resp.Album.PublishTime, 10),
		},
	}

	songs := make([]model.Song, 0, len(resp.Songs))
	for _, item := range resp.Songs {
		artistNames := make([]string, 0, len(item.Ar))
		for _, artist := range item.Ar {
			if artist.Name != "" {
				artistNames = append(artistNames, artist.Name)
			}
		}

		var size int64
		if item.Privilege.Fl >= 320000 && item.H.Size > 0 {
			size = item.H.Size
		} else if item.Privilege.Fl >= 192000 && item.M.Size > 0 {
			size = item.M.Size
		} else {
			size = item.L.Size
		}

		duration := item.Dt / 1000
		bitrate := 128
		if duration > 0 && size > 0 {
			bitrate = int(size * 8 / 1000 / int64(duration))
		}

		songs = append(songs, model.Song{
			Source:   "netease",
			ID:       strconv.Itoa(item.ID),
			Name:     item.Name,
			Artist:   joinArtistNames(artistNames),
			Album:    item.Al.Name,
			AlbumID:  strconv.Itoa(item.Al.ID),
			Duration: duration,
			Size:     size,
			Bitrate:  bitrate,
			Cover:    item.Al.PicURL,
			Link:     fmt.Sprintf("https://music.163.com/#/song?id=%d", item.ID),
			Extra: map[string]string{
				"song_id":  strconv.Itoa(item.ID),
				"album_id": strconv.Itoa(item.Al.ID),
			},
		})
	}

	return album, songs, nil
}

func (n *Netease) fetchPlaylistDetail(playlistID string) (*model.Playlist, []model.Song, error) {
	reqData := map[string]interface{}{
		"id":         playlistID,
		"n":          0, // 0表示不直接返回详情，我们只需要ID列表
		"csrf_token": "",
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post(PlaylistAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return nil, nil, err
	}

	var resp struct {
		Code     int `json:"code"`
		Playlist struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			CoverImgURL string `json:"coverImgUrl"`
			Description string `json:"description"`
			PlayCount   int    `json:"playCount"`
			TrackCount  int    `json:"trackCount"`
			Creator     struct {
				Nickname string `json:"nickname"`
			} `json:"creator"`
			// Use trackIds so we can fetch the full list separately.
			TrackIds []struct {
				ID int `json:"id"`
			} `json:"trackIds"`
		} `json:"playlist"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("netease playlist detail json parse error: %w", err)
	}
	if resp.Code != 200 {
		return nil, nil, fmt.Errorf("netease api error code: %d", resp.Code)
	}

	// Build playlist metadata.
	playlist := &model.Playlist{
		Source:      "netease",
		ID:          strconv.Itoa(resp.Playlist.ID),
		Name:        resp.Playlist.Name,
		Cover:       resp.Playlist.CoverImgURL,
		TrackCount:  resp.Playlist.TrackCount,
		PlayCount:   resp.Playlist.PlayCount,
		Creator:     resp.Playlist.Creator.Nickname,
		Description: resp.Playlist.Description,
		Link:        fmt.Sprintf("https://music.163.com/#/playlist?id=%d", resp.Playlist.ID),
	}

	// Collect all song IDs first.
	var allIDs []string
	for _, tid := range resp.Playlist.TrackIds {
		allIDs = append(allIDs, strconv.Itoa(tid.ID))
	}

	// Fetch song details in batches.
	var allSongs []model.Song
	batchSize := 500
	for i := 0; i < len(allIDs); i += batchSize {
		end := i + batchSize
		if end > len(allIDs) {
			end = len(allIDs)
		}

		batchIDs := allIDs[i:end]
		batchSongs, err := n.fetchSongsBatch(batchIDs)
		if err == nil {
			allSongs = append(allSongs, batchSongs...)
		}
	}

	return playlist, allSongs, nil
}

// fetchSongsBatch fetches song details in batches.
func (n *Netease) fetchSongsBatch(songIDs []string) ([]model.Song, error) {
	if len(songIDs) == 0 {
		return nil, nil
	}

	// Build the c payload: [{"id":123},{"id":456},...]
	var cList []map[string]interface{}
	for _, id := range songIDs {
		cList = append(cList, map[string]interface{}{"id": id})
	}
	cJSON, _ := json.Marshal(cList)
	idsJSON, _ := json.Marshal(songIDs)

	reqData := map[string]interface{}{
		"c":   string(cJSON),
		"ids": string(idsJSON),
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))

	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post(DetailAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Songs []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Ar   []struct {
				Name string `json:"name"`
			} `json:"ar"`
			Al struct {
				Name   string `json:"name"`
				PicURL string `json:"picUrl"`
			} `json:"al"`
			Dt int `json:"dt"`
		} `json:"songs"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	var songs []model.Song
	for _, item := range resp.Songs {
		var artistNames []string
		for _, ar := range item.Ar {
			artistNames = append(artistNames, ar.Name)
		}

		songs = append(songs, model.Song{
			Source:   "netease",
			ID:       strconv.Itoa(item.ID),
			Name:     item.Name,
			Artist:   strings.Join(artistNames, "、"),
			Album:    item.Al.Name,
			Duration: item.Dt / 1000,
			Cover:    item.Al.PicURL,
			Link:     fmt.Sprintf("https://music.163.com/#/song?id=%d", item.ID),
			Extra: map[string]string{
				"song_id": strconv.Itoa(item.ID),
			},
		})
	}
	return songs, nil
}

// Parse parses a song link.
func (n *Netease) Parse(link string) (*model.Song, error) {
	kind, songID, err := parseNeteaseLink(link)
	if err != nil {
		return nil, errInvalidNeteaseLink
	}
	switch kind {
	case neteaseLinkPlaylist:
		return nil, errNeteasePlaylistLink
	case neteaseLinkAlbum:
		return nil, errNeteaseAlbumLink
	case neteaseLinkSong:
	default:
		return nil, errInvalidNeteaseLink
	}

	songs, err := n.fetchSongsBatch([]string{songID})
	if err != nil {
		return nil, fmt.Errorf("fetch song detail failed: %w", err)
	}
	if len(songs) == 0 {
		return nil, errNeteaseSongNotFound
	}
	song := &songs[0]

	downloadURL, err := n.GetDownloadURL(song)
	if err == nil {
		song.URL = downloadURL
	}

	return song, nil
}

func parseNeteaseLink(link string) (neteaseLinkKind, string, error) {
	candidates := []string{link}

	if parsed, err := url.Parse(link); err == nil {
		if parsed.Path != "" && parsed.Path != "/" {
			pathCandidate := parsed.Path
			if parsed.RawQuery != "" {
				pathCandidate += "?" + parsed.RawQuery
			}
			candidates = append(candidates, pathCandidate)
		}
		if fragment := strings.TrimSpace(strings.TrimPrefix(parsed.Fragment, "!")); fragment != "" {
			candidates = append(candidates, fragment)
		}
	}

	for _, candidate := range candidates {
		if kind, id, ok := parseNeteaseLinkCandidate(candidate); ok {
			return kind, id, nil
		}
	}

	return neteaseLinkUnknown, "", errInvalidNeteaseLink
}

func parseNeteaseLinkCandidate(candidate string) (neteaseLinkKind, string, bool) {
	parsed, err := url.Parse(candidate)
	if err != nil {
		return neteaseLinkUnknown, "", false
	}

	var kind neteaseLinkKind
	segments := strings.FieldsFunc(strings.ToLower(parsed.Path), func(r rune) bool {
		return r == '/'
	})
	for _, segment := range segments {
		switch segment {
		case string(neteaseLinkSong):
			kind = neteaseLinkSong
		case string(neteaseLinkAlbum):
			kind = neteaseLinkAlbum
		case string(neteaseLinkPlaylist):
			kind = neteaseLinkPlaylist
		}
	}

	id := parsed.Query().Get("id")
	if !isDigits(id) {
		id = ""
	}

	if kind == neteaseLinkUnknown && len(segments) >= 2 {
		last := segments[len(segments)-1]
		prev := segments[len(segments)-2]
		if isDigits(last) {
			switch prev {
			case string(neteaseLinkSong):
				kind = neteaseLinkSong
			case string(neteaseLinkAlbum):
				kind = neteaseLinkAlbum
			case string(neteaseLinkPlaylist):
				kind = neteaseLinkPlaylist
			}
			if kind != neteaseLinkUnknown {
				id = last
			}
		}
	}

	if kind == neteaseLinkUnknown || id == "" {
		return neteaseLinkUnknown, "", false
	}

	return kind, id, true
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// GetDownloadURL returns a download URL.
func (n *Netease) GetDownloadURL(s *model.Song) (string, error) {
	if s.Source != "netease" {
		return "", errors.New("source mismatch")
	}

	songID := s.ID
	if s.Extra != nil && s.Extra["song_id"] != "" {
		songID = s.Extra["song_id"]
	}

	// Try the higher-quality eapi route for VIP accounts first.
	isVip, _ := n.IsVipAccount()
	if isVip {
		// Prefer higher-quality levels when they are available.
		if url, err := n.getEAPIDownloadURL(songID, "hires"); err == nil && url != "" {
			return url, nil
		} else if url, err := n.getEAPIDownloadURL(songID, "lossless"); err == nil && url != "" {
			return url, nil
		} else if url, err := n.getEAPIDownloadURL(songID, "exhigh"); err == nil && url != "" {
			return url, nil
		}
	}

	// Fall back to the original weapi route.
	reqData := map[string]interface{}{
		"ids": []string{songID},
		"br":  320000,
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post(DownloadAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return "", err
	}

	var resp struct {
		Data []struct {
			URL  string `json:"url"`
			Code int    `json:"code"`
			Br   int    `json:"br"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("json parse error: %w", err)
	}
	if len(resp.Data) == 0 || resp.Data[0].URL == "" {
		return "", errors.New("download url not found (might be vip or copyright restricted)")
	}
	return resp.Data[0].URL, nil
}

// getEAPIDownloadURL fetches a high-quality download URL via eapi.
func (n *Netease) getEAPIDownloadURL(songID string, quality string) (string, error) {
	idNum, err := strconv.Atoi(songID)
	if err != nil {
		return "", fmt.Errorf("invalid song id: %v", err)
	}

	headerJSON := `{"os":"pc","appver":"","osver":"","deviceId":"pyncm!","requestId":"12345678"}`

	payload := map[string]interface{}{
		"ids":        []int{idNum},
		"level":      quality,
		"encodeType": "flac",
		"header":     headerJSON,
	}

	payloadBytes, _ := json.Marshal(payload)
	params := EncryptEApi(DownloadEAPI, string(payloadBytes))

	form := url.Values{}
	form.Set("params", params)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post(DownloadEAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return "", err
	}

	var resp struct {
		Data []struct {
			URL  string `json:"url"`
			Code int    `json:"code"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("eapi json parse error: %w", err)
	}
	if len(resp.Data) == 0 || resp.Data[0].URL == "" {
		return "", errors.New("eapi download url not found")
	}
	return resp.Data[0].URL, nil
}

// GetLyrics fetches lyrics.
func (n *Netease) GetLyrics(s *model.Song) (string, error) {
	if s.Source != "netease" {
		return "", errors.New("source mismatch")
	}

	songID := s.ID
	if s.Extra != nil && s.Extra["song_id"] != "" {
		songID = s.Extra["song_id"]
	}

	reqData := map[string]interface{}{
		"csrf_token": "",
		"id":         songID,
		"lv":         -1,
		"tv":         -1,
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithHeader("Cookie", n.cookie),
		utils.WithRandomIPHeader(),
	}

	lyricAPI := "https://music.163.com/weapi/song/lyric"
	body, err := utils.Post(lyricAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return "", err
	}

	var resp struct {
		Code int `json:"code"`
		Lrc  struct {
			Lyric string `json:"lyric"`
		} `json:"lrc"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("json parse error: %w", err)
	}
	if resp.Code != 200 {
		return "", fmt.Errorf("netease api error code: %d", resp.Code)
	}
	return resp.Lrc.Lyric, nil
}

// GetRecommendedPlaylists returns homepage recommended playlists.
func (n *Netease) GetRecommendedPlaylists() ([]model.Playlist, error) {
	reqData := map[string]interface{}{
		"limit": 30,
		"total": true,
		"n":     1000,
	}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	headers := []utils.RequestOption{
		utils.WithHeader("Referer", Referer),
		utils.WithHeader("Content-Type", "application/x-www-form-urlencoded"),
		utils.WithRandomIPHeader(),
	}

	body, err := utils.Post(RecommendedPlaylistAPI, strings.NewReader(form.Encode()), headers...)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code   int `json:"code"`
		Result []struct {
			ID         int     `json:"id"`
			Name       string  `json:"name"`
			PicURL     string  `json:"picUrl"`
			PlayCount  float64 `json:"playCount"`
			TrackCount int     `json:"trackCount"`
			Copywriter string  `json:"copywriter"`
			Alg        string  `json:"alg"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease recommended playlist json parse error: %w", err)
	}
	if resp.Code != 200 {
		return nil, fmt.Errorf("netease api error code: %d", resp.Code)
	}

	var playlists []model.Playlist
	for _, item := range resp.Result {
		creatorDisplay := "网易云推荐"
		if item.Copywriter != "" {
			creatorDisplay = item.Copywriter
		}

		pl := model.Playlist{
			Source:      "netease",
			ID:          strconv.Itoa(item.ID),
			Name:        item.Name,
			Cover:       item.PicURL,
			PlayCount:   int(item.PlayCount),
			TrackCount:  item.TrackCount,
			Description: item.Copywriter,
			Creator:     creatorDisplay,
			Link:        fmt.Sprintf("https://music.163.com/#/playlist?id=%d", item.ID),
			Extra:       map[string]string{},
		}

		if item.Alg != "" {
			pl.Extra["alg"] = item.Alg
		}

		playlists = append(playlists, pl)
	}

	return playlists, nil
}
