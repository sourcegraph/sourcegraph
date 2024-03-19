package lsifstore

import (
	"context"
	"strings"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"github.com/sourcegraph/scip/bindings/go/scip"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

// GetHover returns the hover text of the symbol at the given position.
func (s *store) GetHover(ctx context.Context, bundleID int, path string, line, character int) (_ string, _ shared.Range, _ bool, err error) {
	ctx, trace, endObservation := s.operations.getHover.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("bundleID", bundleID),
		attribute.String("path", path),
		attribute.Int("line", line),
		attribute.Int("character", character),
	}})
	defer endObservation(1, observation.Args{})

	documentData, exists, err := s.scanFirstDocumentData(s.db.Query(ctx, sqlf.Sprintf(
		hoverDocumentQuery,
		bundleID,
		path,
	)))
	if err != nil || !exists {
		return "", shared.Range{}, false, err
	}

	trace.AddEvent("SCIPData", attribute.Int("numOccurrences", len(documentData.SCIPData.Occurrences)))
	occurrences := scip.FindOccurrences(documentData.SCIPData.Occurrences, int32(line), int32(character))
	trace.AddEvent("FindOccurences", attribute.Int("numIntersectingOccurrences", len(occurrences)))

	for _, occurrence := range occurrences {
		// Return the hover data we can extract from the most specific occurrence
		if hoverText := extractHoverData(documentData.SCIPData, occurrence); len(hoverText) != 0 {
			return strings.Join(hoverText, "\n"), translateRange(scip.NewRange(occurrence.Range)), true, nil
		}
	}

	// We don't have any in-document symbol information with hover data, so we'll now attempt to
	// find the symbol information in the text document that defines a symbol attached to the target
	// occurrence.

	// First, we extract the symbol names and the range of the most specific occurrence associated
	// with it. We construct a map and a slice in parallel as we want to retain the ordering of
	// symbols when processing the documents below.

	symbolNames := make([]string, 0, len(occurrences))
	rangeBySymbol := make(map[string]shared.Range, len(occurrences))

	for _, occurrence := range occurrences {
		if occurrence.Symbol == "" || scip.IsLocalSymbol(occurrence.Symbol) {
			continue
		}

		if _, ok := rangeBySymbol[occurrence.Symbol]; !ok {
			symbolNames = append(symbolNames, occurrence.Symbol)
			rangeBySymbol[occurrence.Symbol] = translateRange(scip.NewRange(occurrence.Range))
		}
	}

	// Open documents from the same index that define one of the symbols. We return documents ordered
	// by path, which is arbitrary but deterministic in the case that multiple files mark a defining
	// occurrence of a symbol.

	documents, err := s.scanDocumentData(s.db.Query(ctx, sqlf.Sprintf(
		hoverSymbolsQuery,
		pq.Array(symbolNames),
		pq.Array([]int{bundleID}),
		bundleID,
	)))
	if err != nil {
		return "", shared.Range{}, false, err
	}

	// Re-perform the symbol information search. This loop is constructed to prefer matches for symbols
	// associated with the most specific occurrences over less specific occurrences. We also make the
	// observation that processing will inline equivalent symbol information nodes into multiple documents
	// in the persistence layer, so we return the first match rather than aggregating and de-duplicating
	// documentation over all matching documents.

	for _, symbolName := range symbolNames {
		for _, document := range documents {
			for _, symbol := range document.SCIPData.Symbols {
				if symbol.Symbol != symbolName {
					continue
				}

				// Return first match
				return strings.Join(symbol.Documentation, "\n"), rangeBySymbol[symbolName], true, nil
			}
		}
	}

	return "", shared.Range{}, false, nil
}

