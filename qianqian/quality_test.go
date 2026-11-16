package qianqian

import (
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestQianqianQualityCandidateMapping(t *testing.T) {
	tests := map[model.AudioQuality]string{
		model.AudioQualityLossless: "3000",
		model.AudioQualityHigh:     "320",
		model.AudioQualityStandard: "128",
		model.AudioQualityLow:      "64",
	}
	for quality, want := range tests {
		candidate, ok := qianqianQualityCandidateFor(quality)
		if !ok {
			t.Fatalf("quality %s was not found", quality)
		}
		if candidate.rate != want {
			t.Fatalf("quality %s rate = %q, want %q", quality, candidate.rate, want)
		}
	}
}

func TestQianqianRejectsUnsupportedQualityBeforeRequest(t *testing.T) {
	q := New("")
	if _, err := q.GetDownloadURLWithQuality(
		&model.Song{Source: "qianqian", ID: "1"},
		string(model.AudioQualityMaster),
	); err == nil {
		t.Fatal("master quality should be rejected")
	}
}

func TestQianqianExactQualityDoesNotReuseUnknownSongURL(t *testing.T) {
	q := New("")
	_, err := q.GetDownloadURLWithQuality(
		&model.Song{
			Source: "qianqian",
			URL:    "https://example.com/already-resolved.mp3",
		},
		string(model.AudioQualityHigh),
	)
	if err == nil {
		t.Fatal("exact quality request reused a URL without verifying its quality")
	}
}
