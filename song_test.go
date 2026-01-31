package main

import (
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/jamendo"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
)

// Common test keywords
// 使用 "痛仰乐队" (Miserable Faith) 代替 "周杰伦"
const testArtistKeyword = "痛仰乐队"

// 使用 "再见杰克" 代替 "小苹果"/"海阔天空"
const testSongKeyword = "再见杰克"

// TestKugouSearch 测试酷狗音乐搜索
func TestKugouSearch(t *testing.T) {
	keyword := testArtistKeyword
	songs, err := kugou.Search(keyword)
	if err != nil {
		t.Fatalf("Kugou Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Fatal("Kugou Search returned no songs")
	}

	for i, song := range songs {
		if song.Source != "kugou" {
			t.Errorf("Song %d: expected source 'kugou', got '%s'", i, song.Source)
		}
		if song.Name == "" {
			t.Errorf("Song %d: empty name", i)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
		// [适配] 验证 Link
		if song.Link == "" {
			t.Errorf("Song %d: empty Link", i)
		}
		// [适配] 验证 Extra
		if song.Extra == nil {
			t.Errorf("Song %d: Extra is nil", i)
		} else if song.Extra["hash"] == "" {
			t.Errorf("Song %d: Extra missing 'hash'", i)
		}
	}

	t.Logf("Kugou: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestKugouGetDownloadURL 测试酷狗音乐下载链接获取
func TestKugouGetDownloadURL(t *testing.T) {
	keyword := testSongKeyword
	songs, err := kugou.Search(keyword)
	if err != nil {
		t.Fatalf("Kugou Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Kugou GetDownloadURL")
	}

	// 尝试前3首歌曲
	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := kugou.GetDownloadURL(song)
			if err != nil {
				t.Logf("Kugou GetDownloadURL failed for %s: %v", song.Name, err)
				return
			}
			if url == "" {
				t.Error("Kugou GetDownloadURL returned empty URL")
			} else {
				t.Logf("Kugou: Got download URL for %s", song.Name)
			}
		})
	}
}

// TestQQSearch 测试QQ音乐搜索
func TestQQSearch(t *testing.T) {
	keyword := testArtistKeyword
	songs, err := qq.Search(keyword)
	if err != nil {
		t.Fatalf("QQ Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Fatal("QQ Search returned no songs")
	}

	for i, song := range songs {
		if song.Source != "qq" {
			t.Errorf("Song %d: expected source 'qq', got '%s'", i, song.Source)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
		// [适配] 验证 Link
		if song.Link == "" {
			t.Errorf("Song %d: empty Link", i)
		}
		// [适配] 验证 Extra
		if song.Extra == nil {
			t.Errorf("Song %d: Extra is nil", i)
		} else if song.Extra["songmid"] == "" {
			t.Errorf("Song %d: Extra missing 'songmid'", i)
		}
	}

	t.Logf("QQ: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestQQGetDownloadURL 测试QQ音乐下载链接获取
func TestQQGetDownloadURL(t *testing.T) {
	keyword := testSongKeyword
	songs, err := qq.Search(keyword)
	if err != nil {
		t.Fatalf("QQ Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing QQ GetDownloadURL")
	}

	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := qq.GetDownloadURL(song)
			if err != nil {
				t.Logf("QQ GetDownloadURL failed for %s: %v", song.Name, err)
				return
			}
			if url == "" {
				t.Error("QQ GetDownloadURL returned empty URL")
			} else {
				t.Logf("QQ: Got download URL for %s", song.Name)
			}
		})
	}
}

// TestMiguSearch 测试咪咕音乐搜索
func TestMiguSearch(t *testing.T) {
	keyword := testArtistKeyword
	songs, err := migu.Search(keyword)
	if err != nil {
		t.Fatalf("Migu Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Fatal("Migu Search returned no songs")
	}

	for i, song := range songs {
		if song.Source != "migu" {
			t.Errorf("Song %d: expected source 'migu', got '%s'", i, song.Source)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
		// [适配] 验证 Link
		if song.Link == "" {
			t.Errorf("Song %d: empty Link", i)
		}
		// [适配] 验证 Extra
		if song.Extra == nil {
			t.Errorf("Song %d: Extra is nil", i)
		} else {
			if song.Extra["content_id"] == "" {
				t.Errorf("Song %d: Extra missing 'content_id'", i)
			}
			if song.Extra["resource_type"] == "" {
				t.Errorf("Song %d: Extra missing 'resource_type'", i)
			}
			if song.Extra["format_type"] == "" {
				t.Errorf("Song %d: Extra missing 'format_type'", i)
			}
		}
	}

	t.Logf("Migu: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestMiguGetDownloadURL 测试咪咕音乐下载链接获取
func TestMiguGetDownloadURL(t *testing.T) {
	keyword := testArtistKeyword
	songs, err := migu.Search(keyword)
	if err != nil {
		t.Fatalf("Migu Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Migu GetDownloadURL")
	}

	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := migu.GetDownloadURL(song)
			if err != nil {
				t.Logf("Migu GetDownloadURL failed for %s: %v", song.Name, err)
				return
			}
			if url == "" {
				t.Error("Migu GetDownloadURL returned empty URL")
			} else {
				t.Logf("Migu: Got download URL for %s", song.Name)
			}
		})
	}
}

// TestNeteaseSearch 测试网易云音乐搜索
func TestNeteaseSearch(t *testing.T) {
	keyword := testSongKeyword
	songs, err := netease.Search(keyword)
	if err != nil {
		t.Fatalf("Netease Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Fatal("Netease Search returned no songs")
	}

	for i, song := range songs {
		if song.Source != "netease" {
			t.Errorf("Song %d: expected source 'netease', got '%s'", i, song.Source)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
		// [适配] 验证 Link
		if song.Link == "" {
			t.Errorf("Song %d: empty Link", i)
		}
		// [适配] 验证 Extra
		if song.Extra == nil {
			t.Errorf("Song %d: Extra is nil", i)
		} else if song.Extra["song_id"] == "" {
			t.Errorf("Song %d: Extra missing 'song_id'", i)
		}
	}

	t.Logf("Netease: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestNeteaseGetDownloadURL 测试网易云音乐下载链接获取
func TestNeteaseGetDownloadURL(t *testing.T) {
	keyword := testSongKeyword
	songs, err := netease.Search(keyword)
	if err != nil {
		t.Fatalf("Netease Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Netease GetDownloadURL")
	}

	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := netease.GetDownloadURL(song)
			if err != nil {
				t.Logf("Netease GetDownloadURL failed for %s: %v", song.Name, err)
				return
			}
			if url == "" {
				t.Error("Netease GetDownloadURL returned empty URL")
			} else {
				t.Logf("Netease: Got download URL for %s", song.Name)
			}
		})
	}
}

// TestKuwoSearch 测试酷我音乐搜索
func TestKuwoSearch(t *testing.T) {
	keyword := testArtistKeyword
	songs, err := kuwo.Search(keyword)
	if err != nil {
		t.Fatalf("Kuwo Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Fatal("Kuwo Search returned no songs")
	}

	for i, song := range songs {
		if song.Source != "kuwo" {
			t.Errorf("Song %d: expected source 'kuwo', got '%s'", i, song.Source)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
		// [适配] 验证 Link
		if song.Link == "" {
			t.Errorf("Song %d: empty Link", i)
		}
		// [适配] 验证 Extra
		if song.Extra == nil {
			t.Errorf("Song %d: Extra is nil", i)
		} else if song.Extra["rid"] == "" {
			t.Errorf("Song %d: Extra missing 'rid'", i)
		}
	}

	t.Logf("Kuwo: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestKuwoGetDownloadURL 测试酷我音乐下载链接获取
func TestKuwoGetDownloadURL(t *testing.T) {
	keyword := testArtistKeyword
	songs, err := kuwo.Search(keyword)
	if err != nil {
		t.Fatalf("Kuwo Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Kuwo GetDownloadURL")
	}

	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := kuwo.GetDownloadURL(song)
			if err != nil {
				t.Logf("Kuwo GetDownloadURL failed for %s: %v", song.Name, err)
				return
			}
			if url == "" {
				t.Error("Kuwo GetDownloadURL returned empty URL")
			} else {
				t.Logf("Kuwo: Got download URL for %s", song.Name)
			}
		})
	}
}

// TestBilibiliSearch 测试Bilibili音频搜索
func TestBilibiliSearch(t *testing.T) {
	keyword := testArtistKeyword
	songs, err := bilibili.Search(keyword)
	if err != nil {
		t.Errorf("Bilibili Search failed (可能触发风控): %v", err)
		return
	}

	if len(songs) == 0 {
		t.Error("Bilibili Search returned no songs")
		return
	}

	for i := 0; i < min(2, len(songs)); i++ {
		song := songs[i]
		if song.Source != "bilibili" {
			t.Errorf("Song %d: expected source 'bilibili', got '%s'", i, song.Source)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
		// [适配] 验证 Link
		if song.Link == "" {
			t.Errorf("Song %d: empty Link", i)
		}
		// [适配] 验证 Extra
		if song.Extra == nil {
			t.Errorf("Song %d: Extra is nil", i)
		} else {
			if song.Extra["bvid"] == "" {
				t.Errorf("Song %d: Extra missing 'bvid'", i)
			}
			if song.Extra["cid"] == "" {
				t.Errorf("Song %d: Extra missing 'cid'", i)
			}
		}
	}

	t.Logf("Bilibili: Found %d songs for keyword '%s' (检查了前2首)", len(songs), keyword)
}

// TestBilibiliGetDownloadURL 测试Bilibili音频下载链接获取
func TestBilibiliGetDownloadURL(t *testing.T) {
	keyword := testArtistKeyword
	songs, err := bilibili.Search(keyword)
	if err != nil {
		t.Errorf("Bilibili Search failed (可能触发风控): %v", err)
		return
	}

	if len(songs) == 0 {
		t.Error("No songs found for testing Bilibili GetDownloadURL")
		return
	}

	for i := 0; i < min(1, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := bilibili.GetDownloadURL(song)
			if err != nil {
				t.Logf("Bilibili GetDownloadURL failed for %s: %v", song.Name, err)
				return
			}
			if url == "" {
				t.Error("Bilibili GetDownloadURL returned empty URL")
			} else {
				t.Logf("Bilibili: Got download URL for %s", song.Name)
			}
		})
	}
}

// TestFiveSingSearch 测试FiveSing音乐搜索
func TestFiveSingSearch(t *testing.T) {
	keyword := "河图"
	songs, err := fivesing.Search(keyword)
	if err != nil {
		t.Fatalf("FiveSing Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("FiveSing Search returned no songs (可能API变更)")
	}

	for i, song := range songs {
		if song.Source != "fivesing" {
			t.Errorf("Song %d: expected source 'fivesing', got '%s'", i, song.Source)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
		// 检查ID格式 (向后兼容)
		if !strings.Contains(song.ID, "|") {
			t.Errorf("Song %d: ID should contain '|' separator, got '%s'", i, song.ID)
		}
		// [适配] 验证 Link
		if song.Link == "" {
			t.Errorf("Song %d: empty Link", i)
		}
		// [适配] 验证 Extra
		if song.Extra == nil {
			t.Errorf("Song %d: Extra is nil", i)
		} else {
			if song.Extra["songid"] == "" {
				t.Errorf("Song %d: Extra missing 'songid'", i)
			}
			if song.Extra["songtype"] == "" {
				t.Errorf("Song %d: Extra missing 'songtype'", i)
			}
		}
	}

	t.Logf("FiveSing: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestFiveSingGetDownloadURL 测试FiveSing音乐下载链接获取
func TestFiveSingGetDownloadURL(t *testing.T) {
	keyword := "河图"
	songs, err := fivesing.Search(keyword)
	if err != nil {
		t.Fatalf("FiveSing Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing FiveSing GetDownloadURL")
	}

	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := fivesing.GetDownloadURL(song)
			if err != nil {
				t.Logf("FiveSing GetDownloadURL failed for %s: %v", song.Name, err)
				return
			}
			if url == "" {
				t.Error("FiveSing GetDownloadURL returned empty URL")
			} else {
				t.Logf("FiveSing: Got download URL for %s", song.Name)
			}
		})
	}
}

// TestQianqianSearch 测试千千音乐搜索
func TestQianqianSearch(t *testing.T) {
	keyword := testArtistKeyword
	songs, err := qianqian.Search(keyword)
	if err != nil {
		t.Fatalf("Qianqian Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("Qianqian Search returned no songs (可能API变更)")
	}

	for i, song := range songs {
		if song.Source != "qianqian" {
			t.Errorf("Song %d: expected source 'qianqian', got '%s'", i, song.Source)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
		// [适配] 验证 Link
		if song.Link == "" {
			t.Errorf("Song %d: empty Link", i)
		}
		// [适配] 验证 Extra
		if song.Extra == nil {
			t.Errorf("Song %d: Extra is nil", i)
		} else if song.Extra["tsid"] == "" {
			t.Errorf("Song %d: Extra missing 'tsid'", i)
		}
	}

	t.Logf("Qianqian: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestQianqianGetDownloadURL 测试千千音乐下载链接获取
func TestQianqianGetDownloadURL(t *testing.T) {
	keyword := testSongKeyword
	songs, err := qianqian.Search(keyword)
	if err != nil {
		t.Fatalf("Qianqian Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Qianqian GetDownloadURL")
	}

	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := qianqian.GetDownloadURL(song)
			if err != nil {
				t.Logf("Qianqian GetDownloadURL failed for %s: %v", song.Name, err)
				return
			}
			if url == "" {
				t.Error("Qianqian GetDownloadURL returned empty URL")
			} else {
				t.Logf("Qianqian: Got download URL for %s", song.Name)
			}
		})
	}
}

// TestSodaSearch 测试汽水音乐搜索
func TestSodaSearch(t *testing.T) {
	keyword := testArtistKeyword
	songs, err := soda.Search(keyword)
	if err != nil {
		t.Fatalf("Soda Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("Soda Search returned no songs (可能API变更)")
	}

	for i, song := range songs {
		if song.Source != "soda" {
			t.Errorf("Song %d: expected source 'soda', got '%s'", i, song.Source)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
		// [适配] 验证 Link
		if song.Link == "" {
			t.Errorf("Song %d: empty Link", i)
		}
		// [适配] 验证 Extra
		if song.Extra == nil {
			t.Errorf("Song %d: Extra is nil", i)
		} else if song.Extra["track_id"] == "" {
			t.Errorf("Song %d: Extra missing 'track_id'", i)
		}
	}

	t.Logf("Soda: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestSodaGetDownloadURL 测试汽水音乐下载链接获取
func TestSodaGetDownloadURL(t *testing.T) {
	keyword := testSongKeyword
	songs, err := soda.Search(keyword)
	if err != nil {
		t.Fatalf("Soda Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Soda GetDownloadURL")
	}

	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := soda.GetDownloadURL(song)
			if err != nil {
				t.Logf("Soda GetDownloadURL failed for %s: %v", song.Name, err)
				return
			}
			if url == "" {
				t.Error("Soda GetDownloadURL returned empty URL")
			} else {
				t.Logf("Soda: Got download URL for %s", song.Name)
			}
		})
	}
}

// TestJamendoSearch 测试Jamendo音乐搜索
func TestJamendoSearch(t *testing.T) {
	keyword := "acoustic"
	songs, err := jamendo.Search(keyword)
	if err != nil {
		t.Fatalf("Jamendo Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("Jamendo Search returned no songs (可能API变更)")
	}

	for i, song := range songs {
		if song.Source != "jamendo" {
			t.Errorf("Song %d: expected source 'jamendo', got '%s'", i, song.Source)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
		// [适配] 验证 Link
		if song.Link == "" {
			t.Errorf("Song %d: empty Link", i)
		}
		// [适配] 验证 Extra
		if song.Extra == nil {
			t.Errorf("Song %d: Extra is nil", i)
		} else if song.Extra["track_id"] == "" {
			t.Errorf("Song %d: Extra missing 'track_id'", i)
		}
	}

	t.Logf("Jamendo: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestJamendoGetDownloadURL 测试Jamendo音乐下载链接获取
func TestJamendoGetDownloadURL(t *testing.T) {
	keyword := "acoustic"
	songs, err := jamendo.Search(keyword)
	if err != nil {
		t.Fatalf("Jamendo Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Jamendo GetDownloadURL")
	}

	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := jamendo.GetDownloadURL(song)
			if err != nil {
				t.Logf("Jamendo GetDownloadURL failed for %s: %v", song.Name, err)
				return
			}
			if url == "" {
				t.Error("Jamendo GetDownloadURL returned empty URL")
			} else {
				t.Logf("Jamendo: Got download URL for %s", song.Name)
			}
		})
	}
}

// TestJooxSearch 测试Joox音乐搜索
func TestJooxSearch(t *testing.T) {
	keyword := testArtistKeyword
	songs, err := joox.Search(keyword)
	if err != nil {
		t.Fatalf("Joox Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("Joox Search returned no songs (可能API变更或地区限制)")
	}

	for i, song := range songs {
		if song.Source != "joox" {
			t.Errorf("Song %d: expected source 'joox', got '%s'", i, song.Source)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
		// [适配] 验证 Link
		if song.Link == "" {
			t.Errorf("Song %d: empty Link", i)
		}
		// [适配] 验证 Extra
		if song.Extra == nil {
			t.Errorf("Song %d: Extra is nil", i)
		} else if song.Extra["songid"] == "" {
			t.Errorf("Song %d: Extra missing 'songid'", i)
		}
	}

	t.Logf("Joox: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestJooxGetDownloadURL 测试Joox音乐下载链接获取
func TestJooxGetDownloadURL(t *testing.T) {
	keyword := testSongKeyword
	songs, err := joox.Search(keyword)
	if err != nil {
		t.Fatalf("Joox Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Joox GetDownloadURL")
	}

	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := joox.GetDownloadURL(song)
			if err != nil {
				t.Logf("Joox GetDownloadURL failed for %s: %v", song.Name, err)
				return
			}
			if url == "" {
				t.Error("Joox GetDownloadURL returned empty URL")
			} else {
				t.Logf("Joox: Got download URL for %s", song.Name)
			}
		})
	}
}

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestAllSourcesSearch 测试所有音乐源的搜索功能
func TestAllSourcesSearch(t *testing.T) {
	keyword := testArtistKeyword
	sources := []struct {
		name   string
		search func(string) ([]model.Song, error)
	}{
		{"kugou", kugou.Search},
		{"qq", qq.Search},
		{"migu", migu.Search},
		{"netease", netease.Search},
		{"kuwo", kuwo.Search},
		{"fivesing", fivesing.Search},
		{"qianqian", qianqian.Search},
		{"soda", soda.Search},
		{"jamendo", jamendo.Search},
		{"joox", joox.Search},
		{"bilibili", bilibili.Search},
	}

	for _, src := range sources {
		src := src // 捕获循环变量
		t.Run(src.name, func(t *testing.T) {
			t.Parallel()
			songs, err := src.search(keyword)
			if err != nil {
				if src.name == "jamendo" {
					t.Skipf("Jamendo search failed (可能关键词不支持): %v", err)
				}
				if src.name == "fivesing" {
					t.Skipf("Fivesing search failed (可能风格不匹配): %v", err)
				}
				t.Errorf("%s Search failed: %v", src.name, err)
				return
			}
			if len(songs) == 0 {
				t.Skipf("%s Search returned no songs (可能API变更)", src.name)
			}
			song := songs[0]
			if song.Source != src.name {
				t.Errorf("%s: expected source '%s', got '%s'", src.name, src.name, song.Source)
			}
			if song.ID == "" {
				t.Errorf("%s: empty ID", src.name)
			}
			// [适配] 验证 Link
			if song.Link == "" {
				t.Errorf("%s: Link is empty", src.name)
			}
			// 通用 Extra 检查：确保所有源都正确初始化了 Extra
			if song.Extra == nil {
				t.Errorf("%s: Extra is nil (should be initialized)", src.name)
			}
			t.Logf("%s: Found %d songs for keyword '%s'", src.name, len(songs), keyword)
		})
	}
}

// TestLyricsInterfaces 测试所有平台的歌词接口
func TestLyricsInterfaces(t *testing.T) {
	supportedSources := []struct {
		name      string
		search    func(string) ([]model.Song, error)
		getLyrics func(*model.Song) (string, error)
	}{
		{"netease", netease.Search, netease.GetLyrics},
		{"kuwo", kuwo.Search, kuwo.GetLyrics},
		{"soda", soda.Search, soda.GetLyrics},
		{"qq", qq.Search, qq.GetLyrics},
		{"kugou", kugou.Search, kugou.GetLyrics},
		{"qianqian", qianqian.Search, qianqian.GetLyrics},
		{"migu", migu.Search, migu.GetLyrics},
		{"fivesing", fivesing.Search, fivesing.GetLyrics},
	}

	for _, src := range supportedSources {
		src := src
		t.Run(src.name+"_lyrics", func(t *testing.T) {
			keyword := testArtistKeyword
			songs, err := src.search(keyword)
			if err != nil {
				t.Skipf("%s search failed: %v", src.name, err)
			}
			if len(songs) == 0 {
				t.Skipf("%s search returned no songs", src.name)
			}

			for i := 0; i < min(2, len(songs)); i++ {
				song := &songs[i]
				t.Run(song.Name, func(t *testing.T) {
					lyrics, err := src.getLyrics(song)
					if err != nil {
						t.Logf("%s GetLyrics failed for %s: %v", src.name, song.Name, err)
						return
					}
					if lyrics != "" {
						if !strings.Contains(lyrics, "[") || !strings.Contains(lyrics, "]") {
							t.Logf("%s: lyrics returned but not in standard LRC format", src.name)
						} else {
							t.Logf("%s: Got lyrics for %s (length: %d chars)", src.name, song.Name, len(lyrics))
						}
					} else {
						t.Logf("%s: No lyrics available for %s", src.name, song.Name)
					}
				})
			}
		})
	}

	unsupportedSources := []struct {
		name      string
		search    func(string) ([]model.Song, error)
		getLyrics func(*model.Song) (string, error)
	}{
		// 注意：Bilibili 的歌词接口有时会触发风控，导致无法获取结果
		// {"bilibili", bilibili.Search, bilibili.GetLyrics},
		{"jamendo", jamendo.Search, jamendo.GetLyrics},
	}

	for _, src := range unsupportedSources {
		src := src
		t.Run(src.name+"_lyrics_unsupported", func(t *testing.T) {
			keyword := testArtistKeyword
			songs, err := src.search(keyword)
			if err != nil {
				if src.name == "jamendo" || src.name == "bilibili" {
					t.Skipf("%s search failed: %v", src.name, err)
				}
				t.Errorf("%s search failed: %v", src.name, err)
				return
			}
			if len(songs) == 0 {
				if src.name == "jamendo" || src.name == "bilibili" {
					t.Skipf("%s search returned no songs", src.name)
				}
				t.Errorf("%s search returned no songs", src.name)
				return
			}

			song := &songs[0]
			lyrics, err := src.getLyrics(song)
			if err != nil {
				if src.name == "fivesing" {
					t.Logf("%s GetLyrics returned error as expected (unsupported): %v", src.name, err)
					return
				}
				t.Errorf("%s GetLyrics should not return error for unsupported platform, got: %v", src.name, err)
				return
			}
			if lyrics != "" {
				t.Logf("%s: GetLyrics returned non-empty string for unsupported platform", src.name)
			}
		})
	}
}

// TestLyricsSourceMismatch 测试歌词接口的源不匹配错误
func TestLyricsSourceMismatch(t *testing.T) {
	wrongSong := &model.Song{
		Source: "wrong_source",
		ID:     "123",
		Name:   "Test Song",
		Artist: "Test Artist",
	}

	platforms := []struct {
		name      string
		getLyrics func(*model.Song) (string, error)
	}{
		{"netease", netease.GetLyrics},
		{"kuwo", kuwo.GetLyrics},
		{"soda", soda.GetLyrics},
		{"qq", qq.GetLyrics},
		{"kugou", kugou.GetLyrics},
		{"qianqian", qianqian.GetLyrics},
		{"migu", migu.GetLyrics},
		{"bilibili", bilibili.GetLyrics},
		{"fivesing", fivesing.GetLyrics},
		{"jamendo", jamendo.GetLyrics},
	}

	for _, platform := range platforms {
		t.Run(platform.name+"_source_mismatch", func(t *testing.T) {
			_, err := platform.getLyrics(wrongSong)
			if err == nil {
				t.Errorf("%s GetLyrics should return error for source mismatch", platform.name)
			} else if !strings.Contains(err.Error(), "source mismatch") {
				t.Errorf("%s GetLyrics error should contain 'source mismatch', got: %v", platform.name, err)
			}
		})
	}
}

// TestParseFunctionality 测试链接解析功能
func TestParseFunctionality(t *testing.T) {
	// 定义要测试的平台
	// 注意：如果某个平台没有实现 Parse (package-level wrapper)，会导致编译错误。
	// 请确保对应包中导出了 Parse 函数。
	tests := []struct {
		name   string
		search func(string) ([]model.Song, error)
		parse  func(string) (*model.Song, error)
	}{
		{"kugou", kugou.Search, kugou.Parse},
		{"qq", qq.Search, qq.Parse},
		{"netease", netease.Search, netease.Parse},
		{"kuwo", kuwo.Search, kuwo.Parse},
		{"fivesing", fivesing.Search, fivesing.Parse},
		{"jamendo", jamendo.Search, jamendo.Parse},
		{"qianqian", qianqian.Search, qianqian.Parse},

		// 注意：Bilibili 的搜索有时会触发风控，导致无法获取结果
		// {"bilibili", bilibili.Search, bilibili.Parse},
		
		// 这三个渠道没有测试例子
		// {"migu", migu.Search, migu.Parse},
		// {"soda", soda.Search, soda.Parse},
		// {"joox", joox.Search, joox.Parse},
	}

	defaultKeyword := testSongKeyword

	for _, tt := range tests {
		tt := tt // 捕获循环变量
		t.Run(tt.name, func(t *testing.T) {
			// 针对特定平台调整关键词以确保能搜索到结果
			keyword := defaultKeyword
			if tt.name == "jamendo" {
				keyword = "acoustic"
			} else if tt.name == "fivesing" {
				keyword = "河图"
			}

			// 1. 搜索以获取一个有效的链接
			songs, err := tt.search(keyword)
			if err != nil {
				t.Skipf("%s search failed: %v", tt.name, err)
			}
			if len(songs) == 0 {
				t.Skipf("%s search returned no songs", tt.name)
			}

			// 2. 选取第一首有 Link 的歌曲
			var originSong *model.Song
			for i := range songs {
				if songs[i].Link != "" {
					originSong = &songs[i]
					break
				}
			}

			if originSong == nil {
				t.Skip("Search results have no links, cannot test Parse")
			}

			t.Logf("Testing Parse with link: %s", originSong.Link)

			// 3. 解析链接
			parsedSong, err := tt.parse(originSong.Link)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			// 4. 验证解析结果
			if parsedSong.Source != tt.name {
				t.Errorf("Expected source %s, got %s", tt.name, parsedSong.Source)
			}
			if parsedSong.ID == "" {
				t.Error("Parsed song has empty ID")
			}
			if parsedSong.Name == "" {
				t.Error("Parsed song has empty Name")
			}
			if parsedSong.URL == "" {
				t.Logf("Warning: Parsed song has empty download URL (Parse usually fetches metadata+url)")
			}

			// 记录结果
			t.Logf("Parse success: %s -> %s (ID: %s)", originSong.Name, parsedSong.Name, parsedSong.ID)
		})
	}
}
