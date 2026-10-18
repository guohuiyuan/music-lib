package soda

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalSignerIdentityPersists(t *testing.T) {
	identityPath := filepath.Join(t.TempDir(), "nested", "soda-identity.json")
	t.Setenv(LocalSignerEnvDeviceID, "")
	t.Setenv(LocalSignerEnvIdentity, "")

	signer := &localSigner{opts: LocalSignerOptions{IdentityFile: identityPath}}
	first, err := signer.identity()
	if err != nil {
		t.Fatalf("identity error: %v", err)
	}
	if first.DeviceID == "" || first.IID == "" || first.FP != first.DeviceID {
		t.Fatalf("生成的设备身份不完整: %#v", first)
	}
	if _, err := os.Stat(identityPath); err != nil {
		t.Fatalf("身份文件未落盘: %v", err)
	}

	second, err := signer.identity()
	if err != nil {
		t.Fatalf("identity 二次读取失败: %v", err)
	}
	if second != first {
		t.Fatalf("设备身份应稳定: %#v vs %#v", first, second)
	}
}

func TestLocalSignerHonoursExplicitDeviceID(t *testing.T) {
	identityPath := filepath.Join(t.TempDir(), "soda-identity.json")
	signer := &localSigner{opts: LocalSignerOptions{DeviceID: "1234567890123456", IdentityFile: identityPath}}
	identity, err := signer.identity()
	if err != nil {
		t.Fatalf("identity error: %v", err)
	}
	if identity.DeviceID != "1234567890123456" || identity.FP != "1234567890123456" {
		t.Fatalf("显式 device_id 未生效: %#v", identity)
	}
}

func TestLocalSignerUnavailableWithoutClient(t *testing.T) {
	t.Setenv(LocalSignerEnvBDMS, "")
	t.Setenv(LocalSignerEnvClientDir, t.TempDir())
	// 关掉 node 探测，避免"运行时可用但 bdms 缺失"被误判为可用。
	t.Setenv(LocalSignerEnvRuntime, "")

	s := (&Soda{}).SetLocalSigner(LocalSignerOptions{ClientDir: t.TempDir()})
	if s.HasLocalSigner() {
		t.Fatalf("没有 bdms.node 时不应报告本机签名器可用")
	}
	if s.HasSigner() {
		t.Fatalf("没有 bdms.node 且无远程签名器时 HasSigner 应为 false")
	}
	gotURL, extra, err := s.signStreamRequest("https://api.qishui.com/x", "{}", map[string]string{"A": "b"})
	if err != nil {
		t.Fatalf("无可用签名器时不应报错: %v", err)
	}
	if gotURL != "https://api.qishui.com/x" || len(extra) != 0 {
		t.Fatalf("无可用签名器时不应改动请求: %s %#v", gotURL, extra)
	}
}

func TestLocalSignerAvailableWithExplicitClientDir(t *testing.T) {
	baseDir := t.TempDir()
	clientDir := filepath.Join(baseDir, "Soda Music", "3.1.0")
	bdmsPath := filepath.Join(clientDir, "resources", "app.asar.unpacked", "bdms.node")
	if err := os.MkdirAll(filepath.Dir(bdmsPath), 0o755); err != nil {
		t.Fatalf("create client resources: %v", err)
	}
	if err := os.WriteFile(bdmsPath, []byte("test"), 0o644); err != nil {
		t.Fatalf("write bdms.node: %v", err)
	}
	runtimePath := filepath.Join(baseDir, "node")
	if err := os.WriteFile(runtimePath, []byte("test"), 0o755); err != nil {
		t.Fatalf("write runtime: %v", err)
	}

	t.Setenv(LocalSignerEnvEnabled, "1")
	t.Setenv(LocalSignerEnvClientDir, "")
	t.Setenv(LocalSignerEnvBDMS, "")
	t.Setenv(LocalSignerEnvRuntime, runtimePath)

	if !LocalSignerAvailableWithOptions(LocalSignerOptions{ClientDir: clientDir}) {
		t.Fatal("自定义 ClientDir 下应检测到本机签名器")
	}
	if LocalSignerAvailableWithOptions(LocalSignerOptions{ClientDir: filepath.Join(baseDir, "missing")}) {
		t.Fatal("缺失的 ClientDir 不应报告本机签名器可用")
	}
}

// 需要本机装有汽水客户端（bdms.node）才能跑：
//
//	QISHUI_LOCAL_SIGNER_TEST=1 go test ./soda -run TestLocalSignerSignsWithClient -v
func TestLocalSignerSignsWithClient(t *testing.T) {
	if strings.TrimSpace(os.Getenv("QISHUI_LOCAL_SIGNER_TEST")) == "" {
		t.Skip("QISHUI_LOCAL_SIGNER_TEST not set")
	}
	s := &Soda{}
	s.SetLocalSigner(LocalSignerOptions{
		IdentityFile: filepath.Join(t.TempDir(), "soda-identity.json"),
	})
	if !s.HasLocalSigner() {
		t.Skip("本机没有可用的 bdms.node / node")
	}
	signedURL, extra, err := s.signStreamRequest(
		"https://api.qishui.com/luna/pc/track_v2?aid=386088&device_id=1&fp=1",
		`{"track_id":"7501674235158431760","media_type":"track"}`,
		map[string]string{
			"User-Agent": pcAppUserAgent,
			"X-SS-STUB":  "ABCD",
			"Cookie":     "sessionid_ss=test",
		},
	)
	if err != nil {
		t.Fatalf("本机签名失败: %v", err)
	}
	if signedURL == "" {
		t.Fatalf("签名后的 URL 为空")
	}
	helios := extra["X-Helios"]
	medusa := extra["X-Medusa"]
	if helios == "" || medusa == "" {
		t.Fatalf("缺少签名头: %#v", extra)
	}
	if len(helios) < 24 || len(medusa) < 200 {
		t.Fatalf("签名头长度异常: helios=%d medusa=%d", len(helios), len(medusa))
	}
	t.Logf("本机签名成功: X-Helios=%d bytes, X-Medusa=%d bytes", len(helios), len(medusa))
}
