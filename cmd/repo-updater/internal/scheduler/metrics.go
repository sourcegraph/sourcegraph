package scheduler

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	schedError = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_repoupdater_sched_error",
		Help: "Incremented each time we encounter an error updating a repository.",
	}, []string{"type"})
	schedLoops = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_repoupdater_sched_loops",
		Help: "Incremented each time the scheduler loops.",
	})
	schedAutoFetch = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_repoupdater_sched_auto_fetch",
		Help: "Incremented each time the scheduler updates a managed repository due to hitting a deadline.",
	})
	schedManualFetch = promauto.NewCounter(prometheus.CounterOpts{
		Name: "src_repoupdater_sched_manual_fetch",
		Help: "Incremented each time the scheduler updates a repository due to user traffic.",
	})
	schedKnownRepos = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "src_repoupdater_sched_known_repos",
		Help: "The number of repositories that are managed by the scheduler.",
	})
	schedUpdateQueueLength = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "src_repoupdater_sched_update_queue_length",
		Help: "The number of repositories that are currently queued for update",
	})
	schedFetchLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "src_repoupdater_sched_latency",
		Help:    "The time between a last change made upstream and the repo being fetched by the scheduler.",
		Buckets: prometheus.ExponentialBucketsRange(30, 60*60*24, 20),
	})
	schedFetchResult = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_repoupdater_sched_fetch_result",
		Help: "Incremented each time we fetch a repository.",
	}, []string{"result"})
)
