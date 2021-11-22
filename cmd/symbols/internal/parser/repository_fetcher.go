package parser

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"path"
	"strings"

	"github.com/opentracing/opentracing-go/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/cmd/symbols/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/cmd/symbols/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type RepositoryFetcher interface {
	FetchRepositoryArchive(ctx context.Context, args types.SearchArgs, paths []string) <-chan parseRequestOrError
}

type repositoryFetcher struct {
	gitserverClient gitserver.GitserverClient
	fetchSem        chan int
	operations      *operations
}

type parseRequest struct {
	path string
	data []byte
}

type parseRequestOrError struct {
	parseRequest parseRequest
	err          error
}

func NewRepositoryFetcher(gitserverClient gitserver.GitserverClient, maximumConcurrentFetches int, observationContext *observation.Context) RepositoryFetcher {
	return &repositoryFetcher{
		gitserverClient: gitserverClient,
		fetchSem:        make(chan int, maximumConcurrentFetches),
		operations:      newOperations(observationContext),
	}
}

func (f *repositoryFetcher) FetchRepositoryArchive(ctx context.Context, args types.SearchArgs, paths []string) <-chan parseRequestOrError {
	requestCh := make(chan parseRequestOrError)

	go func() {
		defer close(requestCh)

		if err := f.fetchRepositoryArchive(ctx, args, paths, func(request parseRequest) {
			requestCh <- parseRequestOrError{parseRequest: request}
		}); err != nil {
			requestCh <- parseRequestOrError{err: err}
		}
	}()

	return requestCh
}

func (f *repositoryFetcher) fetchRepositoryArchive(ctx context.Context, args types.SearchArgs, paths []string, callback func(request parseRequest)) (err error) {
	ctx, traceLog, endObservation := f.operations.fetchRepositoryArchive.WithAndLogger(ctx, &err, observation.Args{LogFields: []log.Field{
		log.String("repo", string(args.Repo)),
		log.String("commitID", string(args.CommitID)),
		log.Int("numPaths", len(paths)),
		log.String("paths", strings.Join(paths, ", ")),
	}})
	defer endObservation(1, observation.Args{})

	onDefer, err := f.limitConcurrentFetches(ctx)
	if err != nil {
		return err
	}
	defer onDefer()
	traceLog(log.Event("acquired fetch semaphore"))

	fetching.Inc()
	defer fetching.Dec()

	rc, err := f.gitserverClient.FetchTar(ctx, args.Repo, args.CommitID, paths)
	if err != nil {
		return err
	}
	defer rc.Close()

	return readTar(ctx, tar.NewReader(rc), callback, traceLog)
}

func (f *repositoryFetcher) limitConcurrentFetches(ctx context.Context) (func(), error) {
	fetchQueueSize.Inc()
	defer fetchQueueSize.Dec()

	select {
	case f.fetchSem <- 1:
		return func() { <-f.fetchSem }, nil

	case <-ctx.Done():
		return func() {}, ctx.Err()
	}
}

func readTar(ctx context.Context, tarReader *tar.Reader, callback func(request parseRequest), traceLog observation.TraceLogger) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		tarHeader, err := tarReader.Next()
		if err != nil {
			if err == io.EOF {
				return nil
			}

			return err
		}

		readTarHeader(tarReader, tarHeader, callback, traceLog)
	}
}

func readTarHeader(tarReader *tar.Reader, tarHeader *tar.Header, callback func(request parseRequest), traceLog observation.TraceLogger) error {
	if !shouldParse(tarHeader) {
		return nil
	}

	// 32MB is the same size used by io.Copy
	buffer := make([]byte, 32*1024)

	traceLog(log.Event("reading tar header prefix"))

	// Read first chunk of tar header contents
	n, err := tarReader.Read(buffer)
	if err != nil && err != io.EOF {
		return err
	}
	traceLog(log.Int("n", n))

	if n == 0 {
		// Empty file, nothing to parse
		return nil
	}

	// Check to see if first 256 bytes contain a 0x00. If so, we'll assume that
	// the file is binary and skip parsing. Otherwise, we'll have some non-zero
	// contents that passed our filters above to parse.

	m := 256
	if n < m {
		m = n
	}
	if bytes.IndexByte(buffer[:m], 0x00) >= 0 {
		return nil
	}

	// Copy buffer into appropriately-sized slice for return
	data := make([]byte, int(tarHeader.Size))
	copy(data, buffer[:n])

	if n < int(tarHeader.Size) {
		traceLog(log.Event("reading remaining tar header content"))

		// Read the remaining contents
		if _, err := io.ReadFull(tarReader, data[n:]); err != nil {
			return err
		}
		traceLog(log.Int("n", int(tarHeader.Size)-n))
	}

	request := parseRequest{path: tarHeader.Name, data: data}
	callback(request)
	return nil
}

// maxFileSize (512KB) is the maximum size of files we attempt to parse.
const maxFileSize = 1 << 19

func shouldParse(tarHeader *tar.Header) bool {
	// We do not search large files
	if tarHeader.Size > maxFileSize {
		return false
	}

	// We only care about files
	if tarHeader.Typeflag != tar.TypeReg && tarHeader.Typeflag != tar.TypeRegA {
		return false
	}

	// JSON files are symbol-less
	if path.Ext(tarHeader.Name) == ".json" {
		return false
	}

	return true
}

var (
	fetching = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "symbols_store_fetching",
		Help: "The number of fetches currently running.",
	})
	fetchQueueSize = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "symbols_store_fetch_queue_size",
		Help: "The number of fetch jobs enqueued.",
	})
	fetchFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "symbols_store_fetch_failed",
		Help: "The total number of archive fetches that failed.",
	})
)
