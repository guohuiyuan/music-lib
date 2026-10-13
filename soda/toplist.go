package soda

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	// TopListAPI 汽水音乐 PC 端榜单详情接口，无需登录（缺失 x-luna-* 头也可用）。
	TopListAPI = "https://api.qishui.com/luna/pc/charts"
	// TopListFallbackAPI 汽水音乐移动端榜单详情接口，作为 PC 接口失败时的回退。
	TopListFallbackAPI = "https://api5-lf.qishui.com/luna/charts"
	// RecommendPlaylistsAPI 汽水音乐推荐歌单接口（发现页歌单流），无需登录。
	RecommendPlaylistsAPI = "https://api.qishui.com/luna/pc/discover/mix"
	// HotSongTopListID 汽水音乐热歌榜 ID，作为「热门单曲」来源。
	HotSongTopListID = "7036274230471712007"
	// DefaultHotSongLimit 热门单曲默认返回条数。
	DefaultHotSongLimit = 100
	// MaxHotSongLimit 热门单曲最大返回条数。
	MaxHotSongLimit = 200
	// sodaChartTopListGroup 汽水音乐官方榜单分组名。
	sodaChartTopListGroup = "汽水官方榜"
	// sodaRecommendPlaylistGroup 汽水音乐推荐歌单分组名。
	sodaRecommendPlaylistGroup = "汽水推荐歌单"
	// sodaTopListFrequency 汽水官方榜单更新频率。
	sodaTopListFrequency = "每周四更新"
	// sodaRecommendChannelCount 拉取推荐歌单的频道数量（sub_channel_id 依次取 0..N-1）。
	sodaRecommendChannelCount = 10
	// sodaRecommendTopListLimit 榜单目录里最多聚合的推荐歌单数量。
	sodaRecommendTopListLimit = 100
)

// sodaTopListDefinition 是汽水音乐内置的官方榜单定义。
// 汽水 PC 端没有公开的榜单目录接口（/luna/pc/charts 只接受数字榜单 ID，
// 传其它路径都会返回 ERR_INVALID_PARAM），因此这里内置官方榜单列表。
type sodaTopListDefinition struct {
	ID          string
	Name        string
	Description string
	Cover       string
	TrackCount  int
}

var sodaTopListDefinitions = []sodaTopListDefinition{
	{
		ID:          "7036274230471712007",
		Name:        "热歌榜",
		Description: "汽水音乐内每周热度最高的50首歌，每周四更新",
		Cover:       "https://p3-luna.douyinpic.com/img/tos-cn-i-b829550vbb/d0d8d48461a62748e84689cdf049b19a.png~tplv-b829550vbb-resize:960:960.png",
		TrackCount:  50,
	},
	{
		ID:          "7060812597884869927",
		Name:        "新歌榜",
		Description: "近期发行的热度最高的50首新歌，每周四更新",
		Cover:       "https://p3-luna.douyinpic.com/img/tos-cn-i-b829550vbb/f12f7eb5b54d0899c7c724df009668a8.png~tplv-b829550vbb-resize:960:960.png",
		TrackCount:  50,
	},
	{
		ID:          "7061475546400005410",
		Name:        "欧美榜",
		Description: "汽水音乐内每周热度最高的50首外文歌曲，每周四更新",
		Cover:       "https://p3-luna.douyinpic.com/img/tos-cn-i-b829550vbb/33747550ed5499b58feda42a21748637.png~tplv-b829550vbb-resize:960:960.png",
		TrackCount:  50,
	},
	{
		ID:          "7415959718721494311",
		Name:        "音乐人歌曲榜",
		Description: "抖音音乐人开放平台上传歌曲，综合每周站内热度进行排序展示",
		Cover:       "https://p3-luna.douyinpic.com/img/tos-cn-v-2774c002/o8FQKiQQBxHWa2hzsBNAgYOX6iEHEAibADAbfB~tplv-b829550vbb-resize:960:960.png",
		TrackCount:  25,
	},
}

