package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func found(t *testing.T, r *Rule, text string) []string {
	t.Helper()
	var out []string
	for _, m := range r.Find(text) {
		out = append(out, text[m[0]:m[1]])
	}
	return out
}

func compiled(t *testing.T, r *Rule) *Rule {
	t.Helper()
	if err := Compile([]*Ruleset{{Name: "test", Rules: []*Rule{r}}}); err != nil {
		t.Fatalf("compile: %v", err)
	}
	return r
}

func TestWordsAreCaseInsensitiveWholeWordsLongestFirst(t *testing.T) {
	r := compiled(t, &Rule{Name: "w", Type: "words", Patterns: []string{" acme ", "acme corp", "", "Čederle", "+420"}})
	got := found(t, r, "ACME Corp, acmeville, Acme, pan Čederle, tel +420.")
	want := []string{"ACME Corp", "Acme", "Čederle", "+420"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q, want %q", got, want)
	}
	if len(r.Patterns) != 4 || r.Patterns[0] != "acme" {
		t.Fatalf("patterns should be trimmed and empty lines dropped: %q", r.Patterns)
	}
}

func TestRegexRedactsFirstCaptureGroupAndHonorsKeep(t *testing.T) {
	r := compiled(t, &Rule{Name: "r", Type: "regex", Patterns: []string{`pass=(\S+)`, `\b\d+\.\d+\.\d+\.\d+\b`}, Keep: `^10\.`})
	got := found(t, r, "pass=hunter2 10.0.0.1 8.8.8.8")
	if strings.Join(got, "|") != "hunter2|8.8.8.8" {
		t.Fatalf("unexpected matches: %q", got)
	}
	if r.Placeholder != DefaultPlaceholder {
		t.Fatalf("expected default placeholder, got %q", r.Placeholder)
	}
}

