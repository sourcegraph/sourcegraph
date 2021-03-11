// Package search is a service which exposes an API to text search a repo at
// a specific commit.
//
// Architecture Notes:
// * Archive is fetched from gitserver
// * Simple HTTP API exposed
// * Currently no concept of authorization
// * On disk cache of fetched archives to reduce load on gitserver
// * Run search on archive. Rely on OS file buffers
// * Simple to scale up since stateless
// * Use ingress with affinity to increase local cache hit ratio
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus/promauto"

	nettrace "golang.org/x/net/trace"

	"github.com/sourcegraph/sourcegraph/cmd/searcher/protocol"
	"github.com/sourcegraph/sourcegraph/internal/store"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"

	"github.com/pkg/errors"

	"github.com/gorilla/schema"
	"github.com/opentracing/opentracing-go/ext"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// maxFileMatches is the limit on number of matching files we return.
	maxFileMatches = 1000

	// maxLineMatches is the limit on number of matches to return in a
	// file.
	maxLineMatches = 100

	// numWorkers is how many concurrent readerGreps run in the case of
	// regexSearch, and the number of parallel workers in the case of
	// structuralSearch.
	numWorkers = 8
)

// Service is the search service. It is an http.Handler.
type Service struct {
	Store *store.Store
	Log   log15.Logger
}

var decoder = schema.NewDecoder()

func init() {
	decoder.IgnoreUnknownKeys(true)
}

