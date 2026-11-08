package spotify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestSpotifyIDParsing(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		kind string
		want string
	}{
		{name: "track url", raw: "https://open.spotify.com/track/abc123?si=test", kind: "track", want: "abc123"},
		{name: "international url", raw: "https://open.spotify.com/intl-zh/album/album123", kind: "album", want: "album123"},
		{name: "embed url", raw: "https://open.spotify.com/embed/playlist/playlist123", kind: "playlist", want: "playlist123"},
		{name: "uri", raw: "spotify:track:uri123", kind: "track", want: "uri123"},
		{name: "raw id", raw: "raw123", kind: "album", want: "raw123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractSpotifyID(tt.raw, tt.kind)
			if err != nil {
				t.Fatalf("extractSpotifyID() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("extractSpotifyID() = %q, want %q", got, tt.want)
			}
		})
	}

	if _, err := extractSpotifyID("https://open.spotify.com/track/abc", "album"); err == nil {
		t.Fatal("extractSpotifyID() accepted mismatched resource type")
	}
}

func TestSpotifyConfigParsing(t *testing.T) {
	oldClientID := os.Getenv("SPOTIFY_CLIENT_ID")
	oldClientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	oldRefreshToken := os.Getenv("SPOTIFY_REFRESH_TOKEN")
	oldAccessToken := os.Getenv("SPOTIFY_ACCESS_TOKEN")
	oldMarket := os.Getenv("SPOTIFY_MARKET")
	t.Cleanup(func() {
		restoreEnv("SPOTIFY_CLIENT_ID", oldClientID)
		restoreEnv("SPOTIFY_CLIENT_SECRET", oldClientSecret)
		restoreEnv("SPOTIFY_REFRESH_TOKEN", oldRefreshToken)
		restoreEnv("SPOTIFY_ACCESS_TOKEN", oldAccessToken)
		restoreEnv("SPOTIFY_MARKET", oldMarket)
	})
	t.Setenv("SPOTIFY_CLIENT_ID", "")
	t.Setenv("SPOTIFY_CLIENT_SECRET", "")
	t.Setenv("SPOTIFY_REFRESH_TOKEN", "")
	t.Setenv("SPOTIFY_ACCESS_TOKEN", "")
	t.Setenv("SPOTIFY_MARKET", "")

	client := New(`{"client_id":"json-id","client_secret":"json-secret","refresh_token":"json-refresh","market":"cn"}`)
	if client.clientID != "json-id" || client.clientSecret != "json-secret" ||
		client.refreshToken != "json-refresh" || client.market != "CN" {
		t.Fatalf("unexpected JSON config: %#v", client)
	}

	client = New("client_id=form-id; client_secret=form-secret; market=jp")
	if client.clientID != "form-id" || client.clientSecret != "form-secret" || client.market != "JP" {
		t.Fatalf("unexpected form config: %#v", client)
	}
}

func TestSpotifyCatalogMethods(t *testing.T) {
	server, restore := newSpotifyTestServer(t)
	defer restore()

	client := New(`{"client_id":"client-id","client_secret":"client-secret","market":"US"}`)
	ctx := testSpotifyServerContext{server: server, client: client}

	songs, err := ctx.client.Search("hello")
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(songs) != 1 || songs[0].Source != "spotify" || songs[0].Extra["preview_only"] != "true" {
		t.Fatalf("unexpected search songs: %#v", songs)
	}

	parsed, err := ctx.client.Parse("https://open.spotify.com/intl-zh/track/track1?si=1")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsed.ID != "track1" || parsed.Name != "Hello" || parsed.Artist != "Artist A" {
		t.Fatalf("unexpected parsed track: %#v", parsed)
	}

	downloadURL, err := ctx.client.GetDownloadURL(parsed)
	if err != nil {
		t.Fatalf("GetDownloadURL() error = %v", err)
	}
	if downloadURL != "https://cdn.example/preview.mp3" {
		t.Fatalf("GetDownloadURL() = %q", downloadURL)
	}

	fallbackURL, err := ctx.client.GetDownloadURL(&model.Song{ID: "no-preview"})
	if err != nil {
		t.Fatalf("GetDownloadURL() public preview fallback error = %v", err)
	}
	if fallbackURL != "https://cdn.example/public-track.mp3" {
		t.Fatalf("GetDownloadURL() public preview fallback = %q", fallbackURL)
	}

	lyrics, err := ctx.client.GetLyrics(parsed)
	if err != nil {
		t.Fatalf("GetLyrics() exact error = %v", err)
	}
	if lyrics != "[00:01.00]Hello" {
		t.Fatalf("GetLyrics() = %q", lyrics)
	}

	lyrics, err = ctx.client.GetLyrics(&model.Song{Name: "Fallback", Artist: "Artist B"})
	if err != nil {
		t.Fatalf("GetLyrics() search error = %v", err)
	}
	if lyrics != "fallback lyrics" {
		t.Fatalf("GetLyrics() fallback = %q", lyrics)
	}

	lyrics, err = ctx.client.GetLyrics(&model.Song{Link: "https://open.spotify.com/track/track1"})
	if err != nil {
		t.Fatalf("GetLyrics() link fallback error = %v", err)
	}
	if lyrics != "[00:01.00]Hello" {
		t.Fatalf("GetLyrics() link fallback = %q", lyrics)
	}
}

