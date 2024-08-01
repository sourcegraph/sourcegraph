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
		logger,
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

	if len(job.DockerSteps) == 0 {
		return []runner.Spec{}, nil
	}

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
}

func kubernetesKey(stepKey string, index int) string {
	if len(stepKey) > 0 {
		return "step.kubernetes." + stepKey
	}
	return fmt.Sprintf("step.kubernetes.%d", index)
}
