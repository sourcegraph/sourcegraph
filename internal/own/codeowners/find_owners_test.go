package codeowners_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/own/codeowners"
	codeownerspb "github.com/sourcegraph/sourcegraph/internal/own/codeowners/v1"
)

type testCase struct {
	pattern string
	paths   []string
}

func TestFileOwnersMatch(t *testing.T) {
	cases := []testCase{
		{
			pattern: "filename",
			paths: []string{
				"/filename",
				"/prefix/filename",
			},
		},
		{
			pattern: "*.md",
			paths: []string{
				"/README.md",
				"/README.md.md",
				"/nested/index.md",
				"/weird/but/matching/.md",
			},
		},
		{
			// Regex components are interpreted literally.
			pattern: "[^a-z].md",
			paths: []string{
				"/[^a-z].md",
				"/nested/[^a-z].md",
			},
		},
		{
			pattern: "foo*bar*baz",
			paths: []string{
				"/foobarbaz",
				"/foo-bar-baz",
				"/foobarbazfoobarbazfoobarbaz",
			},
		},
		{
			pattern: "directory/path/",
			paths: []string{
				"/directory/path/file",
				"/directory/path/deeply/nested/file",
				"/prefix/directory/path/file",
				"/prefix/directory/path/deeply/nested/file",
			},
		},
		{
			pattern: "directory/path/**",
			paths: []string{
				"/directory/path/file",
				"/directory/path/deeply/nested/file",
				"/prefix/directory/path/file",
				"/prefix/directory/path/deeply/nested/file",
			},
		},
		{
			pattern: "directory/*",
			paths: []string{
				"/directory/file",
				"/prefix/directory/another_file",
			},
		},
		{
			pattern: "/toplevelfile",
			paths: []string{
				"/toplevelfile",
			},
		},
		{
			pattern: "/main/src/**/README.md",
			paths: []string{
				"/main/src/README.md",
				"/main/src/foo/bar/README.md",
			},
		},
		// Literal absolute match.
		{
			pattern: "/main/src/README.md",
			paths: []string{
				"/main/src/README.md",
			},
		},
		// Without a leading `/` still matches correctly.
		{
			pattern: "/main/src/README.md",
			paths: []string{
				"main/src/README.md",
			},
		},
	}
	for _, c := range cases {
		for _, path := range c.paths {
			pattern := c.pattern
			owner := []*codeownerspb.Owner{
				{Handle: "foo"},
			}
			rs := codeowners.NewRuleset(
				codeowners.IngestedRulesetSource{},
				&codeownerspb.File{
					Rule: []*codeownerspb.Rule{
						{Pattern: pattern, Owner: owner},
					},
				},
			)
			got := rs.Match(path)
			if !reflect.DeepEqual(got.GetOwner(), owner) {
				t.Errorf("want %q to match %q", pattern, path)
			}
		}
	}
}

