package sgconf

import (
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v2"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/run"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func parseConfigFile(name string) (*Config, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open file %q", name)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.Wrap(err, "reading configuration file")
	}

	return parseConfig(data)
}

func parseConfig(data []byte) (*Config, error) {
	var conf Config
	if err := yaml.Unmarshal(data, &conf); err != nil {
		return nil, err
	}

	root, err := root.RepositoryRoot()
	if err != nil {
		return nil, err
	}

	for name, cmd := range conf.BazelCommands {
		cmd.Config.Name = name
		cmd.Config.RepositoryRoot = root
	}

	for name, cmd := range conf.DockerCommands {
		cmd.Config.Name = name
		cmd.Config.RepositoryRoot = root
	}

	for name, cmd := range conf.Commands {
		cmd.Config.Name = name
		cmd.Config.RepositoryRoot = root
		normalizeCmd(cmd)
	}

	for name, cmd := range conf.Commandsets {
		cmd.Name = name
		conf.Commandsets[name] = cmd
	}

	for name, cmd := range conf.Tests {
		cmd.Config.Name = name
		cmd.Config.RepositoryRoot = root
		normalizeCmd(cmd)
		conf.Tests[name] = cmd
	}

	return &conf, nil
}

func normalizeCmd(cmd *run.Command) {
	// Trim trailing whitespace so extra args apply to last command (instead of being interpreted as
	// a new shell command on a separate line).
	cmd.Cmd = strings.TrimSpace(cmd.Cmd)
}

type Commandset struct {
	Name           string            `yaml:"-"`
	Commands       []string          `yaml:"commands"`
	BazelCommands  []string          `yaml:"bazelCommands"`
	DockerCommands []string          `yaml:"dockerCommands"`
	Checks         []string          `yaml:"checks"`
	Env            map[string]string `yaml:"env"`

	// If this is set to true, then the commandset requires the dev-private
	// repository to be cloned at the same level as the sourcegraph repository.
	RequiresDevPrivate bool `yaml:"requiresDevPrivate"`
}

// UnmarshalYAML implements the Unmarshaler interface.
func (c *Commandset) UnmarshalYAML(unmarshal func(any) error) error {
	// To be backwards compatible we first try to unmarshal as a simple list.
	var list []string
	if err := unmarshal(&list); err == nil {
		c.Commands = list
		return nil
	}

	// If it's not a list we try to unmarshal it as a Commandset. In order to
	// not recurse infinitely (calling UnmarshalYAML over and over) we create a
	// temporary type alias.
	type rawCommandset Commandset
	if err := unmarshal((*rawCommandset)(c)); err != nil {
		return err
	}

	return nil
}

func (c *Commandset) Merge(other *Commandset) *Commandset {
	merged := c

	if other.Name != merged.Name && other.Name != "" {
		merged.Name = other.Name
	}

	if !equal(merged.Commands, other.Commands) && len(other.Commands) != 0 {
		merged.Commands = other.Commands
	}

	if !equal(merged.Checks, other.Checks) && len(other.Checks) != 0 {
		merged.Checks = other.Checks
	}

	if !equal(merged.BazelCommands, other.BazelCommands) && len(other.BazelCommands) != 0 {
		merged.BazelCommands = other.BazelCommands
	}

	if !equal(merged.DockerCommands, other.DockerCommands) && len(other.DockerCommands) != 0 {
		merged.DockerCommands = other.DockerCommands
	}

	for k, v := range other.Env {
		merged.Env[k] = v
	}

	merged.RequiresDevPrivate = other.RequiresDevPrivate

	return merged
}

type Config struct {
	Env               map[string]string             `yaml:"env"`
	Commands          map[string]*run.Command       `yaml:"commands"`
	BazelCommands     map[string]*run.BazelCommand  `yaml:"bazelCommands"`
	DockerCommands    map[string]*run.DockerCommand `yaml:"dockerCommands"`
	Commandsets       map[string]*Commandset        `yaml:"commandsets"`
	DefaultCommandset string                        `yaml:"defaultCommandset"`
	Tests             map[string]*run.Command       `yaml:"tests"`
}

// Merges merges the top-level entries of two Config objects, with the receiver
// being modified.
func (c *Config) Merge(other *Config) {
	for k, v := range other.Env {
		c.Env[k] = v
	}

	for k, v := range other.Commands {
		if original, ok := c.Commands[k]; ok {
			merged := original.Merge(*v)
			c.Commands[k] = &merged
		} else {
			c.Commands[k] = v
		}
	}

	for k, v := range other.Commandsets {
		if original, ok := c.Commandsets[k]; ok {
			c.Commandsets[k] = original.Merge(v)
		} else {
			c.Commandsets[k] = v
		}
	}

	if other.DefaultCommandset != "" {
		c.DefaultCommandset = other.DefaultCommandset
	}

	for k, v := range other.Tests {
		if original, ok := c.Tests[k]; ok {
			merged := original.Merge(*v)
			c.Tests[k] = &merged
		} else {
			c.Tests[k] = v
		}
	}
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

func (c *Config) GetEnv(key string) string {
	// First look into process env, emulating the logic in makeEnv used
	// in internal/run/run.go
	val, ok := os.LookupEnv(key)
	if ok {
		return val
	}
	// Otherwise check in globalConf.Env and *expand* the key, because a value might refer to another env var.
	return os.Expand(c.Env[key], func(lookup string) string {
		if lookup == key {
			return os.Getenv(lookup)
		}

		if e, ok := c.Env[lookup]; ok {
			return e
		}
		return os.Getenv(lookup)
	})
}
