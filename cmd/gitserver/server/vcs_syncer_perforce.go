package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/perforce"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/server/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/server/urlredactor"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// PerforceDepotSyncer is a syncer for Perforce depots.
type PerforceDepotSyncer struct {
	// MaxChanges indicates to only import at most n changes when possible.
	MaxChanges int

	// Client configures the client to use with p4 and enables use of a client spec
	// to find the list of interesting files in p4.
	Client string

	// FusionConfig contains information about the experimental p4-fusion client.
	FusionConfig FusionConfig

	// P4Home is a directory we will pass to `git p4` commands as the
	// $HOME directory as it requires this to write cache data.
	P4Home string
}

func (s *PerforceDepotSyncer) Type() string {
	return "perforce"
}

func (s *PerforceDepotSyncer) CanConnect(ctx context.Context, host, username, password string) error {
	return p4testWithTrust(ctx, host, username, password)
}

// IsCloneable checks to see if the Perforce remote URL is cloneable.
func (s *PerforceDepotSyncer) IsCloneable(ctx context.Context, _ api.RepoName, remoteURL *vcs.URL) error {
	username, password, host, path, _, err := decomposePerforceRemoteURL(remoteURL)
	if err != nil {
		return errors.Wrap(err, "decompose")
	}

	// start with a test and set up trust if necessary
	if err := p4testWithTrust(ctx, host, username, password); err != nil {
		return err
	}

	// the path could be a path into a depot, or it could be just a depot
	// expect it to start with at least one slash
	// (the config defines it as starting with two, but converting it to a URL may change that)
	// the first path part will be the depot - subsequent parts define a directory path into a depot
	// ignore the directory parts for now, and only test for access to the depot
	// TODO: revisit if we want to also test for access to the directories, if any are included
	depot := strings.Split(strings.TrimLeft(path, "/"), "/")[0]

	// get a list of depots that match the supplied depot (if it's defined)
	if depots, err := p4depots(ctx, host, username, password, depot); err != nil {
		return err
	} else if len(depots) == 0 {
		// this user doesn't have access to any depots,
		// or to the given depot
		if depot != "" {
			return errors.Newf("the user %s does not have access to the depot %s on the server %s", username, depot, host)
		} else {
			return errors.Newf("the user %s does not have access to any depots on the server %s", username, host)
		}
	}

	// no overt errors, so this depot is cloneable
	return nil
}

// CloneCommand returns the command to be executed for cloning a Perforce depot as a Git repository.
func (s *PerforceDepotSyncer) CloneCommand(ctx context.Context, remoteURL *vcs.URL, tmpPath string) (*exec.Cmd, error) {
	username, password, p4port, depot, isStreamDepot, err := decomposePerforceRemoteURL(remoteURL)
	if err != nil {
		return nil, errors.Wrap(err, "decompose")
	}

	err = p4testWithTrust(ctx, p4port, username, password)
	if err != nil {
		return nil, errors.Wrap(err, "test with trust")
	}

	var cmd *exec.Cmd
	if s.FusionConfig.Enabled {
		cmd = s.buildP4FusionCmd(ctx, p4port, depot, username, password, tmpPath, isStreamDepot)
	} else {
		// Example: git p4 clone --bare --max-changes 1000 //Sourcegraph/@all /tmp/clone-584194180/.git
		args := append([]string{"p4", "clone", "--bare"}, s.p4CommandOptions()...)
		args = append(args, depot+"@all", tmpPath)
		cmd = exec.CommandContext(ctx, "git", args...)
	}
	cmd.Env = s.p4CommandEnv(p4port, username, password)

	return cmd, nil
}

