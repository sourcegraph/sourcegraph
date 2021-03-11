package state

import (
	"time"

	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
)

func setDraft(c *batches.Changeset) *batches.Changeset {
	switch m := c.Metadata.(type) {
	case *github.PullRequest:
		m.IsDraft = true
	case *gitlab.MergeRequest:
		m.WorkInProgress = true
	}
	return c
}

func timeToUnixMilli(t time.Time) int {
	return int(t.UnixNano()) / int(time.Millisecond)
}
