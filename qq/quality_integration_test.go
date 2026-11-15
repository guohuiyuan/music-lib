package qq

import (
	"os"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestQQQualityOptionsIntegration(t *testing.T) {
	if os.Getenv("MUSICLIB_RUN_QUALITY_INTEGRATION") != "1" {
		t.Skip("set MUSICLIB_RUN_QUALITY_INTEGRATION=1 to probe QQ")
	}

	cookie := strings.TrimSpace(os.Getenv("MUSICLIB_QQ_COOKIE"))
	client := New(cookie)
	options, err := client.AvailableQualityOptions()
	if err != nil {
		t.Fatalf("probe QQ quality options: %v", err)
	}
	if len(options) == 0 {
		t.Fatal("QQ returned no quality options")
	}

	label := "empty cookie"
	if cookie != "" {
		label = "configured cookie"
	}
	for _, option := range options {
		t.Logf("%s: %s (%s, %s, %d kbps, vip=%t)",
			label, option.Value, option.Label, option.Format, option.Bitrate, option.VIP)
	}

	if cookie == "" {
		if len(options) == 0 {
			t.Log("empty cookie: no playable QQ stream tier is available")
		}
		return
	}

	song := &model.Song{ID: qqQualityProbeSongMID, Source: "qq"}
	for _, option := range options {
		url, resolveErr := client.GetDownloadURLWithQuality(song, string(option.Value))
		if resolveErr != nil {
			t.Fatalf("%s quality %s does not resolve: %v", label, option.Value, resolveErr)
		}
		if !strings.HasPrefix(url, "https://ws.stream.qqmusic.qq.com/") {
			t.Fatalf("%s quality %s returned an unexpected URL host", label, option.Value)
		}
	}
}
