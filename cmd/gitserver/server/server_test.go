package server

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/mutablelimiter"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
)

type Test struct {
	Name             string
	Request          *http.Request
	ExpectedCode     int
	ExpectedBody     string
	ExpectedTrailers http.Header
}

func TestRequest(t *testing.T) {
	tests := []Test{
		{
			Name:         "Command",
			Request:      httptest.NewRequest("POST", "/exec", strings.NewReader(`{"repo": "github.com/gorilla/mux", "args": ["testcommand"]}`)),
			ExpectedCode: http.StatusOK,
			ExpectedBody: "teststdout",
			ExpectedTrailers: http.Header{
				"X-Exec-Error":       {""},
				"X-Exec-Exit-Status": {"42"},
				"X-Exec-Stderr":      {"teststderr"},
			},
		},
		{
			Name:         "CommandWithURL",
			Request:      httptest.NewRequest("POST", "/exec", strings.NewReader(`{"repo": "my-mux", "url": "https://github.com/gorilla/mux.git", "args": ["testcommand"]}`)),
			ExpectedCode: http.StatusOK,
			ExpectedBody: "teststdout",
			ExpectedTrailers: http.Header{
				"X-Exec-Error":       {""},
				"X-Exec-Exit-Status": {"42"},
				"X-Exec-Stderr":      {"teststderr"},
			},
		},
		{
			Name:         "NonexistingRepo",
			Request:      httptest.NewRequest("POST", "/exec", strings.NewReader(`{"repo": "github.com/gorilla/doesnotexist", "args": ["testcommand"]}`)),
			ExpectedCode: http.StatusNotFound,
			ExpectedBody: `{"cloneInProgress":false}`,
		},
		{
			Name:         "NonexistingRepoWithURL",
			Request:      httptest.NewRequest("POST", "/exec", strings.NewReader(`{"repo": "my-doesnotexist", "url": "https://github.com/gorilla/doesntexist.git", "args": ["testcommand"]}`)),
			ExpectedCode: http.StatusNotFound,
			ExpectedBody: `{"cloneInProgress":false}`,
		},
		{
			Name:         "UnclonedRepoWithoutURL",
			Request:      httptest.NewRequest("POST", "/exec", strings.NewReader(`{"repo": "github.com/nicksnyder/go-i18n", "args": ["testcommand"]}`)),
			ExpectedCode: http.StatusNotFound,
			ExpectedBody: `{"cloneInProgress":true}`, // we now fetch the URL from GetRemoteURL so it works.
		},
		{
			Name:         "UnclonedRepoWithURL",
			Request:      httptest.NewRequest("POST", "/exec", strings.NewReader(`{"repo": "github.com/nicksnyder/go-i18n", "url": "https://github.com/nicksnyder/go-i18n.git", "args": ["testcommand"]}`)),
			ExpectedCode: http.StatusNotFound,
			ExpectedBody: `{"cloneInProgress":true}`,
		},
		{
			Name:         "Error",
			Request:      httptest.NewRequest("POST", "/exec", strings.NewReader(`{"repo": "github.com/gorilla/mux", "args": ["testerror"]}`)),
			ExpectedCode: http.StatusOK,
			ExpectedTrailers: http.Header{
				"X-Exec-Error":       {"testerror"},
				"X-Exec-Exit-Status": {"0"},
				"X-Exec-Stderr":      {""},
			},
		},
		{
			Name:         "EmptyBody",
			Request:      httptest.NewRequest("POST", "/exec", nil),
			ExpectedCode: http.StatusBadRequest,
			ExpectedBody: `EOF`,
		},
		{
			Name:         "EmptyInput",
			Request:      httptest.NewRequest("POST", "/exec", strings.NewReader("{}")),
			ExpectedCode: http.StatusNotFound,
			ExpectedBody: `{"cloneInProgress":false}`,
		},
	}

	s := &Server{
		ReposDir:          "/testroot",
		skipCloneForTests: true,
		GetRemoteURLFunc: func(ctx context.Context, name api.RepoName) (string, error) {
			return "https://" + string(name) + ".git", nil
		},
		GetVCSSyncer: func(ctx context.Context, name api.RepoName) (VCSSyncer, error) {
			return &GitRepoSyncer{}, nil
		},
	}
	h := s.Handler()

	origRepoCloned := repoCloned
	repoCloned = func(dir GitDir) bool {
		return dir == s.dir("github.com/gorilla/mux") || dir == s.dir("my-mux")
	}
	t.Cleanup(func() { repoCloned = origRepoCloned })

	testGitRepoExists = func(ctx context.Context, remoteURL *url.URL) error {
		if remoteURL.String() == "https://github.com/nicksnyder/go-i18n.git" {
			return nil
		}
		return errors.New("not cloneable")
	}
	t.Cleanup(func() { testGitRepoExists = nil })

	runCommandMock = func(ctx context.Context, cmd *exec.Cmd) (int, error) {
		switch cmd.Args[1] {
		case "testcommand":
			_, _ = cmd.Stdout.Write([]byte("teststdout"))
			_, _ = cmd.Stderr.Write([]byte("teststderr"))
			return 42, nil
		case "testerror":
			return 0, errors.New("testerror")
		}
		return 0, nil
	}
	t.Cleanup(func() { runCommandMock = nil })

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			w := httptest.ResponseRecorder{Body: new(bytes.Buffer)}
			h.ServeHTTP(&w, test.Request)

			res := w.Result()
			if res.StatusCode != test.ExpectedCode {
				t.Errorf("wrong status: expected %d, got %d", test.ExpectedCode, w.Code)
			}

			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(string(body)) != test.ExpectedBody {
				t.Errorf("wrong body: expected %q, got %q", test.ExpectedBody, string(body))
			}

			for k, v := range test.ExpectedTrailers {
				if got := res.Trailer.Get(k); got != v[0] {
					t.Errorf("wrong trailer %q: expected %q, got %q", k, v[0], got)
				}
			}
		})
	}
}

