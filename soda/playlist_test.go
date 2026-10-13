package soda

import (
	"strconv"
	"strings"
	"testing"
)

func TestSodaPlaylistCategories(t *testing.T) {
	categories, err := New("").GetPlaylistCategories()
	if err != nil {
		t.Fatalf("GetPlaylistCategories failed: %v", err)
	}
	if len(categories) != len(sodaPlaylistCategoryTags) {
		t.Fatalf("GetPlaylistCategories returned %d categories, want %d", len(categories), len(sodaPlaylistCategoryTags))
	}

	seen := make(map[string]bool, len(categories))
	for i, category := range categories {
		tag := sodaPlaylistCategoryTags[i]
		if category.Source != "soda" {
			t.Fatalf("category %d source = %q, want soda", i, category.Source)
		}
		if category.ID != strconv.Itoa(tag.ID) {
			t.Fatalf("category %d id = %q, want %d", i, category.ID, tag.ID)
		}
		if category.Name != tag.Name || category.Group != tag.Group {
			t.Fatalf("unexpected category %d: %#v", i, category)
		}
		if category.Extra["sub_channel_id"] != category.ID {
			t.Fatalf("category %d extra = %#v", i, category.Extra)
		}
		if seen[category.ID] {
			t.Fatalf("duplicated category id %q", category.ID)
		}
		seen[category.ID] = true
	}

	// 已知分类必须存在，避免后续误删。
	for _, want := range []string{"0|每日推荐", "8|华语", "14|流行", "16|电子", "19|R&B", "38|说唱", "40|学习", "45|睡前", "69|治愈"} {
		parts := strings.Split(want, "|")
		found := false
		for _, category := range categories {
			if category.ID == parts[0] && category.Name == parts[1] {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected category %s in categories", want)
		}
	}
}

func TestSodaCategoryChannel(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{in: "", want: 0},
		{in: "  ", want: 0},
		{in: "14", want: 14},
		{in: " 69 ", want: 69},
		{in: "abc", wantErr: true},
		{in: "-1", wantErr: true},
	}
	for _, tc := range tests {
		got, err := sodaCategoryChannel(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("sodaCategoryChannel(%q) should return error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("sodaCategoryChannel(%q) error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("sodaCategoryChannel(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSodaCategoryPlaylistRounds(t *testing.T) {
	tests := []struct {
		offset int
		limit  int
		want   int
	}{
		{offset: 0, limit: 5, want: sodaCategoryPlaylistMinRounds},
		{offset: 0, limit: 30, want: 4},
		{offset: 0, limit: 120, want: sodaCategoryPlaylistMaxRounds},
		{offset: 120, limit: 120, want: sodaCategoryPlaylistMaxRounds},
	}
	for _, tc := range tests {
		if got := sodaCategoryPlaylistRounds(tc.offset, tc.limit); got != tc.want {
			t.Fatalf("sodaCategoryPlaylistRounds(%d, %d) = %d, want %d", tc.offset, tc.limit, got, tc.want)
		}
	}
}

func TestSodaBuildCategoryPlaylist(t *testing.T) {
	items, err := parseSodaDiscoverMixResponse([]byte(sodaDiscoverMixTestBody))
	if err != nil {
		t.Fatalf("parseSodaDiscoverMixResponse returned error: %v", err)
	}

	playlist := sodaBuildCategoryPlaylist(items[0], 14)
	if playlist.ID != "7031085120368052254" || playlist.Source != "soda" {
		t.Fatalf("unexpected playlist: %#v", playlist)
	}
	if playlist.Name != "欧美性感男嗓 | 令人着迷的音色" {
		t.Fatalf("unexpected playlist name: %q", playlist.Name)
	}
	if playlist.TrackCount != 77 || playlist.Creator != "汽水音乐APP" {
		t.Fatalf("unexpected playlist metadata: %#v", playlist)
	}
	if playlist.Cover == "" || !strings.Contains(playlist.Cover, "ies-music/pgc_cover_0e9b2ca8bbd3a028e9e1759f460ed89f") {
		t.Fatalf("unexpected playlist cover: %q", playlist.Cover)
	}
	if strings.Contains(playlist.Description, "\n") {
		t.Fatalf("playlist description should not contain newlines: %q", playlist.Description)
	}
	if playlist.Extra["sub_channel_id"] != "14" {
		t.Fatalf("unexpected playlist extra: %#v", playlist.Extra)
	}
	if !strings.Contains(playlist.Link, "7031085120368052254") {
		t.Fatalf("unexpected playlist link: %q", playlist.Link)
	}

	if empty := sodaBuildCategoryPlaylist(sodaUserPlaylistItem{}, 0); empty.ID != "" {
		t.Fatalf("empty item should return empty playlist: %#v", empty)
	}
}

// TestGetPlaylistCategoriesLive 验证分类目录（离线内置），无需网络。
func TestGetPlaylistCategoriesLive(t *testing.T) {
	categories, err := New("").GetPlaylistCategories()
	if err != nil {
		t.Fatalf("GetPlaylistCategories failed: %v", err)
	}
	if len(categories) < 13 {
		t.Fatalf("GetPlaylistCategories returned %d categories, want at least 13", len(categories))
	}
}

// TestGetCategoryPlaylistsLive 验证每个分类都能拉到推荐歌单。
func TestGetCategoryPlaylistsLive(t *testing.T) {
	categories, err := New("").GetPlaylistCategories()
	if err != nil {
		t.Fatalf("GetPlaylistCategories failed: %v", err)
	}

	for _, category := range categories {
		category := category
		t.Run(category.Name, func(t *testing.T) {
			playlists, err := New("").GetCategoryPlaylists(category.ID, 1, 10)
			if err != nil {
				t.Fatalf("GetCategoryPlaylists(%s) failed: %v", category.ID, err)
			}
			if len(playlists) == 0 {
				t.Fatalf("GetCategoryPlaylists(%s) returned no playlists", category.ID)
			}
			if len(playlists) > 10 {
				t.Fatalf("GetCategoryPlaylists(%s) returned %d playlists, want <= 10", category.ID, len(playlists))
			}

			seen := make(map[string]bool, len(playlists))
			for _, playlist := range playlists {
				if playlist.Source != "soda" || playlist.ID == "" || playlist.Name == "" {
					t.Fatalf("unexpected playlist: %#v", playlist)
				}
				if seen[playlist.ID] {
					t.Fatalf("duplicated playlist id %q in category %s", playlist.ID, category.ID)
				}
				seen[playlist.ID] = true
			}
			t.Logf("category %s (%s): %d playlists, first=%s", category.ID, category.Name, len(playlists), playlists[0].Name)
		})
	}
}

// TestGetCategoryPlaylistsDefaultAndPagingLive 验证默认分类与本地分页行为。
func TestGetCategoryPlaylistsDefaultAndPagingLive(t *testing.T) {
	all, err := New("").GetCategoryPlaylists("", 1, 5)
	if err != nil {
		t.Fatalf("GetCategoryPlaylists(\"\") failed: %v", err)
	}
	if len(all) == 0 {
		t.Fatal("GetCategoryPlaylists(\"\") returned no playlists")
	}

	pageOne, err := New("").GetCategoryPlaylists("14", 1, 5)
	if err != nil {
		t.Fatalf("GetCategoryPlaylists(14, 1, 5) failed: %v", err)
	}
	if len(pageOne) == 0 || len(pageOne) > 5 {
		t.Fatalf("unexpected page one size: %d", len(pageOne))
	}

	pageTwo, err := New("").GetCategoryPlaylists("14", 2, 5)
	if err != nil {
		t.Fatalf("GetCategoryPlaylists(14, 2, 5) failed: %v", err)
	}
	if len(pageTwo) == 0 {
		t.Fatal("GetCategoryPlaylists(14, 2, 5) returned no playlists")
	}

	// 第 2 页应主要包含与第 1 页不同的歌单（随机推荐可能有少量重叠）。
	pageOneIDs := make(map[string]bool, len(pageOne))
	for _, playlist := range pageOne {
		pageOneIDs[playlist.ID] = true
	}
	distinct := 0
	for _, playlist := range pageTwo {
		if !pageOneIDs[playlist.ID] {
			distinct++
		}
	}
	if distinct == 0 {
		t.Fatalf("page 2 playlists duplicated page 1: %v", pageTwo)
	}

	if _, err := New("").GetCategoryPlaylists("abc", 1, 5); err == nil {
		t.Fatal("GetCategoryPlaylists should return error for invalid category id")
	}
}

// TestGetCategoryPlaylistsSongsLive 验证分类歌单可以继续解析歌曲。
func TestGetCategoryPlaylistsSongsLive(t *testing.T) {
	playlists, err := New("").GetCategoryPlaylists("8", 1, 5)
	if err != nil {
		t.Fatalf("GetCategoryPlaylists failed: %v", err)
	}
	if len(playlists) == 0 {
		t.Fatal("GetCategoryPlaylists returned no playlists")
	}

	songs, err := New("").GetPlaylistSongs(playlists[0].ID)
	if err != nil {
		t.Fatalf("GetPlaylistSongs(%s) failed: %v", playlists[0].ID, err)
	}
	if len(songs) == 0 {
		t.Fatalf("GetPlaylistSongs(%s) returned no songs", playlists[0].ID)
	}
	if songs[0].Source != "soda" || songs[0].ID == "" || songs[0].Name == "" {
		t.Fatalf("unexpected first song: %#v", songs[0])
	}
	t.Logf("playlist %s (%s) returned %d songs", playlists[0].ID, playlists[0].Name, len(songs))
}

// TestSodaPlaylistCategoryPackageWrappersLive 验证包级封装函数。
func TestSodaPlaylistCategoryPackageWrappersLive(t *testing.T) {
	categories, err := GetPlaylistCategories()
	if err != nil {
		t.Fatalf("GetPlaylistCategories failed: %v", err)
	}
	if len(categories) == 0 {
		t.Fatal("GetPlaylistCategories returned no categories")
	}

	playlists, err := GetCategoryPlaylists(categories[0].ID, 1, 3)
	if err != nil {
		t.Fatalf("GetCategoryPlaylists failed: %v", err)
	}
	if len(playlists) == 0 {
		t.Fatal("GetCategoryPlaylists returned no playlists")
	}
}
