// Code generated by go-mockgen 1.3.0; DO NOT EDIT.

package repomatcher

import (
	"context"
	"sync"

	dbstore "github.com/sourcegraph/sourcegraph/internal/codeintel/stores/dbstore"
)

// MockDBStore is a mock implementation of the DBStore interface (from the
// package
// github.com/sourcegraph/sourcegraph/internal/codeintel/policies/background/repomatcher)
// used for unit testing.
type MockDBStore struct {
	// SelectPoliciesForRepositoryMembershipUpdateFunc is an instance of a
	// mock function object controlling the behavior of the method
	// SelectPoliciesForRepositoryMembershipUpdate.
	SelectPoliciesForRepositoryMembershipUpdateFunc *DBStoreSelectPoliciesForRepositoryMembershipUpdateFunc
	// UpdateReposMatchingPatternsFunc is an instance of a mock function
	// object controlling the behavior of the method
	// UpdateReposMatchingPatterns.
	UpdateReposMatchingPatternsFunc *DBStoreUpdateReposMatchingPatternsFunc
}

// NewMockDBStore creates a new mock of the DBStore interface. All methods
// return zero values for all results, unless overwritten.
func NewMockDBStore() *MockDBStore {
	return &MockDBStore{
		SelectPoliciesForRepositoryMembershipUpdateFunc: &DBStoreSelectPoliciesForRepositoryMembershipUpdateFunc{
			defaultHook: func(context.Context, int) (r0 []dbstore.ConfigurationPolicy, r1 error) {
				return
			},
		},
		UpdateReposMatchingPatternsFunc: &DBStoreUpdateReposMatchingPatternsFunc{
			defaultHook: func(context.Context, []string, int, *int) (r0 error) {
				return
			},
		},
	}
}

// NewStrictMockDBStore creates a new mock of the DBStore interface. All
// methods panic on invocation, unless overwritten.
func NewStrictMockDBStore() *MockDBStore {
	return &MockDBStore{
		SelectPoliciesForRepositoryMembershipUpdateFunc: &DBStoreSelectPoliciesForRepositoryMembershipUpdateFunc{
			defaultHook: func(context.Context, int) ([]dbstore.ConfigurationPolicy, error) {
				panic("unexpected invocation of MockDBStore.SelectPoliciesForRepositoryMembershipUpdate")
			},
		},
		UpdateReposMatchingPatternsFunc: &DBStoreUpdateReposMatchingPatternsFunc{
			defaultHook: func(context.Context, []string, int, *int) error {
				panic("unexpected invocation of MockDBStore.UpdateReposMatchingPatterns")
			},
		},
	}
}

// NewMockDBStoreFrom creates a new mock of the MockDBStore interface. All
// methods delegate to the given implementation, unless overwritten.
func NewMockDBStoreFrom(i DBStore) *MockDBStore {
	return &MockDBStore{
		SelectPoliciesForRepositoryMembershipUpdateFunc: &DBStoreSelectPoliciesForRepositoryMembershipUpdateFunc{
			defaultHook: i.SelectPoliciesForRepositoryMembershipUpdate,
		},
		UpdateReposMatchingPatternsFunc: &DBStoreUpdateReposMatchingPatternsFunc{
			defaultHook: i.UpdateReposMatchingPatterns,
		},
	}
}

// DBStoreSelectPoliciesForRepositoryMembershipUpdateFunc describes the
// behavior when the SelectPoliciesForRepositoryMembershipUpdate method of
// the parent MockDBStore instance is invoked.
type DBStoreSelectPoliciesForRepositoryMembershipUpdateFunc struct {
	defaultHook func(context.Context, int) ([]dbstore.ConfigurationPolicy, error)
	hooks       []func(context.Context, int) ([]dbstore.ConfigurationPolicy, error)
	history     []DBStoreSelectPoliciesForRepositoryMembershipUpdateFuncCall
	mutex       sync.Mutex
}

