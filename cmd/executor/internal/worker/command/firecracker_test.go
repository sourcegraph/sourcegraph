package command_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
)

func TestNewFirecrackerSpec(t *testing.T) {
	tests := []struct {
		name         string
		vmName       string
		scriptPath   string
		spec         command.Spec
		options      command.DockerOptions
		expectedSpec command.Spec
	}{
		{
			name:       "Converts to firecracker spec",
			vmName:     "some-vm",
			scriptPath: "some/path",
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
					"ignite",
					"exec",
					"some-vm",
					"--",
					"docker run --rm -v /work:/data -w /data/some/dir -e FOO=BAR --entrypoint /bin/sh some-image /data/.sourcegraph-executor/some/path",
				},
			},
		},
		{
			name:       "Converts to firecracker spec",
			vmName:     "some-vm",
			scriptPath: "some/path",
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
					"ignite",
					"exec",
					"some-vm",
					"--",
					"docker run --rm -v /work:/data -w /data/some/dir -e FOO=BAR --entrypoint /bin/sh some-image /data/.sourcegraph-executor/some/path",
				},
			},
		},
		{
			name:       "No spec directory",
			vmName:     "some-vm",
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
					"ignite",
					"exec",
					"some-vm",
					"--",
					"docker run --rm -v /work:/data -w /data -e FOO=BAR --entrypoint /bin/sh some-image /data/.sourcegraph-executor/some/path",
				},
			},
		},
		{
			name:   "src-cli",
			vmName: "some-vm",
			spec: command.Spec{
				Key:     "some-key",
				Command: []string{"src", "exec", "-f", "batch.yml"},
				Dir:     "/some/dir",
				Env:     []string{"FOO=BAR"},
			},
			expectedSpec: command.Spec{
				Key: "some-key",
				Command: []string{
					"ignite",
					"exec",
					"some-vm",
					"--",
					"cd /work/some/dir && FOO=BAR src exec -f batch.yml",
				},
			},
		},
		{
			name:   "src-cli without environment variables",
			vmName: "some-vm",
			spec: command.Spec{
				Key:     "some-key",
				Command: []string{"src", "exec", "-f", "batch.yml"},
				Dir:     "/some/dir",
			},
			expectedSpec: command.Spec{
				Key: "some-key",
				Command: []string{
					"ignite",
					"exec",
					"some-vm",
					"--",
					"cd /work/some/dir && src exec -f batch.yml",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualSpec := command.NewFirecrackerSpec(test.vmName, test.scriptPath, test.spec, test.options)
			assert.Equal(t, test.expectedSpec, actualSpec)
		})
	}
}
