package reposcheduler

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type operations struct {
	getRepositoriesForIndexScan *observation.Operation
	getQueuedRepoRev            *observation.Operation
	markRepoRevsAsProcessed     *observation.Operation
	queueRepoRev                *observation.Operation
	isQueued                    *observation.Operation
}

var (
	m = new(metrics.SingletonREDMetrics)
)

func newOperations(observationCtx *observation.Context, storeType storeType) *operations {
	var metricPrefix string
	var operationNamespace string

	if storeType == preciseStore {
		metricPrefix = "codeintel_precise_reposcheduler_store"
		operationNamespace = "precise_reposcheduler"
	} else {
		metricPrefix = "codeintel_syntactic_reposcheduler_store"
		operationNamespace = "syntactic_reposcheduler"
	}

	m := m.Get(func() *metrics.REDMetrics {
		return metrics.NewREDMetrics(
			observationCtx.Registerer,
			metricPrefix,
			metrics.WithLabels("op"),
			metrics.WithCountHelp("Total number of method invocations."),
		)
	})

	op := func(name string) *observation.Operation {
		return observationCtx.Operation(observation.Op{
			Name:              fmt.Sprintf("codeintel.%s.store.%s", operationNamespace, name),
			MetricLabelValues: []string{name},
			Metrics:           m,
		})
	}

	return &operations{
		getRepositoriesForIndexScan: op("GetRepositoriesForIndexScan"),
		getQueuedRepoRev:            op("GetQueuedRepoRev"),
		markRepoRevsAsProcessed:     op("MarkRepoRevsAsProcessed"),
		queueRepoRev:                op("QueueRepoRev"),
		isQueued:                    op("IsQueued"),
	}
}
