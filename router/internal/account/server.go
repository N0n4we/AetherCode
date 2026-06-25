package account

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"aethercode-router/internal/config"
	"aethercode-router/internal/store"
)

type Server struct {
	cfg   config.Config
	store *store.Store
}

func New(cfg config.Config, db *store.Store) *Server {
	return &Server{cfg: cfg, store: db}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/readyz", s.ready)
	mux.HandleFunc("/account/api-keys", s.apiKeys)
	mux.HandleFunc("/account/api-keys/", s.apiKeyByID)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "error": "database not configured"})
		return
	}
	if _, err := s.store.APIKeyVersion(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) apiKeys(w http.ResponseWriter, r *http.Request) {
	accountID, ok := s.authenticatedAccount(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
			return
		}
		created, err := s.store.CreateAPIKey(r.Context(), accountID, body.Name, s.cfg.APIKeyHashSecret)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, created)
	case http.MethodGet:
		keys, err := s.store.APIKeys(r.Context(), accountID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, keys)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (s *Server) apiKeyByID(w http.ResponseWriter, r *http.Request) {
	accountID, ok := s.authenticatedAccount(w, r)
	if !ok {
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/account/api-keys/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || parts[0] == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	id64, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || id64 == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid api key id"})
		return
	}
	if r.Method != http.MethodPost && !(r.Method == http.MethodDelete && parts[1] == "revoke") {
		w.Header().Set("Allow", "POST, DELETE")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var metadata *store.APIKeyMetadata
	switch parts[1] {
	case "disable":
		metadata, err = s.store.DisableAPIKey(r.Context(), accountID, uint(id64))
	case "revoke":
		metadata, err = s.store.RevokeAPIKey(r.Context(), accountID, uint(id64))
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "api key not found"})
		return
	}
	writeJSON(w, http.StatusOK, metadata)
}

func (s *Server) authenticatedAccount(w http.ResponseWriter, r *http.Request) (string, bool) {
	if s.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not configured"})
		return "", false
	}
	if s.cfg.AccountServiceKey != "" && bearerToken(r.Header.Get("Authorization")) != s.cfg.AccountServiceKey {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid account service key"})
		return "", false
	}
	accountID := strings.TrimSpace(r.Header.Get("X-Aether-Account-ID"))
	if accountID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "account identity is required"})
		return "", false
	}
	return accountID, true
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if len(header) >= 7 && strings.EqualFold(header[:7], "Bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
