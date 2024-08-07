package codenav

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	genslices "github.com/life4/genesis/slices"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/scip/bindings/go/scip"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/internal/lsifstore"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/core"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	searcher "github.com/sourcegraph/sourcegraph/internal/search/client"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/languages"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Service struct {
	repoStore    minimalRepoStore
	lsifstore    lsifstore.LsifStore
	gitserver    gitserver.Client
	uploadSvc    UploadService
	searchClient searcher.SearchClient
	operations   *operations
	logger       log.Logger
}

// minimalRepoStore covers the subset of database.RepoStore APIs that we need
// for code navigation.
//
// Prefer calling GetReposSetByIDs instead of calling Get in a loop.
type minimalRepoStore interface {
	Get(context.Context, api.RepoID) (*types.Repo, error)
	GetReposSetByIDs(context.Context, ...api.RepoID) (map[api.RepoID]*types.Repo, error)
}

func newService(
	observationCtx *observation.Context,
	repoStore minimalRepoStore,
	lsifstore lsifstore.LsifStore,
	uploadSvc UploadService,
	gitserver gitserver.Client,
	searchClient searcher.SearchClient,
	logger log.Logger,
) *Service {
	return &Service{
		repoStore:    repoStore,
		lsifstore:    lsifstore,
		gitserver:    gitserver,
		uploadSvc:    uploadSvc,
		searchClient: searchClient,
		operations:   newOperations(observationCtx),
		logger:       logger,
	}
}

// GetHover returns the set of locations defining the symbol at the given position.
func (s *Service) GetHover(ctx context.Context, args PositionalRequestArgs, requestState RequestState) (_ string, _ shared.Range, _ bool, err error) {
	ctx, trace, endObservation := observeResolver(ctx, &err, s.operations.getHover, serviceObserverThreshold,
		observation.Args{Attrs: observation.MergeAttributes(args.Attrs(), requestState.Attrs()...)})
	defer endObservation()

	// TODO(id: support-symbol-based-hovers): We should remove position-based hover
	// information in favor of symbol based hover information.
	lookupMatcher := shared.NewStartPositionMatcher(scip.Position{Line: int32(args.Line), Character: int32(args.Character)})
	adjustedUploads, err := s.getVisibleUploads(ctx, lookupMatcher, requestState)
	if err != nil {
		return "", shared.Range{}, false, err
	}

	// Keep track of each adjusted range we know about enclosing the requested position.
	//
	// If we don't have hover text within the index where the range is defined, we'll
	// have to look in the definition index and search for the text there. We don't
	// want to return the range associated with the definition, as the range is used
	// as a hint to highlight a range in the current document.
	adjustedRanges := make([]shared.Range, 0, len(adjustedUploads))

	cachedUploads := requestState.GetCacheUploads()
	for i := range adjustedUploads {
		adjustedUpload := adjustedUploads[i]
		trace.AddEvent("TODO Domain Owner", attribute.Int("uploadID", adjustedUpload.Upload.ID))
		pos, ok := adjustedUpload.TargetMatcher.PositionBased()
		if !ok {
			panic(fmt.Sprintf("Expected position-based matcher since lookupMatcher was position-based, but got: %+v", adjustedUpload.TargetMatcher))
		}
		// Fetch hover text from the index
		text, rn, exists, err := s.lsifstore.GetHover(
			ctx,
			adjustedUpload.Upload.ID,
			adjustedUpload.TargetPathWithoutRoot(),
			int(pos.Line),
			int(pos.Character),
		)
		if err != nil {
			return "", shared.Range{}, false, errors.Wrap(err, "lsifStore.Hover")
		}
		if !exists {
			continue
		}

		// Adjust the highlighted range back to the appropriate range in the target commit
		_, adjustedRange, success, err := s.getSourceRange(ctx,
			args.RequestArgs, requestState,
			cachedUploads[i].RepositoryID, cachedUploads[i].Commit,
			args.Path, rn)
		if err != nil {
			return "", shared.Range{}, false, err
		}
		if text != "" {
			// Text attached to source range
			return text, adjustedRange, true, nil
		}
		if success {
			adjustedRanges = append(adjustedRanges, adjustedRange)

		}
	}

	// The Slow path:
	//
	// The indexes we searched in doesn't attach hover text to externally defined symbols.
	// Each indexer is free to make that choice as it's a compromise between ease of development,
	// efficiency of indexing, index output sizes, etc. We can deal with this situation by
	// looking for hover text attached to the precise definition (if one exists).

	// The range we will end up returning is interpreted within the context of the current text
	// document, so any range inside of a remote index would be of no use. We'll return the first
	// (inner-most) range that we adjusted from the source index traversals above.
	var adjustedRange shared.Range
	if len(adjustedRanges) > 0 {
		adjustedRange = adjustedRanges[0]
	}

	// Gather all import monikers attached to the ranges enclosing the requested position
	orderedMonikers, err := s.getOrderedMonikers(ctx, adjustedUploads, "import")
	if err != nil {
		return "", shared.Range{}, false, err
	}
	trace.AddEvent("TODO Domain Owner",
		attribute.Int("numMonikers", len(orderedMonikers)),
		attribute.String("monikers", monikersToString(orderedMonikers)))

	// Determine the set of uploads over which we need to perform a moniker search. This will
	// include all all indexes which define one of the ordered monikers. This should not include
	// any of the indexes we have already performed an LSIF graph traversal in above.
	uploads, err := s.getUploadsWithDefinitionsForMonikers(ctx, orderedMonikers, requestState)
	if err != nil {
		return "", shared.Range{}, false, err
	}
	trace.AddEvent("TODO Domain Owner",
		attribute.Int("numDefinitionUploads", len(uploads)),
		attribute.String("definitionUploads", uploadIDsToString(uploads)))

	ids := genslices.Map(uploads, func(u uploadsshared.CompletedUpload) int { return u.ID })
	lookupSymbols := genslices.Map(orderedMonikers, func(m precise.QualifiedMonikerData) string { return m.Identifier })

	usages, _, err := s.lsifstore.GetSymbolUsages(ctx, lsifstore.SymbolUsagesOptions{
		UsageKind:           shared.UsageKindDefinition,
		UploadIDs:           ids,
		LookupSymbols:       lookupSymbols,
		SkipPathsByUploadID: nil,
		Limit:               DefinitionsLimit,
		Offset:              0,
	})
	if err != nil {
		return "", shared.Range{}, false, errors.Wrap(err, "lsifstore.GetSymbolUsages")
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("numLocations", len(usages)))

	for _, usage := range usages {
		// Fetch hover text attached to a definition in the defining index
		text, _, exists, err := s.lsifstore.GetHover(
			ctx,
			usage.UploadID,
			usage.Path,
			usage.Range.Start.Line,
			usage.Range.Start.Character,
		)
		if err != nil {
			return "", shared.Range{}, false, errors.Wrap(err, "lsifStore.Hover")
		}
		if exists && text != "" {
			// Text attached to definition
			return text, adjustedRange, true, nil
		}
	}

	// No text available
	return "", shared.Range{}, false, nil
}

