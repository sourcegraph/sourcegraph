// Code generated by github.com/efritz/go-mockgen 0.1.0; DO NOT EDIT.

package worker

import (
	"context"
	command "github.com/sourcegraph/sourcegraph/enterprise/cmd/executor/internal/command"
	"sync"
)

// MockRunner is a mock implementation of the Runner interface (from the
// package
// github.com/sourcegraph/sourcegraph/enterprise/cmd/executor/internal/command)
// used for unit testing.
type MockRunner struct {
	// RunFunc is an instance of a mock function object controlling the
	// behavior of the method Run.
	RunFunc *RunnerRunFunc
	// SetupFunc is an instance of a mock function object controlling the
	// behavior of the method Setup.
	SetupFunc *RunnerSetupFunc
	// TeardownFunc is an instance of a mock function object controlling the
	// behavior of the method Teardown.
	TeardownFunc *RunnerTeardownFunc
}

// NewMockRunner creates a new mock of the Runner interface. All methods
// return zero values for all results, unless overwritten.
func NewMockRunner() *MockRunner {
	return &MockRunner{
		RunFunc: &RunnerRunFunc{
			defaultHook: func(context.Context, command.CommandSpec) error {
				return nil
			},
		},
		SetupFunc: &RunnerSetupFunc{
			defaultHook: func(context.Context, []string, []string) error {
				return nil
			},
		},
		TeardownFunc: &RunnerTeardownFunc{
			defaultHook: func(context.Context) error {
				return nil
			},
		},
	}
}

// NewMockRunnerFrom creates a new mock of the MockRunner interface. All
// methods delegate to the given implementation, unless overwritten.
func NewMockRunnerFrom(i command.Runner) *MockRunner {
	return &MockRunner{
		RunFunc: &RunnerRunFunc{
			defaultHook: i.Run,
		},
		SetupFunc: &RunnerSetupFunc{
			defaultHook: i.Setup,
		},
		TeardownFunc: &RunnerTeardownFunc{
			defaultHook: i.Teardown,
		},
	}
}

// RunnerRunFunc describes the behavior when the Run method of the parent
// MockRunner instance is invoked.
type RunnerRunFunc struct {
	defaultHook func(context.Context, command.CommandSpec) error
	hooks       []func(context.Context, command.CommandSpec) error
	history     []RunnerRunFuncCall
	mutex       sync.Mutex
}

// Run delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockRunner) Run(v0 context.Context, v1 command.CommandSpec) error {
	r0 := m.RunFunc.nextHook()(v0, v1)
	m.RunFunc.appendCall(RunnerRunFuncCall{v0, v1, r0})
	return r0
}

// SetDefaultHook sets function that is called when the Run method of the
// parent MockRunner instance is invoked and the hook queue is empty.
func (f *RunnerRunFunc) SetDefaultHook(hook func(context.Context, command.CommandSpec) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Run method of the parent MockRunner instance invokes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *RunnerRunFunc) PushHook(hook func(context.Context, command.CommandSpec) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *RunnerRunFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, command.CommandSpec) error {
		return r0
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *RunnerRunFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, command.CommandSpec) error {
		return r0
	})
}

func (f *RunnerRunFunc) nextHook() func(context.Context, command.CommandSpec) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *RunnerRunFunc) appendCall(r0 RunnerRunFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of RunnerRunFuncCall objects describing the
// invocations of this function.
func (f *RunnerRunFunc) History() []RunnerRunFuncCall {
	f.mutex.Lock()
	history := make([]RunnerRunFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// RunnerRunFuncCall is an object that describes an invocation of method Run
// on an instance of MockRunner.
type RunnerRunFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 command.CommandSpec
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c RunnerRunFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c RunnerRunFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// RunnerSetupFunc describes the behavior when the Setup method of the
// parent MockRunner instance is invoked.
type RunnerSetupFunc struct {
	defaultHook func(context.Context, []string, []string) error
	hooks       []func(context.Context, []string, []string) error
	history     []RunnerSetupFuncCall
	mutex       sync.Mutex
}

// Setup delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockRunner) Setup(v0 context.Context, v1 []string, v2 []string) error {
	r0 := m.SetupFunc.nextHook()(v0, v1, v2)
	m.SetupFunc.appendCall(RunnerSetupFuncCall{v0, v1, v2, r0})
	return r0
}

// SetDefaultHook sets function that is called when the Setup method of the
// parent MockRunner instance is invoked and the hook queue is empty.
func (f *RunnerSetupFunc) SetDefaultHook(hook func(context.Context, []string, []string) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Setup method of the parent MockRunner instance invokes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *RunnerSetupFunc) PushHook(hook func(context.Context, []string, []string) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *RunnerSetupFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, []string, []string) error {
		return r0
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *RunnerSetupFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, []string, []string) error {
		return r0
	})
}

func (f *RunnerSetupFunc) nextHook() func(context.Context, []string, []string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *RunnerSetupFunc) appendCall(r0 RunnerSetupFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of RunnerSetupFuncCall objects describing the
// invocations of this function.
func (f *RunnerSetupFunc) History() []RunnerSetupFuncCall {
	f.mutex.Lock()
	history := make([]RunnerSetupFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// RunnerSetupFuncCall is an object that describes an invocation of method
// Setup on an instance of MockRunner.
type RunnerSetupFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 []string
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 []string
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c RunnerSetupFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c RunnerSetupFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// RunnerTeardownFunc describes the behavior when the Teardown method of the
// parent MockRunner instance is invoked.
type RunnerTeardownFunc struct {
	defaultHook func(context.Context) error
	hooks       []func(context.Context) error
	history     []RunnerTeardownFuncCall
	mutex       sync.Mutex
}

// Teardown delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockRunner) Teardown(v0 context.Context) error {
	r0 := m.TeardownFunc.nextHook()(v0)
	m.TeardownFunc.appendCall(RunnerTeardownFuncCall{v0, r0})
	return r0
}

// SetDefaultHook sets function that is called when the Teardown method of
// the parent MockRunner instance is invoked and the hook queue is empty.
func (f *RunnerTeardownFunc) SetDefaultHook(hook func(context.Context) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Teardown method of the parent MockRunner instance invokes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *RunnerTeardownFunc) PushHook(hook func(context.Context) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *RunnerTeardownFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context) error {
		return r0
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *RunnerTeardownFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context) error {
		return r0
	})
}

func (f *RunnerTeardownFunc) nextHook() func(context.Context) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *RunnerTeardownFunc) appendCall(r0 RunnerTeardownFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of RunnerTeardownFuncCall objects describing
// the invocations of this function.
func (f *RunnerTeardownFunc) History() []RunnerTeardownFuncCall {
	f.mutex.Lock()
	history := make([]RunnerTeardownFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// RunnerTeardownFuncCall is an object that describes an invocation of
// method Teardown on an instance of MockRunner.
type RunnerTeardownFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c RunnerTeardownFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c RunnerTeardownFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}
