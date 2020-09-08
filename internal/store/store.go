package store

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/zoekt/ignore"

	"github.com/inconshreveable/log15"
	"github.com/opentracing/opentracing-go"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/diskcache"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/mutablelimiter"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"

	"github.com/opentracing/opentracing-go/ext"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
)

// maxFileSize is the limit on file size in bytes. Only files smaller
// than this are searched.
const maxFileSize = 1 << 20 // 1MB; match https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/zoekt%24+%22-file_limit%22

// Store manages the fetching and storing of git archives. Its main purpose is
// keeping a local disk cache of the fetched archives to help speed up future
// requests for the same archive. As a performance optimization, it is also
// responsible for filtering out files we receive from `git archive` that we
// do not want to search.
//
// We use an LRU to do cache eviction:
// * When to evict is based on the total size of *.zip on disk.
// * What to evict uses the LRU algorithm.
// * We touch files when opening them, so can do LRU based on file
//   modification times.
//
// Note: The store fetches tarballs but stores zips. We want to be able to
// filter which files we cache, so we need a format that supports streaming
// (tar). We want to be able to support random concurrent access for reading,
// so we store as a zip.
type Store struct {
	// FetchTar returns an io.ReadCloser to a tar archive of a repository at the specified Git
	// remote URL and commit ID. If the error implements "BadRequest() bool", it will be used to
	// determine if the error is a bad request (eg invalid repo).
	FetchTar func(ctx context.Context, repo gitserver.Repo, commit api.CommitID) (io.ReadCloser, error)

	// Path is the directory to store the cache
	Path string

	// MaxCacheSizeBytes is the maximum size of the cache in bytes. Note:
	// We can temporarily be larger than MaxCacheSizeBytes. When we go
	// over MaxCacheSizeBytes we trigger delete files until we get below
	// MaxCacheSizeBytes.
	MaxCacheSizeBytes int64

	// once protects Start
	once sync.Once

	// cache is the disk backed cache.
	cache *diskcache.Store

	// fetchLimiter limits concurrent calls to FetchTar.
	fetchLimiter *mutablelimiter.Limiter

	// ZipCache provides efficient access to repo zip files.
	ZipCache ZipCache
}

// Start initializes state and starts background goroutines. It can be called
// more than once. It is optional to call, but starting it earlier avoids a
// search request paying the cost of initializing.
func (s *Store) Start() {
	s.once.Do(func() {
		s.fetchLimiter = mutablelimiter.New(15)
		s.cache = &diskcache.Store{
			Dir:               s.Path,
			Component:         "store",
			BackgroundTimeout: 10 * time.Minute,
			BeforeEvict:       s.ZipCache.delete,
		}
		_ = os.MkdirAll(s.Path, 0700)
		metrics.MustRegisterDiskMonitor(s.Path)
		go s.watchAndEvict()
		go s.watchConfig()
	})
}

// PrepareZip returns the path to a local zip archive of repo at commit.
// It will first consult the local cache, otherwise will fetch from the network.
func (s *Store) PrepareZip(ctx context.Context, repo gitserver.Repo, commit api.CommitID) (path string, err error) {
	span, ctx := ot.StartSpanFromContext(ctx, "Store.prepareZip")
	ext.Component.Set(span, "store")
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", err.Error())
		}
		span.Finish()
	}()

	// Ensure we have initialized
	s.Start()

	// We already validate commit is absolute in ServeHTTP, but since we
	// rely on it for caching we check again.
	if len(commit) != 40 {
		return "", errors.Errorf("commit must be resolved (repo=%q, commit=%q)", repo.Name, commit)
	}

	largeFilePatterns := conf.Get().SearchLargeFiles

	// key is a sha256 hash since we want to use it for the disk name
	h := sha256.Sum256([]byte(fmt.Sprintf("%q %q %q", repo.Name, commit, largeFilePatterns)))
	key := hex.EncodeToString(h[:])
	span.LogKV("key", key)

	// Our fetch can take a long time, and the frontend aggressively cancels
	// requests. So we open in the background to give it extra time.
	type result struct {
		path string
		err  error
	}
	resC := make(chan result, 1)
	go func() {
		start := time.Now()
		// TODO: consider adding a cache method that doesn't actually bother opening the file,
		// since we're just going to close it again immediately.
		bgctx := opentracing.ContextWithSpan(context.Background(), opentracing.SpanFromContext(ctx))
		f, err := s.cache.Open(bgctx, key, func(ctx context.Context) (io.ReadCloser, error) {
			return s.fetch(ctx, repo, commit, largeFilePatterns)
		})
		var path string
		if f != nil {
			path = f.Path
			if f.File != nil {
				f.File.Close()
			}
		}
		if err != nil {
			log15.Error("failed to fetch archive", "repo", repo.Name, "commit", commit, "duration", time.Since(start), "error", err)
		}
		resC <- result{path, err}
	}()

	select {
	case <-ctx.Done():
		return "", ctx.Err()

	case res := <-resC:
		if res.err != nil {
			return "", res.err
		}
		return res.path, nil
	}
}

