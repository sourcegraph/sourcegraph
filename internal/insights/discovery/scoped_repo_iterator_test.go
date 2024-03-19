package discovery

import (
	"context"
	"reflect"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestScopedRepoIteratorForEach(t *testing.T) {
	repoStore := NewMockRepoStore()
	mockResponse := []*types.Repo{{
		ID:   5,
		Name: "github.com/org/repo",
	}, {
		ID:   6,
		Name: "gitlab.com/org1/repo1",
	}}
	repoStore.ListFunc.SetDefaultReturn(mockResponse, nil)

	var names []string
	for _, repo := range mockResponse {
		names = append(names, string(repo.Name))
	}

	iterator, err := NewScopedRepoIterator(context.Background(), names, repoStore)
	if err != nil {
		t.Fatal(err)
	}

	// verify the names argument actually matches what is expected and we arent just trusting a mock blindly
	if !reflect.DeepEqual(repoStore.ListFunc.History()[0].Arg1.Names, names) {
		t.Error("argument mismatch on repo names")
	}

	var gotNames []string
	var gotIds []api.RepoID
	err = iterator.ForEach(context.Background(), func(repoName string, id api.RepoID) error {
		gotNames = append(gotNames, repoName)
		gotIds = append(gotIds, id)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("expect_equal_repo_names", func(t *testing.T) {
		autogold.Expect([]string{"github.com/org/repo", "gitlab.com/org1/repo1"}).Equal(t, gotNames)
	})
	t.Run("expect_equal_repo_ids", func(t *testing.T) {
		autogold.Expect([]api.RepoID{api.RepoID(5), api.RepoID(6)}).Equal(t, gotIds)
	})
}

func TestScopedRepoIterator_PrivateRepos(t *testing.T) {
	logger := logtest.Scoped(t)
	db := database.NewDB(logger, dbtest.NewDB(t))
	ctx := context.Background()

	store := db.Repos()

	err := store.Create(ctx, &types.Repo{
		ID:      1,
		Name:    "insights/repo1",
		Private: true,
	})
	require.NoError(t, err)

	userWithAccess, err := db.Users().Create(ctx, database.NewUser{Username: "user1234"})
	require.NoError(t, err)

	userNoAccess, err := db.Users().Create(ctx, database.NewUser{Username: "user-no-access"})
	require.NoError(t, err)

	// this is required otherwise setting the permissions won't do anything
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			PermissionsUserMapping: &schema.PermissionsUserMapping{
				Enabled: true,
				BindID:  "email",
			},
		},
	})
	t.Cleanup(func() { conf.Mock(nil) })

	_, err = db.Perms().SetRepoPerms(ctx, 1, []authz.UserIDWithExternalAccountID{{UserID: userWithAccess.ID}}, authz.SourceAPI)
	require.NoError(t, err)

	t.Run("non-internal user", func(t *testing.T) {
		newCtx := actor.WithActor(ctx, actor.FromUser(userNoAccess.ID)) // just to make sure this is a different user
		require.NoError(t, err)

		iterator, err := NewScopedRepoIterator(newCtx, []string{"insights/repo1"}, store)
		require.NoError(t, err)
		count := 0
		err = iterator.ForEach(newCtx, func(repoName string, id api.RepoID) error {
			count++
			return nil
		})
		require.NoError(t, err)
		assert.Zero(t, count)
	})

	t.Run("internal user", func(t *testing.T) {
		newCtx := actor.WithInternalActor(ctx)
		require.NoError(t, err)

		iterator, err := NewScopedRepoIterator(newCtx, []string{"insights/repo1"}, store)
		require.NoError(t, err)
		count := 0
		err = iterator.ForEach(newCtx, func(repoName string, id api.RepoID) error {
			count++
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 1, count)
	})
}
