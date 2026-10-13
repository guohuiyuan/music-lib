package soda

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// 汽水 App 端（LunaPC）整曲取流需要「应用级请求签名」头 x-helios / x-medusa。
//
// Web / 分享页端点（seo_track、device_platform=web 的 track_v2）对仅会员曲目只会下发
// 30s/60s 试听片段，与是否登录无关；整曲流走 POST /luna/pc/track_v2，服务端只认原生安全
// 组件（mssdk/metasecml.dll）逐请求生成的签名头，缺签名时返回 HTTP 200 + 空 body。
//
// 参考 sodam（github.com/sodahub-org/sodam）的做法：
//  1. Cookie 仍然来自用户（这里用 go-music-dl 保存的汽水 Cookie）；
//  2. 设备指纹 device_id / iid / fp 必须与签名器一致（签名器 /config 可查，或手动指定）；
//  3. 请求发出前把 {url, method, body, headers} 交给签名服务换取 x-helios / x-medusa。
//
// 签名只能由能跑官方客户端的机器（Windows/macOS）产生，因此签名器是一个可选的外部
// 服务：默认关闭，可用 SetSigner / WithSigner 或环境变量 QISHUI_SIGNER_URL、
// QISHUI_SIGNER_TOKEN 打开。签名器会看到这次请求的完整请求头（含 Cookie），只应指向
// 你自己信任的实例。
const (
	// SignerEnvURL 签名服务地址环境变量（与 Meting-API 同名约定）。
	SignerEnvURL = "QISHUI_SIGNER_URL"
	// SignerEnvToken 签名服务鉴权 token 环境变量（非必填）。
	SignerEnvToken = "QISHUI_SIGNER_TOKEN"

	defaultSignerTimeout = 8 * time.Second
)

// AppCredentials 是 App 端请求需要的设备指纹与静态签名头。
//
//   - DeviceID / IID / FP：URL 查询参数，必须与签名器同一设备；
//   - XHelios / XMedusa：抓包得到的静态签名（只对原请求可重放，一般留空走实时签名器）；
//   - UserAgent：抓包时的客户端 UA，留空用内置默认值。
type AppCredentials struct {
	DeviceID  string
	IID       string
	FP        string
	XHelios   string
	XMedusa   string
	UserAgent string
}

func (c AppCredentials) fpOrDefault() string {
	if value := strings.TrimSpace(c.FP); value != "" {
		return value
	}
	return strings.TrimSpace(c.DeviceID)
}

func (c AppCredentials) userAgentOrDefault() string {
	if value := strings.TrimSpace(c.UserAgent); value != "" {
		return value
	}
	return pcAppUserAgent
}

// staticSignatureHeaders 返回抓包得到的静态签名头（一般留空，由实时签名器覆盖）。
func (c AppCredentials) staticSignatureHeaders() map[string]string {
	headers := map[string]string{}
	if value := strings.TrimSpace(c.XHelios); value != "" {
		headers["x-helios"] = value
	}
	if value := strings.TrimSpace(c.XMedusa); value != "" {
		headers["x-medusa"] = value
	}
	return headers
}

// SignerConfig 描述一个外部签名服务。
type SignerConfig struct {
	URL     string
	Token   string
	Timeout time.Duration
}

// Option 用于 NewWithOptions 定制 Soda 实例。
type Option func(*Soda)

// WithSigner 打开实时签名器（参考 sodam 的签名服务 / libmssdk 的 /sign）。
func WithSigner(signerURL, token string) Option {
	return func(s *Soda) { s.SetSigner(signerURL, token) }
}

// WithAppCredentials 指定设备指纹（默认从签名器 /config 自动获取）。
func WithAppCredentials(creds AppCredentials) Option {
	return func(s *Soda) { s.SetAppCredentials(creds) }
}

// WithEnvSigner 按 QISHUI_SIGNER_URL / QISHUI_SIGNER_TOKEN 打开签名器。
func WithEnvSigner() Option {
	return func(s *Soda) {
		if signer := signerFromEnv(); signer != nil {
			s.signer = signer
		}
	}
}

func signerFromEnv() *SignerConfig {
	value := strings.TrimSpace(os.Getenv(SignerEnvURL))
	if value == "" {
		return nil
	}
	return &SignerConfig{URL: value, Token: strings.TrimSpace(os.Getenv(SignerEnvToken))}
}

// SetSigner 设置/替换签名器；url 为空时等价 ClearSigner。
func (s *Soda) SetSigner(signerURL, token string) *Soda {
	if s == nil {
		return s
	}
	signerURL = strings.TrimSpace(signerURL)
	if signerURL == "" {
		return s.ClearSigner()
	}
	s.signer = &SignerConfig{URL: signerURL, Token: strings.TrimSpace(token)}
	return s
}

