package migu

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

const testZ3DFileKey = "9A14CB2B6AA5DA30CE036BBD6664373E"

func TestMiguZ3DStreamKey(t *testing.T) {
	key, err := miguZ3DStreamKey(testZ3DFileKey)
	if err != nil {
		t.Fatalf("miguZ3DStreamKey() error = %v", err)
	}
	if string(key) != "397BE2C84E0F983C65ED04FC407C8F99" {
		t.Fatalf("miguZ3DStreamKey() = %q", key)
	}
}

func TestDecryptAudio(t *testing.T) {
	streamKey, err := miguZ3DStreamKey(testZ3DFileKey)
	if err != nil {
		t.Fatalf("miguZ3DStreamKey() error = %v", err)
	}
	plain := []byte("RIFF\x10\x00\x00\x00WAVEfmt ")
	encrypted := make([]byte, len(plain))
	for i, value := range plain {
		encrypted[i] = value + streamKey[i&31]
	}

	decrypted, err := DecryptAudio(encrypted, testZ3DFileKey)
	if err != nil {
		t.Fatalf("DecryptAudio() error = %v", err)
	}
	if !bytes.Equal(decrypted, plain) {
		t.Fatalf("DecryptAudio() = %q, want %q", decrypted, plain)
	}
}

func TestDecryptAudioWithZ3DHeader(t *testing.T) {
	streamKey, err := miguZ3DStreamKey(testZ3DFileKey)
	if err != nil {
		t.Fatalf("miguZ3DStreamKey() error = %v", err)
	}
	plain := []byte("fLaC\x00\x00\x00\x22")
	encrypted := make([]byte, len(plain))
	for i, value := range plain {
		encrypted[i] = value + streamKey[i&31]
	}

	decrypted, err := DecryptAudio(append([]byte("Z3D"), encrypted...), testZ3DFileKey)
	if err != nil {
		t.Fatalf("DecryptAudio() error = %v", err)
	}
	if !bytes.Equal(decrypted, plain) {
		t.Fatalf("DecryptAudio() = %q, want %q", decrypted, plain)
	}
}

func TestDecryptAudioRejectsWrongKey(t *testing.T) {
	if _, err := DecryptAudio([]byte("not encrypted audio"), testZ3DFileKey); err == nil {
		t.Fatal("DecryptAudio() accepted an invalid stream")
	}
}

func TestDownloadInfoFromZ3DResponse(t *testing.T) {
	resp := &miguListenResponse{}
	resp.Data.FormatType = "PQ"
	resp.Data.SongItem.Z3DCode.AndroidURL = "ftp://218.200.160.122:21/public/song/歌曲下载/wav_3d/test.wav"
	resp.Data.SongItem.Z3DCode.AndroidFileKey = testZ3DFileKey
	resp.Data.SongItem.Z3DCode.AndroidFileType = "wav"
	resp.Data.SongItem.Z3DCode.AndroidSize = "48406262"

	info := downloadInfoFromResponse(resp, "Z3D", 0)
	if info == nil {
		t.Fatal("downloadInfoFromResponse() returned nil")
	}
	if info.URL != "https://freetyst.nf.migu.cn/public/song/歌曲下载/wav_3d/test.wav" {
		t.Fatalf("URL = %q", info.URL)
	}
	if !info.Encrypted || info.FileKey != testZ3DFileKey {
		t.Fatalf("Z3D metadata = encrypted:%v key:%q", info.Encrypted, info.FileKey)
	}
	if info.Format != "Z3D" || info.Ext != "wav" || info.Size != 48406262 {
		t.Fatalf("info = %+v", info)
	}
}

func TestMiguSongParts(t *testing.T) {
	song := &model.Song{
		ID: "content|2|HQ",
		Extra: map[string]string{
			"content_id":    "content",
			"resource_type": "2",
			"format_type":   "HQ",
		},
	}
	contentID, resourceType, formatType := miguSongParts(song)
	if contentID != "content" || resourceType != "2" || formatType != "HQ" {
		t.Fatalf("miguSongParts() = %q, %q, %q", contentID, resourceType, formatType)
	}
}

