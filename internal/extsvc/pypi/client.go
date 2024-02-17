// Package pypi
//
// A client for PyPI's simple project API as described in
// https://peps.python.org/pep-0503/.
//
// Nomenclature:
//
// A "project" on PyPI is the name of a collection of releases and files, and
// information about them. Projects on PyPI are made and shared by other members
// of the Python community so that you can use them.
//
// A "release" on PyPI is a specific version of a project. For example, the
// requests project has many releases, like "requests 2.10" and "requests 1.2.1".
// A release consists of one or more "files".
//
// A "file", also known as a "package", on PyPI is something that you can
// download and install. Because of different hardware, operating systems, and
// file formats, a release may have several files (packages), like an archive
// containing source code or a binary
//
// https://pypi.org/help/#packages
package pypi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"

	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Client struct {
	// A list of PyPI proxies. Each url should point to the root of the simple-API.
	// For example for pypi.org the url should be https://pypi.org/simple with or
	// without a trailing slash.
	urls []string
	cf   *httpcli.Factory

	// Self-imposed rate-limiter. pypi.org does not impose a rate limiting policy.
	limiter *ratelimit.InstrumentedLimiter
}

func NewClient(urn string, urls []string, httpfactory *httpcli.Factory) *Client {
	return &Client{
		urls:    urls,
		cf:      httpfactory,
		limiter: ratelimit.NewInstrumentedLimiter(urn, ratelimit.NewGlobalRateLimiter(log.Scoped("PyPiClient"), urn)),
	}
}

// Project returns the Files of the simple-API /<project>/ endpoint.
func (c *Client) Project(ctx context.Context, project reposource.PackageName) ([]File, error) {
	doer, err := c.cf.Doer(httpcli.CachedTransportOpt)
	if err != nil {
		return nil, err
	}

	data, err := c.get(ctx, doer, reposource.PackageName(normalize(string(project))))
	if err != nil {
		return nil, errors.Wrap(err, "PyPI")
	}
	defer data.Close()

	return parse(data)
}

// Version returns the File of a project at a specific version from
// the simple-API /<project>/ endpoint.
func (c *Client) Version(ctx context.Context, project reposource.PackageName, version string) (File, error) {
	files, err := c.Project(ctx, project)
	if err != nil {
		return File{}, err
	}

	f, err := FindVersion(version, files)
	if err != nil {
		return File{}, errors.Wrapf(err, "project: %q", project)
	}

	return f, nil
}

// FindVersion finds the File for the given version amongst files from a project.
func FindVersion(version string, files []File) (File, error) {
	if len(files) == 0 {
		return File{}, errors.Errorf("no files")
	}

	// This loop should never iterate over more than a few files.
	if version == "" {
		for i := len(files) - 1; i >= 0; i-- {
			if w, err := ToWheel(files[i]); err == nil {
				version = w.Version
				break
			} else if s, err := ToSDist(files[i]); err == nil {
				version = s.Version
				break
			}
		}
	}

	if version == "" {
		return File{}, &Error{
			code:    404,
			message: "could not find a wheel or source distribution to determine the latest version",
		}
	}

	// We return the first source distribution we can find for the version.
	//
	// In case we cannot find a source distribution, we return the first wheel in
	// lexicographic order to guarantee that we pick the same wheel every time as
	// long as the list of wheels doesn't change.
	//
	// Pep 503 does not prescribe lexicographic order of files returned from the
	// simple API.
	//
	// The consequence is that we might pick a different tarball or wheel when we
	// reclone if the list of files changes. This might break links. We consider
	// this an edge case.
	//
	var minWheelAtVersion *File
	for i, f := range files {
		if wheel, err := ToWheel(f); err != nil {
			if sdist, err := ToSDist(f); err == nil && sdist.Version == version {
				return f, nil
			}
		} else if wheel.Version == version && (minWheelAtVersion == nil || f.Name < minWheelAtVersion.Name) {
			minWheelAtVersion = &files[i]
		}
	}

	if minWheelAtVersion != nil {
		return *minWheelAtVersion, nil
	}

	return File{}, &Error{
		code:    404,
		message: fmt.Sprintf("could not find a wheel or source distribution for version %s", version),
	}
}

