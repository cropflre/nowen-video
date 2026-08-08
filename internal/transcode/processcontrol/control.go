package processcontrol

import "os"

// Suspend pauses every thread in a media process. Platform implementations use
// SIGSTOP/SIGCONT on Unix and NtSuspendProcess/NtResumeProcess on Windows.
func Suspend(process *os.Process) error {
	if process == nil {
		return nil
	}
	return suspend(process)
}

// Resume restores a process previously paused by Suspend.
func Resume(process *os.Process) error {
	if process == nil {
		return nil
	}
	return resume(process)
}
