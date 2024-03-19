package resolvers

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/sourcegraph/log"
	"golang.org/x/exp/maps"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/authz/permssync"
	"github.com/sourcegraph/sourcegraph/internal/collections"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var errDisabledSourcegraphDotCom = errors.New("not enabled on sourcegraph.com")

type Resolver struct {
	logger log.Logger
	db     database.DB
}

// checkLicense returns a user-facing error if the provided feature is not purchased
// with the current license or any error occurred while validating the licence.
func (r *Resolver) checkLicense(feature licensing.Feature) error {
	err := licensing.Check(feature)
	if err != nil {
		if licensing.IsFeatureNotActivated(err) {
			return err
		}

		r.logger.Error("Unable to check license for feature", log.Error(err))
		return errors.New("Unable to check license feature, please refer to logs for actual error message.")
	}
	return nil
}

func NewResolver(observationCtx *observation.Context, db database.DB) graphqlbackend.AuthzResolver {
	return &Resolver{
		logger: observationCtx.Logger.Scoped("authz.Resolver"),
		db:     db,
	}
}

func (r *Resolver) SetRepositoryPermissionsForUsers(ctx context.Context, args *graphqlbackend.RepoPermsArgs) (*graphqlbackend.EmptyResponse, error) {
	if dotcom.SourcegraphDotComMode() {
		return nil, errDisabledSourcegraphDotCom
	}

	if err := r.checkLicense(licensing.FeatureExplicitPermissionsAPI); err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Only site admins can mutate repository permissions.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	repoID, err := graphqlbackend.UnmarshalRepositoryID(args.Repository)
	if err != nil {
		return nil, err
	}
	// Make sure the repo ID is valid.
	if _, err = r.db.Repos().Get(ctx, repoID); err != nil {
		return nil, err
	}

	bindIDs := make([]string, 0, len(args.UserPermissions))
	for _, up := range args.UserPermissions {
		bindIDs = append(bindIDs, up.BindID)
	}

	mapping, err := r.db.Perms().MapUsers(ctx, bindIDs, conf.PermissionsUserMapping())
	if err != nil {
		return nil, err
	}

	pendingBindIDs := make([]string, 0, len(bindIDs))
	for _, bindID := range bindIDs {
		if _, ok := mapping[bindID]; !ok {
			pendingBindIDs = append(pendingBindIDs, bindID)
		}
	}

	userIDs := collections.NewSet(maps.Values(mapping)...)

	p := &authz.RepoPermissions{
		RepoID:  int32(repoID),
		Perm:    authz.Read, // Note: We currently only support read for repository permissions.
		UserIDs: userIDs,
	}

	perms := make([]authz.UserIDWithExternalAccountID, 0, len(userIDs))
	for userID := range userIDs {
		perms = append(perms, authz.UserIDWithExternalAccountID{
			UserID: userID,
		})
	}

	txs, err := r.db.Perms().Transact(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "start transaction")
	}
	defer func() { err = txs.Done(err) }()

	accounts := &extsvc.Accounts{
		ServiceType: authz.SourcegraphServiceType,
		ServiceID:   authz.SourcegraphServiceID,
		AccountIDs:  pendingBindIDs,
	}

	if _, err = txs.SetRepoPerms(ctx, p.RepoID, perms, authz.SourceAPI); err != nil {
		return nil, errors.Wrap(err, "set user repo permissions")
	} else if err = txs.SetRepoPendingPermissions(ctx, accounts, p); err != nil {
		return nil, errors.Wrap(err, "set repository pending permissions")
	}

	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *Resolver) SetRepositoryPermissionsUnrestricted(ctx context.Context, args *graphqlbackend.RepoUnrestrictedArgs) (*graphqlbackend.EmptyResponse, error) {
	if dotcom.SourcegraphDotComMode() {
		return nil, errDisabledSourcegraphDotCom
	}

	if err := r.checkLicense(licensing.FeatureExplicitPermissionsAPI); err != nil {
		return nil, err
	}
	// 🚨 SECURITY: Only site admins can mutate repository permissions.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	ids := make([]int32, 0, len(args.Repositories))
	for _, id := range args.Repositories {
		repoID, err := graphqlbackend.UnmarshalRepositoryID(id)
		if err != nil {
			return nil, errors.Wrap(err, "unmarshalling id")
		}
		ids = append(ids, int32(repoID))
	}

	if err := r.db.Perms().SetRepoPermissionsUnrestricted(ctx, ids, args.Unrestricted); err != nil {
		return nil, errors.Wrap(err, "setting unrestricted field")
	}

	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *Resolver) ScheduleRepositoryPermissionsSync(ctx context.Context, args *graphqlbackend.RepositoryIDArgs) (*graphqlbackend.EmptyResponse, error) {
	if err := r.checkLicense(licensing.FeatureACLs); err != nil {
		return nil, err
	}

	// 🚨 SECURITY: Only site admins can trigger repository permissions syncs.
	err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db)
	if err != nil {
		return nil, err
	}

	repoID, err := graphqlbackend.UnmarshalRepositoryID(args.Repository)
	if err != nil {
		return nil, err
	}

	req := permssync.ScheduleSyncOpts{RepoIDs: []api.RepoID{repoID}, Reason: database.ReasonManualRepoSync, TriggeredByUserID: actor.FromContext(ctx).UID}
	permssync.SchedulePermsSync(ctx, r.logger, r.db, req)

	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *Resolver) ScheduleUserPermissionsSync(ctx context.Context, args *graphqlbackend.UserPermissionsSyncArgs) (*graphqlbackend.EmptyResponse, error) {
	if err := r.checkLicense(licensing.FeatureACLs); err != nil {
		return nil, err
	}

	userID, err := graphqlbackend.UnmarshalUserID(args.User)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: User can trigger permission sync for themselves, site admins for any user.
	if err := auth.CheckSiteAdminOrSameUser(ctx, r.db, userID); err != nil {
		return nil, err
	}

	req := permssync.ScheduleSyncOpts{UserIDs: []int32{userID}, Reason: database.ReasonManualUserSync, TriggeredByUserID: actor.FromContext(ctx).UID}
	if args.Options != nil && args.Options.InvalidateCaches != nil && *args.Options.InvalidateCaches {
		req.Options.InvalidateCaches = true
	}

	permssync.SchedulePermsSync(ctx, r.logger, r.db, req)

	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *Resolver) SetSubRepositoryPermissionsForUsers(ctx context.Context, args *graphqlbackend.SubRepoPermsArgs) (*graphqlbackend.EmptyResponse, error) {
	if err := r.checkLicense(licensing.FeatureExplicitPermissionsAPI); err != nil {
		return nil, err
	}
	if dotcom.SourcegraphDotComMode() {
		return nil, errDisabledSourcegraphDotCom
	}

	// 🚨 SECURITY: Only site admins can mutate repository permissions.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	repoID, err := graphqlbackend.UnmarshalRepositoryID(args.Repository)
	if err != nil {
		return nil, err
	}

	err = r.db.WithTransact(ctx, func(tx database.DB) error {
		// Make sure the repo ID is valid.
		if _, err = tx.Repos().Get(ctx, repoID); err != nil {
			return err
		}

		bindIDs := make([]string, 0, len(args.UserPermissions))
		for _, up := range args.UserPermissions {
			bindIDs = append(bindIDs, up.BindID)
		}

		mapping, err := r.db.Perms().MapUsers(ctx, bindIDs, conf.PermissionsUserMapping())
		if err != nil {
			return err
		}

		for _, perm := range args.UserPermissions {
			if (perm.PathIncludes == nil || perm.PathExcludes == nil) && perm.Paths == nil {
				return errors.New("either both pathIncludes and pathExcludes needs to be set, or paths needs to be set")
			}
		}

		for _, perm := range args.UserPermissions {
			userID, ok := mapping[perm.BindID]
			if !ok {
				return errors.Errorf("user %q not found", perm.BindID)
			}

			var paths []string
			if perm.Paths == nil {
				paths = make([]string, 0, len(*perm.PathIncludes)+len(*perm.PathExcludes))
				for _, include := range *perm.PathIncludes {
					if !strings.HasPrefix(include, "/") { // ensure leading slash
						include = "/" + include
					}
					paths = append(paths, include)
				}
				for _, exclude := range *perm.PathExcludes {
					if !strings.HasPrefix(exclude, "/") { // ensure leading slash
						exclude = "/" + exclude
					}
					paths = append(paths, "-"+exclude) // excludes start with a minus (-)
				}
			} else {
				paths = make([]string, 0, len(*perm.Paths))
				for _, path := range *perm.Paths {
					if strings.HasPrefix(path, "-") {
						if !strings.HasPrefix(path, "-/") {
							path = "-/" + strings.TrimPrefix(path, "-")
						}
					} else {
						if !strings.HasPrefix(path, "/") {
							path = "/" + path
						}
					}
					paths = append(paths, path)
				}
			}

			if err := tx.SubRepoPerms().Upsert(ctx, userID, repoID, authz.SubRepoPermissions{
				Paths: paths,
			}); err != nil {
				return errors.Wrap(err, "upserting sub-repo permissions")
			}
		}
		return nil
	})

	return &graphqlbackend.EmptyResponse{}, err
}

