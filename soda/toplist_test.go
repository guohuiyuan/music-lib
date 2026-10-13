package soda

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

// sodaTestCookie 优先读取 SODA_COOKIE 环境变量，其次读取 music-lib/.env。
func sodaTestCookie() string {
	if cookie := strings.TrimSpace(os.Getenv("SODA_COOKIE")); cookie != "" {
		return cookie
	}

	b, err := os.ReadFile("../.env")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(line, "SODA_COOKIE=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "SODA_COOKIE="))
		}
	}
	return ""
}

const sodaChartTestBody = `{
	"status_info": {"log_id": "202610122249187E1975EA560560B957A7", "now": 1791816558},
	"chart": {
		"id": "7036274230471712007",
		"count_tracks": 50,
		"period": "weekly",
		"release_time": 1791446400,
		"title": "热歌榜",
		"track_ranks": [
			{
				"rank": {"rank_delta": "2", "rank_type": "up", "track_id": "7168861366286190604"},
				"track": {
					"id": "7168861366286190604",
					"name": "茶汤",
					"duration": 306913,
					"vid": "v02ad6g10000cdueirjc77u8jglmo5v0",
					"artists": [{"id": "6799466474814572545", "name": "郁可唯"}],
					"album": {
						"id": "7169056608998131714",
						"name": "微加幸福",
						"url_cover": {
							"uri": "tos-cn-v-2774c002/194b865cfd2f409685f3f7753d89012f",
							"urls": ["https://p3-luna.douyinpic.com/img/", "https://p6-luna.douyinpic.com/img/"],
							"template_prefix": "tplv-b829550vbb"
						}
					},
					"bit_rates": [
						{"br": 132182, "size": 5070665, "quality": "higher"},
						{"br": 318380, "size": 12213487, "quality": "hi_res"}
					],
					"label_info": {"only_vip_playable": true}
				}
			},
			{
				"rank": {"rank": 5, "rank_delta": -1, "rank_type": "down", "track_id": "7091589595099301889"},
				"track": {
					"id": "7091589595099301889",
					"name": "分手已经. . . - Inst.",
					"duration": 308421,
					"artists": [{"name": "测试歌手"}],
					"album": {"id": "7091589595099301888", "name": "测试专辑"},
					"bit_rates": [{"br": 128000, "size": 4934736, "quality": "medium"}]
				}
			},
			{
				"rank": {"rank_delta": "0"},
				"track": {
					"id": "7657099403009804305",
					"name": "甲乙丙丁（你我怎么两清）",
					"duration": 210462
				}
			}
		]
	}
}`

func TestParseSodaChartResponse(t *testing.T) {
	resp, err := parseSodaChartResponse([]byte(sodaChartTestBody))
	if err != nil {
		t.Fatalf("parseSodaChartResponse returned error: %v", err)
	}

	chart := resp.Chart
	if chart.ID != "7036274230471712007" {
		t.Fatalf("chart id = %q, want 7036274230471712007", chart.ID)
	}
	if chart.Title != "热歌榜" {
		t.Fatalf("chart title = %q, want 热歌榜", chart.Title)
	}
	if chart.CountTracks != 50 || chart.Period != "weekly" || chart.ReleaseTime != 1791446400 {
		t.Fatalf("unexpected chart metadata: %#v", chart)
	}
	if len(chart.TrackRanks) != 3 {
		t.Fatalf("chart track ranks = %d, want 3", len(chart.TrackRanks))
	}
}

func TestParseSodaChartResponseError(t *testing.T) {
	if _, err := parseSodaChartResponse([]byte(`{"status_code":1000004,"status_info":{"status_msg":"ERR_INVALID_PARAM"}}`)); err == nil {
		t.Fatal("parseSodaChartResponse should return error when status_code is not 0")
	}
	if _, err := parseSodaChartResponse([]byte(`{"chart":{"title":"empty"}}`)); err == nil {
		t.Fatal("parseSodaChartResponse should return error when track_ranks is empty")
	}
	if _, err := parseSodaChartResponse([]byte(`not json`)); err == nil {
		t.Fatal("parseSodaChartResponse should return error for invalid json")
	}
}

