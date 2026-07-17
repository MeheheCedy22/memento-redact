package redactor

import (
	"testing"

	"github.com/marek/memento-redact/internal/rules"
)

func TestRedactEmailAndPhone(t *testing.T) {
	in := "Contact: alice@example.com or +1 (555) 123-4567"
	res := Redact(in, nil)
	if res.Redacted == in {
		t.Fatalf("expected redaction to change input")
	}
	if len(res.Diffs) == 0 {
		t.Fatalf("expected diffs to be present")
	}
}

func TestRedactRuleApplication(t *testing.T) {
	rule := &rules.Rule{Name: "mask id", Pattern: `ID-\d+`, Replacement: "[REDACTED_ID]", Enabled: true}
	ruleTest := []*rules.Rule{rule}
	res := Redact("ref ID-12345 here", ruleTest)
	if got := res.Redacted; got != "ref [REDACTED_ID] here" {
		t.Fatalf("unexpected redacted text: %q", got)
	}
}
