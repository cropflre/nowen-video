//go:build !windows

package service

import (
	"os"
	"syscall"
)

// Unix 下通过 SIGSTOP / SIGCONT 挂起/恢复 ffmpeg 进程。
// 注意：不能使用 SIGTSTP，该信号可被进程忽略。

func suspendProcess(p *os.Process) error {
	if p == nil {
		return nil
	}
	return p.Signal(syscall.SIGSTOP)
}

func resumeProcess(p *os.Process) error {
	if p == nil {
		return nil
	}
	return p.Signal(syscall.SIGCONT)
}
