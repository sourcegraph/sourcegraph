package registry

import (
	"context"
	"errors"
	"fmt"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	frontendregistry "github.com/sourcegraph/sourcegraph/cmd/frontend/registry/api"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

func init() {
	frontendregistry.ExtensionRegistry.ViewerPublishersFunc = extensionRegistryViewerPublishers
}

func extensionRegistryViewerPublishers(ctx context.Context, db dbutil.DB) ([]graphqlbackend.RegistryPublisher, error) {
	// The feature check here makes it so the any "New extension" form will show an error, so the
	// user finds out before trying to submit the form that the feature is disabled.
	if err := licensing.Check(licensing.FeatureExtensionRegistry); err != nil {
		return nil, err
	}

	var publishers []graphqlbackend.RegistryPublisher
	user, err := graphqlbackend.CurrentUser(ctx, db)
	if err != nil || user == nil {
		return nil, err
	}
	publishers = append(publishers, &registryPublisher{user: user})

	orgs, err := database.GlobalOrgs.GetByUserID(ctx, user.DatabaseID())
	if err != nil {
		return nil, err
	}
	for _, org := range orgs {
		publishers = append(publishers, &registryPublisher{org: graphqlbackend.NewOrg(db, org)})
	}
	return publishers, nil
}

// registryPublisher implements the GraphQL type RegistryPublisher.
type registryPublisher struct {
	user *graphqlbackend.UserResolver
	org  *graphqlbackend.OrgResolver
}

var _ graphqlbackend.RegistryPublisher = &registryPublisher{}

func (r *registryPublisher) ToUser() (*graphqlbackend.UserResolver, bool) {
	return r.user, r.user != nil
}
func (r *registryPublisher) ToOrg() (*graphqlbackend.OrgResolver, bool) { return r.org, r.org != nil }

func (r *registryPublisher) toDBRegistryPublisher() dbPublisher {
	switch {
	case r.user != nil:
		return dbPublisher{UserID: r.user.DatabaseID(), NonCanonicalName: r.user.Username()}
	case r.org != nil:
		return dbPublisher{OrgID: r.org.OrgID(), NonCanonicalName: r.org.Name()}
	default:
		return dbPublisher{}
	}
}

func (r *registryPublisher) RegistryExtensionConnectionURL() (*string, error) {
	p := r.toDBRegistryPublisher()
	url := frontendregistry.PublisherExtensionsURL(p.UserID != 0, p.OrgID != 0, p.NonCanonicalName)
	if url == "" {
		return nil, errRegistryUnknownPublisher
	}
	return &url, nil
}

var errRegistryUnknownPublisher = errors.New("unknown registry extension publisher")

func getRegistryPublisher(ctx context.Context, db dbutil.DB, publisher dbPublisher) (*registryPublisher, error) {
	switch {
	case publisher.UserID != 0:
		user, err := graphqlbackend.UserByIDInt32(ctx, db, publisher.UserID)
		if err != nil {
			return nil, err
		}
		return &registryPublisher{user: user}, nil
	case publisher.OrgID != 0:
		org, err := graphqlbackend.OrgByIDInt32(ctx, db, publisher.OrgID)
		if err != nil {
			return nil, err
		}
		return &registryPublisher{org: org}, nil
	default:
		return nil, errRegistryUnknownPublisher
	}
}

type registryPublisherID struct {
	userID, orgID int32
}

func toRegistryPublisherID(extension *dbExtension) *registryPublisherID {
	return &registryPublisherID{
		userID: extension.Publisher.UserID,
		orgID:  extension.Publisher.OrgID,
	}
}

// unmarshalRegistryPublisherID unmarshals the GraphQL ID into the possible publisher ID
// types.
//
// 🚨 SECURITY
func unmarshalRegistryPublisherID(id graphql.ID) (*registryPublisherID, error) {
	var (
		p   registryPublisherID
		err error
	)
	switch kind := relay.UnmarshalKind(id); kind {
	case "User":
		p.userID, err = graphqlbackend.UnmarshalUserID(id)
	case "Org":
		p.orgID, err = graphqlbackend.UnmarshalOrgID(id)
	default:
		return nil, fmt.Errorf("unknown registry extension publisher type: %q", kind)
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// viewerCanAdminister returns whether the current user is allowed to perform mutations on a
// registry extension with the given publisher.
//
// 🚨 SECURITY
func (p *registryPublisherID) viewerCanAdminister(ctx context.Context, db dbutil.DB) error {
	switch {
	case p.userID != 0:
		// 🚨 SECURITY: Check that the current user is either the publisher or a site admin.
		return backend.CheckSiteAdminOrSameUser(ctx, p.userID)
	case p.orgID != 0:
		// 🚨 SECURITY: Check that the current user is a member of the publisher org.
		return backend.CheckOrgAccess(ctx, db, p.orgID)
	default:
		return errRegistryUnknownPublisher
	}
}
