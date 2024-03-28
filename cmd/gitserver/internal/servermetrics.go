package internal

import (
	"os/exec"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func (s *Server) RegisterMetrics(observationCtx *observation.Context, db dbutil.DB) {
	if runtime.GOOS != "windows" {
		registerEchoMetric(s.logger)
	} else {
		// See https://github.com/sourcegraph/sourcegraph/issues/54317 for details.
		s.logger.Warn("Disabling 'echo' metric")
	}
}

func registerEchoMetric(logger log.Logger) {
	// test the latency of exec, which may increase under certain memory
	// conditions
	echoDuration := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "src_gitserver_echo_duration_seconds",
		Help: "Duration of executing the echo command.",
	})
	prometheus.MustRegister(echoDuration)
	go func() {
		logger = logger.Scoped("echoMetricReporter")
		for {
			time.Sleep(10 * time.Second)
			s := time.Now()
			if err := exec.Command("echo").Run(); err != nil {
				logger.Warn("exec measurement failed", log.Error(err))
				continue
			}
			echoDuration.Set(time.Since(s).Seconds())
		}
	}()
}
