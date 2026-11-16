package soda

import (
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestSodaPCWebPlaylistSearchURL(t *testing.T) {
	raw := sodaPCWebPlaylistSearchURL("邓紫棋", "550e8400-e29b-41d4-a716-446655440000", 20, 20)
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse soda pc web search url: %v", err)
	}
	if parsed.Path != "/luna/pc/search/playlist" {
		t.Fatalf("search path = %q", parsed.Path)
	}

	query := parsed.Query()
	for key, want := range map[string]string{
		"q":               "邓紫棋",
		"cursor":          "20",
		"count":           "20",
		"search_id":       "550e8400-e29b-41d4-a716-446655440000",
		"search_method":   "input",
		"aid":             "386088",
		"device_platform": "web",
		"channel":         "pc_web",
	} {
		if got := query.Get(key); got != want {
			t.Fatalf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestSodaNewSearchIDIsUUID(t *testing.T) {
	first, err := sodaNewSearchID()
	if err != nil {
		t.Fatalf("sodaNewSearchID error: %v", err)
	}
	second, err := sodaNewSearchID()
	if err != nil {
		t.Fatalf("sodaNewSearchID second error: %v", err)
	}
	if len(first) != 36 || first == second {
		t.Fatalf("unexpected search ids: %q %q", first, second)
	}
	if first[14] != '4' || !strings.ContainsRune("89ab", rune(first[19])) {
		t.Fatalf("search id is not UUID v4: %q", first)
	}
}

func TestParseSodaPlaylistSearchResponse(t *testing.T) {
	raw := []byte(`{
		"status_code": 0,
		"result_groups": [{
			"id": "playlists",
			"data": [
				{"entity": {"playlist": {
					"id": "7548732364248416310",
					"title": "邓紫棋热门歌曲",
					"desc": "精选歌单",
					"url_cover": {
						"urls": ["https://p3-luna.douyinpic.com/img/"],
						"uri": "tos-cn-i-b829550vbb/cover",
						"template_prefix": "tplv-b829550vbb"
					},
					"count_tracks": 49,
					"owner": {"id": "owner-1", "nickname": "昵称", "public_name": "公开名称"},
					"stats": {"count_played": 1234},
					"type": 2
				}}},
				{"entity": {"playlist": {
					"id": "7548732364248416310",
					"title": "重复结果"
				}}},
				{"entity": {"playlist": {
					"id": "second-playlist",
					"public_title": "公开标题",
					"count_tracks": 0,
					"resource_cnt": {"track_cnt": 8},
					"owner": {"nickname": "仅昵称"}
				}}},
				{"entity": {"playlist": {"id": ""}}}
			]
		}]
	}`)

	playlists, err := parseSodaPlaylistSearchResponse(raw)
	if err != nil {
		t.Fatalf("parseSodaPlaylistSearchResponse error: %v", err)
	}
	if len(playlists) != 2 {
		t.Fatalf("returned %d playlists, want 2", len(playlists))
	}

	first := playlists[0]
	if first.Source != "soda" || first.ID != "7548732364248416310" || first.Name != "邓紫棋热门歌曲" {
		t.Fatalf("unexpected first playlist: %#v", first)
	}
	if first.Creator != "公开名称" || first.TrackCount != 49 || first.PlayCount != 1234 {
		t.Fatalf("unexpected first playlist metadata: %#v", first)
	}
	if !strings.Contains(first.Cover, "~tplv-b829550vbb-resize:960:960.png") {
		t.Fatalf("cover = %q", first.Cover)
	}
	if first.Link != "https://www.qishui.com/playlist/7548732364248416310" {
		t.Fatalf("link = %q", first.Link)
	}

	second := playlists[1]
	if second.Name != "公开标题" || second.Creator != "仅昵称" || second.TrackCount != 8 {
		t.Fatalf("unexpected second playlist: %#v", second)
	}
}

func TestParseSodaPlaylistSearchResponseEmptySearch(t *testing.T) {
	_, err := parseSodaPlaylistSearchResponse([]byte(`{"extra":{"empty_search":1}}`))
	if err == nil || !strings.Contains(err.Error(), "empty_search") {
		t.Fatalf("empty_search should return an actionable error, got %v", err)
	}
}

func TestValidateSodaSearchBody(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "valid", body: `{"status_code":0,"result_groups":[]}`},
		{name: "empty", body: " \n\t", wantErr: "empty response"},
		{name: "invalid json", body: "<html>", wantErr: "invalid json"},
		{name: "api error", body: `{"status_code":1000016,"status_info":{"status_msg":"登录状态已失效"}}`, wantErr: "1000016"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSodaSearchBody([]byte(tc.body))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("validateSodaSearchBody error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want contains %q", err, tc.wantErr)
			}
		})
	}
}

func TestSodaSearchPlaylistLive(t *testing.T) {
	if strings.TrimSpace(os.Getenv("SODA_SEARCH_LIVE")) == "" {
		t.Skip("SODA_SEARCH_LIVE not set")
	}
	playlists, err := New("").SearchPlaylist("邓紫棋")
	if err != nil {
		t.Fatalf("SearchPlaylist error: %v", err)
	}
	if len(playlists) == 0 {
		t.Fatal("SearchPlaylist returned no playlists")
	}
	foundKeyword := false
	for _, playlist := range playlists {
		if playlist.Source != "soda" || playlist.ID == "" || playlist.Name == "" {
			t.Fatalf("invalid soda playlist: %#v", playlist)
		}
		foundKeyword = foundKeyword || strings.Contains(playlist.Name, "邓紫棋")
	}
	if !foundKeyword {
		t.Fatalf("no keyword-relevant playlist in %d results", len(playlists))
	}
	t.Logf("soda live search returned %d playlists; first=%q", len(playlists), playlists[0].Name)
}

func TestSodaSongSearchLive(t *testing.T) {
	if strings.TrimSpace(os.Getenv("SODA_SEARCH_LIVE")) == "" {
		t.Skip("SODA_SEARCH_LIVE not set")
	}
	client := New(readSodaCookie())
	if !client.HasSigner() {
		t.Skip("soda signer is unavailable")
	}
	songs, err := client.Search("邓紫棋")
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("Search returned no songs")
	}
	for _, song := range songs {
		if song.Source != "soda" || song.ID == "" || song.Name == "" {
			t.Fatalf("invalid soda song: %#v", song)
		}
	}
	t.Logf("soda live song search returned %d songs; first=%q", len(songs), songs[0].Name)
}
