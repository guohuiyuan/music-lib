package qq

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

// qqTestCookie 优先读取 QQ_COOKIE 环境变量，其次读取 music-lib/.env。
func qqTestCookie() string {
	if cookie := strings.TrimSpace(os.Getenv("QQ_COOKIE")); cookie != "" {
		return cookie
	}

	b, err := os.ReadFile("../.env")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(line, "QQ_COOKIE=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "QQ_COOKIE="))
		}
	}
	return ""
}

func TestParseTopLists(t *testing.T) {
	const body = `{
		"code": 0,
		"topList": {
			"code": 0,
			"data": {
				"group": [
					{
						"groupId": 0,
						"groupName": "巅峰榜",
						"toplist": [
							{
								"topId": 26,
								"recType": 10005,
								"topType": 0,
								"updateType": 1,
								"title": "热歌榜",
								"titleDetail": "热歌榜 第285天",
								"intro": "1.榜单定义：QQ音乐站内播放热度前300首歌曲。<br>2.更新频率：每日更新。<br>",
								"period": "2026-10-12",
								"updateTime": "2026-10-12",
								"updateTips": "每日更新",
								"listenNum": 19900000,
								"totalNum": 300,
								"groupType": 0,
								"frontPicUrl": "http://y.gtimg.cn/music/photo_new/T003R300x300M000002bLkjw1iI7KT.jpg",
								"mbFrontPicUrl": "http://y.gtimg.cn/music/photo_new/T002R300x300M000004VSvF52mQoQp.jpg"
							},
							{
								"topId": 201,
								"recType": 10010,
								"title": "MV榜",
								"intro": "QQ音乐播放得分最高的官方MV TOP排行榜。",
								"totalNum": 20,
								"frontPicUrl": "http://y.gtimg.cn/music/photo_new/T003R300x300M000001HFlpf1NOEk1.jpg"
							}
						]
					},
					{
						"groupId": 3,
						"groupName": "全球榜",
						"toplist": [
							{
								"topId": 108,
								"recType": 10005,
								"topType": 1,
								"title": "美国公告牌榜",
								"intro": "Billboard 公告牌Hot 100榜单。",
								"period": "2026_40",
								"updateTime": "2026-10-07",
								"updateTips": "每周更新",
								"listenNum": 1980000,
								"totalNum": 97,
								"headPicUrl": "http://y.gtimg.cn/music/photo_new/T003R500x500M000004Eob6t3bWspV.jpg"
							},
							{
								"topId": 0,
								"title": "invalid"
							}
						]
					}
				]
			}
		}
	}`

	topLists, err := parseTopLists([]byte(body))
	if err != nil {
		t.Fatalf("parseTopLists returned error: %v", err)
	}
	if len(topLists) != 2 {
		t.Fatalf("parseTopLists returned %d top lists, want 2 (MV list and invalid item should be skipped)", len(topLists))
	}

	hot := topLists[0]
	if hot.ID != "26" {
		t.Fatalf("parseTopLists returned id %q, want 26", hot.ID)
	}
	if hot.Source != "qq" {
		t.Fatalf("parseTopLists returned source %q, want qq", hot.Source)
	}
	if hot.Name != "热歌榜" {
		t.Fatalf("parseTopLists returned name %q, want 热歌榜", hot.Name)
	}
	if hot.TrackCount != 300 {
		t.Fatalf("parseTopLists returned track count %d, want 300", hot.TrackCount)
	}
	if hot.PlayCount != 19900000 {
		t.Fatalf("parseTopLists returned play count %d, want 19900000", hot.PlayCount)
	}
	if hot.UpdateFrequency != "每日更新" {
		t.Fatalf("parseTopLists returned update frequency %q, want 每日更新", hot.UpdateFrequency)
	}
	if !strings.HasPrefix(hot.Cover, "https://") {
		t.Fatalf("parseTopLists returned cover %q, want https url", hot.Cover)
	}
	if strings.Contains(hot.Description, "<br>") {
		t.Fatalf("parseTopLists returned description with html tag: %q", hot.Description)
	}
	if !strings.Contains(hot.Link, "toplist/26") {
		t.Fatalf("parseTopLists returned link %q, want it to contain toplist/26", hot.Link)
	}
	if hot.Extra["group"] != "巅峰榜" {
		t.Fatalf("parseTopLists returned group %q, want 巅峰榜", hot.Extra["group"])
	}
	if hot.Extra["period"] != "2026-10-12" {
		t.Fatalf("parseTopLists returned period %q, want 2026-10-12", hot.Extra["period"])
	}
	if hot.Extra["update_time"] != "2026-10-12" {
		t.Fatalf("parseTopLists returned update time %q, want 2026-10-12", hot.Extra["update_time"])
	}

	billboard := topLists[1]
	if billboard.ID != "108" || billboard.Extra["group"] != "全球榜" {
		t.Fatalf("unexpected billboard top list: %#v", billboard)
	}
	if !strings.HasPrefix(billboard.Cover, "https://") {
		t.Fatalf("billboard cover fallback failed: %q", billboard.Cover)
	}
}

