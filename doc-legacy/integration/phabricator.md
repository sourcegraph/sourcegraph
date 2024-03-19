# Phabricator integration with Sourcegraph

> ⚠️ NOTE: Sourcegraph support of Phabricator is limited, and not expected to evolve due to the [announced](https://admin.phacility.com/phame/post/view/11/phacility_is_winding_down_operations/) cease of support for Phabricator.

This Phabricator integration does not support listing and mirroring Phabricator repositories (as it does for repositories on other code hosts). It is intended for use when your repositories are hosted somewhere else (such as GitHub), and Phabricator mirrors repositories from that code host. If your repositories are hosted on Phabricator, you must follow the steps in "[Other Git repository hosts](../admin/external_service/other.md)" to add the repositories so that they are mirrored to Sourcegraph in addition to the steps outlined here to power the integration. Sourcegraph does not currently support using the repository permissions you've set in Phabricator for repositories hosted on Phabricator.

Feature | Supported?
------- | ----------
[Repository syncing and mirroring](../admin/external_service/phabricator.md#repository-linking-and-syncing) | ❌
[Repository association](../admin/external_service/phabricator.md#repository-linking-and-syncing) | ✅
[Repository permission syncing](../admin/permissions/syncing.md) | ❌
[User authentication](../admin/auth/index.md) | ❌
[Browser extension](#browser-extension) | ✅
[Native extension](../admin/external_service/phabricator.md#native-extension) | ✅

## Repository association

Site admins can [associate Phabricator repositories with Sourcegraph](../admin/external_service/phabricator.md#repository-syncing-and-linking).

## Browser extension

The [Sourcegraph browser extension](browser_extension.md) supports Phabricator. When installed in your web browser, it adds hover tooltips, go-to-definition, find-references, and code search to files and diffs viewed on Phabricator.

1.  Install the [Sourcegraph browser extension](browser_extension.md).
1.  [Configure the browser extension](browser_extension.md#configuring-the-sourcegraph-instance-to-use) to use your Sourcegraph instance.
1.  To allow the browser extension to work on your Phabricator instance:
    - Navigate to any page on Phabricator.
    - Right-click the Sourcegraph icon in the browser extension toolbar.
    - Click "Enable Sourcegraph on this domain".
    - Click "Allow" in the permissions request popup.
1.  Visit any file or diff on Phabricator. Hover over code or click the "View file" and "View repository" buttons.

> NOTE: Site admins can also install the [native Phabricator extension](../admin/external_service/phabricator.md#native-extension) to avoid needing each user to install the browser extension.
