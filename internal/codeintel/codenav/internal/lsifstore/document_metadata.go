package lsifstore

import (
	"context"
	"strings"

	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/scip/bindings/go/scip"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

// GetPathExists determines if the path exists in the database.
func (s *store) GetPathExists(ctx context.Context, bundleID int, path string) (_ bool, err error) {
	ctx, _, endObservation := s.operations.getPathExists.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("bundleID", bundleID),
		attribute.String("path", path),
	}})
	defer endObservation(1, observation.Args{})

	exists, _, err := basestore.ScanFirstBool(s.db.Query(ctx, sqlf.Sprintf(
		existsQuery,
		bundleID,
		path,
	)))
	return exists, err
}

const existsQuery = `
SELECT EXISTS (
	SELECT 1
	FROM codeintel_scip_document_lookup sid
	WHERE
		sid.upload_id = %s AND
		sid.document_path = %s
)
`

// Stencil returns all ranges within a single document.
func (s *store) GetStencil(ctx context.Context, bundleID int, path string) (_ []shared.Range, err error) {
	ctx, trace, endObservation := s.operations.getStencil.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("bundleID", bundleID),
		attribute.String("path", path),
	}})
	defer endObservation(1, observation.Args{})

	documentData, exists, err := s.scanFirstDocumentData(s.db.Query(ctx, sqlf.Sprintf(
		stencilQuery,
		bundleID,
		path,
	)))
	if err != nil || !exists {
		return nil, err
	}

	trace.AddEvent("TODO Domain Owner", attribute.Int("numOccurrences", len(documentData.SCIPData.Occurrences)))

	ranges := make([]shared.Range, 0, len(documentData.SCIPData.Occurrences))
	for _, occurrence := range documentData.SCIPData.Occurrences {
		ranges = append(ranges, translateRange(scip.NewRange(occurrence.Range)))
	}

	return ranges, nil
}

const stencilQuery = `
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

// GetRanges returns definition, reference, implementation, and hover data for each range within the given span of lines.
func (s *store) GetRanges(ctx context.Context, bundleID int, path string, startLine, endLine int) (_ []shared.CodeIntelligenceRange, err error) {
	ctx, _, endObservation := s.operations.getRanges.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("bundleID", bundleID),
		attribute.String("path", path),
		attribute.Int("startLine", startLine),
		attribute.Int("endLine", endLine),
	}})
	defer endObservation(1, observation.Args{})

	documentData, exists, err := s.scanFirstDocumentData(s.db.Query(ctx, sqlf.Sprintf(
		rangesDocumentQuery,
		bundleID,
		path,
	)))
	if err != nil || !exists {
		return nil, err
	}

	var ranges []shared.CodeIntelligenceRange
	for _, occurrence := range documentData.SCIPData.Occurrences {
		r := translateRange(scip.NewRange(occurrence.Range))

		if (startLine <= r.Start.Line && r.Start.Line < endLine) || (startLine <= r.End.Line && r.End.Line < endLine) {
			data := extractOccurrenceData(documentData.SCIPData, occurrence)

			ranges = append(ranges, shared.CodeIntelligenceRange{
				Range:           r,
				Definitions:     convertSCIPRangesToLocations(data.definitions, bundleID, path),
				References:      convertSCIPRangesToLocations(data.references, bundleID, path),
				Implementations: convertSCIPRangesToLocations(data.implementations, bundleID, path),
				HoverText:       strings.Join(data.hoverText, "\n"),
			})
		}
	}

	return ranges, nil
}

const rangesDocumentQuery = `
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

func convertSCIPRangesToLocations(ranges []*scip.Range, uploadID int, path string) []shared.Location {
	locations := make([]shared.Location, 0, len(ranges))
	for _, r := range ranges {
		locations = append(locations, shared.Location{
			UploadID: uploadID,
			Path:     path,
			Range:    translateRange(r),
		})
	}

	return locations
}
