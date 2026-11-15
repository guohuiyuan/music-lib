package spotify

import (
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestSpotifyOnlyExposesPreviewQuality(t *testing.T) {
	options := ListQualityOptions()
	if len(options) != 1 || options[0].Value != model.AudioQualityPreview {
		t.Fatalf("listed qualities = %#v, want preview only", options)
	}

	s := New("")
	song := &model.Song{Source: "spotify", ID: "1", URL: "https://example.com/preview.m4a"}
	url, err := s.GetDownloadURLWithQuality(song, string(model.AudioQualityPreview))
	if err != nil {
		t.Fatalf("preview quality rejected: %v", err)
	}
	if url != song.URL {
		t.Fatalf("preview URL = %q, want %q", url, song.URL)
	}
	if _, err := s.GetDownloadURLWithQuality(song, string(model.AudioQualityLossless)); err == nil {
		t.Fatal("lossless should be rejected")
	}
}
