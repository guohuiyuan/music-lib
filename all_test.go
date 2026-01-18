package main

import (
	"os"
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

// TestKugouSearch 测试酷狗音乐搜索
func TestKugouSearch(t *testing.T) {
	keyword := "周杰伦"
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
		if song.Artist == "" {
			t.Errorf("Song %d: empty artist", i)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
	}

	t.Logf("Kugou: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestKugouGetDownloadURL 测试酷狗音乐下载链接获取
func TestKugouGetDownloadURL(t *testing.T) {
	keyword := "小苹果"
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
				// 不标记为失败，因为某些歌曲可能受版权保护
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
	keyword := "周杰伦"
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
		if song.Name == "" {
			t.Errorf("Song %d: empty name", i)
		}
		if song.Artist == "" {
			t.Errorf("Song %d: empty artist", i)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
	}

	t.Logf("QQ: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestQQGetDownloadURL 测试QQ音乐下载链接获取
func TestQQGetDownloadURL(t *testing.T) {
	keyword := "小苹果"
	songs, err := qq.Search(keyword)
	if err != nil {
		t.Fatalf("QQ Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing QQ GetDownloadURL")
	}

	// 尝试前3首歌曲
	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := qq.GetDownloadURL(song)
			if err != nil {
				t.Logf("QQ GetDownloadURL failed for %s: %v", song.Name, err)
				// 不标记为失败，因为某些歌曲可能受版权保护
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
	keyword := "周杰伦"
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
		if song.Name == "" {
			t.Errorf("Song %d: empty name", i)
		}
		if song.Artist == "" {
			t.Errorf("Song %d: empty artist", i)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
	}

	t.Logf("Migu: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestMiguGetDownloadURL 测试咪咕音乐下载链接获取
func TestMiguGetDownloadURL(t *testing.T) {
	keyword := "周杰伦"
	songs, err := migu.Search(keyword)
	if err != nil {
		t.Fatalf("Migu Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Migu GetDownloadURL")
	}

	// 尝试前3首歌曲
	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := migu.GetDownloadURL(song)
			if err != nil {
				t.Logf("Migu GetDownloadURL failed for %s: %v", song.Name, err)
				// 不标记为失败，因为某些歌曲可能受版权保护
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
	keyword := "海阔天空"
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
		if song.Name == "" {
			t.Errorf("Song %d: empty name", i)
		}
		if song.Artist == "" {
			t.Errorf("Song %d: empty artist", i)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
	}

	t.Logf("Netease: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestNeteaseGetDownloadURL 测试网易云音乐下载链接获取
func TestNeteaseGetDownloadURL(t *testing.T) {
	keyword := "海阔天空"
	songs, err := netease.Search(keyword)
	if err != nil {
		t.Fatalf("Netease Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Netease GetDownloadURL")
	}

	// 尝试前3首歌曲
	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := netease.GetDownloadURL(song)
			if err != nil {
				t.Logf("Netease GetDownloadURL failed for %s: %v", song.Name, err)
				// 不标记为失败，因为某些歌曲可能受版权保护
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
	keyword := "周杰伦"
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
		if song.Name == "" {
			t.Errorf("Song %d: empty name", i)
		}
		if song.Artist == "" {
			t.Errorf("Song %d: empty artist", i)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
	}

	t.Logf("Kuwo: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestKuwoGetDownloadURL 测试酷我音乐下载链接获取
func TestKuwoGetDownloadURL(t *testing.T) {
	keyword := "周杰伦"
	songs, err := kuwo.Search(keyword)
	if err != nil {
		t.Fatalf("Kuwo Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Kuwo GetDownloadURL")
	}

	// 尝试前3首歌曲
	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := kuwo.GetDownloadURL(song)
			if err != nil {
				t.Logf("Kuwo GetDownloadURL failed for %s: %v", song.Name, err)
				// 不标记为失败，因为某些歌曲可能受版权保护
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
// 注意：Bilibili API 有严格的风控机制，频繁请求可能导致IP被封
// 此测试默认跳过，仅在需要时通过环境变量启用
func TestBilibiliSearch(t *testing.T) {
	if os.Getenv("ENABLE_BILIBILI_TEST") != "1" {
		t.Skip("跳过 Bilibili 测试（避免风控），设置 ENABLE_BILIBILI_TEST=1 启用")
	}
	
	keyword := "周杰伦"
	songs, err := bilibili.Search(keyword)
	if err != nil {
		// Bilibili API 有风控，如果搜索失败就跳过测试
		t.Skipf("Bilibili Search failed (可能触发风控): %v", err)
	}

	if len(songs) == 0 {
		t.Skip("Bilibili Search returned no songs")
	}

	// 只检查前2首歌曲，减少请求量
	for i := 0; i < min(2, len(songs)); i++ {
		song := songs[i]
		if song.Source != "bilibili" {
			t.Errorf("Song %d: expected source 'bilibili', got '%s'", i, song.Source)
		}
		if song.Name == "" {
			t.Errorf("Song %d: empty name", i)
		}
		if song.Artist == "" {
			t.Errorf("Song %d: empty artist", i)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
	}

	t.Logf("Bilibili: Found %d songs for keyword '%s' (检查了前2首)", len(songs), keyword)
}

// TestBilibiliGetDownloadURL 测试Bilibili音频下载链接获取
// 注意：Bilibili API 有严格的风控机制，频繁请求可能导致IP被封
// 此测试默认跳过，仅在需要时通过环境变量启用
func TestBilibiliGetDownloadURL(t *testing.T) {
	if os.Getenv("ENABLE_BILIBILI_TEST") != "1" {
		t.Skip("跳过 Bilibili 测试（避免风控），设置 ENABLE_BILIBILI_TEST=1 启用")
	}
	
	keyword := "周杰伦"
	songs, err := bilibili.Search(keyword)
	if err != nil {
		// Bilibili API 有风控，如果搜索失败就跳过测试
		t.Skipf("Bilibili Search failed (可能触发风控): %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Bilibili GetDownloadURL")
	}

	// 只尝试前1首歌曲，大幅减少请求量
	for i := 0; i < min(1, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := bilibili.GetDownloadURL(song)
			if err != nil {
				t.Logf("Bilibili GetDownloadURL failed for %s: %v", song.Name, err)
				// 不标记为失败，因为可能触发风控
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
	keyword := "周杰伦"
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
		if song.Name == "" {
			t.Errorf("Song %d: empty name", i)
		}
		if song.Artist == "" {
			t.Errorf("Song %d: empty artist", i)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
		// 检查ID格式是否为 songId|typeEname
		if !strings.Contains(song.ID, "|") {
			t.Errorf("Song %d: ID should contain '|' separator, got '%s'", i, song.ID)
		}
	}

	t.Logf("FiveSing: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestFiveSingGetDownloadURL 测试FiveSing音乐下载链接获取
func TestFiveSingGetDownloadURL(t *testing.T) {
	keyword := "周杰伦"
	songs, err := fivesing.Search(keyword)
	if err != nil {
		t.Fatalf("FiveSing Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing FiveSing GetDownloadURL")
	}

	// 尝试前3首歌曲
	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := fivesing.GetDownloadURL(song)
			if err != nil {
				t.Logf("FiveSing GetDownloadURL failed for %s: %v", song.Name, err)
				// 不标记为失败，因为某些歌曲可能受版权保护
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
	keyword := "周杰伦"
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
		if song.Name == "" {
			t.Errorf("Song %d: empty name", i)
		}
		if song.Artist == "" {
			t.Errorf("Song %d: empty artist", i)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
	}

	t.Logf("Qianqian: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestQianqianGetDownloadURL 测试千千音乐下载链接获取
func TestQianqianGetDownloadURL(t *testing.T) {
	keyword := "苹果汽水"
	songs, err := qianqian.Search(keyword)
	if err != nil {
		t.Fatalf("Qianqian Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Qianqian GetDownloadURL")
	}

	// 尝试前3首歌曲
	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := qianqian.GetDownloadURL(song)
			if err != nil {
				t.Logf("Qianqian GetDownloadURL failed for %s: %v", song.Name, err)
				// 不标记为失败，因为某些歌曲可能受版权保护
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
	keyword := "周杰伦"
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
		if song.Name == "" {
			t.Errorf("Song %d: empty name", i)
		}
		if song.Artist == "" {
			t.Errorf("Song %d: empty artist", i)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
	}

	t.Logf("Soda: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestSodaGetDownloadURL 测试汽水音乐下载链接获取
func TestSodaGetDownloadURL(t *testing.T) {
	keyword := "小苹果"
	songs, err := soda.Search(keyword)
	if err != nil {
		t.Fatalf("Soda Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Soda GetDownloadURL")
	}

	// 尝试前3首歌曲
	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := soda.GetDownloadURL(song)
			if err != nil {
				t.Logf("Soda GetDownloadURL failed for %s: %v", song.Name, err)
				// 不标记为失败，因为某些歌曲可能受版权保护
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
	keyword := "electronic"
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
		if song.Name == "" {
			t.Errorf("Song %d: empty name", i)
		}
		if song.Artist == "" {
			t.Errorf("Song %d: empty artist", i)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
		if song.URL == "" {
			t.Errorf("Song %d: empty URL", i)
		}
	}

	t.Logf("Jamendo: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestJamendoGetDownloadURL 测试Jamendo音乐下载链接获取
func TestJamendoGetDownloadURL(t *testing.T) {
	keyword := "rock"
	songs, err := jamendo.Search(keyword)
	if err != nil {
		t.Fatalf("Jamendo Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Jamendo GetDownloadURL")
	}

	// 尝试前3首歌曲
	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := jamendo.GetDownloadURL(song)
			if err != nil {
				t.Logf("Jamendo GetDownloadURL failed for %s: %v", song.Name, err)
				// 不标记为失败，因为某些歌曲可能受版权保护
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
	keyword := "周杰伦"
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
		if song.Name == "" {
			t.Errorf("Song %d: empty name", i)
		}
		if song.Artist == "" {
			t.Errorf("Song %d: empty artist", i)
		}
		if song.ID == "" {
			t.Errorf("Song %d: empty ID", i)
		}
	}

	t.Logf("Joox: Found %d songs for keyword '%s'", len(songs), keyword)
}

// TestJooxGetDownloadURL 测试Joox音乐下载链接获取
func TestJooxGetDownloadURL(t *testing.T) {
	keyword := "小苹果"
	songs, err := joox.Search(keyword)
	if err != nil {
		t.Fatalf("Joox Search failed: %v", err)
	}

	if len(songs) == 0 {
		t.Skip("No songs found for testing Joox GetDownloadURL")
	}

	// 尝试前3首歌曲
	for i := 0; i < min(3, len(songs)); i++ {
		song := &songs[i]
		t.Run(song.Name, func(t *testing.T) {
			url, err := joox.GetDownloadURL(song)
			if err != nil {
				t.Logf("Joox GetDownloadURL failed for %s: %v", song.Name, err)
				// 不标记为失败，因为某些歌曲可能受版权保护
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
	keyword := "周杰伦"
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
		// 注意：bilibili 有风控，跳过
	}

	for _, src := range sources {
		src := src // 捕获循环变量
		t.Run(src.name, func(t *testing.T) {
			t.Parallel()
			songs, err := src.search(keyword)
			if err != nil {
				// 对于 jamendo，关键词可能是英文，但这里用中文可能无结果，跳过
				if src.name == "jamendo" {
					t.Skipf("Jamendo search failed (可能关键词不支持): %v", err)
				}
				// 其他源失败则标记为错误
				t.Errorf("%s Search failed: %v", src.name, err)
				return
			}
			if len(songs) == 0 {
				t.Skipf("%s Search returned no songs (可能API变更)", src.name)
			}
			// 检查第一首歌曲的基本字段
			song := songs[0]
			if song.Source != src.name {
				t.Errorf("%s: expected source '%s', got '%s'", src.name, src.name, song.Source)
			}
			if song.Name == "" {
				t.Errorf("%s: empty name", src.name)
			}
			if song.Artist == "" {
				t.Errorf("%s: empty artist", src.name)
			}
			if song.ID == "" {
				t.Errorf("%s: empty ID", src.name)
			}
			t.Logf("%s: Found %d songs for keyword '%s'", src.name, len(songs), keyword)
		})
	}
}
