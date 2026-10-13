package migu

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

// miguTopListIndexFixture 是咪咕排行榜目录接口的真实响应结构（节选）：
// 「精选榜单」在真实响应里会出现两个同名分组，需要按 rankId 去重。
const miguTopListIndexFixture = `{
	"code": "000000",
	"info": "成功",
	"data": {
		"header": {"title": "排行榜"},
		"contents": [
			{
				"view": "ZJ-RANK-SPECIAL",
				"style": "咪咕特色榜",
				"contents": [
					{"view": "ZJ-Song-Scroll", "rankId": "83048887", "rankName": "全网热歌榜", "imageUrl": "https://d.musicapp.migu.cn/data/oss/column/00/1w/yi/2981744541a345958d285c75e8fa907c.webp"},
					{"view": "ZJ-Song-Scroll", "rankId": "83049014", "rankName": "抖音热歌榜", "imageUrl": "//d.musicapp.migu.cn/data/oss/column/00/1w/yi/douyin.webp"}
				]
			},
			{
				"view": "ZJ-RANK-NORMAL",
				"style": "咪咕尖叫榜",
				"contents": [
					{"view": "ZJ-Song-Scroll", "rankId": "27553319", "rankName": "新歌榜", "imageUrl": ""}
				]
			},
			{
				"view": "ZJ-RANK-MUSIC",
				"style": "精选榜单",
				"contents": [
					{"view": "ZJ-Song-Scroll", "rankId": "27553319", "rankName": "新歌榜", "imageUrl": ""},
					{"view": "ZJ-Song-Scroll", "rankId": "27186466", "rankName": "热歌榜", "imageUrl": "/data/oss/column/00/1w/yi/hot.webp"}
				]
			},
			{
				"view": "ZJ-RANK-MUSIC",
				"style": "精选榜单",
				"contents": [
					{"view": "ZJ-Song-Scroll", "rankId": "27553319", "rankName": "新歌榜", "imageUrl": ""},
					{"view": "ZJ-Song-Scroll", "rankId": "27186466", "rankName": "热歌榜", "imageUrl": "/data/oss/column/00/1w/yi/hot.webp"}
				]
			}
		]
	}
}`

func TestParseMiguTopLists(t *testing.T) {
	topLists, err := parseMiguTopLists([]byte(miguTopListIndexFixture))
	if err != nil {
		t.Fatalf("parseMiguTopLists returned error: %v", err)
	}
	// 4 个分组里有 2 个重复的「精选榜单」，去重后应剩 4 个榜单。
	if len(topLists) != 4 {
		t.Fatalf("parseMiguTopLists returned %d top lists, want 4 (重复分组需按 rankId 去重)", len(topLists))
	}

	first := topLists[0]
	if first.ID != "83048887" || first.Name != "全网热歌榜" {
		t.Fatalf("first toplist = %+v, want 83048887/全网热歌榜", first)
	}
	if first.Source != "migu" {
		t.Fatalf("first toplist source = %q, want migu", first.Source)
	}
	if first.Extra["rank_id"] != "83048887" || first.Extra["group"] != "咪咕特色榜" {
		t.Fatalf("first toplist extra = %#v, want rank_id/group", first.Extra)
	}
	if first.Extra["toplist_type"] != "rank" {
		t.Fatalf("first toplist type = %q, want rank", first.Extra["toplist_type"])
	}
	if first.Description != "咪咕音乐 · 咪咕特色榜" {
		t.Fatalf("first toplist description = %q, want 咪咕音乐 · 咪咕特色榜", first.Description)
	}
	if !strings.Contains(first.Link, "rankDetail?id=83048887") {
		t.Fatalf("first toplist link = %q, want it to contain rankDetail?id=83048887", first.Link)
	}
	if !strings.HasPrefix(first.Cover, "https://d.musicapp.migu.cn/") {
		t.Fatalf("first toplist cover = %q, want migu https url", first.Cover)
	}

	// 协议相对地址需要补全成 https。
	if topLists[1].Cover != "https://d.musicapp.migu.cn/data/oss/column/00/1w/yi/douyin.webp" {
		t.Fatalf("second toplist cover = %q, want https scheme completed", topLists[1].Cover)
	}
	if topLists[1].Extra["group"] != "咪咕特色榜" {
		t.Fatalf("second toplist group = %q, want 咪咕特色榜", topLists[1].Extra["group"])
	}

	// 新歌榜在尖叫榜与精选榜单里重复出现，去重后只保留第一次出现的分组。
	if topLists[2].ID != "27553319" || topLists[2].Extra["group"] != "咪咕尖叫榜" {
		t.Fatalf("third toplist = %+v, want 27553319 in 咪咕尖叫榜", topLists[2])
	}
	if topLists[3].ID != "27186466" || topLists[3].Extra["group"] != "精选榜单" {
		t.Fatalf("fourth toplist = %+v, want 27186466 in 精选榜单", topLists[3])
	}
	if topLists[3].Cover != "https://d.musicapp.migu.cn/data/oss/column/00/1w/yi/hot.webp" {
		t.Fatalf("fourth toplist cover = %q, want absolute path prefixed", topLists[3].Cover)
	}
}

