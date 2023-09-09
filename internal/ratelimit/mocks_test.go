// Code generated by go-mockgen 1.3.7; DO NOT EDIT.
//
// This file was generated by running `sg generate` (or `go-mockgen`) at the root of
// this repository. To add additional mocks to this or another package, add a new entry
// to the mockgen.yaml file in the root of this repository.

package ratelimit

import (
	"context"
	"sync"
	"time"
)

// MockGlobalLimiter is a mock implementation of the GlobalLimiter interface
// (from the package github.com/sourcegraph/sourcegraph/internal/ratelimit)
// used for unit testing.
type MockGlobalLimiter struct {
	// SetTokenBucketConfigFunc is an instance of a mock function object
	// controlling the behavior of the method SetTokenBucketConfig.
	SetTokenBucketConfigFunc *GlobalLimiterSetTokenBucketConfigFunc
	// WaitFunc is an instance of a mock function object controlling the
	// behavior of the method Wait.
	WaitFunc *GlobalLimiterWaitFunc
	// WaitNFunc is an instance of a mock function object controlling the
	// behavior of the method WaitN.
	WaitNFunc *GlobalLimiterWaitNFunc
}

// NewMockGlobalLimiter creates a new mock of the GlobalLimiter interface.
// All methods return zero values for all results, unless overwritten.
func NewMockGlobalLimiter() *MockGlobalLimiter {
	return &MockGlobalLimiter{
		SetTokenBucketConfigFunc: &GlobalLimiterSetTokenBucketConfigFunc{
			defaultHook: func(context.Context, int32, time.Duration) (r0 error) {
				return
			},
		},
		WaitFunc: &GlobalLimiterWaitFunc{
			defaultHook: func(context.Context) (r0 error) {
				return
			},
		},
		WaitNFunc: &GlobalLimiterWaitNFunc{
			defaultHook: func(context.Context, int) (r0 error) {
				return
			},
		},
	}
}

// NewStrictMockGlobalLimiter creates a new mock of the GlobalLimiter
// interface. All methods panic on invocation, unless overwritten.
func NewStrictMockGlobalLimiter() *MockGlobalLimiter {
	return &MockGlobalLimiter{
		SetTokenBucketConfigFunc: &GlobalLimiterSetTokenBucketConfigFunc{
			defaultHook: func(context.Context, int32, time.Duration) error {
				panic("unexpected invocation of MockGlobalLimiter.SetTokenBucketConfig")
			},
		},
		WaitFunc: &GlobalLimiterWaitFunc{
			defaultHook: func(context.Context) error {
				panic("unexpected invocation of MockGlobalLimiter.Wait")
			},
		},
		WaitNFunc: &GlobalLimiterWaitNFunc{
			defaultHook: func(context.Context, int) error {
				panic("unexpected invocation of MockGlobalLimiter.WaitN")
			},
		},
	}
}

// NewMockGlobalLimiterFrom creates a new mock of the MockGlobalLimiter
// interface. All methods delegate to the given implementation, unless
// overwritten.
func NewMockGlobalLimiterFrom(i GlobalLimiter) *MockGlobalLimiter {
	return &MockGlobalLimiter{
		SetTokenBucketConfigFunc: &GlobalLimiterSetTokenBucketConfigFunc{
			defaultHook: i.SetTokenBucketConfig,
		},
		WaitFunc: &GlobalLimiterWaitFunc{
			defaultHook: i.Wait,
		},
		WaitNFunc: &GlobalLimiterWaitNFunc{
			defaultHook: i.WaitN,
		},
	}
}

// GlobalLimiterSetTokenBucketConfigFunc describes the behavior when the
// SetTokenBucketConfig method of the parent MockGlobalLimiter instance is
// invoked.
type GlobalLimiterSetTokenBucketConfigFunc struct {
	defaultHook func(context.Context, int32, time.Duration) error
	hooks       []func(context.Context, int32, time.Duration) error
	history     []GlobalLimiterSetTokenBucketConfigFuncCall
	mutex       sync.Mutex
}

// SetTokenBucketConfig delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockGlobalLimiter) SetTokenBucketConfig(v0 context.Context, v1 int32, v2 time.Duration) error {
	r0 := m.SetTokenBucketConfigFunc.nextHook()(v0, v1, v2)
	m.SetTokenBucketConfigFunc.appendCall(GlobalLimiterSetTokenBucketConfigFuncCall{v0, v1, v2, r0})
	return r0
}

// SetDefaultHook sets function that is called when the SetTokenBucketConfig
// method of the parent MockGlobalLimiter instance is invoked and the hook
// queue is empty.
func (f *GlobalLimiterSetTokenBucketConfigFunc) SetDefaultHook(hook func(context.Context, int32, time.Duration) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// SetTokenBucketConfig method of the parent MockGlobalLimiter instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *GlobalLimiterSetTokenBucketConfigFunc) PushHook(hook func(context.Context, int32, time.Duration) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *GlobalLimiterSetTokenBucketConfigFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, int32, time.Duration) error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *GlobalLimiterSetTokenBucketConfigFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, int32, time.Duration) error {
		return r0
	})
}

