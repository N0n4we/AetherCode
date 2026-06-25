package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDynamicPriceResolutionAndIdempotentCharges(t *testing.T) {
	ctx := context.Background()
	db, err := Open("sqlite://" + filepath.Join(t.TempDir(), "billing.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	created, err := db.CreateAPIKey(ctx, "acct-billing", "billing key", "hash-secret")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	accountID := created.AccountID
	channelID := uint(9)
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-1 * time.Hour)

	if err := db.CreatePriceConfig(ctx, &PriceConfig{
		PublicModelID:   "gpt-4o",
		UsageClass:      UsageClassRequest,
		CacheState:      CacheStateUnknown,
		UnitPriceMicros: 100,
		EffectiveAt:     older,
	}); err != nil {
		t.Fatalf("create older default price: %v", err)
	}
	if err := db.CreatePriceConfig(ctx, &PriceConfig{
		PublicModelID:   "gpt-4o",
		UsageClass:      UsageClassRequest,
		CacheState:      CacheStateUnknown,
		UnitPriceMicros: 150,
		EffectiveAt:     newer,
	}); err != nil {
		t.Fatalf("create newer default price: %v", err)
	}
	if err := db.CreatePriceConfig(ctx, &PriceConfig{
		AccountID:         &accountID,
		PublicModelID:     "gpt-4o",
		ProviderChannelID: &channelID,
		UsageClass:        UsageClassRequest,
		CacheState:        "hit",
		UnitPriceMicros:   25,
		EffectiveAt:       newer,
	}); err != nil {
		t.Fatalf("create channel cache-hit price: %v", err)
	}

	defaultPrice, err := db.ResolvePrice(ctx, PriceResolutionInput{
		AccountID:     accountID,
		PublicModelID: "gpt-4o",
		UsageClass:    UsageClassRequest,
		CacheState:    "miss",
		At:            time.Now(),
	})
	if err != nil {
		t.Fatalf("resolve default price: %v", err)
	}
	if defaultPrice == nil || defaultPrice.UnitPriceMicros != 150 {
		t.Fatalf("expected newer default price, got %+v", defaultPrice)
	}

	cacheHitPrice, err := db.ResolvePrice(ctx, PriceResolutionInput{
		AccountID:         accountID,
		PublicModelID:     "gpt-4o",
		ProviderChannelID: &channelID,
		UsageClass:        UsageClassRequest,
		CacheState:        "hit",
		At:                time.Now(),
	})
	if err != nil {
		t.Fatalf("resolve cache-hit price: %v", err)
	}
	if cacheHitPrice == nil || cacheHitPrice.UnitPriceMicros != 25 {
		t.Fatalf("expected channel cache-hit price, got %+v", cacheHitPrice)
	}

	event := &UsageEvent{
		EventID:            "usage-idempotent",
		RequestID:          "req-idempotent",
		AccountID:          accountID,
		APIKeyID:           created.ID,
		PublicModelID:      "gpt-4o",
		ProviderChannelID:  &channelID,
		EndpointCapability: EndpointCapabilityOpenAIChatCompletions,
		UsageClass:         UsageClassRequest,
		CacheState:         "hit",
		Outcome:            UsageOutcomeSuccess,
		StatusCode:         200,
		BillableUnits:      3,
		CompletedAt:        time.Now(),
	}
	first, err := db.CreateUsageEvent(ctx, event)
	if err != nil {
		t.Fatalf("create usage event: %v", err)
	}
	second, err := db.CreateUsageEvent(ctx, event)
	if err != nil {
		t.Fatalf("create duplicate usage event: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("duplicate usage write created a new row: first=%d second=%d", first.ID, second.ID)
	}
	if _, err := db.CreateBillableChargeForEvent(ctx, first); err != nil {
		t.Fatalf("create charge: %v", err)
	}
	if _, err := db.CreateBillableChargeForEvent(ctx, second); err != nil {
		t.Fatalf("create duplicate charge: %v", err)
	}
	var charges []BillableCharge
	if err := db.DB().Find(&charges).Error; err != nil {
		t.Fatalf("query charges: %v", err)
	}
	if len(charges) != 1 || charges[0].AmountMicros != 75 {
		t.Fatalf("unexpected idempotent charges: %+v", charges)
	}
}

func TestProviderChannelProjectionPreservesSingleModelSelection(t *testing.T) {
	ctx := context.Background()
	db, err := Open("sqlite://" + filepath.Join(t.TempDir(), "channels.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.CreateProviderChannel(ctx, &ProviderChannel{
		Name:          "primary",
		Provider:      "openai",
		PublicModelID: "gpt-4o",
		UpstreamModel: "deployment-primary",
		Status:        StatusEnabled,
		Priority:      10,
		Weight:        1,
	}); err != nil {
		t.Fatalf("create primary channel: %v", err)
	}
	if err := db.CreateProviderChannel(ctx, &ProviderChannel{
		Name:          "secondary",
		Provider:      "openai",
		PublicModelID: "gpt-4o",
		UpstreamModel: "deployment-secondary",
		Status:        StatusEnabled,
		Priority:      1,
		Weight:        100,
	}); err != nil {
		t.Fatalf("create secondary channel: %v", err)
	}
	cache := NewCache()
	if _, err := ReloadCache(ctx, db, cache); err != nil {
		t.Fatalf("reload cache: %v", err)
	}
	selected, err := cache.Select("gpt-4o", nil)
	if err != nil {
		t.Fatalf("select projected channel: %v", err)
	}
	if selected.PlatformChannelID == 0 || selected.Name != "primary" || selected.UpstreamModel("gpt-4o") != "deployment-primary" {
		t.Fatalf("unexpected selected channel provider: %+v", selected)
	}
}

func TestImportLegacyProvidersAsSingleModelChannels(t *testing.T) {
	ctx := context.Background()
	db, err := Open("sqlite://" + filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.CreateProvider(ctx, &Provider{
		Name:       "legacy",
		Provider:   "openai",
		BaseURL:    "https://provider.example/v1",
		APIKey:     "sk-legacy-secret",
		Models:     "gpt-4o,gpt-4o-mini",
		Status:     StatusEnabled,
		Priority:   5,
		Weight:     10,
		AuthHeader: "Authorization",
		AuthPrefix: "Bearer ",
	}); err != nil {
		t.Fatalf("create legacy provider: %v", err)
	}
	imported, err := db.ImportLegacyProvidersAsChannels(ctx)
	if err != nil {
		t.Fatalf("import legacy providers: %v", err)
	}
	if imported != 2 {
		t.Fatalf("expected two imported channels, got %d", imported)
	}
	channels, err := db.ProviderChannels(ctx)
	if err != nil {
		t.Fatalf("list channels: %v", err)
	}
	if len(channels) != 2 {
		t.Fatalf("expected two channels, got %+v", channels)
	}
	for _, channel := range channels {
		if channel.LegacyProviderID == 0 || channel.PublicModelID == "" {
			t.Fatalf("imported channel missing legacy attribution or modelId: %+v", channel)
		}
		if channel.UpstreamAPIKeySecretRef == "" || strings.Contains(channel.UpstreamAPIKeySecretRef, "sk-legacy-secret") {
			t.Fatalf("imported channel did not redact raw upstream secret: %+v", channel)
		}
	}
}