func TestSodaChartSongs(t *testing.T) {
	resp, err := parseSodaChartResponse([]byte(sodaChartTestBody))
	if err != nil {
		t.Fatalf("parseSodaChartResponse returned error: %v", err)
	}

	songs := sodaChartSongs(&resp.Chart, HotSongTopListID, 0)
	if len(songs) != 3 {
		t.Fatalf("sodaChartSongs returned %d songs, want 3", len(songs))
	}

	first := songs[0]
	if first.Source != "soda" || first.ID != "7168861366286190604" {
		t.Fatalf("unexpected first song: %#v", first)
	}
	if first.Name != "茶汤" || first.Artist != "郁可唯" || first.Album != "微加幸福" {
		t.Fatalf("unexpected first song metadata: %#v", first)
	}
	if first.Duration != 306 {
		t.Fatalf("first song duration = %d, want 306", first.Duration)
	}
	if first.Size != 12213487 {
		t.Fatalf("first song size = %d, want 12213487", first.Size)
	}
	if first.Bitrate <= 0 {
		t.Fatalf("first song bitrate should be derived from size: %d", first.Bitrate)
	}
	if first.Cover == "" || !strings.Contains(first.Cover, "tos-cn-v-2774c002") {
		t.Fatalf("unexpected first song cover: %q", first.Cover)
	}
	// rank.rank 缺失时按序号补 1；rank_delta 兼容字符串与数字。
	if first.Extra["rank"] != "1" {
		t.Fatalf("first song rank = %q, want 1", first.Extra["rank"])
	}
	if first.Extra["rank_delta"] != "2" || first.Extra["rank_type"] != "up" {
		t.Fatalf("unexpected first song rank extra: %#v", first.Extra)
	}
	if first.Extra["toplist_id"] != HotSongTopListID {
		t.Fatalf("unexpected first song toplist id: %#v", first.Extra)
	}
	if first.Extra["vid"] != "v02ad6g10000cdueirjc77u8jglmo5v0" {
		t.Fatalf("unexpected first song vid: %#v", first.Extra)
	}
	if !first.IsVIP {
		t.Fatalf("first song should be marked VIP: %#v", first.Extra)
	}

	// 第二条带显式 rank=5，且 rank_delta 是数字。
	if songs[1].Extra["rank"] != "5" {
		t.Fatalf("second song rank = %q, want 5", songs[1].Extra["rank"])
	}
	if songs[1].Extra["rank_delta"] != "-1" {
		t.Fatalf("second song rank delta = %q, want -1", songs[1].Extra["rank_delta"])
	}

	// 第三条没有任何 rank 字段，按序号补 3。
	if songs[2].Extra["rank"] != "3" {
		t.Fatalf("third song rank = %q, want 3", songs[2].Extra["rank"])
	}

	limited := sodaChartSongs(&resp.Chart, HotSongTopListID, 2)
	if len(limited) != 2 {
		t.Fatalf("sodaChartSongs with limit 2 returned %d songs, want 2", len(limited))
	}
}

func TestSodaBuiltinTopLists(t *testing.T) {
	topLists := sodaBuiltinTopLists()
	if len(topLists) != 4 {
		t.Fatalf("sodaBuiltinTopLists returned %d top lists, want 4", len(topLists))
	}

	byID := make(map[string]model.TopList, len(topLists))
	for _, topList := range topLists {
		byID[topList.ID] = topList
	}

	hot, ok := byID[HotSongTopListID]
	if !ok {
		t.Fatalf("expected 热歌榜 (%s) in top lists", HotSongTopListID)
	}
	if hot.Name != "热歌榜" || hot.Source != "soda" {
		t.Fatalf("unexpected 热歌榜 metadata: %#v", hot)
	}
	if hot.Extra["group"] != sodaChartTopListGroup || hot.Extra["chart_id"] != HotSongTopListID {
		t.Fatalf("unexpected 热歌榜 extra: %#v", hot.Extra)
	}
	if hot.Extra["toplist_type"] != "chart" {
		t.Fatalf("unexpected 热歌榜 toplist type: %#v", hot.Extra)
	}
	if hot.TrackCount != 50 || hot.Cover == "" || hot.Description == "" {
		t.Fatalf("unexpected 热歌榜 metadata: %#v", hot)
	}
	if !strings.Contains(hot.Link, HotSongTopListID) {
		t.Fatalf("unexpected 热歌榜 link: %q", hot.Link)
	}

	for _, topList := range topLists {
		if topList.Name == "" || topList.Cover == "" || topList.UpdateFrequency == "" {
			t.Fatalf("top list missing metadata: %#v", topList)
		}
	}
}

