package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/marek/memento-redact/config"
	"github.com/marek/memento-redact/internal/redactor"
	"github.com/marek/memento-redact/internal/rules"
)

type ruleStore struct {
	mu    sync.RWMutex
	rules []*rules.Rule
}

// New creates an http.Handler that serves the memento-redact Proxy API.
func New(cfg *config.Config) http.Handler {
	store := &ruleStore{}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/v1/rules", makeRulesHandler(cfg, store))
	mux.HandleFunc("/v1/rules/defaults", makeDefaultRulesHandler(cfg))
	mux.HandleFunc("/v1/rules/reset", makeResetRulesHandler(cfg, store))
	mux.HandleFunc("/v1/preview", makePreviewHandler(cfg, store))
	mux.HandleFunc("/v1/proxy", makeProxyHandler(cfg, store))
	return loggingMiddleware(mux)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func makePreviewHandler(cfg *config.Config, store *ruleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		limited := io.LimitReader(r.Body, cfg.MaxPayloadSize)
		b, err := io.ReadAll(limited)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		ruleSet := loadRules(cfg, store)
		res := redactor.Redact(string(b), ruleSet)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(res); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
	}
}

func makeProxyHandler(cfg *config.Config, store *ruleStore) http.HandlerFunc {
	client := &http.Client{Timeout: cfg.RequestTimeout}
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		limited := io.LimitReader(r.Body, cfg.MaxPayloadSize)
		b, err := io.ReadAll(limited)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		// Preview redaction (audit)
		ruleSet := loadRules(cfg, store)
		redacted := redactor.Redact(string(b), ruleSet)

		// Forward to provider if configured
		if cfg.ProviderURL == "" {
			http.Error(w, "provider not configured", http.StatusBadGateway)
			return
		}

		// forward the captured bytes to the provider
		req, err := http.NewRequestWithContext(r.Context(), "POST", cfg.ProviderURL, bytes.NewReader([]byte(redacted.Redacted)))
		if err != nil {
			http.Error(w, "failed to create request", http.StatusInternalServerError)
			return
		}
		req.Header = r.Header.Clone()

		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "provider request failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}

func makeRulesHandler(cfg *config.Config, store *ruleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeRules(w, loadRules(cfg, store))
		case http.MethodPut:
			defer r.Body.Close()
			var incoming []*rules.Rule
			if err := json.NewDecoder(io.LimitReader(r.Body, cfg.MaxPayloadSize)).Decode(&incoming); err != nil {
				http.Error(w, "invalid rules payload", http.StatusBadRequest)
				return
			}
			for _, rule := range incoming {
				if rule == nil {
					http.Error(w, "rule cannot be null", http.StatusBadRequest)
					return
				}
				if rule.Enabled && rule.Pattern != "" {
					if _, err := regexp.Compile(rule.Pattern); err != nil {
						http.Error(w, "invalid rule pattern: "+err.Error(), http.StatusBadRequest)
						return
					}
				}
			}
			if err := rules.SaveRules(cfg.RulesPath, incoming); err != nil {
				http.Error(w, "failed to save rules", http.StatusInternalServerError)
				return
			}
			store.mu.Lock()
			store.rules = incoming
			store.mu.Unlock()
			writeRules(w, incoming)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func makeDefaultRulesHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeRules(w, loadRulesFromPath(cfg.DefaultRulesPath))
	}
}

func makeResetRulesHandler(cfg *config.Config, store *ruleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defaults := loadRulesFromPath(cfg.DefaultRulesPath)
		if err := rules.SaveRules(cfg.RulesPath, defaults); err != nil {
			http.Error(w, "failed to restore defaults", http.StatusInternalServerError)
			return
		}
		store.mu.Lock()
		store.rules = defaults
		store.mu.Unlock()
		writeRules(w, defaults)
	}
}

func loadRules(cfg *config.Config, store *ruleStore) []*rules.Rule {
	store.mu.RLock()
	if store.rules != nil {
		rulesCopy := append([]*rules.Rule(nil), store.rules...)
		store.mu.RUnlock()
		return rulesCopy
	}
	store.mu.RUnlock()

	loaded, err := rules.LoadRules(cfg.RulesPath)
	if err != nil {
		return nil
	}
	store.mu.Lock()
	store.rules = loaded
	store.mu.Unlock()
	return append([]*rules.Rule(nil), loaded...)
}

func loadRulesFromPath(path string) []*rules.Rule {
	loaded, err := rules.LoadRules(path)
	if err != nil {
		return []*rules.Rule{}
	}
	return loaded
}

func writeRules(w http.ResponseWriter, rr []*rules.Rule) {
	w.Header().Set("Content-Type", "application/json")
	if rr == nil {
		rr = []*rules.Rule{}
	}
	if err := json.NewEncoder(w).Encode(rr); err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
