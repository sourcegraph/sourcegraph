// Code generated by github.com/efritz/go-mockgen 0.1.0; DO NOT EDIT.

package inference

import (
	"context"
	"regexp"
	"sync"

	api "github.com/sourcegraph/sourcegraph/internal/api"
	protocol "github.com/sourcegraph/sourcegraph/internal/repoupdater/protocol"
)

// MockGitserverClientWrapper is a mock implementation of the
// GitserverClientWrapper interface (from the package
// github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/autoindex/inference)
// used for unit testing.
type MockGitserverClientWrapper struct {
	// FileExistsFunc is an instance of a mock function object controlling
	// the behavior of the method FileExists.
	FileExistsFunc *GitserverClientWrapperFileExistsFunc
	// ListFilesFunc is an instance of a mock function object controlling
	// the behavior of the method ListFiles.
	ListFilesFunc *GitserverClientWrapperListFilesFunc
	// RawContentsFunc is an instance of a mock function object controlling
	// the behavior of the method RawContents.
	RawContentsFunc *GitserverClientWrapperRawContentsFunc
}

// NewMockGitserverClientWrapper creates a new mock of the
// GitserverClientWrapper interface. All methods return zero values for all
// results, unless overwritten.
func NewMockGitserverClientWrapper() *MockGitserverClientWrapper {
	return &MockGitserverClientWrapper{
		FileExistsFunc: &GitserverClientWrapperFileExistsFunc{
			defaultHook: func(context.Context, string) (bool, error) {
				return false, nil
			},
		},
		ListFilesFunc: &GitserverClientWrapperListFilesFunc{
			defaultHook: func(context.Context, *regexp.Regexp) ([]string, error) {
				return nil, nil
			},
		},
		RawContentsFunc: &GitserverClientWrapperRawContentsFunc{
			defaultHook: func(context.Context, string) ([]byte, error) {
				return nil, nil
			},
		},
	}
}

// NewMockGitserverClientWrapperFrom creates a new mock of the
// MockGitserverClientWrapper interface. All methods delegate to the given
// implementation, unless overwritten.
func NewMockGitserverClientWrapperFrom(i GitserverClientWrapper) *MockGitserverClientWrapper {
	return &MockGitserverClientWrapper{
		FileExistsFunc: &GitserverClientWrapperFileExistsFunc{
			defaultHook: i.FileExists,
		},
		ListFilesFunc: &GitserverClientWrapperListFilesFunc{
			defaultHook: i.ListFiles,
		},
		RawContentsFunc: &GitserverClientWrapperRawContentsFunc{
			defaultHook: i.RawContents,
		},
	}
}

// GitserverClientWrapperFileExistsFunc describes the behavior when the
// FileExists method of the parent MockGitserverClientWrapper instance is
// invoked.
type GitserverClientWrapperFileExistsFunc struct {
	defaultHook func(context.Context, string) (bool, error)
	hooks       []func(context.Context, string) (bool, error)
	history     []GitserverClientWrapperFileExistsFuncCall
	mutex       sync.Mutex
}

