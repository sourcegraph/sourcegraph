package main

import (
	"context"
	"flag"
	"fmt"
	"log" //nolint:logging // TODO move all logging to sourcegraph/log
	"math"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"time"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
)

var (
	reposProcessedCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ghe_feeder_processed",
		Help: "The total number of processed repos (labels: worker)",
	}, []string{"worker"})
	reposFailedCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ghe_feeder_failed",
		Help: "The total number of failed repos (labels: worker, err_type with values {clone, api, push, unknown}",
	}, []string{"worker", "err_type"})
	reposSucceededCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ghe_feeder_succeeded",
		Help: "The total number of succeeded repos",
	})
	reposAlreadyDoneCounter = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ghe_feeder_skipped",
		Help: "The total number of repos already done in previous runs (found in feeder.database)",
	})

	remainingWorkGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ghe_feeder_remaining_work",
		Help: "The number of repos that still need to be processed from the specified input",
	})
)

func main() {
	admin := flag.String("admin", "", "(required) destination GHE admin name")
	token := flag.String("token", os.Getenv("GITHUB_TOKEN"), "(required) GitHub personal access token for the destination GHE instance")
	progressFilepath := flag.String("progress", "feeder.database", "path to a sqlite DB recording the progress made in the feeder (created if it doesn't exist)")
	baseURL := flag.String("baseURL", "", "(required) base URL of GHE instance to feed")
	uploadURL := flag.String("uploadURL", "", "upload URL of GHE instance to feed")
	numWorkers := flag.Int("numWorkers", 20, "number of workers")
	scratchDir := flag.String("scratchDir", "", "scratch dir where to temporarily clone repositories")
	limitPump := flag.Int64("limit", math.MaxInt64, "limit processing to this many repos (for debugging)")
	skipNumLines := flag.Int64("skip", 0, "skip this many lines from input")
	logFilepath := flag.String("logfile", "feeder.log", "path to a log file")
	apiCallsPerSec := flag.Float64("apiCallsPerSec", 100.0, "how many API calls per sec to destination GHE")
	numSimultaneousPushes := flag.Int("numSimultaneousPushes", 10, "number of simultaneous GHE pushes")
	cloneRepoTimeout := flag.Duration("cloneRepoTimeout", time.Minute*3, "how long to wait for a repo to clone")
	numCloningAttempts := flag.Int("numCloningAttempts", 5, "number of cloning attempts before giving up")
	numSimultaneousClones := flag.Int("numSimultaneousClones", 10, "number of simultaneous github.com clones")
	forceOrg := flag.String("force-org", "", "always use this org when adding repositories")

	help := flag.Bool("help", false, "Show help")

	flag.Parse()

	logHandler, err := log15.FileHandler(*logFilepath, log15.LogfmtFormat())
	if err != nil {
		log.Fatal(err)
	}
	log15.Root().SetHandler(logHandler)

	if *help || len(*baseURL) == 0 || len(*token) == 0 || len(*admin) == 0 {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if len(*uploadURL) == 0 {
		*uploadURL = *baseURL
	}

	if len(*scratchDir) == 0 {
		d, err := os.MkdirTemp("", "ghe-feeder")
		if err != nil {
			log15.Error("failed to create scratch dir", "error", err)
			os.Exit(1)
		}
		*scratchDir = d
	}

	u, err := url.Parse(*baseURL)
	if err != nil {
		log15.Error("failed to parse base URL", "baseURL", *baseURL, "error", err)
		os.Exit(1)
	}
	host := u.Host

	ctx := context.Background()
	gheClient, err := newGHEClient(ctx, *baseURL, *uploadURL, *token)
	if err != nil {
		log15.Error("failed to create GHE client", "error", err)
		os.Exit(1)
	}

	fdr, err := newFeederDB(*progressFilepath)
	if err != nil {
		log15.Error("failed to create sqlite DB", "path", *progressFilepath, "error", err)
		os.Exit(1)
	}

	spinner := progressbar.Default(-1, "calculating work")
	numLines, err := numLinesTotal(*skipNumLines)
	if err != nil {
		log15.Error("failed to calculate outstanding work", "error", err)
		os.Exit(1)
	}
	_ = spinner.Finish()

	if numLines > *limitPump {
		numLines = *limitPump
	}

	if numLines == 0 {
		log15.Info("no work remaining in input")
		fmt.Println("no work remaining in input, exiting")
		os.Exit(0)
	}

	remainingWorkGauge.Set(float64(numLines))
	bar := progressbar.New64(numLines)

	work := make(chan string)

	prdc := &producer{
		remaining:    numLines,
		pipe:         work,
		fdr:          fdr,
		logger:       log15.New("source", "producer"),
		bar:          bar,
		skipNumLines: *skipNumLines,
	}

	var wg sync.WaitGroup

	wg.Add(*numWorkers)

	// trap Ctrl+C and call cancel on the context
	ctx, cancel := context.WithCancel(ctx)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	defer func() {
		signal.Stop(c)
		cancel()
	}()
	go func() {
		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		_ = http.ListenAndServe(":2112", nil)
	}()

	rateLimiter := ratelimit.NewInstrumentedLimiter("GHEFeeder", rate.NewLimiter(rate.Limit(*apiCallsPerSec), 100))
	pushSem := make(chan struct{}, *numSimultaneousPushes)
	cloneSem := make(chan struct{}, *numSimultaneousClones)

	var wkrs []*worker

	for i := range *numWorkers {
		name := fmt.Sprintf("worker-%d", i)
		wkrScratchDir := filepath.Join(*scratchDir, name)
		err := os.MkdirAll(wkrScratchDir, 0777)
		if err != nil {
			log15.Error("failed to create worker scratch dir", "scratchDir", *scratchDir, "error", err)
			os.Exit(1)
		}

		wkr := &worker{
			name:               name,
			client:             gheClient,
			index:              i,
			scratchDir:         wkrScratchDir,
			work:               work,
			wg:                 &wg,
			bar:                bar,
			fdr:                fdr,
			logger:             log15.New("source", name),
			rateLimiter:        rateLimiter,
			admin:              *admin,
			token:              *token,
			host:               host,
			pushSem:            pushSem,
			cloneSem:           cloneSem,
			cloneRepoTimeout:   *cloneRepoTimeout,
			numCloningAttempts: *numCloningAttempts,
		}
		if *forceOrg != "" {
			wkr.currentOrg = *forceOrg
			wkr.currentMaxRepos = math.MaxInt
		}

		wkrs = append(wkrs, wkr)
		go wkr.run(ctx)
	}

	err = prdc.pump(ctx)
	if err != nil {
		log15.Error("pump failed", "error", err)
		os.Exit(1)
	}
	close(work)
	wg.Wait()
	_ = bar.Finish()

	s := stats(wkrs, prdc)

	fmt.Println(s)
	log15.Info(s)
}

func stats(wkrs []*worker, prdc *producer) string {
	var numProcessed, numSucceeded, numFailed int64

	for _, wkr := range wkrs {
		numProcessed += wkr.numSucceeded + wkr.numFailed
		numFailed += wkr.numFailed
		numSucceeded += wkr.numSucceeded
	}

	return fmt.Sprintf("\n\nDone: processed %d, succeeded: %d, failed: %d, skipped: %d\n",
		numProcessed, numSucceeded, numFailed, prdc.numAlreadyDone)
}
