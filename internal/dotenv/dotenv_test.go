package dotenv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadParsesKeyValuePairs(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	contents := []byte("# comment\nAPP_NAME=memento-redact\nLISTEN_ADDR=:9090\nQUOTED='hello world'\n")
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("failed to create env file: %v", err)
	}

	if err := Load(path); err != nil {
		t.Fatalf("load env failed: %v", err)
	}
	if got := os.Getenv("APP_NAME"); got != "memento-redact" {
		t.Fatalf("unexpected APP_NAME: %q", got)
	}
	if got := os.Getenv("LISTEN_ADDR"); got != ":9090" {
		t.Fatalf("unexpected LISTEN_ADDR: %q", got)
	}
	if got := os.Getenv("QUOTED"); got != "hello world" {
		t.Fatalf("unexpected QUOTED: %q", got)
	}
}

func TestLoadIgnoresMissingFile(t *testing.T) {
	if err := Load(filepath.Join(t.TempDir(), "missing.env")); err != nil {
		t.Fatalf("missing env file should not error: %v", err)
	}
}
