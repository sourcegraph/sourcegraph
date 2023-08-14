package lsifstore

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type operations struct {
	getPathExists               *observation.Operation
	getStencil                  *observation.Operation
	getRanges                   *observation.Operation
	getMonikersByPosition       *observation.Operation
	getPackageInformation       *observation.Operation
	getDefinitionLocations      *observation.Operation
	getImplementationLocations  *observation.Operation
	getPrototypesLocations      *observation.Operation
	getReferenceLocations       *observation.Operation
	getBulkMonikerLocations     *observation.Operation
	getFullSCIPNameByDescriptor *observation.Operation
	getHover                    *observation.Operation
	getDiagnostics              *observation.Operation
	scipDocument                *observation.Operation
}

var m = new(metrics.SingletonREDMetrics)

func newOperations(observationCtx *observation.Context) *operations {
	redMetrics := m.Get(func() *metrics.REDMetrics {
		return metrics.NewREDMetrics(
			observationCtx.Registerer,
			"codeintel_codenav_lsifstore",
			metrics.WithLabels("op"),
			metrics.WithCountHelp("Total number of method invocations."),
		)
	})

	op := func(name string) *observation.Operation {
		return observationCtx.Operation(observation.Op{
			Name:              fmt.Sprintf("codeintel.codenav.lsifstore.%s", name),
			MetricLabelValues: []string{name},
			Metrics:           redMetrics,
		})
	}

	return &operations{
		getPathExists:               op("GetPathExists"),
		getStencil:                  op("GetStencil"),
		getRanges:                   op("GetRanges"),
		getMonikersByPosition:       op("GetMonikersByPosition"),
		getPackageInformation:       op("GetPackageInformation"),
		getDefinitionLocations:      op("GetDefinitionLocations"),
		getImplementationLocations:  op("GetImplementationLocations"),
		getPrototypesLocations:      op("GetPrototypesLocations"),
		getReferenceLocations:       op("GetReferenceLocations"),
		getBulkMonikerLocations:     op("GetBulkMonikerLocations"),
		getFullSCIPNameByDescriptor: op("GetFullSCIPNameByDescriptor"),
		getHover:                    op("GetHover"),
		getDiagnostics:              op("GetDiagnostics"),
		scipDocument:                op("SCIPDocument"),
	}
}