type NotFoundError struct {
	error
}

func (e NotFoundError) NotFound() bool {
	return true
}

// File represents one anchor element in the response from /<project>/.
//
// https://peps.python.org/pep-0503/
type File struct {
	// The file format for tarballs is <package>-<version>.tar.gz.
	//
	// The file format for wheels (.whl) is described in
	// https://peps.python.org/pep-0491/#file-format.
	Name string

	// URLs may be either absolute or relative as long as they point to the correct
	// location.
	URL string

	// Optional. A repository MAY include a data-gpg-sig attribute on a file link
	// with a value of either true or false to indicate whether or not there is a
	// GPG signature. Repositories that do this SHOULD include it on every link.
	DataGPGSig *bool

	// A repository MAY include a data-requires-python attribute on a file link.
	// This exposes the Requires-Python metadata field, specified in PEP 345, for
	// the corresponding release.
	DataRequiresPython string
}

// parse parses the output of Client.Project into a list of files. Anchor tags
// without href are ignored.
func parse(b io.Reader) ([]File, error) {
	var files []File

	z := html.NewTokenizer(b)

	// We want to iterate over the anchor tags. Quoting from PEP503: "[The project]
	// URL must respond with a valid HTML5 page with a single anchor element per
	// file for the project".
	nextAnchor := func() bool {
		for {
			switch z.Next() {
			case html.ErrorToken:
				return false
			case html.StartTagToken:
				if name, _ := z.TagName(); string(name) == "a" {
					return true
				}
			}
		}
	}

OUTER:
	for nextAnchor() {
		cur := File{}

		// parse attributes.
		for {
			k, v, more := z.TagAttr()
			switch string(k) {
			case "href":
				cur.URL = string(v)
			case "data-requires-python":
				cur.DataRequiresPython = string(v)
			case "data-gpg-sig":
				w, err := strconv.ParseBool(string(v))
				if err != nil {
					continue
				}
				cur.DataGPGSig = &w
			}
			if !more {
				break
			}
		}

		if cur.URL == "" {
			continue
		}

	INNER:
		for {
			switch z.Next() {
			case html.ErrorToken:
				break OUTER
			case html.TextToken:
				cur.Name = string(z.Text())

				// the text of the anchor tag MUST match the final path component (the filename)
				// of the URL. The URL SHOULD include a hash in the form of a URL fragment with
				// the following syntax: #<hashname>=<hashvalue>
				u, err := url.Parse(cur.URL)
				if err != nil {
					return nil, err
				}
				if base := filepath.Base(u.Path); base != cur.Name {
					return nil, errors.Newf("%s != %s: text does not match final path component", cur.Name, base)
				}

				files = append(files, cur)
				break INNER
			}
		}
	}
	if err := z.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	return files, nil
}

// Download downloads a file located at url, respecting the rate limit.
func (c *Client) Download(ctx context.Context, url string) (io.ReadCloser, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	doer, err := c.cf.Doer()
	if err != nil {
		return nil, err
	}

	b, err := c.do(doer, req)
	if err != nil {
		return nil, errors.Wrap(err, "PyPI")
	}
	return b, nil
}

// A SDist is a Python source distribution.
type SDist struct {
	File
	Distribution string
	Version      string
}

func ToSDist(f File) (*SDist, error) {
	name := f.Name

	// source distribution or unsupported other format.
	ext := isSDIST(name)
	if ext == "" {
		return nil, errors.Errorf("%q is not a sdist", name)
	}

	name = strings.TrimSuffix(name, ext)

	// For source distributions we expect the pattern <package>-<version>.<ext>,
	// where <package> might include "-". We determine the package version on a best
	// effort basis by assuming the version is the string between the last "-" and
	// the extension.
	i := strings.LastIndexByte(name, '-')
	if i == -1 {
		return nil, errors.Errorf("%q has an invalid sdist format", name)
	}

	return &SDist{
		File:         f,
		Distribution: name[:i],
		Version:      name[i+1:],
	}, nil
}

