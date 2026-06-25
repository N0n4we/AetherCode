package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
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

func TestCompletionsProxyUsesSelectedProvider(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/completions" {
			t.Fatalf("unexpected upstream path %s", r.URL.Path)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if body["model"] != "upstream-text-model" {
			t.Fatalf("expected mapped model, got %#v", body["model"])
		}
		if body["prompt"] != "hello" {
			t.Fatalf("expected prompt passthrough, got %#v", body["prompt"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl-text","object":"text_completion","choices":[]}`))
	}))
	defer upstream.Close()

	cache := store.NewCache()
	cache.Replace([]store.Provider{{
		ID:           1,
		Name:         "text-provider",
		Provider:     "openai",
		BaseURL:      upstream.URL + "/v1",
		APIKey:       "upstream-key",
		Models:       "public-text-model",
		ModelMapping: store.StringMap{"public-text-model": "upstream-text-model"},
		Status:       store.StatusEnabled,
	}}, 12)

	server := New(config.Config{
		InstanceID:     "test-router",
		RequestTimeout: 5 * time.Second,
		MaxRetries:     0,
		MaxBodyBytes:   1 << 20,
	}, nil, cache, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(`{
		"model":"public-text-model",
		"prompt":"hello",
		"temperature":0.1
	}`))
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Aether-Provider-ID"); got != "1" {
		t.Fatalf("unexpected provider id header %q", got)
	}
	if got := rec.Header().Get("X-Aether-Provider-Name"); got != "text-provider" {
		t.Fatalf("unexpected provider name header %q", got)
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

func TestChatCompletionsSkipsProviderWithoutEndpointCapability(t *testing.T) {
	var embeddingsOnlyCalled atomic.Bool
	embeddingsOnly := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		embeddingsOnlyCalled.Store(true)
		http.Error(w, "embeddings-only provider should not serve chat", http.StatusInternalServerError)
	}))
	defer embeddingsOnly.Close()

	chatProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl-chat","object":"chat.completion","choices":[]}`))
	}))
	defer chatProvider.Close()

	server := newOpenAITestServer(t, []store.Provider{
		{
			ID:                   1,
			Name:                 "embeddings-only",
			Provider:             "openai",
			BaseURL:              embeddingsOnly.URL + "/v1",
			Models:               "public-model",
			Status:               store.StatusEnabled,
			Priority:             10,
			EndpointCapabilities: store.StringList{store.EndpointCapabilityOpenAIEmbeddings},
		},
		{
			ID:                   2,
			Name:                 "chat",
			Provider:             "openai",
			BaseURL:              chatProvider.URL + "/v1",
			Models:               "public-model",
			Status:               store.StatusEnabled,
			Priority:             1,
			EndpointCapabilities: store.StringList{store.EndpointCapabilityOpenAIChatCompletions},
		},
	}, config.Config{MaxRetries: 0})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"public-model",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if embeddingsOnlyCalled.Load() {
		t.Fatal("provider without chat capability was selected")
	}
	if got := rec.Header().Get("X-Aether-Provider-ID"); got != "2" {
		t.Fatalf("expected chat provider metadata, got %q", got)
	}
}

