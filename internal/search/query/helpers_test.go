package query

import (
	"testing"

	"github.com/grafana/regexp"
)

func TestLangToFileRegexp(t *testing.T) {
	cases := []struct {
		lang        string
		matches     []string
		doesntMatch []string
	}{
		{
			lang: "Starlark",
			matches: []string{
				// BUILD.bazel
				"BUILD.bazel",
				"/BUILD.bazel",
				"/a/BUILD.bazel",
				"/a/BUILD",
				"/a/b/BUILD.bazel",
				// *.bzl
				"/a/b/foo.bzl",
				// lowercase
				"build.bazel",
				"build",
				// uppercase
				"BUILD.BAZEL",
				"BUILD.BZL",
			},
			doesntMatch: []string{
				"aBUILD.bazel",
				"aBUILD.bazelb",
				"a/BUILD.bazel/b",
				"BUILD.bazel/b",
				"aBUILDb",
				"BUILDb",
			},
		},
		{
			lang: "Dockerfile",
			matches: []string{
				"Dockerfile",
				"a/Dockerfile",
				"/a/b/Dockerfile",
				"DOCKERFILE",
				"dockerfile",
			},
			doesntMatch: []string{
				"notaDockerfile",
				"a/Dockerfile/b",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.lang, func(t *testing.T) {
			pattern := LangToFileRegexp(c.lang)
			re, err := regexp.Compile(pattern)
			if err != nil {
				t.Fatal(err)
			}

			for _, m := range c.matches {
				if !re.MatchString(m) {
					t.Errorf("expected %q to match %q", pattern, m)
				}
			}

			for _, m := range c.doesntMatch {
				if re.MatchString(m) {
					t.Errorf("expected %q to not match %q", pattern, m)
				}
			}
		})
	}
}
