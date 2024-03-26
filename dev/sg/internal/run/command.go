package run

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/grafana/regexp"
	"github.com/sourcegraph/conc/pool"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/secrets"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/interrupt"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
	"github.com/sourcegraph/sourcegraph/lib/process"
)

type Command struct {
	Config      SGConfigCommandOptions
	Cmd         string   `yaml:"cmd"`
	DefaultArgs string   `yaml:"defaultArgs"`
	Install     string   `yaml:"install"`
	InstallFunc string   `yaml:"install_func"`
	CheckBinary string   `yaml:"checkBinary"`
	Watch       []string `yaml:"watch"`

	// ATTENTION: If you add a new field here, be sure to also handle that
	// field in `Merge` (below).
}

// UnmarshalYAML implements the Unmarshaler interface for Command.
// This allows us to parse the flat YAML configuration into nested struct.
func (cmd *Command) UnmarshalYAML(unmarshal func(any) error) error {
	// In order to not recurse infinitely (calling UnmarshalYAML over and over) we create a
	// temporary type alias.
	// First parse the Command specific options
	type rawCommand Command
	if err := unmarshal((*rawCommand)(cmd)); err != nil {
		return err
	}

	// Then parse the common options from the same list into a nested struct
	return unmarshal(&cmd.Config)
}

func (cmd Command) GetConfig() SGConfigCommandOptions {
	return cmd.Config
}

func (cmd Command) UpdateConfig(f func(*SGConfigCommandOptions)) SGConfigCommand {
	f(&cmd.Config)
	return cmd
}

func (cmd Command) GetName() string {
	return cmd.Config.Name
}

func (cmd Command) GetBinaryLocation() (string, error) {
	if cmd.CheckBinary != "" {
		return filepath.Join(cmd.Config.RepositoryRoot, cmd.CheckBinary), nil
	}
	return "", noBinaryError{name: cmd.Config.Name}
}

func (cmd Command) GetBazelTarget() string {
	return ""
}

func (cmd Command) GetExecCmd(ctx context.Context) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, "bash", "-c", cmd.Cmd), nil
}

func (cmd Command) RunInstall(ctx context.Context, parentEnv map[string]string) error {
	if cmd.requiresInstall() {
		if cmd.hasBashInstaller() {
			return cmd.bashInstall(ctx, parentEnv)
		} else {
			return cmd.functionInstall(ctx, parentEnv)
		}
	}

	return nil
}

// Standard commands ignore installer
func (cmd Command) SetInstallerOutput(chan<- output.FancyLine) {}

func (cmd Command) Count() int {
	return 1
}

func (cmd Command) requiresInstall() bool {
	return cmd.Install != "" || cmd.InstallFunc != ""
}

func (cmd Command) hasBashInstaller() bool {
	return cmd.Install != "" || cmd.InstallFunc == ""
}

func (cmd Command) bashInstall(ctx context.Context, parentEnv map[string]string) error {
	output, err := BashInRoot(ctx, cmd.Install, makeEnv(parentEnv, cmd.Config.Env))
	if err != nil {
		return installErr{cmdName: cmd.Config.Name, output: output, originalErr: err}
	}
	return nil
}

func (cmd Command) functionInstall(ctx context.Context, parentEnv map[string]string) error {
	fn, ok := installFuncs[cmd.InstallFunc]
	if !ok {
		return installErr{cmdName: cmd.Config.Name, originalErr: errors.Newf("no install func with name %q found", cmd.InstallFunc)}
	}
	if err := fn(ctx, makeEnvMap(parentEnv, cmd.Config.Env)); err != nil {
		return installErr{cmdName: cmd.Config.Name, originalErr: err}
	}

	return nil
}

func (cmd Command) getWatchPaths() []string {
	fullPaths := make([]string, len(cmd.Watch))
	for i, path := range cmd.Watch {
		fullPaths[i] = filepath.Join(cmd.Config.RepositoryRoot, path)
	}

	return fullPaths
}

func (cmd Command) StartWatch(ctx context.Context) (<-chan struct{}, error) {
	return WatchPaths(ctx, cmd.getWatchPaths())
}

func (c Command) Merge(other Command) Command {
	merged := c

	merged.Config = c.Config.Merge(other.Config)
	merged.Cmd = mergeStrings(c.Cmd, other.Cmd)
	merged.Install = mergeStrings(c.Install, other.Install)
	merged.InstallFunc = mergeStrings(c.InstallFunc, other.InstallFunc)
	merged.Watch = mergeSlices(c.Watch, other.Watch)
	return merged
}

func mergeStrings(a, b string) string {
	if b != "" {
		return b
	}
	return a
}

func mergeSlices[T any](a, b []T) []T {
	if len(b) > 0 {
		return b
	}
	return a
}

