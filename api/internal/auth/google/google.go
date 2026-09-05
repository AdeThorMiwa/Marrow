package google

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"marrow/internal/auth"

	"github.com/golang-jwt/jwt/v5"
)

const (
	jwksURI    = "https://www.googleapis.com/oauth2/v3/certs"
	issuerProd = "https://accounts.google.com"
	issuerDev  = "accounts.google.com" // some tokens omit the scheme
)

// jwks is the Google-returned key-set shape (typically two RSA keys).
type jwks struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// keyset caches Google's RSA public keys fetched from the JWKS endpoint,
// with a short TTL to handle key rotation without hammering Google's
// endpoint on every login.
type keyset struct {
	mu      sync.RWMutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

const keysetTTL = 1 * time.Hour

var (
	defaultHTTP = &http.Client{Timeout: 5 * time.Second}
	globalKeys  = &keyset{keys: map[string]*rsa.PublicKey{}}
)

// fetchKeyset retrieves Google's JWKS and caches the result for keysetTTL.
func fetchKeyset(ctx context.Context) (map[string]*rsa.PublicKey, error) {
	globalKeys.mu.RLock()
	if len(globalKeys.keys) > 0 && time.Since(globalKeys.fetched) < keysetTTL {
		defer globalKeys.mu.RUnlock()
		return globalKeys.keys, nil
	}
	globalKeys.mu.RUnlock()

	globalKeys.mu.Lock()
	defer globalKeys.mu.Unlock()

	// Double-check after acquiring write lock.
	if len(globalKeys.keys) > 0 && time.Since(globalKeys.fetched) < keysetTTL {
		return globalKeys.keys, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
	if err != nil {
		return nil, fmt.Errorf("google: jwks request: %w", err)
	}
	resp, err := defaultHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google: jwks fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google: jwks: status %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("google: jwks read: %w", err)
	}

	var keys jwks
	if err := json.Unmarshal(raw, &keys); err != nil {
		return nil, fmt.Errorf("google: jwks parse: %w", err)
	}
	if len(keys.Keys) == 0 {
		return nil, errors.New("google: jwks returned no keys")
	}

	parsed := make(map[string]*rsa.PublicKey, len(keys.Keys))
	for _, k := range keys.Keys {
		pub, err := k.toPublicKey()
		if err != nil {
			continue
		}
		parsed[k.Kid] = pub
	}
	if len(parsed) == 0 {
		return nil, errors.New("google: jwks: no parseable keys")
	}

	globalKeys.keys = parsed
	globalKeys.fetched = time.Now()
	return parsed, nil
}

// toPublicKey converts a JWK RSA key to a Go *rsa.PublicKey by decoding
// the base64url-encoded N and E fields into big.Int / int respectively.
func (k jwk) toPublicKey() (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("bad n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("bad e: %w", err)
	}
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
}

// GoogleClaims extends RegisteredClaims with Google-specific id_token fields.
// Implements ClaimsValidator to accept both Google issuer variants in a
// single validation pass — WithIssuer only accepts a single value.
type GoogleClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	jwt.RegisteredClaims
}

// Validate implements ClaimsValidator. It requires iss to be one of the
// known Google issuer strings and aud to contain this client's ID.
func (c *GoogleClaims) Validate() error {
	iss, _ := c.GetIssuer()
	if iss != issuerProd && iss != issuerDev {
		return fmt.Errorf("google: unexpected issuer %q", iss)
	}
	return nil
}

// Provider implements auth.OAuthProvider for Google Sign-In. The client
// sends a Google id_token (obtained via Expo / Google Sign-In on the
// client side); Exchange verifies it via Google's JWKS and returns the
// verified identity.
type Provider struct {
	clientID string
}

// NewProvider builds a Google provider. clientID is the OAuth 2.0 Web
// application client ID from Google Cloud Console — it must match the
// "aud" claim in the id_token exactly.
func NewProvider(clientID string) *Provider {
	return &Provider{clientID: clientID}
}

func (p *Provider) Name() string { return "google" }

// Exchange verifies a Google id_token (passed in the code parameter for
// interface compatibility — see auth.OAuthProvider doc). The token is
// verified against Google's JWKS: signature, issuer, audience, and expiry.
func (p *Provider) Exchange(code, _ string) (auth.OAuthIdentity, error) {
	if code == "" {
		return auth.OAuthIdentity{}, errors.New("google: empty id_token")
	}

	keys, err := fetchKeyset(context.Background())
	if err != nil {
		return auth.OAuthIdentity{}, err
	}

	tok, err := jwt.ParseWithClaims(code, &GoogleClaims{}, func(t *jwt.Token) (any, error) {
		kid, ok := t.Header["kid"].(string)
		if !ok {
			return nil, errors.New("google: missing kid in token header")
		}
		pub, ok := keys[kid]
		if !ok {
			return nil, fmt.Errorf("google: unknown kid %q", kid)
		}
		return pub, nil
	},
		jwt.WithAudience(p.clientID),
		jwt.WithValidMethods([]string{"RS256"}),
	)
	if err != nil || !tok.Valid {
		return auth.OAuthIdentity{}, fmt.Errorf("google: invalid id_token: %w", err)
	}

	claims, ok := tok.Claims.(*GoogleClaims)
	if !ok {
		return auth.OAuthIdentity{}, errors.New("google: failed to parse claims")
	}
	if claims.Email == "" {
		return auth.OAuthIdentity{}, errors.New("google: id_token contains no email")
	}
	if !claims.EmailVerified {
		return auth.OAuthIdentity{}, errors.New("google: email not verified")
	}
	if claims.Subject == "" {
		return auth.OAuthIdentity{}, errors.New("google: id_token contains no subject")
	}

	return auth.OAuthIdentity{
		Provider: "google",
		Subject:  claims.Subject,
		Email:    claims.Email,
		Name:     claims.Name,
	}, nil
}
