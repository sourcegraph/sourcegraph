package lsifstore

import (
	"bytes"
	"context"
	"sort"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"github.com/sourcegraph/scip/bindings/go/scip"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/proto"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/symbols"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func (s *store) SCIPDocument(ctx context.Context, id int, path string) (_ *scip.Document, err error) {
	ctx, _, endObservation := s.operations.scipDocument.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("path", path),
		attribute.Int("uploadID", id),
	}})
	defer endObservation(1, observation.Args{})

	scanner := basestore.NewFirstScanner(func(dbs dbutil.Scanner) (*scip.Document, error) {
		var compressedSCIPPayload []byte
		if err := dbs.Scan(&compressedSCIPPayload); err != nil {
			return nil, err
		}

		scipPayload, err := shared.Decompressor.Decompress(bytes.NewReader(compressedSCIPPayload))
		if err != nil {
			return nil, err
		}

		var document scip.Document
		if err := proto.Unmarshal(scipPayload, &document); err != nil {
			return nil, err
		}
		return &document, nil
	})
	doc, _, err := scanner(s.db.Query(ctx, sqlf.Sprintf(fetchSCIPDocumentQuery, id, path)))
	return doc, err
}

const fetchSCIPDocumentQuery = `
SELECT sd.raw_scip_payload
FROM codeintel_scip_document_lookup sid
JOIN codeintel_scip_documents sd ON sd.id = sid.document_id
WHERE
	sid.upload_id = %s AND
	sid.document_path = %s
`

func (s *store) GetFullSCIPNameByDescriptor(ctx context.Context, uploadIDs []int, symbolNames []string) (names []*symbols.ExplodedSymbol, err error) {
	ctx, _, endObservation := s.operations.getFullSCIPNameByDescriptor.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{}})
	defer endObservation(1, observation.Args{})

	if len(uploadIDs) == 0 || len(symbolNames) == 0 {
		return nil, nil
	}

	return scanExplodedSymbols(s.db.Query(ctx, sqlf.Sprintf(
		getFullSCIPNameByDescriptorQuery,
		pq.Array(uploadIDs),
		sqlf.Join(fuzzyDescriptorSuffixConditions(symbolNames), "OR"),
	)))
}

const getFullSCIPNameByDescriptorQuery = `
WITH

-- Perform search for matching descriptors
fuzzy_descriptor_suffix_ids AS (
	SELECT ssl.upload_id, ssl.id
	FROM codeintel_scip_symbols_lookup ssl
	WHERE
		-- Index conditions for "codeintel_scip_symbols_lookup_reversed_descriptor_suffix_name"
		ssl.upload_id = ANY(%s) AND ssl.segment_type = 'DESCRIPTOR_SUFFIX' AND (%s) AND
		-- Post-index filter condition to ensure we haven't precise descriptors when we have an explicit fuzzy one
		ssl.segment_quality != 'PRECISE'
),

-- Translate fuzzy descriptor identifier into a precise identifier
-- We express this as a union in a CTE to take advantage of the partial indexes on the
-- descriptor and fuzzy descriptor suffixes. When we had inlined these expressions, Postgres
-- generated a parallel seq scan over the entire lookup_leaves table.
raw_descriptor_suffix_ids AS (
	(
		SELECT ll.upload_id, ll.descriptor_suffix_id FROM fuzzy_descriptor_suffix_ids dsi
		JOIN codeintel_scip_symbols_lookup_leaves ll ON (ll.upload_id = dsi.upload_id AND ll.fuzzy_descriptor_suffix_id = dsi.id)
	) UNION (
		SELECT ll.upload_id, ll.descriptor_suffix_id FROM fuzzy_descriptor_suffix_ids dsi
		JOIN codeintel_scip_symbols_lookup_leaves ll ON (ll.upload_id = dsi.upload_id AND ll.fuzzy_descriptor_suffix_id IS NULL AND ll.descriptor_suffix_id = dsi.id)
	)
),
descriptor_suffix_ids AS (
	SELECT DISTINCT dsi.upload_id, dsi.descriptor_suffix_id FROM raw_descriptor_suffix_ids dsi
)

--
-- Follow parent path from descriptor l6->l5->l4->l3->l2->l1
SELECT DISTINCT
    l1.name AS scheme,
    l2.name AS package_manager,
    l3.name AS package_name,
    l4.name AS package_version,
    l5.name AS descriptor_namespace,
    l6.name AS descriptor_suffix
FROM descriptor_suffix_ids dsi
JOIN codeintel_scip_symbols_lookup l6 ON l6.upload_id = dsi.upload_id AND l6.id = dsi.descriptor_suffix_id -- DESCRIPTOR_SUFFIX
JOIN codeintel_scip_symbols_lookup l5 ON l5.upload_id = dsi.upload_id AND l5.id = l6.parent_id             -- DESCRIPTOR_NAMESPACE
JOIN codeintel_scip_symbols_lookup l4 ON l4.upload_id = dsi.upload_id AND l4.id = l5.parent_id             -- PACKAGE_VERSION
JOIN codeintel_scip_symbols_lookup l3 ON l3.upload_id = dsi.upload_id AND l3.id = l4.parent_id             -- PACKAGE_NAME
JOIN codeintel_scip_symbols_lookup l2 ON l2.upload_id = dsi.upload_id AND l2.id = l3.parent_id             -- PACKAGE_MANAGER
JOIN codeintel_scip_symbols_lookup l1 ON l1.upload_id = dsi.upload_id AND l1.id = l2.parent_id             -- SCHEME
`

var scanExplodedSymbols = basestore.NewSliceScanner(func(s dbutil.Scanner) (*symbols.ExplodedSymbol, error) {
	var n symbols.ExplodedSymbol
	err := s.Scan(&n.Scheme, &n.PackageManager, &n.PackageName, &n.PackageVersion, &n.DescriptorNamespace, &n.DescriptorSuffix)
	return &n, err
})

func fuzzyDescriptorSuffixConditions(symbolNames []string) []*sqlf.Query {
	fuzzyDescriptorSuffixMap := make(map[string]struct{}, len(symbolNames))
	for _, symbolName := range symbolNames {
		ex, err := symbols.NewExplodedSymbol(symbolName)
		if err != nil {
			continue
		}
		symbol := ex.FuzzyDescriptorSuffix

		if symbol != "" {
			fuzzyDescriptorSuffixMap[symbol] = struct{}{}
		}
	}

	fuzzyDescriptorSuffixes := make([]string, 0, len(fuzzyDescriptorSuffixMap))
	for symbol := range fuzzyDescriptorSuffixMap {
		fuzzyDescriptorSuffixes = append(fuzzyDescriptorSuffixes, symbol)
	}
	sort.Strings(fuzzyDescriptorSuffixes)

	conds := make([]*sqlf.Query, 0, len(fuzzyDescriptorSuffixes))
	for _, descriptorSuffix := range fuzzyDescriptorSuffixes {
		conds = append(conds, sqlf.Sprintf("reverse(ssl.name) LIKE reverse('%%' || %s)", descriptorSuffix))
	}

	return conds
}
