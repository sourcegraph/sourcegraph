package runner

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/sourcegraph/log"
	batchv1 "k8s.io/api/batch/v1"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/cmdlogger"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/files"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// KubernetesOptions contains options for the Kubernetes runner.
type KubernetesOptions struct {
	Enabled          bool
	ConfigPath       string
	ContainerOptions command.KubernetesContainerOptions
}

type kubernetesRunner struct {
	internalLogger log.Logger
	commandLogger  cmdlogger.Logger
	cmd            *command.KubernetesCommand
	jobNames       []string
	secretName     string
	volumeName     string
	dir            string
	filesStore     files.Store
	options        command.KubernetesContainerOptions
}

var _ Runner = &kubernetesRunner{}

// NewKubernetesRunner creates a new Kubernetes runner.
func NewKubernetesRunner(
	cmd *command.KubernetesCommand,
	commandLogger cmdlogger.Logger,
	dir string,
	filesStore files.Store,
	options command.KubernetesContainerOptions,
) Runner {
	return &kubernetesRunner{
		internalLogger: log.Scoped("kubernetes-runner"),
		commandLogger:  commandLogger,
		cmd:            cmd,
		dir:            dir,
		filesStore:     filesStore,
		options:        options,
	}
}

func (r *kubernetesRunner) Setup(ctx context.Context) error {
	// Nothing to do here.
	return nil
}

func (r *kubernetesRunner) TempDir() string {
	return ""
}

func (r *kubernetesRunner) Teardown(ctx context.Context) error {
	if !r.options.KeepJobs {
		logEntry := r.commandLogger.LogEntry("teardown.kubernetes.job", nil)
		defer logEntry.Close()

		exitCode := 0
		for _, name := range r.jobNames {
			r.internalLogger.Debug("Deleting kubernetes job", log.String("name", name))
			if err := r.cmd.DeleteJob(ctx, r.options.Namespace, name); err != nil {
				r.internalLogger.Error(
					"Failed to delete kubernetes job",
					log.String("jobName", name),
					log.Error(err),
				)
				logEntry.Write([]byte("Failed to delete job " + name))
				exitCode = 1
			}
		}

		if r.secretName != "" {
			if err := r.cmd.DeleteSecret(ctx, r.options.Namespace, r.secretName); err != nil {
				r.internalLogger.Error(
					"Failed to delete kubernetes job secret",
					log.String("secret", r.secretName),
					log.Error(err),
				)
				logEntry.Write([]byte("Failed to delete job secret " + r.secretName))
				exitCode = 1
			}
		}

		if r.volumeName != "" {
			if err := r.cmd.DeleteJobPVC(ctx, r.options.Namespace, r.volumeName); err != nil {
				r.internalLogger.Error(
					"Failed to delete kubernetes job volume",
					log.String("volume", r.volumeName),
					log.Error(err),
				)
				logEntry.Write([]byte("Failed to delete job volume " + r.volumeName))
				exitCode = 1
			}
		}

		logEntry.Finalize(exitCode)
	}

	return nil
}

func (r *kubernetesRunner) Run(ctx context.Context, spec Spec) error {
	var job *batchv1.Job
	workspaceFiles, err := files.GetWorkspaceFiles(ctx, r.filesStore, spec.Job, command.KubernetesJobMountPath)
	if err != nil {
		return err
	}

	jobName := fmt.Sprintf("sg-executor-job-%s-%d", spec.Job.Queue, spec.Job.ID)

	r.secretName = jobName + "-secrets"
	secrets, err := r.cmd.CreateSecrets(ctx, r.options.Namespace, r.secretName, map[string]string{"TOKEN": spec.Job.Token})
	if err != nil {
		return err
	}

	if r.options.JobVolume.Type == command.KubernetesVolumeTypePVC {
		r.volumeName = jobName + "-pvc"
		if err = r.cmd.CreateJobPVC(ctx, r.options.Namespace, r.volumeName, r.options.JobVolume.Size); err != nil {
			return err
		}
	}

	relativeURL, err := makeRelativeURL(r.options.CloneOptions.EndpointURL, r.options.CloneOptions.GitServicePath, spec.Job.RepositoryName)
	if err != nil {
		return errors.Wrap(err, "failed to make relative URL")
	}

	repoOptions := command.RepositoryOptions{
		JobID:               spec.Job.ID,
		CloneURL:            relativeURL.String(),
		RepositoryDirectory: spec.Job.RepositoryDirectory,
		Commit:              spec.Job.Commit,
	}
	job = command.NewKubernetesJob(
		jobName,
		spec.CommandSpecs,
		workspaceFiles,
		secrets,
		r.volumeName,
		repoOptions,
		r.options,
	)
	r.internalLogger.Debug("Creating job", log.Int("jobID", spec.Job.ID))
	if _, err := r.cmd.CreateJob(ctx, r.options.Namespace, job); err != nil {
		return errors.Wrap(err, "creating job")
	}
	r.jobNames = append(r.jobNames, job.Name)

	// Wait for the job to complete before reading the logs. This lets us get also get exit codes.
	r.internalLogger.Debug("Waiting for pod to succeed", log.Int("jobID", spec.Job.ID), log.String("jobName", job.Name))

	pod, podWaitErr := r.cmd.WaitForPodToSucceed(ctx, r.commandLogger, r.options.Namespace, job.Name, spec.CommandSpecs)
	// Handle when the wait failed to do the things.
	if podWaitErr != nil && pod == nil {
		return errors.Wrapf(podWaitErr, "waiting for job %s to complete", job.Name)
	}

	// Now handle the wait error.
	if podWaitErr != nil {
		var errMessage string
		if pod.Status.Message != "" {
			errMessage = fmt.Sprintf("job %s failed: %s", job.Name, pod.Status.Message)
		} else {
			errMessage = fmt.Sprintf("job %s failed", job.Name)
		}
		return errors.New(errMessage)
	}
	r.internalLogger.Debug("Job completed successfully", log.Int("jobID", spec.Job.ID))
	return nil
}

func makeRelativeURL(base string, path ...string) (*url.URL, error) {
	baseURL, err := url.Parse(base)
	if err != nil {
		return nil, err
	}

	return baseURL.ResolveReference(&url.URL{Path: filepath.Join(path...)}), nil
}
