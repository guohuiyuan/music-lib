//go:build windows
// +build windows

package soda

import (
	"os/exec"
	"syscall"
)

// hideLocalSignerWindow 避免签名子进程闪出控制台窗口。
func hideLocalSignerWindow(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}
