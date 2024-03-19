package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/machinebox/graphql"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// updateIssues will update the body of each of the given issues. Each issue update is performed
// as a separate GraphQL query over multiple goroutines (one per logical processor). The returned
// error value, if any, will be an aggregate of errors over all requests.
func updateIssues(ctx context.Context, cli *graphql.Client, issues []*Issue) (err error) {
	ch := make(chan *Issue, len(issues))
	for _, issue := range issues {
		ch <- issue
	}
	close(ch)

	var wg sync.WaitGroup
	errs := make(chan error, len(issues))

	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for issue := range ch {
				if err := updateIssue(ctx, cli, issue); err != nil {
					errs <- errors.Wrap(err, fmt.Sprintf("updateIssue(%q)", issue.Title))
				}
			}
		}()
	}

	wg.Wait()
	close(errs)

	for e := range errs {
		if err == nil {
			err = e
		} else {
			err = errors.Append(err, e)
		}
	}

	return err
}

func updateIssue(ctx context.Context, cli *graphql.Client, issue *Issue) (err error) {
	r := graphql.NewRequest(`
		mutation($issueInput: UpdateIssueInput!) {
			issue: updateIssue(input: $issueInput) {
				issue { updatedAt }
			}
		}
	`)

	r.Var("issueInput", &struct {
		ID   string `json:"id"`
		Body string `json:"body"`
	}{
		ID:   issue.ID,
		Body: issue.Body,
	})

	return cli.Run(ctx, r, nil)
}
