package backend

import (
	"fmt"
	"testing"
	"testing/quick"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"
	"github.com/sourcegraph/zoekt"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/ctags_config"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestZoektIndexOptions_RoundTrip(t *testing.T) {
	var diff string
	f := func(original ZoektIndexOptions) bool {

		var converted ZoektIndexOptions
		converted.FromProto(original.ToProto())

		if diff = cmp.Diff(original, converted); diff != "" {
			return false
		}
		return true
	}

	if err := quick.Check(f, nil); err != nil {
		t.Errorf("ZoektIndexOptions diff (-want +got):\n%s", diff)
	}
}

func TestGetIndexOptions(t *testing.T) {
	const (
		REPO = api.RepoID(iota + 1)
		FOO
		NOT_IN_VERSION_CONTEXT
		PRIORITY
		PUBLIC
		FORK
		ARCHIVED
		RANKED
	)

	name := func(repo api.RepoID) string {
		return fmt.Sprintf("repo-%.2d", repo)
	}

	withBranches := func(c schema.SiteConfiguration, repo api.RepoID, branches ...string) schema.SiteConfiguration {
		if c.ExperimentalFeatures == nil {
			c.ExperimentalFeatures = &schema.ExperimentalFeatures{}
		}
		if c.ExperimentalFeatures.SearchIndexBranches == nil {
			c.ExperimentalFeatures.SearchIndexBranches = map[string][]string{}
		}
		b := c.ExperimentalFeatures.SearchIndexBranches
		b[name(repo)] = append(b[name(repo)], branches...)
		return c
	}

	type caseT struct {
		name              string
		conf              schema.SiteConfiguration
		searchContextRevs []string
		repo              api.RepoID
		want              ZoektIndexOptions
	}

	cases := []caseT{{
		name: "default",
		conf: schema.SiteConfiguration{},
		repo: REPO,
		want: ZoektIndexOptions{
			RepoID:  1,
			Name:    "repo-01",
			Symbols: true,
			Branches: []zoekt.RepositoryBranch{
				{Name: "HEAD", Version: "!HEAD"},
			},
			LanguageMap: ctags_config.DefaultEngines,
		},
	}, {
		name: "public",
		conf: schema.SiteConfiguration{},
		repo: PUBLIC,
		want: ZoektIndexOptions{
			RepoID:  5,
			Name:    "repo-05",
			Public:  true,
			Symbols: true,
			Branches: []zoekt.RepositoryBranch{
				{Name: "HEAD", Version: "!HEAD"},
			},
			LanguageMap: ctags_config.DefaultEngines,
		},
	}, {
		name: "fork",
		conf: schema.SiteConfiguration{},
		repo: FORK,
		want: ZoektIndexOptions{
			RepoID:  6,
			Name:    "repo-06",
			Fork:    true,
			Symbols: true,
			Branches: []zoekt.RepositoryBranch{
				{Name: "HEAD", Version: "!HEAD"},
			},
			LanguageMap: ctags_config.DefaultEngines,
		},
	}, {
		name: "archived",
		conf: schema.SiteConfiguration{},
		repo: ARCHIVED,
		want: ZoektIndexOptions{
			RepoID:   7,
			Name:     "repo-07",
			Archived: true,
			Symbols:  true,
			Branches: []zoekt.RepositoryBranch{
				{Name: "HEAD", Version: "!HEAD"},
			},
			LanguageMap: ctags_config.DefaultEngines,
		},
	}, {
		name: "nosymbols",
		conf: schema.SiteConfiguration{
			SearchIndexSymbolsEnabled: pointers.Ptr(false),
		},
		repo: REPO,
		want: ZoektIndexOptions{
			RepoID: 1,
			Name:   "repo-01",
			Branches: []zoekt.RepositoryBranch{
				{Name: "HEAD", Version: "!HEAD"},
			},
			LanguageMap: ctags_config.DefaultEngines,
		},
	}, {
		name: "largefiles",
		conf: schema.SiteConfiguration{
			SearchLargeFiles: []string{"**/*.jar", "*.bin", "!**/excluded.zip", "\\!included.zip"},
		},
		repo: REPO,
		want: ZoektIndexOptions{
			RepoID:     1,
			Name:       "repo-01",
			Symbols:    true,
			LargeFiles: []string{"**/*.jar", "*.bin", "!**/excluded.zip", "\\!included.zip"},
			Branches: []zoekt.RepositoryBranch{
				{Name: "HEAD", Version: "!HEAD"},
			},
			LanguageMap: ctags_config.DefaultEngines,
		},
	}, {
		name: "conf index branches",
		conf: withBranches(schema.SiteConfiguration{}, REPO, "a", "", "b"),
		repo: REPO,
		want: ZoektIndexOptions{
			RepoID:  1,
			Name:    "repo-01",
			Symbols: true,
			Branches: []zoekt.RepositoryBranch{
				{Name: "HEAD", Version: "!HEAD"},
				{Name: "a", Version: "!a"},
				{Name: "b", Version: "!b"},
			},
			LanguageMap: ctags_config.DefaultEngines,
		},
	}, {
		name: "conf index revisions",
		conf: schema.SiteConfiguration{ExperimentalFeatures: &schema.ExperimentalFeatures{
			SearchIndexRevisions: []*schema.SearchIndexRevisionsRule{
				{Name: "repo-.*", Revisions: []string{"a"}},
			},
		}},
		repo: REPO,
		want: ZoektIndexOptions{
			RepoID:  1,
			Name:    "repo-01",
			Symbols: true,
			Branches: []zoekt.RepositoryBranch{
				{Name: "HEAD", Version: "!HEAD"},
				{Name: "a", Version: "!a"},
			},
			LanguageMap: ctags_config.DefaultEngines,
		},
	}, {
		name: "conf index revisions and branches",
		conf: schema.SiteConfiguration{ExperimentalFeatures: &schema.ExperimentalFeatures{
			SearchIndexBranches: map[string][]string{
				"repo-01": {"a", "b"},
			},
			SearchIndexRevisions: []*schema.SearchIndexRevisionsRule{
				{Name: "repo-.*", Revisions: []string{"a", "c"}},
			},
		}},
		repo: REPO,
		want: ZoektIndexOptions{
			RepoID:  1,
			Name:    "repo-01",
			Symbols: true,
			Branches: []zoekt.RepositoryBranch{
				{Name: "HEAD", Version: "!HEAD"},
				{Name: "a", Version: "!a"},
				{Name: "b", Version: "!b"},
				{Name: "c", Version: "!c"},
			},
			LanguageMap: ctags_config.DefaultEngines,
		},
	}, {
		name:              "with search context revisions",
		conf:              schema.SiteConfiguration{},
		repo:              REPO,
		searchContextRevs: []string{"rev1", "rev2"},
		want: ZoektIndexOptions{
			RepoID:  1,
			Name:    "repo-01",
			Symbols: true,
			Branches: []zoekt.RepositoryBranch{
				{Name: "HEAD", Version: "!HEAD"},
				{Name: "rev1", Version: "!rev1"},
				{Name: "rev2", Version: "!rev2"},
			},
			LanguageMap: ctags_config.DefaultEngines,
		},
	}, {
		name: "with a priority value",
		conf: schema.SiteConfiguration{},
		repo: PRIORITY,
		want: ZoektIndexOptions{
			RepoID:  4,
			Name:    "repo-04",
			Symbols: true,
			Branches: []zoekt.RepositoryBranch{
				{Name: "HEAD", Version: "!HEAD"},
			},
			Priority:    10,
			LanguageMap: ctags_config.DefaultEngines,
		},
	}, {
		name: "with rank",
		conf: schema.SiteConfiguration{},
		repo: RANKED,
		want: ZoektIndexOptions{
			RepoID:  8,
			Name:    "repo-08",
			Symbols: true,
			Branches: []zoekt.RepositoryBranch{
				{Name: "HEAD", Version: "!HEAD"},
			},
			DocumentRanksVersion: "ranked",
			LanguageMap:          ctags_config.DefaultEngines,
		},
	}}

	{
		// Generate case for no more than than 64 branches
		var branches []string
		for i := range 100 {
			branches = append(branches, fmt.Sprintf("%.2d", i))
		}
		want := []zoekt.RepositoryBranch{{Name: "HEAD", Version: "!HEAD"}}
		for i := range 63 {
			want = append(want, zoekt.RepositoryBranch{
				Name:    fmt.Sprintf("%.2d", i),
				Version: fmt.Sprintf("!%.2d", i),
			})
		}
		cases = append(cases, caseT{
			name: "limit branches",
			conf: withBranches(schema.SiteConfiguration{}, REPO, branches...),
			repo: REPO,
			want: ZoektIndexOptions{
				RepoID:      1,
				Name:        "repo-01",
				Symbols:     true,
				Branches:    want,
				LanguageMap: ctags_config.DefaultEngines,
			},
		})
	}

	var getRepoIndexOptions getRepoIndexOptsFn = func(repo api.RepoID) (*RepoIndexOptions, error) {
		var priority float64
		if repo == PRIORITY {
			priority = 10
		}
		var documentRanksVersion string
		if repo == RANKED {
			documentRanksVersion = "ranked"
		}
		return &RepoIndexOptions{
			RepoID:   repo,
			Name:     name(repo),
			Public:   repo == PUBLIC,
			Fork:     repo == FORK,
			Archived: repo == ARCHIVED,
			Priority: priority,
			GetVersion: func(branch string) (string, error) {
				return "!" + branch, nil
			},

			DocumentRanksVersion: documentRanksVersion,
		}, nil
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			getSearchContextRevisions := func(api.RepoID) ([]string, error) { return tc.searchContextRevs, nil }

			got := GetIndexOptions(logtest.Scoped(t), &tc.conf, getRepoIndexOptions, getSearchContextRevisions, tc.repo)

			want := []ZoektIndexOptions{tc.want}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Fatal("mismatch (-want, +got):\n", diff)
			}
		})
	}
}