func (s *PerforceDepotSyncer) buildP4FusionCmd(ctx context.Context, host, depot, username, password, src string, isStreamDepot bool) *exec.Cmd {
	// Example: p4-fusion --path //depot/... --user $P4USER --src clones/ --networkThreads 64 --printBatch 10 --port $P4PORT --lookAhead 2000 --retries 10 --refresh 100
	cmd := exec.CommandContext(ctx, "p4-fusion",
		"--path", depot+"...",
		"--client", s.FusionConfig.Client,
		"--user", username,
		"--src", src,
		"--networkThreads", strconv.Itoa(s.FusionConfig.NetworkThreads),
		"--printBatch", strconv.Itoa(s.FusionConfig.PrintBatch),
		"--port", host,
		"--lookAhead", strconv.Itoa(s.FusionConfig.LookAhead),
		"--retries", strconv.Itoa(s.FusionConfig.Retries),
		"--refresh", strconv.Itoa(s.FusionConfig.Refresh),
		"--maxChanges", strconv.Itoa(s.FusionConfig.MaxChanges),
		"--includeBinaries", strconv.FormatBool(s.FusionConfig.IncludeBinaries),
		"--fsyncEnable", strconv.FormatBool(s.FusionConfig.FsyncEnable),
		"--noColor", "true",
	)
	if isStreamDepot {
		// for stream depots, seek out and add streams to the command
		streams, err := p4streams(ctx, host, username, password, depot)
		if err == nil && len(streams) > 0 {
			for _, stream := range streams {
				if stream.Type != perforce.Virtual {
					cmd.Args = append(cmd.Args, "--branch", stream.Name)
				}
			}
		}
	}
	return cmd
}

// Fetch tries to fetch updates of a Perforce depot as a Git repository.
func (s *PerforceDepotSyncer) Fetch(ctx context.Context, remoteURL *vcs.URL, _ api.RepoName, dir common.GitDir, _ string) ([]byte, error) {
	username, password, host, depot, isStreamDepot, err := decomposePerforceRemoteURL(remoteURL)
	if err != nil {
		return nil, errors.Wrap(err, "decompose")
	}

	err = p4testWithTrust(ctx, host, username, password)
	if err != nil {
		return nil, errors.Wrap(err, "test with trust")
	}

	var cmd *wrexec.Cmd
	if s.FusionConfig.Enabled {
		// Example: p4-fusion --path //depot/... --user $P4USER --src clones/ --networkThreads 64 --printBatch 10 --port $P4PORT --lookAhead 2000 --retries 10 --refresh 100
		root, _ := filepath.Split(string(dir))
		cmd = wrexec.Wrap(ctx, nil, s.buildP4FusionCmd(ctx, host, depot, username, password, root+".git", isStreamDepot))
	} else {
		// Example: git p4 sync --max-changes 1000
		args := append([]string{"p4", "sync"}, s.p4CommandOptions()...)
		cmd = wrexec.CommandContext(ctx, nil, "git", args...)
	}
	cmd.Env = s.p4CommandEnv(host, username, password)
	dir.Set(cmd.Cmd)

	// TODO(keegancsmith)(indradhanush) This is running a remote command and
	// we have runRemoteGitCommand which sets TLS settings/etc. Do we need
	// something for p4?
	output, err := runCommandCombinedOutput(ctx, cmd)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to update with output %q", urlredactor.New(remoteURL).Redact(string(output)))
	}

	if !s.FusionConfig.Enabled {
		// Force update "master" to "refs/remotes/p4/master" where changes are synced into
		cmd = wrexec.CommandContext(ctx, nil, "git", "branch", "-f", "master", "refs/remotes/p4/master")
		cmd.Cmd.Env = append(os.Environ(),
			"P4PORT="+host,
			"P4USER="+username,
			"P4PASSWD="+password,
		)
		dir.Set(cmd.Cmd)
		if output, err := runCommandCombinedOutput(ctx, cmd); err != nil {
			return nil, errors.Wrapf(err, "failed to force update branch with output %q", string(output))
		}
	}

	return output, nil
}

// RemoteShowCommand returns the command to be executed for showing Git remote of a Perforce depot.
func (s *PerforceDepotSyncer) RemoteShowCommand(ctx context.Context, _ *vcs.URL) (cmd *exec.Cmd, err error) {
	// Remote info is encoded as in the current repository
	return exec.CommandContext(ctx, "git", "remote", "show", "./"), nil
}

