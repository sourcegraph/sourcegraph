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

	if other.Name != merged.Name && other.Name != "" {
		merged.Name = other.Name
	}
	if other.Description != merged.Description && other.Description != "" {
		merged.Description = other.Description
	}
	if other.Description != merged.Description && other.Description != "" {
		merged.Description = other.Description
	}
	if other.PreCmd != merged.PreCmd && other.PreCmd != "" {
		merged.PreCmd = other.PreCmd
	}
	if other.Args != merged.Args && other.Args != "" {
		merged.Args = other.Args
	}
	if other.IgnoreStdout != merged.IgnoreStdout && !merged.IgnoreStdout {
		merged.IgnoreStdout = other.IgnoreStdout
	}
	if other.IgnoreStderr != merged.IgnoreStderr && !merged.IgnoreStderr {
		merged.IgnoreStderr = other.IgnoreStderr
	}
	merged.ContinueWatchOnExit = other.ContinueWatchOnExit || merged.ContinueWatchOnExit
	if other.Preamble != merged.Preamble && other.Preamble != "" {
		merged.Preamble = other.Preamble
	}
	if other.Logfile != merged.Logfile && other.Logfile != "" {
		merged.Logfile = other.Logfile
	}
	if other.RepositoryRoot != merged.RepositoryRoot && other.RepositoryRoot != "" {
		merged.RepositoryRoot = other.RepositoryRoot
	}

	for k, v := range other.Env {
		if merged.Env == nil {
			merged.Env = make(map[string]string)
		}
		merged.Env[k] = v
	}

	for k, v := range other.ExternalSecrets {
		if merged.ExternalSecrets == nil {
			merged.ExternalSecrets = make(map[string]secrets.ExternalSecret)
		}
		merged.ExternalSecrets[k] = v
	}

	return merged
}
