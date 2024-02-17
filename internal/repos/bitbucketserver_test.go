package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/internal/testutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/types/typestest"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestBitbucketServerSource_MakeRepo(t *testing.T) {
	ratelimit.SetupForTest(t)
	repos := GetReposFromTestdata(t, "bitbucketserver-repos.json")

	cases := map[string]*schema.BitbucketServerConnection{
		"simple": {
			Url:   "bitbucket.example.com",
			Token: "secret",
		},
		"ssh": {
			Url:                         "https://bitbucket.example.com",
			Token:                       "secret",
			InitialRepositoryEnablement: true,
			GitURLType:                  "ssh",
		},
		"path-pattern": {
			Url:                   "https://bitbucket.example.com",
			Token:                 "secret",
			RepositoryPathPattern: "bb/{projectKey}/{repositorySlug}",
		},
		"username": {
			Url:                   "https://bitbucket.example.com",
			Username:              "foo",
			Token:                 "secret",
			RepositoryPathPattern: "bb/{projectKey}/{repositorySlug}",
		},
	}

	svc := types.ExternalService{
		ID:     1,
		Kind:   extsvc.KindBitbucketServer,
		Config: extsvc.NewEmptyConfig(),
	}

	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			// httpcli uses rcache, so we need to prepare the redis connection.
			rcache.SetupForTest(t)

			s, err := newBitbucketServerSource(logtest.Scoped(t), &svc, config, httpcli.NewFactory(nil))
			if err != nil {
				t.Fatal(err)
			}

			var got []*types.Repo
			for _, r := range repos {
				got = append(got, s.makeRepo(r, false))
			}

			path := filepath.Join("testdata", "bitbucketserver-repos-"+name+".golden")
			testutil.AssertGolden(t, path, Update(name), got)
		})
	}
}

func TestBitbucketServerSource_Exclude(t *testing.T) {
	ratelimit.SetupForTest(t)
	b, err := os.ReadFile(filepath.Join("testdata", "bitbucketserver-repos.json"))
	if err != nil {
		t.Fatal(err)
	}
	var repos []*bitbucketserver.Repo
	if err := json.Unmarshal(b, &repos); err != nil {
		t.Fatal(err)
	}

	cases := map[string]*schema.BitbucketServerConnection{
		"none": {
			Url:   "https://bitbucket.example.com",
			Token: "secret",
		},
		"name": {
			Url:   "https://bitbucket.example.com",
			Token: "secret",
			Exclude: []*schema.ExcludedBitbucketServerRepo{{
				Name: "SG/python-langserver-fork",
			}, {
				Name: "~KEEGAN/rgp",
			}},
		},
		"id": {
			Url:     "https://bitbucket.example.com",
			Token:   "secret",
			Exclude: []*schema.ExcludedBitbucketServerRepo{{Id: 4}},
		},
		"pattern": {
			Url:   "https://bitbucket.example.com",
			Token: "secret",
			Exclude: []*schema.ExcludedBitbucketServerRepo{{
				Pattern: "SG/python.*",
			}, {
				Pattern: "~KEEGAN/.*",
			}},
		},
		"both": {
			Url:   "https://bitbucket.example.com",
			Token: "secret",
			// We match on the bitbucket server repo name, not the repository path pattern.
			RepositoryPathPattern: "bb/{projectKey}/{repositorySlug}",
			Exclude: []*schema.ExcludedBitbucketServerRepo{{
				Id: 1,
			}, {
				Name: "~KEEGAN/rgp",
			}, {
				Pattern: ".*-fork",
			}},
		},
	}

	svc := types.ExternalService{
		ID:     1,
		Kind:   extsvc.KindBitbucketServer,
		Config: extsvc.NewEmptyConfig(),
	}

	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			// httpcli uses rcache, so we need to prepare the redis connection.
			rcache.SetupForTest(t)

			s, err := newBitbucketServerSource(logtest.Scoped(t), &svc, config, httpcli.NewFactory(nil))
			if err != nil {
				t.Fatal(err)
			}

			type output struct {
				Include []string
				Exclude []string
			}
			var got output
			for _, r := range repos {
				name := r.Slug
				if r.Project != nil {
					name = r.Project.Key + "/" + name
				}
				if s.excludes(r) {
					got.Exclude = append(got.Exclude, name)
				} else {
					got.Include = append(got.Include, name)
				}
			}

			path := filepath.Join("testdata", "bitbucketserver-repos-exclude-"+name+".golden")
			testutil.AssertGolden(t, path, Update(name), got)
		})
	}
}

