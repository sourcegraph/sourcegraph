package run

import (
	"context"
	"os/exec"
	"path/filepath"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/env"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/secrets"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

type Command2 struct {
	Config      SGConfigCommandOptions2
	Cmd         string   `yaml:"cmd"`
	DefaultArgs string   `yaml:"defaultArgs"`
	Install     string   `yaml:"install"`
	InstallFunc string   `yaml:"install_func"`
	CheckBinary string   `yaml:"checkBinary"`
	Watch       []string `yaml:"watch"`

	// ATTENTION: If you add a new field here, be sure to also handle that
	// field in `Merge` (below).
	envPriority env.Priority
}

// UnmarshalYAML implements the Unmarshaler interface for Command.
// This allows us to parse the flat YAML configuration into nested struct.
func (cmd *Command2) UnmarshalYAML(unmarshal func(any) error) error {
	// In order to not recurse infinitely (calling UnmarshalYAML over and over) we create a
	// temporary type alias.
	// First parse the Command specific options
	type rawCommand Command2
	if err := unmarshal((*rawCommand)(cmd)); err != nil {
		return err
	}

	var tempConfig struct {
		Description string `yaml:"description"`
		// A command to be run before the command is run but after installation
		PreCmd string `yaml:"precmd"`
		// A list of additional arguments to be passed to the command
		Args         string            `yaml:"args"`
		Env          map[string]string `yaml:"env"`
		IgnoreStdout bool              `yaml:"ignoreStdout"`
		IgnoreStderr bool              `yaml:"ignoreStderr"`
		// If true, the runner will continue watching this commands dependencies
		// if the command's last execution was successful (i.e exitCode = 0)
		ContinueWatchOnExitZero bool `yaml:"continueWatchOnExit"`
		// Preamble is a short and visible message, displayed when the command is launched.
		Preamble string `yaml:"preamble"`

		// Output all logs to a file instead of to stdout/stderr
		Logfile         string                            `yaml:"logfile"`
		ExternalSecrets map[string]secrets.ExternalSecret `yaml:"externalSecrets"`

		RepositoryRoot string
	}

	// Then parse the common options from the same list into a nested struct
	if err := unmarshal(&tempConfig); err != nil {
		return err
	}

	cmd.Config.Description = tempConfig.Description
	cmd.Config.PreCmd = tempConfig.PreCmd
	cmd.Config.Args = tempConfig.Args
	cmd.Config.IgnoreStdout = tempConfig.IgnoreStdout
	cmd.Config.IgnoreStderr = tempConfig.IgnoreStderr
	cmd.Config.ContinueWatchOnExitZero = tempConfig.ContinueWatchOnExitZero
	cmd.Config.Preamble = tempConfig.Preamble
	cmd.Config.Logfile = tempConfig.Logfile
	cmd.Config.ExternalSecrets = tempConfig.ExternalSecrets
	cmd.Config.Env = env.ConvertEnvMap(tempConfig.Env, env.BaseCommandEnvPriority)
	return nil
}

func (cmd Command2) GetConfig() SGConfigCommandOptions2 {
	return cmd.Config
}

func (cmd Command2) UpdateConfig(f func(*SGConfigCommandOptions2)) SGConfigCommand {
	f(&cmd.Config)
	return cmd
}

func (cmd Command2) GetName() string {
	return cmd.Config.Name
}

func (cmd Command2) GetBinaryLocation() (string, error) {
	if cmd.CheckBinary != "" {
		return filepath.Join(cmd.Config.RepositoryRoot, cmd.CheckBinary), nil
	}
	return "", noBinaryError{name: cmd.Config.Name}
}

func (cmd Command2) GetBazelTarget() string {
	return ""
}

func (cmd Command2) GetExecCmd(ctx context.Context) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, "bash", "-c", cmd.Cmd), nil
}

func (cmd Command2) RunInstall(ctx context.Context, parentEnv map[string]string) error {
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
func (cmd Command2) SetInstallerOutput(chan<- output.FancyLine) {}

func (cmd Command2) Count() int {
	return 1
}

func (cmd Command2) requiresInstall() bool {
	return cmd.Install != "" || cmd.InstallFunc != ""
}

func (cmd Command2) hasBashInstaller() bool {
	return cmd.Install != "" || cmd.InstallFunc == ""
}

func (cmd Command2) bashInstall(ctx context.Context, parentEnv map[string]string) error {
	output, err := BashInRoot(ctx, cmd.Install, makeEnv(parentEnv, cmd.Config.Env))
	if err != nil {
		return installErr{cmdName: cmd.Config.Name, output: output, originalErr: err}
	}
	return nil
}

func (cmd Command2) functionInstall(ctx context.Context, parentEnv map[string]string) error {
	fn, ok := installFuncs[cmd.InstallFunc]
	if !ok {
		return installErr{cmdName: cmd.Config.Name, originalErr: errors.Newf("no install func with name %q found", cmd.InstallFunc)}
	}
	if err := fn(ctx, makeEnvMap(parentEnv, cmd.Config.Env)); err != nil {
		return installErr{cmdName: cmd.Config.Name, originalErr: err}
	}

	return nil
}

func (cmd Command2) getWatchPaths() []string {
	fullPaths := make([]string, len(cmd.Watch))
	for i, path := range cmd.Watch {
		fullPaths[i] = filepath.Join(cmd.Config.RepositoryRoot, path)
	}

	return fullPaths
}

func (cmd Command2) StartWatch(ctx context.Context) (<-chan struct{}, error) {
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