func TestGetIndexOptions_getVersion(t *testing.T) {
	conf := schema.SiteConfiguration{}
	getSearchContextRevs := func(api.RepoID) ([]string, error) { return []string{"b1", "b2"}, nil }

	boom := errors.New("boom")
	cases := []struct {
		name    string
		f       func(string) (string, error)
		want    []zoekt.RepositoryBranch
		wantErr string
	}{{
		name: "error",
		f: func(_ string) (string, error) {
			return "", boom
		},
		wantErr: "boom",
	}, {
		// no HEAD means we don't index anything. This leads to zoekt having
		// an empty index.
		name: "no HEAD",
		f: func(branch string) (string, error) {
			if branch == "HEAD" {
				return "", nil
			}
			return "!" + branch, nil
		},
		want: nil,
	}, {
		name: "no branch",
		f: func(branch string) (string, error) {
			if branch == "b1" {
				return "", nil
			}
			return "!" + branch, nil
		},
		want: []zoekt.RepositoryBranch{
			{Name: "HEAD", Version: "!HEAD"},
			{Name: "b2", Version: "!b2"},
		},
	}, {
		name: "all",
		f: func(branch string) (string, error) {
			return "!" + branch, nil
		},
		want: []zoekt.RepositoryBranch{
			{Name: "HEAD", Version: "!HEAD"},
			{Name: "b1", Version: "!b1"},
			{Name: "b2", Version: "!b2"},
		},
	}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			getRepoIndexOptions := func(repo api.RepoID) (*RepoIndexOptions, error) {
				return &RepoIndexOptions{
					GetVersion: tc.f,
				}, nil
			}

			resp := GetIndexOptions(logtest.Scoped(t), &conf, getRepoIndexOptions, getSearchContextRevs, 1)
			if len(resp) != 1 {
				t.Fatalf("expected 1 index options returned, got %d", len(resp))
			}

			got := resp[0]
			if got.Error != tc.wantErr {
				t.Fatalf("expected error %v, got index options %+v and error %v", tc.wantErr, got, got.Error)
			}
			if tc.wantErr != "" {
				return
			}

			if diff := cmp.Diff(tc.want, got.Branches); diff != "" {
				t.Fatal("mismatch (-want, +got):\n", diff)
			}
		})
	}
}

