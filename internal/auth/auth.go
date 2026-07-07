// Package auth verifies Supabase-issued user JWTs (double-mode: JWKS or HS256)
// and adapts the result to the go-sdk auth.TokenVerifier contract.
//
// The verification logic mirrors footics-api internal/auth (same aud + exp +
// method checks, same JWKS cache with a refresh cooldown). The MCP server uses
// the extracted identity only to (a) answer whoami and key the rate-limiter and
// (b) relay the caller's Bearer to footics-api, which re-verifies as the trust
// boundary. It grants no data access on its own.
package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/footics/mcp-server/internal/config"
)

const (
	jwksTTL             = time.Hour
	jwksRefreshCooldown = 30 * time.Second
	jwksMaxBody         = 1 << 20
)

// ExtraEmail / ExtraToken are the keys under which the verifier stashes the
// caller's email and raw Bearer in TokenInfo.Extra (the latter is relayed to
// footics-api by the tool handlers).
const (
	ExtraEmail = "email"
	ExtraToken = "token"
)

// Verifier verifies Supabase user JWTs. Mode is fixed at boot: JWKS (asymmetric
// ES256/RS256, keys fetched from AUTH_JWKS_URL) or HS256 (AUTH_JWT_SECRET).
type Verifier struct {
	secret []byte
	jwks   *jwksCache
	opts   []jwt.ParserOption
	useHS  bool
}

// NewVerifier builds the verifier from config (exactly one mode selected).
func NewVerifier(cfg config.Config) *Verifier {
	common := []jwt.ParserOption{
		jwt.WithAudience(cfg.JWTAud),
		jwt.WithExpirationRequired(),
	}
	if cfg.UseJWKS() {
		return &Verifier{
			jwks: newJWKSCache(cfg.JWKSURL),
			opts: append(common, jwt.WithValidMethods([]string{"ES256", "RS256"})),
		}
	}
	return &Verifier{
		secret: []byte(cfg.JWTSecret),
		useHS:  true,
		opts:   append(common, jwt.WithValidMethods([]string{"HS256"})),
	}
}

// Verify satisfies the go-sdk auth.TokenVerifier signature. On success it returns
// a TokenInfo carrying the user id, expiration, and (in Extra) the email + the
// raw Bearer for relay. On any failure it returns an error unwrapping to
// sdkauth.ErrInvalidToken so the middleware answers 401 + WWW-Authenticate.
func (v *Verifier) Verify(_ context.Context, token string, _ *http.Request) (*sdkauth.TokenInfo, error) {
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, v.keyfunc, v.opts...)
	if err != nil || !parsed.Valid {
		return nil, fmt.Errorf("%w: %v", sdkauth.ErrInvalidToken, err)
	}
	sub, _ := claims["sub"].(string) // Supabase `sub` = user UUID
	if sub == "" {
		return nil, fmt.Errorf("%w: missing sub", sdkauth.ErrInvalidToken)
	}
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return nil, fmt.Errorf("%w: missing exp", sdkauth.ErrInvalidToken)
	}
	email, _ := claims["email"].(string)
	return &sdkauth.TokenInfo{
		UserID:     sub,
		Expiration: exp.Time,
		Extra:      map[string]any{ExtraEmail: email, ExtraToken: token},
	}, nil
}

// keyfunc supplies the verification key for the token's algorithm, refusing any
// algorithm that does not belong to the configured mode.
func (v *Verifier) keyfunc(t *jwt.Token) (any, error) {
	switch t.Method.(type) {
	case *jwt.SigningMethodHMAC:
		if !v.useHS {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return v.secret, nil
	case *jwt.SigningMethodECDSA, *jwt.SigningMethodRSA:
		if v.useHS {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		kid, _ := t.Header["kid"].(string)
		return v.jwks.key(kid)
	default:
		return nil, jwt.ErrTokenSignatureInvalid
	}
}

// --- JWKS cache (copied from footics-api internal/auth) ---------------------

type jwksCache struct {
	url string
	hc  *http.Client

	mu          sync.Mutex
	keys        map[string]crypto.PublicKey
	expiry      time.Time
	lastRefresh time.Time
}

func newJWKSCache(url string) *jwksCache {
	return &jwksCache{url: url, hc: &http.Client{Timeout: 5 * time.Second}, keys: map[string]crypto.PublicKey{}}
}

func (c *jwksCache) key(kid string) (crypto.PublicKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if k, ok := c.keys[kid]; ok && time.Now().Before(c.expiry) {
		return k, nil
	}
	// Cooldown: an unknown kid inside the window is rejected without a fetch, so a
	// spray of bogus kids cannot turn into one origin request each.
	if time.Since(c.lastRefresh) < jwksRefreshCooldown {
		return nil, fmt.Errorf("unknown key id %q", kid)
	}
	if err := c.refresh(); err != nil {
		return nil, err
	}
	if k, ok := c.keys[kid]; ok {
		return k, nil
	}
	return nil, fmt.Errorf("unknown key id %q", kid)
}

func (c *jwksCache) refresh() error {
	c.lastRefresh = time.Now() // attempts count too: failures must not bypass the cooldown

	req, err := http.NewRequest(http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks: %s returned %d", c.url, resp.StatusCode)
	}
	var set struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, jwksMaxBody)).Decode(&set); err != nil {
		return fmt.Errorf("jwks: decode %s: %w", c.url, err)
	}
	keys := make(map[string]crypto.PublicKey, len(set.Keys))
	for _, k := range set.Keys {
		pub, err := k.publicKey()
		if err != nil {
			continue // skip an unsupported key, keep the usable ones
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return fmt.Errorf("jwks: no usable keys at %s", c.url)
	}
	c.keys = keys
	c.expiry = time.Now().Add(jwksTTL)
	return nil
}

// jwk is one JSON Web Key (subset: EC/P-256 and RSA public keys).
type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Crv string `json:"crv"`
	N   string `json:"n"`
	E   string `json:"e"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (k jwk) publicKey() (crypto.PublicKey, error) {
	switch k.Kty {
	case "EC":
		if k.Crv != "P-256" {
			return nil, fmt.Errorf("unsupported EC curve %q", k.Crv)
		}
		x, err := b64(k.X)
		if err != nil {
			return nil, err
		}
		y, err := b64(k.Y)
		if err != nil {
			return nil, err
		}
		return &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}, nil
	case "RSA":
		n, err := b64(k.N)
		if err != nil {
			return nil, err
		}
		e, err := b64(k.E)
		if err != nil {
			return nil, err
		}
		exp := 0
		for _, b := range e {
			exp = exp<<8 | int(b)
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exp}, nil
	default:
		return nil, fmt.Errorf("unsupported key type %q", k.Kty)
	}
}

func b64(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }
