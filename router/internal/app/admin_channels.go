package app

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"aethercode-router/internal/store"
)

func (s *Server) adminProviderChannels(w http.ResponseWriter, r *http.Request) {
	if !s.checkAdminAuth(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		channels, err := s.store.ProviderChannels(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		public := make([]store.PublicProviderChannel, 0, len(channels))
		for _, channel := range channels {
			public = append(public, channel.Public())
		}
		writeJSON(w, http.StatusOK, public)
	case http.MethodPost:
		var channel store.ProviderChannel
		if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		if err := s.store.CreateProviderChannel(r.Context(), &channel); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !s.reloadLocalCache(w, r) {
			return
		}
		writeJSON(w, http.StatusCreated, channel.Public())
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) adminProviderChannelByID(w http.ResponseWriter, r *http.Request) {
	if !s.checkAdminAuth(w, r) {
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, "/internal/provider-channels/"), "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "invalid provider channel id", http.StatusBadRequest)
		return
	}
	id64, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || id64 == 0 {
		http.Error(w, "invalid provider channel id", http.StatusBadRequest)
		return
	}
	id := uint(id64)

	if len(parts) == 2 && parts[1] == "disable" {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := s.store.DisableProviderChannel(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !s.reloadLocalCache(w, r) {
			return
		}
		channel, err := s.store.ProviderChannel(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, channel.Public())
		return
	}
	if len(parts) != 1 {
		http.NotFound(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		channel, err := s.store.ProviderChannel(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, channel.Public())
	case http.MethodPut:
		var channel store.ProviderChannel
		if err := json.NewDecoder(r.Body).Decode(&channel); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		channel.ID = id
		if err := s.store.SaveProviderChannel(r.Context(), &channel); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !s.reloadLocalCache(w, r) {
			return
		}
		writeJSON(w, http.StatusOK, channel.Public())
	case http.MethodDelete:
		if err := s.store.DeleteProviderChannel(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if !s.reloadLocalCache(w, r) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, PUT, DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
