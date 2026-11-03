package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		RequestTimeout:   5 * time.Second,
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

func do(h http.Handler, method, path, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
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

func TestProxyForwardsRedactedBody(t *testing.T) {
	got := make(chan string, 1)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		got <- string(body)
		_, _ = w.Write([]byte("ok"))
	}))
	defer provider.Close()

	cfg := newTestConfig(t)
	cfg.ProviderURL = provider.URL
	req := httptest.NewRequest(http.MethodPost, "/v1/proxy", bytes.NewBufferString("email alice@example.com"))
	rec := httptest.NewRecorder()
	newHandler(t, cfg).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if body := <-got; body != "email [EMAIL_1]" {
		t.Fatalf("proxy forwarded unredacted payload: %q", body)
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
	req := httptest.NewRequest(http.MethodPost, "/v1/rulesets/reset", nil)
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
