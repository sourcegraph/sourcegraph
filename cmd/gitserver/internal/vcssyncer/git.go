package vcssyncer

import (
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/executil"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/urlredactor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// gitRepoSyncer is a syncer for Git repositories.
type gitRepoSyncer struct {
	logger                  log.Logger
	recordingCommandFactory *wrexec.RecordingCommandFactory
	getRemoteURLSource      func(ctx context.Context, name api.RepoName) (RemoteURLSource, error)
}

func NewGitRepoSyncer(
	logger log.Logger,
	r *wrexec.RecordingCommandFactory,
	getRemoteURLSource func(ctx context.Context, name api.RepoName) (RemoteURLSource, error)) *gitRepoSyncer {
	return &gitRepoSyncer{
		logger:                  logger.Scoped("GitRepoSyncer"),
		recordingCommandFactory: r,
		getRemoteURLSource:      getRemoteURLSource}
}

func (s *gitRepoSyncer) Type() string {
	return "git"
}

// TestGitRepoExists is a test fixture that overrides the return value for
// GitRepoSyncer.IsCloneable when it is set.
var TestGitRepoExists func(ctx context.Context, repoName api.RepoName) error

// IsCloneable checks to see if the Git remote URL is cloneable.
func (s *gitRepoSyncer) IsCloneable(ctx context.Context, repoName api.RepoName) (err error) {
	if isAlwaysCloningTest(repoName) {
		return nil
	}

	if TestGitRepoExists != nil {
		return TestGitRepoExists(ctx, repoName)
	}

	source, err := s.getRemoteURLSource(ctx, repoName)
	if err != nil {
		return errors.Wrapf(err, "failed to get remote URL source for %s", repoName)
	}

	remoteURL, err := source.RemoteURL(ctx)
	if err != nil {
		return errors.Wrapf(err, "failed to get remote URL for %s", repoName)
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
			redactedOutput := r.Redact(string(out))
			err = errors.Wrap(err, "failed to check remote access: "+redactedOutput)
		}
		return err
	}

	return nil
}

// TestRepositoryPostFetchCorruptionFunc is a test hook that, when set, overrides the
// behavior of the GitRepoSyncer immediately after a repository has been fetched or cloned. It is
// called with the context and the temporary directory of the repository that was fetched.
//
// This can be used by tests to disrupt a cloned repository (e.g. deleting
//
//	HEAD, zeroing it out, etc.)
var TestRepositoryPostFetchCorruptionFunc func(ctx context.Context, dir common.GitDir)

// Fetch tries to fetch updates of a Git repository.
func (s *gitRepoSyncer) Fetch(ctx context.Context, repoName api.RepoName, dir common.GitDir, progressWriter io.Writer) error {
	source, err := s.getRemoteURLSource(ctx, repoName)
	if err != nil {
		return errors.Wrapf(err, "failed to get remote URL source for %s", repoName)
	}

	// Fetch the remote contents.
	{
		tryWrite(s.logger, progressWriter, "Fetching remote contents\n")

		exitCode, err := s.runFetchCommand(ctx, repoName, source, progressWriter, dir)
		if err != nil {
			return errors.Wrapf(err, "exit code: %d, failed to fetch from remote", exitCode)
		}

		tryWrite(s.logger, progressWriter, "Fetched remote contents\n")
	}

	if TestRepositoryPostFetchCorruptionFunc != nil {
		TestRepositoryPostFetchCorruptionFunc(ctx, dir)
	}

	// Set the local HEAD to the remote HEAD.
	{
		tryWrite(s.logger, progressWriter, "Setting local HEAD to remote HEAD\n")

		err := s.setHEAD(ctx, repoName, dir, source)
		if err != nil {
			return errors.Wrap(err, "failed to set local HEAD to remote HEAD")
		}

		tryWrite(s.logger, progressWriter, "Finished setting local HEAD to remote HEAD\n")
	}

	return nil
}

