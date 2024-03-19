package database

import (
	"context"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// GrantPendingPermissionsArgs contains required arguments to grant pending permissions for a user
// by username or verified email address(es) according to the site configuration.
type GrantPendingPermissionsArgs struct {
	// The user ID that will be used to bind pending permissions.
	UserID int32
	// The permission level to be granted.
	Perm authz.Perms
	// The type of permissions to be granted.
	Type authz.PermType
}

// AuthorizedReposArgs contains required arguments to verify if a user is authorized to access some
// or all of the repositories from the candidate list with the given level and type of permissions.
type AuthorizedReposArgs struct {
	// The candidate list of repositories to be verified.
	Repos []*types.Repo
	// The user whose authorization to access the repos is being checked.
	UserID int32
	// The permission level to be verified.
	Perm authz.Perms
	// The type of permissions to be verified.
	Type authz.PermType
}

// RevokeUserPermissionsArgs contains required arguments to revoke user permissions, it includes all
// possible leads to grant or authorize access for a user.
type RevokeUserPermissionsArgs struct {
	// The user ID that will be used to revoke effective permissions.
	UserID int32
	// The list of external accounts related to the user. This is list because a user could have
	// multiple external accounts, including ones from code hosts and/or Sourcegraph authz provider.
	Accounts []*extsvc.Accounts
}

// AuthzStore contains methods for manipulating user permissions.
type AuthzStore interface {
	// GrantPendingPermissions grants pending permissions for a user. It is a no-op in the OSS version.
	GrantPendingPermissions(ctx context.Context, args *GrantPendingPermissionsArgs) error
	// AuthorizedRepos checks if a user is authorized to access repositories in the candidate list.
	// The returned list must be a list of repositories that are authorized to the given user.
	AuthorizedRepos(ctx context.Context, args *AuthorizedReposArgs) ([]*types.Repo, error)
	// RevokeUserPermissions deletes both effective and pending permissions that could be related to a user.
	RevokeUserPermissions(ctx context.Context, args *RevokeUserPermissionsArgs) error
	// Bulk "RevokeUserPermissions" action.
	RevokeUserPermissionsList(ctx context.Context, argsList []*RevokeUserPermissionsArgs) error
}

// AuthzWith instantiates and returns a new AuthzStore using the other store
// handle. This constructor is overridden.
var AuthzWith func(other basestore.ShareableStore) AuthzStore

// NewAuthzStore returns an OSS AuthzStore set with enterprise implementation.
func NewAuthzStore(logger log.Logger, db DB, clock func() time.Time) AuthzStore {
	return &authzStore{
		logger:   logger,
		store:    Perms(logger, db, clock),
		srpStore: db.SubRepoPerms(),
	}
}

func NewAuthzStoreWith(logger log.Logger, other basestore.ShareableStore, clock func() time.Time) AuthzStore {
	return &authzStore{
		logger:   logger,
		store:    PermsWith(logger, other, clock),
		srpStore: SubRepoPermsWith(other),
	}
}

type authzStore struct {
	logger   log.Logger
	store    PermsStore
	srpStore SubRepoPermsStore
}

// GrantPendingPermissions grants pending permissions for a user, which implements the AuthzStore interface.
// It uses provided arguments to retrieve information directly from the database to offload security concerns
// from the caller.
//
// It's possible that there are more than one verified emails and external accounts associated to the user
// and all of them have pending permissions, we can safely grant all of them whenever possible because permissions
// are unioned.
func (s *authzStore) GrantPendingPermissions(ctx context.Context, args *GrantPendingPermissionsArgs) (err error) {
	if args.UserID <= 0 {
		return nil
	}

	// Gather external accounts associated to the user.
	extAccounts, err := ExternalAccountsWith(s.logger, s.store).List(ctx,
		ExternalAccountsListOptions{
			UserID:         args.UserID,
			ExcludeExpired: true,
		},
	)
	if err != nil {
		return errors.Wrap(err, "list external accounts")
	}

	// A list of permissions to be granted, by username, email and/or external accounts.
	// Plus one because we'll have at least one more username or verified email address.
	perms := make([]*authz.UserGrantPermissions, 0, len(extAccounts)+1)
	for _, acct := range extAccounts {
		perms = append(perms, &authz.UserGrantPermissions{
			UserID:                args.UserID,
			UserExternalAccountID: acct.ID,
			ServiceType:           acct.ServiceType,
			ServiceID:             acct.ServiceID,
			AccountID:             acct.AccountID,
		})
	}

	// Gather username or verified email based on site configuration.
	cfg := conf.PermissionsUserMapping()
	switch cfg.BindID {
	case "email":
		// 🚨 SECURITY: It is critical to ensure only grant emails that are verified.
		emails, err := UserEmailsWith(s.store).ListByUser(ctx, UserEmailsListOptions{
			UserID:       args.UserID,
			OnlyVerified: true,
		})
		if err != nil {
			return errors.Wrap(err, "list verified emails")
		}
		for i := range emails {
			perms = append(perms, &authz.UserGrantPermissions{
				UserID:      args.UserID,
				ServiceType: authz.SourcegraphServiceType,
				ServiceID:   authz.SourcegraphServiceID,
				AccountID:   emails[i].Email,
			})
		}

	case "username":
		user, err := UsersWith(s.logger, s.store).GetByID(ctx, args.UserID)
		if err != nil {
			return errors.Wrap(err, "get user")
		}
		perms = append(perms, &authz.UserGrantPermissions{
			UserID:      args.UserID,
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			AccountID:   user.Username,
		})

	default:
		return errors.Errorf("unrecognized user mapping bind ID type %q", cfg.BindID)
	}

	txs, err := s.store.Transact(ctx)
	if err != nil {
		return errors.Wrap(err, "start transaction")
	}
	defer func() { err = txs.Done(err) }()

	for _, p := range perms {
		err = txs.GrantPendingPermissions(ctx, p)
		if err != nil {
			return errors.Wrap(err, "grant pending permissions")
		}
	}

	return nil
}

// AuthorizedRepos checks if a user is authorized to access repositories in the candidate list,
// which implements the AuthzStore interface.
func (s *authzStore) AuthorizedRepos(ctx context.Context, args *AuthorizedReposArgs) ([]*types.Repo, error) {
	if len(args.Repos) == 0 {
		return args.Repos, nil
	}

	p, err := s.store.LoadUserPermissions(ctx, args.UserID)
	if err != nil {
		return nil, err
	}

	idsMap := make(map[int32]*types.Repo)
	for _, r := range args.Repos {
		idsMap[int32(r.ID)] = r
	}

	filtered := []*types.Repo{}
	for _, r := range p {
		// add repo to filtered if the repo is in user permissions
		if _, ok := idsMap[r.RepoID]; ok {
			filtered = append(filtered, idsMap[r.RepoID])
		}
	}
	return filtered, nil
}

// RevokeUserPermissions deletes both effective and pending permissions that could be related to a user,
// which implements the AuthzStore interface. It proactively clean up left-over pending permissions to
// prevent accidental reuse (i.e. another user with same username or email address(es) but not the same person).
func (s *authzStore) RevokeUserPermissions(ctx context.Context, args *RevokeUserPermissionsArgs) (err error) {
	return s.RevokeUserPermissionsList(ctx, []*RevokeUserPermissionsArgs{args})
}

// Bulk "RevokeUserPermissions" action.
func (s *authzStore) RevokeUserPermissionsList(ctx context.Context, argsList []*RevokeUserPermissionsArgs) (err error) {
	txs, err := s.store.Transact(ctx)
	if err != nil {
		return errors.Wrap(err, "start transaction")
	}
	defer func() { err = txs.Done(err) }()

	for _, args := range argsList {
		if err = txs.DeleteAllUserPermissions(ctx, args.UserID); err != nil {
			return errors.Wrap(err, "delete all user permissions")
		}

		for _, accounts := range args.Accounts {
			if err := txs.DeleteAllUserPendingPermissions(ctx, accounts); err != nil {
				return errors.Wrap(err, "delete all user pending permissions")
			}
		}

		if err = s.srpStore.DeleteByUser(ctx, args.UserID); err != nil {
			return errors.Wrap(err, "delete all user sub-repo permissions")
		}
	}
	return nil
}
