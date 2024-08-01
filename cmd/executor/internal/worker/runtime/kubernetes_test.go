package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/runner"
	"github.com/sourcegraph/sourcegraph/internal/executor/types"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func TestKubernetesRuntime_Name(t *testing.T) {
	r := kubernetesRuntime{}
	assert.Equal(t, "kubernetes", string(r.Name()))
}

func TestKubernetesRuntime_NewRunnerSpecs(t *testing.T) {
	operations := command.NewOperations(observation.TestContextTB(t))

	tests := []struct {
		name           string
		job            types.Job
		mockFunc       func(ws *MockWorkspace)
		expected       []runner.Spec
		expectedErr    error
		assertMockFunc func(t *testing.T, ws *MockWorkspace)
	}{
		{
			name:     "No steps",
			job:      types.Job{},
			expected: []runner.Spec{},
			assertMockFunc: func(t *testing.T, ws *MockWorkspace) {
				require.Len(t, ws.ScriptFilenamesFunc.History(), 0)
			},
		},
		{
			name: "Single step",
			job: types.Job{
				DockerSteps: []types.DockerStep{
					{
						Key:      "key-1",
						Image:    "my-image",
						Commands: []string{"echo", "hello"},
						Dir:      ".",
						Env:      []string{"FOO=bar"},
					},
				},
			},
			expected: []runner.Spec{{
				CommandSpecs: []command.Spec{
					{
						Key:     "step.kubernetes.key-1",
						Name:    "step-kubernetes-key-1",
						Command: []string{"/bin/sh", "/job/.sourcegraph-executor/0.0_@.sh"},
						Dir:     ".",
						Env:     []string{"FOO=bar"},
						Image:   "my-image",
					},
				},
				Image: "my-image",
			}},
			assertMockFunc: func(t *testing.T, ws *MockWorkspace) {
				// for single job pods, the script filename comes from the job id + other attributes
				// and not from the workspace
				require.Len(t, ws.ScriptFilenamesFunc.History(), 0)
			},
		},
		{
			name: "Multiple steps",
			job: types.Job{
				DockerSteps: []types.DockerStep{
					{
						Key:      "key-1",
						Image:    "my-image",
						Commands: []string{"echo", "hello"},
						Dir:      ".",
						Env:      []string{"FOO=bar"},
					},
					{
						Key:      "key-2",
						Image:    "my-image",
						Commands: []string{"echo", "hello"},
						Dir:      ".",
						Env:      []string{"FOO=bar"},
					},
				},
			},
			expected: []runner.Spec{
				{
					CommandSpecs: []command.Spec{
						{
							Key:     "step.kubernetes.key-1",
							Name:    "step-kubernetes-key-1",
							Command: []string{"/bin/sh", "/job/.sourcegraph-executor/0.0_@.sh"},
							Dir:     ".",
							Env:     []string{"FOO=bar"},
							Image:   "my-image",
						},
						{
							Key:     "step.kubernetes.key-2",
							Name:    "step-kubernetes-key-2",
							Command: []string{"/bin/sh", "/job/.sourcegraph-executor/0.0_@.sh"},
							Dir:     ".",
							Env:     []string{"FOO=bar"},
							Image:   "my-image",
						},
					},
					Image: "my-image",
				},
			},
			assertMockFunc: func(t *testing.T, ws *MockWorkspace) {
				// for single job pods, the script filename comes from the job id + other attributes
				// and not from the workspace
				require.Len(t, ws.ScriptFilenamesFunc.History(), 0)
			},
		},
		{
			name: "Default key",
			job: types.Job{
				DockerSteps: []types.DockerStep{
					{
						Image:    "my-image",
						Commands: []string{"echo", "hello"},
						Dir:      ".",
						Env:      []string{"FOO=bar"},
					},
				},
			},
			expected: []runner.Spec{{
				CommandSpecs: []command.Spec{
					{
						Key:     "step.kubernetes.0",
						Name:    "step-kubernetes-0",
						Command: []string{"/bin/sh", "/job/.sourcegraph-executor/0.0_@.sh"},
						Dir:     ".",
						Env:     []string{"FOO=bar"},
						Image:   "my-image",
					},
				},
				Image: "my-image",
			}},
			assertMockFunc: func(t *testing.T, ws *MockWorkspace) {
				// for single job pods, the script filename comes from the job id + other attributes
				// and not from the workspace
				require.Len(t, ws.ScriptFilenamesFunc.History(), 0)
			},
		},
		{
			name: "Single pod job",
			job: types.Job{
				ID:             42,
				RepositoryName: "github.com/sourcegraph/sourcegraph",
				Commit:         "deadbeef",
				DockerSteps: []types.DockerStep{
					{
						Key:      "my-key",
						Image:    "my-image",
						Commands: []string{"echo", "hello"},
						Dir:      ".",
						Env:      []string{"FOO=bar"},
					},
				},
			},
			expected: []runner.Spec{{
				CommandSpecs: []command.Spec{
					{
						Key:     "step.kubernetes.my-key",
						Name:    "step-kubernetes-my-key",
						Command: []string{"/bin/sh", "/job/.sourcegraph-executor/42.0_github.com_sourcegraph_sourcegraph@deadbeef.sh"},
						Dir:     ".",
						Env:     []string{"FOO=bar"},
						Image:   "my-image",
					},
				},
			}},
			assertMockFunc: func(t *testing.T, ws *MockWorkspace) {
				// for single job pods, the script filename comes from the job id + other attributes
				// and not from the workspace
				require.Len(t, ws.ScriptFilenamesFunc.History(), 0)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			ws := NewMockWorkspace()

			if test.mockFunc != nil {
				test.mockFunc(ws)
			}

			r := &kubernetesRuntime{options: command.KubernetesContainerOptions{}, operations: operations}
			actual, err := r.NewRunnerSpecs(ws, test.job)
			if test.expectedErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedErr.Error())
			} else {
				require.NoError(t, err)
				require.Len(t, actual, len(test.expected))
				if len(test.expected) > 0 {
					require.Len(t, actual[0].CommandSpecs, len(test.expected[0].CommandSpecs))
				}
				for _, expected := range test.expected {
					// find the matching actual spec based on the command spec key. There will only ever be one command spec per spec.
					var actualSpec runner.Spec
					for _, spec := range actual {
						if spec.CommandSpecs[0].Key == expected.CommandSpecs[0].Key {
							actualSpec = spec
							break
						}
					}
					require.Greater(t, len(actualSpec.CommandSpecs), 0)

					assert.Equal(t, expected.ScriptPath, actualSpec.ScriptPath)
					assert.Equal(t, expected.CommandSpecs[0], actualSpec.CommandSpecs[0])
				}
			}

			test.assertMockFunc(t, ws)
		})
	}
}
