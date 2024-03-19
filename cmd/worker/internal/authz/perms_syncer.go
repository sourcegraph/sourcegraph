package authz

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/exp/maps"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/collections"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var _ permsSyncer = &permsSyncerImpl{}

type permsSyncer interface {
	syncRepoPerms(context.Context, api.RepoID, bool, authz.FetchPermsOptions) (*database.SetPermissionsResult, database.CodeHostStatusesSet, error)
	syncUserPerms(context.Context, int32, bool, authz.FetchPermsOptions) (*database.SetPermissionsResult, database.CodeHostStatusesSet, error)
}

// permsSyncerImpl is in charge of keeping permissions up-to-date for users and
// repositories.
type permsSyncerImpl struct {
	// The logger to use when logging messages and errors.
	logger log.Logger
	// The generic database handle.
	db database.DB
	// The database interface for any repos and external services operations.
	reposStore repos.Store
	// The mockable function to return the current time.
	clock func() time.Time

	// The lock to ensure there is no concurrent updates (i.e. only one) to the
	// permissions tables. The mutex is used to prevent any potential deadlock that
	// could be caused by concurrent database updates, and it is a simpler and more
	// intuitive approach than trying to solve deadlocks caused by how permissions
	// are stored in the database at the time of writing. In a production setup with
	// thousands of repositories and users, this approach is more effective as the
	// biggest contributor and bottleneck of background permissions syncing slowness
	// is the time spent on API calls (usually minutes) vs a database update
	// operation (usually <1s).
	permsUpdateLock sync.Mutex
	// The database interface for any permissions operations.
	permsStore database.PermsStore
}

// newPermsSyncer returns a new permissions syncer.
func newPermsSyncer(
	logger log.Logger,
	db database.DB,
	reposStore repos.Store,
	permsStore database.PermsStore,
	clock func() time.Time,
) *permsSyncerImpl {
	return &permsSyncerImpl{
		logger:     logger,
		db:         db,
		reposStore: reposStore,
		permsStore: permsStore,
		clock:      clock,
	}
}

