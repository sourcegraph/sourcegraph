package graphqlbackend

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
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
	if err := backend.CheckCurrentUserIsSiteAdmin(ctx); err != nil {
		return nil, err
	}

	var metric *prometheus.GaugeVec
	switch args.Level {
	case "warning":
		metric = testMetricWarning
	case "critical":
		metric = testMetricCritical
	default:
		return nil, fmt.Errorf("invalid alert level %q", args.Level)
	}

	// set metric to firing state
	metric.With(nil).Set(1)

	// reset the metric after some amount of time
	go func(m *prometheus.GaugeVec) {
		time.Sleep(1 * time.Minute)
		m.With(nil).Set(0)
	}(metric)

	return &EmptyResponse{}, nil
}
