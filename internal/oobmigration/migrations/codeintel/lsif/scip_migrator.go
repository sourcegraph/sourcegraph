package lsif

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"
	"github.com/sourcegraph/conc/pool"
	ogscip "github.com/sourcegraph/scip/bindings/go/scip"
	"google.golang.org/protobuf/proto"
	"k8s.io/utils/lru"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/ranges"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/trie"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/batch"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/scip"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type scipMigrator struct {
	store          *basestore.Store
	codeintelStore *basestore.Store
	serializer     *serializer
}

func NewSCIPMigrator(store, codeintelStore *basestore.Store) *scipMigrator {
	return &scipMigrator{
		store:          store,
		codeintelStore: codeintelStore,
		serializer:     newSerializer(),
	}
}

func (m *scipMigrator) ID() int                 { return 20 }
func (m *scipMigrator) Interval() time.Duration { return time.Second }

// Progress returns the ratio between the number of SCIP upload records to SCIP+LSIF upload.
func (m *scipMigrator) Progress(ctx context.Context, applyReverse bool) (float64, error) {
	if applyReverse {
		// If we're applying this in reverse, just report 0% immediately. If we have any SCIP
		// records, we will lose access to them on a downgrade, but will leave them on-disk in
		// the event of a successful re-upgrade.
		return 0, nil
	}

	progress, _, err := basestore.ScanFirstFloat(m.codeintelStore.Query(ctx, sqlf.Sprintf(
		scipMigratorProgressQuery,
	)))
	if err != nil {
		return 0, err
	}

	return progress, nil
}

const scipMigratorProgressQuery = `
SELECT CASE c1.count + c2.count WHEN 0 THEN 1 ELSE cast(c1.count as float) / cast((c1.count + c2.count) as float) END FROM
	(SELECT COUNT(*) as count FROM codeintel_scip_metadata) c1,
	(SELECT COUNT(*) as count FROM lsif_data_metadata) c2
`

func getEnv(name string, defaultValue int) int {
	if value, _ := strconv.Atoi(os.Getenv(name)); value != 0 {
		return value
	}

	return defaultValue
}

var (
	// NOTE: modified in tests
	scipMigratorConcurrencyLevel            = getEnv("SCIP_MIGRATOR_CONCURRENCY_LEVEL", 1)
	scipMigratorUploadReaderBatchSize       = getEnv("SCIP_MIGRATOR_UPLOAD_BATCH_SIZE", 32)
	scipMigratorResultChunkReaderCacheSize  = 8192
	scipMigratorDocumentReaderBatchSize     = 64
	scipMigratorDocumentWriterBatchSize     = 256
	scipMigratorDocumentWriterMaxPayloadSum = 1024 * 1024 * 32
)

func (m *scipMigrator) Up(ctx context.Context) error {
	ch := make(chan struct{}, scipMigratorUploadReaderBatchSize)
	for range scipMigratorUploadReaderBatchSize {
		ch <- struct{}{}
	}
	close(ch)

	p := pool.New().WithContext(ctx)
	for range scipMigratorConcurrencyLevel {
		p.Go(func(ctx context.Context) error {
			for range ch {
				if ok, err := m.upSingle(ctx); err != nil {
					return err
				} else if !ok {
					break
				}
			}

			return nil
		})
	}

	return p.Wait()
}

