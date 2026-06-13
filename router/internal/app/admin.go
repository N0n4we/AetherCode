package app

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"aethercode-router/internal/store"
)

func (s *Server) adminProviders(w http.ResponseWriter, r *http.Request) {
	if !s.checkAdminAuth(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		providers, err := s.store.Providers(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		public := make([]store.PublicProvider, 0, len(providers))
		for _, provider := range providers {
			public = append(public, provider.Public())
		}
		writeJSON(w, http.StatusOK, public)
	case http.MethodPost:
		var provider store.Provider
		if err := json.NewDecoder(r.Body).Decode(&provider); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if provider.Provider == "" || provider.Models == "" {
			http.Error(w, "provider and models are required", http.StatusBadRequest)
			return
		}
		if err := s.store.CreateProvider(r.Context(), &provider); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = store.ReloadCache(r.Context(), s.store, s.cache)
		writeJSON(w, http.StatusCreated, provider.Public())
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) adminProviderByID(w http.ResponseWriter, r *http.Request) {
	if !s.checkAdminAuth(w, r) {
		return
	}
	idPart := strings.TrimPrefix(r.URL.Path, "/internal/providers/")
	if idPart == "sync" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := store.ReloadCache(r.Context(), s.store, s.cache); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "synced"})
		return
	}

	id64, err := strconv.ParseUint(idPart, 10, 64)
	if err != nil || id64 == 0 {
		http.Error(w, "invalid provider id", http.StatusBadRequest)
		return
	}
	id := uint(id64)

	switch r.Method {
	case http.MethodGet:
		provider, err := s.store.Provider(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, provider.Public())
	case http.MethodPut:
		var provider store.Provider
		if err := json.NewDecoder(r.Body).Decode(&provider); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		provider.ID = id
		if provider.Provider == "" || provider.Models == "" {
			http.Error(w, "provider and models are required", http.StatusBadRequest)
			return
		}
		if err := s.store.SaveProvider(r.Context(), &provider); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = store.ReloadCache(r.Context(), s.store, s.cache)
		writeJSON(w, http.StatusOK, provider.Public())
	case http.MethodDelete:
		if err := s.store.DeleteProvider(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = store.ReloadCache(r.Context(), s.store, s.cache)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, PUT, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
