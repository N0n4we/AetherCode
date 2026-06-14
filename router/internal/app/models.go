package app

import (
	"net/http"
	"strings"

	"aethercode-router/internal/store"
)

type openAIModelListResponse struct {
	Object string              `json:"object"`
	Data   []openAIModelObject `json:"data"`
}

type openAIModelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

type geminiModelListResponse struct {
	Models []geminiModelObject `json:"models"`
}

type geminiModelObject struct {
	Name                       string   `json:"name"`
	Version                    string   `json:"version,omitempty"`
	DisplayName                string   `json:"displayName"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

func (s *Server) handleModelMetadataRoute(w http.ResponseWriter, _ *http.Request, descriptor relayRouteDescriptor, params map[string]string) {
	switch descriptor.ResponseFormat {
	case responseFormatOpenAI:
		if model := params["model"]; model != "" {
			s.writeOpenAIModel(w, model)
			return
		}
		s.writeOpenAIModels(w)
	case responseFormatGemini:
		s.writeGeminiModels(w)
	default:
		writeOpenAIError(w, http.StatusInternalServerError, "api_error", "unsupported_model_format", "unsupported model response format")
	}
}

func (s *Server) writeOpenAIModels(w http.ResponseWriter) {
	models := s.cache.Models()
	data := make([]openAIModelObject, 0, len(models))
	for _, model := range models {
		data = append(data, openAIModelObject{
			ID:      model.ID,
			Object:  "model",
			Created: 0,
			OwnedBy: "aethercode",
		})
	}
	writeJSON(w, http.StatusOK, openAIModelListResponse{
		Object: "list",
		Data:   data,
	})
}

func (s *Server) writeOpenAIModel(w http.ResponseWriter, modelID string) {
	model, ok := s.cache.Model(modelID)
	if !ok {
		writeOpenAIError(w, http.StatusNotFound, "invalid_request_error", "model_not_found", "model not found")
		return
	}
	writeJSON(w, http.StatusOK, openAIModelObject{
		ID:      model.ID,
		Object:  "model",
		Created: 0,
		OwnedBy: "aethercode",
	})
}

func (s *Server) writeGeminiModels(w http.ResponseWriter) {
	models := s.cache.Models()
	data := make([]geminiModelObject, 0, len(models))
	for _, model := range models {
		data = append(data, geminiModelObject{
			Name:                       "models/" + model.ID,
			DisplayName:                model.ID,
			SupportedGenerationMethods: geminiGenerationMethods(model),
		})
	}
	writeJSON(w, http.StatusOK, geminiModelListResponse{Models: data})
}

func geminiGenerationMethods(model store.ModelMetadata) []string {
	for _, capability := range model.Capabilities {
		if strings.EqualFold(capability, store.EndpointCapabilityGeminiGenerate) {
			return []string{"generateContent", "streamGenerateContent"}
		}
	}
	return []string{}
}
