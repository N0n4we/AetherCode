package store

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	APIKeyConfigVersionName          = "api_keys"
	ProviderChannelConfigVersionName = "provider_channels"

	APIKeyStatusActive   = "active"
	APIKeyStatusDisabled = "disabled"
	APIKeyStatusRevoked  = "revoked"

	AccountStatusActive = "active"

	UsageOutcomeSuccess = "success"
	UsageOutcomeFailed  = "failed"
	UsageClassRequest   = "request"
	CacheStateUnknown   = "unknown"

	providerChannelIDOffset uint = 1000000
)

type Account struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	ExternalID string    `json:"external_id" gorm:"size:128;uniqueIndex;not null"`
	Name       string    `json:"name" gorm:"size:255"`
	Status     string    `json:"status" gorm:"size:32;not null;default:'active';index"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (Account) TableName() string {
	return "relay_accounts"
}

type APIKey struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	AccountID     uint       `json:"account_id" gorm:"not null;index"`
	Account       Account    `json:"-" gorm:"constraint:OnDelete:CASCADE"`
	Name          string     `json:"name" gorm:"size:255"`
	KeyPrefix     string     `json:"key_prefix" gorm:"size:32;not null;index"`
	KeyHash       string     `json:"-" gorm:"size:128;not null"`
	HashAlgorithm string     `json:"hash_algorithm" gorm:"size:64;not null"`
	Status        string     `json:"status" gorm:"size:32;not null;default:'active';index"`
	LastUsedAt    *time.Time `json:"last_used_at"`
	DisabledAt    *time.Time `json:"disabled_at"`
	RevokedAt     *time.Time `json:"revoked_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (APIKey) TableName() string {
	return "relay_api_keys"
}

type APIKeyMetadata struct {
	ID            uint       `json:"id"`
	AccountID     uint       `json:"account_id"`
	Account       string     `json:"account"`
	Name          string     `json:"name"`
	KeyPrefix     string     `json:"key_prefix"`
	HashAlgorithm string     `json:"hash_algorithm"`
	Status        string     `json:"status"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	DisabledAt    *time.Time `json:"disabled_at,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type APIKeyCreation struct {
	APIKeyMetadata
	Secret string `json:"secret"`
}

type AuthIdentity struct {
	Source            string `json:"source"`
	AccountID         uint   `json:"account_id,omitempty"`
	AccountExternalID string `json:"account,omitempty"`
	APIKeyID          uint   `json:"api_key_id,omitempty"`
	APIKeyPrefix      string `json:"api_key_prefix,omitempty"`
}

func (s *Store) EnsureAccount(ctx context.Context, externalID string) (*Account, error) {
	return ensureAccount(s.db.WithContext(ctx), externalID)
}

func (s *Store) CreateAPIKey(ctx context.Context, accountExternalID string, name string, hashSecret string) (*APIKeyCreation, error) {
	accountExternalID = strings.TrimSpace(accountExternalID)
	if accountExternalID == "" {
		return nil, errors.New("account is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "relay key"
	}

	raw, err := generateAPIKeySecret()
	if err != nil {
		return nil, err
	}
	key := APIKey{
		Name:          name,
		KeyPrefix:     apiKeyPrefix(raw),
		KeyHash:       hashAPIKey(raw, hashSecret),
		HashAlgorithm: "hmac-sha256",
		Status:        APIKeyStatusActive,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		account, err := ensureAccount(tx, accountExternalID)
		if err != nil {
			return err
		}
		key.AccountID = account.ID
		if err := tx.Create(&key).Error; err != nil {
			return err
		}
		return bumpConfigVersions(tx, APIKeyConfigVersionName)
	})
	if err != nil {
		return nil, err
	}

	key.Account = Account{ID: key.AccountID, ExternalID: accountExternalID}
	metadata := key.Metadata()
	metadata.Account = accountExternalID
	return &APIKeyCreation{
		APIKeyMetadata: metadata,
		Secret:         raw,
	}, nil
}

func (s *Store) APIKeys(ctx context.Context, accountExternalID string) ([]APIKeyMetadata, error) {
	accountExternalID = strings.TrimSpace(accountExternalID)
	if accountExternalID == "" {
		return nil, errors.New("account is required")
	}
	var keys []APIKey
	err := s.db.WithContext(ctx).
		Joins("JOIN relay_accounts ON relay_accounts.id = relay_api_keys.account_id").
		Preload("Account").
		Where("relay_accounts.external_id = ?", accountExternalID).
		Order("relay_api_keys.id asc").
		Find(&keys).Error
	if err != nil {
		return nil, err
	}
	out := make([]APIKeyMetadata, 0, len(keys))
	for _, key := range keys {
		metadata := key.Metadata()
		metadata.Account = accountExternalID
		out = append(out, metadata)
	}
	return out, nil
}