// syncRepoPerms processes permissions syncing request in repository-centric way.
// When `noPerms` is true, the method will use partial results to update permissions
// tables even when error occurs.
func (s *permsSyncerImpl) syncRepoPerms(ctx context.Context, repoID api.RepoID, noPerms bool, fetchOpts authz.FetchPermsOptions) (result *database.SetPermissionsResult, providerStates database.CodeHostStatusesSet, err error) {
	ctx, save := s.observe(ctx, "PermsSyncer.syncRepoPerms")
	defer save(requestTypeRepo, int32(repoID), &err)

	repo, err := s.reposStore.RepoStore().Get(ctx, repoID)
	if err != nil {
		if errcode.IsNotFound(err) {
			return result, providerStates, nil
		}
		return result, providerStates, errors.Wrap(err, "get repository")
	}
	var provider authz.Provider

	// Only check authz provider for private repositories because we only need to
	// fetch permissions for private repositories.
	if repo.Private {
		// Loop over repository's sources and see if matching any authz provider's URN.
		providers := s.providersByURNs()
		for urn := range repo.Sources {
			p, ok := providers[urn]
			if ok {
				provider = p
				break
			}
		}
	}

	logger := s.logger.Scoped("syncRepoPerms").With(
		log.Object("repo",
			log.Int32("ID", int32(repo.ID)),
			log.String("name", string(repo.Name)),
			log.Bool("private", repo.Private)),
	)

	// For non-private repositories, we rely on the fact that the `provider` is
	// always nil, and we do not restrict access.
	if provider == nil {
		logger.Debug("skipFetchPerms")

		// We have no authz provider configured for the repository.
		// So we can skip the fetch permissions step and just return empty result here.
		return result, providerStates, nil
	}

	pendingAccountIDsSet := collections.NewSet[string]()
	accountIDsToUserIDs := make(map[string]authz.UserIDWithExternalAccountID) // User External Account ID -> User ID.

	extAccountIDs, err := provider.FetchRepoPerms(ctx, &extsvc.Repository{
		URI:              repo.URI,
		ExternalRepoSpec: repo.ExternalRepo,
	}, fetchOpts)

	// Detect 404 error (i.e. not authorized to call given APIs) that often happens with GitHub.com
	// when the owner of the token only has READ access. However, we don't want to fail
	// so the scheduler won't keep trying to fetch permissions of this same repository, so we
	// return a nil error and log a warning message.
	var apiErr *github.APIError
	if errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound {
		logger.Warn("ignoreUnauthorizedAPIError",
			log.Error(err),
			log.String("suggestion", "GitHub access token user may only have read access to the repository, but needs write for permissions"),
		)
		providerStates = append(providerStates, database.NewProviderStatus(provider, nil, "FetchRepoPerms"))
		return result, providerStates, nil
	}

	// Skip repo if unimplemented.
	if errors.Is(err, &authz.ErrUnimplemented{}) {
		logger.Debug("unimplemented", log.Error(err))

		providerStates = append(providerStates, database.NewProviderStatus(provider, nil, "FetchRepoPerms"))
		return result, providerStates, nil
	}

	providerStates = append(providerStates, database.NewProviderStatus(provider, err, "FetchRepoPerms"))

	if err != nil {
		// Process partial results if this is an initial fetch.
		if !noPerms {
			return result, providerStates, errors.Wrapf(err, "fetch repository permissions for repository %q (id: %d)", repo.Name, repo.ID)
		}
		logger.Warn("proceedWithPartialResults", log.Error(err))
	}

	if len(extAccountIDs) > 0 {
		accountIDs := make([]string, len(extAccountIDs))
		for i := range extAccountIDs {
			accountIDs[i] = string(extAccountIDs[i])
		}

		// Get corresponding internal database IDs.
		accountIDsToUserIDs, err = s.permsStore.GetUserIDsByExternalAccounts(ctx, &extsvc.Accounts{
			ServiceType: provider.ServiceType(),
			ServiceID:   provider.ServiceID(),
			AccountIDs:  accountIDs,
		})
		if err != nil {
			return result, providerStates, errors.Wrapf(err, "get user IDs by external accounts for repository %q (id: %d)", repo.Name, repo.ID)
		}

		// Set up the set of all account IDs that need to be bound to permissions.
		pendingAccountIDsSet.Add(accountIDs...)
	}

	// Load last finished sync job from database.
	lastSyncJob, err := s.db.PermissionSyncJobs().GetLatestFinishedSyncJob(ctx, database.ListPermissionSyncJobOpts{
		RepoID:      int(repoID),
		NotCanceled: true,
	})

	// Save permissions to database.
	// NOTE: Please read the docstring of permsUpdateLock field for reasoning of the lock.
	s.permsUpdateLock.Lock()
	defer s.permsUpdateLock.Unlock()

	txs, err := s.permsStore.Transact(ctx)
	if err != nil {
		return result, providerStates, errors.Wrapf(err, "start transaction for repository %q (id: %d)", repo.Name, repo.ID)
	}
	defer func() { err = txs.Done(err) }()

	// Write to both user_repo_permissions and repo_permissions tables by default.
	if result, err = txs.SetRepoPerms(ctx, int32(repoID), maps.Values(accountIDsToUserIDs), authz.SourceRepoSync); err != nil {
		return result, providerStates, errors.Wrapf(err, "set user repo permissions for repository %q (id: %d)", repo.Name, repo.ID)
	}

	userIDSet := collections.NewSet[int32]()
	for _, perm := range accountIDsToUserIDs {
		// Add existing user to permissions.
		userIDSet.Add(perm.UserID)
	}
	regularCount := len(userIDSet)

	// handle pending permissions
	pendingAccountIDsSet.Remove(maps.Keys(accountIDsToUserIDs)...)
	accounts := &extsvc.Accounts{
		ServiceType: provider.ServiceType(),
		ServiceID:   provider.ServiceID(),
		AccountIDs:  pendingAccountIDsSet.Values(),
	}
	p := &authz.RepoPermissions{
		RepoID: int32(repoID),
		Perm:   authz.Read, // Note: We currently only support read for repository permissions.
	}
	if err = txs.SetRepoPendingPermissions(ctx, accounts, p); err != nil {
		return result, providerStates, errors.Wrapf(err, "set repository pending permissions for repository %q (id: %d)", repo.Name, repo.ID)
	}
	pendingCount := len(p.UserIDs)

	metricsSuccessPermsSyncs.WithLabelValues("repo").Inc()

	var delayMetricField log.Field
	if lastSyncJob != nil && !lastSyncJob.FinishedAt.IsZero() {
		delay := p.SyncedAt.Sub(lastSyncJob.FinishedAt)
		metricsPermsConsecutiveSyncDelay.WithLabelValues("repo").Set(delay.Seconds())
		delayMetricField = log.Duration("consecutiveSyncDelay", delay)
	} else {
		metricsFirstPermsSyncs.WithLabelValues("repo").Inc()
		delay := p.SyncedAt.Sub(repo.CreatedAt)
		metricsPermsFirstSyncDelay.WithLabelValues("repo").Set(delay.Seconds())
		delayMetricField = log.Duration("consecutiveSyncDelay", delay)
	}

	logger.Debug("synced",
		log.Int("regularCount", regularCount),
		log.Int("pendingCount", pendingCount),
		log.Object("fetchOpts", log.Bool("invalidateCaches", fetchOpts.InvalidateCaches)),
		delayMetricField,
	)

	return result, providerStates, nil
}

