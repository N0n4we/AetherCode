package account

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"aethercode-router/internal/config"
	"aethercode-router/internal/store"
)

func TestAPIKeyLifecycleReturnsSecretOnceAndScopesByAccount(t *testing.T) {
	db, err := store.Open("sqlite://" + filepath.Join(t.TempDir(), "account.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	server := New(config.Config{
		AccountServiceKey: "service-secret",
		APIKeyHashSecret:  "hash-secret",
	}, db)

	createReq := httptest.NewRequest(http.MethodPost, "/account/api-keys", strings.NewReader(`{"name":"prod key"}`))
	createReq.Header.Set("Authorization", "Bearer service-secret")
	createReq.Header.Set("X-Aether-Account-ID", "acct-a")
	createRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", createRec.Code, createRec.Body.String())
	}
	var created store.APIKeyCreation
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.Secret == "" || created.KeyPrefix == "" {
		t.Fatalf("create response did not include one-time secret metadata: %+v", created)
	}
	if _, err := db.ValidateAPIKey(createReq.Context(), created.Secret, "hash-secret"); err != nil {
		t.Fatalf("created key did not validate: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/account/api-keys", nil)
	listReq.Header.Set("Authorization", "Bearer service-secret")
	listReq.Header.Set("X-Aether-Account-ID", "acct-a")
	listRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("expected 200 list, got %d: %s", listRec.Code, listRec.Body.String())
	}
	if bytes.Contains(listRec.Body.Bytes(), []byte(created.Secret)) || bytes.Contains(listRec.Body.Bytes(), []byte(`"secret"`)) {
		t.Fatalf("list response exposed raw secret: %s", listRec.Body.String())
	}

	otherReq := httptest.NewRequest(http.MethodPost, "/account/api-keys/"+uintString(created.ID)+"/revoke", nil)
	otherReq.Header.Set("Authorization", "Bearer service-secret")
	otherReq.Header.Set("X-Aether-Account-ID", "acct-b")
	otherRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(otherRec, otherReq)
	if otherRec.Code != http.StatusNotFound {
		t.Fatalf("expected cross-account revoke to fail, got %d: %s", otherRec.Code, otherRec.Body.String())
	}

	revokeReq := httptest.NewRequest(http.MethodPost, "/account/api-keys/"+uintString(created.ID)+"/revoke", nil)
	revokeReq.Header.Set("Authorization", "Bearer service-secret")
	revokeReq.Header.Set("X-Aether-Account-ID", "acct-a")
	revokeRec := httptest.NewRecorder()
	server.Routes().ServeHTTP(revokeRec, revokeReq)
	if revokeRec.Code != http.StatusOK {
		t.Fatalf("expected revoke 200, got %d: %s", revokeRec.Code, revokeRec.Body.String())
	}
	if _, err := db.ValidateAPIKey(revokeReq.Context(), created.Secret, "hash-secret"); err == nil {
		t.Fatalf("revoked key still validated")
	}
}

func uintString(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