func TestParseTopListsError(t *testing.T) {
	if _, err := parseTopLists([]byte(`{"code": 0, "topList": {"code": 500}}`)); err == nil {
		t.Fatal("parseTopLists should return error when toplist code is not 0")
	}
	if _, err := parseTopLists([]byte(`not json`)); err == nil {
		t.Fatal("parseTopLists should return error for invalid json")
	}
}

func TestParseTopListSongs(t *testing.T) {
	const body = `{
		"code": 0,
		"songlist": [
			{
				"cur_count": "1",
				"data": {
					"songid": 8136,
					"songmid": "001fsNdn1zuZnA",
					"songname": "我不难过",
					"albumname": "未完成",
					"albummid": "004VSvF52mQoQp",
					"interval": 320,
					"size128": 5127719,
					"size320": 12819004,
					"sizeflac": 35801703,
					"strMediaMid": "000ao2Na47ZtsG",
					"pay": {"payplay": 1},
					"singer": [{"id": 109, "mid": "001pWERg3vFgg8", "name": "孙燕姿"}]
				}
			},
			{
				"data": {
					"songid": 2,
					"songmid": "",
					"songname": "no mid"
				}
			},
			{
				"data": {
					"songid": 3,
					"songmid": "002fNEP21O7Ghe",
					"songname": "无 flac 歌曲",
					"albumname": "专辑",
					"albummid": "001abc",
					"interval": 200,
					"size128": 3200000,
					"size320": 8000000,
					"singer": [{"name": "歌手A"}, {"name": "歌手B"}]
				}
			}
		]
	}`

	songs, err := parseTopListSongs([]byte(body), "26", 0)
	if err != nil {
		t.Fatalf("parseTopListSongs returned error: %v", err)
	}
	if len(songs) != 2 {
		t.Fatalf("parseTopListSongs returned %d songs, want 2", len(songs))
	}

	first := songs[0]
	if first.Source != "qq" || first.ID != "001fsNdn1zuZnA" || first.Name != "我不难过" {
		t.Fatalf("unexpected first song: %#v", first)
	}
	if first.Artist != "孙燕姿" || first.Album != "未完成" || first.Duration != 320 {
		t.Fatalf("unexpected first song metadata: %#v", first)
	}
	if first.Size != 35801703 || first.Bitrate != 895 {
		t.Fatalf("unexpected first song size/bitrate: size=%d bitrate=%d", first.Size, first.Bitrate)
	}
	if !strings.Contains(first.Cover, "004VSvF52mQoQp") {
		t.Fatalf("unexpected first song cover: %q", first.Cover)
	}
	if first.Extra["songmid"] != "001fsNdn1zuZnA" || first.Extra["song_id"] != "8136" {
		t.Fatalf("unexpected first song extra: %#v", first.Extra)
	}
	if first.Extra["rank"] != "1" || first.Extra["toplist_id"] != "26" {
		t.Fatalf("unexpected first song rank extra: %#v", first.Extra)
	}
	if first.Extra["media_mid"] != "000ao2Na47ZtsG" {
		t.Fatalf("unexpected first song media mid: %#v", first.Extra)
	}
	if !strings.Contains(first.Link, "songDetail/001fsNdn1zuZnA") {
		t.Fatalf("unexpected first song link: %q", first.Link)
	}

	// 第二首没有 cur_count，应回落到 begin+序号。
	second := songs[1]
	if second.Bitrate != 320 || second.Size != 8000000 {
		t.Fatalf("unexpected second song size/bitrate: size=%d bitrate=%d", second.Size, second.Bitrate)
	}
	if second.Artist != "歌手A, 歌手B" {
		t.Fatalf("unexpected second song artist: %q", second.Artist)
	}
	if second.Extra["rank"] != "3" {
		t.Fatalf("unexpected second song rank: %q, want 3", second.Extra["rank"])
	}
}

