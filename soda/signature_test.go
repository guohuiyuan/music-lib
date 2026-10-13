package soda

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSodaPCAppParamsWithCredentials(t *testing.T) {
	params := sodaPCAppParamsWith(&AppCredentials{
		DeviceID: "1022300354942847",
		IID:      "8842030934094602",
	})
	if got := params.Get("device_id"); got != "1022300354942847" {
		t.Fatalf("device_id = %q", got)
	}
	if got := params.Get("iid"); got != "8842030934094602" {
		t.Fatalf("iid = %q", got)
	}
	if got := params.Get("fp"); got != "1022300354942847" {
		t.Fatalf("fp 缺省应回退到 device_id，得到 %q", got)
	}
	if got := params.Get("app_name"); got != "luna_pc" {
		t.Fatalf("app_name = %q", got)
	}

	fallback := sodaPCAppParams()
	if fallback.Get("device_id") == "" || fallback.Get("device_id") == params.Get("device_id") {
		t.Fatalf("未配置指纹时应使用临时 device_id，得到 %q", fallback.Get("device_id"))
	}
}

func TestSodaMD5HexUpper(t *testing.T) {
	got := sodaMD5HexUpper([]byte(`{"track_id":"1"}`))
	const want = "3ECAA8B28B2DB2D40D2456CA3DAD152A"
	if got != want {
		t.Fatalf("X-SS-STUB = %q, want %q（body 的 MD5 大写）", got, want)
	}
}

func TestParseSodaSignResponse(t *testing.T) {
	nested := []byte(`{"ok":true,"headers":{"X-Helios":"h1","X-Medusa":"m1"},"ms_token":"tok","a_bogus":"bog","device_id":"123"}`)
	parsed, err := parseSodaSignResponse(nested)
	if err != nil {
		t.Fatalf("nested parse error: %v", err)
	}
	if parsed.Headers["X-Helios"] != "h1" || parsed.Headers["X-Medusa"] != "m1" {
		t.Fatalf("headers = %#v", parsed.Headers)
	}
	if parsed.MSToken != "tok" || parsed.DeviceID != "123" {
		t.Fatalf("parsed = %#v", parsed)
	}

	flat := []byte(`{"ok":true,"X-Helios":"h2","X-Medusa":"m2"}`)
	parsed, err = parseSodaSignResponse(flat)
	if err != nil {
		t.Fatalf("flat parse error: %v", err)
	}
	if parsed.Headers["x-helios"] != "h2" || parsed.Headers["x-medusa"] != "m2" {
		t.Fatalf("flat headers = %#v", parsed.Headers)
	}

	failed := []byte(`{"ok":false,"error":"unauthorized: missing token"}`)
	if _, err := parseSodaSignResponse(failed); err == nil || !strings.Contains(err.Error(), SignerEnvToken) {
		t.Fatalf("鉴权失败应给出可操作提示，得到 %v", err)
	}
}

func TestSignStreamRequestSendsFullRequestAndMergesHeaders(t *testing.T) {
	var received sodaSignRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer token-1" {
			t.Errorf("Authorization = %q", got)
		}
		raw, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(raw, &received); err != nil {
			t.Errorf("signer request parse error: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"ms_token":"ms-1","headers":{"X-Helios":"helios-1","X-Medusa":"medusa-1"}}`))
	}))
	defer server.Close()

	s := NewWithSigner("sessionid_ss=test", server.URL+"/sign", "token-1")
	headers := map[string]string{
		"User-Agent":   pcAppUserAgent,
		"X-SS-STUB":    "ABCD",
		"Cookie":       "sessionid_ss=test",
		"Content-Type": "application/json; charset=utf-8",
	}
	signedURL, extra, err := s.signStreamRequest("https://api.qishui.com/luna/pc/track_v2?aid=386088", `{"track_id":"1"}`, headers)
	if err != nil {
		t.Fatalf("signStreamRequest error: %v", err)
	}

	if received.Method != "POST" || !strings.Contains(received.URL, "aid=386088") {
		t.Fatalf("signer request = %#v", received)
	}
	if received.Headers["Cookie"] != "sessionid_ss=test" || received.Headers["X-SS-STUB"] != "ABCD" {
		t.Fatalf("签名器必须看到完整请求头，得到 %#v", received.Headers)
	}
	if received.TSMs <= 0 {
		t.Fatalf("ts_ms = %d", received.TSMs)
	}
	if extra["X-Helios"] != "helios-1" || extra["X-Medusa"] != "medusa-1" {
		t.Fatalf("extra headers = %#v", extra)
	}
	parsed, err := url.Parse(signedURL)
	if err != nil {
		t.Fatalf("signed url parse error: %v", err)
	}
	if parsed.Query().Get("msToken") != "ms-1" || parsed.Query().Get("aid") != "386088" {
		t.Fatalf("signed url = %s", signedURL)
	}
}

