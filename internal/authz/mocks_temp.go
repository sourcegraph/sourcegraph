// Code generated by go-mockgen 1.3.7; DO NOT EDIT.
//
// This file was generated by running `sg generate` (or `go-mockgen`) at the root of
// this repository. To add additional mocks to this or another package, add a new entry
// to the mockgen.yaml file in the root of this repository.

package authz

import (
	"context"
	"sync"

	api "github.com/sourcegraph/sourcegraph/internal/api"
)

// MockSubRepoPermissionChecker is a mock implementation of the
// SubRepoPermissionChecker interface (from the package
// github.com/sourcegraph/sourcegraph/internal/authz) used for unit testing.
type MockSubRepoPermissionChecker struct {
	// EnabledFunc is an instance of a mock function object controlling the
	// behavior of the method Enabled.
	EnabledFunc *SubRepoPermissionCheckerEnabledFunc
	// EnabledForRepoFunc is an instance of a mock function object
	// controlling the behavior of the method EnabledForRepo.
	EnabledForRepoFunc *SubRepoPermissionCheckerEnabledForRepoFunc
	// EnabledForRepoIDFunc is an instance of a mock function object
	// controlling the behavior of the method EnabledForRepoID.
	EnabledForRepoIDFunc *SubRepoPermissionCheckerEnabledForRepoIDFunc
	// FilePermissionsFuncFunc is an instance of a mock function object
	// controlling the behavior of the method FilePermissionsFunc.
	FilePermissionsFuncFunc *SubRepoPermissionCheckerFilePermissionsFuncFunc
	// PermissionsFunc is an instance of a mock function object controlling
	// the behavior of the method Permissions.
	PermissionsFunc *SubRepoPermissionCheckerPermissionsFunc
}

// NewMockSubRepoPermissionChecker creates a new mock of the
// SubRepoPermissionChecker interface. All methods return zero values for
// all results, unless overwritten.
func NewMockSubRepoPermissionChecker() *MockSubRepoPermissionChecker {
	return &MockSubRepoPermissionChecker{
		EnabledFunc: &SubRepoPermissionCheckerEnabledFunc{
			defaultHook: func() (r0 bool) {
				return
			},
		},
		EnabledForRepoFunc: &SubRepoPermissionCheckerEnabledForRepoFunc{
			defaultHook: func(context.Context, api.RepoName) (r0 bool, r1 error) {
				return
			},
		},
		EnabledForRepoIDFunc: &SubRepoPermissionCheckerEnabledForRepoIDFunc{
			defaultHook: func(context.Context, api.RepoID) (r0 bool, r1 error) {
				return
			},
		},
		FilePermissionsFuncFunc: &SubRepoPermissionCheckerFilePermissionsFuncFunc{
			defaultHook: func(context.Context, int32, api.RepoName) (r0 FilePermissionFunc, r1 error) {
				return
			},
		},
		PermissionsFunc: &SubRepoPermissionCheckerPermissionsFunc{
			defaultHook: func(context.Context, int32, RepoContent) (r0 Perms, r1 error) {
				return
			},
		},
	}
}

// NewStrictMockSubRepoPermissionChecker creates a new mock of the
// SubRepoPermissionChecker interface. All methods panic on invocation,
// unless overwritten.
func NewStrictMockSubRepoPermissionChecker() *MockSubRepoPermissionChecker {
	return &MockSubRepoPermissionChecker{
		EnabledFunc: &SubRepoPermissionCheckerEnabledFunc{
			defaultHook: func() bool {
				panic("unexpected invocation of MockSubRepoPermissionChecker.Enabled")
			},
		},
		EnabledForRepoFunc: &SubRepoPermissionCheckerEnabledForRepoFunc{
			defaultHook: func(context.Context, api.RepoName) (bool, error) {
				panic("unexpected invocation of MockSubRepoPermissionChecker.EnabledForRepo")
			},
		},
		EnabledForRepoIDFunc: &SubRepoPermissionCheckerEnabledForRepoIDFunc{
			defaultHook: func(context.Context, api.RepoID) (bool, error) {
				panic("unexpected invocation of MockSubRepoPermissionChecker.EnabledForRepoID")
			},
		},
		FilePermissionsFuncFunc: &SubRepoPermissionCheckerFilePermissionsFuncFunc{
			defaultHook: func(context.Context, int32, api.RepoName) (FilePermissionFunc, error) {
				panic("unexpected invocation of MockSubRepoPermissionChecker.FilePermissionsFunc")
			},
		},
		PermissionsFunc: &SubRepoPermissionCheckerPermissionsFunc{
			defaultHook: func(context.Context, int32, RepoContent) (Perms, error) {
				panic("unexpected invocation of MockSubRepoPermissionChecker.Permissions")
			},
		},
	}
}

