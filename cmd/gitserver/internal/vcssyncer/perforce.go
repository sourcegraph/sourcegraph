package vcssyncer

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/schema"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/executil"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/perforce"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/urlredactor"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/process"
)

// perforceDepotSyncer is a syncer for Perforce depots.
type perforceDepotSyncer struct {
	logger                  log.Logger
	recordingCommandFactory *wrexec.RecordingCommandFactory

	// MaxChanges indicates to only import at most n changes when possible.
	MaxChanges int

	// P4Client configures the client to use with p4 and enables use of a client spec
	// to find the list of interesting files in p4.
	P4Client string

	// FusionConfig contains information about the experimental p4-fusion client.
	FusionConfig fusionConfig

	// P4Home is a directory we will pass to `git p4` commands as the
	// $HOME directory as it requires this to write cache data.
	P4Home string
}

func NewPerforceDepotSyncer(logger log.Logger, r *wrexec.RecordingCommandFactory, connection *schema.PerforceConnection, p4Home string) VCSSyncer {
	return &perforceDepotSyncer{
		logger:                  logger.Scoped("PerforceDepotSyncer"),
		recordingCommandFactory: r,
		MaxChanges:              int(connection.MaxChanges),
		P4Client:                connection.P4Client,
		FusionConfig:            configureFusionClient(connection),
		P4Home:                  p4Home,
	}
}

func (s *perforceDepotSyncer) Type() string {
	return "perforce"
}

// IsCloneable checks to see if the Perforce remote URL is cloneable.
func (s *perforceDepotSyncer) IsCloneable(ctx context.Context, _ api.RepoName, remoteURL *vcs.URL) error {
	username, password, host, path, err := perforce.DecomposePerforceRemoteURL(remoteURL)
	if err != nil {
		return errors.Wrap(err, "invalid perforce remote URL")
	}

	return perforce.IsDepotPathCloneable(ctx, s.P4Home, host, username, password, path)
}

// Clone writes a Perforce depot into tmpPath, using a Perforce-to-git-conversion.
// It reports redacted progress logs via the progressWriter.
func (s *perforceDepotSyncer) Clone(ctx context.Context, repo api.RepoName, remoteURL *vcs.URL, targetDir common.GitDir, tmpPath string, progressWriter io.Writer) (err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	redactor := urlredactor.New(remoteURL)

	defer func() {
		if err != nil {
			// Print errors to the progressWriter for easier inspection.
			tryWrite(s.logger, progressWriter, "Error: %s\n", redactor.Redact(err.Error()))
		}
	}()

	// First, make sure the tmpPath exists.
	if err := os.MkdirAll(tmpPath, os.ModePerm); err != nil {
		return errors.Wrapf(err, "clone failed to create tmp dir")
	}

	p4user, p4passwd, p4port, depot, err := perforce.DecomposePerforceRemoteURL(remoteURL)
	if err != nil {
		return errors.Wrap(err, "invalid perforce remote URL")
	}

	// First, do a quick check if we can reach the Perforce server.
	tryWrite(s.logger, progressWriter, "Checking Perforce server connection\n")
	err = perforce.P4TestWithTrust(ctx, s.P4Home, p4port, p4user, p4passwd)
	if err != nil {
		return errors.Wrap(err, "verifying connection to perforce server")
	}
	tryWrite(s.logger, progressWriter, "Perforce server connection succeeded\n")

	var cmd *exec.Cmd
	if s.FusionConfig.Enabled {
		tryWrite(s.logger, progressWriter, "Converting depot using p4-fusion\n")
		cmd = s.buildP4FusionCmd(ctx, depot, p4user, tmpPath, p4port)
	} else {
		tryWrite(s.logger, progressWriter, "Converting depot using git-p4\n")
		// Example: git p4 clone --bare --max-changes 1000 //Sourcegraph/@all /tmp/clone-584194180/.git
		args := append([]string{"p4", "clone", "--bare"}, s.p4CommandOptions()...)
		args = append(args, depot+"@all", tmpPath)
		cmd = exec.CommandContext(ctx, "git", args...)
	}
	cmd.Env = s.p4CommandEnv(p4port, p4user, p4passwd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "failed to attach stdout pipe")
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return errors.Wrap(err, "failed to attach stdout pipe")
	}

	g := pool.New().WithErrors()

	var writeMu sync.Mutex

	g.Go(func() error {
		sc := process.NewOutputScannerWithSplit(stdout, executil.ScanLinesWithCRLF)
		for sc.Scan() {
			line := sc.Text()
			writeMu.Lock()
			if _, err := fmt.Fprintln(progressWriter, redactor.Redact(line)); err != nil {
				return err
			}
			writeMu.Unlock()
		}
		return sc.Err()
	})
	g.Go(func() error {
		sc := process.NewOutputScannerWithSplit(stderr, executil.ScanLinesWithCRLF)
		for sc.Scan() {
			line := sc.Text()
			writeMu.Lock()
			if _, err := fmt.Fprintln(progressWriter, redactor.Redact(line)); err != nil {
				return err
			}
			writeMu.Unlock()
		}
		return sc.Err()
	})

	go func() {
		<-ctx.Done()
		stdout.Close()
		stderr.Close()
	}()

	wrCmd := s.recordingCommandFactory.WrapWithRepoName(ctx, s.logger, repo, cmd).WithRedactorFunc(redactor.Redact)

	// Note: Using Start here does NOT store the output of the command as the
	// command output of the wrexec command.
	if err = wrCmd.Start(); err != nil {
		return errors.Wrap(err, "failed to start p4 conversion command")
	}

	select {
	case <-ctx.Done():
	case err := <-watchErrGroup(g):
		if err != nil {
			return errors.Wrap(err, "failed to read output")
		}
	}

	if err := wrCmd.Wait(); err != nil {
		return errors.Wrap(err, "conversion failed")
	}

	return nil
}