const sodaDiscoverMixTestBody = `{
	"status_info": {"log_id": "202610122305062D9B0E6D06A63E0B7367", "now": 1791817506},
	"has_more": true,
	"inner_block": [
		{
			"type": "discover_playlist",
			"title": "欧美性感男嗓 | 令人着迷的音色",
			"inner_block_id": "7031085120368052254",
			"resources": [
				{
					"resource_id": "7031085120368052254",
					"type": "playlist",
					"entity": {
						"playlist": {
							"id": "7031085120368052254",
							"title": "欧美性感男嗓 | 令人着迷的音色",
							"public_title": "欧美性感男嗓",
							"desc": "众多欧美优质男嗓的经典及热门歌曲\n第二行说明",
							"count_tracks": 77,
							"url_cover": {
								"uri": "ies-music/pgc_cover_0e9b2ca8bbd3a028e9e1759f460ed89f",
								"urls": ["https://p3-luna.douyinpic.com/img/", "https://p6-luna.douyinpic.com/img/"],
								"template_prefix": "tplv-b829550vbb"
							},
							"owner": {"id": "2331429247649368", "nickname": "汽水音乐APP", "public_name": "汽水音乐"}
						}
					}
				}
			]
		},
		{
			"type": "discover_playlist",
			"title": "无效条目",
			"resources": [
				{"type": "playlist", "entity": {"playlist": {"id": ""}}}
			]
		}
	]
}`

func TestParseSodaDiscoverMixResponse(t *testing.T) {
	items, err := parseSodaDiscoverMixResponse([]byte(sodaDiscoverMixTestBody))
	if err != nil {
		t.Fatalf("parseSodaDiscoverMixResponse returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("parseSodaDiscoverMixResponse returned %d playlists, want 1", len(items))
	}
	if items[0].ID != "7031085120368052254" || items[0].Title != "欧美性感男嗓 | 令人着迷的音色" {
		t.Fatalf("unexpected playlist: %#v", items[0])
	}

	if _, err := parseSodaDiscoverMixResponse([]byte(`{"inner_block":[]}`)); err == nil {
		t.Fatal("parseSodaDiscoverMixResponse should return error for empty blocks")
	}
	if _, err := parseSodaDiscoverMixResponse([]byte(`not json`)); err == nil {
		t.Fatal("parseSodaDiscoverMixResponse should return error for invalid json")
	}
}

func TestSodaTopListFromPlaylist(t *testing.T) {
	items, err := parseSodaDiscoverMixResponse([]byte(sodaDiscoverMixTestBody))
	if err != nil {
		t.Fatalf("parseSodaDiscoverMixResponse returned error: %v", err)
	}

	topList := sodaTopListFromPlaylist(items[0])
	if topList.ID != "7031085120368052254" || topList.Source != "soda" {
		t.Fatalf("unexpected top list: %#v", topList)
	}
	if topList.Name != "欧美性感男嗓 | 令人着迷的音色" {
		t.Fatalf("unexpected top list name: %q", topList.Name)
	}
	if topList.TrackCount != 77 {
		t.Fatalf("unexpected top list track count: %d", topList.TrackCount)
	}
	if topList.Cover == "" || !strings.Contains(topList.Cover, "ies-music/pgc_cover_0e9b2ca8bbd3a028e9e1759f460ed89f") {
		t.Fatalf("unexpected top list cover: %q", topList.Cover)
	}
	if strings.Contains(topList.Description, "\n") {
		t.Fatalf("top list description should not contain newlines: %q", topList.Description)
	}
	if !strings.Contains(topList.Description, "第二行说明") {
		t.Fatalf("unexpected top list description: %q", topList.Description)
	}
	if !strings.Contains(topList.Link, "7031085120368052254") {
		t.Fatalf("unexpected top list link: %q", topList.Link)
	}
	if topList.Extra["group"] != sodaRecommendPlaylistGroup || topList.Extra["toplist_type"] != "playlist" {
		t.Fatalf("unexpected top list extra: %#v", topList.Extra)
	}
	if topList.Extra["playlist_id"] != "7031085120368052254" || topList.Extra["creator"] != "汽水音乐APP" {
		t.Fatalf("unexpected top list extra: %#v", topList.Extra)
	}
}

func TestCleanSodaTopListText(t *testing.T) {
	if got := cleanSodaTopListText("a\r\nb\n c", 0); got != "a b c" {
		t.Fatalf("cleanSodaTopListText = %q, want %q", got, "a b c")
	}

	got := cleanSodaTopListText(strings.Repeat("字", 210), 200)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("cleanSodaTopListText should truncate long text: %q", got)
	}
	if runes := []rune(got); len(runes) != 201 {
		t.Fatalf("cleanSodaTopListText returned %d runes, want 201", len(runes))
	}
}

