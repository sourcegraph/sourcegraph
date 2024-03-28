package gitserver

import (
	"bytes"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/api"
)

// CreateRepoDir creates a repo directory for testing purposes.
// This includes creating a tmp dir and deleting it after test finishes running.
func CreateRepoDir(t *testing.T) string {
	return CreateRepoDirWithName(t, "")
}

// CreateRepoDirWithName creates a repo directory with a given name for testing purposes.
// This includes creating a tmp dir and deleting it after test finishes running.
func CreateRepoDirWithName(t *testing.T, name string) string {
	t.Helper()
	if name == "" {
		name = t.Name()
	}
	name = strings.ReplaceAll(name, "/", "-")
	root, err := os.MkdirTemp("", name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		os.RemoveAll(root)
	})
	return root
}

func MustParseTime(layout, value string) time.Time {
	tm, err := time.Parse(layout, value)
	if err != nil {
		panic(err.Error())
	}
	return tm
}

// MakeGitRepository calls initGitRepository to create a new Git repository and returns a handle to
// it.
func MakeGitRepository(t *testing.T, cmds ...string) api.RepoName {
	t.Helper()
	dir := InitGitRepository(t, cmds...)
	repo := api.RepoName(filepath.Base(dir))
	return repo
}

// MakeGitRepositoryAndReturnDir calls initGitRepository to create a new Git repository and returns
// the repo name and directory.
func MakeGitRepositoryAndReturnDir(t *testing.T, cmds ...string) (api.RepoName, string) {
	t.Helper()
	dir := InitGitRepository(t, cmds...)
	repo := api.RepoName(filepath.Base(dir))
	return repo, dir
}

func GetHeadCommitFromGitDir(t *testing.T, gitDir string) string {
	t.Helper()
	cmd := CreateGitCommand(gitDir, "bash", []string{"-c", "git rev-parse HEAD"}...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command %q failed. Output was: %s, Error: %+v\n ", cmd, out, err)
	}
	return strings.Trim(string(out), "\n")
}

// InitGitRepository initializes a new Git repository and runs commands in a new
// temporary directory (returned as dir).
// It also sets ClientMocks.LocalGitCommandReposDir for successful run of local git commands.
func InitGitRepository(t *testing.T, cmds ...string) string {
	t.Helper()
	root := CreateRepoDir(t)
	remotes := filepath.Join(root, "remotes")
	if err := os.MkdirAll(remotes, 0o700); err != nil {
		t.Fatal(err)
	}
	dir, err := os.MkdirTemp(remotes, strings.ReplaceAll(t.Name(), "/", "__"))
	if err != nil {
		t.Fatal(err)
	}

	// setting git repo which is needed for successful run of git command against local file system
	ClientMocks.LocalGitCommandReposDir = remotes

	cmds = append([]string{"git init --initial-branch=master"}, cmds...)
	for _, cmd := range cmds {
		out, err := CreateGitCommand(dir, "bash", "-c", cmd).CombinedOutput()
		if err != nil {
			t.Fatalf("Command %q failed. Output was:\n\n%s", cmd, out)
		}
	}
	return dir
}

func CreateGitCommand(dir, name string, args ...string) *exec.Cmd {
	c := exec.Command(name, args...)
	c.Dir = dir
	c.Env = []string{
		"GIT_CONFIG=" + path.Join(dir, ".git", "config"),
		"GIT_COMMITTER_NAME=a",
		"GIT_COMMITTER_EMAIL=a@a.com",
		"GIT_COMMITTER_DATE=2006-01-02T15:04:05Z",
		"GIT_AUTHOR_NAME=a",
		"GIT_AUTHOR_EMAIL=a@a.com",
		"GIT_AUTHOR_DATE=2006-01-02T15:04:05Z",
	}
	if systemPath, ok := os.LookupEnv("PATH"); ok {
		c.Env = append(c.Env, "PATH="+systemPath)
	}
	return c
}

func appleTime(t string) string {
	ti, _ := time.Parse(time.RFC3339, t)
	return ti.Local().Format("200601021504.05")
}

var times = []string{
	appleTime("2006-01-02T15:04:05Z"),
	appleTime("2014-05-06T19:20:21Z"),
}

// ComputeCommitHash Computes hash of last commit in a given repo dir
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
