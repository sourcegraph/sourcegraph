package vcssyncer

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/regexp"

	"github.com/sourcegraph/sourcegraph/internal/extsvc/pypi"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/httptestutil"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/testutil"
)

var updateRegex = flag.String("update", "", "Update testdata of tests matching the given regex")

func update(name string) bool {
	if updateRegex == nil || *updateRegex == "" {
		return false
	}
	return regexp.MustCompile(*updateRegex).MatchString(name)
}

func TestUnpackPythonPackage_TGZ(t *testing.T) {
	files := []fileInfo{
		{
			path:     "common/file1.py",
			contents: []byte("banana"),
		},
		{
			path:     "common/setup.py",
			contents: []byte("apple"),
		},
		{
			path:     ".git/index",
			contents: []byte("filter me"),
		},
		{
			path:     "/absolute/path/are/filtered",
			contents: []byte("filter me"),
		},
	}

	pkg := bytes.NewReader(createTgz(t, files))

	tmp := t.TempDir()
	if err := unpackPythonPackage(pkg, "https://some.where/pckg.tar.gz", func() (string, error) { return tmp, nil }, tmp); err != nil {
		t.Fatal()
	}

	got := make([]string, 0, len(files))
	if err := filepath.Walk(tmp, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		got = append(got, strings.TrimPrefix(path, tmp))
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sort.Slice(got, func(i, j int) bool {
		return got[i] < got[j]
	})

	// without the filtered files, the rest of the files share a common folder
	// "common" which should also be removed.
	want := []string{"/file1.py", "/setup.py"}
	sort.Slice(want, func(i, j int) bool {
		return want[i] < want[j]
	})

	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("-want,+got\n%s", d)
	}
}

func TestUnpackPythonPackage_ZIP(t *testing.T) {
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	for _, f := range []fileInfo{
		{
			path:     "src/file1.py",
			contents: []byte("banana"),
		},
		{
			path:     "src/file2.py",
			contents: []byte("apple"),
		},
		{
			path:     "setup.py",
			contents: []byte("pear"),
		},
	} {
		fw, err := zw.Create(f.path)
		if err != nil {
			t.Fatal(err)
		}

		_, err = fw.Write(f.contents)
		if err != nil {
			t.Fatal(err)
		}
	}

	err := zw.Close()
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	if err := unpackPythonPackage(&zipBuf, "https://some.where/pckg.zip", func() (string, error) { return os.MkdirTemp(tmp, "zip") }, tmp); err != nil {
		t.Fatal()
	}

	var got []string
	if err := filepath.Walk(tmp, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		got = append(got, strings.TrimPrefix(path, tmp))
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	sort.Slice(got, func(i, j int) bool {
		return got[i] < got[j]
	})

	want := []string{"/src/file1.py", "/src/file2.py", "/setup.py"}
	sort.Slice(want, func(i, j int) bool {
		return want[i] < want[j]
	})

	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("-want,+got\n%s", d)
	}
}

func TestUnpackPythonPackage_InvalidZip(t *testing.T) {
	files := []fileInfo{
		{
			path:     "file1.py",
			contents: []byte("banana"),
		},
	}

	pkg := bytes.NewReader(createTgz(t, files))

	if err := unpackPythonPackage(pkg, "https://some.where/pckg.whl", func() (string, error) { return t.TempDir(), nil }, t.TempDir()); err == nil {
		t.Fatal("no error returned from unpack package")
	}
}

func TestUnpackPythonPackage_UnsupportedFormat(t *testing.T) {
	if err := unpackPythonPackage(bytes.NewReader([]byte{}), "https://some.where/pckg.exe", func() (string, error) { return "", nil }, ""); err == nil {
		t.Fatal()
	}
}

func TestUnpackPythonPackage_Wheel(t *testing.T) {
	ratelimit.SetupForTest(t)

	ctx := context.Background()

	cl := newTestClient(t, "requests", update(t.Name()))
	f, err := cl.Project(ctx, "requests")
	if err != nil {
		t.Fatal(err)
	}

	// Pick a specific wheel.
	var wheelURL string
	for i := len(f) - 1; i >= 0; i-- {
		if f[i].Name == "requests-2.27.1-py2.py3-none-any.whl" {
			wheelURL = f[i].URL
			break
		}
	}
	if wheelURL == "" {
		t.Fatalf("could not find wheel")
	}

	b, err := cl.Download(ctx, wheelURL)
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	if err := unpackPythonPackage(b, wheelURL, func() (string, error) { return os.MkdirTemp(tmp, "wheel") }, tmp); err != nil {
		t.Fatal(err)
	}

	var files []string
	hasher := sha1.New()
	if err := filepath.Walk(tmp, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
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

		files = append(files, strings.TrimPrefix(path, tmp))
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	testutil.AssertGolden(t, "testdata/golden/requests.json", update(t.Name()), struct {
		Hash  string
		Files []string
	}{
		Hash:  hex.EncodeToString(hasher.Sum(nil)),
		Files: files,
	})
}

// newTestClient returns a pypi Client that records its interactions
// to testdata/vcr/.
func newTestClient(t testing.TB, name string, update bool) *pypi.Client {
	cassete := filepath.Join("testdata/vcr/", name)
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

	c, _ := pypi.NewClient("urn", []string{"https://pypi.org/simple"}, doer)
	return c
}