func TestSodaRankPlaylistSongs(t *testing.T) {
	songs := []model.Song{
		{ID: "1", Source: "soda"},
		{ID: "2", Source: "soda"},
		{ID: "3", Source: "soda", Extra: map[string]string{"track_id": "3"}},
	}

	ranked := sodaRankPlaylistSongs(songs, "7031085120368052254", 2)
	if len(ranked) != 2 {
		t.Fatalf("sodaRankPlaylistSongs with limit 2 returned %d songs, want 2", len(ranked))
	}
	for i, song := range ranked {
		if song.Extra["rank"] != strconv.Itoa(i+1) {
			t.Fatalf("song %d rank = %q, want %d", i, song.Extra["rank"], i+1)
		}
		if song.Extra["toplist_id"] != "7031085120368052254" {
			t.Fatalf("song %d toplist id = %q", i, song.Extra["toplist_id"])
		}
	}

	all := sodaRankPlaylistSongs(songs, "7031085120368052254", 0)
	if len(all) != 3 {
		t.Fatalf("sodaRankPlaylistSongs without limit returned %d songs, want 3", len(all))
	}
	if all[2].Extra["track_id"] != "3" {
		t.Fatalf("sodaRankPlaylistSongs should keep original extra: %#v", all[2].Extra)
	}
}

func TestSodaGetTopListSongsInvalidID(t *testing.T) {
	if _, err := New("").GetTopListSongs(""); err == nil {
		t.Fatal("GetTopListSongs should return error for empty id")
	}
	if _, err := New("").GetTopListSongs("abcd"); err == nil {
		t.Fatal("GetTopListSongs should return error for non numeric id")
	}
}

