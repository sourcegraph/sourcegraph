package run

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/analytics"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/interrupt"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/internal/download"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

type Installer interface {
	RunInstall(ctx context.Context, env map[string]string) error

	// Gives a channel which the installer can use to send log messages
	SetInstallerOutput(chan<- output.FancyLine)

	GetName() string

	// Number of programs this target is installing
	Count() int
}

type InstallManager struct {
	// Constructor commands
	*std.Output
	cmds    map[string]Installer
	env     map[string]string
	verbose bool

	// State vars
	installed           chan string
	failures            chan failedInstall
	logs                chan output.FancyLine
	done                int
	total               int
	waitingMessageIndex int
	progress            output.Progress
	ticker              *time.Ticker
	tickInterval        time.Duration
	stats               *installAnalytics
}

func Install(ctx context.Context, env map[string]string, verbose bool, cmds []Installer) error {
	if len(cmds) == 0 {
		return nil
	}
	installer := newInstallManager(cmds, std.Out, env, verbose)

	installer.start(ctx)

	installer.install(ctx, cmds)

	return installer.wait(ctx)
}

func newInstallManager(cmds []Installer, out *std.Output, env map[string]string, verbose bool) *InstallManager {
	total := 0
	cmdsMap := make(map[string]Installer, len(cmds))
	for _, cmd := range cmds {
		total += cmd.Count()
		cmdsMap[cmd.GetName()] = cmd
	}
	return &InstallManager{
		Output:  out,
		cmds:    cmdsMap,
		verbose: verbose,
		env:     env,

		installed: make(chan string, total),
		failures:  make(chan failedInstall, total),
		logs:      make(chan output.FancyLine, 10),
		done:      0,
		total:     total,
	}
}

// starts all progress bars and counters but does not start installation
func (installer *InstallManager) start(ctx context.Context) {
	installer.Write("")
	installer.WriteLine(output.Linef(output.EmojiLightbulb, output.StyleBold, "Installing %d commands...", installer.total))
	installer.Write("")

	installer.progress = installer.Progress([]output.ProgressBar{
		{Label: fmt.Sprintf("Installing %d commands", installer.total), Max: float64(installer.total)},
	}, nil)

	// Every uninterrupted 15 seconds we will print out a waiting message
	installer.startTicker(15 * time.Second)

	installer.startAnalytics(ctx, installer.cmds)
}

// Starts the installation process in a non-blocking process
func (installer *InstallManager) install(ctx context.Context, cmds []Installer) {
	for _, cmd := range cmds {
		go func() {
			// Set the log channel for the installer
			cmd.SetInstallerOutput(installer.logs)

			if err := cmd.RunInstall(ctx, installer.env); err != nil {
				// if failed, put on the failure queue and exit
				installer.failures <- failedInstall{cmdName: cmd.GetName(), err: err}
			}

			installer.installed <- cmd.GetName()
		}()
	}
}

// Blocks until all installations have successfully completed
// or until a failure occurs
func (installer *InstallManager) wait(ctx context.Context) error {
	for {
		select {
		case cmdName := <-installer.installed:
			installer.handleInstalled(cmdName)

			// Everything installed!
			if installer.isDone() {
				installer.complete()
				return nil
			}

		case failure := <-installer.failures:
			installer.handleFailure(failure.cmdName, failure.err)
			return failure

		case log := <-installer.logs:
			installer.progress.WriteLine(log)

		case <-ctx.Done():
			// Context was canceled, exit early
			return ctx.Err()

		case <-installer.tick():
			installer.handleWaiting()
		}
	}
}

func (installer *InstallManager) startTicker(interval time.Duration) {
	installer.ticker = time.NewTicker(interval)
	installer.tickInterval = interval
}

func (installer *InstallManager) startAnalytics(ctx context.Context, cmds map[string]Installer) {
	installer.stats = startInstallAnalytics(ctx, cmds)
}

func (installer *InstallManager) handleInstalled(name string) {
	installer.stats.handleInstalled(name)
	installer.ticker.Reset(installer.tickInterval)

	installer.done += installer.cmds[name].Count()
	delete(installer.cmds, name)

	installer.progress.WriteLine(output.Styledf(output.StyleSuccess, "%s installed", name))
	installer.progress.SetValue(0, float64(installer.done))
	installer.progress.SetLabelAndRecalc(0, fmt.Sprintf("%d/%d commands installed", int(installer.done), int(installer.total)))
}

func (installer *InstallManager) complete() {
	installer.progress.Complete()

	installer.Write("")
	if installer.verbose {
		installer.WriteLine(output.Linef(output.EmojiSuccess, output.StyleSuccess, "Everything installed! Took %s. Booting up the system!", installer.stats.duration()))
	} else {
		installer.WriteLine(output.Linef(output.EmojiSuccess, output.StyleSuccess, "Everything installed! Booting up the system!"))
	}
	installer.Write("")

	// If there are any pendings logs, print them out
	for {
		select {
		case log := <-installer.logs:
			installer.WriteLine(log)
		default:
			return
		}
	}
}

func (installer *InstallManager) handleFailure(name string, err error) {
	installer.progress.Destroy()
	installer.stats.handleFailure(name, err)
	printCmdError(installer.Output.Output, name, err)
}

