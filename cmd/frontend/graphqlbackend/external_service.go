package graphqlbackend

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/repos"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

type externalServiceResolver struct {
	logger          log.Logger
	db              database.DB
	externalService *types.ExternalService
	warning         string

	webhookURLOnce sync.Once
	webhookURL     string
	webhookErr     error
}

type externalServiceAvailabilityStateResolver struct {
	available   *externalServiceAvailable
	unavailable *externalServiceUnavailable
	unknown     *externalServiceUnknown
}

type externalServiceAvailable struct {
	lastCheckedAt time.Time
}

type externalServiceUnavailable struct {
	suspectedReason string
}

type externalServiceUnknown struct{}

const externalServiceIDKind = "ExternalService"

// availabilityCheck indicates which code host types have an availability check implemented. For any
// new code hosts where this check is implemented, add a new entry for the respective kind and set
// the value to true.
var availabilityCheck = map[string]bool{
	extsvc.KindGitHub:          true,
	extsvc.KindGitLab:          true,
	extsvc.KindBitbucketServer: true,
	extsvc.KindBitbucketCloud:  true,
	extsvc.KindAzureDevOps:     true,
	extsvc.KindPerforce:        true,
}

func externalServiceByID(ctx context.Context, db database.DB, gqlID graphql.ID) (*externalServiceResolver, error) {
	// 🚨 SECURITY: check whether user is site-admin
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, db); err != nil {
		return nil, err
	}

	id, err := UnmarshalExternalServiceID(gqlID)
	if err != nil {
		return nil, err
	}

	es, err := db.ExternalServices().GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &externalServiceResolver{logger: log.Scoped("externalServiceResolver"), db: db, externalService: es}, nil
}

func MarshalExternalServiceID(id int64) graphql.ID {
	return relay.MarshalID(externalServiceIDKind, id)
}

func UnmarshalExternalServiceID(id graphql.ID) (externalServiceID int64, err error) {
	if kind := relay.UnmarshalKind(id); kind != externalServiceIDKind {
		err = errors.Errorf("expected graphql ID to have kind %q; got %q", externalServiceIDKind, kind)
		return
	}
	err = relay.UnmarshalSpec(id, &externalServiceID)
	return
}

func TryUnmarshalExternalServiceID(externalServiceID *graphql.ID) (*int64, error) {
	var (
		id  int64
		err error
	)

	if externalServiceID != nil {
		id, err = UnmarshalExternalServiceID(*externalServiceID)
		if err != nil {
			return nil, err
		}
		return &id, nil
	}

	return nil, nil
}

func (r *externalServiceResolver) ID() graphql.ID {
	return MarshalExternalServiceID(r.externalService.ID)
}

func (r *externalServiceResolver) Kind() string {
	return r.externalService.Kind
}

func (r *externalServiceResolver) DisplayName() string {
	return r.externalService.DisplayName
}

func (r *externalServiceResolver) RateLimiterState(ctx context.Context) (*rateLimiterStateResolver, error) {
	info, err := ratelimit.GetGlobalLimiterState(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting rate limiter state")
	}

	state, ok := info[r.externalService.URN()]
	if !ok {
		return nil, nil
	}

	return &rateLimiterStateResolver{state: state}, nil
}

func (r *externalServiceResolver) Config(ctx context.Context) (JSONCString, error) {
	redacted, err := r.externalService.RedactedConfig(ctx)
	if err != nil {
		return "", err
	}
	return JSONCString(redacted), nil
}

func (r *externalServiceResolver) CreatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.externalService.CreatedAt}
}

func (r *externalServiceResolver) UpdatedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.externalService.UpdatedAt}
}

func (r *externalServiceResolver) Creator(ctx context.Context) (*UserResolver, error) {
	if r.externalService.CreatorID == nil {
		return nil, nil
	}

	user, err := r.db.Users().GetByID(ctx, *r.externalService.CreatorID)
	if err != nil {
		if database.IsUserNotFoundErr(err) {
			return nil, nil
		}
		return nil, err
	}

	return NewUserResolver(ctx, r.db, user), nil
}

func (r *externalServiceResolver) LastUpdater(ctx context.Context) (*UserResolver, error) {
	if r.externalService.LastUpdaterID == nil {
		return nil, nil
	}

	user, err := r.db.Users().GetByID(ctx, *r.externalService.LastUpdaterID)
	if err != nil {
		if database.IsUserNotFoundErr(err) {
			return nil, nil
		}
		return nil, err
	}

	return NewUserResolver(ctx, r.db, user), nil
}

