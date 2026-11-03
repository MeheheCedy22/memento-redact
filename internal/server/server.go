package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/marek/memento-redact/config"
	"github.com/marek/memento-redact/internal/redactor"
	"github.com/marek/memento-redact/internal/rules"
)

type ruleStore struct {
	mu   sync.RWMutex
	sets []*rules.Ruleset
}

func (s *ruleStore) get() []*rules.Ruleset {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sets
}

// New creates an http.Handler that serves the memento-redact API and, if present, the web UI.
// It fails if the saved rulesets cannot be loaded, so a broken file never means "redact nothing".
func New(cfg *config.Config) (http.Handler, error) {
	sets, err := rules.Load(cfg.RulesPath)
	if err != nil {
		return nil, err
	}
	store := &ruleStore{sets: sets}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /v1/rulesets", makeGetRulesetsHandler(store))
	mux.HandleFunc("PUT /v1/rulesets", makePutRulesetsHandler(cfg, store))
	mux.HandleFunc("GET /v1/rulesets/defaults", makeDefaultRulesetsHandler(cfg))
	mux.HandleFunc("POST /v1/rulesets/reset", makeResetRulesetsHandler(cfg, store))
	mux.HandleFunc("POST /v1/preview", makePreviewHandler(cfg, store))
	mux.HandleFunc("POST /v1/proxy", makeProxyHandler(cfg, store))
	if cfg.StaticDir != "" {
		mux.Handle("GET /", http.FileServer(http.Dir(cfg.StaticDir)))
	}
	// No login, so stop other websites from making the browser reset, rewrite or proxy anything.
	return loggingMiddleware(new(http.CrossOriginProtection).Handler(mux)), nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// makePreviewHandler redacts a text/plain body with the saved rulesets, or a JSON body
// {"text": "...", "rulesets": [...]} with the given (possibly unsaved) rulesets.
func makePreviewHandler(cfg *config.Config, store *ruleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, ok := readBody(w, r, cfg.MaxPayloadSize)
		if !ok {
			return
		}
		text, sets := string(b), store.get()
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			var req struct {
				Text     string           `json:"text"`
				Rulesets []*rules.Ruleset `json:"rulesets"`
			}
			if err := json.Unmarshal(b, &req); err != nil {
				http.Error(w, "invalid preview payload: "+err.Error(), http.StatusBadRequest)
				return
			}
			text = req.Text
			if req.Rulesets != nil {
				if err := rules.Compile(req.Rulesets); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				sets = req.Rulesets
			}
		}
		writeJSON(w, redactor.Redact(text, sets))
	}
}

func makeProxyHandler(cfg *config.Config, store *ruleStore) http.HandlerFunc {
	client := &http.Client{Timeout: cfg.RequestTimeout}
	return func(w http.ResponseWriter, r *http.Request) {
		b, ok := readBody(w, r, cfg.MaxPayloadSize)
		if !ok {
			return
		}
		if cfg.ProviderURL == "" {
			http.Error(w, "provider not configured", http.StatusBadGateway)
			return
		}
		redacted := redactor.Redact(string(b), store.get())

		// forward only the redacted text to the provider
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, cfg.ProviderURL, bytes.NewReader([]byte(redacted.Redacted)))
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

// makeGetRulesetsHandler returns the saved rulesets as JSON, or as YAML with ?format=yaml.
func makeGetRulesetsHandler(store *ruleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("format") != "yaml" {
			writeJSON(w, store.get())
			return
		}
		data, err := rules.Marshal(store.get())
		if err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(data)
	}
}

// makePutRulesetsHandler validates and persists a full replacement of the rulesets (JSON or YAML body).
func makePutRulesetsHandler(cfg *config.Config, store *ruleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, ok := readBody(w, r, cfg.MaxPayloadSize)
		if !ok {
			return
		}
		sets, err := rules.Parse(b)
		if err != nil {
			http.Error(w, "invalid rulesets: "+err.Error(), http.StatusBadRequest)
			return
		}
		saveAndRespond(w, cfg, store, sets)
	}
}

func makeDefaultRulesetsHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defaults, err := rules.Load(cfg.DefaultRulesPath)
		if err != nil {
			http.Error(w, "failed to load default rulesets: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, defaults)
	}
}

func makeResetRulesetsHandler(cfg *config.Config, store *ruleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defaults, err := rules.Load(cfg.DefaultRulesPath)
		if err != nil {
			http.Error(w, "failed to load default rulesets: "+err.Error(), http.StatusInternalServerError)
			return
		}
		saveAndRespond(w, cfg, store, defaults)
	}
}

func saveAndRespond(w http.ResponseWriter, cfg *config.Config, store *ruleStore, sets []*rules.Ruleset) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := rules.Save(cfg.RulesPath, sets); err != nil {
		log.Printf("save rulesets: %v", err)
		http.Error(w, "failed to save rulesets", http.StatusInternalServerError)
		return
	}
	store.sets = sets
	writeJSON(w, sets)
}

// readBody reads at most limit bytes and rejects larger bodies instead of silently truncating them.
func readBody(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, bool) {
	b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "failed to read body", http.StatusBadRequest)
		}
		return nil, false
	}
	return b, true
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
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
