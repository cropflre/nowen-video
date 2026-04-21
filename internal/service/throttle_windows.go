//go:build windows

package service

import (
	"os"
	"syscall"
	"unsafe"
)

// Windows 进程挂起/恢复通过 NTDLL 的 NtSuspendProcess / NtResumeProcess 实现。
// 相较 SuspendThread 逐线程挂起，这两个未公开但广泛使用的 API 直接挂起整个进程，
// FFmpeg 这类多线程编码进程用它最稳妥。

var (
	modNtdll                = syscall.NewLazyDLL("ntdll.dll")
	procNtSuspendProcess    = modNtdll.NewProc("NtSuspendProcess")
	procNtResumeProcess     = modNtdll.NewProc("NtResumeProcess")
	modKernel32             = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess         = modKernel32.NewProc("OpenProcess")
	procCloseHandle         = modKernel32.NewProc("CloseHandle")
)

const (
	// PROCESS_SUSPEND_RESUME 权限
	processSuspendResume = 0x0800
)

// suspendProcess 挂起 ffmpeg 进程（包含其所有线程），并返回任何底层错误。
func suspendProcess(p *os.Process) error {
	if p == nil {
		return nil
	}
	h, err := openProcessHandle(uint32(p.Pid))
	if err != nil {
		return err
	}
	defer closeHandle(h)
	ret, _, callErr := procNtSuspendProcess.Call(uintptr(h))
	if ret != 0 {
		// NtSuspendProcess 返回 NTSTATUS，0 表示 STATUS_SUCCESS
		return callErr
	}
	return nil
}

// resumeProcess 恢复已挂起的 ffmpeg 进程。
func resumeProcess(p *os.Process) error {
	if p == nil {
		return nil
	}
	h, err := openProcessHandle(uint32(p.Pid))
	if err != nil {
		return err
	}
	defer closeHandle(h)
	ret, _, callErr := procNtResumeProcess.Call(uintptr(h))
	if ret != 0 {
		return callErr
	}
	return nil
}

func openProcessHandle(pid uint32) (uintptr, error) {
	h, _, err := procOpenProcess.Call(uintptr(processSuspendResume), 0, uintptr(pid))
	if h == 0 {
		return 0, err
	}
	return h, nil
}

func closeHandle(h uintptr) {
	if h == 0 {
		return
	}
	_, _, _ = procCloseHandle.Call(h)
}

// unused — 保留给未来使用（避免 go vet 抱怨未使用变量）
var _ = unsafe.Pointer(nil)
