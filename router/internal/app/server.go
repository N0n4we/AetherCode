package app

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"aethercode-router/internal/config"
	"aethercode-router/internal/store"
	"aethercode-router/internal/upstream"
)

type Server struct {
	cfg      config.Config
	store    *store.Store
	cache    *store.Cache
	upstream *upstream.Client
	logger   *slog.Logger
}

func New(cfg config.Config, db *store.Store, cache *store.Cache, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		cfg:      cfg,
		store:    db,
		cache:    cache,
		upstream: upstream.NewClient(cfg.RequestTimeout),
		logger:   logger,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/readyz", s.ready)
	mux.HandleFunc("/internal/status", s.adminStatus)
	mux.HandleFunc("/internal/providers", s.adminProviders)
	mux.HandleFunc("/internal/providers/", s.adminProviderByID)
	mux.HandleFunc("/", s.relayRouteHandler())
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	dbVersion, err := s.store.ProviderVersion(r.Context())
	stats := s.cache.Stats()
	if err != nil || stats.Version == 0 || stats.Version < dbVersion {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"status":       "not_ready",
			"cache":        stats,
			"db_version":   dbVersion,
			"cache_loaded": stats.Version > 0,
		})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ready",
		"cache":      stats,
		"db_version": dbVersion,
	})
}

func bearerToken(header string) string {
	header = strings.TrimSpace(header)
	if len(header) >= 7 && strings.EqualFold(header[:7], "Bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return ""
}

func keyFromRequest(r *http.Request) string {
	if token := bearerToken(r.Header.Get("Authorization")); token != "" {
		return token
	}
	if token := strings.TrimSpace(r.Header.Get("x-api-key")); token != "" {
		return token
	}
	return ""
}

func (s *Server) checkPublicAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.cfg.APIKey == "" {
		return true
	}
	if keyFromRequest(r) == s.cfg.APIKey {
		return true
	}
	writeOpenAIError(w, http.StatusUnauthorized, "invalid_request_error", "unauthorized", "invalid router API key")
	return false
}

func (s *Server) checkAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	if s.cfg.AdminKey == "" {
		http.Error(w, "ROUTER_ADMIN_KEY is not configured", http.StatusForbidden)
		return false
	}
	if keyFromRequest(r) == s.cfg.AdminKey {
		return true
	}
	http.Error(w, "invalid admin key", http.StatusUnauthorized)
	return false
}