// ServeHTTP handles HTTP based search requests
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	running.Inc()
	defer running.Dec()

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	var p protocol.Request
	err = decoder.Decode(&p, r.Form)
	if err != nil {
		http.Error(w, "failed to decode form: "+err.Error(), http.StatusBadRequest)
		return
	}
	if p.Deadline != "" {
		var deadline time.Time
		if err := deadline.UnmarshalText([]byte(p.Deadline)); err != nil {
			http.Error(w, "invalid deadline: "+err.Error(), http.StatusBadRequest)
			return
		}
		dctx, cancel := context.WithDeadline(ctx, deadline)
		defer cancel()
		ctx = dctx
	}
	if !p.PatternMatchesContent && !p.PatternMatchesPath {
		// BACKCOMPAT: Old frontends send neither of these fields, but we still want to
		// search file content in that case.
		p.PatternMatchesContent = true
	}
	if err = validateParams(&p); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	matches, limitHit, deadlineHit, err := s.search(ctx, &p)
	if err != nil {
		code := http.StatusInternalServerError
		if isBadRequest(err) || ctx.Err() == context.Canceled {
			code = http.StatusBadRequest
		} else if isTemporary(err) {
			code = http.StatusServiceUnavailable
		} else {
			log.Printf("internal error serving %#+v: %s", p, err)
		}
		http.Error(w, err.Error(), code)
		return
	}
	if matches == nil {
		// Return an empty list
		matches = make([]protocol.FileMatch, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	resp := protocol.Response{
		Matches:     matches,
		LimitHit:    limitHit,
		DeadlineHit: deadlineHit,
	}
	// The only reasonable error is the client going away now since we know we
	// can encode resp. This happens relatively often due to our
	// graphqlbackend regularly cancelling in-flight requests. We can't send
	// an error response, so we just ignore.
	_ = json.NewEncoder(w).Encode(&resp)
}

const maxFileMatchLimit = 100

func (s *Service) search(ctx context.Context, p *protocol.Request) (matches []protocol.FileMatch, limitHit, deadlineHit bool, err error) {
	tr := nettrace.New("search", fmt.Sprintf("%s@%s", p.Repo, p.Commit))
	tr.LazyPrintf("%s", p.Pattern)

	span, ctx := ot.StartSpanFromContext(ctx, "Search")
	ext.Component.Set(span, "service")
	span.SetTag("repo", p.Repo)
	span.SetTag("url", p.URL)
	span.SetTag("commit", p.Commit)
	span.SetTag("pattern", p.Pattern)
	span.SetTag("isRegExp", strconv.FormatBool(p.IsRegExp))
	span.SetTag("isStructuralPat", strconv.FormatBool(p.IsStructuralPat))
	span.SetTag("languages", p.Languages)
	span.SetTag("isWordMatch", strconv.FormatBool(p.IsWordMatch))
	span.SetTag("isCaseSensitive", strconv.FormatBool(p.IsCaseSensitive))
	span.SetTag("pathPatternsAreRegExps", strconv.FormatBool(p.PathPatternsAreRegExps))
	span.SetTag("pathPatternsAreCaseSensitive", strconv.FormatBool(p.PathPatternsAreCaseSensitive))
	span.SetTag("fileMatchLimit", p.FileMatchLimit)
	span.SetTag("patternMatchesContent", p.PatternMatchesContent)
	span.SetTag("patternMatchesPath", p.PatternMatchesPath)
	span.SetTag("deadline", p.Deadline)
	span.SetTag("indexerEndpoints", p.IndexerEndpoints)
	span.SetTag("select", p.Select)
	defer func(start time.Time) {
		code := "200"
		// We often have canceled and timed out requests. We do not want to
		// record them as errors to avoid noise
		if ctx.Err() == context.Canceled {
			code = "canceled"
			span.SetTag("err", err)
		} else if ctx.Err() == context.DeadlineExceeded {
			code = "timedout"
			span.SetTag("err", err)
			deadlineHit = true
			err = nil // error is fully described by deadlineHit=true return value
		} else if err != nil {
			tr.LazyPrintf("error: %v", err)
			tr.SetError()
			ext.Error.Set(span, true)
			span.SetTag("err", err.Error())
			if isBadRequest(err) {
				code = "400"
			} else if isTemporary(err) {
				code = "503"
			} else {
				code = "500"
			}
		}
		tr.LazyPrintf("code=%s matches=%d limitHit=%v deadlineHit=%v", code, len(matches), limitHit, deadlineHit)
		tr.Finish()
		requestTotal.WithLabelValues(code).Inc()
		span.LogFields(otlog.Int("matches.len", len(matches)))
		span.SetTag("limitHit", limitHit)
		span.SetTag("deadlineHit", deadlineHit)
		span.Finish()
		if s.Log != nil {
			s.Log.Debug("search request", "repo", p.Repo, "commit", p.Commit, "pattern", p.Pattern, "isRegExp", p.IsRegExp, "isStructuralPat", p.IsStructuralPat, "languages", p.Languages, "isWordMatch", p.IsWordMatch, "isCaseSensitive", p.IsCaseSensitive, "patternMatchesContent", p.PatternMatchesContent, "patternMatchesPath", p.PatternMatchesPath, "matches", len(matches), "code", code, "duration", time.Since(start), "indexerEndpoints", p.IndexerEndpoints, "err", err)
		}
	}(time.Now())

	if p.IsStructuralPat && p.Indexed && p.CombyRule != `where "backcompat" == "backcompat"` {
		// Execute the new structural search path that directly calls Zoekt.
		return structuralSearchWithZoekt(ctx, p)
	}

	// Compile pattern before fetching from store incase it is bad.
	var rg *readerGrep
	if !p.IsStructuralPat {
		rg, err = compile(&p.PatternInfo)
		if err != nil {
			return nil, false, false, badRequestError{err.Error()}
		}
	}

	if p.FetchTimeout == "" {
		p.FetchTimeout = "500ms"
	}
	fetchTimeout, err := time.ParseDuration(p.FetchTimeout)
	if err != nil {
		return nil, false, false, err
	}
	prepareCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	getZf := func() (string, *store.ZipFile, error) {
		path, err := s.Store.PrepareZip(prepareCtx, p.Repo, p.Commit)
		if err != nil {
			return "", nil, err
		}
		zf, err := s.Store.ZipCache.Get(path)
		return path, zf, err
	}

	zipPath, zf, err := store.GetZipFileWithRetry(getZf)
	if err != nil {
		return nil, false, false, errors.Wrap(err, "failed to get archive")
	}
	defer zf.Close()

	nFiles := uint64(len(zf.Files))
	bytes := int64(len(zf.Data))
	tr.LazyPrintf("files=%d bytes=%d", nFiles, bytes)
	span.LogFields(
		otlog.Uint64("archive.files", nFiles),
		otlog.Int64("archive.size", bytes))
	archiveFiles.Observe(float64(nFiles))
	archiveSize.Observe(float64(bytes))

	if p.IsStructuralPat && p.CombyRule == `where "backcompat" == "backcompat"` {
		matches, limitHit, err = structuralSearch(ctx, zipPath, p.Pattern, p.CombyRule, "", p.Languages, p.IncludePatterns, p.Repo)
	} else if p.IsStructuralPat {
		matches, limitHit, err = filteredStructuralSearch(ctx, zipPath, zf, &p.PatternInfo, p.Repo)
	} else {
		matches, limitHit, err = regexSearch(ctx, rg, zf, p.FileMatchLimit, p.PatternMatchesContent, p.PatternMatchesPath, p.IsNegated)
	}
	return matches, limitHit, false, err
}

func validateParams(p *protocol.Request) error {
	if p.Repo == "" {
		return errors.New("Repo must be non-empty")
	}
	// Surprisingly this is the same sanity check used in the git source.
	if len(p.Commit) != 40 {
		return errors.Errorf("Commit must be resolved (Commit=%q)", p.Commit)
	}
	if p.Pattern == "" && p.ExcludePattern == "" && len(p.IncludePatterns) == 0 {
		return errors.New("At least one of pattern and include/exclude pattners must be non-empty")
	}
	if p.IsNegated && p.IsStructuralPat {
		return errors.New("Negated patterns are not supported for structural searches")
	}
	return nil
}

const megabyte = float64(1000 * 1000)

var (
	running = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "searcher_service_running",
		Help: "Number of running search requests.",
	})
	archiveSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "searcher_service_archive_size_bytes",
		Help:    "Observes the size when an archive is searched.",
		Buckets: []float64{1 * megabyte, 10 * megabyte, 100 * megabyte, 500 * megabyte, 1000 * megabyte, 5000 * megabyte},
	})
	archiveFiles = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "searcher_service_archive_files",
		Help:    "Observes the number of files when an archive is searched.",
		Buckets: []float64{100, 1000, 10000, 50000, 100000},
	})
	requestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "searcher_service_request_total",
		Help: "Number of returned search requests.",
	}, []string{"code"})
)

type badRequestError struct{ msg string }

func (e badRequestError) Error() string    { return e.msg }
func (e badRequestError) BadRequest() bool { return true }

func isBadRequest(err error) bool {
	e, ok := errors.Cause(err).(interface {
		BadRequest() bool
	})
	return ok && e.BadRequest()
}

func isTemporary(err error) bool {
	e, ok := errors.Cause(err).(interface {
		Temporary() bool
	})
	return ok && e.Temporary()
}
