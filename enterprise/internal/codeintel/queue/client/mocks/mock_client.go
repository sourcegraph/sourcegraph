// Code generated by github.com/efritz/go-mockgen 0.1.0; DO NOT EDIT.

package mocks

import (
	"context"
	client "github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/queue/client"
	store "github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/store"
	"sync"
)

// MockClient is a mock implementation of the Client interface (from the
// package
// github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/queue/client)
// used for unit testing.
type MockClient struct {
	// CompleteFunc is an instance of a mock function object controlling the
	// behavior of the method Complete.
	CompleteFunc *ClientCompleteFunc
	// DequeueFunc is an instance of a mock function object controlling the
	// behavior of the method Dequeue.
	DequeueFunc *ClientDequeueFunc
	// HeartbeatFunc is an instance of a mock function object controlling
	// the behavior of the method Heartbeat.
	HeartbeatFunc *ClientHeartbeatFunc
}

// NewMockClient creates a new mock of the Client interface. All methods
// return zero values for all results, unless overwritten.
func NewMockClient() *MockClient {
	return &MockClient{
		CompleteFunc: &ClientCompleteFunc{
			defaultHook: func(context.Context, int, error) error {
				return nil
			},
		},
		DequeueFunc: &ClientDequeueFunc{
			defaultHook: func(context.Context) (store.Index, bool, error) {
				return store.Index{}, false, nil
			},
		},
		HeartbeatFunc: &ClientHeartbeatFunc{
			defaultHook: func(context.Context, []int) error {
				return nil
			},
		},
	}
}

// NewMockClientFrom creates a new mock of the MockClient interface. All
// methods delegate to the given implementation, unless overwritten.
func NewMockClientFrom(i client.Client) *MockClient {
	return &MockClient{
		CompleteFunc: &ClientCompleteFunc{
			defaultHook: i.Complete,
		},
		DequeueFunc: &ClientDequeueFunc{
			defaultHook: i.Dequeue,
		},
		HeartbeatFunc: &ClientHeartbeatFunc{
			defaultHook: i.Heartbeat,
		},
	}
}

// ClientCompleteFunc describes the behavior when the Complete method of the
// parent MockClient instance is invoked.
type ClientCompleteFunc struct {
	defaultHook func(context.Context, int, error) error
	hooks       []func(context.Context, int, error) error
	history     []ClientCompleteFuncCall
	mutex       sync.Mutex
}

// Complete delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockClient) Complete(v0 context.Context, v1 int, v2 error) error {
	r0 := m.CompleteFunc.nextHook()(v0, v1, v2)
	m.CompleteFunc.appendCall(ClientCompleteFuncCall{v0, v1, v2, r0})
	return r0
}

// SetDefaultHook sets function that is called when the Complete method of
// the parent MockClient instance is invoked and the hook queue is empty.
func (f *ClientCompleteFunc) SetDefaultHook(hook func(context.Context, int, error) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Complete method of the parent MockClient instance inovkes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *ClientCompleteFunc) PushHook(hook func(context.Context, int, error) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *ClientCompleteFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, int, error) error {
		return r0
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *ClientCompleteFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, int, error) error {
		return r0
	})
}

