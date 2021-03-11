package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/handlers"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/sourcegraph/sourcegraph/internal/debugserver"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/logging"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/tracer"
)

var logRequests, _ = strconv.ParseBool(env.Get("LOG_REQUESTS", "", "log HTTP requests"))

const port = "3180"

// requestMu ensures we only do one request at a time to prevent tripping abuse detection.
var requestMu sync.Mutex

var metricWaitingRequestsGauge = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "github_proxy_waiting_requests",
	Help: "Number of proxy requests waiting on the mutex",
})

// list obtained from httputil of headers not to forward.
var hopHeaders = map[string]struct{}{
	"Connection":          {},
	"Proxy-Connection":    {}, // non-standard but still sent by libcurl and rejected by e.g. google
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {}, // canonicalized version of "TE"
	"Trailer":             {}, // not Trailers per URL above; http://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

func main() {
	env.Lock()
	env.HandleHelpFlag()
	logging.Init()
	tracer.Init()
	trace.Init(true)

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGHUP)
		<-c
		os.Exit(0)
	}()

	go debugserver.Start()

	// Use a custom client/transport because GitHub closes keep-alive
	// connections after 60s. In order to avoid running into EOF errors, we use
	// a IdleConnTimeout of 30s, so connections are only kept around for <30s
	client := &http.Client{Transport: &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		IdleConnTimeout: 30 * time.Second,
	}}

	var h http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q2 := r.URL.Query()
		h2 := make(http.Header)
		for k, v := range r.Header {
			if _, found := hopHeaders[k]; !found {
				h2[k] = v
			}
		}

		req2 := &http.Request{
			Method: r.Method,
			Body:   r.Body,
			URL: &url.URL{
				Scheme:   "https",
				Host:     "api.github.com",
				Path:     r.URL.Path,
				RawQuery: q2.Encode(),
			},
			Header: h2,
		}

		metricWaitingRequestsGauge.Inc()
		requestMu.Lock()
		metricWaitingRequestsGauge.Dec()
		resp, err := client.Do(req2)
		requestMu.Unlock()
		if err != nil {
			log15.Warn("proxy error", "err", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		if resp.StatusCode < 400 || !logRequests {
			_, _ = io.Copy(w, resp.Body)
			return
		}
		b, err := ioutil.ReadAll(resp.Body)
		log15.Warn("proxy error", "status", resp.StatusCode, "body", string(b), "bodyErr", err)
		_, _ = io.Copy(w, bytes.NewReader(b))
	})
	if logRequests {
		h = handlers.LoggingHandler(os.Stdout, h)
	}
	h = instrumentHandler(prometheus.DefaultRegisterer, h)
	http.Handle("/", h)

	host := ""
	if env.InsecureDev {
		host = "127.0.0.1"
	}
	addr := net.JoinHostPort(host, port)
	log15.Info("github-proxy: listening", "addr", addr)
	s := http.Server{
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 10 * time.Minute,
		Addr:         addr,
		Handler:      http.DefaultServeMux,
	}
	log.Fatal(s.ListenAndServe())
}

func instrumentHandler(r prometheus.Registerer, h http.Handler) http.Handler {
	var (
		inFlightGauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "src_githubproxy_in_flight_requests",
			Help: "A gauge of requests currently being served by github-proxy.",
		})
		counter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "src_githubproxy_requests_total",
				Help: "A counter for requests to github-proxy.",
			},
			[]string{"code", "method"},
		)
		duration = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "src_githubproxy_request_duration_seconds",
				Help:    "A histogram of latencies for requests.",
				Buckets: []float64{.25, .5, 1, 2.5, 5, 10},
			},
			[]string{"method"},
		)
		responseSize = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "src_githubproxy_response_size_bytes",
				Help:    "A histogram of response sizes for requests.",
				Buckets: []float64{200, 500, 900, 1500},
			},
			[]string{},
		)
	)

	r.MustRegister(inFlightGauge, counter, duration, responseSize)

	return promhttp.InstrumentHandlerInFlight(inFlightGauge,
		promhttp.InstrumentHandlerDuration(duration,
			promhttp.InstrumentHandlerCounter(counter,
				promhttp.InstrumentHandlerResponseSize(responseSize, h),
			),
		),
	)
}
