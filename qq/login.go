package qq

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/model"
)

const (
	qqQRShowAPI  = "https://ssl.ptlogin2.qq.com/ptqrshow"
	qqQRCheckAPI = "https://ssl.ptlogin2.qq.com/ptqrlogin"
)

func CreateQRLogin() (*model.QRLoginSession, error) { return defaultQQ.CreateQRLogin() }

func CheckQRLogin(key string) (*model.QRLoginResult, error) { return defaultQQ.CheckQRLogin(key) }

func (q *QQ) CreateQRLogin() (*model.QRLoginSession, error) {
	params := url.Values{}
	params.Set("appid", "716027609")
	params.Set("e", "2")
	params.Set("l", "M")
	params.Set("s", "3")
	params.Set("d", "72")
	params.Set("v", "4")
	params.Set("t", fmt.Sprintf("%.17f", float64(time.Now().UnixNano())/1e18))
	params.Set("daid", "383")
	params.Set("pt_3rd_aid", "100497308")

	req, err := http.NewRequest("GET", qqQRShowAPI+"?"+params.Encode(), nil)
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
		return nil, fmt.Errorf("qq qr show http status %d", resp.StatusCode)
	}
	image, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	cookies := responseCookies(resp)
	qrsig := strings.TrimSpace(cookies["qrsig"])
	if qrsig == "" {
		return nil, fmt.Errorf("qq qr show missing qrsig")
	}

	key := url.Values{}
	key.Set("qrsig", qrsig)
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

	params := url.Values{}
	params.Set("u1", "https://graph.qq.com/oauth2.0/login_jump")
	params.Set("ptqrtoken", strconv.Itoa(hash33(qrsig)))
	params.Set("ptredirect", "100")
	params.Set("h", "1")
	params.Set("t", "1")
	params.Set("g", "1")
	params.Set("from_ui", "1")
	params.Set("ptlang", "2052")
	params.Set("action", fmt.Sprintf("0-0-%d", time.Now().UnixMilli()))
	params.Set("js_ver", "21072115")
	params.Set("js_type", "1")
	params.Set("login_sig", "")
	params.Set("pt_uistyle", "40")
	params.Set("aid", "716027609")
	params.Set("daid", "383")
	params.Set("pt_3rd_aid", "100497308")
	params.Set("has_onekey", "1")
	params.Set("pttype", "1")
	params.Set("service", "ptqrlogin")
	params.Set("nodirect", "0")

	req, err := http.NewRequest("GET", qqQRCheckAPI+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "https://xui.ptlogin2.qq.com/")
	req.Header.Set("Cookie", "qrsig="+qrsig)
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

	cookies := responseCookies(resp)
	if redirectURL != "" {
		redirectCookies, err := fetchQQRedirectCookies(redirectURL, cookies)
		if err == nil {
			for k, v := range redirectCookies {
				cookies[k] = v
			}
		} else {
			result.Extra["redirect_error"] = err.Error()
		}
	}
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

func fetchQQRedirectCookies(redirectURL string, cookies map[string]string) (map[string]string, error) {
	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	currentURL := strings.TrimSpace(redirectURL)
	collected := make(map[string]string, len(cookies)+8)
	for k, v := range cookies {
		collected[k] = v
	}
	referer := "https://y.qq.com/"

	for i := 0; i < 8 && currentURL != ""; i++ {
		req, err := http.NewRequest("GET", currentURL, nil)
		if err != nil {
			return collected, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("Referer", referer)
		req.Header.Set("Cookie", joinCookieMap(collected))
		resp, err := client.Do(req)
		if err != nil {
			return collected, err
		}

		for k, v := range responseCookies(resp) {
			collected[k] = v
		}

		location := strings.TrimSpace(resp.Header.Get("Location"))
		resp.Body.Close()
		if location == "" || resp.StatusCode < 300 || resp.StatusCode >= 400 {
			break
		}
		nextURL, err := url.Parse(location)
		if err != nil {
			return collected, err
		}
		if !nextURL.IsAbs() {
			baseURL, err := url.Parse(currentURL)
			if err != nil {
				return collected, err
			}
			nextURL = baseURL.ResolveReference(nextURL)
		}
		referer = currentURL
		currentURL = nextURL.String()
	}

	return collected, nil
}

func normalizeQQMusicCookies(cookies map[string]string) map[string]string {
	result := make(map[string]string, len(cookies)+4)
	for k, v := range cookies {
		result[k] = v
	}
	if result["uin"] == "" {
		result["uin"] = firstNonEmptyQQ(result["ptui_loginuin"], result["luin"], result["pt2gguin"], result["superuin"], result["p_uin"])
	}
	if result["qqmusic_key"] == "" {
		result["qqmusic_key"] = firstNonEmptyQQ(result["p_skey"], result["skey"])
	}
	if result["qm_keyst"] == "" {
		result["qm_keyst"] = result["qqmusic_key"]
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