func TestRewriteMiguToneURLToFLAC(t *testing.T) {
	rawURL := "https://freetyst.nf.migu.cn/public/song/%E6%A0%87%E6%B8%85%E9%AB%98%E6%B8%85/MP3_128_16_Stero/123.mp3?Key=test"
	got, ok := rewriteMiguToneURL(rawURL, "SQ")
	if !ok {
		t.Fatal("rewriteMiguToneURL() did not rewrite the URL")
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if !strings.Contains(parsed.Path, "歌曲下载/flac/") || !strings.HasSuffix(parsed.Path, ".flac") {
		t.Fatalf("rewritten path = %q", parsed.Path)
	}
	if parsed.RawQuery != "Key=test" {
		t.Fatalf("rewritten query = %q", parsed.RawQuery)
	}
}

func TestBuildMiguDownloadCandidatesStartsAtHighestQuality(t *testing.T) {
	resp := &miguListenResponse{}
	resp.Data.FormatType = "PQ"
	resp.Data.URL = "https://freetyst.nf.migu.cn/public/song/%E6%A0%87%E6%B8%85%E9%AB%98%E6%B8%85/MP3_128_16_Stero/123.mp3?Key=test"

	candidates := buildMiguDownloadCandidates([]*miguListenResponse{resp}, 0)
	want := []string{"ZQ32", "ZQ24", "ZQ", "SQ", "HQ", "PQ", "LQ"}
	if len(candidates) < len(want) {
		t.Fatalf("candidate count = %d, want at least %d", len(candidates), len(want))
	}
	for i, format := range want {
		if candidates[i].Format != format {
			t.Fatalf("candidate[%d].Format = %q, want %q", i, candidates[i].Format, format)
		}
	}
}

func TestSelectHighestValidDownloadRejectsInvalidAudio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "bytes=0-63" {
			t.Errorf("Range = %q, want bytes=0-63", r.Header.Get("Range"))
		}
		switch r.URL.Path {
		case "/invalid.flac":
			w.Header().Set("Content-Type", "text/html")
			_, _ = io.WriteString(w, "<html>not audio</html>")
		case "/valid.mp3":
			w.Header().Set("Content-Type", "audio/mpeg")
			_, _ = w.Write([]byte("ID3\x04\x00\x00\x00\x00\x00\x00"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	invalid := &DownloadInfo{URL: server.URL + "/invalid.flac", Format: "SQ", Ext: "flac"}
	valid := &DownloadInfo{URL: server.URL + "/valid.mp3", Format: "HQ", Ext: "mp3"}
	if !isMiguAudioMagic([]byte("ID3\x04\x00\x00\x00\x00\x00\x00")) {
		t.Fatal("isMiguAudioMagic() rejected ID3 data")
	}
	if !New("").downloadInfoValid(valid) {
		t.Fatal("downloadInfoValid() rejected valid MP3 data")
	}
	got := New("").selectHighestValidDownload([]*DownloadInfo{invalid, valid})
	if got != valid {
		t.Fatalf("selectHighestValidDownload() = %+v, want valid MP3", got)
	}
	if got.Size != 10 {
		t.Fatalf("valid.Size = %d, want 10", got.Size)
	}
}

func TestSelectHighestValidDownloadDecryptsZ3DProbe(t *testing.T) {
	streamKey, err := miguZ3DStreamKey(testZ3DFileKey)
	if err != nil {
		t.Fatalf("miguZ3DStreamKey() error = %v", err)
	}
	plain := make([]byte, 64)
	copy(plain, []byte("RIFF\x10\x00\x00\x00WAVEfmt "))
	encrypted := make([]byte, len(plain))
	for i, value := range plain {
		encrypted[i] = value + streamKey[i&31]
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(encrypted)
	}))
	defer server.Close()

	info := &DownloadInfo{
		URL:       server.URL + "/encrypted.wav",
		Format:    "Z3D",
		Ext:       "wav",
		Encrypted: true,
		FileKey:   testZ3DFileKey,
	}
	if got := New("").selectHighestValidDownload([]*DownloadInfo{info}); got != info {
		t.Fatalf("selectHighestValidDownload() = %+v, want decrypted Z3D candidate", got)
	}
}

func TestLiveMiguHighestQualityDownload(t *testing.T) {
	cookie := os.Getenv("MIGU_TEST_COOKIE")
	if cookie == "" {
		t.Skip("set MIGU_TEST_COOKIE to run the live Migu download test")
	}

	song := &model.Song{
		Source:  "migu",
		ID:      "600919000009811300|2|Z3D",
		AlbumID: "1138997327",
		Extra: map[string]string{
			"content_id":    "600919000009811300",
			"resource_type": "2",
			"format_type":   "Z3D",
			"copyright_id":  "6005861HZLK",
			"song_id":       "1138997181",
			"album_id":      "1138997327",
		},
	}

	info, err := New(cookie).GetDownloadInfo(song)
	if err != nil {
		t.Fatalf("GetDownloadInfo() error = %v", err)
	}
	if info.URL == "" || info.Format == "" || info.Ext == "" {
		t.Fatalf("unexpected download info: %+v", info)
	}
	if miguFormatRank(info.Format) < miguFormatRank("HQ") {
		t.Fatalf("selected quality %s is below HQ for a track with an accessible HQ stream", info.Format)
	}

	req, err := http.NewRequest(http.MethodGet, info.URL, nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	req.Header.Set("User-Agent", miguAndroidUA)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("download error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d", resp.StatusCode)
	}

	encrypted, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("download read error = %v", err)
	}
	audioData := encrypted
	if info.Encrypted {
		audioData, err = DecryptAudio(encrypted, info.FileKey)
		if err != nil {
			t.Fatalf("DecryptAudio() error = %v", err)
		}
	}
	if !isMiguAudioMagic(audioData) {
		t.Fatalf("downloaded data is not a supported audio stream")
	}
	if info.Size > 0 && int64(len(audioData)) != info.Size {
		t.Fatalf("downloaded size = %d, want %d", len(audioData), info.Size)
	}

	if outputPath := os.Getenv("MIGU_TEST_OUTPUT"); outputPath != "" {
		if err := os.WriteFile(outputPath, audioData, 0644); err != nil {
			t.Fatalf("write output error = %v", err)
		}
	}
}