// syncUserPerms processes permissions syncing request in user-centric way. When `noPerms` is true,
// the method will use partial results to update permissions tables even when error occurs.
func (s *permsSyncerImpl) syncUserPerms(ctx context.Context, userID int32, noPerms bool, fetchOpts authz.FetchPermsOptions) (*database.SetPermissionsResult, database.CodeHostStatusesSet, error) {
	var err error
	ctx, save := s.observe(ctx, "PermsSyncer.syncUserPerms")
	defer save(requestTypeUser, userID, &err)

	user, err := s.db.Users().GetByID(ctx, userID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "get user")
	}

	logger := s.logger.Scoped("syncUserPerms").With(
		log.Object("user",
			log.Int32("ID", userID),
			log.String("name", user.Username)),
	)
	ctx = featureflag.WithFlags(ctx, s.db.FeatureFlags())

	results, err := s.fetchUserPermsViaExternalAccounts(ctx, user, noPerms, fetchOpts)
	providerStates := results.providerStates
	if err != nil {
		return nil, providerStates, errors.Wrapf(err, "fetch permissions via external accounts for user %q (id: %d)", user.Username, user.ID)
	}

	// Get last sync time from the database, we don't care about errors here
	// swallowing errors was previous behavior, so keeping it for now.
	latestSyncJob, err := s.db.PermissionSyncJobs().GetLatestFinishedSyncJob(ctx, database.ListPermissionSyncJobOpts{
		UserID:      int(userID),
		NotCanceled: true,
	})
	if err != nil {
		logger.Warn("get latest finished sync job", log.Error(err))
	}

	// Save new permissions to database.
	repoIDs := collections.Set[int32]{}
	result := &database.SetPermissionsResult{}
	for acctID, rp := range results.repoPerms {
		stats, err := s.saveUserPermsForAccount(ctx, userID, acctID, rp)
		if err != nil {
			return result, providerStates, errors.Wrapf(err, "set user repo permissions for user %q (id: %d, external_account_id: %d)", user.Username, user.ID, acctID)
		}
		result.Added += stats.Added
		result.Found += stats.Found
		result.Removed += stats.Removed

		repoIDs.Add(rp...)
	}

	// Set sub-repository permissions.
	srp := s.db.SubRepoPerms()
	for spec, perm := range results.subRepoPerms {
		if err := srp.UpsertWithSpec(ctx, user.ID, spec, *perm); err != nil {
			return result, providerStates, errors.Wrapf(err, "upserting sub repo perms %v for user %q (id: %d)", spec, user.Username, user.ID)
		}
	}

	if len(results.subRepoPerms) > 0 {
		logger.Debug("subRepoSynced",
			log.Int("count", len(results.subRepoPerms)),
		)
	}

	// NOTE: Please read the docstring of permsUpdateLock field for reasoning of the lock.
	s.permsUpdateLock.Lock()
	defer s.permsUpdateLock.Unlock()

	logger.Debug("synced",
		log.Int("count", len(repoIDs)),
		log.Object("fetchOpts", log.Bool("InvalidateCache", fetchOpts.InvalidateCaches)),
	)

	metricsSuccessPermsSyncs.WithLabelValues("user").Inc()

	if latestSyncJob != nil {
		metricsPermsConsecutiveSyncDelay.WithLabelValues("user").Set(s.clock().Sub(latestSyncJob.FinishedAt).Seconds())
	} else {
		metricsFirstPermsSyncs.WithLabelValues("user").Inc()
		metricsPermsFirstSyncDelay.WithLabelValues("user").Set(s.clock().Sub(user.CreatedAt).Seconds())
	}

	return result, providerStates, nil
}

