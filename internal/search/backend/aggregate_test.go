package backend

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/zoekt"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestFlushCollectSender(t *testing.T) {
	replicas := prefixMap{"1", "2", "3", "4", "5", "6"}
	nonemptyEndpoints := 4

	var endpoints atomicMap
	endpoints.Store(replicas)

	searcher := &HorizontalSearcher{
		Map: &endpoints,
		Dial: func(endpoint string) zoekt.Streamer {
			endpointID, _ := strconv.Atoi(endpoint)
			if endpointID > nonemptyEndpoints {
				return &FakeStreamer{}
			}

			repoList := make([]*zoekt.RepoListEntry, 3)
			results := make([]*zoekt.SearchResult, 3)

			for i := range len(results) {
				repoID := 100*endpointID + i
				repoName := strconv.Itoa(repoID)

				results[i] = &zoekt.SearchResult{
					Files: []zoekt.FileMatch{
						{
							Score:              float64(repoID),
							RepositoryPriority: float64(repoID),
							Repository:         repoName,
						},
					},
				}

				repoList[i] = &zoekt.RepoListEntry{
					Repository: zoekt.Repository{
						Name: repoName,
						ID:   uint32(repoID),
					},
				}
			}

			return &FakeStreamer{
				Results: results,
				Repos:   repoList,
			}
		},
	}
	defer searcher.Close()

	// Start up background goroutines which continuously hit the searcher
	// methods to ensure we are safe under concurrency.
	for range 5 {
		cleanup := backgroundSearch(searcher)
		defer cleanup(t)
	}

	opts := zoekt.SearchOptions{
		UseDocumentRanks: true,
		FlushWallTime:    100 * time.Millisecond,
	}

	// Collect all search results in order they were sent to stream
	var results []*zoekt.SearchResult
	err := searcher.StreamSearch(context.Background(), nil, &opts,
		ZoektStreamFunc(func(event *zoekt.SearchResult) { results = append(results, event) }))
	if err != nil {
		t.Fatal(err)
	}

	// Check the aggregated result was flushed early
	if len(results) == 0 {
		t.Fatal("no results returned from search")
	}
	if results[0].Stats.FlushReason != zoekt.FlushReasonTimerExpired {
		t.Fatalf("expected flush reason %s but got %s", zoekt.FlushReasonTimerExpired, results[0].Stats.FlushReason)
	}

	// Check that the results were streamed in the expected order
	var repos []string
	for _, r := range results {
		if r.Files != nil {
			for _, f := range r.Files {
				repos = append(repos, f.Repository)
			}
		}
	}

	expectedRepos := nonemptyEndpoints * 3
	if len(repos) != expectedRepos {
		t.Fatalf("expected %d results but got %d", expectedRepos, len(repos))
	}

	// The first results should always include one result per endpoint, ordered by score
	want := []string{"400", "300", "200", "100"}
	if !cmp.Equal(want, repos[:nonemptyEndpoints]) {
		t.Errorf("search mismatch (-want +got):\n%s", cmp.Diff(want, repos))
	}
}

func TestFlushCollectSenderMaxSize(t *testing.T) {
	replicas := prefixMap{"1", "2", "3"}

	var endpoints atomicMap
	endpoints.Store(replicas)

	searcher := &HorizontalSearcher{
		Map: &endpoints,
		Dial: func(endpoint string) zoekt.Streamer {
			repoID, _ := strconv.Atoi(endpoint)
			repoName := strconv.Itoa(repoID)

			repoList := []*zoekt.RepoListEntry{{
				Repository: zoekt.Repository{
					Name: repoName,
					ID:   uint32(repoID),
				},
			}}
			results := []*zoekt.SearchResult{{
				Files: []zoekt.FileMatch{
					{
						Score:              float64(repoID),
						RepositoryPriority: float64(repoID),
						Repository:         repoName,
					},
				},
			}}

			return &FakeStreamer{
				Results: results,
				Repos:   repoList,
			}
		},
	}
	defer searcher.Close()

	// Set the maximum bytes size to a low number, so that we collect
	// some results but eventually hit this limit
	cfg := conf.Get()
	maxSizeBytes := 512
	cfg.ExperimentalFeatures.Ranking = &schema.Ranking{
		MaxQueueSizeBytes: &maxSizeBytes,
	}
	conf.Mock(cfg)

	// Always reset the configuration so that it doesn't interfere with other tests
	defer func() {
		cfg.ExperimentalFeatures.Ranking = nil
		conf.Mock(cfg)
	}()

	opts := zoekt.SearchOptions{
		UseDocumentRanks: true,
		FlushWallTime:    100 * time.Millisecond,
	}

	// Collect all search results in order they were sent to stream
	var results []*zoekt.SearchResult
	err := searcher.StreamSearch(context.Background(), nil, &opts,
		ZoektStreamFunc(func(event *zoekt.SearchResult) { results = append(results, event) }))
	if err != nil {
		t.Fatal(err)
	}

	// Check the aggregated result was flushed early
	if len(results) == 0 {
		t.Fatal("no results returned from search")
	}

	if results[0].Stats.FlushReason != zoekt.FlushReasonMaxSize {
		t.Fatalf("expected flush reason %s but got %s", zoekt.FlushReasonMaxSize, results[0].Stats.FlushReason)
	}

	// Check that all search results are streamed out
	var repos []string
	for _, r := range results {
		if r.Files != nil {
			for _, f := range r.Files {
				repos = append(repos, f.Repository)
			}
		}
	}

	sort.Strings(repos)
	want := []string{"1", "2", "3"}
	if !cmp.Equal(want, repos) {
		t.Errorf("search mismatch (-want +got):\n%s", cmp.Diff(want, repos))
	}
}
