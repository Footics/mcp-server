package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("FOOTICS_API_URL", "http://api:8080/")
	t.Setenv("AUTH_JWT_SECRET", "s3cr3t")
	t.Setenv("MCP_PUBLIC_URL", "https://mcp.footics.app/")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.APIBaseURL != "http://api:8080" {
		t.Errorf("APIBaseURL = %q, want trailing slash trimmed", c.APIBaseURL)
	}
	if c.Resource != "https://mcp.footics.app/mcp" {
		t.Errorf("Resource = %q", c.Resource)
	}
	if !c.Enabled || !c.RequireAuth || c.EnableWrites {
		t.Errorf("flags: enabled=%v requireAuth=%v writes=%v, want true/true/false", c.Enabled, c.RequireAuth, c.EnableWrites)
	}
	if c.RateLimitPerMin != DefaultRateLimitPerMin {
		t.Errorf("RateLimitPerMin = %d, want %d", c.RateLimitPerMin, DefaultRateLimitPerMin)
	}
	if c.JWTAud != "authenticated" {
		t.Errorf("JWTAud = %q, want authenticated", c.JWTAud)
	}
	if c.UseJWKS() {
		t.Error("UseJWKS true with only a secret set")
	}
}

func TestLoadRequiresAPIURL(t *testing.T) {
	t.Setenv("AUTH_JWT_SECRET", "s")
	if _, err := Load(); err == nil {
		t.Fatal("want error when FOOTICS_API_URL is unset")
	}
}

func TestLoadAuthModeExclusive(t *testing.T) {
	t.Setenv("FOOTICS_API_URL", "http://api:8080")

	// neither → error
	if _, err := Load(); err == nil {
		t.Fatal("want error when neither JWKS nor secret is set")
	}
	// both → error
	t.Setenv("AUTH_JWKS_URL", "https://x/jwks")
	t.Setenv("AUTH_JWT_SECRET", "s")
	if _, err := Load(); err == nil {
		t.Fatal("want error when both JWKS and secret are set")
	}
}

func TestLoadWritesToggle(t *testing.T) {
	t.Setenv("FOOTICS_API_URL", "http://api:8080")
	t.Setenv("AUTH_JWT_SECRET", "s")
	t.Setenv("MCP_ENABLE_WRITES", "true")
	t.Setenv("MCP_ENABLED", "false")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !c.EnableWrites {
		t.Error("MCP_ENABLE_WRITES=true not honoured")
	}
	if c.Enabled {
		t.Error("MCP_ENABLED=false not honoured")
	}
}
