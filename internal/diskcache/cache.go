package diskcache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log" //nolint:logging // TODO move all logging to sourcegraph/log
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/observation"
	internaltrace "github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Store is an on-disk cache, with items cached via calls to Open.
type Store interface {
	// Open will open a file from the local cache with key. If missing, fetcher
	// will fill the cache first. Open also performs single-flighting for fetcher.
	Open(ctx context.Context, key []string, fetcher Fetcher) (file *File, err error)
	// OpenWithPath will open a file from the local cache with key. If missing, fetcher
	// will fill the cache first. OpenWithPath also performs single-flighting for fetcher.
	OpenWithPath(ctx context.Context, key []string, fetcher FetcherWithPath) (file *File, err error)
	// Evict will remove files from store.Dir until it is smaller than
	// maxCacheSizeBytes. It evicts files with the oldest modification time first.
	Evict(maxCacheSizeBytes int64) (stats EvictStats, err error)
}

type store struct {
	// dir is the directory to cache items.
	dir string

	// component when set is reported to OpenTracing as the component.
	component string

	// backgroundTimeout when non-zero will do fetches in the background with
	// a timeout. This means the context passed to fetch will be
	// context.WithTimeout(context.Background(), backgroundTimeout). When not
	// set fetches are done with the passed in context.
	backgroundTimeout time.Duration

	// beforeEvict, when non-nil, is a function to call before evicting a file.
	// It is passed the path to the file to be evicted and an observation.TraceLogger
	// which can be used to attach fields to a Honeycomb event.
	beforeEvict func(string, observation.TraceLogger)

	observe *operations
}

// NewStore returns a new on-disk cache, which caches data under dir.
//
// It can optionally be configured with a background timeout
// (with `diskcache.WithBackgroundTimeout`), a pre-evict callback
// (with `diskcache.WithBeforeEvict`) and with a configured observation context
// (with `diskcache.WithobservationCtx`).
func NewStore(dir, component string, opts ...StoreOpt) Store {
	s := &store{
		dir:       dir,
		component: component,
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.observe == nil {
		s.observe = newOperations(&observation.Context{}, component)
	}

	return s
}

type StoreOpt func(*store)

func WithBackgroundTimeout(t time.Duration) func(*store) {
	return func(s *store) { s.backgroundTimeout = t }
}

func WithBeforeEvict(f func(string, observation.TraceLogger)) func(*store) {
	return func(s *store) { s.beforeEvict = f }
}

func WithobservationCtx(ctx *observation.Context) func(*store) {
	return func(s *store) { s.observe = newOperations(ctx, s.component) }
}

// File is an os.File, but includes the Path
type File struct {
	*os.File

	// The Path on disk for File
	Path string
}

// Fetcher returns a ReadCloser. It is used by Open if the key is not in the
// cache.
type Fetcher func(context.Context) (io.ReadCloser, error)

// FetcherWithPath writes a cache entry to the given file. It is used by Open if the key
// is not in the cache.
type FetcherWithPath func(context.Context, string) error

func (s *store) Open(ctx context.Context, key []string, fetcher Fetcher) (file *File, err error) {
	return s.OpenWithPath(ctx, key, func(ctx context.Context, path string) error {
		readCloser, err := fetcher(ctx)
		if err != nil {
			return err
		}
		file, err := os.OpenFile(path, os.O_WRONLY, 0o600)
		if err != nil {
			readCloser.Close()
			return errors.Wrap(err, "failed to open temporary archive cache item")
		}
		err = copyAndClose(file, readCloser)
		if err != nil {
			return errors.Wrap(err, "failed to copy and close missing archive cache item")
		}
		return nil
	})
}

func (s *store) OpenWithPath(ctx context.Context, key []string, fetcher FetcherWithPath) (file *File, err error) {
	ctx, trace, endObservation := s.observe.cachedFetch.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("component", s.component),
	}})
	defer endObservation(1, observation.Args{})

	defer func() {
		if file != nil {
			// Update modified time. Modified time is used to decide which
			// files to evict from the cache.
			touch(file.Path)
		}
	}()

	if s.dir == "" {
		return nil, errors.New("diskcache.store.Dir must be set")
	}

	path := s.path(key)
	trace.AddEvent("TODO Domain Owner", attribute.String("key", fmt.Sprint(key)), attribute.String("path", path))

	err = os.MkdirAll(filepath.Dir(path), os.ModePerm)
	if err != nil {
		return nil, err
	}

	// First do a fast-path, assume already on disk
	f, err := os.Open(path)
	if err == nil {
		trace.SetAttributes(attribute.String("source", "fast"))
		return &File{File: f, Path: path}, nil
	}

	// We (probably) have to fetch
	trace.SetAttributes(attribute.String("source", "fetch"))

	// Do the fetch in another goroutine so we can respect ctx cancellation.
	type result struct {
		f   *File
		err error
	}
	ch := make(chan result, 1)
	go func() {
		var err error
		var f *File
		ctx, trace, endObservation := s.observe.backgroundFetch.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
			attribute.Bool("withBackgroundTimeout", s.backgroundTimeout != 0),
		}})
		defer endObservation(1, observation.Args{})

		if s.backgroundTimeout != 0 {
			var cancel context.CancelFunc
			ctx, cancel = withIsolatedTimeout(ctx, s.backgroundTimeout)
			defer cancel()
		}
		f, err = doFetch(ctx, path, fetcher, trace)
		ch <- result{f, err}
	}()

	select {
	case <-ctx.Done():
		// *os.File sets a finalizer to close the file when no longer used, so
		// we don't need to worry about closing the file in the case of context
		// cancellation.
		return nil, ctx.Err()
	case r := <-ch:
		return r.f, r.err
	}
}

