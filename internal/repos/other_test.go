package repos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestSrcExpose_SrcExposeServer(t *testing.T) {
	var body string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/list-repos" {
			http.Error(w, r.URL.String()+" not found", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	defer s.Close()

	cases := []struct {
		name string
		body string
		want []*types.Repo
		err  string
	}{{
		name: "error",
		body: "boom",
		err:  "failed to decode response from src-expose: boom",
	}, {
		name: "nouri",
		body: `{"Items":[{"name": "foo"}]}`,
		err:  "repo without URI",
	}, {
		name: "empty",
		body: `{"items":[]}`,
		want: []*types.Repo{},
	}, {
		name: "minimal",
		body: `{"Items":[{"uri": "/repos/foo", "clonePath":"/repos/foo/.git"},{"uri":"/repos/bar/baz", "clonePath":"/repos/bar/baz/.git"}]}`,
		want: []*types.Repo{{
			URI:  "/repos/foo",
			Name: "/repos/foo",
			ExternalRepo: api.ExternalRepoSpec{
				ServiceID:   s.URL,
				ServiceType: extsvc.TypeOther,
				ID:          "/repos/foo",
			},
			Sources: map[string]*types.SourceInfo{
				"extsvc:other:1": {
					ID:       "extsvc:other:1",
					CloneURL: s.URL + "/repos/foo/.git",
				},
			},
			Metadata: &extsvc.OtherRepoMetadata{RelativePath: "/repos/foo/.git"},
			Private:  true,
		}, {
			URI:  "/repos/bar/baz",
			Name: "/repos/bar/baz",
			ExternalRepo: api.ExternalRepoSpec{
				ServiceID:   s.URL,
				ServiceType: extsvc.TypeOther,
				ID:          "/repos/bar/baz",
			},
			Sources: map[string]*types.SourceInfo{
				"extsvc:other:1": {
					ID:       "extsvc:other:1",
					CloneURL: s.URL + "/repos/bar/baz/.git",
				},
			},
			Metadata: &extsvc.OtherRepoMetadata{RelativePath: "/repos/bar/baz/.git"},
			Private:  true,
		}},
	}, {
		name: "abs-file-path",
		body: `{"Items":[{"uri": "/repos/foo", "clonePath":"/repos/foo/.git", "AbsFilePath": "/src/foo"}]}`,
		want: []*types.Repo{{
			URI:  "/repos/foo",
			Name: "/repos/foo",
			ExternalRepo: api.ExternalRepoSpec{
				ServiceID:   s.URL,
				ServiceType: extsvc.TypeOther,
				ID:          "/repos/foo",
			},
			Sources: map[string]*types.SourceInfo{
				"extsvc:other:1": {
					ID:       "extsvc:other:1",
					CloneURL: s.URL + "/repos/foo/.git",
				},
			},
			Metadata: &extsvc.OtherRepoMetadata{
				RelativePath: "/repos/foo/.git",
				AbsFilePath:  "/src/foo",
			},
			Private: true,
		}},
	}, {
		name: "override",
		body: `{"Items":[{"uri": "/repos/foo", "name": "foo", "description": "hi", "clonePath":"/repos/foo/.git"}]}`,
		want: []*types.Repo{{
			URI:         "/repos/foo",
			Name:        "foo",
			Description: "",
			ExternalRepo: api.ExternalRepoSpec{
				ServiceID:   s.URL,
				ServiceType: extsvc.TypeOther,
				ID:          "/repos/foo",
			},
			Sources: map[string]*types.SourceInfo{
				"extsvc:other:1": {
					ID:       "extsvc:other:1",
					CloneURL: s.URL + "/repos/foo/.git",
				},
			},
			Metadata: &extsvc.OtherRepoMetadata{RelativePath: "/repos/foo/.git"},
			Private:  true,
		}},
	}, {
		name: "immutable",
		body: `{"Items":[{"uri": "/repos/foo", "clonePath":"/repos/foo/.git", "enabled": false, "externalrepo": {"serviceid": "x", "servicetype": "y", "id": "z"}, "sources": {"x":{"id":"x", "cloneurl":"y"}}}]}`,
		want: []*types.Repo{{
			URI:  "/repos/foo",
			Name: "/repos/foo",
			ExternalRepo: api.ExternalRepoSpec{
				ServiceID:   s.URL,
				ServiceType: extsvc.TypeOther,
				ID:          "/repos/foo",
			},
			Sources: map[string]*types.SourceInfo{
				"extsvc:other:1": {
					ID:       "extsvc:other:1",
					CloneURL: s.URL + "/repos/foo/.git",
				},
			},
			Metadata: &extsvc.OtherRepoMetadata{RelativePath: "/repos/foo/.git"},
			Private:  true,
		}},
	}}

	ctx := context.Background()
	source, err := NewOtherSource(ctx, &types.ExternalService{
		ID:     1,
		Kind:   extsvc.KindOther,
		Config: extsvc.NewUnencryptedConfig(fmt.Sprintf(`{"url": %q, "repos": ["%s"]}`, s.URL, "src-expose")),
	}, httpcli.TestExternalClientFactory, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body = tc.body

			repos, validSrcExposeConfiguration, err := source.srcExpose(context.Background())
			if got := fmt.Sprintf("%v", err); !strings.Contains(got, tc.err) {
				t.Fatalf("got error %v, want %v", got, tc.err)
			}
			if !validSrcExposeConfiguration {
				t.Fatal("other source configuration is expected to be compatible with srcExpose requirements")
			}
			if !reflect.DeepEqual(repos, tc.want) {
				t.Fatal("unexpected repos", cmp.Diff(tc.want, repos))
			}
		})
	}
}

