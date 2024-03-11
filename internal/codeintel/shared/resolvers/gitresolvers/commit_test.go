package gitresolvers

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestCommit(t *testing.T) {
	ctx := context.Background()
	gsClient := gitserver.NewMockClient()
	gsClient.ListRefsFunc.SetDefaultReturn([]gitdomain.Ref{
		{ShortName: "v1"}, {ShortName: "v2"}, {ShortName: "v3"}, {ShortName: "v4"}, {ShortName: "v5"},
	}, nil)

	repo := &repoResolver{repo: &types.Repo{
		Name: api.RepoName("github.com/test/test"),
	}}
	resolver := NewGitCommitResolver(gsClient, repo, api.CommitID("deadbeef"), "")

	for range 10 {
		tags, err := resolver.Tags(ctx)
		if err != nil {
			t.Fatalf("unexpected error from tags: %s", err)
		}
		if diff := cmp.Diff([]string{"v1", "v2", "v3", "v4", "v5"}, tags); diff != "" {
			t.Errorf("unexpected tags (-want +got):\n%s", diff)
		}
	}

	if len(gsClient.ListRefsFunc.History()) != 1 {
		t.Fatalf("expected function to be memoized")
	}
}
