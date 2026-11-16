package soda

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/guohuiyuan/music-lib/utils"
)

const (
	sodaAndroidAPIBase         = "https://api.qishui.com/luna"
	sodaAndroidSearchUserAgent = "com.luna.music/100198030 (Linux; U; Android 15; zh_CN_#Hans; ABR-AL80; Build/V417IR;tt-ok/3.12.13.19)"
	sodaAndroidSearchPageSize  = 20
	sodaPCWebSearchPageSize    = 20
	sodaDouyinImageBaseURL     = "https://p3-luna.douyinpic.com/img/"
)

// sodaPCWebPlaylistSearchURL 构造汽水 Web PC 搜索地址。
// Web 搜索不要求 x-helios / x-medusa，公共服务器没有本机签名器时也能正常使用。
func sodaPCWebPlaylistSearchURL(keyword, searchID string, cursor, pageSize int) string {
	return sodaPCWebSearchURL("playlist", keyword, searchID, cursor, pageSize)
}

func sodaPCWebSearchURL(searchType, keyword, searchID string, cursor, pageSize int) string {
	if cursor < 0 {
		cursor = 0
	}
	if pageSize <= 0 {
		pageSize = sodaPCWebSearchPageSize
	}

	params := url.Values{}
	params.Set("q", keyword)
	params.Set("cursor", strconv.Itoa(cursor))
	params.Set("count", strconv.Itoa(pageSize))
	params.Set("search_id", searchID)
	params.Set("search_method", "input")
	params.Set("debug_params", "")
	params.Set("from_search_id", "")
	params.Set("search_scene", "")
	params.Set("aid", "386088")
	params.Set("device_platform", "web")
	params.Set("channel", "pc_web")

	return "https://api.qishui.com/luna/pc/search/" + searchType + "?" + params.Encode()
}

func sodaPCSignedSearchURL(searchType, keyword, searchID string, cursor int, creds *AppCredentials) string {
	params := sodaPCAppParamsWith(creds)
	params.Set("version_name", "3.8.0")
	params.Set("version_code", "38000000")
	params.Set("q", keyword)
	params.Set("cursor", strconv.Itoa(cursor))
	params.Set("search_id", searchID)
	params.Set("search_method", "input")
	params.Set("debug_params", "")
	params.Set("from_search_id", "")
	params.Set("search_scene", "")
	return "https://api.qishui.com/luna/pc/search/" + searchType + "?" + params.Encode()
}

func sodaNewSearchID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("soda search id generate error: %w", err)
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}

func (s *Soda) fetchPCWebPlaylistSearch(keyword string) ([]byte, error) {
	return s.fetchPCWebSearch("playlist", keyword)
}

func (s *Soda) fetchPCWebSearch(searchType, keyword string) ([]byte, error) {
	searchID, err := sodaNewSearchID()
	if err != nil {
		return nil, err
	}

	opts := []utils.RequestOption{
		utils.WithHeader("User-Agent", UserAgent),
		utils.WithHeader("Accept", "application/json"),
	}
	if cookie := strings.TrimSpace(s.cookie); cookie != "" {
		opts = append(opts, utils.WithHeader("Cookie", cookie))
	}
	return utils.Get(
		sodaPCWebSearchURL(searchType, keyword, searchID, 0, sodaPCWebSearchPageSize),
		opts...,
	)
}

func (s *Soda) fetchSignedPCSearch(searchType, keyword string) ([]byte, error) {
	if !s.HasSigner() {
		return nil, errors.New("soda app search requires a local or remote signer")
	}

	creds, err := s.ensureAppCredentials()
	if err != nil {
		return nil, err
	}
	searchID, err := sodaNewSearchID()
	if err != nil {
		return nil, err
	}

	reqURL := sodaPCSignedSearchURL(searchType, keyword, searchID, 0, &creds)
	localUser := "0"
	if strings.TrimSpace(s.cookie) != "" {
		localUser = "1"
	}
	headers := map[string]string{
		"User-Agent":               "LunaPC/3.8.0",
		"Accept":                   "application/json",
		"x-luna-background-type":   "foreground",
		"x-luna-is-background-req": "0",
		"x-luna-is-local-user":     localUser,
	}
	if cookie := strings.TrimSpace(s.cookie); cookie != "" {
		headers["Cookie"] = cookie
	}
	for key, value := range creds.staticSignatureHeaders() {
		headers[key] = value
	}

	signedURL, signatureHeaders, err := s.signRequest(http.MethodGet, reqURL, "", headers)
	if err != nil {
		return nil, err
	}
	for key, value := range signatureHeaders {
		headers[key] = value
	}

	opts := make([]utils.RequestOption, 0, len(headers))
	for key, value := range headers {
		opts = append(opts, utils.WithHeader(key, value))
	}
	body, err := utils.Get(signedURL, opts...)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, errors.New("soda signed search returned an empty response")
	}
	return body, nil
}

