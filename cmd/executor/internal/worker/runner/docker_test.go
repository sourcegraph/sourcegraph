package runner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/runner"
	"github.com/sourcegraph/sourcegraph/internal/executor/types"
)

func TestDockerRunner_Setup(t *testing.T) {
	tests := []struct {
		name               string
		options            command.DockerOptions
		dockerAuthConfig   types.DockerAuthConfig
		expectedDockerAuth string
		expectedErr        error
	}{
		{
			name: "Setup default",
		},
		{
			name: "Default docker auth",
			options: command.DockerOptions{
				DockerAuthConfig: types.DockerAuthConfig{
					Auths: map[string]types.DockerAuthConfigAuth{
						"index.docker.io": {
							Auth: []byte("foobar"),
						},
					},
				},
			},
			expectedDockerAuth: `{"auths":{"index.docker.io":{"auth":"Zm9vYmFy"}}}`,
		},
		{
			name: "Specific docker auth",
			options: command.DockerOptions{
				DockerAuthConfig: types.DockerAuthConfig{
					Auths: map[string]types.DockerAuthConfigAuth{
						"index.docker.io": {
							Auth: []byte("foobar"),
						},
					},
				},
			},
			dockerAuthConfig: types.DockerAuthConfig{
				Auths: map[string]types.DockerAuthConfigAuth{
					"index.docker.io": {
						Auth: []byte("fazbaz"),
					},
				},
			},
			expectedDockerAuth: `{"auths":{"index.docker.io":{"auth":"ZmF6YmF6"}}}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dockerRunner := runner.NewDockerRunner(nil, nil, "", test.options, test.dockerAuthConfig)

			ctx := context.Background()
			err := dockerRunner.Setup(ctx)
			defer dockerRunner.Teardown(ctx)

			if test.expectedErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedErr.Error())
			} else {
				require.NoError(t, err)
				entries, err := os.ReadDir(dockerRunner.TempDir())
				require.NoError(t, err)
				if len(test.expectedDockerAuth) == 0 {
					require.Len(t, entries, 0)
				} else {
					require.Len(t, entries, 1)
					dockerAuthEntries, err := os.ReadDir(filepath.Join(dockerRunner.TempDir(), entries[0].Name()))
					require.NoError(t, err)
					require.Len(t, dockerAuthEntries, 1)
					f, err := os.ReadFile(filepath.Join(dockerRunner.TempDir(), entries[0].Name(), dockerAuthEntries[0].Name()))
					require.NoError(t, err)
					assert.JSONEq(t, test.expectedDockerAuth, string(f))
				}
			}
		})
	}
}

func TestDockerRunner_Teardown(t *testing.T) {
	dockerRunner := runner.NewDockerRunner(nil, nil, "", command.DockerOptions{}, types.DockerAuthConfig{})
	ctx := context.Background()
	err := dockerRunner.Setup(ctx)
	require.NoError(t, err)

	dir := dockerRunner.TempDir()

	_, err = os.Stat(dir)
	require.NoError(t, err)

	err = dockerRunner.Teardown(ctx)
	require.NoError(t, err)

	_, err = os.Stat(dir)
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestDockerRunner_Run(t *testing.T) {
	cmd := runner.NewMockCommand()
	logger := runner.NewMockLogger()
	dir := "/some/dir"
	options := command.DockerOptions{
		ConfigPath:     "/docker/config",
		AddHostGateway: true,
		Resources: command.ResourceOptions{
			NumCPUs:   10,
			Memory:    "1G",
			DiskSpace: "10G",
		},
	}
	spec := runner.Spec{
		CommandSpecs: []command.Spec{
			{
				Key:     "some-key",
				Command: []string{"echo", "hello"},
				Dir:     "/workingdir",
				Env:     []string{"FOO=bar"},
				Image:   "alpine",
			},
		},
		ScriptPath: "/some/script",
	}

	dockerRunner := runner.NewDockerRunner(cmd, logger, dir, options, types.DockerAuthConfig{})

	cmd.RunFunc.PushReturn(nil)

	err := dockerRunner.Run(context.Background(), spec)

	require.NoError(t, err)

	require.Len(t, cmd.RunFunc.History(), 1)
	assert.Equal(t, "some-key", cmd.RunFunc.History()[0].Arg2.Key)
	assert.Equal(t, []string{
		"docker",
		"--config",
		"/docker/config",
		"run",
		"--rm",
		"--add-host=host.docker.internal:host-gateway",
		"--cpus",
		"10",
		"--memory",
		"1G",
		"-v",
		"/some/dir:/data",
		"-w",
		"/data/workingdir",
		"-e",
		"FOO=bar",
		"--entrypoint",
		"/bin/sh",
		"alpine",
		"/data/.sourcegraph-executor/some/script",
	}, cmd.RunFunc.History()[0].Arg2.Command)
}
