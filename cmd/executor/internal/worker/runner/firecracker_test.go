package runner_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/runner"
	"github.com/sourcegraph/sourcegraph/internal/executor/types"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestFirecrackerRunner_Setup(t *testing.T) {
	operations := command.NewOperations(observation.TestContextTB(t))

	tests := []struct {
		name             string
		workspaceDevice  string
		vmName           string
		options          runner.FirecrackerOptions
		dockerAuthConfig types.DockerAuthConfig
		mockFunc         func(cmd *runner.MockCommand)
		assertMockFunc   func(t *testing.T, cmd *runner.MockCommand)
		expectedEntries  map[string]string
		expectedErr      error
	}{
		{
			name:            "Setup default",
			workspaceDevice: "/dev/sda",
			vmName:          "test",
			mockFunc: func(cmd *runner.MockCommand) {
				cmd.RunFunc.SetDefaultReturn(nil)
			},
			assertMockFunc: func(t *testing.T, cmd *runner.MockCommand) {
				require.Len(t, cmd.RunFunc.History(), 1)
				assert.Equal(t, "setup.firecracker.start", cmd.RunFunc.History()[0].Arg2.Key)
				assert.Equal(t, []string{
					"ignite",
					"run",
					"--runtime",
					"docker",
					"--network-plugin",
					"cni",
					"--cpus",
					"0",
					"--memory",
					"",
					"--size",
					"",
					"--volumes",
					"/dev/sda:/work",
					"--ssh",
					"--name",
					"test",
					"--kernel-image",
					"",
					"--kernel-args",
					"console=ttyS0 reboot=k panic=1 pci=off ip=dhcp random.trust_cpu=on i8042.noaux i8042.nomux i8042.nopnp i8042.dumbkbd",
					"--sandbox-image",
					"",
					"",
				}, cmd.RunFunc.History()[0].Arg2.Command)
				assert.Empty(t, cmd.RunFunc.History()[0].Arg2.Dir)
				require.Len(t, cmd.RunFunc.History()[0].Arg2.Env, 1)
				assert.True(t, strings.HasPrefix(cmd.RunFunc.History()[0].Arg2.Env[0], "CNI_CONF_DIR="))
				assert.Equal(t, operations.SetupFirecrackerStart, cmd.RunFunc.History()[0].Arg2.Operation)
			},
			expectedEntries: map[string]string{
				"cni": defaultCNIConfig,
			},
		},
		{
			name:            "Failed to start firecracker",
			workspaceDevice: "/dev/sda",
			vmName:          "test",
			mockFunc: func(cmd *runner.MockCommand) {
				cmd.RunFunc.PushReturn(errors.New("failed"))
			},
			assertMockFunc: func(t *testing.T, cmd *runner.MockCommand) {
				require.Len(t, cmd.RunFunc.History(), 1)
			},
			expectedErr: errors.New("failed to start firecracker vm: failed"),
		},
		{
			name:            "Docker registry mirrors",
			workspaceDevice: "/dev/sda",
			vmName:          "test",
			options: runner.FirecrackerOptions{
				DockerRegistryMirrorURLs: []string{"https://mirror1", "https://mirror2"},
			},
			mockFunc: func(cmd *runner.MockCommand) {
				cmd.RunFunc.SetDefaultReturn(nil)
			},
			assertMockFunc: func(t *testing.T, cmd *runner.MockCommand) {
				require.Len(t, cmd.RunFunc.History(), 1)
				assert.Equal(t, "setup.firecracker.start", cmd.RunFunc.History()[0].Arg2.Key)
				actualCommand := cmd.RunFunc.History()[0].Arg2.Command
				for i, val := range actualCommand {
					if val == "--copy-files" {
						assert.True(t, strings.HasSuffix(actualCommand[i+1], "/docker-daemon.json:/etc/docker/daemon.json"))
						break
					}
				}
			},
			expectedEntries: map[string]string{
				"cni":                defaultCNIConfig,
				"docker-daemon.json": `{"registry-mirrors":["https://mirror1","https://mirror2"]}`,
			},
		},
		{
			name:            "Docker auth config",
			workspaceDevice: "/dev/sda",
			vmName:          "test",
			dockerAuthConfig: types.DockerAuthConfig{
				Auths: map[string]types.DockerAuthConfigAuth{
					"index.docker.io": {
						Auth: []byte("foobar"),
					},
				},
			},
			mockFunc: func(cmd *runner.MockCommand) {
				cmd.RunFunc.SetDefaultReturn(nil)
			},
			assertMockFunc: func(t *testing.T, cmd *runner.MockCommand) {
				require.Len(t, cmd.RunFunc.History(), 1)
				assert.Equal(t, "setup.firecracker.start", cmd.RunFunc.History()[0].Arg2.Key)
				actualCommand := cmd.RunFunc.History()[0].Arg2.Command
				// directory. So we need to do extra work.
				for i, val := range actualCommand {
					if val == "--copy-files" {
						assert.True(t, strings.HasSuffix(actualCommand[i+1], "/etc/docker/cli"))
						break
					}
				}
			},
			expectedEntries: map[string]string{
				"cni":        defaultCNIConfig,
				"dockerAuth": `{"auths":{"index.docker.io":{"auth":"Zm9vYmFy"}}}`,
			},
		},
		{
			name:            "Startup script",
			workspaceDevice: "/dev/sda",
			vmName:          "test",
			options: runner.FirecrackerOptions{
				VMStartupScriptPath: "/tmp/startup.sh",
			},
			mockFunc: func(cmd *runner.MockCommand) {
				cmd.RunFunc.SetDefaultReturn(nil)
			},
			assertMockFunc: func(t *testing.T, cmd *runner.MockCommand) {
				require.Len(t, cmd.RunFunc.History(), 2)
				assert.Equal(t, "setup.firecracker.start", cmd.RunFunc.History()[0].Arg2.Key)
				assert.Equal(t, []string{
					"ignite",
					"run",
					"--runtime",
					"docker",
					"--network-plugin",
					"cni",
					"--cpus",
					"0",
					"--memory",
					"",
					"--size",
					"",
					"--copy-files",
					"/tmp/startup.sh:/tmp/startup.sh",
					"--volumes",
					"/dev/sda:/work",
					"--ssh",
					"--name",
					"test",
					"--kernel-image",
					"",
					"--kernel-args",
					"console=ttyS0 reboot=k panic=1 pci=off ip=dhcp random.trust_cpu=on i8042.noaux i8042.nomux i8042.nopnp i8042.dumbkbd",
					"--sandbox-image",
					"",
					"",
				}, cmd.RunFunc.History()[0].Arg2.Command)
				assert.Equal(t, "setup.startup-script", cmd.RunFunc.History()[1].Arg2.Key)
				assert.Equal(t, []string{
					"ignite",
					"exec",
					"test",
					"--",
					"/tmp/startup.sh",
				}, cmd.RunFunc.History()[1].Arg2.Command)
				assert.Equal(t, operations.SetupStartupScript, cmd.RunFunc.History()[1].Arg2.Operation)
			},
			expectedEntries: map[string]string{
				"cni": defaultCNIConfig,
			},
		},
		{
			name:            "Failed to run startup script",
			workspaceDevice: "/dev/sda",
			vmName:          "test",
			options: runner.FirecrackerOptions{
				VMStartupScriptPath: "/tmp/startup.sh",
			},
			mockFunc: func(cmd *runner.MockCommand) {
				cmd.RunFunc.PushReturn(nil)
				cmd.RunFunc.PushReturn(errors.New("failed"))
			},
			assertMockFunc: func(t *testing.T, cmd *runner.MockCommand) {
				require.Len(t, cmd.RunFunc.History(), 2)
			},
			expectedErr: errors.New("failed to run startup script: failed"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := runner.NewMockCommand()
			logger := runner.NewMockLogger()
			firecrackerRunner := runner.NewFirecrackerRunner(
				cmd,
				logger,
				test.workspaceDevice,
				test.vmName,
				test.options,
				test.dockerAuthConfig,
				operations,
			)

			if test.mockFunc != nil {
				test.mockFunc(cmd)
			}

			ctx := context.Background()
			err := firecrackerRunner.Setup(ctx)
			t.Cleanup(func() {
				firecrackerRunner.Teardown(ctx)
			})

			if test.expectedErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedErr.Error())
			} else {
				require.NoError(t, err)
				fmt.Println(firecrackerRunner.TempDir())
				entries, err := os.ReadDir(firecrackerRunner.TempDir())
				require.NoError(t, err)
				require.Len(t, entries, len(test.expectedEntries))
				for k, expectedVal := range test.expectedEntries {
					if k == "cni" {
						cniEntries, err := os.ReadDir(filepath.Join(firecrackerRunner.TempDir(), k))
						require.NoError(t, err)
						require.Len(t, cniEntries, 1)
						f, err := os.ReadFile(filepath.Join(firecrackerRunner.TempDir(), k, cniEntries[0].Name()))
						require.NoError(t, err)
						assert.JSONEq(t, expectedVal, string(f))
					} else if k == "docker-daemon.json" {
						f, err := os.ReadFile(filepath.Join(firecrackerRunner.TempDir(), k))
						require.NoError(t, err)
						require.JSONEq(t, expectedVal, string(f))
					} else if k == "dockerAuth" {
						var name string
						for _, entry := range entries {
							if strings.HasPrefix(entry.Name(), "docker_auth") {
								name = entry.Name()
								break
							}
						}
						require.NotEmpty(t, name)
						dockerAuthEntries, err := os.ReadDir(filepath.Join(firecrackerRunner.TempDir(), name))
						require.NoError(t, err)
						require.Len(t, dockerAuthEntries, 1)
						f, err := os.ReadFile(filepath.Join(firecrackerRunner.TempDir(), name, dockerAuthEntries[0].Name()))
						require.NoError(t, err)
						assert.JSONEq(t, expectedVal, string(f))
					}
				}
			}

			test.assertMockFunc(t, cmd)
		})
	}
}

