package graphql

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	orderedmap "github.com/wk8/go-ordered-map/v2"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/scip/bindings/go/scip"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/core"
	resolverstubs "github.com/sourcegraph/sourcegraph/internal/codeintel/resolvers"
	sharedresolvers "github.com/sourcegraph/sourcegraph/internal/codeintel/shared/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/resolvers/gitresolvers"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	uploadsgraphql "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/transport/graphql"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
	sgtypes "github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var ErrNotEnabled = errors.New("experimentalFeatures.scipBasedAPIs is not enabled")

type rootResolver struct {
	svc                            CodeNavService
	autoindexingSvc                AutoIndexingService
	gitserverClient                gitserver.Client
	siteAdminChecker               sharedresolvers.SiteAdminChecker
	repoStore                      database.RepoStore
	uploadLoaderFactory            uploadsgraphql.UploadLoaderFactory
	autoIndexJobLoaderFactory      uploadsgraphql.AutoIndexJobLoaderFactory
	locationResolverFactory        *gitresolvers.CachedLocationResolverFactory
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
	autoIndexJobLoaderFactory uploadsgraphql.AutoIndexJobLoaderFactory,
	indexResolverFactory *uploadsgraphql.PreciseIndexResolverFactory,
	locationResolverFactory *gitresolvers.CachedLocationResolverFactory,
	maxIndexSearch int,
) resolverstubs.CodeNavServiceResolver {
	return &rootResolver{
		svc:                            svc,
		autoindexingSvc:                autoindexingSvc,
		gitserverClient:                gitserverClient,
		siteAdminChecker:               siteAdminChecker,
		repoStore:                      repoStore,
		uploadLoaderFactory:            uploadLoaderFactory,
		autoIndexJobLoaderFactory:      autoIndexJobLoaderFactory,
		indexResolverFactory:           indexResolverFactory,
		locationResolverFactory:        locationResolverFactory,
		maximumIndexesPerMonikerSearch: maxIndexSearch,
		operations:                     newOperations(observationCtx),
	}
}

// 🚨 SECURITY: dbstore layer handles authz for query resolution
func (r *rootResolver) GitBlobLSIFData(ctx context.Context, args *resolverstubs.GitBlobLSIFDataArgs) (_ resolverstubs.GitBlobLSIFDataResolver, err error) {
	opts := args.Options()
	ctx, _, endObservation := r.operations.gitBlobLsifData.WithErrors(ctx, &err, observation.Args{Attrs: opts.Attrs()})
	endObservation.OnCancel(ctx, 1, observation.Args{})

	optReqState, err := r.makeRequestState(ctx, args.Repo, opts)
	reqState, ok := optReqState.Get()
	if err != nil || !ok {
		return
	}
	return newGitBlobLSIFDataResolver(
		r.svc,
		r.indexResolverFactory,
		reqState,
		r.uploadLoaderFactory.Create(),
		r.autoIndexJobLoaderFactory.Create(),
		r.locationResolverFactory.Create(),
		r.operations,
	), nil
}

// makeRequestState returns (None, nil) if no uploads exist for the blob
func (r *rootResolver) makeRequestState(ctx context.Context, repo *types.Repo, opts shared.UploadMatchingOptions) (core.Option[codenav.RequestState], error) {
	uploads, err := r.svc.GetClosestCompletedUploadsForBlob(ctx, opts)
	if err != nil || len(uploads) == 0 {
		return core.None[codenav.RequestState](), err
	}
	reqState := codenav.NewRequestState(
		uploads,
		r.repoStore,
		authz.DefaultSubRepoPermsChecker,
		r.gitserverClient,
		repo,
		opts.Commit,
		opts.Path,
		r.maximumIndexesPerMonikerSearch,
	)
	return core.Some(reqState), nil
}

