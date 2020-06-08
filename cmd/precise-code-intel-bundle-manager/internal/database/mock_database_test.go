// Code generated by github.com/efritz/go-mockgen 0.1.0; DO NOT EDIT.

package database

import (
	"context"
	client "github.com/sourcegraph/sourcegraph/internal/codeintel/bundles/client"
	"sync"
)

// MockDatabase is a mock implementation of the Database interface (from the
// package
// github.com/sourcegraph/sourcegraph/cmd/precise-code-intel-bundle-manager/internal/database)
// used for unit testing.
type MockDatabase struct {
	// CloseFunc is an instance of a mock function object controlling the
	// behavior of the method Close.
	CloseFunc *DatabaseCloseFunc
	// DefinitionsFunc is an instance of a mock function object controlling
	// the behavior of the method Definitions.
	DefinitionsFunc *DatabaseDefinitionsFunc
	// ExistsFunc is an instance of a mock function object controlling the
	// behavior of the method Exists.
	ExistsFunc *DatabaseExistsFunc
	// HoverFunc is an instance of a mock function object controlling the
	// behavior of the method Hover.
	HoverFunc *DatabaseHoverFunc
	// MonikerResultsFunc is an instance of a mock function object
	// controlling the behavior of the method MonikerResults.
	MonikerResultsFunc *DatabaseMonikerResultsFunc
	// MonikersByPositionFunc is an instance of a mock function object
	// controlling the behavior of the method MonikersByPosition.
	MonikersByPositionFunc *DatabaseMonikersByPositionFunc
	// PackageInformationFunc is an instance of a mock function object
	// controlling the behavior of the method PackageInformation.
	PackageInformationFunc *DatabasePackageInformationFunc
	// ReferencesFunc is an instance of a mock function object controlling
	// the behavior of the method References.
	ReferencesFunc *DatabaseReferencesFunc
}

// NewMockDatabase creates a new mock of the Database interface. All methods
// return zero values for all results, unless overwritten.
func NewMockDatabase() *MockDatabase {
	return &MockDatabase{
		CloseFunc: &DatabaseCloseFunc{
			defaultHook: func() error {
				return nil
			},
		},
		DefinitionsFunc: &DatabaseDefinitionsFunc{
			defaultHook: func(context.Context, string, int, int) ([]client.Location, error) {
				return nil, nil
			},
		},
		ExistsFunc: &DatabaseExistsFunc{
			defaultHook: func(context.Context, string) (bool, error) {
				return false, nil
			},
		},
		HoverFunc: &DatabaseHoverFunc{
			defaultHook: func(context.Context, string, int, int) (string, client.Range, bool, error) {
				return "", client.Range{}, false, nil
			},
		},
		MonikerResultsFunc: &DatabaseMonikerResultsFunc{
			defaultHook: func(context.Context, string, string, string, int, int) ([]client.Location, int, error) {
				return nil, 0, nil
			},
		},
		MonikersByPositionFunc: &DatabaseMonikersByPositionFunc{
			defaultHook: func(context.Context, string, int, int) ([][]client.MonikerData, error) {
				return nil, nil
			},
		},
		PackageInformationFunc: &DatabasePackageInformationFunc{
			defaultHook: func(context.Context, string, string) (client.PackageInformationData, bool, error) {
				return client.PackageInformationData{}, false, nil
			},
		},
		ReferencesFunc: &DatabaseReferencesFunc{
			defaultHook: func(context.Context, string, int, int) ([]client.Location, error) {
				return nil, nil
			},
		},
	}
}

// NewMockDatabaseFrom creates a new mock of the MockDatabase interface. All
// methods delegate to the given implementation, unless overwritten.
func NewMockDatabaseFrom(i Database) *MockDatabase {
	return &MockDatabase{
		CloseFunc: &DatabaseCloseFunc{
			defaultHook: i.Close,
		},
		DefinitionsFunc: &DatabaseDefinitionsFunc{
			defaultHook: i.Definitions,
		},
		ExistsFunc: &DatabaseExistsFunc{
			defaultHook: i.Exists,
		},
		HoverFunc: &DatabaseHoverFunc{
			defaultHook: i.Hover,
		},
		MonikerResultsFunc: &DatabaseMonikerResultsFunc{
			defaultHook: i.MonikerResults,
		},
		MonikersByPositionFunc: &DatabaseMonikersByPositionFunc{
			defaultHook: i.MonikersByPosition,
		},
		PackageInformationFunc: &DatabasePackageInformationFunc{
			defaultHook: i.PackageInformation,
		},
		ReferencesFunc: &DatabaseReferencesFunc{
			defaultHook: i.References,
		},
	}
}