func TestOpenAICompatibleStreamingResponseFlushesChunksIncrementally(t *testing.T) {
	firstChunkWritten := make(chan struct{})
	releaseSecondChunk := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: first\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		close(firstChunkWritten)
		<-releaseSecondChunk
		_, _ = w.Write([]byte("data: second\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	defer upstream.Close()

	server := newOpenAITestServer(t, []store.Provider{{
		ID:       1,
		Provider: "openai",
		BaseURL:  upstream.URL + "/v1",
		Models:   "public-model",
		Status:   store.StatusEnabled,
	}}, config.Config{MaxRetries: 0})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"public-model",
		"stream":true,
		"messages":[{"role":"user","content":"hello"}]
	}`))
	rec := newFlushRecorder()
	done := make(chan struct{})
	go func() {
		server.Routes().ServeHTTP(rec, req)
		close(done)
	}()

	select {
	case <-firstChunkWritten:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream did not write the first chunk")
	}
	select {
	case <-rec.flushed:
	case <-time.After(2 * time.Second):
		t.Fatal("router did not flush the first streamed chunk")
	}
	if body := rec.BodyString(); !strings.Contains(body, "data: first") {
		t.Fatalf("expected first chunk before upstream finished, got %q", body)
	}

	close(releaseSecondChunk)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("route did not finish after upstream stream completed")
	}
	if rec.Code() != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code(), rec.BodyString())
	}
	if body := rec.BodyString(); !strings.Contains(body, "data: second") {
		t.Fatalf("expected second chunk after completion, got %q", body)
	}
	if rec.FlushCount() == 0 {
		t.Fatal("expected at least one flush")
	}
}

func TestOpenAICompatibleRouteValidation(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		apiKey      string
		authHeader  string
		maxBodySize int64
		status      int
		code        string
	}{
		{
			name:   "invalid method",
			method: http.MethodGet,
			path:   "/v1/chat/completions",
			body:   `{"model":"public-model"}`,
			status: http.StatusMethodNotAllowed,
			code:   "method_not_allowed",
		},
		{
			name:   "invalid json",
			method: http.MethodPost,
			path:   "/v1/chat/completions",
			body:   `{"model":`,
			status: http.StatusBadRequest,
			code:   "invalid_json",
		},
		{
			name:   "missing model",
			method: http.MethodPost,
			path:   "/v1/chat/completions",
			body:   `{"messages":[]}`,
			status: http.StatusBadRequest,
			code:   "model_required",
		},
		{
			name:   "empty model",
			method: http.MethodPost,
			path:   "/v1/chat/completions",
			body:   `{"model":"   "}`,
			status: http.StatusBadRequest,
			code:   "model_required",
		},
		{
			name:        "oversized body",
			method:      http.MethodPost,
			path:        "/v1/completions",
			body:        `{"model":"public-model"}`,
			maxBodySize: 8,
			status:      http.StatusRequestEntityTooLarge,
			code:        "request_too_large",
		},
		{
			name:       "unauthorized public api key",
			method:     http.MethodPost,
			path:       "/v1/chat/completions",
			body:       `{"model":"public-model"}`,
			apiKey:     "router-secret",
			authHeader: "Bearer wrong",
			status:     http.StatusUnauthorized,
			code:       "unauthorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{
				InstanceID:     "test-router",
				RequestTimeout: 5 * time.Second,
				MaxRetries:     0,
				MaxBodyBytes:   1 << 20,
				APIKey:         tt.apiKey,
			}
			if tt.maxBodySize > 0 {
				cfg.MaxBodyBytes = tt.maxBodySize
			}
			cache := store.NewCache()
			server := New(cfg, nil, cache, slog.Default())
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			server.Routes().ServeHTTP(rec, req)

			if rec.Code != tt.status {
				t.Fatalf("expected status %d, got %d: %s", tt.status, rec.Code, rec.Body.String())
			}
			var envelope openAIError
			if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
				t.Fatalf("decode error envelope: %v", err)
			}
			if envelope.Error.Code != tt.code {
				t.Fatalf("expected error code %q, got %q", tt.code, envelope.Error.Code)
			}
		})
	}
}

func TestOpenAIAdaptorPreservesRequestHeadersAndFiltersHopByHopHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-Key"); got != "Token upstream-key" {
			t.Fatalf("unexpected custom auth header %q", got)
		}
		if got := r.Header.Get("X-Provider-Header"); got != "provider-value" {
			t.Fatalf("unexpected provider header %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if body["model"] != "deployment-a" {
			t.Fatalf("expected mapped model, got %#v", body["model"])
		}
		if body["temperature"] != float64(0.25) {
			t.Fatalf("expected temperature passthrough, got %#v", body["temperature"])
		}
		if _, ok := body["group"]; ok {
			t.Fatalf("group should not be forwarded upstream")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Upstream-Trace", "kept")
		_, _ = w.Write([]byte(`{"id":"cmpl-test","object":"chat.completion","choices":[]}`))
	}))
	defer upstream.Close()

	server := newOpenAITestServer(t, []store.Provider{{
		ID:           1,
		Name:         "custom-provider",
		Provider:     "openai",
		BaseURL:      upstream.URL + "/v1",
		APIKey:       "upstream-key",
		AuthHeader:   "X-API-Key",
		AuthPrefix:   "Token ",
		Models:       "gpt-4o",
		ModelMapping: store.StringMap{"gpt-4o": "deployment-a"},
		Headers:      store.StringMap{"X-Provider-Header": "provider-value"},
		Status:       store.StatusEnabled,
	}}, config.Config{MaxRetries: 0})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"gpt-4o",
		"group":"ignored",
		"temperature":0.25,
		"messages":[{"role":"user","content":"hello"}]
	}`))
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Upstream-Trace"); got != "kept" {
		t.Fatalf("expected upstream trace header, got %q", got)
	}
	if got := rec.Header().Get("Connection"); got != "" {
		t.Fatalf("hop-by-hop header should not be forwarded, got %q", got)
	}
	if got := rec.Header().Get("X-Aether-Provider-Name"); got != "custom-provider" {
		t.Fatalf("unexpected provider name header %q", got)
	}
}

