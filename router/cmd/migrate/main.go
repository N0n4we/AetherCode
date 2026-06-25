package main

import (
	"context"
	"log/slog"
	"os"

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
	if err := db.MigratePlatform(context.Background()); err != nil {
		logger.Error("migrate platform schema", "error", err)
		os.Exit(1)
	}
	if os.Getenv("IMPORT_LEGACY_PROVIDERS") == "true" {
		imported, err := db.ImportLegacyProvidersAsChannels(context.Background())
		if err != nil {
			logger.Error("import legacy providers as channels", "error", err)
			os.Exit(1)
		}
		logger.Info("legacy providers imported as provider channels", "imported", imported)
	}
	logger.Info("platform schema migration complete")
}
