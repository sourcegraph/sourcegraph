package gitcli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/honey"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/wrexec"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	execRunning = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "src_gitserver_exec_running",
		Help: "number of gitserver.GitCommand running concurrently.",
	}, []string{"cmd"})
	execDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "src_gitserver_exec_duration_seconds",
		Help:    "gitserver.GitCommand latencies in seconds.",
		Buckets: prometheus.ExponentialBucketsRange(0.01, 60.0, 12),
	}, []string{"cmd"})
)

type commandOpts struct {
	arguments []string

	stdin io.Reader
}

func optsFromFuncs(optFns ...CommandOptionFunc) commandOpts {
	var opts commandOpts
	for _, optFn := range optFns {
		optFn(&opts)
	}
	return opts
}

type CommandOptionFunc func(*commandOpts)

// WithArguments sets the given arguments to the command arguments.
func WithArguments(args ...string) CommandOptionFunc {
	return func(o *commandOpts) {
		o.arguments = args
	}
}

// WithStdin specifies the reader to use for the command's stdin input.
func WithStdin(stdin io.Reader) CommandOptionFunc {
	return func(o *commandOpts) {
		o.stdin = stdin
	}
}

const gitCommandDefaultTimeout = time.Minute

func (g *gitCLIBackend) NewCommand(ctx context.Context, optFns ...CommandOptionFunc) (_ io.ReadCloser, err error) {
	opts := optsFromFuncs(optFns...)

	tr, ctx := trace.New(ctx, "gitcli.NewCommand",
		attribute.StringSlice("args", opts.arguments),
		attribute.String("dir", g.dir.Path()),
	)
	defer func() {
		if err != nil {
			tr.EndWithErr(&err)
		}
	}()

	logger := g.logger.WithTrace(trace.Context(ctx))

	if !IsAllowedGitCmd(logger, opts.arguments, g.dir) {
		blockedCommandExecutedCounter.Inc()
		return nil, ErrBadGitCommand
	}

	if len(opts.arguments) == 0 {
		// Technically can't happen because IsAllowedGitCmd catches this, but
		// if someone ever touches that logic, let's be safe before we access
		// args[0].
		return nil, errors.New("provide arguments")
	}

	subCmd := opts.arguments[0]

	// If no deadline is set, use the default git command timeout.
	cancel := func() {}
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, gitCommandDefaultTimeout)
	}

	cmd := exec.CommandContext(ctx, "git", opts.arguments...)
	g.dir.Set(cmd)

	stderr, stderrBuf := stderrBuffer()
	cmd.Stderr = stderr

	wrappedCmd := g.rcf.WrapWithRepoName(ctx, logger, g.repoName, cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, errors.Wrap(err, "failed to create stdout pipe")
	}

	if opts.stdin != nil {
		cmd.Stdin = opts.stdin
	}

	cmdStart := time.Now()

	if err := wrappedCmd.Start(); err != nil {
		cancel()
		return nil, errors.Wrap(err, "failed to start git process")
	}

	execRunning.WithLabelValues(subCmd).Inc()

	return &cmdReader{
		ctx:        ctx,
		subCmd:     subCmd,
		ctxCancel:  cancel,
		cmdStart:   cmdStart,
		ReadCloser: stdout,
		cmd:        wrappedCmd,
		stderr:     stderrBuf,
		repoName:   g.repoName,
		logger:     logger,
		git:        g,
		tr:         tr,
	}, nil
}

// ErrBadGitCommand is returned from the git CLI backend if the arguments provided
// are not allowed.
var ErrBadGitCommand = errors.New("bad git command, not allowed")

func commandFailedError(ctx context.Context, err error, cmd wrexec.Cmder, stderr []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return &CommandFailedError{
		Inner:      err,
		args:       cmd.Unwrap().Args,
		Stderr:     stderr,
		ExitStatus: cmd.Unwrap().ProcessState.ExitCode(),
	}
}

type CommandFailedError struct {
	Stderr     []byte
	ExitStatus int
	Inner      error
	args       []string
}

func (e *CommandFailedError) Unwrap() error {
	return e.Inner
}

func (e *CommandFailedError) Error() string {
	return fmt.Sprintf("git command %v failed with status code %d (output: %q)", e.args, e.ExitStatus, e.Stderr)
}

type cmdReader struct {
	io.ReadCloser
	ctx       context.Context
	ctxCancel context.CancelFunc
	subCmd    string
	cmdStart  time.Time
	cmd       wrexec.Cmder
	stderr    *bytes.Buffer
	logger    log.Logger
	git       git.GitBackend
	repoName  api.RepoName
	mu        sync.Mutex
	closed    bool
	tr        trace.Trace
	err       error
}

