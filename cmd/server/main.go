package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MeheheCedy22/memento-redact/config"
	"github.com/MeheheCedy22/memento-redact/internal/rules"
	"github.com/MeheheCedy22/memento-redact/internal/server"
)

func main() {
	cfg := config.Load()
	if err := rules.Sync(cfg.RulesPath, cfg.DefaultRulesPath); err != nil {
		log.Fatalf("rulesets %s (presets %s): %v; fix the file, or delete it to start from the presets", cfg.RulesPath, cfg.DefaultRulesPath, err)
	}
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("load rulesets %s: %v", cfg.RulesPath, err)
	}
	if _, err := os.Stat(cfg.StaticDir); err != nil {
		log.Printf("web UI not found in %s (build it with: cd web && npm run build); API only", cfg.StaticDir)
	}

	// Header and body timeouts stop slow clients from holding connections open; no write timeout,
	// since redacting a large text with many rules can take seconds.
	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       time.Minute,
		IdleTimeout:       2 * time.Minute,
	}

	go func() {
		log.Printf("starting server on %s (rulesets: %s)", cfg.ListenAddr, cfg.RulesPath)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}
