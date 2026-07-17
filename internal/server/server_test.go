package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marek/memento-redact/config"
	"github.com/marek/memento-redact/internal/rules"
)

func TestHealthEndpoint(t *testing.T) {
	handler := New(&config.Config{ListenAddr: ":8080", MaxPayloadSize: 1024, RequestTimeout: 0, RulesPath: ""})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"status":"ok"}` {
		t.Fatalf("unexpected health body: %q", got)
	}
}

func TestPreviewEndpointRedactsContent(t *testing.T) {
	tmp := t.TempDir()
	rulesPath := filepath.Join(tmp, "rules.yml")
	if err := rules.SaveRules(rulesPath, []*rules.Rule{{Name: "email", Pattern: `([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`, Replacement: "[REDACTED_EMAIL]", Enabled: true}}); err != nil {
		t.Fatalf("save rules: %v", err)
	}
	handler := New(&config.Config{ListenAddr: ":8080", MaxPayloadSize: 1024, RequestTimeout: 0, RulesPath: rulesPath})
	req := httptest.NewRequest(http.MethodPost, "http://example.com/v1/preview", strings.NewReader("email alice@example.com"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var payload struct {
		Redacted string `json:"redacted"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	if payload.Redacted != "email [REDACTED_EMAIL]" {
		t.Fatalf("unexpected redacted content: %q", payload.Redacted)
	}
}

func TestProxyEndpointForwardsRedactedBody(t *testing.T) {
	backendBody := make(chan string, 1)
	providerErr := make(chan error, 1)
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			providerErr <- err
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		backendBody <- string(body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer provider.Close()

	tmp := t.TempDir()
	rulesPath := filepath.Join(tmp, "rules.yml")
	if err := rules.SaveRules(rulesPath, []*rules.Rule{{Name: "email", Pattern: `([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`, Replacement: "[REDACTED_EMAIL]", Enabled: true}}); err != nil {
		t.Fatalf("save rules: %v", err)
	}
	handler := New(&config.Config{ListenAddr: ":8080", ProviderURL: provider.URL, MaxPayloadSize: 1024, RequestTimeout: 5 * time.Second, RulesPath: rulesPath})
	req := httptest.NewRequest(http.MethodPost, "http://example.com/v1/proxy", bytes.NewBufferString("email alice@example.com"))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	select {
	case got := <-backendBody:
		if got != "email [REDACTED_EMAIL]" {
			t.Fatalf("proxy forwarded unredacted payload: %q", got)
		}
	case err := <-providerErr:
		t.Fatalf("provider read failed: %v", err)
	default:
		t.Fatal("provider did not receive a request")
	}
}

func TestRulesEndpointRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	rulesPath := filepath.Join(tmp, "rules.yml")
	handler := New(&config.Config{ListenAddr: ":8080", MaxPayloadSize: 1024, RequestTimeout: 0, RulesPath: rulesPath})

	payload := []byte(`[{"name":"email","pattern":"([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,})","replacement":"[REDACTED_EMAIL]","enabled":true}]`)
	req := httptest.NewRequest(http.MethodPut, "http://example.com/v1/rules", bytes.NewReader(payload))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status after PUT: %d body=%s", rec.Code, rec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "http://example.com/v1/rules", nil)
	getRec := httptest.NewRecorder()
	handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("unexpected status after GET: %d", getRec.Code)
	}
	if !strings.Contains(getRec.Body.String(), "REDACTED_EMAIL") {
		t.Fatalf("expected saved rule in GET response, got: %s", getRec.Body.String())
	}
}
