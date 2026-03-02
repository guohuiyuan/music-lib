package main

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
)

// getEnvCookie Helper function to parse .env file from the root
func getEnvCookie(key string) string {
	b, err := os.ReadFile(".env")
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, key+"=") {
			return strings.TrimSpace(strings.TrimPrefix(line, key+"="))
		}
	}
	return ""
}

func TestNeteaseVIPStatusAndDownload(t *testing.T) {
	cookie := getEnvCookie("NETEASE_COOKIE")
	n := netease.New(cookie)

	// Jay Chou's song - usually VIP exclusive
	vipKeyword := "稻香"
	songs, err := n.Search(vipKeyword)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	var vipSongID string
	if len(songs) > 0 {
		vipSongID = songs[0].ID
		fmt.Printf("Found song %s by %s, ID: %s\n", songs[0].Name, songs[0].Artist, vipSongID)
	} else {
		t.Fatalf("No songs found for keyword: %s", vipKeyword)
	}

	isVip, err := n.IsVipAccount()
	if err != nil {
		t.Fatalf("IsVipAccount error: %v", err)
	}
	fmt.Printf("Account IsVip: %v\n", isVip)

	song := &model.Song{
		ID:     vipSongID,
		Source: "netease",
		Extra: map[string]string{
			"song_id": vipSongID,
		},
	}

	url, err := n.GetDownloadURL(song)
	if err != nil {
		t.Logf("GetDownloadURL error (expected without VIP cookie): %v", err)
	} else {
		fmt.Printf("VIP Song %s Download URL: %s\n", vipSongID, url)
	}
}

func TestBilibiliVIPStatusAndDownload(t *testing.T) {
	cookie := getEnvCookie("BILIBILI_COOKIE")
	b := bilibili.New(cookie)

	// 周杰伦 Jay Chou music typically contains Hi-Res Bilibili audios
	vipKeyword := "周杰伦"
	songs, err := b.Search(vipKeyword)
	if err != nil {
		t.Fatalf("Search error: %v", err)
	}

	var vipSongID string
	if len(songs) > 0 {
		vipSongID = songs[0].ID
		fmt.Printf("Found Bilibili video %s by %s, ID: %s\n", songs[0].Name, songs[0].Artist, vipSongID)
	} else {
		t.Fatalf("No songs found for keyword: %s", vipKeyword)
	}

	isVip, err := b.IsVipAccount()
	if err != nil {
		t.Fatalf("IsVipAccount error: %v", err)
	}
	fmt.Printf("Bilibili Account IsVip: %v\n", isVip)

	song := &model.Song{
		ID:     vipSongID,
		Source: "bilibili",
	}

	url, err := b.GetDownloadURL(song)
	if err != nil {
		t.Logf("GetDownloadURL error: %v", err)
	} else {
		fmt.Printf("Bilibili Song %s Download URL: %s\n", vipSongID, url[:120]+"...") // Truncate to keep log clean
	}
}

func TestBilibiliVIPHiRes(t *testing.T) {
	cookie := getEnvCookie("BILIBILI_COOKIE")
	b := bilibili.New(cookie)

	isVip, err := b.IsVipAccount()
	if err != nil {
		t.Fatalf("IsVipAccount error: %v", err)
	}
	fmt.Printf("Bilibili Account IsVip: %v\n", isVip)

	// BV1rp4y1e745 is explicitly designated as having 30251 FLAC/Hi-Res tracks in Bilibili Docs
	song := &model.Song{
		ID:     "BV1rp4y1e745|244954665",
		Source: "bilibili",
	}

	url, err := b.GetDownloadURL(song)
	if err != nil {
		t.Fatalf("Failed to fetch Hi-Res Bilibili stream: %v", err)
	}

	fmt.Printf("Bilibili Hi-Res Stream Download URL: %s\n", url[:120]+"...")
}
