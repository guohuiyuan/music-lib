package spotify

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSpotifyAnonymousWebPlayerSearchWithoutCredentials(t *testing.T) {
	clearSpotifyEnvironment(t)
	server, restore, counters := newSpotifyWebPlayerTestServer(t)
	defer restore()
	_ = server

	songs, err := New("").Search("hello")
	if err != nil {
		t.Fatalf("Search() anonymous error = %v", err)
	}
	if len(songs) != 1 ||
		songs[0].ID != "web-track-1" ||
		songs[0].Name != "Web Hello" ||
		songs[0].Artist != "Web Artist, Featured Artist" ||
		songs[0].Album != "Web Album" ||
		songs[0].AlbumID != "web-album-1" ||
		songs[0].Duration != 181 ||
		songs[0].URL != "https://cdn.example/web-preview.mp3" ||
		songs[0].Extra["metadata_source"] != "web_player" ||
		songs[0].Extra["auth_source"] != "anonymous_web_player" {
		t.Fatalf("unexpected anonymous search songs: %#v", songs)
	}

	albums, err := New("").SearchAlbum("hello")
	if err != nil {
		t.Fatalf("SearchAlbum() anonymous error = %v", err)
	}
	if len(albums) != 1 ||
		albums[0].ID != "web-album-1" ||
		albums[0].Name != "Web Album" ||
		albums[0].Creator != "Web Artist" ||
		albums[0].TrackCount != 12 {
		t.Fatalf("unexpected anonymous albums: %#v", albums)
	}

	albumDetail, albumSongs, err := New("").ParseAlbum(albums[0].Link)
	if err != nil {
		t.Fatalf("ParseAlbum() anonymous error = %v", err)
	}
	if albumDetail.ID != "web-album-1" ||
		albumDetail.Name != "Web Album Detail" ||
		albumDetail.Creator != "Web Album Artist" ||
		albumDetail.TrackCount != 2 ||
		albumDetail.Extra["metadata_source"] != "embed" {
		t.Fatalf("unexpected anonymous album detail: %#v", albumDetail)
	}
	if len(albumSongs) != 2 ||
		albumSongs[0].ID != "web-album-track-1" ||
		albumSongs[0].Album != "Web Album Detail" ||
		albumSongs[0].AlbumID != "web-album-1" ||
		albumSongs[0].URL != "https://cdn.example/web-album-track-1.mp3" {
		t.Fatalf("unexpected anonymous album songs: %#v", albumSongs)
	}

	playlists, err := New("").SearchPlaylist("hello")
	if err != nil {
		t.Fatalf("SearchPlaylist() anonymous error = %v", err)
	}
	if len(playlists) != 1 ||
		playlists[0].ID != "web-playlist-1" ||
		playlists[0].Name != "Web Playlist" ||
		playlists[0].Creator != "Spotify" ||
		playlists[0].TrackCount != 25 {
		t.Fatalf("unexpected anonymous playlists: %#v", playlists)
	}

	categories, err := New("").GetPlaylistCategories()
	if err != nil {
		t.Fatalf("GetPlaylistCategories() anonymous error = %v", err)
	}
	if len(categories) == 0 || categories[0].Source != "spotify" {
		t.Fatalf("unexpected anonymous categories: %#v", categories)
	}

	categoryPlaylists, err := New("").GetCategoryPlaylists("pop", 1, 10)
	if err != nil {
		t.Fatalf("GetCategoryPlaylists() anonymous error = %v", err)
	}
	if len(categoryPlaylists) != 1 || categoryPlaylists[0].ID != "web-playlist-1" {
		t.Fatalf("unexpected anonymous category playlists: %#v", categoryPlaylists)
	}

	recommended, err := New("").GetRecommendedPlaylists()
	if err != nil {
		t.Fatalf("GetRecommendedPlaylists() anonymous error = %v", err)
	}
	if len(recommended) == 0 || recommended[0].Source != "spotify" {
		t.Fatalf("unexpected anonymous recommendations: %#v", recommended)
	}

	if got := atomic.LoadInt32(&counters.bootstrap); got != 1 {
		t.Fatalf("bootstrap requests = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&counters.token); got != 1 {
		t.Fatalf("anonymous token requests = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&counters.clientToken); got != 1 {
		t.Fatalf("client token requests = %d, want 1", got)
	}
	if got := atomic.LoadInt32(&counters.search); got != 2 {
		t.Fatalf("search requests = %d, want 2", got)
	}
}

func TestSpotifyAnonymousSearchFallsBackWhenOfficialAPIForbidden(t *testing.T) {
	clearSpotifyEnvironment(t)
	_, restore, counters := newSpotifyWebPlayerTestServer(t)
	defer restore()

	oldAPIBase := spotifyAPIBase
	oldAccountsBase := spotifyAccountsBase
	t.Cleanup(func() {
		spotifyAPIBase = oldAPIBase
		spotifyAccountsBase = oldAccountsBase
	})
	spotifyAPIBase = strings.TrimRight(spotifyOpenBase, "/") + "/v1"
	spotifyAccountsBase = spotifyOpenBase

	client := New(`{"client_id":"client","client_secret":"secret"}`)
	songs, err := client.Search("hello")
	if err != nil {
		t.Fatalf("Search() forbidden fallback error = %v", err)
	}
	if len(songs) != 1 || songs[0].ID != "web-track-1" {
		t.Fatalf("unexpected fallback songs: %#v", songs)
	}
	if got := atomic.LoadInt32(&counters.search); got != 1 {
		t.Fatalf("anonymous search requests = %d, want 1", got)
	}
}

func TestSpotifyGenerateTOTP(t *testing.T) {
	secret := spotifyWebPlayerTOTPSecrets[0].values
	got := spotifyGenerateTOTP(secret, time.Unix(1_763_000_000, 0).UTC())
	if got != "643731" {
		t.Fatalf("spotifyGenerateTOTP() = %q, want 643731", got)
	}
}

func TestSpotifyAppServerClientVersion(t *testing.T) {
	config := base64.StdEncoding.EncodeToString([]byte(
		`{"clientVersion":"1.2.3.test"}`,
	))
	document := `<script id="appServerConfig" type="text/plain">` +
		config +
		`</script>`
	if got := spotifyAppServerClientVersion(document); got != "1.2.3.test" {
		t.Fatalf("spotifyAppServerClientVersion() = %q", got)
	}
}

func TestSpotifyWebPlayerRequestRetriesTransportEOF(t *testing.T) {
	var requests int32
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			requestNumber := atomic.AddInt32(&requests, 1)
			if requestNumber == 1 {
				if hijacker, ok := w.(http.Hijacker); ok {
					connection, _, err := hijacker.Hijack()
					if err == nil {
						_ = connection.Close()
						return
					}
				}
				http.Error(w, "first request failed", http.StatusBadGateway)
				return
			}
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if string(body) != "replayable" {
				http.Error(w, "body was not replayed", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte("ok"))
		},
	))
	defer server.Close()

	oldClient := spotifyWebPlayerHTTPClient
	spotifyWebPlayerHTTPClient = server.Client()
	t.Cleanup(func() {
		spotifyWebPlayerHTTPClient = oldClient
	})

	req, err := http.NewRequest(
		http.MethodPost,
		server.URL,
		strings.NewReader("replayable"),
	)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := spotifyDoWebPlayerRequest(req)
	if err != nil {
		t.Fatalf("spotifyDoWebPlayerRequest() error = %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Fatalf("response body = %q, want ok", body)
	}
	if got := atomic.LoadInt32(&requests); got != 2 {
		t.Fatalf("requests = %d, want 2", got)
	}
}

func TestSpotifyAnonymousWebPlayerLive(t *testing.T) {
	if os.Getenv("SPOTIFY_LIVE_TEST") != "1" {
		t.Skip("set SPOTIFY_LIVE_TEST=1 to run against the live Spotify Web Player")
	}
	clearSpotifyEnvironment(t)
	resetSpotifyWebPlayerTestCache()
	t.Cleanup(resetSpotifyWebPlayerTestCache)

	client := New("")
	songs, err := client.Search("hello")
	if err != nil {
		t.Fatalf("live Spotify song search error = %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("live Spotify song search returned no songs")
	}
	t.Logf("live spotify song: %s - %s (%s)", songs[0].Name, songs[0].Artist, songs[0].ID)

	albums, err := client.SearchAlbum("hello")
	if err != nil {
		t.Fatalf("live Spotify album search error = %v", err)
	}
	if len(albums) == 0 {
		t.Fatal("live Spotify album search returned no albums")
	}
	t.Logf("live spotify album: %s (%s)", albums[0].Name, albums[0].ID)
	albumDetail, albumSongs, err := client.ParseAlbum(albums[0].Link)
	if err != nil {
		t.Fatalf("live Spotify album detail error = %v", err)
	}
	if albumDetail == nil || len(albumSongs) == 0 {
		t.Fatal("live Spotify album detail returned no tracks")
	}
	t.Logf(
		"live spotify album detail: %s (%d tracks)",
		albumDetail.Name,
		len(albumSongs),
	)

	playlists, err := client.SearchPlaylist("hello")
	if err != nil {
		t.Fatalf("live Spotify playlist search error = %v", err)
	}
	if len(playlists) == 0 {
		t.Fatal("live Spotify playlist search returned no playlists")
	}
	t.Logf("live spotify playlist: %s (%s)", playlists[0].Name, playlists[0].ID)
}

type spotifyWebPlayerTestCounters struct {
	bootstrap   int32
	token       int32
	clientToken int32
	search      int32
}

func newSpotifyWebPlayerTestServer(
	t *testing.T,
) (*httptest.Server, func(), *spotifyWebPlayerTestCounters) {
	t.Helper()

	oldOpenBase := spotifyOpenBase
	oldClientTokenBase := spotifyClientTokenBase
	oldPartnerBase := spotifyPartnerBase
	oldAPIBase := spotifyAPIBase
	oldAccountsBase := spotifyAccountsBase
	oldEmbedBase := spotifyEmbedBase
	resetSpotifyWebPlayerTestCache()

	counters := &spotifyWebPlayerTestCounters{}
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/":
				if r.URL.Query().Get("uri") != "" {
					http.NotFound(w, r)
					return
				}
				atomic.AddInt32(&counters.bootstrap, 1)
				http.SetCookie(w, &http.Cookie{
					Name:  "sp_t",
					Value: "test-device-id",
					Path:  "/",
				})
				config := base64.StdEncoding.EncodeToString([]byte(
					`{"clientVersion":"1.3.10.test"}`,
				))
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = fmt.Fprintf(
					w,
					`<html><script id="appServerConfig" type="text/plain">%s</script></html>`,
					config,
				)
			case "/api/token":
				atomic.AddInt32(&counters.token, 1)
				_ = r.ParseForm()
				if r.Form.Get("grant_type") == "client_credentials" {
					writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
						"access_token": "official-token",
						"expires_in":   3600,
					})
					return
				}
				if r.URL.Query().Get("reason") != "init" ||
					r.URL.Query().Get("productType") != "web-player" ||
					len(r.URL.Query().Get("totp")) != 6 ||
					r.URL.Query().Get("totpVer") != "61" {
					writeSpotifyTestJSON(w, http.StatusBadRequest, map[string]any{
						"error": "invalid anonymous token request",
					})
					return
				}
				writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
					"clientId":    "anonymous-client",
					"accessToken": "anonymous-access-token",
				})
			case "/v1/clienttoken":
				atomic.AddInt32(&counters.clientToken, 1)
				var payload struct {
					ClientData struct {
						ClientVersion string `json:"client_version"`
						ClientID      string `json:"client_id"`
						SDKData       struct {
							DeviceID string `json:"device_id"`
						} `json:"js_sdk_data"`
					} `json:"client_data"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if payload.ClientData.ClientVersion != "1.3.10.test" ||
					payload.ClientData.ClientID != "anonymous-client" ||
					payload.ClientData.SDKData.DeviceID != "test-device-id" {
					writeSpotifyTestJSON(w, http.StatusBadRequest, map[string]any{
						"error": "unexpected client token payload",
					})
					return
				}
				writeSpotifyTestJSON(w, http.StatusOK, map[string]any{
					"granted_token": map[string]any{
						"token": "anonymous-client-token",
					},
				})
			case "/embed/album/web-album-1":
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(spotifyAlbumEmbedTestPage()))
			case "/pathfinder/v2/query":
				atomic.AddInt32(&counters.search, 1)
				if r.Header.Get("Authorization") != "Bearer anonymous-access-token" ||
					r.Header.Get("Client-Token") != "anonymous-client-token" ||
					r.Header.Get("Spotify-App-Version") != "1.3.10.test" {
					writeSpotifyTestJSON(w, http.StatusUnauthorized, map[string]any{
						"error": "missing web player headers",
					})
					return
				}
				var payload struct {
					OperationName string `json:"operationName"`
					Extensions    struct {
						PersistedQuery struct {
							Hash string `json:"sha256Hash"`
						} `json:"persistedQuery"`
					} `json:"extensions"`
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if payload.OperationName != spotifyWebSearchOperation ||
					payload.Extensions.PersistedQuery.Hash != spotifyWebSearchHash {
					writeSpotifyTestJSON(w, http.StatusBadRequest, map[string]any{
						"error": "unexpected search payload",
					})
					return
				}
				writeSpotifyTestJSON(w, http.StatusOK, spotifyWebPlayerSearchTestResponse())
			case "/v1/search":
				writeSpotifyTestJSON(w, http.StatusForbidden, map[string]any{
					"error": map[string]any{
						"message": "Active premium subscription required for the owner of the app",
					},
				})
			default:
				http.NotFound(w, r)
			}
		},
	))
	spotifyOpenBase = server.URL
	spotifyClientTokenBase = server.URL
	spotifyPartnerBase = server.URL
	spotifyEmbedBase = server.URL

	return server, func() {
		server.Close()
		spotifyOpenBase = oldOpenBase
		spotifyClientTokenBase = oldClientTokenBase
		spotifyPartnerBase = oldPartnerBase
		spotifyAPIBase = oldAPIBase
		spotifyAccountsBase = oldAccountsBase
		spotifyEmbedBase = oldEmbedBase
		resetSpotifyWebPlayerTestCache()
	}, counters
}

func resetSpotifyWebPlayerTestCache() {
	spotifyWebPlayerAuthCache.Lock()
	spotifyWebPlayerAuthCache.value = nil
	spotifyWebPlayerAuthCache.Unlock()
	spotifyWebPlayerSearchCache.Lock()
	spotifyWebPlayerSearchCache.entries = nil
	spotifyWebPlayerSearchCache.Unlock()
}

func spotifyWebPlayerSearchTestResponse() map[string]any {
	return map[string]any{
		"data": map[string]any{
			"searchV2": map[string]any{
				"tracksV2": map[string]any{
					"items": []any{
						map[string]any{
							"item": map[string]any{
								"data": map[string]any{
									"uri":         "spotify:track:web-track-1",
									"name":        "Web Hello",
									"trackNumber": 2,
									"discNumber":  1,
									"duration": map[string]any{
										"totalMilliseconds": 181000,
									},
									"artists": map[string]any{
										"items": []any{
											map[string]any{
												"profile": map[string]any{"name": "Web Artist"},
											},
											map[string]any{
												"profile": map[string]any{"name": "Featured Artist"},
											},
										},
									},
									"albumOfTrack": map[string]any{
										"uri":  "spotify:album:web-album-1",
										"name": "Web Album",
										"coverArt": map[string]any{
											"sources": []any{
												map[string]any{
													"url":   "https://cdn.example/web-album.jpg",
													"width": 640,
												},
											},
										},
									},
									"previews": map[string]any{
										"audioPreviews": map[string]any{
											"items": []any{
												map[string]any{
													"url": "https://cdn.example/web-preview.mp3",
												},
											},
										},
									},
									"playability":   map[string]any{"playable": true},
									"contentRating": map[string]any{"label": "EXPLICIT"},
								},
							},
						},
					},
				},
				"albumsV2": map[string]any{
					"items": []any{
						map[string]any{
							"item": map[string]any{
								"data": map[string]any{
									"uri":  "spotify:album:web-album-1",
									"name": "Web Album",
									"artists": map[string]any{
										"items": []any{
											map[string]any{
												"profile": map[string]any{"name": "Web Artist"},
											},
										},
									},
									"coverArt": map[string]any{
										"sources": []any{
											map[string]any{
												"url":   "https://cdn.example/web-album.jpg",
												"width": 640,
											},
										},
									},
									"date": map[string]any{
										"year":      2026,
										"isoString": "2026-01-02",
									},
									"tracks": map[string]any{"totalCount": 12},
								},
							},
						},
					},
				},
				"playlistsV2": map[string]any{
					"items": []any{
						map[string]any{
							"item": map[string]any{
								"data": map[string]any{
									"uri":         "spotify:playlist:web-playlist-1",
									"name":        "Web Playlist",
									"description": "Anonymous playlist",
									"ownerV2": map[string]any{
										"data": map[string]any{
											"name": "Spotify",
											"uri":  "spotify:user:spotify",
										},
									},
									"images": map[string]any{
										"items": []any{
											map[string]any{
												"sources": []any{
													map[string]any{
														"url":   "https://cdn.example/web-playlist.jpg",
														"width": 300,
													},
												},
											},
										},
									},
									"content": map[string]any{"totalCount": 25},
								},
							},
						},
					},
				},
			},
		},
	}
}

func spotifyAlbumEmbedTestPage() string {
	payload := map[string]any{
		"props": map[string]any{
			"pageProps": map[string]any{
				"state": map[string]any{
					"data": map[string]any{
						"entity": map[string]any{
							"type":        "album",
							"id":          "web-album-1",
							"uri":         "spotify:album:web-album-1",
							"name":        "Web Album Detail",
							"subtitle":    "Web Album Artist",
							"releaseDate": "2026-01-02",
							"visualIdentity": map[string]any{
								"image": []any{
									map[string]any{
										"url":   "https://cdn.example/web-album-detail.jpg",
										"width": 640,
									},
								},
							},
							"trackList": []any{
								map[string]any{
									"uri":        "spotify:track:web-album-track-1",
									"title":      "Album Track One",
									"subtitle":   "Web Album Artist",
									"duration":   180000,
									"isPlayable": true,
									"audioPreview": map[string]any{
										"url": "https://cdn.example/web-album-track-1.mp3",
									},
								},
								map[string]any{
									"uri":        "spotify:track:web-album-track-2",
									"title":      "Album Track Two",
									"subtitle":   "Web Album Artist",
									"duration":   200000,
									"isPlayable": true,
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
