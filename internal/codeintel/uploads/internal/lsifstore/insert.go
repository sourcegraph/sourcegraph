package lsifstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"sync/atomic"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/scip/bindings/go/scip"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/proto"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/ranges"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/symbols"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/batch"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// TODO - move
type SCIPDataStream struct {
	Metadata         ProcessedMetadata
	DocumentIterator SCIPDocumentVisitor
}

type SCIPDocumentVisitor interface {
	VisitAllDocuments(
		ctx context.Context,
		logger log.Logger,
		p *ProcessedPackageData,
		doIt func(ProcessedSCIPDocument) error,
	) error
}

type ProcessedPackageData struct {
	Packages          []precise.Package
	PackageReferences []precise.PackageReference
}

func (p *ProcessedPackageData) Normalize() {
	sort.Slice(p.Packages, func(i, j int) bool {
		return p.Packages[i].LessThan(&p.Packages[j])
	})
	sort.Slice(p.PackageReferences, func(i, j int) bool {
		return p.PackageReferences[i].Package.LessThan(&p.PackageReferences[j].Package)
	})
}

type ProcessedMetadata struct {
	TextDocumentEncoding string
	ToolName             string
	ToolVersion          string
	ToolArguments        []string
	ProtocolVersion      int
}

type ProcessedSCIPDocument struct {
	Path     string
	Document *scip.Document
	Err      error
}

func (s *store) InsertMetadata(ctx context.Context, uploadID int, meta ProcessedMetadata) (err error) {
	ctx, _, endObservation := s.operations.insertMetadata.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("uploadID", uploadID),
	}})
	defer endObservation(1, observation.Args{})

	if meta.ToolArguments == nil {
		meta.ToolArguments = []string{}
	}

	if err := s.db.Exec(ctx, sqlf.Sprintf(
		insertMetadataQuery,
		uploadID,
		meta.TextDocumentEncoding,
		meta.ToolName,
		meta.ToolVersion,
		pq.Array(meta.ToolArguments),
		meta.ProtocolVersion,
	)); err != nil {
		return err
	}

	return nil
}

const insertMetadataQuery = `
INSERT INTO codeintel_scip_metadata (upload_id, text_document_encoding, tool_name, tool_version, tool_arguments, protocol_version)
VALUES (%s, %s, %s, %s, %s, %s)
`

func (s *store) NewSCIPWriter(ctx context.Context, uploadID int) (SCIPWriter, error) {
	if !s.db.InTransaction() {
		return nil, errors.New("WriteSCIPSymbols must be called in a transaction")
	}

	if err := s.db.Exec(ctx, sqlf.Sprintf(newSCIPWriterTemporarySymbolsTableQuery)); err != nil {
		return nil, err
	}
	if err := s.db.Exec(ctx, sqlf.Sprintf(newSCIPWriterTemporarySymbolLookupTableQuery)); err != nil {
		return nil, err
	}
	if err := s.db.Exec(ctx, sqlf.Sprintf(newSCIPWriterTemporarySymbolLookupLeavesTableQuery)); err != nil {
		return nil, err
	}

	symbolInserter := batch.NewInserter(
		ctx,
		s.db.Handle(),
		"t_codeintel_scip_symbols",
		batch.MaxNumPostgresParameters,
		"symbol_id",
		"document_lookup_id",
		"definition_ranges",
		"reference_ranges",
		"implementation_ranges",
		"type_definition_ranges",
	)

	symbolLookupInserter := batch.NewInserter(
		ctx,
		s.db.Handle(),
		"t_codeintel_scip_symbols_lookup",
		batch.MaxNumPostgresParameters,
		"segment_type",
		"segment_quality",
		"name",
		"id",
		"parent_id",
	)

	symbolLookupLeavesInserter := batch.NewInserter(
		ctx,
		s.db.Handle(),
		"t_codeintel_scip_symbols_lookup_leaves",
		batch.MaxNumPostgresParameters,
		"symbol_id",
		"descriptor_suffix_id",
		"fuzzy_descriptor_suffix_id",
	)

	scipWriter := &scipWriter{
		uploadID:                   uploadID,
		db:                         s.db,
		symbolInserter:             symbolInserter,
		symbolLookupInserter:       symbolLookupInserter,
		symbolLookupLeavesInserter: symbolLookupLeavesInserter,
		count:                      0,
	}

	return scipWriter, nil
}

