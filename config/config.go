package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// Config holds runtime configuration for the server.
type Config struct {
	ListenAddr       string
	MaxPayloadSize   int64
	RulesPath        string
	DefaultRulesPath string
	StaticDir        string
	// AllowedHosts are host names besides localhost that requests may be addressed to; IP addresses always pass.
	AllowedHosts []string
}

// Load reads configuration from environment variables, falling back to defaults.
func Load() *Config {
	return &Config{
		ListenAddr:       getEnv("LISTEN_ADDR", ":8080"),
		RulesPath:        getEnv("RULES_PATH", "data/rulesets.yml"),
		DefaultRulesPath: getEnv("DEFAULT_RULES_PATH", "config/default-rulesets.yml"),
		StaticDir:        getEnv("STATIC_DIR", "web/dist"),
		MaxPayloadSize:   parseBytes("MAX_PAYLOAD_SIZE", 1<<20),
		AllowedHosts:     strings.Fields(strings.ReplaceAll(os.Getenv("ALLOWED_HOSTS"), ",", " ")),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// parseBytes reads a whole number of bytes; anything else (e.g. "10MB") is reported and replaced by fallback.
func parseBytes(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		log.Printf("warning: %s=%q is not a whole number of bytes; using %d", key, value, fallback)
		return fallback
	}
	return parsed
}
