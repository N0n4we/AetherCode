package app

import (
	"net/http"
	"net/url"
	"sort"
	"strings"

	"aethercode-router/internal/store"
	"aethercode-router/internal/upstream"
)

const (
	routeFamilyOpenAI   = "openai"
	routeFamilyClaude   = "claude"
	routeFamilyGemini   = "gemini"
	routeFamilyRealtime = "realtime"
	routeFamilyTask     = "task"

	responseFormatOpenAI = "openai"
	responseFormatGemini = "gemini"
	responseFormatJSON   = "json"

	routeStatusImplemented = "implemented"
	routeStatusMetadata    = "metadata"
	routeStatusUnsupported = "unsupported"

	endpointCapabilityModelMetadata = "metadata.models"
)

type relayRouteDescriptor struct {
	Method           string
	PathPattern      string
	RouteFamily      string
	Capability       string
	ResponseFormat   string
	Implementation   string
	ImplementationID string
}

var relayRouteMatrix = []relayRouteDescriptor{
	{
		Method:           http.MethodPost,
		PathPattern:      "/v1/chat/completions",
		RouteFamily:      routeFamilyOpenAI,
		Capability:       store.EndpointCapabilityOpenAIChatCompletions,
		ResponseFormat:   responseFormatOpenAI,
		Implementation:   routeStatusImplemented,
		ImplementationID: string(upstream.ChatCompletions),
	},
	{
		Method:           http.MethodPost,
		PathPattern:      "/v1/completions",
		RouteFamily:      routeFamilyOpenAI,
		Capability:       store.EndpointCapabilityOpenAICompletions,
		ResponseFormat:   responseFormatOpenAI,
		Implementation:   routeStatusImplemented,
		ImplementationID: string(upstream.Completions),
	},
	{
		Method:         http.MethodGet,
		PathPattern:    "/v1/models",
		RouteFamily:    routeFamilyOpenAI,
		Capability:     endpointCapabilityModelMetadata,
		ResponseFormat: responseFormatOpenAI,
		Implementation: routeStatusMetadata,
	},
	{
		Method:         http.MethodGet,
		PathPattern:    "/v1/models/{model}",
		RouteFamily:    routeFamilyOpenAI,
		Capability:     endpointCapabilityModelMetadata,
		ResponseFormat: responseFormatOpenAI,
		Implementation: routeStatusMetadata,
	},
	{
		Method:         http.MethodGet,
		PathPattern:    "/v1beta/models",
		RouteFamily:    routeFamilyGemini,
		Capability:     endpointCapabilityModelMetadata,
		ResponseFormat: responseFormatGemini,
		Implementation: routeStatusMetadata,
	},
	{
		Method:         http.MethodGet,
		PathPattern:    "/v1beta/openai/models",
		RouteFamily:    routeFamilyOpenAI,
		Capability:     endpointCapabilityModelMetadata,
		ResponseFormat: responseFormatOpenAI,
		Implementation: routeStatusMetadata,
	},
	unsupportedRoute(http.MethodPost, "/v1/responses", routeFamilyOpenAI, store.EndpointCapabilityOpenAIResponses, responseFormatOpenAI),
	unsupportedRoute(http.MethodPost, "/v1/responses/compact", routeFamilyOpenAI, store.EndpointCapabilityOpenAIResponses, responseFormatOpenAI),
	unsupportedRoute(http.MethodPost, "/v1/embeddings", routeFamilyOpenAI, store.EndpointCapabilityOpenAIEmbeddings, responseFormatOpenAI),
	unsupportedRoute(http.MethodPost, "/v1/images/generations", routeFamilyOpenAI, store.EndpointCapabilityOpenAIImages, responseFormatOpenAI),
	unsupportedRoute(http.MethodPost, "/v1/images/edits", routeFamilyOpenAI, store.EndpointCapabilityOpenAIImages, responseFormatOpenAI),
	unsupportedRoute(http.MethodPost, "/v1/audio/transcriptions", routeFamilyOpenAI, store.EndpointCapabilityOpenAIAudio, responseFormatOpenAI),
	unsupportedRoute(http.MethodPost, "/v1/audio/translations", routeFamilyOpenAI, store.EndpointCapabilityOpenAIAudio, responseFormatOpenAI),
	unsupportedRoute(http.MethodPost, "/v1/audio/speech", routeFamilyOpenAI, store.EndpointCapabilityOpenAIAudio, responseFormatOpenAI),
	unsupportedRoute(http.MethodPost, "/v1/rerank", routeFamilyOpenAI, store.EndpointCapabilityOpenAIRerank, responseFormatOpenAI),
	unsupportedRoute(http.MethodPost, "/v1/messages", routeFamilyClaude, store.EndpointCapabilityClaudeMessages, responseFormatJSON),
	unsupportedRoute(http.MethodPost, "/v1beta/models/{model}:generateContent", routeFamilyGemini, store.EndpointCapabilityGeminiGenerate, responseFormatGemini),
	unsupportedRoute(http.MethodPost, "/v1beta/models/{model}:streamGenerateContent", routeFamilyGemini, store.EndpointCapabilityGeminiGenerate, responseFormatGemini),
	unsupportedRoute(http.MethodGet, "/v1/realtime", routeFamilyRealtime, store.EndpointCapabilityRealtime, responseFormatJSON),
	unsupportedRoute(http.MethodPost, "/v1/tasks/video", routeFamilyTask, store.EndpointCapabilityTaskVideo, responseFormatJSON),
	unsupportedRoute(http.MethodGet, "/v1/tasks/{task_id}", routeFamilyTask, store.EndpointCapabilityTaskVideo, responseFormatJSON),
	unsupportedRoute(http.MethodPost, "/v1/videos/generations", routeFamilyTask, store.EndpointCapabilityTaskVideo, responseFormatJSON),
	unsupportedRoute(http.MethodGet, "/v1/videos/{task_id}", routeFamilyTask, store.EndpointCapabilityTaskVideo, responseFormatJSON),
}

