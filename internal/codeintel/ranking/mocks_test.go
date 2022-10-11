// Code generated by go-mockgen 1.3.4; DO NOT EDIT.
//
// This file was generated by running `sg generate` (or `go-mockgen`) at the root of
// this repository. To add additional mocks to this or another package, add a new entry
// to the mockgen.yaml file in the root of this repository.

package ranking

import (
	"context"
	"sync"

	regexp "github.com/grafana/regexp"
	api "github.com/sourcegraph/sourcegraph/internal/api"
	store "github.com/sourcegraph/sourcegraph/internal/codeintel/ranking/internal/store"
	conftypes "github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	schema "github.com/sourcegraph/sourcegraph/schema"
)

// MockStore is a mock implementation of the Store interface (from the
// package
// github.com/sourcegraph/sourcegraph/internal/codeintel/ranking/internal/store)
// used for unit testing.
type MockStore struct {
	// DoneFunc is an instance of a mock function object controlling the
	// behavior of the method Done.
	DoneFunc *StoreDoneFunc
	// GetStarRankFunc is an instance of a mock function object controlling
	// the behavior of the method GetStarRank.
	GetStarRankFunc *StoreGetStarRankFunc
	// TransactFunc is an instance of a mock function object controlling the
	// behavior of the method Transact.
	TransactFunc *StoreTransactFunc
}

// NewMockStore creates a new mock of the Store interface. All methods
// return zero values for all results, unless overwritten.
func NewMockStore() *MockStore {
	return &MockStore{
		DoneFunc: &StoreDoneFunc{
			defaultHook: func(error) (r0 error) {
				return
			},
		},
		GetStarRankFunc: &StoreGetStarRankFunc{
			defaultHook: func(context.Context, api.RepoName) (r0 float64, r1 error) {
				return
			},
		},
		TransactFunc: &StoreTransactFunc{
			defaultHook: func(context.Context) (r0 store.Store, r1 error) {
				return
			},
		},
	}
}

// NewStrictMockStore creates a new mock of the Store interface. All methods
// panic on invocation, unless overwritten.
func NewStrictMockStore() *MockStore {
	return &MockStore{
		DoneFunc: &StoreDoneFunc{
			defaultHook: func(error) error {
				panic("unexpected invocation of MockStore.Done")
			},
		},
		GetStarRankFunc: &StoreGetStarRankFunc{
			defaultHook: func(context.Context, api.RepoName) (float64, error) {
				panic("unexpected invocation of MockStore.GetStarRank")
			},
		},
		TransactFunc: &StoreTransactFunc{
			defaultHook: func(context.Context) (store.Store, error) {
				panic("unexpected invocation of MockStore.Transact")
			},
		},
	}
}

// NewMockStoreFrom creates a new mock of the MockStore interface. All
// methods delegate to the given implementation, unless overwritten.
func NewMockStoreFrom(i store.Store) *MockStore {
	return &MockStore{
		DoneFunc: &StoreDoneFunc{
			defaultHook: i.Done,
		},
		GetStarRankFunc: &StoreGetStarRankFunc{
			defaultHook: i.GetStarRank,
		},
		TransactFunc: &StoreTransactFunc{
			defaultHook: i.Transact,
		},
	}
}

// StoreDoneFunc describes the behavior when the Done method of the parent
// MockStore instance is invoked.
type StoreDoneFunc struct {
	defaultHook func(error) error
	hooks       []func(error) error
	history     []StoreDoneFuncCall
	mutex       sync.Mutex
}

// Done delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockStore) Done(v0 error) error {
	r0 := m.DoneFunc.nextHook()(v0)
	m.DoneFunc.appendCall(StoreDoneFuncCall{v0, r0})
	return r0
}