func (rc *cmdReader) Read(p []byte) (n int, err error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	n, err = rc.ReadCloser.Read(p)
	if err == io.EOF {
		rc.ReadCloser.Close()
		rc.closed = true

		if err := rc.waitCmd(); err != nil {
			return n, err
		}
	}
	return n, err
}

func (rc *cmdReader) Close() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.closed {
		return nil
	}

	// Close the underlying reader.
	err := rc.ReadCloser.Close()

	// And finalize the command.
	return errors.Append(err, rc.waitCmd())
}

func (rc *cmdReader) waitCmd() (err error) {
	defer rc.ctxCancel()

	defer rc.tr.EndWithErr(&err)

	rc.err = rc.cmd.Wait()

	if rc.err != nil {
		if checkMaybeCorruptRepo(rc.ctx, rc.logger, rc.git, rc.repoName, rc.stderr.String()) {
			rc.err = common.ErrRepoCorrupted{Reason: rc.stderr.String()}
		} else {
			rc.err = commandFailedError(rc.ctx, err, rc.cmd, rc.stderr.Bytes())
		}
	}

	rc.trace()

	return rc.err
}

func (rc *cmdReader) trace() {
	duration := time.Since(rc.cmdStart)

	execRunning.WithLabelValues(rc.subCmd).Dec()
	execDuration.WithLabelValues(rc.subCmd).Observe(duration.Seconds())

	isSlow := duration > shortGitCommandSlow(rc.cmd.Unwrap().Args)
	if honey.Enabled() || isSlow {
		act := actor.FromContext(rc.ctx)
		ev := honey.NewEvent("gitserver-exec")
		ev.SetSampleRate(HoneySampleRate(rc.subCmd, act))
		ev.AddField("repo", rc.repoName)
		ev.AddField("cmd", rc.subCmd)
		ev.AddField("args", rc.cmd.Unwrap().Args)
		ev.AddField("actor", act.UIDString())
		ev.AddField("duration_ms", duration.Milliseconds())
		ev.AddField("exit_status", rc.cmd.Unwrap().ProcessState.ExitCode())
		if rc.err != nil {
			ev.AddField("error", rc.err.Error())
		}
		ev.AddField("cmd_duration_ms", duration.Milliseconds())
		ev.AddField("user_time", rc.cmd.Unwrap().ProcessState.UserTime())
		ev.AddField("system_time", rc.cmd.Unwrap().ProcessState.SystemTime())

		if traceID := trace.ID(rc.ctx); traceID != "" {
			ev.AddField("traceID", traceID)
			ev.AddField("trace", trace.URL(traceID, conf.DefaultClient()))
		}

		if honey.Enabled() {
			_ = ev.Send()
		}

		if isSlow {
			rc.logger.Warn("Long exec request", log.Object("ev.Fields", mapToLoggerField(ev.Fields())...))
		}
	}

	ps := rc.cmd.Unwrap().ProcessState
	var rusage syscall.Rusage
	if su, ok := ps.SysUsage().(*syscall.Rusage); ok {
		rusage = *su
	}

	rc.tr.SetAttributes(attribute.Int("exitCode", ps.ExitCode()))
	rc.tr.SetAttributes(attribute.Int64("cmd_duration_ms", duration.Milliseconds()))
	rc.tr.SetAttributes(attribute.Int64("user_time_ms", ps.UserTime().Milliseconds()))
	rc.tr.SetAttributes(attribute.Int64("system_time_ms", ps.SystemTime().Milliseconds()))
	// Track the maximum resident set size of the process during its lifetime.
	rc.tr.SetAttributes(attribute.Int64("max_rss", rusage.Maxrss))
	// Track the number of times the filesystem had to perform input.
	rc.tr.SetAttributes(attribute.Int64("inblock", rusage.Inblock))
	// Track the number of times the filesystem had to perform output.
	rc.tr.SetAttributes(attribute.Int64("oublock", rusage.Oublock))
}

const maxStderrCapture = 1024

// stderrBuffer sets up a limited buffer to capture stderr for error reporting.
func stderrBuffer() (io.Writer, *bytes.Buffer) {
	stderrBuf := bytes.NewBuffer(make([]byte, 0, maxStderrCapture))
	stderr := &limitWriter{W: stderrBuf, N: maxStderrCapture}
	return stderr, stderrBuf
}

// limitWriter is a io.Writer that writes to an W but discards after N bytes.
type limitWriter struct {
	W io.Writer // underling writer
	N int       // max bytes remaining
}

func (l *limitWriter) Write(p []byte) (int, error) {
	if l.N <= 0 {
		return len(p), nil
	}
	origLen := len(p)
	if len(p) > l.N {
		p = p[:l.N]
	}
	n, err := l.W.Write(p)
	l.N -= n
	if l.N <= 0 {
		// If we have written limit bytes, then we can include the discarded
		// part of p in the count.
		n = origLen
	}
	return n, err
}

