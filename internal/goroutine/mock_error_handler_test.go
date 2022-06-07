// Code generated by go-mockgen 1.3.1; DO NOT EDIT.
//
// This file was generated by running `sg generate` (or `go-mockgen`) at the root of
// this repository. To add additional mocks to this or another package, add a new entry
// to the metadata.yaml file in the root of this repository.

package goroutine

import "sync"

// MockErrorHandler is a mock implementation of the ErrorHandler interface
// (from the package github.com/sourcegraph/sourcegraph/internal/goroutine)
// used for unit testing.
type MockErrorHandler struct {
	// HandleErrorFunc is an instance of a mock function object controlling
	// the behavior of the method HandleError.
	HandleErrorFunc *ErrorHandlerHandleErrorFunc
}

// NewMockErrorHandler creates a new mock of the ErrorHandler interface. All
// methods return zero values for all results, unless overwritten.
func NewMockErrorHandler() *MockErrorHandler {
	return &MockErrorHandler{
		HandleErrorFunc: &ErrorHandlerHandleErrorFunc{
			defaultHook: func(error) {
				return
			},
		},
	}
}

// NewStrictMockErrorHandler creates a new mock of the ErrorHandler
// interface. All methods panic on invocation, unless overwritten.
func NewStrictMockErrorHandler() *MockErrorHandler {
	return &MockErrorHandler{
		HandleErrorFunc: &ErrorHandlerHandleErrorFunc{
			defaultHook: func(error) {
				panic("unexpected invocation of MockErrorHandler.HandleError")
			},
		},
	}
}

// NewMockErrorHandlerFrom creates a new mock of the MockErrorHandler
// interface. All methods delegate to the given implementation, unless
// overwritten.
func NewMockErrorHandlerFrom(i ErrorHandler) *MockErrorHandler {
	return &MockErrorHandler{
		HandleErrorFunc: &ErrorHandlerHandleErrorFunc{
			defaultHook: i.HandleError,
		},
	}
}

// ErrorHandlerHandleErrorFunc describes the behavior when the HandleError
// method of the parent MockErrorHandler instance is invoked.
type ErrorHandlerHandleErrorFunc struct {
	defaultHook func(error)
	hooks       []func(error)
	history     []ErrorHandlerHandleErrorFuncCall
	mutex       sync.Mutex
}

// HandleError delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockErrorHandler) HandleError(v0 error) {
	m.HandleErrorFunc.nextHook()(v0)
	m.HandleErrorFunc.appendCall(ErrorHandlerHandleErrorFuncCall{v0})
	return
}

// SetDefaultHook sets function that is called when the HandleError method
// of the parent MockErrorHandler instance is invoked and the hook queue is
// empty.
func (f *ErrorHandlerHandleErrorFunc) SetDefaultHook(hook func(error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// HandleError method of the parent MockErrorHandler instance invokes the
// hook at the front of the queue and discards it. After the queue is empty,
// the default hook function is invoked for any future action.
func (f *ErrorHandlerHandleErrorFunc) PushHook(hook func(error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *ErrorHandlerHandleErrorFunc) SetDefaultReturn() {
	f.SetDefaultHook(func(error) {
		return
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *ErrorHandlerHandleErrorFunc) PushReturn() {
	f.PushHook(func(error) {
		return
	})
}

func (f *ErrorHandlerHandleErrorFunc) nextHook() func(error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *ErrorHandlerHandleErrorFunc) appendCall(r0 ErrorHandlerHandleErrorFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of ErrorHandlerHandleErrorFuncCall objects
// describing the invocations of this function.
func (f *ErrorHandlerHandleErrorFunc) History() []ErrorHandlerHandleErrorFuncCall {
	f.mutex.Lock()
	history := make([]ErrorHandlerHandleErrorFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// ErrorHandlerHandleErrorFuncCall is an object that describes an invocation
// of method HandleError on an instance of MockErrorHandler.
type ErrorHandlerHandleErrorFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c ErrorHandlerHandleErrorFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c ErrorHandlerHandleErrorFuncCall) Results() []interface{} {
	return []interface{}{}
}
