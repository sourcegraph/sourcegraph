package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/batches"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/testutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestExampleRepositoryQuerySplit(t *testing.T) {
	q := "org:sourcegraph"
	want := `["org:sourcegraph created:>=2019","org:sourcegraph created:2018","org:sourcegraph created:2016..2017","org:sourcegraph created:<2016"]`
	have := exampleRepositoryQuerySplit(q)
	if want != have {
		t.Errorf("unexpected example query for %s:\nwant: %s\nhave: %s", q, want, have)
	}
}

func TestGithubSource_CreateChangeset(t *testing.T) {
	repo := &types.Repo{
		Metadata: &github.Repository{
			ID:            "MDEwOlJlcG9zaXRvcnkyMjExNDc1MTM=",
			NameWithOwner: "sourcegraph/automation-testing",
		},
	}

	testCases := []struct {
		name   string
		cs     *Changeset
		err    string
		exists bool
	}{
		{
			name: "success",
			cs: &Changeset{
				Title:     "This is a test PR",
				Body:      "This is the description of the test PR",
				HeadRef:   "refs/heads/test-pr-6",
				BaseRef:   "refs/heads/master",
				Repo:      repo,
				Changeset: &batches.Changeset{},
			},
		},
		{
			name: "already exists",
			cs: &Changeset{
				Title:     "This is a test PR",
				Body:      "This is the description of the test PR",
				HeadRef:   "refs/heads/always-open-pr",
				BaseRef:   "refs/heads/master",
				Repo:      repo,
				Changeset: &batches.Changeset{},
			},
			// If PR already exists we'll just return it, no error
			err:    "",
			exists: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		tc.name = "GithubSource_CreateChangeset_" + strings.Replace(tc.name, " ", "_", -1)

		t.Run(tc.name, func(t *testing.T) {
			// The GithubSource uses the github.Client under the hood, which
			// uses rcache, a caching layer that uses Redis.
			// We need to clear the cache before we run the tests
			rcache.SetupForTest(t)

			cf, save := newClientFactory(t, tc.name)
			defer save(t)

			lg := log15.New()
			lg.SetHandler(log15.DiscardHandler())

			svc := &types.ExternalService{
				Kind: extsvc.KindGitHub,
				Config: marshalJSON(t, &schema.GitHubConnection{
					Url:   "https://github.com",
					Token: os.Getenv("GITHUB_TOKEN"),
				}),
			}

			githubSrc, err := NewGithubSource(svc, cf)
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			if tc.err == "" {
				tc.err = "<nil>"
			}

			exists, err := githubSrc.CreateChangeset(ctx, tc.cs)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			if have, want := exists, tc.exists; have != want {
				t.Errorf("exists:\nhave: %t\nwant: %t", have, want)
			}

			pr, ok := tc.cs.Changeset.Metadata.(*github.PullRequest)
			if !ok {
				t.Fatal("Metadata does not contain PR")
			}

			testutil.AssertGolden(t, "testdata/golden/"+tc.name, update(tc.name), pr)
		})
	}
}

func TestGithubSource_CloseChangeset(t *testing.T) {
	testCases := []struct {
		name string
		cs   *Changeset
		err  string
	}{
		{
			name: "success",
			cs: &Changeset{
				Changeset: &batches.Changeset{
					Metadata: &github.PullRequest{
						ID: "MDExOlB1bGxSZXF1ZXN0MzQ5NTIzMzE0",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		tc.name = "GithubSource_CloseChangeset_" + strings.Replace(tc.name, " ", "_", -1)

		t.Run(tc.name, func(t *testing.T) {
			// The GithubSource uses the github.Client under the hood, which
			// uses rcache, a caching layer that uses Redis.
			// We need to clear the cache before we run the tests
			rcache.SetupForTest(t)

			cf, save := newClientFactory(t, tc.name)
			defer save(t)

			lg := log15.New()
			lg.SetHandler(log15.DiscardHandler())

			svc := &types.ExternalService{
				Kind: extsvc.KindGitHub,
				Config: marshalJSON(t, &schema.GitHubConnection{
					Url:   "https://github.com",
					Token: os.Getenv("GITHUB_TOKEN"),
				}),
			}

			githubSrc, err := NewGithubSource(svc, cf)
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			if tc.err == "" {
				tc.err = "<nil>"
			}

			err = githubSrc.CloseChangeset(ctx, tc.cs)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			pr := tc.cs.Changeset.Metadata.(*github.PullRequest)
			testutil.AssertGolden(t, "testdata/golden/"+tc.name, update(tc.name), pr)
		})
	}
}

func TestGithubSource_ReopenChangeset(t *testing.T) {
	testCases := []struct {
		name string
		cs   *Changeset
		err  string
	}{
		{
			name: "success",
			cs: &Changeset{
				Changeset: &batches.Changeset{
					Metadata: &github.PullRequest{
						// https://github.com/sourcegraph/automation-testing/pull/353
						ID: "MDExOlB1bGxSZXF1ZXN0NDg4MDI2OTk5",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		tc.name = "GithubSource_ReopenChangeset_" + strings.Replace(tc.name, " ", "_", -1)

		t.Run(tc.name, func(t *testing.T) {
			// The GithubSource uses the github.Client under the hood, which
			// uses rcache, a caching layer that uses Redis.
			// We need to clear the cache before we run the tests
			rcache.SetupForTest(t)

			cf, save := newClientFactory(t, tc.name)
			defer save(t)

			lg := log15.New()
			lg.SetHandler(log15.DiscardHandler())

			svc := &types.ExternalService{
				Kind: extsvc.KindGitHub,
				Config: marshalJSON(t, &schema.GitHubConnection{
					Url:   "https://github.com",
					Token: os.Getenv("GITHUB_TOKEN"),
				}),
			}

			githubSrc, err := NewGithubSource(svc, cf)
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			if tc.err == "" {
				tc.err = "<nil>"
			}

			err = githubSrc.ReopenChangeset(ctx, tc.cs)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			pr := tc.cs.Changeset.Metadata.(*github.PullRequest)
			testutil.AssertGolden(t, "testdata/golden/"+tc.name, update(tc.name), pr)
		})
	}
}

func TestGithubSource_UpdateChangeset(t *testing.T) {
	testCases := []struct {
		name string
		cs   *Changeset
		err  string
	}{
		{
			name: "success",
			cs: &Changeset{
				Title:   "This is a new title",
				Body:    "This is a new body",
				BaseRef: "refs/heads/master",
				Changeset: &batches.Changeset{
					Metadata: &github.PullRequest{
						ID: "MDExOlB1bGxSZXF1ZXN0NTA0NDU4Njg1",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		tc.name = "GithubSource_UpdateChangeset_" + strings.Replace(tc.name, " ", "_", -1)

		t.Run(tc.name, func(t *testing.T) {
			// The GithubSource uses the github.Client under the hood, which
			// uses rcache, a caching layer that uses Redis.
			// We need to clear the cache before we run the tests
			rcache.SetupForTest(t)

			cf, save := newClientFactory(t, tc.name)
			defer save(t)

			lg := log15.New()
			lg.SetHandler(log15.DiscardHandler())

			svc := &types.ExternalService{
				Kind: extsvc.KindGitHub,
				Config: marshalJSON(t, &schema.GitHubConnection{
					Url:   "https://github.com",
					Token: os.Getenv("GITHUB_TOKEN"),
				}),
			}

			githubSrc, err := NewGithubSource(svc, cf)
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			if tc.err == "" {
				tc.err = "<nil>"
			}

			err = githubSrc.UpdateChangeset(ctx, tc.cs)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			pr := tc.cs.Changeset.Metadata.(*github.PullRequest)
			testutil.AssertGolden(t, "testdata/golden/"+tc.name, update(tc.name), pr)
		})
	}
}

func TestGithubSource_LoadChangeset(t *testing.T) {
	testCases := []struct {
		name string
		cs   *Changeset
		err  string
	}{
		{
			name: "found",
			cs: &Changeset{
				Repo:      &types.Repo{Metadata: &github.Repository{NameWithOwner: "sourcegraph/sourcegraph"}},
				Changeset: &batches.Changeset{ExternalID: "5550"},
			},
		},
		{
			name: "not-found",
			cs: &Changeset{
				Repo:      &types.Repo{Metadata: &github.Repository{NameWithOwner: "sourcegraph/sourcegraph"}},
				Changeset: &batches.Changeset{ExternalID: "100000"},
			},
			err: "Changeset with external ID 100000 not found",
		},
	}

	for _, tc := range testCases {
		tc := tc
		tc.name = "GithubSource_LoadChangeset_" + tc.name

		t.Run(tc.name, func(t *testing.T) {
			// The GithubSource uses the github.Client under the hood, which
			// uses rcache, a caching layer that uses Redis.
			// We need to clear the cache before we run the tests
			rcache.SetupForTest(t)

			cf, save := newClientFactory(t, tc.name)
			defer save(t)

			lg := log15.New()
			lg.SetHandler(log15.DiscardHandler())

			svc := &types.ExternalService{
				Kind: extsvc.KindGitHub,
				Config: marshalJSON(t, &schema.GitHubConnection{
					Url:   "https://github.com",
					Token: os.Getenv("GITHUB_TOKEN"),
				}),
			}

			githubSrc, err := NewGithubSource(svc, cf)
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			if tc.err == "" {
				tc.err = "<nil>"
			}

			err = githubSrc.LoadChangeset(ctx, tc.cs)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if err != nil {
				return
			}

			meta := tc.cs.Changeset.Metadata.(*github.PullRequest)
			testutil.AssertGolden(t, "testdata/golden/"+tc.name, update(tc.name), meta)
		})
	}
}

func TestGithubSource_GetRepo(t *testing.T) {
	testCases := []struct {
		name          string
		nameWithOwner string
		assert        func(*testing.T, *types.Repo)
		err           string
	}{
		{
			name:          "invalid name",
			nameWithOwner: "thisIsNotANameWithOwner",
			err:           `Invalid GitHub repository: nameWithOwner=thisIsNotANameWithOwner: invalid GitHub repository "owner/name" string: "thisIsNotANameWithOwner"`,
		},
		{
			name:          "not found",
			nameWithOwner: "foobarfoobarfoobar/please-let-this-not-exist",
			err:           `GitHub repository not found`,
		},
		{
			name:          "found",
			nameWithOwner: "sourcegraph/sourcegraph",
			assert: func(t *testing.T, have *types.Repo) {
				t.Helper()

				want := &types.Repo{
					Name:        "github.com/sourcegraph/sourcegraph",
					Description: "Code search and navigation tool (self-hosted)",
					URI:         "github.com/sourcegraph/sourcegraph",
					ExternalRepo: api.ExternalRepoSpec{
						ID:          "MDEwOlJlcG9zaXRvcnk0MTI4ODcwOA==",
						ServiceType: "github",
						ServiceID:   "https://github.com/",
					},
					Sources: map[string]*types.SourceInfo{
						"extsvc:github:0": {
							ID:       "extsvc:github:0",
							CloneURL: "https://github.com/sourcegraph/sourcegraph",
						},
					},
					Metadata: &github.Repository{
						ID:            "MDEwOlJlcG9zaXRvcnk0MTI4ODcwOA==",
						DatabaseID:    41288708,
						NameWithOwner: "sourcegraph/sourcegraph",
						Description:   "Code search and navigation tool (self-hosted)",
						URL:           "https://github.com/sourcegraph/sourcegraph",
					},
				}

				if !reflect.DeepEqual(have, want) {
					t.Errorf("response: %s", cmp.Diff(have, want))
				}
			},
			err: "<nil>",
		},
	}

	for _, tc := range testCases {
		tc := tc
		tc.name = "GITHUB-DOT-COM/" + tc.name

		t.Run(tc.name, func(t *testing.T) {
			// The GithubSource uses the github.Client under the hood, which
			// uses rcache, a caching layer that uses Redis.
			// We need to clear the cache before we run the tests
			rcache.SetupForTest(t)

			cf, save := newClientFactory(t, tc.name)
			defer save(t)

			lg := log15.New()
			lg.SetHandler(log15.DiscardHandler())

			svc := &types.ExternalService{
				Kind: extsvc.KindGitHub,
				Config: marshalJSON(t, &schema.GitHubConnection{
					Url: "https://github.com",
				}),
			}

			githubSrc, err := NewGithubSource(svc, cf)
			if err != nil {
				t.Fatal(err)
			}

			repo, err := githubSrc.GetRepo(context.Background(), tc.nameWithOwner)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if tc.assert != nil {
				tc.assert(t, repo)
			}
		})
	}
}

func TestGithubSource_makeRepo(t *testing.T) {
	b, err := ioutil.ReadFile(filepath.Join("testdata", "github-repos.json"))
	if err != nil {
		t.Fatal(err)
	}
	var repos []*github.Repository
	if err := json.Unmarshal(b, &repos); err != nil {
		t.Fatal(err)
	}

	svc := types.ExternalService{ID: 1, Kind: extsvc.KindGitHub}

	tests := []struct {
		name   string
		schmea *schema.GitHubConnection
	}{
		{
			name: "simple",
			schmea: &schema.GitHubConnection{
				Url: "https://github.com",
			},
		}, {
			name: "ssh",
			schmea: &schema.GitHubConnection{
				Url:        "https://github.com",
				GitURLType: "ssh",
			},
		}, {
			name: "path-pattern",
			schmea: &schema.GitHubConnection{
				Url:                   "https://github.com",
				RepositoryPathPattern: "gh/{nameWithOwner}",
			},
		},
	}
	for _, test := range tests {
		test.name = "GithubSource_makeRepo_" + test.name
		t.Run(test.name, func(t *testing.T) {
			lg := log15.New()
			lg.SetHandler(log15.DiscardHandler())

			s, err := newGithubSource(&svc, test.schmea, nil)
			if err != nil {
				t.Fatal(err)
			}

			var got []*types.Repo
			for _, r := range repos {
				got = append(got, s.makeRepo(r))
			}

			testutil.AssertGolden(t, "testdata/golden/"+test.name, update(test.name), got)
		})
	}
}

func TestMatchOrg(t *testing.T) {
	testCases := map[string]string{
		"":                     "",
		"org:":                 "",
		"org:gorilla":          "gorilla",
		"org:golang-migrate":   "golang-migrate",
		"org:sourcegraph-":     "",
		"org:source--graph":    "",
		"org: sourcegraph":     "",
		"org:$ourcegr@ph":      "",
		"sourcegraph":          "",
		"org:-sourcegraph":     "",
		"org:source graph":     "",
		"org:source org:graph": "",
		"org:SOURCEGRAPH":      "SOURCEGRAPH",
		"org:Game-club-3d-game-birds-gameapp-makerCo":  "Game-club-3d-game-birds-gameapp-makerCo",
		"org:thisorgnameisfartoolongtomatchthisregexp": "",
	}

	for str, want := range testCases {
		if got := matchOrg(str); got != want {
			t.Errorf("error:\nhave: %s\nwant: %s", got, want)
		}
	}
}

func TestGithubSource_ListRepos(t *testing.T) {
	assertAllReposListed := func(want []string) types.ReposAssertion {
		return func(t testing.TB, rs types.Repos) {
			t.Helper()

			have := rs.Names()
			sort.Strings(have)
			sort.Strings(want)

			if !reflect.DeepEqual(have, want) {
				t.Error(cmp.Diff(have, want))
			}
		}
	}

	testCases := []struct {
		name   string
		assert types.ReposAssertion
		mw     httpcli.Middleware
		conf   *schema.GitHubConnection
		err    string
	}{
		{
			name: "found",
			assert: assertAllReposListed([]string{
				"github.com/sourcegraph/about",
				"github.com/sourcegraph/sourcegraph",
			}),
			conf: &schema.GitHubConnection{
				Url:   "https://github.com",
				Token: os.Getenv("GITHUB_ACCESS_TOKEN"),
				Repos: []string{
					"sourcegraph/about",
					"sourcegraph/sourcegraph",
				},
			},
			err: "<nil>",
		},
		{
			name: "graphql fallback",
			mw:   githubGraphQLFailureMiddleware,
			assert: assertAllReposListed([]string{
				"github.com/sourcegraph/about",
				"github.com/sourcegraph/sourcegraph",
			}),
			conf: &schema.GitHubConnection{
				Url:   "https://github.com",
				Token: os.Getenv("GITHUB_ACCESS_TOKEN"),
				Repos: []string{
					"sourcegraph/about",
					"sourcegraph/sourcegraph",
				},
			},
			err: "<nil>",
		},
		{
			name: "orgs",
			assert: assertAllReposListed([]string{
				"github.com/gorilla/websocket",
				"github.com/gorilla/handlers",
				"github.com/gorilla/mux",
				"github.com/gorilla/feeds",
				"github.com/gorilla/sessions",
				"github.com/gorilla/schema",
				"github.com/gorilla/csrf",
				"github.com/gorilla/rpc",
				"github.com/gorilla/pat",
				"github.com/gorilla/css",
				"github.com/gorilla/site",
				"github.com/gorilla/context",
				"github.com/gorilla/securecookie",
				"github.com/gorilla/http",
				"github.com/gorilla/reverse",
				"github.com/gorilla/muxy",
				"github.com/gorilla/i18n",
				"github.com/gorilla/template",
				"github.com/gorilla/.github",
			}),
			conf: &schema.GitHubConnection{
				Url:   "https://github.com",
				Token: os.Getenv("GITHUB_ACCESS_TOKEN"),
				Orgs: []string{
					"gorilla",
				},
			},
			err: "<nil>",
		},
		{
			name: "orgs repository query",
			assert: assertAllReposListed([]string{
				"github.com/gorilla/websocket",
				"github.com/gorilla/handlers",
				"github.com/gorilla/mux",
				"github.com/gorilla/feeds",
				"github.com/gorilla/sessions",
				"github.com/gorilla/schema",
				"github.com/gorilla/csrf",
				"github.com/gorilla/rpc",
				"github.com/gorilla/pat",
				"github.com/gorilla/css",
				"github.com/gorilla/site",
				"github.com/gorilla/context",
				"github.com/gorilla/securecookie",
				"github.com/gorilla/http",
				"github.com/gorilla/reverse",
				"github.com/gorilla/muxy",
				"github.com/gorilla/i18n",
				"github.com/gorilla/template",
				"github.com/gorilla/.github",
				"github.com/golang-migrate/migrate",
				"github.com/torvalds/linux",
				"github.com/torvalds/uemacs",
				"github.com/torvalds/subsurface-for-dirk",
				"github.com/torvalds/libdc-for-dirk",
				"github.com/torvalds/test-tlb",
				"github.com/torvalds/pesconvert",
			}),
			conf: &schema.GitHubConnection{
				Url:   "https://github.com",
				Token: os.Getenv("GITHUB_ACCESS_TOKEN"),
				RepositoryQuery: []string{
					"org:gorilla",
					"org:golang-migrate",
					"org:torvalds",
				},
			},
			err: "<nil>",
		},
	}

	for _, tc := range testCases {
		tc := tc
		tc.name = "GITHUB-LIST-REPOS/" + tc.name
		t.Run(tc.name, func(t *testing.T) {
			// The GithubSource uses the github.Client under the hood, which
			// uses rcache, a caching layer that uses Redis.
			// We need to clear the cache before we run the tests
			rcache.SetupForTest(t)

			var (
				cf   *httpcli.Factory
				save func(testing.TB)
			)
			if tc.mw != nil {
				cf, save = newClientFactory(t, tc.name, tc.mw)
			} else {
				cf, save = newClientFactory(t, tc.name)
			}

			defer save(t)

			lg := log15.New()
			lg.SetHandler(log15.DiscardHandler())

			svc := &types.ExternalService{
				Kind:   extsvc.KindGitHub,
				Config: marshalJSON(t, tc.conf),
			}

			githubSrc, err := NewGithubSource(svc, cf)
			if err != nil {
				t.Fatal(err)
			}

			repos, err := listAll(context.Background(), githubSrc)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if tc.assert != nil {
				tc.assert(t, repos)
			}
		})
	}
}

func githubGraphQLFailureMiddleware(cli httpcli.Doer) httpcli.Doer {
	return httpcli.DoerFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "graphql") {
			return nil, errors.New("graphql request failed")
		}
		return cli.Do(req)
	})
}

func TestGithubSource_WithAuthenticator(t *testing.T) {
	svc := &types.ExternalService{
		Kind: extsvc.KindGitHub,
		Config: marshalJSON(t, &schema.GitHubConnection{
			Url:   "https://github.com",
			Token: os.Getenv("GITHUB_TOKEN"),
		}),
	}

	githubSrc, err := NewGithubSource(svc, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("supported", func(t *testing.T) {
		src, err := githubSrc.WithAuthenticator(&auth.OAuthBearerToken{})
		if err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}

		if gs, ok := src.(*GithubSource); !ok {
			t.Error("cannot coerce Source into GithubSource")
		} else if gs == nil {
			t.Error("unexpected nil Source")
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		for name, tc := range map[string]auth.Authenticator{
			"nil":         nil,
			"BasicAuth":   &auth.BasicAuth{},
			"OAuthClient": &auth.OAuthClient{},
		} {
			t.Run(name, func(t *testing.T) {
				src, err := githubSrc.WithAuthenticator(tc)
				if err == nil {
					t.Error("unexpected nil error")
				} else if _, ok := err.(UnsupportedAuthenticatorError); !ok {
					t.Errorf("unexpected error of type %T: %v", err, err)
				}
				if src != nil {
					t.Errorf("expected non-nil Source: %v", src)
				}
			})
		}
	})
}
