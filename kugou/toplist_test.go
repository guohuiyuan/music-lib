package kugou

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

// kugouTestCookie 优先读取 KUGOU_COOKIE 环境变量，其次读取 music-lib/.env。
func kugouTestCookie() string {
	if cookie := strings.TrimSpace(os.Getenv("KUGOU_COOKIE")); cookie != "" {
		return cookie
	}

	b, err := os.ReadFile("../.env")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(line, "KUGOU_COOKIE=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "KUGOU_COOKIE="))
		}
	}
	return ""
}

func TestParseTopLists(t *testing.T) {
	const body = `{
		"status": 1,
		"errcode": 0,
		"error": "",
		"data": {
			"timestamp": 1791814807,
			"total": 2,
			"info": [
				{
					"rankid": 8888,
					"rankname": "TOP500",
					"ranktype": 2,
					"classify": 1,
					"update_frequency": "每天",
					"update_frequency_type": 1,
					"play_times": 10840742,
					"intro": "数据来源：全曲库歌曲\r\n排序方式：按照歌曲产生的完整播放，收藏和分享数据综合进行排名\r\n更新频率：每天",
					"imgurl": "http://imge.kugou.com/mcommon/{size}/20241219/20241219164209670219.png",
					"img_9": "http://imge.kugou.com/mcommon/{size}/20241219/20241219164221229666.png",
					"rank_id_publish_date": "2026-10-12 08:30:00",
					"issue": 285,
					"rank_cid": 131010,
					"extra": {
						"resp": {
							"all_total": 500,
							"recent_year_total": 124
						}
					}
				},
				{
					"rankid": 0,
					"rankname": "invalid"
				}
			]
		}
	}`

	topLists, err := parseTopLists([]byte(body))
	if err != nil {
		t.Fatalf("parseTopLists returned error: %v", err)
	}
	if len(topLists) != 1 {
		t.Fatalf("parseTopLists returned %d top lists, want 1", len(topLists))
	}

	got := topLists[0]
	if got.ID != "8888" {
		t.Fatalf("parseTopLists returned id %q, want 8888", got.ID)
	}
	if got.Source != "kugou" {
		t.Fatalf("parseTopLists returned source %q, want kugou", got.Source)
	}
	if got.Name != "TOP500" {
		t.Fatalf("parseTopLists returned name %q, want TOP500", got.Name)
	}
	if got.PlayCount != 10840742 {
		t.Fatalf("parseTopLists returned play count %d, want 10840742", got.PlayCount)
	}
	if got.TrackCount != 500 {
		t.Fatalf("parseTopLists returned track count %d, want 500", got.TrackCount)
	}
	if got.UpdateFrequency != "每天" {
		t.Fatalf("parseTopLists returned update frequency %q, want 每天", got.UpdateFrequency)
	}
	if !strings.HasPrefix(got.Cover, "https://") || strings.Contains(got.Cover, "{size}") {
		t.Fatalf("parseTopLists returned cover %q, want normalized https url", got.Cover)
	}
	if strings.Contains(got.Description, "\r") || strings.Contains(got.Description, "\n") {
		t.Fatalf("parseTopLists returned description with newlines: %q", got.Description)
	}
	if got.Extra["group"] != "热门榜" {
		t.Fatalf("parseTopLists returned group %q, want 热门榜", got.Extra["group"])
	}
	if got.Extra["issue"] != "285" {
		t.Fatalf("parseTopLists returned issue %q, want 285", got.Extra["issue"])
	}
	if got.Extra["publish_time"] != "2026-10-12 08:30:00" {
		t.Fatalf("parseTopLists returned publish time %q", got.Extra["publish_time"])
	}
	if !strings.Contains(got.Link, "1-8888.html") {
		t.Fatalf("parseTopLists returned link %q, want it to contain 1-8888.html", got.Link)
	}
}

func TestParseTopListsError(t *testing.T) {
	if _, err := parseTopLists([]byte(`{"status": 0, "errcode": 20001, "error": "sign error"}`)); err == nil {
		t.Fatal("parseTopLists should return error when status is not 1")
	}
	if _, err := parseTopLists([]byte(`not json`)); err == nil {
		t.Fatal("parseTopLists should return error for invalid json")
	}
}

