package qq

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

const (
	qqQRLoginPageAPI = "https://xui.ptlogin2.qq.com/cgi-bin/xlogin"
	qqQRShowAPI      = "https://xui.ptlogin2.qq.com/ssl/ptqrshow"
	qqQRCheckAPI     = "https://xui.ptlogin2.qq.com/ssl/ptqrlogin"
	qqOAuthLoginJump = "https://graph.qq.com/oauth2.0/login_jump"
	qqOAuthAuthorize = "https://graph.qq.com/oauth2.0/authorize"
	qqMusicLoginAPI  = "https://u.y.qq.com/cgi-bin/musicu.fcg"
	qqOAuthClientID  = "100497308"
	qqOAuthScope     = "get_user_info,get_app_friends"
	qqWXQRConnectAPI = "https://open.weixin.qq.com/connect/qrconnect"
	qqWXQRCheckAPI   = "https://lp.open.weixin.qq.com/connect/l/qrconnect"
	qqWXRedirectURI  = "https://y.qq.com/portal/wx_redirect.html?login_type=2&surl=https://y.qq.com/"
	qqWXAppID        = "wx48db31d50e334801"
)

func CreateQRLogin() (*model.QRLoginSession, error) { return defaultQQ.CreateQRLogin() }

func CheckQRLogin(key string) (*model.QRLoginResult, error) { return defaultQQ.CheckQRLogin(key) }

func CreateWXQRLogin() (*model.QRLoginSession, error) { return defaultQQ.CreateWXQRLogin() }

func CheckWXQRLogin(key string) (*model.QRLoginResult, error) { return defaultQQ.CheckWXQRLogin(key) }

func CreateQRLoginByType(loginType string) (*model.QRLoginSession, error) {
	return defaultQQ.CreateQRLoginByType(loginType)
}

func CheckQRLoginByType(loginType, key string) (*model.QRLoginResult, error) {
	return defaultQQ.CheckQRLoginByType(loginType, key)
}

func (q *QQ) CreateQRLoginByType(loginType string) (*model.QRLoginSession, error) {
	switch normalizeQQLoginType(loginType) {
	case "wx":
		return q.CreateWXQRLogin()
	default:
		return q.CreateQRLogin()
	}
}

func (q *QQ) CheckQRLoginByType(loginType, key string) (*model.QRLoginResult, error) {
	switch normalizeQQLoginType(loginType) {
	case "wx":
		return q.CheckWXQRLogin(key)
	default:
		return q.CheckQRLogin(key)
	}
}

