// Command symbols is a service that serves code symbols (functions, variables, etc.) from a repository at a
// specific commit.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/sourcegraph/cmd/symbols/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/cmd/symbols/internal/parser"
	"github.com/sourcegraph/sourcegraph/cmd/symbols/internal/search"
	"github.com/sourcegraph/sourcegraph/cmd/symbols/internal/sqlite"
	"github.com/sourcegraph/sourcegraph/cmd/symbols/internal/symbols"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/debugserver"
	"github.com/sourcegraph/sourcegraph/internal/diskcache"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/httpserver"
	"github.com/sourcegraph/sourcegraph/internal/logging"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/profiler"
	"github.com/sourcegraph/sourcegraph/internal/sentry"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
	"github.com/sourcegraph/sourcegraph/internal/tracer"
)

const addr = ":3184"

func main() {
	config.Load()

	// Set up Google Cloud Profiler when running in Cloud
	if err := profiler.Init(); err != nil {
		log.Fatalf("Failed to start profiler: %v", err)
	}

	env.Lock()
	env.HandleHelpFlag()
	conf.Init()
	logging.Init()
	tracer.Init(conf.DefaultClient())
	sentry.Init(conf.DefaultClient())
	trace.Init()

	if err := config.Validate(); err != nil {
		log.Fatalf("Failed to load configuration: %s", err)
	}

	if config.sanityCheck {
		fmt.Print("Running sanity check...")
		if err := sqlite.SanityCheck(); err != nil {
			fmt.Println("failed ❌", err)
			os.Exit(1)
		}

		fmt.Println("passed ✅")
		os.Exit(0)
	}

	// Initialize tracing/metrics
	observationContext := &observation.Context{
		Logger:     log15.Root(),
		Tracer:     &trace.Tracer{Tracer: opentracing.GlobalTracer()},
		Registerer: prometheus.DefaultRegisterer,
	}

	// Start debug server
	ready := make(chan struct{})
	go debugserver.NewServerRoutine(ready).Start()

	parserPool, err := parser.NewParserPool(parser.NewCtagsParser, config.ctagsProcesses)
	if err != nil {
		log.Fatalf("Failed to parser pool: %s", err)
	}

	gitserverClient := gitserver.NewClient(observationContext)
	parser := parser.NewParser(parserPool, parser.NewRepositoryFetcher(gitserverClient, 15, observationContext))

	cache := &diskcache.Store{
		Dir:               config.cacheDir,
		Component:         "symbols",
		BackgroundTimeout: 20 * time.Minute,
	}
	databaseWriter := sqlite.NewDatabaseWriter(config.cacheDir, gitserverClient, parser)
	searcher := search.NewSearcher(cache, databaseWriter)

	server := httpserver.NewFromAddr(addr, &http.Server{
		ReadTimeout:  75 * time.Second,
		WriteTimeout: 10 * time.Minute,
		Handler:      ot.Middleware(trace.HTTPTraceMiddleware(symbols.NewHandler(searcher))),
	})

	evictionDuration := time.Second * 10
	cacheSizeBytes := int64(config.cacheSizeMB) * 1000 * 1000
	cacheEvicter := sqlite.NewCacheEvicter(evictionDuration, cache, cacheSizeBytes)

	// Mark health server as ready and go!
	close(ready)
	goroutine.MonitorBackgroundRoutines(context.Background(), server, cacheEvicter)
}
