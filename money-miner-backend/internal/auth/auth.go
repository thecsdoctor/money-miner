// Package auth validates Keycloak RS256 bearer tokens against the realm
// JWKS endpoint and extracts the tenant (sub) + realm roles.
package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is what handlers get: the tenant id and realm roles.
type Claims struct {
	Sub   string
	Roles []string
}

// HasRole reports whether the claims carry a realm role.
func (c Claims) HasRole(r string) bool {
	for _, x := range c.Roles {
		if x == r {
			return true
		}
	}
	return false
}

// Validator fetches and caches JWKS keys and verifies tokens.
type Validator struct {
	jwksURL string
	issuer  string
	hc      *http.Client

	mu    sync.RWMutex
	keys  map[string]*rsa.PublicKey // kid -> key
	fresh time.Time
}

func NewValidator(jwksURL, issuer string) *Validator {
	return &Validator{
		jwksURL: jwksURL,
		issuer:  issuer,
		keys:    map[string]*rsa.PublicKey{},
		hc:      &http.Client{Timeout: 10 * time.Second},
	}
}

type jwkSet struct {
	Keys []struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		Use string `json:"use"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

// refresh pulls the JWKS document. Called on startup, when a kid is
// unknown, and periodically (1 h) to pick up key rotation.
func (v *Validator) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks fetch: %s", resp.Status)
	}
	var set jwkSet
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return err
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" {
			continue
		}
		nb, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eb, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		var e int
		for _, b := range eb {
			e = e<<8 + int(b)
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e}
	}
	if len(keys) == 0 {
		return errors.New("jwks: no RSA keys")
	}
	v.mu.Lock()
	v.keys = keys
	v.fresh = time.Now()
	v.mu.Unlock()
	return nil
}

// Warm fetches keys at startup so the first request is fast and a broken
// JWKS URL fails loudly in the logs rather than as 401s.
func (v *Validator) Warm(ctx context.Context) error { return v.refresh(ctx) }

type realmAccess struct {
	Roles []string `json:"roles"`
}

type tokenClaims struct {
	jwt.RegisteredClaims
	RealmAccess realmAccess `json:"realm_access"`
}

// Parse validates a bearer token string and returns tenant claims.
func (v *Validator) Parse(ctx context.Context, bearer string) (Claims, error) {
	bearer = strings.TrimSpace(strings.TrimPrefix(bearer, "Bearer "))
	if bearer == "" {
		return Claims{}, errors.New("empty token")
	}
	var claims tokenClaims
	keyFn := func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		v.mu.RLock()
		key, ok := v.keys[kid]
		stale := time.Since(v.fresh) > time.Hour
		v.mu.RUnlock()
		if !ok || stale {
			if err := v.refresh(ctx); err != nil {
				return nil, err
			}
			v.mu.RLock()
			key, ok = v.keys[kid]
			v.mu.RUnlock()
		}
		if !ok {
			return nil, fmt.Errorf("unknown kid")
		}
		return key, nil
	}
	_, err := jwt.ParseWithClaims(bearer, &claims, keyFn,
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(v.issuer),
		jwt.WithExpirationRequired())
	if err != nil {
		return Claims{}, err
	}
	if claims.Subject == "" {
		return Claims{}, errors.New("no sub")
	}
	return Claims{Sub: claims.Subject, Roles: claims.RealmAccess.Roles}, nil
}

type ctxKey struct{}

// Middleware enforces a valid bearer token and stashes Claims in context.
func (v *Validator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			writeUnauthorized(w)
			return
		}
		claims, err := v.Parse(r.Context(), h)
		if err != nil {
			writeUnauthorized(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, claims)))
	})
}

// FromContext returns the request's claims (set by Middleware).
func FromContext(ctx context.Context) Claims {
	c, _ := ctx.Value(ctxKey{}).(Claims)
	return c
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"missing or invalid bearer token"}}`))
}
