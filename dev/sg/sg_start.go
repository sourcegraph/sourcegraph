package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	sgrun "github.com/sourcegraph/run"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/category"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/run"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/sgconf"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/interrupt"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/lib/cliutil/completions"
	"github.com/sourcegraph/sourcegraph/lib/cliutil/exit"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

func init() {
	postInitHooks = append(postInitHooks,
		func(cmd *cli.Context) {
			// Create 'sg start' help text after flag (and config) initialization
			startCommand.Description = constructStartCmdLongHelp()
		},
		func(cmd *cli.Context) {
			ctx, cancel := context.WithCancel(cmd.Context)
			interrupt.Register(func() {
				cancel()
				// TODO wait for stuff properly.
				time.Sleep(1 * time.Second)
			})
			cmd.Context = ctx
		},
	)
}

const devPrivateDefaultBranch = "master"

var (
	debugStartServices cli.StringSlice
	infoStartServices  cli.StringSlice
	warnStartServices  cli.StringSlice
	errorStartServices cli.StringSlice
	critStartServices  cli.StringSlice
	exceptServices     cli.StringSlice
	onlyServices       cli.StringSlice

	startCommand = &cli.Command{
		Name:      "start",
		ArgsUsage: "[commandset]",
		Usage:     "🌟 Starts the given commandset. Without a commandset it starts the default Sourcegraph dev environment",
		UsageText: `
# Run default environment, Sourcegraph enterprise:
sg start

# List available environments (defined under 'commandSets' in 'sg.config.yaml'):
sg start -help

# Run the enterprise environment with code-intel enabled:
sg start enterprise-codeintel

# Run the environment for Batch Changes development:
sg start batches

# Override the logger levels for specific services
sg start --debug=gitserver --error=enterprise-worker,enterprise-frontend enterprise

# View configuration for a commandset
sg start -describe single-program

# Run a set of commands instead of a commandset
sg start --commands frontend gitserver
`,
		Category: category.Dev,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "describe",
				Usage: "Print details about the selected commandset",
			},
			&cli.BoolFlag{
				Name:  "sgtail",
				Usage: "Connects to running sgtail instance",
			},
			&cli.BoolFlag{
				Name:  "profile",
				Usage: "Starts up pprof on port 6060",
			},
			&cli.BoolFlag{
				Name:    "commands",
				Aliases: []string{"cmd", "cmds"},
				Usage:   "Signifies that you will be passing in individual commands to run, instead of a set of commands",
			},
			&cli.StringSliceFlag{
				Name:        "debug",
				Aliases:     []string{"d"},
				Usage:       "Services to set at debug log level.",
				Destination: &debugStartServices,
			},
			&cli.StringSliceFlag{
				Name:        "info",
				Aliases:     []string{"i"},
				Usage:       "Services to set at info log level.",
				Destination: &infoStartServices,
			},
			&cli.StringSliceFlag{
				Name:        "warn",
				Aliases:     []string{"w"},
				Usage:       "Services to set at warn log level.",
				Destination: &warnStartServices,
			},
			&cli.StringSliceFlag{
				Name:        "error",
				Aliases:     []string{"e"},
				Usage:       "Services to set at info error level.",
				Destination: &errorStartServices,
			},
			&cli.StringSliceFlag{
				Name:        "crit",
				Aliases:     []string{"c"},
				Usage:       "Services to set at info crit level.",
				Destination: &critStartServices,
			},
			&cli.StringSliceFlag{
				Name:        "except",
				Usage:       "List of services of the specified command set to NOT start",
				Destination: &exceptServices,
			},
			&cli.StringSliceFlag{
				Name:        "only",
				Usage:       "List of services of the specified command set to start. Commands NOT in this list will NOT be started.",
				Destination: &onlyServices,
			},
		},
		BashComplete: completions.CompleteArgs(func() (options []string) {
			config, _ := getConfig()
			if config == nil {
				return
			}
			for name := range config.Commandsets {
				options = append(options, name)
			}
			return
		}),
		Action: startExec,
	}
)

func constructStartCmdLongHelp() string {
	var out strings.Builder

	fmt.Fprintf(&out, `Use this to start your Sourcegraph environment!`)

	config, err := getConfig()
	if err != nil {
		out.Write([]byte("\n"))
		std.NewOutput(&out, false).WriteWarningf(err.Error())
		return out.String()
	}

	fmt.Fprintf(&out, "\n\n")
	fmt.Fprintf(&out, "Available commandsets in `%s`:\n", configFile)

	var names []string
	for name := range config.Commandsets {
		switch name {
		case "enterprise-codeintel":
			names = append(names, fmt.Sprintf("%s 🧠", name))
		case "batches":
			names = append(names, fmt.Sprintf("%s 🦡", name))
		default:
			names = append(names, name)
		}
	}
	sort.Strings(names)
	fmt.Fprint(&out, "\n* "+strings.Join(names, "\n* "))

	return out.String()
}

