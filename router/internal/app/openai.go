package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"aethercode-router/internal/store"
	"aethercode-router/internal/upstream"
)

type openAICompatibleRelayFormat string

const openAICompatibleFormat openAICompatibleRelayFormat = "openai-compatible"

type openAICompatibleRequestEnvelope struct {
	format openAICompatibleRelayFormat
	kind   upstream.Kind
	body   replayableOpenAIRequestBody
}

type replayableOpenAIRequestBody struct {
	fields map[string]json.RawMessage
	model  string
}

type selectedProviderMetadata struct {
	provider     *store.Provider
	version      int64
	requestModel string
}

type relayAttemptState struct {
	attempted    map[uint]bool
	lastErr      error
	lastSelected *selectedProviderMetadata
}

type openAICompatibleAdaptorResult struct {
	response *http.Response
	err      error
}

type responseCommitTracker struct {
	statusCommitted bool
	bodyCommitted   bool
	statusCode      int
}

type trackedResponseBodyWriter struct {
	w       http.ResponseWriter
	tracker *responseCommitTracker
}

type openAICompatibleValidationError struct {
	status  int
	typ     string
	code    string
	message string
}

type openAICompatibleAdaptor struct{}

func (s *Server) openAIRoute(kind upstream.Kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method_not_allowed", "method not allowed")
			return
		}
		startedAt := time.Now()
		if !s.checkPublicAuth(w, r) {
			return
		}

		envelope, validationErr := s.openAICompatibleEnvelope(w, r, kind)
		if validationErr != nil {
			s.recordRelayUsage(r, relayUsageRecord{
				StartedAt:          startedAt,
				CompletedAt:        time.Now(),
				EndpointCapability: openAIEndpointCapability(kind),
				Outcome:            store.UsageOutcomeFailed,
				StatusCode:         validationErr.status,
				ErrorCode:          validationErr.code,
			})
			writeOpenAIError(w, validationErr.status, validationErr.typ, validationErr.code, validationErr.message)
			return
		}

		s.relayOpenAICompatible(w, r, envelope, startedAt)
	}
}

func (s *Server) openAICompatibleEnvelope(w http.ResponseWriter, r *http.Request, kind upstream.Kind) (*openAICompatibleRequestEnvelope, *openAICompatibleValidationError) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes))
	if err != nil {
		return nil, &openAICompatibleValidationError{
			status:  http.StatusRequestEntityTooLarge,
			typ:     "invalid_request_error",
			code:    "request_too_large",
			message: "request body is too large",
		}
	}
	defer r.Body.Close()

	var request map[string]json.RawMessage
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, &openAICompatibleValidationError{
			status:  http.StatusBadRequest,
			typ:     "invalid_request_error",
			code:    "invalid_json",
			message: "request body must be valid JSON",
		}
	}

	model, err := modelFromRequest(request)
	if err != nil {
		return nil, &openAICompatibleValidationError{
			status:  http.StatusBadRequest,
			typ:     "invalid_request_error",
			code:    "model_required",
			message: err.Error(),
		}
	}

	return &openAICompatibleRequestEnvelope{
		format: openAICompatibleFormat,
		kind:   kind,
		body:   newReplayableOpenAIRequestBody(request, model),
	}, nil
}