// getUploadsWithDefinitionsForMonikers returns the set of uploads that provide any of the given monikers.
// This method will not return uploads for commits which are unknown to gitserver.
func (s *Service) getUploadsWithDefinitionsForMonikers(ctx context.Context, orderedMonikers []precise.QualifiedMonikerData, requestState RequestState) ([]uploadsshared.CompletedUpload, error) {
	uploads, err := s.uploadSvc.GetCompletedUploadsWithDefinitionsForMonikers(ctx, orderedMonikers)
	if err != nil {
		return nil, errors.Wrap(err, "dbstore.DefinitionDumps")
	}

	uploadsCopy := copyUploads(uploads)
	requestState.dataLoader.SetUploadInCacheMap(uploadsCopy)

	uploadsWithResolvableCommits, err := filterUploadsWithCommits(ctx, requestState.commitCache, uploadsCopy)
	if err != nil {
		return nil, err
	}

	return uploadsWithResolvableCommits, nil
}

// monikerLimit is the maximum number of monikers that can be returned from orderedMonikers.
const monikerLimit = 10

func (s *Service) getOrderedMonikers(ctx context.Context, visibleUploads []visibleUpload, kinds ...string) ([]precise.QualifiedMonikerData, error) {
	monikerSet := newQualifiedMonikerSet()

	for i := range visibleUploads {
		pos, ok := visibleUploads[i].TargetMatcher.PositionBased()
		if !ok {
			panic(fmt.Sprintf("getOrderedMonikers should only be called from GetHover logic, which should start with a position-based matcher, but got: %+v", visibleUploads[i].TargetMatcher))
		}
		rangeMonikers, err := s.lsifstore.GetMonikersByPosition(
			ctx,
			visibleUploads[i].Upload.ID,
			visibleUploads[i].TargetPathWithoutRoot(),
			int(pos.Line),
			int(pos.Character),
		)
		if err != nil {
			return nil, errors.Wrap(err, "lsifStore.MonikersByPosition")
		}

		for _, monikers := range rangeMonikers {
			for _, moniker := range monikers {
				if moniker.PackageInformationID == "" || !sliceContains(kinds, moniker.Kind) {
					continue
				}

				packageInformationData, _, err := s.lsifstore.GetPackageInformation(
					ctx,
					visibleUploads[i].Upload.ID,
					string(moniker.PackageInformationID),
				)
				if err != nil {
					return nil, errors.Wrap(err, "lsifStore.PackageInformation")
				}

				monikerSet.add(precise.QualifiedMonikerData{
					MonikerData:            moniker,
					PackageInformationData: packageInformationData,
				})

				if len(monikerSet.monikers) >= monikerLimit {
					return monikerSet.monikers, nil
				}
			}
		}
	}

	return monikerSet.monikers, nil
}

// getUploadLocations translates a set of locations into an equivalent set of locations in the requested
// commit. If includeFallbackLocations is true, then any range in the indexed commit that cannot be translated
// will use the indexed location. Otherwise, such location are dropped.
func (s *Service) getUploadLocations(ctx context.Context, args RequestArgs, requestState RequestState, locations []shared.Usage, includeFallbackLocations bool) ([]shared.UploadUsage, error) {
	uploadLocations := make([]shared.UploadUsage, 0, len(locations))

	checkerEnabled := authz.SubRepoEnabled(requestState.authChecker)
	var a *actor.Actor
	if checkerEnabled {
		a = actor.FromContext(ctx)
	}
	for _, location := range locations {
		upload, ok := requestState.dataLoader.GetUploadFromCacheMap(location.UploadID)
		if !ok {
			continue
		}

		adjustedLocation, ok, err := s.getUploadUsage(ctx, args, requestState, upload, location)
		if err != nil {
			return nil, err
		}
		if !includeFallbackLocations && !ok {
			continue
		}

		if !checkerEnabled {
			uploadLocations = append(uploadLocations, adjustedLocation)
		} else {
			repo := api.RepoName(adjustedLocation.Upload.RepositoryName)
			if include, err := authz.FilterActorPath(ctx, requestState.authChecker, a, repo, adjustedLocation.Path.RawValue()); err != nil {
				return nil, err
			} else if include {
				uploadLocations = append(uploadLocations, adjustedLocation)
			}
		}
	}

	return uploadLocations, nil
}

// getUploadUsage translates a location (relative to the indexed commit) into an equivalent location in
// the requested commit. If the translation fails, then the original commit and range are used as the
// commit and range of the adjusted location and a false flag is returned.
func (s *Service) getUploadUsage(ctx context.Context, args RequestArgs, requestState RequestState, upload uploadsshared.CompletedUpload, usage shared.Usage) (shared.UploadUsage, bool, error) {
	repoRootRelPath := core.NewRepoRelPath(upload, usage.Path)
	adjustedCommit, adjustedRange, ok, err := s.getSourceRange(ctx, args, requestState, upload.RepositoryID, upload.Commit, repoRootRelPath, usage.Range)
	if err != nil {
		return shared.UploadUsage{}, ok, err
	}
	// Why are we dropping the ok value here??
	// TODO: What if the code has moved?

	return shared.UploadUsage{
		Upload:       upload,
		Path:         repoRootRelPath,
		TargetCommit: adjustedCommit,
		TargetRange:  adjustedRange,
		Symbol:       usage.Symbol,
		Kind:         usage.Kind,
	}, ok, nil
}

