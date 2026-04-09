//go:build !windows

package service

import (
	"os/exec"
)

// setLowPriority 设置 FFmpeg 进程为最低优先级
// Linux/macOS: 通过在命令前添加 nice -n 19 实现
// nice 值范围 -20（最高优先级）到 19（最低优先级）
// 设置为 19 确保转码进程在所有其他进程之后调度，不抢占系统资源
func setLowPriority(cmd *exec.Cmd) {
	// 将原始命令包装为 nice -n 19 <原始命令>
	// 例如: nice -n 19 ffmpeg -y -i input.mkv ...
	originalPath := cmd.Path
	originalArgs := cmd.Args // Args[0] 是程序名
	cmd.Path = "/usr/bin/nice"
	cmd.Args = append([]string{"nice", "-n", "19", originalPath}, originalArgs[1:]...)
}
