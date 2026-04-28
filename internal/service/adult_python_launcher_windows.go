//go:build windows

package service

import (
	"os/exec"
	"syscall"
)

// configurePlatformProcAttr Windows 平台创建新进程组，便于整组 Ctrl-Break
func configurePlatformProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// terminateProcess Windows 上直接 Kill（Windows 的 Signal 支持有限，Flask 子进程无法正确响应 SIGTERM）
func terminateProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
