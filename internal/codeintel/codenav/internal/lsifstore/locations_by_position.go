package lsifstore

import (
	"context"
	"fmt"
	"strings"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"github.com/sourcegraph/scip/bindings/go/scip"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

// GetDefinitionLocations returns the set of locations defining the symbol at the given position.
func (s *store) GetDefinitionLocations(ctx context.Context, bundleID int, path string, line, character, limit, offset int) (_ []shared.Location, _ int, err error) {
	return s.getLocations(ctx, "definition_ranges", extractDefinitionRanges, s.operations.getDefinitionLocations, bundleID, path, line, character, limit, offset)
}

// GetReferenceLocations returns the set of locations referencing the symbol at the given position.
func (s *store) GetReferenceLocations(ctx context.Context, bundleID int, path string, line, character, limit, offset int) (_ []shared.Location, _ int, err error) {
	return s.getLocations(ctx, "reference_ranges", extractReferenceRanges, s.operations.getReferenceLocations, bundleID, path, line, character, limit, offset)
}

// GetImplementationLocations returns the set of locations implementing the symbol at the given position.
func (s *store) GetImplementationLocations(ctx context.Context, bundleID int, path string, line, character, limit, offset int) (_ []shared.Location, _ int, err error) {
	return s.getLocations(ctx, "implementation_ranges", extractImplementationRanges, s.operations.getImplementationLocations, bundleID, path, line, character, limit, offset)
}

// GetPrototypeLocations returns the set of locations that are the prototypes of the symbol at the given position.
func (s *store) GetPrototypeLocations(ctx context.Context, bundleID int, path string, line, character, limit, offset int) (_ []shared.Location, _ int, err error) {
	return s.getLocations(ctx, "implementation_ranges", extractPrototypesRanges, s.operations.getPrototypesLocations, bundleID, path, line, character, limit, offset)
}

// GetBulkMonikerLocations returns the locations (within one of the given uploads) with an attached moniker
// whose scheme+identifier matches one of the given monikers. This method also returns the size of the
// complete result set to aid in pagination.
func (s *store) GetBulkMonikerLocations(ctx context.Context, tableName string, uploadIDs []int, monikers []precise.MonikerData, limit, offset int) (_ []shared.Location, totalCount int, err error) {
	ctx, trace, endObservation := s.operations.getBulkMonikerLocations.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("tableName", tableName),
		attribute.Int("numUploadIDs", len(uploadIDs)),
		attribute.IntSlice("uploadIDs", uploadIDs),
		attribute.Int("numMonikers", len(monikers)),
		attribute.String("monikers", monikersToString(monikers)),
		attribute.Int("limit", limit),
		attribute.Int("offset", offset),
	}})
	defer endObservation(1, observation.Args{})

	if len(uploadIDs) == 0 || len(monikers) == 0 {
		return nil, 0, nil
	}

	symbolNames := make([]string, 0, len(monikers))
	for _, arg := range monikers {
		symbolNames = append(symbolNames, arg.Identifier)
	}

	query := sqlf.Sprintf(
		bulkMonikerResultsQuery,
		pq.Array(symbolNames),
		pq.Array(uploadIDs),
		sqlf.Sprintf(fmt.Sprintf("%s_ranges", strings.TrimSuffix(tableName, "s"))),
	)

	locationData, err := s.scanQualifiedMonikerLocations(s.db.Query(ctx, query))
	if err != nil {
		return nil, 0, err
	}

	totalCount = 0
	for _, monikerLocations := range locationData {
		totalCount += len(monikerLocations.Locations)
	}
	trace.AddEvent("TODO Domain Owner",
		attribute.Int("numUploads", len(locationData)),
		attribute.Int("totalCount", totalCount))

	max := totalCount
	if totalCount > limit {
		max = limit
	}

	locations := make([]shared.Location, 0, max)
outer:
	for _, monikerLocations := range locationData {
		for _, row := range monikerLocations.Locations {
			offset--
			if offset >= 0 {
				continue
			}

			locations = append(locations, shared.Location{
				UploadID: monikerLocations.UploadID,
				Path:     row.URI,
				Range:    newRange(row.StartLine, row.StartCharacter, row.EndLine, row.EndCharacter),
			})

			if len(locations) >= limit {
				break outer
			}
		}
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("numLocations", len(locations)))

	return locations, totalCount, nil
}