const newSCIPWriterTemporarySymbolsTableQuery = `
CREATE TEMPORARY TABLE t_codeintel_scip_symbols (
	symbol_id integer NOT NULL,
	document_lookup_id integer NOT NULL,
	definition_ranges bytea,
	reference_ranges bytea,
	implementation_ranges bytea,
	type_definition_ranges bytea
) ON COMMIT DROP
`

const newSCIPWriterTemporarySymbolLookupTableQuery = `
CREATE TEMPORARY TABLE t_codeintel_scip_symbols_lookup(
	id integer NOT NULL,
	name text NOT NULL,
	segment_type SymbolNameSegmentType NOT NULL,
	segment_quality SymbolNameSegmentQuality,
	parent_id integer
) ON COMMIT DROP
`

const newSCIPWriterTemporarySymbolLookupLeavesTableQuery = `
CREATE TEMPORARY TABLE t_codeintel_scip_symbols_lookup_leaves(
	symbol_id integer NOT NULL,
	descriptor_suffix_id integer NOT NULL,
	fuzzy_descriptor_suffix_id integer NOT NULL
) ON COMMIT DROP
`

type scipWriter struct {
	uploadID                   int
	nextSymbolLookupID         int
	nextSymbolID               int
	db                         *basestore.Store
	symbolInserter             *batch.Inserter
	symbolLookupInserter       *batch.Inserter
	symbolLookupLeavesInserter *batch.Inserter
	count                      uint32
	batchPayloadSum            int
	batch                      []bufferedDocument
}

type bufferedDocument struct {
	path         string
	scipDocument *scip.Document
	payload      []byte
	payloadHash  []byte
}

const (
	DocumentsBatchSize = 256
	MaxBatchPayloadSum = 1024 * 1024 * 32
)