func TestKugouTopListGroupName(t *testing.T) {
	tests := map[int]string{
		1: "热门榜",
		2: "地区榜",
		3: "特色榜",
		4: "全球榜",
		5: "曲风榜",
		9: "其他",
	}
	for classify, want := range tests {
		if got := kugouTopListGroupName(classify); got != want {
			t.Fatalf("kugouTopListGroupName(%d) = %q, want %q", classify, got, want)
		}
	}
}

func TestParseTopListSongs(t *testing.T) {
	const body = `{
		"status": 1,
		"errcode": 0,
		"error": "",
		"data": {
			"timestamp": 1791814808,
			"total": 500,
			"info": [
				{
					"hash": "3FD7E1EC51122C745C637735C8BDE716",
					"hash_high": "BBB4612A8CDADC66379213532AD01EAB",
					"hash_super": "",
					"320hash": "C4F5F3EAB5EDDA7E150A02BBE9D4D2E3",
					"sqhash": "40652F6D57A19CC77E9E2416A5A4F8EE",
					"filesize": 5127719,
					"320filesize": 12819004,
					"sqfilesize": 35801703,
					"duration": 320,
					"bitrate": 128,
					"songname": "我不难过",
					"filename": "孙燕姿 - 我不难过",
					"remark": "未完成",
					"album_id": "965883",
					"audio_id": 284977,
					"album_audio_id": 32092078,
					"mvhash": "578CF283D47FBF2408CCF1AA735BDF17",
					"privilege": 10,
					"album_sizable_cover": "http://imge.kugou.com/stdmusic/{size}/20250624/20250624174203375937.jpg",
					"authors": [{"author_id": 5826, "author_name": "孙燕姿"}],
					"trans_param": {
						"union_cover": "http://imge.kugou.com/stdmusic/{size}/20250624/20250624174203375937.jpg",
						"ogg_320_hash": "DE2469A09E65DA06DA607FFF6B508609",
						"ogg_128_hash": "2EEA0C973256E6239B2F40851657E51E"
					}
				},
				{
					"hash": "",
					"songname": "no hash"
				},
				{
					"hash": "E7A341184B8F800BD4B2B29D32621BDC",
					"320hash": "E6CA90239255338D574FD42D35840406",
					"filesize": 2200000,
					"320filesize": 5442786,
					"duration": 136,
					"bitrate": 0,
					"songname": "",
					"filename": "风里微醺 - 各凭本事闯天地",
					"album_id": "209421287",
					"privilege": 8,
					"authors": [{"author_name": "风里微醺"}],
					"trans_param": {}
				}
			]
		}
	}`

	songs, total, err := parseTopListSongs([]byte(body), "8888", 100)
	if err != nil {
		t.Fatalf("parseTopListSongs returned error: %v", err)
	}
	if total != 500 {
		t.Fatalf("parseTopListSongs returned total %d, want 500", total)
	}
	if len(songs) != 2 {
		t.Fatalf("parseTopListSongs returned %d songs, want 2", len(songs))
	}

	first := songs[0]
	if first.Source != "kugou" || first.ID != "3FD7E1EC51122C745C637735C8BDE716" {
		t.Fatalf("unexpected first song: %#v", first)
	}
	if first.Name != "我不难过" || first.Artist != "孙燕姿" || first.Album != "未完成" {
		t.Fatalf("unexpected first song metadata: %#v", first)
	}
	if first.Duration != 320 || first.Size != 5127719 || first.Bitrate != 128 {
		t.Fatalf("unexpected first song size/bitrate: size=%d bitrate=%d duration=%d", first.Size, first.Bitrate, first.Duration)
	}
	if !strings.HasPrefix(first.Cover, "https://") || strings.Contains(first.Cover, "{size}") {
		t.Fatalf("unexpected first song cover: %q", first.Cover)
	}
	if first.Extra["hash"] != first.ID || first.Extra["file_hash"] != first.ID {
		t.Fatalf("unexpected first song hash extra: %#v", first.Extra)
	}
	if first.Extra["sq_hash"] != "40652F6D57A19CC77E9E2416A5A4F8EE" || first.Extra["hq_hash"] != "C4F5F3EAB5EDDA7E150A02BBE9D4D2E3" {
		t.Fatalf("unexpected first song quality hashes: %#v", first.Extra)
	}
	if first.Extra["high_hash"] != "BBB4612A8CDADC66379213532AD01EAB" {
		t.Fatalf("unexpected first song high hash: %#v", first.Extra)
	}
	if first.Extra["rank"] != "101" || first.Extra["toplist_id"] != "8888" {
		t.Fatalf("unexpected first song rank extra: %#v", first.Extra)
	}
	if first.Extra["audio_id"] != "284977" || first.Extra["album_audio_id"] != "32092078" {
		t.Fatalf("unexpected first song ids: %#v", first.Extra)
	}
	if !strings.Contains(first.Link, "hash=3FD7E1EC51122C745C637735C8BDE716") {
		t.Fatalf("unexpected first song link: %q", first.Link)
	}

	// 第二首没有 songname，应从 filename 回补，并按 offset+序号计算名次。
	second := songs[1]
	if second.Name != "各凭本事闯天地" || second.Artist != "风里微醺" {
		t.Fatalf("unexpected second song metadata: %#v", second)
	}
	if second.Extra["rank"] != "103" {
		t.Fatalf("unexpected second song rank: %q, want 103", second.Extra["rank"])
	}
	if second.Bitrate <= 0 {
		t.Fatalf("second song bitrate should fall back to computed value: %d", second.Bitrate)
	}
}

