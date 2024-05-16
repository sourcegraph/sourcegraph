package gitcli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"runtime"
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
	"github.com/sourcegraph/sourcegraph/internal/bytesize"
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
	highMemoryCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "src_gitserver_exec_high_memory_usage_count",
		Help: "gitcli.GitCommand high memory usage by subcommand",
	}, []string{"cmd"})
)

type commandOpts struct {
	arguments []Argument

	stdin io.Reader
}

func optsFromFuncs(optFns ...CommandOptionFunc) commandOpts {
	var opts commandOpts
	for _, optFn := range optFns {
		optFn(&opts)
	}
	return opts
}

type unexportedArgumentDoNotImplement struct{}

type Argument interface {
	// Make sure some outside package doesn't implement this type by creating
	// a lowercase method on the interface.
	// Argument needs to be supported in the logic further down in this file
	// where []Argument is turned into the string slice of arguments to pass to
	// exec.Command.
	internalArgumentDoNotImplement() unexportedArgumentDoNotImplement
}

// FlagArgument is a string that is safe to pass to exec.Command without
// further sanitization. Use this type ONLY for strings we fully control like
// hardcoded flags.
type FlagArgument struct {
	string
}

func (FlagArgument) internalArgumentDoNotImplement() unexportedArgumentDoNotImplement {
	return unexportedArgumentDoNotImplement{}
}

// SpecSafeValueArgument is a string that is meant to be passed to git as an argument.
// Values of this type must not start with a `-` so they are not interpreted as a
// flag. This value will be verified when building the command.
type SpecSafeValueArgument struct {
	string
}

func (SpecSafeValueArgument) internalArgumentDoNotImplement() unexportedArgumentDoNotImplement {
	return unexportedArgumentDoNotImplement{}
}

// ValueFlagArgument is a flag that takes a value. The value is passed to git
// as a flag and the value joined with a `=`. Use this type ONLY for flag names we
// fully control like hardcoded strings `--points-at`.
type ValueFlagArgument struct {
	Flag  string
	Value string
}

func (ValueFlagArgument) internalArgumentDoNotImplement() unexportedArgumentDoNotImplement {
	return unexportedArgumentDoNotImplement{}
}

// ConfigArgument is a git config flag. Config flags must precede the subcommand
// specified, and are passed to git with the `-c` flag. The ordering will be handled
// by NewCommand.
// Ordering of ConfigArguments is guaranteed to be maintained.
type ConfigArgument struct {
	Key   string
	Value string
}

func (ConfigArgument) internalArgumentDoNotImplement() unexportedArgumentDoNotImplement {
	return unexportedArgumentDoNotImplement{}
}

type CommandOptionFunc func(*commandOpts)

