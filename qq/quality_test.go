package qq

import (
	"reflect"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestListQualityOptionsPreservesSelectionOrder(t *testing.T) {
	options := ListQualityOptions()
	want := []model.AudioQuality{
		model.AudioQualityMaster,
		model.AudioQualityAtmos51,
		model.AudioQualityAtmos,
		model.AudioQualityLossless,
		model.AudioQualityUltra,
		model.AudioQualityHigh,
		model.AudioQualityStandard,
	}
	if len(options) != len(want) {
		t.Fatalf("option count = %d, want %d", len(options), len(want))
	}
	for i := range want {
		if options[i].Value != want[i] {
			t.Fatalf("option[%d] = %q, want %q", i, options[i].Value, want[i])
		}
	}
}

func TestQualityFilenameMapping(t *testing.T) {
	const songMID = "abc123"
	tests := []struct {
		quality string
		want    string
	}{
		{string(model.AudioQualityMaster), "AI00abc123abc123.flac"},
		{string(model.AudioQualityAtmos51), "Q001abc123abc123.flac"},
		{string(model.AudioQualityAtmos), "Q000abc123abc123.flac"},
		{string(model.AudioQualityLossless), "F000abc123abc123.flac"},
		{string(model.AudioQualityUltra), "O801abc123abc123.ogg"},
		{string(model.AudioQualityHigh), "M800abc123abc123.mp3"},
		{string(model.AudioQualityStandard), "M500abc123abc123.mp3"},
	}
	for _, test := range tests {
		candidate, ok := qualityCandidateFor(test.quality)
		if !ok {
			t.Fatalf("quality %q was not found", test.quality)
		}
		if got := qualityFilename(songMID, candidate); got != test.want {
			t.Fatalf("quality %q filename = %q, want %q", test.quality, got, test.want)
		}
	}
}

func TestAvailableQualityOptionsFromPurls(t *testing.T) {
	const songMID = "probe"
	purls := map[string]string{
		qualityFilename(songMID, mustQualityCandidate(t, model.AudioQualityMaster)): "master",
		qualityFilename(songMID, mustQualityCandidate(t, model.AudioQualityHigh)):   "high",
	}
	options := availableQualityOptionsFromPurls(songMID, purls)
	got := make([]model.AudioQuality, 0, len(options))
	for _, option := range options {
		got = append(got, option.Value)
	}
	want := []model.AudioQuality{
		model.AudioQualityMaster,
		model.AudioQualityHigh,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("available qualities = %#v, want %#v", got, want)
	}
}

func TestGetDownloadURLWithQualityRejectsInvalidInputBeforeRequest(t *testing.T) {
	q := New("")
	if _, err := q.GetDownloadURLWithQuality(&model.Song{ID: "song", Source: "qq"}, "not-a-quality"); err == nil {
		t.Fatal("invalid quality should fail")
	}
	if _, err := q.GetDownloadURLWithQuality(&model.Song{ID: "song", Source: "netease"}, string(model.AudioQualityHigh)); err == nil {
		t.Fatal("source mismatch should fail")
	}
}

func mustQualityCandidate(t *testing.T, quality model.AudioQuality) qualityCandidate {
	t.Helper()
	candidate, ok := qualityCandidateFor(string(quality))
	if !ok {
		t.Fatalf("quality candidate %q not found", quality)
	}
	return candidate
}
