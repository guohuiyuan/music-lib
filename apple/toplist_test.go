package apple

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// appleChartFeedFixture 是 Apple 营销工具 Top 100 接口的真实响应结构（节选）。
const appleChartFeedFixture = `{
	"feed": {
		"title": "热门歌曲排行",
		"country": "cn",
		"updated": "Mon, 12 Oct 2026 15:52:31 +0000",
		"results": [
			{
				"artistName": "孙燕姿",
				"id": "255921025",
				"name": "我不难过",
				"releaseDate": "2003-01-10",
				"kind": "songs",
				"artworkUrl100": "https://is1-ssl.mzstatic.com/image/thumb/Music124/v4/24/fc/1d/24fc1d3e-b37f-968f-3d21-880edc8f8d79/mzi.ybwydvjn.jpg/100x100bb.jpg",
				"url": "https://music.apple.com/cn/album/%E6%88%91%E4%B8%8D%E9%9A%BE%E8%BF%87/255920420?i=255921025"
			},
			{
				"artistName": "周杰伦",
				"id": "905226305",
				"name": "我怀念的",
				"releaseDate": "2007-03-22",
				"kind": "songs",
				"artworkUrl100": "https://is1-ssl.mzstatic.com/image/thumb/Music126/v4/aa/bb/cc/dd.jpg/100x100bb.jpg",
				"url": "https://music.apple.com/cn/album/%E6%88%91%E6%80%80%E5%BF%B5%E7%9A%84/255920421?i=905226305"
			}
		]
	}
}`

// appleLookupFixture 是 iTunes 查询接口的真实响应结构（节选）。
const appleLookupFixture = `{
	"resultCount": 1,
	"results": [
		{
			"wrapperType": "track",
			"kind": "song",
			"artistId": 83405200,
			"collectionId": 255920420,
			"trackId": 255921025,
			"artistName": "孙燕姿",
			"collectionName": "未完成",
			"trackName": "我不难过",
			"trackTimeMillis": 320400,
			"previewUrl": "https://audio-ssl.itunes.apple.com/itunes-assets/AudioPreview211/v4/6d/b0/40/6db040c5-34fb-0873-a766-be6a4647a36e/mzaf_2285663894330283624.plus.aac.p.m4a",
			"artworkUrl100": "https://is1-ssl.mzstatic.com/image/thumb/Music124/v4/24/fc/1d/24fc1d3e-b37f-968f-3d21-880edc8f8d79/mzi.ybwydvjn.jpg/100x100bb.jpg",
			"trackViewUrl": "https://music.apple.com/cn/album/%E6%88%91%E4%B8%8D%E9%9A%BE%E8%BF%87/255920420?i=255921025&uo=4",
			"primaryGenreName": "国语流行",
			"releaseDate": "2003-01-10T12:00:00Z"
		}
	]
}`

func TestAppleGetTopLists(t *testing.T) {
	topLists, err := New("").GetTopLists()
	if err != nil {
		t.Fatalf("GetTopLists returned error: %v", err)
	}
	if len(topLists) != len(appleChartStorefronts) {
		t.Fatalf("GetTopLists returned %d top lists, want %d", len(topLists), len(appleChartStorefronts))
	}

	first := topLists[0]
	if first.ID != "most-played:songs:cn" {
		t.Fatalf("first toplist id = %q, want most-played:songs:cn", first.ID)
	}
	if first.Name != "中国内地 Top 100" {
		t.Fatalf("first toplist name = %q, want 中国内地 Top 100", first.Name)
	}
	if first.Source != "apple" {
		t.Fatalf("first toplist source = %q, want apple", first.Source)
	}
	if first.TrackCount != MaxTopListLimit {
		t.Fatalf("first toplist track count = %d, want %d", first.TrackCount, MaxTopListLimit)
	}
	if first.Link != "https://music.apple.com/cn/new/top-charts" {
		t.Fatalf("first toplist link = %q, want Apple 官方排行榜页面", first.Link)
	}
	if first.Extra["storefront"] != "cn" || first.Extra["chart"] != AppleChartFeed {
		t.Fatalf("first toplist extra = %#v, want storefront/chart", first.Extra)
	}
	if first.Extra["toplist_type"] != "chart" {
		t.Fatalf("first toplist type = %q, want chart", first.Extra["toplist_type"])
	}
	if first.Description == "" {
		t.Fatal("first toplist description is empty")
	}

	ids := make(map[string]struct{}, len(topLists))
	for _, item := range topLists {
		ids[item.ID] = struct{}{}
	}
	for _, want := range []string{"most-played:songs:cn", "most-played:songs:us", "most-played:songs:jp", "most-played:songs:kr"} {
		if _, ok := ids[want]; !ok {
			t.Fatalf("GetTopLists missing %s", want)
		}
	}
}

