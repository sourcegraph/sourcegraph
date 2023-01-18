package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
)

func TestServiceConnections(t *testing.T) {
	os.Setenv("CODEINTEL_PG_ALLOW_SINGLE_DB", "true")

	// We override the URLs so service discovery doesn't try and talk to k8s
	searcherKey := "SEARCHER_URL"
	oldSearcherURL := os.Getenv(searcherKey)
	t.Cleanup(func() { os.Setenv(searcherKey, oldSearcherURL) })
	os.Setenv(searcherKey, "http://searcher:3181")

	indexedKey := "INDEXED_SEARCH_SERVERS"
	oldIndexedSearchServers := os.Getenv(indexedKey)
	t.Cleanup(func() { os.Setenv(indexedKey, oldIndexedSearchServers) })
	os.Setenv(indexedKey, "http://indexed-search:6070")

	// We only test that we get something non-empty back.
	sc := serviceConnections(logtest.Scoped(t))
	if reflect.DeepEqual(sc, conftypes.ServiceConnections{}) {
		t.Fatal("expected non-empty service connections")
	}
}

func TestWriteSiteConfig(t *testing.T) {
	db := database.NewMockDB()
	confStore := database.NewMockConfStore()
	conf := &database.SiteConfig{ID: 1}
	confStore.SiteGetLatestFunc.SetDefaultReturn(
		conf,
		nil,
	)
	logger := logtest.Scoped(t)
	db.ConfFunc.SetDefaultReturn(confStore)
	confSource := newConfigurationSource(logger, db)

	t.Run("error when incorrect last ID", func(t *testing.T) {
		err := confSource.Write(context.Background(), conftypes.RawUnified{}, conf.ID-1)
		assert.Error(t, err)
	})

	t.Run("no error when correct last ID", func(t *testing.T) {
		err := confSource.Write(context.Background(), conftypes.RawUnified{}, conf.ID)
		assert.NoError(t, err)
	})
}

func TestReadSiteConfigFile(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		Name  string
		Files []string
		Want  string
		Err   string
	}{{
		Name:  "one",
		Files: []string{`{"hello": "world"}`},
		Want:  `{"hello": "world"}`,
	}, {
		Name: "two",
		Files: []string{
			`// leading comment
{
  // first comment
  "first": "file",
} // trailing comment
`, `{"second": "file"}`},
		Want: `// merged SITE_CONFIG_FILE
{
  // BEGIN $tmp/0.json
  "first": "file",
  // END $tmp/0.json
  // BEGIN $tmp/1.json
  "second": "file",
  // END $tmp/1.json
}`,
	},
		{
			Name: "three",
			Files: []string{
				`{
    "search.index.branches": {
      "github.com/sourcegraph/sourcegraph": ["3.17", "v3.0.0"],
      "github.com/kubernetes/kubernetes": ["release-1.17"],
      "github.com/go-yaml/yaml": ["v2", "v3"]
    }
}`,
				`{
  "observability.alerts": [ {"level":"warning"}, { "level": "critical"} ]
}`},
			Want: `// merged SITE_CONFIG_FILE
{
  // BEGIN $tmp/0.json
  "search.index.branches": {
    "github.com/go-yaml/yaml": [
      "v2",
      "v3"
    ],
    "github.com/kubernetes/kubernetes": [
      "release-1.17"
    ],
    "github.com/sourcegraph/sourcegraph": [
      "3.17",
      "v3.0.0"
    ]
  },
  // END $tmp/0.json
  // BEGIN $tmp/1.json
  "observability.alerts": [
    {
      "level": "warning"
    },
    {
      "level": "critical"
    }
  ],
  // END $tmp/1.json
}`,
		},
		{
			Name: "parse-error",
			Files: []string{
				"{}",
				"{",
			},
			Err: "CloseBraceExpected",
		}}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var paths []string
			for i, b := range c.Files {
				p := filepath.Join(dir, fmt.Sprintf("%d.json", i))
				paths = append(paths, p)
				if err := os.WriteFile(p, []byte(b), 0600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := readSiteConfigFile(paths)
			if c.Err != "" && !strings.Contains(fmt.Sprintf("%s", err), c.Err) {
				t.Fatalf("%s doesn't contain error substring %s", err, c.Err)
			}
			got = bytes.ReplaceAll(got, []byte(dir), []byte("$tmp"))
			if d := cmp.Diff(c.Want, string(got)); d != "" {
				t.Fatalf("unexpected merge (-want, +got):\n%s", d)
			}
		})
	}
}

