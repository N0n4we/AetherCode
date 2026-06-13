package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"aethercode-router/internal/app"
	"aethercode-router/internal/config"
	"aethercode-router/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := config.Load()

	db, err := store.Open(cfg.SQLDSN)
	if err != nil {
		logger.Error("open database", "error", err)
		os.Exit(1)
	}

	cache := store.NewCache()
	if err := store.ReloadCache(context.Background(), db, cache); err != nil {
		logger.Error("load provider cache", "error", err)
		os.Exit(1)
	}

	srv := app.New(cfg, db, cache, logger)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go store.SyncCache(ctx, db, cache, cfg.ConfigSyncInterval, logger)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("router listening", "addr", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("listen", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "error", err)
	}
}
