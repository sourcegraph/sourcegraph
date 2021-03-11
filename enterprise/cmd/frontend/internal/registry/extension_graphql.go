package registry

import (
	"context"
	"strings"
	"time"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	registry "github.com/sourcegraph/sourcegraph/cmd/frontend/registry/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

// extensionDBResolver implements the GraphQL type RegistryExtension.
type extensionDBResolver struct {
	db dbutil.DB
	v  *dbExtension

	// Supplied as part of list endpoints, but
	// calculated as part of single-extension endpoints
	r *dbRelease
}

func (r *extensionDBResolver) ID() graphql.ID {
	return registry.MarshalRegistryExtensionID(registry.RegistryExtensionID{LocalID: r.v.ID})
}

func (r *extensionDBResolver) UUID() string        { return r.v.UUID }
func (r *extensionDBResolver) ExtensionID() string { return r.v.NonCanonicalExtensionID }
func (r *extensionDBResolver) ExtensionIDWithoutRegistry() string {
	if r.v.NonCanonicalRegistry != "" {
		return strings.TrimPrefix(r.v.NonCanonicalExtensionID, r.v.NonCanonicalRegistry+"/")
	}
	return r.v.NonCanonicalExtensionID
}

func (r *extensionDBResolver) Publisher(ctx context.Context) (graphqlbackend.RegistryPublisher, error) {
	return getRegistryPublisher(ctx, r.db, r.v.Publisher)
}

func (r *extensionDBResolver) Name() string { return r.v.Name }
func (r *extensionDBResolver) Manifest(ctx context.Context) (graphqlbackend.ExtensionManifest, error) {
	release, err := r.release(ctx)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return registry.NewExtensionManifest(nil), nil
	}
	return registry.NewExtensionManifest(&release.Manifest), nil
}

func (r *extensionDBResolver) CreatedAt() *graphqlbackend.DateTime {
	return &graphqlbackend.DateTime{Time: r.v.CreatedAt}
}

func (r *extensionDBResolver) UpdatedAt() *graphqlbackend.DateTime {
	return &graphqlbackend.DateTime{Time: r.v.UpdatedAt}
}

func (r *extensionDBResolver) PublishedAt(ctx context.Context) (*graphqlbackend.DateTime, error) {
	release, err := r.release(ctx)
	if err != nil {
		return nil, err
	}
	if release == nil {
		return &graphqlbackend.DateTime{Time: time.Time{}}, nil
	}
	return &graphqlbackend.DateTime{Time: release.CreatedAt}, nil
}

func (r *extensionDBResolver) URL() string {
	return registry.ExtensionURL(r.v.NonCanonicalExtensionID)
}
func (r *extensionDBResolver) RemoteURL() *string { return nil }

func (r *extensionDBResolver) RegistryName() (string, error) {
	return r.v.NonCanonicalRegistry, nil
}

func (r *extensionDBResolver) IsLocal() bool { return true }

func (r *extensionDBResolver) IsWorkInProgress() bool {
	return r.v.NonCanonicalIsWorkInProgress
}

func (r *extensionDBResolver) ViewerCanAdminister(ctx context.Context) (bool, error) {
	err := toRegistryPublisherID(r.v).viewerCanAdminister(ctx, r.db)
	if err == backend.ErrMustBeSiteAdmin || err == backend.ErrNotAnOrgMember || err == backend.ErrNotAuthenticated {
		return false, nil
	}
	if _, ok := err.(*backend.InsufficientAuthorizationError); ok {
		return false, nil
	}
	return err == nil, err
}

func (r *extensionDBResolver) release(ctx context.Context) (*dbRelease, error) {
	if r.r != nil {
		return r.r, nil
	}

	var err error
	r.r, err = getLatestRelease(ctx, r.v.NonCanonicalExtensionID, r.v.ID, "release")
	return r.r, err
}

func strptr(s string) *string { return &s }
