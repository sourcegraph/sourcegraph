package repos

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/extsvc/github"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestRepoExcluderRuleErrors(t *testing.T) {
	var ex repoExcluder

	ex.AddRule(NewRule().Pattern("valid"))
	require.NoError(t, ex.RuleErrors())

	ex.AddRule(NewRule().Pattern("[\\\\"))
	require.Error(t, ex.RuleErrors())
}

func TestRuleExcludes(t *testing.T) {
	startsWithFoo := func(input any) bool {
		if s, ok := input.(string); ok {
			return strings.HasPrefix(s, "foo")
		}
		return false
	}

	t.Run("ExactName", func(t *testing.T) {
		r := &rule{}
		r.Exact("foobar")

		assert.Equal(t, true, r.Excludes("foobar"))
		assert.Equal(t, false, r.Excludes("barfoo"))

		r = &rule{}
		r.Exact("barfoo")
		assert.Equal(t, false, r.Excludes("foobar"))
		assert.Equal(t, true, r.Excludes("barfoo"))

		// Multiple values are OR'd together.
		r = &rule{}
		r.Exact("foobar")
		r.Exact("barfoo")
		assert.Equal(t, true, r.Excludes("foobar"))
		assert.Equal(t, true, r.Excludes("barfoo"))
		assert.Equal(t, false, r.Excludes("f00baz"))
	})

	t.Run("Pattern", func(t *testing.T) {
		r := &rule{}
		r.Pattern("^foo.*")

		assert.Equal(t, true, r.Excludes("foobar"))
		assert.Equal(t, false, r.Excludes("barfoo"))
	})

	t.Run("Generic", func(t *testing.T) {
		r := &rule{}
		r.Generic(startsWithFoo)

		assert.Equal(t, true, r.Excludes("foobar"))
		assert.Equal(t, false, r.Excludes("barfoo"))
	})

	t.Run("multiple conditions", func(t *testing.T) {
		r := &rule{}
		r.Exact("foobar")
		r.Exact("foobar2")
		r.Pattern("^foo.*")

		assert.Equal(t, true, r.Excludes("foobar"))
		assert.Equal(t, true, r.Excludes("foobar2"))
		assert.Equal(t, false, r.Excludes("barfoo"))

		r.Generic(startsWithFoo)

		assert.Equal(t, true, r.Excludes("foobar"))
		assert.Equal(t, false, r.Excludes("barfoo"))

		endsWithFoo := func(input any) bool {
			if s, ok := input.(string); ok {
				return strings.HasSuffix(s, "foo")
			}
			return false
		}

		r.Generic(endsWithFoo)
		// All conditions have to be true and no argument here fulfills all of them
		assert.Equal(t, false, r.Excludes("foobar"))
		assert.Equal(t, false, r.Excludes("barfoo"))
	})
}