// isSDIST returns the file extension if filename has one of the supported sdist
// formats. If the file extension is not supported, isSDIST returns the empty
// string.
func isSDIST(filename string) string {
	switch ext := filepath.Ext(filename); ext {
	case ".zip", ".tar":
		return ext
	}

	switch ext := extN(filename, 2); ext {
	case ".tar.gz", ".tar.bz2", ".tar.xz", ".tar.Z":
		return ext
	default:
		return ""
	}
}

func extN(path string, n int) (ext string) {
	if n == -1 {
		i := strings.Index(path, ".")
		if i == -1 {
			return ""
		}
		return path[i:]
	}
	for i := len(path) - 1; i >= 0 && !os.IsPathSeparator(path[i]); i-- {
		if path[i] == '.' {
			n--
			if n == 0 {
				return path[i:]
			}
		}
	}
	return ""
}

// https://peps.python.org/pep-0491/#file-format
type Wheel struct {
	File
	Distribution string
	Version      string
	BuildTag     string
	PythonTag    string
	ABITag       string
	PlatformTag  string
}

// ToWheel parses a filename of a wheel according to the format specified in
// https://peps.python.org/pep-0491/#file-format
func ToWheel(f File) (*Wheel, error) {
	name := f.Name

	if e := path.Ext(name); e != ".whl" {
		return nil, errors.Errorf("%s does not conform to pep 491", name)
	} else {
		name = name[:len(name)-len(e)]
	}

	pcs := strings.Split(name, "-")
	switch len(pcs) {
	case 5:
		return &Wheel{
			File:         f,
			Distribution: pcs[0],
			Version:      pcs[1],
			BuildTag:     "",
			PythonTag:    pcs[2],
			ABITag:       pcs[3],
			PlatformTag:  pcs[4],
		}, nil
	case 6:
		return &Wheel{
			File:         f,
			Distribution: pcs[0],
			Version:      pcs[1],
			BuildTag:     pcs[2],
			PythonTag:    pcs[3],
			ABITag:       pcs[4],
			PlatformTag:  pcs[5],
		}, nil
	default:
		return nil, errors.Errorf("%s does not conform to pep 491", name)
	}
}

func (c *Client) get(ctx context.Context, doer httpcli.Doer, project reposource.PackageName) (respBody io.ReadCloser, err error) {
	var (
		reqURL *url.URL
		req    *http.Request
	)

	for _, baseURL := range c.urls {
		if err = c.limiter.Wait(ctx); err != nil {
			return nil, err
		}

		reqURL, err = url.Parse(baseURL)
		if err != nil {
			return nil, errors.Errorf("invalid proxy URL %q", baseURL)
		}

		// Go-http-client User-Agents are currently blocked from accessing /simple
		// resources without a trailing slash. This causes a redirect to the
		// canonicalized URL with the trailing slash. PyPI maintainers have been
		// struggling to handle a piece of software with this User-Agent overloading our
		// backends with requests resulting in redirects.
		reqURL.Path = path.Join(reqURL.Path, string(project)) + "/"

		req, err = http.NewRequestWithContext(ctx, "GET", reqURL.String(), nil)
		if err != nil {
			return nil, err
		}

		respBody, err = c.do(doer, req)
		if err == nil || !errcode.IsNotFound(err) {
			break
		}
	}

	return respBody, err
}

func (c *Client) do(doer httpcli.Doer, req *http.Request) (io.ReadCloser, error) {
	resp, err := doer.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()

		bs, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, &Error{path: req.URL.Path, code: resp.StatusCode, message: fmt.Sprintf("failed to read non-200 body: %v", err)}
		}
		return nil, &Error{path: req.URL.Path, code: resp.StatusCode, message: string(bs)}
	}

	return resp.Body, nil
}

type Error struct {
	path    string
	code    int
	message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("bad response with status code %d for %s: %s", e.code, e.path, e.message)
}

func (e *Error) NotFound() bool {
	return e.code == http.StatusNotFound
}

// https://peps.python.org/pep-0503/#normalized-names
var normalizer = lazyregexp.New("[-_.]+")

func normalize(path string) string {
	return strings.ToLower(normalizer.ReplaceAllLiteralString(path, "-"))
}