func (r *Resolver) SetRepositoryPermissionsForBitbucketProject(
	ctx context.Context, args *graphqlbackend.RepoPermsBitbucketProjectArgs,
) (*graphqlbackend.EmptyResponse, error) {
	if dotcom.SourcegraphDotComMode() {
		return nil, errDisabledSourcegraphDotCom
	}

	if err := r.checkLicense(licensing.FeatureExplicitPermissionsAPI); err != nil {
		return nil, err
	}

	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	externalServiceID, err := graphqlbackend.UnmarshalExternalServiceID(args.CodeHost)
	if err != nil {
		return nil, err
	}

	unrestricted := false
	if args.Unrestricted != nil {
		unrestricted = *args.Unrestricted
	}

	// get the external service and check if it is Bitbucket Server
	svc, err := r.db.ExternalServices().GetByID(ctx, externalServiceID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get external service %d", externalServiceID)
	}

	if svc.Kind != extsvc.KindBitbucketServer {
		return nil, errors.Newf("expected Bitbucket Server external service, got: %s", svc.Kind)
	}

	jobID, err := r.db.BitbucketProjectPermissions().Enqueue(ctx, args.ProjectKey, externalServiceID, args.UserPermissions, unrestricted)
	if err != nil {
		return nil, err
	}

	r.logger.Debug("Bitbucket project permissions job enqueued", log.Int("jobID", jobID))

	return &graphqlbackend.EmptyResponse{}, nil
}

