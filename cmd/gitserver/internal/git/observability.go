package git

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	concurrentOps = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "src_gitserver_backend_concurrent_operations",
		Help: "Concurrent operations handled by Gitserver backends.",
	}, []string{"op"})
)

func NewObservableBackend(backend GitBackend) GitBackend {
	return &observableBackend{
		backend:    backend,
		operations: getOperations(),
	}
}

type observableBackend struct {
	operations *operations
	backend    GitBackend
}

func (b *observableBackend) BehindAhead(ctx context.Context, left, right string) (*gitdomain.BehindAhead, error) {
	ctx, _, endObservation := b.operations.getBehindAhead.With(ctx, nil, observation.Args{})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("BehindAhead").Inc()
	defer concurrentOps.WithLabelValues("BehindAhead").Dec()

	return b.backend.BehindAhead(ctx, left, right)
}

func (b *observableBackend) Config() GitConfigBackend {
	return &observableGitConfigBackend{
		backend:    b.backend.Config(),
		operations: b.operations,
	}
}

type observableGitConfigBackend struct {
	operations *operations
	backend    GitConfigBackend
}

func (b *observableGitConfigBackend) Get(ctx context.Context, key string) (_ string, err error) {
	ctx, _, endObservation := b.operations.configGet.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("Config.Get").Inc()
	defer concurrentOps.WithLabelValues("Config.Get").Dec()

	return b.backend.Get(ctx, key)
}

func (b *observableGitConfigBackend) Set(ctx context.Context, key, value string) (err error) {
	ctx, _, endObservation := b.operations.configSet.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("Config.Set").Inc()
	defer concurrentOps.WithLabelValues("Config.Set").Dec()

	return b.backend.Set(ctx, key, value)
}

func (b *observableGitConfigBackend) Unset(ctx context.Context, key string) (err error) {
	ctx, _, endObservation := b.operations.configUnset.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("Config.Unset").Inc()
	defer concurrentOps.WithLabelValues("Config.Unset").Dec()

	return b.backend.Unset(ctx, key)
}

func (b *observableBackend) GetObject(ctx context.Context, objectName string) (_ *gitdomain.GitObject, err error) {
	ctx, _, endObservation := b.operations.getObject.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("GetObject").Inc()
	defer concurrentOps.WithLabelValues("GetObject").Dec()

	return b.backend.GetObject(ctx, objectName)
}

func (b *observableBackend) MergeBase(ctx context.Context, baseRevspec, headRevspec string) (_ api.CommitID, err error) {
	ctx, _, endObservation := b.operations.mergeBase.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("MergeBase").Inc()
	defer concurrentOps.WithLabelValues("MergeBase").Dec()

	return b.backend.MergeBase(ctx, baseRevspec, headRevspec)
}

func (b *observableBackend) Blame(ctx context.Context, commit api.CommitID, path string, opt BlameOptions) (_ BlameHunkReader, err error) {
	ctx, errCollector, endObservation := b.operations.blame.WithErrors(ctx, &err, observation.Args{})
	ctx, cancel := context.WithCancel(ctx)
	endObservation.OnCancel(ctx, 1, observation.Args{})

	concurrentOps.WithLabelValues("Blame").Inc()

	hr, err := b.backend.Blame(ctx, commit, path, opt)
	if err != nil {
		concurrentOps.WithLabelValues("Blame").Dec()
		cancel()
		return nil, err
	}

	return &observableBlameHunkReader{
		inner: hr,
		onClose: func(err error) {
			concurrentOps.WithLabelValues("Blame").Dec()
			errCollector.Collect(&err)
			cancel()
		},
	}, nil
}

type observableBlameHunkReader struct {
	inner   BlameHunkReader
	onClose func(err error)
}

func (hr *observableBlameHunkReader) Read() (*gitdomain.Hunk, error) {
	return hr.inner.Read()
}

func (hr *observableBlameHunkReader) Close() error {
	err := hr.inner.Close()
	hr.onClose(err)
	return err
}