func TestSpotifyAlbumMethods(t *testing.T) {
	server, restore := newSpotifyTestServer(t)
	defer restore()
	client := New(`{"client_id":"client-id","client_secret":"client-secret","market":"US"}`)

	albums, err := client.SearchAlbum("album")
	if err != nil {
		t.Fatalf("SearchAlbum() error = %v", err)
	}
	if len(albums) != 1 || albums[0].ID != "album1" || albums[0].Extra["type"] != "album" {
		t.Fatalf("unexpected albums: %#v", albums)
	}

	songs, err := client.GetAlbumSongs("https://open.spotify.com/album/album1")
	if err != nil {
		t.Fatalf("GetAlbumSongs() error = %v", err)
	}
	if len(songs) != 2 || songs[0].Album != "Test Album" || songs[1].AlbumID != "album1" {
		t.Fatalf("unexpected album songs: %#v", songs)
	}

	playlist, parsedSongs, err := client.ParseAlbum("spotify:album:album1")
	if err != nil {
		t.Fatalf("ParseAlbum() error = %v", err)
	}
	if playlist.ID != "album1" || len(parsedSongs) != 2 {
		t.Fatalf("unexpected parsed album: %#v, songs=%d", playlist, len(parsedSongs))
	}
	_ = server
}

func TestSpotifyPlaylistMethods(t *testing.T) {
	server, restore := newSpotifyTestServer(t)
	defer restore()
	client := New(`{"client_id":"client-id","client_secret":"client-secret","market":"US"}`)

	playlists, err := client.SearchPlaylist("playlist")
	if err != nil {
		t.Fatalf("SearchPlaylist() error = %v", err)
	}
	if len(playlists) != 1 || playlists[0].ID != "playlist1" || playlists[0].TrackCount != 3 {
		t.Fatalf("unexpected playlists: %#v", playlists)
	}

	songs, err := client.GetPlaylistSongs("playlist1")
	if err != nil {
		t.Fatalf("GetPlaylistSongs() error = %v", err)
	}
	if len(songs) != 3 || songs[2].ID != "track3" {
		t.Fatalf("unexpected playlist songs: %#v", songs)
	}

	playlist, parsedSongs, err := client.ParsePlaylist("https://open.spotify.com/intl-en/playlist/playlist1")
	if err != nil {
		t.Fatalf("ParsePlaylist() error = %v", err)
	}
	if playlist.ID != "playlist1" || playlist.Name != "Test Playlist" || len(parsedSongs) != 3 {
		t.Fatalf("unexpected parsed playlist: %#v, songs=%d", playlist, len(parsedSongs))
	}
	_ = server
}

func TestSpotifyPlaylistTracksLegacyFallback(t *testing.T) {
	server, restore := newSpotifyTestServer(t)
	defer restore()
	client := New(`{"client_id":"client-id","client_secret":"client-secret","market":"US"}`)

	songs, err := client.GetPlaylistSongs("legacy-playlist")
	if err != nil {
		t.Fatalf("GetPlaylistSongs() legacy fallback error = %v", err)
	}
	if len(songs) != 1 || songs[0].ID != "legacy-track" {
		t.Fatalf("unexpected legacy playlist songs: %#v", songs)
	}
	_ = server
}

func TestSpotifyPublicPlaylistEmbedFallbackWithoutCredentials(t *testing.T) {
	clearSpotifyEnvironment(t)
	server, restore := newSpotifyTestServer(t)
	defer restore()
	client := New("")

	songs, err := client.GetPlaylistSongs("embed-playlist")
	if err != nil {
		t.Fatalf("GetPlaylistSongs() embed fallback error = %v", err)
	}
	if len(songs) != 2 ||
		songs[0].ID != "embed-track-1" ||
		songs[0].Name != "Embed One" ||
		songs[0].Artist != "Embed Artist One" ||
		songs[0].URL != "https://cdn.example/embed-one.mp3" ||
		songs[0].Extra["metadata_source"] != "embed" {
		t.Fatalf("unexpected embed fallback songs: %#v", songs)
	}

	playlist, parsedSongs, err := client.ParsePlaylist(
		"https://open.spotify.com/playlist/embed-playlist",
	)
	if err != nil {
		t.Fatalf("ParsePlaylist() embed fallback error = %v", err)
	}
	if playlist.Name != "Embed Playlist" ||
		playlist.TrackCount != 2 ||
		len(parsedSongs) != 2 {
		t.Fatalf("unexpected embed playlist: %#v, songs=%d", playlist, len(parsedSongs))
	}

	topSongs, err := client.GetTopListSongs("global-top-50")
	if err != nil {
		t.Fatalf("GetTopListSongs() embed fallback error = %v", err)
	}
	if len(topSongs) != 2 || topSongs[0].Extra["rank"] != "1" {
		t.Fatalf("unexpected embed top songs: %#v", topSongs)
	}
	switch topSongs[0].Extra["metadata_source"] {
	case "embed", "kworb":
	default:
		t.Fatalf(
			"unexpected top songs metadata source: %#v",
			topSongs[0].Extra,
		)
	}
	_ = server
}

