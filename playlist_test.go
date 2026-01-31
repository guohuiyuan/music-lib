package main

import (
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qq"
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
		{"qq", "抖音", qq.SearchPlaylist},
		{"fivesing", "抖音", fivesing.SearchPlaylist},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			playlists, err := tt.search(tt.keyword)
			if err != nil {
				t.Fatalf("[%s] SearchPlaylist failed: %v", tt.name, err)
			}
			if len(playlists) == 0 {
				t.Skipf("[%s] returned no results, skipping verification", tt.name)
			}

			first := playlists[0]
			t.Logf("[%s] Found %d playlists. First: [%s] %s (Tracks: %d)",
				tt.name, len(playlists), first.ID, first.Name, first.TrackCount)

			// 验证基本字段
			if first.ID == "" {
				t.Error("Playlist ID is empty")
			}
			if first.Source != tt.name {
				t.Errorf("Source mismatch: expected %s, got %s", tt.name, first.Source)
			}
		})
	}
}

// ==================== 测试2：仅获取歌单歌曲（使用固定ID） ====================

func TestGetPlaylistSongsOnly(t *testing.T) {
	tests := []struct {
		name       string
		playlistID string
		source     string // 预期返回的 Source 字段
		getSongs   func(string) ([]model.Song, error)
	}{
		{"netease", "988690134", "netease", netease.GetPlaylistSongs},
		{"kugou", "3650904", "kugou", kugou.GetPlaylistSongs},
		{"qq", "9262344645", "qq", qq.GetPlaylistSongs}, 
		{"fivesing", "5b163457b0f5ba3ca80628db", "fivesing", fivesing.GetPlaylistSongs},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("[%s] Fetching songs for playlist ID: %s", tc.name, tc.playlistID)

			songs, err := tc.getSongs(tc.playlistID)
			if err != nil {
				// 网络/API错误容错处理
				if isConnectivityError(err) {
					t.Skipf("[%s] Skipped due to connectivity/API error: %v", tc.name, err)
				}
				t.Fatalf("[%s] GetPlaylistSongs failed: %v", tc.name, err)
			}

			if len(songs) == 0 {
				t.Skipf("[%s] returned 0 songs (empty playlist or invalid ID)", tc.name)
			}

			t.Logf("[%s] Successfully retrieved %d songs", tc.name, len(songs))

			// 验证第一首歌的字段
			first := songs[0]
			t.Logf("  -> First Song: %s - %s (ID: %s)", first.Name, first.Artist, first.ID)

			if first.Source != tc.source {
				t.Errorf("Source mismatch: expected %s, got %s", tc.source, first.Source)
			}
			if first.ID == "" {
				t.Error("Song ID is empty")
			}
		})
	}
}

// ==================== 辅助函数 ====================

// isConnectivityError 判断是否为网络或常见的 API 404 错误，避免让测试直接 Fail
func isConnectivityError(err error) bool {
	if err == nil {
		return false
	}

	// 使用 strings.Contains 替代原本复杂的逻辑
	// 忽略大小写判断通常更稳健
	msg := strings.ToLower(err.Error())

	keywords := []string{
		"404",
		"timeout",
		"connection",
		"refused",
		"no such host",
		"network is unreachable",
		"client_loop", // 有些库的 HTTP client 错误
	}

	for _, k := range keywords {
		if strings.Contains(msg, k) {
			return true
		}
	}
	return false
}
