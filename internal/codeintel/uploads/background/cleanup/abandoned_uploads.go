package cleanup

import (
	"context"
	"time"

	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// HandleAbandonedUpload removes upload records which have not left the uploading state within the given TTL.
func (j *janitor) HandleAbandonedUpload(ctx context.Context) error {
	count, err := j.dbStore.DeleteUploadsStuckUploading(ctx, time.Now().UTC().Add(-ConfigInst.UploadTimeout))
	if err != nil {
		return errors.Wrap(err, "dbstore.DeleteUploadsStuckUploading")
	}
	if count > 0 {
		log15.Debug("Deleted abandoned upload records", "count", count)
		j.metrics.numUploadRecordsRemoved.Add(float64(count))
	}

	return nil
}

// func (j *janitor) HandleError(err error) {
// 	h.metrics.numErrors.Inc()
// 	log15.Error("Failed to delete abandoned uploads", "error", err)
// }
