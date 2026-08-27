package soda

import (
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
	sodaDouyinImageBaseURL     = "https://p3-luna.douyinpic.com/img/"
)

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