func TestGitHubStarsAndSize(t *testing.T) {
	assertExcluded := func(t *testing.T, githubRule *schema.ExcludedGitHubRepo, repo github.Repository, wantExcluded bool) {
		t.Helper()
		rule := &rule{}

		if githubRule.Stars != "" {
			fn, err := buildStarsConstraintsExcludeFn(githubRule.Stars)
			require.NoError(t, err)
			rule.Generic(fn)
		}

		if githubRule.Size != "" {
			fn, err := buildSizeConstraintsExcludeFn(githubRule.Size)
			require.NoError(t, err)
			rule.Generic(fn)
		}

		assert.Equal(
			t,
			wantExcluded,
			rule.Excludes(repo),
			"rule.Stars=%q, rule.Size=%q, repo.StarGazerCount=%d, repo.DiskUsageKibibytes=%d",
			githubRule.Stars,
			githubRule.Size,
			repo.StargazerCount,
			repo.DiskUsageKibibytes,
		)
	}

	t.Run("stars", func(t *testing.T) {
		tests := []struct {
			rule         string
			stars        int
			wantExcluded bool
		}{
			{"< 100", 99, true},
			{"< 100", 100, false},

			{"<= 100", 100, true},
			{"<= 100", 99, true},
			{"<= 100", 101, false},

			{"> 100", 101, true},
			{"> 100", 100, false},

			{">= 100", 100, true},
			{">= 100", 101, true},
			{">= 100", 99, false},
		}
		for _, tt := range tests {
			excludeRule := &schema.ExcludedGitHubRepo{Stars: tt.rule}
			repo := github.Repository{StargazerCount: tt.stars}
			assertExcluded(t, excludeRule, repo, tt.wantExcluded)
		}
	})

	t.Run("size", func(t *testing.T) {
		tests := []struct {
			rule          string
			sizeKibibytes int
			wantExcluded  bool
		}{
			{"< 100 KiB", 99, true},
			{"< 100 KiB", 100, false},

			{"<= 100 KiB", 100, true},
			{"<= 100 KiB", 99, true},
			{"<= 100 KiB", 101, false},

			{"> 100 KiB", 101, true},
			{"> 100 KiB", 100, false},

			{">= 100 KiB", 100, true},
			{">= 100 KiB", 101, true},
			{">= 100 KiB", 99, false},

			{"< 1025 B", 1, true},
			{"< 1024 B", 1, false},
			{"< 1025 B", 2, false},

			{"< 100 KiB", 99, true},
			{"< 100 KiB", 100, false},
			{"< 100 KB", 99, false},
			{"< 102 KB", 99, true},

			{"< 100 MiB", 99, true},
			{"< 100 MiB", 102400, false},
			{"< 101 MiB", 102400, true},
			{"< 100 MB", 102400, false},
			{"< 105 MB", 102400, true},

			{"< 1 GiB", 1024*1024 - 1, true},
			{"< 1 GiB", 1024 * 1024, false},

			{"< 1 GB", 1024*1024 - 1, false},
			{"< 2 GB", 1024 * 1024, true},

			// Ignore repositories with 0 size
			{"< 1 MB", 0, false},
		}

		for _, tt := range tests {
			excludeRule := &schema.ExcludedGitHubRepo{Size: tt.rule}
			repo := github.Repository{DiskUsageKibibytes: tt.sizeKibibytes}
			assertExcluded(t, excludeRule, repo, tt.wantExcluded)
		}
	})

	t.Run("stars and size", func(t *testing.T) {
		rule := &schema.ExcludedGitHubRepo{Stars: "< 100", Size: ">= 1GB"}

		// Less than 100 stars, equal or greater than 1GB in size
		assertExcluded(t, rule, github.Repository{StargazerCount: 99, DiskUsageKibibytes: 976563}, true)
		assertExcluded(t, rule, github.Repository{StargazerCount: 99, DiskUsageKibibytes: 976563 + 1}, true)

		// Equal or greater than 100 stars, greater than 1GB in size
		assertExcluded(t, rule, github.Repository{StargazerCount: 100, DiskUsageKibibytes: 976563 + 1}, false)
		assertExcluded(t, rule, github.Repository{StargazerCount: 101, DiskUsageKibibytes: 976563 + 1}, false)

		// Greater than 100 stars, less than 1 GB
		assertExcluded(t, rule, github.Repository{StargazerCount: 101, DiskUsageKibibytes: 500}, false)
	})
}

func BenchmarkExcludeExact(b *testing.B) {
	// First, we build an excluder with 10,000 exact match rules.
	var ex repoExcluder
	for i := range 10_000 {
		ex.AddRule(NewRule().Exact(fmt.Sprintf("repo-%d", i)))
	}
	require.NoError(b, ex.RuleErrors())

	b.ResetTimer()

	// Now, we check if the excluder excludes a repo that doesn't match any of the rules.
	// That is, so that we have to do the most effort possible.
	for i := 0; i < b.N; i++ {
		ex.ShouldExclude("repo-notfound")
	}

	b.ReportAllocs()
}
