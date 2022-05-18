package execute

import (
	"context"
	"log"

	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/job/jobutil"
	"github.com/sourcegraph/sourcegraph/internal/search/predicate"
	"github.com/sourcegraph/sourcegraph/internal/search/run"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/internal/trace"
)

// Execute is the top-level entrypoint to executing a search. It will
// expand predicates, create jobs, and execute those jobs.
func Execute(
	ctx context.Context,
	stream streaming.Sender,
	inputs *run.SearchInputs,
	clients job.RuntimeClients,
) (_ *search.Alert, err error) {
	tr, ctx := trace.New(ctx, "Execute", "")
	defer func() {
		tr.SetError(err)
		tr.Finish()
	}()

	plan := inputs.Plan
	plan, err = predicate.Expand(ctx, clients, inputs, plan)
	if err != nil {
		return nil, err
	}

	planJob, err := jobutil.NewPlanJob(inputs, plan)
	if err != nil {
		return nil, err
	}

	log.Println("\n" + jobutil.PrettySexp(planJob))

	return planJob.Run(ctx, clients, stream)
}
