package runtime

import (
	"context"
	"fmt"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/cmdlogger"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/files"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/runner"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/workspace"
	"github.com/sourcegraph/sourcegraph/internal/executor/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type dockerRuntime struct {
	cmd          command.Command
	operations   *command.Operations
	filesStore   files.Store
	cloneOptions workspace.CloneOptions
	dockerOpts   command.DockerOptions
}

var _ Runtime = &dockerRuntime{}

func (r *dockerRuntime) Name() Name {
	return NameDocker
}

func (r *dockerRuntime) PrepareWorkspace(ctx context.Context, logger cmdlogger.Logger, job types.Job) (workspace.Workspace, error) {
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

func (r *dockerRuntime) NewRunner(ctx context.Context, logger cmdlogger.Logger, filesStore files.Store, options RunnerOptions) (runner.Runner, error) {
	run := runner.NewDockerRunner(r.cmd, logger, options.Path, r.dockerOpts, options.DockerAuthConfig)
	if err := run.Setup(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to setup docker runner")
	}
	return run, nil
}

func (r *dockerRuntime) NewRunnerSpecs(ws workspace.Workspace, job types.Job) ([]runner.Spec, error) {
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

func dockerKey(stepKey string, index int) string {
	if len(stepKey) > 0 {
		return "step.docker." + stepKey
	}
	return fmt.Sprintf("step.docker.%d", index)
}
