package joox

import (
	"crypto/md5"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strings"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	jooxQualityProbeKeyword = "周杰伦"
	jooxOpenTrackSalt       = "Jo0x@t3Nc3nT"
)

type jooxSongDetailResponse struct {
	Msong     string      `json:"msong"`
	Msinger   string      `json:"msinger"`
	Malbum    string      `json:"malbum"`
	Img       string      `json:"img"`
	MInterval int         `json:"minterval"`
	R320URL   string      `json:"r320Url"`
	R192URL   string      `json:"r192Url"`
	Mp3URL    string      `json:"mp3Url"`
	M4aURL    string      `json:"m4aUrl"`
	KbpsMap   interface{} `json:"kbps_map"`
}

type jooxOpenTrackResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	AlbumName    string `json:"album_name"`
	PlayDuration int    `json:"play_duration"`
	LrcExist     int    `json:"lrc_exist"`
	LrcContent   string `json:"lrc_content"`
	HasHiFi      int    `json:"has_hifi"`
	HasHQ        int    `json:"has_hq"`
	VipFlag      int    `json:"vip_flag"`
	ArtistList   []struct {
		Name string `json:"name"`
	} `json:"artist_list"`
	Images []struct {
		Width int    `json:"width"`
		URL   string `json:"url"`
	} `json:"images"`
	PlayURLList []string `json:"play_url_list"`
}

type jooxSongDetail struct {
	songID   string
	open     *jooxOpenTrackResponse
	openURLs map[model.AudioQuality]string
	resp     jooxSongDetailResponse
	kbps     map[string]interface{}
}

type jooxQualityCandidate struct {
	option model.AudioQualityOption
	key    string
	url    func(jooxSongDetailResponse) string
}

type jooxSignedParam struct {
	key   string
	value interface{}
}

var jooxQualityCandidates = []jooxQualityCandidate{
	{
		option: model.AudioQualityOption{
			Value:  model.AudioQualityLossless,
			Label:  "无损音质",
			Format: "flac",
			VIP:    true,
		},
	},
	{
		option: model.AudioQualityOption{
			Value:   model.AudioQualityHigh,
			Label:   "高品质",
			Format:  "mp3",
			Bitrate: 320,
		},
		key: "320",
		url: func(resp jooxSongDetailResponse) string { return resp.R320URL },
	},
	{
		option: model.AudioQualityOption{
			Value:   model.AudioQualityHigher,
			Label:   "较高品质",
			Format:  "mp3",
			Bitrate: 192,
		},
		key: "192",
		url: func(resp jooxSongDetailResponse) string { return resp.R192URL },
	},
	{
		option: model.AudioQualityOption{
			Value:   model.AudioQualityStandard,
			Label:   "标准音质",
			Format:  "m4a",
			Bitrate: 128,
		},
		key: "128",
		url: func(resp jooxSongDetailResponse) string { return resp.Mp3URL },
	},
	{
		option: model.AudioQualityOption{
			Value:   model.AudioQualityLow,
			Label:   "低品质",
			Format:  "m4a",
			Bitrate: 64,
		},
		key: "96",
		url: func(resp jooxSongDetailResponse) string { return resp.M4aURL },
	},
}

// ListQualityOptions returns every JOOX quality tier understood by either the
// current open API or the legacy web_get_songinfo response.
func ListQualityOptions() []model.AudioQualityOption {
	options := make([]model.AudioQualityOption, 0, len(jooxQualityCandidates))
	for _, candidate := range jooxQualityCandidates {
		options = append(options, candidate.option)
	}
	return options
}

// AvailableQualityOptions checks the configured JOOX account against a real
// catalog track and reports only URLs returned by the playable track response.
func (j *Joox) AvailableQualityOptions() ([]model.AudioQualityOption, error) {
	songs, err := j.Search(jooxQualityProbeKeyword)
	if err != nil {
		return nil, err
	}
	if len(songs) == 0 {
		return nil, errors.New("joox quality probe found no songs")
	}

	var detail *jooxSongDetail
	var lastErr error
	for i := 0; i < len(songs) && i < 5; i++ {
		detail, lastErr = j.fetchSongDetail(songs[i].ID)
		if lastErr == nil && len(detail.availableQualityOptions()) > 0 {
			break
		}
	}
	if detail == nil {
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, errors.New("joox quality probe returned no song details")
	}

	options := detail.availableQualityOptions()
	if len(options) == 0 {
		return nil, errors.New("joox returned no playable quality")
	}
	return options, nil
}

