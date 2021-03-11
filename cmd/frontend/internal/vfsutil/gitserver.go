package vfsutil

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/opentracing/opentracing-go/ext"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
)

// NewGitServer returns a VFS to repo at commit. It is backed by an archive
// fetched from gitserver.
func NewGitServer(repo api.RepoName, commit api.CommitID) *ArchiveFS {
	fetch := func(ctx context.Context) (ar *archiveReader, err error) {
		f, evictor, err := GitServerFetchArchive(ctx, ArchiveOpts{
			Repo:         repo,
			Commit:       commit,
			Format:       ArchiveFormatZip,
			RelativePath: ".",
		})
		if err != nil {
			return nil, err
		}

		zr, err := zipNewFileReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}

		return &archiveReader{
			Reader:  zr,
			Closer:  f,
			Evicter: evictor,
		}, nil
	}
	return &ArchiveFS{fetch: fetch}
}

// ArchiveFormat represents an archive format (zip, tar, etc).
type ArchiveFormat string

const (
	// ArchiveFormatZip indicates a zip archive is desired.
	ArchiveFormatZip ArchiveFormat = "zip"

	// ArchiveFormatTar indicates a tar archive is desired.
	ArchiveFormatTar ArchiveFormat = "tar"
)

// ArchiveOpts describes options for fetching a repository archive.
type ArchiveOpts struct {
	// Repo is the repository whose contents should be fetched.
	Repo api.RepoName

	// Commit is the commit whose contents should be fetched.
	Commit api.CommitID

	// Format indicates the desired archive format.
	Format ArchiveFormat

	// RelativePath indicates the path of the repository that should be archived.
	RelativePath string
}

func (opts *ArchiveOpts) cacheKey() string {
	return fmt.Sprintf("%s@%s/-/%s.%s", opts.Repo, opts.Commit, opts.RelativePath, opts.Format)
}

// GitServerFetchArchive fetches an archive of a repositories contents from gitserver.
func GitServerFetchArchive(ctx context.Context, opts ArchiveOpts) (archive *os.File, cacheEvicter Evicter, err error) {
	span, ctx := ot.StartSpanFromContext(ctx, "Archive Fetch")
	ext.Component.Set(span, "gitserver")
	span.SetTag("repo", opts.Repo)
	span.SetTag("commit", opts.Commit)
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", err)
		}
		span.Finish()
	}()

	if strings.HasPrefix(string(opts.Commit), "-") {
		return nil, nil, errors.New("invalid git revision spec (begins with '-')")
	}

	ff, err := cachedFetch(ctx, "gitserver", opts.cacheKey(), func(ctx context.Context) (io.ReadCloser, error) {
		gitserverFetchTotal.Inc()

		args := []string{"archive", "--format=" + string(opts.Format)}
		if opts.Format == ArchiveFormatZip {
			// Compression level of 0 (no compression) seems to perform the
			// best overall on fast network links, but this has not been tuned
			// thoroughly.
			args = append(args, "-0")
		}
		args = append(args, string(opts.Commit))
		args = append(args, opts.RelativePath)
		cmd := gitserver.DefaultClient.Command("git", args...)
		cmd.Repo = opts.Repo
		r, err := gitserver.StdoutReader(ctx, cmd)
		if err != nil {
			return nil, err
		}
		return r, nil
	})
	if err != nil {
		gitserverFetchFailedTotal.Inc()
		return nil, nil, err
	}
	return ff.File, ff, nil
}

var gitserverFetchTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "vfsutil_vfs_gitserver_fetch_total",
	Help: "Total number of fetches to GitServer.",
})

var gitserverFetchFailedTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "vfsutil_vfs_gitserver_fetch_failed_total",
	Help: "Total number of fetches to GitServer that failed.",
})
