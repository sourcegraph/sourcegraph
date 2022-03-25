// Code generated by go-mockgen 1.1.4; DO NOT EDIT.

package authz

import (
	"context"
	"sync"

	api "github.com/sourcegraph/sourcegraph/internal/api"
)

// MockSubRepoPermissionsGetter is a mock implementation of the
// SubRepoPermissionsGetter interface (from the package
// github.com/sourcegraph/sourcegraph/internal/authz) used for unit testing.
type MockSubRepoPermissionsGetter struct {
	// GetByUserFunc is an instance of a mock function object controlling
	// the behavior of the method GetByUser.
	GetByUserFunc *SubRepoPermissionsGetterGetByUserFunc
	// RepoIdSupportedFunc is an instance of a mock function object
	// controlling the behavior of the method RepoIdSupported.
	RepoIdSupportedFunc *SubRepoPermissionsGetterRepoIdSupportedFunc
	// RepoSupportedFunc is an instance of a mock function object
	// controlling the behavior of the method RepoSupported.
	RepoSupportedFunc *SubRepoPermissionsGetterRepoSupportedFunc
}

// NewMockSubRepoPermissionsGetter creates a new mock of the
// SubRepoPermissionsGetter interface. All methods return zero values for
// all results, unless overwritten.
func NewMockSubRepoPermissionsGetter() *MockSubRepoPermissionsGetter {
	return &MockSubRepoPermissionsGetter{
		GetByUserFunc: &SubRepoPermissionsGetterGetByUserFunc{
			defaultHook: func(context.Context, int32) (map[api.RepoName]SubRepoPermissions, error) {
				return nil, nil
			},
		},
		RepoIdSupportedFunc: &SubRepoPermissionsGetterRepoIdSupportedFunc{
			defaultHook: func(context.Context, api.RepoID) (bool, error) {
				return false, nil
			},
		},
		RepoSupportedFunc: &SubRepoPermissionsGetterRepoSupportedFunc{
			defaultHook: func(context.Context, api.RepoName) (bool, error) {
				return false, nil
			},
		},
	}
}

// NewStrictMockSubRepoPermissionsGetter creates a new mock of the
// SubRepoPermissionsGetter interface. All methods panic on invocation,
// unless overwritten.
func NewStrictMockSubRepoPermissionsGetter() *MockSubRepoPermissionsGetter {
	return &MockSubRepoPermissionsGetter{
		GetByUserFunc: &SubRepoPermissionsGetterGetByUserFunc{
			defaultHook: func(context.Context, int32) (map[api.RepoName]SubRepoPermissions, error) {
				panic("unexpected invocation of MockSubRepoPermissionsGetter.GetByUser")
			},
		},
		RepoIdSupportedFunc: &SubRepoPermissionsGetterRepoIdSupportedFunc{
			defaultHook: func(context.Context, api.RepoID) (bool, error) {
				panic("unexpected invocation of MockSubRepoPermissionsGetter.RepoIdSupported")
			},
		},
		RepoSupportedFunc: &SubRepoPermissionsGetterRepoSupportedFunc{
			defaultHook: func(context.Context, api.RepoName) (bool, error) {
				panic("unexpected invocation of MockSubRepoPermissionsGetter.RepoSupported")
			},
		},
	}
}

// NewMockSubRepoPermissionsGetterFrom creates a new mock of the
// MockSubRepoPermissionsGetter interface. All methods delegate to the given
// implementation, unless overwritten.
func NewMockSubRepoPermissionsGetterFrom(i SubRepoPermissionsGetter) *MockSubRepoPermissionsGetter {
	return &MockSubRepoPermissionsGetter{
		GetByUserFunc: &SubRepoPermissionsGetterGetByUserFunc{
			defaultHook: i.GetByUser,
		},
		RepoIdSupportedFunc: &SubRepoPermissionsGetterRepoIdSupportedFunc{
			defaultHook: i.RepoIdSupported,
		},
		RepoSupportedFunc: &SubRepoPermissionsGetterRepoSupportedFunc{
			defaultHook: i.RepoSupported,
		},
	}
}

