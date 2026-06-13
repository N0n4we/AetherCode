package upstream

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"aethercode-router/internal/store"
	"github.com/teilomillet/gollm/providers"
)

type Kind string

const (
	ChatCompletions Kind = "chat_completions"
	Completions     Kind = "completions"
)

type Client struct {
	httpClient *http.Client
}

func NewClient(timeout time.Duration) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Do(ctx context.Context, provider *store.Provider, kind Kind, requestModel string, body []byte) (*http.Response, error) {
	gollmProvider, err := buildProvider(provider, kind, requestModel)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gollmProvider.Endpoint(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, value := range gollmProvider.Headers() {
		if strings.TrimSpace(key) != "" {
			req.Header.Set(key, value)
		}
	}
	return c.httpClient.Do(req)
}

var registryMu sync.Mutex

func buildProvider(provider *store.Provider, kind Kind, requestModel string) (providers.Provider, error) {
	upstreamModel := provider.UpstreamModel(requestModel)
	apiKey := provider.PickAPIKey()
	extraHeaders := map[string]string{}
	for key, value := range provider.Headers {
		extraHeaders[key] = value
	}

	if provider.BaseURL == "" && provider.AuthHeader == "" && provider.AuthPrefix == "" {
		baseProvider, err := defaultProvider(provider.Provider, apiKey, upstreamModel, extraHeaders)
		if err == nil && kind == ChatCompletions {
			return baseProvider, nil
		}
		if err == nil {
			return genericProvider(provider, kind, upstreamModel, apiKey, extraHeaders, baseProvider.Endpoint())
		}
	}

	baseURL := provider.BaseURL
	if baseURL == "" {
		baseProvider, err := defaultProvider(provider.Provider, apiKey, upstreamModel, extraHeaders)
		if err != nil {
			return nil, err
		}
		baseURL = baseProvider.Endpoint()
	}
	return genericProvider(provider, kind, upstreamModel, apiKey, extraHeaders, baseURL)
}

func defaultProvider(name string, apiKey string, model string, extraHeaders map[string]string) (providers.Provider, error) {
	registryMu.Lock()
	defer registryMu.Unlock()
	return providers.GetDefaultRegistry().Get(name, apiKey, model, extraHeaders)
}

func genericProvider(provider *store.Provider, kind Kind, model string, apiKey string, extraHeaders map[string]string, baseURL string) (providers.Provider, error) {
	name := fmt.Sprintf("aether-router-%d-%s", provider.ID, kind)
	config := providers.ProviderConfig{
		Name:              name,
		Type:              providers.TypeOpenAI,
		Endpoint:          normalizeEndpoint(baseURL, kind),
		AuthHeader:        provider.AuthHeaderOrDefault(),
		AuthPrefix:        provider.AuthPrefixOrDefault(),
		RequiredHeaders:   map[string]string{"Content-Type": "application/json"},
		SupportsSchema:    true,
		SupportsStreaming: true,
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	providers.RegisterGenericProvider(name, config)
	return providers.GetDefaultRegistry().Get(name, apiKey, model, extraHeaders)
}

func normalizeEndpoint(baseURL string, kind Kind) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if kind == ChatCompletions {
		if strings.HasSuffix(baseURL, "/chat/completions") {
			return baseURL
		}
		if strings.HasSuffix(baseURL, "/completions") {
			return strings.TrimSuffix(baseURL, "/completions") + "/chat/completions"
		}
		return baseURL + "/chat/completions"
	}

	if strings.HasSuffix(baseURL, "/chat/completions") {
		return strings.TrimSuffix(baseURL, "/chat/completions") + "/completions"
	}
	if strings.HasSuffix(baseURL, "/completions") {
		return baseURL
	}
	return baseURL + "/completions"
}
