package gosrc

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
)

// Adapted from github.com/golang/gddo/gosrc.

// RuntimeVersion is the version of go stdlib to use. We allow it to be
// different to runtime.Version for test data.
var RuntimeVersion = runtime.Version()

type Directory struct {
	ImportPath  string // the Go import path for this package
	ProjectRoot string // import path prefix for all packages in the project
	CloneURL    string // the VCS clone URL
	RepoPrefix  string // the path to this directory inside the repo, if set
	VCS         string // one of "git", "hg", "svn", etc.
	Rev         string // the VCS revision specifier, if any
}

var errNoMatch = errors.New("no match")

func ResolveImportPath(client httpcli.Doer, importPath string) (*Directory, error) {
	if d, err := resolveStaticImportPath(importPath); err == nil {
		return d, nil
	} else if err != errNoMatch {
		return nil, err
	}
	return resolveDynamicImportPath(client, importPath)
}

func resolveStaticImportPath(importPath string) (*Directory, error) {
	if IsStdlibPkg(importPath) {
		return &Directory{
			ImportPath:  importPath,
			ProjectRoot: "",
			CloneURL:    "https://github.com/golang/go",
			RepoPrefix:  "src",
			VCS:         "git",
			Rev:         RuntimeVersion,
		}, nil
	}

	switch {
	case strings.HasPrefix(importPath, "github.com/"):
		parts := strings.SplitN(importPath, "/", 4)
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid github.com/golang.org import path: %q", importPath)
		}
		repo := parts[0] + "/" + parts[1] + "/" + parts[2]
		return &Directory{
			ImportPath:  importPath,
			ProjectRoot: repo,
			CloneURL:    "https://" + repo,
			VCS:         "git",
		}, nil

	case strings.HasPrefix(importPath, "golang.org/x/"):
		d, err := resolveStaticImportPath(strings.Replace(importPath, "golang.org/x/", "github.com/golang/", 1))
		if err != nil {
			return nil, err
		}
		d.ImportPath = strings.Replace(d.ImportPath, "github.com/golang/", "golang.org/x/", 1)
		d.ProjectRoot = strings.Replace(d.ProjectRoot, "github.com/golang/", "golang.org/x/", 1)
		return d, nil
	}
	return nil, errNoMatch
}

// gopkgSrcTemplate matches the go-source dir templates specified by the
// popular gopkg.in
var gopkgSrcTemplate = lazyregexp.New(`https://(github.com/[^/]*/[^/]*)/tree/([^/]*)\{/dir\}`)

func resolveDynamicImportPath(client httpcli.Doer, importPath string) (*Directory, error) {
	metaProto, im, sm, err := fetchMeta(client, importPath)
	if err != nil {
		return nil, err
	}

	if im.prefix != importPath {
		var imRoot *importMeta
		metaProto, imRoot, _, err = fetchMeta(client, im.prefix)
		if err != nil {
			return nil, err
		}
		if *imRoot != *im {
			return nil, fmt.Errorf("project root mismatch: %q != %q", *imRoot, *im)
		}
	}

	// clonePath is the repo URL from import meta tag, with the "scheme://" prefix removed.
	// It should be used for cloning repositories.
	// repo is the repo URL from import meta tag, with the "scheme://" prefix removed, and
	// a possible ".vcs" suffix trimmed.
	i := strings.Index(im.repo, "://")
	if i < 0 {
		return nil, fmt.Errorf("bad repo URL: %s", im.repo)
	}
	clonePath := im.repo[i+len("://"):]
	repo := strings.TrimSuffix(clonePath, "."+im.vcs)
	dirName := importPath[len(im.prefix):]

	var dir *Directory
	if sm != nil {
		m := gopkgSrcTemplate.FindStringSubmatch(sm.dirTemplate)
		if len(m) > 0 {
			// We are doing best effort, so we ignore err
			dir, _ = resolveStaticImportPath(m[1] + dirName)
			if dir != nil {
				dir.Rev = m[2]
			}
		}
	}

	if dir == nil {
		// We are doing best effort, so we ignore err
		dir, _ = resolveStaticImportPath(repo + dirName)
	}

	if dir == nil {
		dir = &Directory{}
	}
	dir.ImportPath = importPath
	dir.ProjectRoot = im.prefix
	if dir.CloneURL == "" {
		dir.CloneURL = metaProto + "://" + repo + "." + im.vcs
	}
	dir.VCS = im.vcs
	return dir, nil
}