// getSourceRange translates a range (relative to the indexed commit) into an equivalent range in the requested
// commit. If the translation fails, then the original commit and range are returned along with a false-valued
// flag.
func (s *Service) getSourceRange(ctx context.Context, args RequestArgs, requestState RequestState, repositoryID int, commit string, path core.RepoRelPath, rng shared.Range) (string, shared.Range, bool, error) {
	if repositoryID != int(args.RepositoryID) {
		// No diffs between distinct repositories
		return commit, rng, true, nil
	}
	sourceRangeOpt, err := requestState.GitTreeTranslator.TranslateRange(ctx, api.CommitID(commit), args.Commit, path, rng.ToSCIPRange())
	if err != nil {
		return "", shared.Range{}, false, errors.Wrap(err, "gitTreeTranslator.GetTargetCommitRangeFromSourceRange")
	}
	if sourceRange, ok := sourceRangeOpt.Get(); ok {
		return string(args.Commit), shared.TranslateRange(sourceRange), true, nil
	}

	return commit, rng, false, nil
}

// getUploadsByIDs returns a slice of uploads with the given identifiers. This method will not return a
// new upload record for a commit which is unknown to gitserver. The given upload map is used as a
// caching mechanism - uploads present in the map are not fetched again from the database.
func (s *Service) getUploadsByIDs(ctx context.Context, ids []int, requestState RequestState) ([]uploadsshared.CompletedUpload, error) {
	missingIDs := make([]int, 0, len(ids))
	existingUploads := make([]uploadsshared.CompletedUpload, 0, len(ids))

	for _, id := range ids {
		if upload, ok := requestState.dataLoader.GetUploadFromCacheMap(id); ok {
			existingUploads = append(existingUploads, upload)
		} else {
			missingIDs = append(missingIDs, id)
		}
	}

	uploads, err := s.uploadSvc.GetCompletedUploadsByIDs(ctx, missingIDs)
	if err != nil {
		return nil, errors.Wrap(err, "service.GetCompletedUploadsByIDs")
	}

	uploadsWithResolvableCommits, err := filterUploadsWithCommits(ctx, requestState.commitCache, uploads)
	if err != nil {
		return nil, nil
	}
	requestState.dataLoader.SetUploadInCacheMap(uploadsWithResolvableCommits)

	allUploads := append(existingUploads, uploadsWithResolvableCommits...)

	return allUploads, nil
}

// DefinitionsLimit is maximum the number of locations returned from Definitions.
const DefinitionsLimit = 100

func (s *Service) GetDiagnostics(ctx context.Context, args PositionalRequestArgs, requestState RequestState) (diagnosticsAtUploads []DiagnosticAtUpload, _ int, err error) {
	ctx, trace, endObservation := observeResolver(ctx, &err, s.operations.getDiagnostics, serviceObserverThreshold,
		observation.Args{Attrs: observation.MergeAttributes(args.Attrs(), requestState.Attrs()...)})
	defer endObservation()

	uploads := s.filterCachedUploadsContainingPath(ctx, trace, requestState, args.Path)
	if len(uploads) == 0 {
		return nil, 0, errors.New("No valid upload found for provided (repo, commit, path)")
	}

	totalCount := 0

	checkerEnabled := authz.SubRepoEnabled(requestState.authChecker)
	var a *actor.Actor
	if checkerEnabled {
		a = actor.FromContext(ctx)
	}
	for _, upload := range uploads {
		trace.AddEvent("GetDiagnostics", attribute.Int("upload.ID", upload.ID))

		diagnostics, count, err := s.lsifstore.GetDiagnostics(
			ctx,
			upload.ID,
			core.NewUploadRelPath(upload, args.Path),
			args.Limit-len(diagnosticsAtUploads),
			0,
		)
		if err != nil {
			return nil, 0, errors.Wrap(err, "lsifStore.Diagnostics")
		}

		for _, diagnostic := range diagnostics {
			adjustedDiagnostic, err := s.getRequestedCommitDiagnostic(ctx, args.RequestArgs, requestState, upload, diagnostic)
			if err != nil {
				return nil, 0, err
			}

			if !checkerEnabled {
				diagnosticsAtUploads = append(diagnosticsAtUploads, adjustedDiagnostic)
				continue
			}

			// sub-repo checker is enabled, proceeding with check
			if include, err := authz.FilterActorPath(ctx, requestState.authChecker, a, api.RepoName(adjustedDiagnostic.Upload.RepositoryName), adjustedDiagnostic.Path.RawValue()); err != nil {
				return nil, 0, err
			} else if include {
				diagnosticsAtUploads = append(diagnosticsAtUploads, adjustedDiagnostic)
			}
		}

		totalCount += count
	}

	if len(diagnosticsAtUploads) > args.Limit {
		diagnosticsAtUploads = diagnosticsAtUploads[:args.Limit]
	}
	trace.AddEvent("TODO Domain Owner",
		attribute.Int("totalCount", totalCount),
		attribute.Int("numDiagnostics", len(diagnosticsAtUploads)))

	return diagnosticsAtUploads, totalCount, nil
}

// getRequestedCommitDiagnostic translates a diagnostic (relative to the indexed commit) into an equivalent diagnostic
// in the requested commit.
func (s *Service) getRequestedCommitDiagnostic(ctx context.Context, args RequestArgs, requestState RequestState, upload uploadsshared.CompletedUpload, diagnostic shared.Diagnostic[core.UploadRelPath]) (DiagnosticAtUpload, error) {
	rn := shared.Range{
		Start: shared.Position{
			Line:      diagnostic.StartLine,
			Character: diagnostic.StartCharacter,
		},
		End: shared.Position{
			Line:      diagnostic.EndLine,
			Character: diagnostic.EndCharacter,
		},
	}

	// Adjust path in diagnostic before reading it. This value is used in the adjustRange
	// call below, and is also reflected in the embedded diagnostic value in the return.
	diagnostic2 := shared.AdjustDiagnostic(diagnostic, upload)

	adjustedCommit, adjustedRange, _, err := s.getSourceRange(
		ctx,
		args,
		requestState,
		upload.RepositoryID,
		upload.Commit,
		diagnostic2.Path,
		rn,
	)
	if err != nil {
		return DiagnosticAtUpload{}, err
	}

	return DiagnosticAtUpload{
		Diagnostic:     diagnostic2,
		Upload:         upload,
		AdjustedCommit: adjustedCommit,
		AdjustedRange:  adjustedRange,
	}, nil
}