// ClearSigner 关闭签名器（回到仅公开接口：免费整曲 + 会员曲试听）。
func (s *Soda) ClearSigner() *Soda {
	if s == nil {
		return s
	}
	s.signer = nil
	s.signerIdentity = nil
	return s
}

// SetAppCredentials 指定设备指纹；device_id 为空时回退到签名器自动发现。
func (s *Soda) SetAppCredentials(creds AppCredentials) *Soda {
	if s == nil {
		return s
	}
	creds.DeviceID = strings.TrimSpace(creds.DeviceID)
	creds.IID = strings.TrimSpace(creds.IID)
	creds.FP = strings.TrimSpace(creds.FP)
	creds.XHelios = strings.TrimSpace(creds.XHelios)
	creds.XMedusa = strings.TrimSpace(creds.XMedusa)
	creds.UserAgent = strings.TrimSpace(creds.UserAgent)
	s.appCreds = creds
	if creds.DeviceID != "" {
		identity := creds
		s.signerIdentity = &identity
	}
	return s
}

// HasSigner 是否有可用的签名器：本机 bdms.node 或远程签名服务。
func (s *Soda) HasSigner() bool {
	return s.hasRemoteSigner() || s.HasLocalSigner()
}

// hasRemoteSigner 是否配置了远程签名服务。
func (s *Soda) hasRemoteSigner() bool {
	return s != nil && s.signer != nil && strings.TrimSpace(s.signer.URL) != ""
}

// CanFetchSignedStream 是否能走 App 端的整曲取流链路（Cookie + 签名器）。
func (s *Soda) CanFetchSignedStream() bool {
	return s != nil && strings.TrimSpace(s.cookie) != "" && s.HasSigner()
}

