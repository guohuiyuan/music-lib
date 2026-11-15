package kugou

import (
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestKugouHashesForQuality(t *testing.T) {
	song := &model.Song{
		ID: "id",
		Extra: map[string]string{
			"hash":         "0123456789abcdef0123456789abcdef",
			"ogg_128_hash": "1123456789abcdef0123456789abcdef",
			"ogg_320_hash": "2123456789abcdef0123456789abcdef",
			"sq_hash":      "3123456789abcdef0123456789abcdef",
			"res_hash":     "4123456789abcdef0123456789abcdef",
		},
	}
	tests := map[model.AudioQuality]string{
		model.AudioQualityStandard: "1123456789abcdef0123456789abcdef",
		model.AudioQualityHigh:     "2123456789abcdef0123456789abcdef",
		model.AudioQualityLossless: "3123456789abcdef0123456789abcdef",
		model.AudioQualityHiRes:    "4123456789abcdef0123456789abcdef",
	}
	for quality, want := range tests {
		hashes := kugouHashesForQuality(song, quality)
		if len(hashes) == 0 || hashes[0] != want {
			t.Fatalf("quality %s hashes = %#v, want first %q", quality, hashes, want)
		}
	}
}

func TestKugouSongQuality(t *testing.T) {
	tests := []struct {
		song *model.Song
		want model.AudioQuality
	}{
		{&model.Song{Ext: "mp3", Bitrate: 128}, model.AudioQualityStandard},
		{&model.Song{Ext: "mp3", Bitrate: 320}, model.AudioQualityHigh},
		{&model.Song{Ext: "flac", Bitrate: 800}, model.AudioQualityLossless},
		{&model.Song{Ext: "flac", Bitrate: 1200000}, model.AudioQualityHiRes},
		{&model.Song{Ext: "mp3", Size: 9_600_000, Duration: 240}, model.AudioQualityHigh},
		{&model.Song{Ext: "flac", Size: 28_800_000, Duration: 240}, model.AudioQualityLossless},
	}
	for _, test := range tests {
		if got := kugouSongQuality(test.song); got != test.want {
			t.Fatalf("kugouSongQuality(%+v) = %q, want %q", test.song, got, test.want)
		}
	}
}

func TestInferKugouBitrate(t *testing.T) {
	tests := []struct {
		name     string
		bitrate  int
		size     int64
		duration int
		want     int
	}{
		{name: "reported bitrate", bitrate: 320_000, size: 1, duration: 1, want: 320},
		{name: "infer from file size", bitrate: 0, size: 9_600_000, duration: 240, want: 320},
		{name: "infer with milliseconds duration", bitrate: 0, size: 3_840_000, duration: 240_000, want: 128},
		{name: "unknown without metadata", bitrate: 0, want: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := inferKugouBitrate(test.bitrate, test.size, test.duration); got != test.want {
				t.Fatalf("inferKugouBitrate(%d, %d, %d) = %d, want %d",
					test.bitrate, test.size, test.duration, got, test.want)
			}
		})
	}
}

func TestProbeKugouQualityOptionsStopsAfterFirstPlayableSong(t *testing.T) {
	songs := []*model.Song{
		{ID: "first"},
		{ID: "second"},
		{ID: "third"},
	}
	callsBySong := map[string]int{}
	options := probeKugouQualityOptions(
		songs,
		func(song *model.Song, quality model.AudioQuality) bool {
			callsBySong[song.ID]++
			if song.ID != "second" {
				return false
			}
			return quality == model.AudioQualityHigh ||
				quality == model.AudioQualityStandard
		},
	)

	if callsBySong["first"] != len(kugouQualityCandidates) {
		t.Fatalf("first song probe calls = %d, want %d",
			callsBySong["first"], len(kugouQualityCandidates))
	}
	if callsBySong["second"] != len(kugouQualityCandidates) {
		t.Fatalf("second song probe calls = %d, want %d",
			callsBySong["second"], len(kugouQualityCandidates))
	}
	if callsBySong["third"] != 0 {
		t.Fatalf("third song probe calls = %d, want 0", callsBySong["third"])
	}
	if len(options) != 2 ||
		options[0].Value != model.AudioQualityHigh ||
		options[1].Value != model.AudioQualityStandard {
		t.Fatalf("options = %#v, want high then standard", options)
	}
}
