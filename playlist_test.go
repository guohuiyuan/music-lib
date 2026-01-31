package main

import (
	"testing"

	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
)

// ==================== 测试1：仅搜索歌单 ====================

func TestSearchPlaylistOnly(t *testing.T) {
	tests := []struct {
		name    string
		keyword string
		search  func(string) ([]model.Playlist, error)
	}{
		{"netease", "经典老歌", netease.SearchPlaylist},
		{"kugou", "车载音乐", kugou.SearchPlaylist},
		// {"qq", "抖音", qq.SearchPlaylist},
		// {"kuwo", "热门", kuwo.SearchPlaylist},
		// {"migu", "热门", migu.SearchPlaylist},
		// {"soda", "抖音热歌", soda.SearchPlaylist},
		// {"qianqian", "经典", qianqian.SearchPlaylist},
		// {"fivesing", "热门", fivesing.SearchPlaylist},
		// {"joox", "Top Hits", joox.SearchPlaylist},
		// {"jamendo", "Rock", jamendo.SearchPlaylist},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			playlists, err := tt.search(tt.keyword)
			if err != nil {
				t.Fatalf("%s SearchPlaylist failed: %v", tt.name, err)
			}
			if len(playlists) == 0 {
				t.Skipf("%s returned no results", tt.name)
			}

			t.Logf("[%s] Found %d playlists, first: [%s] %s (Tracks: %d)",
				tt.name, len(playlists), playlists[0].ID, playlists[0].Name, playlists[0].TrackCount)

			// 验证基本字段
			if playlists[0].ID == "" {
				t.Error("Playlist ID is empty")
			}
			if playlists[0].Source != tt.name {
				t.Errorf("Source mismatch: expected %s, got %s", tt.name, playlists[0].Source)
			}
		})
	}
}

// ==================== 测试2：仅获取歌单歌曲（使用固定ID） ====================

// PlaylistTestCase 定义测试用的固定歌单
type PlaylistTestCase struct {
	name         string
	playlistID   string
	platformName string
	getSongs     func(string) ([]model.Song, error)
}

func TestGetPlaylistSongsOnly(t *testing.T) {
	// 预置一些稳定的公开歌单ID用于测试
	// 这些ID应该是长期有效的公开歌单
	cases := []PlaylistTestCase{
		// 国内平台 - 使用已知稳定的歌单ID
		{"netease", "988690134", "netease", netease.GetPlaylistSongs}, // 经典老歌
		{"kugou", "3650904", "kugou", kugou.GetPlaylistSongs},         // 车载音乐
		// {"qq", "9262344645", "qq", qq.GetPlaylistSongs},                   // 需要替换为实际有效的QQ歌单ID
		// {"kuwo", "3572807695", "kuwo", kuwo.GetPlaylistSongs},             // 热门
		// {"migu", "231549589", "migu", migu.GetPlaylistSongs},              // 热门收割
		// {"soda", "7200303561195061287", "soda", soda.GetPlaylistSongs},    // 抖音热歌（如果失效可跳过）
		// {"qianqian", "123456", "qianqian", qianqian.GetPlaylistSongs},     // 需要替换
		// {"fivesing", "5cbb13508788ad1d70a70900", "fivesing", fivesing.GetPlaylistSongs},

		// // 海外平台
		// {"joox", "123456", "joox", joox.GetPlaylistSongs},                 // 需要替换
		// {"jamendo", "123456", "jamendo", jamendo.GetPlaylistSongs},        // 需要替换
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("[%s] Fetching songs for playlist ID: %s", tc.name, tc.playlistID)

			songs, err := tc.getSongs(tc.playlistID)
			if err != nil {
				// 如果是404或网络错误，标记为跳过而非失败（可能是ID失效）
				if isConnectivityError(err) {
					t.Skipf("%s GetPlaylistSongs skipped (network/404): %v", tc.name, err)
				}
				t.Fatalf("%s GetPlaylistSongs failed: %v", tc.name, err)
			}

			if len(songs) == 0 {
				t.Skipf("%s returned 0 songs (empty playlist or invalid ID)", tc.name)
			}

			t.Logf("[%s] Successfully retrieved %d songs", tc.name, len(songs))

			// 验证第一首歌的字段
			first := songs[0]
			t.Logf("  First: %s - %s (ID: %s)", first.Name, first.Artist, first.ID)

			if first.Source != tc.platformName {
				t.Errorf("Source mismatch: expected %s, got %s", tc.platformName, first.Source)
			}
			if first.ID == "" {
				t.Error("Song ID is empty")
			}
		})
	}
}

// ==================== 测试3：从链接解析歌单（如果需要） ====================

// 如果需要测试 ParsePlaylist 功能（如果实现了的话）
func TestParsePlaylistLink(t *testing.T) {
	// 示例：测试从URL解析歌单ID
	// 这需要各平台实现 ParsePlaylist 函数
	t.Skip("ParsePlaylist not implemented yet")
}

// ==================== 辅助函数 ====================

func isConnectivityError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return containsAny(errStr, []string{"404", "timeout", "connection", "refused", "no such host"})
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) > len(substr) &&
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
