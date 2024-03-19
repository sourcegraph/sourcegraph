package streaming

import (
	"context"

	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/conc/stream"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/compute"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/client"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func toComputeResult(ctx context.Context, gitserverClient gitserver.Client, cmd compute.Command, match result.Match) (out []compute.Result, _ error) {
	if v, ok := match.(*result.CommitMatch); ok && v.DiffPreview != nil {
		for _, diffMatch := range v.CommitToDiffMatches() {
			runResult, err := cmd.Run(ctx, gitserverClient, diffMatch)
			if err != nil {
				return nil, err
			}
			out = append(out, runResult)
		}
	} else {
		runResult, err := cmd.Run(ctx, gitserverClient, match)
		if err != nil {
			return nil, err
		}
		out = append(out, runResult)
	}
	return out, nil
}

func NewComputeStream(ctx context.Context, logger log.Logger, db database.DB, searchQuery string, computeCommand compute.Command) (<-chan Event, func() (*search.Alert, error)) {
	gitserverClient := gitserver.NewClient("http.computestream")

	eventsC := make(chan Event, 8)
	errorC := make(chan error, 1)
	s := stream.New().WithMaxGoroutines(8)
	cb := func(ev Event, err error) stream.Callback {
		return func() {
			if err != nil {
				select {
				case errorC <- err:
				default:
				}
			} else {
				eventsC <- ev
			}
		}
	}
	stream := streaming.StreamFunc(func(event streaming.SearchEvent) {
		if !event.Stats.Zero() {
			s.Go(func() stream.Callback {
				return cb(Event{nil, event.Stats}, nil)
			})
		}
		for _, match := range event.Results {
			match := match
			s.Go(func() stream.Callback {
				results, err := toComputeResult(ctx, gitserverClient, computeCommand, match)
				return cb(Event{results, streaming.Stats{}}, err)
			})
		}
	})

	patternType := "regexp"
	searchClient := client.New(logger, db, gitserver.NewClient("http.compute.search"))
	inputs, err := searchClient.Plan(
		ctx,
		"",
		&patternType,
		searchQuery,
		search.Precise,
		search.Streaming,
		pointers.Ptr(int32(0)),
	)
	if err != nil {
		close(eventsC)
		close(errorC)

		return eventsC, func() (*search.Alert, error) { return nil, err }
	}

	type finalResult struct {
		alert *search.Alert
		err   error
	}

	pl := pool.NewWithResults[finalResult]()
	pl.Go(func() finalResult {
		defer close(eventsC)
		defer close(errorC)
		defer s.Wait()

		alert, err := searchClient.Execute(ctx, stream, inputs)
		return finalResult{alert: alert, err: err}
	})

	return eventsC, func() (*search.Alert, error) {
		computeErr := <-errorC
		if computeErr != nil {
			return nil, computeErr
		}
		results := pl.Wait()
		r := results[0]
		return r.alert, r.err
	}
}