func TestOpenAIRelayRetriesRetryableStatusWithOriginalBody(t *testing.T) {
	var firstCalled atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalled.Add(1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode first upstream body: %v", err)
		}
		if body["model"] != "first-upstream" {
			t.Errorf("expected first mapped model, got %#v", body["model"])
		}
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer first.Close()

	var secondCalled atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalled.Add(1)
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode second upstream body: %v", err)
		}
		if body["model"] != "second-upstream" {
			t.Errorf("expected second mapped model from original request, got %#v", body["model"])
		}
		if body["temperature"] != float64(0.7) {
			t.Errorf("expected replayed temperature, got %#v", body["temperature"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"retried","object":"chat.completion","choices":[]}`))
	}))
	defer second.Close()

	server := newOpenAITestServer(t, []store.Provider{
		{
			ID:           1,
			Name:         "first",
			Provider:     "openai",
			BaseURL:      first.URL + "/v1",
			Models:       "public-model",
			ModelMapping: store.StringMap{"public-model": "first-upstream"},
			Status:       store.StatusEnabled,
			Priority:     10,
		},
		{
			ID:           2,
			Name:         "second",
			Provider:     "openai",
			BaseURL:      second.URL + "/v1",
			Models:       "public-model",
			ModelMapping: store.StringMap{"public-model": "second-upstream"},
			Status:       store.StatusEnabled,
			Priority:     1,
		},
	}, config.Config{MaxRetries: 1})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"public-model",
		"temperature":0.7,
		"messages":[{"role":"user","content":"hello"}]
	}`))
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if firstCalled.Load() != 1 || secondCalled.Load() != 1 {
		t.Fatalf("expected one call to each provider, got first=%d second=%d", firstCalled.Load(), secondCalled.Load())
	}
	if got := rec.Header().Get("X-Aether-Provider-ID"); got != "2" {
		t.Fatalf("expected second provider metadata, got %q", got)
	}
}

