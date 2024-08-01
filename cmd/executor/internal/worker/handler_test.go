package worker

import (
	"context"
	"testing"

	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/janitor"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/command"
	"github.com/sourcegraph/sourcegraph/cmd/executor/internal/worker/runner"
	"github.com/sourcegraph/sourcegraph/internal/executor/types"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/workerutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestHandler_PreDequeue(t *testing.T) {
	logger := logtest.Scoped(t)

	tests := []struct {
		name              string
		options           Options
		mockFunc          func(cmdRunner *MockCmdRunner)
		expectedDequeue   bool
		expectedExtraArgs any
		expectedErr       error
		assertMockFunc    func(t *testing.T, cmdRunner *MockCmdRunner)
	}{
		{
			name: "Firecracker not enabled",
			options: Options{
				RunnerOptions: runner.Options{
					FirecrackerOptions: runner.FirecrackerOptions{Enabled: false},
				},
			},
			expectedDequeue: true,
			assertMockFunc: func(t *testing.T, cmdRunner *MockCmdRunner) {
				require.Len(t, cmdRunner.CombinedOutputFunc.History(), 0)
			},
		},
		{
			name: "Firecracker enabled",
			options: Options{
				RunnerOptions: runner.Options{
					FirecrackerOptions: runner.FirecrackerOptions{Enabled: true},
				},
				WorkerOptions: workerutil.WorkerOptions{NumHandlers: 1},
			},
			mockFunc: func(cmdRunner *MockCmdRunner) {
				cmdRunner.CombinedOutputFunc.PushReturn([]byte{}, nil)
			},
			expectedDequeue: true,
			assertMockFunc: func(t *testing.T, cmdRunner *MockCmdRunner) {
				require.Len(t, cmdRunner.CombinedOutputFunc.History(), 1)
				assert.Equal(t, "ignite", cmdRunner.CombinedOutputFunc.History()[0].Arg1)
				assert.Equal(
					t,
					[]string{"ps", "-t", "{{ .Name }}:{{ .UID }}"},
					cmdRunner.CombinedOutputFunc.History()[0].Arg2,
				)
			},
		},
		{
			name: "Orphaned VMs",
			options: Options{
				RunnerOptions: runner.Options{
					FirecrackerOptions: runner.FirecrackerOptions{Enabled: true},
				},
				WorkerOptions: workerutil.WorkerOptions{NumHandlers: 1},
			},
			mockFunc: func(cmdRunner *MockCmdRunner) {
				cmdRunner.CombinedOutputFunc.PushReturn([]byte("foo:bar\nfaz:baz"), nil)
			},
			assertMockFunc: func(t *testing.T, cmdRunner *MockCmdRunner) {
				require.Len(t, cmdRunner.CombinedOutputFunc.History(), 1)
			},
		},
		{
			name: "Less Orphaned VMs than Handlers",
			options: Options{
				RunnerOptions: runner.Options{
					FirecrackerOptions: runner.FirecrackerOptions{Enabled: true},
				},
				WorkerOptions: workerutil.WorkerOptions{NumHandlers: 3},
			},
			mockFunc: func(cmdRunner *MockCmdRunner) {
				cmdRunner.CombinedOutputFunc.PushReturn([]byte("foo:bar\nfaz:baz"), nil)
			},
			expectedDequeue: true,
			assertMockFunc: func(t *testing.T, cmdRunner *MockCmdRunner) {
				require.Len(t, cmdRunner.CombinedOutputFunc.History(), 1)
			},
		},
		{
			name: "Failed to get active VMs",
			options: Options{
				RunnerOptions: runner.Options{
					FirecrackerOptions: runner.FirecrackerOptions{Enabled: true},
				},
				WorkerOptions: workerutil.WorkerOptions{NumHandlers: 3},
			},
			mockFunc: func(cmdRunner *MockCmdRunner) {
				cmdRunner.CombinedOutputFunc.PushReturn(nil, errors.New("failed"))
			},
			expectedErr: errors.New("failed"),
			assertMockFunc: func(t *testing.T, cmdRunner *MockCmdRunner) {
				require.Len(t, cmdRunner.CombinedOutputFunc.History(), 1)
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmdRunner := NewMockCmdRunner()

			h := &handler{
				cmdRunner: cmdRunner,
				options:   test.options,
			}

			if test.mockFunc != nil {
				test.mockFunc(cmdRunner)
			}

			dequeueable, extraArgs, err := h.PreDequeue(context.Background(), logger)
			if test.expectedErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedErr.Error())
				assert.Equal(t, test.expectedDequeue, dequeueable)
				assert.Equal(t, test.expectedExtraArgs, extraArgs)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedDequeue, dequeueable)
				assert.Equal(t, test.expectedExtraArgs, extraArgs)
			}

			test.assertMockFunc(t, cmdRunner)
		})
	}
}