func TestServer_handleP4Exec(t *testing.T) {
	tests := []Test{
		{
			Name:         "Command",
			Request:      httptest.NewRequest("POST", "/p4-exec", strings.NewReader(`{"args": ["users"]}`)),
			ExpectedCode: http.StatusOK,
			ExpectedBody: "admin <admin@joe-perforce-server> (admin) accessed 2021/01/31",
			ExpectedTrailers: http.Header{
				"X-Exec-Error":       {""},
				"X-Exec-Exit-Status": {"42"},
				"X-Exec-Stderr":      {"teststderr"},
			},
		},
		{
			Name:         "Error",
			Request:      httptest.NewRequest("POST", "/p4-exec", strings.NewReader(`{"args": ["bad_command"]}`)),
			ExpectedCode: http.StatusBadRequest,
			ExpectedBody: "subcommand \"bad_command\" is not allowed",
		},
		{
			Name:         "EmptyBody",
			Request:      httptest.NewRequest("POST", "/p4-exec", nil),
			ExpectedCode: http.StatusBadRequest,
			ExpectedBody: `EOF`,
		},
		{
			Name:         "EmptyInput",
			Request:      httptest.NewRequest("POST", "/p4-exec", strings.NewReader("{}")),
			ExpectedCode: http.StatusBadRequest,
			ExpectedBody: `args must be greater than or equal to 1`,
		},
	}

	s := &Server{
		skipCloneForTests: true,
	}
	h := s.Handler()

	runCommandMock = func(ctx context.Context, cmd *exec.Cmd) (int, error) {
		switch cmd.Args[1] {
		case "users":
			_, _ = cmd.Stdout.Write([]byte("admin <admin@joe-perforce-server> (admin) accessed 2021/01/31"))
			_, _ = cmd.Stderr.Write([]byte("teststderr"))
			return 42, nil
		}
		return 0, nil
	}
	t.Cleanup(func() { runCommandMock = nil })

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			w := httptest.ResponseRecorder{Body: new(bytes.Buffer)}
			h.ServeHTTP(&w, test.Request)

			res := w.Result()
			if res.StatusCode != test.ExpectedCode {
				t.Errorf("wrong status: expected %d, got %d", test.ExpectedCode, w.Code)
			}

			body, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(string(body)) != test.ExpectedBody {
				t.Errorf("wrong body: expected %q, got %q", test.ExpectedBody, string(body))
			}

			for k, v := range test.ExpectedTrailers {
				if got := res.Trailer.Get(k); got != v[0] {
					t.Errorf("wrong trailer %q: expected %q, got %q", k, v[0], got)
				}
			}
		})
	}
}