const bulkMonikerResultsQuery = `
WITH RECURSIVE
` + symbolIDsCTEs + `
SELECT
	ss.upload_id,
	'scip',
	msn.symbol_name,
	%s,
	document_path
FROM matching_symbol_names msn
JOIN codeintel_scip_symbols ss ON ss.upload_id = msn.upload_id AND ss.symbol_id = msn.id
JOIN codeintel_scip_document_lookup dl ON dl.id = ss.document_lookup_id
ORDER BY ss.upload_id, msn.symbol_name
`

func (s *store) getLocations(
	ctx context.Context,
	scipFieldName string,
	scipExtractor func(*scip.Document, *scip.Occurrence) []*scip.Range,
	operation *observation.Operation,
	bundleID int,
	path string,
	line, character, limit, offset int,
) (_ []shared.Location, _ int, err error) {
	ctx, trace, endObservation := operation.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("bundleID", bundleID),
		attribute.String("path", path),
		attribute.Int("line", line),
		attribute.Int("character", character),
	}})
	defer endObservation(1, observation.Args{})

	documentData, exists, err := s.scanFirstDocumentData(s.db.Query(ctx, sqlf.Sprintf(
		locationsDocumentQuery,
		bundleID,
		path,
	)))
	if err != nil || !exists {
		return nil, 0, err
	}

	trace.AddEvent("SCIPData", attribute.Int("numOccurrences", len(documentData.SCIPData.Occurrences)))
	occurrences := scip.FindOccurrences(documentData.SCIPData.Occurrences, int32(line), int32(character))
	trace.AddEvent("FindOccurences", attribute.Int("numIntersectingOccurrences", len(occurrences)))

	for _, occurrence := range occurrences {
		var locations []shared.Location
		if ranges := scipExtractor(documentData.SCIPData, occurrence); len(ranges) != 0 {
			locations = append(locations, convertSCIPRangesToLocations(ranges, bundleID, path)...)
		}

		if occurrence.Symbol != "" && !scip.IsLocalSymbol(occurrence.Symbol) {
			monikerLocations, err := s.scanQualifiedMonikerLocations(s.db.Query(ctx, sqlf.Sprintf(
				locationsSymbolSearchQuery,
				pq.Array([]string{occurrence.Symbol}),
				pq.Array([]int{bundleID}),
				sqlf.Sprintf(scipFieldName),
				bundleID,
				path,
				sqlf.Sprintf(scipFieldName),
			)))
			if err != nil {
				return nil, 0, err
			}
			for _, monikerLocation := range monikerLocations {
				for _, row := range monikerLocation.Locations {
					locations = append(locations, shared.Location{
						UploadID: monikerLocation.UploadID,
						Path:     row.URI,
						Range:    newRange(row.StartLine, row.StartCharacter, row.EndLine, row.EndCharacter),
					})
				}
			}
		}

		if len(locations) > 0 {
			totalCount := len(locations)

			if offset < len(locations) {
				locations = locations[offset:]
			} else {
				locations = []shared.Location{}
			}

			if len(locations) > limit {
				locations = locations[:limit]
			}

			return locations, totalCount, nil
		}
	}

	return nil, 0, nil
}

const locationsDocumentQuery = `
SELECT
	sd.id,
	sid.document_path,
	sd.raw_scip_payload
FROM codeintel_scip_document_lookup sid
JOIN codeintel_scip_documents sd ON sd.id = sid.document_id
WHERE
	sid.upload_id = %s AND
	sid.document_path = %s
LIMIT 1
`

const locationsSymbolSearchQuery = `
WITH RECURSIVE
` + symbolIDsCTEs + `
SELECT
	ss.upload_id,
	'' AS scheme,
	'' AS identifier,
	ss.%s,
	sid.document_path
FROM codeintel_scip_symbols ss
JOIN codeintel_scip_document_lookup sid ON sid.id = ss.document_lookup_id
JOIN matching_symbol_names msn ON msn.id = ss.symbol_id
WHERE
	ss.upload_id = %s AND
	sid.document_path != %s AND
	ss.%s IS NOT NULL
`

type extractedOccurrenceData struct {
	definitions     []*scip.Range
	references      []*scip.Range
	implementations []*scip.Range
	prototypes      []*scip.Range
	hoverText       []string
}

