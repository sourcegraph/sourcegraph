// Code generated by go-mockgen 1.3.2; DO NOT EDIT.
//
// This file was generated by running `sg generate` (or `go-mockgen`) at the root of
// this repository. To add additional mocks to this or another package, add a new entry
// to the metadata.yaml file in the root of this repository.

package scheduler

import (
	"context"
	"sync"
	"time"

	enterprise "github.com/sourcegraph/sourcegraph/internal/codeintel/policies/enterprise"
	dbstore "github.com/sourcegraph/sourcegraph/internal/codeintel/stores/dbstore"
)

// MockDBStore is a mock implementation of the DBStore interface (from the
// package
// github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/background/scheduler)
// used for unit testing.
type MockDBStore struct {
	// GetConfigurationPoliciesFunc is an instance of a mock function object
	// controlling the behavior of the method GetConfigurationPolicies.
	GetConfigurationPoliciesFunc *DBStoreGetConfigurationPoliciesFunc
	// SelectRepositoriesForIndexScanFunc is an instance of a mock function
	// object controlling the behavior of the method
	// SelectRepositoriesForIndexScan.
	SelectRepositoriesForIndexScanFunc *DBStoreSelectRepositoriesForIndexScanFunc
}

// NewMockDBStore creates a new mock of the DBStore interface. All methods
// return zero values for all results, unless overwritten.
func NewMockDBStore() *MockDBStore {
	return &MockDBStore{
		GetConfigurationPoliciesFunc: &DBStoreGetConfigurationPoliciesFunc{
			defaultHook: func(context.Context, dbstore.GetConfigurationPoliciesOptions) (r0 []dbstore.ConfigurationPolicy, r1 int, r2 error) {
				return
			},
		},
		SelectRepositoriesForIndexScanFunc: &DBStoreSelectRepositoriesForIndexScanFunc{
			defaultHook: func(context.Context, string, string, time.Duration, bool, *int, int) (r0 []int, r1 error) {
				return
			},
		},
	}
}

// NewStrictMockDBStore creates a new mock of the DBStore interface. All
// methods panic on invocation, unless overwritten.
func NewStrictMockDBStore() *MockDBStore {
	return &MockDBStore{
		GetConfigurationPoliciesFunc: &DBStoreGetConfigurationPoliciesFunc{
			defaultHook: func(context.Context, dbstore.GetConfigurationPoliciesOptions) ([]dbstore.ConfigurationPolicy, int, error) {
				panic("unexpected invocation of MockDBStore.GetConfigurationPolicies")
			},
		},
		SelectRepositoriesForIndexScanFunc: &DBStoreSelectRepositoriesForIndexScanFunc{
			defaultHook: func(context.Context, string, string, time.Duration, bool, *int, int) ([]int, error) {
				panic("unexpected invocation of MockDBStore.SelectRepositoriesForIndexScan")
			},
		},
	}
}

// NewMockDBStoreFrom creates a new mock of the MockDBStore interface. All
// methods delegate to the given implementation, unless overwritten.
func NewMockDBStoreFrom(i DBStore) *MockDBStore {
	return &MockDBStore{
		GetConfigurationPoliciesFunc: &DBStoreGetConfigurationPoliciesFunc{
			defaultHook: i.GetConfigurationPolicies,
		},
		SelectRepositoriesForIndexScanFunc: &DBStoreSelectRepositoriesForIndexScanFunc{
			defaultHook: i.SelectRepositoriesForIndexScan,
		},
	}
}

// DBStoreGetConfigurationPoliciesFunc describes the behavior when the
// GetConfigurationPolicies method of the parent MockDBStore instance is
// invoked.
type DBStoreGetConfigurationPoliciesFunc struct {
	defaultHook func(context.Context, dbstore.GetConfigurationPoliciesOptions) ([]dbstore.ConfigurationPolicy, int, error)
	hooks       []func(context.Context, dbstore.GetConfigurationPoliciesOptions) ([]dbstore.ConfigurationPolicy, int, error)
	history     []DBStoreGetConfigurationPoliciesFuncCall
	mutex       sync.Mutex
}

