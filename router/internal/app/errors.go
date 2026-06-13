package app

import (
	"encoding/json"
	"net/http"
)

type openAIError struct {
	Error openAIErrorBody `json:"error"`
}

type openAIErrorBody struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    string  `json:"code"`
}

func writeOpenAIError(w http.ResponseWriter, status int, typ string, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(openAIError{
		Error: openAIErrorBody{
			Message: message,
			Type:    typ,
			Code:    code,
		},
	})
}