// providersByServiceID returns a list of authz.Provider configured in the external services.
// Keys are ServiceID, e.g. "https://github.com/".
func (s *permsSyncerImpl) providersByServiceID() map[string]authz.Provider {
	_, ps := authz.GetProviders()
	providers := make(map[string]authz.Provider, len(ps))
	for _, p := range ps {
		providers[p.ServiceID()] = p
	}
	return providers
}

// providersByURNs returns a list of authz.Provider configured in the external services.
// Keys are URN, e.g. "extsvc:github:1".
func (s *permsSyncerImpl) providersByURNs() map[string]authz.Provider {
	_, ps := authz.GetProviders()
	providers := make(map[string]authz.Provider, len(ps))
	for _, p := range ps {
		providers[p.URN()] = p
	}
	return providers
}

type fetchUserPermsViaExternalAccountsResults struct {
	// A map from external account ID to a list of repository IDs. This stores the
	// repository IDs that the user has access to for each external account.
	repoPerms map[int32][]int32
	// A map from external repository spec to sub-repository permissions. This stores
	// the permissions for sub-repositories of private repositories.
	subRepoPerms map[api.ExternalRepoSpec]*authz.SubRepoPermissions

	providerStates database.CodeHostStatusesSet
}

// fetchUserPermsViaExternalAccounts uses external accounts (aka. login
// connections) to list all accessible private repositories on code hosts for
// the given user.
//
// It returns a list of internal database repository IDs and is a noop when
// ` dotcom.SourcegraphDotComMode()` is true.
func (s *permsSyncerImpl) fetchUserPermsViaExternalAccounts(ctx context.Context, user *types.User, noPerms bool, fetchOpts authz.FetchPermsOptions) (results fetchUserPermsViaExternalAccountsResults, err error) {
	// NOTE: OAuth scope on sourcegraph.com does not grant access to read private
	//  repositories, therefore it is no point wasting effort and code host API rate
	//  limit quota on trying.
	if dotcom.SourcegraphDotComMode() {
		return results, nil
	}

	// Update tokens stored in external accounts.
	accts, err := s.db.UserExternalAccounts().List(ctx,
		database.ExternalAccountsListOptions{
			UserID:         user.ID,
			ExcludeExpired: true,
		},
	)
	if err != nil {
		return results, errors.Wrap(err, "list external accounts")
	}

	// We also want to include any expired accounts for GitLab as they can be
	// refreshed.
	expireGitLabAccounts, err := s.db.UserExternalAccounts().List(ctx,
		database.ExternalAccountsListOptions{
			UserID:      user.ID,
			ServiceType: extsvc.TypeGitLab,
			OnlyExpired: true,
		},
	)
	if err != nil {
		return results, errors.Wrap(err, "list expired gitlab external accounts")
	}
	accts = append(accts, expireGitLabAccounts...)

	serviceToAccounts := make(map[string]*extsvc.Account)
	for _, acct := range accts {
		serviceToAccounts[acct.ServiceType+":"+acct.ServiceID] = acct
	}

	userEmails, err := s.db.UserEmails().ListByUser(ctx,
		database.UserEmailsListOptions{
			UserID:       user.ID,
			OnlyVerified: true,
		},
	)
	if err != nil {
		return results, errors.Wrap(err, "list user verified emails")
	}

	emails := make([]string, len(userEmails))
	for i := range userEmails {
		emails[i] = userEmails[i].Email
	}

	byServiceID := s.providersByServiceID()
	accounts := s.db.UserExternalAccounts()
	logger := s.logger.Scoped("fetchUserPermsViaExternalAccounts").With(log.Int32("userID", user.ID))

	// Check if the user has an external account for every authz provider respectively,
	// and try to fetch the account when not.
	for _, provider := range byServiceID {
		providerLogger := logger.With(log.String("authzProvider", provider.ServiceID()))
		_, ok := serviceToAccounts[provider.ServiceType()+":"+provider.ServiceID()]
		if ok {
			continue
		}

		acct, err := provider.FetchAccount(ctx, user, accts, emails)
		if err != nil {
			results.providerStates = append(results.providerStates, database.NewProviderStatus(provider, err, "FetchAccount"))
			providerLogger.Error("could not fetch account from authz provider", log.Error(err))
			continue
		}

		// Not an operation failure but the authz provider is unable to determine
		// the external account for the current user.
		if acct == nil {
			providerLogger.Debug("no user account found for provider", log.String("provider_urn", provider.URN()), log.Int32("user_id", user.ID))
			continue
		}
		providerLogger.Debug("account found for provider", log.String("provider_urn", provider.URN()), log.Int32("user_id", user.ID), log.Int32("account_id", acct.ID))

		acct, err = accounts.Upsert(ctx, acct)
		if err != nil {
			providerLogger.Error("could not associate external account to user", log.Error(err))
			continue
		}

		accts = append(accts, acct)
	}

	results.subRepoPerms = make(map[api.ExternalRepoSpec]*authz.SubRepoPermissions)
	results.repoPerms = make(map[int32][]int32, len(accts))

	for _, acct := range accts {
		var repoSpecs, includeContainsSpecs, excludeContainsSpecs []api.ExternalRepoSpec

		acctLogger := logger.With(log.Int32("acct.ID", acct.ID))

		provider := byServiceID[acct.ServiceID]
		if provider == nil {
			// We have no authz provider configured for this external account.
			continue
		}

		acctLogger.Debug("update GitHub App installation access", log.Int32("accountID", acct.ID))

		// FetchUserPerms makes API requests using a client that will deal with the token
		// expiration and try to refresh it when necessary. If the client fails to update
		// the token, or if the token is revoked, the "401 Unauthorized" error will be
		// handled here.
		extPerms, err := provider.FetchUserPerms(ctx, acct, fetchOpts)
		results.providerStates = append(results.providerStates, database.NewProviderStatus(provider, err, "FetchUserPerms"))
		if err != nil {
			acctLogger.Debug("error fetching user permissions", log.Error(err))

			unauthorized := errcode.IsUnauthorized(err)
			forbidden := errcode.IsForbidden(err)
			// Detect GitHub account suspension error.
			accountSuspended := errcode.IsAccountSuspended(err)
			if unauthorized || accountSuspended || forbidden {
				// These are fatal errors that mean we should continue as if the account no
				// longer has any access.
				if err = accounts.TouchExpired(ctx, acct.ID); err != nil {
					return results, errors.Wrapf(err, "set expired for external account ID %v", acct.ID)
				}

				if unauthorized {
					acctLogger.Warn("setExternalAccountExpired, token is revoked",
						log.Bool("unauthorized", unauthorized),
					)
					continue
				}
				acctLogger.Debug("setExternalAccountExpired",
					log.Bool("unauthorized", unauthorized),
					log.Bool("accountSuspended", accountSuspended),
					log.Bool("forbidden", forbidden),
				)

				// We still want to continue processing other external accounts.
				continue
			}

			// Skip this external account if unimplemented.
			if errors.Is(err, &authz.ErrUnimplemented{}) {
				continue
			}

			if errcode.IsTemporary(err) {
				// If we have a temporary issue, we should instead return any permissions we
				// already know about to ensure that we don't temporarily remove access for the
				// user because of intermittent errors.
				acctLogger.Warn("temporary error, returning previously synced permissions", log.Error(err))

				extPerms = new(authz.ExternalUserPermissions)

				// Load last synced sub-repo perms for this user and provider.
				currentSubRepoPerms, err := s.db.SubRepoPerms().GetByUserAndService(ctx, user.ID, provider.ServiceType(), provider.ServiceID())
				if err != nil {
					return results, errors.Wrap(err, "fetching existing sub-repo permissions")
				}
				extPerms.SubRepoPermissions = make(map[extsvc.RepoID]*authz.SubRepoPermissions, len(currentSubRepoPerms))
				for k := range currentSubRepoPerms {
					v := currentSubRepoPerms[k]
					extPerms.SubRepoPermissions[extsvc.RepoID(k.ID)] = &v
				}

				// Load last synced repos for this user and account from user_repo_permissions table.
				currentRepos, err := s.permsStore.FetchReposByExternalAccount(ctx, acct.ID)
				if err != nil {
					return results, errors.Wrap(err, "fetching existing repo permissions")
				}
				// Put all the repo IDs into the results.
				for _, repoID := range currentRepos {
					results.repoPerms[acct.ID] = append(results.repoPerms[acct.ID], int32(repoID))
				}
			}

			// Process partial results if this is an initial fetch.
			if !noPerms {
				return results, errors.Wrapf(err, "fetch user permissions for external account %d", acct.ID)
			}
			acctLogger.Warn("proceedWithPartialResults", log.Error(err))
		} else {
			err = accounts.TouchLastValid(ctx, acct.ID)
			if err != nil {
				return results, errors.Wrapf(err, "set last valid for external account %d", acct.ID)
			}
		}

		if extPerms == nil {
			continue
		}

		for _, exact := range extPerms.Exacts {
			repoSpecs = append(repoSpecs,
				api.ExternalRepoSpec{
					ID:          string(exact),
					ServiceType: provider.ServiceType(),
					ServiceID:   provider.ServiceID(),
				},
			)
		}

		// Get corresponding internal database IDs.
		repoNames, err := s.listPrivateRepoNamesBySpecs(ctx, repoSpecs)
		if err != nil {
			return results, errors.Wrap(err, "list private repositories by exact matching")
		}

		// Record any sub-repository permissions.
		for repoID := range extPerms.SubRepoPermissions {
			spec := api.ExternalRepoSpec{
				// This is safe since repoID is an extsvc.RepoID which represents the external id
				// of the repo.
				ID:          string(repoID),
				ServiceType: provider.ServiceType(),
				ServiceID:   provider.ServiceID(),
			}
			results.subRepoPerms[spec] = extPerms.SubRepoPermissions[repoID]
		}

		for _, includePrefix := range extPerms.IncludeContains {
			includeContainsSpecs = append(includeContainsSpecs,
				api.ExternalRepoSpec{
					ID:          string(includePrefix),
					ServiceType: provider.ServiceType(),
					ServiceID:   provider.ServiceID(),
				},
			)
		}

		for _, excludePrefix := range extPerms.ExcludeContains {
			excludeContainsSpecs = append(excludeContainsSpecs,
				api.ExternalRepoSpec{
					ID:          string(excludePrefix),
					ServiceType: provider.ServiceType(),
					ServiceID:   provider.ServiceID(),
				},
			)
		}

		// Exclusions are relative to inclusions, so if there is no inclusion, exclusion
		// are meaningless and no need to trigger a DB query.
		if len(includeContainsSpecs) > 0 {
			rs, err := s.reposStore.RepoStore().ListMinimalRepos(ctx,
				database.ReposListOptions{
					ExternalRepoIncludeContains: includeContainsSpecs,
					ExternalRepoExcludeContains: excludeContainsSpecs,
					OnlyPrivate:                 true,
				},
			)
			if err != nil {
				return results, errors.Wrap(err, "list external repositories by contains matching")
			}
			repoNames = append(repoNames, rs...)
		}

		// repoIDs represents repos the user is allowed to read.
		if len(results.repoPerms[acct.ID]) == 0 {
			// We may already have some repos if we hit a temporary error above in which case
			// we don't want to clear it out.
			results.repoPerms[acct.ID] = make([]int32, 0, len(repoNames))
		}
		for _, r := range repoNames {
			results.repoPerms[acct.ID] = append(results.repoPerms[acct.ID], int32(r.ID))
		}
	}

	return results, nil
}

