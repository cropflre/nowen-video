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
	
	// 尝试查找 nice 命令的路径，如果找不到则直接执行原始命令（不设置优先级）
	nicePath, err := exec.LookPath("nice")
	if err != nil {
		// 如果找不到 nice 命令，直接使用原始命令，不设置优先级
		// 这比因为找不到 nice 而导致整个转码失败要好
		return
	}
	
	cmd.Path = nicePath
	cmd.Args = append([]string{"nice", "-n", "19", originalPath}, originalArgs[1:]...)
}