func TestBitbucketServerSource_WithAuthenticator(t *testing.T) {
	// httpcli uses rcache, so we need to prepare the redis connection.
	rcache.SetupForTest(t)
	ratelimit.SetupForTest(t)

	svc := typestest.MakeExternalService(t,
		extsvc.VariantBitbucketServer,
		&schema.BitbucketServerConnection{
			Url:   "https://bitbucket.sgdev.org",
			Token: os.Getenv("BITBUCKET_SERVER_TOKEN"),
		})

	ctx := context.Background()
	bbsSrc, err := NewBitbucketServerSource(ctx, logtest.Scoped(t), svc, httpcli.NewFactory(nil))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("supported", func(t *testing.T) {
		for name, tc := range map[string]auth.Authenticator{
			"BasicAuth":           &auth.BasicAuth{},
			"OAuthBearerToken":    &auth.OAuthBearerToken{},
			"SudoableOAuthClient": &bitbucketserver.SudoableOAuthClient{},
		} {
			t.Run(name, func(t *testing.T) {
				src, err := bbsSrc.WithAuthenticator(tc)
				if err != nil {
					t.Errorf("unexpected non-nil error: %v", err)
				}

				if gs, ok := src.(*BitbucketServerSource); !ok {
					t.Error("cannot coerce Source into bbsSource")
				} else if gs == nil {
					t.Error("unexpected nil Source")
				}
			})
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		for name, tc := range map[string]auth.Authenticator{
			"nil":         nil,
			"OAuthClient": &auth.OAuthClient{},
		} {
			t.Run(name, func(t *testing.T) {
				src, err := bbsSrc.WithAuthenticator(tc)
				if err == nil {
					t.Error("unexpected nil error")
				} else if !errors.HasType(err, UnsupportedAuthenticatorError{}) {
					t.Errorf("unexpected error of type %T: %v", err, err)
				}
				if src != nil {
					t.Errorf("expected non-nil Source: %v", src)
				}
			})
		}
	})
}

func TestBitbucketServerSource_ListByReposOnly(t *testing.T) {
	ratelimit.SetupForTest(t)
	repos := GetReposFromTestdata(t, "bitbucketserver-repos.json")

	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/1.0/projects/", func(w http.ResponseWriter, r *http.Request) {
		pathArr := strings.Split(r.URL.Path, "/")
		projectKey := pathArr[5]
		repoSlug := pathArr[7]

		for _, repo := range repos {
			if repo.Project.Key == projectKey && repo.Slug == repoSlug {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				json.NewEncoder(w).Encode(repo)
			}
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cases, svc := GetConfig(t, server.URL, "secret")
	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			// httpcli uses rcache, so we need to prepare the redis connection.
			rcache.SetupForTest(t)

			s, err := newBitbucketServerSource(logtest.Scoped(t), &svc, config, httpcli.NewFactory(nil))
			if err != nil {
				t.Fatal(err)
			}

			s.config.Repos = []string{
				"SG/go-langserver",
				"SG/python-langserver",
				"SG/python-langserver-fork",
				"~KEEGAN/rgp",
				"~KEEGAN/rgp-unavailable",
			}

			ctxWithTimeout, cancelFunction := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelFunction()

			results := make(chan SourceResult, 10)
			defer close(results)

			s.ListRepos(ctxWithTimeout, results)
			VerifyData(t, ctxWithTimeout, 4, results)
		})
	}
}