func TestCompileRejectsInvalidRules(t *testing.T) {
	cases := map[string]*Ruleset{
		"invalid regex":    {Name: "s", Rules: []*Rule{{Name: "r", Type: "regex", Patterns: []string{"("}}}},
		"invalid keep":     {Name: "s", Rules: []*Rule{{Name: "r", Type: "words", Keep: "["}}},
		"unknown type":     {Name: "s", Rules: []*Rule{{Name: "r", Type: "glob"}}},
		"missing name":     {Name: "s", Rules: []*Rule{{Type: "words"}}},
		"missing set name": {Name: " "},
	}
	for name, set := range cases {
		if err := Compile([]*Ruleset{set}); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	err := Compile([]*Ruleset{{Name: "Net", Rules: []*Rule{{Name: "IP", Type: "regex", Patterns: []string{"("}}}}})
	if err == nil || !strings.Contains(err.Error(), `"Net"`) || !strings.Contains(err.Error(), `"IP"`) {
		t.Fatalf("error should name ruleset and rule: %v", err)
	}
}

func TestParseAcceptsJSONAndYAML(t *testing.T) {
	jsonSets, err := Parse([]byte(`[{"name":"s","enabled":true,"rules":[{"name":"r","enabled":true,"type":"regex","patterns":["\\bID-\\d+"]}]}]`))
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	yamlSets, err := Parse([]byte("- name: s\n  enabled: true\n  rules:\n    - name: r\n      enabled: true\n      type: regex\n      patterns: ['\\bID-\\d+']\n"))
	if err != nil {
		t.Fatalf("yaml: %v", err)
	}
	for _, sets := range [][]*Ruleset{jsonSets, yamlSets} {
		if got := found(t, sets[0].Rules[0], "ref ID-42"); len(got) != 1 || got[0] != "ID-42" {
			t.Fatalf("unexpected matches: %q", got)
		}
	}
}

func TestSaveLoadRoundTripAndSeeding(t *testing.T) {
	dir := t.TempDir()
	defaults := filepath.Join(dir, "defaults.yml")
	path := filepath.Join(dir, "data", "rulesets.yml")
	if err := os.WriteFile(defaults, []byte("- name: seed\n  enabled: true\n  rules: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSeeded(path, defaults); err != nil {
		t.Fatalf("seed: %v", err)
	}
	sets, err := Load(path)
	if err != nil || len(sets) != 1 || sets[0].Name != "seed" {
		t.Fatalf("seeded load: %v %+v", err, sets)
	}

	sets[0].Name = "edited"
	if err := Save(path, sets); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := EnsureSeeded(path, defaults); err != nil {
		t.Fatalf("seed existing: %v", err)
	}
	sets, err = Load(path)
	if err != nil || sets[0].Name != "edited" {
		t.Fatalf("seeding must not overwrite saved rules: %v %+v", err, sets)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temp files left behind: %v", entries)
	}
}

func TestLoadMissingFileFails(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yml")); err == nil {
		t.Fatal("a missing rules file must be an error, not an empty ruleset")
	}
}

// TestDefaultPresets checks every factory rule against inputs it must and must not redact.
func TestDefaultPresets(t *testing.T) {
	sets, err := Load(filepath.Join("..", "..", "config", "default-rulesets.yml"))
	if err != nil {
		t.Fatalf("default rulesets must load: %v", err)
	}
	cases := []struct {
		rule  string
		input string
		want  []string
	}{
		{"Hosts in our domain", "web01.prod.domain.tld, DB01.domain.tld, alice@domain.tld, domain.tld.evil.com", []string{"web01.prod", "DB01"}},
		{"Company domain", "web01.domain.tld mail@Domain.TLD notdomain.tld", []string{"domain.tld", "Domain.TLD"}},
		{"Names and codenames", "nothing configured yet", nil},
		{"Public IPv4", "10.1.2.3 172.16.0.1 172.32.0.1 192.168.1.1 100.64.0.1 127.0.0.1 169.254.1.1 255.255.255.0 224.0.0.1 8.8.8.8 203.0.113.17 999.1.1.1", []string{"172.32.0.1", "8.8.8.8", "203.0.113.17"}},
		{"Public IPv6", "2001:db8:85a3::8a2e:370:7334 fe80::250:56ff:fe9a:1b2c fd00::1 ff02::1 12:30:45 00:50:56:9a:1b:2c 2001:0db8:0000:0000:0000:ff00:0042:8329", []string{"2001:db8:85a3::8a2e:370:7334", "2001:0db8:0000:0000:0000:ff00:0042:8329"}},
		{"MAC address", "00:50:56:9a:1b:2c and 00-50-56-9A-1B-2C", []string{"00:50:56:9a:1b:2c", "00-50-56-9A-1B-2C"}},
		{"Email", "mail jane.doe+x@corp.example.com now", []string{"jane.doe+x@corp.example.com"}},
		{"Phone number", "+420 777 123 456, +1 (555) 123-4567, 555-123-4567, +420777123456, 10.20.30.40, 2024", []string{"+420 777 123 456", "+1 (555) 123-4567", "555-123-4567", "+420777123456"}},
		{"Key-value secrets", `password=hunter2 "apiKey": "abc123" db_password: s3cr3t -Dspring.datasource.password=jdbc tokens are fine`, []string{"hunter2", "abc123", "s3cr3t", "jdbc"}},
		{"Key-value secrets", "JIRA_PASS=<api_token>\nexport SMTP_PASSWORD=\"correct horse battery\"\nldap.pass: 'a b'\nDB_PASS=\"unclosed\nempty_token=\"\"", []string{"<api_token>", "correct horse battery", "a b", "unclosed"}},
		{"Command-line password flags", `Connect-VIServer vc -User admin -Password 'S3cr3t!' --token=abc mkdir -p dir`, []string{"'S3cr3t!'", "abc"}},
		{"PowerShell plain-text SecureString", `ConvertTo-SecureString "PlainTextPass" -AsPlainText -Force`, []string{`"PlainTextPass"`}},
		{"Auth headers", "Authorization: Bearer abc.def\nCookie: sid=1; csrftoken=2\nHost: x", []string{"Bearer abc.def", "sid=1; csrftoken=2"}},
		{"Credentials in URLs", "mysql://app:s3cret@db.example.com/app https://example.com/a:b", []string{"app:s3cret"}},
		{"Private keys", "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNz\n-----END OPENSSH PRIVATE KEY-----", []string{"-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNz\n-----END OPENSSH PRIVATE KEY-----"}},
		{"Well-known token formats", "eyJhbGciOi.eyJzdWIiOi.SflKxwRJ ghp_abcdefghijklmnopqrstuvwxyz AKIAIOSFODNN7EXAMPLE sk-abcdefghijklmnopqrstuv", []string{"eyJhbGciOi.eyJzdWIiOi.SflKxwRJ", "ghp_abcdefghijklmnopqrstuvwxyz", "AKIAIOSFODNN7EXAMPLE", "sk-abcdefghijklmnopqrstuv"}},
	}
	tested := map[string]bool{}
	for _, tc := range cases {
		t.Run(tc.rule, func(t *testing.T) {
			r := findRule(t, sets, tc.rule)
			tested[tc.rule] = true
			if got := found(t, r, tc.input); strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	for _, set := range sets {
		for _, r := range set.Rules {
			if !tested[r.Name] {
				t.Errorf("default rule %q has no test case", r.Name)
			}
		}
	}
}

func findRule(t *testing.T, sets []*Ruleset, name string) *Rule {
	t.Helper()
	for _, set := range sets {
		for _, r := range set.Rules {
			if r.Name == name {
				return r
			}
		}
	}
	t.Fatalf("rule %q not found", name)
	return nil
}