func (s *Server) relayOpenAICompatible(w http.ResponseWriter, r *http.Request, envelope *openAICompatibleRequestEnvelope, startedAt time.Time) {
	if envelope.format != openAICompatibleFormat {
		s.recordRelayUsage(r, relayUsageRecord{
			StartedAt:          startedAt,
			CompletedAt:        time.Now(),
			ModelID:            envelope.body.model,
			EndpointCapability: openAIEndpointCapability(envelope.kind),
			Outcome:            store.UsageOutcomeFailed,
			StatusCode:         http.StatusInternalServerError,
			ErrorCode:          "unsupported_relay_format",
		})
		writeOpenAIError(w, http.StatusInternalServerError, "api_error", "unsupported_relay_format", "unsupported relay format")
		return
	}

	state := newRelayAttemptState()
	adaptor := openAICompatibleAdaptor{}

	for attempt := 0; attempt <= s.cfg.MaxRetries; attempt++ {
		selected, err := s.selectOpenAICompatibleProvider(envelope, state)
		if err != nil {
			if state.hasAttempted() && state.lastErr != nil {
				s.recordRelayUsage(r, relayUsageRecord{
					StartedAt:          startedAt,
					CompletedAt:        time.Now(),
					ModelID:            envelope.body.model,
					Selected:           state.lastSelected,
					EndpointCapability: openAIEndpointCapability(envelope.kind),
					Outcome:            store.UsageOutcomeFailed,
					StatusCode:         http.StatusBadGateway,
					ErrorCode:          "upstream_error",
				})
				writeOpenAIUpstreamError(w, state.lastErr)
				return
			}
			s.recordRelayUsage(r, relayUsageRecord{
				StartedAt:          startedAt,
				CompletedAt:        time.Now(),
				ModelID:            envelope.body.model,
				EndpointCapability: openAIEndpointCapability(envelope.kind),
				Outcome:            store.UsageOutcomeFailed,
				StatusCode:         http.StatusServiceUnavailable,
				ErrorCode:          "model_not_found",
			})
			writeOpenAIError(w, http.StatusServiceUnavailable, "invalid_request_error", "model_not_found", err.Error())
			return
		}
		state.lastSelected = selected

		result := adaptor.send(r.Context(), s.upstream, envelope, selected)
		if result.err != nil {
			state.lastErr = result.err
			s.logger.Warn("upstream request failed", "provider_id", selected.provider.ID, "error", result.err)
			if attempt < s.cfg.MaxRetries {
				continue
			}
			break
		}

		if shouldRetry(result.response.StatusCode) {
			state.lastErr = fmt.Errorf("upstream status %d", result.response.StatusCode)
			drainAndClose(result.response)
			if attempt < s.cfg.MaxRetries {
				continue
			}
			break
		}

		setOpenAIProviderHeaders(w, selected, s.cfg.InstanceID)
		tracker := &responseCommitTracker{}
		if err := copyUpstreamResponse(w, result.response, tracker); err != nil {
			s.logger.Warn("copy upstream response failed", "provider_id", selected.provider.ID, "committed", tracker.Committed(), "error", err)
		}
		statusCode := tracker.statusCode
		if statusCode == 0 {
			statusCode = result.response.StatusCode
		}
		outcome := store.UsageOutcomeSuccess
		if statusCode >= 400 {
			outcome = store.UsageOutcomeFailed
		}
		s.recordRelayUsage(r, relayUsageRecord{
			StartedAt:          startedAt,
			CompletedAt:        time.Now(),
			ModelID:            envelope.body.model,
			Selected:           selected,
			EndpointCapability: openAIEndpointCapability(envelope.kind),
			Outcome:            outcome,
			StatusCode:         statusCode,
			UpstreamStatus:     result.response.StatusCode,
			CacheState:         cacheStateFromResponse(result.response),
		})
		return
	}

	s.recordRelayUsage(r, relayUsageRecord{
		StartedAt:          startedAt,
		CompletedAt:        time.Now(),
		ModelID:            envelope.body.model,
		Selected:           state.lastSelected,
		EndpointCapability: openAIEndpointCapability(envelope.kind),
		Outcome:            store.UsageOutcomeFailed,
		StatusCode:         http.StatusBadGateway,
		ErrorCode:          "upstream_error",
	})
	writeOpenAIUpstreamError(w, state.lastErr)
}

func newRelayAttemptState() *relayAttemptState {
	return &relayAttemptState{attempted: map[uint]bool{}}
}

func (s *Server) selectOpenAICompatibleProvider(envelope *openAICompatibleRequestEnvelope, state *relayAttemptState) (*selectedProviderMetadata, error) {
	provider, err := s.cache.SelectForCapability(envelope.body.model, openAIEndpointCapability(envelope.kind), state.attempted)
	if err != nil {
		return nil, err
	}
	state.attempted[provider.ID] = true
	return &selectedProviderMetadata{
		provider:     provider,
		version:      s.cache.Version(),
		requestModel: envelope.body.model,
	}, nil
}

func openAIEndpointCapability(kind upstream.Kind) string {
	switch kind {
	case upstream.ChatCompletions:
		return store.EndpointCapabilityOpenAIChatCompletions
	case upstream.Completions:
		return store.EndpointCapabilityOpenAICompletions
	default:
		return ""
	}
}

func (s *relayAttemptState) hasAttempted() bool {
	return len(s.attempted) > 0
}