func BenchmarkQuickRevParseHeadQuickSymbolicRefHead_packed_refs(b *testing.B) {
	tmp, err := ioutil.TempDir("", "gitserver_test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	dir := filepath.Join(tmp, ".git")
	gitDir := GitDir(dir)
	if err := os.Mkdir(dir, 0700); err != nil {
		b.Fatal(err)
	}

	masterRef := "refs/heads/master"
	// This simulates the most amount of work quickRevParseHead has to do, and
	// is also the most common in prod. That is where the final rev is in
	// packed-refs.
	err = ioutil.WriteFile(filepath.Join(dir, "HEAD"), []byte(fmt.Sprintf("ref: %s\n", masterRef)), 0600)
	if err != nil {
		b.Fatal(err)
	}
	// in prod the kubernetes repo has a packed-refs file that is 62446 lines
	// long. Simulate something like that with everything except master
	masterRev := "4d5092a09bca95e0153c423d76ef62d4fcd168ec"
	{
		f, err := os.Create(filepath.Join(dir, "packed-refs"))
		if err != nil {
			b.Fatal(err)
		}
		writeRef := func(refBase string, num int) {
			_, err := fmt.Fprintf(f, "%016x%016x%08x %s-%d\n", rand.Uint64(), rand.Uint64(), rand.Uint32(), refBase, num)
			if err != nil {
				b.Fatal(err)
			}
		}
		for i := 0; i < 32; i++ {
			writeRef("refs/heads/feature-branch", i)
		}
		_, err = fmt.Fprintf(f, "%s refs/heads/master\n", masterRev)
		if err != nil {
			b.Fatal(err)
		}
		for i := 0; i < 10000; i++ {
			// actual format is refs/pull/${i}/head, but doesn't actually
			// matter for testing
			writeRef("refs/pull/head", i)
			writeRef("refs/pull/merge", i)
		}
		err = f.Close()
		if err != nil {
			b.Fatal(err)
		}
	}

	// Exclude setup
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		rev, err := quickRevParseHead(gitDir)
		if err != nil {
			b.Fatal(err)
		}
		if rev != masterRev {
			b.Fatal("unexpected rev: ", rev)
		}
		ref, err := quickSymbolicRefHead(gitDir)
		if err != nil {
			b.Fatal(err)
		}
		if ref != masterRef {
			b.Fatal("unexpected ref: ", ref)
		}
	}

	// Exclude cleanup (defers)
	b.StopTimer()
}

