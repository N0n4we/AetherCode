package app

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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
	}}, 11)

	server := New(config.Config{
		InstanceID:     "test-router",
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
	if got := rec.Header().Get("X-Aether-Router-Instance"); got != "test-router" {
		t.Fatalf("unexpected router instance header %q", got)
	}
	if got := rec.Header().Get("X-Aether-Provider-Version"); got != "11" {
		t.Fatalf("unexpected provider version header %q", got)
	}
}

func TestChatCompletionsIgnoresGroupHintsForSelectionAndDispatch(t *testing.T) {
	preferred := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if group := r.Header.Get("X-Aether-Group"); group != "" {
			t.Fatalf("X-Aether-Group should not be forwarded upstream, got %q", group)
		}
		if group := r.Header.Get("X-Router-Group"); group != "" {
			t.Fatalf("X-Router-Group should not be forwarded upstream, got %q", group)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if _, ok := body["group"]; ok {
			t.Fatalf("group should not be forwarded upstream")
		}
		if body["model"] != "public-model" {
			t.Fatalf("expected public-model, got %#v", body["model"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl-preferred","object":"chat.completion","choices":[]}`))
	}))
	defer preferred.Close()

	var decoyCalled atomic.Bool
	decoy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		decoyCalled.Store(true)
		http.Error(w, "decoy provider should not be selected", http.StatusInternalServerError)
	}))
	defer decoy.Close()

	cache := store.NewCache()
	cache.Replace([]store.Provider{
		{
			ID:       1,
			Name:     "preferred",
			Provider: "openai",
			BaseURL:  preferred.URL + "/v1",
			Models:   "public-model",
			Groups:   "default",
			Status:   store.StatusEnabled,
			Priority: 10,
		},
		{
			ID:       2,
			Name:     "decoy",
			Provider: "openai",
			BaseURL:  decoy.URL + "/v1",
			Models:   "public-model",
			Groups:   "hinted",
			Status:   store.StatusEnabled,
			Priority: 1,
		},
	}, 12)

	server := New(config.Config{
		InstanceID:     "test-router",
		RequestTimeout: 5 * time.Second,
		MaxRetries:     0,
		MaxBodyBytes:   1 << 20,
	}, nil, cache, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"public-model",
		"group":"hinted",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	req.Header.Set("X-Aether-Group", "hinted")
	req.Header.Set("X-Router-Group", "hinted")
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Aether-Provider-ID"); got != "1" {
		t.Fatalf("expected preferred provider 1, got %q", got)
	}
	if decoyCalled.Load() {
		t.Fatalf("decoy provider was selected from group hints")
	}
}
