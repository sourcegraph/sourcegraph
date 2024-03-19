# Code host connections

Sourcegraph can sync repositories from code hosts and other similar services. We designate code hosts between Tier 1 and Tier 2 based on Sourcegraph's capabilities when used with those code hosts.

## Tier 1 code hosts

Tier 1 code hosts are our highest level of support for code hosts. When leveraging a Tier 1 code host, you can expect:

- Scalable repository syncing - Sourcegraph is able to reliably sync repositories from this code host up to 100k repositories. (SLA TBD)
- Scalable permissions syncing - Sourcegraph is able to reliably sync permissions from this code host for up to 10k users. (SLA TBD)
- Authentication - Sourcegraph is able to leverage authentication from this code host (i.e. Login with GitHub).
- Code Search - A developer can seamlessly search across repositories from this code host. (SLAs TBD)
- Code Monitors - A developer can create a code monitor to monitor code in this repository.
- Code Insights - Code Insights reliably works on code sync'd from a tier 1 code host.
- Batch Changes - A Batch Change can be leveraged to submit code changes back to a tier 1 code host while respecting code host permissions.

<table>
   <thead>
      <tr>
        <th>Code Host</th>
        <th>Status</th>
        <th>Repository Syncing</th>
        <th>Permissions Syncing</th>
        <th>Authentication</th>
        <th>Code Search</th>
        <th>Code Monitors</th>
        <th>Code Insights</th>
        <th>Batch Changes</th>
      </tr>
   </thead>
   <tbody>
      <tr>
        <td>GitHub.com</td>
        <td>Tier 1</td>
        <td class="indexer-implemented-y">✓</td> <!-- Repository Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Permissions Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Authentication -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Search -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Monitors -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Insights -->
        <td class="indexer-implemented-y">✓</td> <!-- Batch Changes -->
      </tr>
      <tr>
        <td>GitHub Self-Hosted Enterprise</td>
        <td>Tier 1</td>
        <td class="indexer-implemented-y">✓</td> <!-- Repository Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Permissions Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Authentication -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Search -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Monitors -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Insights -->
        <td class="indexer-implemented-y">✓</td> <!-- Batch Changes -->
      </tr>
      <tr>
        <td>GitLab.com</td>
        <td>Tier 1</td>
        <td class="indexer-implemented-y">✓</td> <!-- Repository Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Permissions Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Authentication -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Search -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Monitors -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Insights -->
        <td class="indexer-implemented-y">✓</td> <!-- Batch Changes -->
      </tr>
      <tr>
        <td>GitLab Self-Hosted</td>
        <td>Tier 1</td>
        <td class="indexer-implemented-y">✓</td> <!-- Repository Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Permissions Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Authentication -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Search -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Monitors -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Insights -->
        <td class="indexer-implemented-y">✓</td> <!-- Batch Changes -->
      </tr>
      <tr>
        <td>Bitbucket Cloud</td>
        <td>Tier 1</td>
        <td class="indexer-implemented-y">✓</td> <!-- Repository Syncing -->
        <td class="indexer-implemented-n">✓</td> <!-- Permissions Syncing -->
        <td class="indexer-implemented-n">✓</td> <!-- Authentication -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Search -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Monitors -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Insights -->
        <td class="indexer-implemented-y">✓</td> <!-- Batch Changes -->
      </tr>
      <tr>
        <td>Bitbucket Server</td>
        <td>Tier 1</td>
        <td class="indexer-implemented-y">✓</td> <!-- Repository Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Permissions Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Authentication -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Search -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Monitors -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Insights -->
        <td class="indexer-implemented-y">✓</td> <!-- Batch Changes -->
      </tr>
      <tr>
        <td>Gerrit</td>
        <td>Tier 1</td>
        <td class="indexer-implemented-y">✓</td> <!-- Repository Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Permissions Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Authentication -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Search -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Monitors -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Insights -->
        <td class="indexer-implemented-y">✓</td> <!-- Batch Changes -->
      </tr>
      <tr>
        <td>Azure DevOps</td>
        <td>Tier 1</td>
        <td class="indexer-implemented-y">✓</td> <!-- Repository Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Permissions Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Authentication -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Search -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Monitors -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Insights -->
        <td class="indexer-implemented-y">✓</td> <!-- Batch Changes -->
      </tr>
      <tr>
        <td>Perforce</td>
        <td>Tier 2 (Working on Tier 1)</td>
        <td class="indexer-implemented-y">✓</td> <!-- Repository Syncing -->
        <td class="indexer-implemented-y">✓</td> <!-- Permissions Syncing -->
        <td class="indexer-implemented-n">✗</td> <!-- Authentication -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Search -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Monitors -->
        <td class="indexer-implemented-n">✗</td> <!-- Code Insights -->
        <td class="indexer-implemented-n">✗</td> <!-- Batch Changes -->
      </tr>
      <tr>
        <td>Plastic SCM (Enterprise)</td>
        <td>Tier 2</td>
        <td class="indexer-implemented-y">✓</td> <!-- Repository Syncing -->
        <td class="indexer-implemented-y">✗</td> <!-- Permissions Syncing -->
        <td class="indexer-implemented-n">✗</td> <!-- Authentication -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Search -->
        <td class="indexer-implemented-y">✓</td> <!-- Code Monitors -->
        <td class="indexer-implemented-n">✓</td> <!-- Code Insights -->
        <td class="indexer-implemented-n">✗</td> <!-- Batch Changes -->
      </tr>
   </tbody>
