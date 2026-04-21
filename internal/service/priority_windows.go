//go:build windows

package service

import (
	"os/exec"
	"syscall"
)

// setLowPriority 设置 FFmpeg 进程为最低优先级
// Windows: 通过 SysProcAttr 设置 IDLE_PRIORITY_CLASS (0x00000040)
// 确保转码进程在所有其他进程之后调度，不抢占系统资源
func setLowPriority(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000040, // IDLE_PRIORITY_CLASS
	}
}
