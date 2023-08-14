package codenav

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type operations struct {
	getReferences          *observation.Operation
	getImplementations     *observation.Operation
	getPrototypes          *observation.Operation
	getDiagnostics         *observation.Operation
	getHover               *observation.Operation
	getDefinitions         *observation.Operation
	getRanges              *observation.Operation
	getStencil             *observation.Operation
	getClosestDumpsForBlob *observation.Operation
	snapshotForDocument    *observation.Operation
	visibleUploadsForPath  *observation.Operation
}

var m = new(metrics.SingletonREDMetrics)

func newOperations(observationCtx *observation.Context) *operations {
	redMetrics := m.Get(func() *metrics.REDMetrics {
		return metrics.NewREDMetrics(
			observationCtx.Registerer,
			"codeintel_codenav",
			metrics.WithLabels("op"),
			metrics.WithCountHelp("Total number of method invocations."),
		)
	})

	op := func(name string) *observation.Operation {
		return observationCtx.Operation(observation.Op{
			Name:              fmt.Sprintf("codeintel.codenav.%s", name),
			MetricLabelValues: []string{name},
			Metrics:           redMetrics,
		})
	}

	return &operations{
		getReferences:          op("getReferences"),
		getImplementations:     op("getImplementations"),
		getPrototypes:          op("getPrototypes"),
		getDiagnostics:         op("getDiagnostics"),
		getHover:               op("getHover"),
		getDefinitions:         op("getDefinitions"),
		getRanges:              op("getRanges"),
		getStencil:             op("getStencil"),
		getClosestDumpsForBlob: op("GetClosestDumpsForBlob"),
		snapshotForDocument:    op("SnapshotForDocument"),
		visibleUploadsForPath:  op("VisibleUploadsForPath"),
	}
}
