package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestPulseRuntimeRemainsDeleted(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read full server main.go: %v", err)
	}

	for _, forbidden := range []string{
		`/pulse/dashboard`,
		`/pulse/analytics`,
		`handlers.Pulse`,
	} {
		if strings.Contains(string(mainSource), forbidden) {
			t.Fatalf("full server must not register retired Pulse runtime: found %q", forbidden)
		}
	}

	for _, path := range []string{
		"../../internal/handler/pulse.go",
		"../../internal/service/pulse.go",
		"../../internal/repository/repo_pulse.go",
	} {
		_, statErr := os.Stat(path)
		if statErr == nil {
			t.Fatalf("retired Pulse runtime file still exists: %s", path)
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("stat retired Pulse runtime file %s: %v", path, statErr)
		}
	}
}
