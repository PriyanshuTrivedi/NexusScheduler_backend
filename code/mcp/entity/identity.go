package entity

import (
	"context"
	"strings"
)

type Identity struct {
	UserID string
	Role   string
}

type identityContextKey struct{}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityContextKey{}, identity)
}

func IdentityFromContext(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityContextKey{}).(Identity)
	if !ok {
		return Identity{}, false
	}

	identity.UserID = strings.TrimSpace(identity.UserID)
	identity.Role = strings.ToLower(strings.TrimSpace(identity.Role))

	return identity, identity.UserID != "" &&
		(identity.Role == "client" || identity.Role == "resource")
}
