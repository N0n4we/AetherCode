package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aethercode-router/internal/config"
	"aethercode-router/internal/store"
)

func TestRelayRouteMatrixDescriptorsHaveCapabilities(t *testing.T) {
	if len(relayRouteMatrix) == 0 {
		t.Fatal("relay route matrix is empty")
	}
	for _, descriptor := range relayRouteMatrix {
		if descriptor.Method == "" || descriptor.PathPattern == "" || descriptor.RouteFamily == "" || descriptor.ResponseFormat == "" || descriptor.Implementation == "" {
			t.Fatalf("route descriptor has missing required fields: %#v", descriptor)
		}
		if descriptor.Capability == "" {
			t.Fatalf("route descriptor missing capability: %#v", descriptor)
		}
	}
}

func TestRouteShellOpenAIAndGeminiModelDiscovery(t *testing.T) {
	server := newOpenAITestServer(t, []store.Provider{
		{
			ID:       1,
			Name:     "openai-provider",
			Provider: "openai",
			APIKey:   "openai-secret",
			Models:   "gpt-4o",
			Groups:   "internal-a",
			Status:   store.StatusEnabled,
		},
		{
			ID:                   2,
			Name:                 "gemini-provider",
			Provider:             "gemini",
			APIKey:               "gemini-secret",
			Models:               "gemini-pro",
			Groups:               "internal-b",
			Status:               store.StatusEnabled,
			EndpointCapabilities: store.StringList{store.EndpointCapabilityGeminiGenerate},
		},
		{
			ID:       3,
			Name:     "wildcard",
			Provider: "openai",
			Models:   "*",
			Status:   store.StatusEnabled,
		},
		{
			ID:       4,
			Name:     "disabled",
			Provider: "openai",
			Models:   "disabled-model",
			Status:   store.StatusDisabled,
		},
	}, config.Config{})

	modelsRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(modelsRec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if modelsRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", modelsRec.Code, modelsRec.Body.String())
	}
	assertNoShellLeak(t, modelsRec.Body.String())
	var openAIList openAIModelListResponse
	if err := json.Unmarshal(modelsRec.Body.Bytes(), &openAIList); err != nil {
		t.Fatalf("decode OpenAI model list: %v", err)
	}
	if openAIList.Object != "list" {
		t.Fatalf("unexpected list object %q", openAIList.Object)
	}
	if got := openAIModelIDs(openAIList.Data); strings.Join(got, ",") != "gemini-pro,gpt-4o" {
		t.Fatalf("unexpected OpenAI model ids: %#v", got)
	}

	lookupRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(lookupRec, httptest.NewRequest(http.MethodGet, "/v1/models/gpt-4o", nil))
	if lookupRec.Code != http.StatusOK {
		t.Fatalf("expected 200 lookup, got %d: %s", lookupRec.Code, lookupRec.Body.String())
	}
	var lookup openAIModelObject
	if err := json.Unmarshal(lookupRec.Body.Bytes(), &lookup); err != nil {
		t.Fatalf("decode OpenAI model lookup: %v", err)
	}
	if lookup.ID != "gpt-4o" || lookup.Object != "model" {
		t.Fatalf("unexpected lookup object: %#v", lookup)
	}

	missingRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(missingRec, httptest.NewRequest(http.MethodGet, "/v1/models/missing", nil))
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 missing model, got %d: %s", missingRec.Code, missingRec.Body.String())
	}
	var missing openAIError
	if err := json.Unmarshal(missingRec.Body.Bytes(), &missing); err != nil {
		t.Fatalf("decode missing-model error: %v", err)
	}
	if missing.Error.Code != "model_not_found" {
		t.Fatalf("expected model_not_found, got %q", missing.Error.Code)
	}

	geminiRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(geminiRec, httptest.NewRequest(http.MethodGet, "/v1beta/models", nil))
	if geminiRec.Code != http.StatusOK {
		t.Fatalf("expected 200 gemini list, got %d: %s", geminiRec.Code, geminiRec.Body.String())
	}
	assertNoShellLeak(t, geminiRec.Body.String())
	var geminiList geminiModelListResponse
	if err := json.Unmarshal(geminiRec.Body.Bytes(), &geminiList); err != nil {
		t.Fatalf("decode Gemini model list: %v", err)
	}
	if len(geminiList.Models) != 2 {
		t.Fatalf("expected two Gemini model entries, got %#v", geminiList.Models)
	}
	var foundGemini bool
	for _, model := range geminiList.Models {
		if model.Name == "models/gemini-pro" {
			foundGemini = true
			if strings.Join(model.SupportedGenerationMethods, ",") != "generateContent,streamGenerateContent" {
				t.Fatalf("unexpected gemini generation methods: %#v", model.SupportedGenerationMethods)
			}
		}
	}
	if !foundGemini {
		t.Fatalf("gemini-pro missing from Gemini model list: %#v", geminiList.Models)
	}

	openAICompatRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(openAICompatRec, httptest.NewRequest(http.MethodGet, "/v1beta/openai/models", nil))
	if openAICompatRec.Code != http.StatusOK {
		t.Fatalf("expected 200 v1beta openai list, got %d: %s", openAICompatRec.Code, openAICompatRec.Body.String())
	}
	var openAICompatList openAIModelListResponse
	if err := json.Unmarshal(openAICompatRec.Body.Bytes(), &openAICompatList); err != nil {
		t.Fatalf("decode v1beta openai model list: %v", err)
	}
	if got := openAIModelIDs(openAICompatList.Data); strings.Join(got, ",") != "gemini-pro,gpt-4o" {
		t.Fatalf("unexpected v1beta openai model ids: %#v", got)
	}
}

