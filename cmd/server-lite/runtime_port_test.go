package main

import (
	"testing"

	"github.com/nowen-video/nowen-video/internal/config"
)

func TestApplyRuntimePortOverridePrefersNowenPort(t *testing.T) {
	t.Setenv("NOWEN_APP_PORT", "28888")
	t.Setenv("SERVER_PORT", "29999")

	cfg := &config.Config{}
	cfg.App.Port = 8080

	if err := applyRuntimePortOverride(cfg); err != nil {
		t.Fatalf("applyRuntimePortOverride() error = %v", err)
	}
	if cfg.App.Port != 28888 {
		t.Fatalf("App.Port = %d, want 28888", cfg.App.Port)
	}
}

func TestApplyRuntimePortOverrideFallsBackToServerPort(t *testing.T) {
	t.Setenv("NOWEN_APP_PORT", "")
	t.Setenv("SERVER_PORT", "28890")

	cfg := &config.Config{}
	cfg.App.Port = 8080

	if err := applyRuntimePortOverride(cfg); err != nil {
		t.Fatalf("applyRuntimePortOverride() error = %v", err)
	}
	if cfg.App.Port != 28890 {
		t.Fatalf("App.Port = %d, want 28890", cfg.App.Port)
	}
}

func TestApplyRuntimePortOverrideRejectsInvalidPort(t *testing.T) {
	t.Setenv("NOWEN_APP_PORT", "70000")
	t.Setenv("SERVER_PORT", "")

	cfg := &config.Config{}
	cfg.App.Port = 8080

	if err := applyRuntimePortOverride(cfg); err == nil {
		t.Fatal("applyRuntimePortOverride() error = nil, want validation error")
	}
	if cfg.App.Port != 8080 {
		t.Fatalf("App.Port changed to %d after invalid override", cfg.App.Port)
	}
}
