package subrepoperms

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/schema"
	"github.com/stretchr/testify/require"
)

func TestSubRepoPermsPermissions(t *testing.T) {
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			ExperimentalFeatures: &schema.ExperimentalFeatures{
				SubRepoPermissions: &schema.SubRepoPermissions{
					Enabled: true,
				},
			},
		},
	})
	t.Cleanup(func() { conf.Mock(nil) })

	testCases := []struct {
		name     string
		userID   int32
		content  authz.RepoContent
		clientFn func() *SubRepoPermsClient
		want     authz.Perms
	}{
		{
			name:   "Empty path",
			userID: 1,
			content: authz.RepoContent{
				Repo: "sample",
				Path: "",
			},
			clientFn: func() *SubRepoPermsClient {
				return NewSubRepoPermsClient(NewMockSubRepoPermissionsGetter())
			},
			want: authz.Read,
		},
		{
			name:   "No rules",
			userID: 1,
			content: authz.RepoContent{
				Repo: "sample",
				Path: "/dev/thing",
			},
			clientFn: func() *SubRepoPermsClient {
				getter := NewMockSubRepoPermissionsGetter()
				getter.GetByUserFunc.SetDefaultHook(func(ctx context.Context, i int32) (map[api.RepoName]authz.SubRepoPermissions, error) {
					return map[api.RepoName]authz.SubRepoPermissions{
						"sample": {
							Paths: []string{},
						},
					}, nil
				})
				return NewSubRepoPermsClient(getter)
			},
			want: authz.None,
		},
		{
			name:   "Exclude",
			userID: 1,
			content: authz.RepoContent{
				Repo: "sample",
				Path: "/dev/thing",
			},
			clientFn: func() *SubRepoPermsClient {
				getter := NewMockSubRepoPermissionsGetter()
				getter.GetByUserFunc.SetDefaultHook(func(ctx context.Context, i int32) (map[api.RepoName]authz.SubRepoPermissions, error) {
					return map[api.RepoName]authz.SubRepoPermissions{
						"sample": {
							Paths: []string{"-/dev/*"},
						},
					}, nil
				})
				return NewSubRepoPermsClient(getter)
			},
			want: authz.None,
		},
		{
			name:   "Include",
			userID: 1,
			content: authz.RepoContent{
				Repo: "sample",
				Path: "/dev/thing",
			},
			clientFn: func() *SubRepoPermsClient {
				getter := NewMockSubRepoPermissionsGetter()
				getter.GetByUserFunc.SetDefaultHook(func(ctx context.Context, i int32) (map[api.RepoName]authz.SubRepoPermissions, error) {
					return map[api.RepoName]authz.SubRepoPermissions{
						"sample": {
							Paths: []string{"/*"},
						},
					}, nil
				})
				return NewSubRepoPermsClient(getter)
			},
			want: authz.None,
		},
		{
			name:   "Last rule takes precedence (exclude)",
			userID: 1,
			content: authz.RepoContent{
				Repo: "sample",
				Path: "/dev/thing",
			},
			clientFn: func() *SubRepoPermsClient {
				getter := NewMockSubRepoPermissionsGetter()
				getter.GetByUserFunc.SetDefaultHook(func(ctx context.Context, i int32) (map[api.RepoName]authz.SubRepoPermissions, error) {
					return map[api.RepoName]authz.SubRepoPermissions{
						"sample": {
							Paths: []string{"/**", "-/dev/*"},
						},
					}, nil
				})
				return NewSubRepoPermsClient(getter)
			},
			want: authz.None,
		},
		{
			name:   "Last rule takes precedence (include)",
			userID: 1,
			content: authz.RepoContent{
				Repo: "sample",
				Path: "/dev/thing",
			},
			clientFn: func() *SubRepoPermsClient {
				getter := NewMockSubRepoPermissionsGetter()
				getter.GetByUserFunc.SetDefaultHook(func(ctx context.Context, i int32) (map[api.RepoName]authz.SubRepoPermissions, error) {
					return map[api.RepoName]authz.SubRepoPermissions{
						"sample": {
							Paths: []string{"-/dev/*", "/**"},
						},
					}, nil
				})
				return NewSubRepoPermsClient(getter)
			},
			want: authz.Read,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := tc.clientFn()
			have, err := client.Permissions(context.Background(), tc.userID, tc.content)
			if err != nil {
				t.Fatal(err)
			}
			if have != tc.want {
				t.Fatalf("have %v, want %v", have, tc.want)
			}
		})
	}
}