func BenchmarkQuickRevParseHeadQuickSymbolicRefHead_unpacked_refs(b *testing.B) {
	tmp, err := ioutil.TempDir("", "gitserver_test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	dir := filepath.Join(tmp, ".git")
	gitDir := GitDir(dir)
	if err := os.Mkdir(dir, 0700); err != nil {
		b.Fatal(err)
	}

	// This simulates the usual case for a repo that HEAD is often
	// updated. The master ref will be unpacked.
	masterRef := "refs/heads/master"
	masterRev := "4d5092a09bca95e0153c423d76ef62d4fcd168ec"
	files := map[string]string{
		"HEAD":              fmt.Sprintf("ref: %s\n", masterRef),
		"refs/heads/master": masterRev + "\n",
	}
	for path, content := range files {
		path = filepath.Join(dir, path)
		err := os.MkdirAll(filepath.Dir(path), 0700)
		if err != nil {
			b.Fatal(err)
		}
		err = ioutil.WriteFile(path, []byte(content), 0600)
		if err != nil {
			b.Fatal(err)
		}
	}

	// Exclude setup
	b.ResetTimer()

	for n := 0; n < b.N; n++ {
		rev, err := quickRevParseHead(gitDir)
		if err != nil {
			b.Fatal(err)
		}
		if rev != masterRev {
			b.Fatal("unexpected rev: ", rev)
		}
		ref, err := quickSymbolicRefHead(gitDir)
		if err != nil {
			b.Fatal(err)
		}
		if ref != masterRef {
			b.Fatal("unexpected ref: ", ref)
		}
	}

	// Exclude cleanup (defers)
	b.StopTimer()
}

func TestUrlRedactor(t *testing.T) {
	testCases := []struct {
		url      string
		message  string
		redacted string
	}{
		{
			url:      "http://token@github.com/foo/bar/",
			message:  "fatal: repository 'http://token@github.com/foo/bar/' not found",
			redacted: "fatal: repository 'http://<redacted>@github.com/foo/bar/' not found",
		},
		{
			url:      "http://user:password@github.com/foo/bar/",
			message:  "fatal: repository 'http://user:password@github.com/foo/bar/' not found",
			redacted: "fatal: repository 'http://user:<redacted>@github.com/foo/bar/' not found",
		},
		{
			url:      "http://git:password@github.com/foo/bar/",
			message:  "fatal: repository 'http://git:password@github.com/foo/bar/' not found",
			redacted: "fatal: repository 'http://git:<redacted>@github.com/foo/bar/' not found",
		},
		{
			url:      "http://token@github.com///repo//nick/",
			message:  "fatal: repository 'http://token@github.com/foo/bar/' not found",
			redacted: "fatal: repository 'http://<redacted>@github.com/foo/bar/' not found",
		},
	}
	for _, testCase := range testCases {
		t.Run("", func(t *testing.T) {
			remoteURL, err := vcs.ParseURL(testCase.url)
			if err != nil {
				t.Fatal(err)
			}
			if actual := newURLRedactor(remoteURL).redact(testCase.message); actual != testCase.redacted {
				t.Fatalf("newUrlRedactor(%q).redact(%q) got %q; want %q", testCase.url, testCase.message, actual, testCase.redacted)
			}
		})
	}
}

func runCmd(t *testing.T, dir string, cmd string, arg ...string) string {
	t.Helper()
	c := exec.Command(cmd, arg...)
	c.Dir = dir
	c.Env = []string{
		"GIT_COMMITTER_NAME=a",
		"GIT_COMMITTER_EMAIL=a@a.com",
		"GIT_AUTHOR_NAME=a",
		"GIT_AUTHOR_EMAIL=a@a.com",
	}
	b, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %s\nOutput: %s", cmd, strings.Join(arg, " "), err, b)
	}
	return string(b)
}

func staticGetRemoteURL(remote string) func(context.Context, api.RepoName) (string, error) {
	return func(context.Context, api.RepoName) (string, error) {
		return remote, nil
	}
}

