// Command "src-expose" serves directories as git repositories over HTTP.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"github.com/peterbourgon/ff/ffcli"
	"gopkg.in/yaml.v2"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var errSilent = errors.New("silent error")

type usageError struct {
	Msg string
}

func (e *usageError) Error() string {
	return e.Msg
}

func explainAddr(addr string) string {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		port = "3434"
	}

	return fmt.Sprintf(`Serving the repositories at http://%s.

FIRST RUN NOTE: If src-expose has not yet been setup on Sourcegraph, then you
need to configure Sourcegraph to sync with src-expose. Paste the following
configuration as an Other External Service in Sourcegraph:

  {
    // url is the http url to src-expose (listening on %s)
    // url should be reachable by Sourcegraph.
    // "http://host.docker.internal:%s" works from Sourcegraph when using Docker for Desktop.
    "url": "http://host.docker.internal:%s",
    "repos": ["src-expose"] // This may change in versions later than 3.9
  }
`, addr, addr, port, port)
}

func explainSnapshotter(s *Snapshotter) string {
	var dirs []string
	for _, d := range s.Dirs {
		dirs = append(dirs, "- "+d.Dir)
	}

	return fmt.Sprintf("Periodically syncing directories as git repositories to %s.\n%s\n", s.Destination, strings.Join(dirs, "\n"))
}

func usageErrorOutput(cmd *ffcli.Command, cmdPath string, err error) string {
	var w strings.Builder
	_, _ = fmt.Fprintf(&w, "%q %s\nSee '%s --help'.\n", cmdPath, err.Error(), cmdPath)
	if cmd.Usage != "" {
		_, _ = fmt.Fprintf(&w, "\nUsage:  %s\n", cmd.Usage)
	}
	if cmd.ShortHelp != "" {
		_, _ = fmt.Fprintf(&w, "\n%s\n", cmd.ShortHelp)
	}
	return w.String()
}

func shortenErrHelp(cmd *ffcli.Command, cmdPath string) {
	// We want to keep the long help, but in the case of exec requesting help we show shorter help output
	if cmd.Exec == nil {
		return
	}

	cmdPath = strings.TrimSpace(cmdPath + " " + cmd.Name)

	exec := cmd.Exec
	cmd.Exec = func(args []string) error {
		err := exec(args)
		if errors.HasType(err, &usageError{}) {
			var w io.Writer
			if cmd.FlagSet != nil {
				w = cmd.FlagSet.Output()
			} else {
				w = os.Stderr
			}
			_, _ = fmt.Fprint(w, usageErrorOutput(cmd, cmdPath, err))
			return errSilent
		}
		return err
	}

	for _, child := range cmd.Subcommands {
		shortenErrHelp(child, cmdPath)
	}
}