func TestOpenAIRelayRetriesNetworkError(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	var secondCalled atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalled.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"retried","object":"chat.completion","choices":[]}`))
	}))
	defer second.Close()

	server := newOpenAITestServer(t, []store.Provider{
		{
			ID:       1,
			Name:     "dead",
			Provider: "openai",
			BaseURL:  deadURL + "/v1",
			Models:   "public-model",
			Status:   store.StatusEnabled,
			Priority: 10,
		},
		{
			ID:       2,
			Name:     "second",
			Provider: "openai",
			BaseURL:  second.URL + "/v1",
			Models:   "public-model",
			Status:   store.StatusEnabled,
			Priority: 1,
		},
	}, config.Config{MaxRetries: 1})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"public-model",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if secondCalled.Load() != 1 {
		t.Fatalf("expected retry to second provider, got %d calls", secondCalled.Load())
	}
}

func TestOpenAIRelayDoesNotRetryNonRetryable4xx(t *testing.T) {
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer first.Close()

	var secondCalled atomic.Bool
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalled.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer second.Close()

	server := newOpenAITestServer(t, []store.Provider{
		{
			ID:       1,
			Provider: "openai",
			BaseURL:  first.URL + "/v1",
			Models:   "public-model",
			Status:   store.StatusEnabled,
			Priority: 10,
		},
		{
			ID:       2,
			Provider: "openai",
			BaseURL:  second.URL + "/v1",
			Models:   "public-model",
			Status:   store.StatusEnabled,
			Priority: 1,
		},
	}, config.Config{MaxRetries: 1})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"public-model",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected upstream 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if secondCalled.Load() {
		t.Fatal("non-retryable 4xx retried another provider")
	}
}

func TestOpenAIRelayReturnsUpstreamErrorWhenProvidersExhausted(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusInternalServerError)
	}))
	defer upstream.Close()

	server := newOpenAITestServer(t, []store.Provider{{
		ID:       1,
		Provider: "openai",
		BaseURL:  upstream.URL + "/v1",
		Models:   "public-model",
		Status:   store.StatusEnabled,
	}}, config.Config{MaxRetries: 1})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"public-model",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
	var envelope openAIError
	if err := json.NewDecoder(rec.Body).Decode(&envelope); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if envelope.Error.Code != "upstream_error" {
		t.Fatalf("expected upstream_error, got %q", envelope.Error.Code)
	}
}

func TestOpenAIRelayDoesNotRetryCommittedStreamingFailure(t *testing.T) {
	var firstCalled atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalled.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Content-Length", "1024")
		_, _ = w.Write([]byte("data: partial\n\n"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}))
	defer first.Close()

	var secondCalled atomic.Bool
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalled.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer second.Close()

	server := newOpenAITestServer(t, []store.Provider{
		{
			ID:       1,
			Provider: "openai",
			BaseURL:  first.URL + "/v1",
			Models:   "public-model",
			Status:   store.StatusEnabled,
			Priority: 10,
		},
		{
			ID:       2,
			Provider: "openai",
			BaseURL:  second.URL + "/v1",
			Models:   "public-model",
			Status:   store.StatusEnabled,
			Priority: 1,
		},
	}, config.Config{MaxRetries: 1})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"public-model",
		"stream":true,
		"messages":[{"role":"user","content":"hello"}]
	}`))
	rec := httptest.NewRecorder()

	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected committed 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "data: partial") {
		t.Fatalf("expected partial streaming body, got %q", rec.Body.String())
	}
	if firstCalled.Load() != 1 {
		t.Fatalf("expected one first-provider call, got %d", firstCalled.Load())
	}
	if secondCalled.Load() {
		t.Fatal("committed streaming failure retried another provider")
	}
}

