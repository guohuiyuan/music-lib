package kuwo

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

// kuwoTestCookie 优先读取 KUWO_COOKIE 环境变量，其次读取 music-lib/.env。
func kuwoTestCookie() string {
	if cookie := strings.TrimSpace(os.Getenv("KUWO_COOKIE")); cookie != "" {
		return cookie
	}

	b, err := os.ReadFile("../.env")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(line, "KUWO_COOKIE=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "KUWO_COOKIE="))
		}
	}
	return ""
}

// kuwoTopListPageFixture 模拟酷我榜单目录页的 SSR 数据：sourceid/pub 被压缩成 IIFE 形参。
const kuwoTopListPageFixture = `<html><body><script>window.__NUXT__=(function(a,b,c,d,e,f,g,h,i){return {layout:"default",data:[{bangMenu:[{name:"官方",list:[{sourceid:f,intro:"榜单规则：按照歌曲在酷我音乐移动端及PC端登录用户中产生的完整播放、下载量、收藏量、搜播量综合热度进行排名，每日更新",name:"酷我热歌榜",id:"489927",source:a,pic:"https:\u002F\u002Fzimg.kuwo.cn\u002Fbang\u002F2\u002F6\u002F1791814801081_489927.png",pub:b},{sourceid:"93",intro:"酷我每日搜索热度飙升最快的歌曲排行榜",name:"酷我飙升榜",id:"489929",source:a,pic:"https:\u002F\u002Fzimg.kuwo.cn\u002Fbang\u002F2\u002F2\u002F1791814803745_489929.png",pub:b}]},{name:"全球",list:[{sourceid:"12",intro:"公告牌单曲榜",name:"美国The Billboard",id:"490006",source:a,pic:"https:\u002F\u002Fzimg.kuwo.cn\u002Fbang\u002F0\u002F1\u002F1791814809000_490006.png",pub:h}]}]}]};}("2","今日更新","",false,0,"16","10月08日更新","10月11日更新","\u002F"));</script></body></html>`

func TestParseKuwoTopLists(t *testing.T) {
	topLists, err := parseKuwoTopLists([]byte(kuwoTopListPageFixture))
	if err != nil {
		t.Fatalf("parseKuwoTopLists returned error: %v", err)
	}
	if len(topLists) != 3 {
		t.Fatalf("parseKuwoTopLists returned %d top lists, want 3", len(topLists))
	}

	first := topLists[0]
	if first.ID != "16" {
		t.Fatalf("first toplist ID = %q, want 16 (IIFE 形参 f 需还原为实参)", first.ID)
	}
	if first.Name != "酷我热歌榜" {
		t.Fatalf("first toplist name = %q, want 酷我热歌榜", first.Name)
	}
	if first.Source != "kuwo" {
		t.Fatalf("first toplist source = %q, want kuwo", first.Source)
	}
	if first.Extra["group"] != "官方" {
		t.Fatalf("first toplist group = %q, want 官方", first.Extra["group"])
	}
	if first.UpdateFrequency != "今日更新" {
		t.Fatalf("first toplist update frequency = %q, want 今日更新", first.UpdateFrequency)
	}
	if !strings.HasPrefix(first.Cover, "https://zimg.kuwo.cn/bang/") {
		t.Fatalf("first toplist cover = %q, want normalized zimg url", first.Cover)
	}
	if first.Link != kuwoTopListPageLink {
		t.Fatalf("first toplist link = %q, want %q", first.Link, kuwoTopListPageLink)
	}
	if first.Description == "" || strings.Contains(first.Description, "\n") {
		t.Fatalf("first toplist description = %q, want single-line text", first.Description)
	}

	if topLists[1].ID != "93" || topLists[1].Extra["group"] != "官方" {
		t.Fatalf("second toplist = %#v, want 93 in 官方", topLists[1])
	}
	third := topLists[2]
	if third.ID != "12" || third.Extra["group"] != "全球" {
		t.Fatalf("third toplist = %#v, want 12 in 全球", third)
	}
	if third.UpdateFrequency != "10月11日更新" {
		t.Fatalf("third toplist update frequency = %q, want 10月11日更新", third.UpdateFrequency)
	}
}

