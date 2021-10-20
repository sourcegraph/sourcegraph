package janitor

import (
	"context"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/internal/goroutine"
)

type documentationSearchCurrentJanitor struct {
	lsifStore LSIFStore
	metrics   *metrics
}

var _ goroutine.Handler = &documentationSearchCurrentJanitor{}
var _ goroutine.ErrorHandler = &documentationSearchCurrentJanitor{}

// NewDocumentationSearchCommitJanitor returns a background routine that periodically removes any
// residual lsif_data_docs_search records that are not the most recent for its key, as identified
// by the recent dump_id in the associated lsif_data_docs_search_current table.
func NewDocumentationSearchCommitJanitor(
	lsifStore LSIFStore,
	interval time.Duration,
	metrics *metrics,
) goroutine.BackgroundRoutine {
	interval = time.Second // TODO

	return goroutine.NewPeriodicGoroutine(context.Background(), interval, &documentationSearchCurrentJanitor{
		lsifStore: lsifStore,
		metrics:   metrics,
	})
}

func (j *documentationSearchCurrentJanitor) Handle(ctx context.Context) (err error) {
	tx, err := j.lsifStore.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	// TODO - return counts and make a dashboard for it
	publicErr := tx.DeleteOldPublicSearchRecords(ctx, time.Second, 100)   // TODO - make configurable
	privateErr := tx.DeleteOldPrivateSearchRecords(ctx, time.Second, 100) // TODO - make configurable

	if publicErr != nil {
		if privateErr != nil {
			return multierror.Append(publicErr, privateErr)
		}

		return publicErr
	}

	return privateErr
}

func (j *documentationSearchCurrentJanitor) HandleError(err error) {
	j.metrics.numErrors.Inc()
	log15.Error("Failed to remove non-current documentation search records", "error", err)
}
