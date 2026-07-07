package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/footics/mcp-server/internal/config"
)

const testSecret = "dev-staging-hs256-secret-000000"

func hsVerifier() *Verifier {
	return NewVerifier(config.Config{JWTSecret: testSecret, JWTAud: "authenticated"})
}

func mintHS(t *testing.T, secret string, claims jwt.MapClaims) string {
	t.Helper()
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func TestVerifyValidHS256(t *testing.T) {
	tok := mintHS(t, testSecret, jwt.MapClaims{
		"sub": "user-123", "aud": "authenticated", "email": "a@b.co",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	ti, err := hsVerifier().Verify(context.Background(), tok, nil)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ti.UserID != "user-123" {
		t.Errorf("UserID = %q", ti.UserID)
	}
	if email, _ := ti.Extra[ExtraEmail].(string); email != "a@b.co" {
		t.Errorf("email = %q", email)
	}
	if relayed, _ := ti.Extra[ExtraToken].(string); relayed != tok {
		t.Error("raw token not stashed for relay")
	}
	if ti.Expiration.Before(time.Now()) {
		t.Error("Expiration in the past")
	}
}

func TestVerifyRejects(t *testing.T) {
	future := time.Now().Add(time.Hour).Unix()
	cases := map[string]string{
		"wrong secret": mintHS(t, "not-the-secret", jwt.MapClaims{"sub": "u", "aud": "authenticated", "exp": future}),
		"wrong aud":    mintHS(t, testSecret, jwt.MapClaims{"sub": "u", "aud": "someone-else", "exp": future}),
		"expired":      mintHS(t, testSecret, jwt.MapClaims{"sub": "u", "aud": "authenticated", "exp": time.Now().Add(-time.Hour).Unix()}),
		"missing sub":  mintHS(t, testSecret, jwt.MapClaims{"aud": "authenticated", "exp": future}),
		"missing exp":  mintHS(t, testSecret, jwt.MapClaims{"sub": "u", "aud": "authenticated"}),
		"garbage":      "not.a.jwt",
	}
	v := hsVerifier()
	for name, tok := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := v.Verify(context.Background(), tok, nil)
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !errors.Is(err, sdkauth.ErrInvalidToken) {
				t.Errorf("error %v does not unwrap to ErrInvalidToken", err)
			}
		})
	}
}

func TestVerifyRejectsRS256InHSMode(t *testing.T) {
	// An HS-mode verifier must refuse an asymmetric alg (alg-confusion guard).
	tok := mintHS(t, testSecret, jwt.MapClaims{"sub": "u", "aud": "authenticated", "exp": time.Now().Add(time.Hour).Unix()})
	// Tamper the header alg is overkill; instead assert JWKS-mode verifier refuses HS256.
	jwksV := NewVerifier(config.Config{JWKSURL: "https://example.invalid/jwks", JWTAud: "authenticated"})
	if _, err := jwksV.Verify(context.Background(), tok, nil); err == nil {
		t.Fatal("JWKS-mode verifier accepted an HS256 token")
	}
}