// listPrivateRepoNamesBySpecs slices over the `repoSpecs` at pace of 10000
// elements at a time to workaround Postgres' limit of 65535 bind parameters
// using exact name matching. This method only includes private repository names
// and does not do deduplication on the returned list.
func (s *permsSyncerImpl) listPrivateRepoNamesBySpecs(ctx context.Context, repoSpecs []api.ExternalRepoSpec) ([]types.MinimalRepo, error) {
	if len(repoSpecs) == 0 {
		return []types.MinimalRepo{}, nil
	}

	remaining := repoSpecs
	nextCut := 10000
	if len(remaining) < nextCut {
		nextCut = len(remaining)
	}

	repoNames := make([]types.MinimalRepo, 0, len(repoSpecs))
	for nextCut > 0 {
		rs, err := s.reposStore.RepoStore().ListMinimalRepos(ctx,
			database.ReposListOptions{
				ExternalRepos: remaining[:nextCut],
				OnlyPrivate:   true,
			},
		)
		if err != nil {
			return nil, err
		}

		repoNames = append(repoNames, rs...)

		remaining = remaining[nextCut:]
		if len(remaining) < nextCut {
			nextCut = len(remaining)
		}
	}
	return repoNames, nil
}

func (s *permsSyncerImpl) saveUserPermsForAccount(ctx context.Context, userID int32, acctID int32, repoIDs []int32) (*database.SetPermissionsResult, error) {
	logger := s.logger.Scoped("saveUserPermsForAccount").With(
		log.Object("user",
			log.Int32("ID", userID),
			log.Int32("ExternalAccountID", acctID)),
	)

	// NOTE: Please read the docstring of permsUpdateLock field for reasoning of the lock.
	s.permsUpdateLock.Lock()
	// Save new permissions to database.
	defer s.permsUpdateLock.Unlock()

	stats, err := s.permsStore.SetUserExternalAccountPerms(ctx, authz.UserIDWithExternalAccountID{
		UserID:            userID,
		ExternalAccountID: acctID,
	}, repoIDs, authz.SourceUserSync)
	if err != nil {
		logger.Warn("saving perms to DB", log.Error(err))
		return nil, err
	}

	return stats, nil
}

