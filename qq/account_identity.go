package qq

import (
	"strconv"
	"strings"
)

// accountUIN returns the numeric QQ account identifier carried by a login cookie.
// QQ QR login may provide either the standard uin cookie or the musicid value
// from the WeChat login response. Unauthenticated clients continue to use 0.
func (q *QQ) accountUIN() int64 {
	if q == nil {
		return 0
	}

	var musicID int64
	for _, part := range strings.Split(q.cookie, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "uin":
			if id, valid := parseQQAccountUIN(value); valid {
				return id
			}
		case "musicid":
			if id, valid := parseQQAccountUIN(value); valid {
				musicID = id
			}
		}
	}
	return musicID
}

func parseQQAccountUIN(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "o")
	if value == "" {
		return 0, false
	}
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil && id >= 0
}