func (r *externalServiceResolver) WebhookURL(ctx context.Context) (*string, error) {
	r.webhookURLOnce.Do(func() {
		parsed, err := extsvc.ParseEncryptableConfig(ctx, r.externalService.Kind, r.externalService.Config)
		if err != nil {
			r.webhookErr = errors.Wrap(err, "parsing external service config")
			return
		}
		u, err := extsvc.WebhookURL(r.externalService.Kind, r.externalService.ID, parsed, conf.ExternalURL())
		if err != nil {
			r.webhookErr = errors.Wrap(err, "building webhook URL")
		}
		// If no webhook URL can be built for the kind, we bail out and don't throw an error.
		if u == "" {
			return
		}
		switch c := parsed.(type) {
		case *schema.BitbucketCloudConnection:
			if c.WebhookSecret != "" {
				r.webhookURL = u
			}
		case *schema.BitbucketServerConnection:
			if c.Webhooks != nil {
				r.webhookURL = u
			}
			if c.Plugin != nil && c.Plugin.Webhooks != nil {
				r.webhookURL = u
			}
		case *schema.GitHubConnection:
			if len(c.Webhooks) > 0 {
				r.webhookURL = u
			}
		case *schema.GitLabConnection:
			if len(c.Webhooks) > 0 {
				r.webhookURL = u
			}
		}
	})
	if r.webhookURL == "" {
		return nil, r.webhookErr
	}
	return &r.webhookURL, r.webhookErr
}

func (r *externalServiceResolver) Warning() *string {
	if r.warning == "" {
		return nil
	}
	return &r.warning
}

func (r *externalServiceResolver) LastSyncError(ctx context.Context) (*string, error) {
	latestError, err := r.db.ExternalServices().GetLastSyncError(ctx, r.externalService.ID)
	if err != nil {
		return nil, err
	}
	if latestError == "" {
		return nil, nil
	}
	return &latestError, nil
}

func (r *externalServiceResolver) RepoCount(ctx context.Context) (int32, error) {
	return r.db.ExternalServices().RepoCount(ctx, r.externalService.ID)
}

func (r *externalServiceResolver) LastSyncAt() *gqlutil.DateTime {
	if r.externalService.LastSyncAt.IsZero() {
		return nil
	}
	return &gqlutil.DateTime{Time: r.externalService.LastSyncAt}
}

func (r *externalServiceResolver) NextSyncAt() *gqlutil.DateTime {
	if r.externalService.NextSyncAt.IsZero() {
		return nil
	}
	return &gqlutil.DateTime{Time: r.externalService.NextSyncAt}
}

func (r *externalServiceResolver) WebhookLogs(ctx context.Context, args *WebhookLogsArgs) (*WebhookLogConnectionResolver, error) {
	return NewWebhookLogConnectionResolver(ctx, r.db, args, webhookLogsExternalServiceID(r.externalService.ID))
}

type externalServiceSyncJobsArgs struct {
	First *int32
}

func (r *externalServiceResolver) SyncJobs(args *externalServiceSyncJobsArgs) (*externalServiceSyncJobConnectionResolver, error) {
	return newExternalServiceSyncJobConnectionResolver(r.db, args, r.externalService.ID)
}

// mockCheckConnection mocks (*externalServiceResolver).CheckConnection.
var mockCheckConnection func(context.Context, *externalServiceResolver) (*externalServiceAvailabilityStateResolver, error)

func (r *externalServiceResolver) CheckConnection(ctx context.Context) (*externalServiceAvailabilityStateResolver, error) {
	if mockCheckConnection != nil {
		return mockCheckConnection(ctx, r)
	}

	if !r.HasConnectionCheck() {
		return &externalServiceAvailabilityStateResolver{unknown: &externalServiceUnknown{}}, nil
	}

	source, err := repos.NewSource(
		ctx,
		log.Scoped("externalServiceResolver.CheckConnection"),
		r.db,
		r.externalService,
		httpcli.NewExternalClientFactory(),
		gitserver.NewClient("graphql.check-connection"),
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create source")
	}

	if err := source.CheckConnection(ctx); err != nil {
		return &externalServiceAvailabilityStateResolver{
			unavailable: &externalServiceUnavailable{suspectedReason: err.Error()},
		}, nil
	}

	return &externalServiceAvailabilityStateResolver{
		available: &externalServiceAvailable{
			lastCheckedAt: time.Now(),
		},
	}, nil
}

func (r *externalServiceResolver) HasConnectionCheck() bool {
	return availabilityCheck[r.externalService.Kind]
}

func (r *externalServiceAvailabilityStateResolver) ToExternalServiceAvailable() (*externalServiceAvailabilityStateResolver, bool) {
	return r, r.available != nil
}

func (r *externalServiceAvailabilityStateResolver) ToExternalServiceUnavailable() (*externalServiceAvailabilityStateResolver, bool) {
	return r, r.unavailable != nil
}

