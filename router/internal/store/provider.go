package store

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	StatusEnabled             = 1
	StatusDisabled            = 2
	DefaultGroup              = "default"
	ProviderConfigVersionName = "providers"
)

type StringMap map[string]string

func (m StringMap) Value() (driver.Value, error) {
	if len(m) == 0 {
		return "{}", nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return string(data), nil
}

func (m *StringMap) Scan(value interface{}) error {
	if value == nil {
		*m = StringMap{}
		return nil
	}
	var data []byte
	switch typed := value.(type) {
	case []byte:
		data = typed
	case string:
		data = []byte(typed)
	default:
		return fmt.Errorf("unsupported StringMap value %T", value)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		*m = StringMap{}
		return nil
	}
	return json.Unmarshal(data, m)
}

type Provider struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Name         string    `json:"name" gorm:"size:128;index"`
	Provider     string    `json:"provider" gorm:"size:64;not null;index"`
	BaseURL      string    `json:"base_url" gorm:"type:text"`
	APIKey       string    `json:"api_key,omitempty" gorm:"column:api_key;type:text"`
	AuthHeader   string    `json:"auth_header" gorm:"size:64"`
	AuthPrefix   string    `json:"auth_prefix" gorm:"size:64"`
	Models       string    `json:"models" gorm:"type:text;not null"`
	Groups       string    `json:"groups" gorm:"type:text;not null;default:'default'"`
	ModelMapping StringMap `json:"model_mapping" gorm:"type:text"`
	Headers      StringMap `json:"headers" gorm:"type:text"`
	Status       int       `json:"status" gorm:"default:1;index"`
	Weight       uint      `json:"weight" gorm:"default:0"`
	Priority     int64     `json:"priority" gorm:"default:0;index"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ConfigVersion struct {
	Name      string    `json:"name" gorm:"primaryKey;size:64"`
	Version   int64     `json:"version" gorm:"not null;default:1"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (ConfigVersion) TableName() string {
	return "router_config_versions"
}

func (Provider) TableName() string {
	return "router_providers"
}

func (p *Provider) BeforeCreate(*gorm.DB) error {
	p.applyDefaults()
	return nil
}

func (p *Provider) BeforeSave(*gorm.DB) error {
	p.applyDefaults()
	return nil
}

func (p *Provider) applyDefaults() {
	p.Provider = strings.TrimSpace(p.Provider)
	p.Name = strings.TrimSpace(p.Name)
	p.BaseURL = strings.TrimSpace(p.BaseURL)
	p.Models = strings.Trim(strings.TrimSpace(p.Models), ",")
	p.Groups = strings.Trim(strings.TrimSpace(p.Groups), ",")
	if p.Groups == "" {
		p.Groups = DefaultGroup
	}
	if p.Status == 0 {
		p.Status = StatusEnabled
	}
	if p.ModelMapping == nil {
		p.ModelMapping = StringMap{}
	}
	if p.Headers == nil {
		p.Headers = StringMap{}
	}
}

func (p Provider) Public() PublicProvider {
	return PublicProvider{
		ID:           p.ID,
		Name:         p.Name,
		Provider:     p.Provider,
		BaseURL:      p.BaseURL,
		AuthHeader:   p.AuthHeader,
		AuthPrefix:   p.AuthPrefix,
		Models:       p.Models,
		Groups:       p.Groups,
		ModelMapping: p.ModelMapping,
		Headers:      p.Headers,
		Status:       p.Status,
		Weight:       p.Weight,
		Priority:     p.Priority,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

type PublicProvider struct {
	ID           uint      `json:"id"`
	Name         string    `json:"name"`
	Provider     string    `json:"provider"`
	BaseURL      string    `json:"base_url"`
	AuthHeader   string    `json:"auth_header"`
	AuthPrefix   string    `json:"auth_prefix"`
	Models       string    `json:"models"`
	Groups       string    `json:"groups"`
	ModelMapping StringMap `json:"model_mapping"`
	Headers      StringMap `json:"headers"`
	Status       int       `json:"status"`
	Weight       uint      `json:"weight"`
	Priority     int64     `json:"priority"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (p Provider) GroupList() []string {
	return splitCSV(p.Groups, DefaultGroup)
}

func (p Provider) ModelList() []string {
	return splitCSV(p.Models, "")
}

func (p Provider) UpstreamModel(model string) string {
	if mapped, ok := p.ModelMapping[model]; ok && strings.TrimSpace(mapped) != "" {
		return mapped
	}
	return model
}

func (p Provider) PickAPIKey() string {
	keys := splitKeys(p.APIKey)
	if len(keys) == 0 {
		return ""
	}
	return keys[rand.Intn(len(keys))]
}

func (p Provider) AuthHeaderOrDefault() string {
	if strings.TrimSpace(p.AuthHeader) != "" {
		return strings.TrimSpace(p.AuthHeader)
	}
	return "Authorization"
}

func (p Provider) AuthPrefixOrDefault() string {
	if p.AuthPrefix != "" {
		return p.AuthPrefix
	}
	if strings.EqualFold(p.AuthHeaderOrDefault(), "Authorization") {
		return "Bearer "
	}
	return ""
}

func splitCSV(value string, fallback string) []string {
	value = strings.Trim(strings.TrimSpace(value), ",")
	if value == "" {
		if fallback == "" {
			return nil
		}
		return []string{fallback}
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func splitKeys(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "[") {
		var keys []string
		if err := json.Unmarshal([]byte(value), &keys); err == nil {
			return compactStrings(keys)
		}
	}
	return compactStrings(strings.Split(strings.Trim(value, "\n"), "\n"))
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

type Store struct {
	db *gorm.DB
}

func Open(dsn string) (*Store, error) {
	db, err := openGorm(dsn)
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(&Provider{}, &ConfigVersion{}); err != nil {
		return nil, err
	}
	store := &Store{db: db}
	if err := store.ensureProviderVersion(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func openGorm(dsn string) (*gorm.DB, error) {
	dsn = strings.TrimSpace(dsn)
	switch {
	case dsn == "":
		return gorm.Open(sqlite.Open("router.db?_busy_timeout=30000"), &gorm.Config{})
	case dsn == "local":
		return gorm.Open(sqlite.Open("router.db?_busy_timeout=30000"), &gorm.Config{})
	case strings.HasPrefix(dsn, "sqlite://"):
		return gorm.Open(sqlite.Open(strings.TrimPrefix(dsn, "sqlite://")), &gorm.Config{})
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		return gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), &gorm.Config{})
	default:
		return gorm.Open(mysql.Open(dsn), &gorm.Config{})
	}
}

func (s *Store) DB() *gorm.DB {
	return s.db
}

func (s *Store) Providers(ctx context.Context) ([]Provider, error) {
	var providers []Provider
	err := s.db.WithContext(ctx).Order("priority desc").Order("id asc").Find(&providers).Error
	return providers, err
}

func (s *Store) CreateProvider(ctx context.Context, provider *Provider) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(provider).Error; err != nil {
			return err
		}
		return bumpProviderVersion(tx)
	})
}

func (s *Store) Provider(ctx context.Context, id uint) (*Provider, error) {
	var provider Provider
	if err := s.db.WithContext(ctx).First(&provider, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

func (s *Store) SaveProvider(ctx context.Context, provider *Provider) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(provider).Error; err != nil {
			return err
		}
		return bumpProviderVersion(tx)
	})
}

func (s *Store) DeleteProvider(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&Provider{}, "id = ?", id).Error; err != nil {
			return err
		}
		return bumpProviderVersion(tx)
	})
}

func (s *Store) ProviderVersion(ctx context.Context) (int64, error) {
	var version ConfigVersion
	if err := s.db.WithContext(ctx).First(&version, "name = ?", ProviderConfigVersionName).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if ensureErr := s.ensureProviderVersion(ctx); ensureErr != nil {
				return 0, ensureErr
			}
			return 1, nil
		}
		return 0, err
	}
	return version.Version, nil
}

func (s *Store) ensureProviderVersion(ctx context.Context) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&ConfigVersion{
		Name:    ProviderConfigVersionName,
		Version: 1,
	}).Error
}

func bumpProviderVersion(tx *gorm.DB) error {
	now := time.Now()
	var version ConfigVersion
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&version, "name = ?", ProviderConfigVersionName).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&ConfigVersion{
			Name:      ProviderConfigVersionName,
			Version:   1,
			UpdatedAt: now,
		}).Error
	}
	if err != nil {
		return err
	}
	return tx.Model(&ConfigVersion{}).
		Where("name = ?", ProviderConfigVersionName).
		Updates(map[string]interface{}{
			"version":    gorm.Expr("version + ?", 1),
			"updated_at": now,
		}).Error
}

type Cache struct {
	mu           sync.RWMutex
	version      int64
	lastSyncedAt time.Time
	byID         map[uint]Provider
	byModel      map[string]map[string][]uint
}

func NewCache() *Cache {
	return &Cache{
		byID:    make(map[uint]Provider),
		byModel: make(map[string]map[string][]uint),
	}
}

type CacheStats struct {
	Version              int64     `json:"version"`
	LastSyncedAt         time.Time `json:"last_synced_at"`
	ProviderCount        int       `json:"provider_count"`
	EnabledProviderCount int       `json:"enabled_provider_count"`
	GroupCount           int       `json:"group_count"`
	ModelCount           int       `json:"model_count"`
}

func ReloadCache(ctx context.Context, db *Store, cache *Cache) (int64, error) {
	version, err := db.ProviderVersion(ctx)
	if err != nil {
		return 0, err
	}
	providers, err := db.Providers(ctx)
	if err != nil {
		return 0, err
	}
	cache.Replace(providers, version)
	return version, nil
}

func SyncCache(ctx context.Context, db *Store, cache *Cache, interval time.Duration, logger *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dbVersion, err := db.ProviderVersion(ctx)
			if err != nil {
				logger.Error("read provider config version", "error", err)
				continue
			}
			if cache.Version() == dbVersion {
				continue
			}
			loadedVersion, err := ReloadCache(ctx, db, cache)
			if err != nil {
				logger.Error("sync providers from database", "error", err, "db_version", dbVersion)
				continue
			}
			logger.Info("providers synced from database", "version", loadedVersion)
		}
	}
}

func (c *Cache) Replace(providers []Provider, version int64) {
	byID := make(map[uint]Provider, len(providers))
	byModel := make(map[string]map[string][]uint)

	for _, provider := range providers {
		provider.applyDefaults()
		byID[provider.ID] = provider
		if provider.Status != StatusEnabled {
			continue
		}
		for _, group := range provider.GroupList() {
			if byModel[group] == nil {
				byModel[group] = make(map[string][]uint)
			}
			for _, model := range provider.ModelList() {
				byModel[group][model] = append(byModel[group][model], provider.ID)
			}
		}
	}

	for group := range byModel {
		for model := range byModel[group] {
			ids := byModel[group][model]
			sort.SliceStable(ids, func(i, j int) bool {
				left := byID[ids[i]]
				right := byID[ids[j]]
				if left.Priority == right.Priority {
					return left.ID < right.ID
				}
				return left.Priority > right.Priority
			})
		}
	}

	c.mu.Lock()
	c.version = version
	c.lastSyncedAt = time.Now()
	c.byID = byID
	c.byModel = byModel
	c.mu.Unlock()
}

func (c *Cache) Version() int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.version
}

func (c *Cache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	enabled := 0
	modelCount := 0
	for _, provider := range c.byID {
		if provider.Status == StatusEnabled {
			enabled++
		}
	}
	for _, models := range c.byModel {
		modelCount += len(models)
	}
	return CacheStats{
		Version:              c.version,
		LastSyncedAt:         c.lastSyncedAt,
		ProviderCount:        len(c.byID),
		EnabledProviderCount: enabled,
		GroupCount:           len(c.byModel),
		ModelCount:           modelCount,
	}
}

func (c *Cache) Select(group string, model string, excluded map[uint]bool) (*Provider, error) {
	group = strings.TrimSpace(group)
	if group == "" {
		group = DefaultGroup
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return nil, errors.New("model is required")
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := c.idsForLocked(group, model)
	if len(ids) == 0 && group != DefaultGroup {
		ids = c.idsForLocked(DefaultGroup, model)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("no provider for group %q and model %q", group, model)
	}

	candidates := make([]Provider, 0, len(ids))
	for _, id := range ids {
		if excluded != nil && excluded[id] {
			continue
		}
		provider, ok := c.byID[id]
		if ok && provider.Status == StatusEnabled {
			candidates = append(candidates, provider)
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no remaining provider for group %q and model %q", group, model)
	}

	topPriority := candidates[0].Priority
	samePriority := candidates[:0]
	for _, candidate := range candidates {
		if candidate.Priority == topPriority {
			samePriority = append(samePriority, candidate)
		}
	}

	selected := pickWeighted(samePriority)
	return &selected, nil
}

func (c *Cache) idsForLocked(group string, model string) []uint {
	models := c.byModel[group]
	if len(models) == 0 {
		return nil
	}
	if ids := models[model]; len(ids) > 0 {
		return ids
	}
	return models["*"]
}

func pickWeighted(providers []Provider) Provider {
	if len(providers) == 1 {
		return providers[0]
	}
	total := 0
	for _, provider := range providers {
		total += int(provider.Weight)
	}
	if total == 0 {
		return providers[rand.Intn(len(providers))]
	}
	n := rand.Intn(total)
	for _, provider := range providers {
		n -= int(provider.Weight)
		if n < 0 {
			return provider
		}
	}
	return providers[len(providers)-1]
}
