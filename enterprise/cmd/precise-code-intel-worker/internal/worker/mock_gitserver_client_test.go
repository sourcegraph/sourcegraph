// Code generated by github.com/efritz/go-mockgen 0.1.0; DO NOT EDIT.

package worker

import (
	"context"
	dbstore "github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/dbstore"
	"sync"
)

// MockGitserverClient is a mock implementation of the gitserverClient
// interface (from the package
// github.com/sourcegraph/sourcegraph/enterprise/cmd/precise-code-intel-worker/internal/worker)
// used for unit testing.
type MockGitserverClient struct {
	// DirectoryChildrenFunc is an instance of a mock function object
	// controlling the behavior of the method DirectoryChildren.
	DirectoryChildrenFunc *GitserverClientDirectoryChildrenFunc
}

// NewMockGitserverClient creates a new mock of the gitserverClient
// interface. All methods return zero values for all results, unless
// overwritten.
func NewMockGitserverClient() *MockGitserverClient {
	return &MockGitserverClient{
		DirectoryChildrenFunc: &GitserverClientDirectoryChildrenFunc{
			defaultHook: func(context.Context, dbstore.Store, int, string, []string) (map[string][]string, error) {
				return nil, nil
			},
		},
	}
}

// surrogateMockGitserverClient is a copy of the gitserverClient interface
// (from the package
// github.com/sourcegraph/sourcegraph/enterprise/cmd/precise-code-intel-worker/internal/worker).
// It is redefined here as it is unexported in the source packge.
type surrogateMockGitserverClient interface {
	DirectoryChildren(context.Context, dbstore.Store, int, string, []string) (map[string][]string, error)
}

// NewMockGitserverClientFrom creates a new mock of the MockGitserverClient
// interface. All methods delegate to the given implementation, unless
// overwritten.
func NewMockGitserverClientFrom(i surrogateMockGitserverClient) *MockGitserverClient {
	return &MockGitserverClient{
		DirectoryChildrenFunc: &GitserverClientDirectoryChildrenFunc{
			defaultHook: i.DirectoryChildren,
		},
	}
}

// GitserverClientDirectoryChildrenFunc describes the behavior when the
// DirectoryChildren method of the parent MockGitserverClient instance is
// invoked.
type GitserverClientDirectoryChildrenFunc struct {
	defaultHook func(context.Context, dbstore.Store, int, string, []string) (map[string][]string, error)
	hooks       []func(context.Context, dbstore.Store, int, string, []string) (map[string][]string, error)
	history     []GitserverClientDirectoryChildrenFuncCall
	mutex       sync.Mutex
}

// DirectoryChildren delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockGitserverClient) DirectoryChildren(v0 context.Context, v1 dbstore.Store, v2 int, v3 string, v4 []string) (map[string][]string, error) {
	r0, r1 := m.DirectoryChildrenFunc.nextHook()(v0, v1, v2, v3, v4)
	m.DirectoryChildrenFunc.appendCall(GitserverClientDirectoryChildrenFuncCall{v0, v1, v2, v3, v4, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the DirectoryChildren
// method of the parent MockGitserverClient instance is invoked and the hook
// queue is empty.
func (f *GitserverClientDirectoryChildrenFunc) SetDefaultHook(hook func(context.Context, dbstore.Store, int, string, []string) (map[string][]string, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// DirectoryChildren method of the parent MockGitserverClient instance
// inovkes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *GitserverClientDirectoryChildrenFunc) PushHook(hook func(context.Context, dbstore.Store, int, string, []string) (map[string][]string, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *GitserverClientDirectoryChildrenFunc) SetDefaultReturn(r0 map[string][]string, r1 error) {
	f.SetDefaultHook(func(context.Context, dbstore.Store, int, string, []string) (map[string][]string, error) {
		return r0, r1
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *GitserverClientDirectoryChildrenFunc) PushReturn(r0 map[string][]string, r1 error) {
	f.PushHook(func(context.Context, dbstore.Store, int, string, []string) (map[string][]string, error) {
		return r0, r1
	})
}

func (f *GitserverClientDirectoryChildrenFunc) nextHook() func(context.Context, dbstore.Store, int, string, []string) (map[string][]string, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *GitserverClientDirectoryChildrenFunc) appendCall(r0 GitserverClientDirectoryChildrenFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of GitserverClientDirectoryChildrenFuncCall
// objects describing the invocations of this function.
func (f *GitserverClientDirectoryChildrenFunc) History() []GitserverClientDirectoryChildrenFuncCall {
	f.mutex.Lock()
	history := make([]GitserverClientDirectoryChildrenFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// GitserverClientDirectoryChildrenFuncCall is an object that describes an
// invocation of method DirectoryChildren on an instance of
// MockGitserverClient.
type GitserverClientDirectoryChildrenFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 dbstore.Store
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 int
	// Arg3 is the value of the 4th argument passed to this method
	// invocation.
	Arg3 string
	// Arg4 is the value of the 5th argument passed to this method
	// invocation.
	Arg4 []string
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 map[string][]string
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c GitserverClientDirectoryChildrenFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3, c.Arg4}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c GitserverClientDirectoryChildrenFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}
