package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds runtime configuration for the proxy.
type Config struct {
	AppName          string        `yaml:"app_name"`
	ListenAddr       string        `yaml:"listen_addr"`
	ProviderURL      string        `yaml:"provider_url"`
	MaxPayloadSize   int64         `yaml:"max_payload_size"`
	RequestTimeout   time.Duration `yaml:"request_timeout_seconds"`
	RulesPath        string        `yaml:"rules_path"`
	DefaultRulesPath string        `yaml:"default_rules_path"`
}

// LoadConfig loads configuration from a YAML file. If the file does not exist,
// returns a sensible default configuration.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		AppName:          getEnv("APP_NAME", "memento-redact"),
		ListenAddr:       getEnv("LISTEN_ADDR", ":8080"),
		ProviderURL:      getEnv("PROVIDER_URL", ""),
		RulesPath:        getEnv("RULES_PATH", "config/rules.yml"),
		DefaultRulesPath: getEnv("DEFAULT_RULES_PATH", "config/default-rules.yml"),
		MaxPayloadSize:   parseInt64(getEnv("MAX_PAYLOAD_SIZE", "1048576"), 1<<20),
		RequestTimeout:   parseDuration(getEnv("REQUEST_TIMEOUT_SECONDS", "15s"), 15*time.Second),
	}

	f, err := os.Open(path)
	if err != nil {
		return cfg, nil
	}
	defer f.Close()

	if err := yaml.NewDecoder(f).Decode(cfg); err != nil {
		return nil, err
	}
	if cfg.AppName == "" {
		cfg.AppName = "memento-redact"
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8080"
	}
	if cfg.MaxPayloadSize == 0 {
		cfg.MaxPayloadSize = 1 << 20
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 15 * time.Second
	}
	if cfg.RulesPath == "" {
		cfg.RulesPath = "config/rules.yml"
	}
	if cfg.DefaultRulesPath == "" {
		cfg.DefaultRulesPath = "config/default-rules.yml"
	}
	return cfg, nil
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