func GetTopLists() ([]model.TopList, error) {
	return defaultSoda.GetTopLists()
}

func GetTopListSongs(topListID string) ([]model.Song, error) {
	return defaultSoda.GetTopListSongs(topListID)
}

func GetHotSongs(limit int) ([]model.Song, error) {
	return defaultSoda.GetHotSongs(limit)
}

// GetTopLists 获取汽水音乐榜单目录（无需登录）：内置官方榜单 + 发现页推荐歌单。
// 推荐歌单接口不可用时静默降级为只返回官方榜单。
func (s *Soda) GetTopLists() ([]model.TopList, error) {
	topLists := sodaBuiltinTopLists()
	if recommended := s.fetchSodaRecommendTopLists(); len(recommended) > 0 {
		topLists = append(topLists, recommended...)
	}
	return topLists, nil
}

// sodaBuiltinTopLists 返回汽水音乐的官方榜单目录（不依赖网络）。
func sodaBuiltinTopLists() []model.TopList {
	topLists := make([]model.TopList, 0, len(sodaTopListDefinitions))
	for _, def := range sodaTopListDefinitions {
		topLists = append(topLists, model.TopList{
			ID:              def.ID,
			Name:            def.Name,
			Cover:           def.Cover,
			Description:     def.Description,
			UpdateFrequency: sodaTopListFrequency,
			TrackCount:      def.TrackCount,
			Source:          "soda",
			Link:            sodaTopListLink(def.ID),
			Extra: map[string]string{
				"chart_id":     def.ID,
				"group":        sodaChartTopListGroup,
				"toplist_type": "chart",
			},
		})
	}
	return topLists
}

// GetTopListSongs 获取指定榜单的歌曲，按榜单名次排序，名次写入歌曲 Extra["rank"]。
// 官方榜单 ID 与推荐歌单 ID 共用这套接口：官方榜单走 charts 接口，
// 推荐歌单走歌单详情接口，一条链路失败时另一条兜底。
func (s *Soda) GetTopListSongs(topListID string) ([]model.Song, error) {
	return s.fetchTopListSongs(topListID, 0)
}

// GetHotSongs 获取汽水音乐热歌榜（热门单曲），limit <= 0 时返回默认条数。
func (s *Soda) GetHotSongs(limit int) ([]model.Song, error) {
	return s.fetchTopListSongs(HotSongTopListID, normalizeSodaHotSongLimit(limit))
}

func (s *Soda) fetchTopListSongs(topListID string, limit int) ([]model.Song, error) {
	topListID = strings.TrimSpace(topListID)
	if topListID == "" {
		return nil, errors.New("soda toplist id is empty")
	}
	if !isSodaDigits(topListID) {
		return nil, fmt.Errorf("invalid soda toplist id: %s", topListID)
	}

	chartFetch := func() ([]model.Song, error) {
		return s.fetchChartTopListSongs(topListID, limit)
	}
	playlistFetch := func() ([]model.Song, error) {
		return s.fetchPlaylistTopListSongs(topListID, limit)
	}

	first, second := chartFetch, playlistFetch
	if !sodaIsBuiltinChartID(topListID) {
		first, second = playlistFetch, chartFetch
	}

	songs, firstErr := first()
	if firstErr == nil {
		return songs, nil
	}
	fallbackSongs, secondErr := second()
	if secondErr == nil {
		return fallbackSongs, nil
	}
	return nil, fmt.Errorf("soda toplist %s failed: %v (fallback: %v)", topListID, firstErr, secondErr)
}

func (s *Soda) fetchChartTopListSongs(topListID string, limit int) ([]model.Song, error) {
	chart, err := s.fetchSodaChart(topListID)
	if err != nil {
		return nil, err
	}

	songs := sodaChartSongs(chart, topListID, limit)
	if len(songs) == 0 {
		return nil, fmt.Errorf("soda toplist %s has no songs", topListID)
	}
	return songs, nil
}

