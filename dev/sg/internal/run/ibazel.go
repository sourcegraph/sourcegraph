package run

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"

	"github.com/grafana/regexp"
	"github.com/nxadm/tail"

	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

func ibazelLogPath(logsDir string) string {
	return path.Join(logsDir, "ibazel.log")
}

func profileEventsPath(logsDir string) string {
	return path.Join(logsDir, "profile.json")
}

var watchErrorRegex = regexp.MustCompile(`Bazel query failed: exit status 7`)

type IBazel struct {
	targets []string
	events  *iBazelEventHandler
	logsDir string
	logFile *os.File
	proc    *startedCmd
	logs    chan<- output.FancyLine
}

// returns a runner to interact with ibazel.
func NewIBazel(targets []string) (*IBazel, error) {
	logsDir, err := initLogsDir()
	if err != nil {
		return nil, err
	}

	logFile, err := os.Create(ibazelLogPath(logsDir))
	if err != nil {
		return nil, err
	}

	return &IBazel{
		targets: cleanTargets(targets),
		events:  newIBazelEventHandler(profileEventsPath(logsDir)),
		logsDir: logsDir,
		logFile: logFile,
	}, nil
}

func cleanTargets(targets []string) []string {
	output := []string{}

	for _, target := range targets {
		if target != "" && !slices.Contains(output, target) {
			output = append(output, target)
		}
	}
	return output
}

func initLogsDir() (string, error) {
	sghomedir, err := root.GetSGHomePath()
	if err != nil {
		return "", err
	}

	logsdir := path.Join(sghomedir, "sg_start/logs")
	os.RemoveAll(logsdir)
	if err := os.MkdirAll(logsdir, 0744); err != nil && !os.IsExist(err) {
		return "", err
	}

	return logsdir, nil

}

func (ibazel *IBazel) GetName() string {
	return fmt.Sprintf("bazel targets (%s)", strings.Join(ibazel.targets, ", "))
}

func (ibazel *IBazel) RunInstall(ctx context.Context, env map[string]string) error {
	if len(ibazel.targets) == 0 {
		// no Bazel commands so we return
		return nil
	}

	err := ibazel.build(ctx)
	if err != nil {
		return err
	}

	go ibazel.events.watch(ctx)

	// block until initial ibazel build is completed
	return ibazel.WaitForInitialBuild(ctx)
}

func (ib *IBazel) SetInstallerOutput(logs chan<- output.FancyLine) {
	logs <- output.Styledf(output.StyleGrey, "iBazel output can be found at %s", ibazelLogPath(ib.logsDir))
	logs <- output.Styledf(output.StyleGrey, "iBazel log events can be found at %s", profileEventsPath(ib.logsDir))
	ib.logs = logs
}

func (ib *IBazel) Count() int {
	return len(ib.targets)
}

func (ib *IBazel) GetExecCmd(ctx context.Context) *exec.Cmd {
	// Writes iBazel events out to a log file. These are much easier to parse
	// than trying to understand the output directly
	profilePath := "--profile_dev=" + profileEventsPath(ib.logsDir)
	// This enables iBazel to try to apply the fixes from .bazel_fix_commands.json automatically
	enableAutoFix := "--run_output_interactive=false"
	args := append([]string{profilePath, enableAutoFix, "build"}, ib.targets...)
	return exec.CommandContext(ctx, "ibazel", args...)
}

func (ib *IBazel) WaitForInitialBuild(ctx context.Context) error {
	defer ib.events.close()
	for event := range ib.events.events {
		if event.Type == buildDone {
			return nil
		}
		if event.Type == buildFailed {
			bytes, err := os.ReadFile(ibazelLogPath(ib.logsDir))
			if err != nil {
				return errors.Newf("initial ibazel build failed\nfailed to read log file at %s: %w", ibazelLogPath(ib.logsDir), err)
			} else {
				return errors.Newf("initial ibazel build failed\niBazel logs:\n%s", string(bytes))
			}
		}
	}
	return nil
}

