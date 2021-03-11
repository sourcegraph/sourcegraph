# Name change

Campaigns has changed name and is  called **batch changes** from Sourcegraph 3.26 (released 2021-03-20). The name is changing so that it is more descriptive, and approachable for new users.

## Summary

- Campaigns is called **batch changes** from 3.26.
- There are no breaking changes in this release, and URLs, CLI commands, API endpoints using the previous name (campaigns) will still work
- Deprecation of URLs, CLI commands, API endpoints is planned for the next major release (yet unplanned), or earlier depending on usage, with advance notice to our customers at least 2 months in advance
- We recommend to migrate to the new name as soon as possible to benefit from new functionalities

## What changes

- "campaigns" is replaced by "batch changes" in the GUI, documentation, customer facing and internal material
- `<sourcegraph-instance>/campaigns/*` URLs are changed to `<sourcegraph-instance>/batch-changes/*`
- the CLI prompt `src campaigns` is replaced by `src batch`
- campaign specs are now called **batch specs**

## Deprecation plan

From 3.26:

- all `<sourcegraph-instance>/campaigns/*` URLs are deprecated. They will still work, and will be removed in the next major release.
- the CLI prompt `src campaigns` and its alias `src campaign` are deprecated. They will still work, and will be removed in the next major release.
- `/campaigns` GraphQL API endpoints are deprecated. They are intended to work, so contact us if you are querying the GraphQL API to make sure that we can support you.