const defaultCNIConfig = `
{
  "cniVersion": "0.4.0",
  "name": "ignite-cni-bridge",
  "plugins": [
    {
  	  "type": "bridge",
  	  "bridge": "ignite0",
  	  "isGateway": true,
  	  "isDefaultGateway": true,
  	  "promiscMode": false,
  	  "ipMasq": true,
  	  "ipam": {
  	    "type": "host-local",
  	    "subnet": "10.61.0.0/16"
  	  }
    },
    {
  	  "type": "portmap",
  	  "capabilities": {
  	    "portMappings": true
  	  }
    },
    {
  	  "type": "firewall"
    },
    {
  	  "type": "isolation"
    },
    {
  	  "name": "slowdown",
  	  "type": "bandwidth",
  	  "ingressRate": 0,
  	  "ingressBurst": 0,
  	  "egressRate": 0,
  	  "egressBurst": 0
    }
  ]
}
`

func TestFirecrackerRunner_Teardown(t *testing.T) {
	cmd := runner.NewMockCommand()
	logger := runner.NewMockLogger()
	operations := command.NewOperations(observation.TestContextTB(t))
	firecrackerRunner := runner.NewFirecrackerRunner(cmd, logger, "/dev", "test", runner.FirecrackerOptions{}, types.DockerAuthConfig{}, operations)

	cmd.RunFunc.PushReturn(nil)

	ctx := context.Background()
	err := firecrackerRunner.Setup(ctx)
	require.NoError(t, err)

	dir := firecrackerRunner.TempDir()

	_, err = os.Stat(dir)
	require.NoError(t, err)

	cmd.RunFunc.PushReturn(nil)

	err = firecrackerRunner.Teardown(ctx)
	require.NoError(t, err)

	_, err = os.Stat(dir)
	require.Error(t, err)
	assert.True(t, os.IsNotExist(err))

	require.Len(t, cmd.RunFunc.History(), 2)
	assert.Equal(t, "setup.firecracker.start", cmd.RunFunc.History()[0].Arg2.Key)
	assert.Equal(t, "teardown.firecracker.remove", cmd.RunFunc.History()[1].Arg2.Key)
	assert.Equal(t, []string{"ignite", "rm", "-f", "test"}, cmd.RunFunc.History()[1].Arg2.Command)
}

