package graphqlbackend

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var testMetricWarning = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "observability_test_metric_warning",
	Help: "Value is 1 if warning test alert should be firing, 0 otherwise - triggered using triggerObservabilityTestAlert",
}, nil)

var testMetricCritical = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "observability_test_metric_critical",
	Help: "Value is 1 if critical test alert should be firing, 0 otherwise - triggered using triggerObservabilityTestAlert",
}, nil)

func (r *schemaResolver) TriggerObservabilityTestAlert(ctx context.Context, args *struct {
	Level string
}) (*EmptyResponse, error) {
	// 🚨 SECURITY: Do not allow arbitrary users to set off alerts.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	var metric *prometheus.GaugeVec
	switch args.Level {
	case "warning":
		metric = testMetricWarning
	case "critical":
		metric = testMetricCritical
	default:
		return nil, errors.Errorf("invalid alert level %q", args.Level)
	}

	// set metric to firing state
	metric.With(nil).Set(1)

	// reset the metric after some amount of time
	go func() {
		time.Sleep(1 * time.Minute)
		metric.With(nil).Set(0)
	}()

	return &EmptyResponse{}, nil
}
