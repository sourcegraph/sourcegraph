package paths

import (
	"bytes"
	"os/exec"
	"testing"
	"time"
)

type testCase struct {
	pattern string
	paths   []string
}

func TestMatch(t *testing.T) {
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
		{
			// A workaround used by embeddings to exclude filenames containing
			// ".." which breaks git show.
			pattern: "**..**",
			paths: []string{
				"doc/foo..bar",
				"doc/foo...bar",
			},
		},
	}

	for _, testCase := range cases {
		pattern, err := Compile(testCase.pattern)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		for _, path := range testCase.paths {
			if !pattern.Match(path) {
				t.Errorf("%q should match %q", testCase.pattern, path)
			}
		}
	}
}

func TestNoMatch(t *testing.T) {
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
				"", // ensure we don't panic on empty string
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
		{
			// A workaround used by embeddings to exclude filenames containing
			// ".." which breaks git show.
			pattern: "**..**",
			paths: []string{
				"doc/foo.bar",
				"doc/foo",
				"README.md",
			},
		},
	}
	for _, testCase := range cases {
		pattern, err := Compile(testCase.pattern)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		for _, path := range testCase.paths {
			if pattern.Match(path) {
				t.Errorf("%q should not match %q", testCase.pattern, path)
			}
		}
	}
}

func BenchmarkMatch(b *testing.B) {
	// A benchmark for a potentially slow pattern run against the paths in the
	// sourcegraph repo.
	//
	// 2023-05-30(keegan) results on my Apple M2 Max:
	// BenchmarkMatch/dot-dot-12        517    2208546 ns/op    0.00014 match_p    156.1 ns/match
	// BenchmarkMatch/top-level-12    21054      56889 ns/op    0.00007 match_p      4.0 ns/match
	// BenchmarkMatch/filename-12       996    1194387 ns/op    0.00551 match_p     84.4 ns/match
	// BenchmarkMatch/dot-star-12       544    2229892 ns/op    0.2988  match_p    157.6 ns/match

	pathsRaw, err := exec.Command("git", "ls-tree", "-r", "--full-tree", "--name-only", "-z", "HEAD").Output()
	if err != nil {
		b.Fatal()
	}
	var paths []string
	for _, p := range bytes.Split(pathsRaw, []byte{0}) {
		paths = append(paths, "/"+string(p))
	}

	cases := []struct {
		name    string
		pattern string
	}{{
		// A workaround used by embeddings to exclude filenames containing
		// ".." which breaks git show.
		name:    "dot-dot",
		pattern: "**..**",
	}, {
		name:    "top-level",
		pattern: "/README.md",
	}, {
		name:    "filename",
		pattern: "main.go",
	}, {
		name:    "dot-star",
		pattern: "*.go",
	}}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			pattern, err := Compile(tc.pattern)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			start := time.Now()

			for range b.N {
				count := 0
				for _, p := range paths {
					if pattern.Match(p) {
						count++
					}
				}
				b.ReportMetric(float64(count)/float64(len(paths)), "match_p")
			}

			b.ReportMetric(float64(time.Since(start).Nanoseconds())/float64(b.N*len(paths)), "ns/match")
		})
	}
}