func extractDefinitionRanges(document *scip.Document, occurrence *scip.Occurrence) []*scip.Range {
	return extractOccurrenceData(document, occurrence).definitions
}

func extractReferenceRanges(document *scip.Document, occurrence *scip.Occurrence) []*scip.Range {
	return extractOccurrenceData(document, occurrence).references
}

func extractImplementationRanges(document *scip.Document, occurrence *scip.Occurrence) []*scip.Range {
	return extractOccurrenceData(document, occurrence).implementations
}

func extractPrototypesRanges(document *scip.Document, occurrence *scip.Occurrence) []*scip.Range {
	return extractOccurrenceData(document, occurrence).prototypes
}

func extractHoverData(document *scip.Document, occurrence *scip.Occurrence) []string {
	return extractOccurrenceData(document, occurrence).hoverText
}

func extractOccurrenceData(document *scip.Document, occurrence *scip.Occurrence) extractedOccurrenceData {
	if occurrence.Symbol == "" {
		return extractedOccurrenceData{
			hoverText: occurrence.OverrideDocumentation,
		}
	}

	var (
		hoverText               []string
		definitionSymbol        = occurrence.Symbol
		referencesBySymbol      = map[string]struct{}{}
		implementationsBySymbol = map[string]struct{}{}
		prototypeBySymbol       = map[string]struct{}{}
	)

	// Extract hover text and relationship data from the symbol information that
	// matches the given occurrence. This will give us additional symbol names that
	// we should include in reference and implementation searches.

	if symbol := scip.FindSymbol(document, occurrence.Symbol); symbol != nil {
		hoverText = symbol.Documentation

		for _, rel := range symbol.Relationships {
			if rel.IsDefinition {
				definitionSymbol = rel.Symbol
			}
			if rel.IsReference {
				referencesBySymbol[rel.Symbol] = struct{}{}
			}
			if rel.IsImplementation {
				prototypeBySymbol[rel.Symbol] = struct{}{}
			}
		}
	}

	for _, sym := range document.Symbols {
		for _, rel := range sym.Relationships {
			if rel.IsImplementation {
				if rel.Symbol == occurrence.Symbol {
					implementationsBySymbol[sym.Symbol] = struct{}{}
				}
			}
		}
	}

	definitions := []*scip.Range{}
	references := []*scip.Range{}
	implementations := []*scip.Range{}
	prototypes := []*scip.Range{}

	// Include original symbol names for reference search below
	referencesBySymbol[occurrence.Symbol] = struct{}{}

	// For each occurrence that references one of the definition, reference, or
	// implementation symbol names, extract and aggregate their source positions.

	for _, occ := range document.Occurrences {
		isDefinition := scip.SymbolRole_Definition.Matches(occ)

		// This occurrence defines this symbol
		if definitionSymbol == occ.Symbol && isDefinition {
			definitions = append(definitions, scip.NewRange(occ.Range))
		}

		// This occurrence references this symbol (or a sibling of it)
		if _, ok := referencesBySymbol[occ.Symbol]; ok && !isDefinition {
			references = append(references, scip.NewRange(occ.Range))
		}

		// This occurrence is a definition of a symbol with an implementation relationship
		if _, ok := implementationsBySymbol[occ.Symbol]; ok && isDefinition && definitionSymbol != occ.Symbol {
			implementations = append(implementations, scip.NewRange(occ.Range))
		}

		// This occurrence is a definition of a symbol with a prototype relationship
		if _, ok := prototypeBySymbol[occ.Symbol]; ok && isDefinition {
			prototypes = append(prototypes, scip.NewRange(occ.Range))
		}
	}

	// Override symbol documentation with occurrence documentation, if it exists
	if len(occurrence.OverrideDocumentation) != 0 {
		hoverText = occurrence.OverrideDocumentation
	}

	return extractedOccurrenceData{
		definitions:     definitions,
		references:      references,
		implementations: implementations,
		hoverText:       hoverText,
		prototypes:      prototypes,
	}
}

func monikersToString(vs []precise.MonikerData) string {
	strs := make([]string, 0, len(vs))
	for _, v := range vs {
		strs = append(strs, fmt.Sprintf("%s:%s:%s", v.Kind, v.Scheme, v.Identifier))
	}

	return strings.Join(strs, ", ")
}

