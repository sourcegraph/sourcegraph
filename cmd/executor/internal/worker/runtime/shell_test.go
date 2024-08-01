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

func TestShellRuntime_Name(t *testing.T) {
	r := shellRuntime{}
	assert.Equal(t, "shell", string(r.Name()))
}

func TestShellRuntime_NewRunnerSpecs(t *testing.T) {
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
			mockFunc: func(ws *MockWorkspace) {
				ws.ScriptFilenamesFunc.SetDefaultReturn([]string{"script.sh"})
			},
			expected: []runner.Spec{{
				CommandSpecs: []command.Spec{
					{
						Key:       "step.docker.key-1",
						Command:   []string(nil),
						Dir:       ".",
						Env:       []string{"FOO=bar"},
						Operation: operations.Exec,
						Image:     "my-image",
					},
				},
				ScriptPath: "script.sh",
			}},
			assertMockFunc: func(t *testing.T, ws *MockWorkspace) {
				require.Len(t, ws.ScriptFilenamesFunc.History(), 1)
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
			mockFunc: func(ws *MockWorkspace) {
				ws.ScriptFilenamesFunc.SetDefaultReturn([]string{"script1.sh", "script2.sh"})
			},
			expected: []runner.Spec{
				{
					CommandSpecs: []command.Spec{
						{
							Key:       "step.docker.key-1",
							Command:   []string(nil),
							Dir:       ".",
							Env:       []string{"FOO=bar"},
							Operation: operations.Exec,
							Image:     "my-image",
						},
					},
					ScriptPath: "script1.sh",
				},
				{
					CommandSpecs: []command.Spec{
						{
							Key:       "step.docker.key-2",
							Command:   []string(nil),
							Dir:       ".",
							Env:       []string{"FOO=bar"},
							Operation: operations.Exec,
							Image:     "my-image",
						},
					},
					ScriptPath: "script2.sh",
				},
			},
			assertMockFunc: func(t *testing.T, ws *MockWorkspace) {
				require.Len(t, ws.ScriptFilenamesFunc.History(), 2)
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
			mockFunc: func(ws *MockWorkspace) {
				ws.ScriptFilenamesFunc.SetDefaultReturn([]string{"script.sh"})
			},
			expected: []runner.Spec{{
				CommandSpecs: []command.Spec{
					{
						Key:       "step.docker.0",
						Command:   []string(nil),
						Dir:       ".",
						Env:       []string{"FOO=bar"},
						Operation: operations.Exec,
						Image:     "my-image",
					},
				},
				ScriptPath: "script.sh",
			}},
			assertMockFunc: func(t *testing.T, ws *MockWorkspace) {
				require.Len(t, ws.ScriptFilenamesFunc.History(), 1)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ws := NewMockWorkspace()

			if test.mockFunc != nil {
				test.mockFunc(ws)
			}

			r := &shellRuntime{operations: operations}
			actual, err := r.NewRunnerSpecs(ws, test.job)
			if test.expectedErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedErr.Error())
			} else {
				require.NoError(t, err)
				require.Len(t, actual, len(test.expected))
				for _, expected := range test.expected {
					// find the matching actual spec based on the command spec key. There will only ever be one command spec per spec.
					var actualSpec runner.Spec
					for _, spec := range actual {
						if spec.CommandSpecs[0].Key == expected.CommandSpecs[0].Key {
							actualSpec = spec
							break
						}
					}
					assert.Equal(t, expected.ScriptPath, actualSpec.ScriptPath)
					assert.Equal(t, expected.CommandSpecs[0], actualSpec.CommandSpecs[0])
				}
			}

			test.assertMockFunc(t, ws)
		})
	}
}
