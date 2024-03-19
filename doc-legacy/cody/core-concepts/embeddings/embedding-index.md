# Generate an Embeddings Index

<p class="subtitle">This guide will explain how to generate an embeddings index that enables codebase-aware answers for Cody.</p>

Codebase-aware answers utilize Code Graph Context to empower Cody with a deeper understanding of the code within selected codebases.

By following the steps outlined in this guide, you can configure the necessary prerequisites, enabling Cody to provide more precise and contextually relevant answers to your coding queries, all tailored to the specific codebase you are currently working in.

## Generate embeddings index

Embedding your codebase is recommended to enhance Cody's understanding of existing code. The process varies and depends on whether you are using a Sourcegraph Enterprise or Sourcegraph.com instance:

### Sourcegraph Enterprise

To embed your codebase and enable codebase-aware answers for Cody, you need to perform the following steps:

- [Enable Cody](./../../overview/enable-cody-enterprise.md#step-1-enable-cody-on-your-sourcegraph-instance) for your Sourcegraph Enterprise instance
- Set up the [Code Graph Context](./../code-graph.md) from your **Site admin**
- Embed your codebase by either [scheduling a one-off embeddings job](./configure-embeddings.md#schedule-embeddings-jobs) or [creating an embeddings policy](./configure-embeddings.md#policies) to keep your index up-to-date automatically

### Sourcegraph.com

For Sourcegraph.com users, view the current [list of repositories with code graph context available](./../../embedded-repos.md). If you'd like to add more repositories to this list, reach out to a Sourcegraph team member in the [Sourcegraph Discord](https://discord.gg/8wJF5EdAyA) in the `#cody-embeddings` channel.

> NOTE: Sourcegraph.com does not support connections to private repositories

## Enable codebase-aware answers

When connected to the correct codebase, Cody provides accurate and relevant answers to your coding questions, taking into account the context of the codebase you are currently working in.

Cody attempts to connect to the appropriate codebase automatically using Git, eliminating the need to manually configure the `Cody: Codebase` (`cody.codebase`) option.

You only need to configure the `Cody: Codebase` (`cody.codebase`) setting if you see `NOT INDEXED` below the chat window.

## Cody VS Code extension settings

Cody tries to connect to the correct codebase using Git at start-up, so the manual configuration of `cody.codebase` is not required.

To enable Cody to set the codebase config using `git remote get-url origin`, ensure that the `Cody: Codebase` (`cody.codebase`) configuration field is unset in your VS Code User and Workspace settings, and remote workspace and folder-level settings if applicable.

If this attempt fails and you see `Embeddings Not Found` below your Cody chatbox, you can manually specify the correct codebase for Cody using the `Cody: Codebase` (`cody.codebase`) configuration option.

### Manual Configuration

To manually configure the `codebase` setting for Cody VS Code extension via the [extension settings](https://code.visualstudio.com/docs/getstarted/settings#_extension-settings), follow these steps:

- Open the VS Code workspace settings by clicking:  `Code` > `Settings` > `Settings` (for macOS) or `File` > `Preferences (Settings)` ( for Windows and Linux)
- Switch to the Workspace settings tab
- Type `Cody: Codebase` in the search bar
- Next, enter the repository name as listed in your Sourcegraph instance in the `Cody: Codebase` field

For example, the name for the [Sourcegraph repository on Sourcegraph.com](https://sourcegraph.com/github.com/sourcegraph/sourcegraph) is `github.com/sourcegraph/sourcegraph`, so enter it into the setting field without the https protocol as follows:

```
github.com/sourcegraph/sourcegraph
```

### `settings.json`

Alternatively, you can configure the `codebase` manually via [`settings.json`](https://code.visualstudio.com/docs/getstarted/settings#_settingsjson) file using the `cody.codebase` configuration contribution point:

```json
{
  "cody.serverEndpoint": "https://sourcegraph.com",
  "cody.codebase": "github.com/sourcegraph/sourcegraph"
}
```