func (s *PerforceDepotSyncer) p4CommandOptions() []string {
	flags := []string{}
	if s.MaxChanges > 0 {
		flags = append(flags, "--max-changes", strconv.Itoa(s.MaxChanges))
	}
	if s.Client != "" {
		flags = append(flags, "--use-client-spec")
	}
	return flags
}

func (s *PerforceDepotSyncer) p4CommandEnv(port, username, password string) []string {
	env := append(os.Environ(),
		"P4PORT="+port,
		"P4USER="+username,
		"P4PASSWD="+password,
	)

	if s.Client != "" {
		env = append(env, "P4CLIENT="+s.Client)
	}

	if s.P4Home != "" {
		// git p4 commands write to $HOME/.gitp4-usercache.txt, we should pass in a
		// directory under our control and ensure that it is writeable.
		env = append(env, "HOME="+s.P4Home)
	}

	return env
}

// decomposePerforceRemoteURL decomposes information back from a clone URL for a
// Perforce depot.
func decomposePerforceRemoteURL(remoteURL *vcs.URL) (username, password, host, depot string, isStreamDepot bool, err error) {
	if remoteURL.Scheme != "perforce" {
		return "", "", "", "", false, errors.New(`scheme is not "perforce"`)
	}
	password, _ = remoteURL.User.Password()
	return remoteURL.User.Username(), password, remoteURL.Host, remoteURL.Path, remoteURL.Query().Has("stream"), nil
}

// p4trust blindly accepts fingerprint of the Perforce server.
func p4trust(ctx context.Context, host string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "p4", "trust", "-y", "-f")
	cmd.Env = append(os.Environ(),
		"P4PORT="+host,
	)

	out, err := runCommandCombinedOutput(ctx, wrexec.Wrap(ctx, log.NoOp(), cmd))
	if err != nil {
		if ctxerr := ctx.Err(); ctxerr != nil {
			err = ctxerr
		}
		if len(out) > 0 {
			err = errors.Errorf("%s (output follows)\n\n%s", err, out)
		}
		return err
	}
	return nil
}

// p4test uses `p4 login -s` to test the Perforce connection: host, port, user, password.
func p4test(ctx context.Context, host, username, password string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// `p4 ping` requires extra-special access, so we want to avoid using it
	//
	// p4 login -s checks the connection and the credentials,
	// so it seems like the perfect alternative to `p4 ping`.
	cmd := exec.CommandContext(ctx, "p4", "login", "-s")
	cmd.Env = append(os.Environ(),
		"P4PORT="+host,
		"P4USER="+username,
		"P4PASSWD="+password,
	)

	out, err := runCommandCombinedOutput(ctx, wrexec.Wrap(ctx, log.NoOp(), cmd))
	if err != nil {
		if ctxerr := ctx.Err(); ctxerr != nil {
			err = errors.Wrap(ctxerr, "p4 login context error")
		}
		if len(out) > 0 {
			err = errors.Errorf("%s (output follows)\n\n%s", err, specifyCommandInErrorMessage(string(out), cmd))
		}
		return err
	}
	return nil
}

