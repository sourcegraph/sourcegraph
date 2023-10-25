package git

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestGetObject(t *testing.T) {
	sampleSHA := "a03384f3a47acae11478ba7b4a6f331564938d4f"
	sampleOID, err := decodeOID(sampleSHA)
	if err != nil {
		t.Fatal(err)
	}
	repoName := api.RepoName("github.com/sourcegraph/sourcegraph")

	tests := []struct {
		name string

		revParse      revParseFunc
		getObjectType getObjectTypeFunc

		repo       api.RepoName
		objectName string
		wantObject *gitdomain.GitObject
		wantError  error
	}{
		{
			name: "Happy path",
			revParse: func(ctx context.Context, rcf *wrexec.RecordingCommandFactory, repo api.RepoName, dir common.GitDir, rev string) (string, error) {
				return sampleSHA, nil
			},
			getObjectType: func(ctx context.Context, rcf *wrexec.RecordingCommandFactory, repo api.RepoName, dir common.GitDir, objectID string) (gitdomain.ObjectType, error) {
				return gitdomain.ObjectTypeCommit, nil
			},
			repo:       repoName,
			objectName: "abc",
			wantObject: &gitdomain.GitObject{ID: sampleOID, Type: gitdomain.ObjectTypeCommit},
			wantError:  nil,
		},
		{
			name: "Revparse repo doesn't exist",
			revParse: func(ctx context.Context, rcf *wrexec.RecordingCommandFactory, repo api.RepoName, dir common.GitDir, rev string) (string, error) {
				return "", &gitdomain.RepoNotExistError{}
			},
			getObjectType: func(ctx context.Context, rcf *wrexec.RecordingCommandFactory, repo api.RepoName, dir common.GitDir, objectID string) (gitdomain.ObjectType, error) {
				return gitdomain.ObjectTypeCommit, nil
			},
			repo:       repoName,
			objectName: "abc",
			wantObject: nil,
			wantError:  &gitdomain.RepoNotExistError{},
		},
		{
			name: "Unknown revision",
			revParse: func(ctx context.Context, rcf *wrexec.RecordingCommandFactory, repo api.RepoName, dir common.GitDir, rev string) (string, error) {
				return "unknown revision: foo", errors.New("unknown revision")
			},
			getObjectType: func(ctx context.Context, rcf *wrexec.RecordingCommandFactory, repo api.RepoName, dir common.GitDir, objectID string) (gitdomain.ObjectType, error) {
				return gitdomain.ObjectTypeCommit, nil
			},
			repo:       repoName,
			objectName: "abc",
			wantObject: nil,
			wantError: &gitdomain.RevisionNotFoundError{
				Repo: repoName,
				Spec: "abc",
			},
		},
		{
			name: "HEAD treated as revision not found",
			revParse: func(ctx context.Context, rcf *wrexec.RecordingCommandFactory, repo api.RepoName, dir common.GitDir, rev string) (string, error) {
				return "HEAD", nil
			},
			getObjectType: func(ctx context.Context, rcf *wrexec.RecordingCommandFactory, repo api.RepoName, dir common.GitDir, objectID string) (gitdomain.ObjectType, error) {
				return gitdomain.ObjectTypeCommit, nil
			},
			repo:       repoName,
			objectName: "abc",
			wantObject: nil,
			wantError: &gitdomain.RevisionNotFoundError{
				Repo: repoName,
				Spec: "abc",
			},
		},
		{
			name: "Bad commit",
			revParse: func(ctx context.Context, rcf *wrexec.RecordingCommandFactory, repo api.RepoName, dir common.GitDir, rev string) (string, error) {
				return "not_valid_commit", nil
			},
			getObjectType: func(ctx context.Context, rcf *wrexec.RecordingCommandFactory, repo api.RepoName, dir common.GitDir, objectID string) (gitdomain.ObjectType, error) {
				return gitdomain.ObjectTypeCommit, nil
			},
			repo:       repoName,
			objectName: "abc",
			wantObject: nil,
			wantError: &gitdomain.BadCommitError{
				Repo:   repoName,
				Spec:   "abc",
				Commit: api.CommitID("not_valid_commit"),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			obj, err := getObject(ctx, wrexec.NewNoOpRecordingCommandFactory(), t.TempDir(), tc.getObjectType, tc.revParse, tc.repo, tc.objectName)
			if diff := cmp.Diff(tc.wantObject, obj); diff != "" {
				t.Errorf("Object does not match: %v", diff)
			}
			if diff := cmp.Diff(tc.wantError, err); diff != "" {
				t.Errorf("Error does not match: %v", diff)
			}
		})
	}
}