func TestHandler_Handle_Legacy(t *testing.T) {
	// No runtime is configured.
	// Will go away once firecracker is implemented.
	internalLogger := logtest.Scoped(t)
	operations := command.NewOperations(observation.TestContextTB(t))

	tests := []struct {
		name           string
		options        Options
		job            types.Job
		mockFunc       func(cmdRunner *MockCmdRunner, command *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore)
		expectedErr    error
		assertMockFunc func(t *testing.T, cmdRunner *MockCmdRunner, command *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore)
	}{
		{
			name:    "Success with no steps",
			options: Options{},
			job:     types.Job{ID: 42, RepositoryName: "my-repo", Commit: "cool-commit"},
			mockFunc: func(cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				cmd.RunFunc.SetDefaultReturn(nil)
			},
			assertMockFunc: func(t *testing.T, cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				require.Len(t, cmdRunner.CombinedOutputFunc.History(), 0)
				require.Len(t, cmd.RunFunc.History(), 6)
				require.Len(t, filesStore.GetFunc.History(), 0)
			},
		},
		{
			name:    "Success with srcCli steps",
			options: Options{},
			job: types.Job{
				ID:             42,
				RepositoryName: "my-repo",
				Commit:         "cool-commit",
				CliSteps: []types.CliStep{
					{
						Key:      "some-step",
						Commands: []string{"echo", "hello"},
						Dir:      ".",
						Env:      []string{"FOO=bar"},
					},
				},
			},
			mockFunc: func(cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				cmd.RunFunc.SetDefaultReturn(nil)
			},
			assertMockFunc: func(t *testing.T, cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				require.Len(t, cmdRunner.CombinedOutputFunc.History(), 0)

				require.Len(t, cmd.RunFunc.History(), 7)
				assert.Equal(t, "step.src.some-step", cmd.RunFunc.History()[6].Arg2.Key)
				assert.Equal(t, []string{"src", "echo", "hello"}, cmd.RunFunc.History()[6].Arg2.Command)
				// Temp directory. Value is covered by other tests. We just want to ensure it's not empty.
				assert.NotEmpty(t, cmd.RunFunc.History()[6].Arg2.Dir)
				assert.Equal(t, []string{"FOO=bar"}, cmd.RunFunc.History()[6].Arg2.Env)
				assert.Equal(t, operations.Exec, cmd.RunFunc.History()[6].Arg2.Operation)

				require.Len(t, filesStore.GetFunc.History(), 0)
			},
		},
		{
			name:    "Success with srcCli steps default key",
			options: Options{},
			job: types.Job{
				ID:             42,
				RepositoryName: "my-repo",
				Commit:         "cool-commit",
				CliSteps: []types.CliStep{
					{
						Commands: []string{"echo", "hello"},
						Dir:      ".",
						Env:      []string{"FOO=bar"},
					},
				},
			},
			mockFunc: func(cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				cmd.RunFunc.SetDefaultReturn(nil)
			},
			assertMockFunc: func(t *testing.T, cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				require.Len(t, cmdRunner.CombinedOutputFunc.History(), 0)

				require.Len(t, cmd.RunFunc.History(), 7)
				assert.Equal(t, "step.src.0", cmd.RunFunc.History()[6].Arg2.Key)

				require.Len(t, filesStore.GetFunc.History(), 0)
			},
		},
		{
			name:    "Success with docker steps",
			options: Options{},
			job: types.Job{
				ID:             42,
				RepositoryName: "my-repo",
				Commit:         "cool-commit",
				DockerSteps: []types.DockerStep{
					{
						Key:      "some-step",
						Image:    "my-image",
						Commands: []string{"echo", "hello"},
						Dir:      ".",
						Env:      []string{"FOO=bar"},
					},
				},
			},
			mockFunc: func(cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				cmd.RunFunc.SetDefaultReturn(nil)
			},
			assertMockFunc: func(t *testing.T, cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				require.Len(t, cmdRunner.CombinedOutputFunc.History(), 0)
				for x, h := range cmd.RunFunc.History() {
					t.Logf("%d: %s", x, h.Arg2.Command)
				}
				require.Len(t, cmd.RunFunc.History(), 7)
				assert.Equal(t, "step.docker.some-step", cmd.RunFunc.History()[6].Arg2.Key)
				// There is a temporary directory in the command. We don't want to assert on it. The value of command
				// is covered by other tests. Just want to ensure it at least contains some expected values.
				assert.Contains(t, cmd.RunFunc.History()[6].Arg2.Command, "docker")
				assert.Contains(t, cmd.RunFunc.History()[6].Arg2.Command, "run")
				assert.Empty(t, cmd.RunFunc.History()[6].Arg2.Dir)
				assert.Nil(t, cmd.RunFunc.History()[6].Arg2.Env)
				assert.Equal(t, operations.Exec, cmd.RunFunc.History()[6].Arg2.Operation)

				require.Len(t, filesStore.GetFunc.History(), 0)
			},
		},
		{
			name:    "Success with docker steps default key",
			options: Options{},
			job: types.Job{
				ID:             42,
				RepositoryName: "my-repo",
				Commit:         "cool-commit",
				DockerSteps: []types.DockerStep{
					{
						Image:    "my-image",
						Commands: []string{"echo", "hello"},
						Dir:      ".",
						Env:      []string{"FOO=bar"},
					},
				},
			},
			mockFunc: func(cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				cmd.RunFunc.SetDefaultReturn(nil)
			},
			assertMockFunc: func(t *testing.T, cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				require.Len(t, cmdRunner.CombinedOutputFunc.History(), 0)

				require.Len(t, cmd.RunFunc.History(), 7)
				assert.Equal(t, "step.docker.0", cmd.RunFunc.History()[6].Arg2.Key)

				require.Len(t, filesStore.GetFunc.History(), 0)
			},
		},
		{
			name:    "failed to setup workspace",
			options: Options{},
			job:     types.Job{ID: 42, RepositoryName: "my-repo", Commit: "cool-commit"},
			mockFunc: func(cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				// fail on first clone step
				cmd.RunFunc.PushReturn(errors.New("failed"))
			},
			assertMockFunc: func(t *testing.T, cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				require.Len(t, cmdRunner.CombinedOutputFunc.History(), 0)
				require.Len(t, cmd.RunFunc.History(), 1)
				require.Len(t, filesStore.GetFunc.History(), 0)
			},
			expectedErr: errors.New("failed to prepare workspace: failed setup.git.init: failed"),
		},
		{
			name:    "failed with srcCli steps",
			options: Options{},
			job: types.Job{
				ID:             42,
				RepositoryName: "my-repo",
				Commit:         "cool-commit",
				CliSteps: []types.CliStep{
					{
						Key:      "some-step",
						Commands: []string{"echo", "hello"},
						Dir:      ".",
						Env:      []string{"FOO=bar"},
					},
				},
			},
			mockFunc: func(cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				// cloning repo needs to be successful
				cmd.RunFunc.PushReturn(nil)
				cmd.RunFunc.PushReturn(nil)
				cmd.RunFunc.PushReturn(nil)
				cmd.RunFunc.PushReturn(nil)
				cmd.RunFunc.PushReturn(nil)
				cmd.RunFunc.PushReturn(nil)
				// Error on running the actual command
				cmd.RunFunc.PushReturn(errors.New("failed"))
			},
			assertMockFunc: func(t *testing.T, cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				require.Len(t, cmdRunner.CombinedOutputFunc.History(), 0)
				require.Len(t, cmd.RunFunc.History(), 7)
				require.Len(t, filesStore.GetFunc.History(), 0)
			},
			expectedErr: errors.New("failed to perform src-cli step: failed"),
		},
		{
			name:    "failed with docker steps",
			options: Options{},
			job: types.Job{
				ID:             42,
				RepositoryName: "my-repo",
				Commit:         "cool-commit",
				DockerSteps: []types.DockerStep{
					{
						Key:      "some-step",
						Image:    "my-image",
						Commands: []string{"echo", "hello"},
						Dir:      ".",
						Env:      []string{"FOO=bar"},
					},
				},
			},
			mockFunc: func(cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				// cloning repo needs to be successful
				cmd.RunFunc.PushReturn(nil)
				cmd.RunFunc.PushReturn(nil)
				cmd.RunFunc.PushReturn(nil)
				cmd.RunFunc.PushReturn(nil)
				cmd.RunFunc.PushReturn(nil)
				cmd.RunFunc.PushReturn(nil)
				// Error on running the actual command
				cmd.RunFunc.PushReturn(errors.New("failed"))
			},
			assertMockFunc: func(t *testing.T, cmdRunner *MockCmdRunner, cmd *MockCommand, logStore *MockExecutionLogEntryStore, filesStore *MockStore) {
				require.Len(t, cmdRunner.CombinedOutputFunc.History(), 0)
				require.Len(t, cmd.RunFunc.History(), 7)
				require.Len(t, filesStore.GetFunc.History(), 0)
			},
			expectedErr: errors.New("failed to perform docker step: failed"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nameSet := janitor.NewNameSet()
			// Used in prepareWorkspace
			cmdRunner := NewMockCmdRunner()
			// Used in prepareWorkspace, runner
			cmd := NewMockCommand()
			// Used in NewLogger
			logStore := NewMockExecutionLogEntryStore()
			// Used in prepareWorkspace
			filesStore := NewMockStore()

			h := &handler{
				nameSet:    nameSet,
				cmdRunner:  cmdRunner,
				cmd:        cmd,
				logStore:   logStore,
				filesStore: filesStore,
				options:    test.options,
				operations: operations,
			}

			if test.mockFunc != nil {
				test.mockFunc(cmdRunner, cmd, logStore, filesStore)
			}

			err := h.Handle(context.Background(), internalLogger, test.job)
			if test.expectedErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}

			test.assertMockFunc(t, cmdRunner, cmd, logStore, filesStore)
		})
	}
}