func TestRelayAccountAPIKeyAuthenticationStatesAndUsageBilling(t *testing.T) {
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Aether-Cache", "hit")
		_, _ = w.Write([]byte(`{"id":"cmpl-account","object":"chat.completion","choices":[]}`))
	}))
	defer upstream.Close()

	db, err := store.Open("sqlite://" + filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.CreateProvider(ctx, &store.Provider{
		Name:     "account-provider",
		Provider: "openai",
		BaseURL:  upstream.URL + "/v1",
		Models:   "public-model",
		Status:   store.StatusEnabled,
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if err := db.CreatePriceConfig(ctx, &store.PriceConfig{
		PublicModelID:   "public-model",
		UsageClass:      store.UsageClassRequest,
		CacheState:      "hit",
		UnitPriceMicros: 25,
	}); err != nil {
		t.Fatalf("create price: %v", err)
	}
	created, err := db.CreateAPIKey(ctx, "acct-relay", "relay key", "hash-secret")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	providerCache := store.NewCache()
	if _, err := store.ReloadCache(ctx, db, providerCache); err != nil {
		t.Fatalf("reload provider cache: %v", err)
	}
	keyCache := store.NewAPIKeyCache()
	if _, err := store.ReloadAPIKeyCache(ctx, db, keyCache); err != nil {
		t.Fatalf("reload api key cache: %v", err)
	}
	server := New(config.Config{
		InstanceID:         "test-router",
		RequestTimeout:     5 * time.Second,
		MaxRetries:         0,
		MaxBodyBytes:       1 << 20,
		AccountKeyAuth:     true,
		APIKeyHashSecret:   "hash-secret",
		ConfigSyncInterval: time.Second,
	}, db, providerCache, slog.Default())
	server.SetAPIKeyCache(keyCache)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"public-model",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	req.Header.Set("Authorization", "Bearer "+created.Secret)
	req.Header.Set("X-Request-ID", "relay-auth-success")
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected active key 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var events []store.UsageEvent
	if err := db.DB().Find(&events).Error; err != nil {
		t.Fatalf("query usage events: %v", err)
	}
	if len(events) != 1 || events[0].AccountID == 0 || events[0].APIKeyID != created.ID || events[0].CacheState != "hit" {
		t.Fatalf("unexpected usage events: %+v", events)
	}
	var charges []store.BillableCharge
	if err := db.DB().Find(&charges).Error; err != nil {
		t.Fatalf("query charges: %v", err)
	}
	if len(charges) != 1 || charges[0].AmountMicros != 25 {
		t.Fatalf("unexpected billable charges: %+v", charges)
	}

	missingReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"public-model"}`))
	missingRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected missing key 401, got %d: %s", missingRec.Code, missingRec.Body.String())
	}

	if _, err := db.DisableAPIKey(ctx, "acct-relay", created.ID); err != nil {
		t.Fatalf("disable key: %v", err)
	}
	if _, err := store.ReloadAPIKeyCache(ctx, db, keyCache); err != nil {
		t.Fatalf("reload disabled key cache: %v", err)
	}
	disabledReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"public-model"}`))
	disabledReq.Header.Set("Authorization", "Bearer "+created.Secret)
	disabledRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(disabledRec, disabledReq)
	if disabledRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected disabled key 401, got %d: %s", disabledRec.Code, disabledRec.Body.String())
	}

	revoked, err := db.CreateAPIKey(ctx, "acct-relay", "revoked key", "hash-secret")
	if err != nil {
		t.Fatalf("create revoked key: %v", err)
	}
	if _, err := db.RevokeAPIKey(ctx, "acct-relay", revoked.ID); err != nil {
		t.Fatalf("revoke key: %v", err)
	}
	if _, err := store.ReloadAPIKeyCache(ctx, db, keyCache); err != nil {
		t.Fatalf("reload revoked key cache: %v", err)
	}
	revokedReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"public-model"}`))
	revokedReq.Header.Set("Authorization", "Bearer "+revoked.Secret)
	revokedRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(revokedRec, revokedReq)
	if revokedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected revoked key 401, got %d: %s", revokedRec.Code, revokedRec.Body.String())
	}
}

func TestRelayUsageDoesNotDeduplicateDistinctRequestsByClientRequestID(t *testing.T) {
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl-account","object":"chat.completion","choices":[]}`))
	}))
	defer upstream.Close()

	db, err := store.Open("sqlite://" + filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.CreateProvider(ctx, &store.Provider{
		Name:     "account-provider",
		Provider: "openai",
		BaseURL:  upstream.URL + "/v1",
		Models:   "public-model",
		Status:   store.StatusEnabled,
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}
	if err := db.CreatePriceConfig(ctx, &store.PriceConfig{
		PublicModelID:   "public-model",
		UsageClass:      store.UsageClassRequest,
		CacheState:      store.CacheStateUnknown,
		UnitPriceMicros: 10,
	}); err != nil {
		t.Fatalf("create price: %v", err)
	}
	created, err := db.CreateAPIKey(ctx, "acct-relay", "relay key", "hash-secret")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	providerCache := store.NewCache()
	if _, err := store.ReloadCache(ctx, db, providerCache); err != nil {
		t.Fatalf("reload provider cache: %v", err)
	}
	keyCache := store.NewAPIKeyCache()
	if _, err := store.ReloadAPIKeyCache(ctx, db, keyCache); err != nil {
		t.Fatalf("reload api key cache: %v", err)
	}
	server := New(config.Config{
		InstanceID:       "test-router",
		RequestTimeout:   5 * time.Second,
		MaxRetries:       0,
		MaxBodyBytes:     1 << 20,
		AccountKeyAuth:   true,
		APIKeyHashSecret: "hash-secret",
	}, db, providerCache, slog.Default())
	server.SetAPIKeyCache(keyCache)

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
			"model":"public-model",
			"messages":[{"role":"user","content":"hello"}]
		}`))
		req.Header.Set("Authorization", "Bearer "+created.Secret)
		req.Header.Set("X-Request-ID", "client-reused-request-id")
		rec := httptest.NewRecorder()
		server.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d expected 200, got %d: %s", i+1, rec.Code, rec.Body.String())
		}
	}

	var events []store.UsageEvent
	if err := db.DB().Order("id asc").Find(&events).Error; err != nil {
		t.Fatalf("query usage events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected two usage events for two distinct requests with reused client request id, got %+v", events)
	}
	if events[0].EventID == events[1].EventID {
		t.Fatalf("distinct requests should not share usage event id %q", events[0].EventID)
	}
	for _, event := range events {
		if event.RequestID != "client-reused-request-id" {
			t.Fatalf("client request id should remain trace metadata, got %+v", event)
		}
	}

	var charges []store.BillableCharge
	if err := db.DB().Order("id asc").Find(&charges).Error; err != nil {
		t.Fatalf("query charges: %v", err)
	}
	if len(charges) != 2 {
		t.Fatalf("expected two billable charges, got %+v", charges)
	}
}

func TestProviderChannelSecretRefSuppliesUpstreamAPIKey(t *testing.T) {
	ctx := context.Background()
	var seenAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"cmpl-channel","object":"chat.completion","choices":[]}`))
	}))
	defer upstream.Close()

	db, err := store.Open("sqlite://" + filepath.Join(t.TempDir(), "channel.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.CreateProviderChannel(ctx, &store.ProviderChannel{
		Name:                    "channel",
		Provider:                "openai",
		PublicModelID:           "public-model",
		UpstreamBaseURL:         upstream.URL + "/v1",
		UpstreamAPIKeySecretRef: "env:TEST_UPSTREAM_API_KEY",
		Status:                  store.StatusEnabled,
	}); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	t.Setenv("TEST_UPSTREAM_API_KEY", "upstream-secret")

	providerCache := store.NewCache()
	if _, err := store.ReloadCache(ctx, db, providerCache); err != nil {
		t.Fatalf("reload provider cache: %v", err)
	}
	server := New(config.Config{
		InstanceID:     "test-router",
		RequestTimeout: 5 * time.Second,
		MaxRetries:     0,
		MaxBodyBytes:   1 << 20,
	}, db, providerCache, slog.Default())

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"public-model",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	rec := httptest.NewRecorder()
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if seenAuth != "Bearer upstream-secret" {
		t.Fatalf("expected provider channel secret ref to supply upstream auth, got %q", seenAuth)
	}
}