// FileExists delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockGitserverClientWrapper) FileExists(v0 context.Context, v1 string) (bool, error) {
	r0, r1 := m.FileExistsFunc.nextHook()(v0, v1)
	m.FileExistsFunc.appendCall(GitserverClientWrapperFileExistsFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the FileExists method of
// the parent MockGitserverClientWrapper instance is invoked and the hook
// queue is empty.
func (f *GitserverClientWrapperFileExistsFunc) SetDefaultHook(hook func(context.Context, string) (bool, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// FileExists method of the parent MockGitserverClientWrapper instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *GitserverClientWrapperFileExistsFunc) PushHook(hook func(context.Context, string) (bool, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *GitserverClientWrapperFileExistsFunc) SetDefaultReturn(r0 bool, r1 error) {
	f.SetDefaultHook(func(context.Context, string) (bool, error) {
		return r0, r1
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *GitserverClientWrapperFileExistsFunc) PushReturn(r0 bool, r1 error) {
	f.PushHook(func(context.Context, string) (bool, error) {
		return r0, r1
	})
}

func (f *GitserverClientWrapperFileExistsFunc) nextHook() func(context.Context, string) (bool, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *GitserverClientWrapperFileExistsFunc) appendCall(r0 GitserverClientWrapperFileExistsFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of GitserverClientWrapperFileExistsFuncCall
// objects describing the invocations of this function.
func (f *GitserverClientWrapperFileExistsFunc) History() []GitserverClientWrapperFileExistsFuncCall {
	f.mutex.Lock()
	history := make([]GitserverClientWrapperFileExistsFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// GitserverClientWrapperFileExistsFuncCall is an object that describes an
// invocation of method FileExists on an instance of
// MockGitserverClientWrapper.
type GitserverClientWrapperFileExistsFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 string
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 bool
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c GitserverClientWrapperFileExistsFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c GitserverClientWrapperFileExistsFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// GitserverClientWrapperListFilesFunc describes the behavior when the
// ListFiles method of the parent MockGitserverClientWrapper instance is
// invoked.
type GitserverClientWrapperListFilesFunc struct {
	defaultHook func(context.Context, *regexp.Regexp) ([]string, error)
	hooks       []func(context.Context, *regexp.Regexp) ([]string, error)
	history     []GitserverClientWrapperListFilesFuncCall
	mutex       sync.Mutex
}

// ListFiles delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockGitserverClientWrapper) ListFiles(v0 context.Context, v1 *regexp.Regexp) ([]string, error) {
	r0, r1 := m.ListFilesFunc.nextHook()(v0, v1)
	m.ListFilesFunc.appendCall(GitserverClientWrapperListFilesFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the ListFiles method of
// the parent MockGitserverClientWrapper instance is invoked and the hook
// queue is empty.
func (f *GitserverClientWrapperListFilesFunc) SetDefaultHook(hook func(context.Context, *regexp.Regexp) ([]string, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// ListFiles method of the parent MockGitserverClientWrapper instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *GitserverClientWrapperListFilesFunc) PushHook(hook func(context.Context, *regexp.Regexp) ([]string, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *GitserverClientWrapperListFilesFunc) SetDefaultReturn(r0 []string, r1 error) {
	f.SetDefaultHook(func(context.Context, *regexp.Regexp) ([]string, error) {
		return r0, r1
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *GitserverClientWrapperListFilesFunc) PushReturn(r0 []string, r1 error) {
	f.PushHook(func(context.Context, *regexp.Regexp) ([]string, error) {
		return r0, r1
	})
}

func (f *GitserverClientWrapperListFilesFunc) nextHook() func(context.Context, *regexp.Regexp) ([]string, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *GitserverClientWrapperListFilesFunc) appendCall(r0 GitserverClientWrapperListFilesFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of GitserverClientWrapperListFilesFuncCall
// objects describing the invocations of this function.
func (f *GitserverClientWrapperListFilesFunc) History() []GitserverClientWrapperListFilesFuncCall {
	f.mutex.Lock()
	history := make([]GitserverClientWrapperListFilesFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// GitserverClientWrapperListFilesFuncCall is an object that describes an
// invocation of method ListFiles on an instance of
// MockGitserverClientWrapper.
type GitserverClientWrapperListFilesFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 *regexp.Regexp
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []string
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c GitserverClientWrapperListFilesFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c GitserverClientWrapperListFilesFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// GitserverClientWrapperRawContentsFunc describes the behavior when the
// RawContents method of the parent MockGitserverClientWrapper instance is
// invoked.
type GitserverClientWrapperRawContentsFunc struct {
	defaultHook func(context.Context, string) ([]byte, error)
	hooks       []func(context.Context, string) ([]byte, error)
	history     []GitserverClientWrapperRawContentsFuncCall
	mutex       sync.Mutex
}

// RawContents delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockGitserverClientWrapper) RawContents(v0 context.Context, v1 string) ([]byte, error) {
	r0, r1 := m.RawContentsFunc.nextHook()(v0, v1)
	m.RawContentsFunc.appendCall(GitserverClientWrapperRawContentsFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the RawContents method
// of the parent MockGitserverClientWrapper instance is invoked and the hook
// queue is empty.
func (f *GitserverClientWrapperRawContentsFunc) SetDefaultHook(hook func(context.Context, string) ([]byte, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// RawContents method of the parent MockGitserverClientWrapper instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *GitserverClientWrapperRawContentsFunc) PushHook(hook func(context.Context, string) ([]byte, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *GitserverClientWrapperRawContentsFunc) SetDefaultReturn(r0 []byte, r1 error) {
	f.SetDefaultHook(func(context.Context, string) ([]byte, error) {
		return r0, r1
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *GitserverClientWrapperRawContentsFunc) PushReturn(r0 []byte, r1 error) {
	f.PushHook(func(context.Context, string) ([]byte, error) {
		return r0, r1
	})
}

func (f *GitserverClientWrapperRawContentsFunc) nextHook() func(context.Context, string) ([]byte, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *GitserverClientWrapperRawContentsFunc) appendCall(r0 GitserverClientWrapperRawContentsFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of GitserverClientWrapperRawContentsFuncCall
// objects describing the invocations of this function.
func (f *GitserverClientWrapperRawContentsFunc) History() []GitserverClientWrapperRawContentsFuncCall {
	f.mutex.Lock()
	history := make([]GitserverClientWrapperRawContentsFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// GitserverClientWrapperRawContentsFuncCall is an object that describes an
// invocation of method RawContents on an instance of
// MockGitserverClientWrapper.
type GitserverClientWrapperRawContentsFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 string
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []byte
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c GitserverClientWrapperRawContentsFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c GitserverClientWrapperRawContentsFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// MockRepoUpdaterClient is a mock implementation of the RepoUpdaterClient
// interface (from the package
// github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/autoindex/inference)
// used for unit testing.
type MockRepoUpdaterClient struct {
	// EnqueueRepoUpdateFunc is an instance of a mock function object
	// controlling the behavior of the method EnqueueRepoUpdate.
	EnqueueRepoUpdateFunc *RepoUpdaterClientEnqueueRepoUpdateFunc
}

// NewMockRepoUpdaterClient creates a new mock of the RepoUpdaterClient
// interface. All methods return zero values for all results, unless
// overwritten.
func NewMockRepoUpdaterClient() *MockRepoUpdaterClient {
	return &MockRepoUpdaterClient{
		EnqueueRepoUpdateFunc: &RepoUpdaterClientEnqueueRepoUpdateFunc{
			defaultHook: func(context.Context, api.RepoName) (*protocol.RepoUpdateResponse, error) {
				return nil, nil
			},
		},
	}
}

// NewMockRepoUpdaterClientFrom creates a new mock of the
// MockRepoUpdaterClient interface. All methods delegate to the given
// implementation, unless overwritten.
func NewMockRepoUpdaterClientFrom(i RepoUpdaterClient) *MockRepoUpdaterClient {
	return &MockRepoUpdaterClient{
		EnqueueRepoUpdateFunc: &RepoUpdaterClientEnqueueRepoUpdateFunc{
			defaultHook: i.EnqueueRepoUpdate,
		},
	}
}

// RepoUpdaterClientEnqueueRepoUpdateFunc describes the behavior when the
// EnqueueRepoUpdate method of the parent MockRepoUpdaterClient instance is
// invoked.
type RepoUpdaterClientEnqueueRepoUpdateFunc struct {
	defaultHook func(context.Context, api.RepoName) (*protocol.RepoUpdateResponse, error)
	hooks       []func(context.Context, api.RepoName) (*protocol.RepoUpdateResponse, error)
	history     []RepoUpdaterClientEnqueueRepoUpdateFuncCall
	mutex       sync.Mutex
}

// EnqueueRepoUpdate delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockRepoUpdaterClient) EnqueueRepoUpdate(v0 context.Context, v1 api.RepoName) (*protocol.RepoUpdateResponse, error) {
	r0, r1 := m.EnqueueRepoUpdateFunc.nextHook()(v0, v1)
	m.EnqueueRepoUpdateFunc.appendCall(RepoUpdaterClientEnqueueRepoUpdateFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the EnqueueRepoUpdate
// method of the parent MockRepoUpdaterClient instance is invoked and the
// hook queue is empty.
func (f *RepoUpdaterClientEnqueueRepoUpdateFunc) SetDefaultHook(hook func(context.Context, api.RepoName) (*protocol.RepoUpdateResponse, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// EnqueueRepoUpdate method of the parent MockRepoUpdaterClient instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *RepoUpdaterClientEnqueueRepoUpdateFunc) PushHook(hook func(context.Context, api.RepoName) (*protocol.RepoUpdateResponse, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *RepoUpdaterClientEnqueueRepoUpdateFunc) SetDefaultReturn(r0 *protocol.RepoUpdateResponse, r1 error) {
	f.SetDefaultHook(func(context.Context, api.RepoName) (*protocol.RepoUpdateResponse, error) {
		return r0, r1
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *RepoUpdaterClientEnqueueRepoUpdateFunc) PushReturn(r0 *protocol.RepoUpdateResponse, r1 error) {
	f.PushHook(func(context.Context, api.RepoName) (*protocol.RepoUpdateResponse, error) {
		return r0, r1
	})
}

func (f *RepoUpdaterClientEnqueueRepoUpdateFunc) nextHook() func(context.Context, api.RepoName) (*protocol.RepoUpdateResponse, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *RepoUpdaterClientEnqueueRepoUpdateFunc) appendCall(r0 RepoUpdaterClientEnqueueRepoUpdateFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of RepoUpdaterClientEnqueueRepoUpdateFuncCall
// objects describing the invocations of this function.
func (f *RepoUpdaterClientEnqueueRepoUpdateFunc) History() []RepoUpdaterClientEnqueueRepoUpdateFuncCall {
	f.mutex.Lock()
	history := make([]RepoUpdaterClientEnqueueRepoUpdateFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// RepoUpdaterClientEnqueueRepoUpdateFuncCall is an object that describes an
// invocation of method EnqueueRepoUpdate on an instance of
// MockRepoUpdaterClient.
type RepoUpdaterClientEnqueueRepoUpdateFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 api.RepoName
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 *protocol.RepoUpdateResponse
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c RepoUpdaterClientEnqueueRepoUpdateFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c RepoUpdaterClientEnqueueRepoUpdateFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}
