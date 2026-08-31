package models

import (
	"context"
	"strings"
)

type User struct {
	ID          string
	Email       string
	DisplayName string
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// contextKey is an unexported type so that only this package can set/read
// the authenticated user on a context — another package can't collide with
// our key by using the same string.
type contextKey struct{}

func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, contextKey{}, u)
}
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(contextKey{}).(User)
	return u, ok
}
