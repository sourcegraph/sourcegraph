package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
)

func mustURL(t *testing.T, u string) *url.URL {
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func TestProvider_FetchUserPerms(t *testing.T) {
	t.Run("nil account", func(t *testing.T) {
		p := NewProvider("", mustURL(t, "https://github.com"), "admin_token", nil)
		_, err := p.FetchUserPerms(context.Background(), nil)
		want := "no account provided"
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	t.Run("not the code host of the account", func(t *testing.T) {
		p := NewProvider("", mustURL(t, "https://github.com"), "admin_token", nil)
		_, err := p.FetchUserPerms(context.Background(),
			&extsvc.Account{
				AccountSpec: extsvc.AccountSpec{
					ServiceType: "gitlab",
					ServiceID:   "https://gitlab.com/",
				},
			},
		)
		want := `not a code host of the account: want "https://gitlab.com/" but have "https://github.com/"`
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	t.Run("no token found in account data", func(t *testing.T) {
		p := NewProvider("", mustURL(t, "https://github.com"), "admin_token", nil)
		_, err := p.FetchUserPerms(context.Background(),
			&extsvc.Account{
				AccountSpec: extsvc.AccountSpec{
					ServiceType: "github",
					ServiceID:   "https://github.com/",
				},
				AccountData: extsvc.AccountData{},
			},
		)
		want := `no token found in the external account data`
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	mockClient := &mockClient{
		MockListAffiliatedRepositories: func(ctx context.Context, visibility github.Visibility, page int) ([]*github.Repository, bool, int, error) {
			switch page {
			case 1:
				return []*github.Repository{
					{ID: "MDEwOlJlcG9zaXRvcnkyNTI0MjU2NzE="},
					{ID: "MDEwOlJlcG9zaXRvcnkyNDQ1MTc1MzY="},
				}, true, 1, nil
			case 2:
				return []*github.Repository{
					{ID: "MDEwOlJlcG9zaXRvcnkyNDI2NTEwMDA="},
				}, false, 1, nil
			}

			return []*github.Repository{}, false, 1, nil
		},
	}
	calledWithToken := false
	mockClient.MockWithToken = func(token string) client {
		calledWithToken = true
		return mockClient
	}

	p := NewProvider("", mustURL(t, "https://github.com"), "admin_token", nil)
	p.client = mockClient

	authData := json.RawMessage(`{"access_token": "my_access_token"}`)
	repoIDs, err := p.FetchUserPerms(context.Background(),
		&extsvc.Account{
			AccountSpec: extsvc.AccountSpec{
				ServiceType: "github",
				ServiceID:   "https://github.com/",
			},
			AccountData: extsvc.AccountData{
				AuthData: &authData,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if !calledWithToken {
		t.Fatal("!calledWithToken")
	}

	wantRepoIDs := []extsvc.RepoID{
		"MDEwOlJlcG9zaXRvcnkyNTI0MjU2NzE=",
		"MDEwOlJlcG9zaXRvcnkyNDQ1MTc1MzY=",
		"MDEwOlJlcG9zaXRvcnkyNDI2NTEwMDA=",
	}
	if diff := cmp.Diff(wantRepoIDs, repoIDs.Exacts); diff != "" {
		t.Fatalf("RpoeIDs mismatch (-want +got):\n%s", diff)
	}
}

func TestProvider_FetchRepoPerms(t *testing.T) {
	t.Run("nil repository", func(t *testing.T) {
		p := NewProvider("", mustURL(t, "https://github.com"), "admin_token", nil)
		_, err := p.FetchRepoPerms(context.Background(), nil)
		want := "no repository provided"
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	t.Run("not the code host of the repository", func(t *testing.T) {
		p := NewProvider("", mustURL(t, "https://github.com"), "admin_token", nil)
		_, err := p.FetchRepoPerms(context.Background(),
			&extsvc.Repository{
				URI: "gitlab.com/user/repo",
				ExternalRepoSpec: api.ExternalRepoSpec{
					ServiceType: "gitlab",
					ServiceID:   "https://gitlab.com/",
				},
			},
		)
		want := `not a code host of the repository: want "https://gitlab.com/" but have "https://github.com/"`
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	p := NewProvider("", mustURL(t, "https://github.com"), "admin_token", nil)
	p.client = &mockClient{
		MockListRepositoryCollaborators: func(ctx context.Context, owner, repo string, page int) ([]*github.Collaborator, bool, error) {
			switch page {
			case 1:
				return []*github.Collaborator{
					{DatabaseID: 57463526},
					{DatabaseID: 67471},
				}, true, nil
			case 2:
				return []*github.Collaborator{
					{DatabaseID: 187831},
				}, false, nil
			}

			return []*github.Collaborator{}, false, nil
		},
	}

	accountIDs, err := p.FetchRepoPerms(context.Background(),
		&extsvc.Repository{
			URI: "github.com/user/repo",
			ExternalRepoSpec: api.ExternalRepoSpec{
				ID:          "github_project_id",
				ServiceType: "github",
				ServiceID:   "https://github.com/",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	wantAccountIDs := []extsvc.AccountID{
		"57463526",
		"67471",
		"187831",
	}
	if diff := cmp.Diff(wantAccountIDs, accountIDs); diff != "" {
		t.Fatalf("AccountIDs mismatch (-want +got):\n%s", diff)
	}
}
