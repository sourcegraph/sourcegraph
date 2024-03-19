# Troubleshooting Cody

<p class="subtitle">Learn about common reasons for errors that you might run into when using Cody and how to troubleshoot them.</p>

If you encounter errors or bugs while using Cody, try applying these troubleshooting methods to understand and configure the issue better. If the problem persists, you can report Cody bugs using the [issue tracker](https://github.com/sourcegraph/cody/issues) or ask in the [Discord](https://discord.gg/s2qDtYGnAE) channel.

## VS Code extension

### Cody is not responding in chat

If you're experiencing issues with Cody not responding in chat, follow these steps:

- Ensure you have the latest version of the [Cody VS Code extension](https://marketplace.visualstudio.com/items?itemName=sourcegraph.cody-ai). Use the VS Code command `Extensions: Check for Extension Updates` to verify
- Check the VS Code error console for relevant error messages. To open it, run the VS Code command `Developer: Toggle Developer Tools` and then look in the `Console` for relevant messages

### Errors trying to install Cody on macOS

If you encounter the following errors:

```bash
Command 'Cody: Set Access Token' resulted in an error

Command 'cody.set-access-token' not found
```

Follow these steps to resolve the issue:

- Close your VS Code editor
- Open your Keychain Access app
- Search for `cody`
- Delete the `vscodesourcegraph.cody-ai` entry in the system keychain on the left
- Reopen the VS Code editor. This should resolve the error

![Opening up Keychain Access](https://storage.googleapis.com/sourcegraph-assets/blog/cody-docs-troubleshooting-keychain-access.png)

### Codebase is `Not Indexed`

If you are logged into Sourcegraph.com, only public open source repositories listed [here](embedded-repos.md) are indexed. To have your open source repository added to the public index, join the [Sourcegraph Discord](https://discord.gg/8wJF5EdAyA) and ask in the `#embeddings-indexing` channel.

If you’re connected to a Sourcegraph Enterprise instance, ask your site admin to [Configure Code Graph Context](core-concepts/code-graph.md) for your Sourcegraph instance. Then, [enable Cody](overview/enable-cody-enterprise.md) for your account.

If you're connected to the Cody app:

- Trigger indexing for a repository by adding it to your app under **Settings > Local repositories**
- Navigate to **Settings > Advanced settings > Embeddings jobs** in the app and schedule embedding
- If your repo lacks a `git remote` or still shows as `Not Indexed`, set `Cody: Codebase` to the repository name under **Settings > Local** repositories in the Cody App

If you've completed the above and are still getting the `NOT INDEXED` error, try updating the `Cody: Codebase` (`cody.codebase`) setting in VS Code to the repository name listed on your Sourcegraph instance.

For more information, see [Generate Index to Enable Codebase-Aware Answers](core-concepts/embeddings/embedding-index.md).

### Signin fails on each VS Code restart

If you find yourself being automatically signed out of Cody every time you restart VS Code, and suspect it's due to keychain authentication issues, you can address this by following the steps provided in the official VS Code documentation on [troubleshooting keychain issues](https://code.visualstudio.com/docs/editor/settings-sync#_troubleshooting-keychain-issues). These guidelines should help you troubleshoot and resolve any keychain-related authentication issues, ensuring a seamless experience with Cody on VS Code.

### Rate limits

On the free plan, Cody provides **500 autocomplete suggestions** and **20 chat and command invokations** per user per month.

On the Pro and Enterprise plans, there are much higher limits that are used to keep our services operational. These limits reset within a day.

### Error logging in VS Code on Linux

If you encounter difficulties logging in to Cody on Linux using your Sourcegraph instance URL, along with a valid access token, and notice that the sign-in process in VS Code hangs, it might be related to underlying networking rules concerning SSL certificates.

To address this, follow these steps:

- Close your VS Code editor
- In your terminal, type and run the following command: `echo "export NODE_TLS_REJECT_UNAUTHORIZED=0">> ~/.bashrc`
- Restart VS Code and try the sign in process again

### Error exceeding `localStorage` quota

When using Cody chat, you may come across this error:

```bash
Failed to execute 'setItem' on 'Storage': Setting the value of 'user-history:$user_id' exceeded the quota.
```

This error indicates that the chat history size surpasses the capacity of your browser's local storage. Cody stores comprehensive context data with each chat message, contributing to this limitation.

To fix this, navigate to https://sourcegraph.example.com/cody/chat and click `Clear Chat History` if your instance is on v5.2.3+. For older versions, clear your browsing data or browser history.