const hoverDocumentQuery = `
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

const symbolIDsCTEs = `
-- Search for the set of trie paths that match one of the given search terms. We
-- do a recursive walk starting at the roots of the trie for a given set of uploads,
-- and only traverse down trie paths that continue to match our search text.
matching_prefixes(upload_id, id, prefix, search) AS (
	(
		-- Base case: Select roots of the tries for this upload that are also a
		-- prefix of the search term. We cut the prefix we matched from our search
		-- term so that we only need to match the _next_ segment, not the entire
		-- reconstructed prefix so far (which is computationally more expensive).

		SELECT
			ssn.upload_id,
			ssn.id,
			ssn.name_segment,
			substring(t.name from length(ssn.name_segment) + 1) AS search
		FROM codeintel_scip_symbol_names ssn
		JOIN unnest(%s::text[]) AS t(name) ON t.name LIKE ssn.name_segment || '%%'
		WHERE
			ssn.upload_id = ANY(%s) AND
			ssn.prefix_id IS NULL AND
			t.name LIKE ssn.name_segment || '%%'
	) UNION (
		-- Iterative case: Follow the edges of the trie nodes in the worktable so far.
		-- If our search term is empty, then any children will be a proper superstring
		-- of our search term - exclude these. If our search term does not match the
		-- name segment, then we share some proper prefix with the search term but
		-- diverge - also exclude these. The remaining rows are all prefixes (or matches)
		-- of the target search term.

		SELECT
			ssn.upload_id,
			ssn.id,
			mp.prefix || ssn.name_segment,
			substring(mp.search from length(ssn.name_segment) + 1) AS search
		FROM matching_prefixes mp
		JOIN codeintel_scip_symbol_names ssn ON
			ssn.upload_id = mp.upload_id AND
			ssn.prefix_id = mp.id
		WHERE
			mp.search != '' AND
			mp.search LIKE ssn.name_segment || '%%'
	)
),

-- Consume from the worktable results defined above. This will throw out any rows
-- that still have a non-empty search field, as this indicates a proper prefix and
-- therefore a non-match. The remaining rows will all be exact matches.
matching_symbol_names AS (
	SELECT mp.upload_id, mp.id, mp.prefix AS symbol_name
	FROM matching_prefixes mp
	WHERE mp.search = ''
)
`

const hoverSymbolsQuery = `
WITH RECURSIVE
` + symbolIDsCTEs + `
SELECT
	sd.id,
	sid.document_path,
	sd.raw_scip_payload
FROM codeintel_scip_document_lookup sid
JOIN codeintel_scip_documents sd ON sd.id = sid.document_id
WHERE EXISTS (
	SELECT 1
	FROM codeintel_scip_symbols ss
	WHERE
		ss.upload_id = %s AND
		ss.symbol_id IN (SELECT id FROM matching_symbol_names) AND
		ss.document_lookup_id = sid.id AND
		ss.definition_ranges IS NOT NULL
)
`

// GetDiagnostics returns the diagnostics for the documents that have the given path prefix. This method
// also returns the size of the complete result set to aid in pagination.
func (s *store) GetDiagnostics(ctx context.Context, bundleID int, prefix string, limit, offset int) (_ []shared.Diagnostic, _ int, err error) {
	ctx, trace, endObservation := s.operations.getDiagnostics.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("bundleID", bundleID),
		attribute.String("prefix", prefix),
		attribute.Int("limit", limit),
		attribute.Int("offset", offset),
	}})
	defer endObservation(1, observation.Args{})

	documentData, err := s.scanDocumentData(s.db.Query(ctx, sqlf.Sprintf(
		diagnosticsQuery,
		bundleID,
		prefix+"%",
	)))
	if err != nil {
		return nil, 0, err
	}
	trace.AddEvent("scanDocumentData", attribute.Int("numDocuments", len(documentData)))

	totalCount := 0
	for _, documentData := range documentData {
		for _, occurrence := range documentData.SCIPData.Occurrences {
			totalCount += len(occurrence.Diagnostics)
		}
	}
	trace.AddEvent("found", attribute.Int("totalCount", totalCount))

	diagnostics := make([]shared.Diagnostic, 0, limit)
	for _, documentData := range documentData {
	occurrenceLoop:
		for _, occurrence := range documentData.SCIPData.Occurrences {
			if len(occurrence.Diagnostics) == 0 {
				continue
			}

			r := scip.NewRange(occurrence.Range)

			for _, diagnostic := range occurrence.Diagnostics {
				offset--

				if offset < 0 && len(diagnostics) < limit {
					diagnostics = append(diagnostics, shared.Diagnostic{
						UploadID: bundleID,
						Path:     documentData.Path,
						DiagnosticData: precise.DiagnosticData{
							Severity:       int(diagnostic.Severity),
							Code:           diagnostic.Code,
							Message:        diagnostic.Message,
							Source:         diagnostic.Source,
							StartLine:      int(r.Start.Line),
							StartCharacter: int(r.Start.Character),
							EndLine:        int(r.End.Line),
							EndCharacter:   int(r.End.Character),
						},
					})
				} else {
					break occurrenceLoop
				}
			}
		}
	}

	return diagnostics, totalCount, nil
}

const diagnosticsQuery = `
SELECT
	sd.id,
	sid.document_path,
	sd.raw_scip_payload
FROM codeintel_scip_document_lookup sid
JOIN codeintel_scip_documents sd ON sd.id = sid.document_id
WHERE
	sid.upload_id = %s AND
	sid.document_path LIKE %s
LIMIT 1
`
