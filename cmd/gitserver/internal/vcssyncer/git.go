package vcssyncer

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/executil"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/urlredactor"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// gitRepoSyncer is a syncer for Git repositories.
type gitRepoSyncer struct {
	logger                  log.Logger
	recordingCommandFactory *wrexec.RecordingCommandFactory
}

func NewGitRepoSyncer(logger log.Logger, r *wrexec.RecordingCommandFactory) *gitRepoSyncer {
	return &gitRepoSyncer{logger: logger.Scoped("GitRepoSyncer"), recordingCommandFactory: r}
}

func (s *gitRepoSyncer) Type() string {
	return "git"
}

// TestGitRepoExists is a test fixture that overrides the return value for
// GitRepoSyncer.IsCloneable when it is set.
var TestGitRepoExists func(ctx context.Context, remoteURL *vcs.URL) error

// IsCloneable checks to see if the Git remote URL is cloneable.
func (s *gitRepoSyncer) IsCloneable(ctx context.Context, repoName api.RepoName, remoteURL *vcs.URL) error {
	if isAlwaysCloningTestRemoteURL(remoteURL) {
		return nil
	}
	if TestGitRepoExists != nil {
		return TestGitRepoExists(ctx, remoteURL)
	}

	args := []string{"ls-remote", remoteURL.String(), "HEAD"}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	r := urlredactor.New(remoteURL)
	cmd := exec.CommandContext(ctx, "git", args...)

	// Configure the command to be able to talk to a remote.
	executil.ConfigureRemoteGitCommand(cmd, remoteURL)

	out, err := s.recordingCommandFactory.WrapWithRepoName(ctx, log.NoOp(), repoName, cmd).WithRedactorFunc(r.Redact).CombinedOutput()
	if err != nil {
		if ctxerr := ctx.Err(); ctxerr != nil {
			err = ctxerr
		}
		if len(out) > 0 {
			err = errors.Wrap(err, "failed to check remote access: "+string(out))
		}
		return err
	}
	return nil
}

// Clone clones a Git repository into tmpPath, reporting redacted progress logs
// via the progressWriter.
// We "clone" a repository by first creating a bare repo and then fetching the
// configured refs into it from the remote.
func (s *gitRepoSyncer) Clone(ctx context.Context, repo api.RepoName, remoteURL *vcs.URL, targetDir common.GitDir, tmpPath string, progressWriter io.Writer) (err error) {
	// First, make sure the tmpPath exists.
	if err := os.MkdirAll(tmpPath, os.ModePerm); err != nil {
		return errors.Wrapf(err, "clone failed to create tmp dir")
	}

	// Next, initialize a bare repo in that tmp path.
	tryWrite(s.logger, progressWriter, "Creating bare repo\n")
	if err := git.MakeBareRepo(ctx, tmpPath); err != nil {
		return err
	}
	tryWrite(s.logger, progressWriter, "Created bare repo at %s\n", tmpPath)

	// Now we build our fetch command. We don't actually clone, instead we init
	// a bare repository and fetch all refs from remote once into local refs.
	cmd, _ := s.fetchCommand(ctx, remoteURL)
	cmd.Dir = tmpPath
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	// see issue #7322: skip LFS content in repositories with Git LFS configured.
	cmd.Env = append(cmd.Env, "GIT_LFS_SKIP_SMUDGE=1")
	executil.ConfigureRemoteGitCommand(cmd, remoteURL)

	tryWrite(s.logger, progressWriter, "Fetching remote contents\n")
	redactor := urlredactor.New(remoteURL)
	wrCmd := s.recordingCommandFactory.WrapWithRepoName(ctx, s.logger, repo, cmd).WithRedactorFunc(redactor.Redact)
	// Note: Using RunCommandWriteOutput here does NOT store the output of the
	// command as the command output of the wrexec command, because the pipes are
	// already used.
	exitCode, err := executil.RunCommandWriteOutput(ctx, wrCmd, progressWriter, redactor.Redact)
	if err != nil {
		return errors.Wrapf(err, "failed to fetch: exit status %d", exitCode)
	}

	return nil
}

// Fetch tries to fetch updates of a Git repository.
func (s *gitRepoSyncer) Fetch(ctx context.Context, remoteURL *vcs.URL, repoName api.RepoName, dir common.GitDir, _ string) ([]byte, error) {
	cmd, configRemoteOpts := s.fetchCommand(ctx, remoteURL)
	dir.Set(cmd)
	r := urlredactor.New(remoteURL)

	if configRemoteOpts {
		// Configure the command to be able to talk to a remote.
		executil.ConfigureRemoteGitCommand(cmd, remoteURL)
	}

	output, err := s.recordingCommandFactory.WrapWithRepoName(ctx, log.NoOp(), repoName, cmd).WithRedactorFunc(r.Redact).CombinedOutput()
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch from remote: "+string(output))
	}
	return output, nil
}

// RemoteShowCommand returns the command to be executed for showing remote of a Git repository.
func (s *gitRepoSyncer) RemoteShowCommand(ctx context.Context, remoteURL *vcs.URL) (cmd *exec.Cmd, err error) {
	return exec.CommandContext(ctx, "git", "remote", "show", remoteURL.String()), nil
}

func (s *gitRepoSyncer) fetchCommand(ctx context.Context, remoteURL *vcs.URL) (cmd *exec.Cmd, configRemoteOpts bool) {
	configRemoteOpts = true
	if customCmd := customFetchCmd(ctx, remoteURL); customCmd != nil {
		cmd = customCmd
		configRemoteOpts = false
	} else if useRefspecOverrides() {
		cmd = refspecOverridesFetchCmd(ctx, remoteURL)
	} else {
		cmd = exec.CommandContext(ctx, "git", "fetch",
			"--progress", "--prune", remoteURL.String(),
			// Normal git refs
			"+refs/heads/*:refs/heads/*", "+refs/tags/*:refs/tags/*",
			// GitHub pull requests
			"+refs/pull/*:refs/pull/*",
			// GitLab merge requests
			"+refs/merge-requests/*:refs/merge-requests/*",
			// Bitbucket pull requests
			"+refs/pull-requests/*:refs/pull-requests/*",
			// Gerrit changesets
			"+refs/changes/*:refs/changes/*",
			// Possibly deprecated refs for sourcegraph zap experiment?
			"+refs/sourcegraph/*:refs/sourcegraph/*")
	}
	return cmd, configRemoteOpts
}

func isAlwaysCloningTestRemoteURL(remoteURL *vcs.URL) bool {
	return strings.EqualFold(remoteURL.Host, "github.com") &&
		strings.EqualFold(remoteURL.Path, "sourcegraphtest/alwayscloningtest")
}