func TestFirecrackerRunner_Run(t *testing.T) {
	cmd := runner.NewMockCommand()
	logger := runner.NewMockLogger()
	operations := command.NewOperations(observation.TestContextTB(t))
	options := runner.FirecrackerOptions{
		DockerOptions: command.DockerOptions{
			ConfigPath:     "/docker/config",
			AddHostGateway: true,
			Resources: command.ResourceOptions{
				NumCPUs:   10,
				Memory:    "1G",
				DiskSpace: "10G",
			},
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

	firecrackerRunner := runner.NewFirecrackerRunner(cmd, logger, "/dev", "test", options, types.DockerAuthConfig{}, operations)

	cmd.RunFunc.PushReturn(nil)

	err := firecrackerRunner.Run(context.Background(), spec)
	require.NoError(t, err)

	require.Len(t, cmd.RunFunc.History(), 1)
	assert.Equal(t, "some-key", cmd.RunFunc.History()[0].Arg2.Key)
	assert.Equal(t, []string{
		"ignite",
		"exec",
		"test",
		"--",
		"docker --config /docker/config run --rm --add-host=host.docker.internal:host-gateway --cpus 10 --memory 1G -v /work:/data -w /data/workingdir -e FOO=bar --entrypoint /bin/sh alpine /data/.sourcegraph-executor/some/script",
	}, cmd.RunFunc.History()[0].Arg2.Command)
}