// SetDefaultHook sets function that is called when the Done method of the
// parent MockStore instance is invoked and the hook queue is empty.
func (f *StoreDoneFunc) SetDefaultHook(hook func(error) error) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Done method of the parent MockStore instance invokes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *StoreDoneFunc) PushHook(hook func(error) error) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *StoreDoneFunc) SetDefaultReturn(r0 error) {
	f.SetDefaultHook(func(error) error {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *StoreDoneFunc) PushReturn(r0 error) {
	f.PushHook(func(error) error {
		return r0
	})
}

func (f *StoreDoneFunc) nextHook() func(error) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *StoreDoneFunc) appendCall(r0 StoreDoneFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of StoreDoneFuncCall objects describing the
// invocations of this function.
func (f *StoreDoneFunc) History() []StoreDoneFuncCall {
	f.mutex.Lock()
	history := make([]StoreDoneFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// StoreDoneFuncCall is an object that describes an invocation of method
// Done on an instance of MockStore.
type StoreDoneFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 error
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c StoreDoneFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c StoreDoneFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}

// StoreGetStarRankFunc describes the behavior when the GetStarRank method
// of the parent MockStore instance is invoked.
type StoreGetStarRankFunc struct {
	defaultHook func(context.Context, api.RepoName) (float64, error)
	hooks       []func(context.Context, api.RepoName) (float64, error)
	history     []StoreGetStarRankFuncCall
	mutex       sync.Mutex
}

// GetStarRank delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockStore) GetStarRank(v0 context.Context, v1 api.RepoName) (float64, error) {
	r0, r1 := m.GetStarRankFunc.nextHook()(v0, v1)
	m.GetStarRankFunc.appendCall(StoreGetStarRankFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the GetStarRank method
// of the parent MockStore instance is invoked and the hook queue is empty.
func (f *StoreGetStarRankFunc) SetDefaultHook(hook func(context.Context, api.RepoName) (float64, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// GetStarRank method of the parent MockStore instance invokes the hook at
// the front of the queue and discards it. After the queue is empty, the
// default hook function is invoked for any future action.
func (f *StoreGetStarRankFunc) PushHook(hook func(context.Context, api.RepoName) (float64, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *StoreGetStarRankFunc) SetDefaultReturn(r0 float64, r1 error) {
	f.SetDefaultHook(func(context.Context, api.RepoName) (float64, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *StoreGetStarRankFunc) PushReturn(r0 float64, r1 error) {
	f.PushHook(func(context.Context, api.RepoName) (float64, error) {
		return r0, r1
	})
}

func (f *StoreGetStarRankFunc) nextHook() func(context.Context, api.RepoName) (float64, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *StoreGetStarRankFunc) appendCall(r0 StoreGetStarRankFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of StoreGetStarRankFuncCall objects describing
// the invocations of this function.
func (f *StoreGetStarRankFunc) History() []StoreGetStarRankFuncCall {
	f.mutex.Lock()
	history := make([]StoreGetStarRankFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// StoreGetStarRankFuncCall is an object that describes an invocation of
// method GetStarRank on an instance of MockStore.
type StoreGetStarRankFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 api.RepoName
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 float64
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c StoreGetStarRankFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c StoreGetStarRankFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// StoreTransactFunc describes the behavior when the Transact method of the
// parent MockStore instance is invoked.
type StoreTransactFunc struct {
	defaultHook func(context.Context) (store.Store, error)
	hooks       []func(context.Context) (store.Store, error)
	history     []StoreTransactFuncCall
	mutex       sync.Mutex
}

// Transact delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockStore) Transact(v0 context.Context) (store.Store, error) {
	r0, r1 := m.TransactFunc.nextHook()(v0)
	m.TransactFunc.appendCall(StoreTransactFuncCall{v0, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the Transact method of
// the parent MockStore instance is invoked and the hook queue is empty.
func (f *StoreTransactFunc) SetDefaultHook(hook func(context.Context) (store.Store, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Transact method of the parent MockStore instance invokes the hook at the
// front of the queue and discards it. After the queue is empty, the default
// hook function is invoked for any future action.
func (f *StoreTransactFunc) PushHook(hook func(context.Context) (store.Store, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *StoreTransactFunc) SetDefaultReturn(r0 store.Store, r1 error) {
	f.SetDefaultHook(func(context.Context) (store.Store, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *StoreTransactFunc) PushReturn(r0 store.Store, r1 error) {
	f.PushHook(func(context.Context) (store.Store, error) {
		return r0, r1
	})
}

func (f *StoreTransactFunc) nextHook() func(context.Context) (store.Store, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *StoreTransactFunc) appendCall(r0 StoreTransactFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of StoreTransactFuncCall objects describing
// the invocations of this function.
func (f *StoreTransactFunc) History() []StoreTransactFuncCall {
	f.mutex.Lock()
	history := make([]StoreTransactFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// StoreTransactFuncCall is an object that describes an invocation of method
// Transact on an instance of MockStore.
type StoreTransactFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 store.Store
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c StoreTransactFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c StoreTransactFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// MockGitserverClient is a mock implementation of the GitserverClient
// interface (from the package
// github.com/sourcegraph/sourcegraph/internal/codeintel/ranking) used for
// unit testing.
type MockGitserverClient struct {
	// ListFilesForRepoFunc is an instance of a mock function object
	// controlling the behavior of the method ListFilesForRepo.
	ListFilesForRepoFunc *GitserverClientListFilesForRepoFunc
}

// NewMockGitserverClient creates a new mock of the GitserverClient
// interface. All methods return zero values for all results, unless
// overwritten.
func NewMockGitserverClient() *MockGitserverClient {
	return &MockGitserverClient{
		ListFilesForRepoFunc: &GitserverClientListFilesForRepoFunc{
			defaultHook: func(context.Context, api.RepoName, string, *regexp.Regexp) (r0 []string, r1 error) {
				return
			},
		},
	}
}

// NewStrictMockGitserverClient creates a new mock of the GitserverClient
// interface. All methods panic on invocation, unless overwritten.
func NewStrictMockGitserverClient() *MockGitserverClient {
	return &MockGitserverClient{
		ListFilesForRepoFunc: &GitserverClientListFilesForRepoFunc{
			defaultHook: func(context.Context, api.RepoName, string, *regexp.Regexp) ([]string, error) {
				panic("unexpected invocation of MockGitserverClient.ListFilesForRepo")
			},
		},
	}
}

// NewMockGitserverClientFrom creates a new mock of the MockGitserverClient
// interface. All methods delegate to the given implementation, unless
// overwritten.
func NewMockGitserverClientFrom(i GitserverClient) *MockGitserverClient {
	return &MockGitserverClient{
		ListFilesForRepoFunc: &GitserverClientListFilesForRepoFunc{
			defaultHook: i.ListFilesForRepo,
		},
	}
}

// GitserverClientListFilesForRepoFunc describes the behavior when the
// ListFilesForRepo method of the parent MockGitserverClient instance is
// invoked.
type GitserverClientListFilesForRepoFunc struct {
	defaultHook func(context.Context, api.RepoName, string, *regexp.Regexp) ([]string, error)
	hooks       []func(context.Context, api.RepoName, string, *regexp.Regexp) ([]string, error)
	history     []GitserverClientListFilesForRepoFuncCall
	mutex       sync.Mutex
}

// ListFilesForRepo delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockGitserverClient) ListFilesForRepo(v0 context.Context, v1 api.RepoName, v2 string, v3 *regexp.Regexp) ([]string, error) {
	r0, r1 := m.ListFilesForRepoFunc.nextHook()(v0, v1, v2, v3)
	m.ListFilesForRepoFunc.appendCall(GitserverClientListFilesForRepoFuncCall{v0, v1, v2, v3, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the ListFilesForRepo
// method of the parent MockGitserverClient instance is invoked and the hook
// queue is empty.
func (f *GitserverClientListFilesForRepoFunc) SetDefaultHook(hook func(context.Context, api.RepoName, string, *regexp.Regexp) ([]string, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// ListFilesForRepo method of the parent MockGitserverClient instance
// invokes the hook at the front of the queue and discards it. After the
// queue is empty, the default hook function is invoked for any future
// action.
func (f *GitserverClientListFilesForRepoFunc) PushHook(hook func(context.Context, api.RepoName, string, *regexp.Regexp) ([]string, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *GitserverClientListFilesForRepoFunc) SetDefaultReturn(r0 []string, r1 error) {
	f.SetDefaultHook(func(context.Context, api.RepoName, string, *regexp.Regexp) ([]string, error) {
		return r0, r1
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *GitserverClientListFilesForRepoFunc) PushReturn(r0 []string, r1 error) {
	f.PushHook(func(context.Context, api.RepoName, string, *regexp.Regexp) ([]string, error) {
		return r0, r1
	})
}

func (f *GitserverClientListFilesForRepoFunc) nextHook() func(context.Context, api.RepoName, string, *regexp.Regexp) ([]string, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *GitserverClientListFilesForRepoFunc) appendCall(r0 GitserverClientListFilesForRepoFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of GitserverClientListFilesForRepoFuncCall
// objects describing the invocations of this function.
func (f *GitserverClientListFilesForRepoFunc) History() []GitserverClientListFilesForRepoFuncCall {
	f.mutex.Lock()
	history := make([]GitserverClientListFilesForRepoFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// GitserverClientListFilesForRepoFuncCall is an object that describes an
// invocation of method ListFilesForRepo on an instance of
// MockGitserverClient.
type GitserverClientListFilesForRepoFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 api.RepoName
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 string
	// Arg3 is the value of the 4th argument passed to this method
	// invocation.
	Arg3 *regexp.Regexp
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []string
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c GitserverClientListFilesForRepoFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c GitserverClientListFilesForRepoFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// MockSiteConfigQuerier is a mock implementation of the SiteConfigQuerier
// interface (from the package
// github.com/sourcegraph/sourcegraph/internal/conf/conftypes) used for unit
// testing.
type MockSiteConfigQuerier struct {
	// SiteConfigFunc is an instance of a mock function object controlling
	// the behavior of the method SiteConfig.
	SiteConfigFunc *SiteConfigQuerierSiteConfigFunc
}

// NewMockSiteConfigQuerier creates a new mock of the SiteConfigQuerier
// interface. All methods return zero values for all results, unless
// overwritten.
func NewMockSiteConfigQuerier() *MockSiteConfigQuerier {
	return &MockSiteConfigQuerier{
		SiteConfigFunc: &SiteConfigQuerierSiteConfigFunc{
			defaultHook: func() (r0 schema.SiteConfiguration) {
				return
			},
		},
	}
}

// NewStrictMockSiteConfigQuerier creates a new mock of the
// SiteConfigQuerier interface. All methods panic on invocation, unless
// overwritten.
func NewStrictMockSiteConfigQuerier() *MockSiteConfigQuerier {
	return &MockSiteConfigQuerier{
		SiteConfigFunc: &SiteConfigQuerierSiteConfigFunc{
			defaultHook: func() schema.SiteConfiguration {
				panic("unexpected invocation of MockSiteConfigQuerier.SiteConfig")
			},
		},
	}
}

// NewMockSiteConfigQuerierFrom creates a new mock of the
// MockSiteConfigQuerier interface. All methods delegate to the given
// implementation, unless overwritten.
func NewMockSiteConfigQuerierFrom(i conftypes.SiteConfigQuerier) *MockSiteConfigQuerier {
	return &MockSiteConfigQuerier{
		SiteConfigFunc: &SiteConfigQuerierSiteConfigFunc{
			defaultHook: i.SiteConfig,
		},
	}
}

// SiteConfigQuerierSiteConfigFunc describes the behavior when the
// SiteConfig method of the parent MockSiteConfigQuerier instance is
// invoked.
type SiteConfigQuerierSiteConfigFunc struct {
	defaultHook func() schema.SiteConfiguration
	hooks       []func() schema.SiteConfiguration
	history     []SiteConfigQuerierSiteConfigFuncCall
	mutex       sync.Mutex
}

// SiteConfig delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockSiteConfigQuerier) SiteConfig() schema.SiteConfiguration {
	r0 := m.SiteConfigFunc.nextHook()()
	m.SiteConfigFunc.appendCall(SiteConfigQuerierSiteConfigFuncCall{r0})
	return r0
}

// SetDefaultHook sets function that is called when the SiteConfig method of
// the parent MockSiteConfigQuerier instance is invoked and the hook queue
// is empty.
func (f *SiteConfigQuerierSiteConfigFunc) SetDefaultHook(hook func() schema.SiteConfiguration) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// SiteConfig method of the parent MockSiteConfigQuerier instance invokes
// the hook at the front of the queue and discards it. After the queue is
// empty, the default hook function is invoked for any future action.
func (f *SiteConfigQuerierSiteConfigFunc) PushHook(hook func() schema.SiteConfiguration) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultHook with a function that returns the
// given values.
func (f *SiteConfigQuerierSiteConfigFunc) SetDefaultReturn(r0 schema.SiteConfiguration) {
	f.SetDefaultHook(func() schema.SiteConfiguration {
		return r0
	})
}

// PushReturn calls PushHook with a function that returns the given values.
func (f *SiteConfigQuerierSiteConfigFunc) PushReturn(r0 schema.SiteConfiguration) {
	f.PushHook(func() schema.SiteConfiguration {
		return r0
	})
}

func (f *SiteConfigQuerierSiteConfigFunc) nextHook() func() schema.SiteConfiguration {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *SiteConfigQuerierSiteConfigFunc) appendCall(r0 SiteConfigQuerierSiteConfigFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of SiteConfigQuerierSiteConfigFuncCall objects
// describing the invocations of this function.
func (f *SiteConfigQuerierSiteConfigFunc) History() []SiteConfigQuerierSiteConfigFuncCall {
	f.mutex.Lock()
	history := make([]SiteConfigQuerierSiteConfigFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// SiteConfigQuerierSiteConfigFuncCall is an object that describes an
// invocation of method SiteConfig on an instance of MockSiteConfigQuerier.
type SiteConfigQuerierSiteConfigFuncCall struct {
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 schema.SiteConfiguration
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c SiteConfigQuerierSiteConfigFuncCall) Args() []interface{} {
	return []interface{}{}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c SiteConfigQuerierSiteConfigFuncCall) Results() []interface{} {
	return []interface{}{c.Result0}
}
