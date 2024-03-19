# How to update pnpm

## Background 

Sourcegraph uses [pnpm](https://pnpm.io/) to handle client-side dependencies and we build our code with [Bazel](../background-information/bazel.md). We use [aspect.dev](https://www.aspect.dev/)' [aspect_rules_js](https://github.com/aspect-build/rules_js) to integrate it with Bazel. 

In practice, this means that when we're updating pnpm, we have to make sure that our Bazel building code is refecting it correctly. 

### When to update? 

Unless we're facing a bug preventing to build code, we should aim to update pnpm as early as possible in the release cycle, so ideally just after a release has been made. This gives everyone enough breathing room to catch and fix possible issues that were introduced in the building system. 

### Who can update? 

`pnpm` belongs to the build system, which while maintained and shephered by the [DevX](https://handbook.sourcegraph.com/departments/engineering/teams/dev-experience/) team, is a shared ownership. Everyone uses the build system daily, so it's only logical that everyone can improve it as well. So go ahead, and open a PR, be sure to tag the DevX team as reviewers for a swift review. 

## Performing the update 

First, let's go through the following checklist to ensure that we can update: 

- [ ] Pick a version that is currently supported by `aspect_rules_js`.
  - Check which version of `rules_js` we're running with this [search query](https://sourcegraph.sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+lang:Starlark+http_archive%28...name+%3D+%22aspect_rules_js%22%2C...%29&patternType=structural&sm=0&groupBy=path).
  - Review this [file](https://github.com/aspect-build/rules_js/blob/main/npm/private/versions.bzl#L6) to see which versions are available. 
  - Also check if the versions we're aiming for is avalaible for the release of `aspect_rules_js` we're using. If not, it means we'll have to update it along the way. 
- [ ] Read the changelog for the new `pnpm` version.
- [ ] Ensure there are no breaking changes introduced since the version that we're currently using.
  - If there are breaking changes, consider reaching out on [#ask-dev-experience](https://sourcegraph.slack.com/archives/C04MYFW01NV) to discuss them. 

Now that we have established that we can move to this version, we can proceed: 

- Updating the local environment tooling 
  - Update [`.tools-version`](https://sourcegraph.sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/.tool-versions) 
    - Change the defined `pnpm` version. 
  - Run `asdf install pnpm` to install the newly defined version.
  - Run `pnpm -v` to ensure it's correctly installed.
- Updating [`package.json`](https://sourcegraph.sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+file:%5Epackage.json%24+%22engines%22:+%7B...%7D&patternType=structural&sm=1&groupBy=path), setting the `"pnpm"` field to the new version.
- Updating the lockfile as it may needs to change with that new version 
  - Run `pnpm i --lockfile-only`
- If during our checklist just above we found out that we need to also update `aspect_rules_js`
  - Grab from the _WORSKPACE_ section the `http_archive` snippet and replace it in the [`WORKSPACE` file](https://sourcegraph.sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/WORKSPACE).
- Build a client Bazel target to ensure it works: 
  - `bazel build //client/web/dev:dev` should build correctly. 

At this point, we have updated the repository, but there are a few places that also need to have the correct `pnpm` version set for miscellaneous scrits to work: 

1. Search [for the previous `pnpm` version](https://sourcegraph.sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+8.1.0&patternType=standard&sm=1&groupBy=path) to make sure we're not missing any. 
  - Usually those are GitHub worflows, where we also need to hardcode that version, but better search and review than blindly upating it.
  - Update them accordingly (the above search query finds out matches that are not all about `pnpm`, be mindful when going through the results). 

Now we're good to go! Let's push all these changes in a PR and tag the DevX team as reviewers. 

> 💡 It's also a good practice to announce the update on [#ask-dev-experience](https://sourcegraph.slack.com/archives/C04MYFW01NV) so everyone knows about the update, making it easier to revert it if another teammate build is broken by the update. 