func BenchmarkFilterActorPaths(b *testing.B) {
	// This benchmark is simulating the code path taken by a monorepo with sub
	// repo permissions. Our goal is to support repos with millions of files.
	// For now we target a lower number since large numbers don't give enough
	// runs of the benchmark to be useful.
	const pathCount = 5_000
	pathPatterns := []string{
		"base/%d/foo.go",
		"%d/stuff/baz",
		"frontend/%d/stuff/baz/bam",
		"subdir/sub/sub/sub/%d",
		"%d/foo/README.md",
		"subdir/remove/me/please/%d",
		"subdir/%d/also-remove/me/please",
		"a/deep/path/%d/.secrets.env",
		"%d/does/not/match/anything",
		"does/%d/not/match/anything",
		"does/not/%d/match/anything",
		"does/not/match/%d/anything",
		"does/not/match/anything/%d",
	}
	paths := []string{
		"config.yaml",
		"dir.yaml",
	}
	for i := 0; len(paths) < pathCount; i++ {
		for _, pat := range pathPatterns {
			paths = append(paths, fmt.Sprintf(pat, i))
		}
	}
	paths = paths[:pathCount]
	sort.Strings(paths)

	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			ExperimentalFeatures: &schema.ExperimentalFeatures{
				SubRepoPermissions: &schema.SubRepoPermissions{
					Enabled: true,
				},
			},
		},
	})
	defer conf.Mock(nil)
	repo := api.RepoName("repo")

	getter := NewMockSubRepoPermissionsGetter()
	getter.GetByUserFunc.SetDefaultHook(func(ctx context.Context, i int32) (map[api.RepoName]authz.SubRepoPermissions, error) {
		return map[api.RepoName]authz.SubRepoPermissions{
			repo: {
				Paths: []string{
					"/base/**",
					"/*/stuff/**",
					"/frontend/**/stuff/*",
					"/config.yaml",
					"/subdir/**",
					"/**/README.md",
					"/dir.yaml",
					"-/subdir/remove/",
					"-/subdir/*/also-remove/**",
					"-/**/.secrets.env",
				},
			},
		}, nil
	})
	checker := NewSubRepoPermsClient(getter)

	a := &actor.Actor{
		UID: 1,
	}
	ctx := actor.WithActor(context.Background(), a)

	b.ResetTimer()
	start := time.Now()

	for n := 0; n <= b.N; n++ {
		filtered, err := authz.FilterActorPaths(ctx, checker, a, repo, paths)
		if err != nil {
			b.Fatal(err)
		}
		if len(filtered) == 0 {
			b.Fatal("expected paths to be returned")
		}
		if len(filtered) == len(paths) {
			b.Fatal("expected to filter out some paths")
		}
	}

	b.ReportMetric(float64(len(paths))*float64(b.N)/time.Since(start).Seconds(), "paths/s")
}

