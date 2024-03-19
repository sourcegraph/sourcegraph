# Ruby dependencies integration with Sourcegraph

You can use Sourcegraph with Ruby dependencies from any RubyGems repository, including rubygems.org or an internal Artifactory.

This integration makes it possible to search and navigate through the source code of published Ruby library (for example, [`shopify_api@12.0.0`](https://sourcegraph.com/rubygems/shopify_api@v12.0.0)).

Feature | Supported?
------- | ----------
[Repository syncing](#repository-syncing) | ✅
[Repository permissions](#repository-syncing) | ❌
[Multiple RubyGems repositories code hosts](#multiple-ruby-dependencies-code-hosts) | ❌

## Setup

See the "[Ruby dependencies](../admin/external_service/ruby.md)" documentation.

## Repository syncing

Site admins can [add Ruby dependencies to Sourcegraph](../admin/external_service/ruby.md#repository-syncing).

## Repository permissions

⚠ Ruby dependencies are visible by all users of the Sourcegraph instance.

## Multiple Ruby dependencies code hosts

⚠️ It's only possible to create one Ruby dependency code host for each Sourcegraph instance.

See the issue [sourcegraph#32461](https://github.com/sourcegraph/sourcegraph/issues/32461) for more details about this limitation.
