package main

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/guohuiyuan/music-lib/apple"
	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/fivesing"
	"github.com/guohuiyuan/music-lib/jamendo"
	"github.com/guohuiyuan/music-lib/joox"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/kuwo"
	"github.com/guohuiyuan/music-lib/migu"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qianqian"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/soda"
	"github.com/guohuiyuan/music-lib/spotify"
)

func TestAudioQualityOptionsIntegration(t *testing.T) {
	if os.Getenv("MUSICLIB_RUN_QUALITY_INTEGRATION") != "1" {
		t.Skip("set MUSICLIB_RUN_QUALITY_INTEGRATION=1 to probe live quality options")
	}

	qqCookie := strings.Trim(strings.TrimSpace(getEnvCookie("QQ_COOKIE")), `"'`)
	neteaseCookie := strings.Trim(
		strings.TrimSpace(getEnvCookie("NETEASE_COOKIE")),
		`"'`,
	)
	kugouCookie := strings.Trim(
		strings.TrimSpace(getEnvCookie("KUGOU_COOKIE")),
		`"'`,
	)
	kuwoCookie := strings.Trim(
		strings.TrimSpace(getEnvCookie("KUWO_COOKIE")),
		`"'`,
	)
	sodaCookie := strings.Trim(
		strings.TrimSpace(getEnvCookie("SODA_COOKIE")),
		`"'`,
	)
	bilibiliCookie := strings.Trim(
		strings.TrimSpace(getEnvCookie("BILIBILI_COOKIE")),
		`"'`,
	)
	cases := []struct {
		name   string
		client interface {
			AvailableQualityOptions() ([]model.AudioQualityOption, error)
		}
	}{
		{name: "qq-configured", client: qq.New(qqCookie)},
		{name: "qq-empty", client: qq.New("")},
		{name: "netease-configured", client: netease.New(neteaseCookie)},
		{name: "netease-empty", client: netease.New("")},
		{name: "kugou-configured", client: kugou.New(kugouCookie)},
		{name: "kugou-empty", client: kugou.New("")},
		{name: "kuwo-configured", client: kuwo.New(kuwoCookie)},
		{name: "kuwo-empty", client: kuwo.New("")},
		{name: "migu-empty", client: migu.New("")},
		{name: "soda-configured", client: soda.New(sodaCookie)},
		{name: "soda-empty", client: soda.New("")},
		{name: "qianqian-empty", client: qianqian.New("")},
		{name: "apple-empty", client: apple.New("")},
		{name: "spotify-empty", client: spotify.New("")},
		{name: "fivesing-empty", client: fivesing.New("")},
		{name: "jamendo-empty", client: jamendo.New("")},
		{name: "joox-empty", client: joox.New("")},
		{name: "bilibili-empty", client: bilibili.New("")},
		{name: "bilibili-configured", client: bilibili.New(bilibiliCookie)},
	}
	for _, test := range cases {
		test := test
		t.Run(test.name, func(t *testing.T) {
			options, err := test.client.AvailableQualityOptions()
			if err != nil {
				t.Fatalf("probe quality options: %v", err)
			}
			t.Logf("%s available qualities (%d)", test.name, len(options))
			for _, option := range options {
				t.Logf("- %s: %s, format=%s, %d kbps, vip=%t",
					option.Value, option.Label, option.Format, option.Bitrate, option.VIP)
			}
		})
	}
}

func TestNewAudioQualityDownloadsIntegration(t *testing.T) {
	if os.Getenv("MUSICLIB_RUN_QUALITY_INTEGRATION") != "1" {
		t.Skip("set MUSICLIB_RUN_QUALITY_INTEGRATION=1 to probe live quality downloads")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	assertPlayable := func(t *testing.T, streamURL string, headers map[string]string) {
		t.Helper()
		if strings.TrimSpace(streamURL) == "" {
			t.Fatal("download URL is empty")
		}
		req, err := http.NewRequest(http.MethodGet, streamURL, nil)
		if err != nil {
			t.Fatalf("create stream request: %v", err)
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")
		req.Header.Set("Range", "bytes=0-0")
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("probe stream: %v", err)
		}
		defer func() {
			_ = resp.Body.Close()
		}()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
			t.Fatalf("probe stream status = %d", resp.StatusCode)
		}
	}

	t.Run("fivesing", func(t *testing.T) {
		provider := fivesing.New("")
		song := &model.Song{Source: "fivesing", ID: "3529943|yc"}
		options, err := provider.AvailableQualityOptions()
		if err != nil {
			t.Fatalf("available qualities: %v", err)
		}
		for _, option := range options {
			streamURL, err := provider.GetDownloadURLWithQuality(song, string(option.Value))
			if err != nil {
				t.Fatalf("%s download URL: %v", option.Value, err)
			}
			assertPlayable(t, streamURL, nil)
		}
	})

	t.Run("jamendo", func(t *testing.T) {
		provider := jamendo.New("")
		songs, err := provider.Search("love")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(songs) == 0 {
			t.Fatal("search returned no songs")
		}
		options, err := provider.AvailableQualityOptions()
		if err != nil {
			t.Fatalf("available qualities: %v", err)
		}
		for _, option := range options {
			resolved := false
			for i := 0; i < len(songs) && i < 8; i++ {
				streamURL, err := provider.GetDownloadURLWithQuality(&songs[i], string(option.Value))
				if err != nil {
					continue
				}
				assertPlayable(t, streamURL, nil)
				resolved = true
				break
			}
			if !resolved {
				t.Fatalf("no playable Jamendo track for %s", option.Value)
			}
		}
	})

	t.Run("joox", func(t *testing.T) {
		provider := joox.New("")
		songs, err := provider.Search(jooxQualityProbeKeywordForTest)
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(songs) == 0 {
			t.Fatal("search returned no songs")
		}
		options, err := provider.AvailableQualityOptions()
		if err != nil {
			t.Fatalf("available qualities: %v", err)
		}
		for _, option := range options {
			streamURL, err := provider.GetDownloadURLWithQuality(&songs[0], string(option.Value))
			if err != nil {
				t.Fatalf("%s download URL: %v", option.Value, err)
			}
			assertPlayable(t, streamURL, nil)
		}
	})

	t.Run("bilibili", func(t *testing.T) {
		cookie := strings.Trim(
			strings.TrimSpace(getEnvCookie("BILIBILI_COOKIE")),
			`"'`,
		)
		provider := bilibili.New(cookie)
		song := &model.Song{
			Source: "bilibili",
			ID:     "BV1zYWB6qED9|43575806705",
		}
		options, err := provider.AvailableQualityOptions()
		if err != nil {
			t.Fatalf("available qualities: %v", err)
		}
		for _, option := range options {
			streamURL, err := provider.GetDownloadURLWithQuality(song, string(option.Value))
			if err != nil {
				t.Fatalf("%s download URL: %v", option.Value, err)
			}
			assertPlayable(t, streamURL, map[string]string{
				"Referer": "https://www.bilibili.com/",
				"Cookie":  cookie,
			})
		}
	})
}

const jooxQualityProbeKeywordForTest = "周杰伦"
