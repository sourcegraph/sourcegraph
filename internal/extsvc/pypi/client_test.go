package pypi

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/regexp"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/httptestutil"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/testutil"
	"github.com/sourcegraph/sourcegraph/internal/unpack"
)

var updateRegex = flag.String("update", "", "Update testdata of tests matching the given regex")

func update(name string) bool {
	if updateRegex == nil || *updateRegex == "" {
		return false
	}
	return regexp.MustCompile(*updateRegex).MatchString(name)
}

func TestDownload(t *testing.T) {
	ctx := context.Background()
	cli := newTestClient(t, "Download", update(t.Name()))

	files, err := cli.Project(ctx, "requests")
	if err != nil {
		t.Fatal(err)
	}

	// Pick the oldest tarball.
	j := -1
	for i, f := range files {
		if path.Ext(f.Name) == ".gz" {
			j = i
			break
		}
	}

	p, err := cli.Download(ctx, files[j].URL)
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	err = unpack.Tgz(p, tmp, unpack.Opts{})
	if err != nil {
		t.Fatal(err)
	}

	hasher := sha1.New()
	var tarFiles []string

	err = filepath.WalkDir(tmp, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		f, lErr := os.Open(path)
		if lErr != nil {
			return lErr
		}
		defer f.Close()
		b, lErr := io.ReadAll(f)
		if lErr != nil {
			return lErr
		}
		hasher.Write(b)
		tarFiles = append(tarFiles, strings.TrimPrefix(path, tmp))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	testutil.AssertGolden(t, "testdata/golden/requests", update(t.Name()), struct {
		TarHash string
		Files   []string
	}{
		TarHash: hex.EncodeToString(hasher.Sum(nil)),
		Files:   tarFiles,
	})
}

func TestProject(t *testing.T) {
	cli := newTestClient(t, "parse", update(t.Name()))
	files, err := cli.Project(context.Background(), "gpg-vault")
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertGolden(t, "testdata/golden/gpg-vault", update(t.Name()), files)
}

func TestVersion(t *testing.T) {
	cli := newTestClient(t, "parse", update(t.Name()))
	f, err := cli.Version(context.Background(), "gpg-vault", "1.4")
	if err != nil {
		t.Fatal(err)
	}
	if want := "gpg-vault-1.4.tar.gz"; want != f.Name {
		t.Fatalf("want %s, got %s", want, f.Name)
	}
}

func TestParse_empty(t *testing.T) {
	b := bytes.NewReader([]byte(`
<!DOCTYPE html>
<html>
  <body>
  </body>
</html>
`))

	_, err := parse(b)
	if err != nil {
		t.Fatal(err)
	}
}

func TestParse_broken(t *testing.T) {
	tmpl, err := template.New("project").Parse(`<!DOCTYPE html>
<html>
  <body>
	{{.Body}}
  </body>
</html>
`)
	if err != nil {
		t.Fatal(err)
	}

	tc := []struct {
		name string
		Body string
	}{
		{
			name: "no text",
			Body: "<a href=\"/frob-1.0.0.tar.gz/\"></a>",
		},
		{
			name: "text does not match base",
			Body: "<a href=\"/frob-1.0.0.tar.gz/\">foo</a>",
		},
	}

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			buf := bytes.Buffer{}
			err = tmpl.Execute(&buf, c)
			if err != nil {
				t.Fatal(err)
			}
			_, err := parse(&buf)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParse_PEP503(t *testing.T) {
	// There may be any other HTML elements on the API pages as long as the required
	// anchor elements exist.
	b := bytes.NewReader([]byte(`
<!DOCTYPE html>
<html>
  <head>
    <meta name="pypi:repository-version" content="1.0">
    <title>Links for frob</title>
  </head>
  <body>
	<h1>Links for frob</h1>
    <a href="/frob-1.0.0.tar.gz/" data-requires-python="&gt;=3">frob-1.0.0.tar.gz</a>
	<h2>More links for frob</h1>
	<div>
	    <a href="/frob-2.0.0.tar.gz/" data-gpg-sig="true">frob-2.0.0.tar.gz</a>
	    <a>frob-3.0.0.tar.gz</a>
	</div>
  </body>
</html>
`))

	got, err := parse(b)
	if err != nil {
		t.Fatal(err)
	}

	tr := true
	want := []File{
		{
			Name:               "frob-1.0.0.tar.gz",
			URL:                "/frob-1.0.0.tar.gz/",
			DataRequiresPython: ">=3",
		},
		{
			Name:       "frob-2.0.0.tar.gz",
			URL:        "/frob-2.0.0.tar.gz/",
			DataGPGSig: &tr,
		},
	}

	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("-want, +got\n%s", d)
	}
}

func TestToWheel(t *testing.T) {
	have := []string{
		"requests-2.16.2-py2.py3-none-any.whl",
		"grpcio-1.46.0rc2-cp39-cp39-win_amd64.whl",
	}
	want := []Wheel{
		{
			File:         File{Name: have[0]},
			Distribution: "requests",
			Version:      "2.16.2",
			BuildTag:     "",
			PythonTag:    "py2.py3",
			ABITag:       "none",
			PlatformTag:  "any",
		},
		{
			File:         File{Name: have[1]},
			Distribution: "grpcio",
			Version:      "1.46.0rc2",
			BuildTag:     "",
			PythonTag:    "cp39",
			ABITag:       "cp39",
			PlatformTag:  "win_amd64",
		},
	}

	var got []Wheel
	for _, h := range have {
		g, err := ToWheel(File{Name: h})
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, *g)
	}

	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("-want, +got\n%s", d)
	}
}