func startExec(ctx *cli.Context) error {
	config, err := getConfig()
	if err != nil {
		return err
	}

	pid, exists, err := run.PidExistsWithArgs(os.Args[1:])
	if err != nil {
		std.Out.WriteAlertf("Could not check if 'sg %s' is already running with the same arguments. Process: %d", strings.Join(os.Args[1:], " "), pid)
		return errors.Wrap(err, "Failed to check if sg is already running with the same arguments or not.")
	}
	if exists {
		std.Out.WriteAlertf("Found 'sg %s' already running with the same arguments. Process: %d", strings.Join(os.Args[1:], " "), pid)
		return errors.New("no concurrent sg start with same arguments allowed")
	}

	if ctx.Bool("sgtail") {
		if err := run.OpenUnixSocket(); err != nil {
			return errors.Wrapf(err, "Did you forget to run sgtail first?")
		}
	}

	// If the commands flag is passed, we just extract the command line arguments as
	// a list of commands to run.
	if ctx.Bool("commands") {
		cmds, err := listToCommands(config, ctx.Args().Slice())
		if err != nil {
			return err
		}
		return cmds.start(ctx.Context)
	}

	set, err := getCommandSet(config, ctx.Args().Slice())
	if err != nil {
		std.Out.WriteLine(output.Styledf(output.StyleWarning, "ERROR: extracting commandset failed %q :(", err))
		return flag.ErrHelp
	}

	if ctx.Bool("describe") {
		out, err := yaml.Marshal(set)
		if err != nil {
			return err
		}

		return std.Out.WriteMarkdown(fmt.Sprintf("# %s\n\n```yaml\n%s\n```\n\n", set.Name, string(out)))
	}

	// If the commandset requires the dev-private repository to be cloned, we
	// check that it's at the right location here.
	if set.RequiresDevPrivate && !NoDevPrivateCheck {
		repoRoot, err := root.RepositoryRoot()
		if err != nil {
			std.Out.WriteLine(output.Styledf(output.StyleWarning, "Failed to determine repository root location: %s", err))
			return exit.NewEmptyExitErr(1)
		}

		devPrivatePath := filepath.Join(repoRoot, "..", "dev-private")
		exists, err := pathExists(devPrivatePath)
		if err != nil {
			std.Out.WriteLine(output.Styledf(output.StyleWarning, "Failed to check whether dev-private repository exists: %s", err))
			return exit.NewEmptyExitErr(1)
		}
		if !exists {
			std.Out.WriteLine(output.Styled(output.StyleWarning, "ERROR: dev-private repository not found!"))
			std.Out.WriteLine(output.Styledf(output.StyleWarning, "It's expected to exist at: %s", devPrivatePath))
			std.Out.WriteLine(output.Styled(output.StyleWarning, "See the documentation for how to get set up: https://sourcegraph.com/docs/dev/setup/quickstart#run-sg-setup"))

			std.Out.Write("")
			overwritePath := filepath.Join(repoRoot, "sg.config.overwrite.yaml")
			std.Out.WriteLine(output.Styledf(output.StylePending, "If you know what you're doing and want disable the check, add the following to %s:", overwritePath))
			std.Out.Write("")
			std.Out.Write(fmt.Sprintf(`  commandsets:
    %s:
      requiresDevPrivate: false
`, set.Name))
			std.Out.Write("")

			return exit.NewEmptyExitErr(1)
		}

		// dev-private exists, let's see if there are any changes
		update := std.Out.Pending(output.Styled(output.StylePending, "Checking for dev-private changes..."))
		shouldUpdate, err := shouldUpdateDevPrivate(ctx.Context, devPrivatePath, devPrivateDefaultBranch)
		if shouldUpdate {
			update.WriteLine(output.Line(output.EmojiInfo, output.StyleSuggestion, "We found some changes in dev-private that you're missing out on! If you want the new changes, 'cd ../dev-private' and then do a 'git stash' and a 'git pull'!"))
		}
		if err != nil {
			update.Close()
			std.Out.WriteWarningf("WARNING: Encountered some trouble while checking if there are remote changes in dev-private!")
			std.Out.Write("")
			std.Out.Write(err.Error())
			std.Out.Write("")
		} else {
			update.Complete(output.Line(output.EmojiSuccess, output.StyleSuccess, "Done checking dev-private changes"))
		}
	}
	if ctx.Bool("profile") {
		// start a pprof server
		go func() {
			err := http.ListenAndServe("127.0.0.1:6060", nil)
			if err != nil {
				std.Out.WriteAlertf("Failed to start pprof server: %s", err)
			}
		}()
		std.Out.WriteAlertf(`pprof profiling started at 6060. Try some of the following to profile:
# Start a web UI on port 6061 to view the current heap profile
go tool pprof -http 127.0.0.1:6061 http://127.0.0.1:6060/debug/pprof/heap

# Start a web UI on port 6061 to view a CPU profile of the next 30 seconds
go tool pprof -http 127.0.0.1:6061 http://127.0.0.1:6060/debug/pprof/profile?seconds=30

Find more here: https://pkg.go.dev/net/http/pprof
or run

go tool pprof -help
`)
	}

	return startCommandSet(ctx.Context, set, config)
}

