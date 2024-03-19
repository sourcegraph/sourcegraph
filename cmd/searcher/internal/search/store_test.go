package search

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestPrepareZip(t *testing.T) {
	s := tmpStore(t)

	wantRepo := api.RepoName("foo")
	wantCommit := api.CommitID("deadbeefdeadbeefdeadbeefdeadbeefdeadbeef")

	returnFetch := make(chan struct{})
	var gotRepo api.RepoName
	var gotCommit api.CommitID
	var fetchZipCalled int64
	s.FetchTar = func(ctx context.Context, repo api.RepoName, commit api.CommitID) (io.ReadCloser, error) {
		<-returnFetch
		atomic.AddInt64(&fetchZipCalled, 1)
		gotRepo = repo
		gotCommit = commit
		return emptyTar(t), nil
	}

	// Fetch same commit in parallel to ensure single-flighting works
	startPrepareZip := make(chan struct{})
	prepareZipErr := make(chan error)
	for range 10 {
		go func() {
			<-startPrepareZip
			_, err := s.PrepareZip(context.Background(), wantRepo, wantCommit, nil)
			prepareZipErr <- err
		}()
	}
	close(startPrepareZip)
	close(returnFetch)
	for range 10 {
		err := <-prepareZipErr
		if err != nil {
			t.Fatal("expected PrepareZip to succeed:", err)
		}
	}

	if gotCommit != wantCommit {
		t.Errorf("fetched wrong commit. got=%v want=%v", gotCommit, wantCommit)
	}
	if gotRepo != wantRepo {
		t.Errorf("fetched wrong repo. got=%v want=%v", gotRepo, wantRepo)
	}

	// Wait for item to appear on disk cache, then test again to ensure we
	// use the disk cache.
	onDisk := false
	for range 500 {
		files, _ := os.ReadDir(s.Path)
		if len(files) != 0 {
			onDisk = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !onDisk {
		t.Fatal("timed out waiting for items to appear in cache at", s.Path)
	}
	_, err := s.PrepareZip(context.Background(), wantRepo, wantCommit, nil)
	if err != nil {
		t.Fatal("expected PrepareZip to succeed:", err)
	}
}

func TestPrepareZip_fetchTarFail(t *testing.T) {
	fetchErr := errors.New("test")
	s := tmpStore(t)
	s.FetchTar = func(ctx context.Context, repo api.RepoName, commit api.CommitID) (io.ReadCloser, error) {
		return nil, fetchErr
	}
	_, err := s.PrepareZip(context.Background(), "foo", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil)
	if !errors.Is(err, fetchErr) {
		t.Fatalf("expected PrepareZip to fail with %v, failed with %v", fetchErr, err)
	}
}

func TestPrepareZip_fetchTarReaderErr(t *testing.T) {
	fetchErr := errors.New("test")
	s := tmpStore(t)
	s.FetchTar = func(ctx context.Context, repo api.RepoName, commit api.CommitID) (io.ReadCloser, error) {
		r, w := io.Pipe()
		w.CloseWithError(fetchErr)
		return r, nil
	}
	_, err := s.PrepareZip(context.Background(), "foo", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil)
	if !errors.Is(err, fetchErr) {
		t.Fatalf("expected PrepareZip to fail with %v, failed with %v", fetchErr, err)
	}
}

func TestPrepareZip_errHeader(t *testing.T) {
	s := tmpStore(t)
	s.FetchTar = func(ctx context.Context, repo api.RepoName, commit api.CommitID) (io.ReadCloser, error) {
		buf := new(bytes.Buffer)
		w := tar.NewWriter(buf)
		w.Flush()
		buf.WriteString("oh yeah")
		err := w.Close()
		if err != nil {
			t.Fatal(err)
		}
		return io.NopCloser(bytes.NewReader(buf.Bytes())), nil
	}
	_, err := s.PrepareZip(context.Background(), "foo", "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef", nil)
	if have, want := errors.Cause(err).Error(), tar.ErrHeader.Error(); have != want {
		t.Fatalf("expected PrepareZip to fail with tar.ErrHeader, failed with %v", err)
	}
	if !errcode.IsTemporary(err) {
		t.Fatalf("expected PrepareZip to fail with a temporary error, failed with %v", err)
	}
}

func TestSearchLargeFiles(t *testing.T) {
	filter := newSearchableFilter(&schema.SiteConfiguration{
		SearchLargeFiles: []string{
			"foo",
			"foo.*",
			"foo_*",
			"*.foo",
			"bar.baz",
			"**/*.bam",
			"qu?.foo",
			"!qux.*",
			"**/quu?.foo",
			"!**/quux.foo",
			"!quuux.foo",
			"quuu?.foo",
			"\\!foo.baz",
			"!!foo.bam",
			"\\!!baz.foo",
		},
	})
	tests := []struct {
		name   string
		search bool
	}{
		// Pass
		{"foo", true},
		{"foo.bar", true},
		{"foo_bar", true},
		{"bar.baz", true},
		{"bar.foo", true},
		{"hello.bam", true},
		{"sub/dir/hello.bam", true},
		{"/sub/dir/hello.bam", true},

		// Pass - with negate meta character
		{"quuux.foo", true},
		{"!foo.baz", true},
		{"!!baz.foo", true},

		// Fail
		{"baz.foo.bar", false},
		{"bar_baz", false},
		{"baz.baz", false},

		// Fail - with negate meta character
		{"qux.foo", false},
		{"/sub/dir/quux.foo", false},
		{"!foo.bam", false},
	}

	for _, test := range tests {
		hdr := &tar.Header{
			Name: test.name,
			Size: maxFileSize + 1,
		}
		if got, want := filter.SkipContent(hdr), !test.search; got != want {
			t.Errorf("case %s got %v want %v", test.name, got, want)
		}
	}
}

func TestSymlink(t *testing.T) {
	dir := t.TempDir()
	if err := createSymlinkRepo(dir); err != nil {
		t.Fatal(err)
	}
	tarReader, err := tarArchive(filepath.Join(dir, "repo"))
	if err != nil {
		t.Fatal(err)
	}
	targetZip := filepath.Join(dir, "archive.zip")
	f, err := os.Create(targetZip)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)

	filter := newSearchableFilter(&schema.SiteConfiguration{})
	filter.CommitIgnore = func(hdr *tar.Header) bool {
		return false
	}
	if err := copySearchable(tarReader, zw, filter); err != nil {
		t.Fatal(err)
	}
	zw.Close()

	zr, err := zip.OpenReader(targetZip)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	cmpContent := func(f *zip.File, want string) {
		link, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b := bytes.Buffer{}
		io.Copy(&b, link)
		if got := strings.TrimRight(b.String(), "\n"); got != want {
			t.Fatalf("wanted \"%s\", got \"%s\"\n", want, got)
		}
	}

	for _, f := range zr.File {
		switch f.Name {
		case "asymlink":
			if f.Mode() != os.ModeSymlink {
				t.Fatalf("wanted %d, got %d", os.ModeSymlink, f.Mode())
			}
			cmpContent(f, "afile")
		case "afile":
			cmpContent(f, "acontent")
		default:
			t.Fatal("unreachable")
		}
	}
}

func createSymlinkRepo(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	script := `mkdir repo
cd repo
git init
git config user.email "you@example.com"
git config user.name "Your Name"
echo acontent > afile
ln -s afile asymlink
git add .
git commit -am amsg
`
	cmd := exec.Command("/bin/sh", "-euxc", script)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.Newf("execution error: %v, output %s", err, out)
	}
	return nil
}

func tarArchive(dir string) (*tar.Reader, error) {
	args := []string{
		"archive",
		"--worktree-attributes",
		"--format=tar",
		"HEAD",
		"--",
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	b := bytes.Buffer{}
	cmd.Stdout = &b
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return tar.NewReader(&b), nil
}

func tmpStore(t *testing.T) *Store {
	d := t.TempDir()
	return &Store{
		GitserverClient: gitserver.NewTestClient(t),
		Path:            d,
		Log:             logtest.Scoped(t),

		ObservationCtx: observation.TestContextTB(t),
	}
}

func emptyTar(t *testing.T) io.ReadCloser {
	buf := new(bytes.Buffer)
	w := tar.NewWriter(buf)
	err := w.Close()
	if err != nil {
		t.Fatal(err)
	}
	return io.NopCloser(bytes.NewReader(buf.Bytes()))
}