func (ib *IBazel) getCommandOptions(ctx context.Context) (commandOptions, error) {
	dir, err := root.RepositoryRoot()
	if err != nil {
		return commandOptions{}, err
	}
	return commandOptions{
		name: "iBazel",
		exec: ib.GetExecCmd(ctx),
		dir:  dir,
		// Don't output iBazel logs (which are all on stderr) until
		// initial build is complete as it will break the progress bar
		stderr: outputOptions{
			buffer: true,
			additionalWriters: []io.Writer{
				ib.logFile,
				&patternMatcher{regex: watchErrorRegex, callback: ib.logWatchError},
			}},
	}, nil
}

// Build starts an ibazel process to build the targets provided in the constructor
// It runs perpetually, watching for file changes
func (ib *IBazel) build(ctx context.Context) error {
	opts, err := ib.getCommandOptions(ctx)
	if err != nil {
		return err
	}
	ib.proc, err = startCmd(ctx, opts)
	return err
}

func (ib *IBazel) StartOutput() {
	ib.proc.StartOutput()
}

func (ib *IBazel) Close() {
	ib.logFile.Close()
	ib.proc.cancel()
}

func (ib *IBazel) logWatchError() {
	buildQuery := `buildfiles(deps(set(%s)))`
	queries := make([]string, len(ib.targets))
	for i, target := range ib.targets {
		queries[i] = fmt.Sprintf(buildQuery, target)
	}

	queryString := strings.Join(queries, " union ")

	msg := `WARNING: iBazel failed to watch for changes, and will be unable to reload upon file changes.
This is likely because bazel query for one of the targets failed. Try running:

bazel query "%s"

to determine which target is crashing the analysis.

`
	ib.logs <- output.Styledf(output.StyleWarning, msg, queryString)
}

type iBazelEventHandler struct {
	events   chan iBazelEvent
	stop     chan struct{}
	filename string
}

func newIBazelEventHandler(filename string) *iBazelEventHandler {
	return &iBazelEventHandler{
		events:   make(chan iBazelEvent),
		stop:     make(chan struct{}),
		filename: filename,
	}
}

// Watch opens the provided profile.json and reads it as it is continuously written by iBazel
// Each time it sees a iBazel event log, it parses it and puts it on the events channel
// This is a blocking function
func (h *iBazelEventHandler) watch(ctx context.Context) {
	_, cancel := context.WithCancelCause(ctx)
	tail, err := tail.TailFile(h.filename, tail.Config{Follow: true, Logger: tail.DiscardingLogger})
	if err != nil {
		cancel(err)
	}
	defer tail.Cleanup()

	for {
		select {
		case line := <-tail.Lines:
			var event iBazelEvent
			if err := json.Unmarshal([]byte(line.Text), &event); err != nil {
				cancel(errors.Newf("failed to unmarshal event json: %s", err))
			}
			h.events <- event
		case <-ctx.Done():
			cancel(ctx.Err())
			return
		case <-h.stop:
			return
		}

	}
}

func (h *iBazelEventHandler) close() {
	h.stop <- struct{}{}
}

// Schema information at https://github.com/bazelbuild/bazel-watcher?tab=readme-ov-file#profiler-events
type iBazelEvent struct {
	// common
	Type      string   `json:"type"`
	Iteration string   `json:"iteration"`
	Time      int64    `json:"time"`
	Targets   []string `json:"targets,omitempty"`
	Elapsed   int64    `json:"elapsed,omitempty"`

	// start event
	IBazelVersion     string `json:"iBazelVersion,omitempty"`
	BazelVersion      string `json:"bazelVersion,omitempty"`
	MaxHeapSize       string `json:"maxHeapSize,omitempty"`
	CommittedHeapSize string `json:"committedHeapSize,omitempty"`

	// change event
	Change string `json:"change,omitempty"`

	// build & reload event
	Changes []string `json:"changes,omitempty"`

	// browser event
	RemoteType    string `json:"remoteType,omitempty"`
	RemoteTime    int64  `json:"remoteTime,omitempty"`
	RemoteElapsed int64  `json:"remoteElapsed,omitempty"`
	RemoteData    string `json:"remoteData,omitempty"`
}

const (
	buildDone   = "BUILD_DONE"
	buildFailed = "BUILD_FAILED"
)
