package soda

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// 本机签名器：不依赖任何常驻服务，进程内直接调用官方客户端自带的 bdms.node。
//
// 为什么需要它：会员整曲流（POST /luna/pc/track_v2）要求 x-helios / x-medusa，
// 而这两个头只能由官方原生组件生成（bdms.node，底层 mssdk/metasecml.dll），
// 没有可复刻的纯软件实现。这里用内置的 JS 垫片把
// bdms.init({deviceId}) + bdms.generateHttpSignatureHeaders(url, headerLines)
// 跑起来：请求体走 stdin、签名头走 stdout，进程随用随起，不占端口、不需要服务。
//
// JS 运行时优先用 node，其次回退到官方客户端自带的 Electron
// （SodaMusic.exe + ELECTRON_RUN_AS_NODE=1），所以只要装了客户端就能用。
// 协议参考 sodam / libresoda 的 mssdk-bridge（AGPL-3.0）。
const (
	// LocalSignerEnvEnabled 本机签名器开关：0/off/false/no 关闭，1/on/true/yes 强制开启，留空自动探测。
	LocalSignerEnvEnabled = "QISHUI_LOCAL_SIGNER"
	// LocalSignerEnvRuntime 指定 JS 运行时（node 可执行文件）。
	LocalSignerEnvRuntime = "QISHUI_NODE"
	// LocalSignerEnvClientDir 指定汽水客户端安装目录。
	LocalSignerEnvClientDir = "QISHUI_CLIENT_DIR"
	// LocalSignerEnvBDMS 直接指定 bdms.node 路径。
	LocalSignerEnvBDMS = "QISHUI_BDMS_PATH"
	// LocalSignerEnvDeviceID 指定设备 did；留空时首次生成并缓存。
	LocalSignerEnvDeviceID = "QISHUI_DEVICE_ID"
	// LocalSignerEnvIdentity 设备身份缓存文件路径。
	LocalSignerEnvIdentity = "QISHUI_IDENTITY_FILE"

	localSignerDefaultTimeout = 20 * time.Second
)

//go:embed assets/qishui-bridge.cjs
var localSignerBridgeJS string

// LocalSignerOptions 描述本机签名器；零值即可用（自动探测运行时与 bdms.node）。
type LocalSignerOptions struct {
	// Runtime 是 JS 运行时；留空依次取 QISHUI_NODE、PATH 里的 node、客户端自带的 Electron。
	Runtime string
	// ClientDir 是汽水客户端安装目录；留空自动探测。
	ClientDir string
	// BDMSAppPath 直接指定 bdms.node。
	BDMSAppPath string
	// DeviceID / IID / FP 是设备身份；DeviceID 留空时生成并缓存到 IdentityFile。
	DeviceID string
	IID      string
	FP       string
	// Timeout 单次签名超时，默认 20s。
	Timeout time.Duration
	// IdentityFile 身份缓存文件，默认 <用户配置目录>/music-lib/soda-identity.json。
	IdentityFile string
}

type localSigner struct {
	opts LocalSignerOptions
}

type localSignerIdentity struct {
	DeviceID string `json:"device_id"`
	IID      string `json:"iid"`
	FP       string `json:"fp"`
}

// WithLocalSigner 打开本机签名器（进程内桥接官方客户端的 bdms.node，无需额外服务）。
func WithLocalSigner(opts LocalSignerOptions) Option {
	return func(s *Soda) { s.SetLocalSigner(opts) }
}

// SetLocalSigner 配置/替换本机签名器；依赖（node、bdms.node）缺失时 HasLocalSigner 返回 false。
func (s *Soda) SetLocalSigner(opts LocalSignerOptions) *Soda {
	if s == nil {
		return s
	}
	s.localSigner = &localSigner{opts: opts}
	return s
}

// ClearLocalSigner 关闭本机签名器。
func (s *Soda) ClearLocalSigner() *Soda {
	if s == nil {
		return s
	}
	s.localSigner = nil
	return s
}

// HasLocalSigner 本机签名器是否可用（已配置且运行时与 bdms.node 都存在）。
func (s *Soda) HasLocalSigner() bool {
	if s == nil || s.localSigner == nil {
		return false
	}
	_, _, err := s.localSigner.resolve()
	return err == nil
}