func TestParseAppleChartID(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "most-played:songs:cn", want: "most-played:songs:cn"},
		{in: "songs:us", want: "most-played:songs:us"},
		{in: "jp", want: "most-played:songs:jp"},
		{in: "  HK  ", want: "most-played:songs:hk"},
		{in: "", wantErr: true},
		{in: "most-played:albums:cn", wantErr: true},
		{in: "new-music:songs:cn", wantErr: true},
		{in: "songs:chn", wantErr: true},
		{in: "songs:cn:extra:more", wantErr: true},
		{in: "songs:../etc", wantErr: true},
	}
	for _, tc := range cases {
		ref, err := parseAppleChartID(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("parseAppleChartID(%q) should fail, got %#v", tc.in, ref)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseAppleChartID(%q) returned error: %v", tc.in, err)
		}
		if ref.ID() != tc.want {
			t.Fatalf("parseAppleChartID(%q) = %s, want %s", tc.in, ref.ID(), tc.want)
		}
	}
}

func TestAppleChartHelpers(t *testing.T) {
	if got := appleArtworkUpgrade("https://is1-ssl.mzstatic.com/image/thumb/Music124/v4/aa/bb.jpg/100x100bb.jpg", 600); got != "https://is1-ssl.mzstatic.com/image/thumb/Music124/v4/aa/bb.jpg/600x600bb.jpg" {
		t.Fatalf("appleArtworkUpgrade = %q", got)
	}
	if got := appleArtworkUpgrade("", 600); got != "" {
		t.Fatalf("appleArtworkUpgrade(empty) = %q, want empty", got)
	}

	if got := appleStripLookupParam("https://music.apple.com/cn/album/x/1?i=2&uo=4"); got != "https://music.apple.com/cn/album/x/1?i=2" {
		t.Fatalf("appleStripLookupParam = %q", got)
	}
	if got := appleStripLookupParam("https://music.apple.com/cn/song/x/1?uo=4"); got != "https://music.apple.com/cn/song/x/1" {
		t.Fatalf("appleStripLookupParam(uo only) = %q", got)
	}

	if got := normalizeAppleHotSongLimit(0); got != DefaultHotSongLimit {
		t.Fatalf("normalizeAppleHotSongLimit(0) = %d, want %d", got, DefaultHotSongLimit)
	}
	if got := normalizeAppleHotSongLimit(-3); got != DefaultHotSongLimit {
		t.Fatalf("normalizeAppleHotSongLimit(-3) = %d, want %d", got, DefaultHotSongLimit)
	}
	if got := normalizeAppleHotSongLimit(500); got != MaxHotSongLimit {
		t.Fatalf("normalizeAppleHotSongLimit(500) = %d, want %d", got, MaxHotSongLimit)
	}
	if got := normalizeAppleHotSongLimit(20); got != 20 {
		t.Fatalf("normalizeAppleHotSongLimit(20) = %d, want 20", got)
	}
}

