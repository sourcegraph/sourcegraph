package graphql

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav"
	resolverstubs "github.com/sourcegraph/sourcegraph/internal/codeintel/resolvers"
	sharedresolvers "github.com/sourcegraph/sourcegraph/internal/codeintel/shared/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/resolvers/gitresolvers"
	uploadsgraphql "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/transport/graphql"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type rootResolver struct {
	svc                            CodeNavService
	autoindexingSvc                AutoIndexingService
	gitserverClient                gitserver.Client
	siteAdminChecker               sharedresolvers.SiteAdminChecker
	repoStore                      database.RepoStore
	uploadLoaderFactory            uploadsgraphql.UploadLoaderFactory
	indexLoaderFactory             uploadsgraphql.IndexLoaderFactory
	locationResolverFactory        *gitresolvers.CachedLocationResolverFactory
	hunkCache                      codenav.HunkCache
	indexResolverFactory           *uploadsgraphql.PreciseIndexResolverFactory
	maximumIndexesPerMonikerSearch int
	operations                     *operations
}

func NewRootResolver(
	observationCtx *observation.Context,
	svc CodeNavService,
	autoindexingSvc AutoIndexingService,
	gitserverClient gitserver.Client,
	siteAdminChecker sharedresolvers.SiteAdminChecker,
	repoStore database.RepoStore,
	uploadLoaderFactory uploadsgraphql.UploadLoaderFactory,
	indexLoaderFactory uploadsgraphql.IndexLoaderFactory,
	indexResolverFactory *uploadsgraphql.PreciseIndexResolverFactory,
	locationResolverFactory *gitresolvers.CachedLocationResolverFactory,
	maxIndexSearch int,
	hunkCacheSize int,
) (resolverstubs.CodeNavServiceResolver, error) {
	hunkCache, err := codenav.NewHunkCache(hunkCacheSize)
	if err != nil {
		return nil, err
	}

	return &rootResolver{
		svc:                            svc,
		autoindexingSvc:                autoindexingSvc,
		gitserverClient:                gitserverClient,
		siteAdminChecker:               siteAdminChecker,
		repoStore:                      repoStore,
		uploadLoaderFactory:            uploadLoaderFactory,
		indexLoaderFactory:             indexLoaderFactory,
		indexResolverFactory:           indexResolverFactory,
		locationResolverFactory:        locationResolverFactory,
		hunkCache:                      hunkCache,
		maximumIndexesPerMonikerSearch: maxIndexSearch,
		operations:                     newOperations(observationCtx),
	}, nil
}

// 🚨 SECURITY: dbstore layer handles authz for query resolution
func (r *rootResolver) GitBlobLSIFData(ctx context.Context, args *resolverstubs.GitBlobLSIFDataArgs) (_ resolverstubs.GitBlobLSIFDataResolver, err error) {
	ctx, _, endObservation := r.operations.gitBlobLsifData.WithErrors(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repoID", int(args.Repo.ID)),
		args.Commit.Attr(),
		attribute.String("path", args.Path),
		attribute.Bool("exactPath", args.ExactPath),
		attribute.String("toolName", args.ToolName),
	}})
	endObservation.OnCancel(ctx, 1, observation.Args{})

	uploads, err := r.svc.GetClosestCompletedUploadsForBlob(ctx, int(args.Repo.ID), string(args.Commit), args.Path, args.ExactPath, args.ToolName)
	if err != nil || len(uploads) == 0 {
		return nil, err
	}

	if len(uploads) == 0 {
		// If we're on sourcegraph.com and it's a rust package repo, index it on-demand
		if dotcom.SourcegraphDotComMode() && strings.HasPrefix(string(args.Repo.Name), "crates/") {
			err = r.autoindexingSvc.QueueRepoRev(ctx, int(args.Repo.ID), string(args.Commit))
		}

		return nil, err
	}

	reqState := codenav.NewRequestState(
		uploads,
		r.repoStore,
		authz.DefaultSubRepoPermsChecker,
		r.gitserverClient,
		args.Repo,
		string(args.Commit),
		args.Path,
		r.maximumIndexesPerMonikerSearch,
		r.hunkCache,
	)

	return newGitBlobLSIFDataResolver(
		r.svc,
		r.indexResolverFactory,
		reqState,
		r.uploadLoaderFactory.Create(),
		r.indexLoaderFactory.Create(),
		r.locationResolverFactory.Create(),
		r.operations,
	), nil
}

// gitBlobLSIFDataResolver is the main interface to bundle-related operations exposed to the GraphQL API. This
// resolver concerns itself with GraphQL/API-specific behaviors (auth, validation, marshaling, etc.).
// All code intel-specific behavior is delegated to the underlying resolver instance, which is defined
// in the parent package.
type gitBlobLSIFDataResolver struct {
	codeNavSvc           CodeNavService
	indexResolverFactory *uploadsgraphql.PreciseIndexResolverFactory
	requestState         codenav.RequestState
	uploadLoader         uploadsgraphql.UploadLoader
	indexLoader          uploadsgraphql.IndexLoader
	locationResolver     *gitresolvers.CachedLocationResolver
	operations           *operations
}

// NewQueryResolver creates a new QueryResolver with the given resolver that defines all code intel-specific
// behavior. A cached location resolver instance is also given to the query resolver, which should be used
// to resolve all location-related values.
func newGitBlobLSIFDataResolver(
	codeNavSvc CodeNavService,
	indexResolverFactory *uploadsgraphql.PreciseIndexResolverFactory,
	requestState codenav.RequestState,
	uploadLoader uploadsgraphql.UploadLoader,
	indexLoader uploadsgraphql.IndexLoader,
	locationResolver *gitresolvers.CachedLocationResolver,
	operations *operations,
) resolverstubs.GitBlobLSIFDataResolver {
	return &gitBlobLSIFDataResolver{
		codeNavSvc:           codeNavSvc,
		uploadLoader:         uploadLoader,
		indexLoader:          indexLoader,
		indexResolverFactory: indexResolverFactory,
		requestState:         requestState,
		locationResolver:     locationResolver,
		operations:           operations,
	}
}

func (r *gitBlobLSIFDataResolver) ToGitTreeLSIFData() (resolverstubs.GitTreeLSIFDataResolver, bool) {
	return r, true
}

func (r *gitBlobLSIFDataResolver) ToGitBlobLSIFData() (resolverstubs.GitBlobLSIFDataResolver, bool) {
	return r, true
}

func (r *gitBlobLSIFDataResolver) VisibleIndexes(ctx context.Context) (_ *[]resolverstubs.PreciseIndexResolver, err error) {
	ctx, traceErrs, endObservation := r.operations.visibleIndexes.WithErrors(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repoID", r.requestState.RepositoryID),
		attribute.String("commit", r.requestState.Commit),
		attribute.String("path", r.requestState.Path),
	}})
	defer endObservation(1, observation.Args{})

	visibleUploads, err := r.codeNavSvc.VisibleUploadsForPath(ctx, r.requestState)
	if err != nil {
		return nil, err
	}

	resolvers := make([]resolverstubs.PreciseIndexResolver, 0, len(visibleUploads))
	for _, u := range visibleUploads {
		upload := u.ConvertToUpload()
		resolver, err := r.indexResolverFactory.Create(
			ctx,
			r.uploadLoader,
			r.indexLoader,
			r.locationResolver,
			traceErrs,
			&upload,
			nil,
		)
		if err != nil {
			return nil, err
		}
		resolvers = append(resolvers, resolver)
	}

	return &resolvers, nil
}
