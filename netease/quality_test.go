package netease

import (
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestNeteaseLevelForQuality(t *testing.T) {
	tests := map[string]string{
		"standard": "standard",
		"higher":   "higher",
		"high":     "exhigh",
		"320k":     "exhigh",
		"lossless": "lossless",
		"hi_res":   "hires",
		"dolby":    "dolby",
		"spatial":  "sky",
		"master":   "jymaster",
	}
	for input, want := range tests {
		got, ok := neteaseLevelForQuality(input)
		if !ok || got != want {
			t.Fatalf("neteaseLevelForQuality(%q) = %q, %t; want %q, true", input, got, ok, want)
		}
	}
	levels, ok := neteaseLevelsForQuality(string(model.AudioQualitySpatial))
	if !ok || len(levels) != 2 || levels[0] != "sky" || levels[1] != "jyeffect" {
		t.Fatalf("Netease spatial levels = %#v, %t; want sky and jyeffect", levels, ok)
	}
	vividLevels, ok := neteaseLevelsForQuality(string(model.AudioQualityVivid))
	if !ok || len(vividLevels) != 1 || vividLevels[0] != "vivid" {
		t.Fatalf("Netease vivid levels = %#v, %t; want vivid", vividLevels, ok)
	}
}

func TestNeteaseActualQuality(t *testing.T) {
	tests := []struct {
		name     string
		bitrate  int
		ext      string
		level    string
		expected model.AudioQuality
	}{
		{name: "api level wins", bitrate: 320_000, ext: "mp3", level: "higher", expected: model.AudioQualityHigher},
		{name: "master level", bitrate: 2_304_000, ext: "wav", level: "jymaster", expected: model.AudioQualityMaster},
		{name: "spatial level", bitrate: 900_000, ext: "flac", level: "sky", expected: model.AudioQualitySpatial},
		{name: "vivid level", bitrate: 320_000, ext: "mp3", level: "vivid", expected: model.AudioQualityVivid},
		{name: "atmos level", bitrate: 900_000, ext: "flac", level: "dolby", expected: model.AudioQualityAtmos},
		{name: "hires level", bitrate: 900_000, ext: "flac", level: "hires", expected: model.AudioQualityHiRes},
		{name: "lossless level", bitrate: 900_000, ext: "flac", level: "lossless", expected: model.AudioQualityLossless},
		{name: "bitrate fallback", bitrate: 128_000, ext: "mp3", expected: model.AudioQualityStandard},
		{name: "flac fallback", bitrate: 1_200_000, ext: "flac", expected: model.AudioQualityHiRes},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := neteaseAudioQuality(test.bitrate, test.ext, test.level); got != test.expected {
				t.Fatalf("quality = %q, want %q", got, test.expected)
			}
		})
	}
}

func TestNeteaseQualityAtLeast(t *testing.T) {
	if neteaseQualityAtLeast(model.AudioQualityHigh, model.AudioQualityLossless) {
		t.Fatal("320 kbps must not satisfy a lossless request")
	}
	if !neteaseQualityAtLeast(model.AudioQualityLossless, model.AudioQualityHigh) {
		t.Fatal("a lossless stream should satisfy a 320 kbps request")
	}
}

func TestNeteaseQualityLadderFrom(t *testing.T) {
	tests := []struct {
		name  string
		index int
		want  []model.AudioQuality
	}{
		{
			name:  "master includes every lower tier",
			index: 0,
			want: []model.AudioQuality{
				model.AudioQualityMaster,
				model.AudioQualitySpatial,
				model.AudioQualityVivid,
				model.AudioQualityAtmos,
				model.AudioQualityHiRes,
				model.AudioQualityLossless,
				model.AudioQualityHigh,
				model.AudioQualityHigher,
				model.AudioQualityStandard,
			},
		},
		{
			name:  "high includes standard tiers",
			index: 6,
			want: []model.AudioQuality{
				model.AudioQualityHigh,
				model.AudioQualityHigher,
				model.AudioQualityStandard,
			},
		},
		{
			name:  "invalid index",
			index: len(neteaseQualityCandidates),
			want:  nil,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := neteaseQualityLadderFrom(test.index)
			if len(options) != len(test.want) {
				t.Fatalf("options = %#v, want %#v", options, test.want)
			}
			for i, option := range options {
				if option.Value != test.want[i] {
					t.Fatalf("options = %#v, want %#v", options, test.want)
				}
			}
		})
	}
}
