package migu

import (
	"os"
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
		model.AudioQualitySpatial:  {"Z3D", "I3D", "3D60"},
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

func TestMiguQualityOptionsFromProvenTone(t *testing.T) {
	tests := []struct {
		tone string
		want []model.AudioQuality
	}{
		{
			tone: "ZQ32",
			want: []model.AudioQuality{
				model.AudioQualityMaster,
				model.AudioQualitySpatial,
				model.AudioQualityHiRes,
				model.AudioQualityLossless,
				model.AudioQualityHigh,
				model.AudioQualityStandard,
				model.AudioQualityLow,
			},
		},
		{
			tone: "SQ",
			want: []model.AudioQuality{
				model.AudioQualityLossless,
				model.AudioQualityHigh,
				model.AudioQualityStandard,
				model.AudioQualityLow,
			},
		},
		{
			tone: "PQ",
			want: []model.AudioQuality{
				model.AudioQualityStandard,
				model.AudioQualityLow,
			},
		},
	}
	for _, test := range tests {
		options := miguQualityOptionsFromTone(test.tone)
		if len(options) != len(test.want) {
			t.Fatalf("tone %s options = %#v, want %#v", test.tone, options, test.want)
		}
		for index, quality := range test.want {
			if options[index].Value != quality {
				t.Fatalf("tone %s options = %#v, want %#v", test.tone, options, test.want)
			}
		}
	}
	if options := miguQualityOptionsFromTone("unknown"); len(options) != 0 {
		t.Fatalf("unknown tone options = %#v, want none", options)
	}
}

func TestMiguQualityRequestTonesFallsBackToBaseResponses(t *testing.T) {
	candidate, ok := miguCandidateForQuality(model.AudioQualityLossless)
	if !ok {
		t.Fatal("lossless candidate not found")
	}
	tones := miguQualityRequestTones(
		&model.Song{
			ID: "content|2|HQ",
			Extra: map[string]string{
				"format_type": "HQ",
			},
		},
		candidate,
	)
	want := []string{"SQ", "HQ", "PQ"}
	if len(tones) != len(want) {
		t.Fatalf("request tones = %#v, want %#v", tones, want)
	}
	for index := range want {
		if tones[index] != want[index] {
			t.Fatalf("request tones = %#v, want %#v", tones, want)
		}
	}
}

func TestLiveMiguQualityOptionsReportProvenHighestTier(t *testing.T) {
	cookie := os.Getenv("MIGU_TEST_COOKIE")
	if cookie == "" {
		t.Skip("set MIGU_TEST_COOKIE to run the live Migu quality probe")
	}

	options, err := New(cookie).AvailableQualityOptions()
	if err != nil {
		t.Fatalf("AvailableQualityOptions() error = %v", err)
	}
	hasLossless := false
	for _, option := range options {
		if option.Value == model.AudioQualityLossless {
			hasLossless = true
			break
		}
	}
	if !hasLossless {
		t.Fatalf("available qualities = %#v, want anonymously accessible lossless", options)
	}
}