//
//

func (s *store) ExtractDefinitionLocationsFromPosition(ctx context.Context, locationKey LocationKey) (_ []shared.Location, _ []string, err error) {
	return s.extractLocationsFromPosition(ctx, extractDefinitionRanges, symbolExtractDefault, s.operations.getDefinitionLocations, locationKey)
}

func (s *store) ExtractReferenceLocationsFromPosition(ctx context.Context, locationKey LocationKey) (_ []shared.Location, _ []string, err error) {
	return s.extractLocationsFromPosition(ctx, extractReferenceRanges, symbolExtractDefault, s.operations.getReferenceLocations, locationKey)
}

func (s *store) ExtractImplementationLocationsFromPosition(ctx context.Context, locationKey LocationKey) (_ []shared.Location, _ []string, err error) {
	return s.extractLocationsFromPosition(ctx, extractImplementationRanges, symbolExtractImplementations, s.operations.getImplementationLocations, locationKey)
}

func (s *store) ExtractPrototypeLocationsFromPosition(ctx context.Context, locationKey LocationKey) (_ []shared.Location, _ []string, err error) {
	return s.extractLocationsFromPosition(ctx, extractPrototypesRanges, symbolExtractPrototype, s.operations.getPrototypesLocations, locationKey)
}

func symbolExtractDefault(document *scip.Document, symbolName string) (symbols []string) {
	if symbol := scip.FindSymbol(document, symbolName); symbol != nil {
		for _, rel := range symbol.Relationships {
			if rel.IsReference {
				symbols = append(symbols, rel.Symbol)
			}
		}
	}

	return append(symbols, symbolName)
}

func symbolExtractImplementations(document *scip.Document, symbolName string) (symbols []string) {
	for _, sym := range document.Symbols {
		for _, rel := range sym.Relationships {
			if rel.IsImplementation {
				if rel.Symbol == symbolName {
					symbols = append(symbols, sym.Symbol)
				}
			}
		}
	}

	return append(symbols, symbolName)
}

func symbolExtractPrototype(document *scip.Document, symbolName string) (symbols []string) {
	if symbol := scip.FindSymbol(document, symbolName); symbol != nil {
		for _, rel := range symbol.Relationships {
			if rel.IsImplementation {
				symbols = append(symbols, rel.Symbol)
			}
		}
	}

	return symbols
}

//
//

func (s *store) extractLocationsFromPosition(
	ctx context.Context,
	extractRanges func(document *scip.Document, occurrence *scip.Occurrence) []*scip.Range,
	extractSymbolNames func(document *scip.Document, symbolName string) []string,
	operation *observation.Operation,
	locationKey LocationKey,
) (_ []shared.Location, _ []string, err error) {
	ctx, trace, endObservation := operation.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("bundleID", locationKey.UploadID),
		attribute.String("path", locationKey.Path),
		attribute.Int("line", locationKey.Line),
		attribute.Int("character", locationKey.Character),
	}})
	defer endObservation(1, observation.Args{})

	documentData, exists, err := s.scanFirstDocumentData(s.db.Query(ctx, sqlf.Sprintf(
		locationsDocumentQuery,
		locationKey.UploadID,
		locationKey.Path,
	)))
	if err != nil || !exists {
		return nil, nil, err
	}

	trace.AddEvent("SCIPData", attribute.Int("numOccurrences", len(documentData.SCIPData.Occurrences)))
	occurrences := scip.FindOccurrences(documentData.SCIPData.Occurrences, int32(locationKey.Line), int32(locationKey.Character))
	trace.AddEvent("FindOccurences", attribute.Int("numIntersectingOccurrences", len(occurrences)))

	var locations []shared.Location
	var symbols []string
	for _, occurrence := range occurrences {
		if ranges := extractRanges(documentData.SCIPData, occurrence); len(ranges) != 0 {
			locations = append(locations, convertSCIPRangesToLocations(ranges, locationKey.UploadID, locationKey.Path)...)
		}

		if occurrence.Symbol != "" && !scip.IsLocalSymbol(occurrence.Symbol) {
			symbols = append(symbols, extractSymbolNames(documentData.SCIPData, occurrence.Symbol)...)
		}
	}

	return deduplicateLocations(locations), deduplicate(symbols, func(s string) string { return s }), nil
}

