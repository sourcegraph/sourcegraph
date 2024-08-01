package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/ignite"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/janitor"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/util"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/cmdlogger"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/files"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/runner"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/runtime"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/workspace"
	"github.com/sourcegraph/sourcegraph/internal/binary"
	"github.com/sourcegraph/sourcegraph/internal/executor/types"
	executorutil "github.com/sourcegraph/sourcegraph/internal/executor/util"
	"github.com/sourcegraph/sourcegraph/internal/honey"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type handler struct {
	nameSet      *janitor.NameSet
	cmdRunner    util.CmdRunner
	cmd          command.Command
	logStore     cmdlogger.ExecutionLogEntryStore
	filesStore   files.Store
	options      Options
	cloneOptions workspace.CloneOptions
	operations   *command.Operations
	jobRuntime   runtime.Runtime
}

var (
	_ workerutil.Handler[types.Job] = &handler{}
	_ workerutil.WithPreDequeue     = &handler{}
)

// PreDequeue determines if the number of VMs with the current instance's VM Prefix is less than
// the maximum number of concurrent handlers. If so, then a new job can be dequeued. Otherwise,
// we have an orphaned VM somewhere on the host that will be cleaned up by the background janitor
// process - refuse to dequeue a job for now so that we do not over-commit on VMs and cause issues
// with keeping our heartbeats due to machine load. We'll continue to check this condition on the
// polling interval
func (h *handler) PreDequeue(ctx context.Context, logger log.Logger) (dequeueable bool, extraDequeueArguments any, err error) {
	if !h.options.RunnerOptions.FirecrackerOptions.Enabled {
		return true, nil, nil
	}

	runningVMsByName, err := ignite.ActiveVMsByName(context.Background(), h.cmdRunner, h.options.VMPrefix, false)
	if err != nil {
		return false, nil, err
	}

	if len(runningVMsByName) < h.options.WorkerOptions.NumHandlers {
		return true, nil, nil
	}

	logger.Warn("Orphaned VMs detected - refusing to dequeue a new job until it's cleaned up",
		log.Int("numRunningVMs", len(runningVMsByName)),
		log.Int("numHandlers", h.options.WorkerOptions.NumHandlers))
	return false, nil, nil
}