func TestParseTopListSongsError(t *testing.T) {
	if _, _, err := parseTopListSongs([]byte(`{"status": 0, "errcode": 20001}`), "8888", 0); err == nil {
		t.Fatal("parseTopListSongs should return error when status is not 1")
	}
	if _, _, err := parseTopListSongs([]byte(`not json`), "8888", 0); err == nil {
		t.Fatal("parseTopListSongs should return error for invalid json")
	}
}

func TestGetTopListSongsInvalidID(t *testing.T) {
	if _, err := New("").GetTopListSongs(""); err == nil {
		t.Fatal("GetTopListSongs should return error for empty id")
	}
	if _, err := New("").GetTopListSongs("abcd"); err == nil {
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
	if len(topLists) < 50 {
		t.Fatalf("GetTopLists returned %d top lists, want at least 50", len(topLists))
	}

	byID := make(map[string]model.TopList, len(topLists))
	for _, topList := range topLists {
		byID[topList.ID] = topList
	}

	// 酷狗 TOP500 榜单 ID 长期稳定为 8888。
	hot, ok := byID[HotSongTopListID]
	if !ok {
		t.Fatalf("expected TOP500 (%s) in top lists, got %d items", HotSongTopListID, len(topLists))
	}
	if hot.Name != "TOP500" {
		t.Fatalf("unexpected TOP500 name: %q", hot.Name)
	}
	if hot.Cover == "" || hot.PlayCount == 0 || hot.Description == "" || hot.UpdateFrequency == "" {
		t.Fatalf("unexpected TOP500 metadata: %#v", hot)
	}
	if hot.TrackCount != 500 {
		t.Fatalf("unexpected TOP500 track count: %d, want 500", hot.TrackCount)
	}
	if hot.Source != "kugou" {
		t.Fatalf("unexpected TOP500 source: %q", hot.Source)
	}
	if hot.Extra["group"] != "热门榜" {
		t.Fatalf("unexpected TOP500 group: %q", hot.Extra["group"])
	}
}

func TestGetTopListSongsLive(t *testing.T) {
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
	if first.Source != "kugou" {
		t.Fatalf("first song source = %q, want kugou", first.Source)
	}
	if first.Extra["rank"] != "1" || first.Extra["toplist_id"] != HotSongTopListID {
		t.Fatalf("unexpected first song extra: %#v", first.Extra)
	}
	if first.Extra["hash"] == "" || first.Duration == 0 || first.Size == 0 {
		t.Fatalf("first song missing download metadata: %#v", first)
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
	if songs[4].Extra["rank"] != "5" {
		t.Fatalf("GetHotSongs(5) last rank = %q, want 5", songs[4].Extra["rank"])
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
	cookie := kugouTestCookie()
	if cookie == "" {
		t.Skip("KUGOU_COOKIE not set, skip logged-in hot songs test")
	}

	k := New(cookie)
	songs, err := k.GetHotSongs(30)
	if err != nil {
		t.Fatalf("GetHotSongs(30) with cookie failed: %v", err)
	}
	if len(songs) != 30 {
		t.Fatalf("GetHotSongs(30) returned %d songs, want 30", len(songs))
	}

	all, err := k.GetHotSongs(0)
	if err != nil {
		t.Fatalf("GetHotSongs(0) with cookie failed: %v", err)
	}
	if len(all) < 30 || len(all) > DefaultHotSongLimit {
		t.Fatalf("GetHotSongs(0) returned %d songs, want between 30 and %d", len(all), DefaultHotSongLimit)
	}
}
