# Developing code navigation

This guide documents our approach to developing code navigation-related features in our codebase. This includes the code navigation logic included in the Sourcegraph instance as well as the [extensions](https://github.com/sourcegraph/code-intel-extensions) that provide code navigation to the web UI, browser extension, and code host integrations.

Services:

- [precise-code-intel-worker](https://github.com/sourcegraph/sourcegraph/blob/main/cmd/precise-code-intel-worker/README.md)

Code navigation-specific code:

- [lib/codeintel](https://github.com/sourcegraph/sourcegraph/tree/main/lib/codeintel)
- [dev/codeintel-qa](https://github.com/sourcegraph/sourcegraph/tree/main/dev/codeintel-qa)
- [enterprise/internal/codeintel](https://github.com/sourcegraph/sourcegraph/tree/main/enterprise/internal/codeintel)
- [cmd/worker/internal/codeintel](https://github.com/sourcegraph/sourcegraph/tree/main/cmd/worker/internal/codeintel)
- [cmd/frontend/internal/codeintel](https://github.com/sourcegraph/sourcegraph/tree/main/cmd/frontend/internal/codeintel)
- [cmd/frontend/internal/executorqueue/queues/codeintel](https://github.com/sourcegraph/sourcegraph/tree/main/cmd/frontend/internal/executorqueue/queues/codeintel)

Docs:

- [Deployment documentation](deployment.md)
- [How indexes are processed](uploads.md)
- [How precise code navigation queries are resolved](queries.md)
- [How code navigation extensions resolve hovers](extensions.md)
- [How Sourcegraph auto-indexes source code](auto-indexing.md)
