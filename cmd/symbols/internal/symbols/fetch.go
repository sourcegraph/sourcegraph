package symbols

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"path"

	"github.com/opentracing/opentracing-go/ext"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
)

type parseRequest struct {
	path string
	data []byte
}

func (s *Service) fetchRepositoryArchive(ctx context.Context, repo api.RepoName, commitID api.CommitID) (<-chan parseRequest, <-chan error, error) {
	fetchQueueSize.Inc()
	s.fetchSem <- 1 // acquire concurrent fetches semaphore
	fetchQueueSize.Dec()

	fetching.Inc()
	span, ctx := ot.StartSpanFromContext(ctx, "Store.fetch")
	ext.Component.Set(span, "store")
	span.SetTag("repo", repo)
	span.SetTag("commit", commitID)

	requestCh := make(chan parseRequest, s.NumParserProcesses)
	errCh := make(chan error, 1)

	// Done is called when the returned reader is closed, or if this function
	// returns an error. It should always be called once.
	doneCalled := false
	done := func(err error) {
		if doneCalled {
			panic("Store.fetch.done called twice")
		}
		doneCalled = true

		if err != nil {
			errCh <- err
		}

		<-s.fetchSem // release concurrent fetches semaphore
		close(requestCh)
		close(errCh)

		if err != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", err.Error())
			fetchFailed.Inc()
		}
		fetching.Dec()
		span.Finish()
	}

	r, err := s.FetchTar(ctx, repo, commitID)
	if err != nil {
		return nil, nil, err
	}

	// After this point we are not allowed to return an error. Instead we can
	// return an error via the errChan we return. If you do want to update this
	// code please ensure we still always call done once.

	go func() {
		defer r.Close()
		buf := make([]byte, 32*1024) // 32*1024 is the same size used by io.Copy
		tr := tar.NewReader(r)
		for {
			if ctx.Err() != nil {
				done(ctx.Err())
				return
			}

			hdr, err := tr.Next()
			if err == io.EOF {
				done(nil)
				return
			}
			if err != nil {
				done(err)
				return
			}

			if path.Ext(hdr.Name) == ".json" {
				continue
			}

			// We only care about files
			if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
				continue
			}
			// We do not search large files
			if hdr.Size > maxFileSize {
				continue
			}
			// Heuristic: Assume file is binary if first 256 bytes contain a 0x00. Best effort, so ignore err.
			n, err := tr.Read(buf)
			if n > 0 && bytes.IndexByte(buf[:n], 0x00) >= 0 {
				continue
			}
			switch err {
			case io.EOF:
				if n == 0 {
					continue
				}
			case nil:
			default:
				done(err)
				return
			}

			// Read the file's contents.
			data := make([]byte, int(hdr.Size))
			copy(data, buf[:n])
			if n < int(hdr.Size) {
				_, err = io.ReadFull(tr, data[n:])
				if err != nil {
					done(err)
					return
				}
			}
			requestCh <- parseRequest{path: hdr.Name, data: data}
		}
	}()

	return requestCh, errCh, nil
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