func (s *Soda) fetchPlaylistTopListSongs(topListID string, limit int) ([]model.Song, error) {
	_, songs, err := s.fetchPlaylistDetail(topListID)
	if err != nil {
		return nil, err
	}

	songs = sodaRankPlaylistSongs(songs, topListID, limit)
	if len(songs) == 0 {
		return nil, fmt.Errorf("soda toplist %s has no songs", topListID)
	}
	return songs, nil
}

// sodaRankPlaylistSongs 给歌单歌曲补上榜单名次与来源榜单 ID，limit <= 0 时返回全部。
func sodaRankPlaylistSongs(songs []model.Song, topListID string, limit int) []model.Song {
	ranked := make([]model.Song, 0, len(songs))
	for i, song := range songs {
		if song.Extra == nil {
			song.Extra = make(map[string]string, 4)
		}
		song.Extra["rank"] = strconv.Itoa(i + 1)
		song.Extra["toplist_id"] = topListID

		ranked = append(ranked, song)
		if limit > 0 && len(ranked) >= limit {
			break
		}
	}
	return ranked
}

// fetchSodaChart 拉取榜单详情，优先使用 PC 接口，失败时回退到移动端接口。
func (s *Soda) fetchSodaChart(topListID string) (*sodaChart, error) {
	chart, pcErr := s.fetchSodaPCChart(topListID)
	if pcErr == nil {
		return chart, nil
	}

	chart, fallbackErr := s.fetchSodaFallbackChart(topListID)
	if fallbackErr != nil {
		return nil, fmt.Errorf("soda toplist %s failed: %w (fallback: %v)", topListID, pcErr, fallbackErr)
	}
	return chart, nil
}

func (s *Soda) fetchSodaPCChart(topListID string) (*sodaChart, error) {
	reqURL := TopListAPI + "/" + url.PathEscape(topListID) + "?" + sodaPCAppParams().Encode()
	body, err := utils.Get(reqURL, s.pcRequestOptions()...)
	if err != nil {
		return nil, err
	}

	resp, err := parseSodaChartResponse(body)
	if err != nil {
		return nil, err
	}
	return &resp.Chart, nil
}

func (s *Soda) fetchSodaFallbackChart(topListID string) (*sodaChart, error) {
	params := url.Values{}
	params.Set("charge", "0")

	opts := []utils.RequestOption{utils.WithHeader("User-Agent", UserAgent)}
	if strings.TrimSpace(s.cookie) != "" {
		opts = append(opts, utils.WithHeader("Cookie", s.cookie))
	}

	body, err := utils.Get(TopListFallbackAPI+"/"+url.PathEscape(topListID)+"?"+params.Encode(), opts...)
	if err != nil {
		return nil, err
	}

	resp, err := parseSodaChartResponse(body)
	if err != nil {
		return nil, err
	}
	return &resp.Chart, nil
}

// ==========================================
// 发现页推荐歌单
// ==========================================

type sodaDiscoverMixResponse struct {
	HasMore    bool                `json:"has_more"`
	InnerBlock []sodaDiscoverBlock `json:"inner_block"`
}

type sodaDiscoverBlock struct {
	Type         string `json:"type"`
	Title        string `json:"title"`
	InnerBlockID string `json:"inner_block_id"`
	Resources    []struct {
		Type   string `json:"type"`
		Entity struct {
			Playlist sodaUserPlaylistItem `json:"playlist"`
		} `json:"entity"`
	} `json:"resources"`
}

