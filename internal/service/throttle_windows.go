//go:build windows

package service

import (
	"os"

	"github.com/nowen-video/nowen-video/internal/transcode/processcontrol"
)

func suspendProcess(process *os.Process) error {
	return processcontrol.Suspend(process)
}

func resumeProcess(process *os.Process) error {
	return processcontrol.Resume(process)
}