func (s *Service) VisibleUploadsForPath(ctx context.Context, requestState RequestState) (uploads []uploadsshared.CompletedUpload, err error) {
	ctx, trace, endObservation := s.operations.visibleUploadsForPath.With(ctx, &err, observation.Args{Attrs: requestState.Attrs()})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("numUploads", len(uploads)),
		}})
	}()

	return s.filterCachedUploadsContainingPath(ctx, trace, requestState, requestState.Path), nil
}

// filterCachedUploadsContainingPath adjusts the current target path for each upload visible from the current target
// commit. If an upload cannot be adjusted, it will be omitted from the returned slice.
func (s *Service) filterCachedUploadsContainingPath(ctx context.Context, trace observation.TraceLogger, requestState RequestState, path core.RepoRelPath) []uploadsshared.CompletedUpload {
	// NOTE(id: path-based-upload-filtering):
	//
	// (70% confidence) There are a few cases here for the uploads cached earlier.
	// 1. The upload was for an older commit.
	//    1a r.requestState.path exists in upload -> This is OK, we can use the upload as-is.
	//    1b r.requestState.path doesn't exist in upload, but there is a path P in upload
	//       such that `git diff upload.Commit..r.requestState.commit` would say that P
	//       was renamed to r.requestState.path.
	//       -> This is not easy to do, see NOTE(id: codenav-file-rename-detection) for details.
	//    1c r.requestState.path doesn't exist in upload, and there is no path in upload
	//       that was detected to be renamed.
	//       -> We should detect this case and skip the upload. However, similar to 1b above,
	//          we can't detect this easily.
	// 2. The upload is for the same commit. In this case, we can be confident that the
	//    path exists in the upload (otherwise we wouldn't have cached it).
	cachedUploads := requestState.GetCacheUploads()

	filteredUploads, err := filterUploadsImpl(ctx, s.lsifstore, cachedUploads, path,
		/*skipDBCheck*/ func(upload uploadsshared.CompletedUpload) bool {
			// See NOTE(id: path-based-upload-filtering)
			return api.CommitID(upload.Commit) == requestState.Commit && path == requestState.Path
		})
	if err != nil {
		trace.Warn("FindDocumentIDs failed", log.Error(err))
	}

	return filteredUploads
}

func (s *Service) GetRanges(ctx context.Context, args PositionalRequestArgs, requestState RequestState, startLine, endLine int) (adjustedRanges []AdjustedCodeIntelligenceRange, err error) {
	ctx, trace, endObservation := observeResolver(ctx, &err, s.operations.getRanges, serviceObserverThreshold,
		observation.Args{Attrs: append(requestState.Attrs(),
			attribute.Int("startLine", startLine),
			attribute.Int("endLine", endLine))},
	)
	defer endObservation()

	uploadsWithPath := s.filterCachedUploadsContainingPath(ctx, trace, requestState, args.Path)
	if len(uploadsWithPath) == 0 {
		return nil, errors.New("No valid upload found for provided (repo, commit, path)")
	}

	for _, upload := range uploadsWithPath {
		trace.AddEvent("TODO Domain Owner", attribute.Int("uploadID", upload.ID))
		ranges, err := s.lsifstore.GetRanges(
			ctx,
			upload.ID,
			core.NewUploadRelPath(upload, args.Path),
			startLine,
			endLine,
		)
		if err != nil {
			return nil, errors.Wrap(err, "lsifStore.Ranges")
		}

		for _, rn := range ranges {
			adjustedRange, ok, err := s.getCodeIntelligenceRange(ctx, args.RequestArgs, requestState, upload, args.Path, rn)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}

			adjustedRanges = append(adjustedRanges, adjustedRange)
		}
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("numRanges", len(adjustedRanges)))

	return adjustedRanges, nil
}

// getCodeIntelligenceRange translates a range summary (relative to the indexed commit) into an
// equivalent range summary in the requested commit. If the translation fails, a false-valued flag
// is returned.
func (s *Service) getCodeIntelligenceRange(
	ctx context.Context, args RequestArgs, requestState RequestState,
	upload uploadsshared.CompletedUpload, targetPath core.RepoRelPath,
	rn shared.CodeIntelligenceRange,
) (AdjustedCodeIntelligenceRange, bool, error) {
	_, adjustedRange, ok, err := s.getSourceRange(ctx, args, requestState, upload.RepositoryID, upload.Commit, targetPath, rn.Range)
	if err != nil || !ok {
		return AdjustedCodeIntelligenceRange{}, false, err
	}

	definitions, err := s.getUploadLocations(ctx, args, requestState, rn.Definitions, false)
	if err != nil {
		return AdjustedCodeIntelligenceRange{}, false, err
	}

	references, err := s.getUploadLocations(ctx, args, requestState, rn.References, false)
	if err != nil {
		return AdjustedCodeIntelligenceRange{}, false, err
	}

	implementations, err := s.getUploadLocations(ctx, args, requestState, rn.Implementations, false)
	if err != nil {
		return AdjustedCodeIntelligenceRange{}, false, err
	}

	transformDedup := func(uu []shared.UploadUsage) []shared.UploadLocation {
		return shared.SortAndDedupLocations(genslices.Map(uu, shared.UploadUsage.ToLocation))
	}

	return AdjustedCodeIntelligenceRange{
		Range:           adjustedRange,
		Definitions:     transformDedup(definitions),
		References:      transformDedup(references),
		Implementations: transformDedup(implementations),
		HoverText:       rn.HoverText,
	}, true, nil
}