func (r *Resolver) CancelPermissionsSyncJob(ctx context.Context, args *graphqlbackend.CancelPermissionsSyncJobArgs) (graphqlbackend.CancelPermissionsSyncJobResultMessage, error) {
	if err := r.checkLicense(licensing.FeatureACLs); err != nil {
		return graphqlbackend.CancelPermissionsSyncJobResultMessageError, err
	}

	// 🚨 SECURITY: Only site admins can cancel permissions sync jobs.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return graphqlbackend.CancelPermissionsSyncJobResultMessageError, err
	}

	syncJobID, err := unmarshalPermissionsSyncJobID(args.Job)
	if err != nil {
		return graphqlbackend.CancelPermissionsSyncJobResultMessageError, err
	}

	reason := ""
	if args.Reason != nil {
		reason = *args.Reason
	}

	err = r.db.PermissionSyncJobs().CancelQueuedJob(ctx, reason, syncJobID)
	// We shouldn't return an error when the job is already processing or not found
	// by ID (might already be cleaned up).
	if err != nil {
		if errcode.IsNotFound(err) {
			return graphqlbackend.CancelPermissionsSyncJobResultMessageNotFound, nil
		}
		return graphqlbackend.CancelPermissionsSyncJobResultMessageError, err
	}
	return graphqlbackend.CancelPermissionsSyncJobResultMessageSuccess, nil
}

