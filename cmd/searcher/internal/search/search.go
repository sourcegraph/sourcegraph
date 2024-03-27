// Package search is a service which exposes an API to text search a repo at
// a specific commit.
//
// Architecture Notes:
//   - Archive is fetched from gitserver
//   - Simple HTTP API exposed
//   - Currently no concept of authorization
//   - On disk cache of fetched archives to reduce load on gitserver
//   - Run search on archive. Rely on OS file buffers
//   - Simple to scale up since stateless
//   - Use ingress with affinity to increase local cache hit ratio
package search

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/zoekt"

	"github.com/sourcegraph/sourcegraph/cmd/searcher/protocol"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	// numWorkers is how many concurrent goroutines run in the case of
	// regexSearch, and the number of parallel workers in the case of
	// structuralSearch.
	numWorkers = 8
)

// Service is the search service. It is an http.Handler.
type Service struct {
	Store *Store
	Log   log.Logger

	Indexed zoekt.Streamer

	// GitDiffSymbols returns the stdout of running "git diff -z --name-status
	// --no-renames commitA commitB" against repo.
	//
	// TODO Git client should be exposing a better API here.
	GitDiffSymbols func(ctx context.Context, repo api.RepoName, commitA, commitB api.CommitID) ([]byte, error)

	// MaxTotalPathsLength is the maximum sum of lengths of all paths in a
	// single call to git archive. This mainly needs to be less than ARG_MAX
	// for the exec.Command on gitserver.
	MaxTotalPathsLength int

	// DisableHybridSearch disables hybrid search, which means searcher will not
	// call out to Zoekt to offload part of the search workload. This is useful for
	// troubleshooting, for example if searcher causes a high load on Zoekt.
	DisableHybridSearch bool
}

func (s *Service) search(ctx context.Context, p *protocol.Request, sender matchSender) (err error) {
	metricRunning.Inc()
	defer metricRunning.Dec()

	var tr trace.Trace
	tr, ctx = trace.New(ctx, "search",
		p.Repo.Attr(),
		p.Commit.Attr(),
		attribute.String("url", p.URL),
		attribute.String("query", p.Query.String()),
		attribute.StringSlice("languages", p.IncludeLangs),
		attribute.Bool("isCaseSensitive", p.IsCaseSensitive),
		attribute.Bool("pathPatternsAreCaseSensitive", p.PathPatternsAreCaseSensitive),
		attribute.Int("limit", p.Limit),
		attribute.Bool("patternMatchesContent", p.PatternMatchesContent),
		attribute.Bool("patternMatchesPath", p.PatternMatchesPath),
		attribute.String("select", p.Select))
	defer tr.End()
	defer func(start time.Time) {
		code := "200"
		// We often have canceled and timed out requests. We do not want to
		// record them as errors to avoid noise
		if ctx.Err() == context.Canceled {
			code = "canceled"
			tr.SetError(err)
		} else if err != nil {
			tr.SetError(err)
			if errcode.IsBadRequest(err) {
				code = "400"
			} else if errcode.IsTemporary(err) {
				code = "503"
			} else {
				code = "500"
			}
		}
		metricRequestTotal.WithLabelValues(code).Inc()
		tr.AddEvent("done",
			attribute.String("code", code),
			attribute.Int("matches.len", sender.SentCount()),
			attribute.Bool("limitHit", sender.LimitHit()),
		)
		s.Log.Debug("search request",
			log.String("repo", string(p.Repo)),
			log.String("commit", string(p.Commit)),
			log.String("query", p.String()),
			log.Bool("isStructuralPat", p.IsStructuralPat),
			log.Strings("languages", p.IncludeLangs),
			log.Bool("isCaseSensitive", p.IsCaseSensitive),
			log.Bool("patternMatchesContent", p.PatternMatchesContent),
			log.Bool("patternMatchesPath", p.PatternMatchesPath),
			log.Int("matches", sender.SentCount()),
			log.String("code", code),
			log.Duration("duration", time.Since(start)),
			log.Error(err))
	}(time.Now())

	if p.FetchTimeout == time.Duration(0) {
		p.FetchTimeout = 500 * time.Millisecond
	}

	if p.IsStructuralPat {
		if p.Indexed {
			// Execute the new structural search path that directly calls Zoekt.
			// TODO use limit in indexed structural search
			return structuralSearchWithZoekt(ctx, s.Log, s.Indexed, p, sender)
		}

		zipPath, zf, err := s.getZipFile(ctx, tr, p, nil)
		if err != nil {
			return errors.Wrap(err, "failed to get archive")
		}
		defer zf.Close()
		return filteredStructuralSearch(ctx, s.Log, zipPath, zf, &p.PatternInfo, p.Repo, sender, int32(p.NumContextLines))
	}

	// Process the query before fetching from store in case it's invalid.
	rm, err := toMatchTree(p.Query, p.IsCaseSensitive)
	if err != nil {
		return badRequestError{err.Error()}
	}

	lm := toLangMatcher(&p.PatternInfo)
	pm, err := toPathMatcher(&p.PatternInfo)
	if err != nil {
		return badRequestError{err.Error()}
	}

	var paths []string
	if !s.DisableHybridSearch {
		logger := logWithTrace(ctx, s.Log).Scoped("hybrid").With(
			log.String("repo", string(p.Repo)),
			log.String("commit", string(p.Commit)),
		)

		unsearched, ok, err := s.hybrid(ctx, logger, p, sender)
		if err != nil {
			// error logging is done inside of s.hybrid so we just return
			// error here.
			return errors.Wrap(err, "hybrid search failed")
		}
		if !ok {
			logger.Debug("hybrid search is falling back to normal unindexed search")
		} else {
			// now we only need to search unsearched
			if len(unsearched) == 0 {
				// indexed search did it all
				return nil
			}
			paths = unsearched
		}
	}

	_, zf, err := s.getZipFile(ctx, tr, p, paths)
	if err != nil {
		return errors.Wrap(err, "failed to get archive")
	}
	defer zf.Close()

	return regexSearch(ctx, rm, pm, lm, zf, p.PatternMatchesContent, p.PatternMatchesPath, p.IsCaseSensitive, sender, p.NumContextLines)
}

