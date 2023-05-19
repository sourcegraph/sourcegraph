package gitserver

import (
	"fmt"
	"sync"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type operations struct {
	search           *observation.Operation
	exec             *observation.Operation
	p4Exec           *observation.Operation
	readDir          *observation.Operation
	lstat            *observation.Operation
	streamBlameFile  *observation.Operation
	blameFile        *observation.Operation
	contributorCount *observation.Operation
	batchLog         *observation.Operation
	batchLogSingle   *observation.Operation
	do               *observation.Operation
}

func newOperations(observationCtx *observation.Context) *operations {
	redMetrics := metrics.NewREDMetrics(
		observationCtx.Registerer,
		"gitserver_client",
		metrics.WithLabels("op"),
		metrics.WithCountHelp("Total number of method invocations."),
	)

	op := func(name string) *observation.Operation {
		return observationCtx.Operation(observation.Op{
			Name:              fmt.Sprintf("gitserver.client.%s", name),
			MetricLabelValues: []string{name},
			Metrics:           redMetrics,
		})
	}

	// suboperations do not have their own metrics but do have their
	// own opentracing spans. This allows us to more granularly track
	// the latency for parts of a request without noising up Prometheus.
	subOp := func(name string) *observation.Operation {
		return observationCtx.Operation(observation.Op{
			Name: fmt.Sprintf("gitserver.client.%s", name),
		})
	}

	return &operations{
		search:           op("Search"),
		exec:             op("Exec"),
		p4Exec:           op("P4Exec"),
		contributorCount: op("ContributorCount"),
		streamBlameFile:  op("StreamBlameFile"),
		blameFile:        op("BlameFile"),
		readDir:          op("ReadDir"),
		lstat:            op("lStat"),
		batchLog:         op("BatchLog"),
		batchLogSingle:   subOp("batchLogSingle"),
		do:               subOp("do"),
	}
}

var (
	operationsInst     *operations
	operationsInstOnce sync.Once
)

func getOperations() *operations {
	operationsInstOnce.Do(func() {
		observationCtx := observation.NewContext(log.Scoped("gitserver.client", "gitserver client"))
		operationsInst = newOperations(observationCtx)
	})

	return operationsInst
}
