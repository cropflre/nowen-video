//go:build windows

package processcontrol

import (
	"fmt"
	"os"
	"syscall"
)

var (
	modNtdll             = syscall.NewLazyDLL("ntdll.dll")
	procNtSuspendProcess = modNtdll.NewProc("NtSuspendProcess")
	procNtResumeProcess  = modNtdll.NewProc("NtResumeProcess")
	modKernel32          = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess      = modKernel32.NewProc("OpenProcess")
	procCloseHandle      = modKernel32.NewProc("CloseHandle")
)

const processSuspendResume = 0x0800

func suspend(process *os.Process) error {
	handle, err := openProcessHandle(uint32(process.Pid))
	if err != nil {
		return err
	}
	defer closeHandle(handle)
	status, _, callErr := procNtSuspendProcess.Call(handle)
	if status != 0 {
		return fmt.Errorf("NtSuspendProcess status 0x%x: %w", status, callErr)
	}
	return nil
}

func resume(process *os.Process) error {
	handle, err := openProcessHandle(uint32(process.Pid))
	if err != nil {
		return err
	}
	defer closeHandle(handle)
	status, _, callErr := procNtResumeProcess.Call(handle)
	if status != 0 {
		return fmt.Errorf("NtResumeProcess status 0x%x: %w", status, callErr)
	}
	return nil
}

func openProcessHandle(pid uint32) (uintptr, error) {
	handle, _, callErr := procOpenProcess.Call(processSuspendResume, 0, uintptr(pid))
	if handle == 0 {
		return 0, callErr
	}
	return handle, nil
}

func closeHandle(handle uintptr) {
	if handle != 0 {
		_, _, _ = procCloseHandle.Call(handle)
	}
}