func (s *Store) DisableAPIKey(ctx context.Context, accountExternalID string, id uint) (*APIKeyMetadata, error) {
	return s.setAPIKeyStatus(ctx, accountExternalID, id, APIKeyStatusDisabled)
}

func (s *Store) RevokeAPIKey(ctx context.Context, accountExternalID string, id uint) (*APIKeyMetadata, error) {
	return s.setAPIKeyStatus(ctx, accountExternalID, id, APIKeyStatusRevoked)
}

func (s *Store) setAPIKeyStatus(ctx context.Context, accountExternalID string, id uint, status string) (*APIKeyMetadata, error) {
	accountExternalID = strings.TrimSpace(accountExternalID)
	if accountExternalID == "" {
		return nil, errors.New("account is required")
	}
	now := time.Now()
	var updated APIKey
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var key APIKey
		err := tx.Joins("JOIN relay_accounts ON relay_accounts.id = relay_api_keys.account_id").
			Where("relay_api_keys.id = ? AND relay_accounts.external_id = ?", id, accountExternalID).
			First(&key).Error
		if err != nil {
			return err
		}
		updates := map[string]interface{}{
			"status":     status,
			"updated_at": now,
		}
		if status == APIKeyStatusDisabled {
			updates["disabled_at"] = now
		}
		if status == APIKeyStatusRevoked {
			updates["revoked_at"] = now
		}
		if err := tx.Model(&APIKey{}).Where("id = ?", key.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Preload("Account").First(&updated, "id = ?", key.ID).Error; err != nil {
			return err
		}
		return bumpConfigVersions(tx, APIKeyConfigVersionName)
	})
	if err != nil {
		return nil, err
	}
	metadata := updated.Metadata()
	metadata.Account = accountExternalID
	return &metadata, nil
}

func (s *Store) ValidateAPIKey(ctx context.Context, raw string, hashSecret string) (*AuthIdentity, error) {
	prefix := apiKeyPrefix(raw)
	if prefix == "" {
		return nil, errors.New("api key is required")
	}
	keys, err := s.apiKeysByPrefix(ctx, prefix)
	if err != nil {
		return nil, err
	}
	return validateAPIKeyCandidates(raw, hashSecret, keys)
}

func (s *Store) APIKeyVersion(ctx context.Context) (int64, error) {
	return s.configVersion(ctx, APIKeyConfigVersionName)
}

func (s *Store) ProviderChannelVersion(ctx context.Context) (int64, error) {
	return s.configVersion(ctx, ProviderChannelConfigVersionName)
}

func (s *Store) activeAPIKeys(ctx context.Context) ([]APIKey, error) {
	var keys []APIKey
	err := s.db.WithContext(ctx).
		Preload("Account").
		Where("relay_api_keys.status = ?", APIKeyStatusActive).
		Find(&keys).Error
	return keys, err
}

func (s *Store) apiKeysByPrefix(ctx context.Context, prefix string) ([]APIKey, error) {
	var keys []APIKey
	err := s.db.WithContext(ctx).
		Preload("Account").
		Where("relay_api_keys.key_prefix = ? AND relay_api_keys.status = ?", prefix, APIKeyStatusActive).
		Find(&keys).Error
	return keys, err
}

func (key APIKey) Metadata() APIKeyMetadata {
	return APIKeyMetadata{
		ID:            key.ID,
		AccountID:     key.AccountID,
		Account:       key.Account.ExternalID,
		Name:          key.Name,
		KeyPrefix:     key.KeyPrefix,
		HashAlgorithm: key.HashAlgorithm,
		Status:        key.Status,
		LastUsedAt:    key.LastUsedAt,
		DisabledAt:    key.DisabledAt,
		RevokedAt:     key.RevokedAt,
		CreatedAt:     key.CreatedAt,
		UpdatedAt:     key.UpdatedAt,
	}
}

func ensureAccount(tx *gorm.DB, externalID string) (*Account, error) {
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return nil, errors.New("account is required")
	}
	account := Account{
		ExternalID: externalID,
		Name:       externalID,
		Status:     AccountStatusActive,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&account).Error; err != nil {
		return nil, err
	}
	if err := tx.First(&account, "external_id = ?", externalID).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func generateAPIKeySecret() (string, error) {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "ak_" + base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

func apiKeyPrefix(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if len(raw) <= 12 {
		return raw
	}
	return raw[:12]
}

func hashAPIKey(raw string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(mac.Sum(nil))
}

func validateAPIKeyCandidates(raw string, hashSecret string, keys []APIKey) (*AuthIdentity, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("api key is required")
	}
	presented := hashAPIKey(raw, hashSecret)
	for _, key := range keys {
		if key.Status != APIKeyStatusActive {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(presented), []byte(key.KeyHash)) != 1 {
			continue
		}
		return &AuthIdentity{
			Source:            "account_api_key",
			AccountID:         key.AccountID,
			AccountExternalID: key.Account.ExternalID,
			APIKeyID:          key.ID,
			APIKeyPrefix:      key.KeyPrefix,
		}, nil
	}
	return nil, errors.New("invalid api key")
}