// fetchSodaRecommendTopLists 并发拉取发现页多个频道的推荐歌单，按频道顺序去重后返回。
// 任意频道失败都会被忽略，全部失败时返回空列表，由调用方降级为只展示官方榜单。
func (s *Soda) fetchSodaRecommendTopLists() []model.TopList {
	batches := make([][]sodaUserPlaylistItem, sodaRecommendChannelCount)

	var wg sync.WaitGroup
	for i := 0; i < sodaRecommendChannelCount; i++ {
		wg.Add(1)
		go func(channel int) {
			defer wg.Done()
			items, err := s.fetchSodaRecommendChannel(channel)
			if err == nil {
				batches[channel] = items
			}
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool)
	topLists := make([]model.TopList, 0, 32)
	for _, items := range batches {
		for _, item := range items {
			if item.ID == "" || seen[item.ID] {
				continue
			}
			seen[item.ID] = true

			topLists = append(topLists, sodaTopListFromPlaylist(item))
			if len(topLists) >= sodaRecommendTopListLimit {
				return topLists
			}
		}
	}
	return topLists
}

func (s *Soda) fetchSodaRecommendChannel(channel int) ([]sodaUserPlaylistItem, error) {
	payload := map[string]any{
		"block_type":                           "discover_playlist_mix",
		"feed_discover_extra":                  map[string]any{},
		"latest_douyin_liked_playlist_show_ts": 0,
		"sub_channel_id":                       channel,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	body, err := utils.Post(RecommendPlaylistsAPI+"?"+sodaPCAppParams().Encode(), bytes.NewReader(data),
		s.pcRequestOptions(utils.WithHeader("Content-Type", "application/json; charset=utf-8"))...,
	)
	if err != nil {
		return nil, err
	}

	return parseSodaDiscoverMixResponse(body)
}

func parseSodaDiscoverMixResponse(body []byte) ([]sodaUserPlaylistItem, error) {
	var resp sodaDiscoverMixResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda discover mix json parse error: %w", err)
	}

	items := make([]sodaUserPlaylistItem, 0, len(resp.InnerBlock))
	for _, block := range resp.InnerBlock {
		for _, resource := range block.Resources {
			if resource.Type != "" && resource.Type != "playlist" {
				continue
			}
			if resource.Entity.Playlist.ID == "" {
				continue
			}
			items = append(items, resource.Entity.Playlist)
		}
	}
	if len(items) == 0 {
		return nil, errors.New("soda discover mix has no playlists")
	}
	return items, nil
}

// sodaTopListFromPlaylist 把发现页推荐歌单转换成榜单目录条目。
func sodaTopListFromPlaylist(playlist sodaUserPlaylistItem) model.TopList {
	name := sodaFirstNonEmpty(playlist.Title, playlist.PublicTitle)
	creator := sodaFirstNonEmpty(playlist.Owner.Nickname, playlist.Owner.PublicName)
	trackCount := playlist.CountTracks
	if trackCount == 0 {
		trackCount = playlist.ResourceCnt.TrackCnt
	}

	description := cleanSodaTopListText(playlist.Desc, 200)
	if description == "" {
		description = "汽水音乐推荐歌单"
	}

	extra := map[string]string{
		"group":        sodaRecommendPlaylistGroup,
		"toplist_type": "playlist",
		"playlist_id":  playlist.ID,
	}
	if creator != "" {
		extra["creator"] = creator
	}

	return model.TopList{
		ID:          playlist.ID,
		Name:        name,
		Cover:       sodaBuildImageURL(playlist.URLCover, "~c5_300x300.jpg"),
		Description: description,
		TrackCount:  trackCount,
		Source:      "soda",
		Link:        fmt.Sprintf("https://www.qishui.com/playlist/%s", playlist.ID),
		Extra:       extra,
	}
}

// cleanSodaTopListText 去掉文案里的换行与多余空白，并按 rune 截断到 limit 长度。
func cleanSodaTopListText(text string, limit int) string {
	text = strings.ReplaceAll(text, "\r\n", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.Join(strings.Fields(text), " ")
	if limit <= 0 {
		return text
	}

	runes := []rune(text)
	if len(runes) > limit {
		return strings.TrimSpace(string(runes[:limit])) + "…"
	}
	return text
}

// sodaIsBuiltinChartID 判断 ID 是否为内置官方榜单。
func sodaIsBuiltinChartID(topListID string) bool {
	for _, def := range sodaTopListDefinitions {
		if def.ID == topListID {
			return true
		}
	}
	return false
}

type sodaChartResponse struct {
	StatusCode int               `json:"status_code"`
	StatusInfo sodaAPIStatusInfo `json:"status_info"`
	Chart      sodaChart         `json:"chart"`
}

type sodaChart struct {
	ID          string               `json:"id"`
	Title       string               `json:"title"`
	CountTracks int                  `json:"count_tracks"`
	Period      string               `json:"period"`
	ReleaseTime int64                `json:"release_time"`
	TrackRanks  []sodaChartTrackRank `json:"track_ranks"`
}

type sodaChartTrackRank struct {
	Rank  sodaChartRank `json:"rank"`
	Track sodaTrack     `json:"track"`
}

type sodaChartRank struct {
	Rank      interface{} `json:"rank"`
	RankDelta interface{} `json:"rank_delta"`
	RankType  string      `json:"rank_type"`
	TrackID   string      `json:"track_id"`
}

func parseSodaChartResponse(body []byte) (*sodaChartResponse, error) {
	var resp sodaChartResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda toplist json parse error: %w", err)
	}
	if resp.StatusCode != 0 {
		msg := strings.TrimSpace(resp.StatusInfo.StatusMsg)
		if msg == "" {
			msg = "unknown error"
		}
		return nil, fmt.Errorf("soda toplist api error: status_code=%d status_msg=%s", resp.StatusCode, msg)
	}
	if len(resp.Chart.TrackRanks) == 0 {
		return nil, errors.New("soda toplist has no tracks")
	}
	return &resp, nil
}

// sodaChartSongs 把榜单条目转换为歌曲列表，limit <= 0 时返回全部条目。
func sodaChartSongs(chart *sodaChart, topListID string, limit int) []model.Song {
	if chart == nil {
		return nil
	}

	songs := make([]model.Song, 0, len(chart.TrackRanks))
	for i, item := range chart.TrackRanks {
		track := item.Track
		if strings.TrimSpace(track.ID) == "" {
			continue
		}

		song := sodaBuildSongFromTrack(track)
		if song.Extra == nil {
			song.Extra = make(map[string]string, 8)
		}

		rank := sodaChartRankNumber(item.Rank.Rank)
		if rank <= 0 {
			rank = i + 1
		}
		song.Extra["rank"] = strconv.Itoa(rank)
		song.Extra["toplist_id"] = topListID
		if value := sodaChartScalarString(item.Rank.RankDelta); value != "" {
			song.Extra["rank_delta"] = value
		}
		if value := strings.TrimSpace(item.Rank.RankType); value != "" {
			song.Extra["rank_type"] = value
		}
		if value := strings.TrimSpace(item.Rank.TrackID); value != "" {
			song.Extra["rank_track_id"] = value
		}
		if value := strings.TrimSpace(track.VID); value != "" {
			song.Extra["vid"] = value
		}

		songs = append(songs, song)
		if limit > 0 && len(songs) >= limit {
			break
		}
	}
	return songs
}

func sodaChartRankNumber(value interface{}) int {
	switch v := value.(type) {
	case float64:
		return int(v + 0.5)
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return 0
}

func sodaChartScalarString(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return v.String()
	case bool:
		return strconv.FormatBool(v)
	default:
		return ""
	}
}

func normalizeSodaHotSongLimit(limit int) int {
	if limit <= 0 {
		return DefaultHotSongLimit
	}
	if limit > MaxHotSongLimit {
		return MaxHotSongLimit
	}
	return limit
}

func sodaTopListLink(topListID string) string {
	return "https://music.douyin.com/qishui/share/playlist?playlist_id=" + url.QueryEscape(topListID)
}
