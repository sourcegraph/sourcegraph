# Renovate dependency updades

We use Renovate to automatically create pull requests to update dependendies.
Renovate is [configured](https://github.com/sourcegraph/renovate-config/blob/master/renovate.json#L5) to create these in the first week of each month.

It will ask teams to review these dependency updates, depending on code ownership.

## How to review an update pull request

As a reviewer of a Renovate pull request, you need to make an assessment of:

- The **usefulness** of the update: Updates are considered useful by default, but e.g. a change to the README does not need to be merged.
- The _amount_ and _areas_ of **testing** needed, which is determined by:
  - The **risk** of updating this dependency, which is determined by:
    - The dependency type (`dependency` or `devDependency`): devDependencies generally only break the entire build or app, rarely individual features.
    - The [semver](https://semver.org/) type: major (breaking), minor (feature), patch (bug fix)
    - The actual changes, which are determined by:
      - The **changelog** (usually inlined in the pull request description)
      - The **commits** made between the version (linked from the pull request description)
    - The amount of **automatic testing** we have in this area (either automatically running on the branch in CI, or manually)
    - Whether the breaking changes would be detected by the TypeScript **type checker**
      Note: This only applies to packages that are written in TypeScript like `rxjs`, since otherwise the types may not have been updated yet and may be "lying".
    - Where/how the dependency **is used**: Are we actually using the feature that has a breaking change? Tip: The PR description includes a badge for Sourcegraph search results that can help finding usages of the package.
- The amount of **migration work** (changes to our codebase) needed
- Select packages: Whether the update is updating to a **nightly/prerelease** build.
  These are not intended to be merged, but if the build fails for them because of a bug, it is beneficial for us to file an issue on the package maintainers.

Once you assessed these aspects, you can make an informed decision to:

- Merge the pull request as-is
- Close it
- Make the needed commits on the branch to migrate, then merge
- _Request changes_ on the pull request with a summary of the migration work neeeded, postponing that work but signalling that somebody already reviewed this pull request.
  Everyone should take a look at open Renovate pull requests that have been open for a while regularly.

When in doubt, you can ping your team on the pull request to get more eyes on it.

## Configuration

We heavily customize Renovate to save more time. Possible configurations include:

- Setting different reviewers for certain dependencies
- Grouping certain dependencies
- Auto merging certain low-risk dependencies
- Updating certain dependencies out-of-schedule as well
- Assigning certain labels for easier filtering

If you see an opportunity to improve the configuration, raise a pull request to update the `renovate.json` in the repository or our [configuration shared between repositories](https://github.com/sourcegraph/renovate-config/blob/master/renovate.json).
