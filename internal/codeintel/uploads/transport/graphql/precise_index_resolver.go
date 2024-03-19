package graphql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"

	"github.com/sourcegraph/sourcegraph/internal/api"
	policiesshared "github.com/sourcegraph/sourcegraph/internal/codeintel/policies/shared"
	policiesgraphql "github.com/sourcegraph/sourcegraph/internal/codeintel/policies/transport/graphql"
	resolverstubs "github.com/sourcegraph/sourcegraph/internal/codeintel/resolvers"
	sharedresolvers "github.com/sourcegraph/sourcegraph/internal/codeintel/shared/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/resolvers/gitresolvers"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type preciseIndexResolver struct {
	uploadsSvc       UploadsService
	policySvc        PolicyService
	gitserverClient  gitserver.Client
	siteAdminChecker sharedresolvers.SiteAdminChecker
	repoStore        database.RepoStore
	locationResolver *gitresolvers.CachedLocationResolver
	traceErrs        *observation.ErrCollector
	upload           *shared.Upload
	index            *uploadsshared.Index
}

func newPreciseIndexResolver(
	ctx context.Context,
	uploadsSvc UploadsService,
	policySvc PolicyService,
	gitserverClient gitserver.Client,
	uploadLoader UploadLoader,
	indexLoader IndexLoader,
	siteAdminChecker sharedresolvers.SiteAdminChecker,
	repoStore database.RepoStore,
	locationResolver *gitresolvers.CachedLocationResolver,
	traceErrs *observation.ErrCollector,
	upload *shared.Upload,
	index *uploadsshared.Index,
) (resolverstubs.PreciseIndexResolver, error) {
	if index != nil && index.AssociatedUploadID != nil && upload == nil {
		v, ok, err := uploadLoader.GetByID(ctx, *index.AssociatedUploadID)
		if err != nil {
			return nil, err
		}
		if ok {
			upload = &v
		}
	}

	if upload != nil {
		if upload.AssociatedIndexID != nil {
			v, ok, err := indexLoader.GetByID(ctx, *upload.AssociatedIndexID)
			if err != nil {
				return nil, err
			}
			if ok {
				index = &v
			}
		}
	}

	return &preciseIndexResolver{
		uploadsSvc:       uploadsSvc,
		policySvc:        policySvc,
		gitserverClient:  gitserverClient,
		siteAdminChecker: siteAdminChecker,
		repoStore:        repoStore,
		locationResolver: locationResolver,
		traceErrs:        traceErrs,
		upload:           upload,
		index:            index,
	}, nil
}

//
//
//

func (r *preciseIndexResolver) ID() graphql.ID {
	var parts []string
	if r.upload != nil {
		parts = append(parts, fmt.Sprintf("U:%d", r.upload.ID))
	}
	if r.index != nil {
		parts = append(parts, fmt.Sprintf("I:%d", r.index.ID))
	}

	return relay.MarshalID("PreciseIndex", strings.Join(parts, ":"))
}

//
//
//
//

func (r *preciseIndexResolver) IsLatestForRepo() bool {
	return r.upload != nil && r.upload.VisibleAtTip
}

func (r *preciseIndexResolver) QueuedAt() *gqlutil.DateTime {
	if r.index != nil {
		return gqlutil.DateTimeOrNil(&r.index.QueuedAt)
	}

	return nil
}

func (r *preciseIndexResolver) UploadedAt() *gqlutil.DateTime {
	if r.upload != nil {
		return gqlutil.DateTimeOrNil(&r.upload.UploadedAt)
	}

	return nil
}

func (r *preciseIndexResolver) IndexingStartedAt() *gqlutil.DateTime {
	if r.index != nil {
		return gqlutil.DateTimeOrNil(r.index.StartedAt)
	}

	return nil
}

func (r *preciseIndexResolver) ProcessingStartedAt() *gqlutil.DateTime {
	if r.upload != nil {
		return gqlutil.DateTimeOrNil(r.upload.StartedAt)
	}

	return nil
}

func (r *preciseIndexResolver) IndexingFinishedAt() *gqlutil.DateTime {
	if r.index != nil {
		return gqlutil.DateTimeOrNil(r.index.FinishedAt)
	}

	return nil
}

func (r *preciseIndexResolver) ProcessingFinishedAt() *gqlutil.DateTime {
	if r.upload != nil {
		return gqlutil.DateTimeOrNil(r.upload.FinishedAt)
	}

	return nil
}

func (r *preciseIndexResolver) Steps() resolverstubs.IndexStepsResolver {
	if r.index != nil {
		return NewIndexStepsResolver(r.siteAdminChecker, *r.index)
	}

	return nil
}

//
//
//
//

func (r *preciseIndexResolver) InputCommit() string {
	if r.upload != nil {
		return r.upload.Commit
	} else if r.index != nil {
		return r.index.Commit
	}

	return ""
}

