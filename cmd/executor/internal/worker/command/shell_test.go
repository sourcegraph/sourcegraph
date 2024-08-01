package command_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func TestNewShellSpec(t *testing.T) {
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
			scriptPath: "some/path",
			spec: command.Spec{
				Image:   "some-image",
				Key:     "some-key",
				Command: []string{"some", "command"},
				Dir:     "/some/dir",
				Env:     []string{"FOO=BAR"},
			},
			expectedSpec: command.Spec{
				Key:       "some-key",
				Command:   []string{"/bin/sh", "/workingDirectory/.sourcegraph-executor/some/path"},
				Dir:       "/workingDirectory/some/dir",
				Env:       []string{"FOO=BAR"},
				Operation: (*observation.Operation)(nil),
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
				Key:       "some-key",
				Command:   []string{"/bin/sh", "/docker/host/mount/path/workingDirectory/.sourcegraph-executor/some/path"},
				Dir:       "/docker/host/mount/path/workingDirectory/some/dir",
				Env:       []string{"FOO=BAR"},
				Operation: (*observation.Operation)(nil),
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
				Key:       "some-key",
				Command:   []string{"/bin/sh", "/workingDirectory/.sourcegraph-executor/some/path"},
				Dir:       "/workingDirectory",
				Env:       []string{"FOO=BAR"},
				Operation: (*observation.Operation)(nil),
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
				Key:       "some-key",
				Command:   []string{"/bin/sh", "/workingDirectory/.sourcegraph-executor/some/path"},
				Dir:       "/workingDirectory/some/dir",
				Env:       []string(nil),
				Operation: (*observation.Operation)(nil),
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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualSpec := command.NewShellSpec(test.workingDir, test.scriptPath, test.spec, test.options)
			assert.Equal(t, test.expectedSpec, actualSpec)
		})
	}
}