// GetDownloadURLWithQuality resolves one exact JOOX tier. The shared core
// fallback is responsible for moving to lower tiers when a request fails.
func (j *Joox) GetDownloadURLWithQuality(song *model.Song, quality string) (string, error) {
	normalized, ok := model.NormalizeAudioQuality(quality)
	if !ok {
		return "", fmt.Errorf("unsupported joox audio quality: %s", quality)
	}
	if song == nil {
		return "", errors.New("song is nil")
	}
	if !strings.EqualFold(strings.TrimSpace(song.Source), "joox") {
		return "", errors.New("source mismatch")
	}
	if normalized == model.AudioQualityAuto {
		return j.GetDownloadURL(song)
	}

	candidate, ok := jooxQualityCandidateFor(normalized)
	if !ok {
		return "", fmt.Errorf("unsupported joox audio quality: %s", quality)
	}
	songID := strings.TrimSpace(song.ID)
	if song.Extra != nil && strings.TrimSpace(song.Extra["songid"]) != "" {
		songID = strings.TrimSpace(song.Extra["songid"])
	}
	if songID == "" {
		return "", errors.New("joox song id is empty")
	}

	detail, err := j.fetchSongDetail(songID)
	if err != nil {
		return "", err
	}
	streamURL := detail.candidateURL(candidate)
	if streamURL == "" {
		return "", fmt.Errorf("joox audio quality %s is unavailable", normalized)
	}
	return streamURL, nil
}

func (j *Joox) fetchSongDetail(songID string) (*jooxSongDetail, error) {
	songID = strings.TrimSpace(songID)
	if songID == "" {
		return nil, errors.New("joox song id is empty")
	}

	openResp, openErr := j.fetchOpenTrack(songID)
	if openErr == nil {
		detail := newJOOXOpenSongDetail(songID, openResp)
		if detail.bestURL() != "" {
			return detail, nil
		}
	}

	legacyDetail, legacyErr := j.fetchLegacySongDetail(songID)
	if legacyErr == nil && legacyDetail.bestURL() != "" {
		return legacyDetail, nil
	}
	if openErr == nil && legacyErr == nil {
		return nil, errors.New("joox returned no playable stream")
	}
	if openErr == nil {
		return nil, fmt.Errorf("joox legacy song info: %w", legacyErr)
	}
	if legacyErr == nil {
		return nil, fmt.Errorf("joox track: %w; legacy returned no playable stream", openErr)
	}
	return nil, fmt.Errorf("joox track: %v; legacy song info: %w", openErr, legacyErr)
}

