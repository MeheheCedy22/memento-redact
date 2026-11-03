package config

import (
	"testing"
	"time"
)

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("LISTEN_ADDR", ":9090")
	t.Setenv("PROVIDER_URL", "https://provider.example/v1")
	t.Setenv("RULES_PATH", "/data/custom.yml")
	t.Setenv("MAX_PAYLOAD_SIZE", "2048")
	t.Setenv("REQUEST_TIMEOUT_SECONDS", "5s")

	cfg := Load()
	if cfg.ListenAddr != ":9090" || cfg.ProviderURL != "https://provider.example/v1" || cfg.RulesPath != "/data/custom.yml" {
		t.Fatalf("env values not applied: %+v", cfg)
	}
	if cfg.MaxPayloadSize != 2048 || cfg.RequestTimeout != 5*time.Second {
		t.Fatalf("unexpected limits: %+v", cfg)
	}
}

func TestLoadDefaultsOnMissingOrInvalidValues(t *testing.T) {
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("MAX_PAYLOAD_SIZE", "-1")
	t.Setenv("REQUEST_TIMEOUT_SECONDS", "soon")

	cfg := Load()
	if cfg.ListenAddr != ":8080" || cfg.MaxPayloadSize != 1<<20 || cfg.RequestTimeout != 15*time.Second {
		t.Fatalf("expected defaults, got %+v", cfg)
	}
	if cfg.DefaultRulesPath != "config/default-rulesets.yml" || cfg.StaticDir != "web/dist" {
		t.Fatalf("unexpected default paths: %+v", cfg)
	}
}
