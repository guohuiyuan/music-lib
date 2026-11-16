package bilibili

import (
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestListQualityOptionsPreservesBilibiliPriority(t *testing.T) {
	options := ListQualityOptions()
	want := []model.AudioQuality{
		model.AudioQualityHiRes,
		model.AudioQualityAtmos,
		model.AudioQualityHigh,
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

func TestBilibiliQualityForID(t *testing.T) {
	tests := []struct {
		id   int
		want model.AudioQuality
		ok   bool
	}{
		{id: 30251, want: model.AudioQualityHiRes, ok: true},
		{id: 30250, want: model.AudioQualityAtmos, ok: true},
		{id: 30280, want: model.AudioQualityHigh, ok: true},
		{id: 30232, want: model.AudioQualityStandard, ok: true},
		{id: 30216, want: model.AudioQualityLow, ok: true},
		{id: 99999, ok: false},
	}
	for _, test := range tests {
		got, ok := bilibiliQualityForID(test.id)
		if ok != test.ok || got != test.want {
			t.Fatalf("id %d = (%s, %t), want (%s, %t)", test.id, got, ok, test.want, test.ok)
		}
	}
}

func TestBilibiliAudioCandidateOrdering(t *testing.T) {
	candidates := make([]bilibiliAudioCandidate, 0, 3)
	for _, audio := range []bilibiliDashAudio{
		{ID: 30216, BaseURL: "https://example.com/64.m4s"},
		{ID: 30280, BaseURL: "https://example.com/192.m4s"},
		{ID: 30251, BaseURL: "https://example.com/hires.flac"},
	} {
		candidates = appendBilibiliAudioCandidate(candidates, audio)
	}
	if candidates[0].option.Value != model.AudioQualityLow {
		t.Fatalf("test setup order changed: %#v", candidates)
	}
}
