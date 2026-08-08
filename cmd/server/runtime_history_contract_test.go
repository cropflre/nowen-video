package main

import (
	"os"
	"strings"
	"testing"
)

func TestRuntimeHistoryRoutesAreSharedAndReadOnly(t *testing.T) {
	checks := []struct {
		path    string
		markers []string
	}{
		{"main.go", []string{
			`admin.GET("/runtime-history", runtimeHistoryHandler.List)`,
			`admin.GET("/runtime-history/summary", runtimeHistoryHandler.Summary)`,
			`admin.GET("/runtime-history/jobs/:id", runtimeHistoryHandler.Detail)`,
		}},
		{"../server-lite/routes_admin.go", []string{
			`admin.GET("/runtime-history", runtimeHistory.List)`,
			`admin.GET("/runtime-history/summary", runtimeHistory.Summary)`,
			`admin.GET("/runtime-history/jobs/:id", runtimeHistory.Detail)`,
		}},
	}
	for _, check := range checks {
		content, err := os.ReadFile(check.path)
		if err != nil {
			t.Fatalf("read %s: %v", check.path, err)
		}
		source := string(content)
		for _, marker := range check.markers {
			if !strings.Contains(source, marker) {
				t.Fatalf("%s missing read-only history route %q", check.path, marker)
			}
		}
		for _, forbidden := range []string{
			`POST("/runtime-history`, `PUT("/runtime-history`, `PATCH("/runtime-history`, `DELETE("/runtime-history`,
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s exposes mutating history route %q", check.path, forbidden)
			}
		}
	}

	serviceSource, err := os.ReadFile("../../internal/service/runtime_history.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"Submit", "ClaimJob", "RenewJobLease", "exec.Command", "CancelJob", "RetryJob"} {
		if strings.Contains(string(serviceSource), forbidden) {
			t.Fatalf("runtime history service regained execution capability %q", forbidden)
		}
	}
}
