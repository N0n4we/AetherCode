package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

type requestBody struct {
	Model    string          `json:"model"`
	Prompt   json.RawMessage `json:"prompt"`
	Messages json.RawMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	addr := getenv("MOCK_PROVIDER_ADDR", ":8081")

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", health)
	mux.HandleFunc("/v1/chat/completions", handleCompletion("chat.completion"))
	mux.HandleFunc("/v1/completions", handleCompletion("text_completion"))

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	logger.Info("mock provider listening", "addr", addr)
	if err := server.ListenAndServe(); err != nil {
		logger.Error("listen", "error", err)
		os.Exit(1)
	}
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func handleCompletion(object string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if auth := r.Header.Get("Authorization"); auth == "" {
			http.Error(w, "missing authorization", http.StatusUnauthorized)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		var req requestBody
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.Model) == "" {
			http.Error(w, "missing model", http.StatusBadRequest)
			return
		}

		pod := getenv("HOSTNAME", "mock-provider")
		content := fmt.Sprintf("mock provider %s handled model %s", pod, req.Model)
		if req.Stream {
			writeStream(w, req.Model, content)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":      "mock-" + fmt.Sprint(time.Now().UnixNano()),
			"object":  object,
			"created": time.Now().Unix(),
			"model":   req.Model,
			"choices": []map[string]interface{}{{
				"index": 0,
				"message": map[string]string{
					"role":    "assistant",
					"content": content,
				},
				"text":          content,
				"finish_reason": "stop",
			}},
			"usage": map[string]int{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		})
	}
}

func writeStream(w http.ResponseWriter, model string, content string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	chunks := []string{"mock ", "stream ", "ok"}
	for _, chunk := range chunks {
		payload := map[string]interface{}{
			"id":      "mock-stream",
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   model,
			"choices": []map[string]interface{}{{
				"index": 0,
				"delta": map[string]string{
					"content": chunk,
				},
				"finish_reason": nil,
			}},
		}
		data, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	_ = content
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