type APIKeyCache struct {
	mu           sync.RWMutex
	version      int64
	lastSyncedAt time.Time
	byPrefix     map[string][]APIKey
}

func NewAPIKeyCache() *APIKeyCache {
	return &APIKeyCache{byPrefix: map[string][]APIKey{}}
}

func ReloadAPIKeyCache(ctx context.Context, db *Store, cache *APIKeyCache) (int64, error) {
	version, err := db.APIKeyVersion(ctx)
	if err != nil {
		return 0, err
	}
	keys, err := db.activeAPIKeys(ctx)
	if err != nil {
		return 0, err
	}
	cache.Replace(keys, version)
	return version, nil
}

func SyncAPIKeyCache(ctx context.Context, db *Store, cache *APIKeyCache, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dbVersion, err := db.APIKeyVersion(ctx)
			if err != nil {
				logger.Error("read api key config version", "error", err)
				continue
			}
			if cache.Version() == dbVersion {
				continue
			}
			loadedVersion, err := ReloadAPIKeyCache(ctx, db, cache)
			if err != nil {
				logger.Error("sync api keys from database", "error", err, "db_version", dbVersion)
				continue
			}
			logger.Info("api keys synced from database", "version", loadedVersion)
		}
	}
}

func (c *APIKeyCache) Replace(keys []APIKey, version int64) {
	byPrefix := map[string][]APIKey{}
	for _, key := range keys {
		if key.Status == APIKeyStatusActive {
			byPrefix[key.KeyPrefix] = append(byPrefix[key.KeyPrefix], key)
		}
	}
	c.mu.Lock()
	c.version = version
	c.lastSyncedAt = time.Now()
	c.byPrefix = byPrefix
	c.mu.Unlock()
}

func (c *APIKeyCache) Validate(raw string, hashSecret string) (*AuthIdentity, error) {
	prefix := apiKeyPrefix(raw)
	if prefix == "" {
		return nil, errors.New("api key is required")
	}
	c.mu.RLock()
	keys := append([]APIKey(nil), c.byPrefix[prefix]...)
	c.mu.RUnlock()
	return validateAPIKeyCandidates(raw, hashSecret, keys)
}

func (c *APIKeyCache) Version() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.version
}

