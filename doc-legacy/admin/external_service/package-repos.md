# Package repositories

<aside class="experimental">
<p>
<span class="badge badge-experimental">Experimental</span> This feature is experimental and might change or be removed in the future. We've released it as an experimental feature to provide a preview of functionality we're working on.
</p>
</aside>

Sourcegraph package repos can synchronize dependency sources (Rust crates, JVM libraries, Node.js packages, Ruby gems, and more) from public and private artifact hosts (such as NPM, Artifactory etc).

## Enable package repositories

Package repositories can be enabled on a per-ecosystem level in your [site configuration](../config/site_config.md), for example:

```json
{
  // ...
  "experimentalFeatures": {
    "jvmPackages": "enabled",
    "goPackagse": "enabled",
    "npmPackages": "enabled",
    "pythonPackagse": "disabled",
    "rubyPackages": "disabled",
    "rustPacakges": "enabled"
  }
  // ...
}
```

## Repository syncing

There are generally two ways of syncing package repositories to the Sourcegraph instance.

1. **Indexing** (recommended): package repositories can be added to the Sourcegraph instance when they are referenced in [code graph data uploads](../../code_navigation/explanations/uploads.md). Code graph indexers can derive package repository references during indexing, which will be used on upload to sync them to your instance. An external service for the given ecosystem must be created in order for these referenced package repositories to be synchronized.
2. **Code host configuration**: each package repository external service supports manually defining versions of packages to sync. See the page specific to the ecosystem you wish to configure. This method can be useful to verify that the credentials are picked up correctly without having to upload an index, as we'll as to poke holes in the filters (detailed below) if necessary.

## Filters

Package repository filters allow you to limit the package repositories and versions that are synced to the Sourcegraph instance. Using glob patterns, you can block or allow certain package repositories or versions on a per-ecosystem level.

There are two kinds of filters:

1. **Package name filters**: these filters match a glob pattern against a package repository's name, and will apply to all versions of matching package repositories.
2. **Package version filters**: these filters match a glob pattern against versions for a specific package repository. It cannot apply to multiple package repositories.

Filters can also have one of two behaviours:

1. **Block filters**: package repositories or versions matching these filters won't be synced.
2. **Allow filters**: only package repositories or versions matching these filters (that don't match a block filter) will be synced.

Having no configured _allow_ filters implicitly allows everything. _Block_ filters have priority over _allow_ filters (a blocked package or version will not be synced even if it is explicitly allowed).

Package repository filter updates may require a few minutes to propagate through the system. The blocked status of a package repository will be updated in the UI and the relevant external service may add or remove repositories after syncing (visible in the site-admin page for the external service).