func (m *scipMigrator) upSingle(ctx context.Context) (_ bool, err error) {
	tx, err := m.codeintelStore.Transact(ctx)
	if err != nil {
		return false, err
	}
	defer func() { err = tx.Done(err) }()

	// Select an upload record to process and lock it in this transaction so that we don't
	// compete with other migrator routines that may be running.
	uploadID, ok, err := basestore.ScanFirstInt(tx.Query(ctx, sqlf.Sprintf(scipMigratorSelectForMigrationQuery)))
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	defer func() {
		if err != nil {
			// Wrap any error after this point with the associated upload ID. This will present
			// itself in the database/UI for site-admins/engineers to locate a poisonous record.
			err = errors.Wrapf(err, "failed to migrate upload %d", uploadID)
		}
	}()

	scipWriter, err := makeSCIPWriter(ctx, tx, uploadID)
	if err != nil {
		return false, err
	}
	if err := migrateUpload(ctx, m.store, tx, m.serializer, scipWriter, uploadID); err != nil {
		return false, err
	}
	if err := scipWriter.Flush(ctx); err != nil {
		return false, err
	}
	if err := deleteLSIFData(ctx, tx, uploadID); err != nil {
		return false, err
	}

	if err := m.store.Exec(ctx, sqlf.Sprintf(scipMigratorMarkUploadAsReindexableQuery, uploadID)); err != nil {
		return false, err
	}

	return true, nil
}

const scipMigratorSelectForMigrationQuery = `
SELECT dump_id
FROM lsif_data_metadata
ORDER BY dump_id
FOR UPDATE SKIP LOCKED
LIMIT 1
`

const scipMigratorMarkUploadAsReindexableQuery = `
UPDATE lsif_uploads
SET should_reindex = true
WHERE id = %s
`

func (m *scipMigrator) Down(ctx context.Context) error {
	// We shouldn't return > 0% on apply reverse, should not be called.
	return nil
}

// migrateUpload converts each LSIF document belonging to the given upload into a SCIP document
// and persists them to the codeintel-db in the given transaction.
func migrateUpload(
	ctx context.Context,
	store *basestore.Store,
	codeintelTx *basestore.Store,
	serializer *serializer,
	scipWriter *scipWriter,
	uploadID int,
) error {
	indexerName, _, err := basestore.ScanFirstString(store.Query(ctx, sqlf.Sprintf(
		scipMigratorIndexerQuery,
		uploadID,
	)))
	if err != nil {
		return err
	}

	numResultChunks, ok, err := basestore.ScanFirstInt(codeintelTx.Query(ctx, sqlf.Sprintf(
		scipMigratorReadMetadataQuery,
		uploadID,
	)))
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	resultChunkCacheSize := scipMigratorResultChunkReaderCacheSize
	if numResultChunks < resultChunkCacheSize {
		resultChunkCacheSize = numResultChunks
	}
	resultChunkCache := lru.New(resultChunkCacheSize)

	scanResultChunks := scanResultChunksIntoMap(serializer, func(idx int, resultChunk ResultChunkData) error {
		resultChunkCache.Add(idx, resultChunk)
		return nil
	})
	scanDocuments := makeDocumentScanner(serializer)

	// Warm result chunk cache if it will all fit in the cache
	if numResultChunks <= resultChunkCacheSize {
		ids := make([]ID, 0, numResultChunks)
		for i := range numResultChunks {
			ids = append(ids, ID(strconv.Itoa(i)))
		}

		if err := scanResultChunks(codeintelTx.Query(ctx, sqlf.Sprintf(
			scipMigratorScanResultChunksQuery,
			uploadID,
			pq.Array(ids),
		))); err != nil {
			return err
		}
	}

	for page := 0; ; page++ {
		documentsByPath, err := scanDocuments(codeintelTx.Query(ctx, sqlf.Sprintf(
			scipMigratorScanDocumentsQuery,
			uploadID,
			scipMigratorDocumentReaderBatchSize,
			page*scipMigratorDocumentReaderBatchSize,
		)))
		if err != nil {
			return err
		}
		if len(documentsByPath) == 0 {
			break
		}

		paths := make([]string, 0, len(documentsByPath))
		for path := range documentsByPath {
			paths = append(paths, path)
		}
		sort.Strings(paths)

		resultIDs := make([][]ID, 0, len(paths))
		for _, path := range paths {
			resultIDs = append(resultIDs, extractResultIDs(documentsByPath[path].Ranges))
		}
		for i, path := range paths {
			scipDocument, err := processDocument(
				ctx,
				codeintelTx,
				serializer,
				resultChunkCache,
				resultChunkCacheSize,
				uploadID,
				numResultChunks,
				indexerName,
				path,
				documentsByPath[path],
				// Load all of the definitions for this document
				resultIDs[i],
				// Load as many definitions from the next document as possible
				resultIDs[i+1:],
			)
			if err != nil {
				return err
			}

			if err := scipWriter.InsertDocument(ctx, path, scipDocument); err != nil {
				return err
			}
		}
	}

	if err := codeintelTx.Exec(ctx, sqlf.Sprintf(
		scipMigratorWriteMetadataQuery,
		uploadID,
	)); err != nil {
		return err
	}

	return nil
}

