package apple

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

const (
	// AppleChartAPI 是 Apple 营销工具提供的热门歌曲排行（Top 100）接口，无需登录。
	// 数据来自 Apple Music 各国家/地区商店的「Top 100」，最多返回 100 首。
	AppleChartAPI = "https://rss.marketingtools.apple.com/api/v2"
	// AppleChartAPIFallback 是同一个榜单接口的备用域名，主域名超时或 5xx 时切换。
	AppleChartAPIFallback = "https://rss.applemarketingtools.com/api/v2"
	// AppleLookupAPI 是 iTunes 公开查询接口，用于补全榜单歌曲的专辑、时长和试听地址。
	AppleLookupAPI = "https://itunes.apple.com/lookup"
	// AppleChartFeed 是榜单类型：most-played 即 Apple 官方「Top 100」。
	AppleChartFeed = "most-played"
	// AppleChartMedia 是榜单媒体类型：目前接入歌曲榜。
	AppleChartMedia = "songs"
	// AppleChartPageFormat 是 Apple Music 官方排行榜页面地址模板（参数为 storefront）。
	AppleChartPageFormat = "https://music.apple.com/%s/new/top-charts"

	// DefaultTopListLimit 榜单默认返回条数。
	DefaultTopListLimit = 100
	// MaxTopListLimit 榜单最大返回条数（接口上限 100）。
	MaxTopListLimit = 100
	// DefaultHotSongLimit 热门单曲默认条数。
	DefaultHotSongLimit = 100
	// MaxHotSongLimit 热门单曲最大条数。
	MaxHotSongLimit = 100

	// appleChartRequestAttempts 榜单接口最大重试轮数（接口高峰期会返回 5xx）。
	appleChartRequestAttempts = 3
	// appleChartRequestTimeout 榜单接口单次请求超时。
	appleChartRequestTimeout = 20 * time.Second
	// appleChartTotalTimeout 榜单接口整体耗时预算，避免限流时页面长时间挂起。
	appleChartTotalTimeout = 45 * time.Second
	// appleChartRetryDelay 榜单接口重试间隔基数。
	appleChartRetryDelay = 400 * time.Millisecond
	// appleLookupMaxIDs iTunes 查询单次最多提交的歌曲 ID 数量。
	appleLookupMaxIDs = 100
)

// appleChartStorefront 是内置榜单对应的国家/地区商店。
type appleChartStorefront struct {
	Code string
	Name string
}

// appleChartStorefronts 是 Apple Music 各国家/地区的 Top 100 歌曲榜。
var appleChartStorefronts = []appleChartStorefront{
	{Code: "cn", Name: "中国内地"},
	{Code: "hk", Name: "中国香港"},
	{Code: "tw", Name: "中国台湾"},
	{Code: "us", Name: "美国"},
	{Code: "jp", Name: "日本"},
	{Code: "kr", Name: "韩国"},
	{Code: "gb", Name: "英国"},
	{Code: "sg", Name: "新加坡"},
}

// appleChartRef 描述一个榜单：榜单类型 + 媒体类型 + 国家/地区商店。
type appleChartRef struct {
	Feed       string
	Media      string
	Storefront string
}

func (r appleChartRef) ID() string {
	return r.Feed + ":" + r.Media + ":" + r.Storefront
}

func (r appleChartRef) PageURL() string {
	return fmt.Sprintf(AppleChartPageFormat, r.Storefront)
}

func (r appleChartRef) FeedURL(host string, limit int) string {
	return fmt.Sprintf("%s/%s/music/%s/%d/%s.json", strings.TrimRight(host, "/"), r.Storefront, r.Feed, limit, r.Media)
}

func GetTopLists() ([]model.TopList, error) {
	return defaultApple.GetTopLists()
}

func GetTopListSongs(topListID string) ([]model.Song, error) {
	return defaultApple.GetTopListSongs(topListID)
}

func GetHotSongs(limit int) ([]model.Song, error) {
	return defaultApple.GetHotSongs(limit)
}

