package run

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/conc/pool"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

type cmdRunner struct {
	*std.Output
	cmds      []SGConfigCommand
	parentEnv map[string]string
	verbose   bool
}

func Commands(ctx context.Context, parentEnv map[string]string, verbose bool, cmds []SGConfigCommand) (err error) {
	if len(cmds) == 0 {
		// Exit early if there are no commands to run.
		return nil
	}
	std.Out.WriteLine(output.Styled(output.StylePending, fmt.Sprintf("Starting %d cmds", len(cmds))))

	repoRoot := cmds[0].GetConfig().RepositoryRoot
	// binaries get installed to <repository-root>/.bin. If the binary is installed with go build, then go
	// will create .bin directory. Some binaries (like docsite) get downloaded instead of built and therefore
	// need the directory to exist before hand.
	binDir := filepath.Join(repoRoot, ".bin")
	if err := os.Mkdir(binDir, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	if err := writePid(); err != nil {
		return err
	}

	runner := cmdRunner{
		std.Out,
		cmds,
		parentEnv,
		verbose,
	}

	return runner.run(ctx)
}

func (runner *cmdRunner) run(ctx context.Context) error {
	p := pool.New().WithContext(ctx).WithCancelOnError().WithFirstError()
	// Start each command concurrently
	for _, cmd := range runner.cmds {
		cmd := cmd
		p.Go(func(ctx context.Context) error {
			config := cmd.GetConfig()
			std.Out.WriteLine(output.Styledf(output.StylePending, "Running %s...", config.Name))

			// Start watching the commands dependencies
			wantRestart, err := cmd.StartWatch(ctx)
			if err != nil {
				runner.printError(cmd, err)
				return err
			}

			// start up the binary
			proc, err := runner.start(ctx, cmd)
			if err != nil {
				runner.printError(cmd, err)
				return errors.Wrapf(err, "failed to start command %q", config.Name)
			}
			defer proc.cancel()

			// Wait forever until we're asked to stop or that restarting returns an error.
			for {
				select {
				// Handle context cancelled
				case <-ctx.Done():
					return ctx.Err()

				// Handle process exit
				case err := <-proc.Exit():
					// If the process failed, we exit immediately
					if err != nil {
						return err
					}

					runner.WriteLine(output.Styledf(output.StyleSuccess, "%s%s exited without error%s", output.StyleBold, config.Name, output.StyleReset))

					// If we shouldn't restart when the process exits, return
					if !config.ContinueWatchOnExit {
						return nil
					}

				// handle file watcher triggered
				case <-wantRestart:
					// If the command has an installer, re-run the install and determine if we should restart
					runner.WriteLine(output.Styledf(output.StylePending, "Change detected. Reloading %s...", config.Name))
					shouldRestart, err := runner.reinstall(ctx, cmd)
					if err != nil {
						runner.printError(cmd, err)

						// If the error came from the install step then we continue watching
						// and will retry building if there is another source change.
						if _, ok := err.(installErr); ok {
							continue
						}
						return err
					}

					if shouldRestart {
						runner.WriteLine(output.Styledf(output.StylePending, "Restarting %s...", config.Name))
						proc.cancel()
						proc, err = runner.start(ctx, cmd)
						if err != nil {
							return err
						}
						defer proc.cancel()
					} else {
						runner.WriteLine(output.Styledf(output.StylePending, "Binary for %s did not change. Not restarting.", config.Name))
					}
				}
			}
		})
	}

	return p.Wait()
}

func (runner *cmdRunner) printError(cmd SGConfigCommand, err error) {
	printCmdError(runner.Output.Output, cmd.GetConfig().Name, err)
}

func (runner *cmdRunner) debug(msg string, args ...any) { //nolint currently unused but a handy tool for debugginlg
	if runner.verbose {
		message := fmt.Sprintf(msg, args...)
		runner.WriteLine(output.Styledf(output.StylePending, "%s[DEBUG]: %s %s", output.StyleBold, output.StyleReset, message))
	}
}

func (runner *cmdRunner) start(ctx context.Context, cmd SGConfigCommand) (*startedCmd, error) {
	return startSgCmd(ctx, cmd, runner.parentEnv)
}

func (runner *cmdRunner) reinstall(ctx context.Context, cmd SGConfigCommand) (bool, error) {
	if installer, ok := cmd.(Installer); !ok {
		// If there is no installer, then we always restart
		return true, nil
	} else {
		bin, err := cmd.GetBinaryLocation()
		if err != nil {
			// If the command doesn't have a CheckBinary, we just ignore it
			if errors.Is(err, noBinaryError{}) {
				return false, nil
			} else {
				return false, err
			}
		}

		oldHash, err := md5HashFile(bin)
		if err != nil {
			return false, err
		}

		if err := installer.RunInstall(ctx, runner.parentEnv); err != nil {
			return false, err
		}
		newHash, err := md5HashFile(bin)
		if err != nil {
			return false, err
		}

		return oldHash != newHash, nil
	}
}

// installErr is returned by runWatch if the cmd.Install step fails.
type installErr struct {
	cmdName string
	output  string

	originalErr error
}

func (e installErr) Error() string {
	return fmt.Sprintf("install of %s failed: %s", e.cmdName, e.output)
}

// runErr is used internally by runWatch to print a message when a
// command failed to reinstall.
type runErr struct {
	cmdName  string
	exitCode int
	stderr   string
	stdout   string
}

func (e runErr) Error() string {
	return fmt.Sprintf("failed to run %s.\nstderr:\n%s\nstdout:\n%s\n", e.cmdName, e.stderr, e.stdout)
}

func printCmdError(out *output.Output, cmdName string, err error) {
	// Don't log context canceled errors because they are not the root issue
	if errors.Is(err, context.Canceled) {
		return
	}

	var message, cmdOut string
	switch e := errors.Cause(err).(type) {
	case installErr:
		message = "Failed to build " + cmdName
		if e.originalErr != nil {
			if errWithout, ok := e.originalErr.(errorWithoutOutputer); ok {
				// If we can, let's strip away the output, otherwise this gets
				// too noisy.
				message += ": " + errWithout.ErrorWithoutOutput()
			} else {
				message += ": " + e.originalErr.Error()
			}
		}
		cmdOut = e.output
	case runErr:
		message = "Failed to run " + cmdName
		cmdOut = fmt.Sprintf("Exit code: %d\n\n", e.exitCode)

		if len(strings.TrimSpace(e.stdout)) > 0 {
			formattedStdout := "\t" + strings.Join(strings.Split(e.stdout, "\n"), "\n\t")
			cmdOut += fmt.Sprintf("Standard out:\n%s\n", formattedStdout)
		}

		if len(strings.TrimSpace(e.stderr)) > 0 {
			formattedStderr := "\t" + strings.Join(strings.Split(e.stderr, "\n"), "\n\t")
			cmdOut += fmt.Sprintf("Standard err:\n%s\n", formattedStderr)
		}

	default:
		var exc *exec.ExitError
		// recurse if it is an exit error
		if errors.As(err, &exc) {
			printCmdError(out, cmdName, runErr{
				cmdName:  cmdName,
				exitCode: exc.ExitCode(),
				stderr:   string(exc.Stderr),
			})
			return
		} else {
			message = fmt.Sprintf("Failed to run %s: %+v", cmdName, err)
		}

	}

	separator := strings.Repeat("-", 80)
	if cmdOut != "" {
		line := output.Linef(
			"", output.StyleWarning,
			"%s\n%s%s:\n%s%s%s%s%s",
			separator, output.StyleBold, message, output.StyleReset,
			cmdOut, output.StyleWarning, separator, output.StyleReset,
		)
		out.WriteLine(line)
	} else {
		line := output.Linef(
			"", output.StyleWarning,
			"%s\n%s%s\n%s%s",
			separator, output.StyleBold, message,
			separator, output.StyleReset,
		)
		out.WriteLine(line)
	}
}

// makeEnv merges environments starting from the left, meaning the first environment will be overriden by the second one, skipping
// any key that has been explicitly defined in the current environment of this process. This enables users to manually overrides
// environment variables explictly, i.e FOO=1 sg start will have FOO=1 set even if a command or commandset sets FOO.
func makeEnv(envs ...map[string]string) (combined []string) {
	for k, v := range makeEnvMap(envs...) {
		combined = append(combined, fmt.Sprintf("%s=%s", k, v))
	}
	return combined
}

func makeEnvMap(envs ...map[string]string) map[string]string {
	combined := map[string]string{}
	for _, pair := range os.Environ() {
		elems := strings.SplitN(pair, "=", 2)
		if len(elems) != 2 {
			panic("space/time continuum wrong")
		}

		combined[elems[0]] = elems[1]
	}

	for _, env := range envs {
		for k, v := range env {
			if _, ok := os.LookupEnv(k); ok {
				// If the key is already set in the process env, we don't
				// overwrite it. That way we can do something like:
				//
				//	SRC_LOG_LEVEL=debug sg run enterprise-frontend
				//
				// to overwrite the default value in sg.config.yaml
				continue
			}

			// Expand env vars and keep track of previously set env vars
			// so they can be used when expanding too.
			// TODO: using range to iterate over the env is not stable and thus
			// this won't work
			expanded := os.Expand(v, func(lookup string) string {
				// If we're looking up the key that we're trying to define, we
				// skip the self-reference and look in the OS
				if lookup == k {
					return os.Getenv(lookup)
				}

				if e, ok := env[lookup]; ok {
					return e
				}
				return os.Getenv(lookup)
			})
			combined[k] = expanded
		}
	}

	return combined
}

func md5HashFile(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return string(h.Sum(nil)), nil
}

func Test(ctx context.Context, cmd SGConfigCommand, parentEnv map[string]string) error {
	name := cmd.GetConfig().Name

	std.Out.WriteLine(output.Styledf(output.StylePending, "Starting testsuite %q.", name))
	proc, err := startSgCmd(ctx, cmd, parentEnv)
	if err != nil {
		printCmdError(std.Out.Output, name, err)
	}
	return proc.Wait()
}
