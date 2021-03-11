package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gorilla/mux"
	"github.com/inconshreveable/log15"
	amclient "github.com/prometheus/alertmanager/api/v2/client"
	"github.com/prometheus/alertmanager/api/v2/client/alert"
	prometheus "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	srcprometheus "github.com/sourcegraph/sourcegraph/internal/src-prometheus"
)

// AlertsStatusReporter summarizes alert activity from Alertmanager
type AlertsStatusReporter struct {
	log          log15.Logger
	alertmanager *amclient.Alertmanager
	prometheus   prometheus.API
}

func NewAlertsStatusReporter(logger log15.Logger, alertmanager *amclient.Alertmanager, prom prometheus.API) *AlertsStatusReporter {
	return &AlertsStatusReporter{
		log:          logger.New("logger", "alerts-status"),
		alertmanager: alertmanager,
		prometheus:   prom,
	}
}

func (s *AlertsStatusReporter) Handler() http.Handler {
	handler := mux.NewRouter()
	handler.StrictSlash(true)
	// see EndpointAlertsStatusHistory usages
	handler.HandleFunc(srcprometheus.EndpointAlertsStatusHistory, func(w http.ResponseWriter, req *http.Request) {
		timespan := 24 * time.Hour
		if timespanParam := req.URL.Query().Get("timespan"); timespanParam != "" {
			var err error
			timespan, err = time.ParseDuration(timespanParam)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(fmt.Sprintf("invalid duration parameter: %s", err)))
				return
			}
		}
		const alertsHistoryQuery = `max by (level,name,service_name,owner)(avg_over_time(alert_count{name!=""}[12h]))`
		results, warn, err := s.prometheus.QueryRange(req.Context(), alertsHistoryQuery,
			prometheus.Range{
				Start: time.Now().Add(-timespan),
				End:   time.Now(),
				Step:  12 * time.Hour,
			})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		if len(warn) > 0 {
			s.log.Warn("site.monitoring.alerts: warnings encountered on prometheus query",
				"timespan", timespan.String(),
				"warnings", warn)
		}
		if results.Type() != model.ValMatrix {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		data := results.(model.Matrix)
		var alerts srcprometheus.MonitoringAlerts
		for _, sample := range data {
			var (
				name        = string(sample.Metric["name"])
				serviceName = string(sample.Metric["service_name"])
				level       = string(sample.Metric["level"])
				owner       = string(sample.Metric["owner"])
				prevVal     *model.SampleValue
			)
			for _, p := range sample.Values {
				// Check for nil so that we don't ignore the first occurrence of an alert - even if the
				// alert is never >0, we want to be aware that it is at least configured correctly and
				// being tracked. Otherwise, if the value in this window is the same as in the previous
				// window just discard it.
				if prevVal != nil && p.Value == *prevVal {
					continue
				}
				// copy value for comparison later
				v := p.Value
				prevVal = &v
				// record alert in results
				alerts = append(alerts, &srcprometheus.MonitoringAlert{
					NameValue:        fmt.Sprintf("%s: %s", level, name),
					ServiceNameValue: serviceName,
					OwnerValue:       owner,
					TimestampValue:   p.Timestamp.Time().UTC().Truncate(time.Hour),
					AverageValue:     float64(p.Value),
				})
			}
		}

		sort.Sort(alerts)

		// summarize alerts status
		b, err := json.Marshal(&srcprometheus.AlertsHistory{Alerts: alerts})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write(b)
	})
	// see EndpointAlertsStatus usages
	handler.HandleFunc(srcprometheus.EndpointAlertsStatus, func(w http.ResponseWriter, req *http.Request) {
		if noAlertmanager == "true" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("alertmanager is disabled"))
			return
		}
		t := true
		f := false
		results, err := s.alertmanager.Alert.GetAlerts(&alert.GetAlertsParams{
			Active:    &t,
			Inhibited: &f,
			Context:   req.Context(),
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		var criticalAlerts, warningAlerts, silencedAlerts int
		servicesWithCriticalAlerts := map[string]struct{}{}
		for _, a := range results.GetPayload() {
			if len(a.Status.SilencedBy) > 0 {
				silencedAlerts++
				continue
			}
			level := a.Labels["level"]
			switch level {
			case "warning":
				warningAlerts++
			case "critical":
				criticalAlerts++
				svc := a.Labels["service_name"]
				servicesWithCriticalAlerts[svc] = struct{}{}
			}
		}
		// summarize alerts status
		b, err := json.Marshal(&srcprometheus.AlertsStatus{
			Silenced:         silencedAlerts,
			Warning:          warningAlerts,
			Critical:         criticalAlerts,
			ServicesCritical: len(servicesWithCriticalAlerts),
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(err.Error()))
			return
		}
		_, _ = w.Write(b)
	})
	return handler
}
