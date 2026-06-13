package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"aethercode-router/internal/store"
	"aethercode-router/internal/upstream"
)

func (s *Server) openAIRoute(kind upstream.Kind) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method_not_allowed", "method not allowed")
			return
		}
		if !s.checkPublicAuth(w, r) {
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes))
		if err != nil {
			writeOpenAIError(w, http.StatusRequestEntityTooLarge, "invalid_request_error", "request_too_large", "request body is too large")
			return
		}
		defer r.Body.Close()

		var request map[string]json.RawMessage
		if err := json.Unmarshal(body, &request); err != nil {
			writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid_json", "request body must be valid JSON")
			return
		}

		model, err := modelFromRequest(request)
		if err != nil {
			writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "model_required", err.Error())
			return
		}
		delete(request, "group")

		excluded := map[uint]bool{}
		var lastErr error
		for attempt := 0; attempt <= s.cfg.MaxRetries; attempt++ {
			provider, err := s.cache.Select(model, excluded)
			if err != nil {
				writeOpenAIError(w, http.StatusServiceUnavailable, "invalid_request_error", "model_not_found", err.Error())
				return
			}
			excluded[provider.ID] = true
			w.Header().Set("X-Aether-Router-Instance", s.cfg.InstanceID)
			w.Header().Set("X-Aether-Provider-ID", strconv.FormatUint(uint64(provider.ID), 10))
			w.Header().Set("X-Aether-Provider-Name", provider.Name)
			w.Header().Set("X-Aether-Provider-Version", strconv.FormatInt(s.cache.Version(), 10))

			upstreamBody, err := encodeUpstreamRequest(request, provider, model)
			if err != nil {
				writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid_request", err.Error())
				return
			}

			resp, err := s.upstream.Do(r.Context(), provider, kind, model, upstreamBody)
			if err != nil {
				lastErr = err
				s.logger.Warn("upstream request failed", "provider_id", provider.ID, "error", err)
				continue
			}

			if shouldRetry(resp.StatusCode) && attempt < s.cfg.MaxRetries {
				lastErr = fmt.Errorf("upstream status %d", resp.StatusCode)
				_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
				_ = resp.Body.Close()
				continue
			}

			copyUpstreamResponse(w, resp)
			return
		}

		message := "upstream request failed"
		if lastErr != nil {
			message = lastErr.Error()
		}
		writeOpenAIError(w, http.StatusBadGateway, "api_error", "upstream_error", message)
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

func encodeUpstreamRequest(request map[string]json.RawMessage, provider *store.Provider, model string) ([]byte, error) {
	cloned := make(map[string]json.RawMessage, len(request))
	for key, value := range request {
		cloned[key] = value
	}
	modelBytes, err := json.Marshal(provider.UpstreamModel(model))
	if err != nil {
		return nil, err
	}
	cloned["model"] = modelBytes
	return json.Marshal(cloned)
}

func shouldRetry(status int) bool {
	return status == http.StatusTooManyRequests || status >= http.StatusInternalServerError
}

func copyUpstreamResponse(w http.ResponseWriter, resp *http.Response) {
	defer resp.Body.Close()
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	flusher, canFlush := w.(http.Flusher)
	if !canFlush {
		_, _ = io.Copy(w, resp.Body)
		return
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, _ = w.Write(buf[:n])
			flusher.Flush()
		}
		if err != nil {
			return
		}
	}
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