func (r *rootResolver) CodeGraphData(ctx context.Context, opts *resolverstubs.CodeGraphDataOpts) (_ *[]resolverstubs.CodeGraphDataResolver, err error) {
	ctx, _, endObservation := r.operations.codeGraphData.WithErrors(ctx, &err, observation.Args{Attrs: opts.Attrs()})
	endObservation.OnCancel(ctx, 1, observation.Args{})

	if !conf.SCIPBasedAPIsEnabled() {
		return nil, ErrNotEnabled
	}

	currentActor := actor.FromContext(ctx)
	hasAccess, err := authz.FilterActorPath(ctx, authz.DefaultSubRepoPermsChecker,
		currentActor, opts.Repo.Name, opts.Path.RawValue())
	if err != nil {
		return nil, err
	} else if !hasAccess {
		return nil, os.ErrNotExist
	}

	gitTreeTranslator := r.MakeGitTreeTranslator(opts.Repo)
	makeResolvers := func(prov codenav.CodeGraphDataProvenance) ([]resolverstubs.CodeGraphDataResolver, error) {
		indexer := ""
		if prov == codenav.ProvenanceSyntactic {
			indexer = shared.SyntacticIndexer
		}
		uploads, err := r.svc.GetClosestCompletedUploadsForBlob(ctx, shared.UploadMatchingOptions{
			RepositoryID:       opts.Repo.ID,
			Commit:             opts.Commit,
			Path:               opts.Path,
			RootToPathMatching: shared.RootMustEnclosePath,
			Indexer:            indexer,
		})
		if err != nil || len(uploads) == 0 {
			return nil, err
		}
		resolvers := []resolverstubs.CodeGraphDataResolver{}
		filteredUploads := preferUploadsWithLongestRoots(uploads)

		for _, upload := range filteredUploads {
			resolvers = append(resolvers, newCodeGraphDataResolver(r.svc, gitTreeTranslator, upload, opts, prov, r.operations))
		}
		return resolvers, nil
	}

	provs := opts.Args.ProvenancesForSCIPData()
	if provs.Precise {
		preciseResolvers, err := makeResolvers(codenav.ProvenancePrecise)
		if len(preciseResolvers) != 0 || err != nil {
			return &preciseResolvers, err
		}
	}

	if provs.Syntactic {
		syntacticResolvers, err := makeResolvers(codenav.ProvenanceSyntactic)
		if len(syntacticResolvers) != 0 || err != nil {
			return &syntacticResolvers, err
		}

		// Enhancement idea: if a syntactic SCIP index is unavailable,
		// but the language is supported by scip-syntax, we could generate
		// a syntactic SCIP index on-the-fly by having the syntax-highlighter
		// analyze the file.
	}

	// We do not currently have any way of generating SCIP data
	// during purely textual means.

	return &[]resolverstubs.CodeGraphDataResolver{}, nil
}

func (r *rootResolver) CodeGraphDataByID(ctx context.Context, rawID graphql.ID) (resolverstubs.CodeGraphDataResolver, error) {
	var id CodeGraphDataID
	if err := relay.UnmarshalSpec(rawID, &id); err != nil {
		return nil, errors.Wrap(err, "malformed ID")
	}
	repo, err := r.repoStore.Get(ctx, id.RepoID)
	if err != nil {
		return nil, err
	}
	opts := resolverstubs.CodeGraphDataOpts{
		Args:   id.Args,
		Repo:   repo,
		Commit: id.Commit,
		Path:   core.NewRepoRelPathUnchecked(id.Path),
	}
	return &codeGraphDataResolver{
		sync.Once{},
		/*document*/ nil,
		/*documentRetrievalError*/ nil,
		r.svc,
		r.MakeGitTreeTranslator(repo),
		id.UploadData,
		&opts,
		id.CodeGraphDataProvenance,
		r.operations,
	}, nil
}

func preferUploadsWithLongestRoots(uploads []shared.CompletedUpload) []shared.CompletedUpload {
	// Use orderedmap instead of a map to preserve the order of the uploads
	// and to avoid introducing non-determinism.
	sortedMap := orderedmap.New[string, shared.CompletedUpload]()
	for _, upload := range uploads {
		key := fmt.Sprintf("%s:%s", upload.Indexer, upload.Commit)
		if val, found := sortedMap.Get(key); found {
			if len(val.Root) < len(upload.Root) {
				sortedMap.Set(key, upload)
			}
		} else {
			sortedMap.Set(key, upload)
		}
	}
	out := make([]shared.CompletedUpload, 0, sortedMap.Len())
	for pair := sortedMap.Oldest(); pair != nil; pair = pair.Next() {
		out = append(out, pair.Value)
	}
	return out
}