func (j *Joox) fetchOpenTrack(songID string) (*jooxOpenTrackResponse, error) {
	cookieValues := jooxCookieValues(j.cookie)
	country := "sg"
	lang := "zh_cn"
	uid := firstNonEmpty(cookieValues["wmid"], "142420656")
	usk := firstNonEmpty(
		cookieValues["session_key"],
		"2a5d97d05dc8fe238150184eaf3519ad",
	)

	signed := []jooxSignedParam{
		{key: "country", value: country},
		{key: "lang", value: lang},
		{key: "lyric", value: 1},
		{key: "fs", value: 1},
		{key: "im", value: 0},
		{key: "uid", value: uid},
		{key: "usk", value: usk},
		{key: "id", value: songID},
	}
	secret := jooxTrackSecret(signed)

	queryParts := make([]string, 0, len(signed))
	for _, param := range signed[:len(signed)-1] {
		queryParts = append(
			queryParts,
			param.key+"="+jooxPercentEncode(fmt.Sprint(param.value)),
		)
	}
	queryParts = append(queryParts, "secret="+secret)

	pathID := strings.ReplaceAll(songID, "/", "_")
	apiURL := fmt.Sprintf(
		"https://cache.api.joox.com/openjoox2/v1/track/%s?%s",
		pathID,
		strings.Join(queryParts, "&"),
	)
	body, err := utils.Get(
		apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Accept", "application/json, text/plain, */*"),
		utils.WithHeader("Origin", "https://www.joox.com"),
		utils.WithHeader("Referer", "https://www.joox.com/"),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	var resp jooxOpenTrackResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("joox open track json error: %w", err)
	}
	if strings.TrimSpace(resp.ID) == "" && strings.TrimSpace(resp.Name) == "" {
		return nil, errors.New("joox open track returned an empty response")
	}
	return &resp, nil
}

func (j *Joox) fetchLegacySongDetail(songID string) (*jooxSongDetail, error) {
	params := url.Values{}
	params.Set("songid", songID)
	params.Set("lang", "zh_cn")
	params.Set("country", "sg")

	apiURL := "https://api.joox.com/web-fcgi-bin/web_get_songinfo?" + params.Encode()
	body, err := utils.Get(
		apiURL,
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Cookie", j.cookie),
		utils.WithHeader("X-Forwarded-For", XForwardedFor),
	)
	if err != nil {
		return nil, err
	}

	bodyStr := strings.TrimSpace(string(body))
	if strings.HasPrefix(bodyStr, "MusicInfoCallback(") {
		bodyStr = strings.TrimPrefix(bodyStr, "MusicInfoCallback(")
		bodyStr = strings.TrimSuffix(bodyStr, ";")
		bodyStr = strings.TrimSuffix(bodyStr, ")")
	}

	var resp jooxSongDetailResponse
	if err := json.Unmarshal([]byte(bodyStr), &resp); err != nil {
		return nil, fmt.Errorf("joox detail json error: %w", err)
	}
	return &jooxSongDetail{
		songID: songID,
		resp:   resp,
		kbps:   jooxKbpsMap(resp.KbpsMap),
	}, nil
}

func newJOOXOpenSongDetail(songID string, resp *jooxOpenTrackResponse) *jooxSongDetail {
	detail := &jooxSongDetail{
		songID:   songID,
		open:     resp,
		openURLs: make(map[model.AudioQuality]string),
	}
	for _, rawURL := range resp.PlayURLList {
		streamURL := strings.TrimSpace(html.UnescapeString(rawURL))
		if streamURL == "" {
			continue
		}
		quality, ok := jooxQualityFromStreamURL(streamURL)
		if !ok {
			continue
		}
		if detail.openURLs[quality] == "" {
			detail.openURLs[quality] = streamURL
		}
	}
	if len(detail.openURLs) == 0 && len(resp.PlayURLList) > 0 {
		detail.openURLs[model.AudioQualityHigh] = strings.TrimSpace(
			html.UnescapeString(resp.PlayURLList[0]),
		)
	}
	return detail
}

func (detail *jooxSongDetail) song() *model.Song {
	name := detail.resp.Msong
	artist := detail.resp.Msinger
	album := detail.resp.Malbum
	duration := detail.resp.MInterval
	cover := detail.resp.Img
	if detail.open != nil {
		name = detail.open.Name
		album = detail.open.AlbumName
		duration = detail.open.PlayDuration
		artists := make([]string, 0, len(detail.open.ArtistList))
		for _, item := range detail.open.ArtistList {
			if strings.TrimSpace(item.Name) != "" {
				artists = append(artists, strings.TrimSpace(item.Name))
			}
		}
		artist = strings.Join(artists, " / ")
		cover = jooxOpenCover(detail.open)
	}

	return &model.Song{
		Source:   "joox",
		ID:       detail.songID,
		Name:     name,
		Artist:   artist,
		Album:    album,
		Duration: duration,
		Cover:    cover,
		URL:      detail.bestURL(),
		Link:     fmt.Sprintf("https://www.joox.com/hk/single/%s", detail.songID),
		Extra: map[string]string{
			"songid": detail.songID,
		},
	}
}

func (detail *jooxSongDetail) bestURL() string {
	for _, candidate := range jooxQualityCandidates {
		if streamURL := detail.candidateURL(candidate); streamURL != "" {
			return streamURL
		}
	}
	if detail.open != nil {
		for _, rawURL := range detail.open.PlayURLList {
			if streamURL := strings.TrimSpace(html.UnescapeString(rawURL)); streamURL != "" {
				return streamURL
			}
		}
	}
	return firstNonEmpty(
		detail.resp.R320URL,
		detail.resp.R192URL,
		detail.resp.Mp3URL,
		detail.resp.M4aURL,
	)
}

func (detail *jooxSongDetail) candidateURL(candidate jooxQualityCandidate) string {
	if detail.open != nil {
		return strings.TrimSpace(detail.openURLs[candidate.option.Value])
	}
	if candidate.url == nil {
		return ""
	}

	streamURL := strings.TrimSpace(candidate.url(detail.resp))
	if streamURL == "" {
		return ""
	}
	if len(detail.kbps) == 0 {
		return streamURL
	}
	value, exists := detail.kbps[candidate.key]
	if !exists || !jooxKbpsValueAvailable(value) {
		return ""
	}
	return streamURL
}

func jooxQualityFromStreamURL(streamURL string) (model.AudioQuality, bool) {
	upper := strings.ToUpper(streamURL)
	switch {
	case strings.Contains(upper, "/F000") || strings.Contains(upper, ".FLAC"):
		return model.AudioQualityLossless, true
	case strings.Contains(upper, "/M800"):
		return model.AudioQualityHigh, true
	case strings.Contains(upper, "/M400"):
		return model.AudioQualityHigher, true
	case strings.Contains(upper, "/M500"), strings.Contains(upper, "/C400"):
		return model.AudioQualityStandard, true
	case strings.Contains(upper, "/C200"), strings.Contains(upper, "/M200"):
		return model.AudioQualityLow, true
	default:
		return "", false
	}
}

func jooxTrackSecret(params []jooxSignedParam) string {
	parts := make([]string, 0, len(params))
	for _, param := range params {
		parts = append(
			parts,
			param.key+"="+jooxPercentEncode(fmt.Sprint(param.value)),
		)
	}
	sum := md5.Sum([]byte(jooxOpenTrackSalt + strings.Join(parts, "&")))
	return fmt.Sprintf("%x", sum)
}

func jooxPercentEncode(value string) string {
	const safe = "!*'()"
	const hex = "0123456789ABCDEF"
	var builder strings.Builder
	for _, b := range []byte(value) {
		if (b >= 'a' && b <= 'z') ||
			(b >= 'A' && b <= 'Z') ||
			(b >= '0' && b <= '9') ||
			b == '-' || b == '.' || b == '_' || b == '~' ||
			strings.ContainsRune(safe, rune(b)) {
			builder.WriteByte(b)
			continue
		}
		builder.WriteByte('%')
		builder.WriteByte(hex[b>>4])
		builder.WriteByte(hex[b&0x0f])
	}
	return builder.String()
}

func jooxCookieValues(cookie string) map[string]string {
	values := make(map[string]string)
	for _, item := range strings.Split(cookie, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(item), "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return values
}

func jooxOpenCover(resp *jooxOpenTrackResponse) string {
	if len(resp.Images) == 0 {
		return ""
	}
	for _, image := range resp.Images {
		if image.Width == 300 && strings.TrimSpace(image.URL) != "" {
			return strings.TrimSpace(image.URL)
		}
	}
	return strings.TrimSpace(resp.Images[0].URL)
}

func jooxKbpsMap(raw interface{}) map[string]interface{} {
	switch value := raw.(type) {
	case string:
		var parsed map[string]interface{}
		if json.Unmarshal([]byte(value), &parsed) == nil {
			return parsed
		}
	case map[string]interface{}:
		return value
	}
	return nil
}

func jooxKbpsValueAvailable(value interface{}) bool {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != "" && strings.TrimSpace(typed) != "0"
	case float64:
		return typed > 0
	case float32:
		return typed > 0
	case int:
		return typed > 0
	case int64:
		return typed > 0
	case json.Number:
		number, err := typed.Int64()
		return err == nil && number > 0
	default:
		return value != nil
	}
}

func jooxQualityCandidateFor(quality model.AudioQuality) (jooxQualityCandidate, bool) {
	for _, candidate := range jooxQualityCandidates {
		if candidate.option.Value == quality {
			return candidate, true
		}
	}
	return jooxQualityCandidate{}, false
}

func (detail *jooxSongDetail) availableQualityOptions() []model.AudioQualityOption {
	options := make([]model.AudioQualityOption, 0, len(jooxQualityCandidates))
	for _, candidate := range jooxQualityCandidates {
		if detail.candidateURL(candidate) != "" {
			options = append(options, candidate.option)
		}
	}
	return options
}
