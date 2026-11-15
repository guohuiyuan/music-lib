package main

import (
	"os"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/apple"
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
	cases := []struct {
		name   string
		client interface {
			AvailableQualityOptions() ([]model.AudioQualityOption, error)
		}
	}{
		{name: "qq-configured", client: qq.New(qqCookie)},
		{name: "qq-empty", client: qq.New("")},
		{name: "netease-empty", client: netease.New("")},
		{name: "kugou-empty", client: kugou.New("")},
		{name: "kuwo-empty", client: kuwo.New("")},
		{name: "migu-empty", client: migu.New("")},
		{name: "soda-empty", client: soda.New("")},
		{name: "qianqian-empty", client: qianqian.New("")},
		{name: "apple-empty", client: apple.New("")},
		{name: "spotify-empty", client: spotify.New("")},
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
