package background

import (
	"context"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/background/queryrunner"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/insights/discovery"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/schema"
)

// newInsightEnqueuer returns a background goroutine which will periodically find all of the search
// and webhook insights across all user settings, and enqueue work for the query runner and webhook
// runner workers to perform.
func newInsightEnqueuer(ctx context.Context, workerBaseStore *basestore.Store, settingStore discovery.SettingStore, observationContext *observation.Context) goroutine.BackgroundRoutine {
	metrics := metrics.NewOperationMetrics(
		observationContext.Registerer,
		"insights_enqueuer",
		metrics.WithCountHelp("Total number of insights enqueuer executions"),
	)
	operation := observationContext.Operation(observation.Op{
		Name:    "Enqueuer.Run",
		Metrics: metrics,
	})

	// Note: We run this goroutine once every 10 minutes, and StalledMaxAge in queryrunner/ is
	// set to 60s. If you change this, make sure the StalledMaxAge is less than this period
	// otherwise there is a fair chance we could enqueue work faster than it can be completed.
	//
	// See also https://github.com/sourcegraph/sourcegraph/pull/17227#issuecomment-779515187 for some very rough
	// data retention / scale concerns.
	return goroutine.NewPeriodicGoroutineWithMetrics(ctx, 12*time.Hour, goroutine.NewHandlerWithErrorMessage(
		"insights_enqueuer",
		func(ctx context.Context) error {
			queryRunnerEnqueueJob := func(ctx context.Context, job *queryrunner.Job) error {
				_, err := queryrunner.EnqueueJob(ctx, workerBaseStore, job)
				return err
			}
			return discoverAndEnqueueInsights(ctx, time.Now, settingStore, queryRunnerEnqueueJob)
		},
	), operation)
}

const queryJobOffsetTime = 30 * time.Second

// discoverAndEnqueueInsights discovers insights defined in the given setting store from user/org/global
// settings and enqueues them to be executed and have insights recorded.
func discoverAndEnqueueInsights(
	ctx context.Context,
	now func() time.Time,
	settingStore discovery.SettingStore,
	enqueueQueryRunnerJob func(ctx context.Context, job *queryrunner.Job) error,
) error {
	insights, err := discovery.Discover(ctx, settingStore)
	if err != nil {
		return errors.Wrap(err, "Discover")
	}

	// Deduplicate series that may be unique (e.g. different name/description) but do not have
	// unique data (i.e. use the same exact search query or webhook URL.)
	var (
		uniqueSeries = map[string]*schema.InsightSeries{}
		multi        error
		offset       time.Duration
	)
	for _, insight := range insights {
		for _, series := range insight.Series {
			seriesID, err := discovery.EncodeSeriesID(series)
			if err != nil {
				multi = multierror.Append(multi, err)
				continue
			}
			_, enqueuedAlready := uniqueSeries[seriesID]
			if enqueuedAlready {
				continue
			}
			uniqueSeries[seriesID] = series

			// Enqueue jobs for each unique series, offsetting each job execution by a minute so we
			// don't execute all queries at once and harm search performance in general.
			if series.Webhook != "" {
				continue // TODO(slimsag): future: add support for webhook insights
			}
			processAfter := now().Add(offset)
			offset += queryJobOffsetTime
			err = enqueueQueryRunnerJob(ctx, &queryrunner.Job{
				SeriesID:     seriesID,
				SearchQuery:  withCountUnlimited(series.Search),
				ProcessAfter: &processAfter,
				State:        "queued",
			})
			if err != nil {
				multi = multierror.Append(multi, err)
			}
		}
	}
	return multi
}

// withCountUnlimited adds `count:9999999` to the given search query string iff `count:` does not
// exist in the query string. This is extremely important as otherwise the number of results our
// search query would return would be incomplete and fluctuate.
//
// TODO(slimsag): future: we should pull in the search query parser to avoid cases where `count:`
// is actually e.g. a search query like `content:"count:"`.
func withCountUnlimited(s string) string {
	if strings.Contains(s, "count:") {
		return s
	}
	return s + " count:9999999"
}