// importMeta represents the values in a go-import meta tag.
//
// See https://golang.org/cmd/go/#hdr-Remote_import_paths.
type importMeta struct {
	prefix string // the import path corresponding to the repository root
	vcs    string // one of "git", "hg", "svn", etc.
	repo   string // root of the VCS repo containing a scheme and not containing a .vcs qualifier
}

// sourceMeta represents the values in a go-source meta tag.
type sourceMeta struct {
	prefix       string
	projectURL   string
	dirTemplate  string
	fileTemplate string
}

func fetchMeta(client httpcli.Doer, importPath string) (scheme string, im *importMeta, sm *sourceMeta, err error) {
	uri := importPath
	if !strings.Contains(uri, "/") {
		// Add slash for root of domain.
		uri = uri + "/"
	}
	uri = uri + "?go-get=1"

	get := func() (*http.Response, error) {
		req, err := http.NewRequest("GET", scheme+"://"+uri, nil)
		if err != nil {
			return nil, err
		}
		return client.Do(req)
	}

	scheme = "https"
	resp, err := get()
	if err != nil || resp.StatusCode != 200 {
		if err == nil {
			resp.Body.Close()
		}
		scheme = "http"
		//nolint:bodyclose // False positive: https://github.com/timakin/bodyclose/issues/29
		resp, err = get()
		if err != nil {
			return scheme, nil, nil, err
		}
	}
	defer resp.Body.Close()
	im, sm, err = parseMeta(scheme, importPath, resp.Body)
	return scheme, im, sm, err
}

func parseMeta(scheme, importPath string, r io.Reader) (im *importMeta, sm *sourceMeta, err error) {
	errorMessage := "go-import meta tag not found"

	d := xml.NewDecoder(r)
	d.Strict = false
metaScan:
	for {
		t, tokenErr := d.Token()
		if tokenErr != nil {
			break metaScan
		}
		switch t := t.(type) {
		case xml.EndElement:
			if strings.EqualFold(t.Name.Local, "head") {
				break metaScan
			}
		case xml.StartElement:
			if strings.EqualFold(t.Name.Local, "body") {
				break metaScan
			}
			if !strings.EqualFold(t.Name.Local, "meta") {
				continue metaScan
			}
			nameAttr := attrValue(t.Attr, "name")
			if nameAttr != "go-import" && nameAttr != "go-source" {
				continue metaScan
			}
			fields := strings.Fields(attrValue(t.Attr, "content"))
			if len(fields) < 1 {
				continue metaScan
			}
			prefix := fields[0]
			if !strings.HasPrefix(importPath, prefix) ||
				!(len(importPath) == len(prefix) || importPath[len(prefix)] == '/') {
				// Ignore if root is not a prefix of the  path. This allows a
				// site to use a single error page for multiple repositories.
				continue metaScan
			}
			switch nameAttr {
			case "go-import":
				if len(fields) != 3 {
					errorMessage = "go-import meta tag content attribute does not have three fields"
					continue metaScan
				}
				if im != nil {
					im = nil
					errorMessage = "more than one go-import meta tag found"
					break metaScan
				}
				im = &importMeta{
					prefix: prefix,
					vcs:    fields[1],
					repo:   fields[2],
				}
			case "go-source":
				if sm != nil {
					// Ignore extra go-source meta tags.
					continue metaScan
				}
				if len(fields) != 4 {
					continue metaScan
				}
				sm = &sourceMeta{
					prefix:       prefix,
					projectURL:   fields[1],
					dirTemplate:  fields[2],
					fileTemplate: fields[3],
				}
			}
		}
	}
	if im == nil {
		return nil, nil, fmt.Errorf("%s at %s://%s", errorMessage, scheme, importPath)
	}
	if sm != nil && sm.prefix != im.prefix {
		sm = nil
	}
	return im, sm, nil
}

func attrValue(attrs []xml.Attr, name string) string {
	for _, a := range attrs {
		if strings.EqualFold(a.Name.Local, name) {
			return a.Value
		}
	}
	return ""
}