func (r *externalServiceAvailabilityStateResolver) ToExternalServiceAvailabilityUnknown() (*externalServiceAvailabilityStateResolver, bool) {
	return r, r.unknown != nil
}

func (r *externalServiceAvailabilityStateResolver) LastCheckedAt() (gqlutil.DateTime, error) {
	return gqlutil.DateTime{Time: r.available.lastCheckedAt}, nil
}

func (r *externalServiceAvailabilityStateResolver) SuspectedReason() (string, error) {
	return r.unavailable.suspectedReason, nil
}

func (r *externalServiceAvailabilityStateResolver) ImplementationNote() string {
	return "not implemented"
}

func (r *externalServiceResolver) SupportsRepoExclusion() bool {
	return r.externalService.SupportsRepoExclusion()
}

func (r *externalServiceResolver) Unrestricted() bool {
	return r.externalService.Unrestricted
}

type externalServiceSyncJobConnectionResolver struct {
	args              *externalServiceSyncJobsArgs
	externalServiceID int64
	db                database.DB

	once       sync.Once
	nodes      []*types.ExternalServiceSyncJob
	totalCount int64
	err        error
}

func newExternalServiceSyncJobConnectionResolver(db database.DB, args *externalServiceSyncJobsArgs, externalServiceID int64) (*externalServiceSyncJobConnectionResolver, error) {
	return &externalServiceSyncJobConnectionResolver{
		args:              args,
		externalServiceID: externalServiceID,
		db:                db,
	}, nil
}

func (r *externalServiceSyncJobConnectionResolver) Nodes(ctx context.Context) ([]*externalServiceSyncJobResolver, error) {
	jobs, _, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}

	nodes := make([]*externalServiceSyncJobResolver, len(jobs))
	for i, j := range jobs {
		nodes[i] = &externalServiceSyncJobResolver{
			job: j,
		}
	}

	return nodes, nil
}

func (r *externalServiceSyncJobConnectionResolver) TotalCount(ctx context.Context) (int32, error) {
	_, totalCount, err := r.compute(ctx)
	return int32(totalCount), err
}

func (r *externalServiceSyncJobConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	jobs, totalCount, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}
	return graphqlutil.HasNextPage(len(jobs) != int(totalCount)), nil
}

func (r *externalServiceSyncJobConnectionResolver) compute(ctx context.Context) ([]*types.ExternalServiceSyncJob, int64, error) {
	r.once.Do(func() {
		opts := database.ExternalServicesGetSyncJobsOptions{
			ExternalServiceID: r.externalServiceID,
		}
		if r.args.First != nil {
			opts.LimitOffset = &database.LimitOffset{
				Limit: int(*r.args.First),
			}
		}
		r.nodes, r.err = r.db.ExternalServices().GetSyncJobs(ctx, opts)
		if r.err != nil {
			return
		}
		r.totalCount, r.err = r.db.ExternalServices().CountSyncJobs(ctx, opts)
	})

	return r.nodes, r.totalCount, r.err
}

type externalServiceSyncJobResolver struct {
	job *types.ExternalServiceSyncJob
}

func marshalExternalServiceSyncJobID(id int64) graphql.ID {
	return relay.MarshalID("ExternalServiceSyncJob", id)
}

func unmarshalExternalServiceSyncJobID(id graphql.ID) (jobID int64, err error) {
	err = relay.UnmarshalSpec(id, &jobID)
	return
}

func externalServiceSyncJobByID(ctx context.Context, db database.DB, gqlID graphql.ID) (Node, error) {
	// Site-admin only for now.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, db); err != nil {
		return nil, err
	}

	id, err := unmarshalExternalServiceSyncJobID(gqlID)
	if err != nil {
		return nil, err
	}

	job, err := db.ExternalServices().GetSyncJobByID(ctx, id)
	if err != nil {
		if errcode.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return &externalServiceSyncJobResolver{job: job}, nil
}

func (r *externalServiceSyncJobResolver) ID() graphql.ID {
	return marshalExternalServiceSyncJobID(r.job.ID)
}

func (r *externalServiceSyncJobResolver) State() string {
	if r.job.Cancel && r.job.State == "processing" {
		return "CANCELING"
	}
	return strings.ToUpper(r.job.State)
}

func (r *externalServiceSyncJobResolver) FailureMessage() *string {
	if r.job.FailureMessage == "" || r.job.Cancel {
		return nil
	}

	return &r.job.FailureMessage
}

func (r *externalServiceSyncJobResolver) QueuedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.job.QueuedAt}
}

