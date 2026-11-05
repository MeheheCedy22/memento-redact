package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/MeheheCedy22/memento-redact/config"
	"github.com/MeheheCedy22/memento-redact/internal/redactor"
	"github.com/MeheheCedy22/memento-redact/internal/rules"
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
	mux.HandleFunc("POST /v1/rulesets/parse", makeParseRulesetsHandler(cfg))
	mux.HandleFunc("GET /v1/rulesets/defaults", makeDefaultRulesetsHandler(cfg))
	mux.HandleFunc("POST /v1/rulesets/reset", makeResetRulesetsHandler(cfg, store))
	mux.HandleFunc("POST /v1/preview", makePreviewHandler(cfg, store))
	if cfg.StaticDir != "" {
		mux.Handle("GET /", http.FileServer(http.Dir(cfg.StaticDir)))
	}
	// No login, so stop other websites from making the browser read, reset or rewrite anything.
	return loggingMiddleware(allowHosts(cfg.AllowedHosts, new(http.CrossOriginProtection).Handler(mux))), nil
}

// allowHosts refuses requests addressed to host names other than localhost and the configured ones.
// Without it, a website could use DNS rebinding to reach this server as "same origin" and read the rules.
func allowHosts(names []string, next http.Handler) http.Handler {
	allowed := map[string]bool{"localhost": true}
	for _, name := range names {
		allowed[strings.ToLower(name)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		host = strings.ToLower(strings.Trim(host, "[]"))
		// rebinding needs a host name, so IP addresses are always safe
		if net.ParseIP(host) == nil && !allowed[host] {
			http.Error(w, "host "+host+" is not allowed; add it to ALLOWED_HOSTS", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
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
		var known []redactor.Entry
		if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			var req struct {
				Text     string           `json:"text"`
				Rulesets []*rules.Ruleset `json:"rulesets"`
				Known    []redactor.Entry `json:"known"` // map of earlier texts in the same conversation
			}
			if err := json.Unmarshal(b, &req); err != nil {
				http.Error(w, "invalid preview payload: "+err.Error(), http.StatusBadRequest)
				return
			}
			text, known = req.Text, req.Known
			if req.Rulesets != nil {
				if err := rules.Compile(req.Rulesets); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				sets = req.Rulesets
			}
		}
		res, err := redactor.Redact(r.Context(), text, sets, known)
		if err != nil {
			return // the client went away
		}
		// warnings travel with the preview, so the Rules page sees them for the rules being edited
		writeJSON(w, struct {
			redactor.Result
			Warnings []rules.Warning `json:"warnings,omitempty"`
		}{res, rules.Lint(sets)})
	}
}

// makeGetRulesetsHandler returns the saved rulesets as JSON, or as YAML with ?format=yaml.
// ?name= returns only that ruleset (any case), still as a list so the file can be imported back.
func makeGetRulesetsHandler(store *ruleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sets := store.get()
		if name := r.URL.Query().Get("name"); name != "" {
			var picked []*rules.Ruleset
			for _, set := range sets {
				if strings.EqualFold(set.Name, name) {
					picked = append(picked, set)
				}
			}
			if picked == nil {
				http.Error(w, fmt.Sprintf("ruleset %q not found", name), http.StatusNotFound)
				return
			}
			sets = picked
		}
		if r.URL.Query().Get("format") != "yaml" {
			writeJSON(w, sets)
			return
		}
		data, err := rules.Marshal(sets)
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

// makeParseRulesetsHandler checks a rules file (JSON or YAML) and returns it as JSON without saving, so the UI can pick from it.
func makeParseRulesetsHandler(cfg *config.Config) http.HandlerFunc {
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
		writeJSON(w, sets)
	}
}

func makeDefaultRulesetsHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defaults, err := rules.LoadDefaults(cfg.DefaultRulesPath)
		if err != nil {
			http.Error(w, "failed to load default rulesets: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, defaults)
	}
}

func makeResetRulesetsHandler(cfg *config.Config, store *ruleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defaults, err := rules.LoadDefaults(cfg.DefaultRulesPath)
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
