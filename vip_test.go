package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/bilibili"
	"github.com/guohuiyuan/music-lib/kugou"
	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/netease"
	"github.com/guohuiyuan/music-lib/qq"
	"github.com/guohuiyuan/music-lib/utils"
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

func isStandardFLAC(data []byte) bool {
	return len(data) >= 4 && bytes.Equal(data[:4], []byte{'f', 'L', 'a', 'C'})
}

func saveTempFLAC(prefix string, data []byte) (string, error) {
	f, err := os.CreateTemp("", prefix+"-*.flac")
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return "", err
	}

	return f.Name(), nil
}

func TestNeteaseVIPStatusAndDownload(t *testing.T) {
	cookie := getEnvCookie("NETEASE_COOKIE")
	n := netease.New(cookie)

	parsedSong, err := n.Parse("https://music.163.com/#/song?id=29732995")
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if parsedSong == nil {
		t.Fatalf("Parse returned nil song")
	}
	fmt.Printf("Parsed song %s by %s, ID: %s\n", parsedSong.Name, parsedSong.Artist, parsedSong.ID)

	if parsedSong.URL == "" {
		downloadURL, getErr := n.GetDownloadURL(parsedSong)
		if getErr != nil {
			t.Fatalf("GetDownloadURL error: %v", getErr)
		}
		parsedSong.URL = downloadURL
	}

	isVip, err := n.IsVipAccount()
	if err != nil {
		t.Fatalf("IsVipAccount error: %v", err)
	}
	fmt.Printf("Account IsVip: %v\n", isVip)

	audioData, err := utils.Get(
		parsedSong.URL,
		utils.WithHeader("Referer", netease.Referer),
		utils.WithHeader("Cookie", cookie),
	)
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}
	if len(audioData) == 0 {
		t.Fatalf("Downloaded audio is empty")
	}
	fmt.Printf("Downloaded %d bytes from parsed URL\n", len(audioData))
	fmt.Printf("Raw header check: standard FLAC=%v (first4=%q)\n", isStandardFLAC(audioData), string(audioData[:4]))

	if isStandardFLAC(audioData) {
		flacPath, saveErr := saveTempFLAC("netease-raw", audioData)
		if saveErr != nil {
			t.Fatalf("Save temp FLAC error: %v", saveErr)
		}
		fmt.Printf("Raw FLAC saved to: %s\n", flacPath)
		fmt.Printf("Raw data is already standard FLAC, skip decrypt\n")
		return
	}

	if len(audioData) < 8 || string(audioData[:8]) != "CTENFDAM" {
		t.Logf("Raw data is not standard FLAC and does not match NCM header, skip decrypt")
		return
	}

	decrypted, outExt, err := netease.DecryptNCM(audioData)
	if err != nil {
		t.Fatalf("Decrypt NCM error: %v", err)
	}

	if len(decrypted) == 0 {
		t.Fatalf("Decrypted audio is empty")
	}
	fmt.Printf("Decrypted %d bytes, source=netease, ext=%s\n", len(decrypted), outExt)
	fmt.Printf("Decrypted header check: standard FLAC=%v (first4=%q)\n", isStandardFLAC(decrypted), string(decrypted[:4]))

	flacPath, saveErr := saveTempFLAC("netease-decrypted", decrypted)
	if saveErr != nil {
		t.Fatalf("Save decrypted temp FLAC error: %v", saveErr)
	}
	fmt.Printf("Decrypted audio saved to: %s\n", flacPath)
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

func TestQQVIPStatusAndDownload(t *testing.T) {
	cookie := getEnvCookie("QQ_COOKIE")
	q := qq.New(cookie)

	isVip, err := q.IsVipAccount()
	if err != nil {
		t.Fatalf("IsVipAccount error: %v", err)
	}
	fmt.Printf("QQ Account IsVip: %v\n", isVip)

	// 周杰伦 - 晴天 (Known VIP Track)
	song := &model.Song{
		ID:     "0039MnYb0qxYhV",
		Source: "qq",
	}

	url, err := q.GetDownloadURL(song)
	if err != nil {
		t.Fatalf("Failed to fetch QQ stream: %v", err)
	}

	fmt.Printf("QQ Stream Download URL: %s\n", url[:120]+"...")
}

func TestKugouVIPStatusAndDownload(t *testing.T) {
	cookie := getEnvCookie("KUGOU_COOKIE")
	if cookie == "" {
		t.Skip("KUGOU_COOKIE not set")
	}
	k := kugou.New(cookie)

	songs, err := k.Search("邓紫棋")
	if err != nil {
		t.Skipf("Search error: %v", err)
	}
	if len(songs) == 0 {
		t.Skip("No songs found for keyword")
	}

	song := songs[0]
	fmt.Printf("Found Kugou song %s by %s, ID: %s\n", song.Name, song.Artist, song.ID)
	fmt.Printf("Candidate hashes: sq=%s, hq=%s, hash=%s\n", song.Extra["sq_hash"], song.Extra["hq_hash"], song.Extra["hash"])

	url, err := k.GetDownloadURL(&song)
	if err != nil {
		t.Fatalf("GetDownloadURL error: %v", err)
	}

	isVip, err := k.IsVipAccount()
	if err != nil {
		t.Fatalf("IsVipAccount error: %v", err)
	}
	fmt.Printf("Kugou Account IsVip: %v\n", isVip)
	fmt.Printf("Kugou Download URL: %s\n", url[:min(120, len(url))])

	audioData, err := utils.Get(
		url,
		utils.WithHeader("Referer", "https://www.kugou.com/"),
		utils.WithHeader("Cookie", cookie),
	)
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}
	if len(audioData) == 0 {
		t.Fatalf("Downloaded audio is empty")
	}

	ext := song.Ext
	if ext == "" {
		if strings.Contains(strings.ToLower(url), ".flac") {
			ext = "flac"
		} else {
			ext = "mp3"
		}
	}
	fmt.Printf("Downloaded %d bytes, ext=%s, standard FLAC=%v\n", len(audioData), ext, isStandardFLAC(audioData))
}