func (r *externalServiceSyncJobResolver) StartedAt() *gqlutil.DateTime {
	if r.job.StartedAt.IsZero() {
		return nil
	}

	return &gqlutil.DateTime{Time: r.job.StartedAt}
}

func (r *externalServiceSyncJobResolver) FinishedAt() *gqlutil.DateTime {
	if r.job.FinishedAt.IsZero() {
		return nil
	}

	return &gqlutil.DateTime{Time: r.job.FinishedAt}
}

func (r *externalServiceSyncJobResolver) ReposSynced() int32 { return r.job.ReposSynced }

func (r *externalServiceSyncJobResolver) RepoSyncErrors() int32 { return r.job.RepoSyncErrors }

func (r *externalServiceSyncJobResolver) ReposAdded() int32 { return r.job.ReposAdded }

func (r *externalServiceSyncJobResolver) ReposDeleted() int32 { return r.job.ReposDeleted }

func (r *externalServiceSyncJobResolver) ReposModified() int32 { return r.job.ReposModified }

func (r *externalServiceSyncJobResolver) ReposUnmodified() int32 { return r.job.ReposUnmodified }

func (r *externalServiceNamespaceConnectionResolver) compute(ctx context.Context) ([]*types.ExternalServiceNamespace, int32, error) {
	r.once.Do(func() {
		config, err := NewSourceConfiguration(r.args.Kind, r.args.Url, r.args.Token)
		if err != nil {
			r.err = err
			return
		}

		externalServiceID, err := TryUnmarshalExternalServiceID(r.args.ID)
		if err != nil {
			r.err = err
			return
		}

		e := newExternalServices(log.Scoped("graphql.externalservicenamespaces"), r.db)
		r.nodes, r.err = e.ListNamespaces(ctx, externalServiceID, r.args.Kind, config)
		r.totalCount = int32(len(r.nodes))
	})

	return r.nodes, r.totalCount, r.err
}

func (r *externalServiceNamespaceConnectionResolver) Nodes(ctx context.Context) ([]*externalServiceNamespaceResolver, error) {
	namespaces, totalCount, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}

	nodes := make([]*externalServiceNamespaceResolver, totalCount)
	for i, j := range namespaces {
		nodes[i] = &externalServiceNamespaceResolver{
			namespace: j,
		}
	}

	return nodes, nil
}

func (r *externalServiceNamespaceConnectionResolver) TotalCount(ctx context.Context) (int32, error) {
	_, totalCount, err := r.compute(ctx)
	return totalCount, err
}

type externalServiceNamespaceResolver struct {
	namespace *types.ExternalServiceNamespace
}

func (r *externalServiceNamespaceResolver) ID() graphql.ID {
	return relay.MarshalID("ExternalServiceNamespace", r.namespace)
}

func (r *externalServiceNamespaceResolver) Name() string {
	return r.namespace.Name
}

func (r *externalServiceNamespaceResolver) ExternalID() string {
	return r.namespace.ExternalID
}

func (r *externalServiceRepositoryConnectionResolver) compute(ctx context.Context) ([]*types.ExternalServiceRepository, error) {
	r.once.Do(func() {
		config, err := NewSourceConfiguration(r.args.Kind, r.args.Url, r.args.Token)
		if err != nil {
			r.err = err
			return
		}

		first := int32(100)
		if r.args.First != nil {
			first = *r.args.First
		}

		externalServiceID, err := TryUnmarshalExternalServiceID(r.args.ID)
		if err != nil {
			r.err = err
			return
		}

		e := newExternalServices(log.Scoped("graphql.externalservicerepositories"), r.db)
		r.nodes, r.err = e.DiscoverRepos(ctx, externalServiceID, r.args.Kind, config, first, r.args.Query, r.args.ExcludeRepos)
	})

	return r.nodes, r.err
}

func (r *externalServiceRepositoryConnectionResolver) Nodes(ctx context.Context) ([]*externalServiceRepositoryResolver, error) {
	sourceRepos, err := r.compute(ctx)
	if err != nil {
		return nil, err
	}

	nodes := make([]*externalServiceRepositoryResolver, len(sourceRepos))
	for i, j := range sourceRepos {
		nodes[i] = &externalServiceRepositoryResolver{
			repo: j,
		}
	}

	return nodes, nil
}

type externalServiceRepositoryResolver struct {
	repo *types.ExternalServiceRepository
}

func (r *externalServiceRepositoryResolver) ID() graphql.ID {
	return relay.MarshalID("ExternalServiceRepository", r.repo)
}

func (r *externalServiceRepositoryResolver) Name() string {
	return string(r.repo.Name)
}

func (r *externalServiceRepositoryResolver) ExternalID() string {
	return r.repo.ExternalID
}
