// Package config loads money-miner server configuration from the environment.
// Every knob has a safe default for local dev; secrets come from .env (never
// committed) or real environment variables in compose.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the immutable runtime configuration of the master server.
type Config struct {
	// Listen is the loopback bind address, e.g. "127.0.0.1:8080".
	Listen string
	// DatabaseURL is the postgres DSN (docker network: host "db").
	DatabaseURL string
	// PublicURL is the external app URL, e.g. https://money-miner.thecsdoctor.com.
	PublicURL string
	// Issuer is the public Keycloak issuer URL of the realm.
	Issuer string
	// JWKSURL is where the server fetches realm public keys (may be the
	// internal container URL — the issuer claim is still validated against
	// Issuer, only key discovery uses this address).
	JWKSURL string
	// AllowSimulated enables the SIMULATED engine (demo/CI only).
	AllowSimulated bool
	// AllowedOrigins gates WS Origin headers when present (browsers).
	AllowedOrigins []string
	// PayoutPollInterval is how often pool APIs are checked for payments.
	PayoutPollInterval time.Duration
	// RetentionInterval is how often metric rollup/retention runs.
	RetentionInterval time.Duration
	// ExchangeKeySalt peppers the pgcrypto passphrase for exchange api keys.
	ExchangeKeySalt string
	// SeedTestData inserts a demo miner/wallet for the test user on boot
	// (SEED_TEST_DATA=true; never in production).
	SeedTestData bool
	// Version is stamped into /v1/healthz (build-time -X main.version).
	Version string
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return def
	}
	return v == "true" || v == "1" || v == "yes"
}

// Load reads configuration from the environment and validates it.
func Load() (Config, error) {
	c := Config{
		Listen:             env("MM_LISTEN", "127.0.0.1:8080"),
		DatabaseURL:        os.Getenv("MM_DATABASE_URL"),
		PublicURL:          strings.TrimSuffix(env("MM_PUBLIC_URL", "http://127.0.0.1:8080"), "/"),
		Issuer:             env("MM_OIDC_ISSUER", "https://auth.thecsdoctor.com/realms/money-miner"),
		JWKSURL:            env("MM_OIDC_JWKS_URL", "http://keycloak:8080/realms/money-miner/protocol/openid-connect/certs"),
		AllowSimulated:     envBool("ALLOW_SIMULATED", false),
		PayoutPollInterval: envDuration("MM_PAYOUT_POLL_INTERVAL", 10*time.Minute),
		RetentionInterval:  envDuration("MM_RETENTION_INTERVAL", time.Hour),
		ExchangeKeySalt:    env("MM_EXCHANGE_KEY_SALT", "money-miner-v0.1"),
		SeedTestData:       envBool("SEED_TEST_DATA", false),
		Version:            env("MM_VERSION", "dev"),
	}
	origins := env("MM_ALLOWED_ORIGINS", c.PublicURL)
	for _, o := range strings.Split(origins, ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.AllowedOrigins = append(c.AllowedOrigins, o)
		}
	}
	if c.DatabaseURL == "" {
		return c, fmt.Errorf("MM_DATABASE_URL is required")
	}
	if c.AllowSimulated {
		// Loud by design: simulated mining must never be silent (dossier 01/07).
		fmt.Fprintln(os.Stderr, "WARNING: ALLOW_SIMULATED=true — the SIMULATED engine is selectable; objects it touches carry engine=simulated and never write payouts")
	}
	return c, nil
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	if n, err := strconv.Atoi(v); err == nil { // bare number = seconds
		return time.Duration(n) * time.Second
	}
	return def
}
