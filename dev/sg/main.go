package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/run"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/secrets"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/stdout"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

const (
	defaultConfigFile          = "sg.config.yaml"
	defaultConfigOverwriteFile = "sg.config.overwrite.yaml"
	defaultSecretsFile         = "sg.secrets.json"
)

var secretsStore *secrets.Store

var (
	BuildCommit = "dev"

	// globalConf is the global config. If a command needs to access it, it *must* call
	// `parseConf` before.
	globalConf *Config

	// Note that these are only available after the main sg CLI app has been run.
	configFlag          string
	overwriteConfigFlag string
	verboseFlag         bool
	skipAutoUpdatesFlag bool
)

var postInitHooks []cli.ActionFunc

var sg = &cli.App{
	Usage:       "The Sourcegraph developer tool!",
	Description: "Learn more: https://docs.sourcegraph.com/dev/background-information/sg",
	Version:     BuildCommit,
	Compiled:    time.Now(),
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:        "verbose",
			Usage:       "toggle verbose mode",
			Value:       false,
			Destination: &verboseFlag,
		},
		&cli.StringFlag{
			Name:        "config",
			Usage:       "specify a sg configuration file",
			TakesFile:   true, // Enable completions
			Value:       defaultConfigFile,
			Destination: &configFlag,
			DefaultText: "path",
			EnvVars:     []string{"SG_CONFIG"},
		},
		&cli.StringFlag{
			Name:        "overwrite",
			Usage:       "configuration overwrites file that is gitignored and can be used to, for example, add credentials",
			TakesFile:   true, // Enable completions
			Value:       defaultConfigOverwriteFile,
			Destination: &overwriteConfigFlag,
			DefaultText: "path",
			EnvVars:     []string{"SG_OVERWRITE"},
		},
		&cli.BoolFlag{
			Name:        "skip-auto-update",
			Usage:       "prevent sg from automatically updating itself",
			Value:       BuildCommit == "dev", // Default to skip in dev, otherwise don't
			Destination: &skipAutoUpdatesFlag,
			EnvVars:     []string{"SG_SKIP_AUTO_UPDATE"},
		},
	},
	Before: func(ctx *cli.Context) error {
		if verboseFlag {
			stdout.Out.SetVerbose()
		}

		// We always try to set this, since we
		// often want to watch files, start commands, etc...
		if err := setMaxOpenFiles(); err != nil {
			writeWarningLinef("Failed to set max open files: %s", err)
		}

		if ctx.Args().First() != "update" {
			// If we're not running "sg update ...", we want to check the version first
			err := checkSgVersion(ctx.Context)
			if err != nil {
				writeWarningLinef("Checking sg version and updating failed: %s", err)
				// Do not exit here, so we don't break user flow when they want to
				// run `sg` but updating fails
			}
		}

		for _, hook := range postInitHooks {
			hook(ctx)
		}

		return nil
	},
	Commands: []*cli.Command{
		// Common dev tasks
		runCommand,
		startCommand,
		testCommand,
		// lintCommand,
		// dbCommand,
		// migrationCommand,
		// ciCommand,
		// generateCommand,

		// Dev environment
		doctorCommand,
		secretCommand,
		setupCommand,

		// Company
		teammateCommand,
		// rfcCommand,
		// liveCommand,

		// sg commands
		versionCommand,
		updateCommand,
		// installCommand,

		// Misc.
		opsCommand,
		// auditCommand,
		// funkyLogoCommand,
	},

	HideVersion:            true,
	EnableBashCompletion:   true,
	UseShortOptionHandling: true,
}

func main() {
	if err := loadSecrets(); err != nil {
		fmt.Printf("failed to open secrets: %s\n", err)
	}
	ctx := secrets.WithContext(context.Background(), secretsStore)

	if err := sg.RunContext(ctx, os.Args); err != nil {
		fmt.Printf("error: %s\n", err)
		os.Exit(1)
	}
}

// setMaxOpenFiles will bump the maximum opened files count.
// It's harmless since the limit only persists for the lifetime of the process and it's quick too.
func setMaxOpenFiles() error {
	const maxOpenFiles = 10000

	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		return errors.Wrap(err, "getrlimit failed")
	}

	if rLimit.Cur < maxOpenFiles {
		rLimit.Cur = maxOpenFiles

		// This may not succeed, see https://github.com/golang/go/issues/30401
		err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
		if err != nil {
			return errors.Wrap(err, "setrlimit failed")
		}
	}

	return nil
}