func TestGetObjectType(t *testing.T) {
	ctx := context.Background()
	rcf := wrexec.NewNoOpRecordingCommandFactory()
	reposDir := t.TempDir()

	// Make a new repo on disk.
	p := filepath.Join(reposDir, "repo", ".git")
	require.NoError(t, os.MkdirAll(p, os.ModePerm))
	dir := common.GitDir(p)

	runCommand(t, p, "git", "init")
	require.NoError(t, os.WriteFile(filepath.Join(p, "fileName1"), []byte("testfile"), os.ModePerm))
	runCommand(t, p, "git", "add", "fileName1")
	runCommand(t, p, "git", "commit", "-m", "commit1", "--author='a <a@a.com>'", "--date", "2006-01-02T15:04:05Z")
	tag := "v1.0.0"
	runCommand(t, p, "git", "tag", tag)

	out := runCommand(t, p, "git", "rev-parse", "HEAD")
	commitSha := string(bytes.TrimSpace(out))

	// Commit.
	{
		oid, err := revParse(ctx, rcf, "testrepo", dir, commitSha)
		require.NoError(t, err)
		gitStart := time.Now()
		out := runCommand(t, p, "git", "cat-file", "-t", "--", oid)
		t.Log("git took", time.Since(gitStart))
		want := gitdomain.ObjectType(bytes.TrimSpace(out))

		goGitStart := time.Now()
		have, err := getObjectType(ctx, rcf, "testrepo", dir, oid)
		t.Log("go git took", time.Since(goGitStart))
		require.NoError(t, err)
		require.Equal(t, want, have)
	}

	// Blob.
	{

	}

	// Tree.
	{

	}

	// Tag.
	{
		oid, err := revParse(ctx, rcf, "testrepo", dir, commitSha)
		require.NoError(t, err)
		gitStart := time.Now()
		out := runCommand(t, p, "git", "cat-file", "-t", "--", oid)
		t.Log("git took", time.Since(gitStart))
		want := gitdomain.ObjectType(bytes.TrimSpace(out))

		goGitStart := time.Now()
		have, err := getObjectType(ctx, rcf, "testrepo", dir, oid)
		t.Log("go git took", time.Since(goGitStart))
		require.NoError(t, err)
		require.Equal(t, want, have)
	}
}

func TestRevParse(t *testing.T) {
	ctx := context.Background()
	rcf := wrexec.NewNoOpRecordingCommandFactory()
	reposDir := t.TempDir()

	// Make a new repo on disk.
	p := filepath.Join(reposDir, "repo", ".git")
	require.NoError(t, os.MkdirAll(p, os.ModePerm))
	dir := common.GitDir(p)

	runCommand(t, p, "git", "init")
	require.NoError(t, os.WriteFile(filepath.Join(p, "fileName1"), []byte("testfile"), os.ModePerm))
	runCommand(t, p, "git", "add", "fileName1")
	runCommand(t, p, "git", "commit", "-m", "commit1", "--author='a <a@a.com>'", "--date", "2006-01-02T15:04:05Z")
	gitStart := time.Now()
	out := runCommand(t, p, "git", "rev-parse", "HEAD")
	t.Log("git took", time.Since(gitStart))
	want := string(bytes.TrimSpace(out))

	goGitStart := time.Now()
	have, err := revParse(ctx, rcf, "testrepo", dir, "HEAD")
	t.Log("go git took", time.Since(goGitStart))
	require.NoError(t, err)
	require.Equal(t, want, have)
}

func runCommand(t *testing.T, repoDir string, executable string, args ...string) []byte {
	cmd := exec.Command(executable, args...)
	cmd.Dir = repoDir
	// set some commiter env vars.
	cmd.Env = append(os.Environ(), "GIT_COMMITTER_NAME=a", "GIT_COMMITTER_EMAIL=a@a.com", "GIT_COMMITTER_DATE=2006-01-02T15:04:05Z")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)
	return out
}