// GetStencil returns the set of locations defining the symbol at the given position.
func (s *Service) GetStencil(ctx context.Context, args PositionalRequestArgs, requestState RequestState) (adjustedRanges []shared.Range, err error) {
	ctx, trace, endObservation := observeResolver(ctx, &err, s.operations.getStencil, serviceObserverThreshold, observation.Args{Attrs: requestState.Attrs()})
	defer endObservation()

	filteredUploads := s.filterCachedUploadsContainingPath(ctx, trace, requestState, args.Path)
	if len(filteredUploads) == 0 {
		return nil, errors.New("No valid upload found for provided (repo, commit, path)")
	}

	for _, upload := range filteredUploads {
		trace.AddEvent("TODO Domain Owner", attribute.Int("uploadID", upload.ID))

		ranges, err := s.lsifstore.GetStencil(ctx, upload.ID, core.NewUploadRelPath(upload, args.Path))
		if err != nil {
			return nil, errors.Wrap(err, "lsifStore.Stencil")
		}

		for i, rn := range ranges {
			// FIXME: change this at it expects an empty uploadsshared.CompletedUpload{}
			cu := requestState.GetCacheUploadsAtIndex(i)
			// Adjust the highlighted range back to the appropriate range in the target commit
			_, adjustedRange, success, err := s.getSourceRange(ctx, args.RequestArgs, requestState, cu.RepositoryID, cu.Commit, args.Path, rn)
			if err != nil {
				return nil, err
			}
			if success {
				adjustedRanges = append(adjustedRanges, adjustedRange)
			}
		}
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("numRanges", len(adjustedRanges)))

	sortedRanges := sortRanges(adjustedRanges)
	return dedupeRanges(sortedRanges), nil
}

func (s *Service) GetClosestCompletedUploadsForBlob(ctx context.Context, opts uploadsshared.UploadMatchingOptions) (_ []uploadsshared.CompletedUpload, err error) {
	ctx, trace, endObservation := s.operations.getClosestCompletedUploadsForBlob.With(ctx, &err, observation.Args{Attrs: opts.Attrs()})
	defer endObservation(1, observation.Args{})

	candidates, err := s.uploadSvc.InferClosestUploads(ctx, opts)
	if err != nil {
		return nil, err
	}

	trace.AddEvent("InferClosestUploads",
		attribute.Int("numCandidates", len(candidates)),
		attribute.String("candidates", uploadIDsToString(candidates)))

	commitChecker := NewCommitCache(s.repoStore, s.gitserver)
	commitChecker.SetResolvableCommit(opts.RepositoryID, opts.Commit)

	candidatesWithExistingCommits, err := filterUploadsWithCommits(ctx, commitChecker, candidates)
	if err != nil {
		return nil, err
	}
	trace.AddEvent("filterUploadsWithCommits",
		attribute.Int("numCandidatesWithExistingCommits", len(candidatesWithExistingCommits)),
		attribute.String("candidatesWithExistingCommits", uploadIDsToString(candidatesWithExistingCommits)))

	candidatesWithExistingCommitsAndPaths, err := filterUploadsWithPaths(ctx, s.lsifstore, opts, candidatesWithExistingCommits)
	if err != nil {
		return nil, errors.Wrap(err, "filtering uploads based on paths")
	}
	trace.AddEvent("filterUploadsWithPaths",
		attribute.Int("numFiltered", len(candidatesWithExistingCommitsAndPaths)),
		attribute.String("filtered", uploadIDsToString(candidatesWithExistingCommitsAndPaths)))

	return candidatesWithExistingCommitsAndPaths, nil
}

// filterUploadsWithCommits only keeps the uploads for commits which are known to gitserver.
// A fresh slice is returned without modifying the original slice.
func filterUploadsWithCommits(ctx context.Context, commitCache CommitCache, uploads []uploadsshared.CompletedUpload) ([]uploadsshared.CompletedUpload, error) {
	rcs := make([]RepositoryCommit, 0, len(uploads))
	for _, upload := range uploads {
		rcs = append(rcs, RepositoryCommit{
			RepositoryID: api.RepoID(upload.RepositoryID),
			Commit:       api.CommitID(upload.Commit),
		})
	}
	exists, err := commitCache.ExistsBatch(ctx, rcs)
	if err != nil {
		return nil, err
	}

	filtered := make([]uploadsshared.CompletedUpload, 0, len(uploads))
	for i, upload := range uploads {
		if exists[i] {
			filtered = append(filtered, upload)
		}
	}

	return filtered, nil
}

type firstPassResult[U any] struct {
	skipDBCheck bool
	upload      U
} // Go doesn't allow type definitions in generic functions

// filterUploadsImpl returns the uploads in 'candidates' containing 'path'.
//
// This is done by consulting the codeintel_scip_document_lookup table
// in a single query.
//
// Params:
//   - If skipDBCheck returns true, then the upload is included in the output slice.
//     Otherwise, the upload is included in the output slice iff the database
//     contains the (uploadID, path) pair.
//
// Post-conditions:
//   - The order of the returned slice matches the order of candidates.
//   - Even if there is an error consulting the database, the candidates for
//     which skipDBCheck was true will be included in the returned slice.
func filterUploadsImpl[U core.UploadLike](
	ctx context.Context,
	lsifstore lsifstore.LsifStore,
	candidates []U,
	path core.RepoRelPath,
	skipDBCheck func(upload U) bool,
) ([]U, error) {
	// Being careful about maintaining determinism here by only
	// iterating based on order in cachedUploads.
	results := []firstPassResult[U]{}
	lookupPaths := map[int]core.UploadRelPath{}
	for _, upload := range candidates {
		uploadRelPath := core.NewUploadRelPath(upload, path)
		skipCheck := skipDBCheck(upload)
		results = append(results, firstPassResult[U]{skipDBCheck: skipCheck, upload: upload})
		if skipCheck {
			continue
		}
		// We don't have to worry about over-writing because even if an
		// upload with the same ID is present multiple times, different
		// copies will have the same Root, so uploadRelPath will be identical.
		lookupPaths[upload.GetID()] = uploadRelPath
	}

	foundDocIds, findDocumentIDsErr := lsifstore.FindDocumentIDs(ctx, lookupPaths)
	// delay emitting the error, return partial results as much as possible
	if foundDocIds == nil {
		foundDocIds = map[int]int{}
	}

	filteredUploads := genslices.MapFilter(results, func(res firstPassResult[U]) (U, bool) {
		if res.skipDBCheck {
			return res.upload, true
		}
		_, found := foundDocIds[res.upload.GetID()]
		return res.upload, found
	})

	return filteredUploads, findDocumentIDsErr
}

