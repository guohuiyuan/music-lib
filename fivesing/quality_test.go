package fivesing

import (
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestListQualityOptionsPreserves5singOrder(t *testing.T) {
	options := ListQualityOptions()
	want := []model.AudioQuality{
		model.AudioQualityHigh,
		model.AudioQualityHigher,
		model.AudioQualityStandard,
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

func TestFivesingAudioLinksSelectExactTier(t *testing.T) {
	links := fivesingAudioLinks{
		sq: "https://example.com/320.mp3",
		hq: "https://example.com/192.mp3",
		lq: "https://example.com/128.mp3",
	}
	for tier, want := range map[string]string{
		"sq": links.sq,
		"hq": links.hq,
		"lq": links.lq,
	} {
		if got := links.urlFor(tier); got != want {
			t.Fatalf("tier %s = %q, want %q", tier, got, want)
		}
	}
	if got := links.best(); got != links.sq {
		t.Fatalf("best = %q, want %q", got, links.sq)
	}
}

func TestFivesingQualityFallsBackToHQWhenSQMissing(t *testing.T) {
	links := fivesingAudioLinks{hq: "https://example.com/192.mp3"}
	if got := links.best(); got != links.hq {
		t.Fatalf("best = %q, want HQ %q", got, links.hq)
	}
}