func TestNormalizeSodaHotSongLimit(t *testing.T) {
	tests := []struct {
		in   int
		want int
	}{
		{in: 0, want: DefaultHotSongLimit},
		{in: -5, want: DefaultHotSongLimit},
		{in: 20, want: 20},
		{in: MaxHotSongLimit + 100, want: MaxHotSongLimit},
	}
	for _, tc := range tests {
		if got := normalizeSodaHotSongLimit(tc.in); got != tc.want {
			t.Fatalf("normalizeSodaHotSongLimit(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestGetTopListsLive(t *testing.T) {
	topLists, err := New("").GetTopLists()
	if err != nil {
		t.Fatalf("GetTopLists failed: %v", err)
	}
	if len(topLists) < 4 {
		t.Fatalf("GetTopLists returned %d top lists, want at least 4", len(topLists))
	}

	charts, playlists := 0, 0
	foundHot := false
	for _, topList := range topLists {
		if topList.Source != "soda" || topList.Name == "" || topList.ID == "" {
			t.Fatalf("unexpected top list: %#v", topList)
		}
		if topList.ID == HotSongTopListID {
			foundHot = true
		}
		switch topList.Extra["toplist_type"] {
		case "chart":
			charts++
		case "playlist":
			playlists++
		}
	}
	if charts != 4 {
		t.Fatalf("GetTopLists returned %d chart top lists, want 4", charts)
	}
	if !foundHot {
		t.Fatalf("expected 热歌榜 (%s) in top lists", HotSongTopListID)
	}
	// 推荐歌单依赖发现页接口，网络波动时允许为空，仅记录便于排查。
	t.Logf("soda top lists: total=%d charts=%d recommend_playlists=%d", len(topLists), charts, playlists)
}

// TestGetTopListSongsFromRecommendPlaylistLive 从推荐歌单进入榜单详情，
// 验证歌单 ID 走歌单详情接口并补上名次。
func TestGetTopListSongsFromRecommendPlaylistLive(t *testing.T) {
	topLists, err := New("").GetTopLists()
	if err != nil {
		t.Fatalf("GetTopLists failed: %v", err)
	}

	var target model.TopList
	for _, topList := range topLists {
		if topList.Extra["toplist_type"] == "playlist" {
			target = topList
			break
		}
	}
	if target.ID == "" {
		t.Skip("soda recommend playlists unavailable, skip playlist toplist test")
	}

	songs, err := New("").GetTopListSongs(target.ID)
	if err != nil {
		t.Fatalf("GetTopListSongs(%s) failed: %v", target.ID, err)
	}
	if len(songs) == 0 {
		t.Fatalf("GetTopListSongs(%s) returned no songs", target.ID)
	}

	first := songs[0]
	if first.Source != "soda" || first.ID == "" || first.Name == "" {
		t.Fatalf("unexpected first song: %#v", first)
	}
	if first.Extra["rank"] != "1" || first.Extra["toplist_id"] != target.ID {
		t.Fatalf("unexpected first song extra: %#v", first.Extra)
	}
	if songs[len(songs)-1].Extra["rank"] != strconv.Itoa(len(songs)) {
		t.Fatalf("last song rank = %q, want %d", songs[len(songs)-1].Extra["rank"], len(songs))
	}
	t.Logf("soda recommend playlist %s (%s) returned %d songs", target.ID, target.Name, len(songs))
}

func TestGetTopListSongsLive(t *testing.T) {
	songs, err := New("").GetTopListSongs(HotSongTopListID)
	if err != nil {
		t.Fatalf("GetTopListSongs failed: %v", err)
	}
	if len(songs) < 40 {
		t.Fatalf("GetTopListSongs returned %d songs, want at least 40", len(songs))
	}

	for i, song := range songs {
		if song.ID == "" || song.Name == "" {
			t.Fatalf("song %d missing metadata: %#v", i, song)
		}
		if song.Source != "soda" {
			t.Fatalf("song %d source = %q, want soda", i, song.Source)
		}
		if song.Extra["rank"] != strconv.Itoa(i+1) {
			t.Fatalf("song %d rank = %q, want %d", i, song.Extra["rank"], i+1)
		}
		if song.Extra["toplist_id"] != HotSongTopListID {
			t.Fatalf("song %d toplist id = %q", i, song.Extra["toplist_id"])
		}
	}

	first := songs[0]
	if first.Artist == "" || first.Duration == 0 || first.Size == 0 || first.Cover == "" {
		t.Fatalf("unexpected first song: %#v", first)
	}
}

func TestGetHotSongsLive(t *testing.T) {
	songs, err := New("").GetHotSongs(5)
	if err != nil {
		t.Fatalf("GetHotSongs failed: %v", err)
	}
	if len(songs) != 5 {
		t.Fatalf("GetHotSongs(5) returned %d songs, want 5", len(songs))
	}
	if songs[4].Extra["rank"] != "5" {
		t.Fatalf("GetHotSongs(5) last rank = %q, want 5", songs[4].Extra["rank"])
	}
}

func TestSodaTopListPackageWrappersLive(t *testing.T) {
	topLists, err := GetTopLists()
	if err != nil {
		t.Fatalf("GetTopLists failed: %v", err)
	}
	if len(topLists) == 0 {
		t.Fatal("GetTopLists returned no top lists")
	}

	songs, err := GetTopListSongs(HotSongTopListID)
	if err != nil {
		t.Fatalf("GetTopListSongs failed: %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("GetTopListSongs returned no songs")
	}

	hot, err := GetHotSongs(3)
	if err != nil {
		t.Fatalf("GetHotSongs failed: %v", err)
	}
	if len(hot) != 3 {
		t.Fatalf("GetHotSongs(3) returned %d songs, want 3", len(hot))
	}
}

// TestGetHotSongsWithCookieLive 使用登录 Cookie 验证热门单曲能力。
func TestGetHotSongsWithCookieLive(t *testing.T) {
	cookie := sodaTestCookie()
	if cookie == "" {
		t.Skip("SODA_COOKIE not set, skip logged-in hot songs test")
	}

	s := New(cookie)
	songs, err := s.GetHotSongs(10)
	if err != nil {
		t.Fatalf("GetHotSongs(10) with cookie failed: %v", err)
	}
	if len(songs) != 10 {
		t.Fatalf("GetHotSongs(10) returned %d songs, want 10", len(songs))
	}
	if songs[0].ID == "" || songs[0].Name == "" {
		t.Fatalf("unexpected first song: %#v", songs[0])
	}
}