// Handle clones the target code into a temporary directory, invokes the target indexer in a
// fresh docker container, and uploads the results to the external frontend API.
func (h *handler) Handle(ctx context.Context, logger log.Logger, job types.Job) (err error) {
	logger = logger.With(
		log.Int("jobID", job.ID),
		log.String("repositoryName", job.RepositoryName),
		log.String("commit", job.Commit))

	start := time.Now()
	defer func() {
		if honey.Enabled() {
			_ = createHoneyEvent(ctx, job, err, time.Since(start)).Send()
		}
	}()

	redactor := newSecretRedactor(union(h.options.RedactedValues, job.RedactedValues))

	// If audit logs are enabled, we log all safe job properties to the logs.
	// Note: This is a best-effort redaction. Depending on encoding, some secrets might
	// still be present in the logs.
	// Thus, the audit logs are disabled by default and strictly opt-in.
	if h.options.EnableJobAuditLogging {
		redactedValues := make([]string, 0, len(job.RedactedValues))
		for _, redactedValue := range job.RedactedValues {
			redactedValues = append(redactedValues, redactedValue)
		}

		virtualMachineFileFields := []log.Field{}
		for path, file := range job.VirtualMachineFiles {
			if binary.IsBinary(file.Content) {
				virtualMachineFileFields = append(virtualMachineFileFields,
					log.Object(path, log.Bool("binaryContent", true)),
				)
				continue
			}

			virtualMachineFileFields = append(virtualMachineFileFields,
				log.Object(path, log.String("content", redactor.Replace(string(file.Content)))),
			)
		}

		redactEachString := func(in []string) []string {
			out := make([]string, 0, len(in))
			for _, s := range in {
				out = append(out, redactor.Replace(s))
			}
			return out
		}

		dockerStepsFields := []log.Field{}
		for i, step := range job.DockerSteps {
			dockerStepsFields = append(dockerStepsFields,
				log.Object(
					fmt.Sprintf("step%d", i),
					log.String("key", step.Key),
					log.String("image", step.Image),
					log.Strings("command", redactEachString(step.Commands)),
					log.String("dir", step.Dir),
					log.Strings("env", redactEachString(step.Env)),
				),
			)
		}

		cliStepsFields := []log.Field{}
		for i, step := range job.CliSteps {
			cliStepsFields = append(cliStepsFields,
				log.Object(
					fmt.Sprintf("step%d", i),
					log.String("key", step.Key),
					log.Strings("command", redactEachString(step.Commands)),
					log.String("dir", step.Dir),
					log.Strings("env", redactEachString(step.Env)),
				),
			)
		}

		logger.Warn(
			"Received new job to process",
			log.Object("job",
				log.Int("id", job.ID),
				log.Int("version", job.Version),
				log.String("queueName", job.Queue),
				log.String("repositoryName", job.RepositoryName),
				log.String("repositoryDirectory", job.RepositoryDirectory),
				log.String("commit", job.Commit),
				log.Bool("fetchTags", job.FetchTags),
				log.Bool("shallowClone", job.ShallowClone),
				log.Strings("sparseCheckout", job.SparseCheckout),
				log.Strings("redactedValueReplacements", redactedValues),
				log.Int("dockerAuthConfigEntries", len(job.DockerAuthConfig.Auths)),
				log.Object("virtualMachineFiles", virtualMachineFileFields...),
				log.Object("dockerSteps", dockerStepsFields...),
				log.Object("cliSteps", cliStepsFields...),
			),
		)
	}

	// 🚨 SECURITY: The job logger must be supplied with all sensitive values that may appear
	// in a command constructed and run in the following function. Note that the command and
	// its output may both contain sensitive values, but only values which we directly
	// interpolate into the command. No command that we run on the host leaks environment
	// variables, and the user-specified commands (which could leak their environment) are
	// run in a clean VM.
	commandLogger := cmdlogger.NewLogger(logger, h.logStore, job, redactor)
	defer func() {
		if flushErr := commandLogger.Flush(); flushErr != nil {
			err = errors.Append(err, flushErr)
		}
	}()

	// src-cli steps do not work in the new runtime environment.
	// Remove this when native SSBC is complete.
	if len(job.CliSteps) > 0 {
		logger.Debug("Handling src-cli steps")
		return h.handle(ctx, logger, commandLogger, job)
	}

	if h.jobRuntime == nil {
		// For backwards compatibility. If no runtime mode is provided, then use the old handler.
		logger.Debug("Runtime not configured. Falling back to legacy handler")
		return h.handle(ctx, logger, commandLogger, job)
	}

	// Setup all the file, mounts, etc...
	logger.Info("Creating workspace")
	ws, err := h.jobRuntime.PrepareWorkspace(ctx, commandLogger, job)
	if err != nil {
		return errors.Wrap(err, "creating workspace")
	}
	defer ws.Remove(ctx, h.options.RunnerOptions.FirecrackerOptions.KeepWorkspaces)

	// Before we setup a VM (and after we teardown), mark the name as in-use so that
	// the janitor process cleaning up orphaned VMs doesn't try to stop/remove the one
	// we're using for the current job.
	name := newVMName(h.options.VMPrefix)
	h.nameSet.Add(name)
	defer h.nameSet.Remove(name)

	// Create the runner that will actually run the commands.
	logger.Info("Setting up runner")
	runtimeRunner, err := h.jobRuntime.NewRunner(
		ctx,
		commandLogger,
		h.filesStore,
		runtime.RunnerOptions{Path: ws.Path(), DockerAuthConfig: job.DockerAuthConfig, Name: name},
	)
	if err != nil {
		return errors.Wrap(err, "creating runtime runner")
	}
	defer func() {
		// Perform this outside of the task execution context. If there is a timeout or
		// cancellation error we don't want to skip cleaning up the resources that we've
		// allocated for the current task.
		if teardownErr := runtimeRunner.Teardown(context.Background()); teardownErr != nil {
			err = errors.Append(err, teardownErr)
		}
	}()

	// Get the commands we will execute.
	logger.Info("Creating commands")
	job.Queue = h.options.QueueName
	commands, err := h.jobRuntime.NewRunnerSpecs(ws, job)
	if err != nil {
		return errors.Wrap(err, "creating commands")
	}

	// Run all the things.
	logger.Info("Running commands")
	skipKey := ""
	for i, spec := range commands {
		if len(skipKey) > 0 && skipKey != spec.CommandSpecs[0].Key {
			continue
		} else if len(skipKey) > 0 {
			// We have a match, so reset the skip key.
			skipKey = ""
		}
		if err := runtimeRunner.Run(ctx, spec); err != nil {
			return errors.Wrapf(err, "running command %q", spec.CommandSpecs[0].Key)
		}
		if executorutil.IsPreStepKey(spec.CommandSpecs[0].Key) {
			// Check if there is a skip file. and if so, what the next step is.
			nextStep, err := runner.NextStep(ws.WorkingDirectory())
			if err != nil {
				return errors.Wrap(err, "checking for skip file")
			}
			if len(nextStep) > 0 {
				skipKey = runtime.CommandKey(h.jobRuntime.Name(), nextStep, i)
				logger.Info("Skipping to step", log.String("key", skipKey))
			}
		}
	}

	return nil
}