func TestGetIndexOptions_batch(t *testing.T) {
	isError := func(repo api.RepoID) bool {
		return repo%20 == 0
	}
	var (
		repos []api.RepoID
		want  []ZoektIndexOptions
	)
	for repo := api.RepoID(1); repo < 100; repo++ {
		repos = append(repos, repo)
		if isError(repo) {
			want = append(want, ZoektIndexOptions{Error: "error"})
		} else {
			want = append(want, ZoektIndexOptions{
				Symbols: true,
				Branches: []zoekt.RepositoryBranch{
					{Name: "HEAD", Version: fmt.Sprintf("!HEAD-%d", repo)},
				},
				LanguageMap: ctags_config.DefaultEngines,
			})
		}
	}
	getRepoIndexOptions := func(repo api.RepoID) (*RepoIndexOptions, error) {
		return &RepoIndexOptions{
			GetVersion: func(branch string) (string, error) {
				if isError(repo) {
					return "", errors.New("error")
				}
				return fmt.Sprintf("!%s-%d", branch, repo), nil
			},
		}, nil
	}

	getSearchContextRevs := func(api.RepoID) ([]string, error) { return nil, nil }

	got := GetIndexOptions(logtest.Scoped(t), &schema.SiteConfiguration{}, getRepoIndexOptions, getSearchContextRevs, repos...)

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatal("mismatch (-want, +got):\n", diff)
	}
}
func TestGetIndexOptions_concurrency(t *testing.T) {
	repos := []api.RepoID{1, 2, 3}
	getRepoIndexOptions := func(repo api.RepoID) (*RepoIndexOptions, error) {
		return &RepoIndexOptions{
			GetVersion: func(branch string) (string, error) {
				return fmt.Sprintf("!%s-%d", branch, repo), nil
			},
		}, nil
	}
	getSearchContextRevs := func(api.RepoID) ([]string, error) { return nil, nil }

	wantConcurrency := 27
	config := &schema.SiteConfiguration{SearchIndexShardConcurrency: wantConcurrency}
	options := GetIndexOptions(logtest.Scoped(t), config, getRepoIndexOptions, getSearchContextRevs, repos...)

	for _, got := range options {
		if wantConcurrency != int(got.ShardConcurrency) {
			t.Fatalf("wrong shard concurrency, want: %d, got: %d", wantConcurrency, got.ShardConcurrency)
		}
	}
}