func TestBitbucketServerSource_ListByRepositoryQuery(t *testing.T) {
	ratelimit.SetupForTest(t)
	repos := GetReposFromTestdata(t, "bitbucketserver-repos.json")

	type Results struct {
		*bitbucketserver.PageToken
		Values any `json:"values"`
	}

	pageToken := bitbucketserver.PageToken{
		Size:          1,
		Limit:         1000,
		IsLastPage:    true,
		Start:         1,
		NextPageStart: 1,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/1.0/repos", func(w http.ResponseWriter, r *http.Request) {
		projectName := r.URL.Query().Get("projectName")

		if projectName == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(Results{
				PageToken: &pageToken,
				Values:    repos,
			})
		} else {
			for _, repo := range repos {
				if projectName == repo.Name {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					json.NewEncoder(w).Encode(Results{
						PageToken: &pageToken,
						Values:    []*bitbucketserver.Repo{repo},
					})
				}
			}
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cases, svc := GetConfig(t, server.URL, "secret")

	tcs := []struct {
		queries []string
		exp     int
	}{
		{
			[]string{
				"?projectName=go-langserver",
				"?projectName=python-langserver",
				"?projectName=python-langserver-fork",
				"?projectName=rgp",
				"?projectName=rgp-unavailable",
			},
			4,
		},
		{
			[]string{
				"all",
			},
			4,
		},
		{
			[]string{
				"none",
			},
			0,
		},
	}

	for _, tc := range tcs {
		tc := tc
		for name, config := range cases {
			t.Run(name, func(t *testing.T) {
				// httpcli uses rcache, so we need to prepare the redis connection.
				rcache.SetupForTest(t)

				s, err := newBitbucketServerSource(logtest.Scoped(t), &svc, config, httpcli.NewFactory(nil))
				if err != nil {
					t.Fatal(err)
				}

				s.config.RepositoryQuery = tc.queries

				ctxWithTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				results := make(chan SourceResult, 10)
				defer close(results)

				s.ListRepos(ctxWithTimeout, results)
				VerifyData(t, ctxWithTimeout, tc.exp, results)
			})
		}
	}

}

func TestBitbucketServerSource_ListByProjectKeyMock(t *testing.T) {
	ratelimit.SetupForTest(t)
	repos := GetReposFromTestdata(t, "bitbucketserver-repos.json")

	type Results struct {
		*bitbucketserver.PageToken
		Values any `json:"values"`
	}

	pageToken := bitbucketserver.PageToken{
		Size:          1,
		Limit:         1000,
		IsLastPage:    true,
		Start:         1,
		NextPageStart: 1,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/1.0/projects/", func(w http.ResponseWriter, r *http.Request) {
		pathArr := strings.Split(r.URL.Path, "/")
		projectKey := pathArr[5]
		values := make([]*bitbucketserver.Repo, 0)

		for _, repo := range repos {
			if repo.Project.Key == projectKey {
				values = append(values, repo)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Results{
			PageToken: &pageToken,
			Values:    values,
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	cases, svc := GetConfig(t, server.URL, "secret")
	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			// httpcli uses rcache, so we need to prepare the redis connection.
			rcache.SetupForTest(t)

			s, err := newBitbucketServerSource(logtest.Scoped(t), &svc, config, httpcli.NewFactory(nil))
			if err != nil {
				t.Fatal(err)
			}

			s.config.ProjectKeys = []string{
				"SG",
				"~KEEGAN",
			}

			ctxWithTimeout, cancelFunction := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelFunction()

			results := make(chan SourceResult, 20)
			defer close(results)

			s.ListRepos(ctxWithTimeout, results)
			VerifyData(t, ctxWithTimeout, 4, results)
		})
	}
}

func TestBitbucketServerSource_ListByProjectKeyAuthentic(t *testing.T) {
	ratelimit.SetupForTest(t)
	url := "https://bitbucket.sgdev.org"
	token := os.Getenv("BITBUCKET_SERVER_TOKEN")

	cases, svc := GetConfig(t, url, token)

	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			// httpcli uses rcache, so we need to prepare the redis connection.
			rcache.SetupForTest(t)

			s, err := newBitbucketServerSource(logtest.Scoped(t), &svc, config, httpcli.NewFactory(nil))
			if err != nil {
				t.Fatal(err)
			}
			cli := bitbucketserver.NewTestClient(t, name, Update(name))
			s.client = cli

			// This project has 2 repositories in it. that's why we expect 2
			// repos further down.
			// As soon as more repositories are added to the
			// "SOURCEGRAPH" project, we need to update this condition.
			wantNumRepos := 2
			s.config.ProjectKeys = []string{
				"SOURCEGRAPH",
			}

			ctxWithTimeOut, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			results := make(chan SourceResult, 5)
			defer close(results)

			s.ListRepos(ctxWithTimeOut, results)

			var got []*types.Repo

			for i := 0; i < wantNumRepos; i++ {
				select {
				case res := <-results:
					got = append(got, res.Repo)
				case <-ctxWithTimeOut.Done():
					t.Fatalf("timeout! expected %d repos, but so far only got %d", wantNumRepos, len(got))
				}
			}

			path := filepath.Join("testdata/authentic", "bitbucketserver-repos-"+name+".golden")
			testutil.AssertGolden(t, path, Update(name), got)
		})
	}

}