func deduplicate[T any](locations []T, keyFn func(T) string) []T {
	seen := map[string]struct{}{}

	filtered := locations[:0]
	for _, l := range locations {
		k := keyFn(l)
		if _, ok := seen[k]; ok {
			continue
		}

		seen[k] = struct{}{}
		filtered = append(filtered, l)
	}

	return filtered
}

func deduplicateLocations(locations []shared.Location) []shared.Location {
	return deduplicate(locations, locationKey)
}

func locationKey(l shared.Location) string {
	return fmt.Sprintf("%d:%s:%d:%d:%d:%d",
		l.UploadID,
		l.Path,
		l.Range.Start.Line,
		l.Range.Start.Character,
		l.Range.End.Line,
		l.Range.End.Character,
	)
}

//
//

func (s *store) GetMinimalBulkMonikerLocations(ctx context.Context, tableName string, uploadIDs []int, skipPaths map[int]string, monikers []precise.MonikerData, limit, offset int) (_ []shared.Location, totalCount int, err error) {
	ctx, trace, endObservation := s.operations.getBulkMonikerLocations.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("tableName", tableName),
		attribute.Int("numUploadIDs", len(uploadIDs)),
		attribute.IntSlice("uploadIDs", uploadIDs),
		attribute.Int("numMonikers", len(monikers)),
		attribute.String("monikers", monikersToString(monikers)),
		attribute.Int("limit", limit),
		attribute.Int("offset", offset),
	}})
	defer endObservation(1, observation.Args{})

	if len(uploadIDs) == 0 || len(monikers) == 0 {
		return nil, 0, nil
	}

	symbolNames := make([]string, 0, len(monikers))
	for _, arg := range monikers {
		symbolNames = append(symbolNames, arg.Identifier)
	}

	var skipConds []*sqlf.Query
	for _, id := range uploadIDs {
		if path, ok := skipPaths[id]; ok {
			skipConds = append(skipConds, sqlf.Sprintf("(%s, %s)", id, path))
		}
	}
	if len(skipConds) == 0 {
		skipConds = append(skipConds, sqlf.Sprintf("(%s, %s)", -1, ""))
	}

	fieldName := fmt.Sprintf("%s_ranges", strings.TrimSuffix(tableName, "s"))
	query := sqlf.Sprintf(
		minimalBulkMonikerResultsQuery,
		pq.Array(symbolNames),
		pq.Array(uploadIDs),
		sqlf.Sprintf(fieldName),
		sqlf.Sprintf(fieldName),
		sqlf.Join(skipConds, ", "),
	)

	locationData, err := s.scanDeduplicatedQualifiedMonikerLocations(s.db.Query(ctx, query))
	if err != nil {
		return nil, 0, err
	}

	totalCount = 0
	for _, monikerLocations := range locationData {
		totalCount += len(monikerLocations.Locations)
	}
	trace.AddEvent("TODO Domain Owner",
		attribute.Int("numUploads", len(locationData)),
		attribute.Int("totalCount", totalCount))

	max := totalCount
	if totalCount > limit {
		max = limit
	}

	locations := make([]shared.Location, 0, max)
outer:
	for _, monikerLocations := range locationData {
		for _, row := range monikerLocations.Locations {
			offset--
			if offset >= 0 {
				continue
			}

			locations = append(locations, shared.Location{
				UploadID: monikerLocations.UploadID,
				Path:     row.URI,
				Range:    newRange(row.StartLine, row.StartCharacter, row.EndLine, row.EndCharacter),
			})

			if len(locations) >= limit {
				break outer
			}
		}
	}
	trace.AddEvent("TODO Domain Owner", attribute.Int("numLocations", len(locations)))

	return locations, totalCount, nil
}

const minimalBulkMonikerResultsQuery = `
WITH RECURSIVE
` + symbolIDsCTEs + `
SELECT
	ss.upload_id,
	%s,
	document_path
FROM codeintel_scip_symbols ss
JOIN codeintel_scip_document_lookup dl ON dl.id = ss.document_lookup_id
JOIN matching_symbol_names msn ON msn.upload_id = ss.upload_id AND msn.id = ss.symbol_id
WHERE
	ss.%s IS NOT NULL AND
	(ss.upload_id, dl.document_path) NOT IN (%s)
ORDER BY ss.upload_id, dl.document_path
`