func (b *observableBackend) SymbolicRefHead(ctx context.Context, short bool) (_ string, err error) {
	ctx, _, endObservation := b.operations.symbolicRefHead.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("SymbolicRefHead").Inc()
	defer concurrentOps.WithLabelValues("SymbolicRefHead").Dec()

	return b.backend.SymbolicRefHead(ctx, short)
}

func (b *observableBackend) RevParseHead(ctx context.Context) (_ api.CommitID, err error) {
	ctx, _, endObservation := b.operations.revParseHead.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("RevParseHead").Inc()
	defer concurrentOps.WithLabelValues("RevParseHead").Dec()

	return b.backend.RevParseHead(ctx)
}

func (b *observableBackend) GetCommit(ctx context.Context, commit api.CommitID, includeModifiedFiles bool) (_ *GitCommitWithFiles, err error) {
	ctx, _, endObservation := b.operations.getCommit.With(ctx, &err, observation.Args{
		Attrs: []attribute.KeyValue{
			attribute.String("commit", string(commit)),
			attribute.Bool("includeModifiedFiles", includeModifiedFiles),
		},
	})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("GetCommit").Inc()
	defer concurrentOps.WithLabelValues("GetCommit").Dec()

	return b.backend.GetCommit(ctx, commit, includeModifiedFiles)
}

func (b *observableBackend) ReadFile(ctx context.Context, commit api.CommitID, path string) (_ io.ReadCloser, err error) {
	ctx, errCollector, endObservation := b.operations.readFile.WithErrors(ctx, &err, observation.Args{})
	ctx, cancel := context.WithCancel(ctx)
	endObservation.OnCancel(ctx, 1, observation.Args{})

	concurrentOps.WithLabelValues("ReadFile").Inc()

	r, err := b.backend.ReadFile(ctx, commit, path)
	if err != nil {
		concurrentOps.WithLabelValues("ReadFile").Dec()
		cancel()
		return nil, err
	}

	return &observableReadCloser{
		inner: r,
		endObservation: func(err error) {
			concurrentOps.WithLabelValues("ReadFile").Dec()
			errCollector.Collect(&err)
			cancel()
		},
	}, nil
}

func (b *observableBackend) Exec(ctx context.Context, args ...string) (_ io.ReadCloser, err error) {
	ctx, errCollector, endObservation := b.operations.exec.WithErrors(ctx, &err, observation.Args{})
	ctx, cancel := context.WithCancel(ctx)
	endObservation.OnCancel(ctx, 1, observation.Args{})

	concurrentOps.WithLabelValues("Exec").Inc()

	r, err := b.backend.Exec(ctx, args...)
	if err != nil {
		concurrentOps.WithLabelValues("Exec").Dec()
		cancel()
		return nil, err
	}

	return &observableReadCloser{
		inner: r,
		endObservation: func(err error) {
			concurrentOps.WithLabelValues("Exec").Dec()
			errCollector.Collect(&err)
			cancel()
		},
	}, nil
}

func (b *observableBackend) ArchiveReader(ctx context.Context, format ArchiveFormat, treeish string, paths []string) (_ io.ReadCloser, err error) {
	ctx, errCollector, endObservation := b.operations.archiveReader.WithErrors(ctx, &err, observation.Args{})
	ctx, cancel := context.WithCancel(ctx)
	endObservation.OnCancel(ctx, 1, observation.Args{})

	concurrentOps.WithLabelValues("ArchiveReader").Inc()

	r, err := b.backend.ArchiveReader(ctx, format, treeish, paths)
	if err != nil {
		concurrentOps.WithLabelValues("ArchiveReader").Dec()
		cancel()
		return nil, err
	}

	return &observableReadCloser{
		inner: r,
		endObservation: func(err error) {
			concurrentOps.WithLabelValues("ArchiveReader").Dec()
			errCollector.Collect(&err)
			cancel()
		},
	}, nil
}

