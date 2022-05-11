// Code generated by go-mockgen 1.2.0; DO NOT EDIT.

package api

import (
	"context"
	"io"
	"sync"

	gitserver "github.com/sourcegraph/sourcegraph/cmd/symbols/gitserver"
	api "github.com/sourcegraph/sourcegraph/internal/api"
)

// MockGitserverClient is a mock implementation of the GitserverClient
// interface (from the package
// github.com/sourcegraph/sourcegraph/cmd/symbols/gitserver) used for unit
// testing.
type MockGitserverClient struct {
	// FetchTarFunc is an instance of a mock function object controlling the
	// behavior of the method FetchTar.
	FetchTarFunc *GitserverClientFetchTarFunc
	// GitDiffFunc is an instance of a mock function object controlling the
	// behavior of the method GitDiff.
	GitDiffFunc *GitserverClientGitDiffFunc
}

// NewMockGitserverClient creates a new mock of the GitserverClient
// interface. All methods return zero values for all results, unless
// overwritten.
func NewMockGitserverClient() *MockGitserverClient {
	return &MockGitserverClient{
		FetchTarFunc: &GitserverClientFetchTarFunc{
			defaultHook: func(context.Context, api.RepoName, api.CommitID, []string) (r0 io.ReadCloser, r1 error) {
				return
			},
		},
		GitDiffFunc: &GitserverClientGitDiffFunc{
			defaultHook: func(context.Context, api.RepoName, api.CommitID, api.CommitID) (r0 gitserver.Changes, r1 error) {
				return
			},
		},
	}
}

// NewStrictMockGitserverClient creates a new mock of the GitserverClient
// interface. All methods panic on invocation, unless overwritten.
func NewStrictMockGitserverClient() *MockGitserverClient {
	return &MockGitserverClient{
		FetchTarFunc: &GitserverClientFetchTarFunc{
			defaultHook: func(context.Context, api.RepoName, api.CommitID, []string) (io.ReadCloser, error) {
				panic("unexpected invocation of MockGitserverClient.FetchTar")
			},
		},
		GitDiffFunc: &GitserverClientGitDiffFunc{
			defaultHook: func(context.Context, api.RepoName, api.CommitID, api.CommitID) (gitserver.Changes, error) {
				panic("unexpected invocation of MockGitserverClient.GitDiff")
			},
		},
	}
}

// NewMockGitserverClientFrom creates a new mock of the MockGitserverClient
// interface. All methods delegate to the given implementation, unless
// overwritten.
func NewMockGitserverClientFrom(i gitserver.GitserverClient) *MockGitserverClient {
	return &MockGitserverClient{
		FetchTarFunc: &GitserverClientFetchTarFunc{
			defaultHook: i.FetchTar,
		},
		GitDiffFunc: &GitserverClientGitDiffFunc{
			defaultHook: i.GitDiff,
		},
	}
}

// GitserverClientFetchTarFunc describes the behavior when the FetchTar
// method of the parent MockGitserverClient instance is invoked.
type GitserverClientFetchTarFunc struct {
	defaultHook func(context.Context, api.RepoName, api.CommitID, []string) (io.ReadCloser, error)
	hooks       []func(context.Context, api.RepoName, api.CommitID, []string) (io.ReadCloser, error)
	history     []GitserverClientFetchTarFuncCall
	mutex       sync.Mutex
}

