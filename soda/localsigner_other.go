//go:build !windows
// +build !windows

package soda

import "os/exec"

// hideLocalSignerWindow 只在 Windows 需要。
func hideLocalSignerWindow(cmd *exec.Cmd) {}
