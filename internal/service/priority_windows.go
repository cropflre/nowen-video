//go:build windows

package service

import (
	"os/exec"
)

// setLowPriority 设置 FFmpeg 进程为最低优先级
// Windows: 通过 SysProcAttr 设置 IDLE_PRIORITY_CLASS
// 确保转码进程在所有其他进程之后调度，不抢占系统资源
func setLowPriority(cmd *exec.Cmd) {
	// Windows 上暂不设置优先级（需要 syscall.SysProcAttr 的 CreationFlags）
	// 主要部署环境为 Linux（NAS/Docker），Windows 仅用于开发
	_ = cmd
}
