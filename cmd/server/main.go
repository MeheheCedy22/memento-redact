package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/marek/memento-redact/config"
	"github.com/marek/memento-redact/internal/dotenv"
	"github.com/marek/memento-redact/internal/server"
)

func main() {
	if err := dotenv.Load(".env"); err != nil {
		log.Printf("warning: load .env: %v", err)
	}

	cfg, err := config.LoadConfig("config/config.yml")
	if err != nil {
		log.Printf("warning: load config: %v", err)
	}

	srv := server.New(cfg)

	httpSrv := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: srv,
	}

	go func() {
		log.Printf("starting server on %s", cfg.ListenAddr)
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
