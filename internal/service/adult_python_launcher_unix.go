//go:build !windows

package service

import (
	"os/exec"
	"syscall"
)

// configurePlatformProcAttr 在 Unix 上为子进程创建新的进程组，便于整组终止
func configurePlatformProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateProcess 向整个进程组发送 SIGTERM
func terminateProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		return syscall.Kill(-pgid, syscall.SIGTERM)
	}
	return cmd.Process.Signal(syscall.SIGTERM)
}