// ensureAppCredentials 返回本次 App 请求要用的设备指纹：
// 显式设置优先，其次缓存；本机签名器用本地身份，远程签名器则向 /config 拉取（与签名器同设备）。
func (s *Soda) ensureAppCredentials() (AppCredentials, error) {
	if s == nil {
		return AppCredentials{}, errors.New("soda instance is nil")
	}
	if creds := s.appCreds; strings.TrimSpace(creds.DeviceID) != "" {
		return creds, nil
	}
	if s.signerIdentity != nil && strings.TrimSpace(s.signerIdentity.DeviceID) != "" {
		creds := *s.signerIdentity
		creds.XHelios = firstNonEmpty(creds.XHelios, s.appCreds.XHelios)
		creds.XMedusa = firstNonEmpty(creds.XMedusa, s.appCreds.XMedusa)
		creds.UserAgent = firstNonEmpty(creds.UserAgent, s.appCreds.UserAgent)
		return creds, nil
	}
	// 本机签名器自带设备身份（生成后会缓存到身份文件），无需远程 /config。
	if s.localSigner != nil {
		identity, err := s.localSigner.identity()
		if err != nil {
			return AppCredentials{}, err
		}
		creds := AppCredentials{
			DeviceID:  identity.DeviceID,
			IID:       identity.IID,
			FP:        identity.FP,
			XHelios:   s.appCreds.XHelios,
			XMedusa:   s.appCreds.XMedusa,
			UserAgent: s.appCreds.UserAgent,
		}
		s.signerIdentity = &creds
		return creds, nil
	}
	if !s.HasSigner() {
		return AppCredentials{}, errors.New("soda app stream requires signer or app credentials")
	}

	identity, err := s.fetchSignerIdentity()
	if err != nil {
		return AppCredentials{}, err
	}
	identity.XHelios = firstNonEmpty(identity.XHelios, s.appCreds.XHelios)
	identity.XMedusa = firstNonEmpty(identity.XMedusa, s.appCreds.XMedusa)
	identity.UserAgent = firstNonEmpty(identity.UserAgent, s.appCreds.UserAgent)
	s.signerIdentity = &identity
	return identity, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type sodaSignerConfigResponse struct {
	DeviceID string `json:"device_id"`
	Identity struct {
		DeviceID string `json:"device_id"`
		IID      string `json:"iid"`
		FP       string `json:"fp"`
	} `json:"identity"`
}

func (s *Soda) signerConfigURL() string {
	base := strings.TrimSpace(s.signer.URL)
	for _, suffix := range []string{"/sign", "/sign/"} {
		if strings.HasSuffix(base, suffix) {
			base = strings.TrimSuffix(base, suffix)
			break
		}
	}
	return strings.TrimRight(base, "/") + "/config"
}

func (s *Soda) fetchSignerIdentity() (AppCredentials, error) {
	req, err := http.NewRequest(http.MethodGet, s.signerConfigURL(), nil)
	if err != nil {
		return AppCredentials{}, err
	}
	req.Header.Set("Accept", "application/json")
	if token := strings.TrimSpace(s.signer.Token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := s.signerClient().Do(req)
	if err != nil {
		return AppCredentials{}, fmt.Errorf("soda signer config request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return AppCredentials{}, fmt.Errorf("soda signer config read failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return AppCredentials{}, fmt.Errorf("soda signer config status: %d", resp.StatusCode)
	}

	var parsed sodaSignerConfigResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return AppCredentials{}, fmt.Errorf("soda signer config json parse error: %w", err)
	}
	creds := AppCredentials{
		DeviceID: firstNonEmpty(parsed.Identity.DeviceID, parsed.DeviceID),
		IID:      strings.TrimSpace(parsed.Identity.IID),
		FP:       strings.TrimSpace(parsed.Identity.FP),
	}
	if creds.DeviceID == "" {
		return AppCredentials{}, errors.New("soda signer config missing device_id")
	}
	return creds, nil
}

func (s *Soda) signerClient() *http.Client {
	timeout := defaultSignerTimeout
	if s != nil && s.signer != nil && s.signer.Timeout > 0 {
		timeout = s.signer.Timeout
	}
	return &http.Client{Timeout: timeout}
}

// sodaSignRequest 与 libresoda / libmssdk 的 /sign 请求体保持一致。
type sodaSignRequest struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Body    string            `json:"body"`
	TSMs    int64             `json:"ts_ms"`
	Headers map[string]string `json:"headers,omitempty"`
}

type sodaSignResponse struct {
	OK       *bool             `json:"ok"`
	Error    string            `json:"error"`
	MSToken  string            `json:"ms_token"`
	ABogus   string            `json:"a_bogus"`
	DeviceID string            `json:"device_id"`
	Headers  map[string]string `json:"headers"`
}

// signStreamRequest 向签名器要一对 x-helios / x-medusa，并返回可能被追加查询参数的 URL
// 以及要补进请求头的键值。
//
// 优先级：显式配置的远程签名服务 > 本机签名器（进程内调 bdms.node）> 无签名。
func (s *Soda) signStreamRequest(reqURL, body string, headers map[string]string) (string, map[string]string, error) {
	if s.hasRemoteSigner() {
		return s.signStreamRequestRemote(reqURL, body, headers)
	}
	// 默认路径：进程内桥接官方客户端的 bdms.node，能不起服务就不起服务。
	if s.localSigner != nil && s.HasLocalSigner() {
		return s.signStreamRequestLocal(reqURL, body, headers)
	}
	return reqURL, map[string]string{}, nil
}

func (s *Soda) signStreamRequestLocal(reqURL, body string, headers map[string]string) (string, map[string]string, error) {
	identity, err := s.ensureAppCredentials()
	if err != nil {
		return reqURL, nil, err
	}
	signed, err := s.localSigner.sign(reqURL, body, headers, identity.DeviceID)
	if err != nil {
		return reqURL, nil, err
	}
	extra := make(map[string]string, len(signed))
	for key, value := range signed {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		extra[key] = value
	}
	if len(extra) == 0 {
		return reqURL, extra, errors.New("soda 本机签名返回空签名")
	}
	return reqURL, extra, nil
}

func (s *Soda) signStreamRequestRemote(reqURL, body string, headers map[string]string) (string, map[string]string, error) {
	payload := sodaSignRequest{
		URL:     reqURL,
		Method:  http.MethodPost,
		Body:    body,
		TSMs:    time.Now().UnixMilli(),
		Headers: copyStringMap(headers),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return reqURL, nil, fmt.Errorf("soda sign request encode error: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, s.signer.URL, bytes.NewReader(encoded))
	if err != nil {
		return reqURL, nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")
	if token := strings.TrimSpace(s.signer.Token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := s.signerClient().Do(req)
	if err != nil {
		return reqURL, nil, fmt.Errorf("soda signer request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return reqURL, nil, fmt.Errorf("soda signer read failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return reqURL, nil, fmt.Errorf("soda signer status: %d (%s)", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	parsed, err := parseSodaSignResponse(raw)
	if err != nil {
		return reqURL, nil, err
	}
	if parsed.DeviceID != "" && strings.TrimSpace(s.appCreds.DeviceID) == "" {
		if s.signerIdentity == nil {
			s.signerIdentity = &AppCredentials{}
		}
		if s.signerIdentity.DeviceID == "" {
			s.signerIdentity.DeviceID = parsed.DeviceID
		}
	}

	extra := map[string]string{}
	for key, value := range parsed.Headers {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		extra[key] = value
	}
	if len(extra) == 0 && parsed.MSToken == "" && parsed.ABogus == "" {
		return reqURL, extra, errors.New("soda signer returned empty signature")
	}
	return applySodaSignatureToURL(reqURL, parsed), extra, nil
}

func parseSodaSignResponse(raw []byte) (*sodaSignResponse, error) {
	var value map[string]json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("soda signer response parse error: %w", err)
	}

	var parsed sodaSignResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("soda signer response parse error: %w", err)
	}
	if parsed.OK != nil && !*parsed.OK {
		message := strings.TrimSpace(parsed.Error)
		if message == "" {
			message = "unknown error"
		}
		if strings.Contains(strings.ToLower(message), "unauthorized") {
			message += "（签名服务要求鉴权：设置 " + SignerEnvToken + " 或 WithSigner 的 token）"
		}
		return nil, fmt.Errorf("soda signer error: %s", message)
	}

	// 兼容 Meting-API 的扁平回包 {"ok":true,"X-Helios":"..","X-Medusa":".."}。
	for _, alias := range []string{"X-Helios", "x-helios", "X_Helios"} {
		if rawValue, ok := value[alias]; ok {
			var text string
			if json.Unmarshal(rawValue, &text) == nil && strings.TrimSpace(text) != "" {
				if parsed.Headers == nil {
					parsed.Headers = map[string]string{}
				}
				parsed.Headers["x-helios"] = text
			}
		}
	}
	for _, alias := range []string{"X-Medusa", "x-medusa", "X_Medusa"} {
		if rawValue, ok := value[alias]; ok {
			var text string
			if json.Unmarshal(rawValue, &text) == nil && strings.TrimSpace(text) != "" {
				if parsed.Headers == nil {
					parsed.Headers = map[string]string{}
				}
				parsed.Headers["x-medusa"] = text
			}
		}
	}
	return &parsed, nil
}

func applySodaSignatureToURL(reqURL string, signed *sodaSignResponse) string {
	if signed == nil || (signed.MSToken == "" && signed.ABogus == "") {
		return reqURL
	}
	parsed, err := url.Parse(reqURL)
	if err != nil {
		return reqURL
	}
	query := parsed.Query()
	if signed.MSToken != "" {
		query.Set("msToken", signed.MSToken)
	}
	if signed.ABogus != "" {
		query.Set("a_bogus", signed.ABogus)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func copyStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return map[string]string{}
	}
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

// sodaMD5HexUpper 是 App 端请求头 X-SS-STUB 的算法（body 的 MD5 大写十六进制）。
func sodaMD5HexUpper(data []byte) string {
	sum := md5.Sum(data)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

func sodaPCAppParamsWith(creds *AppCredentials) url.Values {
	now := time.Now().UnixMilli()
	deviceID := fmt.Sprintf("%d", now)
	iid := fmt.Sprintf("%d", now+1)
	fp := deviceID
	if creds != nil {
		if value := strings.TrimSpace(creds.DeviceID); value != "" {
			deviceID = value
			fp = value
		}
		if value := strings.TrimSpace(creds.IID); value != "" {
			iid = value
		}
		if value := creds.fpOrDefault(); value != "" {
			fp = value
		}
	}

	params := url.Values{}
	params.Set("aid", "386088")
	params.Set("app_name", "luna_pc")
	params.Set("region", "cn")
	params.Set("geo_region", "cn")
	params.Set("os_region", "cn")
	params.Set("sim_region", "")
	params.Set("device_id", deviceID)
	params.Set("cdid", "")
	params.Set("iid", iid)
	params.Set("version_name", "3.3.0")
	params.Set("version_code", "30030000")
	params.Set("channel", "official")
	params.Set("build_mode", "master")
	params.Set("network_carrier", "")
	params.Set("ac", "wifi")
	params.Set("tz_name", "Asia/Shanghai")
	params.Set("resolution", "")
	params.Set("device_platform", "windows")
	params.Set("device_type", "Windows")
	params.Set("os_version", "Windows 11")
	params.Set("fp", fp)
	return params
}

func sodaPCTrackV2URLWith(creds *AppCredentials) string {
	params := sodaPCAppParamsWith(creds)
	return "https://api.qishui.com/luna/pc/track_v2?" + params.Encode()
}
