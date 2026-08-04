package storagefault

import (
	"errors"
	"strings"
	"syscall"
)

const (
	CodeNoSpace          = "no_space"
	CodeReadOnly         = "read_only"
	CodePermissionDenied = "permission_denied"
	CodeUnavailable      = "unavailable"
	CodeIOError          = "io_error"
	CodeUnknown          = "unknown"

	SeverityCritical = "critical"
	SeverityWarning  = "warning"
)

type Classification struct {
	Code      string `json:"code"`
	Severity  string `json:"severity"`
	Retryable bool   `json:"retryable"`
}

func Classify(err error) Classification {
	if err == nil {
		return Classification{}
	}
	switch {
	case errors.Is(err, syscall.ENOSPC):
		return Classification{Code: CodeNoSpace, Severity: SeverityCritical, Retryable: true}
	case errors.Is(err, syscall.EROFS):
		return Classification{Code: CodeReadOnly, Severity: SeverityCritical, Retryable: false}
	case errors.Is(err, syscall.EACCES), errors.Is(err, syscall.EPERM):
		return Classification{Code: CodePermissionDenied, Severity: SeverityCritical, Retryable: false}
	case errors.Is(err, syscall.ENOENT), errors.Is(err, syscall.ENOTDIR), errors.Is(err, syscall.ENODEV), errors.Is(err, syscall.ENXIO):
		return Classification{Code: CodeUnavailable, Severity: SeverityCritical, Retryable: true}
	case errors.Is(err, syscall.EIO):
		return Classification{Code: CodeIOError, Severity: SeverityCritical, Retryable: true}
	}

	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "no space left"), strings.Contains(message, "disk full"):
		return Classification{Code: CodeNoSpace, Severity: SeverityCritical, Retryable: true}
	case strings.Contains(message, "read-only file system"):
		return Classification{Code: CodeReadOnly, Severity: SeverityCritical, Retryable: false}
	case strings.Contains(message, "permission denied"), strings.Contains(message, "operation not permitted"):
		return Classification{Code: CodePermissionDenied, Severity: SeverityCritical, Retryable: false}
	case strings.Contains(message, "stale file handle"), strings.Contains(message, "transport endpoint is not connected"), strings.Contains(message, "not a directory"), strings.Contains(message, "no such file or directory"):
		return Classification{Code: CodeUnavailable, Severity: SeverityCritical, Retryable: true}
	case strings.Contains(message, "input/output error"):
		return Classification{Code: CodeIOError, Severity: SeverityCritical, Retryable: true}
	default:
		return Classification{Code: CodeUnknown, Severity: SeverityWarning, Retryable: true}
	}
}
