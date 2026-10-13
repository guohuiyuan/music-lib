package netease

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

// neteaseTestCookie 优先读取 NETEASE_COOKIE 环境变量，其次读取 music-lib/.env。
func neteaseTestCookie() string {
	if cookie := strings.TrimSpace(os.Getenv("NETEASE_COOKIE")); cookie != "" {
		return cookie
	}

	b, err := os.ReadFile("../.env")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(line, "NETEASE_COOKIE=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "NETEASE_COOKIE="))
		}
	}
	return ""
}

func TestParseTopLists(t *testing.T) {
	const body = `{
		"code": 200,
		"list": [
			{
				"id": 19723756,
				"name": "飙升榜",
				"coverImgUrl": "https://p1.music.126.net/example.jpg",
				"description": "云音乐中每天热度上升最快的100首单曲，每日更新。",
				"updateFrequency": "刚刚更新",
				"trackCount": 100,
				"playCount": 6511300096,
				"ToplistType": "S",
				"trackUpdateTime": 1791807730642
			},
			{
				"id": 0,
				"name": "invalid"
			}
		]
	}`

	topLists, err := parseTopLists([]byte(body))
	if err != nil {
		t.Fatalf("parseTopLists returned error: %v", err)
	}
	if len(topLists) != 1 {
		t.Fatalf("parseTopLists returned %d top lists, want 1", len(topLists))
	}

	got := topLists[0]
	if got.ID != "19723756" {
		t.Fatalf("parseTopLists returned id %q, want 19723756", got.ID)
	}
	if got.Source != "netease" {
		t.Fatalf("parseTopLists returned source %q, want netease", got.Source)
	}
	if got.Name != "飙升榜" {
		t.Fatalf("parseTopLists returned name %q, want 飙升榜", got.Name)
	}
	if got.TrackCount != 100 {
		t.Fatalf("parseTopLists returned track count %d, want 100", got.TrackCount)
	}
	if got.PlayCount != 6511300096 {
		t.Fatalf("parseTopLists returned play count %d, want 6511300096", got.PlayCount)
	}
	if got.UpdateFrequency != "刚刚更新" {
		t.Fatalf("parseTopLists returned update frequency %q, want 刚刚更新", got.UpdateFrequency)
	}
	if !strings.Contains(got.Link, "id=19723756") {
		t.Fatalf("parseTopLists returned link %q, want it to contain id=19723756", got.Link)
	}
	if got.Extra["toplist_type"] != "S" {
		t.Fatalf("parseTopLists returned toplist type %q, want S", got.Extra["toplist_type"])
	}
	if got.Extra["update_time"] != "1791807730642" {
		t.Fatalf("parseTopLists returned update time %q, want 1791807730642", got.Extra["update_time"])
	}
}

func TestParseTopListsError(t *testing.T) {
	if _, err := parseTopLists([]byte(`{"code": 500}`)); err == nil {
		t.Fatal("parseTopLists should return error for code 500")
	}
	if _, err := parseTopLists([]byte(`not json`)); err == nil {
		t.Fatal("parseTopLists should return error for invalid json")
	}
}

func TestGetTopListSongsInvalidID(t *testing.T) {
	if _, err := New("").GetTopListSongs(""); err == nil {
		t.Fatal("GetTopListSongs should return error for empty id")
	}
	if _, err := New("").GetTopListSongs("abc"); err == nil {
		t.Fatal("GetTopListSongs should return error for non numeric id")
	}
}

func TestNormalizeHotSongLimit(t *testing.T) {
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
		if got := normalizeHotSongLimit(tc.in); got != tc.want {
			t.Fatalf("normalizeHotSongLimit(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestGetTopListsLive(t *testing.T) {
	topLists, err := New("").GetTopLists()
	if err != nil {
		t.Fatalf("GetTopLists failed: %v", err)
	}
	if len(topLists) == 0 {
		t.Fatal("GetTopLists returned no top lists")
	}

	byID := make(map[string]model.TopList, len(topLists))
	for _, topList := range topLists {
		byID[topList.ID] = topList
	}

	// 官方榜单 ID 长期稳定：19723756 飙升榜 / 3778678 热歌榜
	soaring, ok := byID["19723756"]
	if !ok {
		t.Fatalf("expected 飙升榜 (19723756) in top lists, got %d items", len(topLists))
	}
	if soaring.Name == "" || soaring.Cover == "" || soaring.TrackCount == 0 || soaring.PlayCount == 0 {
		t.Fatalf("unexpected 飙升榜 metadata: %#v", soaring)
	}

	hot, ok := byID[HotSongTopListID]
	if !ok {
		t.Fatalf("expected 热歌榜 (%s) in top lists, got %d items", HotSongTopListID, len(topLists))
	}
	if !strings.Contains(hot.Name, "热歌") {
		t.Fatalf("unexpected 热歌榜 name: %q", hot.Name)
	}
}

func TestGetTopListSongsLive(t *testing.T) {
	songs, err := New("").GetTopListSongs(HotSongTopListID)
	if err != nil {
		t.Fatalf("GetTopListSongs failed: %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("GetTopListSongs returned no songs")
	}

	first := songs[0]
	if first.ID == "" || first.Name == "" || first.Artist == "" {
		t.Fatalf("unexpected first song: %#v", first)
	}
	if first.Source != "netease" {
		t.Fatalf("first song source = %q, want netease", first.Source)
	}
	if first.Extra["rank"] != "1" || first.Extra["toplist_id"] != HotSongTopListID {
		t.Fatalf("unexpected first song extra: %#v", first.Extra)
	}

	last := songs[len(songs)-1]
	if last.Extra["rank"] != strconv.Itoa(len(songs)) {
		t.Fatalf("last song rank = %q, want %d", last.Extra["rank"], len(songs))
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
}

func TestTopListPackageWrappersLive(t *testing.T) {
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
	cookie := neteaseTestCookie()
	if cookie == "" {
		t.Skip("NETEASE_COOKIE not set, skip logged-in hot songs test")
	}

	n := New(cookie)
	songs, err := n.GetHotSongs(30)
	if err != nil {
		t.Fatalf("GetHotSongs(30) with cookie failed: %v", err)
	}
	if len(songs) != 30 {
		t.Fatalf("GetHotSongs(30) returned %d songs, want 30", len(songs))
	}

	all, err := n.GetHotSongs(0)
	if err != nil {
		t.Fatalf("GetHotSongs(0) with cookie failed: %v", err)
	}
	if len(all) < 30 || len(all) > DefaultHotSongLimit {
		t.Fatalf("GetHotSongs(0) returned %d songs, want between 30 and %d", len(all), DefaultHotSongLimit)
	}
}
