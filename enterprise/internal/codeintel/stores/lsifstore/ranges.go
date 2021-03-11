package lsifstore

import (
	"context"
	"sort"

	"github.com/keegancsmith/sqlf"
	"github.com/opentracing/opentracing-go/log"

	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

// MaximumRangesDefinitionLocations is the maximum limit when querying definition locations for a
// Ranges request.
const MaximumRangesDefinitionLocations = 10000

// Ranges returns definition, reference, and hover data for each range within the given span of lines.
func (s *Store) Ranges(ctx context.Context, bundleID int, path string, startLine, endLine int) (_ []CodeIntelligenceRange, err error) {
	ctx, traceLog, endObservation := s.operations.ranges.WithAndLogger(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("bundleID", bundleID),
		log.String("path", path),
		log.Int("startLine", startLine),
		log.Int("endLine", endLine),
	}})
	defer endObservation(1, observation.Args{})

	documentData, exists, err := s.scanFirstDocumentData(s.Store.Query(ctx, sqlf.Sprintf(rangesDocumentQuery, bundleID, path)))
	if err != nil || !exists {
		return nil, err
	}

	traceLog(log.Int("numRanges", len(documentData.Document.Ranges)))
	ranges := semantic.FindRangesInWindow(documentData.Document.Ranges, startLine, endLine)
	traceLog(log.Int("numIntersectingRanges", len(ranges)))

	definitionResultIDs := extractResultIDs(ranges, func(r semantic.RangeData) semantic.ID { return r.DefinitionResultID })
	definitionLocations, _, err := s.locations(ctx, bundleID, definitionResultIDs, MaximumRangesDefinitionLocations, 0)
	if err != nil {
		return nil, err
	}

	referenceResultIDs := extractResultIDs(ranges, func(r semantic.RangeData) semantic.ID { return r.ReferenceResultID })
	referenceLocations, err := s.locationsWithinFile(ctx, bundleID, referenceResultIDs, path, documentData.Document)
	if err != nil {
		return nil, err
	}

	codeintelRanges := make([]CodeIntelligenceRange, 0, len(ranges))
	for _, r := range ranges {
		codeintelRanges = append(codeintelRanges, CodeIntelligenceRange{
			Range:       newRange(r.StartLine, r.StartCharacter, r.EndLine, r.EndCharacter),
			Definitions: definitionLocations[r.DefinitionResultID],
			References:  referenceLocations[r.ReferenceResultID],
			HoverText:   documentData.Document.HoverResults[r.HoverResultID],
		})
	}
	sort.Slice(codeintelRanges, func(i, j int) bool {
		return compareBundleRanges(codeintelRanges[i].Range, codeintelRanges[j].Range)
	})

	return codeintelRanges, nil
}

const rangesDocumentQuery = `
-- source: enterprise/internal/codeintel/stores/lsifstore/ranges.go:Ranges
SELECT dump_id, path, data FROM lsif_data_documents WHERE dump_id = %s AND path = %s LIMIT 1
`

// locationsWithinFile queries the file-local locations associated with the given definition or reference
// identifiers. Like locations, this method returns a map from result set identifiers to another map from
// document paths to locations within that document.
func (s *Store) locationsWithinFile(ctx context.Context, bundleID int, ids []semantic.ID, path string, documentData semantic.DocumentData) (_ map[semantic.ID][]Location, err error) {
	ctx, traceLog, endObservation := s.operations.locationsWithinFile.WithAndLogger(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("bundleID", bundleID),
		log.Int("numIDs", len(ids)),
		log.String("ids", idsToString(ids)),
		log.String("path", path),
	}})
	defer endObservation(1, observation.Args{})

	if len(ids) == 0 {
		return nil, nil
	}

	// Get the list of indexes we need to read in order to find each result set identifier
	indexes, err := s.translateIDsToResultChunkIndexes(ctx, bundleID, ids)
	if err != nil {
		return nil, err
	}
	traceLog(
		log.Int("numIndexes", len(indexes)),
		log.String("indexes", intsToString(indexes)),
	)

	// Read the result sets and gather the set of range identifiers we need to resolve with
	// the given document data.
	rangeIDsByResultID, _, err := s.readLocationsFromResultChunks(ctx, bundleID, ids, indexes, path)
	if err != nil {
		return nil, err
	}

	// Hydrate the locations result set by replacing range ids with their actual data from their
	// containing document. This refines the map constructed in the previous step.
	locationsByResultID := make(map[semantic.ID][]Location, len(ids))
	totalCount := s.readRangesFromDocument(bundleID, rangeIDsByResultID, locationsByResultID, path, documentData, traceLog)
	traceLog(log.Int("numLocations", totalCount))

	return locationsByResultID, nil
}
