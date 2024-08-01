package command_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
)

func TestNewDockerSpec(t *testing.T) {
	tests := []struct {
		name         string
		workingDir   string
		scriptPath   string
		spec         command.Spec
		options      command.DockerOptions
		expectedSpec command.Spec
	}{
		{
			name:       "Converts to docker spec",
			workingDir: "/workingDirectory",
			scriptPath: "script/path",
			spec: command.Spec{
				Image:   "some-image",
				Key:     "some-key",
				Command: []string{"some", "command"},
				Dir:     "/some/dir",
				Env:     []string{"FOO=BAR"},
			},
			expectedSpec: command.Spec{
				Key: "some-key",
				Command: []string{
					"docker",
					"run",
					"--rm",
					"-v",
					"/workingDirectory:/data",
					"-w",
					"/data/some/dir",
					"-e",
					"FOO=BAR",
					"--entrypoint",
					"/bin/sh",
					"some-image",
					"/data/.sourcegraph-executor/script/path",
				},
			},
		},
		{
			name:       "Docker Host Mount Path",
			workingDir: "/workingDirectory",
			scriptPath: "some/path",
			spec: command.Spec{
				Image:   "some-image",
				Key:     "some-key",
				Command: []string{"some", "command"},
				Dir:     "/some/dir",
				Env:     []string{"FOO=BAR"},
			},
			options: command.DockerOptions{
				Resources: command.ResourceOptions{
					DockerHostMountPath: "/docker/host/mount/path",
				},
			},
			expectedSpec: command.Spec{
				Key: "some-key",
				Command: []string{
					"docker",
					"run",
					"--rm",
					"-v",
					"/docker/host/mount/path/workingDirectory:/data",
					"-w",
					"/data/some/dir",
					"-e",
					"FOO=BAR",
					"--entrypoint",
					"/bin/sh",
					"some-image",
					"/data/.sourcegraph-executor/some/path",
				},
			},
		},
		{
			name:       "Config Path",
			workingDir: "/workingDirectory",
			scriptPath: "some/path",
			spec: command.Spec{
				Image:   "some-image",
				Key:     "some-key",
				Command: []string{"some", "command"},
				Dir:     "/some/dir",
				Env:     []string{"FOO=BAR"},
			},
			options: command.DockerOptions{
				ConfigPath: "/docker/config/path",
			},
			expectedSpec: command.Spec{
				Key: "some-key",
				Command: []string{
					"docker",
					"--config",
					"/docker/config/path",
					"run",
					"--rm",
					"-v",
					"/workingDirectory:/data",
					"-w",
					"/data/some/dir",
					"-e",
					"FOO=BAR",
					"--entrypoint",
					"/bin/sh",
					"some-image",
					"/data/.sourcegraph-executor/some/path",
				},
			},
		},
		{
			name:       "Docker Host Gateway",
			workingDir: "/workingDirectory",
			scriptPath: "some/path",
			spec: command.Spec{
				Image:   "some-image",
				Key:     "some-key",
				Command: []string{"some", "command"},
				Dir:     "/some/dir",
				Env:     []string{"FOO=BAR"},
			},
			options: command.DockerOptions{
				AddHostGateway: true,
			},
			expectedSpec: command.Spec{
				Key: "some-key",
				Command: []string{
					"docker",
					"run",
					"--rm",
					"--add-host=host.docker.internal:host-gateway",
					"-v",
					"/workingDirectory:/data",
					"-w",
					"/data/some/dir",
					"-e",
					"FOO=BAR",
					"--entrypoint",
					"/bin/sh",
					"some-image",
					"/data/.sourcegraph-executor/some/path",
				},
			},
		},
		{
			name:       "CPU and Memory",
			workingDir: "/workingDirectory",
			scriptPath: "some/path",
			spec: command.Spec{
				Image:   "some-image",
				Key:     "some-key",
				Command: []string{"some", "command"},
				Dir:     "/some/dir",
				Env:     []string{"FOO=BAR"},
			},
			options: command.DockerOptions{
				Resources: command.ResourceOptions{
					NumCPUs: 10,
					Memory:  "10G",
				},
			},
			expectedSpec: command.Spec{
				Key: "some-key",
				Command: []string{
					"docker",
					"run",
					"--rm",
					"--cpus",
					"10",
					"--memory",
					"10G",
					"-v",
					"/workingDirectory:/data",
					"-w",
					"/data/some/dir",
					"-e",
					"FOO=BAR",
					"--entrypoint",
					"/bin/sh",
					"some-image",
					"/data/.sourcegraph-executor/some/path",
				},
			},
		},
		{
			name:       "Default Spec Dir",
			workingDir: "/workingDirectory",
			scriptPath: "some/path",
			spec: command.Spec{
				Image:   "some-image",
				Key:     "some-key",
				Command: []string{"some", "command"},
				Env:     []string{"FOO=BAR"},
			},
			expectedSpec: command.Spec{
				Key: "some-key",
				Command: []string{
					"docker",
					"run",
					"--rm",
					"-v",
					"/workingDirectory:/data",
					"-w",
					"/data",
					"-e",
					"FOO=BAR",
					"--entrypoint",
					"/bin/sh",
					"some-image",
					"/data/.sourcegraph-executor/some/path",
				},
			},
		},
		{
			name:       "No environment variables",
			workingDir: "/workingDirectory",
			scriptPath: "some/path",
			spec: command.Spec{
				Image:   "some-image",
				Key:     "some-key",
				Command: []string{"some", "command"},
				Dir:     "/some/dir",
			},
			expectedSpec: command.Spec{
				Key: "some-key",
				Command: []string{
					"docker",
					"run",
					"--rm",
					"-v",
					"/workingDirectory:/data",
					"-w",
					"/data/some/dir",
					"--entrypoint",
					"/bin/sh",
					"some-image",
					"/data/.sourcegraph-executor/some/path",
				},
			},
		},
		{
			name:       "src-cli Spec",
			workingDir: "/workingDirectory",
			spec: command.Spec{
				Key:     "some-key",
				Command: []string{"src", "exec", "-f", "batch.yml"},
				Dir:     "/some/dir",
				Env:     []string{"FOO=BAR"},
			},
			expectedSpec: command.Spec{
				Key:     "some-key",
				Command: []string{"src", "exec", "-f", "batch.yml"},
				Dir:     "/workingDirectory/some/dir",
				Env:     []string{"FOO=BAR"},
			},
		},
		{
			name:       "src-cli Spec with Config Path",
			workingDir: "/workingDirectory",
			spec: command.Spec{
				Key:     "some-key",
				Command: []string{"src", "exec", "-f", "batch.yml"},
				Dir:     "/some/dir",
				Env:     []string{"FOO=BAR"},
			},
			options: command.DockerOptions{
				ConfigPath: "/my/docker/config/path",
			},
			expectedSpec: command.Spec{
				Key:     "some-key",
				Command: []string{"src", "exec", "-f", "batch.yml"},
				Dir:     "/workingDirectory/some/dir",
				Env:     []string{"FOO=BAR", "DOCKER_CONFIG=/my/docker/config/path"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualSpec := command.NewDockerSpec(test.workingDir, test.scriptPath, test.spec, test.options)
			assert.Equal(t, test.expectedSpec, actualSpec)
		})
	}
}