func (f *ClientCompleteFunc) nextHook() func(context.Context, int, error) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *ClientCompleteFunc) appendCall(r0 ClientCompleteFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of ClientCompleteFuncCall objects describing
// the invocations of this function.
func (f *ClientCompleteFunc) History() []ClientCompleteFuncCall {
	f.mutex.Lock()
	history := make([]ClientCompleteFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// ClientCompleteFuncCall is an object that describes an invocation of
// method Complete on an instance of MockClient.
type ClientCompleteFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 error
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c ClientCompleteFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c ClientCompleteFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// ClientDequeueFunc describes the behavior when the Dequeue method of the
// parent MockClient instance is invoked.
type ClientDequeueFunc struct {
	defaultHook func(context.Context) (store.Index, bool, error)
	hooks       []func(context.Context) (store.Index, bool, error)
	history     []ClientDequeueFuncCall
	mutex       sync.Mutex
}

// Dequeue delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockClient) Dequeue(v0 context.Context) (store.Index, bool, error) {
	r0, r1, r2 := m.DequeueFunc.nextHook()(v0)
	m.DequeueFunc.appendCall(ClientDequeueFuncCall{v0, r0, r1, r2})
	return r0, r1, r2
}

// SetDefaultHook sets function that is called when the Dequeue method of
// the parent MockClient instance is invoked and the hook queue is empty.
func (f *ClientDequeueFunc) SetDefaultHook(hook func(context.Context) (store.Index, bool, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Dequeue method of the parent MockClient instance inovkes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *ClientDequeueFunc) PushHook(hook func(context.Context) (store.Index, bool, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *ClientDequeueFunc) SetDefaultReturn(r0 store.Index, r1 bool, r2 error) {
	f.SetDefaultHook(func(context.Context) (store.Index, bool, error) {
		return r0, r1, r2
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *ClientDequeueFunc) PushReturn(r0 store.Index, r1 bool, r2 error) {
	f.PushHook(func(context.Context) (store.Index, bool, error) {
		return r0, r1, r2
	})
}

func (f *ClientDequeueFunc) nextHook() func(context.Context) (store.Index, bool, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *ClientDequeueFunc) appendCall(r0 ClientDequeueFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of ClientDequeueFuncCall objects describing
// the invocations of this function.
func (f *ClientDequeueFunc) History() []ClientDequeueFuncCall {
	f.mutex.Lock()
	history := make([]ClientDequeueFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// ClientDequeueFuncCall is an object that describes an invocation of method
// Dequeue on an instance of MockClient.
type ClientDequeueFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 store.Index
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 bool
	// Result2 is the value of the 3rd result returned from this method
	// invocation.
	Result2 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c ClientDequeueFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c ClientDequeueFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1, c.Result2}
}

// ClientHeartbeatFunc describes the behavior when the Heartbeat method of
// the parent MockClient instance is invoked.
type ClientHeartbeatFunc struct {
	defaultHook func(context.Context, []int) error
	hooks       []func(context.Context, []int) error
	history     []ClientHeartbeatFuncCall
	mutex       sync.Mutex
}

// Heartbeat delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockClient) Heartbeat(v0 context.Context, v1 []int) error {
	r0 := m.HeartbeatFunc.nextHook()(v0, v1)
	m.HeartbeatFunc.appendCall(ClientHeartbeatFuncCall{v0, v1, r0})
	return r0
}

// SetDefaultHook sets function that is called when the Heartbeat method of
// the parent MockClient instance is invoked and the hook queue is empty.
func (f *ClientHeartbeatFunc) SetDefaultHook(hook func(context.Context, []int) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Heartbeat method of the parent MockClient instance inovkes the hook at
// the front of the queue and discards it. After the queue is empty, the
// default hook function is invoked for any future action.
func (f *ClientHeartbeatFunc) PushHook(hook func(context.Context, []int) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *ClientHeartbeatFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, []int) error {
		return r0
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *ClientHeartbeatFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, []int) error {
		return r0
	})
}

func (f *ClientHeartbeatFunc) nextHook() func(context.Context, []int) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *ClientHeartbeatFunc) appendCall(r0 ClientHeartbeatFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of ClientHeartbeatFuncCall objects describing
// the invocations of this function.
func (f *ClientHeartbeatFunc) History() []ClientHeartbeatFuncCall {
	f.mutex.Lock()
	history := make([]ClientHeartbeatFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// ClientHeartbeatFuncCall is an object that describes an invocation of
// method Heartbeat on an instance of MockClient.
type ClientHeartbeatFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 []int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c ClientHeartbeatFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c ClientHeartbeatFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}