// fetch fetches an archive from the network and stores it on disk. It does
// not populate the in-memory cache. You should probably be calling
// prepareZip.
func (s *Store) fetch(ctx context.Context, repo gitserver.Repo, commit api.CommitID, largeFilePatterns []string) (rc io.ReadCloser, err error) {
	fetchQueueSize.Inc()
	ctx, releaseFetchLimiter, err := s.fetchLimiter.Acquire(ctx) // Acquire concurrent fetches semaphore
	if err != nil {
		return nil, err // err will be a context error
	}
	fetchQueueSize.Dec()

	// We expect git archive, even for large repos, to finish relatively
	// quickly.
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)

	fetching.Inc()
	span, ctx := ot.StartSpanFromContext(ctx, "Store.fetch")
	ext.Component.Set(span, "store")
	span.SetTag("repo", repo.Name)
	span.SetTag("repoURL", repo.URL)
	span.SetTag("commit", commit)

	// Done is called when the returned reader is closed, or if this function
	// returns an error. It should always be called once.
	doneCalled := false
	done := func(err error) {
		if doneCalled {
			panic("Store.fetch.done called twice")
		}
		doneCalled = true

		releaseFetchLimiter() // Release concurrent fetches semaphore
		cancel()              // Release context resources
		if err != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", err.Error())
			fetchFailed.Inc()
		}
		fetching.Dec()
		span.Finish()
	}
	defer func() {
		if rc == nil {
			done(err)
		}
	}()

	r, err := s.FetchTar(ctx, repo, commit)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()

	// After this point we are not allowed to return an error. Instead we can
	// return an error via the reader we return. If you do want to update this
	// code please ensure we still always call done once.

	// Write tr to zw. Return the first error encountered, but clean up if
	// we encounter an error.
	go func() {
		// clean up
		var err error
		defer func() {
			done(err)
			// CloseWithError is guaranteed to return a nil error
			_ = pw.CloseWithError(errors.Wrapf(err, "failed to fetch %s@%s", repo, commit))
			r.Close()
		}()

		// get ignore.Matcher
		var ig *ignore.Matcher
		var buf bytes.Buffer
		tee := io.TeeReader(r, &buf)
		ig, err = newIgnoreMatcher(tar.NewReader(tee))
		if err != nil {
			return
		}

		// for buf to contain the entire tar,
		// newIgnoreMatcher has to exhaust tee
		tr := tar.NewReader(&buf)
		zw := zip.NewWriter(pw)
		err = copySearchable(tr, zw, largeFilePatterns, ig)
		if err1 := zw.Close(); err == nil {
			err = err1
		}
	}()

	return pr, nil
}

// newIgnoreMatcher creates an ignore.Matcher from a tar-archive tr.
// Usually we could return early once we have found and parsed an
// ignore-file. However, we have to exhaust tr because newIgnoreMatcher
// is called in the context of an io.TeeReader.
func newIgnoreMatcher(tr *tar.Reader) (*ignore.Matcher, error) {
	ig := &ignore.Matcher{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return ig, nil
		}
		if err != nil {
			if err == tar.ErrHeader {
				return nil, temporaryError{error: err}
			}
			return nil, err
		}
		if hdr.Name == ignore.IgnoreFile {
			ig, err = ignore.ParseIgnoreFile(tr)
			if err != nil {
				return nil, err
			}
		}
	}
}