func (r *Resolver) AuthorizedUserRepositories(ctx context.Context, args *graphqlbackend.AuthorizedRepoArgs) (graphqlbackend.RepositoryConnectionResolver, error) {
	if dotcom.SourcegraphDotComMode() {
		return nil, errDisabledSourcegraphDotCom
	}

	// 🚨 SECURITY: Only site admins can query repository permissions.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	var (
		err    error
		bindID string
		user   *types.User
	)
	if args.Email != nil {
		bindID = *args.Email
		// 🚨 SECURITY: It is critical to ensure the email is verified.
		user, err = r.db.Users().GetByVerifiedEmail(ctx, *args.Email)
	} else if args.Username != nil {
		bindID = *args.Username
		user, err = r.db.Users().GetByUsername(ctx, *args.Username)
	} else {
		return nil, errors.New("neither email nor username is given to identify a user")
	}
	if err != nil && !errcode.IsNotFound(err) {
		return nil, err
	}

	var ids []int32
	if user != nil {
		var perms []authz.Permission
		perms, err = r.db.Perms().LoadUserPermissions(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		ids = make([]int32, len(perms))
		for i, perm := range perms {
			ids[i] = perm.RepoID
		}
		slices.Sort(ids)
	} else {
		p := &authz.UserPendingPermissions{
			ServiceType: authz.SourcegraphServiceType,
			ServiceID:   authz.SourcegraphServiceID,
			BindID:      bindID,
			Perm:        authz.Read, // Note: We currently only support read for repository permissions.
			Type:        authz.PermRepos,
		}
		err = r.db.Perms().LoadUserPendingPermissions(ctx, p)
		if err != nil && err != authz.ErrPermsNotFound {
			return nil, err
		}
		// If no row is found, we return an empty list to the consumer.
		if err == authz.ErrPermsNotFound {
			ids = []int32{}
		} else {
			ids = p.GenerateSortedIDsSlice()
		}
	}

	return &repositoryConnectionResolver{
		db:    r.db,
		ids:   ids,
		first: args.First,
		after: args.After,
	}, nil
}

func (r *Resolver) UsersWithPendingPermissions(ctx context.Context) ([]string, error) {
	// 🚨 SECURITY: Only site admins can query repository permissions.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	return r.db.Perms().ListPendingUsers(ctx, authz.SourcegraphServiceType, authz.SourcegraphServiceID)
}

func (r *Resolver) AuthorizedUsers(ctx context.Context, args *graphqlbackend.RepoAuthorizedUserArgs) (graphqlbackend.UserConnectionResolver, error) {
	// 🚨 SECURITY: Only site admins can query repository permissions.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	repoID, err := graphqlbackend.UnmarshalRepositoryID(args.RepositoryID)
	if err != nil {
		return nil, err
	}
	// Make sure the repo ID is valid.
	if _, err = r.db.Repos().Get(ctx, repoID); err != nil {
		return nil, err
	}

	p, err := r.db.Perms().LoadRepoPermissions(ctx, int32(repoID))
	if err != nil {
		return nil, err
	}
	ids := make([]int32, len(p))
	for i, perm := range p {
		ids[i] = perm.UserID
	}
	slices.Sort(ids)

	return &userConnectionResolver{
		db:    r.db,
		ids:   ids,
		first: args.First,
		after: args.After,
	}, nil
}

func (r *Resolver) AuthzProviderTypes(ctx context.Context) ([]string, error) {
	// 🚨 SECURITY: Only site admins can query for authz providers.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}
	_, providers := authz.GetProviders()
	providerTypes := make([]string, 0, len(providers))
	for _, p := range providers {
		providerTypes = append(providerTypes, p.ServiceType())
	}
	return providerTypes, nil
}