const sgOneLineCmd = `curl --proto '=https' --tlsv1.2 -sSLf https://install.sg.dev | sh`

func checkSgVersion(ctx context.Context) error {
	_, err := root.RepositoryRoot()
	if err != nil {
		// Ignore the error, because we only want to check the version if we're
		// in sourcegraph/sourcegraph
		return nil
	}

	if BuildCommit == "dev" {
		// If `sg` was built with a dirty `./dev/sg` directory it's a dev build
		// and we don't need to display this message.
		return nil
	}

	rev := strings.TrimPrefix(BuildCommit, "dev-")
	out, err := run.GitCmd("rev-list", fmt.Sprintf("%s..origin/main", rev), "./dev/sg")
	if err != nil {
		fmt.Printf("error getting new commits since %s in ./dev/sg: %s\n", rev, err)
		fmt.Printf("try reinstalling sg with `%s`.\n", sgOneLineCmd)
		os.Exit(1)
	}

	out = strings.TrimSpace(out)
	if out == "" {
		// No newer commits found. sg is up to date.
		return nil
	}

	if skipAutoUpdatesFlag {
		stdout.Out.WriteLine(output.Linef("", output.StyleSearchMatch, "╭──────────────────────────────────────────────────────────────────╮  "))
		stdout.Out.WriteLine(output.Linef("", output.StyleSearchMatch, "│                                                                  │░░"))
		stdout.Out.WriteLine(output.Linef("", output.StyleSearchMatch, "│ HEY! New version of sg available. Run 'sg update' to install it. │░░"))
		stdout.Out.WriteLine(output.Linef("", output.StyleSearchMatch, "│       To see what's new, run 'sg version changelog -next'.       │░░"))
		stdout.Out.WriteLine(output.Linef("", output.StyleSearchMatch, "│                                                                  │░░"))
		stdout.Out.WriteLine(output.Linef("", output.StyleSearchMatch, "╰──────────────────────────────────────────────────────────────────╯░░"))
		stdout.Out.WriteLine(output.Linef("", output.StyleSearchMatch, "  ░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░"))
		return nil
	}

	stdout.Out.WriteLine(output.Line(output.EmojiInfo, output.StyleSuggestion, "Auto updating sg ..."))
	newPath, err := updateToPrebuiltSG(ctx)
	if err != nil {
		return err
	}
	return syscall.Exec(newPath, os.Args, os.Environ())
}

func loadSecrets() error {
	homePath, err := root.GetSGHomePath()
	if err != nil {
		return err
	}
	fp := filepath.Join(homePath, defaultSecretsFile)
	secretsStore, err = secrets.LoadFile(fp)
	return err
}

// parseConf parses the config file and the optional overwrite file.
// Iear the conf has already been parsed it's a noop.
func parseConf(confFile, overwriteFile string) (bool, output.FancyLine) {
	if globalConf != nil {
		return true, output.FancyLine{}
	}

	// Try to determine root of repository, so we can look for config there
	repoRoot, err := root.RepositoryRoot()
	if err != nil {
		return false, output.Linef("", output.StyleWarning, "Failed to determine repository root location: %s", err)
	}

	// If the configFlag/overwriteConfigFlag flags have their default value, we
	// take the value as relative to the root of the repository.
	if confFile == defaultConfigFile {
		confFile = filepath.Join(repoRoot, confFile)
	}

	if overwriteFile == defaultConfigOverwriteFile {
		overwriteFile = filepath.Join(repoRoot, overwriteFile)
	}

	globalConf, err = ParseConfigFile(confFile)
	if err != nil {
		return false, output.Linef("", output.StyleWarning, "Failed to parse %s%s%s%s as configuration file:%s\n%s", output.StyleBold, confFile, output.StyleReset, output.StyleWarning, output.StyleReset, err)
	}

	if ok, _ := fileExists(overwriteFile); ok {
		overwriteConf, err := ParseConfigFile(overwriteFile)
		if err != nil {
			return false, output.Linef("", output.StyleWarning, "Failed to parse %s%s%s%s as overwrites configuration file:%s\n%s", output.StyleBold, overwriteFile, output.StyleReset, output.StyleWarning, output.StyleReset, err)
		}
		globalConf.Merge(overwriteConf)
	}

	return true, output.FancyLine{}
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