// path returns the path for key.
func (s *store) path(key []string) string {
	encoded := append([]string{s.dir}, EncodeKeyComponents(key)...)
	return filepath.Join(encoded...) + ".zip"
}

// EncodeKeyComponents uses a sha256 hash of the key since we want to use it for the disk name.
func EncodeKeyComponents(components []string) []string {
	encoded := []string{}
	for _, component := range components {
		h := sha256.Sum256([]byte(component))
		encoded = append(encoded, hex.EncodeToString(h[:]))
	}
	return encoded
}

func doFetch(ctx context.Context, path string, fetcher FetcherWithPath, trace observation.TraceLogger) (file *File, err error) {
	// We have to grab the lock for this key, so we can fetch or wait for
	// someone else to finish fetching.
	urlMu := urlMu(path)
	t := time.Now()
	urlMu.Lock()
	defer urlMu.Unlock()

	trace.AddEvent("acquired url lock", attribute.Int64("urlLock.durationMs", time.Since(t).Milliseconds()))

	// Since we acquired the lock we may have timed out.
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Since we acquired urlMu, someone else may have put the archive onto
	// the disk.
	f, err := os.Open(path)
	if err == nil {
		return &File{File: f, Path: path}, nil
	}
	// Just in case we failed due to something bad on the FS, remove
	_ = os.Remove(path)

	// Fetch since we still can't open up the file
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, errors.Wrap(err, "could not create archive cache dir")
	}

	// We write to a temporary path to prevent another Open finding a
	// partially written file. We ensure the file is writeable and truncate
	// it.
	tmpPath := path + ".part"
	f, err = os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temporary archive cache item")
	}
	f.Close()
	defer os.Remove(tmpPath)

	// We are now ready to actually fetch the file.
	err = fetcher(ctx, tmpPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch missing archive cache item")
	}

	// Sync the contents to disk. If we crash we don't want to leave behind
	// invalid zip files due to unwritten OS buffers.
	if err := fsync(tmpPath); err != nil {
		return nil, errors.Wrap(err, "failed to sync cache item to disk")
	}

	// Put the partially written file in the correct place and open
	err = os.Rename(tmpPath, path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to put cache item in place")
	}

	// Sync the directory. We need to ensure the rename is recorded to disk.
	if err := fsync(filepath.Dir(path)); err != nil {
		return nil, errors.Wrap(err, "failed to sync cache directory to disk")
	}

	f, err = os.Open(path)
	if err != nil {
		return nil, err
	}
	return &File{File: f, Path: path}, nil
}

// EvictStats is information gathered during Evict.
type EvictStats struct {
	// CacheSize is the size of the cache before evicting.
	CacheSize int64

	// Evicted is the number of items evicted.
	Evicted int
}

func (s *store) Evict(maxCacheSizeBytes int64) (stats EvictStats, err error) {
	_, trace, endObservation := s.observe.evict.With(context.Background(), &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.Int64("maxCacheSizeBytes", maxCacheSizeBytes),
	}})
	endObservation(1, observation.Args{})

	isZip := func(fi fs.FileInfo) bool {
		return strings.HasSuffix(fi.Name(), ".zip")
	}

	type absFileInfo struct {
		absPath string
		info    fs.FileInfo
	}
	entries := []absFileInfo{}
	err = filepath.Walk(s.dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					// we can race with diskcache renaming tmp files to final
					// destination. Just ignore these files rather than returning
					// early.
					return nil
				}

				return err
			}
			if !info.IsDir() {
				entries = append(entries, absFileInfo{absPath: path, info: info})
			}
			return nil
		})
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil
		}
		return stats, errors.Wrapf(err, "failed to ReadDir %s", s.dir)
	}

	// Sum up the total size of all zips
	var size int64
	for _, entry := range entries {
		size += entry.info.Size()
	}
	stats.CacheSize = size

	// Nothing to evict
	if size <= maxCacheSizeBytes {
		return stats, nil
	}

	// Keep removing files until we are under the cache size. Remove the
	// oldest first.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].info.ModTime().Before(entries[j].info.ModTime())
	})
	for _, entry := range entries {
		if size <= maxCacheSizeBytes {
			break
		}
		if !isZip(entry.info) {
			continue
		}
		path := entry.absPath
		if s.beforeEvict != nil {
			s.beforeEvict(path, trace)
		}
		err = os.Remove(path)
		if err != nil {
			trace.AddEvent("failed to remove disk cache entry", attribute.String("path", path), internaltrace.Error(err))
			log.Printf("failed to remove %s: %s", path, err)
			continue
		}
		stats.Evicted++
		size -= entry.info.Size()
	}

	trace.SetAttributes(
		attribute.Int("evicted", stats.Evicted),
		attribute.Int64("beforeSizeBytes", stats.CacheSize),
		attribute.Int64("afterSizeBytes", size),
	)

	return stats, nil
}

func copyAndClose(dst io.WriteCloser, src io.ReadCloser) error {
	_, err := io.Copy(dst, src)
	if err1 := src.Close(); err == nil {
		err = err1
	}
	if err1 := dst.Close(); err == nil {
		err = err1
	}
	return err
}

// touch updates the modified time to time.Now(). It is best-effort, and will
// log if it fails.
func touch(path string) {
	t := time.Now()
	if err := os.Chtimes(path, t, t); err != nil {
		log.Printf("failed to touch %s: %s", path, err)
	}
}

func fsync(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	err = f.Sync()
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err
}
