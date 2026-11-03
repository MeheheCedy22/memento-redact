package config

import (
	"strings"
	"testing"
)

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("LISTEN_ADDR", ":9090")
	t.Setenv("RULES_PATH", "/data/custom.yml")
	t.Setenv("MAX_PAYLOAD_SIZE", "2048")
	t.Setenv("ALLOWED_HOSTS", "redact.example.com, myhost.lan")

	cfg := Load()
	if cfg.ListenAddr != ":9090" || cfg.RulesPath != "/data/custom.yml" || cfg.MaxPayloadSize != 2048 {
		t.Fatalf("env values not applied: %+v", cfg)
	}
	if strings.Join(cfg.AllowedHosts, "|") != "redact.example.com|myhost.lan" {
		t.Fatalf("unexpected allowed hosts: %q", cfg.AllowedHosts)
	}
}

func TestLoadDefaultsOnMissingOrInvalidValues(t *testing.T) {
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("ALLOWED_HOSTS", "")
	for _, size := range []string{"-1", "10MB", "2 000 000", "0x100"} {
		t.Setenv("MAX_PAYLOAD_SIZE", size)
		if cfg := Load(); cfg.MaxPayloadSize != 1<<20 {
			t.Errorf("MAX_PAYLOAD_SIZE=%q: expected the default, got %d", size, cfg.MaxPayloadSize)
		}
	}

	cfg := Load()
	if cfg.ListenAddr != ":8080" || len(cfg.AllowedHosts) != 0 {
		t.Fatalf("expected defaults, got %+v", cfg)
	}
	if cfg.DefaultRulesPath != "config/default-rulesets.yml" || cfg.StaticDir != "web/dist" {
		t.Fatalf("unexpected default paths: %+v", cfg)
	}
}
