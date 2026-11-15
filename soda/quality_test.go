package soda

import (
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestSodaDownloadInfoQuality(t *testing.T) {
	tests := []struct {
		info *DownloadInfo
		want model.AudioQuality
	}{
		{&DownloadInfo{Quality: "medium", Bitrate: 128000}, model.AudioQualityStandard},
		{&DownloadInfo{Quality: "higher", Bitrate: 256000}, model.AudioQualityHigher},
		{&DownloadInfo{Quality: "highest", Bitrate: 320000}, model.AudioQualityHigh},
		{&DownloadInfo{Quality: "lossless", Format: "flac", Bitrate: 1000000}, model.AudioQualityLossless},
		{&DownloadInfo{Quality: "hi_res", Format: "flac", Bitrate: 2000000}, model.AudioQualityHiRes},
		{&DownloadInfo{Quality: "spatial", Bitrate: 320000}, model.AudioQualitySpatial},
		{&DownloadInfo{Quality: "lossless", Duration: 30, Preview: true}, model.AudioQualityPreview},
	}
	for _, test := range tests {
		if got := sodaDownloadInfoQuality(test.info); got != test.want {
			t.Fatalf("sodaDownloadInfoQuality(%+v) = %q, want %q", test.info, got, test.want)
		}
	}
}
