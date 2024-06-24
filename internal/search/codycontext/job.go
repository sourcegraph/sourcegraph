package codycontext

import (
	"context"
	"strings"
	"sync"

	"github.com/sourcegraph/conc/pool"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/query"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const DefaultCodeResultsCount = 12
const DefaultTextResultsCount = 3
const SymbolResultsCount = 3

// NewSearchJob creates a new job for Cody context searches. It maps the query into a keyword query by breaking
// it into terms, applying light stemming, then combining the terms through an OR operator.
//
// When the job is run, it executes two child jobs: one for code and one for text. Each search is limited to a small
// number of file matches. The match limits can be adjusted by passing `codyCodeCount` and `codyTextCount` parameters
// (which are not user-facing and only intended for internal use).
//
// The job blocks until all results are collected, then streams them back to the caller. This gives flexibility to
// combine and reorder the results in any way.
func NewSearchJob(plan query.Plan, inputs *search.Inputs, newJob func(query.Basic) (job.Job, error)) (job.Job, error) {
	if len(plan) > 1 {
		return nil, errors.New("The 'codycontext' patterntype does not support multiple clauses")
	}

	codeCount, textCount := getResultLimits(inputs)
	fileMatcher := getFileMatcher(inputs)
	basicQuery := plan[0].ToParseTree()

	q, err := parseQuery(query.StringHuman(basicQuery))

	if err != nil {
		return nil, err
	}

	params := q.keywordQuery.Parameters
	patterns := q.patterns

	// If there are no patterns left, this search was entirely composed of stopwords, so we return no results.
	// ⚠️ We must return a no-op job instead of nil, since the job framework assumes all jobs are non-nil.
	if len(patterns) == 0 {
		return newNoopJob(), nil
	}

	var symbolJob job.Job = newNoopJob()
	if len(q.symbols) > 0 {
		symbolQuery := q.symbolQuery.MapParameters(append(params,
			query.Parameter{Field: query.FieldType, Value: "symbol"},
			query.Parameter{Field: query.FieldFile, Value: textFileFilter, Negated: true}))
		symbolJob, err = newJob(symbolQuery)
		if err != nil {
			return nil, err
		}
	}

	codeQuery := q.keywordQuery.MapParameters(append(params,
		query.Parameter{Field: query.FieldType, Value: "path"},
		query.Parameter{Field: query.FieldType, Value: "file"},
		query.Parameter{Field: query.FieldFile, Value: textFileFilter, Negated: true}))
	codeJob, err := newJob(codeQuery)
	if err != nil {
		return nil, err
	}

	textQuery := q.keywordQuery.MapParameters(append(params,
		query.Parameter{Field: query.FieldType, Value: "path"},
		query.Parameter{Field: query.FieldType, Value: "file"},
		query.Parameter{Field: query.FieldFile, Value: textFileFilter}))
	textJob, err := newJob(textQuery)
	if err != nil {
		return nil, err
	}

	return &searchJob{symbolJob, codeJob, codeCount, textJob, textCount, fileMatcher, patterns}, nil
}

type codyFileMatcher = func(id api.RepoID, s string) bool

func getFileMatcher(inputs *search.Inputs) codyFileMatcher {
	if inputs.Features == nil || inputs.Features.CodyFileMatcher == nil {
		return func(id api.RepoID, s string) bool {
			return true
		}
	}
	return inputs.Features.CodyFileMatcher
}

func getResultLimits(inputs *search.Inputs) (codeCount, textCount int) {
	codeCount = DefaultCodeResultsCount
	textCount = DefaultTextResultsCount
	if inputs.Features == nil {
		return
	}

	if inputs.Features.CodyContextCodeCount > 0 {
		codeCount = inputs.Features.CodyContextCodeCount
	}
	if inputs.Features.CodyContextTextCount > 0 {
		textCount = inputs.Features.CodyContextTextCount
	}
	return
}

var textFileExtensions = map[string]struct{}{
	"adoc":      {},
	"asciidoc":  {},
	"creole":    {},
	"md":        {},
	"markdown":  {},
	"mediawiki": {},
	"muse":      {},
	"org":       {},
	"pod":       {},
	"pod6":      {},
	"rest":      {},
	"rdoc":      {},
	"rst":       {},
	"rmd":       {},
	"textile":   {},
	"txt":       {},
	"wiki":      {},
	"wikitext":  {},
}

var textFileFilter = func() string {
	var extensions []string
	for extension := range textFileExtensions {
		extensions = append(extensions, extension)
	}
	return `(` + strings.Join(extensions, "|") + `)$`
}()

type searchJob struct {
	symbolJob job.Job
	codeJob   job.Job
	codeCount int

	textJob   job.Job
	textCount int

	fileMatcher codyFileMatcher
	patterns    []string
}

func (j *searchJob) Run(ctx context.Context, clients job.RuntimeClients, stream streaming.Sender) (alert *search.Alert, err error) {
	_, ctx, stream, finish := job.StartSpan(ctx, stream, j)
	defer func() { finish(alert, err) }()

	symbolGroup := pool.NewWithResults[response]()
	symbolGroup.Go(func() response {
		count := min(SymbolResultsCount, j.codeCount)
		return j.doSearch(ctx, clients, j.symbolJob, count)
	})

	codeGroup := pool.NewWithResults[response]()
	codeGroup.Go(func() response {
		return j.doSearch(ctx, clients, j.codeJob, j.codeCount)
	})

	textGroup := pool.NewWithResults[response]()
	textGroup.Go(func() response {
		return j.doSearch(ctx, clients, j.textJob, j.textCount)
	})

	textResponse := textGroup.Wait()[0]
	symbolResponse := symbolGroup.Wait()[0]
	codeResponse := codeGroup.Wait()[0]

	j.limitCodeResponse(symbolResponse, &codeResponse)

	// For consistency, always return text results, symbol results, then general code results. This is not
	// critical for response quality, but just makes testing easier.
	for _, r := range []response{textResponse, symbolResponse, codeResponse} {
		stream.Send(streaming.SearchEvent{
			Results: r.matches,
		})
		alert = search.MaxPriorityAlert(alert, r.alert)
		if r.err != nil {
			err = errors.Append(err, r.err)
		}
	}
	return alert, err
}

// limitCodeResponse deduplicates code results that are already included in the symbol results,
// and applies the result limit. Symbol results count towards the total code results limit.
func (j *searchJob) limitCodeResponse(symbolResponse response, codeResponse *response) {
	symbolKeys := make(map[result.Key]struct{})
	for _, match := range symbolResponse.matches {
		symbolKeys[match.Key()] = struct{}{}
	}

	limit := min(len(codeResponse.matches), j.codeCount-len(symbolResponse.matches))
	codeMatches := codeResponse.matches[:0]
	for _, match := range codeResponse.matches {
		if len(codeMatches) >= limit {
			break
		}

		if _, ok := symbolKeys[match.Key()]; !ok {
			codeMatches = append(codeMatches, match)
		}
	}
	codeResponse.matches = codeMatches
}

type response struct {
	matches result.Matches
	err     error
	alert   *search.Alert
}

func (j *searchJob) doSearch(ctx context.Context, clients job.RuntimeClients, job job.Job, limit int) response {
	var (
		mu        sync.Mutex
		collected result.Matches
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream := streaming.StreamFunc(func(e streaming.SearchEvent) {
		mu.Lock()
		defer mu.Unlock()

		if len(collected) >= limit {
			return
		}

		for _, res := range e.Results {
			if fm, ok := res.(*result.FileMatch); ok {
				if !j.fileMatcher(fm.Repo.ID, fm.Path) {
					continue
				}

				collected = append(collected, fm)
				if len(collected) >= limit {
					cancel()
					return
				}
			}
		}
	})

	alert, err := job.Run(ctx, clients, stream)
	return response{collected, err, alert}
}

func (j *searchJob) Name() string {
	return "CodyContextSearchJob"
}

func (j *searchJob) Attributes(v job.Verbosity) (res []attribute.KeyValue) {
	switch v {
	case job.VerbosityMax:
		fallthrough
	case job.VerbosityBasic:
		res = append(res,
			attribute.StringSlice("patterns", j.patterns),
			attribute.Int("codeCount", j.codeCount),
			attribute.Int("textCount", j.textCount),
		)
	}
	return res
}

func (j *searchJob) Children() []job.Describer {
	return nil
}

func (j *searchJob) MapChildren(job.MapFunc) job.Job {
	return j
}

func newNoopJob() *noopJob {
	return &noopJob{}
}

type noopJob struct{}

func (e *noopJob) Run(context.Context, job.RuntimeClients, streaming.Sender) (*search.Alert, error) {
	return nil, nil
}

func (e *noopJob) Name() string                                  { return "NoopJob" }
func (e *noopJob) Attributes(job.Verbosity) []attribute.KeyValue { return nil }
func (e *noopJob) Children() []job.Describer                     { return nil }
func (e *noopJob) MapChildren(job.MapFunc) job.Job               { return e }
