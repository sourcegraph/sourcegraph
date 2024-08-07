package codegraph

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sort"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/proto"

	"github.com/sourcegraph/scip/bindings/go/scip"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/ranges"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/trie"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/collections"
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

func (s *store) NewSyntacticSCIPWriter(uploadID int) (SCIPWriter, error) {
	scipWriter := &scipWriter{
		uploadID:             uploadID,
		db:                   s.db,
		insertedSymbolsCount: 0,
		isSyntactic:          true,
		operations:           s.operations.writerOperations,
	}
	return scipWriter, nil
}

func (s *store) NewPreciseSCIPWriter(ctx context.Context, uploadID int) (SCIPWriter, error) {
	if !s.db.InTransaction() {
		return nil, errors.New("WriteSCIPSymbols must be called in a transaction")
	}

	if err := s.db.Exec(ctx, sqlf.Sprintf(newSCIPWriterTemporarySymbolNamesTableQuery)); err != nil {
		return nil, err
	}
	if err := s.db.Exec(ctx, sqlf.Sprintf(newSCIPWriterTemporarySymbolsTableQuery)); err != nil {
		return nil, err
	}

	symbolNameInserter := batch.NewInserter(
		ctx,
		s.db.Handle(),
		"t_codeintel_scip_symbol_names",
		batch.MaxNumPostgresParameters,
		"id",
		"name_segment",
		"prefix_id",
	)

	symbolInserter := batch.NewInserter(
		ctx,
		s.db.Handle(),
		"t_codeintel_scip_symbols",
		batch.MaxNumPostgresParameters,
		"document_lookup_id",
		"symbol_id",
		"definition_ranges",
		"reference_ranges",
		"implementation_ranges",
		"type_definition_ranges",
	)

	scipWriter := &scipWriter{
		uploadID:             uploadID,
		db:                   s.db,
		symbolNameInserter:   symbolNameInserter,
		symbolInserter:       symbolInserter,
		insertedSymbolsCount: 0,
		operations:           s.operations.writerOperations,
	}

	return scipWriter, nil
}

const newSCIPWriterTemporarySymbolNamesTableQuery = `
CREATE TEMPORARY TABLE t_codeintel_scip_symbol_names (
	id integer NOT NULL,
	name_segment text NOT NULL,
	prefix_id integer
) ON COMMIT DROP
`

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

type scipWriter struct {
	uploadID             int
	nextID               int
	isSyntactic          bool
	db                   *basestore.Store
	symbolNameInserter   *batch.Inserter
	symbolInserter       *batch.Inserter
	insertedSymbolsCount uint32
	batchPayloadSum      int
	batch                []bufferedDocument
	operations           writerOperations
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
		if err := s.flushDocuments(ctx); err != nil {
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
		if err := s.flushDocuments(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *scipWriter) flushDocuments(ctx context.Context) error {
	documents := s.batch
	s.batch = nil
	totalDocumentSize := s.batchPayloadSum
	s.batchPayloadSum = 0

	documentIDs, err := s.insertDocuments(ctx, documents, totalDocumentSize)
	if err != nil {
		return err
	}

	documentLookupIDs, err := s.insertDocumentLookups(ctx, documents, documentIDs)
	if err != nil {
		return err
	}

	// We don't write symbols for syntactic indexes, because we can't perform
	// fuzzy matching against the trie datastructure we build for symbols in the database
	if s.isSyntactic {
		return nil
	}

	invertedRangeIndexes, symbolNameTrie := s.constructTrie(ctx, documents)

	idsBySymbolName := map[string]int{}
	if err = s.insertSymbolNames(ctx, symbolNameTrie, idsBySymbolName); err != nil {
		return err
	}

	if err = s.insertSymbols(ctx, invertedRangeIndexes, idsBySymbolName, documentLookupIDs); err != nil {
		return err
	}

	return nil
}

// Post-condition: len(documentIDs) == len(documents)
func (s *scipWriter) insertDocuments(ctx context.Context, documents []bufferedDocument, totalDocumentSize int) (documentIDs []int, err error) {
	ctx, _, endObservation := s.operations.insertDocuments.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numDocuments", len(documents)),
		attribute.Int("totalDocumentSize", totalDocumentSize),
	}})
	defer endObservation(1, observation.Args{})

	documentIDs, err = batch.WithInserterForIdentifiers(
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
		return nil, err
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
			return nil, err
		}
		documentIDs = documentIDs[:0]
		for _, document := range documents {
			documentIDs = append(documentIDs, idsByHash[hex.EncodeToString(document.payloadHash)])
		}
		if len(idsByHash) != len(hashes) {
			return nil, errors.New("unexpected number of document records inserted/retrieved")
		}
	}

	return documentIDs, nil
}

// Pre-condition: len(documents) == len(documentIDs)
func (s *scipWriter) insertDocumentLookups(ctx context.Context, documents []bufferedDocument, documentIDs []int) (documentLookupIDs []int, err error) {
	ctx, _, endObservation := s.operations.insertDocumentLookups.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numDocuments", len(documents)),
	}})
	defer endObservation(1, observation.Args{})

	documentLookupIDs, err = batch.WithInserterForIdentifiers(
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
		return nil, err
	}
	if len(documentLookupIDs) != len(documents) {
		return nil, errors.New("unexpected number of document lookup records inserted")
	}

	return documentLookupIDs, nil
}

func ptrToZero() *int {
	z := 0
	return &z
}

