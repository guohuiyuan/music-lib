package soda

import (
	"net/url"
	"testing"
)

func TestSodaAndroidSearchURLUsesAndroidParams(t *testing.T) {
	raw := sodaAndroidSearchURL("track", "周杰伦", 2, sodaAndroidSearchPageSize)
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse soda search url: %v", err)
	}
	if parsed.Path != "/luna/search/track" {
		t.Fatalf("search path = %q, want /luna/search/track", parsed.Path)
	}

	query := parsed.Query()
	if query.Get("q") != "周杰伦" {
		t.Fatalf("q = %q, want 周杰伦", query.Get("q"))
	}
	if query.Get("cursor") != "20" {
		t.Fatalf("cursor = %q, want 20", query.Get("cursor"))
	}
	if query.Get("count") != "20" {
		t.Fatalf("count = %q, want 20", query.Get("count"))
	}
	if query.Get("aid") != "386088" {
		t.Fatalf("aid = %q, want 386088", query.Get("aid"))
	}
	if query.Get("device_platform") != "android" {
		t.Fatalf("device_platform = %q, want android", query.Get("device_platform"))
	}
}

func TestSodaBuildImageURLUsesTemplatePrefix(t *testing.T) {
	cover := sodaBuildImageURL(sodaImage{
		Urls:           []string{"https://p3-luna.douyinpic.com/img/"},
		Uri:            "tos-cn-v-2774c002/example",
		TemplatePrefix: "tplv-b829550vbb",
	}, "~c5_300x300.jpg")

	want := "https://p3-luna.douyinpic.com/img/tos-cn-v-2774c002/example~tplv-b829550vbb-resize:960:960.png"
	if cover != want {
		t.Fatalf("cover = %q, want %q", cover, want)
	}
}