func TestAppleChartSongMerge(t *testing.T) {
	var feed appleChartFeedResponse
	if err := json.Unmarshal([]byte(appleChartFeedFixture), &feed); err != nil {
		t.Fatalf("feed fixture unmarshal error: %v", err)
	}
	if len(feed.Feed.Results) != 2 {
		t.Fatalf("feed fixture results = %d, want 2", len(feed.Feed.Results))
	}

	var lookup appleLookupResponse
	if err := json.Unmarshal([]byte(appleLookupFixture), &lookup); err != nil {
		t.Fatalf("lookup fixture unmarshal error: %v", err)
	}
	if len(lookup.Results) != 1 {
		t.Fatalf("lookup fixture results = %d, want 1", len(lookup.Results))
	}

	ref := appleChartRef{Feed: AppleChartFeed, Media: AppleChartMedia, Storefront: "cn"}
	song := appleChartSong(feed.Feed.Results[0], lookup.Results[0], ref)
	if song.ID != "255921025" || song.Name != "我不难过" || song.Artist != "孙燕姿" {
		t.Fatalf("unexpected song meta: %+v", song)
	}
	if song.Album != "未完成" {
		t.Fatalf("song album = %q, want 未完成 (来自 iTunes 查询)", song.Album)
	}
	if song.Duration != 320 {
		t.Fatalf("song duration = %d, want 320", song.Duration)
	}
	if !strings.HasPrefix(song.Cover, "https://is1-ssl.mzstatic.com/") || !strings.Contains(song.Cover, "600x600bb.jpg") {
		t.Fatalf("song cover = %q, want 600x600 封面", song.Cover)
	}
	if !strings.HasPrefix(song.URL, "https://audio-ssl.itunes.apple.com/") {
		t.Fatalf("song preview url = %q, want itunes preview", song.URL)
	}
	if strings.Contains(song.Link, "uo=4") {
		t.Fatalf("song link = %q, want uo 参数已清理", song.Link)
	}
	if song.Ext != "m4a" || song.Source != "apple" {
		t.Fatalf("unexpected song format: ext=%q source=%q", song.Ext, song.Source)
	}
	if song.Extra["storefront"] != "cn" || song.Extra["genre"] != "国语流行" || song.Extra["release_date"] != "2003-01-10" {
		t.Fatalf("unexpected song extra: %#v", song.Extra)
	}
	if song.AlbumID != "255920420" {
		t.Fatalf("song album id = %q, want 255920420", song.AlbumID)
	}

	// 缺少 iTunes 详情时仍能返回榜单基础信息。
	fallback := appleChartSong(feed.Feed.Results[1], appleLookupItem{}, ref)
	if fallback.Name != "我怀念的" || fallback.Artist != "周杰伦" {
		t.Fatalf("unexpected fallback song: %+v", fallback)
	}
	if fallback.URL != "" || fallback.Duration != 0 {
		t.Fatalf("fallback song should not have preview/duration: %+v", fallback)
	}
	if !strings.Contains(fallback.Cover, "600x600bb.jpg") {
		t.Fatalf("fallback song cover = %q, want upgraded artwork", fallback.Cover)
	}
	if fallback.Link == "" {
		t.Fatal("fallback song link should come from feed url")
	}
}

// appleChartTestServer 提供榜单接口与 iTunes 查询接口的本地实现。
func appleChartTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/bad/"):
			w.WriteHeader(http.StatusInternalServerError)
		case strings.Contains(r.URL.Path, "/music/most-played/"):
			if !strings.Contains(r.URL.Path, "/cn/") {
				t.Errorf("unexpected feed path: %s", r.URL.Path)
			}
			if r.Header.Get("Accept") != "application/json" {
				t.Errorf("missing Accept header on feed request")
			}
			_, _ = w.Write([]byte(appleChartFeedFixture))
		case r.URL.Path == "/lookup":
			if got := r.URL.Query().Get("country"); got != "cn" {
				t.Errorf("lookup country = %q, want cn", got)
			}
			if got := r.URL.Query().Get("id"); !strings.Contains(got, "255921025") {
				t.Errorf("lookup id = %q, want it to contain chart ids", got)
			}
			_, _ = w.Write([]byte(appleLookupFixture))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestAppleFetchChartSongsWithFallbackHost(t *testing.T) {
	server := appleChartTestServer(t)
	defer server.Close()

	originalHosts, originalLookup := appleChartHosts, appleLookupAPI
	appleChartHosts = []string{server.URL + "/bad", server.URL}
	appleLookupAPI = server.URL + "/lookup"
	defer func() { appleChartHosts, appleLookupAPI = originalHosts, originalLookup }()

	songs, err := New("").GetTopListSongs("most-played:songs:cn")
	if err != nil {
		t.Fatalf("GetTopListSongs returned error: %v", err)
	}
	if len(songs) != 2 {
		t.Fatalf("GetTopListSongs returned %d songs, want 2", len(songs))
	}

	first := songs[0]
	if first.Extra["rank"] != "1" || first.Extra["toplist_id"] != "most-played:songs:cn" {
		t.Fatalf("unexpected first song extra: %#v", first.Extra)
	}
	if songs[1].Extra["rank"] != "2" {
		t.Fatalf("second song rank = %q, want 2", songs[1].Extra["rank"])
	}
	if first.Album != "未完成" || first.Duration != 320 {
		t.Fatalf("first song should be enriched by lookup: %+v", first)
	}

	// 第一段失败时 fallback 域名生效：第二个条目没有 iTunes 详情也能返回。
	if songs[1].Name != "我怀念的" || songs[1].URL != "" {
		t.Fatalf("unexpected second song: %+v", songs[1])
	}
}