var jobStatuses = map[string]bool{
	"queued":     true,
	"processing": true,
	"completed":  true,
	"canceled":   true,
	"errored":    true,
	"failed":     true,
}

func (r *Resolver) BitbucketProjectPermissionJobs(ctx context.Context, args *graphqlbackend.BitbucketProjectPermissionJobsArgs) (graphqlbackend.BitbucketProjectsPermissionJobsResolver, error) {
	if dotcom.SourcegraphDotComMode() {
		return nil, errDisabledSourcegraphDotCom
	}
	// 🚨 SECURITY: Only site admins can query repository permissions.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}
	loweredAndTrimmedStatus := strings.ToLower(strings.TrimSpace(getOrDefault(args.Status)))
	if loweredAndTrimmedStatus != "" && !jobStatuses[loweredAndTrimmedStatus] {
		return nil, errors.New("Please provide one of the following job statuses: queued, processing, completed, canceled, errored, failed")
	}
	args.Status = &loweredAndTrimmedStatus

	jobs, err := r.db.BitbucketProjectPermissions().ListJobs(ctx, convertJobsArgsToOpts(args))
	if err != nil {
		return nil, errors.Wrap(err, "getting a list of Bitbucket Projects permission sync jobs")
	}
	return NewBitbucketProjectsPermissionJobsResolver(jobs), nil
}

func convertJobsArgsToOpts(args *graphqlbackend.BitbucketProjectPermissionJobsArgs) database.ListJobsOptions {
	if args == nil {
		return database.ListJobsOptions{}
	}

	return database.ListJobsOptions{
		ProjectKeys: getOrDefault(args.ProjectKeys),
		State:       getOrDefault(args.Status),
		Count:       getOrDefault(args.Count),
	}
}

// getOrDefault accepts a pointer of a type T and returns dereferenced value if the pointer
// is not nil, or zero-value for the given type otherwise
func getOrDefault[T any](ptr *T) T {
	var result T
	if ptr == nil {
		return result
	} else {
		return *ptr
	}
}

func (r *Resolver) RepositoryPermissionsInfo(ctx context.Context, id graphql.ID) (graphqlbackend.PermissionsInfoResolver, error) {
	// 🚨 SECURITY: Only site admins can query repository permissions.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	repoID, err := graphqlbackend.UnmarshalRepositoryID(id)
	if err != nil {
		return nil, err
	}
	// Make sure the repo ID is valid and not soft-deleted.
	if _, err = r.db.Repos().Get(ctx, repoID); err != nil {
		return nil, err
	}

	p, err := r.db.Perms().LoadRepoPermissions(ctx, int32(repoID))
	if err != nil {
		return nil, err
	}
	// If there's exactly 1 item and the user ID is 0, it means the repository is unrestricted.
	unrestricted := (len(p) == 1 && p[0].UserID == 0)

	// get max updated_at time from the permissions
	updatedAt := time.Time{}
	for _, permission := range p {
		if permission.UpdatedAt.After(updatedAt) {
			updatedAt = permission.UpdatedAt
		}
	}

	// get sync time from the sync jobs table
	latestSyncJob, err := r.db.PermissionSyncJobs().GetLatestFinishedSyncJob(ctx, database.ListPermissionSyncJobOpts{
		RepoID:      int(repoID),
		NotCanceled: true,
	})
	if err != nil {
		return nil, err
	}
	syncedAt := time.Time{}
	if latestSyncJob != nil {
		syncedAt = latestSyncJob.FinishedAt
	}

	return &permissionsInfoResolver{
		db:           r.db,
		repoID:       repoID,
		perms:        authz.Read,
		syncedAt:     syncedAt,
		updatedAt:    updatedAt,
		source:       "",
		unrestricted: unrestricted,
	}, nil
}