const scipMigratorIndexerQuery = `
SELECT indexer
FROM lsif_uploads
WHERE id = %s
`

const scipMigratorReadMetadataQuery = `
SELECT num_result_chunks
FROM lsif_data_metadata
WHERE dump_id = %s
`

const scipMigratorScanDocumentsQuery = `
SELECT
	path,
	ranges,
	hovers,
	monikers,
	packages,
	diagnostics
FROM lsif_data_documents
WHERE dump_id = %s
ORDER BY path
LIMIT %s
OFFSET %d
`

const scipMigratorWriteMetadataQuery = `
INSERT INTO codeintel_scip_metadata (upload_id, text_document_encoding, tool_name, tool_version, tool_arguments, protocol_version)
VALUES (%s, '', '', '', '{}', 1)
`

// processDocument converts the given LSIF document into a SCIP document and persists it to the
// codeintel-db in the given transaction.
func processDocument(
	ctx context.Context,
	tx *basestore.Store,
	serializer *serializer,
	resultChunkCache *lru.Cache,
	resultChunkCacheSize int,
	uploadID int,
	numResultChunks int,
	indexerName,
	path string,
	document DocumentData,
	resultIDs []ID,
	preloadResultIDs [][]ID,
) (*ogscip.Document, error) {
	// We first read the relevant result chunks for this document into memory, writing them through to the
	// shared result chunk cache to avoid re-fetching result chunks that are used to processed to documents
	// in a row.

	resultChunks, err := fetchResultChunks(
		ctx,
		tx,
		serializer,
		resultChunkCache,
		resultChunkCacheSize,
		uploadID,
		numResultChunks,
		resultIDs,
		preloadResultIDs,
	)
	if err != nil {
		return nil, err
	}

	targetRangeFetcher := func(resultID precise.ID) (rangeIDs []precise.ID) {
		if resultID == "" {
			return nil
		}

		resultChunk, ok := resultChunks[precise.HashKey(resultID, numResultChunks)]
		if !ok {
			return nil
		}

		for _, pair := range resultChunk.DocumentIDRangeIDs[ID(resultID)] {
			rangeIDs = append(rangeIDs, precise.ID(pair.RangeID))
		}

		return rangeIDs
	}

	scipDocument := ogscip.CanonicalizeDocument(scip.ConvertLSIFDocument(
		uploadID,
		targetRangeFetcher,
		indexerName,
		path,
		toPreciseTypes(document),
	))

	return scipDocument, nil
}

