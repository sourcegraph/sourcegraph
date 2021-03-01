# Pings

Sourcegraph periodically sends a ping to Sourcegraph.com to help our product and customer teams. It sends only the high-level data below. It never sends code, repository names, usernames, or any other specific data. To learn more, go to the **Site admin > Pings** page on your instance. (The URL is `https://sourcegraph.example.com/site-admin/pings`.)

## Connection to Sourcegraph
1. To send pings that:
   - Check for new product updates.
   - Send anonymous, non-specific, aggregate metrics back to Sourcegraph.com. You can see the full list below.
1. Sourcegraph extensions are fetched from Sourcegraph.com's extension registry (unless you are using a [private extension registry](https://docs.sourcegraph.com/admin/extensions#publish-extensions-to-a-private-extension-registry)).

There are no other automatic external connections to Sourcegraph.com (or any other site on the internet).

## Critical telemetry

Critical telemetry includes only the high-level data below required for billing, support, updates, and security notices.

- Randomly generated site identifier
- The email address of the initial site installer (or if deleted, the first active site admin), to know who to contact regarding sales, product updates, security updates, and policy updates
- Sourcegraph version string (e.g. "vX.X.X")
- Dependency versions (e.g. "6.0.9" for Redis, or "13.0" for Postgres)
- Deployment type (single Docker image, Docker Compose, Kubernetes cluster, or pure Docker cluster)
- License key associated with your Sourcegraph subscription
- Aggregate count of current monthly users
- Total count of existing user accounts
- Aggregated repository statistics
  - Total size of git repositories stored in bytes
  - Total number of lines of code stored in text search index

## Other telemetry

By default, Sourcegraph also aggregates usage and performance metrics for some product features. No personal or specific information is ever included.

- Whether the instance is deployed on localhost (true/false)
- Which category of authentication provider is in use (built-in, OpenID Connect, an HTTP proxy, SAML, GitHub, GitLab)
- Which code hosts are in use (GitHub, Bitbucket Server, GitLab, Phabricator, Gitolite, AWS CodeCommit, Other)
- Whether new user signup is allowed (true/false)
- Whether a repository has ever been added (true/false)
- Whether a code search has ever been executed (true/false)
- Whether code intelligence has ever been used (true/false)
- Aggregate counts of current daily, weekly, and monthly users
- Aggregate counts of current daily, weekly, and monthly users, by:
  - Whether they are using code host integrations
  - Search modes used (interactive search, plain-text search)
  - Search filters used (e.g. "type:", "repo:", "file:", "lang:", etc.)
- Aggregate daily, weekly, and monthly latencies (in ms) of search queries
- Aggregate daily, weekly, and monthly counts of:
  - Searches using each search mode (interactive search, plain-text search)
  - Searches using each search filter (e.g. "type:", "repo:", "file:", "lang:", etc.)
- Total number of code intelligence queries (e.g., hover tooltips) per week grouped by language
- Number of users performing code intelligence queries (e.g., hover tooltips) per week grouped by language
<!-- depends-on-source: ~/internal/usagestats/campaigns.go -->
- Campaign usage data
  - Total count of page views on the campaign apply page
  - Total count of page views on the campaign details page after creating a campaign
  - Total count of page views on the campaign details page after updating a campaign
  - Total count of created changeset specs
  - Total count of created campaign specs
  - Total count of created campaigns
  - Total count of closed campaigns
  - Total count of changesets created by campaigns
  - Aggregate counts of lines changed, added, deleted in all changesets
  - Total count of changesets created by campaigns that have been merged
  - Aggregate counts of lines changed, added, deleted in all merged changesets
  - Total count of changesets manually added to a campaign
  - Total count of changesets manually added to a campaign that have been merged
  - Aggregate counts of unique monthly users, by:
    - Whether they are contributed to campaigns
    - Whether they only viewed campaigns
  - Weekly campaign (open, closed) and changesets counts (imported, published, unpublished, open, draft, merged, closed) for campaign cohorts created in the last 12 months
- Aggregated counts of users created, deleted, retained, resurrected and churned within the month
- Saved searches usage data
  - Count of saved searches
  - Count of users using saved searches
  - Count of notifications triggered
  - Count of notifications clicked
  - Count of saved search views
- Homepage panel engagement
  - Percentage of panel clicks (out of total views)
  - Total count of unique users engaging with the panels
- Weekly retention rates for user cohorts created in the last 12 weeks
- Search onboarding engagement
  - Total number of views of the onboarding tour
  - Total number of views of each step in the onboarding tour
  - Total number of tours closed
- Sourcegraph extension activation statistics
  - Total number of users that use a given non-default Sourcegraph extension
  - Average number of activations for users that use a given non-default Sourcegraph extension
  - Total number of users that use non-default Sourcegraph extensions
  - Average number of non-default extensions enabled for users that use non-default Sourcegraph extensions
- Code insights usage data
  - Total count of page views on the insights page
  - Count of unique viewers on the insights page
  - Total counts of hovers, clicks, and drags of insights by type (e.g. search, code stats)
  - Total counts of edits, additions, and removals of insights by type
  - Total count of clicks on the "Add more insights" and "Configure insights" buttons on the insights page
  - Weekly count of users that have created an insight, and count of users that have created their first insight this week                  

## CIDR Range for Sourcegraph

Sourcegraph currently uses Cloudflare to provide web application security. You should allow access to all [Cloudflare IP ranges](https://www.cloudflare.com/ips/)