func (s *permsSyncerImpl) observe(ctx context.Context, name string) (context.Context, func(requestType, int32, *error)) {
	began := s.clock()
	tr, ctx := trace.New(ctx, name)

	return ctx, func(typ requestType, id int32, err *error) {
		defer tr.End()
		tr.SetAttributes(attribute.Int64("id", int64(id)))

		var typLabel string
		switch typ {
		case requestTypeRepo:
			typLabel = "repo"
		case requestTypeUser:
			typLabel = "user"
		default:
			tr.SetError(errors.Errorf("unexpected request type: %v", typ))
			return
		}

		success := err == nil || *err == nil
		metricsSyncDuration.WithLabelValues(typLabel, strconv.FormatBool(success)).Observe(time.Since(began).Seconds())

		if !success {
			tr.SetError(*err)
			metricsSyncErrors.WithLabelValues(typLabel).Inc()
		}
	}
}

// requestType is the type of the permissions syncing request. It defines the
// permissions syncing is either repository-centric or user-centric.
type requestType int

// A list of request types, the larger the value, the higher the priority.
// requestTypeUser had the highest because it is often triggered by a user action
// (e.g. sign up, log in).
const (
	requestTypeRepo requestType = iota + 1
	requestTypeUser
)

func (t requestType) String() string {
	switch t {
	case requestTypeRepo:
		return "repo"
	case requestTypeUser:
		return "user"
	}
	return strconv.Itoa(int(t))
}
