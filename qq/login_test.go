package qq

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCompleteQQMusicLogin(t *testing.T) {
	var authorizeForm url.Values
	var loginPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/check":
			http.SetCookie(w, &http.Cookie{Name: "p_skey", Value: "test-pskey"})
			http.Redirect(w, r, "/login_jump", http.StatusFound)
		case "/login_jump":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte("<script>window.parent.postMessage('ok','https://graph.qq.com')</script>"))
		case "/authorize":
			if r.Method != http.MethodPost {
				t.Errorf("authorize method = %s, want POST", r.Method)
			}
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse authorize form: %v", err)
			}
			authorizeForm = r.PostForm
			http.Redirect(w, r, "/callback?code=oauth-code&state=state", http.StatusFound)
		case "/callback":
			http.SetCookie(w, &http.Cookie{Name: "login_type", Value: "1"})
			_, _ = w.Write([]byte("ok"))
		case "/musicu":
			if err := json.NewDecoder(r.Body).Decode(&loginPayload); err != nil {
				t.Errorf("decode login payload: %v", err)
			}
			http.SetCookie(w, &http.Cookie{Name: "uin", Value: "o12345"})
			http.SetCookie(w, &http.Cookie{Name: "qm_keyst", Value: "music-key"})
			http.SetCookie(w, &http.Cookie{Name: "qqmusic_key", Value: "music-key"})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"req":{"code":0,"data":{}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	cookies, err := completeQQMusicLogin(
		client,
		jar,
		server.URL+"/check",
		server.URL+"/authorize",
		server.URL+"/musicu",
	)
	if err != nil {
		t.Fatal(err)
	}
	if cookies["uin"] != "o12345" {
		t.Fatalf("uin = %q, want o12345", cookies["uin"])
	}
	if cookies["qm_keyst"] != "music-key" || cookies["qqmusic_key"] != "music-key" {
		t.Fatalf("music keys = %q / %q, want music-key", cookies["qm_keyst"], cookies["qqmusic_key"])
	}

	if authorizeForm.Get("client_id") != qqOAuthClientID {
		t.Fatalf("client_id = %q, want %q", authorizeForm.Get("client_id"), qqOAuthClientID)
	}
	if authorizeForm.Get("redirect_uri") == "" {
		t.Fatal("authorize redirect_uri is empty")
	}
	if authorizeForm.Get("g_tk") == "" {
		t.Fatal("authorize g_tk is empty")
	}
	if authorizeForm.Get("ui") == "" {
		t.Fatal("authorize ui is empty")
	}

	req, ok := loginPayload["req"].(map[string]interface{})
	if !ok {
		t.Fatalf("login payload req = %#v", loginPayload["req"])
	}
	if req["module"] != "QQConnectLogin.LoginServer" || req["method"] != "QQLogin" {
		t.Fatalf("unexpected login request: %#v", req)
	}
	param, ok := req["param"].(map[string]interface{})
	if !ok || param["code"] != "oauth-code" {
		t.Fatalf("unexpected login params: %#v", req["param"])
	}
}

func TestNormalizeQQMusicCookiesPrefersMusicKey(t *testing.T) {
	got := normalizeQQMusicCookies(map[string]string{
		"uin":         "o123",
		"qm_keyst":    "W_X_test_key",
		"p_skey":      "",
		"qqmusic_key": "",
	})
	if got["qm_keyst"] != "W_X_test_key" {
		t.Fatalf("qm_keyst = %q", got["qm_keyst"])
	}
	if got["qqmusic_key"] != "W_X_test_key" {
		t.Fatalf("qqmusic_key = %q", got["qqmusic_key"])
	}
	if got["uin"] != "o123" {
		t.Fatalf("uin = %q", got["uin"])
	}
}

func TestMergeNonEmptyCookiesDoesNotEraseValues(t *testing.T) {
	target := map[string]string{"p_skey": "valid"}
	mergeNonEmptyCookies(target, map[string]string{"p_skey": "", "uin": "123"})
	if target["p_skey"] != "valid" {
		t.Fatalf("p_skey = %q, want valid", target["p_skey"])
	}
	if target["uin"] != "123" {
		t.Fatalf("uin = %q, want 123", target["uin"])
	}
}

func TestQQCookieStateRoundTrip(t *testing.T) {
	want := map[string]string{"qrsig": "qr-value", "pt_login_sig": "sig-value", "empty": ""}
	encoded := encodeQQCookieState(want)
	if !strings.HasPrefix(encoded, "qrsig=") && !strings.Contains(encoded, "qrsig%3D") {
		// Raw URL base64 usually starts with the encoded ASCII bytes, but the exact prefix is
		// not part of the contract. Decoding is the important behavior.
		t.Logf("encoded state: %s", encoded)
	}
	got := decodeQQCookieState(encoded)
	if got["qrsig"] != "qr-value" {
		t.Fatalf("qrsig = %q, want qr-value", got["qrsig"])
	}
	if got["pt_login_sig"] != "sig-value" {
		t.Fatalf("pt_login_sig = %q, want sig-value", got["pt_login_sig"])
	}
	if _, exists := got["empty"]; exists {
		t.Fatal("empty cookie should not be persisted")
	}
}