func (r *preciseIndexResolver) InputRoot() string {
	if r.upload != nil {
		return r.upload.Root
	} else if r.index != nil {
		return r.index.Root
	}

	return ""
}

func (r *preciseIndexResolver) InputIndexer() string {
	if r.upload != nil {
		return r.upload.Indexer
	} else if r.index != nil {
		return r.index.Indexer
	}

	return ""
}

func (r *preciseIndexResolver) Failure() *string {
	if r.upload != nil && r.upload.FailureMessage != nil {
		return r.upload.FailureMessage
	} else if r.index != nil && r.index.FailureMessage != nil {
		return r.index.FailureMessage
	}

	return nil
}

func (r *preciseIndexResolver) PlaceInQueue() *int32 {
	if r.index != nil && r.index.Rank != nil {
		v := int32(*r.index.Rank)
		return &v
	} else if r.upload != nil && r.upload.Rank != nil {
		v := int32(*r.upload.Rank)
		return &v
	}

	return nil
}

func (r *preciseIndexResolver) Indexer() resolverstubs.CodeIntelIndexerResolver {
	if r.index != nil {
		// Note: check index as index fields may contain docker shas
		return NewCodeIntelIndexerResolver(r.index.Indexer, r.index.Indexer)
	} else if r.upload != nil {
		return NewCodeIntelIndexerResolver(r.upload.Indexer, "")
	}

	return nil
}

func (r *preciseIndexResolver) ShouldReindex(ctx context.Context) bool {
	if r.upload != nil {
		// non-nil upload - this and any index record must both be marked
		return r.upload.ShouldReindex && (r.index == nil || r.index.ShouldReindex)
	}

	// nil upload - an index record must be marked
	return r.index != nil && r.index.ShouldReindex
}

// State returns values matching up with PreciseIndexState
// in the GraphQL API.
func (r *preciseIndexResolver) State() string {
	if r.upload != nil {
		switch shared.UploadState(strings.ToLower(r.upload.State)) {
		case shared.StateUploading:
			return "UPLOADING_INDEX"

		case shared.StateQueued:
			return "QUEUED_FOR_PROCESSING"

		case shared.StateProcessing:
			return "PROCESSING"

		case shared.StateFailed:
			fallthrough
		case shared.StateErrored:
			return "PROCESSING_ERRORED"

		case shared.StateCompleted:
			return "COMPLETED"

		case shared.StateDeleting:
			return "DELETING"

		case shared.StateDeleted:
			return "DELETED"

		default:
			panic(fmt.Sprintf("unrecognized upload state %q", r.upload.State))
		}
	}

	switch shared.IndexState(strings.ToLower(r.index.State)) {
	case shared.IndexStateQueued:
		return "QUEUED_FOR_INDEXING"

	case shared.IndexStateProcessing:
		return "INDEXING"

	case shared.IndexStateFailed:
		fallthrough
	case shared.IndexStateErrored:
		return "INDEXING_ERRORED"

	case shared.IndexStateCompleted:
		// Should not actually occur in practice (where did upload go?)
		return "INDEXING_COMPLETED"

	default:
		panic(fmt.Sprintf("unrecognized index state %q", r.index.State))
	}
}

//
//
//
//

func (r *preciseIndexResolver) ProjectRoot(ctx context.Context) (_ resolverstubs.GitTreeEntryResolver, err error) {
	repoID, commit, root := r.projectRootMetadata()
	resolver, err := r.locationResolver.Path(ctx, repoID, commit, root, true)
	if err != nil || resolver == nil {
		// Do not return typed nil interface
		return nil, err
	}

	return resolver, nil
}

func (r *preciseIndexResolver) Tags(ctx context.Context) ([]string, error) {
	repoID, commit, _ := r.projectRootMetadata()
	resolver, err := r.locationResolver.Commit(ctx, repoID, commit)
	if err != nil || resolver == nil {
		return nil, err
	}

	return resolver.Tags(ctx)
}

func (r *preciseIndexResolver) projectRootMetadata() (
	repoID api.RepoID,
	commit string,
	root string,
) {
	if r.upload != nil {
		return api.RepoID(r.upload.RepositoryID), r.upload.Commit, r.upload.Root
	}

	return api.RepoID(r.index.RepositoryID), r.index.Commit, r.index.Root
}

//
//

var DefaultRetentionPolicyMatchesPageSize = 50