func TestOther_DotComConfig(t *testing.T) {
	makeSource := func(t *testing.T) *OtherSource {
		source, err := NewOtherSource(context.Background(), &types.ExternalService{
			ID:     1,
			Kind:   extsvc.KindOther,
			Config: extsvc.NewUnencryptedConfig(fmt.Sprintf(`{"url": "somegit.com/repo", "repos": ["%s"], "makeReposPublicOnDotCom": true}`, "src-expose")),
		}, nil, nil)
		require.NoError(t, err)
		return source
	}
	source := makeSource(t)

	cloneURL, _ := url.Parse("https://somegit.com/repo")

	// Not on Dotcom, so repo should still be private regardless of config
	repo, err := source.otherRepoFromCloneURL("other:source", cloneURL)
	require.NoError(t, err)
	require.True(t, repo.Private)

	// Enable Dotcom mode. Then repo should be public.
	dotcom.MockSourcegraphDotComMode(t, true)

	source = makeSource(t)

	repo, err = source.otherRepoFromCloneURL("other:source", cloneURL)
	require.NoError(t, err)
	require.False(t, repo.Private)
}

func TestOther_ListRepos(t *testing.T) {
	// We don't test on the details of what we marshal, instead we just write
	// some tests based on the repo names that are returned.

	// Spin up a src-expose server
	var srcExposeRepos []string
	srcExpose := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/list-repos" {
			http.Error(w, r.URL.String()+" not found", http.StatusNotFound)
			return
		}
		var items []srcExposeItem
		for _, name := range srcExposeRepos {
			items = append(items, srcExposeItem{
				URI:       "repos/" + name,
				Name:      name,
				ClonePath: "repos/" + name + ".git",
			})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"Items": items})
	}))
	defer srcExpose.Close()

	cases := []struct {
		Name           string
		Conn           *schema.OtherExternalServiceConnection
		SrcExposeRepos []string
		Want           []string
	}{{
		Name: "src-expose/simple",
		Conn: &schema.OtherExternalServiceConnection{
			Url:   srcExpose.URL,
			Repos: []string{"src-expose"},
		},
		SrcExposeRepos: []string{"a", "b/c", "d"},
		Want:           []string{"a", "b/c", "d"},
	}, {
		Name: "static/simple",
		Conn: &schema.OtherExternalServiceConnection{
			Url:   "http://test",
			Repos: []string{"a", "b/c", "d"},
		},
		Want: []string{"test/a", "test/b/c", "test/d"},
	}, {
		// Pattern is ignored for src-expose
		Name: "src-expose/pattern",
		Conn: &schema.OtherExternalServiceConnection{
			Url:                   srcExpose.URL,
			Repos:                 []string{"src-expose"},
			RepositoryPathPattern: "pre-{repo}",
		},
		SrcExposeRepos: []string{"a", "b/c", "d"},
		Want:           []string{"a", "b/c", "d"},
	}, {
		Name: "static/pattern",
		Conn: &schema.OtherExternalServiceConnection{
			Url:                   "http://test",
			Repos:                 []string{"a", "b/c", "d"},
			RepositoryPathPattern: "pre-{repo}",
		},
		Want: []string{"pre-a", "pre-b/c", "pre-d"},
	}, {
		Name: "src-expose/exclude",
		Conn: &schema.OtherExternalServiceConnection{
			Url:                   srcExpose.URL,
			Repos:                 []string{"src-expose"},
			Exclude:               []*schema.ExcludedOtherRepo{{Name: "not-exact"}, {Name: "exclude/exact"}, {Pattern: "exclude-dir"}},
			RepositoryPathPattern: "pre-{repo}",
		},
		SrcExposeRepos: []string{"keep1", "not-exact/keep2", "exclude-dir/a", "exclude-dir/b", "exclude/exact", "keep3"},
		Want:           []string{"keep1", "not-exact/keep2", "keep3"},
	}, {
		Name: "static/pattern",
		Conn: &schema.OtherExternalServiceConnection{
			Url:                   "http://test",
			Repos:                 []string{"keep1", "not-exact/keep2", "exclude-dir/a", "exclude-dir/b", "exclude/exact", "keep3"},
			Exclude:               []*schema.ExcludedOtherRepo{{Name: "not-exact"}, {Name: "exclude/exact"}, {Pattern: "exclude-dir"}},
			RepositoryPathPattern: "{repo}",
		},
		Want: []string{"keep1", "not-exact/keep2", "keep3"},
	}}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			// need to do this so our test server can marshal the repos
			srcExposeRepos = tc.SrcExposeRepos

			config, err := json.Marshal(tc.Conn)
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			source, err := NewOtherSource(ctx, &types.ExternalService{
				ID:     1,
				Kind:   extsvc.KindOther,
				Config: extsvc.NewUnencryptedConfig(string(config)),
			}, httpcli.NewFactory(httpcli.NewMiddleware()), logtest.Scoped(t))
			if err != nil {
				t.Fatal(err)
			}

			results := make(chan SourceResult)
			go func() {
				defer close(results)
				source.ListRepos(ctx, results)
			}()

			var got []string
			for r := range results {
				if r.Err != nil {
					t.Error(r.Err)
				} else {
					got = append(got, string(r.Repo.Name))
				}
			}

			if d := cmp.Diff(tc.Want, got); d != "" {
				t.Fatalf("unexpected repos (-want, +got):\n%s", d)
			}
		})
	}
}

