package sgconf

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	DefaultFile          = "sg.config.yaml"
	DefaultOverwriteFile = "sg.config.overwrite.yaml"
)

var (
	globalConfOnce sync.Once
	globalConf     *Config
	globalConfErr  error
)

// Get retrieves the global config files and merges them into a single sg config.
//
// It must not be called before flag initalization, i.e. when confFile or overwriteFile is
// not set, or it will panic. This means that it can only be used in (*cli).Action,
// (*cli).Before/(*cli).After, and postInitHooks
func Get(confFile, overwriteFile string) (*Config, error) {
	// If unset, Get was called in an illegal context, since sg.Before validates that the
	// flags are non-empty.
	if confFile == "" || overwriteFile == "" {
		panic("sgconf.Get called before flag initialization")
	}

	globalConfOnce.Do(func() {
		globalConf, globalConfErr = parseConf(confFile, overwriteFile, false)
	})
	return globalConf, globalConfErr
}

// GetWithoutOverwrites retrieves the global config file and doesn't merge it
// with another file..
//
// It must not be called before flag initalization, i.e. when confFile is not
// set, or it will panic. This means that it can only be used in (*cli).Action,
// (*cli).Before/(*cli).After, and postInitHooks
func GetWithoutOverwrites(confFile string) (*Config, error) {
	// If unset, Get was called in an illegal context, since sg.Before validates that the
	// flags are non-empty.
	if confFile == "" {
		panic("sgconf.Get called before flag initialization")
	}

	globalConfOnce.Do(func() {
		globalConf, globalConfErr = parseConf(confFile, "", true)
	})
	return globalConf, globalConfErr
}

func parseConf(confFile, overwriteFile string, noOverwrite bool) (*Config, error) {
	// Try to determine root of repository, so we can look for config there
	repoRoot, err := root.RepositoryRoot()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to determine repository root location")
	}

	// If the configFlag/overwriteConfigFlag flags have their default value, we
	// take the value as relative to the root of the repository.
	if confFile == DefaultFile {
		confFile = filepath.Join(repoRoot, confFile)
	}

	conf, err := parseConfigFile(confFile)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to parse %q as configuration file", confFile)
	}

	if !noOverwrite {
		if overwriteFile == DefaultOverwriteFile {
			overwriteFile = filepath.Join(repoRoot, overwriteFile)
		}
		if ok, _ := fileExists(overwriteFile); ok {
			overwriteConf, err := parseConfigFile(overwriteFile)
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to parse %q as configuration overwrite file", confFile)
			}
			conf = conf.Merge(overwriteConf)
		}
	}

	return conf, nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
