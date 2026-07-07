// Command mcp is the Footics MCP server: a thin client of footics-api /v1.
//
// It verifies the Supabase JWT locally (double-mode JWKS/HS256), serves the RFC
// 9728 protected-resource metadata + WWW-Authenticate challenge so OAuth
// discovery points at Supabase, and exposes the 10 frozen tools over Streamable
// HTTP (stateless). Every read relays the caller's Bearer to footics-api; there
// is no database access here.
package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"

	"github.com/footics/mcp-server/internal/apiclient"
	"github.com/footics/mcp-server/internal/auth"
	"github.com/footics/mcp-server/internal/config"
	"github.com/footics/mcp-server/internal/tools"
)

const version = "0.1.0-go"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	api := apiclient.New(cfg.APIBaseURL)

	server := mcp.NewServer(&mcp.Implementation{Name: "footics-mcp", Version: version}, nil)
	tools.Register(server, tools.Deps{
		API:             api,
		EnableWrites:    cfg.EnableWrites,
		TestUserID:      cfg.TestUserID,
		RateLimitPerMin: cfg.RateLimitPerMin,
	})

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler(cfg, server))
	mux.Handle("/.well-known/oauth-protected-resource", protectedResourceHandler(cfg))
	mux.Handle("/.well-known/oauth-protected-resource/mcp", protectedResourceHandler(cfg))
	mux.HandleFunc("/health", healthHandler(cfg, api))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mode := "hs256"
	if cfg.UseJWKS() {
		mode = "jwks"
	}
	slog.Info("footics-mcp starting",
		"addr", cfg.HTTPAddr, "resource", cfg.Resource, "apiBaseURL", cfg.APIBaseURL,
		"authMode", mode, "requireAuth", cfg.RequireAuth, "enabled", cfg.Enabled,
		"writes", cfg.EnableWrites, "rateLimitPerMin", cfg.RateLimitPerMin,
	)
	// ponytail: bare ListenAndServe — graceful shutdown lands with the container
	// lifecycle at cutover (N4). Nothing here holds state worth draining.
	log.Fatal(http.ListenAndServe(cfg.HTTPAddr, mux))
}

// mcpHandler wraps the Streamable HTTP handler with the kill switch, bearer auth
// (when required), and CORS. Order (outer→inner): CORS → kill switch → auth → MCP.
func mcpHandler(cfg config.Config, server *mcp.Server) http.Handler {
	streamable := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true},
	)

	var h http.Handler = streamable
	if cfg.RequireAuth {
		verifier := auth.NewVerifier(cfg)
		h = sdkauth.RequireBearerToken(verifier.Verify, &sdkauth.RequireBearerTokenOptions{
			ResourceMetadataURL: cfg.PublicURL + "/.well-known/oauth-protected-resource/mcp",
		})(h)
	}
	if !cfg.Enabled {
		h = http.HandlerFunc(serviceOff)
	}
	return withCORS(h)
}

// protectedResourceHandler serves the RFC 9728 metadata (authorization_servers →
// Supabase). Served identically on both well-known paths.
func protectedResourceHandler(cfg config.Config) http.Handler {
	var servers []string
	if cfg.OAuthIssuer != "" {
		servers = []string{cfg.OAuthIssuer}
	}
	return sdkauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:               cfg.Resource,
		AuthorizationServers:   servers,
		BearerMethodsSupported: []string{"header"},
		ScopesSupported:        []string{"openid", "email", "profile"},
		ResourceName:           "Footics MCP",
		ResourceDocumentation:  "https://footics.app",
	})
}

// serviceOff is the MCP_ENABLED=false response: a JSON-RPC error, no auth/API
// traffic (mirrors the TS kill switch).
func serviceOff(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Retry-After", "3600")
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{
		"jsonrpc": "2.0",
		"id":      nil,
		"error": map[string]any{
			"code":    -32000,
			"message": "Footics MCP est temporairement hors service (maintenance). Réessaie plus tard.",
		},
	})
}

// healthHandler reports config + a best-effort footics-api liveness probe. Unlike
// the TS server there is no `db` section — the DB lives behind footics-api now.
func healthHandler(cfg config.Config, api *apiclient.Client) http.HandlerFunc {
	mode := "hs256"
	if cfg.UseJWKS() {
		mode = "jwks"
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !cfg.Enabled {
			writeJSON(w, http.StatusOK, map[string]any{
				"ok": true, "enabled": false, "resource": cfg.Resource,
				"reason": "MCP_ENABLED=false — service coupé volontairement (maintenance).",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":              true,
			"enabled":         true,
			"resource":        cfg.Resource,
			"auth":            map[string]any{"required": cfg.RequireAuth, "mode": mode},
			"writes":          cfg.EnableWrites,
			"rateLimitPerMin": cfg.RateLimitPerMin,
			"api":             map[string]any{"url": cfg.APIBaseURL, "ok": probeAPI(r.Context(), cfg.APIBaseURL)},
		})
	}
}

// probeAPI does a bounded GET of footics-api /healthz.
func probeAPI(ctx context.Context, base string) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/healthz", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// withCORS mirrors the TS CORS contract (browser MCP clients need it) and answers
// the OPTIONS preflight. WWW-Authenticate is exposed so 401 discovery works
// cross-origin.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Mcp-Session-Id, MCP-Protocol-Version, Accept")
		h.Set("Access-Control-Expose-Headers", "Mcp-Session-Id, WWW-Authenticate")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
