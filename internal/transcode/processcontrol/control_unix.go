//go:build !windows

package processcontrol

import (
	"os"
	"syscall"
)

func suspend(process *os.Process) error {
	return process.Signal(syscall.SIGSTOP)
}

func resume(process *os.Process) error {
	return process.Signal(syscall.SIGCONT)
}