func newSecretRedactor(replacements map[string]string) *strings.Replacer {
	oldnew := make([]string, 0, len(replacements)*2)
	for k, v := range replacements {
		oldnew = append(oldnew, k, v)
	}

	return strings.NewReplacer(oldnew...)
}

func createHoneyEvent(_ context.Context, job types.Job, err error, duration time.Duration) honey.Event {
	fields := map[string]any{
		"duration_ms":    duration.Milliseconds(),
		"recordID":       job.RecordID(),
		"repositoryName": job.RepositoryName,
		"commit":         job.Commit,
		"numDockerSteps": len(job.DockerSteps),
		"numCliSteps":    len(job.CliSteps),
	}

	if err != nil {
		fields["error"] = err.Error()
	}

	return honey.NewEventWithFields("executor", fields)
}

func union(a, b map[string]string) map[string]string {
	c := make(map[string]string, len(a)+len(b))

	for k, v := range a {
		c[k] = v
	}
	for k, v := range b {
		c[k] = v
	}

	return c
}

// Handle clones the target code into a temporary directory, invokes the target indexer in a
// fresh docker container, and uploads the results to the external frontend API.
func (h *handler) handle(ctx context.Context, logger log.Logger, commandLogger cmdlogger.Logger, job types.Job) error {
	// Create a working directory for this job which will be removed once the job completes.
	// If a repository is supplied as part of the job configuration, it will be cloned into
	// the working directory.
	logger.Info("Creating workspace")

	ws, err := h.prepareWorkspace(ctx, h.cmd, job, commandLogger)
	if err != nil {
		return errors.Wrap(err, "failed to prepare workspace")
	}
	defer ws.Remove(ctx, h.options.RunnerOptions.FirecrackerOptions.KeepWorkspaces)

	// Before we setup a VM (and after we teardown), mark the name as in-use so that
	// the janitor process cleaning up orphaned VMs doesn't try to stop/remove the one
	// we're using for the current job.
	name := newVMName(h.options.VMPrefix)
	h.nameSet.Add(name)
	defer h.nameSet.Remove(name)

	jobRunner := runner.NewRunner(h.cmd, ws.Path(), name, commandLogger, h.options.RunnerOptions, job.DockerAuthConfig, h.operations)

	logger.Info("Setting up VM")

	// Setup Firecracker VM (if enabled)
	if err = jobRunner.Setup(ctx); err != nil {
		return errors.Wrap(err, "failed to setup virtual machine")
	}
	defer func() {
		// Perform this outside of the task execution context. If there is a timeout or
		// cancellation error we don't want to skip cleaning up the resources that we've
		// allocated for the current task.
		if teardownErr := jobRunner.Teardown(context.Background()); teardownErr != nil {
			err = errors.Append(err, teardownErr)
		}
	}()

	// Invoke each docker step sequentially
	for i, dockerStep := range job.DockerSteps {
		var key string
		if dockerStep.Key != "" {
			key = fmt.Sprintf("step.docker.%s", dockerStep.Key)
		} else {
			key = fmt.Sprintf("step.docker.%d", i)
		}
		dockerStepCommand := runner.Spec{
			CommandSpecs: []command.Spec{
				{
					Key:       key,
					Dir:       dockerStep.Dir,
					Env:       dockerStep.Env,
					Operation: h.operations.Exec,
					Image:     dockerStep.Image,
				},
			},
			ScriptPath: ws.ScriptFilenames()[i],
			Job:        job,
		}

		logger.Info(fmt.Sprintf("Running docker step #%d", i))

		if err = jobRunner.Run(ctx, dockerStepCommand); err != nil {
			return errors.Wrap(err, "failed to perform docker step")
		}
	}

	// Invoke each src-cli step sequentially
	for i, cliStep := range job.CliSteps {
		var key string
		if cliStep.Key != "" {
			key = fmt.Sprintf("step.src.%s", cliStep.Key)
		} else {
			key = fmt.Sprintf("step.src.%d", i)
		}

		cliStepCommand := runner.Spec{
			CommandSpecs: []command.Spec{
				{
					Key:       key,
					Command:   append([]string{"src"}, cliStep.Commands...),
					Dir:       cliStep.Dir,
					Env:       cliStep.Env,
					Operation: h.operations.Exec,
				},
			},
			Job: job,
		}

		logger.Info(fmt.Sprintf("Running src-cli step #%d", i))

		if err = jobRunner.Run(ctx, cliStepCommand); err != nil {
			return errors.Wrap(err, "failed to perform src-cli step")
		}
	}

	return nil
}