func TestGitserverAddr(t *testing.T) {
	cases := []struct {
		name    string
		environ []string
		want    string
	}{{
		name: "test default",
		want: "gitserver:3178",
	}, {
		name:    "default",
		environ: []string{"SRC_GIT_SERVERS=k8s+rpc://gitserver:3178?kind=sts"},
		want:    "k8s+rpc://gitserver:3178?kind=sts",
	}, {
		name:    "exact",
		environ: []string{"SRC_GIT_SERVERS=gitserver-0:3178 gitserver-1:3178"},
		want:    "gitserver-0:3178 gitserver-1:3178",
	}, {
		name: "replicas",
		environ: []string{
			"SRC_GIT_SERVERS=2",
		},
		want: "gitserver-0.gitserver:3178 gitserver-1.gitserver:3178",
	}, {
		name: "replicas helm",
		environ: []string{
			"DEPLOY_TYPE=helm",
			"SRC_GIT_SERVERS=2",
		},
		want: "gitserver-0.gitserver:3178 gitserver-1.gitserver:3178",
	}, {
		name: "replicas docker-compose",
		environ: []string{
			"DEPLOY_TYPE=docker-compose",
			"SRC_GIT_SERVERS=2",
		},
		want: "gitserver-0:3178 gitserver-1:3178",
	}, {
		name: "unset",
		environ: []string{
			"SRC_GIT_SERVERS=",
		},
		want: "gitserver:3178",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gitserverAddr(tc.environ)
			if got != tc.want {
				t.Errorf("mismatch (-want +got):\n%s", cmp.Diff(tc.want, got))
			}
		})
	}
}

func TestSearcherAddr(t *testing.T) {
	cases := []struct {
		name    string
		environ []string
		want    string
	}{{
		name: "default",
		want: "k8s+http://searcher:3181",
	}, {
		name:    "stateful",
		environ: []string{"SEARCHER_URL=k8s+rpc://searcher:3181?kind=sts"},
		want:    "k8s+rpc://searcher:3181?kind=sts",
	}, {
		name:    "exact",
		environ: []string{"SEARCHER_URL=http://searcher-0:3181 http://searcher-1:3181"},
		want:    "http://searcher-0:3181 http://searcher-1:3181",
	}, {
		name: "replicas",
		environ: []string{
			"SEARCHER_URL=2",
		},
		want: "http://searcher-0.searcher:3181 http://searcher-1.searcher:3181",
	}, {
		name: "replicas helm",
		environ: []string{
			"DEPLOY_TYPE=helm",
			"SEARCHER_URL=2",
		},
		want: "http://searcher-0.searcher:3181 http://searcher-1.searcher:3181",
	}, {
		name: "replicas docker-compose",
		environ: []string{
			"DEPLOY_TYPE=docker-compose",
			"SEARCHER_URL=2",
		},
		want: "http://searcher-0:3181 http://searcher-1:3181",
	}, {
		name: "unset",
		environ: []string{
			"SEARCHER_URL=",
		},
		want: "k8s+http://searcher:3181",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := searcherAddr(tc.environ)
			if got != tc.want {
				t.Errorf("mismatch (-want +got):\n%s", cmp.Diff(tc.want, got))
			}
		})
	}
}

