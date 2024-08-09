package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	hashstructure "github.com/mitchellh/hashstructure/v2"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/category"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/run"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/sgconf"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/interrupt"
	"github.com/sourcegraph/sourcegraph/lib/cliutil/completions"
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
				Name:  "tail",
				Usage: "Connects to a running sg tail instance",
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
		BashComplete: func(c *cli.Context) {
			config, _ := getConfig()
			if config == nil {
				return
			}
			completions.CompleteArgs(func() (options []string) {
				if c.Bool("commands") {
					// Suggest commands
					for name := range config.Commands {
						options = append(options, name)
					}
				} else {
					// Suggest commandsets
					for name := range config.Commandsets {
						options = append(options, name)
					}
				}
				return
			})(c)
		},
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
	pid, exists, err := run.PidExistsWithArgs(os.Args[1:])
	if err != nil {
		std.Out.WriteAlertf("Could not check if 'sg %s' is already running with the same arguments. Process: %d", strings.Join(os.Args[1:], " "), pid)
		return errors.Wrap(err, "Failed to check if sg is already running with the same arguments or not.")
	}
	if exists {
		std.Out.WriteAlertf("Found 'sg %s' already running with the same arguments. Process: %d", strings.Join(os.Args[1:], " "), pid)
		return errors.New("no concurrent sg start with same arguments allowed")
	}

	if ctx.Bool("tail") {
		if err := run.OpenUnixSocket(); err != nil {
			return errors.Wrapf(err, "Did you forget to run sg tail first?")
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

	args := StartArgs{
		Describe: ctx.Bool("describe"),
	}

	if ctx.Bool("commands") {
		args.Commands = ctx.Args().Slice()
	} else {
		commandsets := ctx.Args().Slice()
		switch length := len(commandsets); {
		case length > 1:
			std.Out.WriteLine(output.Styled(output.StyleWarning, "ERROR: too many arguments"))
			return flag.ErrHelp
		case length == 1:
			args.CommandSet = commandsets[0]
		}
	}
	return start(ctx.Context, args)
}

func start(ctx context.Context, args StartArgs) error {
	// Start the config watcher
	configs, err := watchConfig(ctx)
	if err != nil {
		return err
	}

	var (
		childCtx context.Context
		cancel   func()
		errs     = make(chan error)
		hash     uint64
	)
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errs:
			if err != nil {
				return err
			}
		case conf := <-configs:
			// Construct the new commands definition and only restart if the changes
			// to the config file are relevant to the commands we're running
			fmt.Println(conf.Env, conf.NewEnv)
			return nil
			cmds, err := args.toCommands(conf)
			for _, cc := range cmds.Commands {
				fmt.Println("command env: ", cc.GetConfig().Env, " \n command name: ", cc.GetConfig().Name)
			}
			fmt.Println(len(cmds.Commands))
			if err != nil {
				return err
			}

			newHash, err := hashstructure.Hash(cmds, hashstructure.FormatV2, nil)
			if err != nil {
				return err
			}
			if hash == newHash {
				continue
			} else {
				hash = newHash
			}

			// Cancel current context if exists, wait for it to close then create a new one
			if cancel != nil {
				cancel()

				// Wait for the context to close and make sure it's a context cancellation error.
				// In the case where all watched commands have already exited with 0 status,
				// there won't be an error so we can just continue
				select {
				case err := <-errs:
					if !errors.Is(err, context.Canceled) {
						return err
					}
				case <-time.After(500 * time.Millisecond):
				}
			}

			// Create a new child context and restart the process
			childCtx, cancel = context.WithCancel(ctx)
			defer cancel()

			//std.Out.ClearScreen()

			go func() {
				if args.Describe {
					errs <- cmds.describe(conf)
				} else {
					errs <- cmds.start(childCtx)
				}
			}()
		}
	}
}

type StartArgs struct {
	Describe   bool
	Commands   []string
	CommandSet string
}