type ProviderChannel struct {
	ID                      uint       `json:"id" gorm:"primaryKey"`
	Name                    string     `json:"name" gorm:"size:128;index"`
	LegacyProviderID        uint       `json:"legacy_provider_id,omitempty" gorm:"index"`
	Provider                string     `json:"provider" gorm:"size:64;not null;index"`
	PublicModelID           string     `json:"model_id" gorm:"column:public_model_id;size:128;not null;index"`
	EndpointCapabilities    StringList `json:"endpoint_capabilities" gorm:"type:text"`
	Status                  int        `json:"status" gorm:"default:1;index"`
	Priority                int64      `json:"priority" gorm:"default:0;index"`
	Weight                  uint       `json:"weight" gorm:"default:0"`
	UpstreamBaseURL         string     `json:"upstream_base_url" gorm:"type:text"`
	UpstreamModel           string     `json:"upstream_model" gorm:"size:255"`
	UpstreamAPIKeySecretRef string     `json:"upstream_api_key_secret_ref" gorm:"size:255"`
	AuthHeader              string     `json:"auth_header" gorm:"size:64"`
	AuthPrefix              string     `json:"auth_prefix" gorm:"size:64"`
	Headers                 StringMap  `json:"headers" gorm:"type:text"`
	ChannelType             string     `json:"channel_type" gorm:"size:64"`
	RelayFormat             string     `json:"relay_format" gorm:"size:64"`
	BillingClass            string     `json:"billing_class" gorm:"size:64"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

func (ProviderChannel) TableName() string {
	return "relay_provider_channels"
}

func (c *ProviderChannel) BeforeCreate(*gorm.DB) error {
	return c.applyDefaults()
}

func (c *ProviderChannel) BeforeSave(*gorm.DB) error {
	return c.applyDefaults()
}

func (c *ProviderChannel) applyDefaults() error {
	c.Name = strings.TrimSpace(c.Name)
	c.Provider = strings.TrimSpace(c.Provider)
	c.PublicModelID = strings.TrimSpace(c.PublicModelID)
	c.UpstreamBaseURL = strings.TrimSpace(c.UpstreamBaseURL)
	c.UpstreamModel = strings.TrimSpace(c.UpstreamModel)
	c.UpstreamAPIKeySecretRef = strings.TrimSpace(c.UpstreamAPIKeySecretRef)
	c.AuthHeader = strings.TrimSpace(c.AuthHeader)
	c.AuthPrefix = strings.TrimSpace(c.AuthPrefix)
	c.ChannelType = strings.TrimSpace(c.ChannelType)
	c.RelayFormat = strings.TrimSpace(c.RelayFormat)
	c.BillingClass = strings.TrimSpace(c.BillingClass)
	if c.Provider == "" {
		return errors.New("provider is required")
	}
	if c.PublicModelID == "" || strings.Contains(c.PublicModelID, ",") || len(strings.Fields(c.PublicModelID)) != 1 {
		return errors.New("provider channel must have exactly one non-empty public modelId")
	}
	if c.Status == 0 {
		c.Status = StatusEnabled
	}
	if c.Headers == nil {
		c.Headers = StringMap{}
	}
	c.EndpointCapabilities = NormalizeEndpointCapabilities(c.EndpointCapabilities)
	if len(c.EndpointCapabilities) == 0 {
		c.EndpointCapabilities = DefaultEndpointCapabilities()
	}
	return nil
}

func (c ProviderChannel) Public() PublicProviderChannel {
	return PublicProviderChannel{
		ID:                      c.ID,
		Name:                    c.Name,
		LegacyProviderID:        c.LegacyProviderID,
		Provider:                c.Provider,
		ModelID:                 c.PublicModelID,
		EndpointCapabilities:    c.EndpointCapabilities,
		Status:                  c.Status,
		Priority:                c.Priority,
		Weight:                  c.Weight,
		UpstreamBaseURL:         c.UpstreamBaseURL,
		UpstreamModel:           c.UpstreamModel,
		UpstreamAPIKeySecretRef: c.UpstreamAPIKeySecretRef,
		AuthHeader:              c.AuthHeader,
		AuthPrefix:              c.AuthPrefix,
		Headers:                 c.Headers,
		ChannelType:             c.ChannelType,
		RelayFormat:             c.RelayFormat,
		BillingClass:            c.BillingClass,
		CreatedAt:               c.CreatedAt,
		UpdatedAt:               c.UpdatedAt,
	}
}

func (c ProviderChannel) ToProvider() Provider {
	mapping := StringMap{}
	if c.UpstreamModel != "" && c.UpstreamModel != c.PublicModelID {
		mapping[c.PublicModelID] = c.UpstreamModel
	}
	return Provider{
		ID:                   providerChannelIDOffset + c.ID,
		PlatformChannelID:    c.ID,
		Name:                 c.Name,
		Provider:             c.Provider,
		BaseURL:              c.UpstreamBaseURL,
		APIKey:               ResolveSecretRef(c.UpstreamAPIKeySecretRef),
		AuthHeader:           c.AuthHeader,
		AuthPrefix:           c.AuthPrefix,
		Models:               c.PublicModelID,
		Groups:               DefaultGroup,
		ModelMapping:         mapping,
		Headers:              c.Headers,
		EndpointCapabilities: c.EndpointCapabilities,
		ChannelType:          c.ChannelType,
		RelayFormat:          c.RelayFormat,
		Status:               c.Status,
		Weight:               c.Weight,
		Priority:             c.Priority,
		CreatedAt:            c.CreatedAt,
		UpdatedAt:            c.UpdatedAt,
	}
}

func ResolveSecretRef(ref string) string {
	ref = strings.TrimSpace(ref)
	switch {
	case strings.HasPrefix(ref, "env:"):
		return os.Getenv(strings.TrimSpace(strings.TrimPrefix(ref, "env:")))
	case strings.HasPrefix(ref, "file:"):
		path := strings.TrimSpace(strings.TrimPrefix(ref, "file:"))
		if path == "" {
			return ""
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	default:
		return ""
	}
}

type PublicProviderChannel struct {
	ID                      uint       `json:"id"`
	Name                    string     `json:"name"`
	LegacyProviderID        uint       `json:"legacy_provider_id,omitempty"`
	Provider                string     `json:"provider"`
	ModelID                 string     `json:"model_id"`
	EndpointCapabilities    StringList `json:"endpoint_capabilities"`
	Status                  int        `json:"status"`
	Priority                int64      `json:"priority"`
	Weight                  uint       `json:"weight"`
	UpstreamBaseURL         string     `json:"upstream_base_url"`
	UpstreamModel           string     `json:"upstream_model"`
	UpstreamAPIKeySecretRef string     `json:"upstream_api_key_secret_ref"`
	AuthHeader              string     `json:"auth_header"`
	AuthPrefix              string     `json:"auth_prefix"`
	Headers                 StringMap  `json:"headers"`
	ChannelType             string     `json:"channel_type"`
	RelayFormat             string     `json:"relay_format"`
	BillingClass            string     `json:"billing_class"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

func (s *Store) ProviderChannels(ctx context.Context) ([]ProviderChannel, error) {
	var channels []ProviderChannel
	err := s.db.WithContext(ctx).Order("priority desc").Order("id asc").Find(&channels).Error
	return channels, err
}

func (s *Store) CreateProviderChannel(ctx context.Context, channel *ProviderChannel) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(channel).Error; err != nil {
			return err
		}
		return bumpConfigVersions(tx, ProviderConfigVersionName, ProviderChannelConfigVersionName)
	})
}

