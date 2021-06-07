// Code generated by go-mockgen 0.1.0; DO NOT EDIT.

package mocks

import (
	"context"
	"sync"

	resolvers "github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/codeintel/resolvers"
	lsifstore "github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/lsifstore"
	semantic "github.com/sourcegraph/sourcegraph/lib/codeintel/semantic"
)

// MockQueryResolver is a mock implementation of the QueryResolver interface
// (from the package
// github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/codeintel/resolvers)
// used for unit testing.
type MockQueryResolver struct {
	// DefinitionsFunc is an instance of a mock function object controlling
	// the behavior of the method Definitions.
	DefinitionsFunc *QueryResolverDefinitionsFunc
	// DiagnosticsFunc is an instance of a mock function object controlling
	// the behavior of the method Diagnostics.
	DiagnosticsFunc *QueryResolverDiagnosticsFunc
	// DocumentationPageFunc is an instance of a mock function object
	// controlling the behavior of the method DocumentationPage.
	DocumentationPageFunc *QueryResolverDocumentationPageFunc
	// HoverFunc is an instance of a mock function object controlling the
	// behavior of the method Hover.
	HoverFunc *QueryResolverHoverFunc
	// RangesFunc is an instance of a mock function object controlling the
	// behavior of the method Ranges.
	RangesFunc *QueryResolverRangesFunc
	// ReferencesFunc is an instance of a mock function object controlling
	// the behavior of the method References.
	ReferencesFunc *QueryResolverReferencesFunc
}

// NewMockQueryResolver creates a new mock of the QueryResolver interface.
// All methods return zero values for all results, unless overwritten.
func NewMockQueryResolver() *MockQueryResolver {
	return &MockQueryResolver{
		DefinitionsFunc: &QueryResolverDefinitionsFunc{
			defaultHook: func(context.Context, int, int) ([]resolvers.AdjustedLocation, error) {
				return nil, nil
			},
		},
		DiagnosticsFunc: &QueryResolverDiagnosticsFunc{
			defaultHook: func(context.Context, int) ([]resolvers.AdjustedDiagnostic, int, error) {
				return nil, 0, nil
			},
		},
		DocumentationPageFunc: &QueryResolverDocumentationPageFunc{
			defaultHook: func(context.Context, string) (*semantic.DocumentationPageData, error) {
				return nil, nil
			},
		},
		HoverFunc: &QueryResolverHoverFunc{
			defaultHook: func(context.Context, int, int) (string, lsifstore.Range, bool, error) {
				return "", lsifstore.Range{}, false, nil
			},
		},
		RangesFunc: &QueryResolverRangesFunc{
			defaultHook: func(context.Context, int, int) ([]resolvers.AdjustedCodeIntelligenceRange, error) {
				return nil, nil
			},
		},
		ReferencesFunc: &QueryResolverReferencesFunc{
			defaultHook: func(context.Context, int, int, int, string) ([]resolvers.AdjustedLocation, string, error) {
				return nil, "", nil
			},
		},
	}
}

// NewMockQueryResolverFrom creates a new mock of the MockQueryResolver
// interface. All methods delegate to the given implementation, unless
// overwritten.
func NewMockQueryResolverFrom(i resolvers.QueryResolver) *MockQueryResolver {
	return &MockQueryResolver{
		DefinitionsFunc: &QueryResolverDefinitionsFunc{
			defaultHook: i.Definitions,
		},
		DiagnosticsFunc: &QueryResolverDiagnosticsFunc{
			defaultHook: i.Diagnostics,
		},
		DocumentationPageFunc: &QueryResolverDocumentationPageFunc{
			defaultHook: i.DocumentationPage,
		},
		HoverFunc: &QueryResolverHoverFunc{
			defaultHook: i.Hover,
		},
		RangesFunc: &QueryResolverRangesFunc{
			defaultHook: i.Ranges,
		},
		ReferencesFunc: &QueryResolverReferencesFunc{
			defaultHook: i.References,
		},
	}
}

// QueryResolverDefinitionsFunc describes the behavior when the Definitions
// method of the parent MockQueryResolver instance is invoked.
type QueryResolverDefinitionsFunc struct {
	defaultHook func(context.Context, int, int) ([]resolvers.AdjustedLocation, error)
	hooks       []func(context.Context, int, int) ([]resolvers.AdjustedLocation, error)
	history     []QueryResolverDefinitionsFuncCall
	mutex       sync.Mutex
}

