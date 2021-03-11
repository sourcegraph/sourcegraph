package backend

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	tracepkg "github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
)

var metricLabels = []string{"method", "success"}
var requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "src_backend_client_request_duration_seconds",
	Help:    "Total time spent on backend endpoints.",
	Buckets: tracepkg.UserLatencyBuckets,
}, metricLabels)

var requestGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
	Name: "src_backend_client_requests",
	Help: "Current number of requests running for a method.",
}, []string{"method"})

func trace(ctx context.Context, server, method string, arg interface{}, err *error) (context.Context, func()) {
	requestGauge.WithLabelValues(server + "." + method).Inc()

	span, ctx := ot.StartSpanFromContext(ctx, server+"."+method)
	span.SetTag("Server", server)
	span.SetTag("Method", method)
	span.SetTag("Argument", fmt.Sprintf("%#v", arg))
	start := time.Now()

	done := func() {
		elapsed := time.Since(start)

		if err != nil && *err != nil {
			span.SetTag("Error", (*err).Error())
		}
		span.Finish()

		name := server + "." + method
		labels := prometheus.Labels{
			"method":  name,
			"success": strconv.FormatBool(err == nil),
		}
		requestDuration.With(labels).Observe(elapsed.Seconds())
		requestGauge.WithLabelValues(name).Dec()

		uid := actor.FromContext(ctx).UID
		errStr := ""
		if err != nil && *err != nil {
			errStr = (*err).Error()
		}
		log15.Debug("TRACE backend", "rpc", name, "uid", uid, "trace", tracepkg.SpanURL(span), "error", errStr, "duration", elapsed)
	}

	return ctx, done
}
