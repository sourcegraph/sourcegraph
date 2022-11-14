// Code generated by go-mockgen 1.3.7; DO NOT EDIT.
//
// This file was generated by running `sg generate` (or `go-mockgen`) at the root of
// this repository. To add additional mocks to this or another package, add a new entry
// to the mockgen.yaml file in the root of this repository.

package api

import (
	"context"
	"io"
	"sync"

	gitserver "github.com/sourcegraph/sourcegraph/cmd/symbols/gitserver"
	api "github.com/sourcegraph/sourcegraph/internal/api"
	gitdomain "github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	types "github.com/sourcegraph/sourcegraph/internal/types"
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
	// LogReverseEachFunc is an instance of a mock function object
	// controlling the behavior of the method LogReverseEach.
	LogReverseEachFunc *GitserverClientLogReverseEachFunc
	// ReadFileFunc is an instance of a mock function object controlling the
	// behavior of the method ReadFile.
	ReadFileFunc *GitserverClientReadFileFunc
	// RevListFunc is an instance of a mock function object controlling the
	// behavior of the method RevList.
	RevListFunc *GitserverClientRevListFunc
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
		LogReverseEachFunc: &GitserverClientLogReverseEachFunc{
			defaultHook: func(context.Context, string, string, int, func(entry gitdomain.LogEntry) error) (r0 error) {
				return
			},
		},
		ReadFileFunc: &GitserverClientReadFileFunc{
			defaultHook: func(context.Context, types.RepoCommitPath) (r0 []byte, r1 error) {
				return
			},
		},
		RevListFunc: &GitserverClientRevListFunc{
			defaultHook: func(context.Context, string, string, func(commit string) (bool, error)) (r0 error) {
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
		LogReverseEachFunc: &GitserverClientLogReverseEachFunc{
			defaultHook: func(context.Context, string, string, int, func(entry gitdomain.LogEntry) error) error {
				panic("unexpected invocation of MockGitserverClient.LogReverseEach")
			},
		},
		ReadFileFunc: &GitserverClientReadFileFunc{
			defaultHook: func(context.Context, types.RepoCommitPath) ([]byte, error) {
				panic("unexpected invocation of MockGitserverClient.ReadFile")
			},
		},
		RevListFunc: &GitserverClientRevListFunc{
			defaultHook: func(context.Context, string, string, func(commit string) (bool, error)) error {
				panic("unexpected invocation of MockGitserverClient.RevList")
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
		LogReverseEachFunc: &GitserverClientLogReverseEachFunc{
			defaultHook: i.LogReverseEach,
		},
		ReadFileFunc: &GitserverClientReadFileFunc{
			defaultHook: i.ReadFile,
		},
		RevListFunc: &GitserverClientRevListFunc{
			defaultHook: i.RevList,
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

// GitserverClientLogReverseEachFunc describes the behavior when the
// LogReverseEach method of the parent MockGitserverClient instance is
// invoked.
type GitserverClientLogReverseEachFunc struct {
	defaultHook func(context.Context, string, string, int, func(entry gitdomain.LogEntry) error) error
	hooks       []func(context.Context, string, string, int, func(entry gitdomain.LogEntry) error) error
	history     []GitserverClientLogReverseEachFuncCall
	mutex       sync.Mutex
}

// LogReverseEach delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockGitserverClient) LogReverseEach(v0 context.Context, v1 string, v2 string, v3 int, v4 func(entry gitdomain.LogEntry) error) error {
	r0 := m.LogReverseEachFunc.nextHook()(v0, v1, v2, v3, v4)
	m.LogReverseEachFunc.appendCall(GitserverClientLogReverseEachFuncCall{v0, v1, v2, v3, v4, r0})
	return r0
}

// SetDefaultHook sets function that is called when the LogReverseEach
// method of the parent MockGitserverClient instance is invoked and the hook
// queue is empty.
func (f *GitserverClientLogReverseEachFunc) SetDefaultHook(hook func(context.Context, string, string, int, func(entry gitdomain.LogEntry) error) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// LogReverseEach method of the parent MockGitserverClient instance invokes
// the hook at the front of the queue and discards it. After the queue is
// empty, the default hook function is invoked for any future action.
func (f *GitserverClientLogReverseEachFunc) PushHook(hook func(context.Context, string, string, int, func(entry gitdomain.LogEntry) error) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *GitserverClientLogReverseEachFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, string, string, int, func(entry gitdomain.LogEntry) error) error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *GitserverClientLogReverseEachFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, string, string, int, func(entry gitdomain.LogEntry) error) error {
		return r0
	})
}

func (f *GitserverClientLogReverseEachFunc) nextHook() func(context.Context, string, string, int, func(entry gitdomain.LogEntry) error) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *GitserverClientLogReverseEachFunc) appendCall(r0 GitserverClientLogReverseEachFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of GitserverClientLogReverseEachFuncCall
// objects describing the invocations of this function.
func (f *GitserverClientLogReverseEachFunc) History() []GitserverClientLogReverseEachFuncCall {
	f.mutex.Lock()
	history := make([]GitserverClientLogReverseEachFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// GitserverClientLogReverseEachFuncCall is an object that describes an
// invocation of method LogReverseEach on an instance of
// MockGitserverClient.
type GitserverClientLogReverseEachFuncCall struct {
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
	Arg3 int
	// Arg4 is the value of the 5th argument passed to this method
	// invocation.
	Arg4 func(entry gitdomain.LogEntry) error
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c GitserverClientLogReverseEachFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3, c.Arg4}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c GitserverClientLogReverseEachFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// GitserverClientReadFileFunc describes the behavior when the ReadFile
// method of the parent MockGitserverClient instance is invoked.
type GitserverClientReadFileFunc struct {
	defaultHook func(context.Context, types.RepoCommitPath) ([]byte, error)
	hooks       []func(context.Context, types.RepoCommitPath) ([]byte, error)
	history     []GitserverClientReadFileFuncCall
	mutex       sync.Mutex
}

// ReadFile delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockGitserverClient) ReadFile(v0 context.Context, v1 types.RepoCommitPath) ([]byte, error) {
	r0, r1 := m.ReadFileFunc.nextHook()(v0, v1)
	m.ReadFileFunc.appendCall(GitserverClientReadFileFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the ReadFile method of
// the parent MockGitserverClient instance is invoked and the hook queue is
// empty.
func (f *GitserverClientReadFileFunc) SetDefaultHook(hook func(context.Context, types.RepoCommitPath) ([]byte, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// ReadFile method of the parent MockGitserverClient instance invokes the
// hook at the front of the queue and discards it. After the queue is empty,
// the default hook function is invoked for any future action.
func (f *GitserverClientReadFileFunc) PushHook(hook func(context.Context, types.RepoCommitPath) ([]byte, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *GitserverClientReadFileFunc) SetDefaultReturn(r0 []byte, r1 error) {
	f.SetDefaultHook(func(context.Context, types.RepoCommitPath) ([]byte, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *GitserverClientReadFileFunc) PushReturn(r0 []byte, r1 error) {
	f.PushHook(func(context.Context, types.RepoCommitPath) ([]byte, error) {
		return r0, r1
	})
}

func (f *GitserverClientReadFileFunc) nextHook() func(context.Context, types.RepoCommitPath) ([]byte, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *GitserverClientReadFileFunc) appendCall(r0 GitserverClientReadFileFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of GitserverClientReadFileFuncCall objects
// describing the invocations of this function.
func (f *GitserverClientReadFileFunc) History() []GitserverClientReadFileFuncCall {
	f.mutex.Lock()
	history := make([]GitserverClientReadFileFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// GitserverClientReadFileFuncCall is an object that describes an invocation
// of method ReadFile on an instance of MockGitserverClient.
type GitserverClientReadFileFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 types.RepoCommitPath
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []byte
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c GitserverClientReadFileFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c GitserverClientReadFileFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// GitserverClientRevListFunc describes the behavior when the RevList method
// of the parent MockGitserverClient instance is invoked.
type GitserverClientRevListFunc struct {
	defaultHook func(context.Context, string, string, func(commit string) (bool, error)) error
	hooks       []func(context.Context, string, string, func(commit string) (bool, error)) error
	history     []GitserverClientRevListFuncCall
	mutex       sync.Mutex
}

// RevList delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockGitserverClient) RevList(v0 context.Context, v1 string, v2 string, v3 func(commit string) (bool, error)) error {
	r0 := m.RevListFunc.nextHook()(v0, v1, v2, v3)
	m.RevListFunc.appendCall(GitserverClientRevListFuncCall{v0, v1, v2, v3, r0})
	return r0
}

// SetDefaultHook sets function that is called when the RevList method of
// the parent MockGitserverClient instance is invoked and the hook queue is
// empty.
func (f *GitserverClientRevListFunc) SetDefaultHook(hook func(context.Context, string, string, func(commit string) (bool, error)) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// RevList method of the parent MockGitserverClient instance invokes the
// hook at the front of the queue and discards it. After the queue is empty,
// the default hook function is invoked for any future action.
func (f *GitserverClientRevListFunc) PushHook(hook func(context.Context, string, string, func(commit string) (bool, error)) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *GitserverClientRevListFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(context.Context, string, string, func(commit string) (bool, error)) error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *GitserverClientRevListFunc) PushReturn(r0 error) {
	f.PushHook(func(context.Context, string, string, func(commit string) (bool, error)) error {
		return r0
	})
}

func (f *GitserverClientRevListFunc) nextHook() func(context.Context, string, string, func(commit string) (bool, error)) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *GitserverClientRevListFunc) appendCall(r0 GitserverClientRevListFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of GitserverClientRevListFuncCall objects
// describing the invocations of this function.
func (f *GitserverClientRevListFunc) History() []GitserverClientRevListFuncCall {
	f.mutex.Lock()
	history := make([]GitserverClientRevListFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// GitserverClientRevListFuncCall is an object that describes an invocation
// of method RevList on an instance of MockGitserverClient.
type GitserverClientRevListFuncCall struct {
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
	Arg3 func(commit string) (bool, error)
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c GitserverClientRevListFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c GitserverClientRevListFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}
