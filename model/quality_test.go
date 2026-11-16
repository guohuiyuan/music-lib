package model

import "testing"

func TestNormalizeAudioQualityProviderAliases(t *testing.T) {
	tests := map[string]AudioQuality{
		"jymaster": AudioQualityMaster,
		"sky":      AudioQualitySpatial,
		"jyeffect": AudioQualitySpatial,
		"vivid":    AudioQualityVivid,
		"dolby":    AudioQualityAtmos,
		"zq32":     AudioQualityMaster,
	}
	for input, want := range tests {
		got, ok := NormalizeAudioQuality(input)
		if !ok || got != want {
			t.Fatalf("NormalizeAudioQuality(%q) = %q, %t; want %q, true", input, got, ok, want)
		}
	}
}