func (q *QQ) CreateQRLogin() (*model.QRLoginSession, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Jar: jar, Timeout: 20 * time.Second}

	loginPageParams := url.Values{}
	loginPageParams.Set("appid", "716027609")
	loginPageParams.Set("daid", "383")
	loginPageParams.Set("style", "33")
	loginPageParams.Set("login_text", "登录")
	loginPageParams.Set("hide_title_bar", "1")
	loginPageParams.Set("hide_border", "1")
	loginPageParams.Set("target", "self")
	loginPageParams.Set("s_url", qqOAuthLoginJump)
	loginPageParams.Set("pt_3rd_aid", qqOAuthClientID)
	loginPageParams.Set("pt_feedback_link", "https://support.qq.com/products/77942?customInfo=.appid"+qqOAuthClientID)
	loginPageParams.Set("theme", "2")
	loginPageParams.Set("verify_theme", "")
	if warmupResp, err := doQQRequest(client, http.MethodGet, qqQRLoginPageAPI+"?"+loginPageParams.Encode(), nil, "", "", nil); err == nil {
		warmupResp.Body.Close()
	}

	params := url.Values{}
	params.Set("appid", "716027609")
	params.Set("e", "2")
	params.Set("l", "M")
	params.Set("s", "3")
	params.Set("d", "72")
	params.Set("v", "4")
	params.Set("t", fmt.Sprintf("%.6f", float64(time.Now().UnixNano())/1e9))
	params.Set("daid", "383")
	params.Set("pt_3rd_aid", qqOAuthClientID)
	params.Set("u1", qqOAuthLoginJump)

	resp, err := doQQRequest(client, http.MethodGet, qqQRShowAPI+"?"+params.Encode(), nil, "https://xui.ptlogin2.qq.com/", "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qq qr show http status %d", resp.StatusCode)
	}
	image, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	cookies := qqCookiesForURLs(jar, qqQRLoginPageAPI, qqQRShowAPI)
	for k, v := range responseCookies(resp) {
		if strings.TrimSpace(v) != "" {
			cookies[k] = v
		}
	}
	qrsig := strings.TrimSpace(cookies["qrsig"])
	if qrsig == "" {
		return nil, fmt.Errorf("qq qr show missing qrsig")
	}

	key := url.Values{}
	key.Set("qrsig", qrsig)
	if loginSig := strings.TrimSpace(cookies["pt_login_sig"]); loginSig != "" {
		key.Set("login_sig", loginSig)
	}
	if state := encodeQQCookieState(cookies); state != "" {
		key.Set("session", state)
	}
	return &model.QRLoginSession{
		Source:    "qq",
		Key:       key.Encode(),
		ImageURL:  "data:image/png;base64," + base64StdEncode(image),
		ExpiresAt: time.Now().Add(2 * time.Minute).Unix(),
		Extra: map[string]string{
			"qrsig": qrsig,
		},
	}, nil
}