func main() {
	log.SetPrefix("")

	var (
		globalFlags    = flag.NewFlagSet("src-expose", flag.ExitOnError)
		globalQuiet    = globalFlags.Bool("quiet", false, "")
		globalVerbose  = globalFlags.Bool("verbose", false, "")
		globalBefore   = globalFlags.String("before", "", "A command to run before sync. It is run from the current working directory.")
		globalReposDir = globalFlags.String("repos-dir", "", "src-expose's git directories. src-expose creates a git repo per directory synced. The git repo is then served to Sourcegraph. The repositories are stored and served relative to this directory. Default: ~/.sourcegraph/src-expose-repos")
		globalConfig   = globalFlags.String("config", "", "If set will be used instead of command line arguments to specify configuration.")
		globalAddr     = globalFlags.String("addr", ":3434", "address on which to serve (end with : for unused port)")
	)

	newLogger := func(prefix string) *log.Logger {
		if *globalQuiet {
			return log.New(io.Discard, "", log.LstdFlags)
		}
		return log.New(os.Stderr, prefix, log.LstdFlags)
	}

	newVerbose := func(prefix string) *log.Logger {
		if !*globalVerbose {
			return log.New(io.Discard, "", log.LstdFlags)
		}
		return log.New(os.Stderr, prefix, log.LstdFlags)
	}

	globalSnapshotter := func() (*Snapshotter, error) {
		var s Snapshotter
		if *globalConfig != "" {
			b, err := os.ReadFile(*globalConfig)
			if err != nil {
				return nil, errors.Errorf("could read configuration at %s: %w", *globalConfig, err)
			}
			if err := yaml.Unmarshal(b, &s); err != nil {
				return nil, errors.Errorf("could not parse configuration at %s: %w", *globalConfig, err)
			}
		}

		if s.Destination == "" {
			s.Destination = *globalReposDir
		}
		if *globalBefore != "" {
			s.Before = *globalBefore
		}

		return &s, nil
	}

	parseSnapshotter := func(args []string) (*Snapshotter, error) {
		s, err := globalSnapshotter()
		if err != nil {
			return nil, err
		}

		if *globalConfig != "" {
			if len(args) != 0 {
				return nil, &usageError{"does not take arguments if --config is specified"}
			}
		} else {
			if len(args) == 0 {
				return nil, &usageError{"requires at least 1 argument or --config to be specified."}
			}
			for _, dir := range args {
				// when specifying source directories naively on the command line (no config file),
				// use the base name as the relative destination directory to avoid collisions
				s.Dirs = append(s.Dirs, &SyncDir{Dir: dir, Destination: path.Base(dir)})
			}
		}

		if err := s.SetDefaults(); err != nil {
			return nil, err
		}

		return s, nil
	}

	serve := &ffcli.Command{
		Name:      "serve",
		Usage:     "src-expose [flags] serve [flags] [path/to/dir/containing/git/dirs]",
		ShortHelp: "Serve git repos for Sourcegraph to list and clone.",
		LongHelp: `src-expose serve will serve the git repositories over HTTP. These can be git
cloned, and they can be discovered by Sourcegraph.

See "src-expose -h" for the flags that can be passed.

src-expose will default to serving ~/.sourcegraph/src-expose-repos`,
		Exec: func(args []string) error {
			var repoDir string
			switch len(args) {
			case 0:
				s, err := globalSnapshotter()
				if err != nil {
					return err
				}
				if err := s.SetDefaults(); err != nil {
					return err
				}
				repoDir = s.Destination

			case 1:
				repoDir = args[0]

			default:
				return &usageError{"requires zero or one arguments"}
			}

			s := &Serve{
				Addr:  *globalAddr,
				Root:  repoDir,
				Info:  newLogger("serve: "),
				Debug: newVerbose("DBUG serve: "),
			}
			return s.Start()
		},
	}

	sync := &ffcli.Command{
		Name:      "sync",
		Usage:     "src-expose [flags] sync [flags] <src1> [<src2> ...]",
		ShortHelp: "Do a one-shot sync of directories",
		Exec: func(args []string) error {
			s, err := parseSnapshotter(args)
			if err != nil {
				return err
			}
			return s.Run(newLogger("sync: "))
		},
	}

	root := &ffcli.Command{
		Name:      "src-expose",
		Usage:     "src-expose [flags] <src1> [<src2> ...]",
		ShortHelp: "Periodically sync directories src1, src2, ... and serve them.",
		LongHelp: `Periodically sync directories src1, src2, ... and serve them.

See "src-expose -h" for the flags that can be passed.

For more advanced uses specify --config pointing to a yaml file.
See https://github.com/sourcegraph/sourcegraph/tree/main/dev/src-expose/examples`,
		Subcommands: []*ffcli.Command{serve, sync},
		FlagSet:     globalFlags,
		Exec: func(args []string) error {
			s, err := parseSnapshotter(args)
			if err != nil {
				return err
			}

			if *globalVerbose {
				b, _ := yaml.Marshal(s)
				_, _ = os.Stdout.Write(b)
				fmt.Println()
			}

			if !*globalQuiet {
				fmt.Println(explainSnapshotter(s))
				fmt.Println(explainAddr(*globalAddr))
			}

			go func() {
				s := &Serve{
					Addr:  *globalAddr,
					Root:  s.Destination,
					Info:  newLogger("serve: "),
					Debug: newVerbose("DBUG serve: "),
				}
				if err := s.Start(); err != nil {
					log.Fatal(err)
				}
			}()

			logger := newLogger("sync: ")
			for {
				if err := s.Run(logger); err != nil {
					return err
				}
				time.Sleep(s.Duration)
			}
		},
	}

	shortenErrHelp(root, "")

	if err := root.Run(os.Args[1:]); err != nil {
		if !errors.IsAny(err, flag.ErrHelp, errSilent) {
			_, _ = fmt.Fprintf(root.FlagSet.Output(), "\nerror: %v\n", err)
		}
		os.Exit(1)
	}
}
