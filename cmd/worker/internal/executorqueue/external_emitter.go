package executorqueue

import (
	"context"
	"sync"

	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type externalEmitter[T workerutil.Record] struct {
	queueName  string
	countFuncs []func(ctx context.Context, includeProcessing bool) (int, error)
	reporters  []reporter
	allocation QueueAllocation
}

var _ goroutine.Handler = &externalEmitter[uploadsshared.Index]{}

type reporter interface {
	ReportCount(ctx context.Context, queueName string, count int)
	GetAllocation(queueAllocation QueueAllocation) float64
}

func (r *externalEmitter[T]) Handle(ctx context.Context) error {
	var count int
	for _, countFunc := range r.countFuncs {
		subCount, err := countFunc(context.Background(), true)
		if err != nil {
			return errors.Wrap(err, "dbworkerstore.QueuedCount")
		}
		count += subCount
	}

	fns := make([]func(), 0, len(r.reporters))
	for _, reporter := range r.reporters {
		reportCount := reporter.ReportCount
		count := int(float64(count) * reporter.GetAllocation(r.allocation))
		fns = append(fns, func() { reportCount(ctx, r.queueName, count) })
	}

	runParallel(fns)
	return nil
}

func runParallel(fns []func()) {
	var wg sync.WaitGroup
	wg.Add(len(fns))

	for _, fn := range fns {
		go func() {
			defer wg.Done()
			fn()
		}()
	}

	wg.Wait()
}
