package storagefault

import (
	"fmt"
	"syscall"
	"testing"
)

func TestClassifyKernelStorageFaults(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		code      string
		retryable bool
	}{
		{name: "no space", err: fmt.Errorf("write probe: %w", syscall.ENOSPC), code: CodeNoSpace, retryable: true},
		{name: "read only", err: fmt.Errorf("create probe: %w", syscall.EROFS), code: CodeReadOnly, retryable: false},
		{name: "permission", err: fmt.Errorf("create probe: %w", syscall.EACCES), code: CodePermissionDenied, retryable: false},
		{name: "unavailable", err: fmt.Errorf("stat mount: %w", syscall.ENODEV), code: CodeUnavailable, retryable: true},
		{name: "io error", err: fmt.Errorf("sync probe: %w", syscall.EIO), code: CodeIOError, retryable: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			classified := Classify(test.err)
			if classified.Code != test.code || classified.Retryable != test.retryable || classified.Severity != SeverityCritical {
				t.Fatalf("unexpected classification: %+v", classified)
			}
		})
	}
}

func TestClassifyNASMessageFallbacks(t *testing.T) {
	classified := Classify(fmt.Errorf("rename failed: stale file handle"))
	if classified.Code != CodeUnavailable || !classified.Retryable {
		t.Fatalf("stale mount was not classified as unavailable: %+v", classified)
	}
}
