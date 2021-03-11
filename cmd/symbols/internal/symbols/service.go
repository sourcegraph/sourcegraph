// Package symbols implements the symbol search service.
package symbols

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	ctags "github.com/sourcegraph/go-ctags"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/diskcache"
)

// Service is the symbols service.
type Service struct {
	// FetchTar returns an io.ReadCloser to a tar archive of a repository at the specified Git
	// remote URL and commit ID. If the error implements "BadRequest() bool", it will be used to
	// determine if the error is a bad request (eg invalid repo).
	FetchTar func(context.Context, api.RepoName, api.CommitID) (io.ReadCloser, error)

	// MaxConcurrentFetchTar is the maximum number of concurrent calls allowed
	// to FetchTar. It defaults to 15.
	MaxConcurrentFetchTar int

	NewParser func() (ctags.Parser, error)

	// NumParserProcesses is the maximum number of ctags parser child processes to run.
	NumParserProcesses int

	// Path is the directory in which to store the cache.
	Path string

	// MaxCacheSizeBytes is the maximum size of the cache in bytes. Note:
	// We can temporarily be larger than MaxCacheSizeBytes. When we go
	// over MaxCacheSizeBytes we trigger delete files until we get below
	// MaxCacheSizeBytes.
	MaxCacheSizeBytes int64

	// cache is the disk backed cache.
	cache *diskcache.Store

	// fetchSem is a semaphore to limit concurrent calls to FetchTar. The
	// semaphore size is controlled by MaxConcurrentFetchTar
	fetchSem chan int

	// pool of ctags parser child processes
	parsers chan ctags.Parser
}

// Start must be called before any requests are handled.
func (s *Service) Start() error {
	if err := s.startParsers(); err != nil {
		return err
	}

	if s.MaxConcurrentFetchTar == 0 {
		s.MaxConcurrentFetchTar = 15
	}
	s.fetchSem = make(chan int, s.MaxConcurrentFetchTar)

	s.cache = &diskcache.Store{
		Dir:               s.Path,
		Component:         "symbols",
		BackgroundTimeout: 20 * time.Minute,
	}
	go s.watchAndEvict()

	return nil
}

// Handler returns the http.Handler that should be used to serve requests.
func (s *Service) Handler() http.Handler {
	if s.parsers == nil {
		panic("must call StartParserPool first")
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/search", s.handleSearch)
	mux.HandleFunc("/healthz", s.handleHealthCheck)

	return mux
}

func (s *Service) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	_, err := w.Write([]byte("Ok"))
	if err != nil {
		log.Printf("failed to write response to health check, err: %s", err)
	}
}

// watchAndEvict is a loop which periodically checks the size of the cache and
// evicts/deletes items if the store gets too large.
func (s *Service) watchAndEvict() {
	if s.MaxCacheSizeBytes == 0 {
		return
	}

	for {
		time.Sleep(10 * time.Second)
		stats, err := s.cache.Evict(s.MaxCacheSizeBytes)
		if err != nil {
			log.Printf("failed to Evict: %s", err)
			continue
		}
		cacheSizeBytes.Set(float64(stats.CacheSize))
		evictions.Add(float64(stats.Evicted))
	}
}

var (
	cacheSizeBytes = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "symbols_store_cache_size_bytes",
		Help: "The total size of items in the on disk cache.",
	})
	evictions = promauto.NewCounter(prometheus.CounterOpts{
		Name: "symbols_store_evictions",
		Help: "The total number of items evicted from the cache.",
	})
)
