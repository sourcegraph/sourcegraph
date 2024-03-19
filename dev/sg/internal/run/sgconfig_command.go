package run

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/rjeczalik/notify"
)

type SGConfigCommand interface {
	// Extracts common config and options, allowing the implementation any final overrides
	GetConfig() SGConfigCommandOptions
	GetBinaryLocation() (string, error)
	GetExecCmd(context.Context) (*exec.Cmd, error)
	UpdateConfig(func(*SGConfigCommandOptions)) SGConfigCommand

	// Optionally returns a bazel target associated with this command
	GetBazelTarget() string

	// Start a file watcher on the relevant filesystem sub-tree for this command
	StartWatch(context.Context) (<-chan struct{}, error)
}

func WatchPaths(ctx context.Context, paths []string, skipEvents ...notify.Event) (<-chan struct{}, error) {
	// Set up the watchers.
	restart := make(chan struct{})
	events := make(chan notify.EventInfo, 1)

	// Do nothing if no watch paths are configured
	if len(paths) == 0 {
		return restart, nil
	}
	relevant := notify.All
	// lil bit magic to remove the skipEvents from the relevant events
	for _, skip := range skipEvents {
		relevant &= ^skip
	}

	for _, path := range paths {
		if err := notify.Watch(path+"/...", events, relevant); err != nil {
			return nil, err
		}
	}
	// Start watching for changes to the source tree
	go func() {
		defer close(events)
		defer notify.Stop(events)

		for {
			select {
			case <-ctx.Done():
				return
			case <-events:
				restart <- struct{}{}
			}
		}
	}()

	return restart, nil
}

type noBinaryError struct {
	name string
	err  error
}

func (e noBinaryError) Error() string {
	return fmt.Sprintf("no-binary-error: %s has no binary", e.name)
}

func (e noBinaryError) Unwrap() error {
	return e.err
}

func (e noBinaryError) Wrap(err error) error {
	e.err = err
	return e
}

func (e noBinaryError) Is(target error) bool {
	_, ok := target.(noBinaryError)
	return ok
}