func (installer *InstallManager) handleWaiting() {
	names := []string{}
	for name := range installer.cmds {
		names = append(names, name)
	}

	msg := waitingMessages[installer.waitingMessageIndex]
	emoji := output.EmojiHourglass
	if installer.waitingMessageIndex > 3 {
		emoji = output.EmojiShrug
	}

	installer.progress.WriteLine(output.Linef(emoji, output.StyleBold, msg, strings.Join(names, ", ")))
	installer.waitingMessageIndex = (installer.waitingMessageIndex + 1) % len(waitingMessages)
}

func (installer *InstallManager) tick() <-chan time.Time {
	return installer.ticker.C
}

func (installer *InstallManager) isDone() bool {
	return len(installer.cmds) == 0
}

type installAnalytics struct {
	Start time.Time
	Spans map[string]*analytics.Span
}

func startInstallAnalytics(ctx context.Context, cmds map[string]Installer) *installAnalytics {
	installer := &installAnalytics{
		Start: time.Now(),
		Spans: make(map[string]*analytics.Span, len(cmds)),
	}

	for cmd := range cmds {
		_, installer.Spans[cmd] = analytics.NewInvocation(ctx, fmt.Sprintf("install %s", cmd), "install_command")
	}

	interrupt.Register(installer.handleInterrupt)

	return installer
}

func (a *installAnalytics) handleInterrupt() {
	for _, span := range a.Spans {
		if span.IsRecording() {
			span.Cancelled()
			span.End()
		}
	}
}

func (a *installAnalytics) handleInstalled(name string) {
	a.Spans[name].Succeeded()
	a.Spans[name].End()
}

func (a *installAnalytics) handleFailure(name string, err error) {
	a.Spans[name].RecordError("failed", err)
	a.Spans[name].End()
}

func (a *installAnalytics) duration() time.Duration {
	return time.Since(a.Start)
}

type installFunc func(context.Context, map[string]string) error

var installFuncs = map[string]installFunc{
	"installCaddy": func(ctx context.Context, env map[string]string) error {
		version := env["CADDY_VERSION"]
		if version == "" {
			return errors.New("could not find CADDY_VERSION in env")
		}

		root, err := root.RepositoryRoot()
		if err != nil {
			return err
		}

		var os string
		switch runtime.GOOS {
		case "linux":
			os = "linux"
		case "darwin":
			os = "mac"
		}

		archiveName := fmt.Sprintf("caddy_%s_%s_%s", version, os, runtime.GOARCH)
		url := fmt.Sprintf("https://github.com/caddyserver/caddy/releases/download/v%s/%s.tar.gz", version, archiveName)

		target := filepath.Join(root, fmt.Sprintf(".bin/caddy_%s", version))

		return download.ArchivedExecutable(ctx, url, target, "caddy")
	},
	"installJaeger": func(ctx context.Context, env map[string]string) error {
		version := env["JAEGER_VERSION"]

		// Make sure the data folder exists.
		disk := env["JAEGER_DISK"]
		if err := os.MkdirAll(disk, 0o755); err != nil {
			return err
		}

		if version == "" {
			return errors.New("could not find JAEGER_VERSION in env")
		}

		root, err := root.RepositoryRoot()
		if err != nil {
			return err
		}

		archiveName := fmt.Sprintf("jaeger-%s-%s-%s", version, runtime.GOOS, runtime.GOARCH)
		url := fmt.Sprintf("https://github.com/jaegertracing/jaeger/releases/download/v%s/%s.tar.gz", version, archiveName)

		target := filepath.Join(root, fmt.Sprintf(".bin/jaeger-all-in-one-%s", version))

		return download.ArchivedExecutable(ctx, url, target, fmt.Sprintf("%s/jaeger-all-in-one", archiveName))
	},
}

// failedInstall is returned by run when a command failed to run and run exits
type failedInstall struct {
	cmdName string
	err     error
}

func (e failedInstall) Error() string {
	return fmt.Sprintf("failed to run %s", e.cmdName)
}

// As per tradition, if you edit this file you must add a new waiting message
var waitingMessages = []string{
	"Still waiting for %s to finish installing...",
	"Yup, still waiting for %s to finish installing...",
	"Here's the bad news: still waiting for %s to finish installing. The good news is that we finally have a chance to talk, no?",
	"Still waiting for %s to finish installing...",
	"Hey, %s, there's people waiting for you, pal",
	"Sooooo, how are ya? Yeah, waiting. I hear you. Wish %s would hurry up.",
	"I mean, what is %s even doing?",
	"I now expect %s to mean 'producing a miracle' with 'installing'",
	"Still waiting for %s to finish installing...",
	"Before this I think the longest I ever had to wait was at Disneyland in '99, but %s is now #1",
	"Still waiting for %s to finish installing...",
	"At this point it could be anything - does your computer still have power? Come on, %s",
	"Might as well check Slack. %s is taking its time...",
	"In German there's a saying: ein guter Käse braucht seine Zeit - a good cheese needs its time. Maybe %s is cheese?",
	"If %ss turns out to be cheese I'm gonna lose it. Hey, hurry up, will ya",
	"Still waiting for %s to finish installing...",
	"You're probably wondering why I've called %s here today...",
}
