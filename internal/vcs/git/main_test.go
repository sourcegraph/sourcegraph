package git

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/server"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
)

var root string

func TestMain(m *testing.M) {
	flag.Parse()

	if !testing.Verbose() {
		log15.Root().SetHandler(log15.DiscardHandler())
	}

	code := m.Run()

	_ = os.RemoveAll(root)

	os.Exit(code)
}

// done in init since the go vet analysis "ctrlflow" is tripped up if this is
// done as part of TestMain.
func init() {
	// Ignore users configuration in tests
	os.Setenv("GIT_CONFIG_NOSYSTEM", "true")
	os.Setenv("HOME", "/dev/null")

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("listen failed: %s", err)
	}

	root, err = ioutil.TempDir("", "test")
	if err != nil {
		log.Fatal(err)
	}

	srv := &http.Server{
		Handler: (&server.Server{
			ReposDir: filepath.Join(root, "repos"),
			GetRemoteURLFunc: func(ctx context.Context, name api.RepoName) (string, error) {
				return filepath.Join(root, "remotes", string(name)), nil
			},
			GetVCSSyncer: func(ctx context.Context, name api.RepoName) (server.VCSSyncer, error) {
				return &server.GitRepoSyncer{}, nil
			},
		}).Handler(),
	}
	go func() {
		if err := srv.Serve(l); err != nil {
			log.Fatal(err)
		}
	}()

	gitserver.DefaultClient.Addrs = func() []string {
		return []string{l.Addr().String()}
	}
}

func AsJSON(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b)
}

var Times = []string{
	AppleTime("2006-01-02T15:04:05Z"),
	AppleTime("2014-05-06T19:20:21Z"),
}

var NonExistentCommitID = api.CommitID(strings.Repeat("a", 40))

// InitGitRepository initializes a new Git repository and runs cmds in a new
// temporary directory (returned as dir).
func InitGitRepository(t testing.TB, cmds ...string) string {
	t.Helper()
	remotes := filepath.Join(root, "remotes")
	if err := os.MkdirAll(remotes, 0700); err != nil {
		t.Fatal(err)
	}
	dir, err := ioutil.TempDir(remotes, strings.Replace(t.Name(), "/", "__", -1))
	if err != nil {
		t.Fatal(err)
	}
	cmds = append([]string{"git init"}, cmds...)
	for _, cmd := range cmds {
		out, err := GitCommand(dir, "bash", "-c", cmd).CombinedOutput()
		if err != nil {
			t.Fatalf("Command %q failed. Output was:\n\n%s", cmd, out)
		}
	}
	return dir
}

func GitCommand(dir, name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Env = append(c.Env, "GIT_CONFIG="+path.Join(dir, ".git", "config"))
	return c
}

// MakeGitRepository calls initGitRepository to create a new Git repository and returns a handle to
// it.
func MakeGitRepository(t testing.TB, cmds ...string) api.RepoName {
	t.Helper()
	dir := InitGitRepository(t, cmds...)
	repo := api.RepoName(filepath.Base(dir))
	if resp, err := gitserver.DefaultClient.RequestRepoUpdate(context.Background(), repo, 0); err != nil {
		t.Fatal(err)
	} else if resp.Error != "" {
		t.Fatal(resp.Error)
	}
	return repo
}

func CommitsEqual(a, b *Commit) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a.Author.Date != b.Author.Date {
		return false
	}
	a.Author.Date = b.Author.Date
	if ac, bc := a.Committer, b.Committer; ac != nil && bc != nil {
		if ac.Date != bc.Date {
			return false
		}
		ac.Date = bc.Date
	} else if !(ac == nil && bc == nil) {
		return false
	}
	return reflect.DeepEqual(a, b)
}

func MustParseTime(layout, value string) time.Time {
	tm, err := time.Parse(layout, value)
	if err != nil {
		panic(err.Error())
	}
	return tm
}

func AppleTime(t string) string {
	ti, _ := time.Parse(time.RFC3339, t)
	return ti.Local().Format("200601021504.05")
}

// Computes hash of last commit in a given repo dir
// On Windows, content of a "link file" differs based on the tool that produced it.
// For example:
// - Cygwin may create four different link types, see https://cygwin.com/cygwin-ug-net/using.html#pathnames-symlinks,
// - MSYS's ln copies target file
// Such behavior makes impossible precalculation of SHA hashes to be used in TestRepository_FileSystem_Symlinks
// because for example Git for Windows (http://git-scm.com) is not aware of symlinks and computes link file's SHA which
// may differ from original file content's SHA.
// As a temporary workaround, we calculating SHA hash by asking git/hg to compute it
func ComputeCommitHash(repoDir string, git bool) string {
	buf := &bytes.Buffer{}

	if git {
		// git cat-file tree "master^{commit}" | git hash-object -t commit --stdin
		cat := exec.Command("git", "cat-file", "commit", "master^{commit}")
		cat.Dir = repoDir
		hash := exec.Command("git", "hash-object", "-t", "commit", "--stdin")
		hash.Stdin, _ = cat.StdoutPipe()
		hash.Stdout = buf
		hash.Dir = repoDir
		_ = hash.Start()
		_ = cat.Run()
		_ = hash.Wait()
	} else {
		hash := exec.Command("hg", "--debug", "id", "-i")
		hash.Dir = repoDir
		hash.Stdout = buf
		_ = hash.Run()
	}
	return strings.TrimSpace(buf.String())
}
