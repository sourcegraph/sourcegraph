# Bitbucket Cloud integration with Sourcegraph

You can use Sourcegraph with Git repositories hosted on [Bitbucket Cloud](https://bitbucket.org).

Feature | Supported?
------- | ----------
[Repository syncing](../admin/external_service/bitbucket_cloud.md) | ✅
[Repository permissions](../admin/external_service/bitbucket_cloud.md#repository-permissions) | ✅
Browser extension | ✅
Native extension | ❌ Not supported on Bitbucket.org

## Repository syncing

Site admins can [add Bitbucket Cloud repositories to Sourcegraph](../admin/external_service/bitbucket_cloud.md).

## User authorization

Site admins can [add Bitbucket Cloud as an authentication provider to Sourcegraph](../admin/auth.md#bitbucket-cloud).
This will allow users to sign into Sourcegraph using their Bitbucket Cloud accounts. Site admins can then also [enable repository permissions](../admin/external_service/bitbucket_cloud.md#repository-permissions) on their Bitbucket Cloud code host connections.
