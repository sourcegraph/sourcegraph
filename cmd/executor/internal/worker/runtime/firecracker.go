package runtime

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/util"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/cmdlogger"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/files"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/runner"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/workspace"
	"github.com/sourcegraph/sourcegraph/internal/executor/types"
)

type firecrackerRuntime struct {
	cmdRunner       util.CmdRunner
	cmd             command.Command
	operations      *command.Operations
	filesStore      files.Store
	cloneOptions    workspace.CloneOptions
	firecrackerOpts runner.FirecrackerOptions
}

var _ Runtime = &firecrackerRuntime{}

func (r *firecrackerRuntime) Name() Name {
	return NameFirecracker
}

func (r *firecrackerRuntime) PrepareWorkspace(ctx context.Context, logger cmdlogger.Logger, job types.Job) (workspace.Workspace, error) {
	return workspace.NewFirecrackerWorkspace(
		ctx,
		r.filesStore,
		job,
		r.firecrackerOpts.DockerOptions.Resources.DiskSpace,
		r.firecrackerOpts.KeepWorkspaces,
		r.cmdRunner,
		r.cmd,
		logger,
		r.cloneOptions,
		r.operations,
	)
}

func (r *firecrackerRuntime) NewRunner(ctx context.Context, logger cmdlogger.Logger, filesStore files.Store, options RunnerOptions) (runner.Runner, error) {
	run := runner.NewFirecrackerRunner(
		r.cmd,
		logger,
		options.Path,
		options.Name,
		r.firecrackerOpts,
		options.DockerAuthConfig,
		r.operations,
	)
	if err := run.Setup(ctx); err != nil {
		return nil, err
	}
	return run, nil
}

func (r *firecrackerRuntime) NewRunnerSpecs(ws workspace.Workspace, job types.Job) ([]runner.Spec, error) {
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
