package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/soda"
)

func getSodaCookie() string {
	if cookie := getEnvCookie("SODA_COOKIE"); strings.TrimSpace(cookie) != "" {
		return strings.TrimSpace(cookie)
	}

	raw, err := os.ReadFile(filepath.Join("..", "qishui.md"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "cookie:") {
			return strings.TrimSpace(line[len("cookie:"):])
		}
	}
	return ""
}

func isMP4Audio(data []byte) bool {
	return len(data) >= 12 && string(data[4:8]) == "ftyp"
}

func TestSodaVIPStatusAndDownload(t *testing.T) {
	cookie := getSodaCookie()
	if cookie == "" {
		t.Skip("SODA_COOKIE or ../qishui.md cookie not set")
	}

	s := soda.New(cookie)
	song, err := s.Parse("https://qishui.douyin.com/s/iQeFw9cE/")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if song.ID != "7304719759323564095" {
		t.Fatalf("song ID = %s, want 7304719759323564095", song.ID)
	}
	if !song.IsVIP {
		t.Fatalf("song should be marked VIP, extra=%#v", song.Extra)
	}
	if song.URL == "" {
		t.Fatal("parsed VIP song should include a cookie-backed download URL")
	}
	if song.Duration < 175 || song.Duration > 185 {
		t.Fatalf("song duration = %d, want about 180 seconds", song.Duration)
	}
	if song.Size < 5*1024*1024 {
		t.Fatalf("download size = %d, looks like a preview stream", song.Size)
	}

	isVip, err := s.IsVipAccount()
	if err != nil {
		t.Fatalf("IsVipAccount error: %v", err)
	}
	if !isVip {
		t.Fatal("Soda account should be detected as VIP for the provided cookie")
	}

	outputPath := filepath.Join(t.TempDir(), "soda-vip.m4a")
	if err := s.Download(song, outputPath); err != nil {
		t.Fatalf("Download error: %v", err)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Read downloaded file error: %v", err)
	}
	if len(data) < 5*1024*1024 {
		t.Fatalf("downloaded file size = %d, looks incomplete", len(data))
	}
	if !isMP4Audio(data) {
		t.Fatalf("downloaded file does not look like an MP4/M4A audio file")
	}
}
