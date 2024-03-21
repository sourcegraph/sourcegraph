# Contributing to Sourcegraph VS Code Extension

Thank you for your interest in contributing to Sourcegraph!
The goal of this document is to provide a high-level overview of how you can contribute to the Sourcegraph VS Code
Extension.
Please refer to our [main CONTRIBUTING](https://github.com/sourcegraph/sourcegraph/blob/main/CONTRIBUTING.md) docs for
general information regarding contributing to any Sourcegraph repository.

## License

Apache

## Feedback

Your feedback is important to us and is greatly appreciated. Please do not hesitate to submit your ideas or suggestions
about how we can improve the extension to
our [VS Code Extension Feedback Discussion Thread](https://github.com/sourcegraph/sourcegraph/discussions/34821) on
GitHub.

## Issues / Bugs

New issues and feature requests can be filed through
our [issue tracker](https://github.com/sourcegraph/sourcegraph/issues/new?labels=team/integrations,vscode-extension&title=VSCode+Bug+report:+&projects=Integrations%20Project%20Board)
using the `vscode-extension` & `team/integrations` label.

## Architecture Diagram

                                   ┌──────────────────────────┐
                                   │  env: Node OR Web Worker │
                       ┌───────────┤ VS Code extension "Core" ├───────────────┐
                       │           │                          │               │
                       │           └──────────────────────────┘               │
                       │                                                      │
         ┌─────────────▼────────────┐                          ┌──────────────▼───────────┐
         │         env: Web         │                          │          env: Web        │
     ┌───┤ "search sidebar" webview │                          │  "search panel" webview  │
     │   │                          │                          │                          │
     │   └──────────────────────────┘                          └──────────────────────────┘
     │
    ┌▼───────────────────────────┐
    │       env: Web Worker      │
    │ Sourcegraph Extension host │
    │                            │
    └────────────────────────────┘

- See below for documentation on state management.
  - One state machine that lives in Core
- See './contract.ts' to see the APIs for the three main components:
  - Core, search sidebar, and search panel.
  - The extension host API is exposed through the search sidebar.
- See './webview/comlink' for documentation on _how_ communication between contexts works.
  - It is _not_ important to understand this layer to add features to the VS Code extension (that's why it exists,
    after all).

## State Management

This extension runs code in 4 (and counting) different execution contexts.
Coordinating state between these contexts is a difficult task. So, instead of managing shared state in each context, we
maintain one state machine in the "Core" context (see above for architecure diagram).
All contexts listen for state updates and emit events on which the state machine may transition.

For example:

- Commands from VS Code extension core
- The first submitted search in a session will cause the state machine to transition from the `search-home` state to
  the `search-results` state.
- This new state will be reflected in both the search sidebar and search panel UIs

We represent a hierarchical state machine in a "flat" manner to reduce code complexity and because our state machine is
simple enough to not necessitate bringing in a library.

```
┌───►home
│
search
│
└───►results
```

- remote-browsing
- idle
- context-invalidated
  becomes:
- [search-home, search-results, remote-browsing, idle, context-invalidated]

Example user flow state transitions:

- User clicks on Sourcegraph logo in VS Code sidebar.
- Extension activates with initial state of `search-home`
- User submits search -> state === `search-results`
- User clicks on a search result, which opens a file -> state === `remote-browsing`
- User copies some code, then focuses an editor for a local file -> state === `idle`

## File Structure

Below is a quick overview of the Sourcegraph extension file structure. It does not include all the files and folders.

```
client/vscode
├── images
├── scripts                       // Command line scripts, for example, script to release and publish the extension
├── src                           // Extension source code
│   └── extension.ts              // Extension entry file
│   └── backend                   // All graphQL queries
│   └── code-intel                // Build the extension host that processes code-intel data
│   └── common                    // Commonly assets that can be shared among different contexts
│   └── commands                  // Build and register commands
│       └── browserActionsNode    // Browser action commands when running as a regular extension where Node.js is available
│       └── browserActionsWeb     // Browser action commands when running as a web extension where Node.js is not available
│   └── file-system               // Build and register the custom file system
│   └── settings                  // Extension settings and configurations
│   └── webview                   // Components to build the search panel and sidebars
│       └── comlink               // Handle communications between contexts
│       └── platform              // Platform context for the webview
│       └── search-panel          // UI for the homepage and search panel
│           └── alias             // Alias files for Web extension. See README file in this directory for details
│       └── sidebars              // UI for all the sidebars
│       └── theming               // Styling the webview using the predefined VS Code themes
│       └── commands.ts           // Commands to build the webview views and panel
├── tests                         // Extension test code
├── .gitignore                    // Ignore build output and node_modules
├── .vscodeignore                 // Ignore build output and node_modules
├── CHANGELOG.md                  // An ordered list of changes and fixes
├── CONTRIBUTING.md               // General guide for developers and contributors
├── package.json                  // Extension manifest
├── README.md                     // General information about the extension
├── tsconfig.json                 // TypeScript configuration
```

## Development

### Build and Run

#### Desktop and Web Version

1. `git clone` the [Sourcegraph repository](https://github.com/sourcegraph/sourcegraph)
1. Install dependencies via `pnpm install` for the Sourcegraph repository
1. Edit files in the `client/vscode` directory
1. To see your changes, open the `Run and Debug` sidebar view in VS Code, and
   select `Launch VS Code Extension` (`Launch VS Code Web Extension` for VS Code Web) from the dropdown menu. (Or, in
   the `client/vscode` directory, run `pnpm run build`.)

### Integration Tests

To run integration tests:

1. In the Sourcegraph repository root, run `pnpm install`
2. In the `client/vscode` directory:
   1. `pnpm build:test` or `pnpm watch:test`
   2. `pnpm test-integration`

## GitPod

The Sourcegraph extension for VS Code also works on GitPod.

#### Desktop Version

To install this extension on GitPod Desktop:

1. Open the Extensions view by clicking on the Extensions icon in the Activity Bar on the side of your workspace
2. Search for `Sourcegraph`
3. Click `install` to install the Sourcegraph extension

#### Web Version

To run and test the web extension on GitPod Web (as well as VS Code and GitHub for the web), you must sideload the
extension from your local machine as suggested in the following steps:

1. `git clone` the [Sourcegraph repository](https://github.com/sourcegraph/sourcegraph)
1. Run `pnpm install && pnpm generate` at the root directory to install dependencies and generate the required schemas
1. Run `pnpm -C client/vscode build` at root to build the Sourcegraph VS Code extension for Web
1. Once the build has been completed, move to the extension’s directory: `cd client/vscode`
1. Start an HTTP server inside the extension’s path to host the extension locally: `pnpx serve --cors -l 8988`
1. In another terminal, generate a publicly-accessible URL from your locally running HTTP server using the localtunnel
   tool: `pnpx localtunnel -p 8988`
   1. A publicly-accessible URL will be generated for you in the output followed by “your url is:”
1. Copy and then open the newly generated URL in a browser and then select “Click to Continue”
1. Open the Command Palette in GitPod Web (a GitPod Workspace using the Open in Browser setting)
1. Select “Developer: Install Web Extension…”
1. Paste the newly generated URL in the input area and select Install
1. The extension is now installed

### Debugging

Please refer to the [How to Contribute](https://github.com/microsoft/vscode/wiki/How-to-Contribute#debugging) guide by
VS Code for debugging tips.

## Questions

If you need guidance or have any questions regarding Sourcegraph or the extension in general, we invite you to connect
with us on the [Sourcegraph Community Slack group](https://about.sourcegraph.com/community).

## Resources

- [Changelog](https://marketplace.visualstudio.com/items/sourcegraph.sourcegraph/changelog)
- [Code of Conduct](https://handbook.sourcegraph.com/company-info-and-process/community/code_of_conduct/)
- [Developing Sourcegraph guide](https://docs.sourcegraph.com/dev)
- [Developing the web clients](https://docs.sourcegraph.com/dev/background-information/web)
- [Feedback / Feature Request](https://github.com/sourcegraph/sourcegraph/discussions/34821)
- [Issue Tracker](https://github.com/sourcegraph/sourcegraph/labels/vscode-extension)
- [Report a bug](https://github.com/sourcegraph/sourcegraph/issues/new?labels=team/integrations,vscode-extension&title=VSCode+Bug+report:+&projects=Integrations%20Project%20Board)
- [Troubleshooting docs](https://docs.sourcegraph.com/admin/how-to/troubleshoot-sg-extension#vs-code-extension)

## Release Process

The release process for the VS Code Extension for Sourcegraph is currently automated.

#### Prerequisite

- Install the [VSCE CLI tool](https://code.visualstudio.com/api/working-with-extensions/publishing-extension#vsce)

#### Release Steps

1. Create a new branch when the main branch is ready for release, or use your current working branch if it is ready for
   release
2. Run `pnpm --filter @sourcegraph/vscode run release:$RELEASE_TYPE` in the root directory
   - $RELEASE_TYPE: major, minor, patch, pre
     - Example: `pnpm --filter @sourcegraph/vscode run release:patch`
   - This command will:
     - Update the package.json file with the next version number
     - Update the changelog format by listing everything under `Unreleased` to the updated version number
     - Make a commit for the release and push to the current branch
3. Open a PR to merge the current branch into main
4. Once the main branch has the updated version number and changelog, run `git push origin main:vsce/release`
   - This will trigger the build pipeline for publishing the extension using the `pnpm release` command
   - Publish release
     to [VS Code Marketplace](https://marketplace.visualstudio.com/items?itemName=sourcegraph.sourcegraph)
   - Publish release to [Open VSX Registry](https://open-vsx.org/extension/sourcegraph/sourcegraph)
   - The extension will be published with the correct package name via
     the [vsce CLI tool](https://code.visualstudio.com/api/working-with-extensions/publishing-extension#vsce)
5. Visit
   the [buildkite page for the vsce/release pipeline](https://buildkite.com/sourcegraph/sourcegraph/builds?branch=vsce%2Frelease)
   to watch the build process

Once the build is completed with no error, you should see the new version being verified for the Sourcegraph extension
in:

- VS Code Marketplace: [Marketplace Publisher Dashboard](https://marketplace.visualstudio.com/manage/publishers)
- Open VSX Registry: [Namespaces Tab in User Settings](https://open-vsx.org/user-settings/namespaces)

> NOTE: It might take up to 10 minutes before the new release is published.
