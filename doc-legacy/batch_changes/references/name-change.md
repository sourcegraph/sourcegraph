# Name change

Campaigns has changed name and is called **batch changes** from Sourcegraph 3.26 (released 2021-03-20). The name is changing so that it is more descriptive, and approachable for new users.

## Summary

- Campaigns is called **batch changes** from 3.26.
- We recommend to migrate to the new name as soon as possible to benefit from new functionalities
- There are no breaking changes in release 3.26 to 3.33, and URLs, CLI commands, API endpoints using the previous name (campaigns) will still work. **These will be removed in Sourcegraph 3.34.**

## What changes

- "campaigns" is replaced by "batch changes" in the GUI, documentation, customer facing and internal material
- `<sourcegraph-instance>/campaigns/*` URLs are changed to `<sourcegraph-instance>/batch-changes/*`
- the CLI prompt `src campaigns` is replaced by `src batch`
- campaign specs are now called **batch specs**

## Deprecation plan

From 3.26 to 3.33 (inclusive):

- all `<sourcegraph-instance>/campaigns/*` URLs are deprecated. They will still work, and will be removed in the future.
- the CLI prompt `src campaigns` and its alias `src campaign` are deprecated. They will still work, and will be removed in the future.
- `campaigns` GraphQL API endpoints are deprecated. They are intended to work, so contact us if you are querying the GraphQL API to make sure that we can support you. They will be removed in the future.

From 3.34:

- all `<sourcegraph-instance>/campaigns/*` URLs are removed.
- the CLI prompt `src campaigns` and its alias `src campaign` are removed.
- `/campaigns` GraphQL API endpoints are removed.