func TestFileOwnersNoMatch(t *testing.T) {
	cases := []testCase{
		{
			pattern: "filename",
			paths: []string{
				"/prefix_filename_suffix",
				"/src/prefix_filename",
				"/finemale/nested",
			},
		},
		{
			pattern: "*.md",
			paths: []string{
				"/README.mdf",
				"/not/matching/without/the/dot/md",
			},
		},
		{
			// Regex components are interpreted literally.
			pattern: "[^a-z].md",
			paths: []string{
				"/-.md",
				"/nested/%.md",
			},
		},
		{
			pattern: "foo*bar*baz",
			paths: []string{
				"/foo-ba-baz",
				"/foobarbaz.md",
			},
		},
		{
			pattern: "directory/leaf/",
			paths: []string{
				// These do not match as the right-most directory name `leaf`
				// is just a prefix to the corresponding directory on the given path.
				"/directory/leaf_and_more/file",
				"/prefix/directory/leaf_and_more/file",
				// These do not match as the pattern matches anything within
				// the sub-directory tree, but not the directory itself.
				"/directory/leaf",
				"/prefix/directory/leaf",
			},
		},
		{
			pattern: "directory/leaf/**",
			paths: []string{
				// These do not match as the right-most directory name `leaf`
				// is just a prefix to the corresponding directory on the given path.
				"/directory/leaf_and_more/file",
				"/prefix/directory/leaf_and_more/file",
				// These do not match as the pattern matches anything within
				// the sub-directory tree, but not the directory itself.
				"/directory/leaf",
				"/prefix/directory/leaf",
			},
		},
		{
			pattern: "directory/*",
			paths: []string{
				"/directory/nested/file",
				"/directory/deeply/nested/file",
			},
		},
		{
			pattern: "/toplevelfile",
			paths: []string{
				"/toplevelfile/nested",
				"/notreally/toplevelfile",
			},
		},
		{
			pattern: "/main/src/**/README.md",
			paths: []string{
				"/main/src/README.mdf",
				"/main/src/README.md/looks-like-a-file-but-was-dir",
				"/main/src/foo/bar/README.mdf",
				"/nested/main/src/README.md",
				"/nested/main/src/foo/bar/README.md",
			},
		},
	}
	for _, c := range cases {
		for _, path := range c.paths {
			pattern := c.pattern
			owner := []*codeownerspb.Owner{
				{Handle: "foo"},
			}
			rs := codeowners.NewRuleset(
				codeowners.IngestedRulesetSource{},
				&codeownerspb.File{
					Rule: []*codeownerspb.Rule{
						{Pattern: pattern, Owner: owner},
					},
				},
			)
			got := rs.Match(path)
			if got.GetOwner() != nil {
				t.Errorf("want %q not to match %q", pattern, path)
			}
		}
	}
}

func TestFileOwnersOrder(t *testing.T) {
	wantOwner := []*codeownerspb.Owner{{Handle: "some-path-owner"}}
	rs := codeowners.NewRuleset(
		codeowners.IngestedRulesetSource{},
		&codeownerspb.File{
			Rule: []*codeownerspb.Rule{
				{
					Pattern: "/top-level-directory/",
					Owner:   []*codeownerspb.Owner{{Handle: "top-level-owner"}},
				},
				// The owner of the last matching pattern is being picked
				{
					Pattern: "some/path/*",
					Owner:   wantOwner,
				},
				{
					Pattern: "does/not/match",
					Owner:   []*codeownerspb.Owner{{Handle: "not-matching-owner"}},
				},
			},
		})
	got := rs.Match("/top-level-directory/some/path/main.go")
	assert.Equal(t, wantOwner, got.GetOwner())
}

func BenchmarkOwnersMatchLiteral(b *testing.B) {
	pattern := "/main/src/foo/bar/README.md"
	paths := []string{
		"/main/src/foo/bar/README.md",
	}
	owner := []*codeownerspb.Owner{
		{Handle: "foo"},
	}
	rs := codeowners.NewRuleset(
		codeowners.IngestedRulesetSource{},
		&codeownerspb.File{
			Rule: []*codeownerspb.Rule{
				{Pattern: pattern, Owner: owner},
			},
		},
	)
	// Warm cache.
	for _, path := range paths {
		rs.Match(path)
	}

	for range b.N {
		rs.Match(pattern)
	}
}

func BenchmarkOwnersMatchRelativeGlob(b *testing.B) {
	pattern := "**/*.md"
	paths := []string{
		"/main/src/foo/bar/README.md",
	}
	owner := []*codeownerspb.Owner{
		{Handle: "foo"},
	}
	rs := codeowners.NewRuleset(
		codeowners.IngestedRulesetSource{},
		&codeownerspb.File{
			Rule: []*codeownerspb.Rule{
				{Pattern: pattern, Owner: owner},
			},
		},
	)
	// Warm cache.
	for _, path := range paths {
		rs.Match(path)
	}

	for range b.N {
		rs.Match(pattern)
	}
}

func BenchmarkOwnersMatchAbsoluteGlob(b *testing.B) {
	pattern := "/main/**/*.md"
	paths := []string{
		"/main/src/foo/bar/README.md",
	}
	owner := []*codeownerspb.Owner{
		{Handle: "foo"},
	}
	rs := codeowners.NewRuleset(
		codeowners.IngestedRulesetSource{},
		&codeownerspb.File{
			Rule: []*codeownerspb.Rule{
				{Pattern: pattern, Owner: owner},
			},
		},
	)
	// Warm cache.
	for _, path := range paths {
		rs.Match(path)
	}

	for range b.N {
		rs.Match(pattern)
	}
}