func (openAICompatibleAdaptor) send(ctx context.Context, client *upstream.Client, envelope *openAICompatibleRequestEnvelope, selected *selectedProviderMetadata) openAICompatibleAdaptorResult {
	upstreamBody, err := envelope.body.encodeFor(selected.provider, selected.requestModel)
	if err != nil {
		return openAICompatibleAdaptorResult{err: err}
	}
	resp, err := client.Do(ctx, selected.provider, envelope.kind, selected.requestModel, upstreamBody)
	return openAICompatibleAdaptorResult{
		response: resp,
		err:      err,
	}
}

func newReplayableOpenAIRequestBody(request map[string]json.RawMessage, model string) replayableOpenAIRequestBody {
	fields := make(map[string]json.RawMessage, len(request))
	for key, value := range request {
		if key == "group" {
			continue
		}
		fields[key] = value
	}
	return replayableOpenAIRequestBody{
		fields: fields,
		model:  model,
	}
}

func modelFromRequest(request map[string]json.RawMessage) (string, error) {
	raw, ok := request["model"]
	if !ok {
		return "", fmt.Errorf("field model is required")
	}
	var model string
	if err := json.Unmarshal(raw, &model); err != nil || strings.TrimSpace(model) == "" {
		return "", fmt.Errorf("field model must be a non-empty string")
	}
	return strings.TrimSpace(model), nil
}

func (b replayableOpenAIRequestBody) encodeFor(provider *store.Provider, model string) ([]byte, error) {
	cloned := make(map[string]json.RawMessage, len(b.fields))
	for key, value := range b.fields {
		cloned[key] = value
	}
	modelBytes, err := json.Marshal(provider.UpstreamModel(model))
	if err != nil {
		return nil, err
	}
	cloned["model"] = modelBytes
	return json.Marshal(cloned)
}

func setOpenAIProviderHeaders(w http.ResponseWriter, selected *selectedProviderMetadata, instanceID string) {
	w.Header().Set("X-Aether-Router-Instance", instanceID)
	w.Header().Set("X-Aether-Provider-ID", strconv.FormatUint(uint64(selected.provider.ID), 10))
	w.Header().Set("X-Aether-Provider-Name", selected.provider.Name)
	w.Header().Set("X-Aether-Provider-Version", strconv.FormatInt(selected.version, 10))
}

func writeOpenAIUpstreamError(w http.ResponseWriter, err error) {
	message := "upstream request failed"
	if err != nil {
		message = err.Error()
	}
	writeOpenAIError(w, http.StatusBadGateway, "api_error", "upstream_error", message)
}

func drainAndClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()
}

func shouldRetry(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func copyUpstreamResponse(w http.ResponseWriter, resp *http.Response, tracker *responseCommitTracker) error {
	defer resp.Body.Close()
	copyHeaders(w.Header(), resp.Header)
	tracker.WriteHeader(w, resp.StatusCode)

	flusher, canFlush := w.(http.Flusher)
	if !canFlush {
		_, err := io.Copy(trackedResponseBodyWriter{w: w, tracker: tracker}, resp.Body)
		return err
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := tracker.Write(w, buf[:n]); writeErr != nil {
				return writeErr
			}
			flusher.Flush()
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (t *responseCommitTracker) WriteHeader(w http.ResponseWriter, statusCode int) {
	if !t.statusCommitted {
		t.statusCommitted = true
		t.statusCode = statusCode
	}
	w.WriteHeader(statusCode)
}

func (t *responseCommitTracker) Write(w http.ResponseWriter, data []byte) (int, error) {
	if !t.statusCommitted {
		t.statusCommitted = true
		t.statusCode = http.StatusOK
	}
	n, err := w.Write(data)
	if n > 0 {
		t.bodyCommitted = true
	}
	return n, err
}

func (t *responseCommitTracker) Committed() bool {
	return t.statusCommitted || t.bodyCommitted
}

func (w trackedResponseBodyWriter) Write(data []byte) (int, error) {
	return w.tracker.Write(w.w, data)
}

func copyHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		if isHopByHopHeader(key) {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func isHopByHopHeader(key string) bool {
	switch strings.ToLower(key) {
	case "connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer", "transfer-encoding", "upgrade":
		return true
	default:
		return false
	}
}