// DatabaseCloseFunc describes the behavior when the Close method of the
// parent MockDatabase instance is invoked.
type DatabaseCloseFunc struct {
	defaultHook func() error
	hooks       []func() error
	history     []DatabaseCloseFuncCall
	mutex       sync.Mutex
}

// Close delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockDatabase) Close() error {
	r0 := m.CloseFunc.nextHook()()
	m.CloseFunc.appendCall(DatabaseCloseFuncCall{r0})
	return r0
}

// SetDefaultHook sets function that is called when the Close method of the
// parent MockDatabase instance is invoked and the hook queue is empty.
func (f *DatabaseCloseFunc) SetDefaultHook(hook func() error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Close method of the parent MockDatabase instance inovkes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *DatabaseCloseFunc) PushHook(hook func() error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *DatabaseCloseFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func() error {
		return r0
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *DatabaseCloseFunc) PushReturn(r0 error) {
	f.PushHook(func() error {
		return r0
	})
}

func (f *DatabaseCloseFunc) nextHook() func() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *DatabaseCloseFunc) appendCall(r0 DatabaseCloseFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of DatabaseCloseFuncCall objects describing
// the invocations of this function.
func (f *DatabaseCloseFunc) History() []DatabaseCloseFuncCall {
	f.mutex.Lock()
	history := make([]DatabaseCloseFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// DatabaseCloseFuncCall is an object that describes an invocation of method
// Close on an instance of MockDatabase.
type DatabaseCloseFuncCall struct {
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c DatabaseCloseFuncCall) Args() []interface{} {
	return []interface{}{}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c DatabaseCloseFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// DatabaseDefinitionsFunc describes the behavior when the Definitions
// method of the parent MockDatabase instance is invoked.
type DatabaseDefinitionsFunc struct {
	defaultHook func(context.Context, string, int, int) ([]client.Location, error)
	hooks       []func(context.Context, string, int, int) ([]client.Location, error)
	history     []DatabaseDefinitionsFuncCall
	mutex       sync.Mutex
}

// Definitions delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockDatabase) Definitions(v0 context.Context, v1 string, v2 int, v3 int) ([]client.Location, error) {
	r0, r1 := m.DefinitionsFunc.nextHook()(v0, v1, v2, v3)
	m.DefinitionsFunc.appendCall(DatabaseDefinitionsFuncCall{v0, v1, v2, v3, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the Definitions method
// of the parent MockDatabase instance is invoked and the hook queue is
// empty.
func (f *DatabaseDefinitionsFunc) SetDefaultHook(hook func(context.Context, string, int, int) ([]client.Location, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Definitions method of the parent MockDatabase instance inovkes the hook
// at the front of the queue and discards it. After the queue is empty, the
// default hook function is invoked for any future action.
func (f *DatabaseDefinitionsFunc) PushHook(hook func(context.Context, string, int, int) ([]client.Location, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *DatabaseDefinitionsFunc) SetDefaultReturn(r0 []client.Location, r1 error) {
	f.SetDefaultHook(func(context.Context, string, int, int) ([]client.Location, error) {
		return r0, r1
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *DatabaseDefinitionsFunc) PushReturn(r0 []client.Location, r1 error) {
	f.PushHook(func(context.Context, string, int, int) ([]client.Location, error) {
		return r0, r1
	})
}

func (f *DatabaseDefinitionsFunc) nextHook() func(context.Context, string, int, int) ([]client.Location, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *DatabaseDefinitionsFunc) appendCall(r0 DatabaseDefinitionsFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of DatabaseDefinitionsFuncCall objects
// describing the invocations of this function.
func (f *DatabaseDefinitionsFunc) History() []DatabaseDefinitionsFuncCall {
	f.mutex.Lock()
	history := make([]DatabaseDefinitionsFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// DatabaseDefinitionsFuncCall is an object that describes an invocation of
// method Definitions on an instance of MockDatabase.
type DatabaseDefinitionsFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 string
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 int
	// Arg3 is the value of the 4th argument passed to this method
	// invocation.
	Arg3 int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []client.Location
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c DatabaseDefinitionsFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c DatabaseDefinitionsFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// DatabaseExistsFunc describes the behavior when the Exists method of the
// parent MockDatabase instance is invoked.
type DatabaseExistsFunc struct {
	defaultHook func(context.Context, string) (bool, error)
	hooks       []func(context.Context, string) (bool, error)
	history     []DatabaseExistsFuncCall
	mutex       sync.Mutex
}

// Exists delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockDatabase) Exists(v0 context.Context, v1 string) (bool, error) {
	r0, r1 := m.ExistsFunc.nextHook()(v0, v1)
	m.ExistsFunc.appendCall(DatabaseExistsFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the Exists method of the
// parent MockDatabase instance is invoked and the hook queue is empty.
func (f *DatabaseExistsFunc) SetDefaultHook(hook func(context.Context, string) (bool, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Exists method of the parent MockDatabase instance inovkes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *DatabaseExistsFunc) PushHook(hook func(context.Context, string) (bool, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *DatabaseExistsFunc) SetDefaultReturn(r0 bool, r1 error) {
	f.SetDefaultHook(func(context.Context, string) (bool, error) {
		return r0, r1
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *DatabaseExistsFunc) PushReturn(r0 bool, r1 error) {
	f.PushHook(func(context.Context, string) (bool, error) {
		return r0, r1
	})
}

func (f *DatabaseExistsFunc) nextHook() func(context.Context, string) (bool, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *DatabaseExistsFunc) appendCall(r0 DatabaseExistsFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of DatabaseExistsFuncCall objects describing
// the invocations of this function.
func (f *DatabaseExistsFunc) History() []DatabaseExistsFuncCall {
	f.mutex.Lock()
	history := make([]DatabaseExistsFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// DatabaseExistsFuncCall is an object that describes an invocation of
// method Exists on an instance of MockDatabase.
type DatabaseExistsFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 string
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 bool
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c DatabaseExistsFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c DatabaseExistsFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// DatabaseHoverFunc describes the behavior when the Hover method of the
// parent MockDatabase instance is invoked.
type DatabaseHoverFunc struct {
	defaultHook func(context.Context, string, int, int) (string, client.Range, bool, error)
	hooks       []func(context.Context, string, int, int) (string, client.Range, bool, error)
	history     []DatabaseHoverFuncCall
	mutex       sync.Mutex
}

// Hover delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockDatabase) Hover(v0 context.Context, v1 string, v2 int, v3 int) (string, client.Range, bool, error) {
	r0, r1, r2, r3 := m.HoverFunc.nextHook()(v0, v1, v2, v3)
	m.HoverFunc.appendCall(DatabaseHoverFuncCall{v0, v1, v2, v3, r0, r1, r2, r3})
	return r0, r1, r2, r3
}

// SetDefaultHook sets function that is called when the Hover method of the
// parent MockDatabase instance is invoked and the hook queue is empty.
func (f *DatabaseHoverFunc) SetDefaultHook(hook func(context.Context, string, int, int) (string, client.Range, bool, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Hover method of the parent MockDatabase instance inovkes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *DatabaseHoverFunc) PushHook(hook func(context.Context, string, int, int) (string, client.Range, bool, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *DatabaseHoverFunc) SetDefaultReturn(r0 string, r1 client.Range, r2 bool, r3 error) {
	f.SetDefaultHook(func(context.Context, string, int, int) (string, client.Range, bool, error) {
		return r0, r1, r2, r3
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *DatabaseHoverFunc) PushReturn(r0 string, r1 client.Range, r2 bool, r3 error) {
	f.PushHook(func(context.Context, string, int, int) (string, client.Range, bool, error) {
		return r0, r1, r2, r3
	})
}

func (f *DatabaseHoverFunc) nextHook() func(context.Context, string, int, int) (string, client.Range, bool, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *DatabaseHoverFunc) appendCall(r0 DatabaseHoverFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of DatabaseHoverFuncCall objects describing
// the invocations of this function.
func (f *DatabaseHoverFunc) History() []DatabaseHoverFuncCall {
	f.mutex.Lock()
	history := make([]DatabaseHoverFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// DatabaseHoverFuncCall is an object that describes an invocation of method
// Hover on an instance of MockDatabase.
type DatabaseHoverFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 string
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 int
	// Arg3 is the value of the 4th argument passed to this method
	// invocation.
	Arg3 int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 string
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 client.Range
	// Result2 is the value of the 3rd result returned from this method
	// invocation.
	Result2 bool
	// Result3 is the value of the 4th result returned from this method
	// invocation.
	Result3 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c DatabaseHoverFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c DatabaseHoverFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1, c.Result2, c.Result3}
}

// DatabaseMonikerResultsFunc describes the behavior when the MonikerResults
// method of the parent MockDatabase instance is invoked.
type DatabaseMonikerResultsFunc struct {
	defaultHook func(context.Context, string, string, string, int, int) ([]client.Location, int, error)
	hooks       []func(context.Context, string, string, string, int, int) ([]client.Location, int, error)
	history     []DatabaseMonikerResultsFuncCall
	mutex       sync.Mutex
}

// MonikerResults delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockDatabase) MonikerResults(v0 context.Context, v1 string, v2 string, v3 string, v4 int, v5 int) ([]client.Location, int, error) {
	r0, r1, r2 := m.MonikerResultsFunc.nextHook()(v0, v1, v2, v3, v4, v5)
	m.MonikerResultsFunc.appendCall(DatabaseMonikerResultsFuncCall{v0, v1, v2, v3, v4, v5, r0, r1, r2})
	return r0, r1, r2
}

// SetDefaultHook sets function that is called when the MonikerResults
// method of the parent MockDatabase instance is invoked and the hook queue
// is empty.
func (f *DatabaseMonikerResultsFunc) SetDefaultHook(hook func(context.Context, string, string, string, int, int) ([]client.Location, int, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// MonikerResults method of the parent MockDatabase instance inovkes the
// hook at the front of the queue and discards it. After the queue is empty,
// the default hook function is invoked for any future action.
func (f *DatabaseMonikerResultsFunc) PushHook(hook func(context.Context, string, string, string, int, int) ([]client.Location, int, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *DatabaseMonikerResultsFunc) SetDefaultReturn(r0 []client.Location, r1 int, r2 error) {
	f.SetDefaultHook(func(context.Context, string, string, string, int, int) ([]client.Location, int, error) {
		return r0, r1, r2
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *DatabaseMonikerResultsFunc) PushReturn(r0 []client.Location, r1 int, r2 error) {
	f.PushHook(func(context.Context, string, string, string, int, int) ([]client.Location, int, error) {
		return r0, r1, r2
	})
}

func (f *DatabaseMonikerResultsFunc) nextHook() func(context.Context, string, string, string, int, int) ([]client.Location, int, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *DatabaseMonikerResultsFunc) appendCall(r0 DatabaseMonikerResultsFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of DatabaseMonikerResultsFuncCall objects
// describing the invocations of this function.
func (f *DatabaseMonikerResultsFunc) History() []DatabaseMonikerResultsFuncCall {
	f.mutex.Lock()
	history := make([]DatabaseMonikerResultsFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// DatabaseMonikerResultsFuncCall is an object that describes an invocation
// of method MonikerResults on an instance of MockDatabase.
type DatabaseMonikerResultsFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 string
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 string
	// Arg3 is the value of the 4th argument passed to this method
	// invocation.
	Arg3 string
	// Arg4 is the value of the 5th argument passed to this method
	// invocation.
	Arg4 int
	// Arg5 is the value of the 6th argument passed to this method
	// invocation.
	Arg5 int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []client.Location
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 int
	// Result2 is the value of the 3rd result returned from this method
	// invocation.
	Result2 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c DatabaseMonikerResultsFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3, c.Arg4, c.Arg5}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c DatabaseMonikerResultsFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1, c.Result2}
}

// DatabaseMonikersByPositionFunc describes the behavior when the
// MonikersByPosition method of the parent MockDatabase instance is invoked.
type DatabaseMonikersByPositionFunc struct {
	defaultHook func(context.Context, string, int, int) ([][]client.MonikerData, error)
	hooks       []func(context.Context, string, int, int) ([][]client.MonikerData, error)
	history     []DatabaseMonikersByPositionFuncCall
	mutex       sync.Mutex
}

// MonikersByPosition delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockDatabase) MonikersByPosition(v0 context.Context, v1 string, v2 int, v3 int) ([][]client.MonikerData, error) {
	r0, r1 := m.MonikersByPositionFunc.nextHook()(v0, v1, v2, v3)
	m.MonikersByPositionFunc.appendCall(DatabaseMonikersByPositionFuncCall{v0, v1, v2, v3, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the MonikersByPosition
// method of the parent MockDatabase instance is invoked and the hook queue
// is empty.
func (f *DatabaseMonikersByPositionFunc) SetDefaultHook(hook func(context.Context, string, int, int) ([][]client.MonikerData, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// MonikersByPosition method of the parent MockDatabase instance inovkes the
// hook at the front of the queue and discards it. After the queue is empty,
// the default hook function is invoked for any future action.
func (f *DatabaseMonikersByPositionFunc) PushHook(hook func(context.Context, string, int, int) ([][]client.MonikerData, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *DatabaseMonikersByPositionFunc) SetDefaultReturn(r0 [][]client.MonikerData, r1 error) {
	f.SetDefaultHook(func(context.Context, string, int, int) ([][]client.MonikerData, error) {
		return r0, r1
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *DatabaseMonikersByPositionFunc) PushReturn(r0 [][]client.MonikerData, r1 error) {
	f.PushHook(func(context.Context, string, int, int) ([][]client.MonikerData, error) {
		return r0, r1
	})
}

func (f *DatabaseMonikersByPositionFunc) nextHook() func(context.Context, string, int, int) ([][]client.MonikerData, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *DatabaseMonikersByPositionFunc) appendCall(r0 DatabaseMonikersByPositionFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of DatabaseMonikersByPositionFuncCall objects
// describing the invocations of this function.
func (f *DatabaseMonikersByPositionFunc) History() []DatabaseMonikersByPositionFuncCall {
	f.mutex.Lock()
	history := make([]DatabaseMonikersByPositionFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// DatabaseMonikersByPositionFuncCall is an object that describes an
// invocation of method MonikersByPosition on an instance of MockDatabase.
type DatabaseMonikersByPositionFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 string
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 int
	// Arg3 is the value of the 4th argument passed to this method
	// invocation.
	Arg3 int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 [][]client.MonikerData
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c DatabaseMonikersByPositionFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c DatabaseMonikersByPositionFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// DatabasePackageInformationFunc describes the behavior when the
// PackageInformation method of the parent MockDatabase instance is invoked.
type DatabasePackageInformationFunc struct {
	defaultHook func(context.Context, string, string) (client.PackageInformationData, bool, error)
	hooks       []func(context.Context, string, string) (client.PackageInformationData, bool, error)
	history     []DatabasePackageInformationFuncCall
	mutex       sync.Mutex
}

// PackageInformation delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockDatabase) PackageInformation(v0 context.Context, v1 string, v2 string) (client.PackageInformationData, bool, error) {
	r0, r1, r2 := m.PackageInformationFunc.nextHook()(v0, v1, v2)
	m.PackageInformationFunc.appendCall(DatabasePackageInformationFuncCall{v0, v1, v2, r0, r1, r2})
	return r0, r1, r2
}

// SetDefaultHook sets function that is called when the PackageInformation
// method of the parent MockDatabase instance is invoked and the hook queue
// is empty.
func (f *DatabasePackageInformationFunc) SetDefaultHook(hook func(context.Context, string, string) (client.PackageInformationData, bool, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// PackageInformation method of the parent MockDatabase instance inovkes the
// hook at the front of the queue and discards it. After the queue is empty,
// the default hook function is invoked for any future action.
func (f *DatabasePackageInformationFunc) PushHook(hook func(context.Context, string, string) (client.PackageInformationData, bool, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *DatabasePackageInformationFunc) SetDefaultReturn(r0 client.PackageInformationData, r1 bool, r2 error) {
	f.SetDefaultHook(func(context.Context, string, string) (client.PackageInformationData, bool, error) {
		return r0, r1, r2
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *DatabasePackageInformationFunc) PushReturn(r0 client.PackageInformationData, r1 bool, r2 error) {
	f.PushHook(func(context.Context, string, string) (client.PackageInformationData, bool, error) {
		return r0, r1, r2
	})
}

func (f *DatabasePackageInformationFunc) nextHook() func(context.Context, string, string) (client.PackageInformationData, bool, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *DatabasePackageInformationFunc) appendCall(r0 DatabasePackageInformationFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of DatabasePackageInformationFuncCall objects
// describing the invocations of this function.
func (f *DatabasePackageInformationFunc) History() []DatabasePackageInformationFuncCall {
	f.mutex.Lock()
	history := make([]DatabasePackageInformationFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// DatabasePackageInformationFuncCall is an object that describes an
// invocation of method PackageInformation on an instance of MockDatabase.
type DatabasePackageInformationFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 string
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 string
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 client.PackageInformationData
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 bool
	// Result2 is the value of the 3rd result returned from this method
	// invocation.
	Result2 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c DatabasePackageInformationFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c DatabasePackageInformationFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1, c.Result2}
}

// DatabaseReferencesFunc describes the behavior when the References method
// of the parent MockDatabase instance is invoked.
type DatabaseReferencesFunc struct {
	defaultHook func(context.Context, string, int, int) ([]client.Location, error)
	hooks       []func(context.Context, string, int, int) ([]client.Location, error)
	history     []DatabaseReferencesFuncCall
	mutex       sync.Mutex
}

// References delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockDatabase) References(v0 context.Context, v1 string, v2 int, v3 int) ([]client.Location, error) {
	r0, r1 := m.ReferencesFunc.nextHook()(v0, v1, v2, v3)
	m.ReferencesFunc.appendCall(DatabaseReferencesFuncCall{v0, v1, v2, v3, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the References method of
// the parent MockDatabase instance is invoked and the hook queue is empty.
func (f *DatabaseReferencesFunc) SetDefaultHook(hook func(context.Context, string, int, int) ([]client.Location, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// References method of the parent MockDatabase instance inovkes the hook at
// the front of the queue and discards it. After the queue is empty, the
// default hook function is invoked for any future action.
func (f *DatabaseReferencesFunc) PushHook(hook func(context.Context, string, int, int) ([]client.Location, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *DatabaseReferencesFunc) SetDefaultReturn(r0 []client.Location, r1 error) {
	f.SetDefaultHook(func(context.Context, string, int, int) ([]client.Location, error) {
		return r0, r1
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *DatabaseReferencesFunc) PushReturn(r0 []client.Location, r1 error) {
	f.PushHook(func(context.Context, string, int, int) ([]client.Location, error) {
		return r0, r1
	})
}

func (f *DatabaseReferencesFunc) nextHook() func(context.Context, string, int, int) ([]client.Location, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *DatabaseReferencesFunc) appendCall(r0 DatabaseReferencesFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of DatabaseReferencesFuncCall objects
// describing the invocations of this function.
func (f *DatabaseReferencesFunc) History() []DatabaseReferencesFuncCall {
	f.mutex.Lock()
	history := make([]DatabaseReferencesFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// DatabaseReferencesFuncCall is an object that describes an invocation of
// method References on an instance of MockDatabase.
type DatabaseReferencesFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 string
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 int
	// Arg3 is the value of the 4th argument passed to this method
	// invocation.
	Arg3 int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []client.Location
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c DatabaseReferencesFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c DatabaseReferencesFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}
