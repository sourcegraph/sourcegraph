# npm dependencies integration with Sourcegraph

You can use Sourcegraph with npm packages from any npm registry, including open source code from npmjs.com or a private registry such as Verdaccio.
This integration makes it possible to search and navigate through the source code of published JavaScript or TypeScript packages (for example, [`@types/gzip-js@0.3.3`](https://sourcegraph.com/npm/types/gzip-js@v0.3.3/-/blob/index.d.ts)).

Feature | Supported?
------- | ----------
[Repository syncing](#repository-syncing) | ✅
[Repository permissions](#repository-syncing) | ❌
[Multiple npm dependencies code hosts](#multiple-npm-dependencies-code-hosts) | ❌

## Setup

See the "[npm dependencies](../admin/external_service/npm.md)" documentation.

## Repository syncing

Site admins can [add npm packages to Sourcegraph](../admin/external_service/npm.md#repository-syncing).

## Repository permissions

⚠️ npm dependency repositories are visible by all users of the Sourcegraph instance.

## Multiple npm dependencies code hosts

⚠️ It's only possible to create one npm dependency code host for each Sourcegraph instance.
See the issue [sourcegraph#32499](https://github.com/sourcegraph/sourcegraph/issues/32499) for more details about this limitation. In most situations, it's possible to work around this limitation by configurating a single private npm registry to proxy multiple underlying registries.