// FetchTar delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockGitserverClient) FetchTar(v0 context.Context, v1 api.RepoName, v2 api.CommitID, v3 []string) (io.ReadCloser, error) {
	r0, r1 := m.FetchTarFunc.nextHook()(v0, v1, v2, v3)
	m.FetchTarFunc.appendCall(GitserverClientFetchTarFuncCall{v0, v1, v2, v3, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the FetchTar method of
// the parent MockGitserverClient instance is invoked and the hook queue is
// empty.
func (f *GitserverClientFetchTarFunc) SetDefaultHook(hook func(context.Context, api.RepoName, api.CommitID, []string) (io.ReadCloser, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// FetchTar method of the parent MockGitserverClient instance invokes the
// hook at the front of the queue and discards it. After the queue is empty,
// the default hook function is invoked for any future action.
func (f *GitserverClientFetchTarFunc) PushHook(hook func(context.Context, api.RepoName, api.CommitID, []string) (io.ReadCloser, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *GitserverClientFetchTarFunc) SetDefaultReturn(r0 io.ReadCloser, r1 error) {
	f.SetDefaultHook(func(context.Context, api.RepoName, api.CommitID, []string) (io.ReadCloser, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *GitserverClientFetchTarFunc) PushReturn(r0 io.ReadCloser, r1 error) {
	f.PushHook(func(context.Context, api.RepoName, api.CommitID, []string) (io.ReadCloser, error) {
		return r0, r1
	})
}

func (f *GitserverClientFetchTarFunc) nextHook() func(context.Context, api.RepoName, api.CommitID, []string) (io.ReadCloser, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *GitserverClientFetchTarFunc) appendCall(r0 GitserverClientFetchTarFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of GitserverClientFetchTarFuncCall objects
// describing the invocations of this function.
func (f *GitserverClientFetchTarFunc) History() []GitserverClientFetchTarFuncCall {
	f.mutex.Lock()
	history := make([]GitserverClientFetchTarFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// GitserverClientFetchTarFuncCall is an object that describes an invocation
// of method FetchTar on an instance of MockGitserverClient.
type GitserverClientFetchTarFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 api.RepoName
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 api.CommitID
	// Arg3 is the value of the 4th argument passed to this method
	// invocation.
	Arg3 []string
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 io.ReadCloser
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c GitserverClientFetchTarFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c GitserverClientFetchTarFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// GitserverClientGitDiffFunc describes the behavior when the GitDiff method
// of the parent MockGitserverClient instance is invoked.
type GitserverClientGitDiffFunc struct {
	defaultHook func(context.Context, api.RepoName, api.CommitID, api.CommitID) (gitserver.Changes, error)
	hooks       []func(context.Context, api.RepoName, api.CommitID, api.CommitID) (gitserver.Changes, error)
	history     []GitserverClientGitDiffFuncCall
	mutex       sync.Mutex
}

// GitDiff delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockGitserverClient) GitDiff(v0 context.Context, v1 api.RepoName, v2 api.CommitID, v3 api.CommitID) (gitserver.Changes, error) {
	r0, r1 := m.GitDiffFunc.nextHook()(v0, v1, v2, v3)
	m.GitDiffFunc.appendCall(GitserverClientGitDiffFuncCall{v0, v1, v2, v3, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the GitDiff method of
// the parent MockGitserverClient instance is invoked and the hook queue is
// empty.
func (f *GitserverClientGitDiffFunc) SetDefaultHook(hook func(context.Context, api.RepoName, api.CommitID, api.CommitID) (gitserver.Changes, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// GitDiff method of the parent MockGitserverClient instance invokes the
// hook at the front of the queue and discards it. After the queue is empty,
// the default hook function is invoked for any future action.
func (f *GitserverClientGitDiffFunc) PushHook(hook func(context.Context, api.RepoName, api.CommitID, api.CommitID) (gitserver.Changes, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *GitserverClientGitDiffFunc) SetDefaultReturn(r0 gitserver.Changes, r1 error) {
	f.SetDefaultHook(func(context.Context, api.RepoName, api.CommitID, api.CommitID) (gitserver.Changes, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *GitserverClientGitDiffFunc) PushReturn(r0 gitserver.Changes, r1 error) {
	f.PushHook(func(context.Context, api.RepoName, api.CommitID, api.CommitID) (gitserver.Changes, error) {
		return r0, r1
	})
}

func (f *GitserverClientGitDiffFunc) nextHook() func(context.Context, api.RepoName, api.CommitID, api.CommitID) (gitserver.Changes, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *GitserverClientGitDiffFunc) appendCall(r0 GitserverClientGitDiffFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of GitserverClientGitDiffFuncCall objects
// describing the invocations of this function.
func (f *GitserverClientGitDiffFunc) History() []GitserverClientGitDiffFuncCall {
	f.mutex.Lock()
	history := make([]GitserverClientGitDiffFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// GitserverClientGitDiffFuncCall is an object that describes an invocation
// of method GitDiff on an instance of MockGitserverClient.
type GitserverClientGitDiffFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 api.RepoName
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 api.CommitID
	// Arg3 is the value of the 4th argument passed to this method
	// invocation.
	Arg3 api.CommitID
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 gitserver.Changes
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c GitserverClientGitDiffFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c GitserverClientGitDiffFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}
