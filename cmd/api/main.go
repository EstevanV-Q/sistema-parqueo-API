package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"parking-backend/internal/app"
	"parking-backend/internal/config"
	"parking-backend/internal/platform/logger"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("configuración inválida", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg.LogLevel, os.Stdout)

	application, err := app.New(cfg)
	if err != nil {
		log.Error("error al iniciar la aplicación", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           application.Router,
		ReadHeaderTimeout: cfg.ReadTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	go func() {
		log.Info("servidor iniciado", "addr", srv.Addr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("error al iniciar servidor", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("apagando servidor...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("error en shutdown", "error", err)
	}
	log.Info("servidor detenido")
}
