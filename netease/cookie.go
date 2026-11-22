package netease

import "strings"

// 网易云的播放接口会根据 Cookie 里的客户端标识决定请求能拿到的音质档位。
// 只带 MUSIC_U（浏览器里复制出来的 Cookie 通常只有登录字段）会被当成未知的
// Web 客户端：账号会员身份可能识别不到，高档请求会被静默降级成 lossless 或
// 更低档位，即使账号本身拥有 Hi-Res、空间音频或超清母带也一样。补上 PC 客户端
// 字段后，同一个账号才能拿到请求的档位（jymaster/hires/sky/vivid/dolby）。
//
// 移植自 go-music-dl#269，由 YupenBob 贡献。
const (
	neteaseClientOS     = "pc"
	neteaseClientAppVer = "8.9.70"
)

// withClientIdentity 补齐网易云要求的客户端标识字段，调用方已经提供非空值时不
// 覆盖。空值会原地替换，避免请求头里出现重复的 os/appver 键；空 Cookie 保持为
// 空，匿名请求不会伪装成已登录的桌面客户端。
func withClientIdentity(cookie string) string {
	cookie = strings.TrimSpace(cookie)
	if cookie == "" {
		return ""
	}

	parts := make([]string, 0, 4)
	seenOS, seenAppVer := false, false
	for _, part := range strings.Split(cookie, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, found := strings.Cut(part, "=")
		if !found {
			parts = append(parts, part)
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch {
		case strings.EqualFold(key, "os"):
			if value == "" {
				value = neteaseClientOS
			}
			seenOS = true
		case strings.EqualFold(key, "appver"):
			if value == "" {
				value = neteaseClientAppVer
			}
			seenAppVer = true
		}
		parts = append(parts, key+"="+value)
	}
	if !seenOS {
		parts = append(parts, "os="+neteaseClientOS)
	}
	if !seenAppVer {
		parts = append(parts, "appver="+neteaseClientAppVer)
	}
	return strings.Join(parts, "; ")
}
