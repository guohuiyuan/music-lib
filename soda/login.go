package soda

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	sodaQRCreateAPI   = "https://api.qishui.com/passport/web/get_qrcode/"
	sodaQRCheckAPI    = "https://api.qishui.com/passport/web/check_qrconnect/"
	sodaSendCodeAPI   = "https://api.qishui.com/passport/web/send_code/"
	sodaValidateAPI   = "https://api.qishui.com/passport/web/validate_code/"
	sodaPassportUA    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) SodaMusic/3.1.0 Chrome/136.0.7103.59 Electron/36.4.0-rs.22.release.main.1 TTElectron/36.4.0-rs.22.release.main.1 Safari/537.36"
	sodaPassportJSVer = "2.4.13"
)

type sodaQRLoginPendingState struct {
	Cookies      map[string]string
	EncryptUID   string
	VerifyParams string
	ExpiresAt    time.Time
}

type sodaQRConnectResponse struct {
	Data struct {
		Status      string `json:"status"`
		ErrorCode   int    `json:"error_code"`
		Redirect    string `json:"redirect_url"`
		Description string `json:"description"`
		UserData    struct {
			Mobile string `json:"mobile"`
		} `json:"user_data"`
	} `json:"data"`
	Message string `json:"message"`
}

var (
	sodaQRLoginMu      sync.Mutex
	sodaQRLoginPending = map[string]sodaQRLoginPendingState{}
)

func CreateQRLogin() (*model.QRLoginSession, error) { return defaultSoda.CreateQRLogin() }
func CheckQRLogin(key string) (*model.QRLoginResult, error) {
	return defaultSoda.CheckQRLogin(key)
}

