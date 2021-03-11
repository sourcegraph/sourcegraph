package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
)

type mockDoer struct {
	do func(*http.Request) (*http.Response, error)
}

func (c *mockDoer) Do(r *http.Request) (*http.Response, error) {
	return c.do(r)
}

func TestOAuthProvider_FetchUserPerms(t *testing.T) {
	t.Run("nil account", func(t *testing.T) {
		p := newOAuthProvider(OAuthProviderOp{
			BaseURL: mustURL(t, "https://gitlab.com"),
		}, nil)
		_, err := p.FetchUserPerms(context.Background(), nil)
		want := "no account provided"
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	t.Run("not the code host of the account", func(t *testing.T) {
		p := newOAuthProvider(OAuthProviderOp{
			BaseURL: mustURL(t, "https://gitlab.com"),
		}, nil)
		_, err := p.FetchUserPerms(context.Background(),
			&extsvc.Account{
				AccountSpec: extsvc.AccountSpec{
					ServiceType: extsvc.TypeGitHub,
					ServiceID:   "https://github.com/",
				},
			},
		)
		want := `not a code host of the account: want "https://github.com/" but have "https://gitlab.com/"`
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	// The OAuthProvider uses the gitlab.Client under the hood,
	// which uses rcache, a caching layer that uses Redis.
	// We need to clear the cache before we run the tests
	rcache.SetupForTest(t)

	p := newOAuthProvider(
		OAuthProviderOp{
			BaseURL: mustURL(t, "https://gitlab.com"),
			Token:   "admin_token",
		},
		&mockDoer{
			do: func(r *http.Request) (*http.Response, error) {
				want := "https://gitlab.com/api/v4/projects?min_access_level=20&per_page=100&visibility=private"
				if r.URL.String() != want {
					return nil, fmt.Errorf("URL: want %q but got %q", want, r.URL)
				}

				want = "Bearer my_access_token"
				got := r.Header.Get("Authorization")
				if got != want {
					return nil, fmt.Errorf("HTTP Authorization: want %q but got %q", want, got)
				}

				body := `[{"id": 1}, {"id": 2}, {"id": 3}]`
				return &http.Response{
					Status:     http.StatusText(http.StatusOK),
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewReader([]byte(body))),
				}, nil
			},
		},
	)

	authData := json.RawMessage(`{"access_token": "my_access_token"}`)
	repoIDs, err := p.FetchUserPerms(context.Background(),
		&extsvc.Account{
			AccountSpec: extsvc.AccountSpec{
				ServiceType: extsvc.TypeGitLab,
				ServiceID:   "https://gitlab.com/",
			},
			AccountData: extsvc.AccountData{
				AuthData: &authData,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	expRepoIDs := []extsvc.RepoID{"1", "2", "3"}
	if diff := cmp.Diff(expRepoIDs, repoIDs.Exacts); diff != "" {
		t.Fatal(diff)
	}
}

func TestOAuthProvider_FetchRepoPerms(t *testing.T) {
	t.Run("nil repository", func(t *testing.T) {
		p := newOAuthProvider(OAuthProviderOp{
			BaseURL: mustURL(t, "https://gitlab.com"),
		}, nil)
		_, err := p.FetchRepoPerms(context.Background(), nil)
		want := "no repository provided"
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	t.Run("not the code host of the repository", func(t *testing.T) {
		p := newOAuthProvider(OAuthProviderOp{
			BaseURL: mustURL(t, "https://gitlab.com"),
		}, nil)
		_, err := p.FetchRepoPerms(context.Background(),
			&extsvc.Repository{
				URI: "github.com/user/repo",
				ExternalRepoSpec: api.ExternalRepoSpec{
					ServiceType: extsvc.TypeGitHub,
					ServiceID:   "https://github.com/",
				},
			},
		)
		want := `not a code host of the repository: want "https://github.com/" but have "https://gitlab.com/"`
		got := fmt.Sprintf("%v", err)
		if got != want {
			t.Fatalf("err: want %q but got %q", want, got)
		}
	})

	// The OAuthProvider uses the gitlab.Client under the hood,
	// which uses rcache, a caching layer that uses Redis.
	// We need to clear the cache before we run the tests
	rcache.SetupForTest(t)

	p := newOAuthProvider(
		OAuthProviderOp{
			BaseURL: mustURL(t, "https://gitlab.com"),
			Token:   "admin_token",
		},
		&mockDoer{
			do: func(r *http.Request) (*http.Response, error) {
				want := "https://gitlab.com/api/v4/projects/gitlab_project_id/members/all?per_page=100"
				if r.URL.String() != want {
					return nil, fmt.Errorf("URL: want %q but got %q", want, r.URL)
				}

				want = "admin_token"
				got := r.Header.Get("Private-Token")
				if got != want {
					return nil, fmt.Errorf("HTTP Private-Token: want %q but got %q", want, got)
				}

				body := `
[
	{"id": 1, "access_level": 10},
	{"id": 2, "access_level": 20},
	{"id": 3, "access_level": 30}
]`
				return &http.Response{
					Status:     http.StatusText(http.StatusOK),
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewReader([]byte(body))),
				}, nil
			},
		},
	)

	accountIDs, err := p.FetchRepoPerms(context.Background(),
		&extsvc.Repository{
			URI: "gitlab.com/user/repo",
			ExternalRepoSpec: api.ExternalRepoSpec{
				ServiceType: "gitlab",
				ServiceID:   "https://gitlab.com/",
				ID:          "gitlab_project_id",
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	// 1 should not be included because of "access_level" < 20
	expAccountIDs := []extsvc.AccountID{"2", "3"}
	if diff := cmp.Diff(expAccountIDs, accountIDs); diff != "" {
		t.Fatal(diff)
	}
}