func (q *QQ) CheckQRLogin(key string) (*model.QRLoginResult, error) {
	values, err := url.ParseQuery(key)
	if err != nil {
		return nil, err
	}
	qrsig := strings.TrimSpace(values.Get("qrsig"))
	if qrsig == "" {
		return nil, fmt.Errorf("qq qr login key missing qrsig")
	}
	loginSig := strings.TrimSpace(values.Get("login_sig"))
	initialCookies := decodeQQCookieState(values.Get("session"))
	if initialCookies == nil {
		initialCookies = map[string]string{}
	}
	initialCookies["qrsig"] = qrsig
	if loginSig != "" {
		initialCookies["pt_login_sig"] = loginSig
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	setQQInitialCookies(jar, initialCookies)
	client := &http.Client{
		Jar:     jar,
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	params := url.Values{}
	params.Set("u1", qqOAuthLoginJump)
	params.Set("ptqrtoken", strconv.Itoa(hash33(qrsig)))
	params.Set("ptredirect", "0")
	params.Set("h", "1")
	params.Set("t", "1")
	params.Set("g", "1")
	params.Set("from_ui", "1")
	params.Set("ptlang", "2052")
	params.Set("action", fmt.Sprintf("0-0-%d", time.Now().UnixMilli()))
	params.Set("js_ver", "26071711")
	params.Set("js_type", "1")
	params.Set("login_sig", loginSig)
	params.Set("pt_uistyle", "40")
	params.Set("aid", "716027609")
	params.Set("daid", "383")
	params.Set("pt_3rd_aid", qqOAuthClientID)
	params.Set("pt_js_version", "c1987b96")

	resp, err := doQQRequest(client, http.MethodGet, qqQRCheckAPI+"?"+params.Encode(), nil, "https://xui.ptlogin2.qq.com/", "", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	raw := string(body)
	code, message, redirectURL := parseQQQRCheck(raw)
	result := &model.QRLoginResult{
		Source:  "qq",
		Key:     key,
		Status:  mapQQQRStatus(code),
		Message: message,
		Extra: map[string]string{
			"code": code,
		},
	}
	if result.Status != model.QRLoginStatusSuccess {
		return result, nil
	}

	if redirectURL != "" {
		cookies, err := finishQQMusicLogin(client, jar, redirectURL)
		if err != nil {
			result.Status = model.QRLoginStatusFailed
			result.Message = err.Error()
			result.Extra["redirect_error"] = err.Error()
			return result, nil
		}
		result.Cookies = normalizeQQMusicCookies(cookies)
		result.Cookie = joinCookieMap(result.Cookies)
		q.cookie = result.Cookie
		q.isVipCache = nil
		return result, nil
	}
	result.Status = model.QRLoginStatusFailed
	result.Message = "qq login redirect url missing"
	return result, nil
}

func (q *QQ) CreateWXQRLogin() (*model.QRLoginSession, error) {
	state := fmt.Sprintf("music-lib-%d", time.Now().UnixNano())
	params := url.Values{}
	params.Set("appid", qqWXAppID)
	params.Set("redirect_uri", qqWXRedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", "snsapi_login")
	params.Set("state", state)
	params.Set("href", "https://y.qq.com/mediastyle/music_v17/src/css/popup_wechat.css#wechat_redirect")
	loginURL := qqWXQRConnectAPI + "?" + params.Encode()

	req, err := http.NewRequest("GET", loginURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://y.qq.com/")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qq wx qr connect http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	uuid := parseQQWXQRUUID(string(body))
	if uuid == "" {
		return nil, fmt.Errorf("qq wx qr connect missing uuid")
	}

	key := url.Values{}
	key.Set("type", "wx")
	key.Set("uuid", uuid)
	key.Set("state", state)
	return &model.QRLoginSession{
		Source:    "qq",
		Key:       key.Encode(),
		URL:       loginURL,
		ImageURL:  "https://open.weixin.qq.com/connect/qrcode/" + url.PathEscape(uuid),
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
		Extra: map[string]string{
			"login_type": "wx",
			"uuid":       uuid,
		},
	}, nil
}

func (q *QQ) CheckWXQRLogin(key string) (*model.QRLoginResult, error) {
	values, err := url.ParseQuery(key)
	if err != nil {
		return nil, err
	}
	uuid := strings.TrimSpace(values.Get("uuid"))
	state := strings.TrimSpace(values.Get("state"))
	if uuid == "" {
		return nil, fmt.Errorf("qq wx qr login key missing uuid")
	}
	if state == "" {
		state = "STATE"
	}

	params := url.Values{}
	params.Set("uuid", uuid)
	params.Set("_", strconv.FormatInt(time.Now().UnixMilli(), 10))
	req, err := http.NewRequest("GET", qqWXQRCheckAPI+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", qqWXQRConnectAPI)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	raw := string(body)
	code, wxCode := parseQQWXQRCheck(raw)
	result := &model.QRLoginResult{
		Source:  "qq",
		Key:     key,
		Status:  mapQQWXQRStatus(code),
		Message: qqWXQRMessage(code, raw),
		Extra: map[string]string{
			"code":       code,
			"login_type": "wx",
		},
	}
	if result.Status != model.QRLoginStatusSuccess {
		return result, nil
	}
	if wxCode == "" {
		result.Status = model.QRLoginStatusFailed
		result.Message = "wechat auth code missing"
		return result, nil
	}

	cookies, extra, err := fetchQQWXLoginCookies(wxCode)
	if err != nil {
		result.Status = model.QRLoginStatusFailed
		result.Message = err.Error()
		return result, nil
	}
	for k, v := range extra {
		result.Extra[k] = v
	}
	result.Extra["state"] = state
	result.Cookies = normalizeQQMusicCookies(cookies)
	result.Cookie = joinCookieMap(result.Cookies)
	q.cookie = result.Cookie
	q.isVipCache = nil
	return result, nil
}

func mapQQQRStatus(code string) model.QRLoginStatus {
	switch code {
	case "0":
		return model.QRLoginStatusSuccess
	case "65":
		return model.QRLoginStatusExpired
	case "66":
		return model.QRLoginStatusWaiting
	case "67":
		return model.QRLoginStatusScanned
	default:
		return model.QRLoginStatusFailed
	}
}

func parseQQQRCheck(raw string) (code, message, redirectURL string) {
	re := regexp.MustCompile(`'([^']*)'`)
	matches := re.FindAllStringSubmatch(raw, -1)
	if len(matches) >= 5 {
		return matches[0][1], matches[4][1], matches[2][1]
	}
	return "", raw, ""
}

func mapQQWXQRStatus(code string) model.QRLoginStatus {
	switch code {
	case "405":
		return model.QRLoginStatusSuccess
	case "402":
		return model.QRLoginStatusExpired
	case "404":
		return model.QRLoginStatusScanned
	case "408":
		return model.QRLoginStatusWaiting
	default:
		return model.QRLoginStatusFailed
	}
}

func qqWXQRMessage(code, raw string) string {
	switch code {
	case "405":
		return "登录成功"
	case "402":
		return "二维码已过期"
	case "404":
		return "已扫码，请在微信中确认"
	case "408":
		return "等待扫码中"
	default:
		return strings.TrimSpace(raw)
	}
}

func parseQQWXQRUUID(raw string) string {
	patterns := []string{
		`connect/l/qrconnect\?uuid=([A-Za-z0-9_-]+)`,
		`window\.QRLogin\.uuid\s*=\s*"([^"]+)"`,
		`/connect/qrcode/([A-Za-z0-9_-]+)`,
	}
	for _, pattern := range patterns {
		matches := regexp.MustCompile(pattern).FindStringSubmatch(raw)
		if len(matches) > 1 && strings.TrimSpace(matches[1]) != "" {
			return strings.TrimSpace(matches[1])
		}
	}
	return ""
}

func parseQQWXQRCheck(raw string) (code, wxCode string) {
	if matches := regexp.MustCompile(`wx_errcode\s*=\s*'?([0-9]+)'?`).FindStringSubmatch(raw); len(matches) > 1 {
		code = strings.TrimSpace(matches[1])
	}
	if matches := regexp.MustCompile(`wx_code\s*=\s*["']([^"']*)["']`).FindStringSubmatch(raw); len(matches) > 1 {
		wxCode = strings.TrimSpace(matches[1])
	}
	return code, wxCode
}

func fetchQQWXLoginCookies(wxCode string) (map[string]string, map[string]string, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"comm": map[string]interface{}{
			"tmeAppID":     "qqmusic",
			"tmeLoginType": "1",
			"g_tk":         5381,
			"platform":     "yqq",
			"ct":           24,
			"cv":           0,
		},
		"req": map[string]interface{}{
			"module": "music.login.LoginServer",
			"method": "Login",
			"param": map[string]string{
				"strAppid": qqWXAppID,
				"code":     wxCode,
			},
		},
	})
	if err != nil {
		return nil, nil, err
	}

	endpoints := []string{
		"https://u.y.qq.com/cgi-bin/musicu.fcg",
		"https://szu.y.qq.com/cgi-bin/musicu.fcg",
		"https://shu.y.qq.com/cgi-bin/musicu.fcg",
	}
	var lastErr error
	for _, apiURL := range endpoints {
		req, err := http.NewRequest("POST", apiURL, strings.NewReader(string(payload)))
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("Referer", qqWXRedirectURI)
		req.Header.Set("Origin", "https://y.qq.com")
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Cookie", "login_type=2")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		cookies := responseCookies(resp)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("qq wx login http status %d", resp.StatusCode)
			continue
		}

		var parsed struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Msg     string `json:"msg"`
			Req     struct {
				Code    int                    `json:"code"`
				Message string                 `json:"message"`
				Msg     string                 `json:"msg"`
				Data    map[string]interface{} `json:"data"`
			} `json:"req"`
		}
		if err := json.Unmarshal(body, &parsed); err != nil {
			lastErr = fmt.Errorf("qq wx login json parse error: %w", err)
			continue
		}
		if parsed.Code != 0 || parsed.Req.Code != 0 {
			msg := firstNonEmptyQQ(parsed.Req.Message, parsed.Req.Msg, parsed.Message, parsed.Msg)
			lastErr = fmt.Errorf("qq wx login api error: %s (code %d, req code %d)", msg, parsed.Code, parsed.Req.Code)
			continue
		}

		for k, v := range qqWXLoginDataCookies(parsed.Req.Data) {
			if cookies[k] == "" {
				cookies[k] = v
			}
		}
		extra := map[string]string{
			"endpoint": apiURL,
		}
		return cookies, extra, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("qq wx login failed")
	}
	return nil, nil, lastErr
}

func qqWXLoginDataCookies(data map[string]interface{}) map[string]string {
	result := map[string]string{}
	value := func(keys ...string) string {
		for _, key := range keys {
			switch v := data[key].(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			case float64:
				if v > 0 {
					return strconv.FormatInt(int64(v), 10)
				}
			}
		}
		return ""
	}

	if musicID := value("musicid", "musicId", "userid", "user_id", "uin"); musicID != "" {
		result["musicid"] = musicID
	}
	if musicKey := value("musickey", "music_key", "qqmusic_key", "qm_keyst", "strMusicKey"); musicKey != "" {
		result["musickey"] = musicKey
		result["qqmusic_key"] = musicKey
		result["qm_keyst"] = musicKey
	}
	if refreshKey := value("refresh_key", "refreshKey"); refreshKey != "" {
		result["refresh_key"] = refreshKey
	}
	if refreshToken := value("refresh_token", "refreshToken"); refreshToken != "" {
		result["refresh_token"] = refreshToken
	}
	if openID := value("openid", "openId", "wxopenid", "strOpenid"); openID != "" {
		result["openid"] = openID
		result["wxopenid"] = openID
	}
	if unionID := value("unionid", "unionId", "wxunionid", "strUnionid"); unionID != "" {
		result["unionid"] = unionID
		result["wxunionid"] = unionID
	}
	if accessToken := value("access_token", "accessToken", "wxaccess_token"); accessToken != "" {
		result["wxaccess_token"] = accessToken
	}
	return result
}

func finishQQMusicLogin(client *http.Client, jar http.CookieJar, redirectURL string) (map[string]string, error) {
	return completeQQMusicLogin(client, jar, redirectURL, qqOAuthAuthorize, qqMusicLoginAPI)
}

func completeQQMusicLogin(client *http.Client, jar http.CookieJar, redirectURL, authorizeURL, musicAPIURL string) (map[string]string, error) {
	loginURL, err := followQQLoginRedirects(client, redirectURL, "https://xui.ptlogin2.qq.com/")
	if err != nil {
		return nil, err
	}

	pSkey := firstNonEmptyQQ(qqJarCookie(jar, loginURL, "p_skey"), qqJarCookie(jar, qqOAuthLoginJump, "p_skey"))
	if pSkey == "" {
		return nil, fmt.Errorf("qq login missing p_skey")
	}

	redirectURI := "https://y.qq.com/portal/wx_redirect.html?login_type=1&surl=" + url.QueryEscape("https://y.qq.com/")
	form := url.Values{}
	form.Set("response_type", "code")
	form.Set("client_id", qqOAuthClientID)
	form.Set("redirect_uri", redirectURI)
	form.Set("scope", qqOAuthScope)
	form.Set("state", "state")
	form.Set("switch", "")
	form.Set("from_ptlogin", "1")
	form.Set("src", "1")
	form.Set("update_auth", "1")
	form.Set("openapi", "1010_1030")
	form.Set("g_tk", strconv.Itoa(gtk33(pSkey)))
	form.Set("auth_time", strconv.FormatInt(time.Now().UnixMilli(), 10))
	form.Set("ui", qqUUID())

	resp, err := doQQRequest(client, http.MethodPost, authorizeURL, []byte(form.Encode()), qqOAuthLoginJump, "application/x-www-form-urlencoded", map[string]string{
		"Origin": "https://graph.qq.com",
	})
	if err != nil {
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("qq oauth authorize http status %d", resp.StatusCode)
	}

	finalURL := resp.Request.URL.String()
	if location := strings.TrimSpace(resp.Header.Get("Location")); location != "" {
		finalURL, err = followQQLoginRedirects(client, resolveQQURL(authorizeURL, location), authorizeURL)
		if err != nil {
			return nil, err
		}
	}
	code := qqQueryValue(finalURL, "code")
	if code == "" {
		return nil, fmt.Errorf("qq oauth authorize missing code")
	}

	payload, err := json.Marshal(map[string]interface{}{
		"comm": map[string]interface{}{
			"g_tk":     5381,
			"platform": "yqq",
			"ct":       24,
			"cv":       0,
		},
		"req": map[string]interface{}{
			"module": "QQConnectLogin.LoginServer",
			"method": "QQLogin",
			"param": map[string]string{
				"code": code,
			},
		},
	})
	if err != nil {
		return nil, err
	}

	resp, err = doQQRequest(client, http.MethodPost, musicAPIURL, payload, "https://y.qq.com/", "application/json", nil)
	if err != nil {
		return nil, err
	}
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qq music login http status %d", resp.StatusCode)
	}

	var parsed struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Msg     string `json:"msg"`
		Req     struct {
			Code    int                    `json:"code"`
			Message string                 `json:"message"`
			Msg     string                 `json:"msg"`
			Data    map[string]interface{} `json:"data"`
		} `json:"req"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("qq music login json parse error: %w", err)
	}
	if parsed.Code != 0 || parsed.Req.Code != 0 {
		message := firstNonEmptyQQ(parsed.Req.Message, parsed.Req.Msg, parsed.Message, parsed.Msg)
		return nil, fmt.Errorf("qq music login api error: %s (code %d, req code %d)", message, parsed.Code, parsed.Req.Code)
	}

	cookies := qqCookiesForURLs(jar, "https://xui.ptlogin2.qq.com/", "https://ssl.ptlogin2.qq.com/", "https://graph.qq.com/", "https://u.y.qq.com/", "https://y.qq.com/")
	mergeNonEmptyCookies(cookies, responseCookies(resp))
	for k, v := range qqWXLoginDataCookies(parsed.Req.Data) {
		if strings.TrimSpace(cookies[k]) == "" {
			cookies[k] = v
		}
	}
	normalized := normalizeQQMusicCookies(cookies)
	if firstNonEmptyQQ(normalized["qm_keyst"], normalized["qqmusic_key"], normalized["musickey"]) == "" {
		return nil, fmt.Errorf("qq music login missing qm_keyst")
	}
	return normalized, nil
}

func followQQLoginRedirects(client *http.Client, currentURL, referer string) (string, error) {
	currentURL = strings.TrimSpace(currentURL)
	for i := 0; i < 10 && currentURL != ""; i++ {
		resp, err := doQQRequest(client, http.MethodGet, currentURL, nil, referer, "", nil)
		if err != nil {
			return currentURL, err
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		resp.Body.Close()

		location := strings.TrimSpace(resp.Header.Get("Location"))
		if location == "" || resp.StatusCode < 300 || resp.StatusCode >= 400 {
			return currentURL, nil
		}
		referer = currentURL
		currentURL = resolveQQURL(currentURL, location)
	}
	if currentURL == "" {
		return "", fmt.Errorf("qq login redirect url is empty")
	}
	return currentURL, fmt.Errorf("qq login too many redirects")
}

func doQQRequest(client *http.Client, method, rawURL string, body []byte, referer, contentType string, headers map[string]string) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, rawURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/126.0 Safari/537.36")
	req.Header.Set("Accept", "*/*")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return client.Do(req)
}

func qqCookiesForURLs(jar http.CookieJar, urls ...string) map[string]string {
	cookies := map[string]string{}
	for _, rawURL := range urls {
		u, err := url.Parse(rawURL)
		if err != nil {
			continue
		}
		for _, cookie := range jar.Cookies(u) {
			if strings.TrimSpace(cookie.Name) != "" && strings.TrimSpace(cookie.Value) != "" {
				cookies[cookie.Name] = cookie.Value
			}
		}
	}
	return cookies
}

func qqJarCookie(jar http.CookieJar, rawURL, key string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	for _, cookie := range jar.Cookies(u) {
		if cookie.Name == key {
			return strings.TrimSpace(cookie.Value)
		}
	}
	return ""
}

func setQQInitialCookies(jar http.CookieJar, cookies map[string]string) {
	values := make([]*http.Cookie, 0, len(cookies))
	for key, value := range cookies {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		values = append(values, &http.Cookie{Name: key, Value: value})
	}
	for _, rawURL := range []string{qqQRLoginPageAPI, qqQRShowAPI, qqOAuthLoginJump, "https://y.qq.com/"} {
		u, err := url.Parse(rawURL)
		if err == nil {
			jar.SetCookies(u, values)
		}
	}
}

func encodeQQCookieState(cookies map[string]string) string {
	raw := joinCookieMap(cookies)
	if raw == "" {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeQQCookieState(encoded string) map[string]string {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil
	}
	result := map[string]string{}
	for _, part := range strings.Split(string(raw), ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[0]) != "" && strings.TrimSpace(kv[1]) != "" {
			result[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return result
}

func mergeNonEmptyCookies(target, source map[string]string) {
	for key, value := range source {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		target[key] = value
	}
}

func resolveQQURL(baseURL, reference string) string {
	ref, err := url.Parse(strings.TrimSpace(reference))
	if err != nil {
		return reference
	}
	if ref.IsAbs() {
		return ref.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return reference
	}
	return base.ResolveReference(ref).String()
}

func qqQueryValue(rawURL, key string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Query().Get(key)
}

func gtk33(value string) int {
	h := 5381
	for _, c := range value {
		h += (h << 5) + int(c)
	}
	return h & 0x7fffffff
}

func qqUUID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(buf)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

func normalizeQQMusicCookies(cookies map[string]string) map[string]string {
	result := make(map[string]string, len(cookies)+4)
	mergeNonEmptyCookies(result, cookies)
	if result["uin"] == "" {
		result["uin"] = firstNonEmptyQQ(result["ptui_loginuin"], result["luin"], result["pt2gguin"], result["superuin"], result["p_uin"], result["musicid"], result["userid"], result["wxuin"])
	}
	if result["qqmusic_key"] == "" {
		result["qqmusic_key"] = firstNonEmptyQQ(result["qm_keyst"], result["musickey"], result["music_key"], result["p_skey"], result["skey"])
	}
	if result["qm_keyst"] == "" {
		result["qm_keyst"] = firstNonEmptyQQ(result["qqmusic_key"], result["musickey"], result["music_key"], result["p_skey"], result["skey"])
	}
	return result
}

func hash33(s string) int {
	h := 0
	for _, c := range s {
		h += (h << 5) + int(c)
	}
	return h & 0x7fffffff
}

func firstNonEmptyQQ(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeQQLoginType(loginType string) string {
	loginType = strings.ToLower(strings.TrimSpace(loginType))
	switch loginType {
	case "wx", "wechat", "weixin":
		return "wx"
	default:
		return "qq"
	}
}

func joinCookieMap(cookies map[string]string) string {
	keys := make([]string, 0, len(cookies))
	for key := range cookies {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+cookies[key])
	}
	return strings.Join(parts, "; ")
}

func responseCookies(resp *http.Response) map[string]string {
	cookies := map[string]string{}
	for _, cookie := range resp.Cookies() {
		if strings.TrimSpace(cookie.Name) != "" {
			cookies[cookie.Name] = cookie.Value
		}
	}
	return cookies
}

func base64StdEncode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
