# Sourcegraph SvelteKit

This folder contains the experimental [SvelteKit](https://kit.svelte.dev/)
implementation of the Sourcegraph app.

## Developing

There are multiple ways to start the app:

1. Standalone and proxying to S2

```bash
cd client/web-sveltekit
pnpm dev
```

Then go to (usually) http://localhost:5173.

Or via `sg`:

```bash
sg start web-sveltekit-standalone
```

Then go to https://sourcegraph.test:5173.

2. Standalone and proxying to dotcom

```bash
cd client/web-sveltekit
pnpm dev:dotcom
```

3. Standalone and proxying to another Sourcegraph instance

```bash
cd client/web-sveltekit
SOURCEGRAPH_API_URL=https://<instance> pnpm dev
```

Then go to (usually) http://localhost:5173.

3. Against a local Sourcegraph instance

```bash
sg start enterprise-sveltekit
```

Then go to https://sourcegraph.test:5173.

### Using code from `@sourcegraph/*`

There are some things to consider when using code from other `@sourcegraph`
packages:

- Since we use the [barrel](https://basarat.gitbook.io/typescript/main-1/barrel)
  style of organizing our modules, many (unused) dependencies are imported into
  the app. This isn't ideal and at best will only increase the initial loading
  time. Some modules, especially those that access browser specific features
  during module initialization, can even cause the dev build to fail.
- Reusing code is great, but also potentially exposes someone who modifies the
  reused code to this package and therefore Svelte (if the reused code changes
  in an incompatible way, this package needs to be updated too). To limit the
  exposure, a module of any `@sourcegraph/*` package should only be imported
  once into this package and only into a TypeScript file.
  The current convention is to import any modules from `@sourcegraph/common`
  into `src/lib/common.ts`, etc.

### Tests

We use vitest for unit tests and playwright for integration tests. Both of these
are located next to the source files they test.
Vitest files end with `.test.ts` and Playwright files end with `.spec.ts`.

For example the Playwright test for testing `src/routes/search/+page.svelte`
is located at `src/routes/search/page.spec.ts`.

Locally you can run the tests with

```sh
pnpm vitest # Run vitest tests
pnpm test # Run playwright tests
```

You can also run playwright tests against a running vite dev server. This is
useful for debugging tests.

```sh
# In one terminal
pnpm dev
```

```sh
# In another terminal
pnpm test:dev
```

In CI, we run vitest and playwright via the `BUILD.bazel` file. You can run e2e tests locally with

```sh
sg bazel test //client/web-sveltekit:e2e_test
```

### Updating Playwright

The Playwright version is defined in the `package.json` of this package. The browser versions are defined in `dev/tool_deps.bzl`.

You may have to find the right combination for both tools to work nicely with each other. The easiest is to start with updating
Playwright, pushing it to CI, and seeing what happens. We will first upgrade
Playwright and install the new browsers, and then update `dev/tool_deps.bzl` based on the newly installed browsers.

**1. Upgrade Playwright**

To [update Playwright](https://playwright.dev/docs/intro#updating-playwright), navigate to `clients/web-sveltekit` and
run `pnpm add @playwright/test@latest playwright@latest` followed by `pnpm exec playwright install --with-deps`.

**2. Update Bazel**

The `install` command from above may have downloaded new browsers. You may see a log message like (on macOS) `Chromium 128.0.6613.18 (playwright build v1129) downloaded to /Users/your-user/Library/Caches/ms-playwright/chromium-1129`.

If you don't have the logs anymore, you can run `pnpm exec playwright install --dry-run` to get an overview.

If your latest browser version is newer than what's listed in `dev/tool_deps.bzl`, you'll need to update `dev/tool_deps.bzl` to
include the new browser version and the zip file's sha integrity. You can calculate it yourself, or
run e.g. `bazel test //client/web-sveltekit:e2e_test` to see the new integrity sha. Example below:

```
Error in download_and_extract: java.io.IOException: Error downloading [https://playwright.azureedge.net/builds/chromium/1129/chromium-mac-arm64.zip] to /private/var/tmp/_bazel_michael/680fb57cd51801cfe03bf19f9d7a0d3e/external/chromium-darwin-arm64/temp15834460500730224298/chromium-mac-arm64.zip: Checksum was sha256-WdF50K2a15LlHbga7y17zBZOb130NMCBiI+760VovQ4= but wanted sha256-5wj+iZyUU7WSAyA8Unriu9swRag3JyAxUUgGgVM+fTw=
```

### Formatting and linting

This package defines its own rules for formatting (which includes support for
Svelte components) and linting. The workspace rules linting and formatting
commands have not been updated yet to keep this experiment contained.

Run

```sh
pnpm run lint
pnpm run format
```

inside this directory.

There is also the `pnpm run check` command which uses `svelte-check` to validate
TypeScript, CSS, etc in Svelte components. This currently produces many errors
because it also validates imported modules from other packages, and we are not
explicitly marking type-only imports with `type` in other parts of the code
base (which is required by this package).
This noise can be avoided by running the corresponding bazel command instead:

```sh
bazel test //client/web-sveltekit:svelte-check
```

### Icons

We use [unplugin-icons](https://github.com/unplugin/unplugin-icons) together
with [unplugin-auto-import](https://github.com/unplugin/unplugin-auto-import)
to manage icons. This allows us to use icons from multiple icon sets without
having to import them manually.

For a list of currently available icon sets see the `@iconify-json/*` packages
in the `package.json` file.

Icon references have the form `I<IconSetName><IconName>`. For example the
[corner down left arrow from Lucide](https://lucide.dev/icons/corner-down-left)
can be referenced as `ILucideCornerDownLeft`.

The icon reference is then used in the `Icon` component. Note that the icon
doesn't have to be imported manually.

```svelte
<script lang="ts">
  import { Icon } from '$lib/Icon.svelte';
</script>

<Icon icon={ILucideCornerDownLeft} />
```

When the development server is running, the icon will be automatically added to
`auto-imports.d.ts` so TypeScript knows about it.

### Data loading with GraphQL

This project makes use of query composition, i.e. components define their own
data dependencies via fragments, which get composed by their callers and are
eventually being used in a query in a loader.

This goal of this approach is to make data dependencies co-located and easier
to change, as well to make the flow of data clearer. Data fetching should only
happen in data loaders, not components.

There are a couple of issues to consider with this approach and sometimes we'll
have to make exceptions:

- Caching: If every loader composes its own query it's possible that two
  queries fetch the same data, in which case we miss out on caching. If caching
  the data is more important than data co-location it might be preferable to
  define a reusable query function. Example: File list for currently opened
  folder (sidebar + folder page)
- Shared data from layout loaders: While it's very convenient that pages have
  access to any data from the ancestor layout loaders, that doesn't work well
  with data dependency co-location. The layout loaders don't know which
  sub-layout or sub-page is loaded and what data it needs.
  Fortunately we don't have a lot of data (yet) that is used this way. The
  prime example for this right now is information about the authenticated user.
  The current approach is to name data-dependencies on the current user as
  `<ComponentName>_AuthenticatedUser` and use that fragment in the
  `AuthenticatedUser` fragment in `src/routes/layout.gql`.
  This approach might change as we uncover more use cases.
- On demand data loading: Not all data is fetched/needed immediately for
  rendering page. Data for e.g. typeaheads is fetched on demand. Ideally the
  related queries are still composed by the data loader, which passes a
  function for fetching the data to the page.

### Rolling out pages to production

For a page to be accessible in production, the server needs to know to serve the
SvelteKit for that page. Due to file based routing we can easily determine available
pages during build time. The list of available pages is generated by the `sg generate`
command, which in turn runs `bazel run //client/web-sveltekit:write_generated`.

To enable a page in production by default, add the following comment to the `+page.svelte`
file:

```svelte
<script lang="ts">
   // @sg EnableRollout
   ...
</script>
```

and run `sg generate` or `bazel run //client/web-sveltekit:write_generated` to update the
list of available pages.

## Production build

A production version of this app can be built with

```sh
pnpm run build
```

Currently SvelteKit is configured to create a client-side single page
application.
