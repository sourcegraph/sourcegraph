package runtime

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/cmdlogger"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/files"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/runner"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/workspace"
	"github.com/sourcegraph/sourcegraph/internal/executor/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type shellRuntime struct {
	cmd          command.Command
	operations   *command.Operations
	filesStore   files.Store
	cloneOptions workspace.CloneOptions
	dockerOpts   command.DockerOptions
}

var _ Runtime = &shellRuntime{}

func (r *shellRuntime) Name() Name {
	return NameShell
}

func (r *shellRuntime) PrepareWorkspace(ctx context.Context, logger cmdlogger.Logger, job types.Job) (workspace.Workspace, error) {
	return workspace.NewDockerWorkspace(
		ctx,
		r.filesStore,
		job,
		r.cmd,
		logger,
		r.cloneOptions,
		r.operations,
	)
}

func (r *shellRuntime) NewRunner(ctx context.Context, logger cmdlogger.Logger, filesStore files.Store, options RunnerOptions) (runner.Runner, error) {
	run := runner.NewShellRunner(r.cmd, logger, options.Path, r.dockerOpts)
	if err := run.Setup(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to setup shell runner")
	}

	return run, nil
}

func (r *shellRuntime) NewRunnerSpecs(ws workspace.Workspace, job types.Job) ([]runner.Spec, error) {
	runnerSpecs := make([]runner.Spec, len(job.DockerSteps))
	for i, step := range job.DockerSteps {
		runnerSpecs[i] = runner.Spec{
			Job: job,
			CommandSpecs: []command.Spec{
				{
					Key:       dockerKey(step.Key, i),
					Command:   nil,
					Dir:       step.Dir,
					Env:       step.Env,
					Operation: r.operations.Exec,
					Image:     step.Image,
				},
			},
			ScriptPath: ws.ScriptFilenames()[i],
		}
	}

	return runnerSpecs, nil
}
