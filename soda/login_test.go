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

func TestSodaQRConnectResultErrorWithMFAShowsSMSFlow(t *testing.T) {
	token := "unit-token-mfa"
	clearSodaQRLoginPending(token)
	var resp sodaQRConnectResponse
	resp.Data.Status = "error"
	resp.Data.ErrorCode = 1105
	resp.Message = "verify required"

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
	result := (&Soda{}).sodaQRConnectResult(token, body, map[string]string{
		"passport_mfa_token": "mfa-token",
	}, resp)

	if result.Status != model.QRLoginStatusScanned {
		t.Fatalf("status = %s, want scanned", result.Status)
	}
	if result.Extra["need_sms"] != "true" {
		t.Fatalf("need_sms not set: %#v", result.Extra)
	}
	if result.Extra["encrypt_uid"] != "encrypted-user" {
		t.Fatalf("encrypt_uid mismatch: %#v", result.Extra)
	}
	if _, ok := getSodaQRLoginPending(token); !ok {
		t.Fatal("MFA pending state was not recorded")
	}
	clearSodaQRLoginPending(token)
}

func TestSodaQRConnectFormMatchesOfficialPCShape(t *testing.T) {
	createQuery := buildSodaQRCreateQuery()
	for _, key := range []string{
		"passport_jssdk_version=2.4.13",
		"passport_jssdk_type=normal",
		"is_from_ttaccountsdk=1",
		"aid=386088",
		"language=zh",
		"account_sdk_source=web",
		"p_js_v=2.4.13",
		"p_js_t=pro",
		"p_zt=3.3.5",
		"p_ver=1.0.29",
		"request_host=app%253A%252F%252Fresources",
		"p_bd=1.0.0.41",
		"is_new_login=1",
		"is_from_iesaccountsaas=1",
		"device_platform=PC",
		"version_code=3.3.0",
		"next=https%3A%2F%2Fapi.qishui.com",
		"need_logo=false",
		"need_short_url=false",
		"is_frontier=true",
	} {
		if !strings.Contains(createQuery, key) {
			t.Fatalf("create query %q missing %s", createQuery, key)
		}
	}

	checkQuery := buildSodaQRCheckQuery()
	for _, key := range []string{
		"passport_jssdk_version=2.4.13",
		"passport_jssdk_type=normal",
		"is_from_ttaccountsdk=1",
		"aid=386088",
		"language=zh",
		"account_sdk_source=web",
		"p_js_v=2.4.13",
		"p_js_t=pro",
		"p_zt=3.3.5",
		"p_ver=1.0.29",
		"request_host=app%253A%252F%252Fresources",
		"p_bd=1.0.0.41",
		"is_new_login=1",
		"is_from_iesaccountsaas=1",
		"device_platform=PC",
		"version_code=3.3.0",
	} {
		if !strings.Contains(checkQuery, key) {
			t.Fatalf("check query %q missing %s", checkQuery, key)
		}
	}

	form := sodaQRConnectForm("token")
	if form.Get("token") != "token" || form.Get("is_frontier") != "true" || form.Get("next") != "https://api.qishui.com" {
		t.Fatalf("unexpected check form: %s", form.Encode())
	}
	if form.Get("aid") != "" || form.Get("passport_jssdk_version") != "" {
		t.Fatalf("check form should not duplicate query params: %s", form.Encode())
	}

	liteQuery := buildSodaPassportLiteQuery()
	for _, key := range []string{
		"passport_jssdk_version=5.1.2",
		"passport_jssdk_type=lite",
		"account_app_language=en-US",
		"new_authn_sdk_version=1.0.0.404-web",
		"did=",
		"iid=",
		"biz_trace_id=",
	} {
		if !strings.Contains(liteQuery, key) {
			t.Fatalf("lite query %q missing %s", liteQuery, key)
		}
	}
}

func containsQueryParam(raw, key string) bool {
	return strings.HasPrefix(raw, key+"=") || strings.Contains(raw, "&"+key+"=")
}