func filterUploadsWithPaths(
	ctx context.Context,
	lsifstore lsifstore.LsifStore,
	opts uploadsshared.UploadMatchingOptions,
	candidates []uploadsshared.CompletedUpload,
) ([]uploadsshared.CompletedUpload, error) {
	return filterUploadsImpl(ctx, lsifstore, candidates, opts.Path,
		/* skipDBCheck */
		func(upload uploadsshared.CompletedUpload) bool {
			switch opts.RootToPathMatching {
			case uploadsshared.RootMustEnclosePath:
				return false
			case uploadsshared.RootEnclosesPathOrPathEnclosesRoot:
				// TODO(efritz) - ensure there's a valid document path for this condition as well
				return true
			default:
				panic("Unhandled case for RootToPathMatching")
			}
		})
}

func copyUploads(uploads []uploadsshared.CompletedUpload) []uploadsshared.CompletedUpload {
	ud := make([]uploadsshared.CompletedUpload, len(uploads))
	copy(ud, uploads)
	return ud
}

// ErrConcurrentModification occurs when a page of a references request cannot be resolved as
// the set of visible uploads have changed since the previous request for the same result set.
var ErrConcurrentModification = errors.New("result set changed while paginating")

// getVisibleUploads adjusts the current target path and the given position for each upload visible
// from the current target commit. If an upload cannot be adjusted, it will be omitted from the
// returned slice.
func (s *Service) getVisibleUploads(ctx context.Context, sourceMatcher shared.Matcher, r RequestState) ([]visibleUpload, error) {
	visibleUploads := make([]visibleUpload, 0, len(r.dataLoader.uploads))
	for i := range r.dataLoader.uploads {
		adjustedUpload, ok, err := s.getVisibleUpload(ctx, sourceMatcher, r.dataLoader.uploads[i], r)
		if err != nil {
			return nil, err
		}
		if ok {
			visibleUploads = append(visibleUploads, adjustedUpload)
		}
	}

	return visibleUploads, nil
}

// getVisibleUpload returns the current target path and the given range for the given upload. If
// the upload cannot be adjusted, a false-valued flag is returned.
func (s *Service) getVisibleUpload(ctx context.Context, sourceMatcher shared.Matcher, upload uploadsshared.CompletedUpload, r RequestState) (visibleUpload, bool, error) {
	// NOTE: Type below is explicitly written as we want to call RawValue() later
	var targetPath core.RepoRelPath = r.Path
	targetMatcher, ok, err := func() (shared.Matcher, bool, error) {
		if sym, sourceRange, ok := sourceMatcher.SymbolBased(); ok {
			optTargetRange, err := r.GitTreeTranslator.TranslateRange(ctx, r.Commit, api.CommitID(upload.Commit), targetPath, sourceRange)
			targetRange, ok := optTargetRange.Get()
			return shared.NewSCIPBasedMatcher(targetRange, sym.UnwrapOr("")), ok, err
		} else if sourcePos, ok := sourceMatcher.PositionBased(); ok {
			optTargetPos, err := r.GitTreeTranslator.TranslatePosition(ctx, r.Commit, api.CommitID(upload.Commit), targetPath, sourcePos)
			targetPos, ok := optTargetPos.Get()
			return shared.NewStartPositionMatcher(targetPos), ok, err
		}
		panic(fmt.Sprintf("Unhandle case for matcher: %+v", sourceMatcher))
	}()
	if err != nil || !ok {
		return visibleUpload{}, false, errors.Wrap(err, "gitTreeTranslator.Translate")
	}

	return visibleUpload{
		Upload:        upload,
		TargetPath:    targetPath,
		TargetMatcher: targetMatcher,
	}, true, nil
}