func checkMaybeCorruptRepo(ctx context.Context, logger log.Logger, git git.GitBackend, repo api.RepoName, stderr string) bool {
	if !stdErrIndicatesCorruption(stderr) {
		return false
	}

	logger = logger.With(log.String("repo", string(repo)), log.String("repo", string(repo)))
	logger.Warn("marking repo for re-cloning due to stderr output indicating repo corruption", log.String("stderr", stderr))

	// We set a flag in the config for the cleanup janitor job to fix. The janitor
	// runs every minute.
	err := git.Config().Set(ctx, gitConfigMaybeCorrupt, strconv.FormatInt(time.Now().Unix(), 10))
	if err != nil {
		logger.Error("failed to set maybeCorruptRepo config", log.Error(err))
	}

	return true
}

// gitConfigMaybeCorrupt is a key we add to git config to signal that a repo may be
// corrupt on disk.
const gitConfigMaybeCorrupt = "sourcegraph.maybeCorruptRepo"

var (
	// objectOrPackFileCorruptionRegex matches stderr lines from git which indicate
	// that a repository's packfiles or commit objects might be corrupted.
	//
	// See https://github.com/sourcegraph/sourcegraph/issues/6676 for more
	// context.
	objectOrPackFileCorruptionRegex = lazyregexp.NewPOSIX(`^error: (Could not read|packfile) `)

	// objectOrPackFileCorruptionRegex matches stderr lines from git which indicate that
	// git's supplemental commit-graph might be corrupted.
	//
	// See https://github.com/sourcegraph/sourcegraph/issues/37872 for more
	// context.
	commitGraphCorruptionRegex = lazyregexp.NewPOSIX(`^fatal: commit-graph requires overflow generation data but has none`)
)

// stdErrIndicatesCorruption returns true if the provided stderr output from a git command indicates
// that there might be repository corruption.
func stdErrIndicatesCorruption(stderr string) bool {
	return objectOrPackFileCorruptionRegex.MatchString(stderr) || commitGraphCorruptionRegex.MatchString(stderr)
}

// shortGitCommandSlow returns the threshold for regarding an git command as
// slow. Some commands such as "git archive" are inherently slower than "git
// rev-parse", so this will return an appropriate threshold given the command.
func shortGitCommandSlow(args []string) time.Duration {
	if len(args) < 1 {
		return time.Second
	}
	switch args[0] {
	case "archive":
		return 1 * time.Minute

	case "blame", "ls-tree", "log", "show":
		return 5 * time.Second

	case "fetch":
		return 10 * time.Second

	default:
		return 2500 * time.Millisecond
	}
}

// mapToLoggerField translates a map to log context fields.
func mapToLoggerField(m map[string]any) []log.Field {
	LogFields := []log.Field{}

	for i, v := range m {

		LogFields = append(LogFields, log.String(i, fmt.Sprint(v)))
	}

	return LogFields
}

// Send 1 in 16 events to honeycomb. This is hardcoded since we only use this
// for Sourcegraph.com.
//
// 2020-05-29 1 in 4. We are currently at the top tier for honeycomb (before
// enterprise) and using double our quota. This gives us room to grow. If you
// find we keep bumping this / missing data we care about we can look into
// more dynamic ways to sample in our application code.
//
// 2020-07-20 1 in 16. Again hitting very high usage. Likely due to recent
// scaling up of the indexed search cluster. Will require more investigation,
// but we should probably segment user request path traffic vs internal batch
// traffic.
//
// 2020-11-02 Dynamically sample. Again hitting very high usage. Same root
// cause as before, scaling out indexed search cluster. We update our sampling
// to instead be dynamic, since "rev-parse" is 12 times more likely than the
// next most common command.
//
// 2021-08-20 over two hours we did 128 * 128 * 1e6 rev-parse requests
// internally. So we update our sampling to heavily downsample internal
// rev-parse, while upping our sampling for non-internal.
// https://ui.honeycomb.io/sourcegraph/datasets/gitserver-exec/result/67e4bLvUddg
//
// 2024-02-23 we are now capturing all execs done in honeycomb, including
// internal stuff like config and janitor jobs. In particular "config" is now
// running as often as rev-parse. rev-list is also higher than most so we
// include it in the big sample rate.
func HoneySampleRate(cmd string, actor *actor.Actor) uint {
	// HACK(keegan) 2022-11-02 IsInternal on sourcegraph.com is always
	// returning false. For now I am also marking it internal if UID is not
	// set to work around us hammering honeycomb.
	internal := actor.IsInternal() || actor.UID == 0
	switch {
	case (cmd == "rev-parse" || cmd == "rev-list" || cmd == "config") && internal:
		return 1 << 14 // 16384

	case internal:
		// we care more about user requests, so downsample internal more.
		return 16

	default:
		return 8
	}
}