func (f *GlobalLimiterSetTokenBucketConfigFunc) nextHook() func(context.Context, int32, time.Duration) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *GlobalLimiterSetTokenBucketConfigFunc) appendCall(r0 GlobalLimiterSetTokenBucketConfigFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of GlobalLimiterSetTokenBucketConfigFuncCall
// objects describing the invocations of this function.
func (f *GlobalLimiterSetTokenBucketConfigFunc) History() []GlobalLimiterSetTokenBucketConfigFuncCall {
	f.mutex.Lock()
	history := make([]GlobalLimiterSetTokenBucketConfigFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// GlobalLimiterSetTokenBucketConfigFuncCall is an object that describes an
// invocation of method SetTokenBucketConfig on an instance of
// MockGlobalLimiter.
type GlobalLimiterSetTokenBucketConfigFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int32
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 time.Duration
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c GlobalLimiterSetTokenBucketConfigFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c GlobalLimiterSetTokenBucketConfigFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// GlobalLimiterWaitFunc describes the behavior when the Wait method of the
// parent MockGlobalLimiter instance is invoked.
type GlobalLimiterWaitFunc struct {
	defaultHook func(context.Context) error
	hooks       []func(context.Context) error
	history     []GlobalLimiterWaitFuncCall
	mutex       sync.Mutex
}

// Wait delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockGlobalLimiter) Wait(v0 context.Context) error {
	r0 := m.WaitFunc.nextHook()(v0)
	m.WaitFunc.appendCall(GlobalLimiterWaitFuncCall{v0, r0})
	return r0
}

// SetDefaultHook sets function that is called when the Wait method of the
// parent MockGlobalLimiter instance is invoked and the hook queue is empty.
func (f *GlobalLimiterWaitFunc) SetDefaultHook(hook func(context.Context) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Wait method of the parent MockGlobalLimiter instance invokes the hook at
// the front of the queue and discards it. After the queue is empty, the
// default hook function is invoked for any future action.
func (f *GlobalLimiterWaitFunc) PushHook(hook func(context.Context) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *GlobalLimiterWaitFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context) error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *GlobalLimiterWaitFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context) error {
		return r0
	})
}

func (f *GlobalLimiterWaitFunc) nextHook() func(context.Context) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *GlobalLimiterWaitFunc) appendCall(r0 GlobalLimiterWaitFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of GlobalLimiterWaitFuncCall objects
// describing the invocations of this function.
func (f *GlobalLimiterWaitFunc) History() []GlobalLimiterWaitFuncCall {
	f.mutex.Lock()
	history := make([]GlobalLimiterWaitFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// GlobalLimiterWaitFuncCall is an object that describes an invocation of
// method Wait on an instance of MockGlobalLimiter.
type GlobalLimiterWaitFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c GlobalLimiterWaitFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c GlobalLimiterWaitFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// GlobalLimiterWaitNFunc describes the behavior when the WaitN method of
// the parent MockGlobalLimiter instance is invoked.
type GlobalLimiterWaitNFunc struct {
	defaultHook func(context.Context, int) error
	hooks       []func(context.Context, int) error
	history     []GlobalLimiterWaitNFuncCall
	mutex       sync.Mutex
}

// WaitN delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockGlobalLimiter) WaitN(v0 context.Context, v1 int) error {
	r0 := m.WaitNFunc.nextHook()(v0, v1)
	m.WaitNFunc.appendCall(GlobalLimiterWaitNFuncCall{v0, v1, r0})
	return r0
}

// SetDefaultHook sets function that is called when the WaitN method of the
// parent MockGlobalLimiter instance is invoked and the hook queue is empty.
func (f *GlobalLimiterWaitNFunc) SetDefaultHook(hook func(context.Context, int) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// WaitN method of the parent MockGlobalLimiter instance invokes the hook at
// the front of the queue and discards it. After the queue is empty, the
// default hook function is invoked for any future action.
func (f *GlobalLimiterWaitNFunc) PushHook(hook func(context.Context, int) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *GlobalLimiterWaitNFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, int) error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *GlobalLimiterWaitNFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, int) error {
		return r0
	})
}

func (f *GlobalLimiterWaitNFunc) nextHook() func(context.Context, int) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *GlobalLimiterWaitNFunc) appendCall(r0 GlobalLimiterWaitNFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of GlobalLimiterWaitNFuncCall objects
// describing the invocations of this function.
func (f *GlobalLimiterWaitNFunc) History() []GlobalLimiterWaitNFuncCall {
	f.mutex.Lock()
	history := make([]GlobalLimiterWaitNFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// GlobalLimiterWaitNFuncCall is an object that describes an invocation of
// method WaitN on an instance of MockGlobalLimiter.
type GlobalLimiterWaitNFuncCall struct {
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
func (c GlobalLimiterWaitNFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c GlobalLimiterWaitNFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}