// CreateQRLogin creates a QR login session for Soda.
func (s *Soda) CreateQRLogin() (*model.QRLoginSession, error) {
	cleanupSodaQRLoginPending()

	params := url.Values{}
	params.Set("next", "https://api.qishui.com")
	params.Set("need_logo", "false")
	params.Set("need_short_url", "false")
	params.Set("is_new_login", "1")

	body, err := utils.Get(sodaQRCreateAPI+"?"+buildSodaPassportQuery()+"&"+params.Encode(),
		utils.WithHeader("User-Agent", sodaPassportUA),
		utils.WithHeader("Accept", "application/json, text/javascript"),
	)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Token    string `json:"token"`
			QRCode   string `json:"qrcode"`
			WebURL   string `json:"web_url"`
			QRCodeB  string `json:"qrcode_index_url"`
			Frontier bool   `json:"is_frontier"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda qr create json error: %w", err)
	}
	if resp.Data.Token == "" {
		return nil, fmt.Errorf("soda qr create failed: %s", resp.Message)
	}

	scanURL := sodaScanLoginURL(resp.Data.Token)
	qrURL := scanURL
	if qrURL == "" {
		qrURL = resp.Data.WebURL
	}
	if qrURL == "" {
		qrURL = resp.Data.QRCodeB
	}

	return &model.QRLoginSession{
		Source:    "soda",
		Key:       resp.Data.Token,
		URL:       qrURL,
		ImageURL:  resp.Data.QRCode,
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
		Extra: map[string]string{
			"token":             resp.Data.Token,
			"qrcode_index_url":  resp.Data.QRCodeB,
			"scan_login_url":    scanURL,
			"is_frontier":       strconvBool(resp.Data.Frontier),
			"raw_qrcode_image":  strconvBool(resp.Data.QRCode != ""),
			"display_qr_source": "passport_qrcode",
		},
	}, nil
}

// CheckQRLogin checks the QR scan status and handles the MFA SMS flow.
// The key format is: "token" for initial check, "token|send_code|encrypt_uid|verify_params" for send_code,
// "token|validate|encrypt_uid|verify_params|code" for validate.
func (s *Soda) CheckQRLogin(key string) (*model.QRLoginResult, error) {
	parts := strings.SplitN(key, "|", 5)

	switch {
	case len(parts) >= 5 && parts[1] == "validate":
		return s.sodaValidateCode(parts[0], parts[2], parts[3], parts[4])
	case len(parts) >= 4 && parts[1] == "send_code":
		return s.sodaSendCode(parts[0], parts[2], parts[3])
	default:
		return s.sodaCheckQRConnect(parts[0])
	}
}

func (s *Soda) sodaCheckQRConnect(token string) (*model.QRLoginResult, error) {
	token = strings.TrimSpace(token)
	params := url.Values{}
	params.Set("token", token)
	params.Set("need_logo", "false")
	params.Set("need_short_url", "false")
	params.Set("is_frontier", "true")
	params.Set("is_new_login", "1")
	params.Set("next", "https://api.qishui.com")
	params.Set("aid", "386088")
	params.Set("passport_jssdk_version", sodaPassportJSVer)
	params.Set("passport_jssdk_type", "normal")
	params.Set("device_platform", "PC")
	params.Set("version_code", "3.3.0")

	apiURL := sodaQRCheckAPI + "?" + buildSodaPassportQuery()

	body, cookies, err := s.postSodaPassport(apiURL, params)
	if err != nil {
		return nil, err
	}

	var resp sodaQRConnectResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda qr check json error: %w", err)
	}

	return s.sodaQRConnectResult(token, body, cookies, resp), nil
}

func (s *Soda) sodaQRConnectResult(token string, body []byte, cookies map[string]string, resp sodaQRConnectResponse) *model.QRLoginResult {
	result := &model.QRLoginResult{
		Source:  "soda",
		Key:     token,
		Message: resp.Message,
	}

	if sodaCookiesHaveSession(cookies) {
		cookie := sodaJoinCookies(cookies)
		result.Status = model.QRLoginStatusSuccess
		result.Cookie = cookie
		result.Cookies = cookies
		result.Message = "登录成功"
		s.cookie = cookie
		s.isVipCache = nil
		clearSodaQRLoginPending(token)
		return result
	}

	switch strings.ToLower(strings.TrimSpace(resp.Data.Status)) {
	case "confirmed":
		// QR scanned and confirmed, but MFA required - need to send SMS code
		mfaToken := extractSodaCookieValue(cookies, "passport_mfa_token")
		encryptUID := extractSodaMFAField(body, "encrypt_uid")
		verifyParams := extractSodaMFAVerifyParams(body, cookies)

		if mfaToken != "" || encryptUID != "" || verifyParams != "" {
			rememberSodaQRLoginPending(token, sodaQRLoginPendingState{
				Cookies:      cookies,
				EncryptUID:   encryptUID,
				VerifyParams: verifyParams,
				ExpiresAt:    time.Now().Add(10 * time.Minute),
			})
			result.Status = model.QRLoginStatusScanned
			result.Message = "扫码成功，需要短信验证"
			mobile := extractSodaMFAField(body, "mobile")
			if mobile == "" {
				mobile = strings.TrimSpace(resp.Data.UserData.Mobile)
			}
			result.Extra = map[string]string{
				"need_sms":       "true",
				"encrypt_uid":    encryptUID,
				"verify_params":  verifyParams,
				"mfa_token":      mfaToken,
				"mobile":         mobile,
				"cookie_pending": strconvBool(sodaCookiesHaveSession(cookies)),
			}
			return result
		}
		result.Status = model.QRLoginStatusScanned
		result.Message = "已扫码确认，等待登录结果"
	case "new", "":
		result.Status = model.QRLoginStatusWaiting
	case "scanned":
		result.Status = model.QRLoginStatusScanned
	case "expired":
		result.Status = model.QRLoginStatusExpired
	case "error", "failed":
		result.Status = model.QRLoginStatusFailed
		result.Message = sodaQRConnectErrorMessage(resp)
		result.Extra = sodaQRConnectExtra(resp)
	default:
		if resp.Data.ErrorCode != 0 {
			result.Status = model.QRLoginStatusFailed
			result.Message = sodaQRConnectErrorMessage(resp)
			result.Extra = sodaQRConnectExtra(resp)
		} else {
			result.Status = model.QRLoginStatusWaiting
		}
	}

	return result
}

func (s *Soda) sodaSendCode(token, encryptUID, verifyParams string) (*model.QRLoginResult, error) {
	pending, hasPending := getSodaQRLoginPending(token)
	if strings.TrimSpace(encryptUID) == "" {
		encryptUID = pending.EncryptUID
	}
	if strings.TrimSpace(verifyParams) == "" {
		verifyParams = pending.VerifyParams
	}
	if strings.TrimSpace(encryptUID) == "" {
		return &model.QRLoginResult{
			Source:  "soda",
			Key:     token,
			Status:  model.QRLoginStatusFailed,
			Message: "缺少短信验证参数，请刷新二维码重试",
		}, nil
	}

	params := url.Values{}
	params.Set("mix_mode", "1")
	params.Set("type", "3737")
	params.Set("encrypt_uid", encryptUID)
	params.Set("verify_ticket", "")
	params.Set("copywriting_key", "qr_connect")
	params.Set("ies_safety_diversion_tag", "mfa")
	params.Set("new_verify_flow", "")
	params.Set("std_verify_way", "mobile_sms_verify")
	params.Set("is6Digits", "1")
	params.Set("aid", "386088")
	params.Set("new_authn_sdk_version", "1.0.0.404-web")

	// Parse and add verify params
	if verifyParams != "" {
		vp, _ := url.ParseQuery(verifyParams)
		for k, vs := range vp {
			if len(vs) > 0 {
				params.Set(k, vs[0])
			}
		}
	}

	apiURL := sodaSendCodeAPI + "?" + buildSodaPassportLiteQuery()

	body, _, err := s.postSodaPassportWithCookie(apiURL, params, sodaCookieHeader(s.cookie, pending.Cookies))
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Mobile    string `json:"mobile"`
			RetryTime int    `json:"retry_time"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda send_code json error: %w", err)
	}

	result := &model.QRLoginResult{
		Source:  "soda",
		Key:     token,
		Status:  model.QRLoginStatusScanned,
		Message: fmt.Sprintf("验证码已发送至 %s", resp.Data.Mobile),
		Extra: map[string]string{
			"need_sms":      "true",
			"need_sms_code": "true",
			"mobile":        resp.Data.Mobile,
			"encrypt_uid":   encryptUID,
			"verify_params": verifyParams,
			"retry_time":    fmt.Sprintf("%d", resp.Data.RetryTime),
		},
	}
	if hasPending {
		pending.EncryptUID = encryptUID
		pending.VerifyParams = verifyParams
		rememberSodaQRLoginPending(token, pending)
	}
	return result, nil
}

