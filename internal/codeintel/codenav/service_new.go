package codenav

import (
	"context"
	"sort"
	"strings"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/internal/lsifstore"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	"github.com/sourcegraph/sourcegraph/internal/collections"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

func (s *Service) GetDefinitions(
	ctx context.Context,
	args PositionalRequestArgs,
	requestState RequestState,
) (_ []shared.UploadLocation, err error) {
	locations, _, err := s.gatherLocations(
		ctx, args, requestState, Cursor{},

		s.operations.getDefinitions, // operation
		"definitions",               // tableName
		false,                       // includeReferencingIndexes
		LocationExtractorFunc(s.lsifstore.ExtractDefinitionLocationsFromPosition),
	)

	return locations, err
}

func (s *Service) GetReferences(
	ctx context.Context,
	args PositionalRequestArgs,
	requestState RequestState,
	cursor Cursor,
) (_ []shared.UploadLocation, nextCursor Cursor, err error) {
	return s.gatherLocations(
		ctx, args, requestState, cursor,

		s.operations.getReferences, // operation
		"references",               // tableName
		true,                       // includeReferencingIndexes
		LocationExtractorFunc(s.lsifstore.ExtractReferenceLocationsFromPosition),
	)
}

func (s *Service) GetImplementations(
	ctx context.Context,
	args PositionalRequestArgs,
	requestState RequestState,
	cursor Cursor,
) (_ []shared.UploadLocation, nextCursor Cursor, err error) {
	return s.gatherLocations(
		ctx, args, requestState, cursor,

		s.operations.getImplementations, // operation
		"implementations",               // tableName
		true,                            // includeReferencingIndexes
		LocationExtractorFunc(s.lsifstore.ExtractImplementationLocationsFromPosition),
	)
}

func (s *Service) GetPrototypes(
	ctx context.Context,
	args PositionalRequestArgs,
	requestState RequestState,
	cursor Cursor,
) (_ []shared.UploadLocation, nextCursor Cursor, err error) {
	return s.gatherLocations(
		ctx, args, requestState, cursor,

		s.operations.getPrototypes, // operation
		"definitions",              // N.B.: we're looking for definitions of interfaces
		false,                      // includeReferencingIndexes
		LocationExtractorFunc(s.lsifstore.ExtractPrototypeLocationsFromPosition),
	)
}

func (s *Service) GetDefinitionsBySymbolNames(
	ctx context.Context,
	args RequestArgs,
	requestState RequestState,
	symbolNames []string,
) (_ []shared.UploadLocation, err error) {
	locations, _, err := s.gatherLocationsBySymbolNames(
		ctx, args, requestState, Cursor{},

		s.operations.getDefinitions, // operation
		"definitions",               // tableName
		false,                       // includeReferencingIndexes
		symbolNames,
	)

	return locations, err
}

//
//

type LocationExtractor interface {
	// Extract converts a location key (a location within a particular index's text document) into a
	// set of locations within _that specific document_ related to the symbol at that position, as well
	// as the set of related symbol names that should be searched in other indexes for a complete result
	// set.
	//
	// The relationship between symbols is implementation specific.
	Extract(ctx context.Context, locationKey lsifstore.LocationKey) ([]shared.Location, []string, error)
}

type LocationExtractorFunc func(ctx context.Context, locationKey lsifstore.LocationKey) ([]shared.Location, []string, error)

func (f LocationExtractorFunc) Extract(ctx context.Context, locationKey lsifstore.LocationKey) ([]shared.Location, []string, error) {
	return f(ctx, locationKey)
}

func (s *Service) gatherLocations(
	ctx context.Context,
	args PositionalRequestArgs,
	requestState RequestState,
	cursor Cursor,
	operation *observation.Operation,
	tableName string,
	includeReferencingIndexes bool,
	extractor LocationExtractor,
) (allLocations []shared.UploadLocation, _ Cursor, err error) {
	ctx, trace, endObservation := observeResolver(ctx, &err, operation, serviceObserverThreshold, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repositoryID", args.RepositoryID),
		attribute.String("commit", args.Commit),
		attribute.String("path", args.Path),
		attribute.Int("numUploads", len(requestState.GetCacheUploads())),
		attribute.String("uploads", uploadIDsToString(requestState.GetCacheUploads())),
		attribute.Int("line", args.Line),
		attribute.Int("character", args.Character),
	}})
	defer endObservation()

	if cursor.Phase == "" {
		cursor.Phase = "local"
	}

	// First, we determine the set of SCIP indexes that can act as one of our "roots" for the
	// following traversal. We see which SCIP indexes cover the particular query position and
	// stash this metadata on the cursor for subsequent queries.

	var visibleUploads []visibleUpload

	// N.B.: cursor is purposefully re-assigned here
	visibleUploads, cursor, err = s.getVisibleUploadsFromCursor(
		ctx,
		args,
		requestState,
		cursor,
	)
	if err != nil {
		return nil, Cursor{}, err
	}

	var visibleUploadIDs []int
	for _, upload := range visibleUploads {
		visibleUploadIDs = append(visibleUploadIDs, upload.Upload.ID)
	}
	trace.AddEvent("VisibleUploads", attribute.IntSlice("visibleUploadIDs", visibleUploadIDs))

	// The following loop calls local and remote location resolution phases in alternation. As
	// each phase controls whether or not it should execute, this is safe.
	//
	// Such a loop exists as each invocation of either phase may produce fewer results than the
	// requested page size. For example, the local phase may have a small number of results but
	// the remote phase has additional results that could fit on the first page. Similarly, if
	// there are many references to a symbol over a large number of indexes but each index has
	// only a small number of locations, they can all be combined into a single page. Running
	// each phase multiple times and combining the results will create a full page, if the
	// result set was not exhausted), on each round-trip call to this service's method.

