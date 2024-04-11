package graphqlbackend

import (
	"context"
	"net/url"
	"strings"
	"sync"

	"github.com/graph-gophers/graphql-go"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	repoupdaterprotocol "github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (r *RepositoryResolver) MirrorInfo() *repositoryMirrorInfoResolver {
	return &repositoryMirrorInfoResolver{repository: r, db: r.db, gitServerClient: r.gitserverClient}
}

type repositoryMirrorInfoResolver struct {
	repository      *RepositoryResolver
	db              database.DB
	gitServerClient gitserver.Client

	// memoize the repo-updater RepoUpdateSchedulerInfo call
	repoUpdateSchedulerInfoOnce   sync.Once
	repoUpdateSchedulerInfoResult *repoupdaterprotocol.RepoUpdateSchedulerInfoResult
	repoUpdateSchedulerInfoErr    error

	// memoize the gitserverRepo
	gsRepoOnce sync.Once
	gsRepo     *types.GitserverRepo
	gsRepoErr  error
}

func (r *repositoryMirrorInfoResolver) computeGitserverRepo(ctx context.Context) (*types.GitserverRepo, error) {
	r.gsRepoOnce.Do(func() {
		r.gsRepo, r.gsRepoErr = r.db.GitserverRepos().GetByID(ctx, r.repository.IDInt32())
	})
	return r.gsRepo, r.gsRepoErr
}

func (r *repositoryMirrorInfoResolver) repoUpdateSchedulerInfo(ctx context.Context) (*repoupdaterprotocol.RepoUpdateSchedulerInfoResult, error) {
	r.repoUpdateSchedulerInfoOnce.Do(func() {
		args := repoupdaterprotocol.RepoUpdateSchedulerInfoArgs{
			ID: r.repository.IDInt32(),
		}
		r.repoUpdateSchedulerInfoResult, r.repoUpdateSchedulerInfoErr = repoupdater.DefaultClient.RepoUpdateSchedulerInfo(ctx, args)
	})
	return r.repoUpdateSchedulerInfoResult, r.repoUpdateSchedulerInfoErr
}

// TODO(flying-robot): this regex and the majority of the removeUserInfo function can
// be extracted to a common location in a subsequent change.
var nonSCPURLRegex = lazyregexp.New(`^(git\+)?(https?|ssh|rsync|file|git|perforce)://`)

func (r *repositoryMirrorInfoResolver) RemoteURL(ctx context.Context) (string, error) {
	// 🚨 SECURITY: The remote URL might contain secret credentials in the URL userinfo, so
	// only allow site admins to see it.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return "", err
	}

	// removeUserinfo strips the userinfo component of a remote URL. The provided string s
	// will be returned if it cannot be parsed as a URL.
	removeUserinfo := func(s string) string {
		// Support common syntax (HTTPS, SSH, etc.)
		if nonSCPURLRegex.MatchString(s) {
			u, err := url.Parse(s)
			if err != nil {
				return s
			}
			u.User = nil
			return u.String()
		}

		// Support SCP-style syntax.
		u, err := url.Parse("fake://" + strings.Replace(s, ":", "/", 1))
		if err != nil {
			return s
		}
		u.User = nil
		return strings.Replace(strings.Replace(u.String(), "fake://", "", 1), "/", ":", 1)
	}

	repo, err := r.repository.getRepo(ctx)
	if err != nil {
		return "", err
	}

	cloneURLs := repo.CloneURLs()
	if len(cloneURLs) == 0 {
		// This should never happen: clone URL is enforced to be a non-empty string
		// in our store, and we delete repos once they have no external service connection
		// anymore.
		return "", errors.Errorf("no sources for %q", repo)
	}

	return removeUserinfo(cloneURLs[0]), nil
}

func (r *repositoryMirrorInfoResolver) Cloned(ctx context.Context) (bool, error) {
	info, err := r.computeGitserverRepo(ctx)
	if err != nil {
		return false, err
	}

	return info.CloneStatus == types.CloneStatusCloned, nil
}

func (r *repositoryMirrorInfoResolver) CloneInProgress(ctx context.Context) (bool, error) {
	info, err := r.computeGitserverRepo(ctx)
	if err != nil {
		return false, err
	}

	return info.CloneStatus == types.CloneStatusCloning, nil
}

func (r *repositoryMirrorInfoResolver) CloneProgress(ctx context.Context) (*string, error) {
	progress, err := r.gitServerClient.RepoCloneProgress(ctx, r.repository.RepoName())
	if err != nil {
		return nil, err
	}

	return strptr(progress.CloneProgress), nil
}

func (r *repositoryMirrorInfoResolver) LastError(ctx context.Context) (*string, error) {
	info, err := r.computeGitserverRepo(ctx)
	if err != nil {
		return nil, err
	}

	return strptr(info.LastError), nil
}

