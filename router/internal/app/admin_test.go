package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"aethercode-router/internal/config"
	"aethercode-router/internal/store"
)

func TestAdminProviderCreateListAndStatusExposeCapabilitiesWithoutSecrets(t *testing.T) {
	server := newAdminTestServer(t)

	createBody := []byte(`{
		"name":"legacy",
		"provider":"openai",
		"base_url":"http://upstream/v1",
		"api_key":"secret-key",
		"models":"gpt-4o",
		"groups":"default"
	}`)
	createReq := httptest.NewRequest(http.MethodPost, "/internal/providers", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer admin-secret")
	createRec := httptest.NewRecorder()

	server.Routes().ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	assertNoAPIKey(t, createRec.Body.Bytes())

	var created store.PublicProvider
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created provider: %v", err)
	}
	if !hasCapability(created.EndpointCapabilities, store.EndpointCapabilityOpenAIChatCompletions) ||
		!hasCapability(created.EndpointCapabilities, store.EndpointCapabilityOpenAICompletions) {
		t.Fatalf("legacy provider did not expose default capabilities: %#v", created.EndpointCapabilities)
	}

	explicitBody := []byte(`{
		"name":"embeddings",
		"provider":"openai",
		"base_url":"http://upstream/v1",
		"api_key":"secret-key-2",
		"models":"text-embedding-3-small",
		"endpoint_capabilities":["OpenAI.Embeddings"],
		"channel_type":"openai",
		"relay_format":"openai-compatible"
	}`)
	explicitReq := httptest.NewRequest(http.MethodPost, "/internal/providers", bytes.NewReader(explicitBody))
	explicitReq.Header.Set("Authorization", "Bearer admin-secret")
	explicitRec := httptest.NewRecorder()

	server.Routes().ServeHTTP(explicitRec, explicitReq)

	if explicitRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for explicit provider, got %d: %s", explicitRec.Code, explicitRec.Body.String())
	}
	assertNoAPIKey(t, explicitRec.Body.Bytes())
	var explicit store.PublicProvider
	if err := json.Unmarshal(explicitRec.Body.Bytes(), &explicit); err != nil {
		t.Fatalf("decode explicit provider: %v", err)
	}
	if len(explicit.EndpointCapabilities) != 1 || explicit.EndpointCapabilities[0] != store.EndpointCapabilityOpenAIEmbeddings {
		t.Fatalf("unexpected explicit capabilities: %#v", explicit.EndpointCapabilities)
	}
	if explicit.ChannelType != "openai" || explicit.RelayFormat != "openai-compatible" {
		t.Fatalf("unexpected channel metadata: channel=%q relay=%q", explicit.ChannelType, explicit.RelayFormat)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/internal/providers", nil)
	listReq.Header.Set("Authorization", "Bearer admin-secret")
	listRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 list, got %d: %s", listRec.Code, listRec.Body.String())
	}
	assertNoAPIKey(t, listRec.Body.Bytes())
	var listed []store.PublicProvider
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode provider list: %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("expected two providers, got %d", len(listed))
	}
	for _, provider := range listed {
		if len(provider.EndpointCapabilities) == 0 {
			t.Fatalf("listed provider missing capabilities: %#v", provider)
		}
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/internal/status", nil)
	statusReq.Header.Set("Authorization", "Bearer admin-secret")
	statusRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(statusRec, statusReq)

	if statusRec.Code != http.StatusOK {
		t.Fatalf("expected 200 status, got %d: %s", statusRec.Code, statusRec.Body.String())
	}
	var status struct {
		Cache struct {
			CapabilityCounts map[string]int `json:"capability_counts"`
		} `json:"cache"`
	}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.Cache.CapabilityCounts[store.EndpointCapabilityOpenAIChatCompletions] != 1 ||
		status.Cache.CapabilityCounts[store.EndpointCapabilityOpenAICompletions] != 1 ||
		status.Cache.CapabilityCounts[store.EndpointCapabilityOpenAIEmbeddings] != 1 {
		t.Fatalf("unexpected capability counts: %+v", status.Cache.CapabilityCounts)
	}
}

func TestAdminProviderChannelsValidateSingleModelAndProjectToCache(t *testing.T) {
	server := newAdminTestServer(t)

	invalidReq := httptest.NewRequest(http.MethodPost, "/internal/provider-channels", bytes.NewReader([]byte(`{
		"name":"invalid",
		"provider":"openai",
		"model_id":"gpt-4o,gpt-4o-mini"
	}`)))
	invalidReq.Header.Set("Authorization", "Bearer admin-secret")
	invalidRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(invalidRec, invalidReq)
	if invalidRec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid multi-model channel to fail, got %d: %s", invalidRec.Code, invalidRec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/internal/provider-channels", bytes.NewReader([]byte(`{
		"name":"channel-a",
		"provider":"openai",
		"upstream_base_url":"http://upstream/v1",
		"upstream_model":"deployment-a",
		"upstream_api_key_secret_ref":"gcp-secret-manager:projects/test/secrets/openai",
		"model_id":"gpt-4o",
		"endpoint_capabilities":["openai.chat_completions"],
		"priority":20,
		"weight":100
	}`)))
	createReq.Header.Set("Authorization", "Bearer admin-secret")
	createRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected channel create 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	if bytes.Contains(createRec.Body.Bytes(), []byte("sk-")) {
		t.Fatalf("channel response exposed an upstream secret value: %s", createRec.Body.String())
	}

	selected, err := server.cache.SelectForCapability("gpt-4o", store.EndpointCapabilityOpenAIChatCompletions, nil)
	if err != nil {
		t.Fatalf("select projected provider channel: %v", err)
	}
	if selected.PlatformChannelID == 0 {
		t.Fatalf("selected provider did not preserve platform channel attribution: %+v", selected)
	}
	if got := selected.UpstreamModel("gpt-4o"); got != "deployment-a" {
		t.Fatalf("expected upstream model mapping, got %q", got)
	}
}

func newAdminTestServer(t *testing.T) *Server {
	t.Helper()
	db, err := store.Open("sqlite://" + filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	cache := store.NewCache()
	if _, err := store.ReloadCache(context.Background(), db, cache); err != nil {
		t.Fatalf("initial reload: %v", err)
	}
	return New(config.Config{
		InstanceID:     "test-router",
		RequestTimeout: 5 * time.Second,
		MaxRetries:     0,
		MaxBodyBytes:   1 << 20,
		AdminKey:       "admin-secret",
	}, db, cache, slog.Default())
}

func assertNoAPIKey(t *testing.T, body []byte) {
	t.Helper()
	var raw any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode json for secret check: %v", err)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("re-encode json for secret check: %v", err)
	}
	if bytes.Contains(data, []byte("api_key")) || bytes.Contains(data, []byte("secret-key")) {
		t.Fatalf("admin-safe response exposed provider secret: %s", string(data))
	}
}

func hasCapability(capabilities store.StringList, capability string) bool {
	for _, configured := range capabilities {
		if configured == capability {
			return true
		}
	}
	return false
}