// Definitions delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockQueryResolver) Definitions(v0 context.Context, v1 int, v2 int) ([]resolvers.AdjustedLocation, error) {
	r0, r1 := m.DefinitionsFunc.nextHook()(v0, v1, v2)
	m.DefinitionsFunc.appendCall(QueryResolverDefinitionsFuncCall{v0, v1, v2, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the Definitions method
// of the parent MockQueryResolver instance is invoked and the hook queue is
// empty.
func (f *QueryResolverDefinitionsFunc) SetDefaultHook(hook func(context.Context, int, int) ([]resolvers.AdjustedLocation, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Definitions method of the parent MockQueryResolver instance invokes the
// hook at the front of the queue and discards it. After the queue is empty,
// the default hook function is invoked for any future action.
func (f *QueryResolverDefinitionsFunc) PushHook(hook func(context.Context, int, int) ([]resolvers.AdjustedLocation, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *QueryResolverDefinitionsFunc) SetDefaultReturn(r0 []resolvers.AdjustedLocation, r1 error) {
	f.SetDefaultHook(func(context.Context, int, int) ([]resolvers.AdjustedLocation, error) {
		return r0, r1
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *QueryResolverDefinitionsFunc) PushReturn(r0 []resolvers.AdjustedLocation, r1 error) {
	f.PushHook(func(context.Context, int, int) ([]resolvers.AdjustedLocation, error) {
		return r0, r1
	})
}

func (f *QueryResolverDefinitionsFunc) nextHook() func(context.Context, int, int) ([]resolvers.AdjustedLocation, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *QueryResolverDefinitionsFunc) appendCall(r0 QueryResolverDefinitionsFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of QueryResolverDefinitionsFuncCall objects
// describing the invocations of this function.
func (f *QueryResolverDefinitionsFunc) History() []QueryResolverDefinitionsFuncCall {
	f.mutex.Lock()
	history := make([]QueryResolverDefinitionsFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// QueryResolverDefinitionsFuncCall is an object that describes an
// invocation of method Definitions on an instance of MockQueryResolver.
type QueryResolverDefinitionsFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []resolvers.AdjustedLocation
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c QueryResolverDefinitionsFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c QueryResolverDefinitionsFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// QueryResolverDiagnosticsFunc describes the behavior when the Diagnostics
// method of the parent MockQueryResolver instance is invoked.
type QueryResolverDiagnosticsFunc struct {
	defaultHook func(context.Context, int) ([]resolvers.AdjustedDiagnostic, int, error)
	hooks       []func(context.Context, int) ([]resolvers.AdjustedDiagnostic, int, error)
	history     []QueryResolverDiagnosticsFuncCall
	mutex       sync.Mutex
}

// Diagnostics delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockQueryResolver) Diagnostics(v0 context.Context, v1 int) ([]resolvers.AdjustedDiagnostic, int, error) {
	r0, r1, r2 := m.DiagnosticsFunc.nextHook()(v0, v1)
	m.DiagnosticsFunc.appendCall(QueryResolverDiagnosticsFuncCall{v0, v1, r0, r1, r2})
	return r0, r1, r2
}

// SetDefaultHook sets function that is called when the Diagnostics method
// of the parent MockQueryResolver instance is invoked and the hook queue is
// empty.
func (f *QueryResolverDiagnosticsFunc) SetDefaultHook(hook func(context.Context, int) ([]resolvers.AdjustedDiagnostic, int, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Diagnostics method of the parent MockQueryResolver instance invokes the
// hook at the front of the queue and discards it. After the queue is empty,
// the default hook function is invoked for any future action.
func (f *QueryResolverDiagnosticsFunc) PushHook(hook func(context.Context, int) ([]resolvers.AdjustedDiagnostic, int, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *QueryResolverDiagnosticsFunc) SetDefaultReturn(r0 []resolvers.AdjustedDiagnostic, r1 int, r2 error) {
	f.SetDefaultHook(func(context.Context, int) ([]resolvers.AdjustedDiagnostic, int, error) {
		return r0, r1, r2
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *QueryResolverDiagnosticsFunc) PushReturn(r0 []resolvers.AdjustedDiagnostic, r1 int, r2 error) {
	f.PushHook(func(context.Context, int) ([]resolvers.AdjustedDiagnostic, int, error) {
		return r0, r1, r2
	})
}

func (f *QueryResolverDiagnosticsFunc) nextHook() func(context.Context, int) ([]resolvers.AdjustedDiagnostic, int, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *QueryResolverDiagnosticsFunc) appendCall(r0 QueryResolverDiagnosticsFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of QueryResolverDiagnosticsFuncCall objects
// describing the invocations of this function.
func (f *QueryResolverDiagnosticsFunc) History() []QueryResolverDiagnosticsFuncCall {
	f.mutex.Lock()
	history := make([]QueryResolverDiagnosticsFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// QueryResolverDiagnosticsFuncCall is an object that describes an
// invocation of method Diagnostics on an instance of MockQueryResolver.
type QueryResolverDiagnosticsFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []resolvers.AdjustedDiagnostic
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 int
	// Result2 is the value of the 3rd result returned from this method
	// invocation.
	Result2 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c QueryResolverDiagnosticsFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c QueryResolverDiagnosticsFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1, c.Result2}
}

// QueryResolverDocumentationPageFunc describes the behavior when the
// DocumentationPage method of the parent MockQueryResolver instance is
// invoked.
type QueryResolverDocumentationPageFunc struct {
	defaultHook func(context.Context, string) (*semantic.DocumentationPageData, error)
	hooks       []func(context.Context, string) (*semantic.DocumentationPageData, error)
	history     []QueryResolverDocumentationPageFuncCall
	mutex       sync.Mutex
}

// DocumentationPage delegates to the next hook function in the queue and
// stores the parameter and result values of this invocation.
func (m *MockQueryResolver) DocumentationPage(v0 context.Context, v1 string) (*semantic.DocumentationPageData, error) {
	r0, r1 := m.DocumentationPageFunc.nextHook()(v0, v1)
	m.DocumentationPageFunc.appendCall(QueryResolverDocumentationPageFuncCall{v0, v1, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the DocumentationPage
// method of the parent MockQueryResolver instance is invoked and the hook
// queue is empty.
func (f *QueryResolverDocumentationPageFunc) SetDefaultHook(hook func(context.Context, string) (*semantic.DocumentationPageData, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// DocumentationPage method of the parent MockQueryResolver instance invokes
// the hook at the front of the queue and discards it. After the queue is
// empty, the default hook function is invoked for any future action.
func (f *QueryResolverDocumentationPageFunc) PushHook(hook func(context.Context, string) (*semantic.DocumentationPageData, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *QueryResolverDocumentationPageFunc) SetDefaultReturn(r0 *semantic.DocumentationPageData, r1 error) {
	f.SetDefaultHook(func(context.Context, string) (*semantic.DocumentationPageData, error) {
		return r0, r1
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *QueryResolverDocumentationPageFunc) PushReturn(r0 *semantic.DocumentationPageData, r1 error) {
	f.PushHook(func(context.Context, string) (*semantic.DocumentationPageData, error) {
		return r0, r1
	})
}

func (f *QueryResolverDocumentationPageFunc) nextHook() func(context.Context, string) (*semantic.DocumentationPageData, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *QueryResolverDocumentationPageFunc) appendCall(r0 QueryResolverDocumentationPageFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of QueryResolverDocumentationPageFuncCall
// objects describing the invocations of this function.
func (f *QueryResolverDocumentationPageFunc) History() []QueryResolverDocumentationPageFuncCall {
	f.mutex.Lock()
	history := make([]QueryResolverDocumentationPageFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// QueryResolverDocumentationPageFuncCall is an object that describes an
// invocation of method DocumentationPage on an instance of
// MockQueryResolver.
type QueryResolverDocumentationPageFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 string
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 *semantic.DocumentationPageData
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c QueryResolverDocumentationPageFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c QueryResolverDocumentationPageFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// QueryResolverHoverFunc describes the behavior when the Hover method of
// the parent MockQueryResolver instance is invoked.
type QueryResolverHoverFunc struct {
	defaultHook func(context.Context, int, int) (string, lsifstore.Range, bool, error)
	hooks       []func(context.Context, int, int) (string, lsifstore.Range, bool, error)
	history     []QueryResolverHoverFuncCall
	mutex       sync.Mutex
}

// Hover delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockQueryResolver) Hover(v0 context.Context, v1 int, v2 int) (string, lsifstore.Range, bool, error) {
	r0, r1, r2, r3 := m.HoverFunc.nextHook()(v0, v1, v2)
	m.HoverFunc.appendCall(QueryResolverHoverFuncCall{v0, v1, v2, r0, r1, r2, r3})
	return r0, r1, r2, r3
}

// SetDefaultHook sets function that is called when the Hover method of the
// parent MockQueryResolver instance is invoked and the hook queue is empty.
func (f *QueryResolverHoverFunc) SetDefaultHook(hook func(context.Context, int, int) (string, lsifstore.Range, bool, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Hover method of the parent MockQueryResolver instance invokes the hook at
// the front of the queue and discards it. After the queue is empty, the
// default hook function is invoked for any future action.
func (f *QueryResolverHoverFunc) PushHook(hook func(context.Context, int, int) (string, lsifstore.Range, bool, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *QueryResolverHoverFunc) SetDefaultReturn(r0 string, r1 lsifstore.Range, r2 bool, r3 error) {
	f.SetDefaultHook(func(context.Context, int, int) (string, lsifstore.Range, bool, error) {
		return r0, r1, r2, r3
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *QueryResolverHoverFunc) PushReturn(r0 string, r1 lsifstore.Range, r2 bool, r3 error) {
	f.PushHook(func(context.Context, int, int) (string, lsifstore.Range, bool, error) {
		return r0, r1, r2, r3
	})
}

func (f *QueryResolverHoverFunc) nextHook() func(context.Context, int, int) (string, lsifstore.Range, bool, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *QueryResolverHoverFunc) appendCall(r0 QueryResolverHoverFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of QueryResolverHoverFuncCall objects
// describing the invocations of this function.
func (f *QueryResolverHoverFunc) History() []QueryResolverHoverFuncCall {
	f.mutex.Lock()
	history := make([]QueryResolverHoverFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// QueryResolverHoverFuncCall is an object that describes an invocation of
// method Hover on an instance of MockQueryResolver.
type QueryResolverHoverFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 string
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 lsifstore.Range
	// Result2 is the value of the 3rd result returned from this method
	// invocation.
	Result2 bool
	// Result3 is the value of the 4th result returned from this method
	// invocation.
	Result3 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c QueryResolverHoverFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c QueryResolverHoverFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1, c.Result2, c.Result3}
}

// QueryResolverRangesFunc describes the behavior when the Ranges method of
// the parent MockQueryResolver instance is invoked.
type QueryResolverRangesFunc struct {
	defaultHook func(context.Context, int, int) ([]resolvers.AdjustedCodeIntelligenceRange, error)
	hooks       []func(context.Context, int, int) ([]resolvers.AdjustedCodeIntelligenceRange, error)
	history     []QueryResolverRangesFuncCall
	mutex       sync.Mutex
}

// Ranges delegates to the next hook function in the queue and stores the
// parameter and result values of this invocation.
func (m *MockQueryResolver) Ranges(v0 context.Context, v1 int, v2 int) ([]resolvers.AdjustedCodeIntelligenceRange, error) {
	r0, r1 := m.RangesFunc.nextHook()(v0, v1, v2)
	m.RangesFunc.appendCall(QueryResolverRangesFuncCall{v0, v1, v2, r0, r1})
	return r0, r1
}

// SetDefaultHook sets function that is called when the Ranges method of the
// parent MockQueryResolver instance is invoked and the hook queue is empty.
func (f *QueryResolverRangesFunc) SetDefaultHook(hook func(context.Context, int, int) ([]resolvers.AdjustedCodeIntelligenceRange, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// Ranges method of the parent MockQueryResolver instance invokes the hook
// at the front of the queue and discards it. After the queue is empty, the
// default hook function is invoked for any future action.
func (f *QueryResolverRangesFunc) PushHook(hook func(context.Context, int, int) ([]resolvers.AdjustedCodeIntelligenceRange, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *QueryResolverRangesFunc) SetDefaultReturn(r0 []resolvers.AdjustedCodeIntelligenceRange, r1 error) {
	f.SetDefaultHook(func(context.Context, int, int) ([]resolvers.AdjustedCodeIntelligenceRange, error) {
		return r0, r1
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *QueryResolverRangesFunc) PushReturn(r0 []resolvers.AdjustedCodeIntelligenceRange, r1 error) {
	f.PushHook(func(context.Context, int, int) ([]resolvers.AdjustedCodeIntelligenceRange, error) {
		return r0, r1
	})
}

func (f *QueryResolverRangesFunc) nextHook() func(context.Context, int, int) ([]resolvers.AdjustedCodeIntelligenceRange, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *QueryResolverRangesFunc) appendCall(r0 QueryResolverRangesFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of QueryResolverRangesFuncCall objects
// describing the invocations of this function.
func (f *QueryResolverRangesFunc) History() []QueryResolverRangesFuncCall {
	f.mutex.Lock()
	history := make([]QueryResolverRangesFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// QueryResolverRangesFuncCall is an object that describes an invocation of
// method Ranges on an instance of MockQueryResolver.
type QueryResolverRangesFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 int
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []resolvers.AdjustedCodeIntelligenceRange
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c QueryResolverRangesFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c QueryResolverRangesFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1}
}

// QueryResolverReferencesFunc describes the behavior when the References
// method of the parent MockQueryResolver instance is invoked.
type QueryResolverReferencesFunc struct {
	defaultHook func(context.Context, int, int, int, string) ([]resolvers.AdjustedLocation, string, error)
	hooks       []func(context.Context, int, int, int, string) ([]resolvers.AdjustedLocation, string, error)
	history     []QueryResolverReferencesFuncCall
	mutex       sync.Mutex
}

// References delegates to the next hook function in the queue and stores
// the parameter and result values of this invocation.
func (m *MockQueryResolver) References(v0 context.Context, v1 int, v2 int, v3 int, v4 string) ([]resolvers.AdjustedLocation, string, error) {
	r0, r1, r2 := m.ReferencesFunc.nextHook()(v0, v1, v2, v3, v4)
	m.ReferencesFunc.appendCall(QueryResolverReferencesFuncCall{v0, v1, v2, v3, v4, r0, r1, r2})
	return r0, r1, r2
}

// SetDefaultHook sets function that is called when the References method of
// the parent MockQueryResolver instance is invoked and the hook queue is
// empty.
func (f *QueryResolverReferencesFunc) SetDefaultHook(hook func(context.Context, int, int, int, string) ([]resolvers.AdjustedLocation, string, error)) {
	f.defaultHook = hook
}

// PushHook adds a function to the end of hook queue. Each invocation of the
// References method of the parent MockQueryResolver instance invokes the
// hook at the front of the queue and discards it. After the queue is empty,
// the default hook function is invoked for any future action.
func (f *QueryResolverReferencesFunc) PushHook(hook func(context.Context, int, int, int, string) ([]resolvers.AdjustedLocation, string, error)) {
	f.mutex.Lock()
	f.hooks = append(f.hooks, hook)
	f.mutex.Unlock()
}

// SetDefaultReturn calls SetDefaultDefaultHook with a function that returns
// the given values.
func (f *QueryResolverReferencesFunc) SetDefaultReturn(r0 []resolvers.AdjustedLocation, r1 string, r2 error) {
	f.SetDefaultHook(func(context.Context, int, int, int, string) ([]resolvers.AdjustedLocation, string, error) {
		return r0, r1, r2
	})
}

// PushReturn calls PushDefaultHook with a function that returns the given
// values.
func (f *QueryResolverReferencesFunc) PushReturn(r0 []resolvers.AdjustedLocation, r1 string, r2 error) {
	f.PushHook(func(context.Context, int, int, int, string) ([]resolvers.AdjustedLocation, string, error) {
		return r0, r1, r2
	})
}

func (f *QueryResolverReferencesFunc) nextHook() func(context.Context, int, int, int, string) ([]resolvers.AdjustedLocation, string, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if len(f.hooks) == 0 {
		return f.defaultHook
	}

	hook := f.hooks[0]
	f.hooks = f.hooks[1:]
	return hook
}

func (f *QueryResolverReferencesFunc) appendCall(r0 QueryResolverReferencesFuncCall) {
	f.mutex.Lock()
	f.history = append(f.history, r0)
	f.mutex.Unlock()
}

// History returns a sequence of QueryResolverReferencesFuncCall objects
// describing the invocations of this function.
func (f *QueryResolverReferencesFunc) History() []QueryResolverReferencesFuncCall {
	f.mutex.Lock()
	history := make([]QueryResolverReferencesFuncCall, len(f.history))
	copy(history, f.history)
	f.mutex.Unlock()

	return history
}

// QueryResolverReferencesFuncCall is an object that describes an invocation
// of method References on an instance of MockQueryResolver.
type QueryResolverReferencesFuncCall struct {
	// Arg0 is the value of the 1st argument passed to this method
	// invocation.
	Arg0 context.Context
	// Arg1 is the value of the 2nd argument passed to this method
	// invocation.
	Arg1 int
	// Arg2 is the value of the 3rd argument passed to this method
	// invocation.
	Arg2 int
	// Arg3 is the value of the 4th argument passed to this method
	// invocation.
	Arg3 int
	// Arg4 is the value of the 5th argument passed to this method
	// invocation.
	Arg4 string
	// Result0 is the value of the 1st result returned from this method
	// invocation.
	Result0 []resolvers.AdjustedLocation
	// Result1 is the value of the 2nd result returned from this method
	// invocation.
	Result1 string
	// Result2 is the value of the 3rd result returned from this method
	// invocation.
	Result2 error
}

// Args returns an interface slice containing the arguments of this
// invocation.
func (c QueryResolverReferencesFuncCall) Args() []interface{} {
	return []interface{}{c.Arg0, c.Arg1, c.Arg2, c.Arg3, c.Arg4}
}

// Results returns an interface slice containing the results of this
// invocation.
func (c QueryResolverReferencesFuncCall) Results() []interface{} {
	return []interface{}{c.Result0, c.Result1, c.Result2}
}
