package gitlaboauth

import (
	"context"
	"fmt"

	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
)

// unexported key type prevents collisions
type key int

const userKey key = iota

// WithUser returns a copy of ctx that stores the GitLab User.
func WithUser(ctx context.Context, user *gitlab.User) context.Context {
	return context.WithValue(ctx, userKey, user)
}

// UserFromContext returns the GitLab User from the ctx.
func UserFromContext(ctx context.Context) (*gitlab.User, error) {
	user, ok := ctx.Value(userKey).(*gitlab.User)
	if !ok {
		return nil, fmt.Errorf("gitlab: Context missing GitLab User")
	}
	return user, nil
}