// copySearchable copies searchable files from tr to zw. A searchable file is
// any file that is under size limit, non-binary, and not matching an ignore-pattern.
func copySearchable(tr *tar.Reader, zw *zip.Writer, largeFilePatterns []string, ig *ignore.Matcher) error {
	// 32*1024 is the same size used by io.Copy
	buf := make([]byte, 32*1024)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			// Gitserver sometimes returns invalid headers. However, it only
			// seems to occur in situations where a retry would likely solve
			// it. So mark the error as temporary, to avoid failing the whole
			// search. https://github.com/sourcegraph/sourcegraph/issues/3799
			if err == tar.ErrHeader {
				return temporaryError{error: err}
			}
			return err
		}

		// We only care about files
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}

		// files matching an ignore-pattern are skipped and
		// not written to the cache
		if ig.Match(hdr.Name) {
			continue
		}

		// We are happy with the file, so we can write it to zw.
		w, err := zw.CreateHeader(&zip.FileHeader{
			Name:   hdr.Name,
			Method: zip.Store,
		})
		if err != nil {
			return err
		}

		n, err := tr.Read(buf)
		switch err {
		case io.EOF:
			if n == 0 {
				continue
			}
		case nil:
		default:
			return err
		}

		// We do not search the content of large files unless they are
		// allowed.
		if hdr.Size > maxFileSize && !ignoreSizeMax(hdr.Name, largeFilePatterns) {
			continue
		}

		// Heuristic: Assume file is binary if first 256 bytes contain a
		// 0x00. Best effort, so ignore err. We only search names of binary files.
		if n > 0 && bytes.IndexByte(buf[:n], 0x00) >= 0 {
			continue
		}

		// First write the data already read into buf
		nw, err := w.Write(buf[:n])
		if err != nil {
			return err
		}
		if nw != n {
			return io.ErrShortWrite
		}

		_, err = io.CopyBuffer(w, tr, buf)
		if err != nil {
			return err
		}

	}
}

func (s *Store) String() string {
	return "Store(" + s.Path + ")"
}

// watchAndEvict is a loop which periodically checks the size of the cache and
// evicts/deletes items if the store gets too large.
func (s *Store) watchAndEvict() {
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

// watchConfig updates fetchLimiter as the number of gitservers change.
func (s *Store) watchConfig() {
	for {
		// Allow roughly 10 fetches per gitserver
		limit := 10 * len(gitserver.DefaultClient.Addrs(context.Background()))
		if limit == 0 {
			limit = 15
		}
		s.fetchLimiter.SetLimit(limit)

		time.Sleep(10 * time.Second)
	}
}

// ignoreSizeMax determines whether the max size should be ignored. It uses
// the glob syntax found here: https://golang.org/pkg/path/filepath/#Match.
func ignoreSizeMax(name string, patterns []string) bool {
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if m, _ := filepath.Match(pattern, name); m {
			return true
		}
	}
	return false
}

var (
	cacheSizeBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "searcher_store_cache_size_bytes",
		Help: "The total size of items in the on disk cache.",
	})
	evictions = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "searcher_store_evictions",
		Help: "The total number of items evicted from the cache.",
	})
	fetching = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "searcher_store_fetching",
		Help: "The number of fetches currently running.",
	})
	fetchQueueSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "searcher_store_fetch_queue_size",
		Help: "The number of fetch jobs enqueued.",
	})
	fetchFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "searcher_store_fetch_failed",
		Help: "The total number of archive fetches that failed.",
	})
)

// temporaryError wraps an error but adds the Temporary method. It does not
// implement Cause so that errors.Cause() returns an error which implements
// Temporary.
type temporaryError struct {
	error
}

func (temporaryError) Temporary() bool {
	return true
}

func init() {
	prometheus.MustRegister(cacheSizeBytes)
	prometheus.MustRegister(evictions)
	prometheus.MustRegister(fetching)
	prometheus.MustRegister(fetchQueueSize)
	prometheus.MustRegister(fetchFailed)
}