func (s *Store) ProviderChannel(ctx context.Context, id uint) (*ProviderChannel, error) {
	var channel ProviderChannel
	if err := s.db.WithContext(ctx).First(&channel, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &channel, nil
}

func (s *Store) SaveProviderChannel(ctx context.Context, channel *ProviderChannel) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(channel).Error; err != nil {
			return err
		}
		return bumpConfigVersions(tx, ProviderConfigVersionName, ProviderChannelConfigVersionName)
	})
}

func (s *Store) DisableProviderChannel(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&ProviderChannel{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     StatusDisabled,
			"updated_at": time.Now(),
		}).Error; err != nil {
			return err
		}
		return bumpConfigVersions(tx, ProviderConfigVersionName, ProviderChannelConfigVersionName)
	})
}

func (s *Store) DeleteProviderChannel(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&ProviderChannel{}, "id = ?", id).Error; err != nil {
			return err
		}
		return bumpConfigVersions(tx, ProviderConfigVersionName, ProviderChannelConfigVersionName)
	})
}

func (s *Store) RoutingProviders(ctx context.Context) ([]Provider, error) {
	providers, err := s.Providers(ctx)
	if err != nil {
		return nil, err
	}
	channels, err := s.ProviderChannels(ctx)
	if err != nil {
		return nil, err
	}
	for _, channel := range channels {
		providers = append(providers, channel.ToProvider())
	}
	return providers, nil
}

func (s *Store) ImportLegacyProvidersAsChannels(ctx context.Context) (int, error) {
	providers, err := s.Providers(ctx)
	if err != nil {
		return 0, err
	}
	imported := 0
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, provider := range providers {
			for _, modelID := range provider.ModelList() {
				if modelID == "" || modelID == "*" {
					continue
				}
				var count int64
				if err := tx.Model(&ProviderChannel{}).
					Where("legacy_provider_id = ? AND public_model_id = ?", provider.ID, modelID).
					Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					continue
				}
				secretRef := ""
				if strings.TrimSpace(provider.APIKey) != "" {
					secretRef = fmt.Sprintf("legacy-router-providers/%d/api_key", provider.ID)
				}
				channel := ProviderChannel{
					Name:                    provider.Name,
					LegacyProviderID:        provider.ID,
					Provider:                provider.Provider,
					PublicModelID:           modelID,
					EndpointCapabilities:    provider.NormalizedEndpointCapabilities(),
					Status:                  provider.Status,
					Priority:                provider.Priority,
					Weight:                  provider.Weight,
					UpstreamBaseURL:         provider.BaseURL,
					UpstreamModel:           provider.UpstreamModel(modelID),
					UpstreamAPIKeySecretRef: secretRef,
					AuthHeader:              provider.AuthHeader,
					AuthPrefix:              provider.AuthPrefix,
					Headers:                 provider.Headers,
					ChannelType:             provider.ChannelType,
					RelayFormat:             provider.RelayFormat,
				}
				if err := tx.Create(&channel).Error; err != nil {
					return err
				}
				imported++
			}
		}
		if imported == 0 {
			return nil
		}
		return bumpConfigVersions(tx, ProviderConfigVersionName, ProviderChannelConfigVersionName)
	})
	return imported, err
}