// Merge maps properly merges the two, as opposed to every other merge method which
// simply overwrites the first with the second.
// This is to preserve the behavior of the original code.
func mergeMaps[K comparable, V any](a, b map[K]V) map[K]V {
	if a == nil {
		return b
	}
	for k, v := range b {
		a[k] = v
	}

	return a
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i, v := range a {
		if v != b[i] {
			return false
		}
	}

	return true
}

var sgConn net.Conn

func OpenUnixSocket() error {
	var err error
	sgConn, err = net.Dial("unix", "/tmp/sg.sock")
	return err
}

func getSecrets(ctx context.Context, name string, extSecrets map[string]secrets.ExternalSecret) (map[string]string, error) {
	secretsEnv := map[string]string{}

	if len(extSecrets) == 0 {
		return secretsEnv, nil
	}

	secretsStore, err := secrets.FromContext(ctx)
	if err != nil {
		return nil, errors.Errorf("failed to get secrets store: %v", err)
	}

	var errs error
	for envName, secret := range extSecrets {
		secretsEnv[envName], err = secretsStore.GetExternal(ctx, secret)
		if err != nil {
			errs = errors.Append(errs,
				errors.Wrapf(err, "failed to access secret %q for command %q", envName, name))
		}
	}
	return secretsEnv, errs
}

type startedCmd struct {
	*exec.Cmd
	opts   commandOptions
	cancel func()

	outEg  *pool.ErrorPool
	result chan error
}

type commandOptions struct {
	name   string
	exec   *exec.Cmd
	dir    string
	env    []string
	stdout outputOptions
	stderr outputOptions
}

type outputOptions struct {
	// When true, output will be ignored and not written to any writers
	ignore bool

	// When non-nil, all output will be flushed to this file and not to the terminal
	logfile io.Writer

	// when enabled, output will not be streamed to the writers until
	// after the process is begun, only captured for later retrieval
	buffer bool

	// Buffer that captures the output for error logging
	captured io.ReadWriter

	// Additional writers to write output to
	additionalWriters []io.Writer

	// Channel that is used to signal that output should start streaming
	// when buffer is true
	start chan struct{}
}

func startSgCmd(ctx context.Context, cmd SGConfigCommand, parentEnv map[string]string) (*startedCmd, error) {
	exec, err := cmd.GetExecCmd(ctx)
	if err != nil {
		return nil, err
	}

	conf := cmd.GetConfig()

	secretsEnv, err := getSecrets(ctx, conf.Name, conf.ExternalSecrets)
	if err != nil {
		std.Out.WriteLine(output.Styledf(output.StyleWarning, "[%s] %s %s",
			conf.Name, output.EmojiFailure, err.Error()))
	}

	opts := commandOptions{
		name:   conf.Name,
		exec:   exec,
		env:    makeEnv(parentEnv, secretsEnv, conf.Env),
		dir:    conf.RepositoryRoot,
		stdout: outputOptions{ignore: conf.IgnoreStdout},
		stderr: outputOptions{ignore: conf.IgnoreStderr},
	}
	if conf.Logfile != "" {
		if logfile, err := initLogFile(conf.Logfile); err != nil {
			return nil, err
		} else {
			opts.stdout.logfile = logfile
			opts.stderr.logfile = logfile
		}
	}

	if conf.Preamble != "" {
		std.Out.WriteLine(output.Styledf(output.StyleOrange, "[%s] %s %s", conf.Name, output.EmojiInfo, conf.Preamble))
	}

	return startCmd(ctx, opts)
}

func initLogFile(logfile string) (io.Writer, error) {
	if strings.HasPrefix(logfile, "~/") || strings.HasPrefix(logfile, "$HOME") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get user home directory")
		}
		logfile = filepath.Join(home, strings.Replace(strings.Replace(logfile, "~/", "", 1), "$HOME", "", 1))
	}
	parent := filepath.Dir(logfile)
	if err := os.MkdirAll(parent, os.ModePerm); err != nil {
		return nil, err
	}
	// we don't have to worry about the file existing already and growing large, since this will truncate the file if it exists
	return os.Create(logfile)
}