func (r *rootResolver) UsagesForSymbol(ctx context.Context, unresolvedArgs *resolverstubs.UsagesForSymbolArgs) (_ resolverstubs.UsageConnectionResolver, err error) {
	ctx, trace, endObservation := r.operations.usagesForSymbol.With(ctx, &err, observation.Args{Attrs: unresolvedArgs.Attrs()})

	numPreciseResults := 0
	numSyntacticResults := 0
	numSearchBasedResults := 0
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("results.precise", numPreciseResults),
			attribute.Int("results.syntactic", numSyntacticResults),
			attribute.Int("results.searchBased", numSearchBasedResults),
		}})
	}()

	if !conf.SCIPBasedAPIsEnabled() {
		return nil, ErrNotEnabled
	}

	const maxUsagesCount = 100
	args, err := unresolvedArgs.Resolve(ctx, r.repoStore, r.gitserverClient, maxUsagesCount)
	if err != nil {
		return nil, err
	}

	trace = trace.WithFields(
		log.Int("repo.id", int(args.Repo.ID)),
		log.String("repo.name", string(args.Repo.Name)),
		log.String("commitID", string(args.CommitID)),
		log.String("path", args.Path.RawValue()),
		log.String("range", args.Range.String()))

	// TODO: We should make precise use this translator
	gitTreeTranslator := r.MakeGitTreeTranslator(&args.Repo)
	remainingCount := int(args.RemainingCount)
	provsForSCIPData := args.Symbol.ProvenancesForSCIPData()
	usageResolvers := []resolverstubs.UsageResolver{}

	cursor := args.Cursor
	if cursor.IsPrecise() {
		nextPreciseCursor, preciseUsageResolvers := r.preciseUsages(ctx, trace, args, remainingCount)
		usageResolvers = append(usageResolvers, preciseUsageResolvers...)
		numPreciseResults = len(preciseUsageResolvers)
		remainingCount -= min(remainingCount, numPreciseResults)
		cursor = cursor.AdvanceCursor(nextPreciseCursor, provsForSCIPData)
	}

	previousSyntacticSearch := core.None[codenav.PreviousSyntacticSearch]()
	if cursor.IsSyntactic() && remainingCount > 0 {
		usagesForSymbolArgs := codenav.UsagesForSymbolArgs{
			Repo:        args.Repo,
			Commit:      args.CommitID,
			Path:        args.Path,
			SymbolRange: args.Range,
		}
		nextSyntacticCursor, syntacticUsageResolvers, prevSearch := r.syntacticUsages(ctx, trace, gitTreeTranslator, usagesForSymbolArgs)
		previousSyntacticSearch = prevSearch
		usageResolvers = append(usageResolvers, syntacticUsageResolvers...)
		numSyntacticResults = len(syntacticUsageResolvers)
		remainingCount -= min(remainingCount, numSyntacticResults)
		cursor = cursor.AdvanceCursor(nextSyntacticCursor, provsForSCIPData)
	}

	if cursor.IsSearchBased() && remainingCount > 0 {
		usagesForSymbolArgs := codenav.UsagesForSymbolArgs{
			Repo:        args.Repo,
			Commit:      args.CommitID,
			Path:        args.Path,
			SymbolRange: args.Range,
		}
		var syntacticFilter codenav.SearchBasedSyntacticFilter
		if provsForSCIPData.Syntactic {
			syntacticFilter = codenav.NewSyntacticFilter(previousSyntacticSearch)
		} else {
			syntacticFilter = codenav.NoSyntacticFilter()
		}
		nextSearchBasedCursor, searchBasedUsageResolvers := r.searchBasedUsages(
			ctx, trace, gitTreeTranslator, usagesForSymbolArgs, syntacticFilter,
		)
		usageResolvers = append(usageResolvers, searchBasedUsageResolvers...)
		numSearchBasedResults = len(searchBasedUsageResolvers)
		cursor = cursor.AdvanceCursor(nextSearchBasedCursor, provsForSCIPData)
	}

	pageInfo := resolverstubs.NewSimplePageInfo(false)
	if !cursor.IsDone() {
		if len(usageResolvers) > 0 {
			pageInfo = resolverstubs.NewPageInfoFromCursor(cursor.Encode())
		} else {
			trace.Error("cursor should be done if no usageResolvers were found",
				log.String("UsagesCursor", cursor.Encode()))
		}
	}
	return &usageConnectionResolver{
		nodes:    usageResolvers,
		pageInfo: pageInfo,
	}, nil
}