func (s *Service) SnapshotForDocument(ctx context.Context, repositoryID api.RepoID, commit api.CommitID, path core.RepoRelPath, uploadID int) (data []shared.SnapshotData, err error) {
	ctx, _, endObservation := s.operations.snapshotForDocument.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repoID", int(repositoryID)),
		attribute.String("commit", string(commit)),
		attribute.String("path", path.RawValue()),
		attribute.Int("uploadID", uploadID),
	}})
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("snapshotSymbols", len(data)),
		}})
	}()

	uploads, err := s.uploadSvc.GetCompletedUploadsByIDs(ctx, []int{uploadID})
	if err != nil {
		return nil, err
	}

	if len(uploads) == 0 {
		return nil, nil
	}

	upload := uploads[0]

	optDocument, err := s.lsifstore.SCIPDocument(ctx, upload.ID, core.NewUploadRelPath(upload, path))
	if err != nil {
		return nil, err
	}

	document, ok := optDocument.Get()
	if !ok {
		return nil, errors.New("no document found")
	}

	r, err := s.gitserver.NewFileReader(ctx, api.RepoName(upload.RepositoryName), api.CommitID(upload.Commit), path.RawValue())
	if err != nil {
		return nil, err
	}
	file, err := io.ReadAll(r)
	r.Close()
	if err != nil {
		return nil, err
	}

	// client-side normalizes the file to LF, so normalize CRLF files to that so the offsets are correct
	file = bytes.ReplaceAll(file, []byte("\r\n"), []byte("\n"))

	repo, err := s.repoStore.Get(ctx, api.RepoID(upload.RepositoryID))
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}
	gittranslator := NewGitTreeTranslator(s.gitserver, *repo)

	linemap := newLinemap(string(file))
	formatter := scip.LenientVerboseSymbolFormatter
	symtab := document.SymbolTable()

	for _, occ := range document.Occurrences {
		var snapshotData shared.SnapshotData

		formatted, err := formatter.Format(occ.Symbol)
		if err != nil {
			formatted = fmt.Sprintf("error formatting %q", occ.Symbol)
		}

		originalRange := scip.NewRangeUnchecked(occ.Range)

		lineOffset := int32(linemap.positions[originalRange.Start.Line])
		line := file[lineOffset : lineOffset+originalRange.Start.Character]

		tabCount := bytes.Count(line, []byte("\t"))

		var snap strings.Builder
		snap.WriteString(strings.Repeat(" ", (int(originalRange.Start.Character)-tabCount)+(tabCount*4)))
		snap.WriteString(strings.Repeat("^", int(originalRange.End.Character-originalRange.Start.Character)))
		snap.WriteRune(' ')

		isDefinition := occ.SymbolRoles&int32(scip.SymbolRole_Definition) > 0
		if isDefinition {
			snap.WriteString("definition")
		} else {
			snap.WriteString("reference")
		}
		snap.WriteRune(' ')
		snap.WriteString(formatted)

		snapshotData.Symbol = snap.String()

		// hasOverrideDocumentation := len(occ.OverrideDocumentation) > 0
		// if hasOverrideDocumentation {
		// 	documentation := occ.OverrideDocumentation[0]
		// 	writeDocumentation(&b, documentation, prefix, true)
		// }

		if info, ok := symtab[occ.Symbol]; ok && isDefinition {
			// for _, documentation := range info.Documentation {
			// 	// At least get the first line of documentation if there is leading whitespace
			// 	documentation = strings.TrimSpace(documentation)
			// 	writeDocumentation(&b, documentation, prefix, false)
			// }
			slices.SortFunc(info.Relationships, func(a, b *scip.Relationship) int {
				return cmp.Compare(a.Symbol, b.Symbol)
			})
			for _, relationship := range info.Relationships {
				var b strings.Builder
				b.WriteString(strings.Repeat(" ", (int(originalRange.Start.Character)-tabCount)+(tabCount*4)))
				b.WriteString(strings.Repeat("^", int(originalRange.End.Character-originalRange.Start.Character)))
				b.WriteString(" relationship ")

				formatted, err = formatter.Format(relationship.Symbol)
				if err != nil {
					formatted = fmt.Sprintf("error formatting %q", occ.Symbol)
				}

				b.WriteString(formatted)
				if relationship.IsImplementation {
					b.WriteString(" implementation")
				}
				if relationship.IsReference {
					b.WriteString(" reference")
				}
				if relationship.IsTypeDefinition {
					b.WriteString(" type_definition")
				}

				snapshotData.AdditionalData = append(snapshotData.AdditionalData, b.String())
			}
		}

		newPositionOpt, err := gittranslator.TranslatePosition(ctx, commit, upload.GetCommit(), path, originalRange.Start)
		if err != nil {
			return nil, err
		}
		newPosition, ok := newPositionOpt.Get()
		// if the line was changed, then we're not providing precise codeintel for this line, so skip it
		if !ok {
			continue
		}

		snapshotData.DocumentOffset = linemap.positions[newPosition.Line+1]

		data = append(data, snapshotData)
	}

	return
}

func (s *Service) SCIPDocument(ctx context.Context, gitTreeTranslator GitTreeTranslator, upload core.UploadLike, targetCommit api.CommitID, path core.RepoRelPath) (*scip.Document, error) {
	optRawDocument, err := s.lsifstore.SCIPDocument(ctx, upload.GetID(), core.NewUploadRelPath(upload, path))
	if err != nil {
		return nil, err
	}
	rawDocument, ok := optRawDocument.Get()
	if !ok {
		return nil, errors.New("document not found")
	}
	// The caller shouldn't need to care whether the document was uploaded
	// for a different root or not.
	rawDocument.RelativePath = path.RawValue()
	if upload.GetCommit() == targetCommit {
		return rawDocument, nil
	}
	translated := make([]*scip.Occurrence, 0, len(rawDocument.Occurrences))
	for _, occ := range rawDocument.Occurrences {
		sourceRange := scip.NewRangeUnchecked(occ.Range)
		targetRangeOpt, err := gitTreeTranslator.TranslateRange(ctx, upload.GetCommit(), targetCommit, path, sourceRange)
		if err != nil {
			return nil, errors.Wrap(err, "While translating ranges between commits")
		}
		targetRange, success := targetRangeOpt.Get()
		if !success {
			continue
		}
		occ.Range = targetRange.SCIPRange()
		translated = append(translated, occ)
	}
	rawDocument.Occurrences = translated
	return rawDocument, nil
}

type SyntacticUsagesErrorCode int

const (
	SU_NoSyntacticIndex SyntacticUsagesErrorCode = iota
	SU_NoSymbolAtRequestedRange
	SU_FailedToSearch
	SU_FailedToAdjustRange
	SU_Fatal
)

type SyntacticUsagesError struct {
	Code            SyntacticUsagesErrorCode
	UnderlyingError error
}

var _ error = SyntacticUsagesError{}

func (e SyntacticUsagesError) Error() string {
	msg := ""
	switch e.Code {
	case SU_NoSyntacticIndex:
		msg = "No syntactic index"
	case SU_NoSymbolAtRequestedRange:
		msg = "No symbol at requested range"
	case SU_FailedToSearch:
		msg = "Failed to get candidate matches via searcher"
	case SU_FailedToAdjustRange:
		msg = "Failed to adjust range across commits"
	case SU_Fatal:
		msg = "fatal error"
	}
	if e.UnderlyingError == nil {
		return msg
	}
	return fmt.Sprintf("%s: %s", msg, e.UnderlyingError)
}

func (s *Service) getSyntacticUpload(ctx context.Context, trace observation.TraceLogger, args UsagesForSymbolArgs) (uploadsshared.CompletedUpload, *SyntacticUsagesError) {
	uploads, err := s.GetClosestCompletedUploadsForBlob(ctx, uploadsshared.UploadMatchingOptions{
		RepositoryID:       args.Repo.ID,
		Commit:             args.Commit,
		Indexer:            uploadsshared.SyntacticIndexer,
		Path:               args.Path,
		RootToPathMatching: uploadsshared.RootMustEnclosePath,
	})

	if err != nil || len(uploads) == 0 {
		return uploadsshared.CompletedUpload{}, &SyntacticUsagesError{
			Code:            SU_NoSyntacticIndex,
			UnderlyingError: err,
		}
	}

	if len(uploads) > 1 {
		trace.Warn(
			"Multiple syntactic uploads found, picking the first one",
			log.String("repo", args.Repo.URI),
			log.String("commit", args.Commit.Short()),
			log.String("path", args.Path.RawValue()),
		)
	}
	return uploads[0], nil
}

