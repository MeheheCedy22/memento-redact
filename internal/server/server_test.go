package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MeheheCedy22/memento-redact/config"
	"github.com/MeheheCedy22/memento-redact/internal/rules"
)

const emailRules = `- name: pii
  enabled: true
  rules:
    - name: email
      enabled: true
      type: regex
      patterns: ['[\w.]+@[\w.]+\.\w+']
      placeholder: '[EMAIL_#]'
`

func newTestConfig(t *testing.T) *config.Config {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		MaxPayloadSize:   1024,
		RulesPath:        filepath.Join(dir, "data", "rulesets.yml"),
		DefaultRulesPath: filepath.Join(dir, "defaults.yml"),
		StaticDir:        filepath.Join(dir, "web"),
	}
	if err := os.WriteFile(cfg.DefaultRulesPath, []byte(emailRules), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := rules.Sync(cfg.RulesPath, cfg.DefaultRulesPath); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func newHandler(t *testing.T, cfg *config.Config) http.Handler {
	t.Helper()
	h, err := New(cfg)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	return h
}

// do sends a request to h as a local client would: addressed to localhost.
func do(h http.Handler, method, path, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "http://localhost:8080"+path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func redacted(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}
	var res struct {
		Redacted string `json:"redacted"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return res.Redacted
}

func TestHealthEndpoint(t *testing.T) {
	rec := do(newHandler(t, newTestConfig(t)), http.MethodGet, "/health", "", "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("unexpected health response: %d %q", rec.Code, rec.Body.String())
	}
}

func TestPreviewUsesSavedRulesetsForPlainText(t *testing.T) {
	h := newHandler(t, newTestConfig(t))
	if got := redacted(t, do(h, http.MethodPost, "/v1/preview", "text/plain", "mail alice@example.com")); got != "mail [EMAIL_1]" {
		t.Fatalf("unexpected redaction: %q", got)
	}
}

func TestPreviewUsesUnsavedRulesetsFromJSON(t *testing.T) {
	h := newHandler(t, newTestConfig(t))
	body := `{"text":"ticket ID-7 for alice@example.com","rulesets":[{"name":"s","enabled":true,"rules":[{"name":"id","enabled":true,"type":"regex","patterns":["ID-\\d+"],"placeholder":"[TICKET_#]"}]}]}`
	if got := redacted(t, do(h, http.MethodPost, "/v1/preview", "application/json", body)); got != "ticket [TICKET_1] for alice@example.com" {
		t.Fatalf("unexpected redaction: %q", got)
	}

	bad := `{"text":"x","rulesets":[{"name":"s","enabled":true,"rules":[{"name":"broken","enabled":true,"type":"regex","patterns":["("]}]}]}`
	if rec := do(h, http.MethodPost, "/v1/preview", "application/json", bad); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "broken") {
		t.Fatalf("expected 400 naming the rule, got %d %q", rec.Code, rec.Body.String())
	}
}

func TestPreviewContinuesNumberingFromKnownEntries(t *testing.T) {
	h := newHandler(t, newTestConfig(t))
	body := `{"text":"bob@example.com, alice@example.com","known":[{"token":"[EMAIL_7]","original":"alice@example.com","placeholder":"[EMAIL_#]","numbers":[7]}]}`
	if got := redacted(t, do(h, http.MethodPost, "/v1/preview", "application/json", body)); got != "[EMAIL_8], [EMAIL_7]" {
		t.Fatalf("known entries must keep their token and numbering must continue: %q", got)
	}
}

func TestPreviewRejectsOversizedPayload(t *testing.T) {
	h := newHandler(t, newTestConfig(t))
	if rec := do(h, http.MethodPost, "/v1/preview", "text/plain", strings.Repeat("a", 2048)); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}

func TestRulesetsPersistAcrossRestartsAndInvalidSavesAreRejected(t *testing.T) {
	cfg := newTestConfig(t)
	h := newHandler(t, cfg)

	put := `[{"name":"org","enabled":true,"rules":[{"name":"words","enabled":true,"type":"words","patterns":["Phoenix"],"placeholder":"[PROJECT_#]"}]}]`
	if rec := do(h, http.MethodPut, "/v1/rulesets", "application/json", put); rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodPut, "/v1/rulesets", "application/json", `[{"name":"org","rules":[{"name":"x","type":"regex","patterns":["("]}]}]`); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid put should be 400, got %d", rec.Code)
	}

	restarted := newHandler(t, cfg)
	if got := redacted(t, do(restarted, http.MethodPost, "/v1/preview", "text/plain", "project phoenix")); got != "project [PROJECT_1]" {
		t.Fatalf("saved rules not used after restart: %q", got)
	}
	yamlRec := do(restarted, http.MethodGet, "/v1/rulesets?format=yaml", "", "")
	if !strings.Contains(yamlRec.Header().Get("Content-Type"), "yaml") || !strings.Contains(yamlRec.Body.String(), "- Phoenix") {
		t.Fatalf("unexpected yaml export: %q", yamlRec.Body.String())
	}

	// a YAML export can be imported back unchanged
	if rec := do(restarted, http.MethodPut, "/v1/rulesets", "application/yaml", yamlRec.Body.String()); rec.Code != http.StatusOK {
		t.Fatalf("yaml import: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSecurityHeaders(t *testing.T) {
	rec := do(newHandler(t, newTestConfig(t)), http.MethodGet, "/health", "", "")
	for header, want := range map[string]string{
		"Content-Security-Policy": "frame-ancestors 'none'",
		"X-Frame-Options":         "DENY",
		"X-Content-Type-Options":  "nosniff",
	} {
		if got := rec.Header().Get(header); !strings.Contains(got, want) {
			t.Errorf("%s = %q, want it to contain %q", header, got, want)
		}
	}
}

func TestPreviewReturnsRuleWarnings(t *testing.T) {
	h := newHandler(t, newTestConfig(t))
	body := `{"text":"host=web01 rest","rulesets":[{"name":"S","enabled":true,"rules":[{"name":"R","enabled":true,"type":"regex","patterns":["host=(.*)"]}]}]}`
	rec := do(h, http.MethodPost, "/v1/preview", "application/json", body)
	var res struct {
		Redacted string
		Warnings []rules.Warning
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil || res.Redacted != "host=[REDACTED_1]" || len(res.Warnings) != 1 || res.Warnings[0].Rule != "R" {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body.String())
	}
	if rec := do(h, http.MethodPost, "/v1/preview", "text/plain", "x"); strings.Contains(rec.Body.String(), "warnings") {
		t.Fatalf("careful rules must not get warnings: %s", rec.Body.String())
	}
}

func TestExportOneRuleset(t *testing.T) {
	h := newHandler(t, newTestConfig(t))
	put := `[{"name":"Org","enabled":true,"rules":[{"name":"w","enabled":true,"type":"words","patterns":["Phoenix"]}]},
		{"name":"Other","enabled":true,"rules":[{"name":"w","enabled":true,"type":"words","patterns":["Atlas"]}]}]`
	if rec := do(h, http.MethodPut, "/v1/rulesets", "application/json", put); rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	rec := do(h, http.MethodGet, "/v1/rulesets?format=yaml&name=org", "", "")
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "Phoenix") || strings.Contains(body, "Atlas") {
		t.Fatalf("one ruleset: %d %q", rec.Code, body)
	}
	// the file is still a list, so it can be imported
	if sets, err := rules.Parse([]byte(body)); err != nil || len(sets) != 1 {
		t.Fatalf("export does not parse back: %v %v", sets, err)
	}
	if rec := do(h, http.MethodGet, "/v1/rulesets?name=missing", "", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("missing ruleset: expected 404, got %d", rec.Code)
	}
}

func TestParseChecksAFileWithoutSaving(t *testing.T) {
	h := newHandler(t, newTestConfig(t))
	file := "- name: Extra\n  enabled: true\n  rules:\n    - {name: ids, enabled: true, type: regex, patterns: ['ID-\\d+']}\n"
	rec := do(h, http.MethodPost, "/v1/rulesets/parse", "application/yaml", file)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"Extra"`) || !strings.Contains(rec.Body.String(), `"placeholder":"[REDACTED_#]"`) {
		t.Fatalf("parse: %d %s", rec.Code, rec.Body.String())
	}
	if saved := do(h, http.MethodGet, "/v1/rulesets", "", ""); strings.Contains(saved.Body.String(), "Extra") {
		t.Fatalf("parse must not save: %s", saved.Body.String())
	}
	if rec := do(h, http.MethodPost, "/v1/rulesets/parse", "application/yaml", "- name: Bad\n  rules:\n    - {name: r, type: regex, patterns: ['(']}\n"); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"Bad"`) {
		t.Fatalf("invalid file: %d %s", rec.Code, rec.Body.String())
	}
}

func TestResetRestoresDefaults(t *testing.T) {
	cfg := newTestConfig(t)
	h := newHandler(t, cfg)
	if rec := do(h, http.MethodPut, "/v1/rulesets", "application/json", `[]`); rec.Code != http.StatusOK {
		t.Fatalf("put: %d", rec.Code)
	}
	if got := redacted(t, do(h, http.MethodPost, "/v1/preview", "text/plain", "a@b.com")); got != "a@b.com" {
		t.Fatalf("expected no redaction with empty rulesets, got %q", got)
	}
	if rec := do(h, http.MethodPost, "/v1/rulesets/reset", "", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"pii"`) {
		t.Fatalf("reset: %d %s", rec.Code, rec.Body.String())
	}
	if got := redacted(t, do(newHandler(t, cfg), http.MethodPost, "/v1/preview", "text/plain", "a@b.com")); got != "[EMAIL_1]" {
		t.Fatalf("defaults not persisted by reset: %q", got)
	}
	if rec := do(h, http.MethodGet, "/v1/rulesets/defaults", "", ""); !strings.Contains(rec.Body.String(), `"pii"`) {
		t.Fatalf("defaults endpoint: %s", rec.Body.String())
	}
}