func (r *rootResolver) preciseUsages(
	ctx context.Context, trace observation.TraceLogger, args codenav.UsagesForSymbolResolvedArgs, remainingCount int,
) (core.Option[codenav.UsagesCursor], []resolverstubs.UsageResolver) {
	optRequestState, err := r.makeRequestState(ctx, &args.Repo, shared.UploadMatchingOptions{
		RepositoryID:       args.Repo.ID,
		Commit:             args.CommitID,
		Path:               args.Path,
		RootToPathMatching: shared.RootMustEnclosePath,
		Indexer:            "", // any precise indexer is OK
	})
	if err != nil {
		trace.Error("failed to construct request state", log.Error(err))
		return core.None[codenav.UsagesCursor](), nil
	}
	requestState, ok := optRequestState.Get()
	if !ok {
		if args.Symbol != nil && args.Symbol.EqualsProvenance == codenav.ProvenancePrecise {
			trace.Warn("expected precise matches for symbol but didn't find any matching uploads",
				log.String("symbol", args.Symbol.EqualsName))
		}
		return core.None[codenav.UsagesCursor](), nil
	}
	preciseUsages, nextCursor, err := r.svc.PreciseUsages(ctx, requestState, args)
	if err != nil {
		trace.Error("CodeNavService.PreciseUsages", log.Error(err))
		return core.None[codenav.UsagesCursor](), nil
	}
	if len(preciseUsages) > remainingCount {
		trace.Warn("number of precise usages exceeded limit",
			log.Int("limit", remainingCount),
			log.Int("numPreciseUsages", len(preciseUsages)))
	}
	usageResolvers, err := NewPreciseUsageResolvers(ctx, r.gitserverClient, preciseUsages)
	if err != nil {
		trace.Warn("errors when constructing precise resolvers", log.Error(err))
	}
	trace.AddEvent("PreciseUsages", attribute.Int("count", len(usageResolvers)))
	return nextCursor, usageResolvers
}

func (r *rootResolver) syntacticUsages(
	ctx context.Context, trace observation.TraceLogger, gitTreeTranslator codenav.GitTreeTranslator, args codenav.UsagesForSymbolArgs,
) (core.Option[codenav.UsagesCursor], []resolverstubs.UsageResolver, core.Option[codenav.PreviousSyntacticSearch]) {
	syntacticResult, err := r.svc.SyntacticUsages(ctx, gitTreeTranslator, args)
	if err != nil {
		switch err.Code {
		case codenav.SU_Fatal:
			trace.Error("CodeNavService.SyntacticUsages", log.String("error", err.Error()))
		case codenav.SU_NoSymbolAtRequestedRange:
		case codenav.SU_NoSyntacticIndex:
		case codenav.SU_FailedToSearch:
		default:
			trace.Info("CodeNavService.SyntacticUsages", log.String("error", err.Error()))
		}
		return core.None[codenav.UsagesCursor](), nil, core.None[codenav.PreviousSyntacticSearch]()
	}
	usageResolvers := make([]resolverstubs.UsageResolver, 0, len(syntacticResult.Matches))
	for _, result := range syntacticResult.Matches {
		usageResolvers = append(usageResolvers, NewSyntacticUsageResolver(result, args.Repo.Name, args.Commit))
	}
	return syntacticResult.NextCursor, usageResolvers, core.Some(syntacticResult.PreviousSyntacticSearch)
}

func (r *rootResolver) searchBasedUsages(
	ctx context.Context, trace observation.TraceLogger, gitTreeTranslator codenav.GitTreeTranslator,
	args codenav.UsagesForSymbolArgs, syntacticFilter codenav.SearchBasedSyntacticFilter,
) (core.Option[codenav.UsagesCursor], []resolverstubs.UsageResolver) {
	result, err := r.svc.SearchBasedUsages(ctx, gitTreeTranslator, args, syntacticFilter)
	if err != nil {
		trace.Error("CodeNavService.SearchBasedUsages", log.Error(err))
		return core.None[codenav.UsagesCursor](), nil
	}
	usageResolvers := make([]resolverstubs.UsageResolver, 0, len(result.Matches))
	for _, match := range result.Matches {
		usageResolvers = append(usageResolvers, NewSearchBasedUsageResolver(match, args.Repo.Name, args.Commit))
	}
	return result.NextCursor, usageResolvers
}

