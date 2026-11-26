package netease

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const regressionPlaylistLink = "https://music.163.com/#/playlist?id=7507256008"

func TestParsePlaylistRegression(t *testing.T) {
	playlist, songs, err := ParsePlaylist(regressionPlaylistLink)
	if err != nil {
		t.Fatalf("ParsePlaylist failed for %s: %v", regressionPlaylistLink, err)
	}
	if playlist == nil {
		t.Fatalf("ParsePlaylist returned nil playlist for %s", regressionPlaylistLink)
	}
	if playlist.ID != "7507256008" {
		t.Fatalf("ParsePlaylist returned unexpected playlist id: got %q", playlist.ID)
	}
	if len(songs) == 0 {
		t.Fatalf("ParsePlaylist returned no songs for %s", regressionPlaylistLink)
	}
}

func TestParseNeteaseLinkRegression(t *testing.T) {
	tests := []struct {
		name     string
		link     string
		wantKind neteaseLinkKind
		wantID   string
	}{
		{
			name:     "hash playlist link",
			link:     regressionPlaylistLink,
			wantKind: neteaseLinkPlaylist,
			wantID:   "7507256008",
		},
		{
			name:     "direct song link",
			link:     "https://music.163.com/song?id=29732995",
			wantKind: neteaseLinkSong,
			wantID:   "29732995",
		},
		{
			name:     "direct album link",
			link:     "https://music.163.com/album?id=32311",
			wantKind: neteaseLinkAlbum,
			wantID:   "32311",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			kind, id, err := parseNeteaseLink(tc.link)
			if err != nil {
				t.Fatalf("parseNeteaseLink(%q) returned error: %v", tc.link, err)
			}
			if kind != tc.wantKind {
				t.Fatalf("parseNeteaseLink(%q) kind mismatch: got %q want %q", tc.link, kind, tc.wantKind)
			}
			if id != tc.wantID {
				t.Fatalf("parseNeteaseLink(%q) id mismatch: got %q want %q", tc.link, id, tc.wantID)
			}
		})
	}
}

func TestParseWithPlaylistLinkRegression(t *testing.T) {
	song, err := Parse(regressionPlaylistLink)
	if !errors.Is(err, errNeteasePlaylistLink) {
		t.Fatalf("Parse(%q) error mismatch: got %v", regressionPlaylistLink, err)
	}
	if song != nil {
		t.Fatalf("Parse(%q) returned unexpected song: %#v", regressionPlaylistLink, song)
	}
}

func TestParseRecommendedPlaylistsWithLargeIDRegression(t *testing.T) {
	const body = `{
		"code": 200,
		"result": [
			{
				"id": 2230318386,
				"name": "large id playlist",
				"type": 2230318386,
				"picUrl": "https://p1.music.126.net/example.jpg",
				"playCount": 1000,
				"trackCount": 20,
				"copywriter": "",
				"alg": "recommend"
			}
		]
	}`

	playlists, err := parseRecommendedPlaylists([]byte(body))
	if err != nil {
		t.Fatalf("parseRecommendedPlaylists returned error: %v", err)
	}
	if len(playlists) != 1 {
		t.Fatalf("parseRecommendedPlaylists returned %d playlists, want 1", len(playlists))
	}
	if playlists[0].ID != "2230318386" {
		t.Fatalf("parseRecommendedPlaylists returned id %q, want 2230318386", playlists[0].ID)
	}
	if playlists[0].Name != "large id playlist" {
		t.Fatalf("parseRecommendedPlaylists returned name %q, want large id playlist", playlists[0].Name)
	}
}

func TestCloudSearchUsesEAPIEndpoint(t *testing.T) {
	if !strings.Contains(SearchAPI, "/eapi/cloudsearch/pc") {
		t.Fatalf("SearchAPI = %q, want /eapi/cloudsearch/pc endpoint", SearchAPI)
	}
	rawPayload := `{"limit":10,"offset":0,"s":"晴天","type":1}`
	cipherHex := EncryptEApi(SearchAPI, rawPayload)
	cipherBytes, err := hex.DecodeString(cipherHex)
	if err != nil {
		t.Fatalf("hex decode eapi params: %v", err)
	}
	plain, err := aesECBDecrypt([]byte("e82ckenh8dichen8"), cipherBytes)
	if err != nil {
		t.Fatalf("decrypt eapi params: %v", err)
	}
	parts := strings.Split(string(pkcs7Unpad(plain)), "-36cd479b6b5-")
	if len(parts) != 3 || parts[0] != "/api/cloudsearch/pc" {
		t.Fatalf("unexpected decrypted eapi envelope: %q", parts)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(parts[1]), &got); err != nil || got["s"] != "晴天" {
		t.Fatalf("unexpected decrypted search payload: %v, %v", got, err)
	}
}