// GetTopLists 返回 Apple Music 各国家/地区的官方 Top 100 歌曲榜（无需登录）。
// 榜单目录是内置的，打开页面时不会请求接口；榜单歌曲在点击时按需拉取。
func (a *Apple) GetTopLists() ([]model.TopList, error) {
	topLists := make([]model.TopList, 0, len(appleChartStorefronts))
	for _, store := range appleChartStorefronts {
		ref := appleChartRef{Feed: AppleChartFeed, Media: AppleChartMedia, Storefront: store.Code}
		topLists = append(topLists, model.TopList{
			ID:              ref.ID(),
			Name:            store.Name + " Top 100",
			Description:     "Apple Music · " + store.Name + "热门歌曲 Top 100",
			TrackCount:      MaxTopListLimit,
			Source:          "apple",
			Link:            ref.PageURL(),
			UpdateFrequency: "每日更新",
			Extra: map[string]string{
				"storefront":   store.Code,
				"chart":        ref.Feed,
				"media":        ref.Media,
				"toplist_type": "chart",
			},
		})
	}
	if len(topLists) == 0 {
		return nil, errors.New("apple toplist index is empty")
	}
	return topLists, nil
}

// GetTopListSongs 拉取指定榜单的全部歌曲，名次写入 Song.Extra["rank"]。
// 榜单 ID 支持 "most-played:songs:cn"、"songs:cn"，以及直接传商店代码 "cn"。
func (a *Apple) GetTopListSongs(topListID string) ([]model.Song, error) {
	ref, err := parseAppleChartID(topListID)
	if err != nil {
		return nil, err
	}
	return a.fetchAppleChartSongs(ref, MaxTopListLimit)
}

// GetHotSongs 返回当前实例商店的 Top 100 歌曲榜，作为 Apple Music 的热门单曲。
func (a *Apple) GetHotSongs(limit int) ([]model.Song, error) {
	ref := appleChartRef{
		Feed:       AppleChartFeed,
		Media:      AppleChartMedia,
		Storefront: a.chartStorefront(),
	}
	return a.fetchAppleChartSongs(ref, normalizeAppleHotSongLimit(limit))
}

func (a *Apple) chartStorefront() string {
	storefront := strings.ToLower(strings.TrimSpace(a.storefront))
	if storefront == "" {
		return defaultStorefront
	}
	return storefront
}

func (a *Apple) fetchAppleChartSongs(ref appleChartRef, limit int) ([]model.Song, error) {
	feed, err := fetchAppleChartFeed(ref, MaxTopListLimit)
	if err != nil {
		return nil, err
	}

	items := feed.Feed.Results
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("apple toplist %s is empty", ref.ID())
	}

	// iTunes 查询用于补全专辑、时长和试听地址，失败时仍返回榜单基础信息。
	details := a.fetchAppleLookupDetails(ref.Storefront, items)

	songs := make([]model.Song, 0, len(items))
	for i, item := range items {
		song := appleChartSong(item, details[strings.TrimSpace(item.ID)], ref)
		if song.Extra == nil {
			song.Extra = make(map[string]string, 4)
		}
		song.Extra["rank"] = strconv.Itoa(i + 1)
		song.Extra["toplist_id"] = ref.ID()
		songs = append(songs, song)
	}
	return songs, nil
}

// fetchAppleChartFeed 拉取榜单接口，遇到 5xx/超时时按域名与次数重试。
func fetchAppleChartFeed(ref appleChartRef, limit int) (*appleChartFeedResponse, error) {
	var lastErr error
	deadline := time.Now().Add(appleChartTotalTimeout)

	for attempt := 0; attempt < appleChartRequestAttempts; attempt++ {
		for _, host := range appleChartHosts {
			body, err := appleChartGet(ref.FeedURL(host, limit))
			if err == nil {
				var resp appleChartFeedResponse
				if err = json.Unmarshal(body, &resp); err == nil {
					if len(resp.Feed.Results) == 0 {
						err = errors.New("apple chart feed is empty")
					} else {
						return &resp, nil
					}
				}
			}
			lastErr = err
		}
		if attempt < appleChartRequestAttempts-1 && time.Now().Before(deadline) {
			time.Sleep(time.Duration(attempt+1) * appleChartRetryDelay)
		}
		if !time.Now().Before(deadline) {
			break
		}
	}

	if lastErr == nil {
		lastErr = errors.New("apple chart request failed")
	}
	return nil, fmt.Errorf("apple toplist %s request failed: %w", ref.ID(), lastErr)
}