func TestSpotifyPublicPlaylistEmbedUsesAlternativeHost(t *testing.T) {
	clearSpotifyEnvironment(t)
	server, restore := newSpotifyTestServer(t)
	defer restore()

	spotifyOpenBase = server.URL + "/unavailable"
	spotifyEmbedBase = server.URL

	songs, err := New("").GetPlaylistSongs("embed-playlist")
	if err != nil {
		t.Fatalf("GetPlaylistSongs() alternative embed error = %v", err)
	}
	if len(songs) != 2 || songs[0].ID != "embed-track-1" {
		t.Fatalf("unexpected alternative embed songs: %#v", songs)
	}
}

func TestSpotifyPublicTrackEmbedWithoutCredentials(t *testing.T) {
	clearSpotifyEnvironment(t)
	_, restore := newSpotifyTestServer(t)
	defer restore()
	client := New("")

	song, err := client.Parse(
		"https://open.spotify.com/track/public-track?si=test",
	)
	if err != nil {
		t.Fatalf("Parse() public track fallback error = %v", err)
	}
	if song.ID != "public-track" ||
		song.Name != "Public Track" ||
		song.Artist != "Public Artist" ||
		song.URL != "https://cdn.example/public-track.mp3" ||
		song.Extra["metadata_source"] != "embed" {
		t.Fatalf("unexpected public track: %#v", song)
	}

	downloadURL, err := client.GetDownloadURL(&model.Song{
		ID: "public-track",
	})
	if err != nil {
		t.Fatalf("GetDownloadURL() public track fallback error = %v", err)
	}
	if downloadURL != "https://cdn.example/public-track.mp3" {
		t.Fatalf("GetDownloadURL() public track = %q", downloadURL)
	}
}

func TestSpotifyPublicTrackEmbedUsesAlternativeHost(t *testing.T) {
	clearSpotifyEnvironment(t)
	server, restore := newSpotifyTestServer(t)
	defer restore()

	spotifyOpenBase = server.URL + "/unavailable"
	spotifyEmbedBase = server.URL

	downloadURL, err := New("").GetDownloadURL(&model.Song{
		ID: "public-track",
	})
	if err != nil {
		t.Fatalf("GetDownloadURL() alternative host error = %v", err)
	}
	if downloadURL != "https://cdn.example/public-track.mp3" {
		t.Fatalf("GetDownloadURL() alternative host = %q", downloadURL)
	}
}

func TestSpotifyPublicTrackReportsRegionUnavailable(t *testing.T) {
	clearSpotifyEnvironment(t)
	_, restore := newSpotifyTestServer(t)
	defer restore()

	_, err := New("").GetDownloadURL(&model.Song{
		ID: "blocked-track",
	})
	if err == nil || !strings.Contains(err.Error(), "track unavailable") {
		t.Fatalf("GetDownloadURL() region error = %v", err)
	}
}

func TestSpotifyPublicTopListUsesChartFallbackWhenSpotifyUnavailable(t *testing.T) {
	clearSpotifyEnvironment(t)
	server, restore := newSpotifyTestServer(t)
	defer restore()

	spotifyOpenBase = server.URL + "/unavailable"
	spotifyEmbedBase = server.URL + "/unavailable"

	songs, err := New("").GetTopListSongs("global-top-50")
	if err != nil {
		t.Fatalf("GetTopListSongs() chart fallback error = %v", err)
	}
	if len(songs) != 2 ||
		songs[0].ID != "charttrack1" ||
		songs[0].Name != "Housewife" ||
		songs[0].Artist != "Tate McRae, Featured & Artist" {
		t.Fatalf("unexpected chart fallback songs: %#v", songs)
	}
}