func (r *Resolver) UserPermissionsInfo(ctx context.Context, id graphql.ID) (graphqlbackend.PermissionsInfoResolver, error) {
	userID, err := graphqlbackend.UnmarshalUserID(id)
	if err != nil {
		return nil, err
	}

	// 🚨 SECURITY: User can query own permissions, site admins all user permissions.
	if err := auth.CheckSiteAdminOrSameUser(ctx, r.db, userID); err != nil {
		return nil, err
	}

	// Make sure the user ID is valid and not soft-deleted.
	if _, err = r.db.Users().GetByID(ctx, userID); err != nil {
		return nil, err
	}

	perms, err := r.db.Perms().LoadUserPermissions(ctx, userID)
	if err != nil {
		return nil, err
	}

	// get max updated_at time from the permissions
	updatedAt := time.Time{}
	var source string

	for _, p := range perms {
		if p.UpdatedAt.After(updatedAt) {
			updatedAt = p.UpdatedAt
			source = p.Source.ToGraphQL()
		}
	}

	return &permissionsInfoResolver{
		db:        r.db,
		userID:    userID,
		perms:     authz.Read,
		updatedAt: updatedAt,
		source:    source,
	}, nil
}

func (r *Resolver) PermissionsSyncJobs(ctx context.Context, args graphqlbackend.ListPermissionsSyncJobsArgs) (*graphqlutil.ConnectionResolver[graphqlbackend.PermissionsSyncJobResolver], error) {
	// 🚨 SECURITY: Only site admins can query sync jobs records or the users themselves.
	if args.UserID != nil {
		userID, err := graphqlbackend.UnmarshalUserID(*args.UserID)
		if err != nil {
			return nil, err
		}

		if err := auth.CheckSiteAdminOrSameUser(ctx, r.db, userID); err != nil {
			return nil, err
		}
	} else if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	return NewPermissionsSyncJobsResolver(r.db, args)
}

func (r *Resolver) PermissionsSyncingStats(ctx context.Context) (graphqlbackend.PermissionsSyncingStatsResolver, error) {
	stats := permissionsSyncingStats{
		db: r.db,
	}

	// 🚨 SECURITY: Only site admins can query permissions syncing stats.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return stats, err
	}

	return stats, nil
}

type permissionsSyncingStats struct {
	db database.DB
}

func (s permissionsSyncingStats) QueueSize(ctx context.Context) (int32, error) {
	count, err := s.db.PermissionSyncJobs().Count(ctx, database.ListPermissionSyncJobOpts{State: database.PermissionsSyncJobStateQueued})
	return int32(count), err
}

func (s permissionsSyncingStats) UsersWithLatestJobFailing(ctx context.Context) (int32, error) {
	return s.db.PermissionSyncJobs().CountUsersWithFailingSyncJob(ctx)
}

func (s permissionsSyncingStats) ReposWithLatestJobFailing(ctx context.Context) (int32, error) {
	return s.db.PermissionSyncJobs().CountReposWithFailingSyncJob(ctx)
}

func (s permissionsSyncingStats) UsersWithNoPermissions(ctx context.Context) (int32, error) {
	count, err := s.db.Perms().CountUsersWithNoPerms(ctx)
	return int32(count), err
}

func (s permissionsSyncingStats) ReposWithNoPermissions(ctx context.Context) (int32, error) {
	count, err := s.db.Perms().CountReposWithNoPerms(ctx)
	return int32(count), err
}

func (s permissionsSyncingStats) UsersWithStalePermissions(ctx context.Context) (int32, error) {
	count, err := s.db.Perms().CountUsersWithStalePerms(ctx, new(auth.Backoff).SyncUserBackoff())

	return int32(count), err
}

func (s permissionsSyncingStats) ReposWithStalePermissions(ctx context.Context) (int32, error) {
	count, err := s.db.Perms().CountReposWithStalePerms(ctx, new(auth.Backoff).SyncRepoBackoff())

	return int32(count), err
}