func (s *Service) getZipFile(ctx context.Context, tr trace.Trace, p *protocol.Request, paths []string) (string, *zipFile, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, p.FetchTimeout)
	defer cancel()

	zipPath, zf, err := getZipFileWithRetry(func() (string, *zipFile, error) {
		path, err := s.Store.PrepareZip(fetchCtx, p.Repo, p.Commit, paths)
		if err != nil {
			return "", nil, err
		}
		zf, err := s.Store.zipCache.Get(path)
		return path, zf, err
	})

	if err == nil {
		nFiles := uint64(len(zf.Files))
		bytes := int64(len(zf.Data))
		tr.AddEvent("archive",
			attribute.Int64("archive.files", int64(nFiles)),
			attribute.Int64("archive.size", bytes))
		metricArchiveFiles.Observe(float64(nFiles))
		metricArchiveSize.Observe(float64(bytes))
	}

	return zipPath, zf, err
}

func validateParams(r *protocol.Request) error {
	if r.Repo == "" {
		return errors.New("Repo must be non-empty")
	}
	// Surprisingly this is the same sanity check used in the git source.
	if len(r.Commit) != 40 {
		return errors.Errorf("Commit must be resolved (Commit=%q)", r.Commit)
	}

	if p, ok := r.Query.(*protocol.PatternNode); ok {
		if p.Value == "" && r.ExcludePaths == "" && len(r.IncludePaths) == 0 &&
			len(r.IncludeLangs) == 0 && len(r.ExcludeLangs) == 0 {
			return errors.New("At least one of pattern and include/exclude patterns must be non-empty")
		}
	}
	return nil
}

const megabyte = float64(1000 * 1000)

var (
	metricRunning = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "searcher_service_running",
		Help: "Number of running search requests.",
	})
	metricArchiveSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "searcher_service_archive_size_bytes",
		Help:    "Observes the size when an archive is searched.",
		Buckets: []float64{1 * megabyte, 10 * megabyte, 100 * megabyte, 500 * megabyte, 1000 * megabyte, 5000 * megabyte},
	})
	metricArchiveFiles = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "searcher_service_archive_files",
		Help:    "Observes the number of files when an archive is searched.",
		Buckets: []float64{100, 1000, 10000, 50000, 100000},
	})
	metricRequestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "searcher_service_request_total",
		Help: "Number of returned search requests.",
	}, []string{"code"})
)

type badRequestError struct{ msg string }

func (e badRequestError) Error() string    { return e.msg }
func (e badRequestError) BadRequest() bool { return true }
