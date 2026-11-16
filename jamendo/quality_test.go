package jamendo

import (
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestListQualityOptionsPreservesJamendoOrder(t *testing.T) {
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

func TestJamendoQualityURLUsesDownloadAndStreamMaps(t *testing.T) {
	item := jamendoTrackItem{
		Download: map[string]string{
			"flac": "https://example.com/download.flac",
			"mp32": "https://example.com/download-192.mp3",
			"mp3":  "https://example.com/download.mp3",
			"ogg":  "https://example.com/download.ogg",
		},
		Stream: map[string]string{
			"mp33": "https://example.com/stream-320.mp3",
			"mp3":  "https://example.com/stream-96.mp3",
		},
	}
	tests := []struct {
		quality model.AudioQuality
		want    string
	}{
		{quality: model.AudioQualityLossless, want: item.Download["flac"]},
		{quality: model.AudioQualityHigh, want: item.Stream["mp33"]},
		{quality: model.AudioQualityHigher, want: item.Download["mp32"]},
		{quality: model.AudioQualityStandard, want: item.Download["ogg"]},
		{quality: model.AudioQualityLow, want: item.Stream["mp3"]},
	}
	for _, test := range tests {
		candidate, ok := jamendoQualityCandidateFor(test.quality)
		if !ok {
			t.Fatalf("missing candidate %s", test.quality)
		}
		if got := jamendoQualityURL(item, candidate); got != test.want {
			t.Fatalf("quality %s = %q, want %q", test.quality, got, test.want)
		}
	}
}

func TestJamendoQualityURLDoesNotTreatDownloadMP3AsLow(t *testing.T) {
	item := jamendoTrackItem{
		Download: map[string]string{
			"mp3": "https://example.com/download-192.mp3",
		},
	}
	candidate, ok := jamendoQualityCandidateFor(model.AudioQualityHigher)
	if !ok {
		t.Fatal("missing higher candidate")
	}
	if got := jamendoQualityURL(item, candidate); got != item.Download["mp3"] {
		t.Fatalf("higher quality = %q, want %q", got, item.Download["mp3"])
	}

	low, ok := jamendoQualityCandidateFor(model.AudioQualityLow)
	if !ok {
		t.Fatal("missing low candidate")
	}
	if got := jamendoQualityURL(item, low); got != "" {
		t.Fatalf("low quality = %q, want empty", got)
	}
}
