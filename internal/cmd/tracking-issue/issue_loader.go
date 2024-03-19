package main

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/machinebox/graphql"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	costPerSearch         = 30
	maxCostPerRequest     = 1000
	queriesPerLoadRequest = 10
)

// IssueLoader efficiently fetches issues and pull request that match a given set
// of queries.
type IssueLoader struct {
	queries   []string
	fragments []string
	args      [][]string
	cursors   []string
	done      []bool
}

// LoadIssues will load all issues and pull requests matching the configured queries by making
// multiple queries in parallel and merging and deduplicating the result. Tracking issues are
// filtered out of the resulting issues list.
func LoadIssues(ctx context.Context, cli *graphql.Client, queries []string) (issues []*Issue, pullRequests []*PullRequest, err error) {
	chunks := chunkQueries(queries)
	ch := make(chan []string, len(chunks))
	for _, chunk := range chunks {
		ch <- chunk
	}
	close(ch)

	var wg sync.WaitGroup
	issuesCh := make(chan []*Issue, len(chunks))
	pullRequestsCh := make(chan []*PullRequest, len(chunks))
	errs := make(chan error, len(chunks))

	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for chunk := range ch {
				issues, pullRequests, err := loadIssues(ctx, cli, chunk)
				if err != nil {
					errs <- errors.Wrap(err, fmt.Sprintf("loadIssues(%s)", strings.Join(queries, ", ")))
				} else {
					issuesCh <- issues
					pullRequestsCh <- pullRequests
				}
			}
		}()
	}

	wg.Wait()
	close(errs)
	close(issuesCh)
	close(pullRequestsCh)

	for chunk := range issuesCh {
		issues = append(issues, chunk...)
	}
	for chunk := range pullRequestsCh {
		pullRequests = append(pullRequests, chunk...)
	}

	for e := range errs {
		if err == nil {
			err = e
		} else {
			err = errors.Append(err, e)
		}
	}

	return deduplicateIssues(issues), deduplicatePullRequests(pullRequests), err
}

// chunkQueries returns the given queries spread across a number of slices. Each
// slice should contain at most queriesPerLoadRequest elements.
func chunkQueries(queries []string) (chunks [][]string) {
	for i := 0; i < len(queries); i += queriesPerLoadRequest {
		if n := i + queriesPerLoadRequest; n < len(queries) {
			chunks = append(chunks, queries[i:n])
		} else {
			chunks = append(chunks, queries[i:])
		}
	}

	return chunks
}

// loadIssues will load all issues and pull requests matching the configured queries.
// Tracking issues are filtered out of the resulting issues list.
func loadIssues(ctx context.Context, cli *graphql.Client, queries []string) (issues []*Issue, pullRequests []*PullRequest, _ error) {
	return NewIssueLoader(queries).Load(ctx, cli)
}

// NewIssueLoader creates a new IssueLoader with the given queries.
func NewIssueLoader(queries []string) *IssueLoader {
	fragments, args := makeFragmentArgs(len(queries))

	return &IssueLoader{
		queries:   queries,
		fragments: fragments,
		args:      args,
		cursors:   make([]string, len(queries)),
		done:      make([]bool, len(queries)),
	}
}

// Load will load all issues and pull requests matching the configured queries.
// Tracking issues are filtered out of the resulting issues list.
func (l *IssueLoader) Load(ctx context.Context, cli *graphql.Client) (issues []*Issue, pullRequests []*PullRequest, _ error) {
	for {
		r, ok := l.makeNextRequest()
		if !ok {
			break
		}

		pageIssues, pagePullRequests, err := l.performRequest(ctx, cli, r)
		if err != nil {
			return nil, nil, err
		}

		issues = append(issues, pageIssues...)
		pullRequests = append(pullRequests, pagePullRequests...)
	}

	return issues, pullRequests, nil
}

// makeNextRequest will construct a new request based on the given cursor values.
// If no request should be performed, this method will return a false-valued flag.
func (l *IssueLoader) makeNextRequest() (*graphql.Request, bool) {
	var args []string
	var fragments []string
	vars := map[string]any{}

	cost := 0
	for i := range l.queries {
		cost += costPerSearch

		if l.done[i] || cost > maxCostPerRequest {
			continue
		}

		args = append(args, l.args[i]...)
		fragments = append(fragments, l.fragments[i])
		vars[fmt.Sprintf("query%d", i)] = l.queries[i]
		vars[fmt.Sprintf("count%d", i)] = costPerSearch
		if l.cursors[i] != "" {
			vars[fmt.Sprintf("cursor%d", i)] = l.cursors[i]
		}
	}

	if len(fragments) == 0 {
		return nil, false
	}

	r := graphql.NewRequest(fmt.Sprintf(`query(%s) { %s }`, strings.Join(args, ", "), strings.Join(fragments, "\n")))
	for k, v := range vars {
		r.Var(k, v)
	}

	return r, true
}

// performRequest will perform the given request and return the deserialized
// list of issues and pull requests.
func (l *IssueLoader) performRequest(ctx context.Context, cli *graphql.Client, r *graphql.Request) (issues []*Issue, pullRequests []*PullRequest, _ error) {
	var payload map[string]SearchResult
	if err := cli.Run(ctx, r, &payload); err != nil {
		return nil, nil, err
	}

	for name, result := range payload {
		// Note: the search fragment aliases have the form `search123`
		index, err := strconv.Atoi(name[6:])
		if err != nil {
			return nil, nil, err
		}

		searchIssues, searchPullRequests := unmarshalSearchNodes(result.Nodes)
		issues = append(issues, searchIssues...)
		pullRequests = append(pullRequests, searchPullRequests...)

		if len(result.Nodes) > 0 && result.PageInfo.HasNextPage {
			l.cursors[index] = result.PageInfo.EndCursor
		} else {
			l.done[index] = true
		}
	}

	return issues, pullRequests, nil
}

// makeFragmentArgs makes `n` named GraphQL fragment and an associated set of variables.
// This is used to later construct a GraphQL request with a subset of these queries.
func makeFragmentArgs(n int) (fragments []string, args [][]string) {
	for i := range n {
		fragments = append(fragments, makeSearchQuery(fmt.Sprintf("%d", i)))

		args = append(args, []string{
			fmt.Sprintf("$query%d: String!", i),
			fmt.Sprintf("$count%d: Int!", i),
			fmt.Sprintf("$cursor%d: String", i),
		})
	}

	return fragments, args
}
