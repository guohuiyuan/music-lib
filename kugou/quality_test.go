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
		model.AudioQualityUltra:    "4123456789abcdef0123456789abcdef",
		model.AudioQualitySpatial:  "4123456789abcdef0123456789abcdef",
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
		{&model.Song{URL: "https://example.com/track", Ext: "mp3", Bitrate: 128}, model.AudioQualityStandard},
		{&model.Song{URL: "https://example.com/track", Ext: "mp3", Bitrate: 320}, model.AudioQualityHigh},
		{&model.Song{URL: "https://example.com/track", Ext: "flac", Bitrate: 800}, model.AudioQualityLossless},
		{&model.Song{URL: "https://example.com/track", Ext: "flac", Bitrate: 1200000}, model.AudioQualityHiRes},
		{&model.Song{URL: "https://example.com/track", Ext: "flac", Size: 121_767_659, Duration: 270}, model.AudioQualityUltra},
		{&model.Song{URL: "https://example.com/track", Ext: "mp3", Size: 9_600_000, Duration: 240}, model.AudioQualityHigh},
		{&model.Song{URL: "https://example.com/track", Ext: "flac", Size: 28_800_000, Duration: 240}, model.AudioQualityLossless},
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

func TestKugouV5QualityNames(t *testing.T) {
	tests := map[model.AudioQuality]string{
		model.AudioQualityUltra:    "viper_clear",
		model.AudioQualitySpatial:  "viper_atmos",
		model.AudioQualityHiRes:    "high",
		model.AudioQualityLossless: "flac",
		model.AudioQualityHigh:     "320",
		model.AudioQualityStandard: "128",
	}
	for quality, want := range tests {
		names := kugouV5QualityNames(quality)
		if len(names) == 0 || names[0] != want {
			t.Fatalf("kugouV5QualityNames(%s) = %#v, want first %q", quality, names, want)
		}
	}
}

func TestKugouQualitySatisfies(t *testing.T) {
	tests := []struct {
		name      string
		song      *model.Song
		requested model.AudioQuality
		want      bool
	}{
		{
			name:      "hires accepts 24bit",
			song:      &model.Song{URL: "https://example.com/track", Ext: "flac", Size: 55_397_039, Duration: 270},
			requested: model.AudioQualityHiRes,
			want:      true,
		},
		{
			name:      "hires accepts 96khz",
			song:      &model.Song{URL: "https://example.com/track", Ext: "flac", Size: 121_767_659, Duration: 270},
			requested: model.AudioQualityHiRes,
			want:      true,
		},
		{
			name:      "ultra accepts 96khz",
			song:      &model.Song{URL: "https://example.com/track", Ext: "flac", Size: 121_767_659, Duration: 270},
			requested: model.AudioQualityUltra,
			want:      true,
		},
		{
			name:      "spatial accepts 6ch size",
			song:      &model.Song{URL: "https://example.com/track", Ext: "flac", Size: 72_293_078, Duration: 270},
			requested: model.AudioQualitySpatial,
			want:      true,
		},
		{
			name:      "spatial rejects 24bit",
			song:      &model.Song{URL: "https://example.com/track", Ext: "flac", Size: 55_397_039, Duration: 270},
			requested: model.AudioQualitySpatial,
			want:      false,
		},
		{
			name:      "high accepts 320",
			song:      &model.Song{URL: "https://example.com/track", Ext: "mp3", Size: 10_792_943, Duration: 270},
			requested: model.AudioQualityHigh,
			want:      true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := kugouQualitySatisfies(test.song, test.requested); got != test.want {
				t.Fatalf("kugouQualitySatisfies() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestAvailableQualityOptionsAnonymousReturnsStandard(t *testing.T) {
	options, err := New("").AvailableQualityOptions()
	if err != nil {
		t.Fatalf("AvailableQualityOptions: %v", err)
	}
	if len(options) != 1 || options[0].Value != model.AudioQualityStandard {
		t.Fatalf("anonymous options = %#v, want only standard", options)
	}
}

func TestProbeKugouQualityOptionsAggregatesAcrossSongs(t *testing.T) {
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
			switch song.ID {
			case "first":
				return quality == model.AudioQualityHigh
			case "second":
				return quality == model.AudioQualityStandard
			default:
				return false
			}
		},
	)

	if callsBySong["first"] != len(kugouQualityCandidates) {
		t.Fatalf("first song probe calls = %d, want %d",
			callsBySong["first"], len(kugouQualityCandidates))
	}
	if callsBySong["second"] == 0 || callsBySong["third"] == 0 {
		t.Fatalf("later songs were not probed: %#v", callsBySong)
	}
	if len(options) != 2 ||
		options[0].Value != model.AudioQualityHigh ||
		options[1].Value != model.AudioQualityStandard {
		t.Fatalf("options = %#v, want high then standard", options)
	}
}
