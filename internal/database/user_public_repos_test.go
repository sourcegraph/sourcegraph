package database

import (
	"context"
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestUserPublicRepos_Set(t *testing.T) {
	db := dbtesting.GetDB(t)
	ctx := context.Background()
	u := Users(db)
	r := Repos(db)
	upr := UserPublicRepos(db)

	user, err := u.Create(ctx, NewUser{
		Username: "u",
		Password: "p",
	})
	if err != nil {
		t.Errorf("Expected no error, got %s ", err)
	}

	err = r.Create(ctx, &types.Repo{
		Name: "test",
		URI:  "https://example.com",
	})
	if err != nil {
		t.Errorf("Expected no error, got %s", err)
	}

	repo, err := r.GetByName(ctx, "test")
	if err != nil {
		t.Errorf("Expected no error, got %s", err)
	}

	err = upr.SetUserRepo(ctx, UserPublicRepo{
		UserID:  user.ID,
		RepoURI: repo.URI,
		RepoID:  repo.ID,
	})
	if err != nil {
		t.Errorf("Expected no error, got %s", err)
	}

	repos, err := upr.ListByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}
	if wanted, got := 1, len(repos); wanted != got {
		t.Errorf("wanted %v repos, got %v", wanted, got)
	}
	if wanted, got := repo.ID, repos[0].RepoID; got != wanted {
		t.Errorf("wanted repo ID %v, got %v", wanted, got)
	}
}

func TestUserPublicRepos_SetUserRepos(t *testing.T) {
	db := dbtesting.GetDB(t)
	ctx := context.Background()
	u := Users(db)
	r := Repos(db)
	upr := UserPublicRepos(db)

	user, err := u.Create(ctx, NewUser{
		Username: "u",
		Password: "p",
	})
	if err != nil {
		t.Errorf("Expected no error, got %s ", err)
	}

	repos := []*types.Repo{
		{
			Name: api.RepoName("test1"),
			URI:  "https://foo.com/1",
		},
		{
			Name: api.RepoName("test2"),
			URI:  "https://foo.com/2",
		},
		{
			Name: api.RepoName("test3"),
			URI:  "https://foo.com/3",
		},
	}
	err = r.Create(ctx, repos...)
	if err != nil {
		t.Errorf("Expected no error, got %s", err)
	}

	// set initial repo
	err = upr.SetUserRepo(ctx, UserPublicRepo{
		UserID:  user.ID,
		RepoURI: repos[0].URI,
		RepoID:  repos[0].ID,
	})
	if err != nil {
		t.Errorf("Expected no error, got %s", err)
	}

	err = upr.SetUserRepos(ctx, user.ID, []UserPublicRepo{
		{
			RepoID:  repos[0].ID,
			RepoURI: repos[0].URI,
		},
		{
			RepoID:  repos[1].ID,
			RepoURI: repos[1].URI,
		},
		{
			RepoID:  repos[2].ID,
			RepoURI: repos[2].URI,
		},
	})
	if err != nil {
		t.Errorf("Expected no error, got %s", err)
	}

	userRepos, err := upr.ListByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("Expected no error, got %s", err)
	}
	if wanted, got := 3, len(userRepos); wanted != got {
		t.Errorf("wanted %v repos, got %v", wanted, got)
	}
	for i := range userRepos {
		if wanted, got := repos[i].URI, userRepos[i].RepoURI; wanted != got {
			t.Errorf("wanted repo ID %v, got %v", wanted, got)
		}
		if wanted, got := repos[i].ID, userRepos[i].RepoID; got != wanted {
			t.Errorf("wanted repo ID %v, got %v", wanted, got)
		}
	}
}
