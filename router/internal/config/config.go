package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Addr               string
	InstanceID         string
	SQLDSN             string
	ConfigSyncInterval time.Duration
	RequestTimeout     time.Duration
	MaxRetries         int
	MaxBodyBytes       int64
	APIKey             string
	AdminKey           string
	AccountKeyAuth     bool
	APIKeyHashSecret   string
	AccountServiceKey  string
}

func Load() Config {
	return Config{
		Addr:               getenv("ROUTER_ADDR", ":8080"),
		InstanceID:         getenv("ROUTER_INSTANCE_ID", hostname()),
		SQLDSN:             os.Getenv("SQL_DSN"),
		ConfigSyncInterval: getDuration("CONFIG_SYNC_INTERVAL", 5*time.Second),
		RequestTimeout:     getDuration("UPSTREAM_TIMEOUT", 5*time.Minute),
		MaxRetries:         getInt("UPSTREAM_MAX_RETRIES", 2),
		MaxBodyBytes:       int64(getInt("MAX_BODY_BYTES", 32<<20)),
		APIKey:             os.Getenv("ROUTER_API_KEY"),
		AdminKey:           os.Getenv("ROUTER_ADMIN_KEY"),
		AccountKeyAuth:     getBool("RELAY_ACCOUNT_KEY_AUTH", false),
		APIKeyHashSecret:   os.Getenv("API_KEY_HASH_SECRET"),
		AccountServiceKey:  getenv("ACCOUNT_SERVICE_KEY", os.Getenv("ROUTER_ADMIN_KEY")),
	}
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "local"
	}
	return name
}

func getenv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
