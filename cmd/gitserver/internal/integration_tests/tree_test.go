package inttests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	srp "github.com/sourcegraph/sourcegraph/internal/authz/subrepoperms"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestReadDir_SubRepoFiltering(t *testing.T) {
	ctx := actor.WithActor(context.Background(), &actor.Actor{
		UID: 1,
	})
	gitCommands := []string{
		"touch file1",
		"git add file1",
		"git commit -m commit1",
		"mkdir app",
		"touch app/file2",
		"git add app",
		"git commit -m commit2",
	}
	repo := MakeGitRepository(t, gitCommands...)
	commitID := api.CommitID("b1c725720de2bbd0518731b4a61959797ff345f3")
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			ExperimentalFeatures: &schema.ExperimentalFeatures{
				SubRepoPermissions: &schema.SubRepoPermissions{
					Enabled: true,
				},
			},
		},
	})
	defer conf.Mock(nil)
	srpGetter := dbmocks.NewMockSubRepoPermsStore()
	testSubRepoPerms := map[api.RepoName]authz.SubRepoPermissions{
		repo: {
			Paths: []string{"/**", "-/app/**"},
		},
	}
	srpGetter.GetByUserFunc.SetDefaultReturn(testSubRepoPerms, nil)
	checker := srp.NewSubRepoPermsClient(srpGetter)

	source := gitserver.NewTestClientSource(t, GitserverAddresses)
	client := gitserver.NewTestClient(t).WithClientSource(source).WithChecker(checker)
	files, err := client.ReadDir(ctx, repo, commitID, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	// Because we have a wildcard matcher we still allow directory visibility
	assert.Len(t, files, 1)
	assert.Equal(t, "file1", files[0].Name())
	assert.False(t, files[0].IsDir())
}
