package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	port      = "8080"
	envLogDir = "LOG_DIR"
)

func run(ctx context.Context, wg *sync.WaitGroup, config *Config) {
	defer wg.Done()

	bc, err := newClient()
	if err != nil {
		panic(err)
	}

	sc, err := newStreamClient()
	if err != nil {
		panic(err)
	}

	clientForProtocol := func(p Protocol) genericClient {
		switch p {
		case Batch:
			return bc
		case Stream:
			return sc
		}
		return nil
	}

	loopSearch := func(ctx context.Context, c genericClient, qc *QueryConfig) {
		if qc.Interval == 0 {
			qc.Interval = 5 * time.Minute
		}

		log := log15.New("name", qc.Name, "query", qc.Query, "type", c.clientType())

		// Randomize start to a random time in the initial interval so our
		// queries aren't all scheduled at the same time.
		randomStart := time.Duration(int64(float64(qc.Interval) * rand.Float64()))
		select {
		case <-ctx.Done():
			return
		case <-time.After(randomStart):
		}

		ticker := time.NewTicker(qc.Interval)
		defer ticker.Stop()

		for {
			var m *metrics
			var err error
			if qc.Query != "" {
				m, err = c.search(ctx, qc.Query, qc.Name)
			} else if qc.Snippet != "" {
				m, err = c.attribution(ctx, qc.Snippet, qc.Name)
			} else {
				log.Error("snippet and query unset")
				return
			}
			if err != nil {
				log.Error(err.Error())
			} else {

				log.Info("metrics", "trace", m.trace, "duration", m.took, "first_result", m.firstResult, "match_count", m.matchCount)

				tookSeconds, firstResultSeconds := m.took.Seconds(), m.firstResult.Seconds()

				tsv.Log(qc.Name, c.clientType(), m.trace, m.matchCount, tookSeconds, firstResultSeconds)
				durationSearchSeconds.WithLabelValues(qc.Name, c.clientType()).Observe(tookSeconds)
				firstResultSearchSeconds.WithLabelValues(qc.Name, c.clientType()).Observe(firstResultSeconds)
				matchCount.WithLabelValues(qc.Name, c.clientType()).Set(float64(m.matchCount))
			}

			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}

	scheduleQuery := func(ctx context.Context, qc *QueryConfig) {
		if len(qc.Protocols) == 0 {
			qc.Protocols = allProtocols
		}

		for _, protocol := range qc.Protocols {
			client := clientForProtocol(protocol)
			wg.Add(1)
			go func() {
				defer wg.Done()
				loopSearch(ctx, client, qc)
			}()
		}
	}

	for _, qc := range config.Queries {
		scheduleQuery(ctx, qc)
	}
}

type genericClient interface {
	search(ctx context.Context, query, queryName string) (*metrics, error)
	attribution(ctx context.Context, snippet, queryName string) (*metrics, error)
	clientType() string
}

func startServer(wg *sync.WaitGroup) *http.Server {
	http.HandleFunc("/health", health)
	http.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{Addr: ":" + port}

	go func() {
		defer wg.Done()
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			panic(err.Error())
		}
	}()
	return srv
}

type tsvLogger struct {
	mu  sync.Mutex
	w   io.Writer
	buf bytes.Buffer
}

func (t *tsvLogger) Log(a ...any) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.buf.Reset()
	t.buf.WriteString(time.Now().UTC().Format(time.RFC3339))
	for _, v := range a {
		t.buf.WriteByte('\t')
		_, _ = fmt.Fprintf(&t.buf, "%v", v)
	}
	t.buf.WriteByte('\n')
	_, _ = t.buf.WriteTo(t.w)
}

var (
	tsv *tsvLogger
)

func main() {
	logDir := os.Getenv(envLogDir)
	if logDir == "" {
		logDir = "."
	}

	log15.Root().SetHandler(log15.MultiHandler(
		log15.StreamHandler(os.Stderr, log15.LogfmtFormat()),
		log15.StreamHandler(&lumberjack.Logger{
			Filename: filepath.Join(logDir, "search_blitz.log"),
			MaxSize:  10, // Megabyte
			MaxAge:   90, // days
			Compress: true,
		}, log15.JsonFormat())))

	// We also log to a TSV file since its easy to interact with via AWK.
	tsv = &tsvLogger{w: &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "search_blitz.tsv"),
		MaxSize:    10, // Megabyte
		MaxBackups: 90, // days
		Compress:   true,
	}}

	ctx, cleanup := SignalSensitiveContext()
	defer cleanup()

	env := os.Getenv("SEARCH_BLITZ_ENV")
	queryFile := os.Getenv("SEARCH_BLITZ_QUERY_FILE")

	config, err := loadConfig(queryFile, env)
	if err != nil {
		panic(err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go run(ctx, &wg, config)

	wg.Add(1)
	srv := startServer(&wg)
	log15.Info("server running on :" + port)

	<-ctx.Done()
	_ = srv.Shutdown(ctx)
	log15.Info("server shut down gracefully")

	wg.Wait()
}

// SignalSensitiveContext returns a background context that is canceled after receiving an
// interrupt or terminate signal. A second signal will abort the program. This function returns
// the context and a function that should be  deferred by the caller to clean up internal channels.
func SignalSensitiveContext() (ctx context.Context, cleanup func()) {
	ctx, cancel := context.WithCancel(context.Background())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		i := 0
		for range signals {
			cancel()

			if i > 0 {
				os.Exit(1)
			}
			i++
		}
	}()

	return ctx, func() {
		cancel()
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
		close(signals)
	}
}
