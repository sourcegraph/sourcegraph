package vcssyncer

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/schema"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/executil"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/gitserverfs"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/perforce"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/urlredactor"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// perforceDepotSyncer is a syncer for Perforce depots.
type perforceDepotSyncer struct {
	logger                  log.Logger
	recordingCommandFactory *wrexec.RecordingCommandFactory
	fs                      gitserverfs.FS
	connection              *schema.PerforceConnection

	// fusionConfig contains information about the experimental p4-fusion client.
	fusionConfig fusionConfig

	// getRemoteURLSource returns the RemoteURLSource for the given repository.
	getRemoteURLSource func(ctx context.Context, name api.RepoName) (RemoteURLSource, error)
}

func NewPerforceDepotSyncer(logger log.Logger, r *wrexec.RecordingCommandFactory, fs gitserverfs.FS, connection *schema.PerforceConnection, getRemoteURLSource func(ctx context.Context, name api.RepoName) (RemoteURLSource, error)) VCSSyncer {
	return &perforceDepotSyncer{
		logger:                  logger.Scoped("PerforceDepotSyncer"),
		recordingCommandFactory: r,
		fs:                      fs,
		connection:              connection,
		fusionConfig:            configureFusionClient(connection),
		getRemoteURLSource:      getRemoteURLSource,
	}
}

func (s *perforceDepotSyncer) Type() string {
	return "perforce"
}

// IsCloneable checks to see if the Perforce remote URL is cloneable.
func (s *perforceDepotSyncer) IsCloneable(ctx context.Context, repoName api.RepoName) error {
	source, err := s.getRemoteURLSource(ctx, repoName)
	if err != nil {
		return errors.Wrap(err, "getting remote URL source")
	}

	remoteURL, err := source.RemoteURL(ctx)
	if err != nil {
		return errors.Wrap(err, "getting remote URL") // This should never happen for Perforce
	}

	username, password, host, path, err := perforce.DecomposePerforceRemoteURL(remoteURL)
	if err != nil {
		return errors.Wrap(err, "invalid perforce remote URL")
	}

	return perforce.IsDepotPathCloneable(ctx, s.fs, perforce.IsDepotPathCloneableArguments{
		P4Port:   host,
		P4User:   username,
		P4Passwd: password,

		DepotPath: path,
	})
}

// Example: p4-fusion --path //depot/... --user $P4USER --src clones/ --networkThreads 64 --printBatch 10 --port $P4PORT --lookAhead 2000 --retries 10 --refresh 100 --noColor true --noBaseCommit true
func buildP4FusionCmd(ctx context.Context, fusionConfig fusionConfig, depot, username, src, port string) *exec.Cmd {
	return exec.CommandContext(ctx, "p4-fusion",
		"--path", depot+"...",
		"--client", fusionConfig.Client,
		"--user", username,
		"--src", src,
		"--networkThreads", strconv.Itoa(fusionConfig.NetworkThreads),
		"--printBatch", strconv.Itoa(fusionConfig.PrintBatch),
		"--port", port,
		"--lookAhead", strconv.Itoa(fusionConfig.LookAhead),
		"--retries", strconv.Itoa(fusionConfig.Retries),
		"--refresh", strconv.Itoa(fusionConfig.Refresh),
		"--maxChanges", strconv.Itoa(fusionConfig.MaxChanges),
		"--includeBinaries", strconv.FormatBool(fusionConfig.IncludeBinaries),
		"--fsyncEnable", strconv.FormatBool(fusionConfig.FsyncEnable),
		"--noColor", "true",
		// We don't want an empty commit for a sane merge base across branches,
		// since we don't use them and the empty commit breaks changelist parsing.
		"--noBaseCommit", "true",
	)
}

