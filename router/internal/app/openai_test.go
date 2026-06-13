package app

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aethercode-router/internal/config"
	"aethercode-router/internal/store"
)

func TestChatCompletionsProxyUsesSelectedProvider(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected upstream path %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer upstream-key" {
			t.Fatalf("unexpected auth header %q", auth)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if body["model"] != "upstream-model" {
			t.Fatalf("expected mapped model, got %#v", body["model"])
		}
		if _, ok := body["group"]; ok {
			t.Fatalf("group should not be forwarded upstream")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl-test","object":"chat.completion","choices":[]}`))
	}))
	defer upstream.Close()

	cache := store.NewCache()
	cache.Replace([]store.Provider{{
		ID:           1,
		Provider:     "openai",
		BaseURL:      upstream.URL + "/v1",
		APIKey:       "upstream-key",
		Models:       "public-model",
		Groups:       "default",
		ModelMapping: store.StringMap{"public-model": "upstream-model"},
		Status:       store.StatusEnabled,
	}})

	server := New(config.Config{
		RequestTimeout: 5 * time.Second,
		MaxRetries:     0,
		MaxBodyBytes:   1 << 20,
	}, nil, cache, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"public-model",
		"group":"default",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("unexpected content type %q", ct)
	}
}