func unsupportedRoute(method string, pattern string, family string, capability string, format string) relayRouteDescriptor {
	return relayRouteDescriptor{
		Method:         method,
		PathPattern:    pattern,
		RouteFamily:    family,
		Capability:     capability,
		ResponseFormat: format,
		Implementation: routeStatusUnsupported,
	}
}

func (s *Server) relayRouteHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var pathMatched []relayRouteDescriptor
		for _, descriptor := range relayRouteMatrix {
			params, ok := matchRoutePath(descriptor.PathPattern, r.URL.Path)
			if !ok {
				continue
			}
			pathMatched = append(pathMatched, descriptor)
			if descriptor.Method != r.Method {
				continue
			}
			s.handleRelayRoute(w, r, descriptor, params)
			return
		}

		if len(pathMatched) > 0 {
			writeMethodNotAllowed(w, pathMatched)
			return
		}
		http.NotFound(w, r)
	}
}

func (s *Server) handleRelayRoute(w http.ResponseWriter, r *http.Request, descriptor relayRouteDescriptor, params map[string]string) {
	switch descriptor.Implementation {
	case routeStatusImplemented:
		s.handleImplementedRelayRoute(w, r, descriptor)
	case routeStatusMetadata:
		if !s.checkPublicAuth(w, r) {
			return
		}
		s.handleModelMetadataRoute(w, r, descriptor, params)
	case routeStatusUnsupported:
		if !s.checkPublicAuth(w, r) {
			return
		}
		writeUnsupportedEndpoint(w, descriptor)
	default:
		writeOpenAIError(w, http.StatusInternalServerError, "api_error", "unsupported_route_status", "unsupported route status")
	}
}

func (s *Server) handleImplementedRelayRoute(w http.ResponseWriter, r *http.Request, descriptor relayRouteDescriptor) {
	switch descriptor.ImplementationID {
	case string(upstream.ChatCompletions):
		s.openAIRoute(upstream.ChatCompletions)(w, r)
	case string(upstream.Completions):
		s.openAIRoute(upstream.Completions)(w, r)
	default:
		writeOpenAIError(w, http.StatusInternalServerError, "api_error", "unsupported_route_status", "unsupported route implementation")
	}
}

func writeMethodNotAllowed(w http.ResponseWriter, descriptors []relayRouteDescriptor) {
	methods := make([]string, 0, len(descriptors))
	seen := map[string]bool{}
	for _, descriptor := range descriptors {
		if seen[descriptor.Method] {
			continue
		}
		seen[descriptor.Method] = true
		methods = append(methods, descriptor.Method)
	}
	sort.Strings(methods)
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeOpenAIError(w, http.StatusMethodNotAllowed, "invalid_request_error", "method_not_allowed", "method not allowed")
}

type unsupportedEndpointError struct {
	Error unsupportedEndpointErrorBody `json:"error"`
}

type unsupportedEndpointErrorBody struct {
	Message     string `json:"message"`
	Type        string `json:"type"`
	Code        string `json:"code"`
	Method      string `json:"method"`
	PathPattern string `json:"path_pattern"`
	RouteFamily string `json:"route_family"`
	Capability  string `json:"capability"`
}

func writeUnsupportedEndpoint(w http.ResponseWriter, descriptor relayRouteDescriptor) {
	writeJSON(w, http.StatusNotImplemented, unsupportedEndpointError{
		Error: unsupportedEndpointErrorBody{
			Message:     "endpoint is recognized but not implemented",
			Type:        "invalid_request_error",
			Code:        "unsupported_endpoint",
			Method:      descriptor.Method,
			PathPattern: descriptor.PathPattern,
			RouteFamily: descriptor.RouteFamily,
			Capability:  descriptor.Capability,
		},
	})
}

func matchRoutePath(pattern string, path string) (map[string]string, bool) {
	patternSegments := splitRoutePath(pattern)
	pathSegments := splitRoutePath(path)
	if len(patternSegments) != len(pathSegments) {
		return nil, false
	}

	params := map[string]string{}
	for i, patternSegment := range patternSegments {
		name, value, ok := matchRouteSegment(patternSegment, pathSegments[i])
		if !ok {
			return nil, false
		}
		if name != "" {
			decoded, err := url.PathUnescape(value)
			if err != nil {
				return nil, false
			}
			params[name] = decoded
		}
	}
	return params, true
}

func splitRoutePath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func matchRouteSegment(pattern string, segment string) (string, string, bool) {
	open := strings.Index(pattern, "{")
	close := strings.Index(pattern, "}")
	if open < 0 || close < 0 || close < open {
		return "", "", pattern == segment
	}

	prefix := pattern[:open]
	name := pattern[open+1 : close]
	suffix := pattern[close+1:]
	if name == "" || !strings.HasPrefix(segment, prefix) || !strings.HasSuffix(segment, suffix) {
		return "", "", false
	}
	value := strings.TrimSuffix(strings.TrimPrefix(segment, prefix), suffix)
	if value == "" {
		return "", "", false
	}
	return name, value, true
}