// LocalSignerAvailable 用默认配置探测本机签名器是否可用（供上层做状态展示）。
func LocalSignerAvailable() bool {
	return LocalSignerAvailableWithOptions(LocalSignerOptions{})
}

// LocalSignerAvailableWithOptions 用指定配置探测本机签名器是否可用。
// 上层可传入自定义的 ClientDir，以支持非默认位置的汽水音乐客户端。
func LocalSignerAvailableWithOptions(opts LocalSignerOptions) bool {
	if !localSignerAutoEnabled() {
		return false
	}
	signer := &localSigner{opts: opts}
	_, _, err := signer.resolve()
	return err == nil
}

func localSignerAutoEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(LocalSignerEnvEnabled))) {
	case "0", "off", "false", "no":
		return false
	case "1", "on", "true", "yes":
		return true
	}
	return true
}

// resolve 返回 (JS 运行时, bdms.node 路径, 错误)。
func (l *localSigner) resolve() (string, string, error) {
	opts := LocalSignerOptions{}
	if l != nil {
		opts = l.opts
	}
	clientDir := resolveLocalClientDir(opts.ClientDir)
	bdmsPath := resolveLocalBDMSPath(opts.BDMSAppPath, clientDir)
	if bdmsPath == "" {
		return "", "", errors.New("未找到 bdms.node：设置 QISHUI_BDMS_PATH / QISHUI_CLIENT_DIR，或安装汽水音乐客户端")
	}
	runtimePath, err := resolveLocalSignerRuntime(opts.Runtime, clientDir)
	if err != nil {
		return "", "", err
	}
	return runtimePath, bdmsPath, nil
}

func resolveLocalClientDir(explicit string) string {
	for _, candidate := range []string{strings.TrimSpace(explicit), strings.TrimSpace(os.Getenv(LocalSignerEnvClientDir))} {
		if candidate != "" {
			return candidate
		}
	}
	roots := make([]string, 0, 3)
	if localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localAppData != "" {
		roots = append(roots,
			filepath.Join(localAppData, "Programs", "Soda Music"),
			filepath.Join(localAppData, "Programs", "汽水音乐"),
		)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		roots = append(roots, filepath.Join(home, "Applications", "Soda Music.app", "Contents", "MacOS"))
	}
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		best := ""
		for _, entry := range entries {
			name := entry.Name()
			if !entry.IsDir() || len(strings.Split(name, ".")) != 3 {
				continue
			}
			if best == "" || name > best {
				best = name
			}
		}
		if best != "" {
			return filepath.Join(root, best)
		}
		return root
	}
	return ""
}