// appleChartGet 使用带超时的独立客户端请求榜单与 iTunes 查询接口。
func appleChartGet(rawURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", appleHomepageURL+"/")

	resp, err := appleChartHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("apple chart http error: status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// fetchAppleLookupDetails 用 iTunes 查询补全榜单歌曲的详细信息，失败时返回空结果。
func (a *Apple) fetchAppleLookupDetails(storefront string, items []appleChartFeedItem) map[string]appleLookupItem {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if id := strings.TrimSpace(item.ID); id != "" {
			ids = append(ids, id)
		}
	}

	details := make(map[string]appleLookupItem, len(ids))
	for start := 0; start < len(ids); start += appleLookupMaxIDs {
		end := start + appleLookupMaxIDs
		if end > len(ids) {
			end = len(ids)
		}

		params := url.Values{}
		params.Set("id", strings.Join(ids[start:end], ","))
		params.Set("country", storefront)
		params.Set("entity", "song")
		params.Set("limit", "200")

		body, err := appleChartGet(appleLookupAPI + "?" + params.Encode())
		if err != nil {
			continue
		}

		var resp appleLookupResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			continue
		}
		for _, item := range resp.Results {
			if item.TrackID == 0 {
				continue
			}
			details[strconv.FormatInt(item.TrackID, 10)] = item
		}
	}
	return details
}

var appleStorefrontPattern = regexp.MustCompile(`^[a-z]{2}$`)

// appleChartHosts 是榜单接口的候选域名，测试时可替换为本地服务。
var appleChartHosts = []string{AppleChartAPI, AppleChartAPIFallback}

// appleLookupAPI 是 iTunes 查询接口地址，测试时可替换为本地服务。
var appleLookupAPI = AppleLookupAPI

// appleChartHTTPClient 是榜单与 iTunes 查询专用的 HTTP 客户端。
var appleChartHTTPClient = &http.Client{Timeout: appleChartRequestTimeout}

func parseAppleChartID(topListID string) (appleChartRef, error) {
	trimmed := strings.ToLower(strings.TrimSpace(topListID))
	if trimmed == "" {
		return appleChartRef{}, errors.New("apple toplist id is empty")
	}

	ref := appleChartRef{Feed: AppleChartFeed, Media: AppleChartMedia}
	parts := strings.Split(trimmed, ":")
	switch len(parts) {
	case 1:
		ref.Storefront = parts[0]
	case 2:
		ref.Media = parts[0]
		ref.Storefront = parts[1]
	case 3:
		ref.Feed = parts[0]
		ref.Media = parts[1]
		ref.Storefront = parts[2]
	default:
		return appleChartRef{}, fmt.Errorf("invalid apple toplist id: %s", topListID)
	}

	if ref.Feed != AppleChartFeed {
		return appleChartRef{}, fmt.Errorf("unsupported apple toplist feed: %s", ref.Feed)
	}
	if ref.Media != AppleChartMedia {
		return appleChartRef{}, fmt.Errorf("unsupported apple toplist media: %s", ref.Media)
	}
	if !appleStorefrontPattern.MatchString(ref.Storefront) {
		return appleChartRef{}, fmt.Errorf("invalid apple toplist storefront: %s", topListID)
	}
	return ref, nil
}