func (b *observableBackend) ResolveRevision(ctx context.Context, revspec string) (_ api.CommitID, err error) {
	ctx, _, endObservation := b.operations.resolveRevision.With(ctx, &err, observation.Args{
		Attrs: []attribute.KeyValue{
			attribute.String("revspec", revspec),
		},
	})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("ResolveRevision").Inc()
	defer concurrentOps.WithLabelValues("ResolveRevision").Dec()

	return b.backend.ResolveRevision(ctx, revspec)
}

func (b *observableBackend) RevAtTime(ctx context.Context, revspec string, t time.Time) (_ api.CommitID, err error) {
	ctx, _, endObservation := b.operations.revAtTime.With(ctx, &err, observation.Args{
		Attrs: []attribute.KeyValue{
			attribute.String("revspec", revspec),
		},
	})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("RevAtTime").Inc()
	defer concurrentOps.WithLabelValues("RevAtTime").Dec()

	return b.backend.RevAtTime(ctx, revspec, t)
}

type observableReadCloser struct {
	inner          io.ReadCloser
	endObservation func(err error)
}

func (r *observableReadCloser) Read(p []byte) (int, error) {
	return r.inner.Read(p)
}

func (r *observableReadCloser) Close() error {
	err := r.inner.Close()
	r.endObservation(err)
	return err
}

func (b *observableBackend) ListRefs(ctx context.Context, opt ListRefsOpts) (_ RefIterator, err error) {
	ctx, errCollector, endObservation := b.operations.listRefs.WithErrors(ctx, &err, observation.Args{})
	ctx, cancel := context.WithCancel(ctx)
	endObservation.OnCancel(ctx, 1, observation.Args{})

	concurrentOps.WithLabelValues("ListRefs").Inc()

	it, err := b.backend.ListRefs(ctx, opt)
	if err != nil {
		concurrentOps.WithLabelValues("ListRefs").Dec()
		cancel()
		return nil, err
	}

	return &observableRefIterator{
		inner: it,
		onClose: func(err error) {
			concurrentOps.WithLabelValues("ListRefs").Dec()
			errCollector.Collect(&err)
			cancel()
		},
	}, nil
}

type observableRefIterator struct {
	inner   RefIterator
	onClose func(err error)
}

func (hr *observableRefIterator) Next() (*gitdomain.Ref, error) {
	return hr.inner.Next()
}

func (hr *observableRefIterator) Close() error {
	err := hr.inner.Close()
	hr.onClose(err)
	return err
}

func (b *observableBackend) RawDiff(ctx context.Context, base string, head string, typ GitDiffComparisonType, paths ...string) (_ io.ReadCloser, err error) {
	ctx, errCollector, endObservation := b.operations.rawDiff.WithErrors(ctx, &err, observation.Args{})
	ctx, cancel := context.WithCancel(ctx)
	endObservation.OnCancel(ctx, 1, observation.Args{})

	concurrentOps.WithLabelValues("RawDiff").Inc()

	r, err := b.backend.RawDiff(ctx, base, head, typ, paths...)
	if err != nil {
		concurrentOps.WithLabelValues("RawDiff").Dec()
		cancel()
		return nil, err
	}

	return &observableReadCloser{
		inner: r,
		endObservation: func(err error) {
			concurrentOps.WithLabelValues("RawDiff").Dec()
			errCollector.Collect(&err)
			cancel()
		},
	}, nil
}

func (b *observableBackend) ContributorCounts(ctx context.Context, opt ContributorCountsOpts) (_ []*gitdomain.ContributorCount, err error) {
	ctx, _, endObservation := b.operations.contributorCounts.With(ctx, &err, observation.Args{
		Attrs: []attribute.KeyValue{
			attribute.String("range", opt.Range),
			attribute.Stringer("after", opt.After),
			attribute.String("path", opt.Path),
		},
	})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("ContributorCounts").Inc()
	defer concurrentOps.WithLabelValues("ContributorCounts").Dec()

	return b.backend.ContributorCounts(ctx, opt)
}

