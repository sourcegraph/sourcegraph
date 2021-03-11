package result

import (
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

type CommitMatch struct {
	Commit         git.Commit
	RepoName       types.RepoName
	Refs           []string
	SourceRefs     []string
	MessagePreview *HighlightedString
	DiffPreview    *HighlightedString
	Body           HighlightedString
}

// ResultCount for CommitSearchResult returns the number of highlights if there
// are highlights and 1 otherwise. We implemented this method because we want to
// return a more meaningful result count for streaming while maintaining backward
// compatibility for our GraphQL API. The GraphQL API calls ResultCount on the
// resolver, while streaming calls ResultCount on CommitSearchResult.
func (r *CommitMatch) ResultCount() int {
	if n := len(r.Body.Highlights); n > 0 {
		return n
	}
	// Queries such as type:commit after:"1 week ago" don't have highlights. We count
	// those results as 1.
	return 1
}

func (r *CommitMatch) Limit(limit int) int {
	if len(r.Body.Highlights) == 0 {
		return limit - 1 // just counting the commit
	} else if len(r.Body.Highlights) > limit {
		r.Body.Highlights = r.Body.Highlights[:limit]
		return 0
	}
	return limit - len(r.Body.Highlights)
}
