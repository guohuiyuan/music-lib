package kugou

import (
	"os"
	"testing"
)

func TestKugouQualityOptionsIntegration(t *testing.T) {
	if os.Getenv("MUSICLIB_RUN_QUALITY_INTEGRATION") != "1" {
		t.Skip("set MUSICLIB_RUN_QUALITY_INTEGRATION=1 to probe Kugou")
	}

	client := New("")
	songs, err := client.Search(kugouQualityProbeKeyword)
	if err != nil {
		t.Fatalf("search probe songs: %v", err)
	}
	if len(songs) == 0 {
		t.Fatal("Kugou search returned no probe songs")
	}
	for i := range songs {
		if i >= 3 {
			break
		}
		song := &songs[i]
		t.Logf("probe song %s - %s id=%s extra=%v", song.Name, song.Artist, song.ID, song.Extra)
		for _, option := range kugouQualityCandidates {
			info, probeErr := client.GetDownloadURLWithQuality(song, string(option.Value))
			if probeErr != nil {
				t.Logf("  %s: %v", option.Value, probeErr)
				continue
			}
			t.Logf("  %s: %s", option.Value, info)
		}
	}

	options, err := client.AvailableQualityOptions()
	if err != nil {
		t.Fatalf("probe quality options: %v", err)
	}
	for _, option := range options {
		t.Logf("available: %s (%s, %s, %d kbps)", option.Value, option.Label, option.Format, option.Bitrate)
	}
}