func TestSpotifyBrowseAndRankingMethods(t *testing.T) {
	server, restore := newSpotifyTestServer(t)
	defer restore()
	client := New(`{"client_id":"client-id","client_secret":"client-secret","market":"US"}`)

	categories, err := client.GetPlaylistCategories()
	if err != nil {
		t.Fatalf("GetPlaylistCategories() error = %v", err)
	}
	if len(categories) != 1 || categories[0].ID != "pop" || categories[0].Source != "spotify" {
		t.Fatalf("unexpected categories: %#v", categories)
	}

	categoryPlaylists, err := client.GetCategoryPlaylists("pop", 1, 10)
	if err != nil {
		t.Fatalf("GetCategoryPlaylists() error = %v", err)
	}
	if len(categoryPlaylists) != 1 || categoryPlaylists[0].ID != "category-playlist" {
		t.Fatalf("unexpected category playlists: %#v", categoryPlaylists)
	}

	recommended, err := client.GetRecommendedPlaylists()
	if err != nil {
		t.Fatalf("GetRecommendedPlaylists() error = %v", err)
	}
	if len(recommended) != 1 || recommended[0].ID != "featured-playlist" {
		t.Fatalf("unexpected recommended playlists: %#v", recommended)
	}

	topLists, err := client.GetTopLists()
	if err != nil {
		t.Fatalf("GetTopLists() error = %v", err)
	}
	if len(topLists) != len(spotifyTopListDefinitions) || topLists[0].Source != "spotify" {
		t.Fatalf("unexpected top lists: %#v", topLists)
	}

	topSongs, err := client.GetTopListSongs("global-top-50")
	if err != nil {
		t.Fatalf("GetTopListSongs() error = %v", err)
	}
	if len(topSongs) != 3 || topSongs[0].Extra["rank"] != "1" {
		t.Fatalf("unexpected top songs: %#v", topSongs)
	}

	hotSongs, err := client.GetHotSongs(2)
	if err != nil {
		t.Fatalf("GetHotSongs() error = %v", err)
	}
	if len(hotSongs) != 2 {
		t.Fatalf("GetHotSongs() length = %d, want 2", len(hotSongs))
	}
	_ = server
}

func TestSpotifyUserPlaylistsUseUserToken(t *testing.T) {
	server, restore := newSpotifyTestServer(t)
	defer restore()
	client := New(`{"access_token":"user-token"}`)

	playlists, err := client.GetUserPlaylists(2, 2)
	if err != nil {
		t.Fatalf("GetUserPlaylists() error = %v", err)
	}
	if len(playlists) != 2 || playlists[0].ID != "user-playlist-3" || playlists[1].ID != "user-playlist-4" {
		t.Fatalf("unexpected user playlists: %#v", playlists)
	}
	_ = server
}

func TestSpotifyMissingCredentials(t *testing.T) {
	client := &Spotify{market: "US"}
	_, err := client.Search("hello")
	if err == nil || !strings.Contains(err.Error(), "client_id and client_secret") {
		t.Fatalf("Search() credentials error = %v", err)
	}
}

func TestSpotifyTopListDefinitionsAreUnique(t *testing.T) {
	if len(spotifyTopListDefinitions) < 6 {
		t.Fatalf("top list definitions = %d, want at least 6", len(spotifyTopListDefinitions))
	}

	ids := make(map[string]struct{}, len(spotifyTopListDefinitions))
	playlistIDs := make(map[string]struct{}, len(spotifyTopListDefinitions))
	for _, definition := range spotifyTopListDefinitions {
		if _, exists := ids[definition.ID]; exists {
			t.Fatalf("duplicate top list id %q", definition.ID)
		}
		if _, exists := playlistIDs[definition.PlaylistID]; exists {
			t.Fatalf("duplicate playlist id %q", definition.PlaylistID)
		}
		if strings.TrimSpace(definition.ChartCountry) == "" {
			t.Fatalf("top list %q is missing a public chart country", definition.ID)
		}
		ids[definition.ID] = struct{}{}
		playlistIDs[definition.PlaylistID] = struct{}{}
		if definition.ID == "viral-global" || definition.PlaylistID == "37i9dQZEVXbLiRSasKsNU9" {
			t.Fatal("retired Viral 50 global playlist must not be exposed")
		}
		if !strings.HasPrefix(definition.PlaylistID, "37i9dQ") || len(definition.PlaylistID) != 22 {
			t.Fatalf("unexpected Spotify editorial playlist id %q", definition.PlaylistID)
		}
	}
}

