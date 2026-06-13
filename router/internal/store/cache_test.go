package store

import "testing"

func TestCacheSelectPriorityAndExclude(t *testing.T) {
	cache := NewCache()
	cache.Replace([]Provider{
		{ID: 1, Provider: "openai", Models: "gpt-4", Groups: "default", Status: StatusEnabled, Priority: 10},
		{ID: 2, Provider: "openai", Models: "gpt-4", Groups: "default", Status: StatusEnabled, Priority: 5},
	})

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
