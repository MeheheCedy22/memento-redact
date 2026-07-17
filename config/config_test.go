package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfigDefaultsWhenFileMissing(t *testing.T) {
	t.Setenv("APP_NAME", "memento-redact")
	t.Setenv("LISTEN_ADDR", ":9090")
	t.Setenv("PROVIDER_URL", "https://provider.example/v1")
	t.Setenv("RULES_PATH", "config/custom-rules.yml")
	t.Setenv("MAX_PAYLOAD_SIZE", "2048")
	t.Setenv("REQUEST_TIMEOUT_SECONDS", "5s")

	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yml"))
	if err != nil {
		t.Fatalf("expected missing file to return defaults, got error: %v", err)
	}
	if cfg.ListenAddr != ":9090" {
		t.Fatalf("unexpected listen addr: %s", cfg.ListenAddr)
	}
	if cfg.ProviderURL != "https://provider.example/v1" {
		t.Fatalf("unexpected provider url: %s", cfg.ProviderURL)
	}
	if cfg.MaxPayloadSize != 2048 {
		t.Fatalf("unexpected payload size: %d", cfg.MaxPayloadSize)
	}
	if cfg.RequestTimeout != 5*time.Second {
		t.Fatalf("unexpected timeout: %s", cfg.RequestTimeout)
	}
	if cfg.RulesPath != "config/custom-rules.yml" {
		t.Fatalf("unexpected rules path: %s", cfg.RulesPath)
	}
}

func TestLoadConfigFileOverridesEnvironment(t *testing.T) {
	t.Setenv("APP_NAME", "memento-redact")
	t.Setenv("LISTEN_ADDR", ":9090")
	t.Setenv("PROVIDER_URL", "https://provider.example/v1")
	t.Setenv("RULES_PATH", "config/custom-rules.yml")
	t.Setenv("MAX_PAYLOAD_SIZE", "2048")
	t.Setenv("REQUEST_TIMEOUT_SECONDS", "5s")

	path := filepath.Join(t.TempDir(), "config.yml")
	contents := []byte("app_name: memento-redact\nlisten_addr: :8081\nprovider_url: https://api.example/v1\nmax_payload_size: 4096\nrequest_timeout_seconds: 12s\nrules_path: config/rules.yml\n")
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config failed: %v", err)
	}
	if cfg.ListenAddr != ":8081" || cfg.ProviderURL != "https://api.example/v1" {
		t.Fatalf("file values should override env values: %+v", cfg)
	}
	if cfg.MaxPayloadSize != 4096 {
		t.Fatalf("unexpected payload size: %d", cfg.MaxPayloadSize)
	}
	if cfg.RequestTimeout != 12*time.Second {
		t.Fatalf("unexpected timeout: %s", cfg.RequestTimeout)
	}
}