func TestRouteShellUnsupportedRoutesReturnStructured501AndDoNotDispatch(t *testing.T) {
	var upstreamCalled atomic.Bool
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	server := newOpenAITestServer(t, []store.Provider{{
		ID:                   1,
		Provider:             "openai",
		BaseURL:              upstream.URL + "/v1",
		APIKey:               "secret-key",
		Models:               "text-embedding-3-small",
		Groups:               "private-group",
		Status:               store.StatusEnabled,
		EndpointCapabilities: store.StringList{store.EndpointCapabilityOpenAIEmbeddings},
	}}, config.Config{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(`{"model":"text-embedding-3-small","group":"private-group"}`))
	server.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d: %s", rec.Code, rec.Body.String())
	}
	assertNoShellLeak(t, rec.Body.String())
	var envelope unsupportedEndpointError
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode unsupported error: %v", err)
	}
	if envelope.Error.Code != "unsupported_endpoint" {
		t.Fatalf("expected unsupported_endpoint, got %q", envelope.Error.Code)
	}
	if envelope.Error.RouteFamily != routeFamilyOpenAI || envelope.Error.Capability != store.EndpointCapabilityOpenAIEmbeddings {
		t.Fatalf("unexpected route metadata: %#v", envelope.Error)
	}
	if upstreamCalled.Load() {
		t.Fatal("unsupported route dispatched upstream")
	}

	geminiRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(geminiRec, httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-pro:generateContent", nil))
	if geminiRec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 gemini, got %d: %s", geminiRec.Code, geminiRec.Body.String())
	}
	var geminiEnvelope unsupportedEndpointError
	if err := json.Unmarshal(geminiRec.Body.Bytes(), &geminiEnvelope); err != nil {
		t.Fatalf("decode gemini unsupported error: %v", err)
	}
	if geminiEnvelope.Error.RouteFamily != routeFamilyGemini || geminiEnvelope.Error.Capability != store.EndpointCapabilityGeminiGenerate {
		t.Fatalf("unexpected gemini route metadata: %#v", geminiEnvelope.Error)
	}
}

func TestRouteShellUnknownPathAndMethodNotAllowed(t *testing.T) {
	server := newOpenAITestServer(t, nil, config.Config{})

	unknownRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(unknownRec, httptest.NewRequest(http.MethodGet, "/v1/not-a-real-route", nil))
	if unknownRec.Code != http.StatusNotFound {
		t.Fatalf("expected normal 404, got %d: %s", unknownRec.Code, unknownRec.Body.String())
	}
	if strings.Contains(unknownRec.Body.String(), "unsupported_endpoint") {
		t.Fatalf("unknown path should not return unsupported endpoint body: %s", unknownRec.Body.String())
	}

	methodRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(methodRec, httptest.NewRequest(http.MethodGet, "/v1/embeddings", nil))
	if methodRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d: %s", methodRec.Code, methodRec.Body.String())
	}
	if allow := methodRec.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("expected Allow POST, got %q", allow)
	}
	var envelope openAIError
	if err := json.Unmarshal(methodRec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode method error: %v", err)
	}
	if envelope.Error.Code != "method_not_allowed" {
		t.Fatalf("expected method_not_allowed, got %q", envelope.Error.Code)
	}
}

func openAIModelIDs(models []openAIModelObject) []string {
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}

func assertNoShellLeak(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{"group", "internal-a", "internal-b", "private-group", "secret"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(forbidden)) {
			t.Fatalf("route shell response leaked %q: %s", forbidden, body)
		}
	}
}
