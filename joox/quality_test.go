package joox

import (
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestListQualityOptionsPreservesJOOXOrder(t *testing.T) {
	options := ListQualityOptions()
	want := []model.AudioQuality{
		model.AudioQualityLossless,
		model.AudioQualityHigh,
		model.AudioQualityHigher,
		model.AudioQualityStandard,
		model.AudioQualityLow,
	}
	if len(options) != len(want) {
		t.Fatalf("options length = %d, want %d", len(options), len(want))
	}
	for i := range want {
		if options[i].Value != want[i] {
			t.Fatalf("option %d = %s, want %s", i, options[i].Value, want[i])
		}
	}
}

func TestJOOXKbpsMapControlsAvailableTiers(t *testing.T) {
	detail := &jooxSongDetail{
		resp: jooxSongDetailResponse{
			R320URL: "https://example.com/320.mp3",
			R192URL: "https://example.com/192.mp3",
			Mp3URL:  "https://example.com/128.mp3",
			M4aURL:  "https://example.com/96.m4a",
		},
		kbps: jooxKbpsMap(`{"320":"0","192":"12345","128":54321,"96":0}`),
	}
	options := detail.availableQualityOptions()
	if len(options) != 2 {
		t.Fatalf("available options = %#v, want two", options)
	}
	if options[0].Value != model.AudioQualityHigher {
		t.Fatalf("first option = %s, want higher", options[0].Value)
	}
	if options[1].Value != model.AudioQualityStandard {
		t.Fatalf("second option = %s, want standard", options[1].Value)
	}
	if got := detail.bestURL(); got != detail.resp.R192URL {
		t.Fatalf("best URL = %q, want %q", got, detail.resp.R192URL)
	}
}

func TestJOOXOpenTrackMapsPlayURLList(t *testing.T) {
	detail := newJOOXOpenSongDetail("song-id", &jooxOpenTrackResponse{
		ID:   "song-id",
		Name: "song",
		PlayURLList: []string{
			"https://ak-hk.stream.music.joox.com/M800TRACK.mp3?vkey=1&amp;hdnts=2",
			"https://ak-hk.stream.music.joox.com/C400TRACK.m4a?vkey=3&amp;hdnts=4",
			"https://ak-hk.stream.music.joox.com/C200TRACK.m4a?vkey=5&amp;hdnts=6",
		},
	})
	options := detail.availableQualityOptions()
	if len(options) != 3 {
		t.Fatalf("available options = %#v, want three", options)
	}
	want := []model.AudioQuality{
		model.AudioQualityHigh,
		model.AudioQualityStandard,
		model.AudioQualityLow,
	}
	for i := range want {
		if options[i].Value != want[i] {
			t.Fatalf("option %d = %s, want %s", i, options[i].Value, want[i])
		}
	}
	for _, streamURL := range detail.openURLs {
		if strings.Contains(streamURL, "&amp;") {
			t.Fatalf("stream URL was not HTML-unescaped: %q", streamURL)
		}
	}
}

func TestJOOXTrackSecretMatchesWebAlgorithm(t *testing.T) {
	got := jooxTrackSecret([]jooxSignedParam{
		{key: "country", value: "sg"},
		{key: "lang", value: "zh_cn"},
		{key: "lyric", value: 1},
		{key: "fs", value: 1},
		{key: "im", value: 0},
		{key: "uid", value: "142420656"},
		{key: "usk", value: "2a5d97d05dc8fe238150184eaf3519ad"},
		{key: "id", value: "vT8gcKzRFXzZypW6PJuLCA=="},
	})
	const want = "e1c226db9ca1778734c3d0f99e52a165"
	if got != want {
		t.Fatalf("secret = %s, want %s", got, want)
	}
}

func TestJOOXFallsBackToURLsWhenKbpsMapMissing(t *testing.T) {
	detail := &jooxSongDetail{
		resp: jooxSongDetailResponse{
			R192URL: "https://example.com/192.mp3",
		},
	}
	if got := detail.bestURL(); got != detail.resp.R192URL {
		t.Fatalf("best URL = %q, want %q", got, detail.resp.R192URL)
	}
}