func (b *observableBackend) FirstEverCommit(ctx context.Context) (_ api.CommitID, err error) {
	ctx, _, endObservation := b.operations.firstEverCommit.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("FirstEverCommit").Inc()
	defer concurrentOps.WithLabelValues("FirstEverCommit").Dec()

	return b.backend.FirstEverCommit(ctx)
}

func (b *observableBackend) ChangedFiles(ctx context.Context, base, head string) (ChangedFilesIterator, error) {
	ctx, _, endObservation := b.operations.changedFiles.With(ctx, nil, observation.Args{})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("ChangedFiles").Inc()
	defer concurrentOps.WithLabelValues("ChangedFiles").Dec()

	return b.backend.ChangedFiles(ctx, base, head)
}

func (b *observableBackend) Stat(ctx context.Context, commit api.CommitID, path string) (_ fs.FileInfo, err error) {
	ctx, _, endObservation := b.operations.stat.With(ctx, &err, observation.Args{
		Attrs: []attribute.KeyValue{
			attribute.String("commit", string(commit)),
			attribute.String("path", path),
		},
	})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("Stat").Inc()
	defer concurrentOps.WithLabelValues("Stat").Dec()

	return b.backend.Stat(ctx, commit, path)
}

func (b *observableBackend) ReadDir(ctx context.Context, commit api.CommitID, path string, recursive bool) (_ ReadDirIterator, err error) {
	ctx, errCollector, endObservation := b.operations.readDir.WithErrors(ctx, &err, observation.Args{
		Attrs: []attribute.KeyValue{
			attribute.String("commit", string(commit)),
			attribute.String("path", path),
			attribute.Bool("recursive", recursive),
		},
	})
	ctx, cancel := context.WithCancel(ctx)
	endObservation.OnCancel(ctx, 1, observation.Args{})

	concurrentOps.WithLabelValues("ReadDir").Inc()

	it, err := b.backend.ReadDir(ctx, commit, path, recursive)
	if err != nil {
		concurrentOps.WithLabelValues("ReadDir").Dec()
		cancel()
		return nil, err
	}

	return &observableReadDirIterator{
		inner: it,
		onClose: func(err error) {
			concurrentOps.WithLabelValues("ReadDir").Dec()
			errCollector.Collect(&err)
			cancel()
		},
	}, nil
}

type observableReadDirIterator struct {
	inner   ReadDirIterator
	onClose func(err error)
}

func (hr *observableReadDirIterator) Next() (fs.FileInfo, error) {
	return hr.inner.Next()
}

func (hr *observableReadDirIterator) Close() error {
	err := hr.inner.Close()
	hr.onClose(err)
	return err
}

func (b *observableBackend) LatestCommitTimestamp(ctx context.Context) (_ time.Time, err error) {
	ctx, _, endObservation := b.operations.latestCommitTimestamp.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("LatestCommitTimestamp").Inc()
	defer concurrentOps.WithLabelValues("LatestCommitTimestamp").Dec()

	return b.backend.LatestCommitTimestamp(ctx)
}

func (b *observableBackend) RefHash(ctx context.Context) (_ []byte, err error) {
	ctx, _, endObservation := b.operations.refHash.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	concurrentOps.WithLabelValues("RefHash").Inc()
	defer concurrentOps.WithLabelValues("RefHash").Dec()

	return b.backend.RefHash(ctx)
}