func TestAppleFetchChartSongsAllHostsFail(t *testing.T) {
	server := appleChartTestServer(t)
	defer server.Close()

	originalHosts, originalLookup := appleChartHosts, appleLookupAPI
	appleChartHosts = []string{server.URL + "/bad", server.URL + "/bad"}
	appleLookupAPI = server.URL + "/lookup"
	defer func() { appleChartHosts, appleLookupAPI = originalHosts, originalLookup }()

	songs, err := New("").GetTopListSongs("most-played:songs:cn")
	if err == nil {
		t.Fatal("GetTopListSongs should fail when every host fails")
	}
	if songs != nil {
		t.Fatalf("GetTopListSongs should return nil songs on failure, got %v", songs)
	}
	if !strings.Contains(err.Error(), "most-played:songs:cn") {
		t.Fatalf("error should mention toplist id: %v", err)
	}
}

func TestGetAppleTopListsLive(t *testing.T) {
	topLists, err := New("").GetTopLists()
	if err != nil {
		t.Fatalf("GetTopLists failed: %v", err)
	}
	if len(topLists) < 4 {
		t.Fatalf("GetTopLists returned %d top lists, want at least 4", len(topLists))
	}
	for _, item := range topLists {
		if item.Link == "" || !strings.Contains(item.Link, "/new/top-charts") {
			t.Fatalf("toplist %s link = %q, want Apple 排行榜页面", item.ID, item.Link)
		}
	}
}

func TestGetAppleTopListSongsLive(t *testing.T) {
	songs, err := New("").GetTopListSongs("most-played:songs:cn")
	if err != nil {
		t.Fatalf("GetTopListSongs failed: %v", err)
	}
	if len(songs) < 50 {
		t.Fatalf("GetTopListSongs returned %d songs, want at least 50", len(songs))
	}

	first := songs[0]
	if first.ID == "" || first.Name == "" || first.Artist == "" {
		t.Fatalf("unexpected first song: %#v", first)
	}
	if first.Source != "apple" {
		t.Fatalf("first song source = %q, want apple", first.Source)
	}
	if first.Extra["rank"] != "1" || first.Extra["toplist_id"] != "most-played:songs:cn" {
		t.Fatalf("unexpected first song extra: %#v", first.Extra)
	}
	if !strings.HasPrefix(first.Cover, "https://") {
		t.Fatalf("first song cover = %q, want https artwork", first.Cover)
	}
	if first.Duration == 0 {
		t.Fatalf("first song duration should be filled by iTunes lookup: %#v", first)
	}

	last := songs[len(songs)-1]
	if last.Extra["rank"] != "100" {
		t.Fatalf("last song rank = %q, want 100", last.Extra["rank"])
	}
}

func TestGetAppleHotSongsLive(t *testing.T) {
	hot, err := New("storefront=cn").GetHotSongs(10)
	if err != nil {
		t.Fatalf("GetHotSongs failed: %v", err)
	}
	if len(hot) != 10 {
		t.Fatalf("GetHotSongs(10) returned %d songs, want 10", len(hot))
	}
	if hot[9].Extra["rank"] != "10" {
		t.Fatalf("GetHotSongs(10) last rank = %q, want 10", hot[9].Extra["rank"])
	}
	if hot[0].Extra["storefront"] != "cn" {
		t.Fatalf("GetHotSongs storefront = %q, want cn", hot[0].Extra["storefront"])
	}
}

func TestAppleTopListPackageWrappersLive(t *testing.T) {
	topLists, err := GetTopLists()
	if err != nil {
		t.Fatalf("GetTopLists failed: %v", err)
	}
	if len(topLists) == 0 {
		t.Fatal("GetTopLists returned no top lists")
	}

	songs, err := GetTopListSongs("most-played:songs:cn")
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