func startCommandSet(ctx context.Context, set *sgconf.Commandset, conf *sgconf.Config) error {
	commands, err := commandSetToCommands(conf, set)
	if err != nil {
		return err
	}

	return commands.start(ctx)
}

func shouldUpdateDevPrivate(ctx context.Context, path, branch string) (bool, error) {
	// git fetch so that we check whether there are any remote changes
	if err := sgrun.Bash(ctx, fmt.Sprintf("git fetch origin %s", branch)).Dir(path).Run().Wait(); err != nil {
		return false, err
	}
	// Now we check if there are any changes. If the output is empty, we're not missing out on anything.
	outputStr, err := sgrun.Bash(ctx, fmt.Sprintf("git diff --shortstat origin/%s", branch)).Dir(path).Run().String()
	if err != nil {
		return false, err
	}
	return len(outputStr) > 0, err

}

func getCommandSet(config *sgconf.Config, args []string) (*sgconf.Commandset, error) {
	switch length := len(args); {
	case length > 1:
		std.Out.WriteLine(output.Styled(output.StyleWarning, "ERROR: too many arguments"))
		return nil, flag.ErrHelp
	case length == 1:
		if set, ok := config.Commandsets[args[0]]; ok {
			return set, nil
		} else {
			std.Out.WriteLine(output.Styledf(output.StyleWarning, "ERROR: commandset %q not found :(", args[0]))
			return nil, flag.ErrHelp
		}

	default:
		if set, ok := config.Commandsets[config.DefaultCommandset]; ok {
			return set, nil
		} else {
			std.Out.WriteLine(output.Styled(output.StyleWarning, "ERROR: No commandset specified and no 'defaultCommandset' specified in sg.config.yaml\n"))
			return nil, flag.ErrHelp
		}
	}
}

type Commands struct {
	checks   []string
	commands []run.SGConfigCommand
	env      map[string]string
	ibazel   *run.IBazel
}

func (cmds *Commands) add(cmd ...run.SGConfigCommand) {
	cmds.commands = append(cmds.commands, cmd...)
}

func (cmds *Commands) getBazelTargets() (targets []string) {
	for _, cmd := range cmds.commands {
		target := cmd.GetBazelTarget()
		if target != "" && !slices.Contains(targets, target) {
			targets = append(targets, target)
		}
	}

	return targets
}

func (cmds *Commands) getInstallers() (installers []run.Installer, err error) {
	for _, cmd := range cmds.commands {
		if installer, ok := cmd.(run.Installer); ok {
			installers = append(installers, installer)
		}
	}
	targets := cmds.getBazelTargets()
	if len(targets) > 0 {
		if cmds.ibazel, err = run.NewIBazel(targets); err != nil {
			return nil, err
		}
		installers = append(installers, cmds.ibazel)
	}
	return
}

func (cmds *Commands) start(ctx context.Context) error {
	if err := runChecksWithName(ctx, cmds.checks); err != nil {
		return err
	}

	if len(cmds.commands) == 0 {
		std.Out.WriteLine(output.Styled(output.StyleWarning, "WARNING: no commands to run"))
		return nil
	}

	installers, err := cmds.getInstallers()
	if err != nil {
		return err
	}

	if err := run.Install(ctx, cmds.env, verbose, installers); err != nil {
		return err
	}

	if cmds.ibazel != nil {
		cmds.ibazel.StartOutput()
		defer cmds.ibazel.Close()
	}

	return run.Commands(ctx, cmds.env, verbose, cmds.commands)
}

