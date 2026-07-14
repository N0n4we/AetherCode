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

func TestTokenBasedDynamicChargeReflectsRequestCost(t *testing.T) {
	ctx := context.Background()
	db, err := Open("sqlite://" + filepath.Join(t.TempDir(), "token-billing.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	created, err := db.CreateAPIKey(ctx, "acct-token", "token key", "hash-secret")
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}
	accountID := created.AccountID
	channelID := uint(42)

	// Default model price is a cheap per-request base fee.
	if err := db.CreatePriceConfig(ctx, &PriceConfig{
		PublicModelID:   "gpt-4o",
		UsageClass:      UsageClassRequest,
		CacheState:      CacheStateUnknown,
		UnitPriceMicros: 5,
	}); err != nil {
		t.Fatalf("create default price: %v", err)
	}
	// Channel-specific price bills per token, with a discounted cached rate.
	if err := db.CreatePriceConfig(ctx, &PriceConfig{
		PublicModelID:              "gpt-4o",
		ProviderChannelID:          &channelID,
		UsageClass:                 UsageClassRequest,
		CacheState:                 CacheStateUnknown,
		UnitPriceMicros:            10,
		InputUnitPriceMicros:       2,
		CachedInputUnitPriceMicros: 1,
		OutputUnitPriceMicros:      5,
	}); err != nil {
		t.Fatalf("create channel token price: %v", err)
	}

	event := &UsageEvent{
		EventID:           "usage-token",
		RequestID:         "req-token",
		AccountID:         accountID,
		APIKeyID:          created.ID,
		PublicModelID:     "gpt-4o",
		ProviderChannelID: &channelID,
		UsageClass:        UsageClassRequest,
		CacheState:        CacheStateUnknown,
		Outcome:           UsageOutcomeSuccess,
		StatusCode:        200,
		InputUnits:        1000,
		CachedInputUnits:  200,
		OutputUnits:       500,
		CompletedAt:       time.Now(),
	}
	persisted, err := db.CreateUsageEvent(ctx, event)
	if err != nil {
		t.Fatalf("create usage event: %v", err)
	}
	if _, err := db.CreateBillableChargeForEvent(ctx, persisted); err != nil {
		t.Fatalf("create charge: %v", err)
	}

	var charges []BillableCharge
	if err := db.DB().Find(&charges).Error; err != nil {
		t.Fatalf("query charges: %v", err)
	}
	// base 10 + regular input (800*2=1600) + cached (200*1=200) + output (500*5=2500) = 4310.
	if len(charges) != 1 || charges[0].AmountMicros != 4310 {
		t.Fatalf("unexpected token-based charge: %+v", charges)
	}
	if charges[0].InputUnits != 1000 || charges[0].CachedInputUnits != 200 || charges[0].OutputUnits != 500 {
		t.Fatalf("charge did not record usage breakdown: %+v", charges[0])
	}
}

func TestCachedInputFallsBackToInputRate(t *testing.T) {
	price := PriceConfig{
		UnitPriceMicros:      0,
		InputUnitPriceMicros: 3,
		// CachedInputUnitPriceMicros intentionally unset (0) to verify fallback.
		OutputUnitPriceMicros: 4,
	}
	event := &UsageEvent{
		BillableUnits:    1,
		InputUnits:       100,
		CachedInputUnits: 40,
		OutputUnits:      10,
	}
	// Cached tokens fall back to the input rate, so the total is
	// base 0 + input (100*3=300) + output (10*4=40) = 340.
	if got := price.AmountMicros(event); got != 340 {
		t.Fatalf("expected cached fallback amount 340, got %d", got)
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
