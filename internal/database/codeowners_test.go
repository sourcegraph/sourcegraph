package database

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	codeownerspb "github.com/sourcegraph/sourcegraph/internal/own/codeowners/v1"
	owntypes "github.com/sourcegraph/sourcegraph/internal/own/types"
	"github.com/sourcegraph/sourcegraph/internal/timeutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// createRepos is a helper to set up repos we use in this test to satisfy foreign key constraints.
// repo IDs are generated automatically, so we just specify the number we want.
func createRepos(t *testing.T, ctx context.Context, store RepoStore, numOfRepos int) {
	t.Helper()
	for i := range numOfRepos {
		if err := store.Create(ctx, &types.Repo{
			Name: api.RepoName(fmt.Sprintf("%d", i)),
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCodeowners_CreateUpdateDelete(t *testing.T) {
	ctx := context.Background()

	logger := logtest.NoOp(t)
	db := NewDB(logger, dbtest.NewDB(t))

	createRepos(t, ctx, db.Repos(), 6)
	store := db.Codeowners()

	t.Run("create new codeowners file", func(t *testing.T) {
		codeowners := newCodeownersFile("*", "everyone", api.RepoID(1))
		if err := store.CreateCodeownersFile(ctx, codeowners); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("create codeowners duplicate error", func(t *testing.T) {
		codeowners := newCodeownersFile("*", "everyone", api.RepoID(2))
		if err := store.CreateCodeownersFile(ctx, codeowners); err != nil {
			t.Fatal(err)
		}
		secondErr := store.CreateCodeownersFile(ctx, codeowners)
		if secondErr == nil {
			t.Fatal("expect duplicate codeowners to error")
		}
		require.ErrorAs(t, ErrCodeownersFileAlreadyExists, &secondErr)
	})

	t.Run("update codeowners file", func(t *testing.T) {
		codeowners := newCodeownersFile("*", "everyone", api.RepoID(3))
		if err := store.CreateCodeownersFile(ctx, codeowners); err != nil {
			t.Fatal(err)
		}
		codeowners = newCodeownersFile("*", "notEveryone", api.RepoID(3))
		if err := store.UpdateCodeownersFile(ctx, codeowners); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("update non existent codeowners file", func(t *testing.T) {
		codeowners := newCodeownersFile("*", "notEveryone", api.RepoID(4))
		err := store.UpdateCodeownersFile(ctx, codeowners)
		if err == nil {
			t.Fatal("expected not found error")
		}
		require.ErrorAs(t, CodeownersFileNotFoundError{}, &err)
	})

	t.Run("delete", func(t *testing.T) {
		repoID := api.RepoID(5)
		codeowners := newCodeownersFile("*", "everyone", repoID)
		if err := store.CreateCodeownersFile(ctx, codeowners); err != nil {
			t.Fatal(err)
		}
		if err := store.DeleteCodeownersForRepos(ctx, 5); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("delete non existent codeowners file", func(t *testing.T) {
		err := store.DeleteCodeownersForRepos(ctx, 6)
		if err == nil {
			t.Fatal("did not return useful not found information")
		}
		require.ErrorAs(t, CodeownersFileNotFoundError{}, &err)
	})
}

func TestCodeowners_GetListCount(t *testing.T) {
	ctx := context.Background()

	logger := logtest.NoOp(t)
	db := NewDB(logger, dbtest.NewDB(t))

	createRepos(t, ctx, db.Repos(), 2)

	store := db.Codeowners()

	createFile := func(file *owntypes.CodeownersFile) *owntypes.CodeownersFile {
		if err := store.CreateCodeownersFile(ctx, file); err != nil {
			t.Fatal(err)
		}
		return file
	}
	repo1Codeowners := createFile(newCodeownersFile("*", "person", api.RepoID(1)))
	repo2Codeowners := createFile(newCodeownersFile("*", "everyone", api.RepoID(2)))

	t.Run("get", func(t *testing.T) {
		t.Run("not found", func(t *testing.T) {
			_, err := store.GetCodeownersForRepo(ctx, api.RepoID(100))
			if err == nil {
				t.Fatal("expected an error")
			}
			require.ErrorAs(t, CodeownersFileNotFoundError{}, &err)
		})
		t.Run("get by repo ID", func(t *testing.T) {
			got, err := store.GetCodeownersForRepo(ctx, api.RepoID(1))
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(repo1Codeowners, got, protocmp.Transform()); diff != "" {
				t.Fatal(diff)
			}
		})
		t.Run("get by repo ID after update", func(t *testing.T) {
			got, err := store.GetCodeownersForRepo(ctx, api.RepoID(2))
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(repo2Codeowners, got, protocmp.Transform()); diff != "" {
				t.Fatal(diff)
			}
			repo2Codeowners.UpdatedAt = timeutil.Now()
			if err := store.UpdateCodeownersFile(ctx, repo2Codeowners); err != nil {
				t.Fatal(err)
			}
			got, err = store.GetCodeownersForRepo(ctx, api.RepoID(2))
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(repo2Codeowners, got, protocmp.Transform()); diff != "" {
				t.Fatal(diff)
			}
		})
	})

	t.Run("list", func(t *testing.T) {
		all := []*owntypes.CodeownersFile{repo1Codeowners, repo2Codeowners}

		// List all
		have, cursor, err := store.ListCodeowners(ctx, ListCodeownersOpts{})
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(all, have, protocmp.Transform()); diff != "" {
			t.Fatal(diff)
		}
		//require.Equal(t, all, have)
		if cursor != 0 {
			t.Fatal("incorrect cursor returned")
		}

		// List with cursor pagination
		var lastCursor int32
		for i := range len(all) {
			t.Run(fmt.Sprintf("list codeowners n#%d", i), func(t *testing.T) {
				opts := ListCodeownersOpts{LimitOffset: &LimitOffset{Limit: 1}, Cursor: lastCursor}
				cf, c, err := store.ListCodeowners(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				lastCursor = c
				if diff := cmp.Diff(all[i], cf[0], protocmp.Transform()); diff != "" {
					t.Error(diff)
				}
			})
		}
	})

	t.Run("count", func(t *testing.T) {
		got, err := store.CountCodeownersFiles(ctx)
		if err != nil {
			t.Fatal(err)
		}
		require.Equal(t, int32(2), got)
	})
}

// newCodeownersFile returns a simple test Codeowners file with one pattern and one owner.
func newCodeownersFile(pattern, handle string, repoID api.RepoID) *owntypes.CodeownersFile {
	return &owntypes.CodeownersFile{
		Contents: fmt.Sprintf("%s @%s", pattern, handle),
		Proto: &codeownerspb.File{
			Rule: []*codeownerspb.Rule{
				{
					Pattern: pattern,
					Owner:   []*codeownerspb.Owner{{Handle: handle}},
				},
			},
		},
		RepoID: repoID,
	}
}
