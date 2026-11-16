package kuwo

import (
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestKuwoAudioQuality(t *testing.T) {
	tests := []struct {
		bitrate int
		format  string
		want    model.AudioQuality
	}{
		{64, "mp3", model.AudioQualityLow},
		{128, "mp3", model.AudioQualityStandard},
		{128000, "mp3", model.AudioQualityStandard},
		{320, "mp3", model.AudioQualityHigh},
		{800, "flac", model.AudioQualityLossless},
		{2000, "flac", model.AudioQualityHiRes},
	}
	for _, test := range tests {
		if got := kuwoAudioQuality(test.bitrate, test.format); got != test.want {
			t.Fatalf("kuwoAudioQuality(%d, %q) = %q, want %q", test.bitrate, test.format, got, test.want)
		}
	}
}

func TestKuwoExactQualityDoesNotReuseUnknownSongURL(t *testing.T) {
	k := New("")
	_, err := k.GetDownloadURLWithQuality(
		&model.Song{
			Source: "kuwo",
			URL:    "https://example.com/already-resolved.mp3",
		},
		string(model.AudioQualityHigh),
	)
	if err == nil {
		t.Fatal("exact quality request reused a URL without verifying its quality")
	}
}