func BenchmarkOwnersMismatchLiteral(b *testing.B) {
	pattern := "/main/src/foo/bar/README.md"
	paths := []string{
		"/main/src/foo/bar/README.txt",
	}
	owner := []*codeownerspb.Owner{
		{Handle: "foo"},
	}
	rs := codeowners.NewRuleset(
		codeowners.IngestedRulesetSource{},
		&codeownerspb.File{
			Rule: []*codeownerspb.Rule{
				{Pattern: pattern, Owner: owner},
			},
		},
	)
	// Warm cache.
	for _, path := range paths {
		rs.Match(path)
	}

	for range b.N {
		rs.Match(pattern)
	}
}

func BenchmarkOwnersMismatchRelativeGlob(b *testing.B) {
	pattern := "**/*.md"
	paths := []string{
		"/main/src/foo/bar/README.txt",
	}
	owner := []*codeownerspb.Owner{
		{Handle: "foo"},
	}
	rs := codeowners.NewRuleset(
		codeowners.IngestedRulesetSource{},
		&codeownerspb.File{
			Rule: []*codeownerspb.Rule{
				{Pattern: pattern, Owner: owner},
			},
		},
	)
	// Warm cache.
	for _, path := range paths {
		rs.Match(path)
	}

	for range b.N {
		rs.Match(pattern)
	}
}

func BenchmarkOwnersMismatchAbsoluteGlob(b *testing.B) {
	pattern := "/main/**/*.md"
	paths := []string{
		"/main/src/foo/bar/README.txt",
	}
	owner := []*codeownerspb.Owner{
		{Handle: "foo"},
	}
	rs := codeowners.NewRuleset(
		codeowners.IngestedRulesetSource{},
		&codeownerspb.File{
			Rule: []*codeownerspb.Rule{
				{Pattern: pattern, Owner: owner},
			},
		},
	)
	// Warm cache.
	for _, path := range paths {
		rs.Match(path)
	}

	for range b.N {
		rs.Match(pattern)
	}
}

func BenchmarkOwnersMatchMultiHole(b *testing.B) {
	pattern := "/main/**/foo/**/*.md"
	paths := []string{
		"/main/src/foo/bar/README.md",
	}
	owner := []*codeownerspb.Owner{
		{Handle: "foo"},
	}
	rs := codeowners.NewRuleset(
		codeowners.IngestedRulesetSource{},
		&codeownerspb.File{
			Rule: []*codeownerspb.Rule{
				{Pattern: pattern, Owner: owner},
			},
		},
	)
	// Warm cache.
	for _, path := range paths {
		rs.Match(path)
	}

	for range b.N {
		rs.Match(pattern)
	}
}

func BenchmarkOwnersMismatchMultiHole(b *testing.B) {
	pattern := "/main/**/foo/**/*.md"
	paths := []string{
		"/main/src/foo/bar/README.txt",
	}
	owner := []*codeownerspb.Owner{
		{Handle: "foo"},
	}
	rs := codeowners.NewRuleset(
		codeowners.IngestedRulesetSource{},
		&codeownerspb.File{
			Rule: []*codeownerspb.Rule{
				{Pattern: pattern, Owner: owner},
			},
		},
	)
	// Warm cache.
	for _, path := range paths {
		rs.Match(path)
	}

	for range b.N {
		rs.Match(pattern)
	}
}

func BenchmarkOwnersMatchLiteralLargeRuleset(b *testing.B) {
	pattern := "/main/src/foo/bar/README.md"
	paths := []string{
		"/main/src/foo/bar/README.md",
	}
	owner := []*codeownerspb.Owner{
		{Handle: "foo"},
	}
	f := &codeownerspb.File{
		Rule: []*codeownerspb.Rule{
			{Pattern: pattern, Owner: owner},
		},
	}
	for i := range 10000 {
		f.Rule = append(f.Rule, &codeownerspb.Rule{Pattern: fmt.Sprintf("%s-%d", pattern, i), Owner: owner})
	}
	rs := codeowners.NewRuleset(codeowners.IngestedRulesetSource{}, f)
	// Warm cache.
	for _, path := range paths {
		rs.Match(path)
	}

	for range b.N {
		rs.Match(pattern)
	}
}
