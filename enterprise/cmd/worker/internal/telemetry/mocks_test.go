// Code generated by go-mockgen 1.3.7; DO NOT EDIT.
//
// This file was generated by running `sg generate` (or `go-mockgen`) at the root of
// this repository. To add additional mocks to this or another package, add a new entry
// to the mockgen.yaml file in the root of this repository.

package telemetry

import (
	"context"
	"sync"
)

// MockBookmarkStore is a mock implementation of the bookmarkStore interface
// (from the package
// github.com/sourcegraph/sourcegraph/enterprise/cmd/worker/internal/telemetry)
// used for unit testing.
type MockBookmarkStore struct {
	// GetBookmarkFunc is an instance of a mock function object controlling
	// the behavior of the method GetBookmark.
	GetBookmarkFunc *BookmarkStoreGetBookmarkFunc
	// UpdateBookmarkFunc is an instance of a mock function object
	// controlling the behavior of the method UpdateBookmark.
	UpdateBookmarkFunc *BookmarkStoreUpdateBookmarkFunc
}

// NewMockBookmarkStore creates a new mock of the bookmarkStore interface.
// All methods return zero values for all results, unless overwritten.
func NewMockBookmarkStore() *MockBookmarkStore {
	return &MockBookmarkStore{
		GetBookmarkFunc: &BookmarkStoreGetBookmarkFunc{
			defaultHook: func(context.Context) (r0 int, r1 error) {
				return
			},
		},
		UpdateBookmarkFunc: &BookmarkStoreUpdateBookmarkFunc{
			defaultHook: func(context.Context, int) (r0 error) {
				return
			},
		},
	}
}

// NewStrictMockBookmarkStore creates a new mock of the bookmarkStore
// interface. All methods panic on invocation, unless overwritten.
func NewStrictMockBookmarkStore() *MockBookmarkStore {
	return &MockBookmarkStore{
		GetBookmarkFunc: &BookmarkStoreGetBookmarkFunc{
			defaultHook: func(context.Context) (int, error) {
				panic("unexpected invocation of MockBookmarkStore.GetBookmark")
			},
		},
		UpdateBookmarkFunc: &BookmarkStoreUpdateBookmarkFunc{
			defaultHook: func(context.Context, int) error {
				panic("unexpected invocation of MockBookmarkStore.UpdateBookmark")
			},
		},
	}
}

// surrogateMockBookmarkStore is a copy of the bookmarkStore interface (from
// the package
// github.com/sourcegraph/sourcegraph/enterprise/cmd/worker/internal/telemetry).
// It is redefined here as it is unexported in the source package.
type surrogateMockBookmarkStore interface {
	GetBookmark(context.Context) (int, error)
	UpdateBookmark(context.Context, int) error
}

// NewMockBookmarkStoreFrom creates a new mock of the MockBookmarkStore
// interface. All methods delegate to the given implementation, unless
// overwritten.
func NewMockBookmarkStoreFrom(i surrogateMockBookmarkStore) *MockBookmarkStore {
	return &MockBookmarkStore{
		GetBookmarkFunc: &BookmarkStoreGetBookmarkFunc{
			defaultHook: i.GetBookmark,
		},
		UpdateBookmarkFunc: &BookmarkStoreUpdateBookmarkFunc{
			defaultHook: i.UpdateBookmark,
		},
	}
}

// BookmarkStoreGetBookmarkFunc describes the behavior when the GetBookmark
// method of the parent MockBookmarkStore instance is invoked.
type BookmarkStoreGetBookmarkFunc struct {
	defaultHook func(context.Context) (int, error)
	hooks       []func(context.Context) (int, error)
	history     []BookmarkStoreGetBookmarkFuncCall
	mutex       sync.Mutex
}

