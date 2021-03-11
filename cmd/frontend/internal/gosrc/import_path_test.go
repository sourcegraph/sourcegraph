// Copyright 2014 The Go Authors. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file or at
// https://developers.google.com/open-source/licenses/bsd.

package gosrc

import (
	"io/ioutil"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

type testTransport map[string]string

func (t testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	statusCode := http.StatusOK
	req.URL.RawQuery = ""
	body, ok := t[req.URL.String()]
	if !ok {
		statusCode = http.StatusNotFound
	}
	resp := &http.Response{
		StatusCode: statusCode,
		Body:       ioutil.NopCloser(strings.NewReader(body)),
	}
	return resp, nil
}

func TestResolveImportPath(t *testing.T) {
	tests := []struct {
		importPath string
		dir        *Directory
	}{
		// static
		{"fmt", &Directory{
			ImportPath:  "fmt",
			ProjectRoot: "",
			CloneURL:    "https://github.com/golang/go",
			RepoPrefix:  "src",
			VCS:         "git",
			Rev:         runtime.Version(),
		}},
		{"cmd/internal/obj/x86", &Directory{
			ImportPath:  "cmd/internal/obj/x86",
			ProjectRoot: "",
			CloneURL:    "https://github.com/golang/go",
			RepoPrefix:  "src",
			VCS:         "git",
			Rev:         runtime.Version(),
		}},
		{"github.com/foo/bar", &Directory{
			ImportPath:  "github.com/foo/bar",
			ProjectRoot: "github.com/foo/bar",
			CloneURL:    "https://github.com/foo/bar",
			VCS:         "git",
		}},
		{"github.com/foo/bar/baz", &Directory{
			ImportPath:  "github.com/foo/bar/baz",
			ProjectRoot: "github.com/foo/bar",
			CloneURL:    "https://github.com/foo/bar",
			VCS:         "git",
		}},
		{"github.com/foo/bar/baz/bam", &Directory{
			ImportPath:  "github.com/foo/bar/baz/bam",
			ProjectRoot: "github.com/foo/bar",
			CloneURL:    "https://github.com/foo/bar",
			VCS:         "git",
		}},
		{"golang.org/x/foo", &Directory{
			ImportPath:  "golang.org/x/foo",
			ProjectRoot: "golang.org/x/foo",
			CloneURL:    "https://github.com/golang/foo",
			VCS:         "git",
		}},
		{"golang.org/x/foo/bar", &Directory{
			ImportPath:  "golang.org/x/foo/bar",
			ProjectRoot: "golang.org/x/foo",
			CloneURL:    "https://github.com/golang/foo",
			VCS:         "git",
		}},
		{"github.com/foo", nil},

		// dynamic (see client setup below)
		{"alice.org/pkg", &Directory{
			ImportPath:  "alice.org/pkg",
			ProjectRoot: "alice.org/pkg",
			CloneURL:    "https://github.com/alice/pkg",
			VCS:         "git",
		}},
		{"alice.org/pkg/sub", &Directory{
			ImportPath:  "alice.org/pkg/sub",
			ProjectRoot: "alice.org/pkg",
			CloneURL:    "https://github.com/alice/pkg",
			VCS:         "git",
		}},
		{"alice.org/pkg/http", &Directory{
			ImportPath:  "alice.org/pkg/http",
			ProjectRoot: "alice.org/pkg",
			CloneURL:    "https://github.com/alice/pkg",
			VCS:         "git",
		}},
		{"alice.org/pkg/ignore", &Directory{
			ImportPath:  "alice.org/pkg/ignore",
			ProjectRoot: "alice.org/pkg",
			CloneURL:    "https://github.com/alice/pkg",
			VCS:         "git",
		}},
		{"alice.org/pkg/mismatch", nil},
		{"alice.org/pkg/multiple", nil},
		{"alice.org/pkg/notfound", nil},

		{"bob.com/pkg", &Directory{
			ImportPath:  "bob.com/pkg",
			ProjectRoot: "bob.com/pkg",
			CloneURL:    "https://vcs.net/bob/pkg.git",
			VCS:         "git",
		}},
		{"bob.com/pkg/sub", &Directory{
			ImportPath:  "bob.com/pkg/sub",
			ProjectRoot: "bob.com/pkg",
			CloneURL:    "https://vcs.net/bob/pkg.git",
			VCS:         "git",
		}},

		{"gopkg.in/yaml.v2", &Directory{
			ImportPath:  "gopkg.in/yaml.v2",
			ProjectRoot: "gopkg.in/yaml.v2",
			CloneURL:    "https://github.com/go-yaml/yaml",
			VCS:         "git",
			Rev:         "v2",
		}},
		{"gopkg.in/stretchr/testify.v1/assert", &Directory{
			ImportPath:  "gopkg.in/stretchr/testify.v1/assert",
			ProjectRoot: "gopkg.in/stretchr/testify.v1",
			CloneURL:    "https://github.com/stretchr/testify",
			VCS:         "git",
			Rev:         "v1.1.4",
		}},
		{"honnef.co/go/staticcheck/cmd/staticcheck", &Directory{
			ImportPath:  "honnef.co/go/staticcheck/cmd/staticcheck",
			ProjectRoot: "honnef.co/go/staticcheck",
			CloneURL:    "https://github.com/dominikh/go-staticcheck",
			VCS:         "git",
		}},

		{"golang.org/x", nil},
	}

	pages := map[string]string{
		// Package at root of a GitHub repo.
		"https://alice.org/pkg": `<head> <meta name="go-import" content="alice.org/pkg git https://github.com/alice/pkg"></head>`,
		// Package in sub-diretory.
		"https://alice.org/pkg/sub": `<head> <meta name="go-import" content="alice.org/pkg git https://github.com/alice/pkg"><body>`,
		// Fallback to http.
		"http://alice.org/pkg/http": `<head> <meta name="go-import" content="alice.org/pkg git https://github.com/alice/pkg">`,
		// Meta tag in sub-directory does not match meta tag at root.
		"https://alice.org/pkg/mismatch": `<head> <meta name="go-import" content="alice.org/pkg hg https://github.com/alice/pkg">`,
		// More than one matching meta tag.
		"http://alice.org/pkg/multiple": `<head> ` +
			`<meta name="go-import" content="alice.org/pkg git https://github.com/alice/pkg">` +
			`<meta name="go-import" content="alice.org/pkg git https://github.com/alice/pkg">`,
		// Unknown meta name
		"https://alice.org/pkg/ignore": `<meta name="go-junk" content="alice.org/pkg http://alice.org/pkg http://alice.org/pkg{/dir} http://alice.org/pkg{/dir}?f={file}#Line{line}">` +
			// go-import tag for the package
			`<meta name="go-import" content="alice.org/pkg git https://github.com/alice/pkg">` +
			// go-import with wrong number of fields
			`<meta name="go-import" content="alice.org/pkg https://github.com/alice/pkg">` +
			// go-import with no fields
			`<meta name="go-import" content="">` +
			// meta tag for a different package
			`<meta name="go-import" content="alice.org/other git https://github.com/alice/other">` +
			// meta tag for a different package
			`<meta name="go-import" content="alice.org/other git https://github.com/alice/other">` +
			`</head>` +
			// go-import outside of head
			`<meta name="go-import" content="alice.org/pkg git https://github.com/alice/pkg">`,

		// Package at root of a Git repo.
		"https://bob.com/pkg": `<head> <meta name="go-import" content="bob.com/pkg git https://vcs.net/bob/pkg.git">`,
		// Package at in sub-directory of a Git repo.
		"https://bob.com/pkg/sub": `<head> <meta name="go-import" content="bob.com/pkg git https://vcs.net/bob/pkg.git">`,

		// Some popular entries
		"https://gopkg.in/yaml.v2": `<head>
<meta name="go-import" content="gopkg.in/yaml.v2 git https://gopkg.in/yaml.v2">
<meta name="go-source" content="gopkg.in/yaml.v2 _ https://github.com/go-yaml/yaml/tree/v2{/dir} https://github.com/go-yaml/yaml/blob/v2{/dir}/{file}#L{line}">
</head>`,
		"https://gopkg.in/stretchr/testify.v1/assert": `<head>
<meta name="go-import" content="gopkg.in/stretchr/testify.v1 git https://gopkg.in/stretchr/testify.v1">
<meta name="go-source" content="gopkg.in/stretchr/testify.v1 _ https://github.com/stretchr/testify/tree/v1.1.4{/dir} https://github.com/stretchr/testify/blob/v1.1.4{/dir}/{file}#L{line}">
</head>`,
		"https://gopkg.in/stretchr/testify.v1": `<head>
<meta name="go-import" content="gopkg.in/stretchr/testify.v1 git https://gopkg.in/stretchr/testify.v1">
<meta name="go-source" content="gopkg.in/stretchr/testify.v1 _ https://github.com/stretchr/testify/tree/v1.1.4{/dir} https://github.com/stretchr/testify/blob/v1.1.4{/dir}/{file}#L{line}">
</head>`,
		"https://honnef.co/go/staticcheck/cmd/staticcheck": `<head> <meta name="go-import" content="honnef.co/go/staticcheck git https://github.com/dominikh/go-staticcheck"> </head>`,
		"https://honnef.co/go/staticcheck":                 `<head> <meta name="go-import" content="honnef.co/go/staticcheck git https://github.com/dominikh/go-staticcheck"> </head>`,
	}
	client := &http.Client{Transport: testTransport(pages)}

	for _, tt := range tests {
		dir, err := ResolveImportPath(client, tt.importPath)

		if tt.dir == nil {
			if err == nil {
				t.Errorf("resolveImportPath(client, %q) did not return expected error", tt.importPath)
			}
			continue
		}

		if err != nil {
			t.Errorf("resolveImportPath(client, %q) return unexpected error: %v", tt.importPath, err)
			continue
		}

		if !reflect.DeepEqual(dir, tt.dir) {
			t.Errorf("resolveImportPath(client, %q) =\n     %+v,\nwant %+v", tt.importPath, dir, tt.dir)
		}
	}
}
