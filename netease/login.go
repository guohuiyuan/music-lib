package netease

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
	"github.com/guohuiyuan/music-lib/utils"
)

const (
	neteaseQRKeyAPI   = "https://music.163.com/weapi/login/qrcode/unikey?csrf_token="
	neteaseQRCheckAPI = "https://music.163.com/weapi/login/qrcode/client/login?csrf_token="
)

func CreateQRLogin() (*model.QRLoginSession, error) { return defaultNetease.CreateQRLogin() }

func CheckQRLogin(key string) (*model.QRLoginResult, error) { return defaultNetease.CheckQRLogin(key) }

func (n *Netease) CreateQRLogin() (*model.QRLoginSession, error) {
	reqData := map[string]interface{}{"type": 1}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	body, _, err := n.postQRLogin(neteaseQRKeyAPI, form)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code   int    `json:"code"`
		UniKey string `json:"unikey"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease qr key json parse error: %w", err)
	}
	if resp.Code != 200 || strings.TrimSpace(resp.UniKey) == "" {
		return nil, fmt.Errorf("netease qr key api error: code=%d", resp.Code)
	}

	loginURL := "https://music.163.com/login?codekey=" + url.QueryEscape(resp.UniKey)
	return &model.QRLoginSession{
		Source:    "netease",
		Key:       resp.UniKey,
		URL:       loginURL,
		ExpiresAt: time.Now().Add(5 * time.Minute).Unix(),
	}, nil
}

func (n *Netease) CheckQRLogin(key string) (*model.QRLoginResult, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("netease qr login key is empty")
	}

	reqData := map[string]interface{}{"key": key, "type": 1}
	reqJSON, _ := json.Marshal(reqData)
	params, encSecKey := EncryptWeApi(string(reqJSON))
	form := url.Values{}
	form.Set("params", params)
	form.Set("encSecKey", encSecKey)

	body, cookies, err := n.postQRLogin(neteaseQRCheckAPI, form)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Cookie  string `json:"cookie"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("netease qr check json parse error: %w", err)
	}

	result := &model.QRLoginResult{
		Source:  "netease",
		Key:     key,
		Status:  mapNeteaseQRStatus(resp.Code),
		Message: resp.Message,
		Extra: map[string]string{
			"code": fmt.Sprintf("%d", resp.Code),
		},
	}
	if result.Status == model.QRLoginStatusSuccess {
		cookie := strings.TrimSpace(resp.Cookie)
		if cookie == "" {
			cookie = joinCookieMap(cookies)
		}
		result.Cookie = cookie
		result.Cookies = cookies
		n.cookie = cookie
		n.isVipCache = nil
	}
	return result, nil
}

func (n *Netease) postQRLogin(apiURL string, form url.Values) ([]byte, map[string]string, error) {
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", Referer)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	utils.WithRandomIPHeader()(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("netease qr login http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return body, responseCookies(resp), nil
}

func mapNeteaseQRStatus(code int) model.QRLoginStatus {
	switch code {
	case 800:
		return model.QRLoginStatusExpired
	case 801:
		return model.QRLoginStatusWaiting
	case 802:
		return model.QRLoginStatusScanned
	case 803:
		return model.QRLoginStatusSuccess
	default:
		return model.QRLoginStatusFailed
	}
}

func responseCookies(resp *http.Response) map[string]string {
	cookies := map[string]string{}
	for _, cookie := range resp.Cookies() {
		if strings.TrimSpace(cookie.Name) == "" {
			continue
		}
		cookies[cookie.Name] = cookie.Value
	}
	return cookies
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