func normalizeAppleHotSongLimit(limit int) int {
	if limit <= 0 {
		return DefaultHotSongLimit
	}
	if limit > MaxHotSongLimit {
		return MaxHotSongLimit
	}
	return limit
}

// appleChartSong 把榜单条目与 iTunes 详情合成为通用歌曲结构。
func appleChartSong(item appleChartFeedItem, detail appleLookupItem, ref appleChartRef) model.Song {
	song := model.Song{
		Source: "apple",
		ID:     strings.TrimSpace(item.ID),
		Name:   appleFirstNonEmpty(item.Name, detail.TrackName),
		Artist: appleFirstNonEmpty(item.ArtistName, detail.ArtistName),
		Album:  strings.TrimSpace(detail.CollectionName),
		Cover:  appleArtworkUpgrade(appleFirstNonEmpty(detail.ArtworkURL100, item.ArtworkURL100), 600),
		URL:    strings.TrimSpace(detail.PreviewURL),
		Ext:    "m4a",
		Link:   appleFirstNonEmpty(appleStripLookupParam(detail.TrackViewURL), strings.TrimSpace(item.URL)),
		Extra: map[string]string{
			"storefront":   ref.Storefront,
			"chart":        ref.Feed,
			"release_date": strings.TrimSpace(item.ReleaseDate),
		},
	}
	if detail.TrackTimeMillis > 0 {
		song.Duration = detail.TrackTimeMillis / 1000
	}
	if detail.CollectionID != 0 {
		song.AlbumID = strconv.FormatInt(detail.CollectionID, 10)
	}
	if genre := strings.TrimSpace(detail.PrimaryGenreName); genre != "" {
		song.Extra["genre"] = genre
	}
	return song
}

func appleFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// appleArtworkUpgrade 把 100x100 的封面地址放大到指定尺寸。
func appleArtworkUpgrade(raw string, size int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if size <= 0 {
		size = 600
	}
	re := regexp.MustCompile(`/\d+x\d+bb\.(jpg|png|webp)`)
	return re.ReplaceAllString(raw, fmt.Sprintf("/%dx%dbb.$1", size, size))
}

// appleStripLookupParam 去掉 iTunes 查询链接上的 uo 参数。
func appleStripLookupParam(link string) string {
	link = strings.TrimSpace(link)
	if link == "" {
		return ""
	}
	re := regexp.MustCompile(`[?&]uo=\d+`)
	cleaned := re.ReplaceAllString(link, "")
	cleaned = strings.ReplaceAll(cleaned, "?&", "?")
	return strings.TrimSuffix(strings.TrimSuffix(cleaned, "?"), "&")
}

// --- 榜单接口响应结构 ---

type appleChartFeedResponse struct {
	Feed struct {
		Title   string               `json:"title"`
		Country string               `json:"country"`
		Updated string               `json:"updated"`
		Results []appleChartFeedItem `json:"results"`
	} `json:"feed"`
}

type appleChartFeedItem struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ArtistName    string `json:"artistName"`
	Kind          string `json:"kind"`
	ReleaseDate   string `json:"releaseDate"`
	ArtworkURL100 string `json:"artworkUrl100"`
	URL           string `json:"url"`
}

type appleLookupResponse struct {
	ResultCount int               `json:"resultCount"`
	Results     []appleLookupItem `json:"results"`
}

type appleLookupItem struct {
	WrapperType      string `json:"wrapperType"`
	Kind             string `json:"kind"`
	TrackID          int64  `json:"trackId"`
	TrackName        string `json:"trackName"`
	ArtistName       string `json:"artistName"`
	CollectionID     int64  `json:"collectionId"`
	CollectionName   string `json:"collectionName"`
	TrackTimeMillis  int    `json:"trackTimeMillis"`
	PreviewURL       string `json:"previewUrl"`
	ArtworkURL100    string `json:"artworkUrl100"`
	TrackViewURL     string `json:"trackViewUrl"`
	PrimaryGenreName string `json:"primaryGenreName"`
	ReleaseDate      string `json:"releaseDate"`
}
