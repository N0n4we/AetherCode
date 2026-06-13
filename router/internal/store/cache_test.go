package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCacheSelectPriorityAndExclude(t *testing.T) {
	cache := NewCache()
	cache.Replace([]Provider{
		{ID: 1, Provider: "openai", Models: "gpt-4", Groups: "default", Status: StatusEnabled, Priority: 10},
		{ID: 2, Provider: "openai", Models: "gpt-4", Groups: "default", Status: StatusEnabled, Priority: 5},
	}, 7)

	selected, err := cache.Select("default", "gpt-4", nil)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if selected.ID != 1 {
		t.Fatalf("expected provider 1, got %d", selected.ID)
	}

	selected, err = cache.Select("default", "gpt-4", map[uint]bool{1: true})
	if err != nil {
		t.Fatalf("select with exclude: %v", err)
	}
	if selected.ID != 2 {
		t.Fatalf("expected provider 2, got %d", selected.ID)
	}
	if stats := cache.Stats(); stats.Version != 7 || stats.ProviderCount != 2 || stats.EnabledProviderCount != 2 {
		t.Fatalf("unexpected cache stats: %+v", stats)
	}
}

func TestProviderModelMappingAndKeySelection(t *testing.T) {
	provider := Provider{
		APIKey:       "key-a\nkey-b",
		ModelMapping: StringMap{"public-model": "upstream-model"},
	}
	if got := provider.UpstreamModel("public-model"); got != "upstream-model" {
		t.Fatalf("expected upstream-model, got %q", got)
	}
	if got := provider.UpstreamModel("other"); got != "other" {
		t.Fatalf("expected passthrough model, got %q", got)
	}
	if key := provider.PickAPIKey(); key != "key-a" && key != "key-b" {
		t.Fatalf("unexpected key %q", key)
	}
}

func TestProviderMutationsBumpVersion(t *testing.T) {
	ctx := context.Background()
	db, err := Open("sqlite://" + filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	version, err := db.ProviderVersion(ctx)
	if err != nil {
		t.Fatalf("initial version: %v", err)
	}
	if version != 1 {
		t.Fatalf("expected initial version 1, got %d", version)
	}

	provider := &Provider{
		Name:     "mock",
		Provider: "openai",
		BaseURL:  "http://mock/v1",
		APIKey:   "key",
		Models:   "test-model",
		Groups:   "default",
		Status:   StatusEnabled,
	}
	if err := db.CreateProvider(ctx, provider); err != nil {
		t.Fatalf("create provider: %v", err)
	}
	version, err = db.ProviderVersion(ctx)
	if err != nil {
		t.Fatalf("version after create: %v", err)
	}
	if version != 2 {
		t.Fatalf("expected version 2 after create, got %d", version)
	}

	cache := NewCache()
	loaded, err := ReloadCache(ctx, db, cache)
	if err != nil {
		t.Fatalf("reload cache: %v", err)
	}
	if loaded != 2 || cache.Version() != 2 {
		t.Fatalf("expected loaded cache version 2, loaded=%d cache=%d", loaded, cache.Version())
	}

	provider.Weight = 10
	if err := db.SaveProvider(ctx, provider); err != nil {
		t.Fatalf("save provider: %v", err)
	}
	version, err = db.ProviderVersion(ctx)
	if err != nil {
		t.Fatalf("version after save: %v", err)
	}
	if version != 3 {
		t.Fatalf("expected version 3 after save, got %d", version)
	}
}