func (s *Soda) fetchTrackSearch(keyword string) ([]byte, error) {
	var attempts []string
	tryBody := func(source string, body []byte, err error) ([]byte, bool) {
		if err == nil {
			err = validateSodaSearchBody(body)
		}
		if err == nil {
			return body, true
		}
		attempts = append(attempts, source+": "+err.Error())
		return nil, false
	}

	if s.HasSigner() {
		body, err := s.fetchSignedPCSearch("track", keyword)
		if body, ok := tryBody("signed", body, err); ok {
			return body, nil
		}
	}

	body, err := s.fetchPCWebSearch("track", keyword)
	if body, ok := tryBody("web", body, err); ok {
		return body, nil
	}

	body, err = s.fetchAndroidSearch("track", keyword, 1, sodaAndroidSearchPageSize)
	if body, ok := tryBody("android", body, err); ok {
		return body, nil
	}
	if len(attempts) == 0 {
		attempts = append(attempts, "all soda track search endpoints returned an empty response")
	}
	return nil, fmt.Errorf("soda track search failed: %s", strings.Join(attempts, "; "))
}

func validateSodaSearchBody(body []byte) error {
	if len(bytes.TrimSpace(body)) == 0 {
		return errors.New("empty response")
	}
	var resp struct {
		StatusCode int               `json:"status_code"`
		StatusInfo sodaAPIStatusInfo `json:"status_info"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	if resp.StatusCode != 0 {
		msg := strings.TrimSpace(resp.StatusInfo.StatusMsg)
		if msg == "" {
			msg = "unknown error"
		}
		return fmt.Errorf("status_code=%d status_msg=%s", resp.StatusCode, msg)
	}
	return nil
}

func sodaAndroidSearchURL(searchType, keyword string, page, pageSize int) string {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = sodaAndroidSearchPageSize
	}

	params := sodaAndroidSearchParams()
	params.Set("q", keyword)
	params.Set("cursor", strconv.Itoa((page-1)*pageSize))
	params.Set("count", strconv.Itoa(pageSize))
	params.Set("aid", "386088")

	return sodaAndroidAPIBase + "/search/" + searchType + "?" + params.Encode()
}

func sodaAndroidSearchParams() url.Values {
	params := url.Values{}
	values := map[string]string{
		"device_platform":              "android",
		"os":                           "android",
		"ssmix":                        "a",
		"cdid":                         "46556f98-1720-4248-83da-62b74b60b46a",
		"channel":                      "xiaomi_8478_64",
		"aid":                          "8478",
		"app_name":                     "luna",
		"version_code":                 "100198030",
		"version_name":                 "19.8.0",
		"manifest_version_code":        "100198030",
		"update_version_code":          "100198030",
		"resolution":                   "1080*1920",
		"dpi":                          "480",
		"device_type":                  "ABR-AL80",
		"device_brand":                 "HUAWEI",
		"language":                     "zh",
		"os_api":                       "35",
		"os_version":                   "15",
		"ac":                           "wifi",
		"device_model":                 "ABR-AL80",
		"save_power":                   "0",
		"font_size":                    "1.00",
		"luna_first_launch_apk_type":   "normal_apk",
		"diversion_channel_name":       "xiaomi_8478_64",
		"is_car_play":                  "0",
		"battery":                      "0.99",
		"network_speed":                "10156",
		"hybrid_version_code":          "100198030",
		"tz_name":                      "Asia/Shanghai",
		"tz_offset":                    "28800",
		"luna_register_time":           "1784311292",
		"diversion_category_level_two": "Xiaomi%E5%95%86%E5%BA%97-%E8%87%AA%E7%84%B6",
		"package":                      "com.luna.music",
		"charge":                       "0",
		"luna_apk_type":                "normal_apk",
		"output_device_type":           "Phone",
		"volume":                       "1.00",
		"brightness":                   "0.08",
		"need_personal_recommend":      "1",
		"is_teen_mode":                 "0",
		"sim_region":                   "cn",
		"diversion_category_level_one": "%E5%8E%82%E5%95%86%E5%95%86%E5%BA%97-%E8%87%AA%E7%84%B6",
		"android_device_type":          "default",
		"iid":                          "2204957404569386",
		"device_id":                    "2204957404565290",
		"_rticket":                     strconv.FormatInt(time.Now().UnixMilli(), 10),
	}
	for key, value := range values {
		params.Set(key, value)
	}
	return params
}

func (s *Soda) fetchAndroidSearch(searchType, keyword string, page, pageSize int) ([]byte, error) {
	return utils.Get(sodaAndroidSearchURL(searchType, keyword, page, pageSize),
		s.androidSearchOptions()...)
}

func (s *Soda) androidSearchOptions() []utils.RequestOption {
	opts := []utils.RequestOption{
		utils.WithHeader("User-Agent", sodaAndroidSearchUserAgent),
		utils.WithHeader("content-type", "application/json; charset=UTF-8"),
	}
	if cookie := strings.TrimSpace(s.cookie); cookie != "" {
		opts = append(opts, utils.WithHeader("Cookie", cookie))
	}
	return opts
}
