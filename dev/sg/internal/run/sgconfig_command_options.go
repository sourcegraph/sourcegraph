package run

import "github.com/sourcegraph/sourcegraph/dev/sg/internal/secrets"

// Common sg command parameters shared by all command types
type SGConfigCommandOptions struct {
	Name        string
	Description string `yaml:"description"`
	// A command to be run before the command is run but after installation
	PreCmd string `yaml:"precmd"`
	// A list of additional arguments to be passed to the command
	Args         string            `yaml:"args"`
	Env          map[string]string `yaml:"env"`
	IgnoreStdout bool              `yaml:"ignoreStdout"`
	IgnoreStderr bool              `yaml:"ignoreStderr"`
	// If true, the runner will continue watching this commands dependencies
	// even if the command exits with a zero status code.
	ContinueWatchOnExit bool `yaml:"continueWatchOnExit"`
	// Preamble is a short and visible message, displayed when the command is launched.
	Preamble string `yaml:"preamble"`

	// Output all logs to a file instead of to stdout/stderr
	Logfile         string                            `yaml:"logfile"`
	ExternalSecrets map[string]secrets.ExternalSecret `yaml:"external_secrets"`

	RepositoryRoot string
}

func (opts SGConfigCommandOptions) Merge(other SGConfigCommandOptions) SGConfigCommandOptions {
	merged := opts

	merged.Name = mergeStrings(merged.Name, other.Name)
	merged.Description = mergeStrings(merged.Description, other.Description)
	merged.PreCmd = mergeStrings(merged.PreCmd, other.PreCmd)
	merged.Args = mergeStrings(merged.Args, other.Args)
	merged.IgnoreStdout = other.IgnoreStdout || merged.IgnoreStdout
	merged.IgnoreStderr = other.IgnoreStderr || merged.IgnoreStderr
	merged.ContinueWatchOnExit = other.ContinueWatchOnExit || merged.ContinueWatchOnExit
	merged.Preamble = mergeStrings(merged.Preamble, other.Preamble)
	merged.Logfile = mergeStrings(merged.Logfile, other.Logfile)
	merged.RepositoryRoot = mergeStrings(merged.RepositoryRoot, other.RepositoryRoot)
	merged.Env = mergeMaps(merged.Env, other.Env)
	merged.ExternalSecrets = mergeMaps(merged.ExternalSecrets, other.ExternalSecrets)

	return merged
}
