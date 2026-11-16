package kugou

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetRecommendedPlaylistsFallsBackWhenHomepageReturnsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/primary":
			_, _ = w.Write([]byte(`{"plist":{"list":{"info":[]}}}`))
		case "/fallback":
			_, _ = w.Write([]byte(`{"data":{"info":[{"specialid":123,"global_specialid":"collection_3_test","specialname":"测试推荐歌单","imgurl":"https://example.com/{size}.jpg","playcount":42,"username":"测试用户","intro":"测试描述"}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	playlists, err := New("").getRecommendedPlaylists(server.URL+"/primary", server.URL+"/fallback")
	if err != nil {
		t.Fatalf("getRecommendedPlaylists() error = %v", err)
	}
	if len(playlists) != 1 {
		t.Fatalf("getRecommendedPlaylists() returned %d playlists, want 1", len(playlists))
	}

	playlist := playlists[0]
	if playlist.ID != "123" {
		t.Fatalf("playlist ID = %q, want 123", playlist.ID)
	}
	if playlist.Name != "测试推荐歌单" {
		t.Fatalf("playlist Name = %q, want 测试推荐歌单", playlist.Name)
	}
	if playlist.Cover != "https://example.com/240.jpg" {
		t.Fatalf("playlist Cover = %q, want resized cover URL", playlist.Cover)
	}
	if playlist.PlayCount != 42 || playlist.Creator != "测试用户" {
		t.Fatalf("unexpected playlist metadata: %#v", playlist)
	}
}