// Example: p4-fusion --path //depot/... --user $P4USER --src clones/ --networkThreads 64 --printBatch 10 --port $P4PORT --lookAhead 2000 --retries 10 --refresh 100
func (s *perforceDepotSyncer) buildP4FusionCmd(ctx context.Context, depot, username, src, port string) *exec.Cmd {
	return exec.CommandContext(ctx, "p4-fusion",
		"--path", depot+"...",
		"--client", s.FusionConfig.Client,
		"--user", username,
		"--src", src,
		"--networkThreads", strconv.Itoa(s.FusionConfig.NetworkThreads),
		"--printBatch", strconv.Itoa(s.FusionConfig.PrintBatch),
		"--port", port,
		"--lookAhead", strconv.Itoa(s.FusionConfig.LookAhead),
		"--retries", strconv.Itoa(s.FusionConfig.Retries),
		"--refresh", strconv.Itoa(s.FusionConfig.Refresh),
		"--maxChanges", strconv.Itoa(s.FusionConfig.MaxChanges),
		"--includeBinaries", strconv.FormatBool(s.FusionConfig.IncludeBinaries),
		"--fsyncEnable", strconv.FormatBool(s.FusionConfig.FsyncEnable),
		"--noColor", "true",
	)
}

// Fetch tries to fetch updates of a Perforce depot as a Git repository.
func (s *perforceDepotSyncer) Fetch(ctx context.Context, remoteURL *vcs.URL, _ api.RepoName, dir common.GitDir, _ string) ([]byte, error) {
	p4user, p4passwd, p4port, depot, err := perforce.DecomposePerforceRemoteURL(remoteURL)
	if err != nil {
		return nil, errors.Wrap(err, "invalid perforce remote URL")
	}

	// First, do a quick check if we can reach the Perforce server.
	err = perforce.P4TestWithTrust(ctx, s.P4Home, p4port, p4user, p4passwd)
	if err != nil {
		return nil, errors.Wrap(err, "verifying connection to perforce server")
	}

	var cmd *wrexec.Cmd
	if s.FusionConfig.Enabled {
		// Example: p4-fusion --path //depot/... --user $P4USER --src clones/ --networkThreads 64 --printBatch 10 --port $P4PORT --lookAhead 2000 --retries 10 --refresh 100
		root, _ := filepath.Split(string(dir))
		cmd = wrexec.Wrap(ctx, nil, s.buildP4FusionCmd(ctx, depot, p4user, root+".git", p4port))
	} else {
		// Example: git p4 sync --max-changes 1000
		args := append([]string{"p4", "sync"}, s.p4CommandOptions()...)
		cmd = wrexec.CommandContext(ctx, nil, "git", args...)
	}
	cmd.Env = s.p4CommandEnv(p4port, p4user, p4passwd)
	dir.Set(cmd.Cmd)

	// TODO(keegancsmith)(indradhanush) This is running a remote command and
	// we have runRemoteGitCommand which sets TLS settings/etc. Do we need
	// something for p4?
	output, err := executil.RunCommandCombinedOutput(ctx, cmd)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update with output %q", urlredactor.New(remoteURL).Redact(string(output)))
	}

	if !s.FusionConfig.Enabled {
		// Force update "master" to "refs/remotes/p4/master" where changes are synced into
		cmd = wrexec.CommandContext(ctx, nil, "git", "branch", "-f", "master", "refs/remotes/p4/master")
		cmd.Cmd.Env = append(os.Environ(),
			"P4PORT="+p4port,
			"P4USER="+p4user,
			"P4PASSWD="+p4passwd,
			"HOME="+s.P4Home,
		)
		dir.Set(cmd.Cmd)
		if output, err := executil.RunCommandCombinedOutput(ctx, cmd); err != nil {
			return nil, errors.Wrapf(err, "failed to force update branch with output %q", string(output))
		}
	}

	return output, nil
}

