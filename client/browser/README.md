# Sourcegraph browser extension

[![code style: prettier](https://img.shields.io/badge/code_style-prettier-ff69b4.svg)](https://github.com/prettier/prettier)
![license](https://img.shields.io/badge/license-MIT-blue.svg)

[![chrome version](https://img.shields.io/chrome-web-store/v/dgjhfomjieaadpoljlnidmbgkdffpack.svg?logo=Google%20Chrome&logoColor=white)](https://chrome.google.com/webstore/detail/sourcegraph/dgjhfomjieaadpoljlnidmbgkdffpack)
[![chrome users](https://img.shields.io/chrome-web-store/users/dgjhfomjieaadpoljlnidmbgkdffpack.svg)](https://chrome.google.com/webstore/detail/sourcegraph/dgjhfomjieaadpoljlnidmbgkdffpack)
[![chrome rating](https://img.shields.io/chrome-web-store/rating/dgjhfomjieaadpoljlnidmbgkdffpack.svg)](https://chrome.google.com/webstore/detail/sourcegraph/dgjhfomjieaadpoljlnidmbgkdffpack)

## Overview

The Sourcegraph browser extension adds tooltips to code on GitHub, Phabricator, and Bitbucket.
The tooltips include features like:

- symbol type information & documentation
- go to definition & find references (currently for Go, Java, TypeScript, JavaScript, Python)
- find references

#### 🚀 Install: [**Sourcegraph for Chrome**](https://chrome.google.com/webstore/detail/sourcegraph/dgjhfomjieaadpoljlnidmbgkdffpack)

#### 🚀 Install: [**Sourcegraph for Firefox**](https://sourcegraph.com/docs/integration/browser_extension)

#### 🚀 Install: [**Sourcegraph for Safari**](https://apps.apple.com/us/app/sourcegraph-for-safari/id1543262193)

It works as follows:

- when visiting e.g. https://github.com/..., the extension injects a content script (contentPage.main.bundle.js)
- there is a background script running to access certain chrome APIs, like storage (backgroundPage.main.bundle.js)
- a "code view" contains rendered (syntax highlighted) code (in an HTML table); the extension adds event listeners to the code view which control the tooltip
- when the user mouses over a code table cell, the extension modifies the DOM node:
  - text nodes are wrapped in `<span>` (so hover/click events have appropriate specificity)
  - element nodes may be recursively split into multiple element nodes (e.g. a `<span>&Router{namedRoutes:<span>` contains multiple code tokens, and event targets need more granular ranges)
  - We assume syntax highlighting takes care of the base case of wrapping a discrete language symbol
  - tooltip data is fetched from the Sourcegraph API
- when an event occurs, we modify a central state store about what kind of tooltip to display
- code subscribes to the central store updates, and creates/adds/removes/hides an absolutely positioned element (the tooltip)

## Project layout

- `src/`
  - `browser-extension/`
    Entrypoint for browser extension builds. (Includes bundled assets, background scripts, options)
    - `web-extension-api/`
      [A wrapper around the web extension APIs.](./src/browser-extension/web-extension-api/README.md)
  - `native-integration/`
    Entrypoint for the native code host integrations (Phabricator, Gitlab and Bitbucket).
  - `shared/`
    Code shared by the browser extension and the native integrations. Ideally, nothing in here should reach into any other directory.
    - `code-hosts/`
      Contains the implementations of code-host specific features for each supported code host.
      - `shared/`
        Code shared between multiple code hosts.
  - `config/`
    Configuration code that adds properties to `window` that make it easier to tell what environment the script is running in. This is useful because the code can be run in the content script, background, options page, or in the actual page when injected by Phabricator and each environment will have different ways to do different things.
  - `end-to-end/`
    E2E test suite.
- `scripts/`
  Build scripts.
- `config/`
  Build configs.
- `build/`
  Generated directory containing the build output and the generated bundles for each browser.

## Requirements

- `node`
- `pnpm`
- `make`

## Development

To build all the browser extensions for all browsers at once:

```bash
pnpm run dev
```

To only build for a single browser (which makes builds faster in local development), set the env var `TARGETS=chrome` or `TARGETS=firefox`.

Now, follow the steps below for the browser you intend to work with.

### Chrome

- Browse to [chrome://extensions](chrome://extensions).
- If you already have the Sourcegraph extension installed, disable it using the toggle.
- Enable 'developer mode', click on [Load unpacked extensions](https://developer.chrome.com/extensions/getstarted#unpacked), save it in the `sourcegraph/client/browser/build/chrome` folder.
- Browse to any public repository on GitHub to confirm it is working.
- After making changes, it is necessary to refresh the extension. This is done by going to [chrome://extensions](chrome://extensions) and clicking the "Reload" icon.

![File-path](https://user-images.githubusercontent.com/20326070/96859153-75764300-1461-11eb-8b82-0febc9327723.png)

#### Updating the bundle

Click reload for Sourcegraph at `chrome://extensions`

### Firefox (manual)

- Go to `about:debugging`
- Select "Enable add-on debugging"
- Click "Load Temporary Add-on" and select "firefox-bundle.xpi"
- [More information](https://developer.mozilla.org/en-US/docs/Tools/about:debugging#Add-ons)

#### Updating the bundle

Click reload for Sourcegraph at `about:debugging`

### Safari

> **Note**: Requires MacOS with Xcode installed

1. `pnpm --filter @sourcegraph/browser dev:safari`
1. Open Safari then: `Develop > Allow Unsigned Extensions`
1. Open `client/browser/build/Sourcegraph for Safari/Sourcegraph for Safari.xcodeproj` using Xcode
1. Choose `Sourcegraph for Safari (macOS)` in the top toolbar and click Run icon
1. It should build and add extension to your local Safari (Check `Preferences > Extensions tab`)

[See more details.](https://developer.apple.com/documentation/safariservices/safari_web_extensions/running_your_safari_web_extension)

## Testing

- Unit tests: `sg test bext`
- Integration tests
  - run
    - build browser extension with `sg test bext-build`
    - run tests with `sg test bext-integration`
  - develop
    - add/edit test case or at least its part with navigation to a certain page
    - if there's no page snapshot for created test or page URL referenced in the existing test has been changed, test will fail with 'Page not found' error
    - to generate or update page snapshots for tests run `pnpm record-integration`
- E2E tests: `sg test bext-build` & `sg test bext-e2e`

### E2E tests

The test suite in `end-to-end/github.test.ts` runs on the release branch `bext/release` in both Chrome and Firefox against a Sourcegraph Docker instance.

The test suite in end-to-end/phabricator.test.ts tests the Phabricator native integration.
It assumes an existing Sourcegraph and Phabricator instance that has the Phabricator extension installed.
There are automated scripts to set up the Phabricator instance, see https://sourcegraph.com/docs/dev/phabricator_gitolite.
It currently does not run in CI and is intended to be run manually for release testing.

`end-to-end/bitbucket.test.ts` tests the browser extension on a Bitbucket Server instance.

`end-to-end/gitlab.test.ts` tests the browser extension on gitlab.com (or a private Gitlab instance).

### Integration tests

All test suites in `integration` run in CI. These tests run the browser extension against recordings of code hosts (using [Polly.JS](https://netflix.github.io/pollyjs/#/)) and mock data for our GraphQL API.

To update all recordings, run `pnpm record-integration`. To update a subset of recordings, run `POLLYJS_MODE=record SOURCEGRAPH_BASE_URL=https://sourcegraph.com pnpm test-integration --grep=YOUR_PATTERN`, where `YOUR_PATTERN` is typically a test name.

## Deploy

Deployment the Chrome web store happen automatically in CI when the `bext/release` branch is updated.
Releases are also uploaded to the [GitHub releases
page](https://github.com/sourcegraph/browser-extensions/releases) and tagged in
git.

To release the latest commit on `main`, ensure your `main` branch is up-to-date and run:

```sh
git push origin main:bext/release
```

## Manual build of the browser extension

This describes the manual build process to produce the packed extension (xpi/zip) from scratch from the source code.

Requires `node` version specified in [`.nvmrc`](../.nvmrc). In the steps below we use [`nvm`](https://github.com/nvm-sh/nvm) to automatically select the node version.

Tested on Ubuntu 20.04 and Mac OS 10.15.5.

### Obtain the source code

#### A. Obtain the source code by cloning the repository

Clone the public repository with `git clone`:

```sh
git clone git@github.com:sourcegraph/sourcegraph
cd sourcegraph
```

#### B. Obtain the source code by downloading the zip

Alternatively (instead of cloning the repository), you can obtain the source code as a zip for a particular commit hash or a branch.

For example, to build from commit `e1547ea0e9`:

```sh
curl -OL https://github.com/sourcegraph/sourcegraph/archive/e1547ea0e9.zip
unzip e1547ea0e9.zip
cd sourcegraph-e1547ea0e99475dd748a4e3bb1a81cee71c0f7fd
```

### Install dependencies and build

Use `nvm` to select the Node.js version specified in `.nvmrc`.

```sh
nvm install
```

Install dependencies with `pnpm install` (install it globally with `npm i -g pnpm` if needed) and build.

```sh
pnpm install
pnpm run build-browser-extension
```

The build step automatically pulls in [sourcegraph/code-intel-extensions](https://github.com/sourcegraph/code-intel-extensions) as a dependency.

The output will be in `browser/build`:

- Firefox add-on:
  - Packed: `browser/build/bundles/firefox-bundle.xpi`
  - Unpacked: `browser/build/firefox`
- Chrome extension:
  - Packed: `browser/build/bundles/chrome-bundle.zip`
  - Unpacked: `browser/build/chrome`

## Create a zip of the browser extension source code

The `pnpm run create-source-zip` command will create `sourcegraph.zip`, an archive of the source that can be used to do a build of the browser extension.

This will pull the source code at a given revision (by default, the `bext/release` branch on GitHub) and create a zip of the source code, which can then be used to reproduce the exact build. Some directories of the repo, which are not relevant to the browser extension, are excluded from the archive.

See [scripts/create-source-zip.js](scripts/create-source-zip.js).

Use this process to create a source code zip to attach to a Firefox add-on submission.

```
cd client/browser
pnpm run create-source-zip
```

## Testing the Phabricator native extension locally

This is an adjusted version of @lguychard's [comment here](https://github.com/sourcegraph/sourcegraph/pull/11825#issuecomment-652040751).

1. Start a Sourcegraph instance locally (`sg start`)
2. Start Phabricator instance, running the latest version of Phabricator, using `./dev/phabricator/start.sh`
3. Install the Sourcegraph extension into Phabricator using `./dev/install-sourcegraph.sh`
4. Navigated to `127.0.0.1` to access the Phabricator instance, and logged in with `admin` / `sourcegraph` (as printed by `./dev/phabricator/start.sh`)
5. Navigated to `http://127.0.0.1/config/group/sourcegraph/` (Config -> Application Settings -> Sourcegraph)
6. Edited `corsOrigin` in site config at `https://sourcegraph.test:3443/site-admin/configuration` to include `http://127.0.0.1` (might need to be added in `dev-private` repo).
7. Added a repository by mirroring a public GitHub repository (we have some docs for this [here](https://sourcegraph.com/docs/dev/phabricator_gitolite), but they need improving):
   - Navigated to `http://127.0.0.1/diffusion/edit/?vcs=git`
   - Created a repository with name `JsonRPC2`, callsign `JRPC`, short name `jrpc`
   - Navigated to `http://127.0.0.1/source/jrpc/manage/uris/` (URIs)
   - For all three default URIs:
     - Edited, set I/O type "No IO", Display type "Hidden")
   - Added a new URI, set it to `https://github.com/sourcegraph/jsonrpc2.git`
   - Set I/O type "Observe", display type "Visible" (to mirror from GitHub)
   - In `http://127.0.0.1/source/jrpc/manage/`, "Activate repository"
   - Reload `http://127.0.0.1/source/jrpc/manage/` until status showed up as "Fully imported"
8. If you are having issues with the daemons not updating the status, you can do the following:
   - Shell into the Docker container running Phabricator: `docker exec -it phabricator /bin/bash`
   - Switch into the main Phabricator directory: `cd /opt/bitnami/phabricator`
   - Restart the daemons: `./bin/phd restart`
9. Navigated to `http://127.0.0.1/source/jrpc/browse/master/async.go`

Whenever you make changes to the native extension code, you need to run `pnpm build` inside the `client/browser` directory before seeing the changes in effect on the Phabricator instance.
Hello World
