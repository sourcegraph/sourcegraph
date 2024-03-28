package lsifstore

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type operations struct {
	insertMetadata              *observation.Operation
	idsWithMeta                 *observation.Operation
	reconcileCandidates         *observation.Operation
	deleteLsifDataByUploadIds   *observation.Operation
	deleteUnreferencedDocuments *observation.Operation
}

var m = new(metrics.SingletonREDMetrics)

func newOperations(observationCtx *observation.Context) *operations {
	redMetrics := m.Get(func() *metrics.REDMetrics {
		return metrics.NewREDMetrics(
			observationCtx.Registerer,
			"codeintel_uploads_lsifstore",
			metrics.WithLabels("op"),
			metrics.WithCountHelp("Total number of method invocations."),
		)
	})

	op := func(name string) *observation.Operation {
		return observationCtx.Operation(observation.Op{
			Name:              fmt.Sprintf("codeintel.uploads.lsifstore.%s", name),
			MetricLabelValues: []string{name},
			Metrics:           redMetrics,
		})
	}

	return &operations{
		insertMetadata:              op("InsertMetadata"),
		idsWithMeta:                 op("IDsWithMeta"),
		reconcileCandidates:         op("ReconcileCandidates"),
		deleteLsifDataByUploadIds:   op("DeleteLsifDataByUploadIds"),
		deleteUnreferencedDocuments: op("DeleteUnreferencedDocuments"),
	}
}