// SubRepoPermissionsGetterGetByUserFunc describes the behavior when the
// GetByUser method of the parent MockSubRepoPermissionsGetter instance is
// invoked.
type SubRepoPermissionsGetterGetByUserFunc struct {
	defaultHook func(context.Context, int32) (map[api.RepoName]SubRepoPermissions, error)
	hooks       []func(context.Context, int32) (map[api.RepoName]SubRepoPermissions, error)
	history     []SubRepoPermissionsGetterGetByUserFuncCall
	mutex       sync.Mutex
}

// GetByUser delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockSubRepoPermissionsGetter) GetByUser(v0 context.Context, v1 int32) (map[api.RepoName]SubRepoPermissions, error) {
	r0, r1 := m.GetByUserFunc.nextHook()(v0, v1)
	m.GetByUserFunc.appendCall(SubRepoPermissionsGetterGetByUserFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the GetByUser method of
// the parent MockSubRepoPermissionsGetter instance is invoked and the hook
// queue is empty.
func (f *SubRepoPermissionsGetterGetByUserFunc) SetDefaultHook(hook func(context.Context, int32) (map[api.RepoName]SubRepoPermissions, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// GetByUser method of the parent MockSubRepoPermissionsGetter instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *SubRepoPermissionsGetterGetByUserFunc) PushHook(hook func(context.Context, int32) (map[api.RepoName]SubRepoPermissions, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *SubRepoPermissionsGetterGetByUserFunc) SetDefaultReturn(r0 map[api.RepoName]SubRepoPermissions, r1 error) {
	f.SetDefaultHook(func(context.Context, int32) (map[api.RepoName]SubRepoPermissions, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *SubRepoPermissionsGetterGetByUserFunc) PushReturn(r0 map[api.RepoName]SubRepoPermissions, r1 error) {
	f.PushHook(func(context.Context, int32) (map[api.RepoName]SubRepoPermissions, error) {
		return r0, r1
	})
}

func (f *SubRepoPermissionsGetterGetByUserFunc) nextHook() func(context.Context, int32) (map[api.RepoName]SubRepoPermissions, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *SubRepoPermissionsGetterGetByUserFunc) appendCall(r0 SubRepoPermissionsGetterGetByUserFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of SubRepoPermissionsGetterGetByUserFuncCall
// objects describing the invocations of this function.
func (f *SubRepoPermissionsGetterGetByUserFunc) History() []SubRepoPermissionsGetterGetByUserFuncCall {
	f.mutex.Lock()
	history := make([]SubRepoPermissionsGetterGetByUserFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// SubRepoPermissionsGetterGetByUserFuncCall is an object that describes an
// invocation of method GetByUser on an instance of
// MockSubRepoPermissionsGetter.
type SubRepoPermissionsGetterGetByUserFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int32
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 map[api.RepoName]SubRepoPermissions
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c SubRepoPermissionsGetterGetByUserFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c SubRepoPermissionsGetterGetByUserFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// SubRepoPermissionsGetterRepoIdSupportedFunc describes the behavior when
// the RepoIdSupported method of the parent MockSubRepoPermissionsGetter
// instance is invoked.
type SubRepoPermissionsGetterRepoIdSupportedFunc struct {
	defaultHook func(context.Context, api.RepoID) (bool, error)
	hooks       []func(context.Context, api.RepoID) (bool, error)
	history     []SubRepoPermissionsGetterRepoIdSupportedFuncCall
	mutex       sync.Mutex
}

// RepoIdSupported delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockSubRepoPermissionsGetter) RepoIdSupported(v0 context.Context, v1 api.RepoID) (bool, error) {
	r0, r1 := m.RepoIdSupportedFunc.nextHook()(v0, v1)
	m.RepoIdSupportedFunc.appendCall(SubRepoPermissionsGetterRepoIdSupportedFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the RepoIdSupported
// method of the parent MockSubRepoPermissionsGetter instance is invoked and
// the hook queue is empty.
func (f *SubRepoPermissionsGetterRepoIdSupportedFunc) SetDefaultHook(hook func(context.Context, api.RepoID) (bool, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// RepoIdSupported method of the parent MockSubRepoPermissionsGetter
// instance invokes the hook at the front of the queue and discards it.
// After the queue is empty, the default hook function is invoked for any
// future action.
func (f *SubRepoPermissionsGetterRepoIdSupportedFunc) PushHook(hook func(context.Context, api.RepoID) (bool, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *SubRepoPermissionsGetterRepoIdSupportedFunc) SetDefaultReturn(r0 bool, r1 error) {
	f.SetDefaultHook(func(context.Context, api.RepoID) (bool, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *SubRepoPermissionsGetterRepoIdSupportedFunc) PushReturn(r0 bool, r1 error) {
	f.PushHook(func(context.Context, api.RepoID) (bool, error) {
		return r0, r1
	})
}

func (f *SubRepoPermissionsGetterRepoIdSupportedFunc) nextHook() func(context.Context, api.RepoID) (bool, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *SubRepoPermissionsGetterRepoIdSupportedFunc) appendCall(r0 SubRepoPermissionsGetterRepoIdSupportedFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of
// SubRepoPermissionsGetterRepoIdSupportedFuncCall objects describing the
// invocations of this function.
func (f *SubRepoPermissionsGetterRepoIdSupportedFunc) History() []SubRepoPermissionsGetterRepoIdSupportedFuncCall {
	f.mutex.Lock()
	history := make([]SubRepoPermissionsGetterRepoIdSupportedFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// SubRepoPermissionsGetterRepoIdSupportedFuncCall is an object that
// describes an invocation of method RepoIdSupported on an instance of
// MockSubRepoPermissionsGetter.
type SubRepoPermissionsGetterRepoIdSupportedFuncCall struct {
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
func (c SubRepoPermissionsGetterRepoIdSupportedFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c SubRepoPermissionsGetterRepoIdSupportedFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// SubRepoPermissionsGetterRepoSupportedFunc describes the behavior when the
// RepoSupported method of the parent MockSubRepoPermissionsGetter instance
// is invoked.
type SubRepoPermissionsGetterRepoSupportedFunc struct {
	defaultHook func(context.Context, api.RepoName) (bool, error)
	hooks       []func(context.Context, api.RepoName) (bool, error)
	history     []SubRepoPermissionsGetterRepoSupportedFuncCall
	mutex       sync.Mutex
}

// RepoSupported delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockSubRepoPermissionsGetter) RepoSupported(v0 context.Context, v1 api.RepoName) (bool, error) {
	r0, r1 := m.RepoSupportedFunc.nextHook()(v0, v1)
	m.RepoSupportedFunc.appendCall(SubRepoPermissionsGetterRepoSupportedFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the RepoSupported method
// of the parent MockSubRepoPermissionsGetter instance is invoked and the
// hook queue is empty.
func (f *SubRepoPermissionsGetterRepoSupportedFunc) SetDefaultHook(hook func(context.Context, api.RepoName) (bool, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// RepoSupported method of the parent MockSubRepoPermissionsGetter instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *SubRepoPermissionsGetterRepoSupportedFunc) PushHook(hook func(context.Context, api.RepoName) (bool, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *SubRepoPermissionsGetterRepoSupportedFunc) SetDefaultReturn(r0 bool, r1 error) {
	f.SetDefaultHook(func(context.Context, api.RepoName) (bool, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *SubRepoPermissionsGetterRepoSupportedFunc) PushReturn(r0 bool, r1 error) {
	f.PushHook(func(context.Context, api.RepoName) (bool, error) {
		return r0, r1
	})
}

func (f *SubRepoPermissionsGetterRepoSupportedFunc) nextHook() func(context.Context, api.RepoName) (bool, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *SubRepoPermissionsGetterRepoSupportedFunc) appendCall(r0 SubRepoPermissionsGetterRepoSupportedFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of
// SubRepoPermissionsGetterRepoSupportedFuncCall objects describing the
// invocations of this function.
func (f *SubRepoPermissionsGetterRepoSupportedFunc) History() []SubRepoPermissionsGetterRepoSupportedFuncCall {
	f.mutex.Lock()
	history := make([]SubRepoPermissionsGetterRepoSupportedFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// SubRepoPermissionsGetterRepoSupportedFuncCall is an object that describes
// an invocation of method RepoSupported on an instance of
// MockSubRepoPermissionsGetter.
type SubRepoPermissionsGetterRepoSupportedFuncCall struct {
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
func (c SubRepoPermissionsGetterRepoSupportedFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c SubRepoPermissionsGetterRepoSupportedFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}