// RemoteShowCommand returns the command to be executed for showing Git remote of a Perforce depot.
func (s *perforceDepotSyncer) RemoteShowCommand(ctx context.Context, _ *vcs.URL) (cmd *exec.Cmd, err error) {
	// Remote info is encoded as in the current repository
	return exec.CommandContext(ctx, "git", "remote", "show", "./"), nil
}

func (s *perforceDepotSyncer) p4CommandOptions() []string {
	flags := []string{}
	if s.MaxChanges > 0 {
		flags = append(flags, "--max-changes", strconv.Itoa(s.MaxChanges))
	}
	if s.P4Client != "" {
		flags = append(flags, "--use-client-spec")
	}
	return flags
}

func (s *perforceDepotSyncer) p4CommandEnv(p4port, p4user, p4passwd string) []string {
	env := append(
		os.Environ(),
		"P4PORT="+p4port,
		"P4USER="+p4user,
		"P4PASSWD="+p4passwd,
	)

	if s.P4Client != "" {
		env = append(env, "P4CLIENT="+s.P4Client)
	}

	if s.P4Home != "" {
		// git p4 commands write to $HOME/.gitp4-usercache.txt, we should pass in a
		// directory under our control and ensure that it is writeable.
		env = append(env, "HOME="+s.P4Home)
	}

	return env
}

// fusionConfig allows configuration of the p4-fusion client.
type fusionConfig struct {
	// Enabled: Enable the p4-fusion client for cloning and fetching repos
	Enabled bool
	// Client: The client spec tht should be used
	Client string
	// LookAhead: How many CLs in the future, at most, shall we keep downloaded by
	// the time it is to commit them
	LookAhead int
	// NetworkThreads: The number of threads in the threadpool for running network
	// calls. Defaults to the number of logical CPUs.
	NetworkThreads int
	// NetworkThreadsFetch: The same as network threads but specifically used when
	// fetching rather than cloning.
	NetworkThreadsFetch int
	// PrintBatch:  The p4 print batch size
	PrintBatch int
	// Refresh: How many times a connection should be reused before it is refreshed
	Refresh int
	// Retries: How many times a command should be retried before the process exits
	// in a failure
	Retries int
	// MaxChanges limits how many changes to fetch during the initial clone. A
	// default of -1 means fetch all changes
	MaxChanges int
	// IncludeBinaries sets whether to include binary files
	IncludeBinaries bool
	// FsyncEnable enables fsync() while writing objects to disk to ensure they get
	// written to permanent storage immediately instead of being cached. This is to
	// mitigate data loss in events of hardware failure.
	FsyncEnable bool
}

func configureFusionClient(conn *schema.PerforceConnection) fusionConfig {
	// Set up default settings first
	fc := fusionConfig{
		Enabled:             false,
		Client:              conn.P4Client,
		LookAhead:           2000,
		NetworkThreads:      12,
		NetworkThreadsFetch: 12,
		PrintBatch:          10,
		Refresh:             100,
		Retries:             10,
		MaxChanges:          -1,
		IncludeBinaries:     false,
		FsyncEnable:         false,
	}

	if conn.FusionClient == nil {
		return fc
	}

	// Required
	fc.Enabled = conn.FusionClient.Enabled

	// Optional
	if conn.FusionClient.LookAhead > 0 {
		fc.LookAhead = conn.FusionClient.LookAhead
	}
	if conn.FusionClient.NetworkThreads > 0 {
		fc.NetworkThreads = conn.FusionClient.NetworkThreads
	}
	if conn.FusionClient.NetworkThreadsFetch > 0 {
		fc.NetworkThreadsFetch = conn.FusionClient.NetworkThreadsFetch
	}
	if conn.FusionClient.PrintBatch > 0 {
		fc.PrintBatch = conn.FusionClient.PrintBatch
	}
	if conn.FusionClient.Refresh > 0 {
		fc.Refresh = conn.FusionClient.Refresh
	}
	if conn.FusionClient.Retries > 0 {
		fc.Retries = conn.FusionClient.Retries
	}
	if conn.FusionClient.MaxChanges > 0 {
		fc.MaxChanges = conn.FusionClient.MaxChanges
	}
	fc.IncludeBinaries = conn.FusionClient.IncludeBinaries
	fc.FsyncEnable = conn.FusionClient.FsyncEnable

	return fc
}
