package auth

import "errors"

// OAuthProvider is the seam for federated login (Google, GitHub, ...).
// Email/password is the current implemented path; this interface exists so
// handlers can be written against a provider abstraction now and concrete
// providers (in their own subpackage, with their own state) dropped in
// later without touching the auth flow. The artifact of a completed flow is
// an OAuthIdentity — an email the user has proven they control via an
// external IdP — which the service layer maps to a users row.
type OAuthProvider interface {
	// Name is the provider's registry key, e.g. "google".
	Name() string
	// Exchange turns a one-time authorization code (obtained by the client
	// from the IdP) into a verified identity. state is the value the client
	// returned with the code, used for CSRF protection.
	Exchange(code, state string) (OAuthIdentity, error)
}

// OAuthIdentity is a successfully-verified external identity. Subject is the
// stable, provider-specific user id; Email is the verified address.
type OAuthIdentity struct {
	Provider string
	Subject  string
	Email    string
	Name     string
}

// OAuthRegistry holds the configured providers by Name.
type OAuthRegistry struct {
	providers map[string]OAuthProvider
}

func NewOAuthRegistry() *OAuthRegistry {
	return &OAuthRegistry{providers: map[string]OAuthProvider{}}
}

// Register adds p, keyed by p.Name(). A later Register for the same name
// replaces the earlier one (idempotent startup).
func (r *OAuthRegistry) Register(p OAuthProvider) {
	r.providers[p.Name()] = p
}

// Get returns the provider with the given name.
func (r *OAuthRegistry) Get(name string) (OAuthProvider, error) {
	p, ok := r.providers[name]
	if !ok {
		return nil, errors.New("unknown oauth provider: " + name)
	}
	return p, nil
}

// Has reports whether at least one provider is registered — used to decide
// whether to advertise OAuth options in any public endpoint.
func (r *OAuthRegistry) Has() bool {
	return len(r.providers) > 0
}
