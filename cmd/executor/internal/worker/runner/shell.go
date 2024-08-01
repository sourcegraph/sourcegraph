package runner

import (
	"context"
	"os"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/cmdlogger"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type shellRunner struct {
	cmd            command.Command
	dir            string
	internalLogger log.Logger
	commandLogger  cmdlogger.Logger
	options        command.DockerOptions
	// tmpDir is used to store temporary files used for docker execution.
	tmpDir string
}

var _ Runner = &shellRunner{}

// NewShellRunner creates a new runner that runs shell commands.
func NewShellRunner(
	cmd command.Command,
	logger cmdlogger.Logger,
	dir string,
	options command.DockerOptions,
) Runner {
	return &shellRunner{
		cmd:            cmd,
		dir:            dir,
		internalLogger: log.Scoped("shell-runner"),
		commandLogger:  logger,
		options:        options,
	}
}

func (r *shellRunner) Setup(ctx context.Context) error {
	dir, err := os.MkdirTemp("", "executor-shell-runner")
	if err != nil {
		return errors.Wrap(err, "failed to create tmp dir for shell runner")
	}
	r.tmpDir = dir

	return nil
}

func (r *shellRunner) TempDir() string {
	return r.tmpDir
}

func (r *shellRunner) Teardown(ctx context.Context) error {
	if err := os.RemoveAll(r.tmpDir); err != nil {
		r.internalLogger.Error("Failed to remove shell state tmp dir", log.String("tmpDir", r.tmpDir), log.Error(err))
	}

	return nil
}

func (r *shellRunner) Run(ctx context.Context, spec Spec) error {
	shellSpec := command.NewShellSpec(r.dir, spec.ScriptPath, spec.CommandSpecs[0], r.options)
	return r.cmd.Run(ctx, r.commandLogger, shellSpec)
}