func (s *Soda) sodaValidateCode(token, encryptUID, verifyParams, code string) (*model.QRLoginResult, error) {
	pending, _ := getSodaQRLoginPending(token)
	if strings.TrimSpace(encryptUID) == "" {
		encryptUID = pending.EncryptUID
	}
	if strings.TrimSpace(verifyParams) == "" {
		verifyParams = pending.VerifyParams
	}
	if strings.TrimSpace(encryptUID) == "" {
		return &model.QRLoginResult{
			Source:  "soda",
			Key:     token,
			Status:  model.QRLoginStatusFailed,
			Message: "缺少短信验证参数，请刷新二维码重试",
		}, nil
	}

	params := url.Values{}
	params.Set("mix_mode", "1")
	params.Set("type", "3737")
	params.Set("encrypt_uid", encryptUID)
	params.Set("verify_ticket", "")
	params.Set("copywriting_key", "qr_connect")
	params.Set("ies_safety_diversion_tag", "mfa")
	params.Set("new_verify_flow", "")
	params.Set("std_verify_way", "mobile_sms_verify")
	params.Set("code", code)
	params.Set("aid", "386088")
	params.Set("new_authn_sdk_version", "1.0.0.404-web")

	if verifyParams != "" {
		vp, _ := url.ParseQuery(verifyParams)
		for k, vs := range vp {
			if len(vs) > 0 {
				params.Set(k, vs[0])
			}
		}
	}

	apiURL := sodaValidateAPI + "?" + buildSodaPassportLiteQuery()

	body, cookies, err := s.postSodaPassportWithCookie(apiURL, params, sodaCookieHeader(s.cookie, pending.Cookies))
	if err != nil {
		return nil, err
	}

	var resp struct {
		Data struct {
			Ticket string `json:"ticket"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("soda validate_code json error: %w", err)
	}

	if resp.Data.Ticket == "" && resp.Message != "success" {
		return &model.QRLoginResult{
			Source:  "soda",
			Key:     token,
			Status:  model.QRLoginStatusFailed,
			Message: "验证码错误: " + resp.Message,
		}, nil
	}

	// Success - collect cookies
	mergedCookies := mergeSodaCookies(pending.Cookies, cookies)
	cookie := sodaJoinCookies(mergedCookies)
	result := &model.QRLoginResult{
		Source:  "soda",
		Key:     token,
		Status:  model.QRLoginStatusSuccess,
		Cookie:  cookie,
		Cookies: mergedCookies,
		Message: "登录成功",
	}
	if cookie != "" {
		s.cookie = cookie
		s.isVipCache = nil
	}
	clearSodaQRLoginPending(token)
	return result, nil
}

func (s *Soda) postSodaPassport(apiURL string, form url.Values) ([]byte, map[string]string, error) {
	return s.postSodaPassportWithCookie(apiURL, form, s.cookie)
}

func (s *Soda) postSodaPassportWithCookie(apiURL string, form url.Values, cookie string) ([]byte, map[string]string, error) {
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", sodaPassportUA)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json, text/javascript")
	if flowID := strings.TrimSpace(form.Get("std_verify_flow_id")); flowID != "" {
		req.Header.Set("X-Tt-Passport-Verify-Portrait", flowID)
	}
	if strings.TrimSpace(cookie) != "" {
		req.Header.Set("Cookie", strings.TrimSpace(cookie))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	cookies := make(map[string]string)
	for _, c := range resp.Cookies() {
		if c.Name != "" && c.Value != "" {
			cookies[c.Name] = c.Value
		}
	}
	return bodyBytes, cookies, nil
}

func sodaQRConnectErrorMessage(resp sodaQRConnectResponse) string {
	for _, value := range []string{resp.Data.Description, resp.Message} {
		value = strings.TrimSpace(value)
		if value != "" {
			if resp.Data.ErrorCode != 0 {
				return fmt.Sprintf("%s (code=%d)", value, resp.Data.ErrorCode)
			}
			return value
		}
	}
	if resp.Data.ErrorCode != 0 {
		return fmt.Sprintf("Soda QR 登录失败 (code=%d)", resp.Data.ErrorCode)
	}
	return "Soda QR 登录失败"
}

func sodaQRConnectExtra(resp sodaQRConnectResponse) map[string]string {
	extra := map[string]string{}
	if status := strings.TrimSpace(resp.Data.Status); status != "" {
		extra["api_status"] = status
	}
	if resp.Data.ErrorCode != 0 {
		extra["error_code"] = fmt.Sprintf("%d", resp.Data.ErrorCode)
	}
	if redirect := strings.TrimSpace(resp.Data.Redirect); redirect != "" {
		extra["redirect_url"] = redirect
	}
	return extra
}

func buildSodaPassportQuery() string {
	now := time.Now().UnixMilli()
	params := url.Values{}
	params.Set("passport_jssdk_version", sodaPassportJSVer)
	params.Set("passport_jssdk_type", "normal")
	params.Set("is_from_ttaccountsdk", "1")
	params.Set("aid", "386088")
	params.Set("language", "zh")
	params.Set("is_new_login", "1")
	params.Set("is_from_iesaccountsaas", "1")
	params.Set("device_id", fmt.Sprintf("%d", now))
	params.Set("install_id", fmt.Sprintf("%d", now+1))
	params.Set("device_platform", "PC")
	params.Set("version_code", "3.3.0")
	return params.Encode()
}

func sodaScanLoginURL(token string) string {
	params := url.Values{}
	params.Set("token", strings.TrimSpace(token))
	params.Set("os", "Windows")
	params.Set("computer_name", "go-music-dl")
	return "https://bff-pc.qishui.com/light/invoke/scan_login?" + params.Encode()
}

func buildSodaPassportLiteQuery() string {
	now := time.Now().UnixMilli()
	params := url.Values{}
	params.Set("passport_jssdk_version", "5.1.2")
	params.Set("passport_jssdk_type", "lite")
	params.Set("is_from_ttaccountsdk", "1")
	params.Set("aid", "386088")
	params.Set("language", "zh")
	params.Set("new_authn_sdk_version", "1.0.0.404-web")
	params.Set("is_new_login", "1")
	params.Set("is_from_iesaccountsaas", "1")
	params.Set("device_id", fmt.Sprintf("%d", now))
	params.Set("install_id", fmt.Sprintf("%d", now+1))
	params.Set("device_platform", "PC")
	params.Set("version_code", "3.3.0")
	return params.Encode()
}

func extractSodaCookieValue(cookies map[string]string, name string) string {
	return cookies[name]
}

func extractSodaMFAField(body []byte, field string) string {
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}
	if v := findSodaJSONString(raw, field); v != "" {
		return v
	}
	if field == "encrypt_uid" {
		return findSodaJSONString(raw, "sec_user_id")
	}
	return ""
}

func extractSodaMFAVerifyParams(body []byte, cookies map[string]string) string {
	var raw interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return ""
	}

	params := url.Values{}
	collectSodaVerifyParams(raw, params)
	if mfa := cookies["passport_mfa_token"]; mfa != "" {
		params.Set("passport_mfa_token", mfa)
	}
	return params.Encode()
}

func normalizeSodaJSONKey(key string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(strings.TrimSpace(key)))
}

func findSodaJSONString(value interface{}, field string) string {
	want := normalizeSodaJSONKey(field)
	switch v := value.(type) {
	case map[string]interface{}:
		for key, child := range v {
			if normalizeSodaJSONKey(key) == want {
				switch s := child.(type) {
				case string:
					return strings.TrimSpace(s)
				case float64:
					return fmt.Sprintf("%.0f", s)
				}
			}
			if found := findSodaJSONString(child, field); found != "" {
				return found
			}
		}
	case []interface{}:
		for _, child := range v {
			if found := findSodaJSONString(child, field); found != "" {
				return found
			}
		}
	case string:
		var nested interface{}
		if strings.HasPrefix(strings.TrimSpace(v), "{") && json.Unmarshal([]byte(v), &nested) == nil {
			return findSodaJSONString(nested, field)
		}
	}
	return ""
}

func collectSodaVerifyParams(value interface{}, params url.Values) {
	allow := map[string]bool{
		"passport_mfa_retry_tag": true,
		"std_verify_flow_id":     true,
		"std_verify_scene":       true,
		"std_verify_template":    true,
		"std_verify_token":       true,
		"std_verify_type":        true,
		"std_verify_way":         true,
	}
	switch v := value.(type) {
	case map[string]interface{}:
		for key, child := range v {
			if allow[key] {
				switch s := child.(type) {
				case string:
					if strings.TrimSpace(s) != "" {
						params.Set(key, strings.TrimSpace(s))
					}
				case float64:
					params.Set(key, fmt.Sprintf("%.0f", s))
				}
			}
			collectSodaVerifyParams(child, params)
		}
	case []interface{}:
		for _, child := range v {
			collectSodaVerifyParams(child, params)
		}
	case string:
		text := strings.TrimSpace(v)
		if strings.Contains(text, "std_verify_") || strings.Contains(text, "passport_mfa_retry_tag") {
			if idx := strings.Index(text, "?"); idx >= 0 {
				text = text[idx+1:]
			}
			if parsed, err := url.ParseQuery(text); err == nil {
				for key, values := range parsed {
					if allow[key] && len(values) > 0 && strings.TrimSpace(values[0]) != "" {
						params.Set(key, strings.TrimSpace(values[0]))
					}
				}
			}
		}
		var nested interface{}
		if strings.HasPrefix(text, "{") && json.Unmarshal([]byte(text), &nested) == nil {
			collectSodaVerifyParams(nested, params)
		}
	}
}

func rememberSodaQRLoginPending(token string, state sodaQRLoginPendingState) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	if state.ExpiresAt.IsZero() {
		state.ExpiresAt = time.Now().Add(10 * time.Minute)
	}
	state.Cookies = mergeSodaCookies(nil, state.Cookies)
	sodaQRLoginMu.Lock()
	defer sodaQRLoginMu.Unlock()
	cleanupSodaQRLoginPendingLocked(time.Now())
	sodaQRLoginPending[token] = state
}

func getSodaQRLoginPending(token string) (sodaQRLoginPendingState, bool) {
	sodaQRLoginMu.Lock()
	defer sodaQRLoginMu.Unlock()
	now := time.Now()
	cleanupSodaQRLoginPendingLocked(now)
	state, ok := sodaQRLoginPending[strings.TrimSpace(token)]
	if !ok || (!state.ExpiresAt.IsZero() && now.After(state.ExpiresAt)) {
		return sodaQRLoginPendingState{}, false
	}
	state.Cookies = mergeSodaCookies(nil, state.Cookies)
	return state, true
}

func clearSodaQRLoginPending(token string) {
	sodaQRLoginMu.Lock()
	defer sodaQRLoginMu.Unlock()
	delete(sodaQRLoginPending, strings.TrimSpace(token))
}

func cleanupSodaQRLoginPending() {
	sodaQRLoginMu.Lock()
	defer sodaQRLoginMu.Unlock()
	cleanupSodaQRLoginPendingLocked(time.Now())
}

func cleanupSodaQRLoginPendingLocked(now time.Time) {
	for token, state := range sodaQRLoginPending {
		if !state.ExpiresAt.IsZero() && now.After(state.ExpiresAt) {
			delete(sodaQRLoginPending, token)
		}
	}
}

func mergeSodaCookies(cookieMaps ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, cookies := range cookieMaps {
		for key, value := range cookies {
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key != "" && value != "" {
				merged[key] = value
			}
		}
	}
	return merged
}

func sodaCookieHeader(base string, cookies map[string]string) string {
	cookie := sodaJoinCookies(cookies)
	base = strings.TrimSpace(base)
	switch {
	case base == "":
		return cookie
	case cookie == "":
		return base
	default:
		return base + "; " + cookie
	}
}

func sodaCookiesHaveSession(cookies map[string]string) bool {
	for _, key := range []string{"sessionid", "sessionid_ss", "sid_tt", "sid_guard"} {
		if strings.TrimSpace(cookies[key]) != "" {
			return true
		}
	}
	return false
}

func strconvBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func sodaJoinCookies(cookies map[string]string) string {
	if len(cookies) == 0 {
		return ""
	}
	keys := make([]string, 0, len(cookies))
	for k := range cookies {
		if k != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if cookies[k] != "" {
			parts = append(parts, k+"="+cookies[k])
		}
	}
	return strings.Join(parts, "; ")
}