func TestSignStreamRequestWithoutSignerKeepsRequestUntouched(t *testing.T) {
	s := New("sessionid_ss=test")
	s.ClearSigner()
	s.ClearLocalSigner() // 本机有客户端时也一并关掉，才代表"完全没有签名器"
	gotURL, extra, err := s.signStreamRequest("https://api.qishui.com/x", "{}", map[string]string{"A": "b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotURL != "https://api.qishui.com/x" || len(extra) != 0 {
		t.Fatalf("无签名器时不应改动请求：%s %#v", gotURL, extra)
	}
}

func TestFetchSignerIdentityFromConfig(t *testing.T) {
	var configPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != configPath {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"deviceId":"102***847","identity":{"device_id":"1022300354942847","iid":"8842030934094602","fp":"1022300354942847"}}`))
	}))
	defer server.Close()
	configPath = "/config"

	s := NewWithSigner("sessionid_ss=test", server.URL+"/sign", "")
	s.ClearLocalSigner() // 只留远程签名器，验证 /config 身份发现
	creds, err := s.ensureAppCredentials()
	if err != nil {
		t.Fatalf("ensureAppCredentials error: %v", err)
	}
	if creds.DeviceID != "1022300354942847" || creds.IID != "8842030934094602" {
		t.Fatalf("creds = %#v", creds)
	}
	// 第二次应命中缓存（关掉服务也能用）。
	server.Close()
	creds, err = s.ensureAppCredentials()
	if err != nil || creds.DeviceID != "1022300354942847" {
		t.Fatalf("缓存失效: %#v err=%v", creds, err)
	}
}

func TestNewReadsSignerFromEnv(t *testing.T) {
	t.Setenv(SignerEnvURL, "http://127.0.0.1:9/sign")
	t.Setenv(SignerEnvToken, "env-token")
	s := New("cookie")
	if !s.HasSigner() || s.signer.Token != "env-token" {
		t.Fatalf("环境变量签名器未生效: %#v", s.signer)
	}
	s.ClearSigner()
	if s.hasRemoteSigner() {
		t.Fatal("ClearSigner 未生效")
	}
}

// readSodaCookie 与仓库根目录测试一致：优先环境变量，其次 music-lib/.env。
func readSodaCookie() string {
	if value := strings.TrimSpace(os.Getenv("SODA_COOKIE")); value != "" {
		return value
	}
	raw, err := os.ReadFile(filepath.Join("..", ".env"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "SODA_COOKIE=") {
			return strings.TrimSpace(strings.TrimPrefix(line, "SODA_COOKIE="))
		}
	}
	return ""
}

// TestSodaSignedPCStreamNetwork 打真实签名器 + 真实汽水接口：
// 有签名时 App 端点返回非空 track_player（缺签名是 HTTP 200 + 0 字节）。
func TestSodaSignedPCStreamNetwork(t *testing.T) {
	if os.Getenv("SODA_SKIP_NETWORK") != "" {
		t.Skip("SODA_SKIP_NETWORK set")
	}
	signerURL := strings.TrimSpace(os.Getenv(SignerEnvURL))
	if signerURL == "" && !LocalSignerAvailable() {
		t.Skip("QISHUI_SIGNER_URL not set and no local signer (bdms.node) available")
	}
	cookie := readSodaCookie()
	if cookie == "" {
		t.Skip("SODA_COOKIE not set")
	}

	s := NewWithSigner(cookie, signerURL, strings.TrimSpace(os.Getenv(SignerEnvToken)))
	if signerURL == "" {
		t.Logf("使用本机签名器（进程内调 bdms.node）")
	}
	creds, err := s.ensureAppCredentials()
	if err != nil {
		t.Fatalf("signer identity error: %v", err)
	}
	t.Logf("signer device_id=%s iid=%s", creds.DeviceID, creds.IID)

	const trackID = "7501674235158431760" // 下完这场雨（会员曲目）
	resp, err := s.fetchPCTrackV2(trackID)
	if err != nil {
		t.Fatalf("signed pc track_v2 error: %v", err)
	}
	if strings.TrimSpace(resp.Track.ID) == "" && strings.TrimSpace(resp.TrackInfo.ID) == "" {
		t.Fatalf("signed pc track_v2 missing track: %#v", resp)
	}
	info, ok := sodaBestFromVideoModel(resp.TrackPlayer.VideoModel)
	if !ok {
		t.Fatalf("signed pc track_v2 missing video model")
	}
	t.Logf("signed stream: quality=%q format=%q bitrate=%d size=%d duration=%.1fs preview=%v",
		info.Quality, info.Format, info.Bitrate, info.Size, info.Duration,
		sodaDownloadInfoIsPreview(info, sodaTrackDurationSeconds(resp.primaryTrack())))

	// 端到端：拉流 → play_auth 解密 → 产出可播放 m4a。
	streamReq, err := http.NewRequest(http.MethodGet, info.URL, nil)
	if err != nil {
		t.Fatalf("stream request error: %v", err)
	}
	streamReq.Header.Set("User-Agent", UserAgent)
	streamResp, err := (&http.Client{Timeout: 60 * time.Second}).Do(streamReq)
	if err != nil {
		t.Fatalf("stream download error: %v", err)
	}
	defer streamResp.Body.Close()
	encrypted, err := io.ReadAll(streamResp.Body)
	if err != nil {
		t.Fatalf("stream read error: %v", err)
	}
	if streamResp.StatusCode != http.StatusOK {
		t.Fatalf("stream status = %d", streamResp.StatusCode)
	}
	decrypted := encrypted
	if strings.TrimSpace(info.PlayAuth) != "" {
		decrypted, err = DecryptAudio(encrypted, info.PlayAuth)
		if err != nil {
			t.Fatalf("DecryptAudio error: %v", err)
		}
	}
	if len(decrypted) < 12 || string(decrypted[4:8]) != "ftyp" {
		head := decrypted
		if len(head) > 12 {
			head = head[:12]
		}
		t.Fatalf("解密结果不是 MP4（前 12 字节 %x）", head)
	}
	outPath := filepath.Join(t.TempDir(), "signed.m4a")
	if err := os.WriteFile(outPath, decrypted, 0o644); err != nil {
		t.Fatalf("write error: %v", err)
	}
	t.Logf("decrypted m4a: %s (%d bytes) → %s", outPath, len(decrypted), outPath)
}
