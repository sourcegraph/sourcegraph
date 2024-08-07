// Code generated by go-mockgen 1.3.7; DO NOT EDIT.
//
// This file was generated by running `sg generate` (or `go-mockgen`) at the root of
// this repository. To add additional mocks to this or another package, add a new entry
// to the mockgen.yaml file in the root of this repository.

package processor

import (
	"context"
	"sync"

	api "github.com/sourcegraph/sourcegraph/internal/api"
	types "github.com/sourcegraph/sourcegraph/internal/types"
)

// MockRepoStore is a mock implementation of the RepoStore interface (from
// the package
// github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/background/processor)
// used for unit testing.
type MockRepoStore struct {
	// GetFunc is an instance of a mock function object controlling the
	// behavior of the method Get.
	GetFunc *RepoStoreGetFunc
}

// NewMockRepoStore creates a new mock of the RepoStore interface. All
// methods return zero values for all results, unless overwritten.
func NewMockRepoStore() *MockRepoStore {
	return &MockRepoStore{
		GetFunc: &RepoStoreGetFunc{
			defaultHook: func(context.Context, api.RepoID) (r0 *types.Repo, r1 error) {
				return
			},
		},
	}
}

// NewStrictMockRepoStore creates a new mock of the RepoStore interface. All
// methods panic on invocation, unless overwritten.
func NewStrictMockRepoStore() *MockRepoStore {
	return &MockRepoStore{
		GetFunc: &RepoStoreGetFunc{
			defaultHook: func(context.Context, api.RepoID) (*types.Repo, error) {
				panic("unexpected invocation of MockRepoStore.Get")
			},
		},
	}
}

// NewMockRepoStoreFrom creates a new mock of the MockRepoStore interface.
// All methods delegate to the given implementation, unless overwritten.
func NewMockRepoStoreFrom(i RepoStore) *MockRepoStore {
	return &MockRepoStore{
		GetFunc: &RepoStoreGetFunc{
			defaultHook: i.Get,
		},
	}
}

// RepoStoreGetFunc describes the behavior when the Get method of the parent
// MockRepoStore instance is invoked.
type RepoStoreGetFunc struct {
	defaultHook func(context.Context, api.RepoID) (*types.Repo, error)
	hooks       []func(context.Context, api.RepoID) (*types.Repo, error)
	history     []RepoStoreGetFuncCall
	mutex       sync.Mutex
}

// Get delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockRepoStore) Get(v0 context.Context, v1 api.RepoID) (*types.Repo, error) {
	r0, r1 := m.GetFunc.nextHook()(v0, v1)
	m.GetFunc.appendCall(RepoStoreGetFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the Get method of the
// parent MockRepoStore instance is invoked and the hook queue is empty.
func (f *RepoStoreGetFunc) SetDefaultHook(hook func(context.Context, api.RepoID) (*types.Repo, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Get method of the parent MockRepoStore instance invokes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *RepoStoreGetFunc) PushHook(hook func(context.Context, api.RepoID) (*types.Repo, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *RepoStoreGetFunc) SetDefaultReturn(r0 *types.Repo, r1 error) {
	f.SetDefaultHook(func(context.Context, api.RepoID) (*types.Repo, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *RepoStoreGetFunc) PushReturn(r0 *types.Repo, r1 error) {
	f.PushHook(func(context.Context, api.RepoID) (*types.Repo, error) {
		return r0, r1
	})
}

func (f *RepoStoreGetFunc) nextHook() func(context.Context, api.RepoID) (*types.Repo, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *RepoStoreGetFunc) appendCall(r0 RepoStoreGetFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of RepoStoreGetFuncCall objects describing the
// invocations of this function.
func (f *RepoStoreGetFunc) History() []RepoStoreGetFuncCall {
	f.mutex.Lock()
	history := make([]RepoStoreGetFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// RepoStoreGetFuncCall is an object that describes an invocation of method
// Get on an instance of MockRepoStore.
type RepoStoreGetFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 api.RepoID
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 *types.Repo
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c RepoStoreGetFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c RepoStoreGetFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}