func (s *gitRepoSyncer) runFetchCommand(ctx context.Context, repoName api.RepoName, source RemoteURLSource, progressWriter io.Writer, dir common.GitDir) (exitCode int, err error) {
	remoteURL, err := source.RemoteURL(ctx)
	if err != nil {
		return -1, errors.Wrap(err, "failed to get remote URL")
	}

	var cmd *exec.Cmd

	configRemoteOpts := true
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

	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}

	// see issue #7322: skip LFS content in repositories with Git LFS configured.
	cmd.Env = append(cmd.Env, "GIT_LFS_SKIP_SMUDGE=1")

	// Set the working directory for the command.
	dir.Set(cmd)

	if configRemoteOpts {
		// Configure the command to be able to talk to a remote.
		executil.ConfigureRemoteGitCommand(cmd, remoteURL)
	}

	redactor := urlredactor.New(remoteURL)
	wrCmd := s.recordingCommandFactory.WrapWithRepoName(ctx, s.logger, repoName, cmd).WithRedactorFunc(redactor.Redact)

	// Note: Using RunCommandWriteOutput here does NOT store the output of the
	// command as the command output of the wrexec command, because the pipes are
	// already used.

	return executil.RunCommandWriteOutput(ctx, wrCmd, progressWriter, redactor.Redact)
}

var headBranchPattern = lazyregexp.New(`HEAD branch: (.+?)\n`)

// setHEAD configures git repo defaults (such as what HEAD is) which are
// needed for git commands to work.
func (s *gitRepoSyncer) setHEAD(ctx context.Context, repoName api.RepoName, dir common.GitDir, source RemoteURLSource) error {
	// Verify that there is a HEAD file within the repo, and that it is of
	// non-zero length.
	if err := git.EnsureHEAD(dir); err != nil {
		s.logger.Error("failed to ensure HEAD exists", log.Error(err), log.String("repo", string(repoName)))
	}

	// Fallback to git's default branch name if git remote show fails.
	headBranch := "master"

	// Get the default branch name from the remote.
	{
		remoteURL, err := source.RemoteURL(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get remote URL for %s", repoName)
		}

		// try to fetch HEAD from origin
		cmd := exec.CommandContext(ctx, "git", "remote", "show", remoteURL.String())
		dir.Set(cmd)
		r := urlredactor.New(remoteURL)

		// Configure the command to be able to talk to a remote.
		executil.ConfigureRemoteGitCommand(cmd, remoteURL)

		output, err := s.recordingCommandFactory.WrapWithRepoName(ctx, s.logger, repoName, cmd).WithRedactorFunc(r.Redact).CombinedOutput()
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				exitErr.Stderr = []byte(r.Redact(string(exitErr.Stderr))) // Ensure we redact the error message

				s.logger.Error("Failed to fetch remote info", log.Error(exitErr), log.String("output", string(output)))
				return errors.Wrap(exitErr, "failed to fetch remote info")
			}

			s.logger.Error("Failed to fetch remote info", log.Error(err), log.String("output", string(output)))
			return errors.Wrap(err, "failed to fetch remote info")
		}

		submatches := headBranchPattern.FindSubmatch(output)
		if len(submatches) == 2 {
			submatch := string(submatches[1])
			if submatch != "(unknown)" {
				headBranch = submatch
			}
		}
	}

	// check if branch pointed to by HEAD exists
	{
		cmd := exec.CommandContext(ctx, "git", "rev-parse", headBranch, "--")
		dir.Set(cmd)
		if err := cmd.Run(); err != nil {
			// branch does not exist, pick first branch
			cmd := exec.CommandContext(ctx, "git", "branch")
			dir.Set(cmd)
			output, err := cmd.Output()
			if err != nil {
				s.logger.Error("Failed to list branches", log.Error(err), log.String("output", string(output)))
				return errors.Wrap(err, "failed to list branches")
			}
			lines := strings.Split(string(output), "\n")
			branch := strings.TrimPrefix(strings.TrimPrefix(lines[0], "* "), "  ")
			if branch != "" {
				headBranch = branch
			}
		}
	}

	// set HEAD
	{
		cmd := exec.CommandContext(ctx, "git", "symbolic-ref", "HEAD", "refs/heads/"+headBranch)
		dir.Set(cmd)
		output, err := cmd.CombinedOutput()
		if err != nil {
			s.logger.Error("Failed to set HEAD", log.Error(err), log.String("output", string(output)))
			return errors.Wrap(err, "Failed to set HEAD")
		}
	}

	return nil
}

func isAlwaysCloningTest(name api.RepoName) bool {
	return protocol.NormalizeRepo(name).Equal("github.com/sourcegraphtest/alwayscloningtest")
}
