package jobutil

import (
	"github.com/sourcegraph/sourcegraph/internal/search/commit"
	"github.com/sourcegraph/sourcegraph/internal/search/job"
	"github.com/sourcegraph/sourcegraph/internal/search/repos"
	"github.com/sourcegraph/sourcegraph/internal/search/run"
	"github.com/sourcegraph/sourcegraph/internal/search/searcher"
	"github.com/sourcegraph/sourcegraph/internal/search/structural"
	"github.com/sourcegraph/sourcegraph/internal/search/symbol"
	"github.com/sourcegraph/sourcegraph/internal/search/zoekt"
)

var allJobs = []job.Job{
	&zoekt.ZoektRepoSubsetSearchJob{},
	&zoekt.ZoektSymbolSearchJob{},
	&searcher.SearcherJob{},
	&searcher.SymbolSearcherJob{},
	&run.RepoSearchJob{},
	&zoekt.ZoektGlobalSearchJob{},
	&structural.StructuralSearchJob{},
	&commit.CommitSearchJob{},
	&symbol.ReferenceSearcherJob{},
	&symbol.RepoUniverseSymbolSearchJob{},
	&repos.ComputeExcludedReposJob{},
	&NoopJob{},

	&repoPagerJob{},

	&AndJob{},
	&OrJob{},
	&ParallelJob{},
	&SequentialJob{},

	&TimeoutJob{},
	&LimitJob{},
	&subRepoPermsFilterJob{},
	&selectJob{},
	&alertJob{},
}