// NewMockSubRepoPermissionCheckerFrom creates a new mock of the
// MockSubRepoPermissionChecker interface. All methods delegate to the given
// implementation, unless overwritten.
func NewMockSubRepoPermissionCheckerFrom(i SubRepoPermissionChecker) *MockSubRepoPermissionChecker {
	return &MockSubRepoPermissionChecker{
		EnabledFunc: &SubRepoPermissionCheckerEnabledFunc{
			defaultHook: i.Enabled,
		},
		EnabledForRepoFunc: &SubRepoPermissionCheckerEnabledForRepoFunc{
			defaultHook: i.EnabledForRepo,
		},
		EnabledForRepoIDFunc: &SubRepoPermissionCheckerEnabledForRepoIDFunc{
			defaultHook: i.EnabledForRepoID,
		},
		FilePermissionsFuncFunc: &SubRepoPermissionCheckerFilePermissionsFuncFunc{
			defaultHook: i.FilePermissionsFunc,
		},
		PermissionsFunc: &SubRepoPermissionCheckerPermissionsFunc{
			defaultHook: i.Permissions,
		},
	}
}

// SubRepoPermissionCheckerEnabledFunc describes the behavior when the
// Enabled method of the parent MockSubRepoPermissionChecker instance is
// invoked.
type SubRepoPermissionCheckerEnabledFunc struct {
	defaultHook func() bool
	hooks       []func() bool
	history     []SubRepoPermissionCheckerEnabledFuncCall
	mutex       sync.Mutex
}

// Enabled delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockSubRepoPermissionChecker) Enabled() bool {
	r0 := m.EnabledFunc.nextHook()()
	m.EnabledFunc.appendCall(SubRepoPermissionCheckerEnabledFuncCall{r0})
	return r0
}

