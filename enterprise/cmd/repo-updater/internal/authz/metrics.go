package authz

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// The metrics that are exposed to Prometheus.
var (
	metricsNoPerms = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "src_repoupdater_perms_syncer_no_perms",
		Help: "The number of records that do not have any permissions",
	}, []string{"type"})
	metricsStalePerms = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "src_repoupdater_perms_syncer_stale_perms",
		Help: "The number of records that have stale permissions",
	}, []string{"type"})
	metricsPermsGap = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "src_repoupdater_perms_syncer_perms_gap_seconds",
		Help: "The time gap between least and most up to date permissions",
	}, []string{"type"})
	metricsSyncDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "src_repoupdater_perms_syncer_sync_duration_seconds",
		Help:    "Time spent on syncing permissions",
		Buckets: []float64{1, 2, 5, 10, 30, 60, 120},
	}, []string{"type", "success"})
	metricsSyncErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_repoupdater_perms_syncer_sync_errors_total",
		Help: "Total number of permissions sync errors",
	}, []string{"type"})
	metricsQueueSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "src_repoupdater_perms_syncer_queue_size",
		Help: "The size of the sync request queue",
	})
)
