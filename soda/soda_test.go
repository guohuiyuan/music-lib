package soda

import "testing"

func TestSodaExtractTrackIDFromText(t *testing.T) {
	const id = "7304719759323564095"

	cases := []string{
		id,
		"https://www.qishui.com/track/" + id,
		"https://music.douyin.com/qishui/share/track?track_id=" + id + "&auto_play_bgm=1",
		"https://www.douyin.com/qishui/song/" + id,
		`_ROUTER_DATA = {"loaderData":{"track_page":{"track_id":"` + id + `"}}}`,
		"https%3A%2F%2Fmusic.douyin.com%2Fqishui%2Fshare%2Ftrack%3Ftrack_id%3D" + id,
	}

	for _, tc := range cases {
		if got := sodaExtractTrackIDFromText(tc); got != id {
			t.Fatalf("sodaExtractTrackIDFromText(%q) = %q, want %q", tc, got, id)
		}
	}
}

func TestSodaExtractPlaylistIDFromText(t *testing.T) {
	const id = "7291667294287183907"

	cases := []string{
		id,
		"https://www.qishui.com/playlist/" + id,
		"https://music.douyin.com/qishui/share/playlist?playlist_id=" + id + "&auto_play_bgm=1",
		`_ROUTER_DATA = {"loaderData":{"playlist_page":{"playlist_id":"` + id + `"}}}`,
		"https%3A%2F%2Fmusic.douyin.com%2Fqishui%2Fshare%2Fplaylist%3Fplaylist_id%3D" + id,
	}

	for _, tc := range cases {
		if got := sodaExtractPlaylistIDFromText(tc); got != id {
			t.Fatalf("sodaExtractPlaylistIDFromText(%q) = %q, want %q", tc, got, id)
		}
	}
}

func TestSodaLabelInfoIsVIP(t *testing.T) {
	if (sodaLabelInfo{}).IsVIP() {
		t.Fatal("empty label info should not be VIP")
	}

	label := sodaLabelInfo{
		QualityMap: map[string]sodaQualityPolicy{
			"lossless": {
				PlayDetail: &sodaQualityBenefit{NeedVIP: true},
			},
		},
	}
	if !label.IsVIP() {
		t.Fatal("quality map with need_vip should be VIP")
	}

	label = sodaLabelInfo{OnlyVIPDownload: true}
	if !label.IsVIP() {
		t.Fatal("only_vip_download should be VIP")
	}
}

func TestSodaBuildSongFromTrackMarksVIP(t *testing.T) {
	song := sodaBuildSongFromTrack(sodaTrack{
		ID:       "7304719759323564095",
		Name:     "落了白",
		Duration: 180822,
		Artists:  []sodaArtist{{Name: "蒋雪儿Snow.J"}},
		Album: sodaAlbum{
			ID:   "1",
			Name: "落了白",
		},
		BitRates:  []sodaBitRate{{Size: 5882690, Quality: "highest"}},
		LabelInfo: sodaLabelInfo{OnlyVIPDownload: true},
	})

	if !song.IsVIP {
		t.Fatal("song should be marked VIP")
	}
	if song.Extra["is_vip"] != "true" || song.Extra["only_vip_download"] != "true" {
		t.Fatalf("missing VIP extra flags: %#v", song.Extra)
	}
	if song.Duration != 180 {
		t.Fatalf("duration = %d, want 180", song.Duration)
	}
}

func TestSodaDownloadInfoIsPreview(t *testing.T) {
	if !sodaDownloadInfoIsPreview(&DownloadInfo{Duration: 60}, 180) {
		t.Fatal("60-second stream should be treated as preview for a 180-second track")
	}
	if sodaDownloadInfoIsPreview(&DownloadInfo{Duration: 178}, 180) {
		t.Fatal("near full-duration stream should not be treated as preview")
	}
}
