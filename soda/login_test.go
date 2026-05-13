package soda

import (
	"strings"
	"testing"

	"github.com/guohuiyuan/music-lib/model"
)

func TestExtractSodaMFAFields(t *testing.T) {
	body := []byte(`{
		"data": {
			"encrypt_uid": "encrypted-user",
			"mobile": "182******92",
			"verify": {
				"std_verify_flow_id": "flow.login",
				"std_verify_token": "token_lf",
				"std_verify_scene": "account_login",
				"std_verify_template": "ato",
				"std_verify_type": "MFA"
			}
		}
	}`)

	if got := extractSodaMFAField(body, "encrypt_uid"); got != "encrypted-user" {
		t.Fatalf("encrypt_uid = %q, want encrypted-user", got)
	}
	if got := extractSodaMFAField(body, "mobile"); got != "182******92" {
		t.Fatalf("mobile = %q, want masked mobile", got)
	}

	params := extractSodaMFAVerifyParams(body, map[string]string{"passport_mfa_token": "mfa"})
	for _, key := range []string{"std_verify_flow_id", "std_verify_token", "std_verify_scene", "passport_mfa_token"} {
		if params == "" || !containsQueryParam(params, key) {
			t.Fatalf("verify params %q missing %s", params, key)
		}
	}
}

func TestSodaQRLoginPendingStateMergesCookies(t *testing.T) {
	token := "unit-token"
	clearSodaQRLoginPending(token)
	rememberSodaQRLoginPending(token, sodaQRLoginPendingState{
		Cookies:      map[string]string{"sessionid": "old"},
		EncryptUID:   "uid",
		VerifyParams: "std_verify_token=t",
	})

	state, ok := getSodaQRLoginPending(token)
	if !ok {
		t.Fatal("pending login state not found")
	}
	merged := mergeSodaCookies(state.Cookies, map[string]string{"d_ticket": "ticket"})
	if merged["sessionid"] != "old" || merged["d_ticket"] != "ticket" {
		t.Fatalf("merged cookies mismatch: %#v", merged)
	}
	clearSodaQRLoginPending(token)
}

func TestSodaQRConnectResultConfirmedWithSessionCookieSucceeds(t *testing.T) {
	token := "unit-token"
	var resp sodaQRConnectResponse
	resp.Data.Status = "confirmed"
	resp.Message = "success"

	result := (&Soda{}).sodaQRConnectResult(token, []byte(`{"data":{"status":"confirmed"},"message":"success"}`), map[string]string{
		"sessionid": "sid",
		"sid_tt":    "sid",
	}, resp)

	if result.Status != model.QRLoginStatusSuccess {
		t.Fatalf("status = %s, want success", result.Status)
	}
	if !strings.Contains(result.Cookie, "sessionid=sid") {
		t.Fatalf("cookie missing sessionid: %q", result.Cookie)
	}
}

func TestSodaQRConnectResultErrorIncludesAPIStatus(t *testing.T) {
	var resp sodaQRConnectResponse
	resp.Data.Status = "error"
	resp.Data.ErrorCode = 16
	resp.Message = "error"

	result := (&Soda{}).sodaQRConnectResult("unit-token", nil, nil, resp)
	if result.Status != model.QRLoginStatusFailed {
		t.Fatalf("status = %s, want failed", result.Status)
	}
	if !strings.Contains(result.Message, "code=16") {
		t.Fatalf("message should include error code, got %q", result.Message)
	}
	if result.Extra["api_status"] != "error" || result.Extra["error_code"] != "16" {
		t.Fatalf("extra mismatch: %#v", result.Extra)
	}
}

func containsQueryParam(raw, key string) bool {
	return strings.HasPrefix(raw, key+"=") || strings.Contains(raw, "&"+key+"=")
}
