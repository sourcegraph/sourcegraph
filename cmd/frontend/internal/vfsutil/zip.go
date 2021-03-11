package vfsutil

import (
	"context"
	"io"
	"net/http"

	"golang.org/x/net/context/ctxhttp"

	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/trace/ot"

	"github.com/opentracing/opentracing-go/ext"
)

// NewZipVFS downloads a zip archive from a URL (or fetches from the local cache
// on disk) and returns a new VFS backed by that zip archive.
func NewZipVFS(url string, onFetchStart, onFetchFailed func(), evictOnClose bool) (*ArchiveFS, error) {
	fetch := func(ctx context.Context) (ar *archiveReader, err error) {
		span, ctx := ot.StartSpanFromContext(ctx, "zip Fetch")
		ext.Component.Set(span, "zipvfs")
		span.SetTag("url", url)
		defer func() {
			if err != nil {
				ext.Error.Set(span, true)
				span.SetTag("err", err)
			}
			span.Finish()
		}()

		ff, err := cachedFetch(ctx, "zipvfs", url, func(ctx context.Context) (io.ReadCloser, error) {
			onFetchStart()
			request, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to construct a new request with URL %s", url)
			}
			request.Header.Add("Accept", "application/zip")
			resp, err := ctxhttp.Do(ctx, nil, request)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to fetch zip archive from %s", url)
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				return nil, errors.Errorf("zip URL %s returned HTTP %d", url, resp.StatusCode)
			}
			return resp.Body, nil
		})
		if err != nil {
			onFetchFailed()
			return nil, errors.Wrapf(err, "failed to fetch/write/open zip archive from %s", url)
		}
		f := ff.File

		zr, err := zipNewFileReader(f)
		if err != nil {
			f.Close()
			return nil, errors.Wrapf(err, "failed to read zip archive from %s", url)
		}

		if len(zr.File) == 0 {
			f.Close()
			return nil, errors.Errorf("zip archive from %s is empty", url)
		}

		return &archiveReader{
			Reader:           zr,
			Closer:           f,
			StripTopLevelDir: true,
		}, nil
	}

	return &ArchiveFS{fetch: fetch, EvictOnClose: evictOnClose}, nil
}
