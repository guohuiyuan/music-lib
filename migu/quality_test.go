package migu

import (
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestMiguQualityToneMapping(t *testing.T) {
	tests := map[model.AudioQuality][]string{
		model.AudioQualityLow:      {"LQ"},
		model.AudioQualityStandard: {"PQ"},
		model.AudioQualityHigh:     {"HQ"},
		model.AudioQualityLossless: {"SQ"},
		model.AudioQualityHiRes:    {"ZQ24", "ZQ"},
		model.AudioQualityMaster:   {"ZQ32"},
	}
	for quality, want := range tests {
		candidate, ok := miguCandidateForQuality(quality)
		if !ok {
			t.Fatalf("quality %s not found", quality)
		}
		if len(candidate.tones) != len(want) {
			t.Fatalf("quality %s tones = %#v, want %#v", quality, candidate.tones, want)
		}
		for i := range want {
			if candidate.tones[i] != want[i] {
				t.Fatalf("quality %s tones = %#v, want %#v", quality, candidate.tones, want)
			}
		}
	}
}
