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

func TestShellRunner_Setup(t *testing.T) {
	tests := []struct {
		name               string
		dockerAuthConfig   types.DockerAuthConfig
		expectedDockerAuth string
		expectedErr        error
	}{
		{
			name: "Setup default",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := command.DockerOptions{}
			shellRunner := runner.NewShellRunner(nil, nil, "", options)

			ctx := context.Background()
			err := shellRunner.Setup(ctx)
			defer shellRunner.Teardown(ctx)

			if test.expectedErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedErr.Error())
			} else {
				require.NoError(t, err)
				entries, err := os.ReadDir(shellRunner.TempDir())
				require.NoError(t, err)
				if len(test.expectedDockerAuth) == 0 {
					require.Len(t, entries, 0)
				} else {
					require.Len(t, entries, 1)
					dockerAuthEntries, err := os.ReadDir(filepath.Join(shellRunner.TempDir(), entries[0].Name()))
					require.NoError(t, err)
					require.Len(t, dockerAuthEntries, 1)
					f, err := os.ReadFile(filepath.Join(shellRunner.TempDir(), entries[0].Name(), dockerAuthEntries[0].Name()))
					require.NoError(t, err)
					assert.JSONEq(t, test.expectedDockerAuth, string(f))
				}
			}
		})
	}
}

func TestShellRunner_Teardown(t *testing.T) {
	shellRunner := runner.NewShellRunner(nil, nil, "", command.DockerOptions{})
	ctx := context.Background()
	err := shellRunner.Setup(ctx)
	require.NoError(t, err)

	dir := shellRunner.TempDir()

	_, err = os.Stat(dir)
	require.NoError(t, err)

	err = shellRunner.Teardown(ctx)
	require.NoError(t, err)

	_, err = os.Stat(dir)
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))
}

func TestShellRunner_Run(t *testing.T) {
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

	shellRunner := runner.NewShellRunner(cmd, logger, dir, options)

	cmd.RunFunc.PushReturn(nil)

	err := shellRunner.Run(context.Background(), spec)

	require.NoError(t, err)

	require.Len(t, cmd.RunFunc.History(), 1)
	assert.Equal(t, "some-key", cmd.RunFunc.History()[0].Arg2.Key)
	assert.Equal(t, []string{"/bin/sh", "/some/dir/.sourcegraph-executor/some/script"}, cmd.RunFunc.History()[0].Arg2.Command)
}
