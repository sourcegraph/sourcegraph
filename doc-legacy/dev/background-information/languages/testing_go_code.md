# Testing Go code

This document contains tips for writing and running unit tests for Go code.

## Conventions

- **Naming tests in Go code.** We strive to follow the same naming convention for Go test functions as described for [naming example functions in the Go testing package](https://golang.org/pkg/testing/#hdr-Examples).

## Organizing code and refactoring for testability

If you find yourself having a difficult time testing a piece of Go code, it is likely due to one of the following factors:

- The code does too much
- The code relies on global state
- The code relies on real external services

In all of these cases, you will find it easier to test if you break the code into smaller units where each one can be tested independently and without global state or external behavior.

**If your code relies on a global or external service**, then you can refactor that piece of code to instead take a _reference_ to that service. This section runs through an example of how to refactor such code without affecting the callers. The original code relies on a global database connection, but the same issue occurs in code that refers to external APIs (GitHub, Slack, Zoekt, etc).

```go
func UntestableFunction() (*SomeThing, error) {
    value, err := GlobalDbConn.DoThing()
    check(err)

    st := // some things with value
    return st, nil
}
```

First, we break this into two parts: a public function with the same signature, and a new function that injects its dependencies explicitly.

```go
func UntestableFunction() (*SomeThing, error) {
    return testableFunction(GlobalDbConn)
}

func testableFunction(conn *DbConn) (*SomeThing, error) {
    value, err := conn.DoThing()
    check(err)

    st := // some things with value
    return st, nil
}
```

If the dependency of `testableFunction` is already something that can be mocked (if the value is an interface, or the struct value can be created easily and has very little internal behavior), then most of the benefit has already been seen. 

Otherwise, we should apply an additional step to make the dependency mockable in the tests. This requires that we extract the _interface_ of the dependency from its implementation, and take any value that conforms to that interface.

```go
func UntestableFunction() (*SomeThing, error) {
    return testableFunction(GlobalDbConn)
}

type IDBConn interface {
    DoThing() (*Thing, error)
}

func testableFunction(conn IDBConn) (*SomeThing, error) {
    value, err := conn.DoThing()
    check(err)

    st := // some things with value
    return st, nil
}
```

Now, the tests for this new function can create their own `IDBConn` with whatever behavior for `DoThing` that is required. See the section on [mocking](#mocks) below for more details.

## Testing APIs

External HTTP APIs should be tested with the `"httptest"` package.

For an example usage, see the [bundle manager client tests](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@0cb60598806d68e4c4edace9ed2a801e3f8495bf/-/blob/enterprise/internal/codeintel/bundles/client/bundle_client_test.go#L13), that mock the internal bundle manager service with canned responses. Request values are asserted in the test HTTP handler itself, comparing the requested HTTP method, path, and query args against the expected values.

If you need to test interactions with an external HTTP API, take a look at the `"httptestutil"` package. The `NewRecorder` function can be used to create an HTTP client that records and replays HTTP requests. See [`bitbucketcloud.NewTestClient`](https://github.com/sourcegraph/sourcegraph/blob/f2e55799acad8b6b28cb3b6fd47cc55993d36dc4/internal/extsvc/bitbucketcloud/testing.go#L22-L47) for an example. Or take a look at [other usages of `httptestutil.NewRecorder`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@master/-/blob/enterprise/internal/campaigns/resolvers/main_test.go#L37:27&tab=references).

## Assertions

We use the Go's stdlib `"testing"` package to drive unit tests and assertions. Here are some tips when writing tests:

### Handling unexpected errors

An unexpected error value in a test should be met with an immediate failure. This prevents the test from emitting spurious errors, as either the function's remaining results will be zero-values, or there was a failure to perform a specific side-effect that the remainder of the test may rely on having occurred.

```go
func TestSprocket(t *testing.T) {
    value, err := NewSprocket().Sprock(context.Background())
    if err != nil {
        t.Fatalf("unexpected error sprocking: %s", err)
    }

    // value can be asserted safely here
}
```

### Asserting expected complex values

Expected values that are not simple scalars (really, anything not comparable with `==`) should use [go-cmp](https://github.com/google/go-cmp) to create a highly-readable diff string.

```go
import "github.com/google/go-cmp/cmp"

func TestCoolPlanets(t *testing.T) {
	planets := CoolPlanets()

	expectedPlanets := []string{
		"Neptune",
		"Uranus",    // ...
		"Pluto",     // we still love you
		"NOT Venus", // this thing is hot af
	}
	if diff := cmp.Diff(expectedPlanets, planets); diff != "" {
		t.Errorf("unexpected planets (-want +got):\n%s", diff)
	}
}
```

*Caveat*: go-cmp uses reflection therefore all comparable fields must be exported.

## Mocks

If your code depends on a value defined as an interface (which all dependencies **should** be), you can use [derision-test/go-mockgen](https://github.com/derision-test/go-mockgen) to create programmable stubs that conform to the target interface. This replaces an old pattern in the code base that declared mocks defined as [globally settable functions](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@b5b5fc8d885710eb559ff3d6122c9360b31fec78/-/blob/internal/vcs/git/mocks.go#L15).

For an example usage of generated mocks, see the [TestDefinitions](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@0cb60598806d68e4c4edace9ed2a801e3f8495bf/-/blob/enterprise/internal/codeintel/resolvers/query_test.go#L16) test for the code intel resolvers. Each method defined on an interface has a default implementation that returns zero-values for all of its results and can be configured to:

- call a hook function on every invocation (via `SetDefaultHook`)
- call a hook function for the _next_ invocation (via `PushHook`)
- return canned values on every invocation (via `SetDefaultReturn`)
- return canned values for the _next_ invocation (via `PushReturn`)

Using `PushHook` and `PushReturn`, you can create an ordered stack of results (e.g. return `X`, then `Y`, then `Z`, then return `W` for every subsequent invocation). Hooks also let you perform additional logic based on the input parameters, test-local state, and perform side effects on invocation.

Invocations can be inspected by querying the `History` of a method, which stores the parameter values and return values of each invocation of the mock. This provides a concise way to later examine invocation counts and parameter values to ensure the dependency is being called in the expected manner.

## Testing time

Testing code that interfaces with wall time is [difficult](https://github.com/sourcegraph/sourcegraph/pull/11426#issuecomment-642428217).

If you have code that requires use of the `"time"` package, your first action should be to refactor the code, if possible, so that the parts that deal with time and the parts that deal with the other testable logic are cleanly separable. If your function calls `time.Now`, see if it is possible to pass the current time as a parameter. If your function sleeps or requires a ticker, see if you can extract the time-irrelevant code into a function that can be tested separately.

If all else fails, you can make use of [derision-test/glock](https://github.com/derision-test/glock) to mock the behavior of the time package. This requires that the code under test uses a `glock.Clock` value rather than the time package directly (see the [section above](#organizing-code-and-refactoring-for-testability) for tips on refactoring your code). This package provides a _real_ clock implementation, which is a light wrapper over the time package, and a _mock_ clock implementation that allows the clock (and underlying tickers) to be advanced arbitrarily during the test.

For an example usage of a mock clock, see the [TestRetry](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@0cb60598806d68e4c4edace9ed2a801e3f8495bf/-/blob/enterprise/internal/codeintel/bundles/client/retry_test.go#L13) test, which tests a retry loop with a sleeping backoff. This test decouples wall time from the clock by advancing the clock in a goroutine as far as necessary to unstick the execution of the retry loop.

## Testing with a database

When testing code that depends on a database connection, you may want to test how your code interacts with a real database, or you may want to mock out the database calls to speed up tests and isolate the logic being tested.

### Testing with a mocked database

Helpers for mocking out a database can be found in the `internal/database/dbmocks` package. For each store in `internal/database`, as well as for the `database.DB` type, there is an associated mock in the `dbmocks` package that can be used in place of the store or db interface. The mocks are generated with `go-mockgen` (see "Mocks" above for details).

```go
func getRepo(db database.DB, id int) *types.Repo {
	return db.Repos().Get(id)
}

func TestGetRepos(t *testing.T) {
	t.Parallel()
	repoStore := dbmocks.NewMockRepoStore()
	repoStore.GetFunc.SetDefaultReturn(&types.Repo{Name: "my cool repo", ID: 123})

	db := dbmocks.NewMockDB()
	db.ReposFunc.SetDefaultReturn(repoStore)

	got := getRepos(db, 0)
	if got.ID != 123 {
		t.Fatalf("wrong ID: %d", got.ID)
	}
}
```

Note that, in order for the mock repo store to be used in the tested function, we need to use the `db.Repos()` method rather than the function constructors like `database.Repos(db)`. In time, these function constructors will be removed, but if you're running into nil panics when trying to use the injected mocks, this is probably the issue.

Additionally, you might see instances of global database mocks around the codebase, like `database.Mocks.Repos.Get = func() []*types.Repo { ... }`. These global mocks are deprecated, and will be removed in time. Prefer the injected database mocks to the global mocks.

### Testing with a real database

If you would like to run your test against a real database instance, look no further than the `internal/database/dbtest` package. To get a handle to a freshly migrated database, just call `dbtest.NewDB(t)`. This will return a new `*sql.DB` handle that points to a clean database instance that will only be used for the current test, so you don't have to worry about conflicts with other tests, and your tests can run in parallel.

Note that getting a new, clean database instance is somewhat expensive, so if you can reuse a handle between sub-tests, it's likely worth it. It costs ~3s for the first call to `dbtest.NewDB()` in a package, then ~0.4s for each additional call after that. We only migrate the database once per package, but copying a migrated database still isn't free.

```go
func createRepo(db database.DB, name string, id int) {
	db.Repos().Create(name, id)
}

func getRepo(db database.DB, id int) *types.Repo {
	return db.Repos().Get(id)
}

func TestGetRepos(t *testing.T) {
	t.Parallel()
	db := dbtest.NewDB(logger, t)

	createRepo(db, "my cool repo", 123)
	got := getRepo(db, 123)

	if got.Name != "my cool repo" {
		t.Fatalf("wrong name: %s", got.Name)
	}
}
```

You will also see references to the `internal/database/dbtesting` package in the codebase, but use of that package is waning because it relies on a global database connection, which is not isolated between tests, and cannot be safely parallelized.

### Verifying fixes to flaky tests

For verifying fixes to flaky tests, pass the `-count` flag to `go test`.
This bypasses the default caching of test results, and repeatedly runs the test in a loop.
From the [official `go test` docs](https://pkg.go.dev/cmd/go/internal/test):

```text
	-count n
	    Run each test, benchmark, and fuzz seed n times (default 1).
	    If -cpu is set, run n times for each GOMAXPROCS value.
	    Examples are always run once. -count does not apply to
	    fuzz tests matched by -fuzz.
```

Example usage:

```bash
go test ./path/to/package -run MyTestName -count 100
```