func TestSymbolsAddr(t *testing.T) {
	cases := []struct {
		name    string
		environ []string
		want    string
	}{{
		name: "default",
		want: "http://symbols:3184",
	}, {
		name:    "stateful",
		environ: []string{"SYMBOLS_URL=k8s+rpc://symbols:3184?kind=sts"},
		want:    "k8s+rpc://symbols:3184?kind=sts",
	}, {
		name:    "exact",
		environ: []string{"SYMBOLS_URL=http://symbols-0:3184 http://symbols-1:3184"},
		want:    "http://symbols-0:3184 http://symbols-1:3184",
	}, {
		name: "replicas",
		environ: []string{
			"SYMBOLS_URL=2",
		},
		want: "http://symbols-0.symbols:3184 http://symbols-1.symbols:3184",
	}, {
		name: "replicas helm",
		environ: []string{
			"DEPLOY_TYPE=helm",
			"SYMBOLS_URL=2",
		},
		want: "http://symbols-0.symbols:3184 http://symbols-1.symbols:3184",
	}, {
		name: "replicas docker-compose",
		environ: []string{
			"DEPLOY_TYPE=docker-compose",
			"SYMBOLS_URL=2",
		},
		want: "http://symbols-0:3184 http://symbols-1:3184",
	}, {
		name: "unset",
		environ: []string{
			"SYMBOLS_URL=",
		},
		want: "http://symbols:3184",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := symbolsAddr(tc.environ)
			if got != tc.want {
				t.Errorf("mismatch (-want +got):\n%s", cmp.Diff(tc.want, got))
			}
		})
	}
}

func TestZoektAddr(t *testing.T) {
	cases := []struct {
		name    string
		environ []string
		want    string
	}{{
		name: "default",
		want: "k8s+rpc://indexed-search:6070?kind=sts",
	}, {
		name:    "old",
		environ: []string{"ZOEKT_HOST=127.0.0.1:3070"},
		want:    "127.0.0.1:3070",
	}, {
		name:    "new",
		environ: []string{"INDEXED_SEARCH_SERVERS=indexed-search-0.indexed-search:6070 indexed-search-1.indexed-search:6070"},
		want:    "indexed-search-0.indexed-search:6070 indexed-search-1.indexed-search:6070",
	}, {
		name: "prefer new",
		environ: []string{
			"ZOEKT_HOST=127.0.0.1:3070",
			"INDEXED_SEARCH_SERVERS=indexed-search-0.indexed-search:6070 indexed-search-1.indexed-search:6070",
		},
		want: "indexed-search-0.indexed-search:6070 indexed-search-1.indexed-search:6070",
	}, {
		name: "replicas",
		environ: []string{
			"INDEXED_SEARCH_SERVERS=2",
		},
		want: "indexed-search-0.indexed-search:6070 indexed-search-1.indexed-search:6070",
	}, {
		name: "replicas helm",
		environ: []string{
			"DEPLOY_TYPE=helm",
			"INDEXED_SEARCH_SERVERS=2",
		},
		want: "indexed-search-0.indexed-search:6070 indexed-search-1.indexed-search:6070",
	}, {
		name: "replicas docker-compose",
		environ: []string{
			"DEPLOY_TYPE=docker-compose",
			"INDEXED_SEARCH_SERVERS=2",
		},
		want: "zoekt-webserver-0:6070 zoekt-webserver-1:6070",
	}, {
		name: "unset new",
		environ: []string{
			"ZOEKT_HOST=127.0.0.1:3070",
			"INDEXED_SEARCH_SERVERS=",
		},
		want: "",
	}, {
		name: "unset old",
		environ: []string{
			"ZOEKT_HOST=",
		},
		want: "",
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := zoektAddr(tc.environ)
			if got != tc.want {
				t.Errorf("mismatch (-want +got):\n%s", cmp.Diff(tc.want, got))
			}
		})
	}
}