func TestLiveMiguFLACDownload(t *testing.T) {
	cookie := os.Getenv("MIGU_TEST_COOKIE")
	if cookie == "" {
		t.Skip("set MIGU_TEST_COOKIE to run the live Migu download test")
	}

	song := &model.Song{
		Source: "migu",
		ID:     "600902000006889366|2|HQ",
		Extra: map[string]string{
			"content_id":    "600902000006889366",
			"resource_type": "2",
			"format_type":   "HQ",
			"copyright_id":  "60054701923",
		},
	}

	info, err := New(cookie).GetDownloadInfo(song)
	if err != nil {
		t.Fatalf("GetDownloadInfo() error = %v", err)
	}
	if info.Encrypted || info.Format != "SQ" || info.Ext != "flac" {
		t.Fatalf("unexpected download info: %+v", info)
	}

	req, err := http.NewRequest(http.MethodGet, info.URL, nil)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	req.Header.Set("User-Agent", miguAndroidUA)
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		t.Fatalf("download error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("download status = %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("download read error = %v", err)
	}
	if !bytes.HasPrefix(data, []byte("fLaC")) {
		t.Fatalf("downloaded data is not FLAC")
	}

	if outputPath := os.Getenv("MIGU_TEST_FLAC_OUTPUT"); outputPath != "" {
		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			t.Fatalf("write output error = %v", err)
		}
	}
}
