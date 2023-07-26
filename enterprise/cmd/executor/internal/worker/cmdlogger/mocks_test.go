// Code generated by go-mockgen 1.3.7; DO NOT EDIT.
//
// This file was generated by running `sg generate` (or `go-mockgen`) at the root of
// this repository. To add additional mocks to this or another package, add a new entry
// to the mockgen.yaml file in the root of this repository.

package cmdlogger

import (
	"context"
	"sync"

	executor "github.com/sourcegraph/sourcegraph/internal/executor"
	types "github.com/sourcegraph/sourcegraph/internal/executor/types"
)

// MockExecutionLogEntryStore is a mock implementation of the
// ExecutionLogEntryStore interface (from the package
// github.com/sourcegraph/sourcegraph/enterprise/cmd/executor/internal/worker/cmdlogger)
// used for unit testing.
type MockExecutionLogEntryStore struct {
	// AddExecutionLogEntryFunc is an instance of a mock function object
	// controlling the behavior of the method AddExecutionLogEntry.
	AddExecutionLogEntryFunc *ExecutionLogEntryStoreAddExecutionLogEntryFunc
	// UpdateExecutionLogEntryFunc is an instance of a mock function object
	// controlling the behavior of the method UpdateExecutionLogEntry.
	UpdateExecutionLogEntryFunc *ExecutionLogEntryStoreUpdateExecutionLogEntryFunc
}

// NewMockExecutionLogEntryStore creates a new mock of the
// ExecutionLogEntryStore interface. All methods return zero values for all
// results, unless overwritten.
func NewMockExecutionLogEntryStore() *MockExecutionLogEntryStore {
	return &MockExecutionLogEntryStore{
		AddExecutionLogEntryFunc: &ExecutionLogEntryStoreAddExecutionLogEntryFunc{
			defaultHook: func(context.Context, types.Job, executor.ExecutionLogEntry) (r0 int, r1 error) {
				return
			},
		},
		UpdateExecutionLogEntryFunc: &ExecutionLogEntryStoreUpdateExecutionLogEntryFunc{
			defaultHook: func(context.Context, types.Job, int, executor.ExecutionLogEntry) (r0 error) {
				return
			},
		},
	}
}

// NewStrictMockExecutionLogEntryStore creates a new mock of the
// ExecutionLogEntryStore interface. All methods panic on invocation, unless
// overwritten.
func NewStrictMockExecutionLogEntryStore() *MockExecutionLogEntryStore {
	return &MockExecutionLogEntryStore{
		AddExecutionLogEntryFunc: &ExecutionLogEntryStoreAddExecutionLogEntryFunc{
			defaultHook: func(context.Context, types.Job, executor.ExecutionLogEntry) (int, error) {
				panic("unexpected invocation of MockExecutionLogEntryStore.AddExecutionLogEntry")
			},
		},
		UpdateExecutionLogEntryFunc: &ExecutionLogEntryStoreUpdateExecutionLogEntryFunc{
			defaultHook: func(context.Context, types.Job, int, executor.ExecutionLogEntry) error {
				panic("unexpected invocation of MockExecutionLogEntryStore.UpdateExecutionLogEntry")
			},
		},
	}
}

// NewMockExecutionLogEntryStoreFrom creates a new mock of the
// MockExecutionLogEntryStore interface. All methods delegate to the given
// implementation, unless overwritten.
func NewMockExecutionLogEntryStoreFrom(i ExecutionLogEntryStore) *MockExecutionLogEntryStore {
	return &MockExecutionLogEntryStore{
		AddExecutionLogEntryFunc: &ExecutionLogEntryStoreAddExecutionLogEntryFunc{
			defaultHook: i.AddExecutionLogEntry,
		},
		UpdateExecutionLogEntryFunc: &ExecutionLogEntryStoreUpdateExecutionLogEntryFunc{
			defaultHook: i.UpdateExecutionLogEntry,
		},
	}
}