func TestSubRepoPermissionsCanReadDirectoriesInPath(t *testing.T) {
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			ExperimentalFeatures: &schema.ExperimentalFeatures{
				SubRepoPermissions: &schema.SubRepoPermissions{
					Enabled: true,
				},
			},
		},
	})
	t.Cleanup(func() { conf.Mock(nil) })
	repoName := api.RepoName("repo")

	testCases := []struct {
		paths         []string
		canReadAll    []string
		cannotReadAny []string
	}{
		{
			paths:         []string{"foo/bar/thing.txt"},
			canReadAll:    []string{"foo/", "foo/bar/"},
			cannotReadAny: []string{"foo/thing.txt", "foo/bar/other.txt"},
		},
		{
			paths:      []string{"foo/bar/**"},
			canReadAll: []string{"foo/", "foo/bar/", "foo/bar/baz/", "foo/bar/baz/fox/"},
		},
		{
			paths:         []string{"foo/bar/"},
			canReadAll:    []string{"foo/", "foo/bar/"},
			cannotReadAny: []string{"foo/thing.txt", "foo/bar/thing.txt"},
		},
		{
			paths:         []string{"baz/*/foo/bar/thing.txt"},
			canReadAll:    []string{"baz/", "baz/x/", "baz/x/foo/bar/"},
			cannotReadAny: []string{"baz/thing.txt"},
		},
		// If we have a wildcard in a path we allow all directories that are not
		// explicitly excluded.
		{
			paths:      []string{"**/foo/bar/thing.txt"},
			canReadAll: []string{"foo/", "foo/bar/"},
		},
		{
			paths:      []string{"*/foo/bar/thing.txt"},
			canReadAll: []string{"foo/", "foo/bar/"},
		},
		{
			paths:      []string{"/**/foo/bar/thing.txt"},
			canReadAll: []string{"foo/", "foo/bar/"},
		},
		{
			paths:      []string{"/*/foo/bar/thing.txt"},
			canReadAll: []string{"foo/", "foo/bar/"},
		},
		{
			paths:      []string{"-/**", "/storage/redis/**"},
			canReadAll: []string{"storage/", "/storage/", "/storage/redis/"},
		},
		{
			paths:      []string{"-/**", "-/storage/**", "/storage/redis/**"},
			canReadAll: []string{"storage/", "/storage/", "/storage/redis/"},
		},
		// Even with a wildcard include rule, we should still exclude directories that
		// are explicitly excluded later
		{
			paths:         []string{"/**", "-/storage/**"},
			canReadAll:    []string{"/foo"},
			cannotReadAny: []string{"storage/", "/storage/", "/storage/redis/"},
		},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			getter := NewMockSubRepoPermissionsGetter()
			getter.GetByUserFunc.SetDefaultHook(func(ctx context.Context, i int32) (map[api.RepoName]authz.SubRepoPermissions, error) {
				return map[api.RepoName]authz.SubRepoPermissions{
					repoName: {
						Paths: tc.paths,
					},
				}, nil
			})
			client := NewSubRepoPermsClient(getter)

			ctx := context.Background()

			for _, path := range tc.canReadAll {
				content := authz.RepoContent{
					Repo: repoName,
					Path: path,
				}
				perm, err := client.Permissions(ctx, 1, content)
				if err != nil {
					t.Error(err)
				}
				if !perm.Include(authz.Read) {
					t.Errorf("Should be able to read %q, cannot", path)
				}
			}

			for _, path := range tc.cannotReadAny {
				content := authz.RepoContent{
					Repo: repoName,
					Path: path,
				}
				perm, err := client.Permissions(ctx, 1, content)
				if err != nil {
					t.Error(err)
				}
				if perm.Include(authz.Read) {
					t.Errorf("Should not be able to read %q, can", path)
				}
			}
		})
	}
}

func TestSubRepoPermsPermissionsCache(t *testing.T) {
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			ExperimentalFeatures: &schema.ExperimentalFeatures{
				SubRepoPermissions: &schema.SubRepoPermissions{
					Enabled: true,
				},
			},
		},
	})
	t.Cleanup(func() { conf.Mock(nil) })

	getter := NewMockSubRepoPermissionsGetter()
	client := NewSubRepoPermsClient(getter)

	ctx := context.Background()
	content := authz.RepoContent{
		Repo: api.RepoName("thing"),
		Path: "/stuff",
	}

	// Should hit DB only once
	for range 3 {
		_, err := client.Permissions(ctx, 1, content)
		if err != nil {
			t.Fatal(err)
		}

		h := getter.GetByUserFunc.History()
		if len(h) != 1 {
			t.Fatal("Should have been called once")
		}
	}

	// Trigger expiry
	client.since = func(time time.Time) time.Duration {
		return defaultCacheTTL + 1
	}

	_, err := client.Permissions(ctx, 1, content)
	if err != nil {
		t.Fatal(err)
	}

	h := getter.GetByUserFunc.History()
	if len(h) != 2 {
		t.Fatal("Should have been called twice")
	}
}

func TestRepoEnabledCache(t *testing.T) {
	cache := newRepoEnabledCache(time.Hour)

	_, cacheHit := cache.RepoIsEnabled(api.RepoID(42))
	require.False(t, cacheHit)

	cache.SetRepoIsEnabled(api.RepoID(42), true)
	enabled, cacheHit := cache.RepoIsEnabled(api.RepoID(42))
	require.True(t, cacheHit)
	require.True(t, enabled)

	cache.SetRepoIsEnabled(api.RepoID(43), false)
	enabled, cacheHit = cache.RepoIsEnabled(api.RepoID(43))
	require.True(t, cacheHit)
	require.False(t, enabled)

	cache.lastReset = time.Now().Add(-10 * time.Hour)

	_, cacheHit = cache.RepoIsEnabled(api.RepoID(42))
	require.False(t, cacheHit)
}