func TestParseMiguTopListsErrors(t *testing.T) {
	if _, err := parseMiguTopLists([]byte(`not json`)); err == nil {
		t.Fatal("parseMiguTopLists should fail on invalid json")
	}
	if _, err := parseMiguTopLists([]byte(`{"code":"100001","info":"参数错误","data":{"contents":[]}}`)); err == nil {
		t.Fatal("parseMiguTopLists should fail on non-zero code")
	}
	if _, err := parseMiguTopLists([]byte(`{"code":"000000","data":{"contents":[]}}`)); err == nil {
		t.Fatal("parseMiguTopLists should fail when no rank is parsed")
	}
	// 缺少 rankId / rankName 的条目需要被跳过，全部无效时报错。
	if _, err := parseMiguTopLists([]byte(`{"code":"000000","data":{"contents":[{"style":"咪咕特色榜","contents":[{"rankId":"","rankName":"无ID"},{"rankId":"1","rankName":""}]}]}}`)); err == nil {
		t.Fatal("parseMiguTopLists should skip incomplete ranks and fail on empty result")
	}
}

// miguRankInfoSongData 生成榜单接口里的 songData 字段（结构与真实响应一致）。
const miguRankInfoSongDataFixture = `{\"album\":\"自由的你\",\"albumId\":\"1144025198\",\"audioFormats\":[{\"aformat\":\"020007\",\"asize\":\"4751571\",\"formatType\":\"PQ\",\"iformat\":\"020007\",\"isize\":\"4751571\",\"resourceType\":\"2\"},{\"aformat\":\"011005\",\"asize\":\"67730988\",\"formatType\":\"ZQ24\",\"iformat\":\"011006\",\"isize\":\"68717397\",\"resourceType\":\"2\"}],\"contentId\":\"600930000002751847\",\"copyrightId\":\"69053700067\",\"duration\":47,\"img1\":\"https://d.musicapp.migu.cn/data/oss/resource/00/60/za/d79797a915c0478ba046aa898b13640d.webp\",\"singerList\":[{\"id\":\"1000000747\",\"name\":\"G.E.M.邓紫棋\"}],\"songId\":\"1144025199\",\"songName\":\"自由的你\"}`

// miguRankInfoPageFixture 是榜单歌曲接口的单页响应（节选，songData 为真实字段）。
const miguRankInfoPageFixture = `{
	"code": "000000",
	"info": "成功",
	"data": {
		"title": "全网热歌榜",
		"desc": "每日更新",
		"totalCount": 50,
		"hasNextPage": false,
		"contents": [
			{
				"resType": "2",
				"resId": "600930000002751847",
				"txt": "自由的你",
				"txt2": "G.E.M.邓紫棋",
				"score": "77.20",
				"songData": "` + miguRankInfoSongDataFixture + `"
			},
			{
				"resType": "2",
				"resId": "600930000002751848",
				"txt": "没有 songData 的条目",
				"score": "70.00",
				"songData": ""
			}
		]
	}
}`

func TestParseMiguRankInfoResponse(t *testing.T) {
	resp, err := parseMiguRankInfoResponse([]byte(miguRankInfoPageFixture))
	if err != nil {
		t.Fatalf("parseMiguRankInfoResponse returned error: %v", err)
	}
	if resp.Data.TotalCount != 50 || len(resp.Data.Contents) != 2 {
		t.Fatalf("unexpected rank info: totalCount=%d contents=%d", resp.Data.TotalCount, len(resp.Data.Contents))
	}
	if resp.Data.Contents[0].Score != "77.20" {
		t.Fatalf("first item score = %q, want 77.20", resp.Data.Contents[0].Score)
	}

	if _, err := parseMiguRankInfoResponse([]byte(`{"code":"000000","data":{"contents":[]}}`)); err == nil {
		t.Fatal("parseMiguRankInfoResponse should fail on empty contents")
	}
	if _, err := parseMiguRankInfoResponse([]byte(`{"code":"100002","info":"榜单不存在","data":{"contents":[{"txt":"x"}]}}`)); err == nil {
		t.Fatal("parseMiguRankInfoResponse should fail on non-zero code")
	}
}