// p4depots returns all of the depots to which the user has access on the host
// and whose names match the given nameFilter, which can contain asterisks (*) for wildcards
// if nameFilter is blank, return all depots
func p4depots(ctx context.Context, host, username, password, nameFilter string) ([]perforce.DepotSpec, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if nameFilter == "" {
		cmd = exec.CommandContext(ctx, "p4", "-Mj", "-ztag", "depots")
	} else {
		cmd = exec.CommandContext(ctx, "p4", "-Mj", "-ztag", "depots", "-e", nameFilter)
	}
	cmd.Env = append(os.Environ(),
		"P4PORT="+host,
		"P4USER="+username,
		"P4PASSWD="+password,
	)

	out, err := runCommandCombinedOutput(ctx, wrexec.Wrap(ctx, log.NoOp(), cmd))
	if err != nil {
		if ctxerr := ctx.Err(); ctxerr != nil {
			err = errors.Wrap(ctxerr, "p4 depots context error")
		}
		if len(out) > 0 {
			err = errors.Wrapf(err, `failed to run command "p4 depots" (output follows)\n\n%s`, specifyCommandInErrorMessage(string(out), cmd))
		}
		return nil, err
	}
	depots := make([]perforce.DepotSpec, 0)
	if len(out) > 0 {
		// the output of `p4 -Mj -ztag depots` is a series of JSON-formatted depot definitions, one per line
		buf := bufio.NewScanner(bytes.NewBuffer(out))
		for buf.Scan() {
			depot := perforce.DepotSpec{}
			err := json.Unmarshal(buf.Bytes(), &depot)
			if err != nil {
				return nil, errors.Wrap(err, "malformed output from p4 depots")
			}
			depots = append(depots, depot)
		}
		if err := buf.Err(); err != nil {
			return nil, errors.Wrap(err, "malformed output from p4 depots")
		}
		return depots, nil
	}

	// no error, but also no depots. Maybe the user doesn't have access to any depots?
	return depots, nil
}

// p4streams returns all of the streams in a given depot
func p4streams(ctx context.Context, host, username, password, depot string) ([]perforce.StreamSpec, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "p4", "-Mj", "-ztag", "streams", depot+"...")
	cmd.Env = append(os.Environ(),
		"P4PORT="+host,
		"P4USER="+username,
		"P4PASSWD="+password,
	)

	out, err := runCommandCombinedOutput(ctx, wrexec.Wrap(ctx, log.NoOp(), cmd))
	if err != nil {
		if ctxerr := ctx.Err(); ctxerr != nil {
			err = ctxerr
		}
		if len(out) > 0 {
			err = errors.Wrapf(err, `failed to run command "p4 streams" (output follows)\n\n%s`, specifyCommandInErrorMessage(string(out), cmd))
		}
		return nil, err
	}
	streams := make([]perforce.StreamSpec, 0)
	if len(out) > 0 {
		// the output of `p4 -Mj -ztag streams` is a series of JSON-formatted stream definitions, one per line
		buf := bufio.NewScanner(bytes.NewBuffer(out))
		for buf.Scan() {
			stream := perforce.StreamSpec{}
			err := json.Unmarshal(buf.Bytes(), &stream)
			if err != nil {
				return nil, errors.Wrap(err, "malformed output from p4 streams")
			}
			streams = append(streams, stream)
		}
		if err := buf.Err(); err != nil {
			return nil, errors.Wrap(err, "malformed output from p4 streams")
		}
		return streams, nil
	}

	// no error, but also no streams. Maybe the depot isn't a streams depot?
	return streams, nil
}

func specifyCommandInErrorMessage(errorMsg string, command *exec.Cmd) string {
	if !strings.Contains(errorMsg, "this operation") {
		return errorMsg
	}
	if len(command.Args) == 0 {
		return errorMsg
	}
	return strings.Replace(errorMsg, "this operation", fmt.Sprintf("`%s`", strings.Join(command.Args, " ")), 1)
}

// p4testWithTrust attempts to test the Perforce server and performs a trust operation when needed.
func p4testWithTrust(ctx context.Context, host, username, password string) error {
	// Attempt to check connectivity, may be prompted to trust.
	err := p4test(ctx, host, username, password)
	if err == nil {
		return nil // The test worked, session still valid for the user
	}

	if strings.Contains(err.Error(), "To allow connection use the 'p4 trust' command.") {
		err := p4trust(ctx, host)
		if err != nil {
			return errors.Wrap(err, "trust")
		}
		return nil
	}

	// Something unexpected happened, bubble up the error
	return err
}

// FusionConfig allows configuration of the p4-fusion client
type FusionConfig struct {
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