// GetConfigurationPolicies delegates to the next hook function in the queue
// and stores the parameter and result values of this invocation.
func (m *MockDBStore) GetConfigurationPolicies(v0 context.Context, v1 dbstore.GetConfigurationPoliciesOptions) ([]dbstore.ConfigurationPolicy, int, error) {
	r0, r1, r2 := m.GetConfigurationPoliciesFunc.nextHook()(v0, v1)
	m.GetConfigurationPoliciesFunc.appendCall(DBStoreGetConfigurationPoliciesFuncCall{v0, v1, r0, r1, r2})
	return r0, r1, r2
}

// SetDefaultHook sets function that is called when the
// GetConfigurationPolicies method of the parent MockDBStore instance is
// invoked and the hook queue is empty.
func (f *DBStoreGetConfigurationPoliciesFunc) SetDefaultHook(hook func(context.Context, dbstore.GetConfigurationPoliciesOptions) ([]dbstore.ConfigurationPolicy, int, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// GetConfigurationPolicies method of the parent MockDBStore instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *DBStoreGetConfigurationPoliciesFunc) PushHook(hook func(context.Context, dbstore.GetConfigurationPoliciesOptions) ([]dbstore.ConfigurationPolicy, int, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *DBStoreGetConfigurationPoliciesFunc) SetDefaultReturn(r0 []dbstore.ConfigurationPolicy, r1 int, r2 error) {
	f.SetDefaultHook(func(context.Context, dbstore.GetConfigurationPoliciesOptions) ([]dbstore.ConfigurationPolicy, int, error) {
		return r0, r1, r2
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *DBStoreGetConfigurationPoliciesFunc) PushReturn(r0 []dbstore.ConfigurationPolicy, r1 int, r2 error) {
	f.PushHook(func(context.Context, dbstore.GetConfigurationPoliciesOptions) ([]dbstore.ConfigurationPolicy, int, error) {
		return r0, r1, r2
	})
}

func (f *DBStoreGetConfigurationPoliciesFunc) nextHook() func(context.Context, dbstore.GetConfigurationPoliciesOptions) ([]dbstore.ConfigurationPolicy, int, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *DBStoreGetConfigurationPoliciesFunc) appendCall(r0 DBStoreGetConfigurationPoliciesFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of DBStoreGetConfigurationPoliciesFuncCall
// objects describing the invocations of this function.
func (f *DBStoreGetConfigurationPoliciesFunc) History() []DBStoreGetConfigurationPoliciesFuncCall {
	f.mutex.Lock()
	history := make([]DBStoreGetConfigurationPoliciesFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// DBStoreGetConfigurationPoliciesFuncCall is an object that describes an
// invocation of method GetConfigurationPolicies on an instance of
// MockDBStore.
type DBStoreGetConfigurationPoliciesFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 dbstore.GetConfigurationPoliciesOptions
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []dbstore.ConfigurationPolicy
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 int
	// Result2 is the value of the 3rd result returned from this method
	// invocation.
	Result2 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c DBStoreGetConfigurationPoliciesFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c DBStoreGetConfigurationPoliciesFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1, c.Result2}
}

// DBStoreSelectRepositoriesForIndexScanFunc describes the behavior when the
// SelectRepositoriesForIndexScan method of the parent MockDBStore instance
// is invoked.
type DBStoreSelectRepositoriesForIndexScanFunc struct {
	defaultHook func(context.Context, string, string, time.Duration, bool, *int, int) ([]int, error)
	hooks       []func(context.Context, string, string, time.Duration, bool, *int, int) ([]int, error)
	history     []DBStoreSelectRepositoriesForIndexScanFuncCall
	mutex       sync.Mutex
}

// SelectRepositoriesForIndexScan delegates to the next hook function in the
// queue and stores the parameter and result values of this invocation.
func (m *MockDBStore) SelectRepositoriesForIndexScan(v0 context.Context, v1 string, v2 string, v3 time.Duration, v4 bool, v5 *int, v6 int) ([]int, error) {
	r0, r1 := m.SelectRepositoriesForIndexScanFunc.nextHook()(v0, v1, v2, v3, v4, v5, v6)
	m.SelectRepositoriesForIndexScanFunc.appendCall(DBStoreSelectRepositoriesForIndexScanFuncCall{v0, v1, v2, v3, v4, v5, v6, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the
// SelectRepositoriesForIndexScan method of the parent MockDBStore instance
// is invoked and the hook queue is empty.
func (f *DBStoreSelectRepositoriesForIndexScanFunc) SetDefaultHook(hook func(context.Context, string, string, time.Duration, bool, *int, int) ([]int, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// SelectRepositoriesForIndexScan method of the parent MockDBStore instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *DBStoreSelectRepositoriesForIndexScanFunc) PushHook(hook func(context.Context, string, string, time.Duration, bool, *int, int) ([]int, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *DBStoreSelectRepositoriesForIndexScanFunc) SetDefaultReturn(r0 []int, r1 error) {
	f.SetDefaultHook(func(context.Context, string, string, time.Duration, bool, *int, int) ([]int, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *DBStoreSelectRepositoriesForIndexScanFunc) PushReturn(r0 []int, r1 error) {
	f.PushHook(func(context.Context, string, string, time.Duration, bool, *int, int) ([]int, error) {
		return r0, r1
	})
}

func (f *DBStoreSelectRepositoriesForIndexScanFunc) nextHook() func(context.Context, string, string, time.Duration, bool, *int, int) ([]int, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *DBStoreSelectRepositoriesForIndexScanFunc) appendCall(r0 DBStoreSelectRepositoriesForIndexScanFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of
// DBStoreSelectRepositoriesForIndexScanFuncCall objects describing the
// invocations of this function.
func (f *DBStoreSelectRepositoriesForIndexScanFunc) History() []DBStoreSelectRepositoriesForIndexScanFuncCall {
	f.mutex.Lock()
	history := make([]DBStoreSelectRepositoriesForIndexScanFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// DBStoreSelectRepositoriesForIndexScanFuncCall is an object that describes
// an invocation of method SelectRepositoriesForIndexScan on an instance of
// MockDBStore.
type DBStoreSelectRepositoriesForIndexScanFuncCall struct {
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
	Arg3 time.Duration
	// Arg4 is the value of the 5th argument passed to this method
	// invocation.
	Arg4 bool
	// Arg5 is the value of the 6th argument passed to this method
	// invocation.
	Arg5 *int
	// Arg6 is the value of the 7th argument passed to this method
	// invocation.
	Arg6 int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []int
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c DBStoreSelectRepositoriesForIndexScanFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3, c.Arg4, c.Arg5, c.Arg6}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c DBStoreSelectRepositoriesForIndexScanFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// MockPolicyMatcher is a mock implementation of the PolicyMatcher interface
// (from the package
// github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/background/scheduler)
// used for unit testing.
type MockPolicyMatcher struct {
	// CommitsDescribedByPolicyFunc is an instance of a mock function object
	// controlling the behavior of the method CommitsDescribedByPolicy.
	CommitsDescribedByPolicyFunc *PolicyMatcherCommitsDescribedByPolicyFunc
}

// NewMockPolicyMatcher creates a new mock of the PolicyMatcher interface.
// All methods return zero values for all results, unless overwritten.
func NewMockPolicyMatcher() *MockPolicyMatcher {
	return &MockPolicyMatcher{
		CommitsDescribedByPolicyFunc: &PolicyMatcherCommitsDescribedByPolicyFunc{
			defaultHook: func(context.Context, int, []dbstore.ConfigurationPolicy, time.Time, ...string) (r0 map[string][]enterprise.PolicyMatch, r1 error) {
				return
			},
		},
	}
}

// NewStrictMockPolicyMatcher creates a new mock of the PolicyMatcher
// interface. All methods panic on invocation, unless overwritten.
func NewStrictMockPolicyMatcher() *MockPolicyMatcher {
	return &MockPolicyMatcher{
		CommitsDescribedByPolicyFunc: &PolicyMatcherCommitsDescribedByPolicyFunc{
			defaultHook: func(context.Context, int, []dbstore.ConfigurationPolicy, time.Time, ...string) (map[string][]enterprise.PolicyMatch, error) {
				panic("unexpected invocation of MockPolicyMatcher.CommitsDescribedByPolicy")
			},
		},
	}
}

// NewMockPolicyMatcherFrom creates a new mock of the MockPolicyMatcher
// interface. All methods delegate to the given implementation, unless
// overwritten.
func NewMockPolicyMatcherFrom(i PolicyMatcher) *MockPolicyMatcher {
	return &MockPolicyMatcher{
		CommitsDescribedByPolicyFunc: &PolicyMatcherCommitsDescribedByPolicyFunc{
			defaultHook: i.CommitsDescribedByPolicy,
		},
	}
}

// PolicyMatcherCommitsDescribedByPolicyFunc describes the behavior when the
// CommitsDescribedByPolicy method of the parent MockPolicyMatcher instance
// is invoked.
type PolicyMatcherCommitsDescribedByPolicyFunc struct {
	defaultHook func(context.Context, int, []dbstore.ConfigurationPolicy, time.Time, ...string) (map[string][]enterprise.PolicyMatch, error)
	hooks       []func(context.Context, int, []dbstore.ConfigurationPolicy, time.Time, ...string) (map[string][]enterprise.PolicyMatch, error)
	history     []PolicyMatcherCommitsDescribedByPolicyFuncCall
	mutex       sync.Mutex
}

// CommitsDescribedByPolicy delegates to the next hook function in the queue
// and stores the parameter and result values of this invocation.
func (m *MockPolicyMatcher) CommitsDescribedByPolicy(v0 context.Context, v1 int, v2 []dbstore.ConfigurationPolicy, v3 time.Time, v4 ...string) (map[string][]enterprise.PolicyMatch, error) {
	r0, r1 := m.CommitsDescribedByPolicyFunc.nextHook()(v0, v1, v2, v3, v4...)
	m.CommitsDescribedByPolicyFunc.appendCall(PolicyMatcherCommitsDescribedByPolicyFuncCall{v0, v1, v2, v3, v4, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the
// CommitsDescribedByPolicy method of the parent MockPolicyMatcher instance
// is invoked and the hook queue is empty.
func (f *PolicyMatcherCommitsDescribedByPolicyFunc) SetDefaultHook(hook func(context.Context, int, []dbstore.ConfigurationPolicy, time.Time, ...string) (map[string][]enterprise.PolicyMatch, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// CommitsDescribedByPolicy method of the parent MockPolicyMatcher instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *PolicyMatcherCommitsDescribedByPolicyFunc) PushHook(hook func(context.Context, int, []dbstore.ConfigurationPolicy, time.Time, ...string) (map[string][]enterprise.PolicyMatch, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *PolicyMatcherCommitsDescribedByPolicyFunc) SetDefaultReturn(r0 map[string][]enterprise.PolicyMatch, r1 error) {
	f.SetDefaultHook(func(context.Context, int, []dbstore.ConfigurationPolicy, time.Time, ...string) (map[string][]enterprise.PolicyMatch, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *PolicyMatcherCommitsDescribedByPolicyFunc) PushReturn(r0 map[string][]enterprise.PolicyMatch, r1 error) {
	f.PushHook(func(context.Context, int, []dbstore.ConfigurationPolicy, time.Time, ...string) (map[string][]enterprise.PolicyMatch, error) {
		return r0, r1
	})
}

func (f *PolicyMatcherCommitsDescribedByPolicyFunc) nextHook() func(context.Context, int, []dbstore.ConfigurationPolicy, time.Time, ...string) (map[string][]enterprise.PolicyMatch, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *PolicyMatcherCommitsDescribedByPolicyFunc) appendCall(r0 PolicyMatcherCommitsDescribedByPolicyFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of
// PolicyMatcherCommitsDescribedByPolicyFuncCall objects describing the
// invocations of this function.
func (f *PolicyMatcherCommitsDescribedByPolicyFunc) History() []PolicyMatcherCommitsDescribedByPolicyFuncCall {
	f.mutex.Lock()
	history := make([]PolicyMatcherCommitsDescribedByPolicyFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// PolicyMatcherCommitsDescribedByPolicyFuncCall is an object that describes
// an invocation of method CommitsDescribedByPolicy on an instance of
// MockPolicyMatcher.
type PolicyMatcherCommitsDescribedByPolicyFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 []dbstore.ConfigurationPolicy
	// Arg3 is the value of the 4th argument passed to this method
	// invocation.
	Arg3 time.Time
	// Arg4 is a slice containing the values of the variadic arguments
	// passed to this method invocation.
	Arg4 []string
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 map[string][]enterprise.PolicyMatch
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation. The variadic slice argument is flattened in this array such
// that one positional argument and three variadic arguments would result in
// a slice of four, not two.
func (c PolicyMatcherCommitsDescribedByPolicyFuncCall) Args() []interface{} {
	trailing := []interface{}{}
	for _, val := range c.Arg4 {
		trailing = append(trailing, val)
	}

	return append([]interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3}, trailing...)
}

// Results returns an interface slice containing the results of this
// invocation.
func (c PolicyMatcherCommitsDescribedByPolicyFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}