func newOpenAITestServer(t *testing.T, providers []store.Provider, override config.Config) *Server {
	t.Helper()
	cache := store.NewCache()
	cache.Replace(providers, 42)
	cfg := config.Config{
		InstanceID:     "test-router",
		RequestTimeout: 5 * time.Second,
		MaxRetries:     0,
		MaxBodyBytes:   1 << 20,
	}
	if override.MaxRetries != 0 {
		cfg.MaxRetries = override.MaxRetries
	}
	if override.MaxBodyBytes != 0 {
		cfg.MaxBodyBytes = override.MaxBodyBytes
	}
	if override.APIKey != "" {
		cfg.APIKey = override.APIKey
	}
	return New(cfg, nil, cache, slog.Default())
}

type flushRecorder struct {
	header     http.Header
	body       bytes.Buffer
	wrote      chan struct{}
	flushed    chan struct{}
	writeOnce  sync.Once
	flushOnce  sync.Once
	mu         sync.Mutex
	statusCode int
	flushes    int
}

func newFlushRecorder() *flushRecorder {
	return &flushRecorder{
		header:  make(http.Header),
		wrote:   make(chan struct{}),
		flushed: make(chan struct{}),
	}
}

func (r *flushRecorder) Header() http.Header {
	return r.header
}

func (r *flushRecorder) WriteHeader(statusCode int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.statusCode == 0 {
		r.statusCode = statusCode
	}
}

func (r *flushRecorder) Write(data []byte) (int, error) {
	r.mu.Lock()
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	n, err := r.body.Write(data)
	r.mu.Unlock()
	r.writeOnce.Do(func() {
		close(r.wrote)
	})
	return n, err
}

func (r *flushRecorder) Flush() {
	r.mu.Lock()
	r.flushes++
	r.mu.Unlock()
	r.flushOnce.Do(func() {
		close(r.flushed)
	})
}

func (r *flushRecorder) Code() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.statusCode == 0 {
		return http.StatusOK
	}
	return r.statusCode
}

func (r *flushRecorder) BodyString() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.String()
}

func (r *flushRecorder) FlushCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.flushes
}
