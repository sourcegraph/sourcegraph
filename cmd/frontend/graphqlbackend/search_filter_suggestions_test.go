package graphqlbackend

import (
	"context"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/db"
	"reflect"
	"testing"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/types"
)

func TestSearchFilterSuggestions(t *testing.T) {
	mockResolveRepoGroups = func() (map[string][]*types.Repo, error) {
		return map[string][]*types.Repo{
			"repogroup1": {},
			"repogroup2": {},
		}, nil
	}
	defer func() { mockResolveRepoGroups = nil }()

	db.Mocks.Repos.List = func(_ context.Context, _ db.ReposListOptions) ([]*types.Repo, error) {
		return []*types.Repo{
			{Name: "github.com/foo/repo"},
			{Name: "bar-repo"},
		}, nil
	}
	defer func() { db.Mocks.Repos.List = nil }()

	r, err := (&schemaResolver{}).SearchFilterSuggestions(context.Background())
	if err != nil {
		t.Fatal("SearchFilterSuggestions:", err)
	}

	want := &searchFilterSuggestions{
		repogroups: []string{"repogroup1", "repogroup2"},
		repos:      []string{`^github\.com/foo/repo$`, "^bar-repo$"},
	}

	if !reflect.DeepEqual(r, want) {
		t.Errorf("got != want\ngot:  %v\nwant: %v", r, want)
	}
}
