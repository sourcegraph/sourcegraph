package lsifstore

import (
	"context"
	"sync/atomic"

	"github.com/hashicorp/go-multierror"
	"github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
	"github.com/sourcegraph/sourcegraph/internal/database/batch"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

// CurrentDocumentSchemaVersion is the schema version used for new lsif_data_documents rows.
const CurrentDocumentSchemaVersion = 2

func (s *Store) WriteMeta(ctx context.Context, bundleID int, meta semantic.MetaData) (err error) {
	ctx, endObservation := s.operations.writeMeta.With(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("bundleID", bundleID),
	}})
	defer endObservation(1, observation.Args{})

	inserter := batch.NewBatchInserter(ctx, s.Handle().DB(), "lsif_data_metadata", "dump_id", "num_result_chunks")

	defer func() {
		if flushErr := inserter.Flush(ctx); flushErr != nil {
			err = multierror.Append(err, errors.Wrap(flushErr, "inserter.Flush"))
		}
	}()

	return inserter.Insert(ctx, bundleID, meta.NumResultChunks)
}

func (s *Store) WriteDocuments(ctx context.Context, bundleID int, documents chan semantic.KeyedDocumentData) (err error) {
	ctx, traceLog, endObservation := s.operations.writeDocuments.WithAndLogger(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("bundleID", bundleID),
	}})
	defer endObservation(1, observation.Args{})

	var count uint32

	inserter := func(inserter *batch.BatchInserter) error {
		for v := range documents {
			data, err := s.serializer.MarshalDocumentData(v.Document)
			if err != nil {
				return err
			}

			if err := inserter.Insert(ctx, bundleID, v.Path, data, CurrentDocumentSchemaVersion, len(v.Document.Diagnostics)); err != nil {
				return err
			}

			atomic.AddUint32(&count, 1)
		}

		return nil
	}

	if err := withBatchInserter(ctx, s.Handle().DB(), "lsif_data_documents", []string{"dump_id", "path", "data", "schema_version", "num_diagnostics"}, inserter); err != nil {
		return err
	}
	traceLog(log.Int("count", int(count)))
	return nil
}

func (s *Store) WriteResultChunks(ctx context.Context, bundleID int, resultChunks chan semantic.IndexedResultChunkData) (err error) {
	ctx, traceLog, endObservation := s.operations.writeResultChunks.WithAndLogger(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("bundleID", bundleID),
	}})
	defer endObservation(1, observation.Args{})

	var count uint32

	inserter := func(inserter *batch.BatchInserter) error {
		for v := range resultChunks {
			data, err := s.serializer.MarshalResultChunkData(v.ResultChunk)
			if err != nil {
				return err
			}

			if err := inserter.Insert(ctx, bundleID, v.Index, data); err != nil {
				return err
			}

			atomic.AddUint32(&count, 1)
		}

		return nil
	}

	if err := withBatchInserter(ctx, s.Handle().DB(), "lsif_data_result_chunks", []string{"dump_id", "idx", "data"}, inserter); err != nil {
		return err
	}
	traceLog(log.Int("count", int(count)))
	return nil
}

func (s *Store) WriteDefinitions(ctx context.Context, bundleID int, monikerLocations chan semantic.MonikerLocations) (err error) {
	ctx, traceLog, endObservation := s.operations.writeDefinitions.WithAndLogger(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("bundleID", bundleID),
	}})
	defer endObservation(1, observation.Args{})

	count, err := s.writeDefinitionReferences(ctx, bundleID, "lsif_data_definitions", monikerLocations)
	if err != nil {
		return err
	}
	traceLog(log.Int("count", count))
	return nil
}

func (s *Store) WriteReferences(ctx context.Context, bundleID int, monikerLocations chan semantic.MonikerLocations) (err error) {
	ctx, traceLog, endObservation := s.operations.writeReferences.WithAndLogger(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("bundleID", bundleID),
	}})
	defer endObservation(1, observation.Args{})

	count, err := s.writeDefinitionReferences(ctx, bundleID, "lsif_data_references", monikerLocations)
	if err != nil {
		return err
	}
	traceLog(log.Int("count", count))
	return nil
}

func (s *Store) writeDefinitionReferences(ctx context.Context, bundleID int, tableName string, monikerLocations chan semantic.MonikerLocations) (int, error) {
	var count uint32

	inserter := func(inserter *batch.BatchInserter) error {
		for v := range monikerLocations {
			data, err := s.serializer.MarshalLocations(v.Locations)
			if err != nil {
				return err
			}

			if err := inserter.Insert(ctx, bundleID, v.Scheme, v.Identifier, data); err != nil {
				return err
			}

			atomic.AddUint32(&count, 1)
		}

		return nil
	}

	err := withBatchInserter(ctx, s.Handle().DB(), tableName, []string{"dump_id", "scheme", "identifier", "data"}, inserter)
	return int(count), err
}

func withBatchInserter(ctx context.Context, db dbutil.DB, tableName string, columns []string, f func(inserter *batch.BatchInserter) error) (err error) {
	return goroutine.RunWorkers(goroutine.SimplePoolWorker(func() error {
		inserter := batch.NewBatchInserter(ctx, db, tableName, columns...)
		defer func() {
			if flushErr := inserter.Flush(ctx); flushErr != nil {
				err = multierror.Append(err, errors.Wrap(flushErr, "inserter.Flush"))
			}
		}()

		return f(inserter)
	}))
}
