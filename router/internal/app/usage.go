package app

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"aethercode-router/internal/store"
)

type relayUsageRecord struct {
	StartedAt          time.Time
	CompletedAt        time.Time
	ModelID            string
	Selected           *selectedProviderMetadata
	EndpointCapability string
	Outcome            string
	StatusCode         int
	UpstreamStatus     int
	CacheState         string
	ErrorCode          string
}

func (s *Server) recordRelayUsage(r *http.Request, record relayUsageRecord) {
	if s.store == nil {
		return
	}
	identity, ok := authIdentityFromContext(r.Context())
	if !ok || identity.AccountID == 0 || identity.APIKeyID == 0 {
		return
	}
	requestID := requestIDFromRequest(r)
	eventToken := usageEventTokenFromContext(r.Context())
	if eventToken == "" {
		eventToken = randomRequestToken()
	}
	completedAt := record.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now()
	}
	startedAt := record.StartedAt
	if startedAt.IsZero() {
		startedAt = completedAt
	}
	outcome := record.Outcome
	if outcome == "" {
		outcome = store.UsageOutcomeFailed
	}
	cacheState := record.CacheState
	if cacheState == "" {
		cacheState = store.CacheStateUnknown
	}

	event := &store.UsageEvent{
		EventID:            "relay:" + eventToken + ":" + sanitizeRequestID(record.EndpointCapability),
		RequestID:          requestID,
		AccountID:          identity.AccountID,
		APIKeyID:           identity.APIKeyID,
		PublicModelID:      record.ModelID,
		EndpointCapability: record.EndpointCapability,
		UsageClass:         store.UsageClassRequest,
		CacheState:         cacheState,
		Outcome:            outcome,
		StatusCode:         record.StatusCode,
		UpstreamStatus:     record.UpstreamStatus,
		BillableUnits:      1,
		ErrorCode:          record.ErrorCode,
		StartedAt:          startedAt,
		CompletedAt:        completedAt,
		DurationMillis:     completedAt.Sub(startedAt).Milliseconds(),
	}
	if record.Selected != nil && record.Selected.provider != nil {
		event.ProviderID = record.Selected.provider.ID
		if record.Selected.provider.PlatformChannelID != 0 {
			channelID := record.Selected.provider.PlatformChannelID
			event.ProviderChannelID = &channelID
		}
	}

	persisted, err := s.store.CreateUsageEvent(r.Context(), event)
	if err != nil {
		s.logger.Warn("record relay usage event failed", "request_id", requestID, "error", err)
		return
	}
	if _, err := s.store.CreateBillableChargeForEvent(r.Context(), persisted); err != nil {
		s.logger.Warn("record billable charge failed", "request_id", requestID, "usage_event_id", persisted.ID, "error", err)
	}
}

func requestIDFromRequest(r *http.Request) string {
	for _, header := range []string{"X-Request-ID", "X-Aether-Request-ID"} {
		value := strings.TrimSpace(r.Header.Get(header))
		if value != "" {
			return sanitizeRequestID(value)
		}
	}
	return randomRequestToken()
}

func randomRequestToken() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf[:])
}

func sanitizeRequestID(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == ':' {
			builder.WriteRune(r)
		}
	}
	if builder.Len() == 0 {
		return "request"
	}
	return builder.String()
}

func cacheStateFromResponse(resp *http.Response) string {
	if resp == nil {
		return store.CacheStateUnknown
	}
	for _, header := range []string{"X-Aether-Cache", "X-Cache", "CF-Cache-Status"} {
		switch strings.ToLower(strings.TrimSpace(resp.Header.Get(header))) {
		case "hit", "cache-hit", "cached":
			return "hit"
		case "miss", "cache-miss", "dynamic", "bypass":
			return "miss"
		}
	}
	return store.CacheStateUnknown
}
