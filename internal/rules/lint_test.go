package rules

import (
	"path/filepath"
	"strings"
	"testing"
)

func lint(t *testing.T, typ string, patterns ...string) []string {
	t.Helper()
	sets := []*Ruleset{{Name: "S", Enabled: true, Rules: []*Rule{{Name: "R", Enabled: true, Type: typ, Patterns: patterns}}}}
	if err := Compile(sets); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, w := range Lint(sets) {
		out = append(out, w.Message)
	}
	return out
}

func TestLintWarns(t *testing.T) {
	cases := []struct {
		typ, pattern string
		want         []string // substrings, one per warning, in order
	}{
		{"regex", `(?i)\b([a-z0-9-]+)\.corp\.example.* \b`, []string{"Pattern 1: .* takes the rest of the line"}},
		{"regex", `password=(.+)`, []string{".* takes the rest"}},
		{"regex", `SRV-\d{3}`, []string{`no \b at the start`, `no \b at the end`}},
		{"regex", `\b(acme)`, []string{`no \b at the end`}},
		{"regex", `([5-7]\d+)\b`, []string{`no \b at the start`}},
		{"regex", `\w+`, []string{`Redacts ordinary text, like "The", "quick", "brown"`}},
		{"words", "the", []string{`Redacts ordinary text, like "The".`}},
	}
	for _, c := range cases {
		got := lint(t, c.typ, c.pattern)
		if len(got) != len(c.want) {
			t.Errorf("%s: got %q, want %d warnings", c.pattern, got, len(c.want))
			continue
		}
		for i := range got {
			if !strings.Contains(got[i], c.want[i]) {
				t.Errorf("%s: warning %d is %q, want it to contain %q", c.pattern, i+1, got[i], c.want[i])
			}
		}
	}
}

func TestLintAcceptsCarefulPatterns(t *testing.T) {
	for _, p := range []string{
		`(?i)\bSRV-\d{3}\b`,
		`(?i)\bpassword\s*[:=]\s*(\S+)`, // only the group is redacted, and \S+ runs to the end of the value
		`(?i)\b([a-z0-9-]+)\.corp\.example\b`,
		`-----BEGIN [A-Z ]+-----[\s\S]*?-----END [A-Z ]+-----`,
		`^\d+$`,
		`[\w.+-]+@[\w-]+\.[\w.-]+`,  // starts with a repeat of all word characters: cannot start inside a word
		`\b(?:\d{1,3}\.){3}\d{1,3}`, // a range is a length limit, not a missing \b
		`(?s)note:(.*?)\n`,          // non-greedy
	} {
		if got := lint(t, "regex", p); got != nil {
			t.Errorf("%s: unexpected warnings %q", p, got)
		}
	}
	if got := lint(t, "words", "Project Phoenix", "web01"); got != nil {
		t.Errorf("words: unexpected warnings %q", got)
	}
}

func TestDefaultPresetsHaveNoWarnings(t *testing.T) {
	sets, err := Load(filepath.Join("..", "..", "config", "default-rulesets.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range Lint(sets) {
		t.Errorf("%s / %s: %s", w.Ruleset, w.Rule, w.Message)
	}
}