func startCmd(ctx context.Context, opts commandOptions) (*startedCmd, error) {
	sc := &startedCmd{
		opts: opts,
	}

	ctx, cancel := context.WithCancel(ctx)
	sc.cancel = func() {
		// The default cancel function will use a SIGKILL (9) which does
		// not allow processes to cleanup. If they have spawned child processes
		// those child processes will be orphaned and continue running.
		// SIGINT will instead gracefully shut down the process and child processes.
		if sc.Cmd.Process != nil {
			// We created a process group above which we kill here.
			pgid, err := syscall.Getpgid(sc.Cmd.Process.Pid)
			if err != nil {
				// Ignore Errno 3 (No such process) as this means the process has already exited
				if !errors.Is(err, syscall.Errno(0x3)) {
					panic(errors.Wrapf(err, "failed to get process group ID for %s (PID %d)", sc.opts.name, sc.Cmd.Process.Pid))
				}
				// note the minus sign; this signals that we want to kill the whole process group
			} else if err := syscall.Kill(-pgid, syscall.SIGINT); err != nil {
				panic(errors.Wrapf(err, "failed kill process group ID %d for cmd %s ", pgid, sc.opts.name))
			}
		}

		cancel()
	}
	// Register an interrput handler
	interrupt.Register(sc.cancel)

	sc.Cmd = opts.exec
	sc.Cmd.Dir = opts.dir
	sc.Cmd.Env = opts.env

	// This sets up a process group which we kill later.
	// This allows us to ensure that any child processes are killed as well when this exits
	// This will only work on POSIX systems
	sc.Cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := sc.connectOutput(ctx); err != nil {
		sc.cancel()
		return nil, err
	}

	if err := sc.Start(); err != nil {
		sc.cancel()
		return nil, err
	}
	return sc, nil
}

func (sc *startedCmd) connectOutput(ctx context.Context) error {
	stdoutWriter := sc.getOutputWriter(ctx, &sc.opts.stdout, "stdout")
	stderrWriter := sc.getOutputWriter(ctx, &sc.opts.stderr, "stderr")

	eg, err := process.PipeOutputUnbuffered(ctx, sc.Cmd, stdoutWriter, stderrWriter)
	if err != nil {
		return err
	}
	sc.outEg = eg

	return nil
}

func (sc *startedCmd) getOutputWriter(ctx context.Context, opts *outputOptions, outputName string) io.Writer {
	writers := opts.additionalWriters
	if writers == nil {
		writers = []io.Writer{}
	}
	if opts.captured == nil {
		opts.captured = &prefixSuffixSaver{N: 32 << 10}
	}
	writers = append(writers, opts.captured)

	if opts.ignore {
		std.Out.WriteLine(output.Styledf(output.StyleSuggestion, "Ignoring %s of %s", outputName, sc.opts.name))
	} else if opts.logfile != nil {
		return opts.logfile
	} else {
		// Create a channel to signal when output should start. If buffering is disabled, close
		// the channel so output starts immediately.
		opts.start = make(chan struct{})
		if !opts.buffer {
			close(opts.start)
		}

		writers = append(writers, newBufferedCmdLogger(ctx, sc.opts.name, std.Out.Output, opts.start))
	}

	if sgConn != nil {
		sink := func(data string) {
			sgConn.Write([]byte(fmt.Sprintf("%s: %s\n", sc.opts.name, data)))
		}
		writers = append(writers, process.NewLogger(ctx, sink))
	}

	return io.MultiWriter(writers...)
}

func (sc *startedCmd) Exit() <-chan error {
	if sc.result == nil {
		sc.result = make(chan error)
		go func() {
			sc.result <- sc.Wait()
		}()
	}
	return sc.result
}

func (sc *startedCmd) Wait() error {
	err := sc.wait()
	var e *exec.ExitError
	if errors.As(err, &e) {
		err = runErr{
			cmdName:  sc.opts.name,
			exitCode: e.ExitCode(),
			stderr:   sc.CapturedStderr(),
			stdout:   sc.CapturedStdout(),
		}
	}

	return err
}

func (sc *startedCmd) wait() error {
	if err := sc.outEg.Wait(); err != nil {
		return err
	}
	return sc.Cmd.Wait()
}

func (sc *startedCmd) CapturedStdout() string {
	return captured(sc.opts.stdout)
}

func (sc *startedCmd) CapturedStderr() string {
	return captured(sc.opts.stderr)
}

func captured(opts outputOptions) string {
	if opts.captured == nil {
		return ""
	}

	if output, err := io.ReadAll(opts.captured); err == nil {
		return string(output)
	}

	return ""
}

// Begins writing output to StdOut and StdErr if it was previously buffered
func (sc *startedCmd) StartOutput() {
	sc.startOutput(sc.opts.stdout)
	sc.startOutput(sc.opts.stderr)
}

func (sc *startedCmd) startOutput(opts outputOptions) {
	if opts.buffer && opts.start != nil {
		close(opts.start)
	}
}

// patternMatcher is writer which looks for a regular expression in the
// written bytes and calls a callback if a match is found
// by default it only looks for the matched pattern once
type patternMatcher struct {
	regex    *regexp.Regexp
	callback func()
	buffer   bytes.Buffer
	multi    bool
	disabled bool
}

func (writer *patternMatcher) Write(p []byte) (int, error) {
	if writer.disabled {
		return len(p), nil
	}
	n, err := writer.buffer.Write(p)
	if err != nil {
		return n, err
	}
	if writer.regex.MatchReader(&writer.buffer) {
		writer.callback()
		if !writer.multi {
			writer.disabled = true
		}
	}
	return n, err
}