outer:
	for cursor.Phase != "done" {
		for _, gatherLocations := range []gatherLocationsFunc{s.gatherLocalLocations, s.gatherRemoteLocationsShim} {
			trace.AddEvent("Gather", attribute.String("phase", cursor.Phase), attribute.Int("numLocationsGathered", len(allLocations)))

			if len(allLocations) >= args.Limit {
				// we've filled our page, exit with current results
				break outer
			}

			var locations []shared.UploadLocation

			// N.B.: cursor is purposefully re-assigned here
			locations, cursor, err = gatherLocations(
				ctx,
				trace,
				args.RequestArgs,
				requestState,
				tableName,
				includeReferencingIndexes,
				cursor,
				args.Limit-len(allLocations), // remaining space in the page
				extractor,
				visibleUploads,
			)
			if err != nil {
				return nil, Cursor{}, err
			}
			allLocations = append(allLocations, locations...)
		}
	}

	return allLocations, cursor, nil
}

func (s *Service) gatherLocationsBySymbolNames(
	ctx context.Context,
	args RequestArgs,
	requestState RequestState,
	cursor Cursor,
	operation *observation.Operation,
	tableName string,
	includeReferencingIndexes bool,
	symbolNames []string,
) (allLocations []shared.UploadLocation, _ Cursor, err error) {
	ctx, trace, endObservation := observeResolver(ctx, &err, operation, serviceObserverThreshold, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("repositoryID", args.RepositoryID),
		attribute.String("commit", args.Commit),
		attribute.Int("numUploads", len(requestState.GetCacheUploads())),
		attribute.String("uploads", uploadIDsToString(requestState.GetCacheUploads())),
	}})
	defer endObservation()

	if cursor.Phase == "" {
		cursor.Phase = "remote"
	}

	if len(cursor.SymbolNames) == 0 {
		// Set cursor symbol names if we haven't yet
		cursor.SymbolNames = symbolNames
	}

	// The following loop calls to fill additional results into the currently-being-constructed page.
	// Such a loop exists as each invocation of either phase may produce fewer results than the requested
	// page size. For example, if there are many references to a symbol over a large number of indexes but
	// each index has only a small number of locations, they can all be combined into a single page.
	// Running each phase multiple times and combining the results will create a full page, if the result
	// set was not exhausted), on each round-trip call to this service's method.

	for cursor.Phase != "done" {
		trace.AddEvent("Gather", attribute.String("phase", cursor.Phase), attribute.Int("numLocationsGathered", len(allLocations)))

		if len(allLocations) >= args.Limit {
			// we've filled our page, exit with current results
			break
		}

		var locations []shared.UploadLocation

		// N.B.: cursor is purposefully re-assigned here
		locations, cursor, err = s.gatherRemoteLocations(
			ctx,
			trace,
			args,
			requestState,
			cursor,
			tableName,
			includeReferencingIndexes,
			args.Limit-len(allLocations), // remaining space in the page
		)
		if err != nil {
			return nil, Cursor{}, err
		}
		allLocations = append(allLocations, locations...)
	}

	return allLocations, cursor, nil
}