// fetchResultChunks queries for the set of result chunks containing one of the given result set
// identifiers. The output of this function is a map from result chunk index to unmarshalled data.
func fetchResultChunks(
	ctx context.Context,
	tx *basestore.Store,
	serializer *serializer,
	resultChunkCache *lru.Cache,
	resultChunkCacheSize int,
	uploadID int,
	numResultChunks int,
	ids []ID,
	preloadIDs [][]ID,
) (map[int]ResultChunkData, error) {
	// Stores a set of indexes that need to be loaded from the database. The value associated
	// with an index is true if the result chunk should be returned to the caller and false if
	// it should only be preloaded and written to the cache.
	indexMap := map[int]bool{}

	// The map from result chunk index to data payload we'll return. We first populate what
	// we already have from the cache, then we fetch (and cache) the remaining indexes from
	// the database.
	resultChunks := map[int]ResultChunkData{}

outer:
	for i, ids := range append([][]ID{ids}, preloadIDs...) {
		for _, id := range ids {
			if len(indexMap) >= resultChunkCacheSize && i != 0 {
				// Only add fetch preload IDs if we have more room in our request
				break outer
			}

			// Calculate result chunk index that this identifier belongs to
			idx := precise.HashKey(precise.ID(id), numResultChunks)

			// Skip if we already loaded this result chunk from the cache
			if _, ok := resultChunks[idx]; ok {
				continue
			}

			// Attempt to load result chunk data from the cache. If it's present then we can add it to
			// the output map immediately. If it's not present in the cache, then we'll need to fetch it
			// from the database. Collect each such result chunk index so we can do a batch load.

			if rawResultChunk, ok := resultChunkCache.Get(idx); ok {
				if i == 0 {
					// Don't stash preloaded result chunks for return
					resultChunks[idx] = rawResultChunk.(ResultChunkData)
				}
			} else {
				// Store true if it's not _only_ a preload; note that a definition ID and a preloaded ID
				// can hash to the same index. In this case we do need to return it from this call as well
				// as the call when processing the next document.
				indexMap[idx] = i == 0 || indexMap[idx]
			}
		}
	}

	if len(indexMap) > 0 {
		indexes := make([]int, len(indexMap))
		for index := range indexMap {
			indexes = append(indexes, index)
		}

		// Fetch missing result chunks from the database. Add each of the loaded result chunks into
		// the cache shared while processing this particular upload.

		scanResultChunks := scanResultChunksIntoMap(serializer, func(idx int, resultChunk ResultChunkData) error {
			if indexMap[idx] {
				// Don't stash preloaded result chunks for return
				resultChunks[idx] = resultChunk
			}

			// Always cache
			resultChunkCache.Add(idx, resultChunk)
			return nil
		})
		if err := scanResultChunks(tx.Query(ctx, sqlf.Sprintf(
			scipMigratorScanResultChunksQuery,
			uploadID,
			pq.Array(indexes),
		))); err != nil {
			return nil, err
		}
	}

	return resultChunks, nil
}

const scipMigratorScanResultChunksQuery = `
SELECT
	idx,
	data
FROM lsif_data_result_chunks
WHERE
	dump_id = %s AND
	idx = ANY(%s)
`

type scipWriter struct {
	tx                 *basestore.Store
	symbolNameInserter *batch.Inserter
	symbolInserter     *batch.Inserter
	uploadID           int
	nextID             int
	batchPayloadSum    int
	batch              []bufferedDocument
}

type bufferedDocument struct {
	path         string
	scipDocument *ogscip.Document
	payload      []byte
	payloadHash  []byte
}

// makeSCIPWriter creates a small wrapper over batch inserts of SCIP data. Each document
// should be written to Postgres by calling Write. The Flush method should be called after
// each document has been processed.
func makeSCIPWriter(ctx context.Context, tx *basestore.Store, uploadID int) (*scipWriter, error) {
	if err := tx.Exec(ctx, sqlf.Sprintf(makeSCIPWriterTemporarySymbolNamesTableQuery)); err != nil {
		return nil, err
	}
	if err := tx.Exec(ctx, sqlf.Sprintf(makeSCIPWriterTemporarySymbolsTableQuery)); err != nil {
		return nil, err
	}

	symbolNameInserter := batch.NewInserter(
		ctx,
		tx.Handle(),
		"t_codeintel_scip_symbol_names",
		batch.MaxNumPostgresParameters,
		"id",
		"name_segment",
		"prefix_id",
	)

	symbolInserter := batch.NewInserter(
		ctx,
		tx.Handle(),
		"t_codeintel_scip_symbols",
		batch.MaxNumPostgresParameters,
		"document_lookup_id",
		"symbol_id",
		"definition_ranges",
		"reference_ranges",
		"implementation_ranges",
	)

	return &scipWriter{
		tx:                 tx,
		symbolNameInserter: symbolNameInserter,
		symbolInserter:     symbolInserter,
		uploadID:           uploadID,
	}, nil
}