func (r *rootResolver) MakeGitTreeTranslator(repo *sgtypes.Repo) codenav.GitTreeTranslator {
	return codenav.NewGitTreeTranslator(r.gitserverClient, *repo)
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
	autoIndexJobLoader   uploadsgraphql.AutoIndexJobLoader
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
	autoIndexJobLoader uploadsgraphql.AutoIndexJobLoader,
	locationResolver *gitresolvers.CachedLocationResolver,
	operations *operations,
) resolverstubs.GitBlobLSIFDataResolver {
	return &gitBlobLSIFDataResolver{
		codeNavSvc:           codeNavSvc,
		uploadLoader:         uploadLoader,
		autoIndexJobLoader:   autoIndexJobLoader,
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
	ctx, traceErrs, endObservation := r.operations.visibleIndexes.WithErrors(ctx, &err, observation.Args{Attrs: r.requestState.Attrs()})
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
			r.autoIndexJobLoader,
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

type codeGraphDataResolver struct {
	// Retrieved data/state
	retrievedDocument      sync.Once
	document               *scip.Document
	documentRetrievalError error

	// Arguments
	svc               CodeNavService
	gitTreeTranslator codenav.GitTreeTranslator
	upload            UploadData
	opts              *resolverstubs.CodeGraphDataOpts
	provenance        codenav.CodeGraphDataProvenance

	// O11y
	operations *operations
}

// UploadData represents the subset of information of shared.CompletedUpload
// that we actually care about for the purposes of the GraphQL API.
//
// All fields are left public for JSON marshaling/unmarshaling.
type UploadData struct {
	UploadID       int
	Commit         string
	Root           string
	Indexer        string
	IndexerVersion string
}

func (u UploadData) GetID() int {
	return u.UploadID
}

func (u UploadData) GetRoot() string {
	return u.Root
}

func (u UploadData) GetCommit() api.CommitID {
	return api.CommitID(u.Commit)
}

var _ core.UploadLike = UploadData{}

func NewUploadData(upload shared.CompletedUpload) UploadData {
	return UploadData{
		UploadID:       upload.ID,
		Commit:         upload.Commit,
		Root:           upload.Root,
		Indexer:        upload.Indexer,
		IndexerVersion: upload.IndexerVersion,
	}
}

func newCodeGraphDataResolver(
	svc CodeNavService,
	gitTreeTranslator codenav.GitTreeTranslator,
	upload shared.CompletedUpload,
	opts *resolverstubs.CodeGraphDataOpts,
	provenance codenav.CodeGraphDataProvenance,
	operations *operations,
) resolverstubs.CodeGraphDataResolver {
	return &codeGraphDataResolver{
		sync.Once{},
		/*document*/ nil,
		/*documentRetrievalError*/ nil,
		svc,
		gitTreeTranslator,
		NewUploadData(upload),
		opts,
		provenance,
		operations,
	}
}

// CodeGraphDataID represents the serializable state needed to materialize
// a CodeGraphData value from an opaque GraphQL ID.
//
// All fields are left public for JSON marshaling/unmarshaling.
type CodeGraphDataID struct {
	UploadData
	Args *resolverstubs.CodeGraphDataArgs
	api.RepoID
	Commit api.CommitID
	Path   string
	codenav.CodeGraphDataProvenance
}

func (c *codeGraphDataResolver) tryRetrieveDocument(ctx context.Context) (*scip.Document, error) {
	// NOTE(id: scip-doc-optimization): In the case of pagination, if we retrieve the document ID
	// from the database, we can avoid performing a JOIN between codeintel_scip_document_lookup
	// and codeintel_scip_documents
	c.retrievedDocument.Do(func() {
		c.document, c.documentRetrievalError = c.svc.SCIPDocument(ctx, c.gitTreeTranslator, c.upload, c.opts.Commit, c.opts.Path)
	})
	return c.document, c.documentRetrievalError
}

func (c *codeGraphDataResolver) ID() graphql.ID {
	dataID := CodeGraphDataID{
		c.upload,
		c.opts.Args,
		c.opts.Repo.ID,
		c.opts.Commit,
		c.opts.Path.RawValue(),
		c.provenance,
	}
	return relay.MarshalID(resolverstubs.CodeGraphDataIDKind, dataID)
}

func (c *codeGraphDataResolver) Provenance(_ context.Context) (codenav.CodeGraphDataProvenance, error) {
	return c.provenance, nil
}

func (c *codeGraphDataResolver) Commit(_ context.Context) (string, error) {
	return c.upload.Commit, nil
}

func (c *codeGraphDataResolver) ToolInfo(_ context.Context) (*resolverstubs.CodeGraphToolInfo, error) {
	return &resolverstubs.CodeGraphToolInfo{Name_: &c.upload.Indexer, Version_: &c.upload.IndexerVersion}, nil
}