// SelectPoliciesForRepositoryMembershipUpdate delegates to the next hook
// function in the queue and stores the parameter and result values of this
// invocation.
func (m *MockDBStore) SelectPoliciesForRepositoryMembershipUpdate(v0 context.Context, v1 int) ([]dbstore.ConfigurationPolicy, error) {
	r0, r1 := m.SelectPoliciesForRepositoryMembershipUpdateFunc.nextHook()(v0, v1)
	m.SelectPoliciesForRepositoryMembershipUpdateFunc.appendCall(DBStoreSelectPoliciesForRepositoryMembershipUpdateFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the
// SelectPoliciesForRepositoryMembershipUpdate method of the parent
// MockDBStore instance is invoked and the hook queue is empty.
func (f *DBStoreSelectPoliciesForRepositoryMembershipUpdateFunc) SetDefaultHook(hook func(context.Context, int) ([]dbstore.ConfigurationPolicy, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// SelectPoliciesForRepositoryMembershipUpdate method of the parent
// MockDBStore instance invokes the hook at the front of the queue and
// discards it. After the queue is empty, the default hook function is
// invoked for any future action.
func (f *DBStoreSelectPoliciesForRepositoryMembershipUpdateFunc) PushHook(hook func(context.Context, int) ([]dbstore.ConfigurationPolicy, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *DBStoreSelectPoliciesForRepositoryMembershipUpdateFunc) SetDefaultReturn(r0 []dbstore.ConfigurationPolicy, r1 error) {
	f.SetDefaultHook(func(context.Context, int) ([]dbstore.ConfigurationPolicy, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *DBStoreSelectPoliciesForRepositoryMembershipUpdateFunc) PushReturn(r0 []dbstore.ConfigurationPolicy, r1 error) {
	f.PushHook(func(context.Context, int) ([]dbstore.ConfigurationPolicy, error) {
		return r0, r1
	})
}

func (f *DBStoreSelectPoliciesForRepositoryMembershipUpdateFunc) nextHook() func(context.Context, int) ([]dbstore.ConfigurationPolicy, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *DBStoreSelectPoliciesForRepositoryMembershipUpdateFunc) appendCall(r0 DBStoreSelectPoliciesForRepositoryMembershipUpdateFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of
// DBStoreSelectPoliciesForRepositoryMembershipUpdateFuncCall objects
// describing the invocations of this function.
func (f *DBStoreSelectPoliciesForRepositoryMembershipUpdateFunc) History() []DBStoreSelectPoliciesForRepositoryMembershipUpdateFuncCall {
	f.mutex.Lock()
	history := make([]DBStoreSelectPoliciesForRepositoryMembershipUpdateFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// DBStoreSelectPoliciesForRepositoryMembershipUpdateFuncCall is an object
// that describes an invocation of method
// SelectPoliciesForRepositoryMembershipUpdate on an instance of
// MockDBStore.
type DBStoreSelectPoliciesForRepositoryMembershipUpdateFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []dbstore.ConfigurationPolicy
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c DBStoreSelectPoliciesForRepositoryMembershipUpdateFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c DBStoreSelectPoliciesForRepositoryMembershipUpdateFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// DBStoreUpdateReposMatchingPatternsFunc describes the behavior when the
// UpdateReposMatchingPatterns method of the parent MockDBStore instance is
// invoked.
type DBStoreUpdateReposMatchingPatternsFunc struct {
	defaultHook func(context.Context, []string, int, *int) error
	hooks       []func(context.Context, []string, int, *int) error
	history     []DBStoreUpdateReposMatchingPatternsFuncCall
	mutex       sync.Mutex
}

// UpdateReposMatchingPatterns delegates to the next hook function in the
// queue and stores the parameter and result values of this invocation.
func (m *MockDBStore) UpdateReposMatchingPatterns(v0 context.Context, v1 []string, v2 int, v3 *int) error {
	r0 := m.UpdateReposMatchingPatternsFunc.nextHook()(v0, v1, v2, v3)
	m.UpdateReposMatchingPatternsFunc.appendCall(DBStoreUpdateReposMatchingPatternsFuncCall{v0, v1, v2, v3, r0})
	return r0
}

// SetDefaultHook sets function that is called when the
// UpdateReposMatchingPatterns method of the parent MockDBStore instance is
// invoked and the hook queue is empty.
func (f *DBStoreUpdateReposMatchingPatternsFunc) SetDefaultHook(hook func(context.Context, []string, int, *int) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// UpdateReposMatchingPatterns method of the parent MockDBStore instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *DBStoreUpdateReposMatchingPatternsFunc) PushHook(hook func(context.Context, []string, int, *int) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *DBStoreUpdateReposMatchingPatternsFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, []string, int, *int) error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *DBStoreUpdateReposMatchingPatternsFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, []string, int, *int) error {
		return r0
	})
}

func (f *DBStoreUpdateReposMatchingPatternsFunc) nextHook() func(context.Context, []string, int, *int) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *DBStoreUpdateReposMatchingPatternsFunc) appendCall(r0 DBStoreUpdateReposMatchingPatternsFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of DBStoreUpdateReposMatchingPatternsFuncCall
// objects describing the invocations of this function.
func (f *DBStoreUpdateReposMatchingPatternsFunc) History() []DBStoreUpdateReposMatchingPatternsFuncCall {
	f.mutex.Lock()
	history := make([]DBStoreUpdateReposMatchingPatternsFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// DBStoreUpdateReposMatchingPatternsFuncCall is an object that describes an
// invocation of method UpdateReposMatchingPatterns on an instance of
// MockDBStore.
type DBStoreUpdateReposMatchingPatternsFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 []string
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 int
	// Arg3 is the value of the 4th argument passed to this method
	// invocation.
	Arg3 *int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c DBStoreUpdateReposMatchingPatternsFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c DBStoreUpdateReposMatchingPatternsFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}
