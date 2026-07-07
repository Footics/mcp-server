// Package config loads runtime configuration from the environment, failing
// closed with a clear error when a required value is missing or invalid.
//
// The Go MCP server is a thin client of footics-api /v1: it verifies the
// Supabase JWT locally (double-mode JWKS/HS256, same logic as footics-api
// internal/auth) and relays the caller's Bearer to the API. It has NO database
// access — there is deliberately no DATABASE_URL here.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DefaultRateLimitPerMin is the per-user tool-call ceiling (calls/minute).
const DefaultRateLimitPerMin = 30

// Config holds all runtime configuration, built once at boot by Load.
type Config struct {
	HTTPAddr string // HTTP_ADDR (default ":8080")

	// APIBaseURL is footics-api's base URL (FOOTICS_API_URL), e.g.
	// http://api:8080 in the compose network. The client appends /v1/... paths
	// and relays the caller's Bearer. Required.
	APIBaseURL string

	// PublicURL is this server's public origin (MCP_PUBLIC_URL). Resource is
	// PublicURL + "/mcp" — the OAuth protected-resource identifier.
	PublicURL string
	Resource  string

	// User-auth mode, driven by which of these is set (exactly one), mirroring
	// footics-api: JWKSURL selects asymmetric verification (ES256/RS256), JWTSecret
	// selects HS256 (legacy Supabase secret). Aud defaults to "authenticated".
	JWKSURL   string // AUTH_JWKS_URL
	JWTSecret string // AUTH_JWT_SECRET
	JWTAud    string // AUTH_JWT_AUD

	// OAuthIssuer is advertised as authorization_servers[0] in the protected
	// resource metadata (MCP_OAUTH_ISSUER), e.g. https://<ref>.supabase.co/auth/v1.
	// Empty → omitted from the metadata.
	OAuthIssuer string

	Enabled         bool   // MCP_ENABLED (default true) — kill switch (503)
	RequireAuth     bool   // MCP_REQUIRE_AUTH (default true)
	EnableWrites    bool   // MCP_ENABLE_WRITES (default false) — submit_prediction
	RateLimitPerMin int    // MCP_RATE_LIMIT_PER_MIN (default 30; 0 = off)
	TestUserID      string // MCP_TEST_USER_ID — identity fallback when RequireAuth=false
}

// UseJWKS reports whether the asymmetric (JWKS) verification mode is selected.
func (c Config) UseJWKS() bool { return c.JWKSURL != "" }

// Load reads configuration from the environment, applying defaults and returning
// a clear error for any missing-required or invalid value.
func Load() (Config, error) {
	public := strings.TrimRight(getenv("MCP_PUBLIC_URL", "https://mcp.footics.app"), "/")
	c := Config{
		HTTPAddr:     getenv("HTTP_ADDR", ":8080"),
		APIBaseURL:   strings.TrimRight(strings.TrimSpace(os.Getenv("FOOTICS_API_URL")), "/"),
		PublicURL:    public,
		Resource:     public + "/mcp",
		JWKSURL:      strings.TrimSpace(os.Getenv("AUTH_JWKS_URL")),
		JWTSecret:    strings.TrimSpace(os.Getenv("AUTH_JWT_SECRET")),
		JWTAud:       getenv("AUTH_JWT_AUD", "authenticated"),
		OAuthIssuer:  strings.TrimSpace(os.Getenv("MCP_OAUTH_ISSUER")),
		Enabled:      getenvBool("MCP_ENABLED", true),
		RequireAuth:  getenvBool("MCP_REQUIRE_AUTH", true),
		EnableWrites: getenvBool("MCP_ENABLE_WRITES", false),
		TestUserID:   strings.TrimSpace(os.Getenv("MCP_TEST_USER_ID")),
	}

	rl, err := getenvInt("MCP_RATE_LIMIT_PER_MIN", DefaultRateLimitPerMin)
	if err != nil {
		return c, err
	}
	c.RateLimitPerMin = rl

	if c.APIBaseURL == "" {
		return c, fmt.Errorf("FOOTICS_API_URL is required (base URL of footics-api /v1)")
	}

	// Exactly one of AUTH_JWKS_URL / AUTH_JWT_SECRET — they select mutually
	// exclusive verification modes (mirrors footics-api config.Load).
	switch {
	case c.JWKSURL == "" && c.JWTSecret == "":
		return c, fmt.Errorf("exactly one of AUTH_JWKS_URL or AUTH_JWT_SECRET is required, got neither")
	case c.JWKSURL != "" && c.JWTSecret != "":
		return c, fmt.Errorf("AUTH_JWKS_URL and AUTH_JWT_SECRET are mutually exclusive, got both")
	}

	return c, nil
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// getenvBool mirrors the TS parsing: "false" (case-sensitive) disables a
// default-true flag; "true" enables a default-false flag; anything else = default.
func getenvBool(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	if def {
		return v != "false"
	}
	return v == "true"
}

func getenvInt(k string, def int) (int, error) {
	v := os.Getenv(k)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", k, v)
	}
	return n, nil
}
