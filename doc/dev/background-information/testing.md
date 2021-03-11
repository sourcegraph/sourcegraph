# Testing

_This documentation is specifically for the tests in the [sourcegraph/sourcegraph](https://github.com/sourcegraph/sourcegraph) repository. For our general testing principles, please see "[Testing Principles](testing_principles.md)"._

## Backend tests

To run tests for the Go backend, run `go test ./...`, or specify a package
directly, `go test ./util/textutil`.

## Client unit tests (web app and browser extension)

- First run `yarn` in the Sourcegraph root directory if it is a fresh clone.
- To run all unit tests, run `yarn test` from the root directory.
- To run unit tests in development (only running the tests related to uncommitted code), run `yarn test --watch`.
  - And/or use [vscode-jest](https://github.com/jest-community/vscode-jest) with `jest.autoEnable: true` (and, if you want, `jest.showCoverageOnLoad: true`)
- To debug tests in VS Code, use [vscode-jest](https://github.com/jest-community/vscode-jest) and click the **Debug** code lens next to any `test('name ...', ...)` definition in your test file (be sure to set a breakpoint or break on uncaught exceptions by clicking in the left gutter).
- You can also run `yarn test` from any of the individual project dirs (`client/shared/`, `client/web/`, `client/browser/`).

Usually while developing you will either have `yarn test --watch` running in a terminal or you will use vscode-jest.

Test coverage from unit tests is tracked in [Codecov](https://codecov.io/gh/sourcegraph/sourcegraph) under the `unit` flag.

### React component snapshot tests

[React component snapshot tests](https://jestjs.io/docs/en/tutorial-react) are one way of testing React components. They make it easy to see when changes to a React component result in different output. Snapshots are files at `__snapshots__/MyComponent.test.tsx.snap` relative to the component's file, and they are committed (so that you can see the changes in `git diff` or when reviewing a PR).

- See the [React component snapshot tests documentation](https://jestjs.io/docs/en/tutorial-react).
- See [existing test files that use `react-test-renderer`](https://sourcegraph.com/search?q=repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+lang:typescript+react-test-renderer) for usage examples.
- Use the jest watcher's <kbd>u</kbd> keyboard shortcut (or `yarn test --updateSnapshot`) to update all snapshot files. Be sure to review the diff!

## Browser-based tests

Browser-based tests act like a user by opening a browser and clicking, typing, and navigating around in an automated fashion.
We use [Puppeteer](https://pptr.dev/) to control the browser programmatically, while the test itself runs in the test runner [Mocha](https://mochajs.org/).

We have two kinds of these tests in accordance with our [testing principles in the handbook](https://about.sourcegraph.com/handbook/engineering/testing#testing-pyramid).
Please refer to the handbook for the trade-offs and use cases of each, and find specific instructions on how to run each further below.

### Debugging browser-based tests

During a test run, the console from the browser will also be printed to the terminal, prefixed with "🖥 Browser console:".
Not every browser error log indicates a failure, but it can be helpful in debugging.
Make sure to always first look at the test failure at the bottom of the logs, which includes the error message and stack trace.

When a test fails, a screenshot is saved to the `./puppeteer` directory.
In iTerm (macOS) and on Buildkite, it is also displayed inline in the terminal log output.
This may trigger a prompt "Allow Terminal-initiated download?" in iTerm.
Tick "Remember my choice" and click "Yes" if you want the inline screenshots to show up.

When a browser-based test fails ([example](https://buildkite.com/sourcegraph/sourcegraph/builds/29935#1ee967cf-eb2e-4af0-8afc-0770d1779c1d)), CI displays a snapshot of the failure [inline](https://buildkite.com/docs/pipelines/links-and-images-in-log-output) in the Buildkite output and Jest prints the :

For end-to-end tests that failed in CI, a video of the session is available in the **Artifacts** tab:

![image](https://user-images.githubusercontent.com/1387653/54873783-65851180-4d9b-11e9-98e5-e23b1166f2c5.png)

#### Driver options

Our test driver accepts various environment variables that can be used to control Puppeteer's behavior:

| Environment variable  | Purpose                                                          |
| --------------------- | ---------------------------------------------------------------- |
| `BROWSER`             | Whether to run `firefox` or `chrome` (default)                   |
| `LOG_BROWSER_CONSOLE` | Log the browser console output to the terminal (default `true`). |
| `SLOWMO`              | Slow down each interaction by a delay (ms).                      |
| `HEADLESS`            | Run the tests without a visible browser window.                  |
| `DEVTOOLS`            | Whether to run all tests with the browser devtools open          |
| `KEEP_BROWSER`        | If `true`, browser window will remain open after tests ran       |

#### Filtering tests

There are multiple useful ways you can filter the running tests for debugging.

To stop the test run on the first failing test, append `--bail` to your command.

You can also single-out one test with `it.only`/`test.only`:

```TypeScript
it.only('widgetizes quuxinators', async () => {
    // ...
})
```

Alternatively, you can use `-g` to filter tests, e.g. `env ... yarn test-e2e -g "some test name"`.

You can find a complete list of all possible options in the [Mocha documentation](https://mochajs.org/#command-line-usage).

#### Troubleshooting failing browser-based tests

Some common failure modes:

- Timed out waiting for http://localhost:7080 to be up: the `sourcegraph/server` container failed to start, so check the container logs that appear further down in the Buildkite output.
- Timed out waiting for a selector to match because the CSS class in the web app changed: update the test code and implementation if the CSS selector is not a stable `test-*` identifier.
- Timed out waiting for a selector to match because the page was still loading: use `waitForSelector(selector, { visible: true })`.
- Page disconnected or browser session closed: another part of the test code might have called `page.close()` asynchronously, the browser crashed (check the video), or the build got canceled.
- Node was detached from the DOM: components can change the DOM asynchronously, make sure to not rely on element handles.
- Timing problems: Use `retry()` to "poll" for a condition that cannot be expressed through `waitForSelector()` (as opposed to relying on a fixed `setTimeout()`).

Retrying the Buildkite step can help determine whether the test is flaky or broken. If it's flaky, [disable it with `it.skip()` and file an issue on the author](https://about.sourcegraph.com/handbook/engineering/testing#flaky-tests).

#### Viewing browser-based tests live in CI

In the rare condition that CI appears stuck on end-to-end or integration tests and the video recording does not help, you can view the screen in [VNC Viewer](https://www.realvnc.com/en/connect/download/viewer/) (free) by forwarding port 5900 to the pod. Find the pod name on the top right of the step in Buildkite:

![image](https://user-images.githubusercontent.com/1387653/54874133-9157c580-4da2-11e9-89c8-d8c1e53687ad.png)

You might have to inspect element to view it.

Drop the `-N` suffix from the name, then run:

```
gcloud container clusters get-credentials ci --zone us-central1-a --project sourcegraph-dev
kubectl port-forward -n buildkite <buildkite agent pod> 5900:5900
```

Open VNC Viewer and type in `localhost:5900`. Hit <kbd>Enter</kbd> and accept the warning. Now you'll be able to see what's causing the tests to hang (e.g. a prompt or alert that hasn't been dismissed).

### Client integration tests

Client integration tests test only the client code (JS and CSS).
The role of these integration tests is to provide in-browser testing of complex UI flows in isolation from the Sourcegraph backend.
All backend interactions are stubbed or recorded and replayed.
The integration test suite for the webapp can be found in [`web/src/integration`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/web/src/integration).

Test coverage from integration tests is tracked in [Codecov](https://codecov.io/gh/sourcegraph/sourcegraph) under the flag `integration`.

#### Running client integration tests

To run integration tests for the web app:

1. Run `yarn watch-web` in the repository root in a separate terminal to watch files and build a JavaScript bundle. You can also launch it as the VS Code task "Watch web app".
  - Alternatively, `yarn build-web` will only build a bundle once.
  - If you need to build an Enterprise bundle (to test Enterprise features such as Batch Changes), set `ENTERPRISE=1`
1. Run `yarn test-integration` in the repository root to run the tests.

A Sourcegraph instance does not need to be running, because all backend interactions are stubbed.

See the above sections for how to debug the tests, which applies to both integration and end-to-end tests.

#### Writing integration tests

Just like end-to-end tests, integration tests use the [test driver](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/shared/src/testing/driver.ts#L129:17) which is created for each test in a `before()` hook.
In opposite to end-to-end tests, integration tests do not need to set up any backend state.
Instead, integration tests create a _test context_ object before every test using `beforeEach()`, which manages the mocked responses.

##### Mocking GraphQL responses

Calling `testContext.overrideGraphQl()` in a test or `beforeEach()` hook with an object map allows you to override GraphQL queries and mutations made by the client code. The map is indexed by the unique query name specified in the implementation, for example [`ResolveRepo`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph@a3b40f3ae9376b42ce9a67b5a33f177ba98ac050/-/blob/browser/src/shared/repo/backend.tsx?subtree=true#L32-36).
The TypeScript types of the overrides are specifically generated for each query to validate the shape of the mock results and provide autocompletion.
If, during a test, a query is made that has no corresponding mock, the request will be rejected and an error will be logged with details about the query.

There are default mock responses for queries made in almost every test, which you can extend with object spread syntax if needed.

`testContext.overrideGraphQL()` can be called multiple times during a test if the response to one or more queries should change, e.g. after an action was taken that causes a change on the backend (see the next section for how to assert those).

##### Waiting for a mutation and checking passed variables

To verify that the client sent a GraphQL mutation to the backend, you can use `testContext.waitForRequest()`.
Pass it a callback that triggers the request (e.g. clicking a "Save" button in a form).
The function returns the variables that were passed to the mutation, which can be asserted with `assert.deepStrictEqual()`.

Only use `testContext.waitForRequest()` for behavior you need to test, not to generally wait for parts of the application to load.
Whether a query is made for loading is an implementation detail, instead assert and wait on the DOM using `waitForSelector()` or `retry()`.

##### Mocking JSContext

The backend provides the webapp with a context object under `window.context`.
You can override this object in integration tests using `testContext.overrideJsContext()`.
There is a default mock JSContext that you can extend with object spread syntax if needed.

### End-to-end tests

End-to-end tests test the whole app: JavaScript, CSS styles, and backend.
They can be found in [`web/src/end-to-end`](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/tree/web/src/end-to-end).

The **regression test suite** is a special end-to-end test suite, which was created specifically for release testing and also contains some manual verification steps. As part of moving most of our current end-to-end tests to client & backend integration tests, the regression test suite will gradually be trimmed and phased out.

Test coverage by end-to-end tests is tracked in [Codecov](https://codecov.io/gh/sourcegraph/sourcegraph) under the flag `e2e`.

#### Running end-to-end tests

To run all end-to-end tests locally, a local instance needs to be running.
To run the tests against it, **create a user `test` with password `testtesttest` and promote it site admin**.
Then run in the repository root:

```
env TEST_USER_PASSWORD=testtesttest GITHUB_TOKEN=<token> yarn test-e2e
```

There's a GitHub test token in `../dev-private/enterprise/dev/external-services-config.json`.

This will open Chromium, add a code host, clone repositories, and execute the e2e tests.

For regression tests, you can also run tests selectively with a command like `yarn run test:regression:search` in the `web/` directory, which runs the tests for search functionality.

#### Writing end-to-end tests

End-to-end tests need to set up all backend and session state needed for the test in `before()` or `beforeEach()` hooks.
This includes signing the user in, setting up external services and syncing repositories.
Setup should be idempotent, so that tests can be run multiple times without failure or expensive re-setups.
Prefer using the API for setup over clicking through the UI, because it is less likely to change and faster.
The [test driver](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/shared/src/testing/driver.ts#L129:17) has some convenience methods for common tasks, e.g. `driver.ensureExternalService()` and `driver.ensureLoggedIn()`.

### Writing browser-based tests

Open an existing test file of the respective test suite or create a new file if the test you intend to write does not semantically fit into the existing test files.
You can use an existing test file as a template.

Tests follow this shape:

```TypeScript
describe('quuxinator form', () => {
  it('widgetizes quuxinators', async () => {
      await page.goto(baseURL + '/quuxinator/widgetize')
      await page.waitForSelector('.widgetize', { visible: true })
      // ...
  })
  // ... more it()s ...
})
```

The full [Puppeteer API](https://github.com/GoogleChrome/puppeteer/blob/master/docs/api.md) is quite large, but most tests only use a few common commands:

- `await page.goto(baseURL + '/some/route')` navigate to a URL
- `await page.waitForSelector(selector, { visible: true })` wait for an element to appear
- `await page.click(selector)` click on an element (must be visible, but not necessarily in the viewport)

#### Finding elements with CSS selectors

The easiest way to write CSS selectors is to inspect the element in your browser and look at the CSS classes. From there, you can write a selector and get immediate feedback:

![image](https://user-images.githubusercontent.com/1387653/54873834-6c605400-4d9c-11e9-823b-faa8871df395.png)

CSS selectors in e2e tests should always refer to CSS classes prefixed with `test-`. This makes them easy to spot in the implementation and therefor less likely to accidentally break. `test-` classes are never referenced in stylesheets, they are added _in addition_ to styling classes. If an element you are trying to select does not have a `test-` class yet, modify the implementation to add it.

If the element you are trying to select appears multiple times on the page (e.g. a button in a list) and you need to select a specific instance, you can use `data-test-*` attributes in the implementation:

```HTML
<div data-test-item-name={this.props.name}>
  <span>{this.props.name}</span>
  <button className="test-item-delete-button">Delete</button>
</div>
```

Then you can select the button with `[data-test-item-name="foo"] .test-item-delete-button`.

#### Element references

It's generally unreliable to hold references to items that are acted upon later.
In other words, don't do this:

```ts
const elem = page.selector('.selector')
elem.click()
```

Do this:

```ts
page.click('.selector')
```

You can execute more complex interactions atomically _within the browser_ using `page.evaluate()`.
Note that the passed callback cannot refer to any scope variables as it is executed in the browser.
It can however be passed JSON-stringifyable parameters and return a JSON-stringifyable return value.

### Testing visual regressions

#### Reviewing visual changes in a PR

When you submit a PR, a check from https://percy.io/Sourcegraph/Sourcegraph will appear:

![image](https://user-images.githubusercontent.com/1387653/54873096-ac1f3f80-4d8c-11e9-93ff-377b28121df4.png)

If Percy failed CI ❌ then click on the **Details** link to review the visual changes:

![image](https://user-images.githubusercontent.com/1387653/54873144-c0177100-4d8d-11e9-839c-3e344a6d872b.jpg)

Click the image on the right to toggle between diff and full image mode to review the change. Diff mode shows the changes in red.

If the changes are intended, click **Approve** 👍

Once you approve all of the changes, the Percy check will turn green ✅

#### Adding a new visual snapshot test

Open an existing appropiate browser-based test file (end-to-end or integration) or create a new one.
You can take screenshot in any test by calling `percySnapshot()`:

```TypeScript
test('Repositories list', async function () {
    await page.goto(baseURL + '/site-admin/repositories?query=gorilla%2Fmux')
    await page.waitForSelector('[test-repository-name="/github.com/gorilla/mux"]', { visible: true })
    await percySnapshot(page, this.currentTest!.fullTitle())
})
```

When running in CI, this will take a screenshot of the web page at that point in time in the test and upload it to Percy.
When you submit the PR, Percy will fail until you approve the new snapshot.

#### Flakiness in snapshot tests

Flakiness in snapshot tests can be caused by the search response time, order of results, animations, premature snapshots while the page is still loading, etc.

This can be solved with [Percy specific CSS](https://docs.percy.io/docs/percy-specific-css) that will be applied only when taking the snapshot and allow you to hide flaky elements with `display: none`. In simple cases, you can simply apply the `percy-hide` CSS class to the problematic element and it will be hidden from Percy.

## Continuous Integration

The test suite is exercised on every pull request. For the moment CI output
access is limited to Sourcegraph employees, though we hope to enable public
read-only access soon.

The test pipeline is generated by `dev/ci/gen-pipeline.go`, and written to a
YAML file by dev/ci/init-pipeline.yml. This pipeline is immediately scheduled by
Buildkite to run on the Sourcegraph build farm. Some things that are tested
include:

- all of the Go source files that have tests
- dev/check/all.sh (gofmt, lint, go generator, no Security TODOs, Bash syntax, others)
- JS formatting/linting (prettier, eslint, stylelint, graphql-lint)
- Dockerfile linter (hadolint)
- Check whether the Go module folders are "tidy" (go mod tidy)

## Release testing

To manually test against a Kubernetes cluster, use https://k8s.sgdev.org.

For testing with a single Docker image, run something like

```
IMAGE=sourcegraph/server:3.25.2 ./dev/run-server-image.sh
```
