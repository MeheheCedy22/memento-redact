package rules

import (
	"path/filepath"
	"testing"
)

func TestRuleApplyCompilesLazily(t *testing.T) {
	rule := &Rule{Name: "email", Pattern: `([a-z]+@[a-z]+\.[a-z]+)`, Replacement: "[REDACTED]", Enabled: true}
	got := rule.Apply("email alice@example.com")
	if got != "email [REDACTED]" {
		t.Fatalf("unexpected redaction: %q", got)
	}
}

func TestLoadAndSaveRulesRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.yml")
	input := []*Rule{
		{Name: "email", Pattern: `([a-z]+@[a-z]+\.[a-z]+)`, Replacement: "[REDACTED_EMAIL]", Enabled: true},
		{Name: "phone", Pattern: `\+?[0-9\-() ]+`, Replacement: "[REDACTED_PHONE]", Enabled: true},
	}
	if err := SaveRules(path, input); err != nil {
		t.Fatalf("save rules failed: %v", err)
	}

	loaded, err := LoadRules(path)
	if err != nil {
		t.Fatalf("load rules failed: %v", err)
	}
	if len(loaded) != len(input) {
		t.Fatalf("unexpected rule count: %d", len(loaded))
	}
	if got := loaded[0].Apply("alice@example.com"); got != "[REDACTED_EMAIL]" {
		t.Fatalf("unexpected loaded rule behavior: %q", got)
	}
}

func TestLoadRulesMissingFileReturnsNil(t *testing.T) {
	loaded, err := LoadRules(filepath.Join(t.TempDir(), "missing.yml"))
	if err != nil {
		t.Fatalf("missing rules file should not error: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected nil rules for missing file, got %d", len(loaded))
	}
}

func TestProductionRulesFileLoads(t *testing.T) {
	loaded, err := LoadRules(filepath.Join("..", "..", "config", "rules.yml"))
	if err != nil {
		t.Fatalf("production rules file should load: %v", err)
	}
	if len(loaded) < 10 {
		t.Fatalf("expected a production ruleset with many entries, got %d", len(loaded))
	}
	for _, rule := range loaded {
		if rule.Category == "" {
			t.Fatalf("expected rule category for %q", rule.Name)
		}
		if rule.Description == "" {
			t.Fatalf("expected rule description for %q", rule.Name)
		}
	}
}