func GetReposFromTestdata(t *testing.T, filename string) []*bitbucketserver.Repo {
	b, err := os.ReadFile(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatal(err)
	}

	var repos []*bitbucketserver.Repo
	if err := json.Unmarshal(b, &repos); err != nil {
		t.Fatal(err)
	}

	return repos
}

func GetConfig(t *testing.T, serverUrl string, token string) (map[string]*schema.BitbucketServerConnection, types.ExternalService) {
	cases := map[string]*schema.BitbucketServerConnection{
		"simple": {
			Url:   serverUrl,
			Token: token,
		},
		"ssh": {
			Url:                         serverUrl,
			Token:                       token,
			InitialRepositoryEnablement: true,
			GitURLType:                  "ssh",
		},
		"path-pattern": {
			Url:                   serverUrl,
			Token:                 token,
			RepositoryPathPattern: "bb/{projectKey}/{repositorySlug}",
		},
		"username": {
			Url:                   serverUrl,
			Username:              "foo",
			Token:                 token,
			RepositoryPathPattern: "bb/{projectKey}/{repositorySlug}",
		},
	}

	svc := types.ExternalService{
		ID:     1,
		Kind:   extsvc.KindBitbucketServer,
		Config: extsvc.NewEmptyConfig(),
	}

	return cases, svc
}

func VerifyData(t *testing.T, ctx context.Context, numExpectedResults int, results chan SourceResult) {
	numTotalResults := len(results)
	numReceivedFromResults := 0

	if numTotalResults != numExpectedResults {
		fmt.Println("numTotalResults:", numTotalResults, ", numExpectedResults:", numExpectedResults)
		t.Fatal(errors.New("wrong number of results"))
	}

	repoNameMap := map[string]struct{}{
		"SG/go-langserver":          {},
		"SG/python-langserver":      {},
		"SG/python-langserver-fork": {},
		"~KEEGAN/rgp":               {},
		"~KEEGAN/rgp-unavailable":   {},
		"SOURCEGRAPH/jsonrpc2":      {},
	}

	for {
		select {
		case res := <-results:
			repoNameArr := strings.Split(string(res.Repo.Name), "/")
			repoName := repoNameArr[1] + "/" + repoNameArr[2]
			if _, ok := repoNameMap[repoName]; ok {
				numReceivedFromResults++
			} else {
				t.Fatal(errors.New("wrong repo returned"))
			}
		case <-ctx.Done():
			t.Fatal(errors.New("timeout!"))
		default:
			if numReceivedFromResults == numExpectedResults {
				return
			}
		}
	}
}