func TestParseKuwoTopListsErrors(t *testing.T) {
	if _, err := parseKuwoTopLists([]byte(`<html><body>no data</body></html>`)); err == nil {
		t.Fatal("parseKuwoTopLists should return error when SSR data is missing")
	}
	if _, err := parseKuwoTopLists([]byte(`<script>window.__NUXT__=(function(a){return {};}("2"));</script>`)); err == nil {
		t.Fatal("parseKuwoTopLists should return error when bang menu is missing")
	}
	if _, err := parseKuwoTopLists([]byte(`<script>window.__NUXT__=(function(a){return {bangMenu:[]};}("2"));</script>`)); err == nil {
		t.Fatal("parseKuwoTopLists should return error when bang menu is empty")
	}
}

func TestParseKuwoTopListSongs(t *testing.T) {
	const body = `{
		"name": "酷我热歌榜",
		"num": "300",
		"musiclist": [
			{
				"id": "624683929",
				"name": "山风山风等等我",
				"artist": "万海东",
				"album": "山风山风等等我",
				"albumid": "97652689",
				"artistid": "197256",
				"duration": "74",
				"song_duration": "209",
				"formats": "MP3128|ALFLAC",
				"score100": "87",
				"trend": "e0",
				"rank_change": "0",
				"highest_rank": "1",
				"isnew": "0",
				"isRedSong": 0,
				"pay": "16711935",
				"copyright": "0",
				"online": "1"
			},
			{
				"id": "675496",
				"name": "爱只是个传说",
				"artist": "雨宗林",
				"album": "心灵驾驭",
				"albumid": "205359",
				"artistid": "3226",
				"duration": "251",
				"formats": "MP3128",
				"score100": "0",
				"trend": "e1",
				"rank_change": "-3",
				"highest_rank": "5",
				"isnew": "1",
				"isRedSong": 1,
				"pay": "0",
				"copyright": "1",
				"online": "0"
			}
		]
	}`

	songs, total, err := parseKuwoTopListSongs([]byte(body), HotSongTopListID, 100)
	if err != nil {
		t.Fatalf("parseKuwoTopListSongs returned error: %v", err)
	}
	if total != 300 {
		t.Fatalf("parseKuwoTopListSongs total = %d, want 300", total)
	}
	if len(songs) != 2 {
		t.Fatalf("parseKuwoTopListSongs returned %d songs, want 2", len(songs))
	}

	first := songs[0]
	if first.ID != "624683929" || first.Name != "山风山风等等我" || first.Artist != "万海东" {
		t.Fatalf("unexpected first song: %#v", first)
	}
	if first.Source != "kuwo" {
		t.Fatalf("first song source = %q, want kuwo", first.Source)
	}
	if first.AlbumID != "97652689" {
		t.Fatalf("first song AlbumID = %q, want 97652689", first.AlbumID)
	}
	if first.Duration != 209 {
		t.Fatalf("first song duration = %d, want 209 (song_duration 优先)", first.Duration)
	}
	if first.Extra["rank"] != "101" {
		t.Fatalf("first song rank = %q, want 101", first.Extra["rank"])
	}
	if first.Extra["toplist_id"] != HotSongTopListID || first.Extra["rid"] != "624683929" {
		t.Fatalf("unexpected first song extra: %#v", first.Extra)
	}
	if first.Extra["artist_id"] != "197256" || first.Extra["highest_rank"] != "1" {
		t.Fatalf("unexpected first song extra: %#v", first.Extra)
	}
	if !strings.Contains(first.Link, "play_detail/624683929") {
		t.Fatalf("unexpected first song link: %q", first.Link)
	}

	second := songs[1]
	if second.Extra["rank"] != "102" {
		t.Fatalf("second song rank = %q, want 102", second.Extra["rank"])
	}
	if second.Extra["rank_change"] != "-3" || second.Extra["is_new"] != "1" {
		t.Fatalf("unexpected second song extra: %#v", second.Extra)
	}
	if second.Duration != 251 {
		t.Fatalf("second song duration = %d, want 251 (缺 song_duration 时回落)", second.Duration)
	}
}

func TestParseKuwoTopListSongsEmpty(t *testing.T) {
	songs, total, err := parseKuwoTopListSongs([]byte(`{"name":"","num":"0","musiclist":[]}`), "999999", 0)
	if err != nil {
		t.Fatalf("parseKuwoTopListSongs returned error: %v", err)
	}
	if len(songs) != 0 || total != 0 {
		t.Fatalf("parseKuwoTopListSongs = (%d songs, total %d), want empty", len(songs), total)
	}

	if _, _, err := parseKuwoTopListSongs([]byte(`not json`), "16", 0); err == nil {
		t.Fatal("parseKuwoTopListSongs should return error for invalid json")
	}
}