type PriceConfig struct {
	ID                uint   `json:"id" gorm:"primaryKey"`
	AccountID         *uint  `json:"account_id" gorm:"index"`
	PublicModelID     string `json:"model_id" gorm:"size:128;index"`
	ProviderChannelID *uint  `json:"provider_channel_id" gorm:"index"`
	UsageClass        string `json:"usage_class" gorm:"size:64;not null;index"`
	CacheState        string `json:"cache_state" gorm:"size:64;not null;index"`
	Currency          string `json:"currency" gorm:"size:16;not null;default:'USD'"`
	// UnitPriceMicros is the per-request base fee applied for each billable
	// request unit. It is retained for backward compatibility and for models
	// billed on a flat per-request basis.
	UnitPriceMicros int64 `json:"unit_price_micros" gorm:"not null"`
	// InputUnitPriceMicros is the price charged per non-cached input (prompt)
	// token reported by the upstream response.
	InputUnitPriceMicros int64 `json:"input_unit_price_micros" gorm:"not null;default:0"`
	// OutputUnitPriceMicros is the price charged per output (completion) token
	// reported by the upstream response.
	OutputUnitPriceMicros int64 `json:"output_unit_price_micros" gorm:"not null;default:0"`
	// CachedInputUnitPriceMicros is the price charged per cached input token.
	// When unset it falls back to InputUnitPriceMicros so cached tokens are
	// never silently free unless explicitly configured.
	CachedInputUnitPriceMicros int64     `json:"cached_input_unit_price_micros" gorm:"not null;default:0"`
	EffectiveAt                time.Time `json:"effective_at" gorm:"not null;index"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

// AmountMicros computes the dynamic billable amount for a usage event using
// the actual request cost captured on the event. The total combines the flat
// per-request base fee with token-based charges so pricing reflects the real
// input, cached-input, and output token counts reported by the selected
// channel's upstream response.
func (p PriceConfig) AmountMicros(event *UsageEvent) int64 {
	if event == nil {
		return 0
	}

	billableUnits := event.BillableUnits
	if billableUnits <= 0 {
		billableUnits = 1
	}
	amount := p.UnitPriceMicros * billableUnits

	inputUnits := event.InputUnits
	if inputUnits < 0 {
		inputUnits = 0
	}
	cachedUnits := event.CachedInputUnits
	if cachedUnits < 0 {
		cachedUnits = 0
	}
	if cachedUnits > inputUnits {
		cachedUnits = inputUnits
	}
	regularInput := inputUnits - cachedUnits

	cachedRate := p.CachedInputUnitPriceMicros
	if cachedRate == 0 {
		cachedRate = p.InputUnitPriceMicros
	}

	outputUnits := event.OutputUnits
	if outputUnits < 0 {
		outputUnits = 0
	}

	amount += p.InputUnitPriceMicros * regularInput
	amount += cachedRate * cachedUnits
	amount += p.OutputUnitPriceMicros * outputUnits
	return amount
}

func (PriceConfig) TableName() string {
	return "relay_price_configs"
}

func (p *PriceConfig) BeforeCreate(*gorm.DB) error {
	p.applyDefaults()
	return nil
}

func (p *PriceConfig) BeforeSave(*gorm.DB) error {
	p.applyDefaults()
	return nil
}

func (p *PriceConfig) applyDefaults() {
	p.PublicModelID = strings.TrimSpace(p.PublicModelID)
	p.UsageClass = strings.TrimSpace(p.UsageClass)
	if p.UsageClass == "" {
		p.UsageClass = UsageClassRequest
	}
	p.CacheState = strings.TrimSpace(p.CacheState)
	if p.CacheState == "" {
		p.CacheState = CacheStateUnknown
	}
	p.Currency = strings.ToUpper(strings.TrimSpace(p.Currency))
	if p.Currency == "" {
		p.Currency = "USD"
	}
	if p.EffectiveAt.IsZero() {
		p.EffectiveAt = time.Now()
	}
}

type UsageEvent struct {
	ID                 uint      `json:"id" gorm:"primaryKey"`
	EventID            string    `json:"event_id" gorm:"size:128;uniqueIndex;not null"`
	RequestID          string    `json:"request_id" gorm:"size:128;index"`
	AccountID          uint      `json:"account_id" gorm:"not null;index"`
	APIKeyID           uint      `json:"api_key_id" gorm:"not null;index"`
	PublicModelID      string    `json:"model_id" gorm:"size:128;index"`
	ProviderID         uint      `json:"provider_id" gorm:"index"`
	ProviderChannelID  *uint     `json:"provider_channel_id" gorm:"index"`
	EndpointCapability string    `json:"endpoint_capability" gorm:"size:128;index"`
	UsageClass         string    `json:"usage_class" gorm:"size:64;not null;index"`
	CacheState         string    `json:"cache_state" gorm:"size:64;not null;index"`
	Outcome            string    `json:"outcome" gorm:"size:32;not null;index"`
	StatusCode         int       `json:"status_code"`
	UpstreamStatus     int       `json:"upstream_status"`
	InputUnits         int64     `json:"input_units"`
	CachedInputUnits   int64     `json:"cached_input_units"`
	OutputUnits        int64     `json:"output_units"`
	TotalUnits         int64     `json:"total_units"`
	BillableUnits      int64     `json:"billable_units"`
	ErrorCode          string    `json:"error_code" gorm:"size:128"`
	StartedAt          time.Time `json:"started_at"`
	CompletedAt        time.Time `json:"completed_at" gorm:"index"`
	DurationMillis     int64     `json:"duration_millis"`
	CreatedAt          time.Time `json:"created_at"`
}

func (UsageEvent) TableName() string {
	return "relay_usage_events"
}

func (e *UsageEvent) BeforeCreate(*gorm.DB) error {
	e.applyDefaults()
	return nil
}

func (e *UsageEvent) applyDefaults() {
	e.EventID = strings.TrimSpace(e.EventID)
	e.RequestID = strings.TrimSpace(e.RequestID)
	e.PublicModelID = strings.TrimSpace(e.PublicModelID)
	e.EndpointCapability = strings.TrimSpace(e.EndpointCapability)
	e.UsageClass = strings.TrimSpace(e.UsageClass)
	if e.UsageClass == "" {
		e.UsageClass = UsageClassRequest
	}
	e.CacheState = strings.TrimSpace(e.CacheState)
	if e.CacheState == "" {
		e.CacheState = CacheStateUnknown
	}
	e.Outcome = strings.TrimSpace(e.Outcome)
	if e.Outcome == "" {
		e.Outcome = UsageOutcomeFailed
	}
	if e.BillableUnits == 0 {
		e.BillableUnits = 1
	}
	if e.InputUnits < 0 {
		e.InputUnits = 0
	}
	if e.OutputUnits < 0 {
		e.OutputUnits = 0
	}
	if e.CachedInputUnits < 0 {
		e.CachedInputUnits = 0
	}
	if e.CachedInputUnits > e.InputUnits {
		e.CachedInputUnits = e.InputUnits
	}
	if e.TotalUnits <= 0 {
		e.TotalUnits = e.InputUnits + e.OutputUnits
	}
	if e.CompletedAt.IsZero() {
		e.CompletedAt = time.Now()
	}
	if e.StartedAt.IsZero() {
		e.StartedAt = e.CompletedAt
	}
	if e.DurationMillis == 0 && !e.CompletedAt.IsZero() && !e.StartedAt.IsZero() {
		e.DurationMillis = e.CompletedAt.Sub(e.StartedAt).Milliseconds()
	}
}

type BillableCharge struct {
	ID                         uint      `json:"id" gorm:"primaryKey"`
	UsageEventID               uint      `json:"usage_event_id" gorm:"uniqueIndex;not null"`
	AccountID                  uint      `json:"account_id" gorm:"not null;index"`
	APIKeyID                   uint      `json:"api_key_id" gorm:"not null;index"`
	PriceConfigID              *uint     `json:"price_config_id" gorm:"index"`
	Currency                   string    `json:"currency" gorm:"size:16;not null"`
	UnitPriceMicros            int64     `json:"unit_price_micros"`
	InputUnitPriceMicros       int64     `json:"input_unit_price_micros"`
	OutputUnitPriceMicros      int64     `json:"output_unit_price_micros"`
	CachedInputUnitPriceMicros int64     `json:"cached_input_unit_price_micros"`
	BillableUnits              int64     `json:"billable_units"`
	InputUnits                 int64     `json:"input_units"`
	CachedInputUnits           int64     `json:"cached_input_units"`
	OutputUnits                int64     `json:"output_units"`
	AmountMicros               int64     `json:"amount_micros"`
	UsageClass                 string    `json:"usage_class" gorm:"size:64;not null"`
	CacheState                 string    `json:"cache_state" gorm:"size:64;not null"`
	CreatedAt                  time.Time `json:"created_at"`
}

func (BillableCharge) TableName() string {
	return "relay_billable_charges"
}

func (s *Store) CreatePriceConfig(ctx context.Context, price *PriceConfig) error {
	return s.db.WithContext(ctx).Create(price).Error
}

func (s *Store) CreateUsageEvent(ctx context.Context, event *UsageEvent) (*UsageEvent, error) {
	if event == nil {
		return nil, errors.New("usage event is required")
	}
	event.applyDefaults()
	if event.EventID == "" {
		return nil, errors.New("usage event id is required")
	}
	err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(event).Error
	if err != nil {
		return nil, err
	}
	var persisted UsageEvent
	if err := s.db.WithContext(ctx).First(&persisted, "event_id = ?", event.EventID).Error; err != nil {
		return nil, err
	}
	return &persisted, nil
}

func (s *Store) CreateBillableChargeForEvent(ctx context.Context, event *UsageEvent) (*BillableCharge, error) {
	if event == nil {
		return nil, errors.New("usage event is required")
	}
	price, err := s.ResolvePrice(ctx, PriceResolutionInput{
		AccountID:         event.AccountID,
		PublicModelID:     event.PublicModelID,
		ProviderChannelID: event.ProviderChannelID,
		UsageClass:        event.UsageClass,
		CacheState:        event.CacheState,
		At:                event.CompletedAt,
	})
	if err != nil {
		return nil, err
	}
	charge := BillableCharge{
		UsageEventID:     event.ID,
		AccountID:        event.AccountID,
		APIKeyID:         event.APIKeyID,
		BillableUnits:    event.BillableUnits,
		InputUnits:       event.InputUnits,
		CachedInputUnits: event.CachedInputUnits,
		OutputUnits:      event.OutputUnits,
		UsageClass:       event.UsageClass,
		CacheState:       event.CacheState,
		Currency:         "USD",
	}
	if price != nil {
		charge.PriceConfigID = &price.ID
		charge.Currency = price.Currency
		charge.UnitPriceMicros = price.UnitPriceMicros
		charge.InputUnitPriceMicros = price.InputUnitPriceMicros
		charge.OutputUnitPriceMicros = price.OutputUnitPriceMicros
		charge.CachedInputUnitPriceMicros = price.CachedInputUnitPriceMicros
		charge.AmountMicros = price.AmountMicros(event)
	}
	err = s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&charge).Error
	if err != nil {
		return nil, err
	}
	var persisted BillableCharge
	if err := s.db.WithContext(ctx).First(&persisted, "usage_event_id = ?", event.ID).Error; err != nil {
		return nil, err
	}
	return &persisted, nil
}

type PriceResolutionInput struct {
	AccountID         uint
	PublicModelID     string
	ProviderChannelID *uint
	UsageClass        string
	CacheState        string
	At                time.Time
}

func (s *Store) ResolvePrice(ctx context.Context, input PriceResolutionInput) (*PriceConfig, error) {
	if input.At.IsZero() {
		input.At = time.Now()
	}
	input.PublicModelID = strings.TrimSpace(input.PublicModelID)
	input.UsageClass = strings.TrimSpace(input.UsageClass)
	if input.UsageClass == "" {
		input.UsageClass = UsageClassRequest
	}
	input.CacheState = strings.TrimSpace(input.CacheState)
	if input.CacheState == "" {
		input.CacheState = CacheStateUnknown
	}

	var prices []PriceConfig
	err := s.db.WithContext(ctx).
		Where("effective_at <= ?", input.At).
		Order("effective_at desc").
		Order("id desc").
		Find(&prices).Error
	if err != nil {
		return nil, err
	}

	var selected *PriceConfig
	selectedScore := -1
	for i := range prices {
		price := prices[i]
		score, ok := price.matches(input)
		if !ok {
			continue
		}
		if score > selectedScore {
			selected = &prices[i]
			selectedScore = score
		}
	}
	return selected, nil
}

func (p PriceConfig) matches(input PriceResolutionInput) (int, bool) {
	score := 0
	if p.AccountID != nil {
		if input.AccountID == 0 || *p.AccountID != input.AccountID {
			return 0, false
		}
		score += 8
	}
	if p.ProviderChannelID != nil {
		if input.ProviderChannelID == nil || *p.ProviderChannelID != *input.ProviderChannelID {
			return 0, false
		}
		score += 4
	}
	if p.PublicModelID != "" {
		if p.PublicModelID != input.PublicModelID {
			return 0, false
		}
		score += 2
	}
	if p.UsageClass != "" {
		if p.UsageClass != input.UsageClass {
			return 0, false
		}
		score++
	}
	if p.CacheState != "" && p.CacheState != CacheStateUnknown {
		if p.CacheState != input.CacheState {
			return 0, false
		}
		score++
	}
	return score, true
}

func (s *Store) MigratePlatform(ctx context.Context) error {
	return s.db.WithContext(ctx).AutoMigrate(
		&Account{},
		&APIKey{},
		&ProviderChannel{},
		&PriceConfig{},
		&UsageEvent{},
		&BillableCharge{},
	)
}

func bumpConfigVersions(tx *gorm.DB, names ...string) error {
	for _, name := range names {
		if err := bumpConfigVersion(tx, name); err != nil {
			return err
		}
	}
	return nil
}

func bumpConfigVersion(tx *gorm.DB, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("config version name is required")
	}
	now := time.Now()
	var version ConfigVersion
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&version, "name = ?", name).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&ConfigVersion{
			Name:      name,
			Version:   1,
			UpdatedAt: now,
		}).Error
	}
	if err != nil {
		return err
	}
	return tx.Model(&ConfigVersion{}).
		Where("name = ?", name).
		Updates(map[string]interface{}{
			"version":    gorm.Expr("version + ?", 1),
			"updated_at": now,
		}).Error
}

func (s *Store) configVersion(ctx context.Context, name string) (int64, error) {
	var version ConfigVersion
	if err := s.db.WithContext(ctx).First(&version, "name = ?", name).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if ensureErr := s.ensureConfigVersion(ctx, name); ensureErr != nil {
				return 0, ensureErr
			}
			return 1, nil
		}
		return 0, err
	}
	return version.Version, nil
}

func (s *Store) ensureConfigVersion(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("config version name is required")
	}
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&ConfigVersion{
		Name:    name,
		Version: 1,
	}).Error
}