// WithArguments sets the given arguments to the command arguments.
func WithArguments(args ...Argument) CommandOptionFunc {
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

func (g *gitCLIBackend) NewCommand(ctx context.Context, subcommand string, optFns ...CommandOptionFunc) (_ io.ReadCloser, err error) {
	opts := optsFromFuncs(optFns...)

	args, err := g.argsFromArguments(subcommand, opts.arguments)
	if err != nil {
		return nil, err
	}

	tr, ctx := trace.New(ctx, "gitcli.NewCommand",
		attribute.StringSlice("args", args),
		attribute.String("dir", g.dir.Path()),
	)
	defer func() {
		if err != nil {
			tr.EndWithErr(&err)
		}
	}()

	logger := g.logger.WithTrace(trace.Context(ctx))

	// If no deadline is set, use the default git command timeout.
	cancel := func() {}
	if _, ok := ctx.Deadline(); !ok {
		ctx, cancel = context.WithTimeout(ctx, gitCommandDefaultTimeout)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Cancel = func() error {
		// Send SIGKILL to the process group instead of just the process
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	g.dir.Set(cmd)

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// We use setpgid here so that child processes live in their own process groups.
	// This is helpful for two things:
	// - We can kill a process group to make sure that any subprocesses git might spawn
	//   will also receive the termination signal. The standard go implementation sends
	//   a SIGKILL only to the process itself. By using process groups, we can tell all
	//   children to shut down as well.
	// - We want to track maxRSS for tracking purposes to identify memory usage by command
	//   and linux tracks the maxRSS as "the maximum resident set size used (in kilobytes)"
	//   of the process in the process group that had the highest maximum resident set size.
	//   Read: If we don't use a separate process group here, we usually get the maxRSS from
	//   the process with the biggest memory usage in the process group, which is gitserver.
	//   So we cannot track the memory well. This is leaky, as it only tracks the largest sub-
	//   process, but it gives us a good indication of the general resource consumption.
	cmd.SysProcAttr.Setpgid = true

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

	execRunning.WithLabelValues(subcommand).Inc()

	cr := &cmdReader{
		ctx:        ctx,
		subcommand: subcommand,
		ctxCancel:  cancel,
		cmdStart:   cmdStart,
		stdout:     stdout,
		cmd:        wrappedCmd,
		stderr:     stderrBuf,
		repoName:   g.repoName,
		logger:     logger,
		git:        g,
		tr:         tr,
	}

	return cr, nil
}

// argsFromArguments takes a slice of arguments and returns a slice of strings
// that can be passed to exec.Command.
// This method makes sure to sanitize the arguments to avoid injection attacks,
// and makes some smart ordering decisions like putting ConfigArgument flags
// always at the beginning.
func (g *gitCLIBackend) argsFromArguments(subcommand string, args []Argument) ([]string, error) {
	var configFlags []string
	var flags []string

	for _, arg := range args {
		switch v := arg.(type) {
		case FlagArgument:
			// Safe flags are required to be sanitized by the caller, thus we can
			// just append them without further checking.
			flags = append(flags, v.string)
		case SpecSafeValueArgument:
			if err := checkSpecArgSafety(v.string); err != nil {
				return nil, err
			}
			flags = append(flags, v.string)
		case ValueFlagArgument:
			flags = append(flags, v.Flag+"="+v.Value)
		case ConfigArgument:
			configFlags = append(configFlags, "-c", v.Key+"="+v.Value)
			continue
		default:
			return nil, errors.Newf("unknown argument type %T", v)
		}
	}

	// The final command has to be of this form:
	//   git [config flags] subcommand [flags]
	out := make([]string, 0, len(configFlags)+1+len(flags))
	out = append(out, configFlags...)
	out = append(out, subcommand)
	out = append(out, flags...)

	return out, nil
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
	stdout     io.Reader
	ctx        context.Context
	ctxCancel  context.CancelFunc
	subcommand string
	cmdStart   time.Time
	cmd        wrexec.Cmder
	stderr     *bytes.Buffer
	logger     log.Logger
	git        git.GitBackend
	repoName   api.RepoName
	mu         sync.Mutex
	tr         trace.Trace
	err        error
	waitOnce   sync.Once
}

func (rc *cmdReader) Read(p []byte) (n int, err error) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	n, err = rc.stdout.Read(p)
	// If the command has finished, we close the stdout pipe and wait on the command
	// to free any leftover resources. If it errored, this will return the command
	// error from Read.
	if err == io.EOF {
		if err := rc.waitCmd(); err != nil {
			return n, err
		}
	}
	return n, err
}

func (rc *cmdReader) Close() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	return rc.waitCmd()
}

func (rc *cmdReader) waitCmd() error {
	// Waiting on a command should only happen once, so
	// we synchronize all potential calls to Read and Close
	// here, and memoize the error.
	rc.waitOnce.Do(func() {
		rc.err = rc.cmd.Wait()

		if rc.err != nil {
			if checkMaybeCorruptRepo(rc.logger, rc.git, rc.repoName, rc.stderr.String()) {
				rc.err = common.ErrRepoCorrupted{Reason: rc.stderr.String()}
			} else {
				rc.err = commandFailedError(rc.ctx, rc.err, rc.cmd, rc.stderr.Bytes())
			}
		}

		rc.trace()
		rc.tr.EndWithErr(&rc.err)
		rc.ctxCancel()
	})

	return rc.err
}

// const highMemoryUsageThreshold = 500 * bytesize.MiB

func (rc *cmdReader) trace() {
	duration := time.Since(rc.cmdStart)

	execRunning.WithLabelValues(rc.subcommand).Dec()
	execDuration.WithLabelValues(rc.subcommand).Observe(duration.Seconds())

	processState := rc.cmd.Unwrap().ProcessState
	var sysUsage syscall.Rusage
	s, ok := processState.SysUsage().(*syscall.Rusage)
	if ok {
		sysUsage = *s
	}

	memUsage := rssToByteSize(sysUsage.Maxrss)

	isSlow := duration > shortGitCommandSlow(rc.subcommand)
	// TODO: Disabled until this also works on linux, this only works on macOS right now
	// and causes noise.
	isHighMem := false // memUsage > highMemoryUsageThreshold

	if isHighMem {
		highMemoryCounter.WithLabelValues(rc.subcommand).Inc()
	}

	if honey.Enabled() || isSlow || isHighMem {
		act := actor.FromContext(rc.ctx)
		ev := honey.NewEvent("gitserver-exec")
		ev.SetSampleRate(HoneySampleRate(rc.subcommand, act))
		ev.AddField("repo", rc.repoName)
		ev.AddField("cmd", rc.subcommand)
		ev.AddField("args", rc.cmd.Unwrap().Args)
		ev.AddField("actor", act.UIDString())
		ev.AddField("exit_status", processState.ExitCode())
		if rc.err != nil {
			ev.AddField("error", rc.err.Error())
		}
		ev.AddField("cmd_duration_ms", duration.Milliseconds())
		ev.AddField("user_time_ms", processState.UserTime().Milliseconds())
		ev.AddField("system_time_ms", processState.SystemTime().Milliseconds())
		ev.AddField("cmd_ru_maxrss_kib", memUsage/bytesize.KiB)
		ev.AddField("cmd_ru_minflt", sysUsage.Minflt)
		ev.AddField("cmd_ru_majflt", sysUsage.Majflt)
		ev.AddField("cmd_ru_inblock", sysUsage.Inblock)
		ev.AddField("cmd_ru_oublock", sysUsage.Oublock)

		if traceID := trace.ID(rc.ctx); traceID != "" {
			ev.AddField("traceID", traceID)
			ev.AddField("trace", trace.URL(traceID))
		}

		if honey.Enabled() {
			_ = ev.Send()
		}

		if isSlow {
			rc.logger.Warn("Long exec request", log.Object("ev.Fields", mapToLoggerField(ev.Fields())...))
		}
		if isHighMem {
			rc.logger.Warn("High memory usage exec request", log.Object("ev.Fields", mapToLoggerField(ev.Fields())...))
		}
	}

	rc.tr.SetAttributes(attribute.Int("exit_code", processState.ExitCode()))
	rc.tr.SetAttributes(attribute.Int64("cmd_duration_ms", duration.Milliseconds()))
	rc.tr.SetAttributes(attribute.Int64("user_time_ms", processState.UserTime().Milliseconds()))
	rc.tr.SetAttributes(attribute.Int64("system_time_ms", processState.SystemTime().Milliseconds()))
	rc.tr.SetAttributes(attribute.Int64("cmd_ru_maxrss_kib", int64(memUsage/bytesize.KiB)))
	rc.tr.SetAttributes(attribute.Int64("cmd_ru_minflt", sysUsage.Minflt))
	rc.tr.SetAttributes(attribute.Int64("cmd_ru_majflt", sysUsage.Majflt))
	rc.tr.SetAttributes(attribute.Int64("cmd_ru_inblock", sysUsage.Inblock))
	rc.tr.SetAttributes(attribute.Int64("cmd_ru_oublock", sysUsage.Oublock))
}

func rssToByteSize(rss int64) bytesize.Bytes {
	if runtime.GOOS == "darwin" {
		// darwin tracks maxrss in bytes.
		return bytesize.Bytes(rss) * bytesize.B
	}
	// maxrss is tracked in KiB on Linux.
	return bytesize.Bytes(rss) * bytesize.KiB
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

func checkMaybeCorruptRepo(logger log.Logger, git git.GitBackend, repo api.RepoName, stderr string) bool {
	if !stdErrIndicatesCorruption(stderr) {
		return false
	}

	logger = logger.With(log.String("repo", string(repo)), log.String("repo", string(repo)))
	logger.Warn("marking repo for re-cloning due to stderr output indicating repo corruption", log.String("stderr", stderr))

	// We set a flag in the config for the cleanup janitor job to fix. The janitor
	// runs every minute.
	// We use a background context here to record corruption events even when the
	// context has since been cancelled.
	err := git.Config().Set(context.Background(), gitConfigMaybeCorrupt, strconv.FormatInt(time.Now().Unix(), 10))
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
func shortGitCommandSlow(subcommand string) time.Duration {
	switch subcommand {
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
