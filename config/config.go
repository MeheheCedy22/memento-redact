package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds runtime configuration for the proxy.
type Config struct {
	ListenAddr       string
	ProviderURL      string
	MaxPayloadSize   int64
	RequestTimeout   time.Duration
	RulesPath        string
	DefaultRulesPath string
	StaticDir        string
}

// Load reads configuration from environment variables (see .env.example), falling back to defaults.
func Load() *Config {
	return &Config{
		ListenAddr:       getEnv("LISTEN_ADDR", ":8080"),
		ProviderURL:      getEnv("PROVIDER_URL", ""),
		RulesPath:        getEnv("RULES_PATH", "data/rulesets.yml"),
		DefaultRulesPath: getEnv("DEFAULT_RULES_PATH", "config/default-rulesets.yml"),
		StaticDir:        getEnv("STATIC_DIR", "web/dist"),
		MaxPayloadSize:   parseInt64(getEnv("MAX_PAYLOAD_SIZE", "1048576"), 1<<20),
		RequestTimeout:   parseDuration(getEnv("REQUEST_TIMEOUT_SECONDS", "15s"), 15*time.Second),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseInt64(value string, fallback int64) int64 {
	var parsed int64
	_, err := fmt.Sscan(value, &parsed)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseDuration(value string, fallback time.Duration) time.Duration {
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return fallback
	}
	return duration
}
