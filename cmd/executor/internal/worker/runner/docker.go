package runner

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/cmdlogger"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/internal/executor/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type dockerRunner struct {
	cmd              command.Command
	dir              string
	internalLogger   log.Logger
	commandLogger    cmdlogger.Logger
	options          command.DockerOptions
	dockerAuthConfig types.DockerAuthConfig
	// tmpDir is used to store temporary files used for docker execution.
	tmpDir string
}

var _ Runner = &dockerRunner{}

func NewDockerRunner(
	cmd command.Command,
	logger cmdlogger.Logger,
	dir string,
	options command.DockerOptions,
	dockerAuthConfig types.DockerAuthConfig,
) Runner {
	// Use the option configuration unless the user has provided a custom configuration.
	actualDockerAuthConfig := options.DockerAuthConfig
	if len(dockerAuthConfig.Auths) > 0 {
		actualDockerAuthConfig = dockerAuthConfig
	}

	return &dockerRunner{
		cmd:              cmd,
		dir:              dir,
		internalLogger:   log.Scoped("docker-runner"),
		commandLogger:    logger,
		options:          options,
		dockerAuthConfig: actualDockerAuthConfig,
	}
}

func (r *dockerRunner) TempDir() string {
	return r.tmpDir
}

func (r *dockerRunner) Setup(ctx context.Context) error {
	dir, err := os.MkdirTemp("", "executor-docker-runner")
	if err != nil {
		return errors.Wrap(err, "failed to create tmp dir for docker runner")
	}
	r.tmpDir = dir

	// If docker auth config is present, write it.
	if len(r.dockerAuthConfig.Auths) > 0 {
		d, err := json.Marshal(r.dockerAuthConfig)
		if err != nil {
			return err
		}

		dockerConfigPath, err := os.MkdirTemp(r.tmpDir, "docker_auth")
		if err != nil {
			return err
		}
		r.options.ConfigPath = dockerConfigPath

		if err = os.WriteFile(filepath.Join(r.options.ConfigPath, "config.json"), d, os.ModePerm); err != nil {
			return err
		}
	}

	return nil
}

func (r *dockerRunner) Teardown(ctx context.Context) error {
	if err := os.RemoveAll(r.tmpDir); err != nil {
		r.internalLogger.Error(
			"Failed to remove docker state tmp dir",
			log.String("tmpDir", r.tmpDir),
			log.Error(err),
		)
	}

	return nil
}

func (r *dockerRunner) Run(ctx context.Context, spec Spec) error {
	dockerSpec := command.NewDockerSpec(r.dir, spec.ScriptPath, spec.CommandSpecs[0], r.options)
	return r.cmd.Run(ctx, r.commandLogger, dockerSpec)
}
