package jobutil

import (
	"fmt"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/search/commit"
	"github.com/sourcegraph/sourcegraph/internal/search/filter"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/repos"
	"github.com/sourcegraph/sourcegraph/internal/search/run"
	"github.com/sourcegraph/sourcegraph/internal/search/searcher"
	"github.com/sourcegraph/sourcegraph/internal/search/structural"
	"github.com/sourcegraph/sourcegraph/internal/search/symbol"
	"github.com/sourcegraph/sourcegraph/internal/search/zoekt"
)

type Mapper struct {
	MapJob func(job job.Job) job.Job

	// Search Jobs (leaf nodes)
	MapZoektRepoSubsetSearchJob    func(*zoekt.ZoektRepoSubsetSearchJob) *zoekt.ZoektRepoSubsetSearchJob
	MapZoektSymbolSearchJob        func(*zoekt.ZoektSymbolSearchJob) *zoekt.ZoektSymbolSearchJob
	MapSearcherJob                 func(*searcher.SearcherJob) *searcher.SearcherJob
	MapSymbolSearcherJob           func(*searcher.SymbolSearcherJob) *searcher.SymbolSearcherJob
	MapRepoSearchJob               func(*run.RepoSearchJob) *run.RepoSearchJob
	MapRepoUniverseTextSearchJob   func(*zoekt.ZoektGlobalSearchJob) *zoekt.ZoektGlobalSearchJob
	MapStructuralSearchJob         func(*structural.StructuralSearchJob) *structural.StructuralSearchJob
	MapCommitSearchJob             func(*commit.CommitSearchJob) *commit.CommitSearchJob
	MapReferenceSearcherJob        func(*symbol.ReferenceSearcherJob) *symbol.ReferenceSearcherJob
	MapRepoUniverseSymbolSearchJob func(*symbol.RepoUniverseSymbolSearchJob) *symbol.RepoUniverseSymbolSearchJob
	MapComputeExcludedReposJob     func(*repos.ComputeExcludedReposJob) *repos.ComputeExcludedReposJob

	// Repo pager Job (pre-step for some Search Jobs)
	MapRepoPagerJob func(*repoPagerJob) *repoPagerJob

	// Expression Jobs
	MapAndJob func(children []job.Job) []job.Job
	MapOrJob  func(children []job.Job) []job.Job

	// Combinator Jobs
	MapParallelJob   func(children []job.Job) []job.Job
	MapSequentialJob func(children []job.Job) []job.Job
	MapTimeoutJob    func(timeout time.Duration, child job.Job) (time.Duration, job.Job)
	MapLimitJob      func(limit int, child job.Job) (int, job.Job)
	MapSelectJob     func(path filter.SelectPath, child job.Job) (filter.SelectPath, job.Job)
	MapAlertJob      func(inputs *run.SearchInputs, child job.Job) (*run.SearchInputs, job.Job)

	// Filter Jobs
	MapSubRepoPermsFilterJob func(child job.Job) job.Job
}