func TestConvertMiguRankItemToSong(t *testing.T) {
	resp, err := parseMiguRankInfoResponse([]byte(miguRankInfoPageFixture))
	if err != nil {
		t.Fatalf("parseMiguRankInfoResponse returned error: %v", err)
	}

	m := New("")
	song := m.convertRankItemToSong(resp.Data.Contents[0])
	if song == nil {
		t.Fatal("convertRankItemToSong returned nil for valid songData")
	}
	if song.Name != "自由的你" || song.Artist != "G.E.M.邓紫棋" || song.Album != "自由的你" {
		t.Fatalf("unexpected song meta: %+v", song)
	}
	if song.Source != "migu" {
		t.Fatalf("song source = %q, want migu", song.Source)
	}
	if song.Ext != "flac" || song.Size != 67730988 {
		t.Fatalf("song format = %q size = %d, want flac/67730988 (最高音质 ZQ24)", song.Ext, song.Size)
	}
	if song.Duration != 47 {
		t.Fatalf("song duration = %d, want 47", song.Duration)
	}
	if !strings.HasPrefix(song.Cover, "https://d.musicapp.migu.cn/") {
		t.Fatalf("song cover = %q, want migu url", song.Cover)
	}
	if song.Extra["content_id"] != "600930000002751847" || song.Extra["copyright_id"] != "69053700067" {
		t.Fatalf("unexpected song extra: %#v", song.Extra)
	}
	if !strings.Contains(song.Link, "600930000002751847") {
		t.Fatalf("song link = %q, want it to contain content id", song.Link)
	}

	if empty := m.convertRankItemToSong(resp.Data.Contents[1]); empty != nil {
		t.Fatalf("convertRankItemToSong should skip item without songData, got %+v", empty)
	}
	if broken := m.convertRankItemToSong(miguRankInfoItem{SongData: "{not json"}); broken != nil {
		t.Fatalf("convertRankItemToSong should skip broken songData, got %+v", broken)
	}
}

func TestMiguTopListHelpers(t *testing.T) {
	for _, id := range []string{"83048887", "1", "0"} {
		if !isMiguTopListID(id) {
			t.Fatalf("isMiguTopListID(%q) = false, want true", id)
		}
	}
	for _, id := range []string{"", "83048887a", "migu", "-1", "830 48887"} {
		if isMiguTopListID(id) {
			t.Fatalf("isMiguTopListID(%q) = true, want false", id)
		}
	}

	if got := normalizeMiguHotSongLimit(0); got != DefaultHotSongLimit {
		t.Fatalf("normalizeMiguHotSongLimit(0) = %d, want %d", got, DefaultHotSongLimit)
	}
	if got := normalizeMiguHotSongLimit(-5); got != DefaultHotSongLimit {
		t.Fatalf("normalizeMiguHotSongLimit(-5) = %d, want %d", got, DefaultHotSongLimit)
	}
	if got := normalizeMiguHotSongLimit(500); got != MaxHotSongLimit {
		t.Fatalf("normalizeMiguHotSongLimit(500) = %d, want %d", got, MaxHotSongLimit)
	}
	if got := normalizeMiguHotSongLimit(10); got != 10 {
		t.Fatalf("normalizeMiguHotSongLimit(10) = %d, want 10", got)
	}

	link := miguTopListLink("27553319")
	if link != "https://music.migu.cn/v5/#/rankDetail?id=27553319&playlistType=charts" {
		t.Fatalf("miguTopListLink = %q, want rankDetail 页面地址", link)
	}
}

func TestMiguGetTopListsFallback(t *testing.T) {
	fallbackBody := strings.ReplaceAll(miguTopListIndexFixture, "精选榜单", "PC 精选榜单")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/primary" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(fallbackBody))
	}))
	defer server.Close()

	original := miguTopListIndexAPIs
	miguTopListIndexAPIs = []string{server.URL + "/primary", server.URL + "/fallback"}
	defer func() { miguTopListIndexAPIs = original }()

	topLists, err := New("").GetTopLists()
	if err != nil {
		t.Fatalf("GetTopLists should fall back to the second index api: %v", err)
	}
	if len(topLists) != 4 {
		t.Fatalf("fallback GetTopLists returned %d top lists, want 4", len(topLists))
	}
	if topLists[3].Extra["group"] != "PC 精选榜单" {
		t.Fatalf("fallback group = %q, want PC 精选榜单", topLists[3].Extra["group"])
	}
}

func TestMiguGetTopListsAllFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	original := miguTopListIndexAPIs
	miguTopListIndexAPIs = []string{server.URL + "/a", server.URL + "/b"}
	defer func() { miguTopListIndexAPIs = original }()

	lists, err := New("").GetTopLists()
	if err == nil {
		t.Fatal("GetTopLists should fail when every index api fails")
	}
	if lists != nil {
		t.Fatalf("GetTopLists should return nil lists on failure, got %v", lists)
	}
	if !strings.Contains(err.Error(), "fallback") {
		t.Fatalf("error should mention fallback attempt: %v", err)
	}
}

// miguRankInfoItemJSON 生成榜单歌曲接口的单条记录，contentID 相同即为同一首歌。
func miguRankInfoItemJSON(contentID, songID, name, artist, score string) string {
	songData := fmt.Sprintf(
		`{\"album\":\"%s\",\"albumId\":\"%s\",\"audioFormats\":[{\"aformat\":\"020007\",\"asize\":\"4751571\",\"formatType\":\"PQ\",\"resourceType\":\"2\"}],\"contentId\":\"%s\",\"copyrightId\":\"%s\",\"duration\":47,\"img1\":\"https://d.musicapp.migu.cn/data/oss/resource/%s.webp\",\"singerList\":[{\"name\":\"%s\"}],\"songId\":\"%s\",\"songName\":\"%s\"}`,
		name, songID, contentID, contentID, contentID, artist, songID, name,
	)
	return fmt.Sprintf(
		`{"resType":"2","resId":"%s","txt":"%s","txt2":"%s","score":"%s","songData":"%s"}`,
		contentID, name, artist, score, songData,
	)
}

// miguRankInfoPageJSON 生成一页榜单歌曲响应。
func miguRankInfoPageJSON(hasNextPage bool, items ...string) string {
	return fmt.Sprintf(
		`{"code":"000000","info":"成功","data":{"title":"全网热歌榜","totalCount":%d,"hasNextPage":%t,"contents":[%s]}}`,
		len(items), hasNextPage, strings.Join(items, ","),
	)
}

func TestMiguFetchTopListSongsPagination(t *testing.T) {
	songA := miguRankInfoItemJSON("600930000002751847", "1144025199", "自由的你", "G.E.M.邓紫棋", "77.20")
	songB := miguRankInfoItemJSON("600930000002751900", "1144025200", "重复的歌", "测试歌手", "66.00")
	songC := miguRankInfoItemJSON("600930000002751950", "1144025250", "第二页的歌", "测试歌手", "55.00")

	// 第 2 页第一首与第 1 页第二首是同一首歌，用于验证跨页去重与名次重排。
	page1 := miguRankInfoPageJSON(true, songA, songB)
	page2 := miguRankInfoPageJSON(false, songB, songC)

	var requestedPages []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pageNo := r.URL.Query().Get("pageNo")
		requestedPages = append(requestedPages, pageNo)
		if r.URL.Query().Get("rankId") != HotSongTopListID {
			t.Errorf("rankId = %q, want %q", r.URL.Query().Get("rankId"), HotSongTopListID)
		}
		if pageNo == "1" {
			_, _ = w.Write([]byte(page1))
			return
		}
		_, _ = w.Write([]byte(page2))
	}))
	defer server.Close()

	original := miguTopListSongsAPI
	miguTopListSongsAPI = server.URL
	defer func() { miguTopListSongsAPI = original }()

	songs, err := New("").GetTopListSongs(HotSongTopListID)
	if err != nil {
		t.Fatalf("GetTopListSongs returned error: %v", err)
	}
	if len(songs) != 3 {
		t.Fatalf("GetTopListSongs returned %d songs, want 3 (跨页去重)", len(songs))
	}
	if strings.Join(requestedPages, ",") != "1,2" {
		t.Fatalf("requested pages = %v, want [1 2]", requestedPages)
	}
	if songs[0].Extra["rank"] != "1" || songs[1].Extra["rank"] != "2" || songs[2].Extra["rank"] != "3" {
		t.Fatalf("unexpected ranks: %q/%q/%q", songs[0].Extra["rank"], songs[1].Extra["rank"], songs[2].Extra["rank"])
	}
	if songs[2].Name != "第二页的歌" {
		t.Fatalf("third song name = %q, want 第二页的歌", songs[2].Name)
	}
	if songs[0].Extra["toplist_id"] != HotSongTopListID {
		t.Fatalf("toplist_id = %q, want %q", songs[0].Extra["toplist_id"], HotSongTopListID)
	}
	if songs[0].Extra["rank_score"] != "77.20" {
		t.Fatalf("rank_score = %q, want 77.20", songs[0].Extra["rank_score"])
	}

	limited, err := New("").GetHotSongs(1)
	if err != nil {
		t.Fatalf("GetHotSongs(1) returned error: %v", err)
	}
	if len(limited) != 1 || limited[0].Extra["rank"] != "1" {
		t.Fatalf("GetHotSongs(1) = %+v, want single song ranked 1", limited)
	}
}