func (s *Service) getVisibleUploadsFromCursor(
	ctx context.Context,
	args PositionalRequestArgs,
	requestState RequestState,
	cursor Cursor,
) ([]visibleUpload, Cursor, error) {
	if cursor.VisibleUploads != nil {
		visibleUploads := make([]visibleUpload, 0, len(cursor.VisibleUploads))
		for _, u := range cursor.VisibleUploads {
			upload, ok := requestState.dataLoader.GetUploadFromCacheMap(u.UploadID)
			if !ok {
				return nil, Cursor{}, ErrConcurrentModification
			}

			visibleUploads = append(visibleUploads, visibleUpload{
				Upload:                upload,
				TargetPath:            u.TargetPath,
				TargetPosition:        u.TargetPosition,
				TargetPathWithoutRoot: u.TargetPathWithoutRoot,
			})
		}

		return visibleUploads, cursor, nil
	}

	visibleUploads, err := s.getVisibleUploads(ctx, args.Line, args.Character, requestState)
	if err != nil {
		return nil, Cursor{}, err
	}

	cursorVisibleUpload := make([]CursorVisibleUpload, 0, len(visibleUploads))
	for i := range visibleUploads {
		cursorVisibleUpload = append(cursorVisibleUpload, CursorVisibleUpload{
			UploadID:              visibleUploads[i].Upload.ID,
			TargetPath:            visibleUploads[i].TargetPath,
			TargetPosition:        visibleUploads[i].TargetPosition,
			TargetPathWithoutRoot: visibleUploads[i].TargetPathWithoutRoot,
		})
	}

	cursor.VisibleUploads = cursorVisibleUpload
	return visibleUploads, cursor, nil
}

type gatherLocationsFunc func(
	ctx context.Context,
	trace observation.TraceLogger,
	args RequestArgs,
	requestState RequestState,
	tableName string,
	includeReferencingIndexes bool,
	cursor Cursor,
	limit int,
	extractor LocationExtractor,
	visibleUploads []visibleUpload,
) ([]shared.UploadLocation, Cursor, error)

const skipPrefix = "lsif ."

func (s *Service) gatherLocalLocations(
	ctx context.Context,
	trace observation.TraceLogger,
	args RequestArgs,
	requestState RequestState,
	tableName string,
	includeReferencingIndexes bool,
	cursor Cursor,
	limit int,
	extractor LocationExtractor,
	visibleUploads []visibleUpload,
) (allLocations []shared.UploadLocation, _ Cursor, _ error) {
	if cursor.Phase != "local" {
		// not our turn
		return nil, cursor, nil
	}
	if cursor.LocalUploadOffset >= len(visibleUploads) {
		// nothing left to do
		cursor.Phase = "remote"
		return nil, cursor, nil
	}
	unconsumedVisibleUploads := visibleUploads[cursor.LocalUploadOffset:]

	var unconsumedVisibleUploadIDs []int
	for _, u := range unconsumedVisibleUploads {
		unconsumedVisibleUploadIDs = append(unconsumedVisibleUploadIDs, u.Upload.ID)
	}
	trace.AddEvent("GatherLocalLocations", attribute.IntSlice("unconsumedVisibleUploadIDs", unconsumedVisibleUploadIDs))

	// Create local copy of mutable cursor scope and normalize it before use.
	// We will re-assign these values back to the response cursor before the
	// function exits.
	allSymbolNames := collections.NewSet(cursor.SymbolNames...)
	skipPathsByUploadID := cursor.SkipPathsByUploadID

	if skipPathsByUploadID == nil {
		// prevent writes to nil map
		skipPathsByUploadID = map[int]string{}
	}

	for _, visibleUpload := range unconsumedVisibleUploads {
		if len(allLocations) >= limit {
			// break if we've already hit our page maximum
			break
		}

		// Gather response locations directly from the document containing the
		// target position. This may also return relevant symbol names that we
		// collect for a remote search.
		locations, symbolNames, err := extractor.Extract(
			ctx,
			lsifstore.LocationKey{
				UploadID:  visibleUpload.Upload.ID,
				Path:      visibleUpload.TargetPathWithoutRoot,
				Line:      visibleUpload.TargetPosition.Line,
				Character: visibleUpload.TargetPosition.Character,
			},
		)
		if err != nil {
			return nil, Cursor{}, err
		}
		trace.AddEvent("ReadDocument", attribute.Int("numLocations", len(locations)), attribute.Int("numSymbolNames", len(symbolNames)))

		// remaining space in the page
		pageLimit := limit - len(allLocations)

		// Perform pagination on this level instead of in lsifstore; we bring back the
		// raw SCIP document payload anyway, so there's no reason to hide behind the API
		// that it's doing that amount of work.
		totalCount := len(locations)
		locations = pageSlice(locations, pageLimit, cursor.LocalLocationOffset)

		// adjust cursor offset for next page
		cursor = cursor.BumpLocalLocationOffset(len(locations), totalCount)

		// consume locations
		if len(locations) > 0 {
			adjustedLocations, err := s.getUploadLocations(
				ctx,
				args,
				requestState,
				locations,
				true,
			)
			if err != nil {
				return nil, Cursor{}, err
			}
			allLocations = append(allLocations, adjustedLocations...)

			// Stash paths with non-empty locations in the cursor so we can prevent
			// local and "remote" searches from returning duplicate sets of of target
			// ranges.
			skipPathsByUploadID[visibleUpload.Upload.ID] = visibleUpload.TargetPathWithoutRoot
		}

		// stash relevant symbol names in cursor
		for _, symbolName := range symbolNames {
			if !strings.HasPrefix(symbolName, skipPrefix) {
				allSymbolNames.Add(symbolName)
			}
		}
	}

	// re-assign mutable cursor scope to response cursor
	cursor.SymbolNames = allSymbolNames.Sorted(compareStrings)
	cursor.SkipPathsByUploadID = skipPathsByUploadID

	return allLocations, cursor, nil
}