func TestNewFailsOnBrokenRulesFile(t *testing.T) {
	cfg := newTestConfig(t)
	if err := os.WriteFile(cfg.RulesPath, []byte("- name: x\n  rules:\n    - name: y\n      type: regex\n      patterns: ['(']\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(cfg); err == nil {
		t.Fatal("a broken rules file must stop the server instead of redacting nothing")
	}
}

func TestServesWebUI(t *testing.T) {
	cfg := newTestConfig(t)
	if err := os.MkdirAll(cfg.StaticDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.StaticDir, "index.html"), []byte("<h1>ui</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rec := do(newHandler(t, cfg), http.MethodGet, "/", "", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ui") {
		t.Fatalf("unexpected UI response: %d %q", rec.Code, rec.Body.String())
	}
}

func TestRejectsCrossSiteWrites(t *testing.T) {
	h := newHandler(t, newTestConfig(t))
	req := httptest.NewRequest(http.MethodPost, "http://localhost:8080/v1/rulesets/reset", nil)
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a cross-site page must not reset rules, got %d", rec.Code)
	}
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("same-origin UI request should pass, got %d", rec.Code)
	}
}

func TestRejectsUnknownHostNames(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.AllowedHosts = []string{"Redact.Example.com"}
	h := newHandler(t, cfg)
	for host, want := range map[string]int{
		"evil.example:8080":         http.StatusForbidden, // DNS rebinding: an attacker's name pointing at 127.0.0.1
		"evil.example":              http.StatusForbidden,
		"localhost:8080":            http.StatusOK,
		"127.0.0.1:8080":            http.StatusOK,
		"[::1]:8080":                http.StatusOK,
		"192.168.1.20":              http.StatusOK,
		"redact.example.com":        http.StatusOK,
		"REDACT.example.com:443":    http.StatusOK,
		"redact.example.com.evil.x": http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodGet, "/v1/rulesets", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Host %q: got %d, want %d", host, rec.Code, want)
		}
	}
}