func (b *observableBackend) UploadPack(ctx context.Context, input io.Reader, protocol string, advertiseRefs bool) (_ io.ReadCloser, err error) {
	ctx, errCollector, endObservation := b.operations.uploadPack.WithErrors(ctx, &err, observation.Args{
		Attrs: []attribute.KeyValue{
			attribute.Bool("advertiseRefs", advertiseRefs),
			attribute.String("protocol", protocol),
		},
	})
	ctx, cancel := context.WithCancel(ctx)
	endObservation.OnCancel(ctx, 1, observation.Args{})

	concurrentOps.WithLabelValues("UploadPack").Inc()

	r, err := b.backend.UploadPack(ctx, input, protocol, advertiseRefs)
	if err != nil {
		concurrentOps.WithLabelValues("UploadPack").Dec()
		cancel()
		return nil, err
	}

	return &observableReadCloser{
		inner: r,
		endObservation: func(err error) {
			concurrentOps.WithLabelValues("UploadPack").Dec()
			errCollector.Collect(&err)
			cancel()
		},
	}, nil
}

type operations struct {
	configGet             *observation.Operation
	configSet             *observation.Operation
	configUnset           *observation.Operation
	getObject             *observation.Operation
	mergeBase             *observation.Operation
	blame                 *observation.Operation
	symbolicRefHead       *observation.Operation
	revParseHead          *observation.Operation
	readFile              *observation.Operation
	exec                  *observation.Operation
	getCommit             *observation.Operation
	archiveReader         *observation.Operation
	resolveRevision       *observation.Operation
	listRefs              *observation.Operation
	revAtTime             *observation.Operation
	rawDiff               *observation.Operation
	contributorCounts     *observation.Operation
	firstEverCommit       *observation.Operation
	getBehindAhead        *observation.Operation
	changedFiles          *observation.Operation
	stat                  *observation.Operation
	readDir               *observation.Operation
	latestCommitTimestamp *observation.Operation
	refHash               *observation.Operation
	uploadPack            *observation.Operation
}

func newOperations(observationCtx *observation.Context) *operations {
	redMetrics := metrics.NewREDMetrics(
		observationCtx.Registerer,
		"gitserver_backend",
		metrics.WithLabels("op"),
		metrics.WithCountHelp("Total number of method invocations."),
	)

	op := func(name string) *observation.Operation {
		return observationCtx.Operation(observation.Op{
			Name:              fmt.Sprintf("gitserver.backend.%s", name),
			MetricLabelValues: []string{name},
			Metrics:           redMetrics,
			ErrorFilter: func(err error) observation.ErrorFilterBehaviour {
				if errors.HasType(err, &gitdomain.RevisionNotFoundError{}) {
					return observation.EmitForHoney | observation.EmitForTraces
				}
				if errors.Is(err, os.ErrNotExist) {
					return observation.EmitForHoney | observation.EmitForTraces
				}
				return observation.EmitForDefault
			},
		})
	}

	return &operations{
		configGet:             op("config-get"),
		configSet:             op("config-set"),
		configUnset:           op("config-unset"),
		getObject:             op("get-object"),
		mergeBase:             op("merge-base"),
		blame:                 op("blame"),
		symbolicRefHead:       op("symbolic-ref-head"),
		revParseHead:          op("rev-parse-head"),
		readFile:              op("read-file"),
		exec:                  op("exec"),
		getCommit:             op("get-commit"),
		archiveReader:         op("archive-reader"),
		resolveRevision:       op("resolve-revision"),
		listRefs:              op("list-refs"),
		revAtTime:             op("rev-at-time"),
		rawDiff:               op("raw-diff"),
		contributorCounts:     op("contributor-counts"),
		firstEverCommit:       op("first-ever-commit"),
		getBehindAhead:        op("get-behind-ahead"),
		changedFiles:          op("changed-files"),
		stat:                  op("stat"),
		readDir:               op("read-dir"),
		latestCommitTimestamp: op("latest-commit-timestamp"),
		refHash:               op("ref-hash"),
		uploadPack:            op("upload-pack"),
	}
}

var (
	operationsInst     *operations
	operationsInstOnce sync.Once
)

func getOperations() *operations {
	operationsInstOnce.Do(func() {
		observationCtx := observation.NewContext(log.Scoped("gitserver.backend"))
		operationsInst = newOperations(observationCtx)
	})

	return operationsInst
}
