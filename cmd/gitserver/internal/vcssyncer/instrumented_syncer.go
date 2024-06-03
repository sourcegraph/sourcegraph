package vcssyncer

import (
	"context"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/internal/api"
)

// fetchBuckets are the buckets used for the fetch and clone duration histograms.
// The buckets range from .005s to 120 minutes.
var fetchBuckets = append(
	[]float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 20, 30},
	prometheus.LinearBuckets(60, 5*60, 24)...,
)

var (
	metricIsCloneableDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vcssyncer_is_cloneable_duration_seconds",
		Help:    "Time taken to check if a repository is cloneable",
		Buckets: prometheus.DefBuckets,
	}, []string{"type", "success"})

	metricFetchDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "vcssyncer_fetch_duration_seconds",
		Help:    "Time taken to fetch a repository",
		Buckets: fetchBuckets,
	}, []string{"type", "success"})
)

func newInstrumentedSyncer(syncer VCSSyncer) VCSSyncer {
	typ := syncer.Type()
	typ = strings.ToLower(typ)
	typ = strings.ReplaceAll(typ, " ", "_")
	typ = strings.ReplaceAll(typ, "-", "_")
	typ = strings.ReplaceAll(typ, ".", "_")

	return &instrumentedSyncer{
		base:               syncer,
		formattedTypeLabel: typ,
	}
}

// instrumentedSyncer wraps a VCSSyncer and records metrics for each method call.
type instrumentedSyncer struct {
	base               VCSSyncer
	formattedTypeLabel string
}

func (i *instrumentedSyncer) Type() string {
	return i.base.Type()
}

func (i *instrumentedSyncer) IsCloneable(ctx context.Context, repoName api.RepoName) (err error) {
	if !i.shouldObserve() {
		return i.base.IsCloneable(ctx, repoName)
	}

	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		succeeded := err == nil

		metricIsCloneableDuration.WithLabelValues(i.formattedTypeLabel, strconv.FormatBool(succeeded)).Observe(duration)
	}()

	return i.base.IsCloneable(ctx, repoName)
}

func (i *instrumentedSyncer) Fetch(ctx context.Context, repoName api.RepoName, dir common.GitDir, progressWriter io.Writer) (err error) {
	if !i.shouldObserve() {
		return i.base.Fetch(ctx, repoName, dir, progressWriter)
	}

	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		succeeded := err == nil

		metricFetchDuration.WithLabelValues(i.formattedTypeLabel, strconv.FormatBool(succeeded)).Observe(duration)
	}()

	return i.base.Fetch(ctx, repoName, dir, progressWriter)
}

func (i *instrumentedSyncer) shouldObserve() bool {
	// check to see if the base is another instance of instrumented syncer
	// if so, we should skip the observation to avoid double counting

	_, ok := i.base.(*instrumentedSyncer)
	return !ok
}

var _ VCSSyncer = &instrumentedSyncer{}