type SearchBasedMatch struct {
	Path               core.RepoRelPath
	Range              scip.Range
	SurroundingContent string
	IsDefinition       bool
}

func (s SearchBasedMatch) GetRange() scip.Range {
	return s.Range
}

func (s SearchBasedMatch) GetIsDefinition() bool {
	return s.IsDefinition
}

func (s SearchBasedMatch) GetSurroundingContent() string {
	return s.SurroundingContent
}

type SyntacticMatch struct {
	Path               core.RepoRelPath
	Range              scip.Range
	SurroundingContent string
	IsDefinition       bool
	Symbol             string
}

func (s SyntacticMatch) GetRange() scip.Range {
	return s.Range
}
func (s SyntacticMatch) GetIsDefinition() bool {
	return s.IsDefinition
}
func (s SyntacticMatch) GetSurroundingContent() string {
	return s.SurroundingContent
}

type SyntacticUsagesResult struct {
	Matches                 []SyntacticMatch
	PreviousSyntacticSearch PreviousSyntacticSearch
	NextCursor              core.Option[UsagesCursor]
}

type UsagesForSymbolArgs struct {
	Repo        types.Repo
	Commit      api.CommitID
	Path        core.RepoRelPath
	SymbolRange scip.Range
	Limit       int32
	Cursor      UsagesCursor
}

func (s *Service) SyntacticUsages(
	ctx context.Context,
	gitTreeTranslator GitTreeTranslator,
	args UsagesForSymbolArgs,
) (SyntacticUsagesResult, *SyntacticUsagesError) {
	// The `nil` in the second argument is here, because `With` does not work with custom error types.
	ctx, trace, endObservation := s.operations.syntacticUsages.With(ctx, nil, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repoId", int(args.Repo.ID)),
		attribute.String("commit", string(args.Commit)),
		attribute.String("path", args.Path.RawValue()),
		attribute.String("symbolRange", args.SymbolRange.String()),
		attribute.Int("limit", int(args.Limit)),
	}})
	defer endObservation(1, observation.Args{})

	upload, err := s.getSyntacticUpload(ctx, trace, args)
	if err != nil {
		return SyntacticUsagesResult{}, err
	}
	index := NewMappedIndexFromTranslator(s.lsifstore, gitTreeTranslator, upload, args.Commit)
	return syntacticUsagesImpl(ctx, trace, s.searchClient, index, args)
}

const MAX_FILE_SIZE_FOR_SYMBOL_DETECTION_BYTES = 10_000_000

func (s *Service) symbolNameFromGit(ctx context.Context, args UsagesForSymbolArgs) (string, error) {
	stat, err := s.gitserver.Stat(ctx, args.Repo.Name, args.Commit, args.Path.RawValue())
	if err != nil {
		return "", err
	}

	if stat.Size() > MAX_FILE_SIZE_FOR_SYMBOL_DETECTION_BYTES {
		return "", errors.New("code navigation is not supported for files larger than 10MB")
	}

	r, err := s.gitserver.NewFileReader(ctx, args.Repo.Name, args.Commit, args.Path.RawValue())
	if err != nil {
		return "", err
	}
	defer r.Close()
	symbolName, err := sliceRangeFromReader(r, args.SymbolRange)
	if err != nil {
		return "", err
	}
	return symbolName, nil
}

// PreviousSyntacticSearch is used to avoid recomputing information
// we've already collected during syntactic usages during
// search-based usages.
type PreviousSyntacticSearch struct {
	MappedIndex MappedIndex
	SymbolName  string
	Language    string
}

func languageFromFilepath(trace observation.TraceLogger, path core.RepoRelPath) (string, error) {
	langs, _ := languages.GetLanguages(path.RawValue(), nil)
	if len(langs) == 0 {
		return "", errors.New("Unknown language")
	}
	if len(langs) > 1 {
		trace.Info("Ambiguous language for file, arbitrarily choosing the first", log.String("path", path.RawValue()), log.Strings("langs", langs))
	}
	return langs[0], nil
}

type SearchBasedUsagesResult struct {
	Matches    []SearchBasedMatch
	NextCursor core.Option[UsagesCursor]
}

func (s *Service) SearchBasedUsages(
	ctx context.Context,
	gitTreeTranslator GitTreeTranslator,
	args UsagesForSymbolArgs,
	previousSyntacticSearch core.Option[PreviousSyntacticSearch],
) (_ SearchBasedUsagesResult, err error) {
	ctx, trace, endObservation := s.operations.searchBasedUsages.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repoId", int(args.Repo.ID)),
		attribute.String("commit", string(args.Commit)),
		attribute.String("path", args.Path.RawValue()),
		attribute.String("symbolRange", args.SymbolRange.String()),
		attribute.Int("limit", int(args.Limit)),
	}})
	defer endObservation(1, observation.Args{})

	var language string
	var symbolName string
	var syntacticIndex core.Option[MappedIndex]

	if prev, ok := previousSyntacticSearch.Get(); ok {
		language = prev.Language
		symbolName = prev.SymbolName
		syntacticIndex = core.Some[MappedIndex](prev.MappedIndex)
	} else {
		language, err = languageFromFilepath(trace, args.Path)
		if err != nil {
			return SearchBasedUsagesResult{}, err
		}

		nameFromGit, err := s.symbolNameFromGit(ctx, args)
		if err != nil {
			return SearchBasedUsagesResult{}, err
		}
		symbolName = nameFromGit

		upload, uploadErr := s.getSyntacticUpload(ctx, trace, args)
		if uploadErr != nil {
			trace.Info("no syntactic upload found, return all search-based results", log.Error(err))
		} else {
			syntacticIndex = core.Some[MappedIndex](NewMappedIndexFromTranslator(s.lsifstore, gitTreeTranslator, upload, args.Commit))
		}
	}
	return searchBasedUsagesImpl(ctx, trace, s.searchClient, args, symbolName, language, syntacticIndex)
}