</table>

#### Status definitions

A code host status is:

- 🟢 _Generally Available:_ Available as a normal product feature up to 100k repositories.
- 🟡 _Partially available:_ Available, but may be limited in some significant ways (either missing or buggy functionality). If you plan to leverage this, please contact your Customer Engineer.
- 🔴 _Not available:_ This functionality is not available within Sourcegraph

## Tier 2: Code Hosts

We recognize there are other code hosts including CVS, SVN, and many more. Today, we do not offer native integrations with these code hosts and customers are advised to leverage [Src-srv-git](./non-git.md) and the [explicit permissions API](../permissions/api.md) as a way to ingest code and permissions respectively into Sourcegraph.

[Src-srv-git](./non-git.md) and the [explicit permissions API](../permissions/api.md) follow the same scale guidance shared above (up to 100k repos and 10k users).

## Configure a code host connection

**Site admins** can configure Sourcegraph to sync code from the following code hosts:

- [GitHub](github.md)
- [GitLab](gitlab.md)
- [Bitbucket Cloud](bitbucket_cloud.md)
- [Bitbucket Server / Bitbucket Data Center](bitbucket_server.md)
- [Azure DevOps](azuredevops.md)
- [Gerrit](gerrit.md)
- [Other Git code hosts (using a Git URL)](other.md)
- [Non-Git code hosts](non-git.md)
  - [Perforce](../repo/perforce.md)
  - [Plastic SCM](../repo/plasticscm.md)
  - [Package repository hosts](package-repos.md)
    - [JVM dependencies](jvm.md)
    - [Go dependencies](go.md)
    - [npm dependencies](npm.md)
    - [Python dependencies](python.md)
    - [Ruby dependencies](ruby.md)
    - [Rust dependencies](rust.md)

## Rate limits

For information on code host-related rate limits, see [rate limits](./rate_limits.md).

## Temporarily disabling requests to code hosts

It may be the case that you'd like to temporarily disable all `git` and API requests from Sourcegraph to a code host. Adding the following to your site configuration will stop Sourcegraph from sending requests to the configured code host connections:

> WARNING: disabling all git and API requests to codehosts will also disable permissions syncs, batch changes, discovery of new repos, and updates to currently synched repos. Synching with codehosts is a core functionality of Sourcegraph and many other features may also be affected.

```json
"disableAutoGitUpdates": true,
"disableAutoCodeHostSyncs": true,
"gitMaxCodehostRequestsPerSecond": 0,
"gitMaxConcurrentClones": 0
```

## Testing Code Host Connections

> WARNING: Sourcegraph 4.4.0 customers are reporting a bug where the connection test is failing when Sourcegraph is running behind proxies where TCP dial cannot be used with ports 80/443. This causes repositories to stop syncing. If you're experiencing this issue, please upgrade to 4.4.1 where normal HTTP requests are used instead.

In Sourcegraph 4.4, site administrators have the ability to test a code host connection via the site-admin UI to improve the debuggability when something goes wrong. This check confirms that Sourcegraph has the ability to connect with the respective code host via TCP dial.