func TestParseTopListSongsError(t *testing.T) {
	if _, err := parseTopListSongs([]byte(`{"code": -1}`), "26", 0); err == nil {
		t.Fatal("parseTopListSongs should return error for non zero code")
	}
	if _, err := parseTopListSongs([]byte(`not json`), "26", 0); err == nil {
		t.Fatal("parseTopListSongs should return error for invalid json")
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
	if len(topLists) < 20 {
		t.Fatalf("GetTopLists returned %d top lists, want at least 20", len(topLists))
	}

	byID := make(map[string]model.TopList, len(topLists))
	for _, topList := range topLists {
		byID[topList.ID] = topList
	}

	// QQ 音乐热歌榜 ID 长期稳定为 26。
	hot, ok := byID[HotSongTopListID]
	if !ok {
		t.Fatalf("expected 热歌榜 (%s) in top lists, got %d items", HotSongTopListID, len(topLists))
	}
	if !strings.Contains(hot.Name, "热歌") {
		t.Fatalf("unexpected 热歌榜 name: %q", hot.Name)
	}
	if hot.Cover == "" || hot.TrackCount == 0 || hot.PlayCount == 0 || hot.UpdateFrequency == "" {
		t.Fatalf("unexpected 热歌榜 metadata: %#v", hot)
	}
	if hot.Source != "qq" {
		t.Fatalf("unexpected 热歌榜 source: %q", hot.Source)
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
	if first.ID == "" || first.Name == "" || first.Artist == "" || first.Album == "" {
		t.Fatalf("unexpected first song: %#v", first)
	}
	if first.Source != "qq" {
		t.Fatalf("first song source = %q, want qq", first.Source)
	}
	if first.Extra["rank"] != "1" || first.Extra["toplist_id"] != HotSongTopListID {
		t.Fatalf("unexpected first song extra: %#v", first.Extra)
	}
	if first.Extra["songmid"] == "" || first.Duration == 0 || first.Size == 0 {
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
	cookie := qqTestCookie()
	if cookie == "" {
		t.Skip("QQ_COOKIE not set, skip logged-in hot songs test")
	}

	q := New(cookie)
	songs, err := q.GetHotSongs(30)
	if err != nil {
		t.Fatalf("GetHotSongs(30) with cookie failed: %v", err)
	}
	if len(songs) != 30 {
		t.Fatalf("GetHotSongs(30) returned %d songs, want 30", len(songs))
	}

	all, err := q.GetHotSongs(0)
	if err != nil {
		t.Fatalf("GetHotSongs(0) with cookie failed: %v", err)
	}
	if len(all) < 30 || len(all) > DefaultHotSongLimit {
		t.Fatalf("GetHotSongs(0) returned %d songs, want between 30 and %d", len(all), DefaultHotSongLimit)
	}
}
