// Code generated by go-mockgen 1.3.7; DO NOT EDIT.
//
// This file was generated by running `sg generate` (or `go-mockgen`) at the root of
// this repository. To add additional mocks to this or another package, add a new entry
// to the mockgen.yaml file in the root of this repository.

package dotcom

import (
	"context"
	"sync"

	graphql "github.com/Khan/genqlient/graphql"
)

// MockClient is a mock implementation of the Client interface (from the
// package github.com/Khan/genqlient/graphql) used for unit testing.
type MockClient struct {
	// MakeRequestFunc is an instance of a mock function object controlling
	// the behavior of the method MakeRequest.
	MakeRequestFunc *ClientMakeRequestFunc
}

// NewMockClient creates a new mock of the Client interface. All methods
// return zero values for all results, unless overwritten.
func NewMockClient() *MockClient {
	return &MockClient{
		MakeRequestFunc: &ClientMakeRequestFunc{
			defaultHook: func(context.Context, *graphql.Request, *graphql.Response) (r0 error) {
				return
			},
		},
	}
}

// NewStrictMockClient creates a new mock of the Client interface. All
// methods panic on invocation, unless overwritten.
func NewStrictMockClient() *MockClient {
	return &MockClient{
		MakeRequestFunc: &ClientMakeRequestFunc{
			defaultHook: func(context.Context, *graphql.Request, *graphql.Response) error {
				panic("unexpected invocation of MockClient.MakeRequest")
			},
		},
	}
}

// NewMockClientFrom creates a new mock of the MockClient interface. All
// methods delegate to the given implementation, unless overwritten.
func NewMockClientFrom(i graphql.Client) *MockClient {
	return &MockClient{
		MakeRequestFunc: &ClientMakeRequestFunc{
			defaultHook: i.MakeRequest,
		},
	}
}

// ClientMakeRequestFunc describes the behavior when the MakeRequest method
// of the parent MockClient instance is invoked.
type ClientMakeRequestFunc struct {
	defaultHook func(context.Context, *graphql.Request, *graphql.Response) error
	hooks       []func(context.Context, *graphql.Request, *graphql.Response) error
	history     []ClientMakeRequestFuncCall
	mutex       sync.Mutex
}

// MakeRequest delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockClient) MakeRequest(v0 context.Context, v1 *graphql.Request, v2 *graphql.Response) error {
	r0 := m.MakeRequestFunc.nextHook()(v0, v1, v2)
	m.MakeRequestFunc.appendCall(ClientMakeRequestFuncCall{v0, v1, v2, r0})
	return r0
}

// SetDefaultHook sets function that is called when the MakeRequest method
// of the parent MockClient instance is invoked and the hook queue is empty.
func (f *ClientMakeRequestFunc) SetDefaultHook(hook func(context.Context, *graphql.Request, *graphql.Response) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// MakeRequest method of the parent MockClient instance invokes the hook at
// the front of the queue and discards it. After the queue is empty, the
// default hook function is invoked for any future action.
func (f *ClientMakeRequestFunc) PushHook(hook func(context.Context, *graphql.Request, *graphql.Response) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *ClientMakeRequestFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, *graphql.Request, *graphql.Response) error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *ClientMakeRequestFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, *graphql.Request, *graphql.Response) error {
		return r0
	})
}

func (f *ClientMakeRequestFunc) nextHook() func(context.Context, *graphql.Request, *graphql.Response) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *ClientMakeRequestFunc) appendCall(r0 ClientMakeRequestFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of ClientMakeRequestFuncCall objects
// describing the invocations of this function.
func (f *ClientMakeRequestFunc) History() []ClientMakeRequestFuncCall {
	f.mutex.Lock()
	history := make([]ClientMakeRequestFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// ClientMakeRequestFuncCall is an object that describes an invocation of
// method MakeRequest on an instance of MockClient.
type ClientMakeRequestFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 *graphql.Request
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 *graphql.Response
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c ClientMakeRequestFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c ClientMakeRequestFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}