// ExecutionLogEntryStoreAddExecutionLogEntryFunc describes the behavior
// when the AddExecutionLogEntry method of the parent
// MockExecutionLogEntryStore instance is invoked.
type ExecutionLogEntryStoreAddExecutionLogEntryFunc struct {
	defaultHook func(context.Context, types.Job, executor.ExecutionLogEntry) (int, error)
	hooks       []func(context.Context, types.Job, executor.ExecutionLogEntry) (int, error)
	history     []ExecutionLogEntryStoreAddExecutionLogEntryFuncCall
	mutex       sync.Mutex
}

// AddExecutionLogEntry delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockExecutionLogEntryStore) AddExecutionLogEntry(v0 context.Context, v1 types.Job, v2 executor.ExecutionLogEntry) (int, error) {
	r0, r1 := m.AddExecutionLogEntryFunc.nextHook()(v0, v1, v2)
	m.AddExecutionLogEntryFunc.appendCall(ExecutionLogEntryStoreAddExecutionLogEntryFuncCall{v0, v1, v2, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the AddExecutionLogEntry
// method of the parent MockExecutionLogEntryStore instance is invoked and
// the hook queue is empty.
func (f *ExecutionLogEntryStoreAddExecutionLogEntryFunc) SetDefaultHook(hook func(context.Context, types.Job, executor.ExecutionLogEntry) (int, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// AddExecutionLogEntry method of the parent MockExecutionLogEntryStore
// instance invokes the hook at the front of the queue and discards it.
// After the queue is empty, the default hook function is invoked for any
// future action.
func (f *ExecutionLogEntryStoreAddExecutionLogEntryFunc) PushHook(hook func(context.Context, types.Job, executor.ExecutionLogEntry) (int, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *ExecutionLogEntryStoreAddExecutionLogEntryFunc) SetDefaultReturn(r0 int, r1 error) {
	f.SetDefaultHook(func(context.Context, types.Job, executor.ExecutionLogEntry) (int, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *ExecutionLogEntryStoreAddExecutionLogEntryFunc) PushReturn(r0 int, r1 error) {
	f.PushHook(func(context.Context, types.Job, executor.ExecutionLogEntry) (int, error) {
		return r0, r1
	})
}

func (f *ExecutionLogEntryStoreAddExecutionLogEntryFunc) nextHook() func(context.Context, types.Job, executor.ExecutionLogEntry) (int, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *ExecutionLogEntryStoreAddExecutionLogEntryFunc) appendCall(r0 ExecutionLogEntryStoreAddExecutionLogEntryFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of
// ExecutionLogEntryStoreAddExecutionLogEntryFuncCall objects describing the
// invocations of this function.
func (f *ExecutionLogEntryStoreAddExecutionLogEntryFunc) History() []ExecutionLogEntryStoreAddExecutionLogEntryFuncCall {
	f.mutex.Lock()
	history := make([]ExecutionLogEntryStoreAddExecutionLogEntryFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// ExecutionLogEntryStoreAddExecutionLogEntryFuncCall is an object that
// describes an invocation of method AddExecutionLogEntry on an instance of
// MockExecutionLogEntryStore.
type ExecutionLogEntryStoreAddExecutionLogEntryFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 types.Job
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 executor.ExecutionLogEntry
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 int
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c ExecutionLogEntryStoreAddExecutionLogEntryFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c ExecutionLogEntryStoreAddExecutionLogEntryFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// ExecutionLogEntryStoreUpdateExecutionLogEntryFunc describes the behavior
// when the UpdateExecutionLogEntry method of the parent
// MockExecutionLogEntryStore instance is invoked.
type ExecutionLogEntryStoreUpdateExecutionLogEntryFunc struct {
	defaultHook func(context.Context, types.Job, int, executor.ExecutionLogEntry) error
	hooks       []func(context.Context, types.Job, int, executor.ExecutionLogEntry) error
	history     []ExecutionLogEntryStoreUpdateExecutionLogEntryFuncCall
	mutex       sync.Mutex
}

// UpdateExecutionLogEntry delegates to the next hook function in the queue
// and stores the parameter and result values of this invocation.
func (m *MockExecutionLogEntryStore) UpdateExecutionLogEntry(v0 context.Context, v1 types.Job, v2 int, v3 executor.ExecutionLogEntry) error {
	r0 := m.UpdateExecutionLogEntryFunc.nextHook()(v0, v1, v2, v3)
	m.UpdateExecutionLogEntryFunc.appendCall(ExecutionLogEntryStoreUpdateExecutionLogEntryFuncCall{v0, v1, v2, v3, r0})
	return r0
}

// SetDefaultHook sets function that is called when the
// UpdateExecutionLogEntry method of the parent MockExecutionLogEntryStore
// instance is invoked and the hook queue is empty.
func (f *ExecutionLogEntryStoreUpdateExecutionLogEntryFunc) SetDefaultHook(hook func(context.Context, types.Job, int, executor.ExecutionLogEntry) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// UpdateExecutionLogEntry method of the parent MockExecutionLogEntryStore
// instance invokes the hook at the front of the queue and discards it.
// After the queue is empty, the default hook function is invoked for any
// future action.
func (f *ExecutionLogEntryStoreUpdateExecutionLogEntryFunc) PushHook(hook func(context.Context, types.Job, int, executor.ExecutionLogEntry) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *ExecutionLogEntryStoreUpdateExecutionLogEntryFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, types.Job, int, executor.ExecutionLogEntry) error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *ExecutionLogEntryStoreUpdateExecutionLogEntryFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, types.Job, int, executor.ExecutionLogEntry) error {
		return r0
	})
}

func (f *ExecutionLogEntryStoreUpdateExecutionLogEntryFunc) nextHook() func(context.Context, types.Job, int, executor.ExecutionLogEntry) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *ExecutionLogEntryStoreUpdateExecutionLogEntryFunc) appendCall(r0 ExecutionLogEntryStoreUpdateExecutionLogEntryFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of
// ExecutionLogEntryStoreUpdateExecutionLogEntryFuncCall objects describing
// the invocations of this function.
func (f *ExecutionLogEntryStoreUpdateExecutionLogEntryFunc) History() []ExecutionLogEntryStoreUpdateExecutionLogEntryFuncCall {
	f.mutex.Lock()
	history := make([]ExecutionLogEntryStoreUpdateExecutionLogEntryFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// ExecutionLogEntryStoreUpdateExecutionLogEntryFuncCall is an object that
// describes an invocation of method UpdateExecutionLogEntry on an instance
// of MockExecutionLogEntryStore.
type ExecutionLogEntryStoreUpdateExecutionLogEntryFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 types.Job
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 int
	// Arg3 is the value of the 4th argument passed to this method
	// invocation.
	Arg3 executor.ExecutionLogEntry
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c ExecutionLogEntryStoreUpdateExecutionLogEntryFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c ExecutionLogEntryStoreUpdateExecutionLogEntryFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// MockLogEntry is a mock implementation of the LogEntry interface (from the
// package
// github.com/sourcegraph/sourcegraph/enterprise/cmd/executor/internal/worker/cmdlogger)
// used for unit testing.
type MockLogEntry struct {
	// CloseFunc is an instance of a mock function object controlling the
	// behavior of the method Close.
	CloseFunc *LogEntryCloseFunc
	// CurrentLogEntryFunc is an instance of a mock function object
	// controlling the behavior of the method CurrentLogEntry.
	CurrentLogEntryFunc *LogEntryCurrentLogEntryFunc
	// FinalizeFunc is an instance of a mock function object controlling the
	// behavior of the method Finalize.
	FinalizeFunc *LogEntryFinalizeFunc
	// WriteFunc is an instance of a mock function object controlling the
	// behavior of the method Write.
	WriteFunc *LogEntryWriteFunc
}

// NewMockLogEntry creates a new mock of the LogEntry interface. All methods
// return zero values for all results, unless overwritten.
func NewMockLogEntry() *MockLogEntry {
	return &MockLogEntry{
		CloseFunc: &LogEntryCloseFunc{
			defaultHook: func() (r0 error) {
				return
			},
		},
		CurrentLogEntryFunc: &LogEntryCurrentLogEntryFunc{
			defaultHook: func() (r0 executor.ExecutionLogEntry) {
				return
			},
		},
		FinalizeFunc: &LogEntryFinalizeFunc{
			defaultHook: func(int) {
				return
			},
		},
		WriteFunc: &LogEntryWriteFunc{
			defaultHook: func([]byte) (r0 int, r1 error) {
				return
			},
		},
	}
}

// NewStrictMockLogEntry creates a new mock of the LogEntry interface. All
// methods panic on invocation, unless overwritten.
func NewStrictMockLogEntry() *MockLogEntry {
	return &MockLogEntry{
		CloseFunc: &LogEntryCloseFunc{
			defaultHook: func() error {
				panic("unexpected invocation of MockLogEntry.Close")
			},
		},
		CurrentLogEntryFunc: &LogEntryCurrentLogEntryFunc{
			defaultHook: func() executor.ExecutionLogEntry {
				panic("unexpected invocation of MockLogEntry.CurrentLogEntry")
			},
		},
		FinalizeFunc: &LogEntryFinalizeFunc{
			defaultHook: func(int) {
				panic("unexpected invocation of MockLogEntry.Finalize")
			},
		},
		WriteFunc: &LogEntryWriteFunc{
			defaultHook: func([]byte) (int, error) {
				panic("unexpected invocation of MockLogEntry.Write")
			},
		},
	}
}

// NewMockLogEntryFrom creates a new mock of the MockLogEntry interface. All
// methods delegate to the given implementation, unless overwritten.
func NewMockLogEntryFrom(i LogEntry) *MockLogEntry {
	return &MockLogEntry{
		CloseFunc: &LogEntryCloseFunc{
			defaultHook: i.Close,
		},
		CurrentLogEntryFunc: &LogEntryCurrentLogEntryFunc{
			defaultHook: i.CurrentLogEntry,
		},
		FinalizeFunc: &LogEntryFinalizeFunc{
			defaultHook: i.Finalize,
		},
		WriteFunc: &LogEntryWriteFunc{
			defaultHook: i.Write,
		},
	}
}

// LogEntryCloseFunc describes the behavior when the Close method of the
// parent MockLogEntry instance is invoked.
type LogEntryCloseFunc struct {
	defaultHook func() error
	hooks       []func() error
	history     []LogEntryCloseFuncCall
	mutex       sync.Mutex
}

// Close delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockLogEntry) Close() error {
	r0 := m.CloseFunc.nextHook()()
	m.CloseFunc.appendCall(LogEntryCloseFuncCall{r0})
	return r0
}

// SetDefaultHook sets function that is called when the Close method of the
// parent MockLogEntry instance is invoked and the hook queue is empty.
func (f *LogEntryCloseFunc) SetDefaultHook(hook func() error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Close method of the parent MockLogEntry instance invokes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *LogEntryCloseFunc) PushHook(hook func() error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *LogEntryCloseFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func() error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *LogEntryCloseFunc) PushReturn(r0 error) {
	f.PushHook(func() error {
		return r0
	})
}

func (f *LogEntryCloseFunc) nextHook() func() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *LogEntryCloseFunc) appendCall(r0 LogEntryCloseFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of LogEntryCloseFuncCall objects describing
// the invocations of this function.
func (f *LogEntryCloseFunc) History() []LogEntryCloseFuncCall {
	f.mutex.Lock()
	history := make([]LogEntryCloseFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// LogEntryCloseFuncCall is an object that describes an invocation of method
// Close on an instance of MockLogEntry.
type LogEntryCloseFuncCall struct {
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c LogEntryCloseFuncCall) Args() []interface{} {
	return []interface{}{}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c LogEntryCloseFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// LogEntryCurrentLogEntryFunc describes the behavior when the
// CurrentLogEntry method of the parent MockLogEntry instance is invoked.
type LogEntryCurrentLogEntryFunc struct {
	defaultHook func() executor.ExecutionLogEntry
	hooks       []func() executor.ExecutionLogEntry
	history     []LogEntryCurrentLogEntryFuncCall
	mutex       sync.Mutex
}

// CurrentLogEntry delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockLogEntry) CurrentLogEntry() executor.ExecutionLogEntry {
	r0 := m.CurrentLogEntryFunc.nextHook()()
	m.CurrentLogEntryFunc.appendCall(LogEntryCurrentLogEntryFuncCall{r0})
	return r0
}

// SetDefaultHook sets function that is called when the CurrentLogEntry
// method of the parent MockLogEntry instance is invoked and the hook queue
// is empty.
func (f *LogEntryCurrentLogEntryFunc) SetDefaultHook(hook func() executor.ExecutionLogEntry) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// CurrentLogEntry method of the parent MockLogEntry instance invokes the
// hook at the front of the queue and discards it. After the queue is empty,
// the default hook function is invoked for any future action.
func (f *LogEntryCurrentLogEntryFunc) PushHook(hook func() executor.ExecutionLogEntry) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *LogEntryCurrentLogEntryFunc) SetDefaultReturn(r0 executor.ExecutionLogEntry) {
	f.SetDefaultHook(func() executor.ExecutionLogEntry {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *LogEntryCurrentLogEntryFunc) PushReturn(r0 executor.ExecutionLogEntry) {
	f.PushHook(func() executor.ExecutionLogEntry {
		return r0
	})
}

func (f *LogEntryCurrentLogEntryFunc) nextHook() func() executor.ExecutionLogEntry {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *LogEntryCurrentLogEntryFunc) appendCall(r0 LogEntryCurrentLogEntryFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of LogEntryCurrentLogEntryFuncCall objects
// describing the invocations of this function.
func (f *LogEntryCurrentLogEntryFunc) History() []LogEntryCurrentLogEntryFuncCall {
	f.mutex.Lock()
	history := make([]LogEntryCurrentLogEntryFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// LogEntryCurrentLogEntryFuncCall is an object that describes an invocation
// of method CurrentLogEntry on an instance of MockLogEntry.
type LogEntryCurrentLogEntryFuncCall struct {
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 executor.ExecutionLogEntry
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c LogEntryCurrentLogEntryFuncCall) Args() []interface{} {
	return []interface{}{}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c LogEntryCurrentLogEntryFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// LogEntryFinalizeFunc describes the behavior when the Finalize method of
// the parent MockLogEntry instance is invoked.
type LogEntryFinalizeFunc struct {
	defaultHook func(int)
	hooks       []func(int)
	history     []LogEntryFinalizeFuncCall
	mutex       sync.Mutex
}

// Finalize delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockLogEntry) Finalize(v0 int) {
	m.FinalizeFunc.nextHook()(v0)
	m.FinalizeFunc.appendCall(LogEntryFinalizeFuncCall{v0})
	return
}

// SetDefaultHook sets function that is called when the Finalize method of
// the parent MockLogEntry instance is invoked and the hook queue is empty.
func (f *LogEntryFinalizeFunc) SetDefaultHook(hook func(int)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Finalize method of the parent MockLogEntry instance invokes the hook at
// the front of the queue and discards it. After the queue is empty, the
// default hook function is invoked for any future action.
func (f *LogEntryFinalizeFunc) PushHook(hook func(int)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *LogEntryFinalizeFunc) SetDefaultReturn() {
	f.SetDefaultHook(func(int) {
		return
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *LogEntryFinalizeFunc) PushReturn() {
	f.PushHook(func(int) {
		return
	})
}

func (f *LogEntryFinalizeFunc) nextHook() func(int) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *LogEntryFinalizeFunc) appendCall(r0 LogEntryFinalizeFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of LogEntryFinalizeFuncCall objects describing
// the invocations of this function.
func (f *LogEntryFinalizeFunc) History() []LogEntryFinalizeFuncCall {
	f.mutex.Lock()
	history := make([]LogEntryFinalizeFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// LogEntryFinalizeFuncCall is an object that describes an invocation of
// method Finalize on an instance of MockLogEntry.
type LogEntryFinalizeFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 int
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c LogEntryFinalizeFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c LogEntryFinalizeFuncCall) Results() []interface{} {
	return []interface{}{}
}

// LogEntryWriteFunc describes the behavior when the Write method of the
// parent MockLogEntry instance is invoked.
type LogEntryWriteFunc struct {
	defaultHook func([]byte) (int, error)
	hooks       []func([]byte) (int, error)
	history     []LogEntryWriteFuncCall
	mutex       sync.Mutex
}

// Write delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockLogEntry) Write(v0 []byte) (int, error) {
	r0, r1 := m.WriteFunc.nextHook()(v0)
	m.WriteFunc.appendCall(LogEntryWriteFuncCall{v0, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the Write method of the
// parent MockLogEntry instance is invoked and the hook queue is empty.
func (f *LogEntryWriteFunc) SetDefaultHook(hook func([]byte) (int, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Write method of the parent MockLogEntry instance invokes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *LogEntryWriteFunc) PushHook(hook func([]byte) (int, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *LogEntryWriteFunc) SetDefaultReturn(r0 int, r1 error) {
	f.SetDefaultHook(func([]byte) (int, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *LogEntryWriteFunc) PushReturn(r0 int, r1 error) {
	f.PushHook(func([]byte) (int, error) {
		return r0, r1
	})
}

func (f *LogEntryWriteFunc) nextHook() func([]byte) (int, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *LogEntryWriteFunc) appendCall(r0 LogEntryWriteFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of LogEntryWriteFuncCall objects describing
// the invocations of this function.
func (f *LogEntryWriteFunc) History() []LogEntryWriteFuncCall {
	f.mutex.Lock()
	history := make([]LogEntryWriteFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// LogEntryWriteFuncCall is an object that describes an invocation of method
// Write on an instance of MockLogEntry.
type LogEntryWriteFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 []byte
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 int
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c LogEntryWriteFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c LogEntryWriteFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// MockLogger is a mock implementation of the Logger interface (from the
// package
// github.com/sourcegraph/sourcegraph/enterprise/cmd/executor/internal/worker/cmdlogger)
// used for unit testing.
type MockLogger struct {
	// FlushFunc is an instance of a mock function object controlling the
	// behavior of the method Flush.
	FlushFunc *LoggerFlushFunc
	// LogEntryFunc is an instance of a mock function object controlling the
	// behavior of the method LogEntry.
	LogEntryFunc *LoggerLogEntryFunc
}

// NewMockLogger creates a new mock of the Logger interface. All methods
// return zero values for all results, unless overwritten.
func NewMockLogger() *MockLogger {
	return &MockLogger{
		FlushFunc: &LoggerFlushFunc{
			defaultHook: func() (r0 error) {
				return
			},
		},
		LogEntryFunc: &LoggerLogEntryFunc{
			defaultHook: func(string, []string) (r0 LogEntry) {
				return
			},
		},
	}
}

// NewStrictMockLogger creates a new mock of the Logger interface. All
// methods panic on invocation, unless overwritten.
func NewStrictMockLogger() *MockLogger {
	return &MockLogger{
		FlushFunc: &LoggerFlushFunc{
			defaultHook: func() error {
				panic("unexpected invocation of MockLogger.Flush")
			},
		},
		LogEntryFunc: &LoggerLogEntryFunc{
			defaultHook: func(string, []string) LogEntry {
				panic("unexpected invocation of MockLogger.LogEntry")
			},
		},
	}
}

// NewMockLoggerFrom creates a new mock of the MockLogger interface. All
// methods delegate to the given implementation, unless overwritten.
func NewMockLoggerFrom(i Logger) *MockLogger {
	return &MockLogger{
		FlushFunc: &LoggerFlushFunc{
			defaultHook: i.Flush,
		},
		LogEntryFunc: &LoggerLogEntryFunc{
			defaultHook: i.LogEntry,
		},
	}
}

// LoggerFlushFunc describes the behavior when the Flush method of the
// parent MockLogger instance is invoked.
type LoggerFlushFunc struct {
	defaultHook func() error
	hooks       []func() error
	history     []LoggerFlushFuncCall
	mutex       sync.Mutex
}

// Flush delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockLogger) Flush() error {
	r0 := m.FlushFunc.nextHook()()
	m.FlushFunc.appendCall(LoggerFlushFuncCall{r0})
	return r0
}

// SetDefaultHook sets function that is called when the Flush method of the
// parent MockLogger instance is invoked and the hook queue is empty.
func (f *LoggerFlushFunc) SetDefaultHook(hook func() error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Flush method of the parent MockLogger instance invokes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *LoggerFlushFunc) PushHook(hook func() error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *LoggerFlushFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func() error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *LoggerFlushFunc) PushReturn(r0 error) {
	f.PushHook(func() error {
		return r0
	})
}

func (f *LoggerFlushFunc) nextHook() func() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *LoggerFlushFunc) appendCall(r0 LoggerFlushFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of LoggerFlushFuncCall objects describing the
// invocations of this function.
func (f *LoggerFlushFunc) History() []LoggerFlushFuncCall {
	f.mutex.Lock()
	history := make([]LoggerFlushFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// LoggerFlushFuncCall is an object that describes an invocation of method
// Flush on an instance of MockLogger.
type LoggerFlushFuncCall struct {
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c LoggerFlushFuncCall) Args() []interface{} {
	return []interface{}{}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c LoggerFlushFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// LoggerLogEntryFunc describes the behavior when the LogEntry method of the
// parent MockLogger instance is invoked.
type LoggerLogEntryFunc struct {
	defaultHook func(string, []string) LogEntry
	hooks       []func(string, []string) LogEntry
	history     []LoggerLogEntryFuncCall
	mutex       sync.Mutex
}

// LogEntry delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockLogger) LogEntry(v0 string, v1 []string) LogEntry {
	r0 := m.LogEntryFunc.nextHook()(v0, v1)
	m.LogEntryFunc.appendCall(LoggerLogEntryFuncCall{v0, v1, r0})
	return r0
}

// SetDefaultHook sets function that is called when the LogEntry method of
// the parent MockLogger instance is invoked and the hook queue is empty.
func (f *LoggerLogEntryFunc) SetDefaultHook(hook func(string, []string) LogEntry) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// LogEntry method of the parent MockLogger instance invokes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *LoggerLogEntryFunc) PushHook(hook func(string, []string) LogEntry) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *LoggerLogEntryFunc) SetDefaultReturn(r0 LogEntry) {
	f.SetDefaultHook(func(string, []string) LogEntry {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *LoggerLogEntryFunc) PushReturn(r0 LogEntry) {
	f.PushHook(func(string, []string) LogEntry {
		return r0
	})
}

func (f *LoggerLogEntryFunc) nextHook() func(string, []string) LogEntry {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *LoggerLogEntryFunc) appendCall(r0 LoggerLogEntryFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of LoggerLogEntryFuncCall objects describing
// the invocations of this function.
func (f *LoggerLogEntryFunc) History() []LoggerLogEntryFuncCall {
	f.mutex.Lock()
	history := make([]LoggerLogEntryFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// LoggerLogEntryFuncCall is an object that describes an invocation of
// method LogEntry on an instance of MockLogger.
type LoggerLogEntryFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 string
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 []string
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 LogEntry
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c LoggerLogEntryFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c LoggerLogEntryFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}
