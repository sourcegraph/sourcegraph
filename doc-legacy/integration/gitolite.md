---
ignoreDisconnectedPageCheck: true
---

# Gitolite integration with Sourcegraph

You can use Sourcegraph with [Gitolite](http://gitolite.com/).

Feature | Supported?
------- | ----------
[Repository syncing](../admin/external_service/gitolite.md#repository-syncing) | ✅
[Browser extension](#browser-extension) | ❌

## Repository syncing

Site admins can [sync Gitolite repositories to Sourcegraph](../admin/external_service/gitolite.md#repository-syncing).

## Browser extension

The [Sourcegraph browser extension](browser_extension.md) does not support Gitolite because Gitolite has no web interface. If your Gitolite repositories are also available on another code host (such as GitHub or Phabricator), you can use the browser extension with those services.