func (s *scipWriter) constructTrie(ctx context.Context, documents []bufferedDocument) ([][]shared.InvertedRangeIndex, trie.Trie) {
	var err error
	_, _, endObservation := s.operations.constructTrie.With(ctx, &err, observation.Args{})
	numSymbolNames := ptrToZero()
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("numSymbolNames", *numSymbolNames),
		}})
	}()

	symbolNameSet := collections.NewSet[string]()
	invertedRangeIndexes := make([][]shared.InvertedRangeIndex, 0, len(documents))
	for _, document := range documents {
		index := shared.ExtractSymbolIndexes(document.scipDocument)
		invertedRangeIndexes = append(invertedRangeIndexes, index)

		for _, invertedRange := range index {
			symbolNameSet.Add(invertedRange.SymbolName)
		}
	}
	symbolNames := make([]string, 0, len(symbolNameSet))
	for symbolName := range symbolNameSet {
		symbolNames = append(symbolNames, symbolName)
	}
	sort.Strings(symbolNames)
	*numSymbolNames = len(symbolNames)

	var symbolNameTrie trie.Trie
	symbolNameTrie, s.nextID = trie.NewTrie(symbolNames, s.nextID)
	return invertedRangeIndexes, symbolNameTrie
}

func (s *scipWriter) insertSymbolNames(ctx context.Context, symbolNameTrie trie.Trie, idsBySymbolName map[string]int) (err error) {
	ctx, _, endObservation := s.operations.insertSymbolNames.With(ctx, &err, observation.Args{})
	// In general, this number will be higher than numSymbolNames because of the prefix tree structure.
	// For example, if making a prefix tree with ['abc','abd'] will give 3 nodes ['ab','c','d'].
	symbolNameFragmentCount := ptrToZero()
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("numSymbolNameFragments", *symbolNameFragmentCount),
		}})
	}()

	symbolNameByIDs := map[int]string{}
	return symbolNameTrie.Traverse(func(id int, parentID *int, prefix string) error {
		name := prefix
		if parentID != nil {
			parentPrefix, ok := symbolNameByIDs[*parentID]
			if !ok {
				return errors.Newf("malformed trie - expected prefix with id=%d to exist", *parentID)
			}

			name = parentPrefix + prefix
		}
		symbolNameByIDs[id] = name
		idsBySymbolName[name] = id

		if err := s.symbolNameInserter.Insert(ctx, id, prefix, parentID); err != nil {
			return err
		}
		*symbolNameFragmentCount = *symbolNameFragmentCount + 1

		return nil
	})
}

func (s *scipWriter) insertSymbols(ctx context.Context, invertedRangeIndexes [][]shared.InvertedRangeIndex, idsBySymbolName map[string]int, documentLookupIDs []int) (err error) {
	ctx, _, endObservation := s.operations.insertSymbols.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int("numSymbolIDs", len(idsBySymbolName)),
	}})
	rangeCount := ptrToZero()
	insertCount := ptrToZero()
	defer func() {
		endObservation(1, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Int("numRanges", *rangeCount),
			attribute.Int("numInserts", *insertCount),
		}})
	}()

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
			*rangeCount = *rangeCount + ((len(index.DefinitionRanges) + len(index.ReferenceRanges) +
				len(index.ImplementationRanges) + len(index.TypeDefinitionRanges)) / 4)

			symbolID, ok := idsBySymbolName[index.SymbolName]
			if !ok {
				return errors.Newf("malformed trie - expected %q to be a member", index.SymbolName)
			}

			if err := s.symbolInserter.Insert(
				ctx,
				documentLookupIDs[i],
				symbolID,
				definitionRanges,
				referenceRanges,
				implementationRanges,
				typeDefinitionRanges,
			); err != nil {
				return err
			}
			*insertCount = *insertCount + 1
			s.insertedSymbolsCount = s.insertedSymbolsCount + 1
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
	if err := s.flushDocuments(ctx); err != nil {
		return 0, err
	}
	// We don't need to flush symbols for syntactic indexes, as we don't write any for them
	if s.isSyntactic {
		return s.insertedSymbolsCount, nil
	}
	if err := s.flushSymbolNames(ctx); err != nil {
		return 0, err
	}
	if err := s.flushSymbols(ctx); err != nil {
		return 0, err
	}
	return s.insertedSymbolsCount, nil
}

func (s *scipWriter) flushSymbols(ctx context.Context) (err error) {
	ctx, _, endObservation := s.operations.flushSymbols.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	if err := s.symbolInserter.Flush(ctx); err != nil {
		return err
	}
	return s.db.Exec(ctx, sqlf.Sprintf(scipWriterFlushSymbolsQuery, s.uploadID, 1))
}

func (s *scipWriter) flushSymbolNames(ctx context.Context) (err error) {
	ctx, _, endObservation := s.operations.flushSymbolNames.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	// Flush all symbols data into temp tables
	if err := s.symbolNameInserter.Flush(ctx); err != nil {
		return err
	}
	// Move all data from temp tables into target tables
	return s.db.Exec(ctx, sqlf.Sprintf(scipWriterFlushSymbolNamesQuery, s.uploadID))
}

const scipWriterFlushSymbolNamesQuery = `
INSERT INTO codeintel_scip_symbol_names (
	upload_id,
	id,
	name_segment,
	prefix_id
)
SELECT
	%s,
	source.id,
	source.name_segment,
	source.prefix_id
FROM t_codeintel_scip_symbol_names source
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