func TestCloneRepo(t *testing.T) {
	ctx := context.Background()
	remote := tmpDir(t)
	repoName := api.RepoName("example.com/foo/bar")
	db := dbtesting.GetDB(t)

	dbRepo := &types.Repo{
		Name:        repoName,
		Description: "Test",
	}
	// Insert the repo into our database
	if err := database.Repos(db).Create(ctx, dbRepo); err != nil {
		t.Fatal(err)
	}
	assertCloneStatus := func(status types.CloneStatus) {
		t.Helper()
		fromDB, err := database.GitserverRepos(db).GetByID(ctx, dbRepo.ID)
		if err != nil {
			t.Fatal(err)
		}
		if fromDB.CloneStatus != status {
			t.Fatalf("Want %q, got %q", status, fromDB.CloneStatus)
		}
	}

	if err := database.GitserverRepos(db).Upsert(ctx, &types.GitserverRepo{
		RepoID:      dbRepo.ID,
		ShardID:     "test",
		CloneStatus: types.CloneStatusNotCloned,
	}); err != nil {
		t.Fatal(err)
	}
	assertCloneStatus(types.CloneStatusNotCloned)

	repo := remote
	cmd := func(name string, arg ...string) string {
		t.Helper()
		return runCmd(t, repo, name, arg...)
	}

	// Setup a repo with a commit so we can see if we can clone it.
	cmd("git", "init", ".")
	cmd("sh", "-c", "echo hello world > hello.txt")
	cmd("git", "add", "hello.txt")
	cmd("git", "commit", "-m", "hello")
	wantCommit := cmd("git", "rev-parse", "HEAD")
	// Add a bad tag
	cmd("git", "tag", "HEAD")

	reposDir := tmpDir(t)

	s := &Server{
		ReposDir:         reposDir,
		GetRemoteURLFunc: staticGetRemoteURL(remote),
		GetVCSSyncer: func(ctx context.Context, name api.RepoName) (VCSSyncer, error) {
			return &GitRepoSyncer{}, nil
		},
		DB:               db,
		ctx:              context.Background(),
		locker:           &RepositoryLocker{},
		cloneLimiter:     mutablelimiter.New(1),
		cloneableLimiter: mutablelimiter.New(1),
	}
	_, err := s.cloneRepo(ctx, repoName, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Wait until the clone is done. Please do not use this code snippet
	// outside of a test. We only know this works since our test only starts
	// one clone and will have nothing else attempt to lock.
	dst := s.dir(repoName)
	for i := 0; i < 1000; i++ {
		_, cloning := s.locker.Status(dst)
		if !cloning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assertCloneStatus(types.CloneStatusCloned)

	repo = filepath.Dir(string(dst))
	gotCommit := cmd("git", "rev-parse", "HEAD")
	if wantCommit != gotCommit {
		t.Fatal("failed to clone:", gotCommit)
	}

	// Test blocking with a failure (already exists since we didn't specify overwrite)
	_, err = s.cloneRepo(context.Background(), repoName, &cloneOptions{Block: true})
	if !os.IsExist(errors.Cause(err)) {
		t.Fatalf("expected clone repo to fail with already exists: %s", err)
	}
	assertCloneStatus(types.CloneStatusCloned)

	// Test blocking with overwrite. First add random file to GIT_DIR. If the
	// file is missing after cloning we know the directory was replaced
	mkFiles(t, string(dst), "HELLO")
	_, err = s.cloneRepo(context.Background(), repoName, &cloneOptions{Block: true, Overwrite: true})
	if err != nil {
		t.Fatal(err)
	}
	assertCloneStatus(types.CloneStatusCloned)

	if _, err := os.Stat(dst.Path("HELLO")); !os.IsNotExist(err) {
		t.Fatalf("expected clone to be overwritten: %s", err)
	}

	repo = filepath.Dir(string(dst))
	gotCommit = cmd("git", "rev-parse", "HEAD")
	if wantCommit != gotCommit {
		t.Fatal("failed to clone:", gotCommit)
	}
}

func TestRemoveBadRefs(t *testing.T) {
	dir := tmpDir(t)
	gitDir := GitDir(filepath.Join(dir, ".git"))

	cmd := func(name string, arg ...string) string {
		t.Helper()
		return runCmd(t, dir, name, arg...)
	}

	// Setup a repo with a commit so we can add bad refs
	cmd("git", "init", ".")
	cmd("sh", "-c", "echo hello world > hello.txt")
	cmd("git", "add", "hello.txt")
	cmd("git", "commit", "-m", "hello")
	want := cmd("git", "rev-parse", "HEAD")

	for _, name := range []string{"HEAD", "head", "Head", "HeAd"} {
		// Tag
		cmd("git", "tag", name)

		if dontWant := cmd("git", "rev-parse", "HEAD"); dontWant == want {
			t.Logf("WARNING: git tag %s failed to produce ambiguous output: %s", name, dontWant)
		}

		removeBadRefs(context.Background(), gitDir)

		if got := cmd("git", "rev-parse", "HEAD"); got != want {
			t.Fatalf("git tag %s failed to be removed: %s", name, got)
		}

		// Ref
		if err := ioutil.WriteFile(filepath.Join(dir, ".git", "refs", "heads", name), []byte(want), 0600); err != nil {
			t.Fatal(err)
		}

		if dontWant := cmd("git", "rev-parse", "HEAD"); dontWant == want {
			t.Logf("WARNING: git ref %s failed to produce ambiguous output: %s", name, dontWant)
		}

		removeBadRefs(context.Background(), gitDir)

		if got := cmd("git", "rev-parse", "HEAD"); got != want {
			t.Fatalf("git ref %s failed to be removed: %s", name, got)
		}
	}
}

func TestCloneRepo_EnsureValidity(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("with no remote HEAD file", func(t *testing.T) {
		var (
			remote   = tmpDir(t)
			reposDir = tmpDir(t)
			cmd      = func(name string, arg ...string) string {
				t.Helper()
				return runCmd(t, remote, name, arg...)
			}
		)

		cmd("git", "init", ".")
		cmd("rm", ".git/HEAD")

		server := &Server{
			ReposDir:         reposDir,
			GetRemoteURLFunc: staticGetRemoteURL(remote),
			GetVCSSyncer: func(ctx context.Context, name api.RepoName) (VCSSyncer, error) {
				return &GitRepoSyncer{}, nil
			},
			ctx:              ctx,
			locker:           &RepositoryLocker{},
			cloneLimiter:     mutablelimiter.New(1),
			cloneableLimiter: mutablelimiter.New(1),
		}
		if _, err := server.cloneRepo(ctx, "example.com/foo/bar", nil); err == nil {
			t.Fatal("expected an error, got none")
		}
	})
	t.Run("with an empty remote HEAD file", func(t *testing.T) {
		var (
			remote   = tmpDir(t)
			reposDir = tmpDir(t)
			cmd      = func(name string, arg ...string) string {
				t.Helper()
				return runCmd(t, remote, name, arg...)
			}
		)

		cmd("git", "init", ".")
		cmd("sh", "-c", ": > .git/HEAD")

		server := &Server{
			ReposDir:         reposDir,
			GetRemoteURLFunc: staticGetRemoteURL(remote),
			GetVCSSyncer: func(ctx context.Context, name api.RepoName) (VCSSyncer, error) {
				return &GitRepoSyncer{}, nil
			},
			ctx:              ctx,
			locker:           &RepositoryLocker{},
			cloneLimiter:     mutablelimiter.New(1),
			cloneableLimiter: mutablelimiter.New(1),
		}
		if _, err := server.cloneRepo(ctx, "example.com/foo/bar", nil); err == nil {
			t.Fatal("expected an error, got none")
		}
	})
	t.Run("with no local HEAD file", func(t *testing.T) {
		var (
			remote   = tmpDir(t)
			reposDir = tmpDir(t)
			cmd      = func(name string, arg ...string) string {
				t.Helper()
				return runCmd(t, remote, name, arg...)
			}
		)

		cmd("git", "init", ".")
		cmd("sh", "-c", "echo hello world > hello.txt")
		cmd("git", "add", "hello.txt")
		cmd("git", "commit", "-m", "hello")

		s := &Server{
			ReposDir:         reposDir,
			GetRemoteURLFunc: staticGetRemoteURL(remote),
			GetVCSSyncer: func(ctx context.Context, name api.RepoName) (VCSSyncer, error) {
				return &GitRepoSyncer{}, nil
			},
			ctx:              ctx,
			locker:           &RepositoryLocker{},
			cloneLimiter:     mutablelimiter.New(1),
			cloneableLimiter: mutablelimiter.New(1),
		}
		testRepoCorrupter = func(_ context.Context, tmpDir GitDir) {
			cmd("sh", "-c", fmt.Sprintf("rm %s/HEAD", tmpDir))
		}
		t.Cleanup(func() { testRepoCorrupter = nil })
		if _, err := s.cloneRepo(ctx, "example.com/foo/bar", nil); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		dst := s.dir("example.com/foo/bar")
		for i := 0; i < 1000; i++ {
			_, cloning := s.locker.Status(dst)
			if !cloning {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		head, err := ioutil.ReadFile(fmt.Sprintf("%s/HEAD", dst))
		if os.IsNotExist(err) {
			t.Fatal("expected a reconstituted HEAD, but no file exists")
		}
		if head == nil {
			t.Fatal("expected a reconstituted HEAD, but the file is empty")
		}
	})
	t.Run("with an empty local HEAD file", func(t *testing.T) {
		var (
			remote   = tmpDir(t)
			reposDir = tmpDir(t)
			cmd      = func(name string, arg ...string) string {
				t.Helper()
				return runCmd(t, remote, name, arg...)
			}
		)

		cmd("git", "init", ".")
		cmd("sh", "-c", "echo hello world > hello.txt")
		cmd("git", "add", "hello.txt")
		cmd("git", "commit", "-m", "hello")

		s := &Server{
			ReposDir:         reposDir,
			GetRemoteURLFunc: staticGetRemoteURL(remote),
			GetVCSSyncer: func(ctx context.Context, name api.RepoName) (VCSSyncer, error) {
				return &GitRepoSyncer{}, nil
			},
			ctx:              ctx,
			locker:           &RepositoryLocker{},
			cloneLimiter:     mutablelimiter.New(1),
			cloneableLimiter: mutablelimiter.New(1),
		}
		testRepoCorrupter = func(_ context.Context, tmpDir GitDir) {
			cmd("sh", "-c", fmt.Sprintf(": > %s/HEAD", tmpDir))
		}
		t.Cleanup(func() { testRepoCorrupter = nil })
		if _, err := s.cloneRepo(ctx, "example.com/foo/bar", nil); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		dst := s.dir("example.com/foo/bar")
		for i := 0; i < 1000; i++ {
			_, cloning := s.locker.Status(dst)
			if !cloning {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}

		head, err := ioutil.ReadFile(fmt.Sprintf("%s/HEAD", dst))
		if os.IsNotExist(err) {
			t.Fatal("expected a reconstituted HEAD, but no file exists")
		}
		if head == nil {
			t.Fatal("expected a reconstituted HEAD, but the file is empty")
		}
	})
}

func TestHostnameMatch(t *testing.T) {
	testCases := []struct {
		hostname    string
		addr        string
		shouldMatch bool
	}{
		{
			hostname:    "gitserver-1",
			addr:        "gitserver-1",
			shouldMatch: true,
		},
		{
			hostname:    "gitserver-1",
			addr:        "gitserver-1.gitserver:3178",
			shouldMatch: true,
		},
		{
			hostname:    "gitserver-1",
			addr:        "gitserver-10.gitserver:3178",
			shouldMatch: false,
		},
		{
			hostname:    "gitserver-1",
			addr:        "gitserver-10",
			shouldMatch: false,
		},
		{
			hostname:    "gitserver-10",
			addr:        "",
			shouldMatch: false,
		},
		{
			hostname:    "gitserver-10",
			addr:        "gitserver-10:3178",
			shouldMatch: true,
		},
		{
			hostname:    "gitserver-10",
			addr:        "gitserver-10:3178",
			shouldMatch: true,
		},
		{
			hostname:    "gitserver-0.prod",
			addr:        "gitserver-0.prod.default.namespace",
			shouldMatch: true,
		},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			s := Server{Hostname: tc.hostname}
			have := s.hostnameMatch(tc.addr)
			if have != tc.shouldMatch {
				t.Fatalf("Want %v, got %v", tc.shouldMatch, have)
			}
		})
	}
}

func TestSyncRepoState(t *testing.T) {
	ctx := context.Background()
	db := dbtesting.GetDB(t)
	remoteDir := tmpDir(t)

	cmd := func(name string, arg ...string) string {
		t.Helper()
		return runCmd(t, remoteDir, name, arg...)
	}

	// Setup a repo with a commit so we can see if we can clone it.
	cmd("git", "init", ".")
	cmd("sh", "-c", "echo hello world > hello.txt")
	cmd("git", "add", "hello.txt")
	cmd("git", "commit", "-m", "hello")

	reposDir := tmpDir(t)
	repoName := api.RepoName("example.com/foo/bar")
	hostname := "test"

	s := &Server{
		ReposDir:         reposDir,
		GetRemoteURLFunc: staticGetRemoteURL(remoteDir),
		GetVCSSyncer: func(ctx context.Context, name api.RepoName) (VCSSyncer, error) {
			return &GitRepoSyncer{}, nil
		},
		Hostname:         hostname,
		DB:               db,
		ctx:              ctx,
		locker:           &RepositoryLocker{},
		cloneLimiter:     mutablelimiter.New(1),
		cloneableLimiter: mutablelimiter.New(1),
	}

	_, err := s.cloneRepo(ctx, repoName, &cloneOptions{Block: true})
	if err != nil {
		t.Fatal(err)
	}

	dbRepo := &types.Repo{
		Name:        repoName,
		URI:         string(repoName),
		Description: "Test",
	}

	// Insert the repo into our database
	err = database.Repos(db).Create(ctx, dbRepo)
	if err != nil {
		t.Fatal(err)
	}

	_, err = database.GitserverRepos(db).GetByID(ctx, dbRepo.ID)
	if err == nil {
		// GitserverRepo should not exist
		t.Fatal("Expected an error")
	}

	err = s.syncRepoState([]string{hostname}, 10, 10)
	if err != nil {
		t.Fatal(err)
	}

	gr, err := database.GitserverRepos(db).GetByID(ctx, dbRepo.ID)
	if err != nil {
		t.Fatal(err)
	}

	if gr.CloneStatus != types.CloneStatusCloned {
		t.Fatalf("Want %v, got %v", types.CloneStatusCloned, gr.CloneStatus)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	if !testing.Verbose() {
		log15.Root().SetHandler(log15.DiscardHandler())
	}
	os.Exit(m.Run())
}