type srcExposeRequestBody struct {
	Root string `json:"root"`
}

func TestOther_SrcExposeRequest(t *testing.T) {
	cases := []struct {
		Name           string
		Conn           *schema.OtherExternalServiceConnection
		ValidRequest   bool
		Method         string
		ValidSrcExpose bool
		Body           srcExposeRequestBody
	}{{
		Name: "src-expose",
		Conn: &schema.OtherExternalServiceConnection{
			Repos: []string{"src-expose"},
		},
		ValidRequest:   true,
		Method:         http.MethodGet,
		ValidSrcExpose: true,
	}, {
		Name: "src-serve",
		Conn: &schema.OtherExternalServiceConnection{
			Repos: []string{"src-serve"},
		},
		ValidRequest:   true,
		Method:         http.MethodGet,
		ValidSrcExpose: true,
	}, {
		Name: "invalid src-expose",
		Conn: &schema.OtherExternalServiceConnection{
			Repos: []string{"myrepo"},
		},
		ValidRequest:   false,
		Method:         http.MethodGet,
		ValidSrcExpose: false,
	},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			config, err := json.Marshal(tc.Conn)
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			source, err := NewOtherSource(ctx, &types.ExternalService{
				ID:     1,
				Kind:   extsvc.KindOther,
				Config: extsvc.NewUnencryptedConfig(string(config)),
			}, httpcli.NewFactory(httpcli.NewMiddleware()), logtest.Scoped(t))
			if err != nil {
				t.Fatal(err)
			}

			req, validSrcExposeConfig, err := source.srcExposeRequest()
			if err != nil {
				t.Fatal(err)
			}

			if validSrcExposeConfig != tc.ValidSrcExpose {
				t.Fatalf("got validSrcExposeConfig %v, want %v", validSrcExposeConfig, tc.ValidSrcExpose)
			}

			if tc.ValidRequest {
				if req == nil {
					t.Fatalf("expected non-nil request")
				}

				if req.Method != tc.Method {
					t.Fatalf("got http method %v, want %v", req.Method, tc.Method)
				}

				if req.Body != nil {
					var gotReqBody srcExposeRequestBody
					if err := json.NewDecoder(req.Body).Decode(&gotReqBody); err != nil {
						t.Fatalf("error returned by Decode: %s", err.Error())
					}

					if gotReqBody.Root != tc.Body.Root {
						t.Fatalf("got request body root property %v, want %v", gotReqBody.Root, tc.Body.Root)
					}

					if d := cmp.Diff(tc.Body, gotReqBody); d != "" {
						t.Fatalf("unexpected repos (-want, +got):\n%s", d)
					}
				}
			}
		})
	}
}