// GetBookmark delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockBookmarkStore) GetBookmark(v0 context.Context) (int, error) {
	r0, r1 := m.GetBookmarkFunc.nextHook()(v0)
	m.GetBookmarkFunc.appendCall(BookmarkStoreGetBookmarkFuncCall{v0, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the GetBookmark method
// of the parent MockBookmarkStore instance is invoked and the hook queue is
// empty.
func (f *BookmarkStoreGetBookmarkFunc) SetDefaultHook(hook func(context.Context) (int, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// GetBookmark method of the parent MockBookmarkStore instance invokes the
// hook at the front of the queue and discards it. After the queue is empty,
// the default hook function is invoked for any future action.
func (f *BookmarkStoreGetBookmarkFunc) PushHook(hook func(context.Context) (int, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *BookmarkStoreGetBookmarkFunc) SetDefaultReturn(r0 int, r1 error) {
	f.SetDefaultHook(func(context.Context) (int, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *BookmarkStoreGetBookmarkFunc) PushReturn(r0 int, r1 error) {
	f.PushHook(func(context.Context) (int, error) {
		return r0, r1
	})
}

func (f *BookmarkStoreGetBookmarkFunc) nextHook() func(context.Context) (int, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *BookmarkStoreGetBookmarkFunc) appendCall(r0 BookmarkStoreGetBookmarkFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of BookmarkStoreGetBookmarkFuncCall objects
// describing the invocations of this function.
func (f *BookmarkStoreGetBookmarkFunc) History() []BookmarkStoreGetBookmarkFuncCall {
	f.mutex.Lock()
	history := make([]BookmarkStoreGetBookmarkFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// BookmarkStoreGetBookmarkFuncCall is an object that describes an
// invocation of method GetBookmark on an instance of MockBookmarkStore.
type BookmarkStoreGetBookmarkFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 int
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c BookmarkStoreGetBookmarkFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c BookmarkStoreGetBookmarkFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// BookmarkStoreUpdateBookmarkFunc describes the behavior when the
// UpdateBookmark method of the parent MockBookmarkStore instance is
// invoked.
type BookmarkStoreUpdateBookmarkFunc struct {
	defaultHook func(context.Context, int) error
	hooks       []func(context.Context, int) error
	history     []BookmarkStoreUpdateBookmarkFuncCall
	mutex       sync.Mutex
}

// UpdateBookmark delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockBookmarkStore) UpdateBookmark(v0 context.Context, v1 int) error {
	r0 := m.UpdateBookmarkFunc.nextHook()(v0, v1)
	m.UpdateBookmarkFunc.appendCall(BookmarkStoreUpdateBookmarkFuncCall{v0, v1, r0})
	return r0
}

// SetDefaultHook sets function that is called when the UpdateBookmark
// method of the parent MockBookmarkStore instance is invoked and the hook
// queue is empty.
func (f *BookmarkStoreUpdateBookmarkFunc) SetDefaultHook(hook func(context.Context, int) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// UpdateBookmark method of the parent MockBookmarkStore instance invokes
// the hook at the front of the queue and discards it. After the queue is
// empty, the default hook function is invoked for any future action.
func (f *BookmarkStoreUpdateBookmarkFunc) PushHook(hook func(context.Context, int) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *BookmarkStoreUpdateBookmarkFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, int) error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *BookmarkStoreUpdateBookmarkFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, int) error {
		return r0
	})
}

func (f *BookmarkStoreUpdateBookmarkFunc) nextHook() func(context.Context, int) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *BookmarkStoreUpdateBookmarkFunc) appendCall(r0 BookmarkStoreUpdateBookmarkFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of BookmarkStoreUpdateBookmarkFuncCall objects
// describing the invocations of this function.
func (f *BookmarkStoreUpdateBookmarkFunc) History() []BookmarkStoreUpdateBookmarkFuncCall {
	f.mutex.Lock()
	history := make([]BookmarkStoreUpdateBookmarkFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// BookmarkStoreUpdateBookmarkFuncCall is an object that describes an
// invocation of method UpdateBookmark on an instance of MockBookmarkStore.
type BookmarkStoreUpdateBookmarkFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c BookmarkStoreUpdateBookmarkFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c BookmarkStoreUpdateBookmarkFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}