func resolveLocalBDMSPath(explicit, clientDir string) string {
	if value := strings.TrimSpace(explicit); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv(LocalSignerEnvBDMS)); value != "" {
		return value
	}
	if clientDir == "" {
		return ""
	}
	candidates := []string{
		filepath.Join(clientDir, "resources", "app.asar.unpacked", "bdms.node"),
		filepath.Join(clientDir, "resources", "app.asar.unpacked", "mssdk", "bdms.node"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func resolveLocalSignerRuntime(explicit, clientDir string) (string, error) {
	if value := strings.TrimSpace(explicit); value != "" {
		return value, nil
	}
	if value := strings.TrimSpace(os.Getenv(LocalSignerEnvRuntime)); value != "" {
		return value, nil
	}
	if path, err := exec.LookPath("node"); err == nil {
		return path, nil
	}
	// 回退：官方客户端自带 Electron，可以用 ELECTRON_RUN_AS_NODE 当 node 用。
	if clientDir != "" {
		candidates := []string{
			filepath.Join(clientDir, "SodaMusic.exe"),
			filepath.Join(clientDir, "Soda Music.exe"),
			filepath.Join(clientDir, "SodaMusic"),
		}
		for _, candidate := range candidates {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate, nil
			}
		}
		if runtime.GOOS == "darwin" {
			if entries, err := os.ReadDir(clientDir); err == nil {
				for _, entry := range entries {
					if strings.HasPrefix(entry.Name(), "Soda") {
						return filepath.Join(clientDir, entry.Name()), nil
					}
				}
			}
		}
	}
	return "", errors.New("未找到 JS 运行时：安装 Node.js 或设置 " + LocalSignerEnvRuntime)
}

// identity 返回本机签名器使用的设备身份；缺失时生成并写入缓存文件。
func (l *localSigner) identity() (localSignerIdentity, error) {
	opts := LocalSignerOptions{}
	if l != nil {
		opts = l.opts
	}
	identity := localSignerIdentity{
		DeviceID: firstNonEmpty(opts.DeviceID, os.Getenv(LocalSignerEnvDeviceID)),
		IID:      strings.TrimSpace(opts.IID),
		FP:       strings.TrimSpace(opts.FP),
	}

	path := localSignerIdentityPath(opts.IdentityFile)
	stored, readErr := readLocalSignerIdentity(path)
	if readErr == nil {
		identity.DeviceID = firstNonEmpty(identity.DeviceID, stored.DeviceID)
		identity.IID = firstNonEmpty(identity.IID, stored.IID)
		identity.FP = firstNonEmpty(identity.FP, stored.FP)
	}
	unchanged := readErr == nil &&
		identity.DeviceID == stored.DeviceID &&
		identity.IID == stored.IID &&
		identity.FP == stored.FP &&
		identity.DeviceID != ""
	if unchanged {
		return identity, nil
	}

	seed := time.Now().UnixMilli()
	if identity.DeviceID == "" {
		identity.DeviceID = localSignerDeviceID(seed)
	}
	if identity.IID == "" {
		identity.IID = localSignerDeviceID(seed + 1)
	}
	if identity.FP == "" {
		identity.FP = identity.DeviceID
	}
	// 缓存失败不影响签名，只是下次会重新生成。
	_ = writeLocalSignerIdentity(path, identity)
	return identity, nil
}

func localSignerIdentityPath(explicit string) string {
	if value := strings.TrimSpace(explicit); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv(LocalSignerEnvIdentity)); value != "" {
		return value
	}
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "music-lib", "soda-identity.json")
}

func readLocalSignerIdentity(path string) (localSignerIdentity, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return localSignerIdentity{}, err
	}
	var identity localSignerIdentity
	if err := json.Unmarshal(raw, &identity); err != nil {
		return localSignerIdentity{}, err
	}
	identity.DeviceID = strings.TrimSpace(identity.DeviceID)
	identity.IID = strings.TrimSpace(identity.IID)
	identity.FP = strings.TrimSpace(identity.FP)
	if identity.DeviceID == "" {
		return localSignerIdentity{}, errors.New("soda identity file missing device_id")
	}
	return identity, nil
}

func writeLocalSignerIdentity(path string, identity localSignerIdentity) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func localSignerDeviceID(seed int64) string {
	return fmt.Sprintf("%d%03d", seed, rand.Intn(1000))
}

func (l *localSigner) timeout() time.Duration {
	if l != nil && l.opts.Timeout > 0 {
		return l.opts.Timeout
	}
	return localSignerDefaultTimeout
}

// sign 起一个短命进程做一次签名：node（或客户端 Electron）执行内置垫片，
// 请求体走 stdin，签好的头走 stdout。
func (l *localSigner) sign(reqURL, body string, headers map[string]string, deviceID string) (map[string]string, error) {
	runtimePath, bdmsPath, err := l.resolve()
	if err != nil {
		return nil, err
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil, errors.New("soda 本机签名缺少 device_id")
	}

	payload, err := json.Marshal(map[string]interface{}{
		"url":     reqURL,
		"method":  "POST",
		"body":    body,
		"ts_ms":   time.Now().UnixMilli(),
		"headers": copyStringMap(headers),
	})
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), l.timeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, runtimePath, "-e", localSignerBridgeJS)
	cmd.Env = append(os.Environ(),
		"ELECTRON_RUN_AS_NODE=1",
		LocalSignerEnvBDMS+"="+bdmsPath,
		LocalSignerEnvDeviceID+"="+deviceID,
	)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	hideLocalSignerWindow(cmd)

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("soda 本机签名失败: %s", message)
	}

	var parsed struct {
		Headers map[string]string `json:"headers"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &parsed); err != nil {
		return nil, fmt.Errorf("soda 本机签名输出解析失败: %w", err)
	}
	if len(parsed.Headers) == 0 {
		return nil, errors.New("soda 本机签名输出为空")
	}
	return parsed.Headers, nil
}