func (r *preciseIndexResolver) RetentionPolicyOverview(ctx context.Context, args *resolverstubs.LSIFUploadRetentionPolicyMatchesArgs) (resolverstubs.CodeIntelligenceRetentionPolicyMatchesConnectionResolver, error) {
	if r.upload == nil {
		return nil, nil
	}

	var afterID int64
	if args.After != nil {
		var err error
		afterID, err = resolverstubs.UnmarshalID[int64](graphql.ID(*args.After))
		if err != nil {
			return nil, err
		}
	}

	pageSize := DefaultRetentionPolicyMatchesPageSize
	if args.First != nil {
		pageSize = int(*args.First)
	}

	var term string
	if args.Query != nil {
		term = *args.Query
	}

	matches, totalCount, err := r.policySvc.GetRetentionPolicyOverview(ctx, *r.upload, args.MatchesOnly, pageSize, afterID, term, time.Now())
	if err != nil {
		return nil, err
	}

	resolvers := make([]resolverstubs.CodeIntelligenceRetentionPolicyMatchResolver, 0, len(matches))
	for _, policy := range matches {
		resolvers = append(resolvers, newRetentionPolicyMatcherResolver(r.repoStore, policy))
	}

	return resolverstubs.NewTotalCountConnectionResolver(resolvers, 0, int32(totalCount)), nil
}

func (r *preciseIndexResolver) AuditLogs(ctx context.Context) (*[]resolverstubs.LSIFUploadsAuditLogsResolver, error) {
	if r.upload == nil {
		return nil, nil
	}

	logs, err := r.uploadsSvc.GetAuditLogsForUpload(ctx, r.upload.ID)
	if err != nil {
		return nil, err
	}

	resolvers := make([]resolverstubs.LSIFUploadsAuditLogsResolver, 0, len(logs))
	for _, uploadLog := range logs {
		resolvers = append(resolvers, newLSIFUploadsAuditLogsResolver(uploadLog))
	}

	return &resolvers, nil
}

//
//

type retentionPolicyMatcherResolver struct {
	repoStore    database.RepoStore
	policy       policiesshared.RetentionPolicyMatchCandidate
	errCollector *observation.ErrCollector
}

func newRetentionPolicyMatcherResolver(repoStore database.RepoStore, policy policiesshared.RetentionPolicyMatchCandidate) resolverstubs.CodeIntelligenceRetentionPolicyMatchResolver {
	return &retentionPolicyMatcherResolver{repoStore: repoStore, policy: policy}
}

func (r *retentionPolicyMatcherResolver) ConfigurationPolicy() resolverstubs.CodeIntelligenceConfigurationPolicyResolver {
	if r.policy.ConfigurationPolicy == nil {
		return nil
	}

	return policiesgraphql.NewConfigurationPolicyResolver(r.repoStore, *r.policy.ConfigurationPolicy, r.errCollector)
}

func (r *retentionPolicyMatcherResolver) Matches() bool {
	return r.policy.Matched
}

func (r *retentionPolicyMatcherResolver) ProtectingCommits() *[]string {
	return &r.policy.ProtectingCommits
}

//
//

type lsifUploadsAuditLogResolver struct {
	log shared.UploadLog
}

func newLSIFUploadsAuditLogsResolver(log shared.UploadLog) resolverstubs.LSIFUploadsAuditLogsResolver {
	return &lsifUploadsAuditLogResolver{log: log}
}

func (r *lsifUploadsAuditLogResolver) Reason() *string { return r.log.Reason }

func (r *lsifUploadsAuditLogResolver) ChangedColumns() (values []resolverstubs.AuditLogColumnChange) {
	for _, transition := range r.log.TransitionColumns {
		values = append(values, newAuditLogColumnChangeResolver(transition))
	}

	return values
}

func (r *lsifUploadsAuditLogResolver) LogTimestamp() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.log.LogTimestamp}
}

func (r *lsifUploadsAuditLogResolver) UploadDeletedAt() *gqlutil.DateTime {
	return gqlutil.DateTimeOrNil(r.log.RecordDeletedAt)
}

func (r *lsifUploadsAuditLogResolver) UploadID() graphql.ID {
	return resolverstubs.MarshalID("LSIFUpload", r.log.UploadID)
}
func (r *lsifUploadsAuditLogResolver) InputCommit() string  { return r.log.Commit }
func (r *lsifUploadsAuditLogResolver) InputRoot() string    { return r.log.Root }
func (r *lsifUploadsAuditLogResolver) InputIndexer() string { return r.log.Indexer }
func (r *lsifUploadsAuditLogResolver) UploadedAt() gqlutil.DateTime {
	return gqlutil.DateTime{Time: r.log.UploadedAt}
}

func (r *lsifUploadsAuditLogResolver) Operation() string {
	return strings.ToUpper(r.log.Operation)
}

//
//

type auditLogColumnChangeResolver struct {
	columnTransition map[string]*string
}

func newAuditLogColumnChangeResolver(columnTransition map[string]*string) resolverstubs.AuditLogColumnChange {
	return &auditLogColumnChangeResolver{columnTransition}
}

func (r *auditLogColumnChangeResolver) Column() string {
	return *r.columnTransition["column"]
}

func (r *auditLogColumnChangeResolver) Old() *string {
	return r.columnTransition["old"]
}

func (r *auditLogColumnChangeResolver) New() *string {
	return r.columnTransition["new"]
}
