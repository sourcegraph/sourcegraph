// Package authz contains common logic and interfaces for authorization to
// external providers (such as GitLab).
package authz

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// ExternalUserPermissions is a collection of accessible repository/project IDs
// (on code host). It contains exact IDs, as well as prefixes to both include
// and exclude IDs.
//
// 🚨 SECURITY: Every call site should evaluate all fields of this struct to
// have a complete set of IDs.
type ExternalUserPermissions struct {
	Exacts          []extsvc.RepoID
	IncludePrefixes []extsvc.RepoID
	ExcludePrefixes []extsvc.RepoID
}

// Provider defines a source of truth of which repositories a user is authorized to view. The
// user is identified by an extsvc.Account instance. Examples of authz providers include the
// following:
//
// * Code host
// * LDAP groups
// * SAML identity provider (via SAML group permissions)
//
// In most cases, an authz provider represents a code host, because it is the source of truth for
// repository permissions.
type Provider interface {
	// FetchAccount returns the external account that identifies the user to this authz provider,
	// taking as input the current list of external accounts associated with the
	// user. Implementations should always recompute the returned account (rather than returning an
	// element of `current` if it has the correct ServiceID and ServiceType).
	//
	// Implementations should use only the `user` and `current` parameters to compute the returned
	// external account. Specifically, they should not try to get the currently authenticated user
	// from the context parameter.
	//
	// The `user` argument should always be non-nil. If no external account can be computed for the
	// provided user, implementations should return nil, nil.
	FetchAccount(ctx context.Context, user *types.User, current []*extsvc.Account) (mine *extsvc.Account, err error)

	// FetchUserPerms returns a collection of accessible repository/project IDs (on
	// code host) that the given account has read access on the code host. The
	// repository/project ID should be the same value as it would be used as or
	// prefix of api.ExternalRepoSpec.ID. The returned set should only include
	// private repositories/project IDs.
	//
	// Because permissions fetching APIs are often expensive, the implementation should
	// try to return partial but valid results in case of error, and it is up to callers
	// to decide whether to discard.
	FetchUserPerms(ctx context.Context, account *extsvc.Account) (*ExternalUserPermissions, error)

	// FetchRepoPerms returns a list of user IDs (on code host) who have read access to
	// the given repository/project on the code host. The user ID should be the same value
	// as it would be used as extsvc.Account.AccountID. The returned list should
	// include both direct access and inherited from the group/organization/team membership.
	//
	// Because permissions fetching APIs are often expensive, the implementation should
	// try to return partial but valid results in case of error, and it is up to callers
	// to decide whether to discard.
	FetchRepoPerms(ctx context.Context, repo *extsvc.Repository) ([]extsvc.AccountID, error)

	// ServiceType returns the service type (e.g., "gitlab") of this authz provider.
	ServiceType() string

	// ServiceID returns the service ID (e.g., "https://gitlab.mycompany.com/") of this authz
	// provider.
	ServiceID() string

	// URN returns the unique resource identifier of external service where the authz provider
	// is defined.
	URN() string

	// Validate checks the configuration and credentials of the authz provider and returns any
	// problems.
	Validate() (problems []string)
}