func (args StartArgs) toCommands(conf *sgconf.Config) (*Commands, error) {
	if conf == nil {
		return nil, errors.New("config is nil")
	}

	// If the commands flag is passed, we just extract the command line arguments as
	// a list of commands to run. Else we extract the commandset and parse out its individual commands
	if len(args.Commands) > 0 {
		return listToCommands(conf, args.Commands)
	} else {
		set, err := getCommandSet(conf, args.CommandSet)
		if err != nil {
			std.Out.WriteLine(output.Styledf(output.StyleWarning, "ERROR: extracting commandset failed %q :(", err))
			return nil, flag.ErrHelp
		}
		if set.IsDeprecated() {
			std.Out.WriteLine(output.Styledf(output.StyleBold, set.Deprecated))
			return nil, errors.Newf("commandset %q is deprecated", args.CommandSet)
		}

		return commandSetToCommands(conf, set)
	}
}

func getCommandSet(config *sgconf.Config, name string) (*sgconf.Commandset, error) {
	if name == "" {
		name = config.DefaultCommandset
	}
	if set, ok := config.Commandsets[name]; ok {
		return set, nil
	} else {
		std.Out.WriteLine(output.Styledf(output.StyleWarning, "ERROR: commandset %q not found :(", name))
		return nil, flag.ErrHelp
	}
}

// Public keys are considered part of hash calculation
type Commands struct {
	Name     string
	Checks   []string
	Commands []run.SGConfigCommand
	Env      map[string]string
	ibazel   *run.IBazel
}

func (cmds *Commands) add(cmd ...run.SGConfigCommand) {
	cmds.Commands = append(cmds.Commands, cmd...)
}

func (cmds *Commands) getBazelTargets() (targets []string) {
	for _, cmd := range cmds.Commands {
		target := cmd.GetBazelTarget()
		if target != "" && !slices.Contains(targets, target) {
			targets = append(targets, target)
		}
	}

	return targets
}

func (cmds *Commands) getInstallers() (installers []run.Installer, err error) {
	for _, cmd := range cmds.Commands {
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
	if err := runChecksWithName(ctx, cmds.Checks); err != nil {
		return err
	}

	if len(cmds.Commands) == 0 {
		std.Out.WriteLine(output.Styled(output.StyleWarning, "WARNING: no commands to run"))
		return nil
	}

	installers, err := cmds.getInstallers()
	if err != nil {
		return err
	}

	if err := run.Install(ctx, cmds.Env, verbose, installers); err != nil {
		return err
	}

	if cmds.ibazel != nil {
		cmds.ibazel.StartOutput()
		defer cmds.ibazel.Close()
	}

	return run.Commands(ctx, cmds.Env, verbose, cmds.Commands)
}

func listToCommands(config *sgconf.Config, names []string) (*Commands, error) {
	if len(names) == 0 {
		std.Out.WriteLine(output.Styled(output.StyleWarning, "ERROR: no commands passed"))
		return nil, flag.ErrHelp
	}
	var cmds Commands
	for _, arg := range names {
		if cmd, ok := getCommand(config, arg); ok {
			cmds.add(cmd)
		} else {
			std.Out.WriteLine(output.Styledf(output.StyleWarning, "ERROR: command %q not found :(", arg))
			return nil, flag.ErrHelp
		}
	}
	cmds.Env = config.Env

	return &cmds, nil
}

func commandSetToCommands(config *sgconf.Config, set *sgconf.Commandset) (*Commands, error) {
	cmds := Commands{
		Name: set.Name,
	}
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

	cmds.Env = config.Env
	for k, v := range set.Env {
		cmds.Env[k] = v
	}

	addLogLevel := createLogLevelAdder(logLevelOverrides())
	for i, cmd := range cmds.Commands {
		cmds.Commands[i] = cmd.UpdateConfig(addLogLevel)
	}

	cmds.Checks = set.Checks

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

func (cmds *Commands) describe(config *sgconf.Config) error {
	if cmds.Name == "" {
		for _, cmd := range cmds.Commands {
			out, err := yaml.Marshal(cmd)
			if err != nil {
				return err
			}
			if err = std.Out.WriteMarkdown(fmt.Sprintf("# %s\n\n```yaml\n%s\n```\n\n", cmd.GetConfig().Name, string(out))); err != nil {
				return err
			}
		}

		return nil
	} else {
		set, err := getCommandSet(config, cmds.Name)
		if err != nil {
			return nil
		}
		out, err := yaml.Marshal(set)
		if err != nil {
			return err
		}

		return std.Out.WriteMarkdown(fmt.Sprintf("# %s\n\n```yaml\n%s\n```\n\n", set.Name, string(out)))
	}
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
