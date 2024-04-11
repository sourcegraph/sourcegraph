package jobutil

import (
	"context"
	"sync"

	"github.com/grafana/regexp"

	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/internal/search/streaming"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// NewFileHasContributorsJob creates a filter job to post-filter results for the file:has.contributor() predicate.
//
// has.contributor() predicates are grouped together by inclusivity vs. exclusivity before being passed to constructor.
// All predicates are AND'ed together i.e. result will be filtered out and not returned in result page if any predicate
// does not pass.
func NewFileHasContributorsJob(child job.Job, include, exclude []*regexp.Regexp) job.Job {
	return &fileHasContributorsJob{
		child:   child,
		include: include,
		exclude: exclude,
	}
}

type fileHasContributorsJob struct {
	child job.Job

	include []*regexp.Regexp
	exclude []*regexp.Regexp
}

func (j *fileHasContributorsJob) Run(ctx context.Context, clients job.RuntimeClients, stream streaming.Sender) (alert *search.Alert, err error) {
	_, ctx, stream, finish := job.StartSpan(ctx, stream, j)
	defer finish(alert, err)

	var (
		mu   sync.Mutex
		errs error
	)

	filteredStream := streaming.StreamFunc(func(event streaming.SearchEvent) {
		filtered := event.Results[:0]
		for _, res := range event.Results {
			// Filter out any result that is not a file
			if fm, ok := res.(*result.FileMatch); ok {
				// We send one fetch contributors request per file path.
				// We should quit early on context deadline exceeded.
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					mu.Lock()
					errs = errors.Append(errs, ctx.Err())
					mu.Unlock()
					break
				}
				fileMatchContributors, err := getFileContributors(ctx, clients.Gitserver, fm)
				if err != nil {
					mu.Lock()
					errs = errors.Append(errs, err)
					mu.Unlock()
					continue
				}

				// ensure match passes all exclusion filters
				excludeFilters := j.Filtered(fileMatchContributors, true)

				// ensure match passes all inclusion filters
				includeFilters := j.Filtered(fileMatchContributors, false)

				if !excludeFilters || !includeFilters {
					continue
				}

				filtered = append(filtered, fm)
			}
		}

		event.Results = filtered

		stream.Send(event)
	})

	alert, err = j.child.Run(ctx, clients, filteredStream)
	if err != nil {
		errs = errors.Append(errs, err)
	}
	return alert, errs
}

func (j *fileHasContributorsJob) MapChildren(fn job.MapFunc) job.Job {
	cp := *j
	cp.child = job.Map(j.child, fn)
	return &cp
}

func (j *fileHasContributorsJob) Name() string {
	return "FileHasContributorsFilterJob"
}

func (j *fileHasContributorsJob) Children() []job.Describer {
	return []job.Describer{j.child}
}

func (j *fileHasContributorsJob) Attributes(v job.Verbosity) (res []attribute.KeyValue) {
	switch v {
	case job.VerbosityMax:
		fallthrough
	case job.VerbosityBasic:
		include, exclude := j.regexpToStr()
		res = append(res,
			attribute.StringSlice("includeContributors", include),
			attribute.StringSlice("excludeContributors", exclude),
		)
	}
	return res
}

func (j *fileHasContributorsJob) regexpToStr() (includeStr, excludeStr []string) {
	for _, re := range j.include {
		includeStr = append(includeStr, re.String())
	}

	for _, re := range j.exclude {
		excludeStr = append(excludeStr, re.String())
	}

	return includeStr, excludeStr
}

func getFileContributors(ctx context.Context, client gitserver.Client, fm *result.FileMatch) ([]*gitdomain.ContributorCount, error) {
	opts := gitserver.ContributorOptions{
		Range: string(fm.CommitID),
		Path:  fm.Path,
	}
	contributors, err := client.ContributorCount(ctx, fm.Repo.ID, opts)

	if err != nil {
		return nil, err
	}

	return contributors, nil
}

// Filtered returns true if the match passes filter validation and should be returned with results page.
// Filters are AND'ed together. Filters are negation filters if excludeContributors is true.
func (j *fileHasContributorsJob) Filtered(contributors []*gitdomain.ContributorCount, excludeContributors bool) bool {
	filters := j.include
	if excludeContributors {
		filters = j.exclude
	}
	for _, filter := range filters {
		if match(contributors, filter) == excludeContributors {
			// Result needs to be filtered out
			return false
		}
	}

	// Result passed all filters
	return true
}

func match(contributors []*gitdomain.ContributorCount, regexp *regexp.Regexp) bool {
	for _, contributor := range contributors {
		if regexp.Match([]byte(contributor.Name)) || regexp.Match([]byte(contributor.Email)) {
			return true
		}
	}

	return false
}