func TestGetKuwoTopListSongsInvalidID(t *testing.T) {
	if _, err := New("").GetTopListSongs(""); err == nil {
		t.Fatal("GetTopListSongs should return error for empty id")
	}
	if _, err := New("").GetTopListSongs("abc"); err == nil {
		t.Fatal("GetTopListSongs should return error for non numeric id")
	}
}

func TestNormalizeKuwoHotSongLimit(t *testing.T) {
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
		if got := normalizeKuwoHotSongLimit(tc.in); got != tc.want {
			t.Fatalf("normalizeKuwoHotSongLimit(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestGetKuwoTopListsLive(t *testing.T) {
	topLists, err := New("").GetTopLists()
	if err != nil {
		t.Fatalf("GetTopLists failed: %v", err)
	}
	if len(topLists) < 30 {
		t.Fatalf("GetTopLists returned %d top lists, want at least 30", len(topLists))
	}

	groups := make(map[string]bool)
	byID := make(map[string]struct{ id, name, group, cover string }, len(topLists))
	for _, topList := range topLists {
		groups[topList.Extra["group"]] = true
		byID[topList.ID] = struct {
			id, name, group, cover string
		}{topList.ID, topList.Name, topList.Extra["group"], topList.Cover}
	}
	if !groups["官方"] {
		t.Fatalf("GetTopLists missing 官方 group: %v", groups)
	}

	hot, ok := byID[HotSongTopListID]
	if !ok {
		t.Fatalf("expected 热歌榜 (%s) in top lists", HotSongTopListID)
	}
	if hot.name != "酷我热歌榜" {
		t.Fatalf("unexpected 热歌榜 name: %q", hot.name)
	}
	if hot.group != "官方" {
		t.Fatalf("unexpected 热歌榜 group: %q", hot.group)
	}
	if !strings.HasPrefix(hot.cover, "http") {
		t.Fatalf("unexpected 热歌榜 cover: %q", hot.cover)
	}
}

func TestGetKuwoTopListSongsLive(t *testing.T) {
	songs, err := New("").GetTopListSongs(HotSongTopListID)
	if err != nil {
		t.Fatalf("GetTopListSongs failed: %v", err)
	}
	if len(songs) < 100 {
		t.Fatalf("GetTopListSongs returned %d songs, want at least 100", len(songs))
	}

	first := songs[0]
	if first.ID == "" || first.Name == "" || first.Artist == "" {
		t.Fatalf("unexpected first song: %#v", first)
	}
	if first.Source != "kuwo" {
		t.Fatalf("first song source = %q, want kuwo", first.Source)
	}
	if first.Extra["rank"] != "1" || first.Extra["toplist_id"] != HotSongTopListID {
		t.Fatalf("unexpected first song extra: %#v", first.Extra)
	}
	if first.Duration == 0 {
		t.Fatalf("first song duration should not be zero: %#v", first)
	}

	last := songs[len(songs)-1]
	if last.Extra["rank"] != strconv.Itoa(len(songs)) {
		t.Fatalf("last song rank = %q, want %d", last.Extra["rank"], len(songs))
	}
}

func TestGetKuwoHotSongsLive(t *testing.T) {
	hot, err := New("").GetHotSongs(5)
	if err != nil {
		t.Fatalf("GetHotSongs failed: %v", err)
	}
	if len(hot) != 5 {
		t.Fatalf("GetHotSongs(5) returned %d songs, want 5", len(hot))
	}
	if hot[4].Extra["rank"] != "5" {
		t.Fatalf("GetHotSongs(5) last rank = %q, want 5", hot[4].Extra["rank"])
	}
}

func TestKuwoTopListPackageWrappersLive(t *testing.T) {
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

// TestGetKuwoHotSongsWithCookieLive 使用登录 Cookie 验证热门单曲能力。
func TestGetKuwoHotSongsWithCookieLive(t *testing.T) {
	cookie := kuwoTestCookie()
	if cookie == "" {
		t.Skip("KUWO_COOKIE not set, skip logged-in hot songs test")
	}

	k := New(cookie)
	songs, err := k.GetHotSongs(30)
	if err != nil {
		t.Fatalf("GetHotSongs(30) with cookie failed: %v", err)
	}
	if len(songs) != 30 {
		t.Fatalf("GetHotSongs(30) returned %d songs, want 30", len(songs))
	}

	lists, err := k.GetTopLists()
	if err != nil {
		t.Fatalf("GetTopLists with cookie failed: %v", err)
	}
	if len(lists) < 30 {
		t.Fatalf("GetTopLists with cookie returned %d top lists, want at least 30", len(lists))
	}
}
