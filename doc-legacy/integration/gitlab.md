# GitLab integration with Sourcegraph

You can use Sourcegraph with [GitLab.com](https://gitlab.com) and GitLab CE/EE.  Read a high-level overview of how Sourcegraph and GitLab work together in our [GitLab solution brief](https://sourcegraph.com/guides/sourcegraph-gitlab-solution-brief.pdf).

Feature | Supported?
------- | ----------
[Repository syncing](../admin/external_service/gitlab.md#repository-syncing) | ✅
[Repository permissions](../admin/external_service/gitlab.md#repository-permissions) | ✅
[User authentication](../admin/external_service/gitlab.md#user-authentication) | ✅
[GitLab UI native integration](#gitlab-ui-native-integration) | ✅
[Browser extension](#browser-extension) | ✅

## Repository syncing

Site admins can [add GitLab repositories to Sourcegraph](../admin/external_service/gitlab.md#repository-syncing).

## Repository permissions

Site admins can [configure Sourcegraph to respect GitLab repository access permissions](../admin/external_service/gitlab.md#repository-permissions).

## User authentication

Site admins can [configure Sourcegraph to allow users to sign in via GitLab](../admin/external_service/gitlab.md#user-authentication).

## GitLab UI native integration

GitLab instances can be configured to show Sourcegraph code navigation natively. See the [GitLab integration docs](../admin/external_service/gitlab.md#native-integration) for how to enable this on your GitLab instance.

![GitLab native integration](img/gitlab-code-intel.gif)

## Browser extension

The [Sourcegraph browser extension](browser_extension.md) supports GitLab. When installed in your web browser, it adds hover tooltips, go-to-definition, find-references, and code search to files and merge requests viewed on GitLab.

1. Install the [Sourcegraph browser extension](browser_extension.md).
1. [Configure the browser extension](browser_extension.md#configuring-the-sourcegraph-instance-to-use) to use your Sourcegraph instance.

- You can also use [`https://sourcegraph.com`](https://sourcegraph.com) for public code from GitLab.com only.

1. To allow the browser extension to work on your GitLab instance:
    - Navigate to any page on GitLab.
    - Right-click the Sourcegraph icon in the browser extension toolbar.
    - Click "Enable Sourcegraph on this domain".
    - Click "Allow" in the permissions request popup.
1. Visit any file or merge request on GitLab. Hover over code or click the "View file" and "View repository" buttons.