func (s *scipWriter) InsertDocument(ctx context.Context, path string, scipDocument *scip.Document) error {
	if s.batchPayloadSum >= MaxBatchPayloadSum {
		if err := s.flush(ctx); err != nil {
			return err
		}
	}

	payload, err := proto.Marshal(scipDocument)
	if err != nil {
		return err
	}

	compressedPayload, err := shared.Compressor.Compress(bytes.NewReader(payload))
	if err != nil {
		return err
	}

	s.batch = append(s.batch, bufferedDocument{
		path:         path,
		scipDocument: scipDocument,
		payload:      compressedPayload,
		payloadHash:  hashPayload(payload),
	})
	s.batchPayloadSum += len(payload)

	if len(s.batch) >= DocumentsBatchSize {
		if err := s.flush(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *scipWriter) flush(ctx context.Context) error {
	documents := s.batch
	s.batch = nil
	s.batchPayloadSum = 0

	documentIDs, err := batch.WithInserterForIdentifiers(
		ctx,
		s.db.Handle(),
		"codeintel_scip_documents",
		batch.MaxNumPostgresParameters,
		[]string{
			"schema_version",
			"payload_hash",
			"raw_scip_payload",
		},
		"ON CONFLICT DO NOTHING",
		"id",
		func(inserter *batch.Inserter) error {
			for _, document := range documents {
				if err := inserter.Insert(ctx, 1, document.payloadHash, document.payload); err != nil {
					return err
				}
			}

			return nil
		},
	)
	if err != nil {
		return err
	}
	if len(documentIDs) != len(documents) {
		hashes := make([][]byte, 0, len(documents))
		hashSet := make(map[string]struct{}, len(documents))
		for _, document := range documents {
			key := hex.EncodeToString(document.payloadHash)
			if _, ok := hashSet[key]; !ok {
				hashSet[key] = struct{}{}
				hashes = append(hashes, document.payloadHash)
			}
		}
		idsByHash, err := scanIDsByHash(s.db.Query(ctx, sqlf.Sprintf(scipWriterWriteFetchDocumentsQuery, pq.Array(hashes))))
		if err != nil {
			return err
		}
		documentIDs = documentIDs[:0]
		for _, document := range documents {
			documentIDs = append(documentIDs, idsByHash[hex.EncodeToString(document.payloadHash)])
		}
		if len(idsByHash) != len(hashes) {
			return errors.New("unexpected number of document records inserted/retrieved")
		}
	}

	documentLookupIDs, err := batch.WithInserterForIdentifiers(
		ctx,
		s.db.Handle(),
		"codeintel_scip_document_lookup",
		batch.MaxNumPostgresParameters,
		[]string{
			"upload_id",
			"document_path",
			"document_id",
		},
		"",
		"id",
		func(inserter *batch.Inserter) error {
			for i, document := range documents {
				if err := inserter.Insert(ctx, s.uploadID, document.path, documentIDs[i]); err != nil {
					return err
				}
			}

			return nil
		},
	)
	if err != nil {
		return err
	}
	if len(documentLookupIDs) != len(documents) {
		return errors.New("unexpected number of document lookup records inserted")
	}

	symbolNameMap := map[string]struct{}{}
	invertedRangeIndexes := make([][]shared.InvertedRangeIndex, 0, len(documents))
	for _, document := range documents {
		index := shared.ExtractSymbolIndexes(document.scipDocument)
		invertedRangeIndexes = append(invertedRangeIndexes, index)

		for _, invertedRange := range index {
			symbolNameMap[invertedRange.SymbolName] = struct{}{}
		}
	}
	symbolNames := make([]string, 0, len(symbolNameMap))
	for symbolName := range symbolNameMap {
		symbolNames = append(symbolNames, symbolName)
	}
	sort.Strings(symbolNames)

	// Convert symbol names into a tree structure we'll insert into the database
	// All identifiers here are created ahead of the insertion so we do not need
	// to do multiple round-trips to get new insertion identifiers for pending
	// data - everything is known up-front.

	id := func() int { id := s.nextSymbolLookupID; s.nextSymbolLookupID++; return id }
	cache, traverser := constructSymbolLookupTable(symbolNames, id)

	// Bulk insert the content of the tree / descriptor-no-suffix map
	visit := func(segmentType string, segmentQuality *string, name string, id int, parentID *int) error {
		return s.symbolLookupInserter.Insert(ctx, segmentType, segmentQuality, name, id, parentID)
	}
	if err := traverser(visit); err != nil {
		return err
	}

	for i, invertedRangeIndexes := range invertedRangeIndexes {
		for _, index := range invertedRangeIndexes {
			definitionRanges, err := ranges.EncodeRanges(index.DefinitionRanges)
			if err != nil {
				return err
			}
			referenceRanges, err := ranges.EncodeRanges(index.ReferenceRanges)
			if err != nil {
				return err
			}
			implementationRanges, err := ranges.EncodeRanges(index.ImplementationRanges)
			if err != nil {
				return err
			}
			typeDefinitionRanges, err := ranges.EncodeRanges(index.TypeDefinitionRanges)
			if err != nil {
				return err
			}

			s.nextSymbolID++
			ids := cache[index.SymbolName]

			if err := s.symbolLookupLeavesInserter.Insert(
				ctx,
				s.nextSymbolID,
				ids.descriptorSuffixID,
				ids.fuzzyDescriptorSuffixID,
			); err != nil {
				return err
			}

			if err := s.symbolInserter.Insert(
				ctx,
				s.nextSymbolID,
				documentLookupIDs[i],
				definitionRanges,
				referenceRanges,
				implementationRanges,
				typeDefinitionRanges,
			); err != nil {
				return err
			}

			atomic.AddUint32(&s.count, 1)
		}
	}

	return nil
}

const scipWriterWriteFetchDocumentsQuery = `
SELECT
	encode(payload_hash, 'hex'),
	id
FROM codeintel_scip_documents
WHERE payload_hash = ANY(%s)
`

func (s *scipWriter) Flush(ctx context.Context) (uint32, error) {
	// Flush all buffered documents
	if err := s.flush(ctx); err != nil {
		return 0, err
	}

	// Flush all data into temp tables
	if err := s.symbolLookupInserter.Flush(ctx); err != nil {
		return 0, err
	}
	if err := s.symbolLookupLeavesInserter.Flush(ctx); err != nil {
		return 0, err
	}
	if err := s.symbolInserter.Flush(ctx); err != nil {
		return 0, err
	}

	// Move all data from temp tables into target tables
	if err := s.db.Exec(ctx, sqlf.Sprintf(scipWriterFlushSymbolLookupQuery, s.uploadID)); err != nil {
		return 0, err
	}
	if err := s.db.Exec(ctx, sqlf.Sprintf(scipWriterFlushSymbolLookupLeavesQuery, s.uploadID)); err != nil {
		return 0, err
	}
	if err := s.db.Exec(ctx, sqlf.Sprintf(scipWriterFlushSymbolsQuery, s.uploadID, 2)); err != nil {
		return 0, err
	}

	return s.count, nil
}

const scipWriterFlushSymbolLookupQuery = `
INSERT INTO codeintel_scip_symbols_lookup (
	upload_id,
	id,
	name,
	segment_type,
	segment_quality,
	parent_id
)
SELECT
	%s,
	source.id,
	source.name,
	source.segment_type,
	source.segment_quality,
	source.parent_id
FROM t_codeintel_scip_symbols_lookup source
`

const scipWriterFlushSymbolLookupLeavesQuery = `
INSERT INTO codeintel_scip_symbols_lookup_leaves (
	upload_id,
	symbol_id,
	descriptor_suffix_id,
	fuzzy_descriptor_suffix_id
)
SELECT
	%s,
	source.symbol_id,
	source.descriptor_suffix_id,
	source.fuzzy_descriptor_suffix_id
FROM t_codeintel_scip_symbols_lookup_leaves source
`

const scipWriterFlushSymbolsQuery = `
INSERT INTO codeintel_scip_symbols (
	upload_id,
	symbol_id,
	document_lookup_id,
	schema_version,
	definition_ranges,
	reference_ranges,
	implementation_ranges,
	type_definition_ranges
)
SELECT
	%s,
	source.symbol_id,
	source.document_lookup_id,
	%s,
	source.definition_ranges,
	source.reference_ranges,
	source.implementation_ranges,
	source.type_definition_ranges
FROM t_codeintel_scip_symbols source
`

// hashPayload returns a sha256 checksum of the given payload.
func hashPayload(payload []byte) []byte {
	hash := sha256.New()
	_, _ = hash.Write(payload)
	return hash.Sum(nil)
}

var scanIDsByHash = basestore.NewMapScanner(func(s dbutil.Scanner) (hash string, id int, _ error) {
	err := s.Scan(&hash, &id)
	return hash, id, err
})

// NOTE(scip-migration): This code also exists in the SCIP symbol names out-of-band migration.
// Changes (esp bug fixes) here that are backwards compatible should also be made in that copy
// as long as the migration has not been deprecated. Any backwards-incompatible changes should
// deprecate that migration and start a new version.
//
// See the migrator in .../codeintel/scip/symbols_migrator.go for more detail.

type explodedIDs struct {
	descriptorSuffixID      int
	fuzzyDescriptorSuffixID int
}

type visitFunc func(segmentType string, segmentQuality *string, name string, id int, parentID *int) error

func constructSymbolLookupTable(symbolNames []string, id func() int) (map[string]explodedIDs, func(visit visitFunc) error) {
	cache := map[string]explodedIDs{}     // Tracks symbol name -> identifiers in the scheme tree
	schemeTree := map[string]SchemeNode{} // Tracks scheme -> manager -> name -> version -> descriptor namespace -> descriptor suffix
	qualityMap := map[int]string{}        // Tracks descriptor node ids -> quality (PRECISE, FUZZY, BOTH)

	// Create helpers to create new tree nodes with (upload-)unique identifiers
	createSchemeNode := func() SchemeNode { return SchemeNode(newNodeWithID[PackageManagerNode](id())) }
	createPackageManagerNode := func() PackageManagerNode { return PackageManagerNode(newNodeWithID[PackageNameNode](id())) }
	createPackageNameNode := func() PackageNameNode { return PackageNameNode(newNodeWithID[PackageVersionNode](id())) }
	createPackageVersionNode := func() PackageVersionNode { return PackageVersionNode(newNodeWithID[NamespaceNode](id())) }
	createNamespaceNode := func() NamespaceNode { return NamespaceNode(newNodeWithID[DescriptorNode](id())) }
	createDescriptorWithQuality := func(quality string) func() DescriptorNode {
		return func() DescriptorNode {
			id := id()
			qualityMap[id] = quality
			return DescriptorNode(newNodeWithID[descriptor](id))
		}
	}
	createPreciseDescriptor := createDescriptorWithQuality("PRECISE")
	createFuzzyDescriptor := createDescriptorWithQuality("FUZZY")
	createUnionDescriptor := createDescriptorWithQuality("BOTH")

	for _, symbolName := range symbolNames {
		symbol, err := symbols.NewExplodedSymbol(symbolName)
		if err != nil {
			return nil, nil
		}

		// Assign the parts of the exploded symbol into the scheme tree. If a prefix of the exploded symbol
		// is already in the tree then existing nodes will be re-used. Laying out the exploded in a tree
		// structure will allow us to trace parentage (required for fast lookups) when we insert these into
		// the database.

		schemeNode := getOrCreate(schemeTree, symbol.Scheme, createSchemeNode)                                       // depth 0
		packageManagerNode := getOrCreate(schemeNode.children, symbol.PackageManager, createPackageManagerNode)      // depth 1
		packageNameNode := getOrCreate(packageManagerNode.children, symbol.PackageName, createPackageNameNode)       // depth 2
		packageVersionNode := getOrCreate(packageNameNode.children, symbol.PackageVersion, createPackageVersionNode) // depth 3
		namespace := getOrCreate(packageVersionNode.children, symbol.DescriptorNamespace, createNamespaceNode)       // depth 4
		descriptorSuffixID, fuzzyDescriptorSuffixID := getOrCreateLeafIDs(                                           // depth 5
			namespace,
			symbol.DescriptorSuffix, symbol.FuzzyDescriptorSuffix,
			createPreciseDescriptor, createFuzzyDescriptor, createUnionDescriptor,
		)

		cache[symbolName] = explodedIDs{
			descriptorSuffixID:      descriptorSuffixID,
			fuzzyDescriptorSuffixID: fuzzyDescriptorSuffixID,
		}
	}

	segmentTypeByDepth := []string{
		"SCHEME",               // depth 0
		"PACKAGE_MANAGER",      // depth 1
		"PACKAGE_NAME",         // depth 2
		"PACKAGE_VERSION",      // depth 3
		"DESCRIPTOR_NAMESPACE", // depth 4
		"DESCRIPTOR_SUFFIX",    // depth 5
		/*                   */ // depth PANIC
	}
	segmentQualityForID := func(id int) *string {
		if quality, ok := qualityMap[id]; ok {
			return &quality
		}

		return nil
	}

	traverser := func(visit visitFunc) error {
		if err := traverse(schemeTree, func(name string, id, depth int, parentID *int) error {
			return visit(segmentTypeByDepth[depth], segmentQualityForID(id), name, id, parentID)
		}); err != nil {
			return err
		}

		return nil
	}

	return cache, traverser
}

func getOrCreateLeafIDs(
	namespace treeNode[treeNode[descriptor]],
	descriptorSuffix, fuzzyDescriptorSuffix string,
	createPreciseDescriptor, createFuzzyDescriptor, createUnionDescriptor func() treeNode[descriptor],
) (int, int) {
	if descriptorSuffix == fuzzyDescriptorSuffix {
		// Common case: no difference - create a single leaf node
		descriptor := getOrCreate(namespace.children, descriptorSuffix, createUnionDescriptor)
		return descriptor.id, descriptor.id
	}

	// General case: unique fuzzy descriptor - create two leaf nodes
	descriptor := getOrCreate(namespace.children, descriptorSuffix, createPreciseDescriptor)
	fuzzyDescriptor := getOrCreate(namespace.children, fuzzyDescriptorSuffix, createFuzzyDescriptor)
	return descriptor.id, fuzzyDescriptor.id
}