func TestSpotifyRetriesExpiredTokenAndRateLimit(t *testing.T) {
	oldAPIBase := spotifyAPIBase
	oldAccountsBase := spotifyAccountsBase
	t.Cleanup(func() {
		spotifyAPIBase = oldAPIBase
		spotifyAccountsBase = oldAccountsBase
	})

	var tokenRequests int32
	var apiRequests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token":
			atomic.AddInt32(&tokenRequests, 1)
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "client_credentials" {
				writeSpotifyTestJSON(w, http.StatusBadRequest, map[string]any{"error": "bad grant"})
				return
			}
			writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
				"access_token": "fresh-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			})
		case "/v1/tracks/retry":
			requestNumber := atomic.AddInt32(&apiRequests, 1)
			switch {
			case r.Header.Get("Authorization") == "Bearer expired":
				writeSpotifyTestJSON(w, http.StatusUnauthorized, map[string]any{
					"error": map[string]any{"message": "expired"},
				})
			case requestNumber == 2:
				w.Header().Set("Retry-After", "0")
				writeSpotifyTestJSON(w, http.StatusTooManyRequests, map[string]any{
					"error": map[string]any{"message": "rate limited"},
				})
			default:
				writeSpotifyTestJSON(w, http.StatusOK, testSpotifyTrack("retry", "Retry"))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	spotifyAPIBase = server.URL + "/v1"
	spotifyAccountsBase = server.URL
	client := New(`{"client_id":"client-id","client_secret":"client-secret","access_token":"expired"}`)
	track, err := client.getTrack("retry")
	if err != nil {
		t.Fatalf("getTrack() retry error = %v", err)
	}
	if track.ID != "retry" {
		t.Fatalf("getTrack() id = %q, want retry", track.ID)
	}
	if got := atomic.LoadInt32(&tokenRequests); got != 1 {
		t.Fatalf("token requests = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&apiRequests); got != 3 {
		t.Fatalf("api requests = %d, want 3", got)
	}
}

func TestSpotifyRefreshTokenIssuesUserToken(t *testing.T) {
	oldAPIBase := spotifyAPIBase
	oldAccountsBase := spotifyAccountsBase
	t.Cleanup(func() {
		spotifyAPIBase = oldAPIBase
		spotifyAccountsBase = oldAccountsBase
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/token":
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "refresh_token" ||
				r.Form.Get("refresh_token") != "refresh-token" ||
				r.Form.Get("client_id") != "client-id" {
				writeSpotifyTestJSON(w, http.StatusBadRequest, map[string]any{"error": "bad refresh request"})
				return
			}
			writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
				"access_token":  "fresh-user-token",
				"refresh_token": "rotated-refresh-token",
				"token_type":    "Bearer",
				"expires_in":    3600,
			})
		case "/v1/me/playlists":
			if r.Header.Get("Authorization") != "Bearer fresh-user-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
				"items": []any{testSpotifyPlaylist("personal", "Personal", 2)},
				"total": 1,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	spotifyAPIBase = server.URL + "/v1"
	spotifyAccountsBase = server.URL
	client := New(`{"client_id":"client-id","refresh_token":"refresh-token"}`)
	playlists, err := client.GetUserPlaylists(1, 1)
	if err != nil {
		t.Fatalf("GetUserPlaylists() error = %v", err)
	}
	if len(playlists) != 1 || playlists[0].ID != "personal" {
		t.Fatalf("unexpected user playlists: %#v", playlists)
	}
	if client.accessToken != "fresh-user-token" || client.refreshToken != "rotated-refresh-token" {
		t.Fatalf("unexpected refreshed token state: access=%q refresh=%q", client.accessToken, client.refreshToken)
	}
}

type testSpotifyServerContext struct {
	server *httptest.Server
	client *Spotify
}

func newSpotifyTestServer(t *testing.T) (*httptest.Server, func()) {
	t.Helper()

	oldAPIBase := spotifyAPIBase
	oldAccountsBase := spotifyAccountsBase
	oldLRCLIBBase := lrclibAPIBase
	oldOpenBase := spotifyOpenBase
	oldEmbedBase := spotifyEmbedBase
	oldKworbBase := spotifyKworbBase

	server := httptest.NewServer(http.HandlerFunc(handleSpotifyTestRequest))
	spotifyAPIBase = server.URL + "/v1"
	spotifyAccountsBase = server.URL
	spotifyOpenBase = server.URL
	spotifyEmbedBase = server.URL
	spotifyKworbBase = server.URL + "/kworb"
	lrclibAPIBase = server.URL + "/lrclib"

	return server, func() {
		server.Close()
		spotifyAPIBase = oldAPIBase
		spotifyAccountsBase = oldAccountsBase
		spotifyOpenBase = oldOpenBase
		spotifyEmbedBase = oldEmbedBase
		spotifyKworbBase = oldKworbBase
		lrclibAPIBase = oldLRCLIBBase
	}
}

func handleSpotifyTestRequest(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/embed/playlist/") ||
		(r.URL.Path == "/" &&
			strings.HasPrefix(r.URL.Query().Get("uri"), "spotify:playlist:")) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(spotifyEmbedTestPage()))
		return
	}
	if strings.HasPrefix(r.URL.Path, "/embed/track/") ||
		(r.URL.Path == "/" &&
			strings.HasPrefix(r.URL.Query().Get("uri"), "spotify:track:")) {
		id := strings.TrimPrefix(r.URL.Path, "/embed/track/")
		if id == r.URL.Path {
			id = strings.TrimPrefix(r.URL.Query().Get("uri"), "spotify:track:")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if id == "blocked-track" {
			_, _ = w.Write([]byte(spotifyRegionBlockedTestPage()))
		} else {
			_, _ = w.Write([]byte(spotifyTrackEmbedTestPage(id)))
		}
		return
	}
	if r.URL.Path == "/oembed" {
		writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
			"title":         "Embed Playlist",
			"thumbnail_url": "https://cdn.example/embed-playlist.jpg",
		})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/kworb/spotify/country/") &&
		strings.HasSuffix(r.URL.Path, "_daily.html") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(spotifyKworbTestPage()))
		return
	}
	if r.URL.Path == "/api/token" {
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "client_credentials" {
			writeSpotifyTestJSON(w, http.StatusBadRequest, map[string]any{"error": "bad grant"})
			return
		}
		writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
			"access_token": "test-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
		return
	}

	if strings.HasPrefix(r.URL.Path, "/lrclib/") {
		handleSpotifyLyricsTestRequest(w, r)
		return
	}

	if r.Header.Get("Authorization") == "" {
		writeSpotifyTestJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{"message": "missing token"}})
		return
	}

	query := r.URL.Query()
	switch r.URL.Path {
	case "/v1/search":
		switch query.Get("type") {
		case "track":
			writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
				"tracks": map[string]any{"items": []any{testSpotifyTrack("track1", "Hello")}, "total": 1},
			})
		case "album":
			writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
				"albums": map[string]any{"items": []any{testSpotifyAlbum()}, "total": 1},
			})
		case "playlist":
			writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
				"playlists": map[string]any{"items": []any{testSpotifyPlaylist("playlist1", "Test Playlist", 3)}, "total": 1},
			})
		default:
			writeSpotifyTestJSON(w, http.StatusBadRequest, map[string]any{"error": "bad type"})
		}
	case "/v1/tracks/track1":
		writeSpotifyTestJSON(w, http.StatusOK, testSpotifyTrack("track1", "Hello"))
	case "/v1/tracks/no-preview":
		track := testSpotifyTrack("no-preview", "No Preview")
		track["preview_url"] = nil
		writeSpotifyTestJSON(w, http.StatusOK, track)
	case "/v1/albums/album1":
		writeSpotifyTestJSON(w, http.StatusOK, testSpotifyAlbum())
	case "/v1/albums/album1/tracks":
		if query.Get("offset") != "" && query.Get("offset") != "0" {
			writeSpotifyTestJSON(w, http.StatusOK, map[string]any{"items": []any{}, "total": 2})
			return
		}
		writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
			"items": []any{
				testSimpleTrack("album-track-1", "Album Track 1"),
				testSimpleTrack("album-track-2", "Album Track 2"),
			},
			"total": 2,
			"next":  "next-page",
		})
	case "/v1/playlists/playlist1", "/v1/playlists/37i9dQZEVXbMDoHDwVN2tF":
		id := "playlist1"
		name := "Test Playlist"
		if strings.Contains(r.URL.Path, "37i9dQZEVXbMDoHDwVN2tF") {
			id = "37i9dQZEVXbMDoHDwVN2tF"
			name = "Global Top 50"
		}
		writeSpotifyTestJSON(w, http.StatusOK, testSpotifyPlaylist(id, name, 3))
	case "/v1/playlists/playlist1/items", "/v1/playlists/37i9dQZEVXbMDoHDwVN2tF/items":
		if query.Get("limit") != "50" {
			writeSpotifyTestJSON(w, http.StatusBadRequest, map[string]any{"error": "playlist limit must be 50"})
			return
		}
		if query.Get("additional_types") != "track" {
			writeSpotifyTestJSON(w, http.StatusBadRequest, map[string]any{"error": "playlist must request tracks only"})
			return
		}
		offset := query.Get("offset")
		if offset == "2" {
			writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
				"items": []any{
					map[string]any{"item": testSpotifyTrack("track3", "Track 3")},
				},
				"total": 3,
			})
			return
		}
		writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
			"items": []any{
				map[string]any{"item": testSpotifyTrack("track1", "Hello")},
				map[string]any{"item": testSpotifyTrack("track2", "Track 2")},
			},
			"total": 3,
			"next":  "next-page",
		})
	case "/v1/playlists/legacy-playlist/items":
		writeSpotifyTestJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]any{"message": "playlist items endpoint unavailable"},
		})
	case "/v1/playlists/legacy-playlist/tracks":
		if query.Get("limit") != "50" {
			writeSpotifyTestJSON(w, http.StatusBadRequest, map[string]any{"error": "playlist limit must be 50"})
			return
		}
		if query.Get("additional_types") != "track" {
			writeSpotifyTestJSON(w, http.StatusBadRequest, map[string]any{"error": "playlist must request tracks only"})
			return
		}
		writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
			"items": []any{
				map[string]any{"track": testSpotifyTrack("legacy-track", "Legacy Track")},
			},
			"total": 1,
		})
	case "/v1/browse/categories":
		writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
			"categories": map[string]any{
				"items": []any{
					map[string]any{"id": "pop", "name": "Pop", "icons": []any{map[string]any{"url": "https://cdn.example/pop.jpg", "width": 300}}},
				},
			},
		})
	case "/v1/browse/categories/pop/playlists":
		writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
			"playlists": map[string]any{"items": []any{testSpotifyPlaylist("category-playlist", "Category Playlist", 10)}},
		})
	case "/v1/browse/featured-playlists":
		writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
			"message":   "Featured",
			"playlists": map[string]any{"items": []any{testSpotifyPlaylist("featured-playlist", "Featured Playlist", 10)}},
		})
	case "/v1/me/playlists":
		if r.Header.Get("Authorization") != "Bearer user-token" {
			writeSpotifyTestJSON(w, http.StatusForbidden, map[string]any{"error": map[string]any{"message": "user token required"}})
			return
		}
		if query.Get("offset") == "2" {
			writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
				"items": []any{
					testSpotifyPlaylist("user-playlist-3", "User Playlist 3", 1),
					testSpotifyPlaylist("user-playlist-4", "User Playlist 4", 1),
				},
				"total": 4,
			})
			return
		}
		writeSpotifyTestJSON(w, http.StatusBadRequest, map[string]any{"error": "unexpected offset"})
	default:
		writeSpotifyTestJSON(w, http.StatusNotFound, map[string]any{
			"error": map[string]any{"message": "unexpected path: " + r.URL.Path},
		})
	}
}