func (s *Service) gatherRemoteLocationsShim(
	ctx context.Context,
	trace observation.TraceLogger,
	args RequestArgs,
	requestState RequestState,
	tableName string,
	includeReferencingIndexes bool,
	cursor Cursor,
	limit int,
	_ LocationExtractor,
	_ []visibleUpload,
) ([]shared.UploadLocation, Cursor, error) {
	return s.gatherRemoteLocations(
		ctx,
		trace,
		args,
		requestState,
		cursor,
		tableName,
		includeReferencingIndexes,
		limit,
	)
}

func (s *Service) gatherRemoteLocations(
	ctx context.Context,
	trace observation.TraceLogger,
	args RequestArgs,
	requestState RequestState,
	cursor Cursor,
	tableName string,
	includeReferencingIndexes bool,
	limit int,
) ([]shared.UploadLocation, Cursor, error) {
	if cursor.Phase != "remote" {
		// not our turn
		return nil, cursor, nil
	}
	trace.AddEvent("GatherRemoteLocations", attribute.StringSlice("symbolNames", cursor.SymbolNames))

	monikers, err := symbolsToMonikers(cursor.SymbolNames)
	if err != nil {
		return nil, Cursor{}, err
	}
	if len(monikers) == 0 {
		// no symbol names from local phase
		return nil, exhaustedCursor, nil
	}

	// Ensure we have a batch of upload ids over which to perform a symbol search, if such
	// a batch exists. This batch must be hydrated in the associated request data loader.
	// See the function body for additional complaints on this subject.
	//
	// N.B.: cursor is purposefully re-assigned here
	var includeFallbackLocations bool
	cursor, includeFallbackLocations, err = s.prepareCandidateUploads(
		ctx,
		trace,
		args,
		requestState,
		cursor,
		includeReferencingIndexes,
		monikers,
	)
	if err != nil {
		return nil, Cursor{}, err
	}

	// If we have no upload ids stashed in our cursor at this point then there are no more
	// uploads to search in and we've reached the end of our our result set. Congratulations!
	if len(cursor.UploadIDs) == 0 {
		return nil, exhaustedCursor, nil
	}
	trace.AddEvent("RemoteSymbolSearch", attribute.IntSlice("uploadIDs", cursor.UploadIDs))

	// Finally, query time!
	// Fetch indexed ranges of the given symbols within the given uploads.

	monikerArgs := make([]precise.MonikerData, 0, len(monikers))
	for _, moniker := range monikers {
		monikerArgs = append(monikerArgs, moniker.MonikerData)
	}
	locations, totalCount, err := s.lsifstore.GetMinimalBulkMonikerLocations(
		ctx,
		tableName,
		cursor.UploadIDs,
		cursor.SkipPathsByUploadID,
		monikerArgs,
		limit,
		cursor.RemoteLocationOffset,
	)
	if err != nil {
		return nil, Cursor{}, err
	}

	// adjust cursor offset for next page
	cursor = cursor.BumpRemoteLocationOffset(len(locations), totalCount)

	// Adjust locations back to target commit
	adjustedLocations, err := s.getUploadLocations(ctx, args, requestState, locations, includeFallbackLocations)
	if err != nil {
		return nil, Cursor{}, err
	}

	return adjustedLocations, cursor, nil
}