func TestMiguFetchTopListSongsInvalidID(t *testing.T) {
	for _, id := range []string{"", "   ", "migu", "83048887a"} {
		if songs, err := New("").GetTopListSongs(id); err == nil || songs != nil {
			t.Fatalf("GetTopListSongs(%q) should fail, got songs=%v err=%v", id, songs, err)
		}
	}
}

func TestGetMiguTopListsLive(t *testing.T) {
	topLists, err := New("").GetTopLists()
	if err != nil {
		t.Fatalf("GetTopLists failed: %v", err)
	}
	// 移动端目录共 18 个榜单（PC 端兜底只有 9 个）。
	if len(topLists) < 9 {
		t.Fatalf("GetTopLists returned %d top lists, want at least 9", len(topLists))
	}

	var hotTopList *model.TopList
	for i := range topLists {
		if topLists[i].ID == HotSongTopListID {
			hotTopList = &topLists[i]
			break
		}
	}
	if hotTopList == nil {
		t.Fatalf("GetTopLists should contain 全网热歌榜 (%s)", HotSongTopListID)
	}
	if hotTopList.Name != "全网热歌榜" {
		t.Fatalf("hot toplist name = %q, want 全网热歌榜", hotTopList.Name)
	}
	if hotTopList.Extra["group"] == "" {
		t.Fatalf("hot toplist group is empty: %#v", hotTopList.Extra)
	}
	if !strings.HasPrefix(hotTopList.Cover, "http") {
		t.Fatalf("hot toplist cover = %q, want http url", hotTopList.Cover)
	}
}

func TestGetMiguTopListSongsLive(t *testing.T) {
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
	if first.Source != "migu" {
		t.Fatalf("first song source = %q, want migu", first.Source)
	}
	if first.Extra["rank"] != "1" || first.Extra["toplist_id"] != HotSongTopListID {
		t.Fatalf("unexpected first song extra: %#v", first.Extra)
	}
	if first.Duration == 0 || first.Size == 0 {
		t.Fatalf("first song should have duration/size: %#v", first)
	}
	if !strings.HasPrefix(first.Cover, "http") {
		t.Fatalf("first song cover = %q, want http url", first.Cover)
	}

	last := songs[len(songs)-1]
	if last.Extra["rank"] != fmt.Sprint(len(songs)) {
		t.Fatalf("last song rank = %q, want %d", last.Extra["rank"], len(songs))
	}
}

func TestGetMiguHotSongsLive(t *testing.T) {
	hot, err := New("").GetHotSongs(10)
	if err != nil {
		t.Fatalf("GetHotSongs failed: %v", err)
	}
	if len(hot) != 10 {
		t.Fatalf("GetHotSongs(10) returned %d songs, want 10", len(hot))
	}
	if hot[9].Extra["rank"] != "10" {
		t.Fatalf("GetHotSongs(10) last rank = %q, want 10", hot[9].Extra["rank"])
	}

	// limit 超过单榜上限时截断到 50。
	all, err := New("").GetHotSongs(200)
	if err != nil {
		t.Fatalf("GetHotSongs(200) failed: %v", err)
	}
	if len(all) != MaxHotSongLimit {
		t.Fatalf("GetHotSongs(200) returned %d songs, want %d", len(all), MaxHotSongLimit)
	}
}

func TestMiguTopListPackageWrappersLive(t *testing.T) {
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