// Fetch tries to fetch updates of a Perforce depot as a Git repository.
func (s *perforceDepotSyncer) Fetch(ctx context.Context, repoName api.RepoName, dir common.GitDir, progressWriter io.Writer) error {
	source, err := s.getRemoteURLSource(ctx, repoName)
	if err != nil {
		return errors.Wrap(err, "getting remote URL source")
	}

	remoteURL, err := source.RemoteURL(ctx)
	if err != nil {
		return errors.Wrap(err, "getting remote URL") // This should never happen for Perforce
	}

	p4user, p4passwd, p4port, depot, err := perforce.DecomposePerforceRemoteURL(remoteURL)
	if err != nil {
		return errors.Wrap(err, "invalid perforce remote URL")
	}

	// First, do a quick check if we can reach the Perforce server.
	tryWrite(s.logger, progressWriter, "Checking Perforce server connection\n")
	err = perforce.P4TestWithTrust(ctx, s.fs, perforce.P4TestWithTrustArguments{
		P4Port:   p4port,
		P4User:   p4user,
		P4Passwd: p4passwd,
	})
	if err != nil {
		return errors.Wrap(err, "verifying connection to perforce server")
	}
	tryWrite(s.logger, progressWriter, "Perforce server connection succeeded\n")

	var cmd *exec.Cmd
	if s.fusionConfig.Enabled {
		tryWrite(s.logger, progressWriter, "Converting depot using p4-fusion\n")
		// Example: p4-fusion --path //depot/... --user $P4USER --src clones/ --networkThreads 64 --printBatch 10 --port $P4PORT --lookAhead 2000 --retries 10 --refresh 100
		root, _ := filepath.Split(string(dir))
		cmd = buildP4FusionCmd(ctx, s.fusionConfig, depot, p4user, root+".git", p4port)
	} else {
		// TODO: This used to call the following for clone:
		// tryWrite(s.logger, progressWriter, "Converting depot using git-p4\n")
		// // Example: git p4 clone --bare --max-changes 1000 //Sourcegraph/@all /tmp/clone-584194180/.git
		// args := append([]string{"p4", "clone", "--bare"}, s.p4CommandOptions()...)
		// args = append(args, depot+"@all", tmpPath)
		// cmd = exec.CommandContext(ctx, "git", args...)
		tryWrite(s.logger, progressWriter, "Converting depot using git-p4\n")
		// Example: git p4 sync --max-changes 1000
		args := append([]string{"p4", "sync"}, p4CommandOptions(s.connection)...)
		cmd = exec.CommandContext(ctx, "git", args...)
	}
	cmd.Env, err = p4CommandEnv(s.fs, string(dir), p4port, p4user, p4passwd, s.connection.P4Client)
	if err != nil {
		return errors.Wrap(err, "failed to build p4 command env")
	}
	dir.Set(cmd)

	// TODO(keegancsmith)(indradhanush) This is running a remote command and
	// we have runRemoteGitCommand which sets TLS settings/etc. Do we need
	// something for p4?
	redactor := urlredactor.New(remoteURL)
	wrCmd := s.recordingCommandFactory.WrapWithRepoName(ctx, s.logger, repoName, cmd).WithRedactorFunc(redactor.Redact)
	// Note: Using RunCommandWriteOutput here does NOT store the output of the
	// command as the command output of the wrexec command, because the pipes are
	// already used.
	exitCode, err := executil.RunCommandWriteOutput(ctx, wrCmd, progressWriter, redactor.Redact)
	if err != nil {
		return errors.Wrapf(err, "failed to run p4->git conversion: exit code %d", exitCode)
	}

	if !s.fusionConfig.Enabled {
		p4home, err := s.fs.P4HomeDir()
		if err != nil {
			return errors.Wrap(err, "failed to create p4home")
		}

		// Force update "master" to "refs/remotes/p4/master" where changes are synced into
		cmd := wrexec.CommandContext(ctx, nil, "git", "branch", "-f", "master", "refs/remotes/p4/master")
		cmd.Cmd.Env = append(os.Environ(),
			"P4PORT="+p4port,
			"P4USER="+p4user,
			"P4PASSWD="+p4passwd,
			"HOME="+p4home,
		)
		dir.Set(cmd.Cmd)
		if output, err := cmd.CombinedOutput(); err != nil {
			return errors.Wrapf(err, "failed to force update branch with output %q", string(output))
		}
	}

	// The update was successful, after a git fetch it is expected that a repos
	// FETCH_HEAD has either been updated, or that HEAD has been touched, even
	// if no changes were fetched.
	// Since we use this for last_fetched, we touch it here.
	if err := os.Chtimes(dir.Path("HEAD"), time.Time{}, time.Now()); err != nil {
		s.logger.Error("failed to touch HEAD after perforce fetch", log.Error(err))
	}

	return nil
}

func p4CommandOptions(connection *schema.PerforceConnection) []string {
	flags := []string{}
	if connection.MaxChanges > 0 {
		flags = append(flags, "--max-changes", strconv.Itoa(int(connection.MaxChanges)))
	}
	if connection.P4Client != "" {
		flags = append(flags, "--use-client-spec")
	}
	return flags
}

func p4CommandEnv(fs gitserverfs.FS, cmdCWD, p4port, p4user, p4passwd, p4Client string) ([]string, error) {
	env := append(
		os.Environ(),
		"P4PORT="+p4port,
		"P4USER="+p4user,
		"P4PASSWD="+p4passwd,
		"P4CLIENTPATH="+cmdCWD,
	)

	if p4Client != "" {
		env = append(env, "P4CLIENT="+p4Client)
	}

	p4home, err := fs.P4HomeDir()
	if err != nil {
		return nil, err
	}

	// git p4 commands write to $HOME/.gitp4-usercache.txt, we should pass in a
	// directory under our control and ensure that it is writeable.
	env = append(env, "HOME="+p4home)

	return env, nil
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
		PrintBatch:          100,
		Refresh:             1000,
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