func listToCommands(config *sgconf.Config, names []string) (*Commands, error) {
	var cmds Commands
	for _, arg := range names {
		if cmd, ok := getCommand(config, arg); ok {
			cmds.add(cmd)
		} else {
			std.Out.WriteLine(output.Styledf(output.StyleWarning, "ERROR: command %q not found :(", arg))
			return nil, flag.ErrHelp
		}
	}
	cmds.env = config.Env

	return &cmds, nil
}

func commandSetToCommands(config *sgconf.Config, set *sgconf.Commandset) (*Commands, error) {
	cmds := Commands{}
	if ccmds, err := getCommands(set.Commands, set, config.Commands); err != nil {
		return nil, err
	} else {
		cmds.add(ccmds...)
	}

	if bcmds, err := getCommands(set.BazelCommands, set, config.BazelCommands); err != nil {
		return nil, err
	} else {
		cmds.add(bcmds...)
	}

	if dcmds, err := getCommands(set.DockerCommands, set, config.DockerCommands); err != nil {
		return nil, err
	} else {
		cmds.add(dcmds...)
	}

	cmds.env = config.Env
	for k, v := range set.Env {
		cmds.env[k] = v
	}

	addLogLevel := createLogLevelAdder(logLevelOverrides())
	for i, cmd := range cmds.commands {
		cmds.commands[i] = cmd.UpdateConfig(addLogLevel)
	}

	return &cmds, nil

}

func getCommand(config *sgconf.Config, name string) (run.SGConfigCommand, bool) {
	if cmd, ok := config.BazelCommands[name]; ok {
		return cmd, ok
	}
	if cmd, ok := config.DockerCommands[name]; ok {
		return cmd, ok
	}
	if cmd, ok := config.Commands[name]; ok {
		return cmd, ok
	}
	return nil, false
}

func getCommands[T run.SGConfigCommand](commands []string, set *sgconf.Commandset, conf map[string]T) ([]run.SGConfigCommand, error) {
	exceptList := exceptServices.Value()
	exceptSet := make(map[string]interface{}, len(exceptList))
	for _, svc := range exceptList {
		exceptSet[svc] = struct{}{}
	}

	onlyList := onlyServices.Value()
	onlySet := make(map[string]interface{}, len(onlyList))
	for _, svc := range onlyList {
		onlySet[svc] = struct{}{}
	}

	cmds := make([]run.SGConfigCommand, 0, len(commands))
	for _, name := range commands {
		cmd, ok := conf[name]
		if !ok {
			return nil, errors.Errorf("command %q not found in commandset %q", name, set.Name)
		}

		if _, excluded := exceptSet[name]; excluded {
			std.Out.WriteLine(output.Styledf(output.StylePending, "Skipping command %s since it's in --except.", name))
			continue
		}

		// No --only specified, just add command
		if len(onlySet) == 0 {
			cmds = append(cmds, cmd)
		} else {
			if _, inSet := onlySet[name]; inSet {
				cmds = append(cmds, cmd)
			} else {
				std.Out.WriteLine(output.Styledf(output.StylePending, "Skipping command %s since it's not included in --only.", name))
			}
		}

	}
	return cmds, nil
}

// logLevelOverrides builds a map of commands -> log level that should be overridden in the environment.
func logLevelOverrides() map[string]string {
	levelServices := make(map[string][]string)
	levelServices["debug"] = debugStartServices.Value()
	levelServices["info"] = infoStartServices.Value()
	levelServices["warn"] = warnStartServices.Value()
	levelServices["error"] = errorStartServices.Value()
	levelServices["crit"] = critStartServices.Value()

	overrides := make(map[string]string)
	for level, services := range levelServices {
		for _, service := range services {
			overrides[service] = level
		}
	}

	return overrides
}

// enrichWithLogLevels will add any logger level overrides to a given command if they have been specified.
func createLogLevelAdder(overrides map[string]string) func(*run.SGConfigCommandOptions) {
	return func(config *run.SGConfigCommandOptions) {
		logLevelVariable := "SRC_LOG_LEVEL"

		if level, ok := overrides[config.Name]; ok {
			std.Out.WriteLine(output.Styledf(output.StylePending, "Setting log level: %s for command %s.", level, config.Name))
			if config.Env == nil {
				config.Env = make(map[string]string, 1)
			}

			config.Env[logLevelVariable] = level
		}
	}
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