func TestFindVersion(t *testing.T) {
	mkTarball := func(version string) File {
		n := "request" + "-" + version + ".tar.gz"
		return File{
			Name: n,
			URL:  "https://cdn/" + n,
		}
	}

	tags1 := []string{"1", "cp38", "manylinux_2_17_x86_64.manylinux2014_x86_64"}
	tags2 := []string{"2", "cp39", "win32"}

	mkWheel := func(version string, tags ...string) File {
		if tags == nil {
			tags = []string{"py2.py3", "none", "any"}
		}
		n := "request" + "-" + version + "-" + strings.Join(tags, "-") + ".whl"
		return File{
			Name: n,
			URL:  "https://cdn/" + n,
		}
	}

	tc := []struct {
		name    string
		files   []File
		version string
		want    File
	}{
		{
			name: "only tarballs",
			files: []File{
				mkTarball("1.2.2"),
				mkTarball("1.2.3"),
				mkTarball("1.2.4"),
			},
			version: "1.2.3",
			want:    mkTarball("1.2.3"),
		},
		{
			name: "tarballs and wheels",
			files: []File{
				mkTarball("1.2.2"),
				mkWheel("1.2.2"),
				mkTarball("1.2.3"),
				mkWheel("1.2.3"),
				mkTarball("1.2.4"),
				mkWheel("1.2.4"),
			},
			version: "1.2.3",
			want:    mkTarball("1.2.3"),
		},
		{
			name: "many wheels",
			files: []File{
				mkWheel("1.2.2"),
				mkWheel("1.2.3"),
				mkWheel("1.2.3", tags1...),
				mkWheel("1.2.3", tags2...),
				mkWheel("1.2.4"),
			},
			version: "1.2.3",
			want:    mkWheel("1.2.3", tags1...),
		},
		{
			name: "many wheels, random order",
			files: []File{
				mkWheel("1.2.3"),
				mkWheel("1.2.3", tags2...),
				mkWheel("1.2.4"),
				mkWheel("1.2.3", tags1...),
				mkWheel("1.2.2"),
			},
			version: "1.2.3",
			want:    mkWheel("1.2.3", tags1...),
		},
		{
			name: "no tarball for target version",
			files: []File{
				mkTarball("1.2.2"),
				mkWheel("1.2.2"),
				mkWheel("1.2.3"),
				mkTarball("1.2.4"),
				mkWheel("1.2.4"),
			},
			version: "1.2.3",
			want:    mkWheel("1.2.3"),
		},
		{
			name: "pick latest version",
			files: []File{
				mkTarball("1.2.2"),
				mkWheel("1.2.2"),
				mkWheel("1.2.3"),
				mkTarball("1.2.4"),
				mkWheel("1.2.4"),
			},
			version: "",
			want:    mkTarball("1.2.4"),
		},
	}

	for _, c := range tc {
		t.Run(c.name, func(t *testing.T) {
			got, err := FindVersion(c.version, c.files)
			if err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(c.want, got); d != "" {
				t.Fatalf("-want,+got:\n%s", d)
			}
		})
	}
}

func TestIsSDIST(t *testing.T) {
	tc := []struct {
		have string
		want string
	}{
		{
			have: "file.tar.gz",
			want: ".tar.gz",
		},
		{
			have: "file.tar",
			want: ".tar",
		},
		{
			have: "file.tar.Z",
			want: ".tar.Z",
		},
		{
			have: "file.zip",
			want: ".zip",
		},
		{
			have: "file.tar.xz",
			want: ".tar.xz",
		},
		{
			have: "file.tar.bz2",
			want: ".tar.bz2",
		},
		{
			have: "file.foo",
			want: "",
		},
		{
			have: "file.foo.bz",
			want: "",
		},
		{
			have: "",
			want: "",
		},
		{
			have: "foo",
			want: "",
		},
	}

	for _, c := range tc {
		t.Run(c.have, func(t *testing.T) {
			if got := isSDIST(c.have); got != c.want {
				t.Fatalf("want %q, got %q", c.want, got)
			}
		})
	}
}

// newTestClient returns a pypi Client that records its interactions
// to testdata/vcr/.
func newTestClient(t testing.TB, name string, update bool) *Client {
	cassete := filepath.Join("testdata/vcr/", normalize(name))
	rec, err := httptestutil.NewRecorder(cassete, update)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := rec.Stop(); err != nil {
			t.Errorf("failed to update test data: %s", err)
		}
	})

	doer := httpcli.NewFactory(nil, httptestutil.NewRecorderOpt(rec))

	c := NewClient("urn", []string{"https://pypi.org/simple"}, doer)
	c.limiter = ratelimit.NewInstrumentedLimiter("pypi", rate.NewLimiter(100, 10))
	return c
}