func TestHandler_Handle(t *testing.T) {
	internalLogger := logtest.Scoped(t)
	operations := command.NewOperations(observation.TestContextTB(t))

	tests := []struct {
		name           string
		options        Options
		job            types.Job
		mockFunc       func(jobRuntime *MockRuntime, logStore *MockExecutionLogEntryStore, jobRunner *MockRunner, jobWorkspace *MockWorkspace)
		expectedErr    error
		assertMockFunc func(t *testing.T, jobRuntime *MockRuntime, logStore *MockExecutionLogEntryStore, jobRunner *MockRunner, jobWorkspace *MockWorkspace)
	}{
		{
			name:    "Success with no steps",
			options: Options{},
			job:     types.Job{ID: 42, RepositoryName: "my-repo", Commit: "cool-commit"},
			mockFunc: func(jobRuntime *MockRuntime, logStore *MockExecutionLogEntryStore, jobRunner *MockRunner, jobWorkspace *MockWorkspace) {
				jobRuntime.PrepareWorkspaceFunc.PushReturn(jobWorkspace, nil)
				jobRuntime.NewRunnerFunc.PushReturn(jobRunner, nil)
				jobRuntime.NewRunnerSpecsFunc.PushReturn(nil, nil)
				jobRunner.RunFunc.PushReturn(nil)
			},
			assertMockFunc: func(t *testing.T, jobRuntime *MockRuntime, logStore *MockExecutionLogEntryStore, jobRunner *MockRunner, jobWorkspace *MockWorkspace) {
				require.Len(t, jobRuntime.PrepareWorkspaceFunc.History(), 1)
				require.Len(t, jobWorkspace.RemoveFunc.History(), 1)
				require.Len(t, jobRuntime.NewRunnerFunc.History(), 1)
				require.Len(t, jobRunner.TeardownFunc.History(), 1)
				require.Len(t, jobRuntime.NewRunnerSpecsFunc.History(), 1)
				require.Len(t, jobRuntime.NewRunnerSpecsFunc.History()[0].Arg1.DockerSteps, 0)
				require.Len(t, jobRunner.RunFunc.History(), 0)
			},
		},
		{
			name:    "Success with steps",
			options: Options{},
			job: types.Job{
				ID:             42,
				RepositoryName: "my-repo",
				Commit:         "cool-commit",
				DockerSteps: []types.DockerStep{
					{
						Key:      "some-step",
						Image:    "my-image",
						Commands: []string{"echo", "hello"},
						Dir:      ".",
						Env:      []string{"FOO=bar"},
					},
				},
			},
			mockFunc: func(jobRuntime *MockRuntime, logStore *MockExecutionLogEntryStore, jobRunner *MockRunner, jobWorkspace *MockWorkspace) {
				jobRuntime.PrepareWorkspaceFunc.PushReturn(jobWorkspace, nil)
				jobRuntime.NewRunnerFunc.PushReturn(jobRunner, nil)
				jobRuntime.NewRunnerSpecsFunc.PushReturn([]runner.Spec{
					{
						CommandSpecs: []command.Spec{
							{
								Key:       "my-key",
								Command:   []string{"echo", "hello"},
								Dir:       ".",
								Env:       []string{"FOO=bar"},
								Operation: operations.Exec,
							},
						},
						Image:      "my-image",
						ScriptPath: "./foo",
					},
				}, nil)
				jobRunner.RunFunc.PushReturn(nil)
			},
			assertMockFunc: func(t *testing.T, jobRuntime *MockRuntime, logStore *MockExecutionLogEntryStore, jobRunner *MockRunner, jobWorkspace *MockWorkspace) {
				require.Len(t, jobRuntime.PrepareWorkspaceFunc.History(), 1)
				require.Len(t, jobWorkspace.RemoveFunc.History(), 1)
				require.Len(t, jobRuntime.NewRunnerFunc.History(), 1)
				assert.NotEmpty(t, jobRuntime.NewRunnerFunc.History()[0].Arg3.Name)
				require.Len(t, jobRunner.TeardownFunc.History(), 1)
				require.Len(t, jobRuntime.NewRunnerSpecsFunc.History(), 1)
				require.Len(t, jobRuntime.NewRunnerSpecsFunc.History()[0].Arg1.DockerSteps, 1)
				assert.Equal(t, "some-step", jobRuntime.NewRunnerSpecsFunc.History()[0].Arg1.DockerSteps[0].Key)
				assert.Equal(t, "my-image", jobRuntime.NewRunnerSpecsFunc.History()[0].Arg1.DockerSteps[0].Image)
				assert.Equal(t, []string{"echo", "hello"}, jobRuntime.NewRunnerSpecsFunc.History()[0].Arg1.DockerSteps[0].Commands)
				assert.Equal(t, ".", jobRuntime.NewRunnerSpecsFunc.History()[0].Arg1.DockerSteps[0].Dir)
				assert.Equal(t, []string{"FOO=bar"}, jobRuntime.NewRunnerSpecsFunc.History()[0].Arg1.DockerSteps[0].Env)
				require.Len(t, jobRunner.RunFunc.History(), 1)
				assert.Equal(t, "my-image", jobRunner.RunFunc.History()[0].Arg1.Image)
				assert.Equal(t, "./foo", jobRunner.RunFunc.History()[0].Arg1.ScriptPath)
				require.Len(t, jobRunner.RunFunc.History()[0].Arg1.CommandSpecs, 1)
				assert.Equal(t, []string{"echo", "hello"}, jobRunner.RunFunc.History()[0].Arg1.CommandSpecs[0].Command)
			},
		},
		{
			name:    "failed to setup workspace",
			options: Options{},
			job:     types.Job{ID: 42, RepositoryName: "my-repo", Commit: "cool-commit"},
			mockFunc: func(jobRuntime *MockRuntime, logStore *MockExecutionLogEntryStore, jobRunner *MockRunner, jobWorkspace *MockWorkspace) {
				jobRuntime.PrepareWorkspaceFunc.PushReturn(nil, errors.New("failed"))
			},
			assertMockFunc: func(t *testing.T, jobRuntime *MockRuntime, logStore *MockExecutionLogEntryStore, jobRunner *MockRunner, jobWorkspace *MockWorkspace) {
				require.Len(t, jobRuntime.PrepareWorkspaceFunc.History(), 1)
				require.Len(t, jobWorkspace.RemoveFunc.History(), 0)
				require.Len(t, jobRuntime.NewRunnerFunc.History(), 0)
				require.Len(t, jobRunner.TeardownFunc.History(), 0)
				require.Len(t, jobRuntime.NewRunnerSpecsFunc.History(), 0)
				require.Len(t, jobRunner.RunFunc.History(), 0)
			},
			expectedErr: errors.New("creating workspace: failed"),
		},
		{
			name:    "failed to setup runner",
			options: Options{},
			job:     types.Job{ID: 42, RepositoryName: "my-repo", Commit: "cool-commit"},
			mockFunc: func(jobRuntime *MockRuntime, logStore *MockExecutionLogEntryStore, jobRunner *MockRunner, jobWorkspace *MockWorkspace) {
				jobRuntime.PrepareWorkspaceFunc.PushReturn(jobWorkspace, nil)
				jobRuntime.NewRunnerFunc.PushReturn(nil, errors.New("failed"))
			},
			assertMockFunc: func(t *testing.T, jobRuntime *MockRuntime, logStore *MockExecutionLogEntryStore, jobRunner *MockRunner, jobWorkspace *MockWorkspace) {
				require.Len(t, jobRuntime.PrepareWorkspaceFunc.History(), 1)
				require.Len(t, jobWorkspace.RemoveFunc.History(), 1)
				require.Len(t, jobRuntime.NewRunnerFunc.History(), 1)
				require.Len(t, jobRunner.TeardownFunc.History(), 0)
				require.Len(t, jobRuntime.NewRunnerSpecsFunc.History(), 0)
				require.Len(t, jobRunner.RunFunc.History(), 0)
			},
			expectedErr: errors.New("creating runtime runner: failed"),
		},
		{
			name:    "failed to create commands",
			options: Options{},
			job:     types.Job{ID: 42, RepositoryName: "my-repo", Commit: "cool-commit"},
			mockFunc: func(jobRuntime *MockRuntime, logStore *MockExecutionLogEntryStore, jobRunner *MockRunner, jobWorkspace *MockWorkspace) {
				jobRuntime.PrepareWorkspaceFunc.PushReturn(jobWorkspace, nil)
				jobRuntime.NewRunnerFunc.PushReturn(jobRunner, nil)
				jobRuntime.NewRunnerSpecsFunc.PushReturn(nil, errors.New("failed"))
			},
			assertMockFunc: func(t *testing.T, jobRuntime *MockRuntime, logStore *MockExecutionLogEntryStore, jobRunner *MockRunner, jobWorkspace *MockWorkspace) {
				require.Len(t, jobRuntime.PrepareWorkspaceFunc.History(), 1)
				require.Len(t, jobWorkspace.RemoveFunc.History(), 1)
				require.Len(t, jobRuntime.NewRunnerFunc.History(), 1)
				require.Len(t, jobRunner.TeardownFunc.History(), 1)
				require.Len(t, jobRuntime.NewRunnerSpecsFunc.History(), 1)
				require.Len(t, jobRunner.RunFunc.History(), 0)
			},
			expectedErr: errors.New("creating commands: failed"),
		},
		{
			name:    "failed to run command",
			options: Options{},
			job:     types.Job{ID: 42, RepositoryName: "my-repo", Commit: "cool-commit"},
			mockFunc: func(jobRuntime *MockRuntime, logStore *MockExecutionLogEntryStore, jobRunner *MockRunner, jobWorkspace *MockWorkspace) {
				jobRuntime.PrepareWorkspaceFunc.PushReturn(jobWorkspace, nil)
				jobRuntime.NewRunnerFunc.PushReturn(jobRunner, nil)
				jobRuntime.NewRunnerSpecsFunc.PushReturn([]runner.Spec{
					{
						CommandSpecs: []command.Spec{
							{
								Key:       "my-key",
								Command:   []string{"echo", "hello"},
								Dir:       ".",
								Env:       []string{"FOO=bar"},
								Operation: operations.Exec,
							},
						},
						Image:      "my-image",
						ScriptPath: "./foo",
					},
				}, nil)
				jobRunner.RunFunc.PushReturn(errors.New("failed"))
			},
			assertMockFunc: func(t *testing.T, jobRuntime *MockRuntime, logStore *MockExecutionLogEntryStore, jobRunner *MockRunner, jobWorkspace *MockWorkspace) {
				require.Len(t, jobRuntime.PrepareWorkspaceFunc.History(), 1)
				require.Len(t, jobWorkspace.RemoveFunc.History(), 1)
				require.Len(t, jobRuntime.NewRunnerFunc.History(), 1)
				require.Len(t, jobRunner.TeardownFunc.History(), 1)
				require.Len(t, jobRuntime.NewRunnerSpecsFunc.History(), 1)
				require.Len(t, jobRunner.RunFunc.History(), 1)
			},
			expectedErr: errors.New("running command \"my-key\": failed"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nameSet := janitor.NewNameSet()
			jobRuntime := NewMockRuntime()
			jobRunner := NewMockRunner()
			jobWorkspace := NewMockWorkspace()
			// Used in NewLogger
			logStore := NewMockExecutionLogEntryStore()

			h := &handler{
				nameSet:    nameSet,
				jobRuntime: jobRuntime,
				logStore:   logStore,
				options:    test.options,
				operations: operations,
			}

			if test.mockFunc != nil {
				test.mockFunc(jobRuntime, logStore, jobRunner, jobWorkspace)
			}

			err := h.Handle(context.Background(), internalLogger, test.job)
			if test.expectedErr != nil {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}

			test.assertMockFunc(t, jobRuntime, logStore, jobRunner, jobWorkspace)
		})
	}
}