// prepareWorkspace creates and returns a temporary directory in which acts the workspace
// while processing a single job. It is up to the caller to ensure that this directory is
// removed after the job has finished processing. If a repository name is supplied, then
// that repository will be cloned (through the frontend API) into the workspace.
func (h *handler) prepareWorkspace(
	ctx context.Context,
	cmd command.Command,
	job types.Job,
	commandLogger cmdlogger.Logger,
) (workspace.Workspace, error) {
	if h.options.RunnerOptions.FirecrackerOptions.Enabled {
		return workspace.NewFirecrackerWorkspace(
			ctx,
			h.filesStore,
			job,
			h.options.RunnerOptions.DockerOptions.Resources.DiskSpace,
			h.options.RunnerOptions.FirecrackerOptions.KeepWorkspaces,
			h.cmdRunner,
			cmd,
			commandLogger,
			h.cloneOptions,
			h.operations,
		)
	}

	return workspace.NewDockerWorkspace(
		ctx,
		h.filesStore,
		job,
		cmd,
		commandLogger,
		h.cloneOptions,
		h.operations,
	)
}

func newVMName(vmPrefix string) string {
	vmNameSuffix := uuid.NewString()

	// Construct a unique name for the VM prefixed by something that differentiates
	// VMs created by this executor instance and another one that happens to run on
	// the same host (as is the case in dev). This prefix is expected to match the
	// prefix given to ignite.CurrentlyRunningVMs in other parts of this service.
	name := fmt.Sprintf("%s-%s", vmPrefix, vmNameSuffix)
	return name
}