func (m *Mapper) Map(j job.Job) job.Job {
	if j == nil {
		return nil
	}

	if m.MapJob != nil {
		j = m.MapJob(j)
	}

	switch j := j.(type) {
	case *zoekt.ZoektRepoSubsetSearchJob:
		if m.MapZoektRepoSubsetSearchJob != nil {
			j = m.MapZoektRepoSubsetSearchJob(j)
		}
		return j

	case *zoekt.ZoektSymbolSearchJob:
		if m.MapZoektSymbolSearchJob != nil {
			j = m.MapZoektSymbolSearchJob(j)
		}
		return j

	case *searcher.SearcherJob:
		if m.MapSearcherJob != nil {
			j = m.MapSearcherJob(j)
		}
		return j

	case *searcher.SymbolSearcherJob:
		if m.MapSymbolSearcherJob != nil {
			j = m.MapSymbolSearcherJob(j)
		}
		return j

	case *run.RepoSearchJob:
		if m.MapRepoSearchJob != nil {
			j = m.MapRepoSearchJob(j)
		}
		return j

	case *zoekt.ZoektGlobalSearchJob:
		if m.MapRepoUniverseTextSearchJob != nil {
			j = m.MapRepoUniverseTextSearchJob(j)
		}
		return j

	case *structural.StructuralSearchJob:
		if m.MapStructuralSearchJob != nil {
			j = m.MapStructuralSearchJob(j)
		}
		return j

	case *commit.CommitSearchJob:
		if m.MapCommitSearchJob != nil {
			j = m.MapCommitSearchJob(j)
		}
		return j

	case *symbol.ReferenceSearcherJob:
		if m.MapReferenceSearcherJob != nil {
			j = m.MapReferenceSearcherJob(j)
		}
		return j

	case *symbol.RepoUniverseSymbolSearchJob:
		if m.MapRepoUniverseSymbolSearchJob != nil {
			j = m.MapRepoUniverseSymbolSearchJob(j)
		}
		return j

	case *repos.ComputeExcludedReposJob:
		if m.MapComputeExcludedReposJob != nil {
			j = m.MapComputeExcludedReposJob(j)
		}
		return j

	case *repoPagerJob:
		child := m.Map(j.child)
		j.child = child
		if m.MapRepoPagerJob != nil {
			j = m.MapRepoPagerJob(j)
		}
		return j

	case *AndJob:
		children := make([]job.Job, 0, len(j.children))
		for _, child := range j.children {
			children = append(children, m.Map(child))
		}
		if m.MapAndJob != nil {
			children = m.MapAndJob(children)
		}
		return NewAndJob(children...)

	case *OrJob:
		children := make([]job.Job, 0, len(j.children))
		for _, child := range j.children {
			children = append(children, m.Map(child))
		}
		if m.MapOrJob != nil {
			children = m.MapOrJob(children)
		}
		return NewOrJob(children...)

	case *ParallelJob:
		children := make([]job.Job, 0, len(j.children))
		for _, child := range j.children {
			children = append(children, m.Map(child))
		}
		if m.MapParallelJob != nil {
			children = m.MapParallelJob(children)
		}
		return NewParallelJob(children...)

	case *SequentialJob:
		children := make([]job.Job, 0, len(j.children))
		for _, child := range j.children {
			children = append(children, m.Map(child))
		}
		if m.MapSequentialJob != nil {
			children = m.MapSequentialJob(children)
		}
		return NewSequentialJob(children...)

	case *TimeoutJob:
		child := m.Map(j.child)
		timeout := j.timeout
		if m.MapTimeoutJob != nil {
			timeout, child = m.MapTimeoutJob(timeout, child)
		}
		return NewTimeoutJob(timeout, child)

	case *LimitJob:
		child := m.Map(j.child)
		limit := j.limit
		if m.MapLimitJob != nil {
			limit, child = m.MapLimitJob(limit, child)
		}
		return NewLimitJob(limit, child)

	case *selectJob:
		child := m.Map(j.child)
		filter := j.path
		if m.MapLimitJob != nil {
			filter, child = m.MapSelectJob(filter, child)
		}
		return NewSelectJob(filter, child)

	case *alertJob:
		child := m.Map(j.child)
		inputs := j.inputs
		if m.MapLimitJob != nil {
			inputs, child = m.MapAlertJob(inputs, child)
		}
		return NewAlertJob(inputs, child)

	case *subRepoPermsFilterJob:
		child := m.Map(j.child)
		if m.MapSubRepoPermsFilterJob != nil {
			child = m.MapSubRepoPermsFilterJob(child)
		}
		return NewFilterJob(child)

	case *NoopJob:
		return j

	default:
		panic(fmt.Sprintf("unsupported job %T for job.Mapper", j))
	}
}

func MapAtom(j job.Job, f func(job.Job) job.Job) job.Job {
	mapper := Mapper{
		MapJob: func(currentJob job.Job) job.Job {
			switch typedJob := currentJob.(type) {
			case
				*zoekt.ZoektRepoSubsetSearchJob,
				*zoekt.ZoektSymbolSearchJob,
				*searcher.SearcherJob,
				*searcher.SymbolSearcherJob,
				*run.RepoSearchJob,
				*structural.StructuralSearchJob,
				*commit.CommitSearchJob,
				*symbol.ReferenceSearcherJob,
				*symbol.RepoUniverseSymbolSearchJob,
				*repos.ComputeExcludedReposJob,
				*NoopJob:
				return f(typedJob)
			default:
				return currentJob
			}
		},
	}
	return mapper.Map(j)
}