func spotifyEmbedTestPage() string {
	payload := map[string]any{
		"props": map[string]any{
			"pageProps": map[string]any{
				"state": map[string]any{
					"data": map[string]any{
						"entity": map[string]any{
							"type":     "playlist",
							"id":       "embed-playlist",
							"name":     "Embed Playlist",
							"subtitle": "Spotify",
							"coverArt": map[string]any{
								"sources": []any{
									map[string]any{
										"url":    "https://cdn.example/embed-cover.jpg",
										"width":  640,
										"height": 640,
									},
								},
							},
							"trackList": []any{
								map[string]any{
									"uri":        "spotify:track:embed-track-1",
									"title":      "Embed One",
									"subtitle":   "Embed Artist One",
									"duration":   201000,
									"isPlayable": true,
									"audioPreview": map[string]any{
										"url": "https://cdn.example/embed-one.mp3",
									},
								},
								map[string]any{
									"uri":        "spotify:track:embed-track-2",
									"title":      "Embed Two",
									"subtitle":   "Embed Artist Two",
									"duration":   199000,
									"isPlayable": true,
									"audioPreview": map[string]any{
										"url": "https://cdn.example/embed-two.mp3",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	encoded, _ := json.Marshal(payload)
	return `<!doctype html><html><body><script id="__NEXT_DATA__" type="application/json">` +
		string(encoded) +
		`</script></body></html>`
}

func spotifyTrackEmbedTestPage(id string) string {
	if strings.TrimSpace(id) == "" {
		id = "public-track"
	}
	payload := map[string]any{
		"props": map[string]any{
			"pageProps": map[string]any{
				"state": map[string]any{
					"data": map[string]any{
						"entity": map[string]any{
							"type":     "track",
							"id":       id,
							"uri":      "spotify:track:" + id,
							"name":     "Public Track",
							"title":    "Public Track",
							"subtitle": "Public Artist",
							"duration": 208000,
							"artists": []any{
								map[string]any{"name": "Public Artist"},
							},
							"audioPreview": map[string]any{
								"url": "https://cdn.example/public-track.mp3",
							},
							"visualIdentity": map[string]any{
								"image": []any{
									map[string]any{
										"url":    "https://cdn.example/public-track.jpg",
										"width":  640,
										"height": 640,
									},
								},
							},
						},
					},
				},
			},
		},
	}
	encoded, _ := json.Marshal(payload)
	return `<!doctype html><html><body><script id="__NEXT_DATA__" type="application/json">` +
		string(encoded) +
		`</script></body></html>`
}

func spotifyRegionBlockedTestPage() string {
	payload := map[string]any{
		"props": map[string]any{
			"pageProps": map[string]any{
				"status":      451,
				"title":       "Sorry, that’s not currently available.",
				"description": "This content is not available in your country.",
			},
		},
	}
	encoded, _ := json.Marshal(payload)
	return `<!doctype html><html><body><script id="__NEXT_DATA__" type="application/json">` +
		string(encoded) +
		`</script></body></html>`
}

func spotifyKworbTestPage() string {
	return `<html><body>
<span class="pagetitle">Spotify Daily Chart - Global - 2026/11/06</span>
<table class="sortable" id="spotifydaily"><tbody>
<tr><td class="np">1</td>
<td class="np">=</td>
<td class="text mp"><div><a href="../artist/artist1.html">Tate McRae</a> -
<a href="../track/charttrack1.html">Housewife</a> (w/
<a href="../artist/artist2.html">Featured &amp; Artist</a>)</div></td>
<td>22</td></tr>
<tr><td class="np">2</td>
<td class="np">+1</td>
<td class="text mp"><div><a href="../artist/artist3.html">Chart Artist</a> -
<a href="../track/charttrack2.html">Second Song</a></div></td>
<td>10</td></tr>
</tbody></table>
</body></html>`
}

func clearSpotifyEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"SPOTIFY_CLIENT_ID",
		"SPOTIFY_CLIENT_SECRET",
		"SPOTIFY_REFRESH_TOKEN",
		"SPOTIFY_ACCESS_TOKEN",
		"SPOTIFY_MARKET",
	} {
		t.Setenv(name, "")
	}
}

func handleSpotifyLyricsTestRequest(w http.ResponseWriter, r *http.Request) {
	query, _ := url.ParseQuery(r.URL.RawQuery)
	switch r.URL.Path {
	case "/lrclib/get":
		if query.Get("track_name") == "Hello" && query.Get("artist_name") == "Artist A" {
			writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
				"syncedLyrics": "[00:01.00]Hello",
				"plainLyrics":  "Hello",
			})
			return
		}
		http.NotFound(w, r)
	case "/lrclib/search":
		writeSpotifyTestJSON(w, http.StatusOK, []any{
			map[string]any{"syncedLyrics": nil, "plainLyrics": "fallback lyrics"},
		})
	default:
		http.NotFound(w, r)
	}
}

func testSpotifyTrack(id, name string) map[string]any {
	return map[string]any{
		"id":            id,
		"name":          name,
		"duration_ms":   180000,
		"track_number":  1,
		"disc_number":   1,
		"preview_url":   "https://cdn.example/preview.mp3",
		"external_urls": map[string]any{"spotify": "https://open.spotify.com/track/" + id},
		"artists":       []any{map[string]any{"id": "artist1", "name": "Artist A"}},
		"album":         testSpotifyAlbum(),
		"external_ids":  map[string]any{"isrc": "USABC1234567"},
	}
}

func testSimpleTrack(id, name string) map[string]any {
	track := testSpotifyTrack(id, name)
	delete(track, "album")
	return track
}

func testSpotifyAlbum() map[string]any {
	return map[string]any{
		"id":            "album1",
		"name":          "Test Album",
		"album_type":    "album",
		"total_tracks":  2,
		"release_date":  "2026-01-01",
		"label":         "Test Label",
		"external_urls": map[string]any{"spotify": "https://open.spotify.com/album/album1"},
		"artists":       []any{map[string]any{"id": "artist1", "name": "Artist A"}},
		"images":        []any{map[string]any{"url": "https://cdn.example/album.jpg", "width": 640, "height": 640}},
	}
}

func testSpotifyPlaylist(id, name string, total int) map[string]any {
	return map[string]any{
		"id":            id,
		"name":          name,
		"description":   "Test description",
		"snapshot_id":   "snapshot",
		"external_urls": map[string]any{"spotify": "https://open.spotify.com/playlist/" + id},
		"owner":         map[string]any{"id": "owner1", "display_name": "Owner"},
		"images":        []any{map[string]any{"url": "https://cdn.example/playlist.jpg", "width": 640}},
		"items":         map[string]any{"total": total},
	}
}

func writeSpotifyTestJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func restoreEnv(key, value string) {
	if value == "" {
		_ = os.Unsetenv(key)
		return
	}
	_ = os.Setenv(key, value)
}

func TestSpotifyTestServerUsesExpectedPaths(t *testing.T) {
	server, restore := newSpotifyTestServer(t)
	defer restore()
	response, err := http.Get(server.URL + "/v1/search")
	if err != nil {
		t.Fatalf("http.Get() error = %v", err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Fatalf("response.Body.Close() error = %v", err)
		}
	}()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("response status = %d, want %d", response.StatusCode, http.StatusUnauthorized)
	}
}

func ExampleSpotify_Search() {
	fmt.Println("configure spotify client_id and client_secret, then call Search")
	// Output: configure spotify client_id and client_secret, then call Search
}