func (r *repositoryMirrorInfoResolver) LastSyncOutput(ctx context.Context) (*string, error) {
	output, ok, err := r.db.GitserverRepos().GetLastSyncOutput(ctx, r.repository.name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	return &output, nil
}

func (r *repositoryMirrorInfoResolver) UpdatedAt(ctx context.Context) (*gqlutil.DateTime, error) {
	info, err := r.computeGitserverRepo(ctx)
	if err != nil {
		return nil, err
	}

	if info.LastFetched.IsZero() {
		return nil, nil
	}

	return &gqlutil.DateTime{Time: info.LastFetched}, nil
}

func (r *repositoryMirrorInfoResolver) NextSyncAt(ctx context.Context) (*gqlutil.DateTime, error) {
	info, err := r.repoUpdateSchedulerInfo(ctx)
	if err != nil {
		return nil, err
	}

	if info == nil || info.Schedule == nil || info.Schedule.Due.IsZero() {
		return nil, nil
	}
	return &gqlutil.DateTime{Time: info.Schedule.Due}, nil
}

func (r *repositoryMirrorInfoResolver) IsCorrupted(ctx context.Context) (bool, error) {
	info, err := r.computeGitserverRepo(ctx)
	if err != nil {
		return false, err
	}

	if info.CorruptedAt.IsZero() {
		return false, err
	}
	return true, nil
}

func (r *repositoryMirrorInfoResolver) CorruptionLogs(ctx context.Context) ([]*corruptionLogResolver, error) {
	info, err := r.computeGitserverRepo(ctx)
	if err != nil {
		return nil, err
	}

	logs := make([]*corruptionLogResolver, 0, len(info.CorruptionLogs))
	for _, l := range info.CorruptionLogs {
		logs = append(logs, &corruptionLogResolver{log: l})
	}

	return logs, nil
}

type corruptionLogResolver struct {
	log types.RepoCorruptionLog
}

func (r *corruptionLogResolver) Timestamp() (gqlutil.DateTime, error) {
	return gqlutil.DateTime{Time: r.log.Timestamp}, nil
}

func (r *corruptionLogResolver) Reason() (string, error) {
	return r.log.Reason, nil
}

func (r *repositoryMirrorInfoResolver) ByteSize(ctx context.Context) (BigInt, error) {
	info, err := r.computeGitserverRepo(ctx)
	if err != nil {
		return 0, err
	}

	return BigInt(info.RepoSizeBytes), err
}

func (r *repositoryMirrorInfoResolver) Shard(ctx context.Context) (*string, error) {
	// 🚨 SECURITY: This is a query that reveals internal details of the
	// instance that only the admin should be able to see.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	info, err := r.computeGitserverRepo(ctx)
	if err != nil {
		return nil, err
	}

	if info.ShardID == "" {
		return nil, nil
	}

	return &info.ShardID, nil
}

func (r *repositoryMirrorInfoResolver) UpdateSchedule(ctx context.Context) (*updateScheduleResolver, error) {
	info, err := r.repoUpdateSchedulerInfo(ctx)
	if err != nil {
		return nil, err
	}
	if info.Schedule == nil {
		return nil, nil
	}
	return &updateScheduleResolver{schedule: info.Schedule}, nil
}

type updateScheduleResolver struct {
	schedule *repoupdaterprotocol.RepoScheduleState
}

func (r *updateScheduleResolver) IntervalSeconds() int32 {
	return int32(r.schedule.IntervalSeconds)
}

func (r *updateScheduleResolver) Due() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.schedule.Due}
}

func (r *updateScheduleResolver) Index() int32 {
	return int32(r.schedule.Index)
}

func (r *updateScheduleResolver) Total() int32 {
	return int32(r.schedule.Total)
}

func (r *repositoryMirrorInfoResolver) UpdateQueue(ctx context.Context) (*updateQueueResolver, error) {
	info, err := r.repoUpdateSchedulerInfo(ctx)
	if err != nil {
		return nil, err
	}
	if info.Queue == nil {
		return nil, nil
	}
	return &updateQueueResolver{queue: info.Queue}, nil
}

type updateQueueResolver struct {
	queue *repoupdaterprotocol.RepoQueueState
}

func (r *updateQueueResolver) Updating() bool {
	return r.queue.Updating
}

func (r *updateQueueResolver) Index() int32 {
	return int32(r.queue.Index)
}

func (r *updateQueueResolver) Total() int32 {
	return int32(r.queue.Total)
}

func (r *schemaResolver) CheckMirrorRepositoryConnection(ctx context.Context, args *struct {
	Repository graphql.ID
}) (*checkMirrorRepositoryConnectionResult, error) {
	// 🚨 SECURITY: This is an expensive operation and the errors may contain secrets,
	// so only site admins may run it.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	repoID, err := UnmarshalRepositoryID(args.Repository)
	if err != nil {
		return nil, err
	}
	repo, err := backend.NewRepos(r.logger, r.db, r.gitserverClient).Get(ctx, repoID)
	if err != nil {
		return nil, err
	}

	var result checkMirrorRepositoryConnectionResult
	if err := r.gitserverClient.IsRepoCloneable(ctx, repo.ID); err != nil {
		result.errorMessage = err.Error()
	}
	return &result, nil
}

type checkMirrorRepositoryConnectionResult struct {
	errorMessage string
}

func (r *checkMirrorRepositoryConnectionResult) Error() *string {
	if r.errorMessage == "" {
		return nil
	}
	return &r.errorMessage
}

func (r *schemaResolver) UpdateMirrorRepository(ctx context.Context, args *struct {
	Repository graphql.ID
}) (*EmptyResponse, error) {
	// 🚨 SECURITY: There is no reason why non-site-admins would need to run this operation.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	repo, err := r.repositoryByID(ctx, args.Repository)
	if err != nil {
		return nil, err
	}

	if _, err := repoupdater.DefaultClient.EnqueueRepoUpdate(ctx, repo.RepoName()); err != nil {
		return nil, err
	}
	return &EmptyResponse{}, nil
}
