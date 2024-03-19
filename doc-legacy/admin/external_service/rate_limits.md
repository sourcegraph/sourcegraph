# Rate limits

Sourcegraph respects and enforces various rate limits to ensure optimal and reliable performance. Sourcegraph has two types of rate limits:

- [External rate limits](#external-rate-limits)
- [Internal rate limits](#internal-rate-limits)

For other ways to control repo update frequency, see [Repository update frequency](../repo/update_frequency.md).

## External rate limits

External rate limits refer to the rate limits set by external services integrated with Sourcegraph, such as code hosts (GitHub, GitLab, Bitbucket, etc.). Sourcegraph always adheres to and never exceeds the external rate limits of integrated services.

Many code hosts give continuous feedback on rate limiting. Sourcegraph tracks this feedback, if available, and delays automatic background requests (permissions syncing, repo discovery, etc.) if rate limits are encountered.

No configuration is necessary to enable external rate limit monitoring.

> NOTE: When configuring code host connections on Sourcegraph, always include a `token` even if only accessing public repositories, as code hosts impose severe rate limits for unauthenticated requests (see [GitHub](https://docs.github.com/en/rest/overview/resources-in-the-rest-api?apiVersion=2022-11-28#rate-limits-for-requests-from-personal-accounts) for example).

If working with a self-hosted code host, consult the code host documentation to learn how to configure rate limits on your code host.

Sourcegraph monitors external rate limits for the following code hosts:
- [GitHub](../external_service/github.md#rate-limits)
- [GitLab](../external_service/gitlab.md#rate-limits)
- [Bitbucket Cloud](../external_service/bitbucket_cloud.md#rate-limits)
- [Azure DevOps](../external_service/azuredevops.md#rate-limits)

## Internal rate limits

Internal rate limits refer to self-imposed rate limits within Sourcegraph. While Sourcegraph adheres to external rate limits, sometimes more control is necessary, or a code host might not have rate limit monitoring available or configured. In these cases, internal rate limits can be configured.

A [global default internal rate limit](../config/site_config.md#defaultRateLimit) can be configured in the [site configuration](../config/site_config.md). This limit applies to all code host connections that don't have a specific rate limit configured.

> NOTE: This is the default rate limit _per code host connection_. It is not the total rate limit of all the code host connections.

To configure internal rate limits for a specific code host connection:
- Within the code connection configuration, add the following:
```json
{
  // ...
  "rateLimit": {
    "enabled": true,
    "requestsPerHour": 5000
  }
}
```

Requests to the configured code host will be staggered as to not exceed `"requestsPerHour"`. This will override the default rate limit (if configured).

> NOTE: Configuring a rate limit will impact Sourcegraph's ability to stay up to date with repository changes and user permissions. To ensure that Sourcegraph stays up to date, consider configuring [webhooks](../config/webhooks/incoming.md).

- For Sourcegraph <=3.38, if rate limiting is configured more than once for the same code host instance, the most restrictive limit will be used.
- For Sourcegraph >=3.39, rate limiting should be enabled and configured for each individual code host connection.

To see the status of configured internal rate limits, visit **Site admin > Instrumentation > repo-updater > Rate Limiter State**. This page lists internal rate limits by code host, for example:

```json
{
  "extsvc:github:1": {
    "Burst": 10,
    "Limit": 2,
    "Infinite": false
  }
}
```

This entry tells us that a rate limit is configured for a GitHub external service. `Burst` means that a maximum of 10 requests can be made in quick succession. After that, requests will be limited to 2 (the `Limit` value) per second. If `Infinite` is `true`, no internal rate limiting is applied for this connection.

Sourcegraph supports internal rate limit configuration for the following connections:
- [GitHub](./github.md#rateLimit)
- [GitLab](./gitlab.md#rateLimit)
- [Bitbucket Cloud](./bitbucket_cloud.md#rateLimit)
- [Bitbucket Server](./bitbucket_server.md#rateLimit)
- [Perforce](../repo/perforce.md#rateLimit)
- [Go Modules](./go.md#rateLimit)
- [JVM Packages](./jvm.md#rateLimit)
- [NPM Packages](./npm.md#rateLimit)
- [Python Packages](./python.md#rateLimit)
- [Ruby Packages](./ruby.md#rateLimit)
- [Rust Packages](./rust.md#rateLimit)
