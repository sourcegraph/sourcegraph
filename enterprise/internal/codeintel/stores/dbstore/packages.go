package dbstore

import (
	"context"

	"github.com/opentracing/opentracing-go/log"

	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
	"github.com/sourcegraph/sourcegraph/internal/database/batch"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

// UpdatePackages upserts package data tied to the given upload.
func (s *Store) UpdatePackages(ctx context.Context, dumpID int, packages []semantic.Package) (err error) {
	ctx, endObservation := s.operations.updatePackages.With(ctx, &err, observation.Args{LogFields: []log.Field{
		log.Int("numPackages", len(packages)),
	}})
	defer endObservation(1, observation.Args{})

	if len(packages) == 0 {
		return nil
	}

	inserter := batch.NewBatchInserter(ctx, s.Store.Handle().DB(), "lsif_packages", "dump_id", "scheme", "name", "version")
	for _, p := range packages {
		if err := inserter.Insert(ctx, dumpID, p.Scheme, p.Name, p.Version); err != nil {
			return err
		}
	}

	return inserter.Flush(ctx)
}
