package enqueuer

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type operations struct {
	QueueIndex              *observation.Operation
	InferIndexConfiguration *observation.Operation
}

func newOperations(observationContext *observation.Context) *operations {
	metrics := metrics.NewOperationMetrics(
		observationContext.Registerer,
		"codeintel_autoindex_enqueuer",
		metrics.WithLabels("op"),
		metrics.WithCountHelp("Total number of method invocations."),
	)

	op := func(name string) *observation.Operation {
		return observationContext.Operation(observation.Op{
			Name:         fmt.Sprintf("codeintel.autoindex-enqueuer.%s", name),
			MetricLabels: []string{name},
			Metrics:      metrics,
		})
	}

	return &operations{
		QueueIndex:              op("QueueIndex"),
		InferIndexConfiguration: op("InferIndexConfiguration"),
	}
}
