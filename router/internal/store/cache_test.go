package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestCacheSelectExactModelIDIgnoresGroupsAndAllowsMultipleProviders(t *testing.T) {
	cache := NewCache()
	cache.Replace([]Provider{
		{ID: 1, Provider: "openai", Models: "gpt-4o", Groups: "internal-a", Status: StatusEnabled, Priority: 10},
		{ID: 2, Provider: "openai", Models: "gpt-4o", Groups: "internal-b", Status: StatusEnabled, Priority: 5},
		{ID: 3, Provider: "openai", Models: "gpt-4o", Groups: "internal-c", Status: StatusEnabled, Priority: 1},
	}, 7)

	selected, err := cache.Select("gpt-4o", nil)
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	if selected.ID != 1 {
		t.Fatalf("expected provider 1, got %d", selected.ID)
	}

	selected, err = cache.Select("gpt-4o", map[uint]bool{1: true})
	if err != nil {
		t.Fatalf("select with exclude: %v", err)
	}
	if selected.ID != 2 {
		t.Fatalf("expected provider 2, got %d", selected.ID)
	}
	selected, err = cache.Select("gpt-4o", map[uint]bool{1: true, 2: true})
	if err != nil {
		t.Fatalf("select third provider with exclude: %v", err)
	}
	if selected.ID != 3 {
		t.Fatalf("expected provider 3, got %d", selected.ID)
	}
	if stats := cache.Stats(); stats.Version != 7 || stats.ProviderCount != 3 || stats.EnabledProviderCount != 3 || stats.GroupCount != 0 || stats.ModelCount != 1 {
		t.Fatalf("unexpected cache stats: %+v", stats)
	}
}

func TestCacheSelectWildcardFallbackAndNoProviderError(t *testing.T) {
	cache := NewCache()
	cache.Replace([]Provider{
		{ID: 9, Provider: "openai", Models: "*", Groups: "internal", Status: StatusEnabled},
	}, 1)

	selected, err := cache.Select("unlisted-model", nil)
	if err != nil {
		t.Fatalf("select wildcard: %v", err)
	}
	if selected.ID != 9 {
		t.Fatalf("expected wildcard provider 9, got %d", selected.ID)
	}

	empty := NewCache()
	empty.Replace(nil, 2)
	_, err = empty.Select("missing-model", nil)
	if err == nil {
		t.Fatalf("expected no-provider error")
	}
	if !strings.Contains(err.Error(), `no provider for model "missing-model"`) {
		t.Fatalf("unexpected no-provider error: %v", err)
	}
}

func TestCacheSelectStatusPriorityWeightAndRetryExclusion(t *testing.T) {
	cache := NewCache()
	cache.Replace([]Provider{
		{ID: 1, Provider: "openai", Models: "gpt-4o", Status: StatusDisabled, Priority: 100, Weight: 1000},
		{ID: 2, Provider: "openai", Models: "gpt-4o", Status: StatusEnabled, Priority: 10, Weight: 0},
		{ID: 3, Provider: "openai", Models: "gpt-4o", Status: StatusEnabled, Priority: 10, Weight: 100},
		{ID: 4, Provider: "openai", Models: "gpt-4o", Status: StatusEnabled, Priority: 1, Weight: 1000},
	}, 3)

	selected, err := cache.Select("gpt-4o", nil)
	if err != nil {
		t.Fatalf("select weighted: %v", err)
	}
	if selected.ID != 3 {
		t.Fatalf("expected weighted high-priority provider 3, got %d", selected.ID)
	}

	selected, err = cache.Select("gpt-4o", map[uint]bool{3: true})
	if err != nil {
		t.Fatalf("select after retry exclusion: %v", err)
	}
	if selected.ID != 2 {
		t.Fatalf("expected remaining high-priority provider 2, got %d", selected.ID)
	}

	selected, err = cache.Select("gpt-4o", map[uint]bool{2: true, 3: true})
	if err != nil {
		t.Fatalf("select lower priority after retry exclusion: %v", err)
	}
	if selected.ID != 4 {
		t.Fatalf("expected lower-priority provider 4, got %d", selected.ID)
	}

	_, err = cache.Select("gpt-4o", map[uint]bool{2: true, 3: true, 4: true})
	if err == nil {
		t.Fatalf("expected no-remaining-provider error")
	}
	if !strings.Contains(err.Error(), `no remaining provider for model "gpt-4o"`) {
		t.Fatalf("unexpected no-remaining-provider error: %v", err)
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