const makeSCIPWriterTemporarySymbolNamesTableQuery = `
CREATE TEMPORARY TABLE t_codeintel_scip_symbol_names (
	id integer NOT NULL,
	name_segment text NOT NULL,
	prefix_id integer
) ON COMMIT DROP
`

const makeSCIPWriterTemporarySymbolsTableQuery = `
CREATE TEMPORARY TABLE t_codeintel_scip_symbols (
	symbol_id integer NOT NULL,
	document_lookup_id integer NOT NULL,
	definition_ranges bytea,
	reference_ranges bytea,
	implementation_ranges bytea
) ON COMMIT DROP
`

// InsertDocument batches a new document, document lookup row, and all of its symbols for insertion.
func (s *scipWriter) InsertDocument(
	ctx context.Context,
	path string,
	scipDocument *ogscip.Document,
) error {
	if s.batchPayloadSum >= scipMigratorDocumentWriterMaxPayloadSum {
		if err := s.flush(ctx); err != nil {
			return err
		}
	}

	uniquePrefix := []byte(fmt.Sprintf(
		"lsif-%d:%d:",
		s.uploadID,
		time.Now().UnixNano()/int64(time.Millisecond)),
	)

	payload, err := proto.Marshal(scipDocument)
	if err != nil {
		return err
	}

	compressedPayload, err := compressor.compress(bytes.NewReader(payload))
	if err != nil {
		return err
	}

	s.batch = append(s.batch, bufferedDocument{
		path:         path,
		scipDocument: scipDocument,
		payload:      compressedPayload,
		payloadHash:  append(uniquePrefix, hashPayload(payload)...),
	})
	s.batchPayloadSum += len(compressedPayload)

	if len(s.batch) >= scipMigratorDocumentWriterBatchSize {
		if err := s.flush(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (s *scipWriter) flush(ctx context.Context) (err error) {
	documents := s.batch
	s.batch = nil
	s.batchPayloadSum = 0

	// NOTE: This logic differs from similar logic in scip_write.go when processing SCIP uploads.
	// In that scenario, we have to be careful of inserting a row with an existing `payload_hash`.
	// Because we have a unique prefix containing the upload ID here, and we have no expectation
	// that interned LSIF graphs will produce the same SCIP document, there should be no expected
	// collisions on insertion here.

	documentIDs, err := batch.WithInserterForIdentifiers(
		ctx,
		s.tx.Handle(),
		"codeintel_scip_documents",
		batch.MaxNumPostgresParameters,
		[]string{
			"schema_version",
			"payload_hash",
			"raw_scip_payload",
		},
		"",
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
		return errors.New("unexpected number of document records inserted")
	}

	documentLookupIDs, err := batch.WithInserterForIdentifiers(
		ctx,
		s.tx.Handle(),
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

	var symbolNameTrie trie.Trie
	symbolNameTrie, s.nextID = trie.NewTrie(symbolNames, s.nextID)

	symbolNameByIDs := map[int]string{}
	idsBySymbolName := map[string]int{}

	if err := symbolNameTrie.Traverse(func(id int, parentID *int, prefix string) error {
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

		return nil
	}); err != nil {
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
			); err != nil {
				return err
			}
		}
	}

	return nil
}

// Flush ensures that all symbol writes have hit the database, and then moves all of the
// rows from the temporary table into the permanent one.
func (s *scipWriter) Flush(ctx context.Context) error {
	// Flush all buffered documents
	if err := s.flush(ctx); err != nil {
		return err
	}

	// Flush all data into temp tables
	if err := s.symbolNameInserter.Flush(ctx); err != nil {
		return err
	}
	if err := s.symbolInserter.Flush(ctx); err != nil {
		return err
	}

	// Move all data from temp tables into target tables
	if err := s.tx.Exec(ctx, sqlf.Sprintf(scipWriterFlushSymbolNamesQuery, s.uploadID)); err != nil {
		return err
	}
	if err := s.tx.Exec(ctx, sqlf.Sprintf(scipWriterFlushSymbolsQuery, s.uploadID)); err != nil {
		return err
	}

	return nil
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
ON CONFLICT DO NOTHING
`

const scipWriterFlushSymbolsQuery = `
INSERT INTO codeintel_scip_symbols (
	upload_id,
	symbol_id,
	document_lookup_id,
	schema_version,
	definition_ranges,
	reference_ranges,
	implementation_ranges
)
SELECT
	%s,
	source.symbol_id,
	source.document_lookup_id,
	1,
	source.definition_ranges,
	source.reference_ranges,
	source.implementation_ranges
FROM t_codeintel_scip_symbols source
ON CONFLICT DO NOTHING
`

var lsifTableNames = []string{
	"lsif_data_metadata",
	"lsif_data_documents",
	"lsif_data_result_chunks",
	"lsif_data_definitions",
	"lsif_data_references",
	"lsif_data_implementations",
}

func deleteLSIFData(ctx context.Context, tx *basestore.Store, uploadID int) error {
	for _, tableName := range lsifTableNames {
		if err := tx.Exec(ctx, sqlf.Sprintf(
			deleteLSIFDataQuery,
			sqlf.Sprintf(tableName),
			uploadID,
		)); err != nil {
			return err
		}
	}

	return nil
}

const deleteLSIFDataQuery = `
DELETE FROM %s WHERE dump_id = %s
`

func makeDocumentScanner(serializer *serializer) func(rows basestore.Rows, queryErr error) (map[string]DocumentData, error) {
	return basestore.NewMapScanner(func(s dbutil.Scanner) (string, DocumentData, error) {
		var path string
		var data MarshalledDocumentData
		if err := s.Scan(&path, &data.Ranges, &data.HoverResults, &data.Monikers, &data.PackageInformation, &data.Diagnostics); err != nil {
			return "", DocumentData{}, err
		}

		document, err := serializer.UnmarshalDocumentData(data)
		if err != nil {
			return "", DocumentData{}, err
		}

		return path, document, nil
	})
}

func scanResultChunksIntoMap(serializer *serializer, f func(idx int, resultChunk ResultChunkData) error) func(rows basestore.Rows, queryErr error) error {
	return basestore.NewCallbackScanner(func(s dbutil.Scanner) (bool, error) {
		var idx int
		var rawData []byte
		if err := s.Scan(&idx, &rawData); err != nil {
			return false, err
		}

		data, err := serializer.UnmarshalResultChunkData(rawData)
		if err != nil {
			return false, err
		}

		if err := f(idx, data); err != nil {
			return false, err
		}

		return true, nil
	})
}

// extractResultIDs extracts the non-empty identifiers of the LSIF definition and implementation
// results attached to any of the given ranges. The returned identifiers are unique and ordered.
func extractResultIDs(ranges map[ID]RangeData) []ID {
	resultIDMap := map[ID]struct{}{}
	for _, r := range ranges {
		if r.DefinitionResultID != "" {
			resultIDMap[r.DefinitionResultID] = struct{}{}
		}
		if r.ImplementationResultID != "" {
			resultIDMap[r.ImplementationResultID] = struct{}{}
		}
	}

	ids := make([]ID, 0, len(resultIDMap))
	for id := range resultIDMap {
		ids = append(ids, id)
	}
	return ids
}

// hashPayload returns a sha256 checksum of the given payload.
func hashPayload(payload []byte) []byte {
	hash := sha256.New()
	_, _ = hash.Write(payload)
	return hash.Sum(nil)
}
