package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// Viper and the working directory are process-wide; these tests must remain serial.
func setupPersistenceTest(t *testing.T) string {
	t.Helper()
	t.Chdir(t.TempDir())
	dataDir := t.TempDir()
	t.Setenv("NOWEN_APP_DATA_DIR", dataDir)
	t.Setenv("NOWEN_CACHE_CACHE_DIR", t.TempDir())
	t.Setenv("NOWEN_SECRETS_JWT_SECRET", "persistence-test-secret-only-1234567890")
	viper.Reset()
	t.Cleanup(viper.Reset)
	return dataDir
}

func reloadConfig(t *testing.T) *Config {
	t.Helper()
	viper.Reset() // Simulate a fresh process, without runtime viper.Set overrides.
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func writeTestConfig(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func makeConfigReadonly(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
}

func TestAIConfigSurvivesRestartInCustomDataDirectory(t *testing.T) {
	dataDir := setupPersistenceTest(t)
	legacy := filepath.Join("config", "ai.yaml")
	writeTestConfig(t, legacy, "model: old-model\nwhisper_model: keep-this-field\napi_base: https://old.example\n")
	makeConfigReadonly(t, legacy)
	cfg := reloadConfig(t)
	cfg.AI.Model = "saved-model"
	cfg.AI.APIBase = ""
	if err := cfg.SaveAIConfig(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "config", "ai.yaml")); err != nil {
		t.Fatal(err)
	}
	reloaded := reloadConfig(t)
	if reloaded.AI.Model != "saved-model" || reloaded.AI.APIBase != "" {
		t.Fatalf("AI configuration lost after restart: model=%q base=%q", reloaded.AI.Model, reloaded.AI.APIBase)
	}
	if viper.GetString("ai.whisper_model") != "keep-this-field" {
		t.Fatal("saving AI configuration removed an unmodeled legacy field")
	}
	content, _ := os.ReadFile(legacy)
	if !strings.Contains(string(content), "old-model") {
		t.Fatal("read-only deployment fragment was modified")
	}
}

func TestReadonlyMainConfigFallbackSurvivesRestart(t *testing.T) {
	dataDir := setupPersistenceTest(t)
	writeTestConfig(t, "config.yaml", "app:\n  port: 18080\n")
	makeConfigReadonly(t, "config.yaml")
	cfg := reloadConfig(t)
	viper.Set("app.port", 18081)
	if err := cfg.saveConfig(); err != nil {
		t.Fatal(err)
	}
	if !sameConfigPath(cfg.loadedConfigFile, filepath.Join(dataDir, "config.yaml")) {
		t.Fatalf("wrong fallback path: %s", cfg.loadedConfigFile)
	}
	if got := reloadConfig(t).App.Port; got != 18081 {
		t.Fatalf("saved port lost after restart: got %d", got)
	}
	content, _ := os.ReadFile("config.yaml")
	if !strings.Contains(string(content), "18080") {
		t.Fatal("read-only main configuration was modified")
	}
}

func TestConfigDataDirectoryFromDeploymentFiles(t *testing.T) {
	for _, source := range []string{"config.yaml", filepath.Join("config", "app.yaml")} {
		t.Run(source, func(t *testing.T) {
			setupPersistenceTest(t)
			t.Setenv("NOWEN_APP_DATA_DIR", "")
			body := "data_dir: persisted\n"
			if source == "config.yaml" {
				body = "app:\n  data_dir: persisted\n"
			}
			writeTestConfig(t, source, body)
			cfg := reloadConfig(t)
			cfg.AI.Model = "custom-directory-model"
			if err := cfg.SaveAIConfig(); err != nil {
				t.Fatal(err)
			}
			if got := reloadConfig(t).AI.Model; got != "custom-directory-model" {
				t.Fatalf("configuration written to data_dir was not reloaded: %s", got)
			}
		})
	}
}

func TestSecretsSaveAndClearOverrideReadonlyDeploymentFragment(t *testing.T) {
	setupPersistenceTest(t)
	writeTestConfig(t, filepath.Join("config", "secrets.yaml"), "tmdb_api_key: old-key\n")
	makeConfigReadonly(t, filepath.Join("config", "secrets.yaml"))
	for _, value := range []string{"new-key", ""} {
		cfg := reloadConfig(t)
		if err := cfg.SetTMDbAPIKey(value); err != nil {
			t.Fatal(err)
		}
		if got := reloadConfig(t).Secrets.TMDbAPIKey; got != value {
			t.Fatalf("saved key lost after restart: got %q, want %q", got, value)
		}
	}
}

func TestEnvironmentOverridesSavedAndDeploymentConfiguration(t *testing.T) {
	setupPersistenceTest(t)
	writeTestConfig(t, filepath.Join("config", "app.yaml"), "port: 18080\n")
	writeTestConfig(t, filepath.Join("config", "ai.yaml"), "model: deployment-model\n")
	cfg := reloadConfig(t)
	cfg.AI.Model = "saved-model"
	if err := cfg.SaveAIConfig(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NOWEN_APP_PORT", "18082")
	t.Setenv("NOWEN_AI_MODEL", "environment-model")
	reloaded := reloadConfig(t)
	if reloaded.App.Port != 18082 || reloaded.AI.Model != "environment-model" {
		t.Fatalf("environment precedence lost: port=%d model=%q", reloaded.App.Port, reloaded.AI.Model)
	}
}

func TestMalformedPersistedConfigIsReported(t *testing.T) {
	dataDir := setupPersistenceTest(t)
	writeTestConfig(t, filepath.Join(dataDir, "config.yaml"), "app: [broken yaml\n")
	if _, err := Load(); err == nil {
		t.Fatal("malformed saved configuration was silently ignored")
	}
}

func TestAtomicConfigWriteFailurePreservesTarget(t *testing.T) {
	setupPersistenceTest(t)
	// An existing directory cannot be replaced by a config file on any supported OS.
	blocked := filepath.Join("blocked", "config.yaml")
	original := filepath.Join(blocked, "original-content")
	writeTestConfig(t, original, "original-content")
	settings := viper.New()
	settings.Set("app.port", 18081)
	if err := writeConfigAtomically(settings, blocked); err == nil {
		t.Fatal("expected replacement error")
	}
	files, err := filepath.Glob(filepath.Join("blocked", ".nowen-config-*.yaml"))
	if err != nil || len(files) != 0 {
		t.Fatalf("temporary config files left behind: %v (%v)", files, err)
	}
	content, _ := os.ReadFile(original)
	if string(content) != "original-content" {
		t.Fatal("existing configuration was damaged")
	}
}

func TestWritableMainFileFallsBackWhenDirectoryRejectsReplacement(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix directory permissions and a non-root user")
	}
	dataDir := setupPersistenceTest(t)
	writeTestConfig(t, "config.yaml", "app:\n  port: 18080\n")
	cfg := reloadConfig(t)
	if !isWritableFile(cfg.loadedConfigFile) {
		t.Fatal("fixture main file must be writable")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cwd, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(cwd, 0o755) })
	viper.Set("app.port", 18081)
	if err := cfg.saveConfig(); err != nil {
		t.Fatal(err)
	}
	if !sameConfigPath(cfg.loadedConfigFile, filepath.Join(dataDir, "config.yaml")) {
		t.Fatalf("failed atomic replacement did not use data fallback: %s", cfg.loadedConfigFile)
	}
	if got := reloadConfig(t).App.Port; got != 18081 {
		t.Fatalf("fallback port lost after restart: %d", got)
	}
}