// SetDefaultHook sets function that is called when the Enabled method of
// the parent MockSubRepoPermissionChecker instance is invoked and the hook
// queue is empty.
func (f *SubRepoPermissionCheckerEnabledFunc) SetDefaultHook(hook func() bool) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Enabled method of the parent MockSubRepoPermissionChecker instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *SubRepoPermissionCheckerEnabledFunc) PushHook(hook func() bool) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *SubRepoPermissionCheckerEnabledFunc) SetDefaultReturn(r0 bool) {
	f.SetDefaultHook(func() bool {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *SubRepoPermissionCheckerEnabledFunc) PushReturn(r0 bool) {
	f.PushHook(func() bool {
		return r0
	})
}

func (f *SubRepoPermissionCheckerEnabledFunc) nextHook() func() bool {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *SubRepoPermissionCheckerEnabledFunc) appendCall(r0 SubRepoPermissionCheckerEnabledFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of SubRepoPermissionCheckerEnabledFuncCall
// objects describing the invocations of this function.
func (f *SubRepoPermissionCheckerEnabledFunc) History() []SubRepoPermissionCheckerEnabledFuncCall {
	f.mutex.Lock()
	history := make([]SubRepoPermissionCheckerEnabledFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// SubRepoPermissionCheckerEnabledFuncCall is an object that describes an
// invocation of method Enabled on an instance of
// MockSubRepoPermissionChecker.
type SubRepoPermissionCheckerEnabledFuncCall struct {
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 bool
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c SubRepoPermissionCheckerEnabledFuncCall) Args() []interface{} {
	return []interface{}{}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c SubRepoPermissionCheckerEnabledFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// SubRepoPermissionCheckerEnabledForRepoFunc describes the behavior when
// the EnabledForRepo method of the parent MockSubRepoPermissionChecker
// instance is invoked.
type SubRepoPermissionCheckerEnabledForRepoFunc struct {
	defaultHook func(context.Context, api.RepoName) (bool, error)
	hooks       []func(context.Context, api.RepoName) (bool, error)
	history     []SubRepoPermissionCheckerEnabledForRepoFuncCall
	mutex       sync.Mutex
}

// EnabledForRepo delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockSubRepoPermissionChecker) EnabledForRepo(v0 context.Context, v1 api.RepoName) (bool, error) {
	r0, r1 := m.EnabledForRepoFunc.nextHook()(v0, v1)
	m.EnabledForRepoFunc.appendCall(SubRepoPermissionCheckerEnabledForRepoFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the EnabledForRepo
// method of the parent MockSubRepoPermissionChecker instance is invoked and
// the hook queue is empty.
func (f *SubRepoPermissionCheckerEnabledForRepoFunc) SetDefaultHook(hook func(context.Context, api.RepoName) (bool, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// EnabledForRepo method of the parent MockSubRepoPermissionChecker instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *SubRepoPermissionCheckerEnabledForRepoFunc) PushHook(hook func(context.Context, api.RepoName) (bool, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *SubRepoPermissionCheckerEnabledForRepoFunc) SetDefaultReturn(r0 bool, r1 error) {
	f.SetDefaultHook(func(context.Context, api.RepoName) (bool, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *SubRepoPermissionCheckerEnabledForRepoFunc) PushReturn(r0 bool, r1 error) {
	f.PushHook(func(context.Context, api.RepoName) (bool, error) {
		return r0, r1
	})
}

func (f *SubRepoPermissionCheckerEnabledForRepoFunc) nextHook() func(context.Context, api.RepoName) (bool, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *SubRepoPermissionCheckerEnabledForRepoFunc) appendCall(r0 SubRepoPermissionCheckerEnabledForRepoFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of
// SubRepoPermissionCheckerEnabledForRepoFuncCall objects describing the
// invocations of this function.
func (f *SubRepoPermissionCheckerEnabledForRepoFunc) History() []SubRepoPermissionCheckerEnabledForRepoFuncCall {
	f.mutex.Lock()
	history := make([]SubRepoPermissionCheckerEnabledForRepoFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// SubRepoPermissionCheckerEnabledForRepoFuncCall is an object that
// describes an invocation of method EnabledForRepo on an instance of
// MockSubRepoPermissionChecker.
type SubRepoPermissionCheckerEnabledForRepoFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 api.RepoName
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 bool
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c SubRepoPermissionCheckerEnabledForRepoFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c SubRepoPermissionCheckerEnabledForRepoFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// SubRepoPermissionCheckerEnabledForRepoIDFunc describes the behavior when
// the EnabledForRepoID method of the parent MockSubRepoPermissionChecker
// instance is invoked.
type SubRepoPermissionCheckerEnabledForRepoIDFunc struct {
	defaultHook func(context.Context, api.RepoID) (bool, error)
	hooks       []func(context.Context, api.RepoID) (bool, error)
	history     []SubRepoPermissionCheckerEnabledForRepoIDFuncCall
	mutex       sync.Mutex
}

// EnabledForRepoID delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockSubRepoPermissionChecker) EnabledForRepoID(v0 context.Context, v1 api.RepoID) (bool, error) {
	r0, r1 := m.EnabledForRepoIDFunc.nextHook()(v0, v1)
	m.EnabledForRepoIDFunc.appendCall(SubRepoPermissionCheckerEnabledForRepoIDFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the EnabledForRepoID
// method of the parent MockSubRepoPermissionChecker instance is invoked and
// the hook queue is empty.
func (f *SubRepoPermissionCheckerEnabledForRepoIDFunc) SetDefaultHook(hook func(context.Context, api.RepoID) (bool, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// EnabledForRepoID method of the parent MockSubRepoPermissionChecker
// instance invokes the hook at the front of the queue and discards it.
// After the queue is empty, the default hook function is invoked for any
// future action.
func (f *SubRepoPermissionCheckerEnabledForRepoIDFunc) PushHook(hook func(context.Context, api.RepoID) (bool, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *SubRepoPermissionCheckerEnabledForRepoIDFunc) SetDefaultReturn(r0 bool, r1 error) {
	f.SetDefaultHook(func(context.Context, api.RepoID) (bool, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *SubRepoPermissionCheckerEnabledForRepoIDFunc) PushReturn(r0 bool, r1 error) {
	f.PushHook(func(context.Context, api.RepoID) (bool, error) {
		return r0, r1
	})
}

func (f *SubRepoPermissionCheckerEnabledForRepoIDFunc) nextHook() func(context.Context, api.RepoID) (bool, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *SubRepoPermissionCheckerEnabledForRepoIDFunc) appendCall(r0 SubRepoPermissionCheckerEnabledForRepoIDFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of
// SubRepoPermissionCheckerEnabledForRepoIDFuncCall objects describing the
// invocations of this function.
func (f *SubRepoPermissionCheckerEnabledForRepoIDFunc) History() []SubRepoPermissionCheckerEnabledForRepoIDFuncCall {
	f.mutex.Lock()
	history := make([]SubRepoPermissionCheckerEnabledForRepoIDFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// SubRepoPermissionCheckerEnabledForRepoIDFuncCall is an object that
// describes an invocation of method EnabledForRepoID on an instance of
// MockSubRepoPermissionChecker.
type SubRepoPermissionCheckerEnabledForRepoIDFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 api.RepoID
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 bool
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c SubRepoPermissionCheckerEnabledForRepoIDFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c SubRepoPermissionCheckerEnabledForRepoIDFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// SubRepoPermissionCheckerFilePermissionsFuncFunc describes the behavior
// when the FilePermissionsFunc method of the parent
// MockSubRepoPermissionChecker instance is invoked.
type SubRepoPermissionCheckerFilePermissionsFuncFunc struct {
	defaultHook func(context.Context, int32, api.RepoName) (FilePermissionFunc, error)
	hooks       []func(context.Context, int32, api.RepoName) (FilePermissionFunc, error)
	history     []SubRepoPermissionCheckerFilePermissionsFuncFuncCall
	mutex       sync.Mutex
}

// FilePermissionsFunc delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockSubRepoPermissionChecker) FilePermissionsFunc(v0 context.Context, v1 int32, v2 api.RepoName) (FilePermissionFunc, error) {
	r0, r1 := m.FilePermissionsFuncFunc.nextHook()(v0, v1, v2)
	m.FilePermissionsFuncFunc.appendCall(SubRepoPermissionCheckerFilePermissionsFuncFuncCall{v0, v1, v2, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the FilePermissionsFunc
// method of the parent MockSubRepoPermissionChecker instance is invoked and
// the hook queue is empty.
func (f *SubRepoPermissionCheckerFilePermissionsFuncFunc) SetDefaultHook(hook func(context.Context, int32, api.RepoName) (FilePermissionFunc, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// FilePermissionsFunc method of the parent MockSubRepoPermissionChecker
// instance invokes the hook at the front of the queue and discards it.
// After the queue is empty, the default hook function is invoked for any
// future action.
func (f *SubRepoPermissionCheckerFilePermissionsFuncFunc) PushHook(hook func(context.Context, int32, api.RepoName) (FilePermissionFunc, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *SubRepoPermissionCheckerFilePermissionsFuncFunc) SetDefaultReturn(r0 FilePermissionFunc, r1 error) {
	f.SetDefaultHook(func(context.Context, int32, api.RepoName) (FilePermissionFunc, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *SubRepoPermissionCheckerFilePermissionsFuncFunc) PushReturn(r0 FilePermissionFunc, r1 error) {
	f.PushHook(func(context.Context, int32, api.RepoName) (FilePermissionFunc, error) {
		return r0, r1
	})
}

func (f *SubRepoPermissionCheckerFilePermissionsFuncFunc) nextHook() func(context.Context, int32, api.RepoName) (FilePermissionFunc, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *SubRepoPermissionCheckerFilePermissionsFuncFunc) appendCall(r0 SubRepoPermissionCheckerFilePermissionsFuncFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of
// SubRepoPermissionCheckerFilePermissionsFuncFuncCall objects describing
// the invocations of this function.
func (f *SubRepoPermissionCheckerFilePermissionsFuncFunc) History() []SubRepoPermissionCheckerFilePermissionsFuncFuncCall {
	f.mutex.Lock()
	history := make([]SubRepoPermissionCheckerFilePermissionsFuncFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// SubRepoPermissionCheckerFilePermissionsFuncFuncCall is an object that
// describes an invocation of method FilePermissionsFunc on an instance of
// MockSubRepoPermissionChecker.
type SubRepoPermissionCheckerFilePermissionsFuncFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int32
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 api.RepoName
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 FilePermissionFunc
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c SubRepoPermissionCheckerFilePermissionsFuncFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c SubRepoPermissionCheckerFilePermissionsFuncFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// SubRepoPermissionCheckerPermissionsFunc describes the behavior when the
// Permissions method of the parent MockSubRepoPermissionChecker instance is
// invoked.
type SubRepoPermissionCheckerPermissionsFunc struct {
	defaultHook func(context.Context, int32, RepoContent) (Perms, error)
	hooks       []func(context.Context, int32, RepoContent) (Perms, error)
	history     []SubRepoPermissionCheckerPermissionsFuncCall
	mutex       sync.Mutex
}

// Permissions delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockSubRepoPermissionChecker) Permissions(v0 context.Context, v1 int32, v2 RepoContent) (Perms, error) {
	r0, r1 := m.PermissionsFunc.nextHook()(v0, v1, v2)
	m.PermissionsFunc.appendCall(SubRepoPermissionCheckerPermissionsFuncCall{v0, v1, v2, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the Permissions method
// of the parent MockSubRepoPermissionChecker instance is invoked and the
// hook queue is empty.
func (f *SubRepoPermissionCheckerPermissionsFunc) SetDefaultHook(hook func(context.Context, int32, RepoContent) (Perms, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Permissions method of the parent MockSubRepoPermissionChecker instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *SubRepoPermissionCheckerPermissionsFunc) PushHook(hook func(context.Context, int32, RepoContent) (Perms, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *SubRepoPermissionCheckerPermissionsFunc) SetDefaultReturn(r0 Perms, r1 error) {
	f.SetDefaultHook(func(context.Context, int32, RepoContent) (Perms, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *SubRepoPermissionCheckerPermissionsFunc) PushReturn(r0 Perms, r1 error) {
	f.PushHook(func(context.Context, int32, RepoContent) (Perms, error) {
		return r0, r1
	})
}

func (f *SubRepoPermissionCheckerPermissionsFunc) nextHook() func(context.Context, int32, RepoContent) (Perms, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *SubRepoPermissionCheckerPermissionsFunc) appendCall(r0 SubRepoPermissionCheckerPermissionsFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of SubRepoPermissionCheckerPermissionsFuncCall
// objects describing the invocations of this function.
func (f *SubRepoPermissionCheckerPermissionsFunc) History() []SubRepoPermissionCheckerPermissionsFuncCall {
	f.mutex.Lock()
	history := make([]SubRepoPermissionCheckerPermissionsFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// SubRepoPermissionCheckerPermissionsFuncCall is an object that describes
// an invocation of method Permissions on an instance of
// MockSubRepoPermissionChecker.
type SubRepoPermissionCheckerPermissionsFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int32
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 RepoContent
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 Perms
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c SubRepoPermissionCheckerPermissionsFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c SubRepoPermissionCheckerPermissionsFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}
