package metrics

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/lib/errors"

	du "github.com/sourcegraph/sourcegraph/internal/diskusage"
)

type testRegisterer struct{}

func (testRegisterer) Register(prometheus.Collector) error  { return nil }
func (testRegisterer) MustRegister(...prometheus.Collector) {}
func (testRegisterer) Unregister(prometheus.Collector) bool { return true }

// NoOpRegisterer is a behaviorless Prometheus Registerer usable for unit tests
// or to disable metrics collection.
var NoOpRegisterer prometheus.Registerer = testRegisterer{}

// registerer exists so we can override it in tests
var registerer = prometheus.DefaultRegisterer

// RequestMeter wraps a Prometheus request meter (counter + duration histogram) updated by requests made by derived
// http.RoundTrippers.
type RequestMeter struct {
	counter  *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

const (
	labelCategory  = "category"
	labelCode      = "code"
	labelHost      = "host"
	labelTask      = "task"
	labelFromCache = "from_cache"
)

var taskKey struct{}

// ContextWithTask adds the "job" value to the context
func ContextWithTask(ctx context.Context, task string) context.Context {
	return context.WithValue(ctx, taskKey, task)
}

// TaskFromContext will return the job, if any, stored in the context. If none is
// found the default string "unknown" is returned
func TaskFromContext(ctx context.Context) string {
	if task, ok := ctx.Value(taskKey).(string); ok {
		return task
	}
	return "unknown"
}

var (
	requestMeterMu         sync.Mutex
	requestCounterMetrics  map[string]*prometheus.CounterVec
	requestDurationMetrics map[string]*prometheus.HistogramVec
)

// NewRequestMeter creates a new request meter.
func NewRequestMeter(subsystem, help string) *RequestMeter {
	requestMeterMu.Lock()
	defer requestMeterMu.Unlock()

	requestCounter, ok := requestCounterMetrics[subsystem]
	if !ok {
		if requestCounterMetrics == nil {
			requestCounterMetrics = make(map[string]*prometheus.CounterVec)
		}
		requestCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "src",
			Subsystem: subsystem,
			Name:      "requests_total",
			Help:      help,
		}, []string{labelCategory, labelCode, labelHost, labelTask, labelFromCache})
		requestCounterMetrics[subsystem] = requestCounter
		registerer.MustRegister(requestCounter)
	}

	// TODO(uwedeportivo):
	// A prometheus histogram has a request counter built in.
	// It will have the suffix _count (ie src_subsystem_request_duration_count).
	// See if we can get rid of requestCounter (if it hasn't been used by a customer yet) and use this counter instead.
	requestDuration, ok := requestDurationMetrics[subsystem]
	if !ok {
		if requestDurationMetrics == nil {
			requestDurationMetrics = make(map[string]*prometheus.HistogramVec)
		}
		requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "src",
			Subsystem: subsystem,
			Name:      "request_duration_seconds",
			Help:      "Time (in seconds) spent on request.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"category", "code", "host"})
		requestDurationMetrics[subsystem] = requestDuration
		registerer.MustRegister(requestDuration)
	}

	return &RequestMeter{
		counter:  requestCounter,
		duration: requestDuration,
	}
}

// Transport returns an http.RoundTripper that updates rm for each request. The categoryFunc is called to
// determine the category label for each request.
func (rm *RequestMeter) Transport(transport http.RoundTripper, categoryFunc func(*url.URL) string) http.RoundTripper {
	return &requestCounterMiddleware{
		meter:        rm,
		transport:    transport,
		categoryFunc: categoryFunc,
	}
}

// Doer is a copy of the httpcli.Doer interface. We need it to avoid circular imports.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Doer returns a Doer which implements httpcli.Doer that updates rm for each
// request. The categoryFunc is called to determine the category label for each
// request.
func (rm *RequestMeter) Doer(cli Doer, categoryFunc func(*url.URL) string) Doer {
	return &requestCounterMiddleware{
		meter:        rm,
		cli:          cli,
		categoryFunc: categoryFunc,
	}
}

type requestCounterMiddleware struct {
	meter        *RequestMeter
	cli          Doer
	transport    http.RoundTripper
	categoryFunc func(*url.URL) string
}

func (t *requestCounterMiddleware) RoundTrip(r *http.Request) (resp *http.Response, err error) {
	start := time.Now()
	if t.transport != nil {
		resp, err = t.transport.RoundTrip(r)
	} else if t.cli != nil {
		resp, err = t.cli.Do(r)
	}

	category := t.categoryFunc(r.URL)

	var code string
	if err != nil {
		code = "error"
	} else {
		code = strconv.Itoa(resp.StatusCode)
	}

	// X-From-Cache=1 if the returned response is from the cache created by
	// httpcli.NewCachedTransportOpt
	var fromCache = "false"
	if resp != nil && resp.Header.Get("X-From-Cache") != "" {
		fromCache = "true"
	}

	d := time.Since(start)
	t.meter.counter.With(map[string]string{
		labelCategory:  category,
		labelCode:      code,
		labelHost:      r.URL.Host,
		labelTask:      TaskFromContext(r.Context()),
		labelFromCache: fromCache,
	}).Inc()

	t.meter.duration.WithLabelValues(category, code, r.URL.Host).Observe(d.Seconds())
	return
}

func (t *requestCounterMiddleware) Do(req *http.Request) (*http.Response, error) {
	return t.RoundTrip(req)
}

// MustRegisterDiskMonitor exports two prometheus metrics
// "src_disk_space_available_bytes{path=$path}" and
// "src_disk_space_total_bytes{path=$path}". The values exported are for the
// filesystem that path is on.
//
// It is safe to call this function more than once for the same path.
func MustRegisterDiskMonitor(path string) {
	mustRegisterOnce(newDiskCollector(path))
}

type diskCollector struct {
	path          string
	availableDesc *prometheus.Desc
	totalDesc     *prometheus.Desc
	logger        log.Logger
}

func newDiskCollector(path string) prometheus.Collector {
	constLabels := prometheus.Labels{"path": path}
	return &diskCollector{
		path: path,
		availableDesc: prometheus.NewDesc(
			"src_disk_space_available_bytes",
			"Amount of free space disk space.",
			nil,
			constLabels,
		),
		totalDesc: prometheus.NewDesc(
			"src_disk_space_total_bytes",
			"Amount of total disk space.",
			nil,
			constLabels,
		),
		logger: log.Scoped("diskCollector"),
	}
}

func (c *diskCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.availableDesc
	ch <- c.totalDesc
}

func (c *diskCollector) Collect(ch chan<- prometheus.Metric) {
	usage, err := du.New(c.path)
	if err != nil {
		c.logger.Error("error getting disk usage info", log.Error(err))
		return
	}
	ch <- prometheus.MustNewConstMetric(c.availableDesc, prometheus.GaugeValue, float64(usage.Available()))
	ch <- prometheus.MustNewConstMetric(c.totalDesc, prometheus.GaugeValue, float64(usage.Size()))
}

func mustRegisterOnce(c prometheus.Collector) {
	err := registerer.Register(c)
	if err != nil && !errors.HasType(err, prometheus.AlreadyRegisteredError{}) {
		panic(err)
	}
}
