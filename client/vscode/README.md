# Sourcegraph for Visual Studio Code

[![vs marketplace](https://img.shields.io/vscode-marketplace/v/sourcegraph.sourcegraph.svg?label=vs%20marketplace)](https://marketplace.visualstudio.com/items?itemName=sourcegraph.sourcegraph) [![downloads](https://img.shields.io/vscode-marketplace/d/sourcegraph.sourcegraph.svg)](https://marketplace.visualstudio.com/items?itemName=sourcegraph.sourcegraph)

![Search Gif](https://storage.googleapis.com/sourcegraph-assets/VS%20Marketplace/tableContainer2.gif)

Sourcegraph’s code search allows you to find & fix things fast across all your code.

Sourcegraph for VS Code allows you to search millions of open source repositories right from your VS Code IDE—for free.
You can learn from helpful code examples, search best practices, and re-use code from millions of repositories across
the open source universe.

Sourcegraph’s Code Intelligence feature provides fast, cross-repository navigation with “Go to definition” and “Find
references” features, allowing you to understand new code quickly and find answers in your code across codebases of any
size.

You can read more about Sourcegraph on our [website](https://about.sourcegraph.com/).

Not the extension you're looking for? Download
the [Cody extension](https://marketplace.visualstudio.com/items?itemName=sourcegraph.cody-ai) for code AI assistance.

## Installation

### From the Visual Studio Marketplace:

1. Install Sourcegraph from
   the [Visual Studio Marketplace](https://marketplace.visualstudio.com/items?itemName=sourcegraph.sourcegraph).
2. Launch VS Code, and click on the Sourcegraph (Wildcard) icon in the VS Code Activity Bar to open the Sourcegraph
   extension. Alternatively, you can launch the extension by pressing <kbd>Cmd</kbd>+<kbd>Shift</kbd>+<kbd>P</kbd>
   or <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>P</kbd> and searching for “Sourcegraph: Open search tab.”

### From within VS Code:

1. Open the extensions tab on the left side of VS Code (<kbd>Cmd</kbd>+<kbd>Shift</kbd>+<kbd>X</kbd> or <kbd>
   Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>X</kbd>).
2. Search for `Sourcegraph` -> `Install` and `Reload`.

### From Gitpod VS Code Workspaces:

1. Open the `Extensions view` by clicking on the Extensions icon in
   the [Activity Bar](https://code.visualstudio.com/api/ux-guidelines/overview#activity-bar) on the side of your Gitpod
   VS Code workspace
2. Search for [Sourcegraph](https://open-vsx.org/extension/sourcegraph/sourcegraph) -> `Install` and `Reload`.

## Using the Sourcegraph extension

To get started and open the Sourcegraph extension, simply click the Sourcegraph (Wildcard) icon in the VS Code Activity
Bar.

Sourcegraph functions like any search engine; simply type in your search query, and Sourcegraph will populate search
results.

Learn more about Sourcegraph search in our [docs](https://docs.sourcegraph.com/code_search).

For example, you can search for "auth provider" in a Go repository with a search like this one:

```
repo:sourcegraph/sourcegraph lang:go auth provider
```

![Lang search gif](https://storage.googleapis.com/sourcegraph-assets/VS%20Marketplace/sourcegraph_search.gif)

## Adding and searching your own code

### Creating an account

In addition to searching open source code, you can create a Sourcegraph account to search your own private and public
repositories. You can create an account and sync your repositories with the following steps:

1. Click the `Create an account` button in the sidebar of the Sourcegraph extension. You will be directed to
   sourcegraph.com in your browser.
2. Follow the instructions to create and configure your Sourcegraph instance

### Connecting to your Sourcegraph instance

1. In Sourcegraph, in your account settings, navigate to `Access tokens`, then click `Generate new token`.
2. Once you have generated a token, navigate to your VS Code Settings, then navigate to "Extension settings".
3. Navigate to `Code preferences`, then click `Settings`.
4. Search for `Sourcegraph`, and enter the newly generated access token as well as your Sourcegraph instance URL.
5. Add custom headers using the `sourcegraph.requestHeaders` setting (added in v2.0.9) if a specific header is required
   to make connection to your private instance.

### Adding it to your workspace's recommended extensions

1. Create a `.vscode` folder (if not already present) in the root of your repository
2. Inside that folder, create a new file named `extensions.json` (if it doesn't already exist) with the following
   structure.

```
{
	"recommendations": ["sourcegraph.sourcegraph"]
}
```

3. If the file does exist, append `"sourcegraph.sourcegraph"` to the `"recommendations"` array.
4. Push the changes to your repository for your other colleagues to share.

Alternatively you can use the `Extensions: Configure Recommended Extensions (Workspace Folder)` command from within VS
Code.

## Keyboard Shortcuts:

| Description                                  | Mac                                          | Linux / Windows                               |
| -------------------------------------------- | -------------------------------------------- | --------------------------------------------- |
| Open Sourcegraph Search Tab/Search Selection | <kbd>Cmd</kbd>+<kbd>Shift</kbd>+<kbd>8</kbd> | <kbd>Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>8</kbd> |
| Open File in Sourcegraph Cloud               | <kbd>Option</kbd>+<kbd>A</kbd>               | <kbd>Alt</kbd>+<kbd>A</kbd>                   |
| Search Selected Text in Sourcegraph Cloud    | <kbd>Option</kbd>+<kbd>S</kbd>               | <kbd>Alt</kbd>+<kbd>S</kbd>                   |

## Extension Settings

This extension contributes the following settings:

| Setting                           | Description                                                                                                                                                                                                                     | Example                                                      |
| --------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------ |
| sourcegraph.url                   | Specify your on-premises Sourcegraph instance here, if applicable. The extension is connected to Sourcegraph Cloud by default.                                                                                                  | "https://your-sourcegraph.com"                               |
| sourcegraph.accessToken           | [Depreciated after 2.2.12] The access token to query the Sourcegraph API. Required to use this extension with private instances. Create a new access token at `${SOURCEGRAPH_URL}/users/<sourcegraph-username>/settings/tokens` | null                                                         |
| sourcegraph.remoteUrlReplacements | Object, where each `key` is replaced by `value` in the remote url.                                                                                                                                                              | {"github": "gitlab", "master": "main"}                       |
| sourcegraph.defaultBranch         | String to set the name of the default branch. Always open files in the default branch.                                                                                                                                          | "master"                                                     |
| sourcegraph.requestHeaders        | Takes object, where each value pair will be added to the request headers made to your instance.                                                                                                                                 | {"Cache-Control": "no-cache", "Proxy-Authenticate": "Basic"} |
| sourcegraph.basePath              | The file path on the machine to the folder that is expected to contain all repositories. We will try to open search results using the basePath.                                                                                 | "/Users/USERNAME/Documents/"                                 |
| sourcegraph.proxyProtocol         | The protocol to use when proxying requests to the Sourcegraph instance.                                                                                                                                                         | "http", "https"                                              |
| sourcegraph.proxyHost             | The host to use when proxying requests to the Sourcegraph instance. It shouldn't include a protocol (like "http://") or a port (like ":7080"). When this is set, port must be set as well.                                      | "localhost"                                                  |
| sourcegraph.proxyPort             | The port to use when proxying requests to the Sourcegraph instance. When this is set, host must be set as well.                                                                                                                 | 80, 443, 7080, 9090                                          |
| sourcegraph.proxyPath             | The full path to a file when proxying requests to the Sourcegraph instance via a UNIX socket.                                                                                                                                   | "/home/user/path/unix.socket"                                |

## Questions & Feedback

Feedback and feature requests can be submitted to
our [VS Code Extension Feedback Discussion Board](https://github.com/sourcegraph/sourcegraph/discussions/34821) on
GitHub.

## Uninstallation

1. Open the extensions tab on the left side of VS Code (<kbd>Cmd</kbd>+<kbd>Shift</kbd>+<kbd>X</kbd> or <kbd>
   Ctrl</kbd>+<kbd>Shift</kbd>+<kbd>X</kbd>).
2. Search for `Sourcegraph` -> Gear icon -> `Uninstall` and `Reload`.

## Changelog

Click [here](https://marketplace.visualstudio.com/items/sourcegraph.sourcegraph/changelog) to check the full changelog.

VS Code will auto-update extensions to the highest version available. Even if you have opted into a pre-release version,
you will be updated to the released version when a higher version is released.

The Sourcegraph extension uses major.EVEN_NUMBER.patch (eg. 2.0.1) for release versions and major.ODD_NUMBER.patch (eg.
2.1.1) for pre-release versions.```

## Development

Please see the [CONTRIBUTING](./CONTRIBUTING.md) document if you are interested in contributing directly to our code
base.
Hello World
