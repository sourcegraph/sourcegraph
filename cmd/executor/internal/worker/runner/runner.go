package runner

import (
	"context"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/util"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/cmdlogger"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/internal/executor/types"
)

// Runner is the interface between an executor and the host on which commands
// are invoked. Having this interface at this level allows us to use the same
// code paths for local development (via shell + docker) as well as production
// usage (via Firecracker).
type Runner interface {
	// Setup prepares the runner to invoke a series of commands.
	Setup(ctx context.Context) error

	// TempDir returns the path to a temporary directory that can be used to.
	// Mostly used for unit testing.
	TempDir() string

	// Teardown disposes of any resources created in Setup.
	Teardown(ctx context.Context) error

	// Run invokes a command in the runner context.
	Run(ctx context.Context, spec Spec) error
}

// Spec represents a command that can be run on a machine, whether that
// is the host, in a virtual machine, or in a docker container.
type Spec struct {
	Job          types.Job
	CommandSpecs []command.Spec
	ScriptPath   string
}

// Options are the options that can be passed to the runner.
type Options struct {
	DockerOptions      command.DockerOptions
	FirecrackerOptions FirecrackerOptions
	KubernetesOptions  KubernetesOptions
}

// NewRunner creates a new runner with the given options.
// TODO: this is for backwards compatibility with the old command runner. It will be removed in favor of the runtime
// implementation - src-cli required to be removed.
func NewRunner(cmd command.Command, dir, vmName string, logger cmdlogger.Logger, options Options, dockerAuthConfig types.DockerAuthConfig, operations *command.Operations) Runner {
	if util.HasShellBuildTag() {
		return NewShellRunner(cmd, logger, dir, options.DockerOptions)
	}

	if !options.FirecrackerOptions.Enabled {
		return NewDockerRunner(cmd, logger, dir, options.DockerOptions, dockerAuthConfig)
	}
	return NewFirecrackerRunner(cmd, logger, dir, vmName, options.FirecrackerOptions, dockerAuthConfig, operations)
}
