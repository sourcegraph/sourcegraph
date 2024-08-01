package runtime

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/cmdlogger"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/files"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/runner"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/workspace"
	"github.com/sourcegraph/sourcegraph/internal/executor/types"
)

type kubernetesRuntime struct {
	cmd          command.Command
	kubeCmd      *command.KubernetesCommand
	filesStore   files.Store
	cloneOptions workspace.CloneOptions
	operations   *command.Operations
	options      command.KubernetesContainerOptions
}

var _ Runtime = &kubernetesRuntime{}

func (r *kubernetesRuntime) Name() Name {
	return NameKubernetes
}

func (r *kubernetesRuntime) PrepareWorkspace(ctx context.Context, logger cmdlogger.Logger, job types.Job) (workspace.Workspace, error) {
	return workspace.NewKubernetesWorkspace(
		ctx,
		r.filesStore,
		job,
		r.cmd,
		logger,
		r.cloneOptions,
		command.KubernetesExecutorMountPath,
		r.options.SingleJobPod,
		r.operations,
	)
}

func (r *kubernetesRuntime) NewRunner(ctx context.Context, logger cmdlogger.Logger, filesStore files.Store, options RunnerOptions) (runner.Runner, error) {
	jobRunner := runner.NewKubernetesRunner(r.kubeCmd, logger, options.Path, filesStore, r.options)
	if err := jobRunner.Setup(ctx); err != nil {
		return nil, err
	}
	return jobRunner, nil
}

func (r *kubernetesRuntime) NewRunnerSpecs(ws workspace.Workspace, job types.Job) ([]runner.Spec, error) {
	// TODO switch to the single job in 5.2
	if r.options.SingleJobPod {
		spec := runner.Spec{
			Job: job,
		}

		specs := make([]command.Spec, len(job.DockerSteps))
		for i, step := range job.DockerSteps {
			scriptName := files.ScriptNameFromJobStep(job, i)

			key := kubernetesKey(step.Key, i)
			specs[i] = command.Spec{
				Key:  key,
				Name: strings.ReplaceAll(key, ".", "-"),
				Command: []string{
					"/bin/sh",
					filepath.Join(command.KubernetesJobMountPath, files.ScriptsPath, scriptName),
				},
				Dir:   step.Dir,
				Env:   step.Env,
				Image: step.Image,
			}
		}
		spec.CommandSpecs = specs

		return []runner.Spec{spec}, nil
	} else {
		runnerSpecs := make([]runner.Spec, len(job.DockerSteps))
		for i, step := range job.DockerSteps {
			key := kubernetesKey(step.Key, i)
			runnerSpecs[i] = runner.Spec{
				Job: job,
				CommandSpecs: []command.Spec{
					{
						Key:  key,
						Name: strings.ReplaceAll(key, ".", "-"),
						Command: []string{
							"/bin/sh",
							filepath.Join(command.KubernetesJobMountPath, files.ScriptsPath, ws.ScriptFilenames()[i]),
						},
						Dir:       step.Dir,
						Env:       step.Env,
						Operation: r.operations.Exec,
						Image:     step.Image,
					},
				},
			}
		}
		return runnerSpecs, nil
	}
}

func kubernetesKey(stepKey string, index int) string {
	if len(stepKey) > 0 {
		return "step.kubernetes." + stepKey
	}
	return fmt.Sprintf("step.kubernetes.%d", index)
}
