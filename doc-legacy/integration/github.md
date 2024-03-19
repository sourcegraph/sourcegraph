# GitHub integration with Sourcegraph

You can use Sourcegraph with [GitHub.com](https://github.com) and [GitHub Enterprise](https://enterprise.github.com).

Feature | Supported?
------- | ----------
[Repository syncing](../admin/external_service/github.md#selecting-repositories-to-sync) | ✅
[Repository permissions](../admin/external_service/github.md#repository-permissions) | ✅
[User authentication](../admin/external_service/github.md#user-authentication) | ✅
[Browser extension](#browser-extension) | ✅

## Repository syncing

Site admins can [add GitHub repositories to Sourcegraph](../admin/external_service/github.md#selecting-repositories-to-sync).

## Repository permissions

Site admins can [configure Sourcegraph to respect GitHub repository access permissions](../admin/external_service/github.md#repository-permissions).

## User authentication

Site admins can [configure Sourcegraph to allow users to sign in via GitHub](../admin/external_service/github.md#user-authentication).

## Browser extension

The [Sourcegraph browser extension](browser_extension.md) supports GitHub. When installed in your web browser, it adds hover tooltips, go-to-definition, find-references, and code search to files and pull requests viewed on GitHub and GitHub Enterprise.

1.  Install the [Sourcegraph browser extension](browser_extension.md).
1.  [Configure the browser extension](browser_extension.md#configuring-the-sourcegraph-instance-to-use) to use your Sourcegraph instance.

- You can also use [`https://sourcegraph.com`](https://sourcegraph.com) for public code only.

1.  GitHub Enterprise only: Click the Sourcegraph icon in the browser toolbar to open the settings page. If a permissions notice is displayed, click **Grant permissions** to allow the browser extension to work on your GitHub Enterprise instance.
1.  Visit any file or pull request on GitHub. Hover over code or click the "View file" and "View repository" buttons.
