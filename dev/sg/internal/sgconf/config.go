package sgconf

import (
	"io"
	"os"
	"strings"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/env"
	"gopkg.in/yaml.v2"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/run"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func parseConfigFile(name string, isOverwriteFile bool) (*Config, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot open file %q", name)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, errors.Wrap(err, "reading configuration file")
	}

	return parseConfig(data, isOverwriteFile)
}

func parseConfig(data []byte, isOverwriteFile bool) (*Config, error) {
	var tmpConf struct {
		Env               map[string]string             `yaml:"env"`
		Commands          map[string]*run.Command       `yaml:"commands"`
		BazelCommands     map[string]*run.BazelCommand  `yaml:"bazelCommands"`
		DockerCommands    map[string]*run.DockerCommand `yaml:"dockerCommands"`
		Commandsets       map[string]*Commandset        `yaml:"commandsets"`
		DefaultCommandset string                        `yaml:"defaultCommandset"`
		Tests             map[string]*run.Command       `yaml:"tests"`
	}
	if err := yaml.Unmarshal(data, &tmpConf); err != nil {
		return nil, err
	}

	var conf Config

	//conf.Commands = tmpConf.Commands
	//conf.BazelCommands = tmpConf.BazelCommands
	//conf.DockerCommands = tmpConf.DockerCommands
	//conf.Commandsets = tmpConf.Commandsets
	//conf.DefaultCommandset = tmpConf.DefaultCommandset
	//conf.Tests = tmpConf.Tests

	root, err := root.RepositoryRoot()
	if err != nil {
		return nil, err
	}

	conf.NewEnv = make(map[string]env.EnvVar, len(tmpConf.Env))
	for k, v := range tmpConf.Env {
		conf.NewEnv[k] = env.New(k, v, env.GlobalEnvPriority)
	}

	conf.BazelCommands = make(map[string]*run.BazelCommand, len(tmpConf.BazelCommands))
	for name, cmd := range tmpConf.BazelCommands {
		cmd.Config.Name = name
		cmd.Config.RepositoryRoot = root

		tmpCommand, ok := tmpConf.BazelCommands[name]
		if !ok {
			return nil, errors.New("cannot find command in temp config")
		}
		cmd.Config.NewEnv = make(map[string]env.EnvVar, len(tmpCommand.Config.Env))
		for k, v := range tmpCommand.Config.Env {
			cmd.Config.NewEnv[k] = env.New(k, v, env.BaseCommandEnvPriority)
		}
	}

	for name, cmd := range conf.DockerCommands {
		cmd.Config.Name = name
		cmd.Config.RepositoryRoot = root

		tmpCommand, ok := tmpConf.DockerCommands[name]
		if !ok {
			return nil, errors.New("cannot find command in temp config")
		}
		cmd.Config.NewEnv = make(map[string]env.EnvVar, len(tmpCommand.Config.Env))
		for k, v := range tmpCommand.Config.Env {
			cmd.Config.NewEnv[k] = env.New(k, v, env.BaseCommandEnvPriority)
		}
	}

	for name, cmd := range conf.Commands {
		cmd.Config.Name = name
		cmd.Config.RepositoryRoot = root
		normalizeCmd(cmd)

		tmpCommand, ok := tmpConf.Commands[name]
		if !ok {
			return nil, errors.New("cannot find command in temp config")
		}
		cmd.Config.NewEnv = make(map[string]env.EnvVar, len(tmpCommand.Config.Env))
		for k, v := range tmpCommand.Config.Env {
			cmd.Config.NewEnv[k] = env.New(k, v, env.BaseCommandEnvPriority)
		}
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
	Deprecated     string            `yaml:"deprecated"`
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

func (c *Commandset) IsDeprecated() bool {
	return c.Deprecated != ""
}

func (c *Commandset) Merge(other *Commandset) *Commandset {
	merged := c

	if other.Name != merged.Name && other.Name != "" {
		merged.Name = other.Name
	}

	if len(other.Commands) != 0 {
		merged.Commands = other.Commands
	}

	if len(other.Checks) != 0 {
		merged.Checks = other.Checks
	}

	if len(other.BazelCommands) != 0 {
		merged.BazelCommands = other.BazelCommands
	}

	if len(other.DockerCommands) != 0 {
		merged.DockerCommands = other.DockerCommands
	}

	for k, v := range other.Env {
		merged.Env[k] = v
	}

	return merged
}

// If you add an entry here, remember to add it to the merge function.
type Config struct {
	Env               map[string]string             `yaml:"env"`
	Commands          map[string]*run.Command       `yaml:"commands"`
	BazelCommands     map[string]*run.BazelCommand  `yaml:"bazelCommands"`
	DockerCommands    map[string]*run.DockerCommand `yaml:"dockerCommands"`
	Commandsets       map[string]*Commandset        `yaml:"commandsets"`
	DefaultCommandset string                        `yaml:"defaultCommandset"`
	Tests             map[string]*run.Command       `yaml:"tests"`

	NewEnv map[string]env.EnvVar
}

//func (c *Config) SetIsOverwriteConfig(b bool) {
//	c.isOverwriteConfig = b
//}
//
//func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
//	println("unmarshalling ...", c.Env)
//	var tempConfig struct {
//		Env               map[string]string             `yaml:"env"`
//		Commands          map[string]*run.Command       `yaml:"commands"`
//		BazelCommands     map[string]*run.BazelCommand  `yaml:"bazelCommands"`
//		DockerCommands    map[string]*run.DockerCommand `yaml:"dockerCommands"`
//		Commandsets       map[string]*Commandset        `yaml:"commandsets"`
//		DefaultCommandset string                        `yaml:"defaultCommandset"`
//		Tests             map[string]*run.Command       `yaml:"tests"`
//	}
//
//	if err := unmarshal(&tempConfig); err != nil {
//		return err
//	}
//
//	c.NewEnv = make(map[string]env.EnvVar, len(tempConfig.Env))
//	for k, v := range tempConfig.Env {
//		c.NewEnv[k] = env.New(k, v, env.GlobalEnvPriority)
//	}
//	c.Commands = tempConfig.Commands
//	c.BazelCommands = tempConfig.BazelCommands
//	c.DockerCommands = tempConfig.DockerCommands
//	c.Commandsets = tempConfig.Commandsets
//	c.DefaultCommandset = tempConfig.DefaultCommandset
//	c.Tests = tempConfig.Tests
//	return nil
//}

// Merge merges the top-level entries of two Config objects, using the
// values from `other` if they are set as overrides and returns a new config
func (c *Config) Merge(other *Config) *Config {
	merged := *c
	for k, v := range other.Env {
		merged.Env[k] = v
	}

	for name, override := range other.Commands {
		if original, ok := merged.Commands[name]; ok {
			merged.Commands[name] = pointers.Ptr(original.Merge(*override))
		} else {
			merged.Commands[name] = override
		}
	}

	for name, override := range other.BazelCommands {
		if original, ok := merged.BazelCommands[name]; ok {
			merged.BazelCommands[name] = pointers.Ptr(original.Merge(*override))
		} else {
			merged.BazelCommands[name] = override
		}
	}

	for name, override := range other.DockerCommands {
		if original, ok := merged.DockerCommands[name]; ok {
			merged.DockerCommands[name] = pointers.Ptr(original.Merge(*override))
		} else {
			merged.DockerCommands[name] = override
		}
	}

	for name, override := range other.Commandsets {
		if original, ok := merged.Commandsets[name]; ok {
			merged.Commandsets[name] = original.Merge(override)
		} else {
			merged.Commandsets[name] = override
		}
	}

	if other.DefaultCommandset != "" {
		merged.DefaultCommandset = other.DefaultCommandset
	}

	for name, override := range other.Tests {
		if original, ok := merged.Tests[name]; ok {
			merged.Tests[name] = pointers.Ptr(original.Merge(*override))
		} else {
			merged.Tests[name] = override
		}
	}

	return &merged
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