func (s *Service) prepareCandidateUploads(
	ctx context.Context,
	trace observation.TraceLogger,
	args RequestArgs,
	requestState RequestState,
	cursor Cursor,
	includeReferencingIndexes bool,
	monikers []precise.QualifiedMonikerData,
) (_ Cursor, fallback bool, _ error) {
	fallback = true // TODO - document

	// We always want to look into the uploads that define one of the symbols for our
	// "remote" phase. We'll conditionally also look at uploads that contain only a
	// reference (see below). We deal with the former set of uploads first in the
	// cursor.

	if len(cursor.DefinitionIDs) == 0 && len(cursor.UploadIDs) == 0 && cursor.RemoteUploadOffset == 0 {
		// N.B.: We only end up in in this branch on the first time it's invoked while
		// in the remote phase. If there truly are no definitions, we'll either have a
		// non-empty set of upload ids, or a non-zero remote upload offset on the next
		// invocation. If there are neither definitions nor an upload batch, we'll end
		// up returning an exhausted cursor from _this_ invocation.

		uploads, err := s.getUploadsWithDefinitionsForMonikers(ctx, monikers, requestState)
		if err != nil {
			return Cursor{}, false, err
		}
		idMap := make(map[int]struct{}, len(uploads)+len(cursor.VisibleUploads))
		for _, upload := range cursor.VisibleUploads {
			idMap[upload.UploadID] = struct{}{}
		}
		for _, upload := range uploads {
			idMap[upload.ID] = struct{}{}
		}
		ids := make([]int, 0, len(idMap))
		for id := range idMap {
			ids = append(ids, id)
		}
		sort.Ints(ids)

		fallback = false
		cursor.UploadIDs = ids
		cursor.DefinitionIDs = ids
		trace.AddEvent("Loaded indexes with definitions of symbols", attribute.IntSlice("ids", ids))
	}

	// TODO - redocument
	// This traversal isn't looking in uploads without definitions to one of the symbols
	if includeReferencingIndexes {
		// If we have no upload ids stashed in our cursor, then we'll try to fetch the next
		// batch of uploads in which we'll search for symbol names. If our remote upload offset
		// is set to -1 here, then it indicates the end of the set of relevant upload records.

		if len(cursor.UploadIDs) == 0 && cursor.RemoteUploadOffset != -1 {
			uploadIDs, _, totalCount, err := s.uploadSvc.GetUploadIDsWithReferences(
				ctx,
				monikers,
				cursor.DefinitionIDs,
				args.RepositoryID,
				args.Commit,
				requestState.maximumIndexesPerMonikerSearch, // limit
				cursor.RemoteUploadOffset,                   // offset
			)
			if err != nil {
				return Cursor{}, false, err
			}

			cursor.UploadIDs = uploadIDs
			trace.AddEvent("Loaded batch of indexes with references to symbols", attribute.IntSlice("ids", uploadIDs))

			// adjust cursor offset for next page
			cursor = cursor.BumpRemoteUploadOffset(len(uploadIDs), totalCount)
		}
	}

	// Hydrate upload records into the request state data loader. This must be called prior
	// to the invocation of getUploadLocation, which will silently throw out records belonging
	// to uploads that have not yet fetched from the database. We've assumed that the data loader
	// is consistently up-to-date with any extant upload identifier reference.
	//
	// FIXME: That's a dangerous design assumption we should get rid of.
	if _, err := s.getUploadsByIDs(ctx, cursor.UploadIDs, requestState); err != nil {
		return Cursor{}, false, err
	}

	return cursor, fallback, nil
}

//
//

func symbolsToMonikers(symbolNames []string) ([]precise.QualifiedMonikerData, error) {
	var monikers []precise.QualifiedMonikerData
	for _, symbolName := range symbolNames {
		parsedSymbol, err := scip.ParseSymbol(symbolName)
		if err != nil {
			return nil, err
		}
		if parsedSymbol.Package == nil {
			continue
		}

		monikers = append(monikers, precise.QualifiedMonikerData{
			MonikerData: precise.MonikerData{
				Scheme:     parsedSymbol.Scheme,
				Identifier: symbolName,
			},
			PackageInformationData: precise.PackageInformationData{
				Manager: parsedSymbol.Package.Manager,
				Name:    parsedSymbol.Package.Name,
				Version: parsedSymbol.Package.Version,
			},
		})
	}

	return monikers, nil
}

func pageSlice[T any](s []T, limit, offset int) []T {
	if offset < len(s) {
		s = s[offset:]
	} else {
		s = []T{}
	}

	if len(s) > limit {
		s = s[:limit]
	}

	return s
}

func compareStrings(a, b string) bool {
	return a < b
}
