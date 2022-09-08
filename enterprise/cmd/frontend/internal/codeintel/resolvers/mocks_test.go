// Code generated by go-mockgen 1.3.4; DO NOT EDIT.
//
// This file was generated by running `sg generate` (or `go-mockgen`) at the root of
// this repository. To add additional mocks to this or another package, add a new entry
// to the mockgen.yaml file in the root of this repository.

package resolvers

import (
	"context"
	"sync"
)

// MockDBStore is a mock implementation of the DBStore interface (from the
// package
// github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/codeintel/resolvers)
// used for unit testing.
type MockDBStore struct {
	// LanguagesRequestedByFunc is an instance of a mock function object
	// controlling the behavior of the method LanguagesRequestedBy.
	LanguagesRequestedByFunc *DBStoreLanguagesRequestedByFunc
	// RequestLanguageSupportFunc is an instance of a mock function object
	// controlling the behavior of the method RequestLanguageSupport.
	RequestLanguageSupportFunc *DBStoreRequestLanguageSupportFunc
}

// NewMockDBStore creates a new mock of the DBStore interface. All methods
// return zero values for all results, unless overwritten.
func NewMockDBStore() *MockDBStore {
	return &MockDBStore{
		LanguagesRequestedByFunc: &DBStoreLanguagesRequestedByFunc{
			defaultHook: func(context.Context, int) (r0 []string, r1 error) {
				return
			},
		},
		RequestLanguageSupportFunc: &DBStoreRequestLanguageSupportFunc{
			defaultHook: func(context.Context, int, string) (r0 error) {
				return
			},
		},
	}
}

// NewStrictMockDBStore creates a new mock of the DBStore interface. All
// methods panic on invocation, unless overwritten.
func NewStrictMockDBStore() *MockDBStore {
	return &MockDBStore{
		LanguagesRequestedByFunc: &DBStoreLanguagesRequestedByFunc{
			defaultHook: func(context.Context, int) ([]string, error) {
				panic("unexpected invocation of MockDBStore.LanguagesRequestedBy")
			},
		},
		RequestLanguageSupportFunc: &DBStoreRequestLanguageSupportFunc{
			defaultHook: func(context.Context, int, string) error {
				panic("unexpected invocation of MockDBStore.RequestLanguageSupport")
			},
		},
	}
}

// NewMockDBStoreFrom creates a new mock of the MockDBStore interface. All
// methods delegate to the given implementation, unless overwritten.
func NewMockDBStoreFrom(i DBStore) *MockDBStore {
	return &MockDBStore{
		LanguagesRequestedByFunc: &DBStoreLanguagesRequestedByFunc{
			defaultHook: i.LanguagesRequestedBy,
		},
		RequestLanguageSupportFunc: &DBStoreRequestLanguageSupportFunc{
			defaultHook: i.RequestLanguageSupport,
		},
	}
}

// DBStoreLanguagesRequestedByFunc describes the behavior when the
// LanguagesRequestedBy method of the parent MockDBStore instance is
// invoked.
type DBStoreLanguagesRequestedByFunc struct {
	defaultHook func(context.Context, int) ([]string, error)
	hooks       []func(context.Context, int) ([]string, error)
	history     []DBStoreLanguagesRequestedByFuncCall
	mutex       sync.Mutex
}

// LanguagesRequestedBy delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockDBStore) LanguagesRequestedBy(v0 context.Context, v1 int) ([]string, error) {
	r0, r1 := m.LanguagesRequestedByFunc.nextHook()(v0, v1)
	m.LanguagesRequestedByFunc.appendCall(DBStoreLanguagesRequestedByFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the LanguagesRequestedBy
// method of the parent MockDBStore instance is invoked and the hook queue
// is empty.
func (f *DBStoreLanguagesRequestedByFunc) SetDefaultHook(hook func(context.Context, int) ([]string, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// LanguagesRequestedBy method of the parent MockDBStore instance invokes
// the hook at the front of the queue and discards it. After the queue is
// empty, the default hook function is invoked for any future action.
func (f *DBStoreLanguagesRequestedByFunc) PushHook(hook func(context.Context, int) ([]string, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *DBStoreLanguagesRequestedByFunc) SetDefaultReturn(r0 []string, r1 error) {
	f.SetDefaultHook(func(context.Context, int) ([]string, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *DBStoreLanguagesRequestedByFunc) PushReturn(r0 []string, r1 error) {
	f.PushHook(func(context.Context, int) ([]string, error) {
		return r0, r1
	})
}

func (f *DBStoreLanguagesRequestedByFunc) nextHook() func(context.Context, int) ([]string, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *DBStoreLanguagesRequestedByFunc) appendCall(r0 DBStoreLanguagesRequestedByFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of DBStoreLanguagesRequestedByFuncCall objects
// describing the invocations of this function.
func (f *DBStoreLanguagesRequestedByFunc) History() []DBStoreLanguagesRequestedByFuncCall {
	f.mutex.Lock()
	history := make([]DBStoreLanguagesRequestedByFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// DBStoreLanguagesRequestedByFuncCall is an object that describes an
// invocation of method LanguagesRequestedBy on an instance of MockDBStore.
type DBStoreLanguagesRequestedByFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []string
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c DBStoreLanguagesRequestedByFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c DBStoreLanguagesRequestedByFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// DBStoreRequestLanguageSupportFunc describes the behavior when the
// RequestLanguageSupport method of the parent MockDBStore instance is
// invoked.
type DBStoreRequestLanguageSupportFunc struct {
	defaultHook func(context.Context, int, string) error
	hooks       []func(context.Context, int, string) error
	history     []DBStoreRequestLanguageSupportFuncCall
	mutex       sync.Mutex
}

// RequestLanguageSupport delegates to the next hook function in the queue
// and stores the parameter and result values of this invocation.
func (m *MockDBStore) RequestLanguageSupport(v0 context.Context, v1 int, v2 string) error {
	r0 := m.RequestLanguageSupportFunc.nextHook()(v0, v1, v2)
	m.RequestLanguageSupportFunc.appendCall(DBStoreRequestLanguageSupportFuncCall{v0, v1, v2, r0})
	return r0
}

// SetDefaultHook sets function that is called when the
// RequestLanguageSupport method of the parent MockDBStore instance is
// invoked and the hook queue is empty.
func (f *DBStoreRequestLanguageSupportFunc) SetDefaultHook(hook func(context.Context, int, string) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// RequestLanguageSupport method of the parent MockDBStore instance invokes
// the hook at the front of the queue and discards it. After the queue is
// empty, the default hook function is invoked for any future action.
func (f *DBStoreRequestLanguageSupportFunc) PushHook(hook func(context.Context, int, string) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *DBStoreRequestLanguageSupportFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, int, string) error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *DBStoreRequestLanguageSupportFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, int, string) error {
		return r0
	})
}

func (f *DBStoreRequestLanguageSupportFunc) nextHook() func(context.Context, int, string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *DBStoreRequestLanguageSupportFunc) appendCall(r0 DBStoreRequestLanguageSupportFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of DBStoreRequestLanguageSupportFuncCall
// objects describing the invocations of this function.
func (f *DBStoreRequestLanguageSupportFunc) History() []DBStoreRequestLanguageSupportFuncCall {
	f.mutex.Lock()
	history := make([]DBStoreRequestLanguageSupportFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// DBStoreRequestLanguageSupportFuncCall is an object that describes an
// invocation of method RequestLanguageSupport on an instance of
// MockDBStore.
type DBStoreRequestLanguageSupportFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 string
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c DBStoreRequestLanguageSupportFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c DBStoreRequestLanguageSupportFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}
