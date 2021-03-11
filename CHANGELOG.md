<!--
###################################### READ ME ###########################################
### This changelog should always be read on `main` branch. Its contents on version   ###
### branches do not necessarily reflect the changes that have gone into that branch.   ###
##########################################################################################
-->

# Changelog

All notable changes to Sourcegraph are documented in this file.

<!-- START CHANGELOG -->

## Unreleased

### Added

- Opsgenie API keys can now be added via an environment variable. [#18662](https://github.com/sourcegraph/sourcegraph/pull/18662)
- It's now possible to control where code insights are displayed through the boolean settings `insights.displayLocation.homepage`, `insights.displayLocation.insightsPage` and `insights.displayLocation.directory`. [#18979](https://github.com/sourcegraph/sourcegraph/pull/18979)

### Changed

- Campaigns have been renamed to Batch Changes! See [#18771](https://github.com/sourcegraph/sourcegraph/issues/18771) for a detailed log on what has been renamed.
  - A new [Sourcegraph CLI](https://docs.sourcegraph.com/cli) version will use `src batch [preview|apply]` commands, while keeping the old ones working to be used with older Sourcegraph versions.
  - Old URLs in the application and in the documentation will redirect.
  - GraphQL API entities with "campaign" in their name have been deprecated and have new Batch Changes counterparts:
    - Deprecated GraphQL entities: `CampaignState`, `Campaign`, `CampaignSpec`, `CampaignConnection`, `CampaignsCodeHostConnection`, `CampaignsCodeHost`, `CampaignsCredential`, `CampaignDescription`
    - Deprecated GraphQL mutations: `createCampaign`, `applyCampaign`, `moveCampaign`, `closeCampaign`, `deleteCampaign`, `createCampaignSpec`, `createCampaignsCredential`, `deleteCampaignsCredential`
    - Deprecated GraphQL queries: `Org.campaigns`, `User.campaigns`, `User.campaignsCodeHosts`, `camapigns`, `campaign`
  - Site settings with `campaigns` in their name have been replaced with equivalent `batchChanges` settings.
- A repository's `remote.origin.url` is not stored on gitserver disk anymore. Note: if you use the experimental feature `customGitFetch` your setting may need to be updated to specify the remote URL. [#18535](https://github.com/sourcegraph/sourcegraph/pull/18535)
- Repositories and files containing spaces will now render with escaped spaces in the query bar rather than being
  quoted. [#18642](https://github.com/sourcegraph/sourcegraph/pull/18642)
- Sourcegraph is now built with Go 1.16. [#18447](https://github.com/sourcegraph/sourcegraph/pull/18447)
- Cursor hover information in the search query bar will now display after 150ms (previously 0ms). [#18916](https://github.com/sourcegraph/sourcegraph/pull/18916)

### Fixed

- Auto complete suggestions for repositories and files containing spaces will now be automatically escaped when accepting the suggestion. [#18635](https://github.com/sourcegraph/sourcegraph/issues/18635)
- An issue causing repository results containing spaces to not be clickable in some cases. [#18668](https://github.com/sourcegraph/sourcegraph/pull/18668)
- Closing a batch change now correctly closes the entailed changesets, when requested by the user. [#18957](https://github.com/sourcegraph/sourcegraph/pull/18957)

### Removed

- Removed the deprecated GraphQL fields `SearchResults.repositoriesSearched` and `SearchResults.indexedRepositoriesSearched`.
- Removed the deprecated search field `max`

## 3.25.2

### Fixed

- A security vulnerability with in the authentication workflow has been fixed. [#18686](https://github.com/sourcegraph/sourcegraph/pull/18686)

## 3.25.1

### Added

- Experimental: Sync Perforce depots directly through the Sourcegraph UI. To enable, use the feature flag `"experimentalFeatures": { "perforce": "enabled" }`. For more information, see [how to add your Perforce depots](https://docs.sourcegraph.com/admin/repo/perforce). [#16703](https://github.com/sourcegraph/sourcegraph/issues/16703)

## 3.25.0

**IMPORTANT** Sourcegraph now uses Go 1.15. This may break AWS RDS database connections with older x509 certificates. Please follow the Amazon [docs](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL-certificate-rotation.html) to rotate your certificate.

### Added

- New site config option `"log": { "sentry": { "backendDSN": "<REDACTED>" } }` to use a separate Sentry project for backend errors. [#17363](https://github.com/sourcegraph/sourcegraph/pull/17363)
- Structural search now supports searching indexed branches other than default. [#17726](https://github.com/sourcegraph/sourcegraph/pull/17726)
- Structural search now supports searching unindexed revisions. [#17967](https://github.com/sourcegraph/sourcegraph/pull/17967)
- New site config option `"allowSignup"` for SAML authentication to determine if automatically create new users is allowed. [#17989](https://github.com/sourcegraph/sourcegraph/pull/17989)
- Experimental: The webapp can now stream search results to the client, improving search performance. To enable it, add `{ "experimentalFeatures": { "searchStreaming": true } }` in user settings. [#16097](https://github.com/sourcegraph/sourcegraph/pull/16097)
- New product research sign-up page. This can be accessed by all users in their user settings. [#17945](https://github.com/sourcegraph/sourcegraph/pull/17945)
- New site config option `productResearchPage.enabled` to disable access to the product research sign-up page. [#17945](https://github.com/sourcegraph/sourcegraph/pull/17945)
- Pings now contain Sourcegraph extension activation statistics. [#16421](https://github.com/sourcegraph/sourcegraph/pull/16421)
- Pings now contain aggregate Sourcegraph extension activation statistics: the number of users and number of activations per (public) extension per week, and the number of total extension users per week and average extensions activated per user. [#16421](https://github.com/sourcegraph/sourcegraph/pull/16421)
- Pings now contain aggregate code insights usage data: total insight views, interactions, edits, creations, removals, and counts of unique users that view and create insights. [#16421](https://github.com/sourcegraph/sourcegraph/pull/17805)
- When previewing a campaign spec, changesets can be filtered by current state or the action(s) to be performed. [#16960](https://github.com/sourcegraph/sourcegraph/issues/16960)

### Changed

- Alert solutions links included in [monitoring alerts](https://docs.sourcegraph.com/admin/observability/alerting) now link to the relevant documentation version. [#17828](https://github.com/sourcegraph/sourcegraph/pull/17828)
- Secrets (such as access tokens and passwords) will now appear as REDACTED when editing external service config, and in graphql API responses. [#17261](https://github.com/sourcegraph/sourcegraph/issues/17261)
- Sourcegraph is now built with Go 1.15
  - Go `1.15` introduced changes to SSL/TLS connection validation which requires certificates to include a `SAN`. This field was not included in older certificates and clients relied on the `CN` field. You might see an error like `x509: certificate relies on legacy Common Name field`. We recommend that customers using Sourcegraph with an external database and connecting to it using SSL/TLS check whether the certificate is up to date.
  - RDS Customers please reference [AWS' documentation on updating the SSL/TLS certificate](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL-certificate-rotation.html).
- Search results on `.rs` files now recommend `lang:rust` instead of `lang:renderscript` as a filter. [#18316](https://github.com/sourcegraph/sourcegraph/pull/18316)
- Campaigns users creating Personal Access Tokens on GitHub are now asked to request the `user:email` scope in addition to the [previous scopes](https://docs.sourcegraph.com/@3.24/admin/external_service/github#github-api-token-and-access). This will be used in a future Sourcegraph release to display more fine-grained information on the progress of pull requests. [#17555](https://github.com/sourcegraph/sourcegraph/issues/17555)

### Fixed

- Fixes an issue that prevented the hard deletion of a user if they had saved searches. [#17461](https://github.com/sourcegraph/sourcegraph/pull/17461)
- Fixes an issue that caused some missing results for `type:commit` when a pattern was used instead of the `message` field. [#17490](https://github.com/sourcegraph/sourcegraph/pull/17490#issuecomment-764004758)
- Fixes an issue where cAdvisor-based alerts would not fire correctly for services with multiple replicas. [#17600](https://github.com/sourcegraph/sourcegraph/pull/17600)
- Significantly improved performance of structural search on monorepo deployments [#17846](https://github.com/sourcegraph/sourcegraph/pull/17846)
- Fixes an issue where upgrades on Kubernetes may fail due to null environment variable lists in deployment manifests [#1781](https://github.com/sourcegraph/deploy-sourcegraph/pull/1781)
- Fixes an issue where counts on search filters were inaccurate. [#18158](https://github.com/sourcegraph/sourcegraph/pull/18158)
- Fixes services with emptyDir volumes being evicted from nodes. [#1852](https://github.com/sourcegraph/deploy-sourcegraph/pull/1852)

### Removed

- Removed the `search.migrateParser` setting. As of 3.20 and onward, a new parser processes search queries by default. Previously, `search.migrateParser` was available to enable the legacy parser. Enabling/disabling this setting now no longer has any effect. [#17344](https://github.com/sourcegraph/sourcegraph/pull/17344)

## 3.24.1

### Fixed

- Fixes an issue that SAML is not able to proceed with the error `Expected Enveloped and C14N transforms`. [#13032](https://github.com/sourcegraph/sourcegraph/issues/13032)

## 3.24.0

### Added

- Panels in the [Sourcegraph monitoring dashboards](https://docs.sourcegraph.com/admin/observability/metrics#grafana) now:
  - include links to relevant alerts documentation and the new [monitoring dashboards reference](https://docs.sourcegraph.com/admin/observability/dashboards). [#16939](https://github.com/sourcegraph/sourcegraph/pull/16939)
  - include alert events and version changes annotations that can be enabled from the top of each service dashboard. [#17198](https://github.com/sourcegraph/sourcegraph/pull/17198)
- Suggested filters in the search results page can now be scrolled. [#17097](https://github.com/sourcegraph/sourcegraph/pull/17097)
- Structural search queries can now be used in saved searches by adding `patternType:structural`. [#17265](https://github.com/sourcegraph/sourcegraph/pull/17265)

### Changed

- Dashboard links included in [monitoring alerts](https://docs.sourcegraph.com/admin/observability/alerting) now:
  - link directly to the relevant Grafana panel, instead of just the service dashboard. [#17014](https://github.com/sourcegraph/sourcegraph/pull/17014)
  - link to a time frame relevant to the alert, instead of just the past few hours. [#17034](https://github.com/sourcegraph/sourcegraph/pull/17034)
- Added `serviceKind` field of the `ExternalServiceKind` type to `Repository.externalURLs` GraphQL API, `serviceType` field is deprecated and will be removed in the future releases. [#14979](https://github.com/sourcegraph/sourcegraph/issues/14979)
- Deprecated the GraphQL fields `SearchResults.repositoriesSearched` and `SearchResults.indexedRepositoriesSearched`.
- The minimum Kubernetes version required to use the [Kubernetes deployment option](https://docs.sourcegraph.com/admin/install/kubernetes) is now [v1.15 (released June 2019)](https://kubernetes.io/blog/2019/06/19/kubernetes-1-15-release-announcement/).

### Fixed

- Imported changesets acquired an extra button to download the "generated diff", which did nothing, since imported changesets don't have a generated diff. This button has been removed. [#16778](https://github.com/sourcegraph/sourcegraph/issues/16778)
- Quoted global filter values (case, patterntype) are now properly extracted and set in URL parameters. [#16186](https://github.com/sourcegraph/sourcegraph/issues/16186)
- The endpoint for "Open in Sourcegraph" functionality in editor extensions now uses code host connection information to resolve the repository, which makes it more correct and respect the `repositoryPathPattern` setting. [#16846](https://github.com/sourcegraph/sourcegraph/pull/16846)
- Fixed an issue that prevented search expressions of the form `repo:foo (rev:a or rev:b)` from evaluating all revisions [#16873](https://github.com/sourcegraph/sourcegraph/pull/16873)
- Updated language detection library. Includes language detection for `lang:starlark`. [#16900](https://github.com/sourcegraph/sourcegraph/pull/16900)
- Fixed retrieving status for indexed tags and deduplicated main branches in the indexing settings page. [#13787](https://github.com/sourcegraph/sourcegraph/issues/13787)
- Specifying a ref that doesn't exist would show an alert, but still return results [#15576](https://github.com/sourcegraph/sourcegraph/issues/15576)
- Fixed search highlighting the wrong line. [#10468](https://github.com/sourcegraph/sourcegraph/issues/10468)
- Fixed an issue where searches of the form `foo type:file` returned results of type `path` too. [#17076](https://github.com/sourcegraph/sourcegraph/issues/17076)
- Fixed queries like `(type:commit or type:diff)` so that if the query matches both the commit message and the diff, both are returned as results. [#16899](https://github.com/sourcegraph/sourcegraph/issues/16899)
- Fixed container monitoring and provisioning dashboard panels not displaying metrics in certain deployment types and environments. If you continue to have issues with these panels not displaying any metrics after upgrading, please [open an issue](https://github.com/sourcegraph/sourcegraph/issues/new).
- Fixed a nonexistent field in site configuration being marked as "required" when configuring PagerDuty alert notifications. [#17277](https://github.com/sourcegraph/sourcegraph/pull/17277)
- Fixed cases of incorrect highlighting for symbol definitions in the definitions panel. [#17258](https://github.com/sourcegraph/sourcegraph/pull/17258)
- Fixed a Cross-Site Scripting vulnerability where quick links created on the homepage were not sanitized and allowed arbitrary JavaScript execution. [#17099](https://github.com/sourcegraph/sourcegraph/pull/17099)

### Removed

- Interactive mode has now been removed. [#16868](https://github.com/sourcegraph/sourcegraph/pull/16868).

## 3.23.0

### Added

- Password reset link expiration can be customized via `auth.passwordResetLinkExpiry` in the site config. [#13999](https://github.com/sourcegraph/sourcegraph/issues/13999)
- Campaign steps may now include environment variables from outside of the campaign spec using [array syntax](http://docs.sourcegraph.com/campaigns/references/campaign_spec_yaml_reference#environment-array). [#15822](https://github.com/sourcegraph/sourcegraph/issues/15822)
- The total size of all Git repositories and the lines of code for indexed branches are displayed in the site admin overview. [#15125](https://github.com/sourcegraph/sourcegraph/issues/15125)
- Extensions can now add decorations to files on the sidebar tree view and tree page through the experimental `FileDecoration` API. [#15833](https://github.com/sourcegraph/sourcegraph/pull/15833)
- Extensions can now easily query the Sourcegraph GraphQL API through a dedicated API method. [#15566](https://github.com/sourcegraph/sourcegraph/pull/15566)
- Individual changesets can now be downloaded as a diff. [#16098](https://github.com/sourcegraph/sourcegraph/issues/16098)
- The campaigns preview page is much more detailed now, especially when updating existing campaigns. [#16240](https://github.com/sourcegraph/sourcegraph/pull/16240)
- When a newer version of a campaign spec is uploaded, a message is now displayed when viewing the campaign or an outdated campaign spec. [#14532](https://github.com/sourcegraph/sourcegraph/issues/14532)
- Changesets in a campaign can now be searched by title and repository name. [#15781](https://github.com/sourcegraph/sourcegraph/issues/15781)
- Experimental: [`transformChanges` in campaign specs](https://docs.sourcegraph.com/campaigns/references/campaign_spec_yaml_reference#transformchanges) is now available as a feature preview to allow users to create multiple changesets in a single repository. [#16235](https://github.com/sourcegraph/sourcegraph/pull/16235)
- The `gitUpdateInterval` site setting was added to allow custom git update intervals based on repository names. [#16765](https://github.com/sourcegraph/sourcegraph/pull/16765)
- Various additions to syntax highlighting and hover tooltips in the search query bar (e.g., regular expressions). Can be disabled with `{ "experimentalFeatures": { "enableSmartQuery": false } }` in case of unlikely adverse effects. [#16742](https://github.com/sourcegraph/sourcegraph/pull/16742)
- Search queries may now scope subexpressions across repositories and files, and also allow greater freedom for combining search filters. See the updated documentation on [search subexpressions](https://docs.sourcegraph.com/code_search/tutorials/search_subexpressions) to learn more. [#16866](https://github.com/sourcegraph/sourcegraph/pull/16866)

### Changed

- Search indexer tuned to wait longer before assuming a deadlock has occurred. Previously if the indexserver had many cores (40+) and indexed a monorepo it could give up. [#16110](https://github.com/sourcegraph/sourcegraph/pull/16110)
- The total size of all Git repositories and the lines of code for indexed branches will be sent back in pings as part of critical telemetry. [#16188](https://github.com/sourcegraph/sourcegraph/pull/16188)
- The `gitserver` container now has a dependency on Postgres. This does not require any additional configuration unless access to Postgres requires a sidecar proxy / firewall rules. [#16121](https://github.com/sourcegraph/sourcegraph/pull/16121)
- Licensing is now enforced for campaigns: creating a campaign with more than five changesets requires a valid license. Please [contact Sourcegraph with any licensing questions](https://about.sourcegraph.com/contact/sales/). [#15715](https://github.com/sourcegraph/sourcegraph/issues/15715)

### Fixed

- Syntax highlighting on files with mixed extension case (e.g. `.CPP` vs `.cpp`) now works as expected. [#11327](https://github.com/sourcegraph/sourcegraph/issues/11327)
- After applying a campaign, some GitLab MRs might have had outdated state shown in the UI until the next sync with the code host. [#16100](https://github.com/sourcegraph/sourcegraph/pull/16100)
- The web app no longer sends stale text document content to extensions. [#14965](https://github.com/sourcegraph/sourcegraph/issues/14965)
- The blob viewer now supports multiple decorations per line as intended. [#15063](https://github.com/sourcegraph/sourcegraph/issues/15063)
- Repositories with plus signs in their name can now be navigated to as expected. [#15079](https://github.com/sourcegraph/sourcegraph/issues/15079)

### Removed

-

## 3.22.1

### Changed

- Reduced memory and CPU required for updating the code intelligence commit graph [#16517](https://github.com/sourcegraph/sourcegraph/pull/16517)

## 3.22.0

### Added

- GraphQL and TOML syntax highlighting is now back (special thanks to @rvantonder) [#13935](https://github.com/sourcegraph/sourcegraph/issues/13935)
- Zig and DreamMaker syntax highlighting.
- Campaigns now support publishing GitHub draft PRs and GitLab WIP MRs. [#7998](https://github.com/sourcegraph/sourcegraph/issues/7998)
- `indexed-searcher`'s watchdog can be configured and has additional instrumentation. This is useful when diagnosing [zoekt-webserver is restarting due to watchdog](https://docs.sourcegraph.com/admin/observability/troubleshooting#scenario-zoekt-webserver-is-restarting-due-to-watchdog). [#15148](https://github.com/sourcegraph/sourcegraph/pull/15148)
- Pings now contain Redis & Postgres server versions. [14405](https://github.com/sourcegraph/sourcegraph/14405)
- Aggregated usage data of the search onboarding tour is now included in pings. The data tracked are: total number of views of the onboarding tour, total number of views of each step in the onboarding tour, total number of tours closed. [#15113](https://github.com/sourcegraph/sourcegraph/pull/15113)
- Users can now specify credentials for code hosts to enable campaigns for non site-admin users. [#15506](https://github.com/sourcegraph/sourcegraph/pull/15506)
- A `campaigns.restrictToAdmins` site configuration option has been added to prevent non site-admin users from using campaigns. [#15785](https://github.com/sourcegraph/sourcegraph/pull/15785)
- Number of page views on campaign apply page, page views on campaign details page after create/update, closed campaigns, created campaign specs and changesets specs and the sum of changeset diff stats will be sent back in pings. [#15279](https://github.com/sourcegraph/sourcegraph/pull/15279)
- Users can now explicitly set their primary email address. [#15683](https://github.com/sourcegraph/sourcegraph/pull/15683)
- "[Why code search is still needed for monorepos](https://docs.sourcegraph.com/adopt/code_search_in_monorepos)" doc page

### Changed

- Improved contrast / visibility in comment syntax highlighting. [#14546](https://github.com/sourcegraph/sourcegraph/issues/14546)
- Campaigns are no longer in beta. [#14900](https://github.com/sourcegraph/sourcegraph/pull/14900)
- Campaigns now have a fancy new icon. [#14740](https://github.com/sourcegraph/sourcegraph/pull/14740)
- Search queries with an unbalanced closing paren `)` are now invalid, since this likely indicates an error. Previously, patterns with dangling `)` were valid in some cases. Note that patterns with dangling `)` can still be searched, but should be quoted via `content:"foo)"`. [#15042](https://github.com/sourcegraph/sourcegraph/pull/15042)
- Extension providers can now return AsyncIterables, enabling dynamic provider results without dependencies. [#15042](https://github.com/sourcegraph/sourcegraph/issues/15061)
- Deprecated the `"email.smtp": { "disableTLS" }` site config option, this field has been replaced by `"email.smtp": { "noVerifyTLS" }`. [#15682](https://github.com/sourcegraph/sourcegraph/pull/15682)

### Fixed

- The `file:` added to the search field when navigating to a tree or file view will now behave correctly when the file path contains spaces. [#12296](https://github.com/sourcegraph/sourcegraph/issues/12296)
- OAuth login now respects site configuration `experimentalFeatures: { "tls.external": {...} }` for custom certificates and skipping TLS verify. [#14144](https://github.com/sourcegraph/sourcegraph/issues/14144)
- If the `HEAD` file in a cloned repo is absent or truncated, background cleanup activities will use a best-effort default to remedy the situation. [#14962](https://github.com/sourcegraph/sourcegraph/pull/14962)
- Search input will always show suggestions. Previously we only showed suggestions for letters and some special characters. [#14982](https://github.com/sourcegraph/sourcegraph/pull/14982)
- Fixed an issue where `not` keywords were not recognized inside expression groups, and treated incorrectly as patterns. [#15139](https://github.com/sourcegraph/sourcegraph/pull/15139)
- Fixed an issue where hover pop-ups would not show on the first character of a valid hover range in search queries. [#15410](https://github.com/sourcegraph/sourcegraph/pull/15410)
- Fixed an issue where submodules configured with a relative URL resulted in non-functional hyperlinks in the file tree UI. [#15286](https://github.com/sourcegraph/sourcegraph/issues/15286)
- Pushing commits to public GitLab repositories with campaigns now works, since we use the configured token even if the repository is public. [#15536](https://github.com/sourcegraph/sourcegraph/pull/15536)
- `.kts` is now highlighted properly as Kotlin code, fixed various other issues in Kotlin syntax highlighting.
- Fixed an issue where the value of `content:` was treated literally when the regular expression toggle is active. [#15639](https://github.com/sourcegraph/sourcegraph/pull/15639)
- Fixed an issue where non-site admins were prohibited from updating some of their other personal metadata when `auth.enableUsernameChanges` was `false`. [#15663](https://github.com/sourcegraph/sourcegraph/issues/15663)
- Fixed the `url` fields of repositories and trees in GraphQL returning URLs that were not %-encoded (e.g. when the repository name contained spaces). [#15667](https://github.com/sourcegraph/sourcegraph/issues/15667)
- Fixed "Find references" showing errors in the references panel in place of the syntax-highlighted code for repositories with spaces in their name. [#15618](https://github.com/sourcegraph/sourcegraph/issues/15618)
- Fixed an issue where specifying the `repohasfile` filter did not return results as expected unless `repo` was specified. [#15894](https://github.com/sourcegraph/sourcegraph/pull/15894)
- Fixed an issue causing user input in the search query field to be erased in some cases. [#15921](https://github.com/sourcegraph/sourcegraph/issues/15921).

### Removed

-

## 3.21.2

:warning: WARNING :warning: For users of single-image Sourcegraph instance, please delete the secret key file `/var/lib/sourcegraph/token` inside the container before attempting to upgrade to 3.21.x.

### Fixed

- Fix externalURLs alert logic [#14980](https://github.com/sourcegraph/sourcegraph/pull/14980)

## 3.21.1

:warning: WARNING :warning: For users of single-image Sourcegraph instance, please delete the secret key file `/var/lib/sourcegraph/token` inside the container before attempting to upgrade to 3.21.x.

### Fixed

- Fix alerting for native integration condition [#14775](https://github.com/sourcegraph/sourcegraph/pull/14775)
- Fix query with large repo count hanging [#14944](https://github.com/sourcegraph/sourcegraph/pull/14944)
- Fix server upgrade where codeintel database does not exist [#14953](https://github.com/sourcegraph/sourcegraph/pull/14953)
- CVE-2019-18218 in postgres docker image [#14954](https://github.com/sourcegraph/sourcegraph/pull/14954)
- Fix an issue where .git/HEAD in invalid [#14962](https://github.com/sourcegraph/sourcegraph/pull/14962)
- Repository syncing will not happen more frequently than the repoListUpdateInterval config value [#14901](https://github.com/sourcegraph/sourcegraph/pull/14901) [#14983](https://github.com/sourcegraph/sourcegraph/pull/14983)

## 3.21.0

:warning: WARNING :warning: For users of single-image Sourcegraph instance, please delete the secret key file `/var/lib/sourcegraph/token` inside the container before attempting to upgrade to 3.21.x.

### Added

- The new GraphQL API query field `namespaceByName(name: String!)` makes it easier to look up the user or organization with the given name. Previously callers needed to try looking up the user and organization separately.
- Changesets created by campaigns will now include a link back to the campaign in their body text. [#14033](https://github.com/sourcegraph/sourcegraph/issues/14033)
- Users can now preview commits that are going to be created in their repositories in the campaign preview UI. [#14181](https://github.com/sourcegraph/sourcegraph/pull/14181)
- If emails are configured, the user will be sent an email when important account information is changed. This currently encompasses changing/resetting the password, adding/removing emails, and adding/removing access tokens. [#14320](https://github.com/sourcegraph/sourcegraph/pull/14320)
- A subset of changesets can now be published by setting the `published` flag in campaign specs [to an array](https://docs.sourcegraph.com/@main/campaigns/campaign_spec_yaml_reference#publishing-only-specific-changesets), which allows only specific changesets within a campaign to be published based on the repository name. [#13476](https://github.com/sourcegraph/sourcegraph/pull/13476)
- Homepage panels are now enabled by default. [#14287](https://github.com/sourcegraph/sourcegraph/issues/14287)
- The most recent ping data is now available to site admins via the Site-admin > Pings page. [#13956](https://github.com/sourcegraph/sourcegraph/issues/13956)
- Homepage panel engagement metrics will be sent back in pings. [#14589](https://github.com/sourcegraph/sourcegraph/pull/14589)
- Homepage now has a footer with links to different extensibility features. [#14638](https://github.com/sourcegraph/sourcegraph/issues/14638)
- Added an onboarding tour of Sourcegraph for new users. It can be enabled in user settings with `experimentalFeatures.showOnboardingTour` [#14636](https://github.com/sourcegraph/sourcegraph/pull/14636)
- Added an onboarding tour of Sourcegraph for new users. [#14636](https://github.com/sourcegraph/sourcegraph/pull/14636)
- Repository GraphQL queries now support an `after` parameter that permits cursor-based pagination. [#13715](https://github.com/sourcegraph/sourcegraph/issues/13715)
- Searches in the Recent Searches panel and other places are now syntax highlighted. [#14443](https://github.com/sourcegraph/sourcegraph/issues/14443)

### Changed

- Interactive search mode is now disabled by default because the new plain text search input is smarter. To reenable it, add `{ "experimentalFeatures": { "splitSearchModes": true } }` in user settings.
- The extension registry has been redesigned to make it easier to find non-default Sourcegraph extensions.
- Tokens and similar sensitive information included in the userinfo portion of remote repository URLs will no longer be visible on the Mirroring settings page. [#14153](https://github.com/sourcegraph/sourcegraph/pull/14153)
- The sign in and sign up forms have been redesigned with better input validation.
- Kubernetes admins mounting [configuration files](https://docs.sourcegraph.com/admin/config/advanced_config_file#kubernetes-configmap) are encouraged to change how the ConfigMap is mounted. See the new documentation. Previously our documentation suggested using subPath. However, this lead to Kubernetes not automatically updating the files on configuration change. [#14297](https://github.com/sourcegraph/sourcegraph/pull/14297)
- The precise code intel bundle manager will now expire any converted LSIF data that is older than `PRECISE_CODE_INTEL_MAX_DATA_AGE` (30 days by default) that is also not visible from the tip of the default branch.
- `SRC_LOG_LEVEL=warn` is now the default in Docker Compose and Kubernetes deployments, reducing the amount of uninformative log spam. [#14458](https://github.com/sourcegraph/sourcegraph/pull/14458)
- Permissions data that were stored in deprecated binary format are abandoned. Downgrade from 3.21 to 3.20 is OK, but to 3.19 or prior versions might experience missing/incomplete state of permissions for a short period of time. [#13740](https://github.com/sourcegraph/sourcegraph/issues/13740)
- The query builder page is now disabled by default. To reenable it, add `{ "experimentalFeatures": { "showQueryBuilder": true } }` in user settings.
- The GraphQL `updateUser` mutation now returns the updated user (instead of an empty response).

### Fixed

- Git clone URLs now validate their format correctly. [#14313](https://github.com/sourcegraph/sourcegraph/pull/14313)
- Usernames set in Slack `observability.alerts` now apply correctly. [#14079](https://github.com/sourcegraph/sourcegraph/pull/14079)
- Path segments in breadcrumbs get truncated correctly again on small screen sizes instead of inflating the header bar. [#14097](https://github.com/sourcegraph/sourcegraph/pull/14097)
- GitLab pipelines are now parsed correctly and show their current status in campaign changesets. [#14129](https://github.com/sourcegraph/sourcegraph/pull/14129)
- Fixed an issue where specifying any repogroups would effectively search all repositories for all repogroups. [#14190](https://github.com/sourcegraph/sourcegraph/pull/14190)
- Changesets that were previously closed after being detached from a campaign are now reopened when being reattached. [#14099](https://github.com/sourcegraph/sourcegraph/pull/14099)
- Previously large files that match the site configuration [search.largeFiles](https://docs.sourcegraph.com/admin/config/site_config#search-largeFiles) would not be indexed if they contained a large number of unique trigrams. We now index those files as well. Note: files matching the glob still need to be valid utf-8. [#12443](https://github.com/sourcegraph/sourcegraph/issues/12443)
- Git tags without a `creatordate` value will no longer break tag search within a repository. [#5453](https://github.com/sourcegraph/sourcegraph/issues/5453)
- Campaigns pages now work properly on small viewports. [#14292](https://github.com/sourcegraph/sourcegraph/pull/14292)
- Fix an issue with viewing repositories that have spaces in the repository name [#2867](https://github.com/sourcegraph/sourcegraph/issues/2867)

### Removed

- Syntax highlighting for GraphQL, INI, TOML, and Perforce files has been removed [due to incompatible/absent licenses](https://github.com/sourcegraph/sourcegraph/issues/13933). We plan to [add it back in the future](https://github.com/sourcegraph/sourcegraph/issues?q=is%3Aissue+is%3Aopen+add+syntax+highlighting+for+develop+a+).
- Search scope pages (`/search/scope/:id`) were removed.
- User-defined search scopes are no longer shown below the search bar on the homepage. Use the [`quicklinks`](https://docs.sourcegraph.com/user/personalization/quick_links) setting instead to display links there.
- The explore page (`/explore`) was removed.
- The sign out page was removed.
- The unused GraphQL types `DiffSearchResult` and `DeploymentConfiguration` were removed.
- The deprecated GraphQL mutation `updateAllMirrorRepositories`.
- The deprecated GraphQL field `Site.noRepositoriesEnabled`.
- Total counts of users by product area have been removed from pings.
- Aggregate daily, weekly, and monthly latencies (in ms) of code intelligence events (e.g., hover tooltips) have been removed from pings.

## 3.20.1

### Fixed

- gomod: rollback go-diff to v0.5.3 (v0.6.0 causes panic in certain cases) [#13973](https://github.com/sourcegraph/sourcegraph/pull/13973).
- Fixed an issue causing the scoped query in the search field to be erased when viewing files. [#13954](https://github.com/sourcegraph/sourcegraph/pull/13954).

## 3.20.0

### Added

- Site admins can now force a specific user to re-authenticate on their next request or visit. [#13647](https://github.com/sourcegraph/sourcegraph/pull/13647)
- Sourcegraph now watches its [configuration files](https://docs.sourcegraph.com/admin/config/advanced_config_file) (when using external files) and automatically applies the changes to Sourcegraph's configuration when they change. For example, this allows Sourcegraph to detect when a Kubernetes ConfigMap changes. [#13646](https://github.com/sourcegraph/sourcegraph/pull/13646)
- To define repository groups (`search.repositoryGroups` in global, org, or user settings), you can now specify regular expressions in addition to single repository names. [#13730](https://github.com/sourcegraph/sourcegraph/pull/13730)
- The new site configuration property `search.limits` configures the maximum search timeout and the maximum number of repositories to search for various types of searches. [#13448](https://github.com/sourcegraph/sourcegraph/pull/13448)
- Files and directories can now be excluded from search by adding the file `.sourcegraph/ignore` to the root directory of a repository. Each line in the _ignore_ file is interpreted as a globbing pattern. [#13690](https://github.com/sourcegraph/sourcegraph/pull/13690)
- Structural search syntax now allows regular expressions in patterns. Also, `...` can now be used in place of `:[_]`. See the [documentation](https://docs.sourcegraph.com/@main/code_search/reference/structural) for example syntax. [#13809](https://github.com/sourcegraph/sourcegraph/pull/13809)
- The total size of all Git repositories and the lines of code for indexed branches will be sent back in pings. [#13764](https://github.com/sourcegraph/sourcegraph/pull/13764)
- Experimental: A new homepage UI for Sourcegraph Server shows the user their recent searches, repositories, files, and saved searches. It can be enabled with `experimentalFeatures.showEnterpriseHomePanels`. [#13407](https://github.com/sourcegraph/sourcegraph/issues/13407)

### Changed

- Campaigns are enabled by default for all users. Site admins may view and create campaigns; everyone else may only view campaigns. The new site configuration property `campaigns.enabled` can be used to disable campaigns for all users. The properties `campaigns.readAccess`, `automation.readAccess.enabled`, and `"experimentalFeatures": { "automation": "enabled" }}` are deprecated and no longer have any effect.
- Diff and commit searches are limited to 10,000 repositories (if `before:` or `after:` filters are used), or 50 repositories (if no time filters are used). You can configure this limit in the site configuration property `search.limits`. [#13386](https://github.com/sourcegraph/sourcegraph/pull/13386)
- The site configuration `maxReposToSearch` has been deprecated in favor of the property `maxRepos` on `search.limits`. [#13439](https://github.com/sourcegraph/sourcegraph/pull/13439)
- Search queries are now processed by a new parser that will always be enabled going forward. There should be no material difference in behavior. In case of adverse effects, the previous parser can be reenabled by setting `"search.migrateParser": false` in settings. [#13435](https://github.com/sourcegraph/sourcegraph/pull/13435)
- It is now possible to search for file content that excludes a term using the `NOT` operator. [#12412](https://github.com/sourcegraph/sourcegraph/pull/12412)
- `NOT` is available as an alternative syntax of `-` on supported keywords `repo`, `file`, `content`, `lang`, and `repohasfile`. [#12412](https://github.com/sourcegraph/sourcegraph/pull/12412)
- Negated content search is now also supported for unindexed repositories. Previously it was only supported for indexed repositories [#13359](https://github.com/sourcegraph/sourcegraph/pull/13359).
- The experimental feature flag `andOrQuery` is deprecated. [#13435](https://github.com/sourcegraph/sourcegraph/pull/13435)
- After a user's password changes, they will be signed out on all devices and must sign in again. [#13647](https://github.com/sourcegraph/sourcegraph/pull/13647)
- `rev:` is available as alternative syntax of `@` for searching revisions instead of the default branch [#13133](https://github.com/sourcegraph/sourcegraph/pull/13133)
- Campaign URLs have changed to use the campaign name instead of an opaque ID. The old URLs no longer work. [#13368](https://github.com/sourcegraph/sourcegraph/pull/13368)
- A new `external_service_repos` join table was added. The migration required to make this change may take a few minutes.

### Fixed

- User satisfaction/NPS surveys will now correctly provide a range from 0–10, rather than 0–9. [#13163](https://github.com/sourcegraph/sourcegraph/pull/13163)
- Fixed a bug where we returned repositories with invalid revisions in the search results. Now, if a user specifies an invalid revision, we show an alert. [#13271](https://github.com/sourcegraph/sourcegraph/pull/13271)
- Previously it wasn't possible to search for certain patterns containing `:` because they would not be considered valid filters. We made these checks less strict. [#10920](https://github.com/sourcegraph/sourcegraph/pull/10920)
- When a user signs out of their account, all of their sessions will be invalidated, not just the session where they signed out. [#13647](https://github.com/sourcegraph/sourcegraph/pull/13647)
- URL information will no longer be leaked by the HTTP referer header. This prevents the user's password reset code from being leaked. [#13804](https://github.com/sourcegraph/sourcegraph/pull/13804)
- GitLab OAuth2 user authentication now respects `tls.external` site setting. [#13814](https://github.com/sourcegraph/sourcegraph/pull/13814)

### Removed

- The smartSearchField feature is now always enabled. The `experimentalFeatures.smartSearchField` settings option has been removed.

## 3.19.2

### Fixed

- search: always limit commit and diff to less than 10,000 repos [a97f81b0f7](https://github.com/sourcegraph/sourcegraph/commit/a97f81b0f79535253bd7eae6c30d5c91d48da5ca)
- search: configurable limits on commit/diff search [1c22d8ce1](https://github.com/sourcegraph/sourcegraph/commit/1c22d8ce13c149b3fa3a7a26f8cb96adc89fc556)
- search: add site configuration for maxTimeout [d8d61b43c0f](https://github.com/sourcegraph/sourcegraph/commit/d8d61b43c0f0d229d46236f2f128ca0f93455172)

## 3.19.1

### Fixed

- migrations: revert migration causing deadlocks in some deployments [#13194](https://github.com/sourcegraph/sourcegraph/pull/13194)

## 3.19.0

### Added

- Emails can be now be sent to SMTP servers with self-signed certificates, using `email.smtp.disableTLS`. [#12243](https://github.com/sourcegraph/sourcegraph/pull/12243)
- Saved search emails now include a link to the user's saved searches page. [#11651](https://github.com/sourcegraph/sourcegraph/pull/11651)
- Campaigns can now be synced using GitLab webhooks. [#12139](https://github.com/sourcegraph/sourcegraph/pull/12139)
- Configured `observability.alerts` can now be tested using a GraphQL endpoint, `triggerObservabilityTestAlert`. [#12532](https://github.com/sourcegraph/sourcegraph/pull/12532)
- The Sourcegraph CLI can now serve local repositories for Sourcegraph to clone. This was previously in a command called `src-expose`. See [serving local repositories](https://docs.sourcegraph.com/admin/external_service/src_serve_git) in our documentation to find out more. [#12363](https://github.com/sourcegraph/sourcegraph/issues/12363)
- The count of retained, churned, resurrected, new and deleted users will be sent back in pings. [#12136](https://github.com/sourcegraph/sourcegraph/pull/12136)
- Saved search usage will be sent back in pings. [#12956](https://github.com/sourcegraph/sourcegraph/pull/12956)
- Any request with `?trace=1` as a URL query parameter will enable Jaeger tracing (if Jaeger is enabled). [#12291](https://github.com/sourcegraph/sourcegraph/pull/12291)
- Password reset emails will now be automatically sent to users created by a site admin if email sending is configured and password reset is enabled. Previously, site admins needed to manually send the user this password reset link. [#12803](https://github.com/sourcegraph/sourcegraph/pull/12803)
- Syntax highlighting for `and` and `or` search operators. [#12694](https://github.com/sourcegraph/sourcegraph/pull/12694)
- It is now possible to search for file content that excludes a term using the `NOT` operator. Negating pattern syntax requires setting `"search.migrateParser": true` in settings and is currently only supported for literal and regexp queries on indexed repositories. [#12412](https://github.com/sourcegraph/sourcegraph/pull/12412)
- `NOT` is available as an alternative syntax of `-` on supported keywords `repo`, `file`, `content`, `lang`, and `repohasfile`. `NOT` requires setting `"search.migrateParser": true` option in settings. [#12520](https://github.com/sourcegraph/sourcegraph/pull/12520)

### Changed

- Repository permissions are now always checked and updated asynchronously ([background permissions syncing](https://docs.sourcegraph.com/admin/repo/permissions#background-permissions-syncing)) instead of blocking each operation. The site config option `permissions.backgroundSync` (which enabled this behavior in previous versions) is now a no-op and is deprecated.
- [Background permissions syncing](https://docs.sourcegraph.com/admin/repo/permissions#background-permissions-syncing) (`permissions.backgroundSync`) has become the only option for mirroring repository permissions from code hosts. All relevant site configurations are deprecated.

### Fixed

- Fixed site admins are getting errors when visiting user settings page in OSS version. [#12313](https://github.com/sourcegraph/sourcegraph/pull/12313)
- `github-proxy` now respects the environment variables `HTTP_PROXY`, `HTTPS_PROXY` and `NO_PROXY` (or the lowercase versions thereof). Other services already respect these variables, but this was missed. If you need a proxy to access github.com set the environment variable for the github-proxy container. [#12377](https://github.com/sourcegraph/sourcegraph/issues/12377)
- `sourcegraph-frontend` now respects the `tls.external` experimental setting as well as the proxy environment variables. In proxy environments this allows Sourcegraph to fetch extensions. [#12633](https://github.com/sourcegraph/sourcegraph/issues/12633)
- Fixed a bug that would sometimes cause trailing parentheses to be removed from search queries upon page load. [#12960](https://github.com/sourcegraph/sourcegraph/issues/12690)
- Indexed search will no longer stall if a specific index job stalls. Additionally at scale many corner cases causing indexing to stall have been fixed. [#12502](https://github.com/sourcegraph/sourcegraph/pull/12502)
- Indexed search will quickly recover from rebalancing / roll outs. When a indexed search shard goes down, its repositories are re-indexed by other shards. This takes a while and during a rollout leads to effectively re-indexing all repositories. We now avoid indexing the redistributed repositories once a shard comes back online. [#12474](https://github.com/sourcegraph/sourcegraph/pull/12474)
- Indexed search has many improvements to observability. More detailed Jaeger traces, detailed logging during startup and more prometheus metrics.
- The site admin repository needs-index page is significantly faster. Previously on large instances it would usually timeout. Now it should load within a second. [#12513](https://github.com/sourcegraph/sourcegraph/pull/12513)
- User password reset page now respects the value of site config `auth.minPasswordLength`. [#12971](https://github.com/sourcegraph/sourcegraph/pull/12971)
- Fixed an issue where duplicate search results would show for queries with `or`-expressions. [#12531](https://github.com/sourcegraph/sourcegraph/pull/12531)
- Faster indexed search queries over a large number of repositories. Searching 100k+ repositories is now ~400ms faster and uses much less memory. [#12546](https://github.com/sourcegraph/sourcegraph/pull/12546)

### Removed

- Deprecated site settings `lightstepAccessToken` and `lightstepProject` have been removed. We now only support sending traces to Jaeger. Configure Jaeger with `observability.tracing` site setting.
- Removed `CloneInProgress` option from GraphQL Repositories API. [#12560](https://github.com/sourcegraph/sourcegraph/pull/12560)

## 3.18.0

### Added

- To search across multiple revisions of the same repository, list multiple branch names (or other revspecs) separated by `:` in your query, as in `repo:myrepo@branch1:branch2:branch2`. To search all branches, use `repo:myrepo@*refs/heads/`. Previously this was only supported for diff and commit searches and only available via the experimental site setting `searchMultipleRevisionsPerRepository`.
- The "Add repositories" page (/site-admin/external-services/new) now displays a dismissable notification explaining how and why we access code host data. [#11789](https://github.com/sourcegraph/sourcegraph/pull/11789).
- New `observability.alerts` features:
  - Notifications now provide more details about relevant alerts.
  - Support for email and OpsGenie notifications has been added. Note that to receive email alerts, `email.address` and `email.smtp` must be configured.
  - Some notifiers now have new options:
    - PagerDuty notifiers: `severity` and `apiUrl`
    - Webhook notifiers: `bearerToken`
  - A new `disableSendResolved` option disables notifications for when alerts resolve themselves.
- Recently firing critical alerts can now be displayed to admins via site alerts, use the flag `{ "alerts.hideObservabilitySiteAlerts": false }` to enable these alerts in user configuration.
- Specific alerts can now be silenced using `observability.silenceAlerts`. [#12087](https://github.com/sourcegraph/sourcegraph/pull/12087)
- Revisions listed in `experimentalFeatures.versionContext` will be indexed for faster searching. This is the first support towards indexing non-default branches. [#6728](https://github.com/sourcegraph/sourcegraph/issues/6728)
- Revisions listed in `experimentalFeatures.versionContext` or `experimentalFeatures.search.index.branches` will be indexed for faster searching. This is the first support towards indexing non-default branches. [#6728](https://github.com/sourcegraph/sourcegraph/issues/6728)
- Campaigns are now supported on GitLab.
- Campaigns now support GitLab and allow users to create, update and track merge requests on GitLab instances.
- Added a new section on the search homepage on Sourcegraph.com. It is currently feature flagged behind `experimentalFeatures.showRepogroupHomepage` in settings.
- Added new repository group pages.

### Changed

- Some monitoring alerts now have more useful descriptions. [#11542](https://github.com/sourcegraph/sourcegraph/pull/11542)
- Searching `fork:true` or `archived:true` has the same behaviour as searching `fork:yes` or `archived:yes` respectively. Previously it incorrectly had the same behaviour as `fork:only` and `archived:only` respectively. [#11740](https://github.com/sourcegraph/sourcegraph/pull/11740)
- Configuration for `observability.alerts` has changed and notifications are now provided by Prometheus Alertmanager. [#11832](https://github.com/sourcegraph/sourcegraph/pull/11832)
  - Removed: `observability.alerts.id`.
  - Removed: Slack notifiers no longer accept `mentionUsers`, `mentionGroups`, `mentionChannel`, and `token` options.

### Fixed

- The single-container `sourcegraph/server` image now correctly reports its version.
- An issue where repositories would not clone and index in some edge cases where the clones were deleted or not successful on gitserver. [#11602](https://github.com/sourcegraph/sourcegraph/pull/11602)
- An issue where repositories previously deleted on gitserver would not immediately reclone on system startup. [#11684](https://github.com/sourcegraph/sourcegraph/issues/11684)
- An issue where the sourcegraph/server Jaeger config was invalid. [#11661](https://github.com/sourcegraph/sourcegraph/pull/11661)
- An issue where valid search queries were improperly hinted as being invalid in the search field. [#11688](https://github.com/sourcegraph/sourcegraph/pull/11688)
- Reduce frontend memory spikes by limiting the number of goroutines launched by our GraphQL resolvers. [#11736](https://github.com/sourcegraph/sourcegraph/pull/11736)
- Fixed a bug affecting Sourcegraph icon display in our Phabricator native integration [#11825](https://github.com/sourcegraph/sourcegraph/pull/11825).
- Improve performance of site-admin repositories status page. [#11932](https://github.com/sourcegraph/sourcegraph/pull/11932)
- An issue where search autocomplete for files didn't add the right path. [#12241](https://github.com/sourcegraph/sourcegraph/pull/12241)

### Removed

- Backwards compatibility for "critical configuration" (a type of configuration that was deprecated in December 2019) was removed. All critical configuration now belongs in site configuration.
- Experimental feature setting `{ "experimentalFeatures": { "searchMultipleRevisionsPerRepository": true } }` will be removed in 3.19. It is now always on. Please remove references to it.
- Removed "Cloning" tab in site-admin Repository Status page. [#12043](https://github.com/sourcegraph/sourcegraph/pull/12043)
- The `blacklist` configuration option for Gitolite that was deprecated in 3.17 has been removed in 3.19. Use `exclude.pattern` instead. [#12345](https://github.com/sourcegraph/sourcegraph/pull/12345)

## 3.17.3

### Fixed

- git: Command retrying made a copy that was never used [#11807](https://github.com/sourcegraph/sourcegraph/pull/11807)
- frontend: Allow opt out of EnsureRevision when making a comparison query [#11811](https://github.com/sourcegraph/sourcegraph/pull/11811)
- Fix Phabricator icon class [#11825](https://github.com/sourcegraph/sourcegraph/pull/11825)

## 3.17.2

### Fixed

- An issue where repositories previously deleted on gitserver would not immediately reclone on system startup. [#11684](https://github.com/sourcegraph/sourcegraph/issues/11684)

## 3.17.1

### Added

- Improved search indexing metrics

### Changed

- Some monitoring alerts now have more useful descriptions. [#11542](https://github.com/sourcegraph/sourcegraph/pull/11542)

### Fixed

- The single-container `sourcegraph/server` image now correctly reports its version.
- An issue where repositories would not clone and index in some edge cases where the clones were deleted or not successful on gitserver. [#11602](https://github.com/sourcegraph/sourcegraph/pull/11602)
- An issue where the sourcegraph/server Jaeger config was invalid. [#11661](https://github.com/sourcegraph/sourcegraph/pull/11661)

## 3.17.0

### Added

- The search results page now shows a small UI notification if either repository forks or archives are excluded, when `fork` or `archived` options are not explicitly set. [#10624](https://github.com/sourcegraph/sourcegraph/pull/10624)
- Prometheus metric `src_gitserver_repos_removed_disk_pressure` which is incremented everytime we remove a repository due to disk pressure. [#10900](https://github.com/sourcegraph/sourcegraph/pull/10900)
- `gitolite.exclude` setting in [Gitolite external service config](https://docs.sourcegraph.com/admin/external_service/gitolite#configuration) now supports a regular expression via the `pattern` field. This is consistent with how we exclude in other external services. Additionally this is a replacement for the deprecated `blacklist` configuration. [#11403](https://github.com/sourcegraph/sourcegraph/pull/11403)
- Notifications about Sourcegraph being out of date will now be shown to site admins and users (depending on how out-of-date it is).
- Alerts are now configured using `observability.alerts` in the site configuration, instead of via the Grafana web UI. This does not yet support all Grafana notification channel types, and is not yet supported on `sourcegraph/server` ([#11473](https://github.com/sourcegraph/sourcegraph/issues/11473)). For more details, please refer to the [Sourcegraph alerting guide](https://docs.sourcegraph.com/admin/observability/alerting).
- Experimental basic support for detecting if your Sourcegraph instance is over or under-provisioned has been added through a set of dashboards and warning-level alerts based on container utilization.
- Query [operators](https://docs.sourcegraph.com/code_search/reference/queries#operators) `and` and `or` are now enabled by default in all search modes for searching file content. [#11521](https://github.com/sourcegraph/sourcegraph/pull/11521)

### Changed

- Repository search within a version context will link to the revision in the version context. [#10860](https://github.com/sourcegraph/sourcegraph/pull/10860)
- Background permissions syncing becomes the default method to sync permissions from code hosts. Please [read our documentation for things to keep in mind before upgrading](https://docs.sourcegraph.com/admin/repo/permissions#background-permissions-syncing). [#10972](https://github.com/sourcegraph/sourcegraph/pull/10972)
- The styling of the hover overlay was overhauled to never have badges or the close button overlap content while also always indicating whether the overlay is currently pinned. The styling on code hosts was also improved. [#10956](https://github.com/sourcegraph/sourcegraph/pull/10956)
- Previously, it was required to quote most patterns in structural search. This is no longer a restriction and single and double quotes in structural search patterns are interpreted literally. Note: you may still use `content:"structural-pattern"` if the pattern without quotes conflicts with other syntax. [#11481](https://github.com/sourcegraph/sourcegraph/pull/11481)

### Fixed

- Dynamic repo search filters on branches which contain special characters are correctly escaped now. [#10810](https://github.com/sourcegraph/sourcegraph/pull/10810)
- Forks and archived repositories at a specific commit are searched without the need to specify "fork:yes" or "archived:yes" in the query. [#10864](https://github.com/sourcegraph/sourcegraph/pull/10864)
- The git history for binary files is now correctly shown. [#11034](https://github.com/sourcegraph/sourcegraph/pull/11034)
- Links to AWS Code Commit repositories have been fixed after the URL schema has been changed. [#11019](https://github.com/sourcegraph/sourcegraph/pull/11019)
- A link to view all repositories will now always appear on the Explore page. [#11113](https://github.com/sourcegraph/sourcegraph/pull/11113)
- The Site-admin > Pings page no longer incorrectly indicates that pings are disabled when they aren't. [#11229](https://github.com/sourcegraph/sourcegraph/pull/11229)
- Match counts are now accurately reported for indexed search. [#11242](https://github.com/sourcegraph/sourcegraph/pull/11242)
- When background permissions syncing is enabled, it is now possible to only enforce permissions for repositories from selected code hosts (instead of enforcing permissions for repositories from all code hosts). [#11336](https://github.com/sourcegraph/sourcegraph/pull/11336)
- When more than 200+ repository revisions in a search are unindexed (very rare), the remaining repositories are reported as missing instead of Sourcegraph issuing e.g. several thousand unindexed search requests which causes system slowness and ultimately times out - ensuring searches are still fast even if there are indexing issues on a deployment of Sourcegraph. This does not apply if `index:no` is present in the query.

### Removed

- Automatic syncing of Campaign webhooks for Bitbucket Server. [#10962](https://github.com/sourcegraph/sourcegraph/pull/10962)
- The `blacklist` configuration option for Gitolite is DEPRECATED and will be removed in 3.19. Use `exclude.pattern` instead.

## 3.16.2

### Fixed

- Search: fix indexed search match count [#7fc96](https://github.com/sourcegraph/sourcegraph/commit/7fc96d319f49f55da46a7649ccf261aa7e8327c3)
- Sort detected languages properly [#e7750](https://github.com/sourcegraph/sourcegraph/commit/e77507d060a40355e7b86fb093d21a7149ea03ac)

## 3.16.1

### Fixed

- Fix repo not found error for patches [#11021](https://github.com/sourcegraph/sourcegraph/pull/11021).
- Show expired license screen [#10951](https://github.com/sourcegraph/sourcegraph/pull/10951).
- Sourcegraph is now built with Go 1.14.3, fixing issues running Sourcegraph onUbuntu 19 and 20. [#10447](https://github.com/sourcegraph/sourcegraph/issues/10447)

## 3.16.0

### Added

- Autocompletion for `repogroup` filters in search queries. [#10141](https://github.com/sourcegraph/sourcegraph/pull/10286)
- If the experimental feature flag `codeInsights` is enabled, extensions can contribute content to directory pages through the experimental `ViewProvider` API. [#10236](https://github.com/sourcegraph/sourcegraph/pull/10236)
  - Directory pages are then represented as an experimental `DirectoryViewer` in the `visibleViewComponents` of the extension API. **Note: This may break extensions that were assuming `visibleViewComponents` were always `CodeEditor`s and did not check the `type` property.** Extensions checking the `type` property will continue to work. [#10236](https://github.com/sourcegraph/sourcegraph/pull/10236)
- [Major syntax highlighting improvements](https://github.com/sourcegraph/syntect_server/pull/29), including:
  - 228 commits / 1 year of improvements to the syntax highlighter library Sourcegraph uses ([syntect](https://github.com/trishume/syntect)).
  - 432 commits / 1 year of improvements to the base syntax definitions for ~36 languages Sourcegraph uses ([sublimehq/Packages](https://github.com/sublimehq/Packages)).
  - 30 new file extensions/names now detected.
  - Likely fixes other major instability and language support issues. #9557
  - Added [Smarty](#2885), [Ethereum / Solidity / Vyper)](#2440), [Cuda](#5907), [COBOL](#10154), [vb.NET](#4901), and [ASP.NET](#4262) syntax highlighting.
  - Fixed OCaml syntax highlighting #3545
  - Bazel/Starlark support improved (.star, BUILD, and many more extensions now properly highlighted). #8123
- New permissions page in both user and repository settings when background permissions syncing is enabled (`"permissions.backgroundSync": {"enabled": true}`). [#10473](https://github.com/sourcegraph/sourcegraph/pull/10473) [#10655](https://github.com/sourcegraph/sourcegraph/pull/10655)
- A new dropdown for choosing version contexts appears on the left of the query input when version contexts are specified in `experimentalFeatures.versionContext` in site configuration. Version contexts allow you to scope your search to specific sets of repos at revisions.
- Campaign changeset usage counts including changesets created, added and merged will be sent back in pings. [#10591](https://github.com/sourcegraph/sourcegraph/pull/10591)
- Diff views now feature syntax highlighting and can be properly copy-pasted. [#10437](https://github.com/sourcegraph/sourcegraph/pull/10437)
- Admins can now download an anonymized usage statistics ZIP archive in the **Site admin > Usage stats**. Opting to share this archive with the Sourcegraph team helps us make the product even better. [#10475](https://github.com/sourcegraph/sourcegraph/pull/10475)
- Extension API: There is now a field `versionContext` and subscribable `versionContextChanges` in `Workspace` to allow extensions to respect the instance's version context.
- The smart search field, providing syntax highlighting, hover tooltips, and validation on filters in search queries, is now activated by default. It can be disabled by setting `{ "experimentalFeatures": { "smartSearchField": false } }` in global settings.

### Changed

- The `userID` and `orgID` fields in the SavedSearch type in the GraphQL API have been replaced with a `namespace` field. To get the ID of the user or org that owns the saved search, use `namespace.id`. [#5327](https://github.com/sourcegraph/sourcegraph/pull/5327)
- Tree pages now redirect to blob pages if the path is not a tree and vice versa. [#10193](https://github.com/sourcegraph/sourcegraph/pull/10193)
- Files and directories that are not found now return a 404 status code. [#10193](https://github.com/sourcegraph/sourcegraph/pull/10193)
- The site admin flag `disableNonCriticalTelemetry` now allows Sourcegraph admins to disable most anonymous telemetry. Visit https://docs.sourcegraph.com/admin/pings to learn more. [#10402](https://github.com/sourcegraph/sourcegraph/pull/10402)

### Fixed

- In the OSS version of Sourcegraph, authorization providers are properly initialized and GraphQL APIs are no longer blocked. [#3487](https://github.com/sourcegraph/sourcegraph/issues/3487)
- Previously, GitLab repository paths containing certain characters could not be excluded (slashes and periods in parts of the paths). These characters are now allowed, so the repository paths can be excluded. [#10096](https://github.com/sourcegraph/sourcegraph/issues/10096)
- Symbols for indexed commits in languages Haskell, JSONNet, Kotlin, Scala, Swift, Thrift, and TypeScript will show up again. Previously our symbol indexer would not know how to extract symbols for those languages even though our unindexed symbol service did. [#10357](https://github.com/sourcegraph/sourcegraph/issues/10357)
- When periodically re-cloning a repository it will still be available. [#10663](https://github.com/sourcegraph/sourcegraph/pull/10663)

### Removed

- The deprecated feature discussions has been removed. [#9649](https://github.com/sourcegraph/sourcegraph/issues/9649)

## 3.15.2

### Fixed

- Fix repo not found error for patches [#11021](https://github.com/sourcegraph/sourcegraph/pull/11021).
- Show expired license screen [#10951](https://github.com/sourcegraph/sourcegraph/pull/10951).

## 3.15.1

### Fixed

- A potential security vulnerability with in the authentication workflow has been fixed. [#10167](https://github.com/sourcegraph/sourcegraph/pull/10167)
- An issue where `sourcegraph/postgres-11.4:3.15.0` was incorrectly an older version of the image incompatible with non-root Kubernetes deployments. `sourcegraph/postgres-11.4:3.15.1` now matches the same image version found in Sourcegraph 3.14.3 (`20-04-07_56b20163`).
- An issue that caused the search result type tabs to be overlapped in Safari. [#10191](https://github.com/sourcegraph/sourcegraph/pull/10191)

## 3.15.0

### Added

- Users and site administrators can now view a log of their actions/events in the user settings. [#9141](https://github.com/sourcegraph/sourcegraph/pull/9141)
- With the new `visibility:` filter search results can now be filtered based on a repository's visibility (possible filter values: `any`, `public` or `private`). [#8344](https://github.com/sourcegraph/sourcegraph/issues/8344)
- [`sourcegraph/git-extras`](https://sourcegraph.com/extensions/sourcegraph/git-extras) is now enabled by default on new instances [#3501](https://github.com/sourcegraph/sourcegraph/issues/3501)
- The Sourcegraph Docker image will now copy `/etc/sourcegraph/gitconfig` to `$HOME/.gitconfig`. This is a convenience similiar to what we provide for [repositories that need HTTP(S) or SSH authentication](https://docs.sourcegraph.com/admin/repo/auth). [#658](https://github.com/sourcegraph/sourcegraph/issues/658)
- Permissions background syncing is now supported for GitHub via site configuration `"permissions.backgroundSync": {"enabled": true}`. [#8890](https://github.com/sourcegraph/sourcegraph/issues/8890)
- Search: Adding `stable:true` to a query ensures a deterministic search result order. This is an experimental parameter. It applies only to file contents, and is limited to at max 5,000 results (consider using [the paginated search API](https://docs.sourcegraph.com/api/graphql/search#sourcegraph-3-9-experimental-paginated-search) if you need more than that.). [#9681](https://github.com/sourcegraph/sourcegraph/pull/9681).
- After completing the Sourcegraph user feedback survey, a button may appear for tweeting this feedback at [@srcgraph](https://twitter.com/srcgraph). [#9728](https://github.com/sourcegraph/sourcegraph/pull/9728)
- `git fetch` and `git clone` now inherit the parent process environment variables. This allows site admins to set `HTTPS_PROXY` or [git http configurations](https://git-scm.com/docs/git-config/2.26.0#Documentation/git-config.txt-httpproxy) via environment variables. For cluster environments site admins should set this on the gitserver container. [#250](https://github.com/sourcegraph/sourcegraph/issues/250)
- Experimental: Search for file contents using `and`- and `or`-expressions in queries. Enabled via the global settings value `{"experimentalFeatures": {"andOrQuery": "enabled"}}`. [#8567](https://github.com/sourcegraph/sourcegraph/issues/8567)
- Always include forks or archived repositories in searches via the global/org/user settings with `"search.includeForks": true` or `"search.includeArchived": true` respectively. [#9927](https://github.com/sourcegraph/sourcegraph/issues/9927)
- observability (debugging): It is now possible to log all Search and GraphQL requests slower than N milliseconds, using the new site configuration options `observability.logSlowGraphQLRequests` and `observability.logSlowSearches`.
- observability (monitoring): **More metrics monitored and alerted on, more legible dashboards**
  - Dashboard panels now show an orange/red background color when the defined warning/critical alert threshold has been met, making it even easier to see on a dashboard what is in a bad state.
  - Symbols: failing `symbols` -> `frontend-internal` requests are now monitored. [#9732](https://github.com/sourcegraph/sourcegraph/issues/9732)
  - Frontend dasbhoard: Search error types are now broken into distinct panels for improved visibility/legibility.
    - **IMPORTANT**: If you have previously configured alerting on any of these panels or on "hard search errors", you will need to reconfigure it after upgrading.
  - Frontend dasbhoard: Search error and latency are now broken down by type: Browser requests, search-based code intel requests, and API requests.
- observability (debugging): **Distributed tracing is a powerful tool for investigating performance issues.** The following changes have been made with the goal of making it easier to use distributed tracing with Sourcegraph:

  - The site configuration field `"observability.tracing": { "sampling": "..." }` allows a site admin to control which requests generate tracing data.
    - `"all"` will trace all requests.
    - `"selective"` (recommended) will trace all requests initiated from an end-user URL with `?trace=1`. Non-end-user-initiated requests can set a HTTP header `X-Sourcegraph-Should-Trace: true`. This is the recommended setting, as `"all"` can generate large amounts of tracing data that may cause network and memory resource contention in the Sourcegraph instance.
    - `"none"` (default) turns off tracing.
  - Jaeger is now the officially supported distributed tracer. The following is the recommended site configuration to connect Sourcegraph to a Jaeger agent (which must be deployed on the same host and listening on the default ports):

    ```
    "observability.tracing": {
      "sampling": "selective"
    }
    ```

  - Jaeger is now included in the Sourcegraph deployment configuration by default if you are using Kubernetes, Docker Compose, or the pure Docker cluster deployment model. (It is not yet included in the single Docker container distribution.) It will be included as part of upgrading to 3.15 in these deployment models, unless disabled.
  - The site configuration field, `useJaeger`, is deprecated in favor of `observability.tracing`.
  - Support for configuring Lightstep as a distributed tracer is deprecated and will be removed in a subsequent release. Instances that use Lightstep with Sourcegraph are encouraged to migrate to Jaeger (directions for running Jaeger alongside Sourcegraph are included in the installation instructions).

### Changed

- Multiple backwards-incompatible changes in the parts of the GraphQL API related to Campaigns [#9106](https://github.com/sourcegraph/sourcegraph/issues/9106):
  - `CampaignPlan.status` has been removed, since we don't need it anymore after moving execution of campaigns to src CLI in [#8008](https://github.com/sourcegraph/sourcegraph/pull/8008).
  - `CampaignPlan` has been renamed to `PatchSet`.
  - `ChangesetPlan`/`ChangesetPlanConnection` has been renamed to `Patch`/`PatchConnection`.
  - `CampaignPlanPatch` has been renamed to `PatchInput`.
  - `Campaign.plan` has been renamed to `Campaign.patchSet`.
  - `Campaign.changesetPlans` has been renamed to `campaign.changesetPlan`.
  - `createCampaignPlanFromPatches` mutation has been renamed to `createPatchSetFromPatches`.
- Removed the scoped search field on tree pages. When browsing code, the global search query will now get scoped to the current tree or file. [#9225](https://github.com/sourcegraph/sourcegraph/pull/9225)
- Instances without a license key that exceed the published user limit will now display a notice to all users.

### Fixed

- `.*` in the filter pattern were ignored and led to missing search results. [#9152](https://github.com/sourcegraph/sourcegraph/pull/9152)
- The Phabricator integration no longer makes duplicate requests to Phabricator's API on diff views. [#8849](https://github.com/sourcegraph/sourcegraph/issues/8849)
- Changesets on repositories that aren't available on the instance anymore are now hidden instead of failing. [#9656](https://github.com/sourcegraph/sourcegraph/pull/9656)
- observability (monitoring):
  - **Dashboard and alerting bug fixes**
    - Syntect Server dashboard: "Worker timeouts" can no longer appear to go negative. [#9523](https://github.com/sourcegraph/sourcegraph/issues/9523)
    - Symbols dashboard: "Store fetch queue size" can no longer appear to go negative. [#9731](https://github.com/sourcegraph/sourcegraph/issues/9731)
    - Syntect Server dashboard: "Worker timeouts" no longer incorrectly shows multiple values. [#9524](https://github.com/sourcegraph/sourcegraph/issues/9524)
    - Searcher dashboard: "Search errors on unindexed repositories" no longer includes cancelled search requests (which are expected).
    - Fixed an issue where NaN could leak into the `alert_count` metric. [#9832](https://github.com/sourcegraph/sourcegraph/issues/9832)
    - Gitserver: "resolve_revision_duration_slow" alert is no longer flaky / non-deterministic. [#9751](https://github.com/sourcegraph/sourcegraph/issues/9751)
    - Git Server dashboard: there is now a panel to show concurrent command executions to match the defined alerts. [#9354](https://github.com/sourcegraph/sourcegraph/issues/9354)
    - Git Server dashboard: adjusted the critical disk space alert to 15% so it can now fire. [#9351](https://github.com/sourcegraph/sourcegraph/issues/9351)
  - **Dashboard visiblity and legibility improvements**
    - all: "frontend internal errors" are now broken down just by route, which makes reading the graph easier. [#9668](https://github.com/sourcegraph/sourcegraph/issues/9668)
    - Frontend dashboard: panels no longer show misleading duplicate labels. [#9660](https://github.com/sourcegraph/sourcegraph/issues/9660)
    - Syntect Server dashboard: panels are no longer compacted, for improved visibility. [#9525](https://github.com/sourcegraph/sourcegraph/issues/9525)
    - Frontend dashboard: panels are no longer compacted, for improved visibility. [#9356](https://github.com/sourcegraph/sourcegraph/issues/9356)
    - Searcher dashboard: "Search errors on unindexed repositories" is now broken down by code instead of instance for improved readability. [#9670](https://github.com/sourcegraph/sourcegraph/issues/9670)
    - Symbols dashboard: metrics are now aggregated instead of per-instance, for improved visibility. [#9730](https://github.com/sourcegraph/sourcegraph/issues/9730)
    - Firing alerts are now correctly sorted at the top of dashboards by default. [#9766](https://github.com/sourcegraph/sourcegraph/issues/9766)
    - Panels at the bottom of the home dashboard no longer appear clipped / cut off. [#9768](https://github.com/sourcegraph/sourcegraph/issues/9768)
    - Git Server dashboard: disk usage now shown in percentages to match the alerts that can fire. [#9352](https://github.com/sourcegraph/sourcegraph/issues/9352)
    - Git Server dashboard: the 'echo command duration test' panel now properly displays units in seconds. [#7628](https://github.com/sourcegraph/sourcegraph/issues/7628)
    - Dashboard panels showing firing alerts no longer over-count firing alerts due to the number of service replicas. [#9353](https://github.com/sourcegraph/sourcegraph/issues/9353)

### Removed

- The experimental feature discussions is marked as deprecated. GraphQL and configuration fields related to it will be removed in 3.16. [#9649](https://github.com/sourcegraph/sourcegraph/issues/9649)

## 3.14.4

### Fixed

- A potential security vulnerability with in the authentication workflow has been fixed. [#10167](https://github.com/sourcegraph/sourcegraph/pull/10167)

## 3.14.3

### Fixed

- phabricator: Duplicate requests to phabricator API from sourcegraph extensions. [#8849](https://github.com/sourcegraph/sourcegraph/issues/8849)

## 3.14.2

### Fixed

- campaigns: Ignore changesets where repo does not exist anymore. [#9656](https://github.com/sourcegraph/sourcegraph/pull/9656)

## 3.14.1

### Added

- monitoring: new Permissions dashboard to show stats of repository permissions.

### Changed

- Site-Admin/Instrumentation in the Kubernetes cluster deployment now includes indexed-search.

## 3.14.0

### Added

- Site-Admin/Instrumentation is now available in the Kubernetes cluster deployment [8805](https://github.com/sourcegraph/sourcegraph/pull/8805).
- Extensions can now specify a `baseUri` in the `DocumentFilter` when registering providers.
- Admins can now exclude GitHub forks and/or archived repositories from the set of repositories being mirrored in Sourcegraph with the `"exclude": [{"forks": true}]` or `"exclude": [{"archived": true}]` GitHub external service configuration. [#8974](https://github.com/sourcegraph/sourcegraph/pull/8974)
- Campaign changesets can be filtered by State, Review State and Check State. [#8848](https://github.com/sourcegraph/sourcegraph/pull/8848)
- Counts of users of and searches conducted with interactive and plain text search modes will be sent back in pings, aggregated daily, weekly, and monthly.
- Aggregated counts of daily, weekly, and monthly active users of search will be sent back in pings.
- Counts of number of searches conducted using each filter will be sent back in pings, aggregated daily, weekly, and monthly.
- Counts of number of users conducting searches containing each filter will be sent back in pings, aggregated daily, weekly, and monthly.
- Added more entries (Bash, Erlang, Julia, OCaml, Scala) to the list of suggested languages for the `lang:` filter.
- Permissions background sync is now supported for GitLab and Bitbucket Server via site configuration `"permissions.backgroundSync": {"enabled": true}`.
- Indexed search exports more prometheus metrics and debug logs to aid debugging performance issues. [#9111](https://github.com/sourcegraph/sourcegraph/issues/9111)
- monitoring: the Frontend dashboard now shows in excellent detail how search is behaving overall and at a glance.
- monitoring: added alerts for when hard search errors (both timeouts and general errors) are high.
- monitoring: added alerts for when partial search timeouts are high.
- monitoring: added alerts for when search 90th and 99th percentile request duration is high.
- monitoring: added alerts for when users are being shown an abnormally large amount of search alert user suggestions and no results.
- monitoring: added alerts for when the internal indexed and unindexed search services are returning bad responses.
- monitoring: added alerts for when gitserver may be under heavy load due to many concurrent command executions or under-provisioning.

### Changed

- The "automation" feature was renamed to "campaigns".
  - `campaigns.readAccess.enabled` replaces the deprecated site configuration property `automation.readAccess.enabled`.
  - The experimental feature flag was not renamed (because it will go away soon) and remains `{"experimentalFeatures": {"automation": "enabled"}}`.
- The [Kubernetes deployment](https://github.com/sourcegraph/deploy-sourcegraph) for **existing** installations requires a
  [migration step](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/docs/migrate.md) when upgrading
  past commit [821032e2ee45f21f701](https://github.com/sourcegraph/deploy-sourcegraph/commit/821032e2ee45f21f701caac624e4f090c59fd259) or when upgrading to 3.14.
  New installations starting with the mentioned commit or with 3.14 do not need this migration step.
- Aggregated search latencies (in ms) of search queries are now included in [pings](https://docs.sourcegraph.com/admin/pings).
- The [Kubernetes deployment](https://github.com/sourcegraph/deploy-sourcegraph) frontend role has added services as a resource to watch/listen/get.
  This change does not affect the newly-introduced, restricted Kubernetes config files.
- Archived repositories are excluded from search by default. Adding `archived:yes` includes archived repositories.
- Forked repositories are excluded from search by default. Adding `fork:yes` includes forked repositories.
- CSRF and session cookies now set `SameSite=None` when Sourcegraph is running behind HTTPS and `SameSite=Lax` when Sourcegraph is running behind HTTP in order to comply with a [recent IETF proposal](https://web.dev/samesite-cookies-explained/#samesitenone-must-be-secure). As a side effect, the Sourcegraph browser extension and GitLab/Bitbucket native integrations can only connect to private instances that have HTTPS configured. If your private instance is only running behind HTTP, please configure your instance to use HTTPS in order to continue using these.
- The Bitbucket Server rate limit that Sourcegraph self-imposes has been raised from 120 req/min to 480 req/min to account for Sourcegraph instances that make use of Sourcegraphs' Bitbucket Server repository permissions and campaigns at the same time (which require a larger number of API requests against Bitbucket Server). The new number is based on Sourcegraph consuming roughly 8% the average API request rate of a large customers' Bitbucket Server instance. [#9048](https://github.com/sourcegraph/sourcegraph/pull/9048/files)
- If a single, unambiguous commit SHA is used in a search query (e.g., `repo@c98f56`) and a search index exists at this commit (i.e., it is the `HEAD` commit), then the query is searched using the index. Prior to this change, unindexed search was performed for any query containing an `@commit` specifier.

### Fixed

- Zoekt's watchdog ensures the service is down upto 3 times before exiting. The watchdog would misfire on startup on resource constrained systems, with the retries this should make a false positive far less likely. [#7867](https://github.com/sourcegraph/sourcegraph/issues/7867)
- A regression in repo-updater was fixed that lead to every repository's git clone being updated every time the list of repositories was synced from the code host. [#8501](https://github.com/sourcegraph/sourcegraph/issues/8501)
- The default timeout of indexed search has been increased. Previously indexed search would always return within 3s. This lead to broken behaviour on new instances which had yet to tune resource allocations. [#8720](https://github.com/sourcegraph/sourcegraph/pull/8720)
- Bitbucket Server older than 5.13 failed to sync since Sourcegraph 3.12. This was due to us querying for the `archived` label, but Bitbucket Server 5.13 does not support labels. [#8883](https://github.com/sourcegraph/sourcegraph/issues/8883)
- monitoring: firing alerts are now ordered at the top of the list in dashboards by default for better visibility.
- monitoring: fixed an issue where some alerts would fail to report in for the "Total alerts defined" panel in the overview dashboard.

### Removed

- The v3.11 migration to merge critical and site configuration has been removed. If you are still making use of the deprecated `CRITICAL_CONFIG_FILE`, your instance may not start up. See the [migration notes for Sourcegraph 3.11](https://docs.sourcegraph.com/admin/migration/3_11) for more information.

## 3.13.2

### Fixed

- The default timeout of indexed search has been increased. Previously indexed search would always return within 3s. This lead to broken behaviour on new instances which had yet to tune resource allocations. [#8720](https://github.com/sourcegraph/sourcegraph/pull/8720)
- Bitbucket Server older than 5.13 failed to sync since Sourcegraph 3.12. This was due to us querying for the `archived` label, but Bitbucket Server 5.13 does not support labels. [#8883](https://github.com/sourcegraph/sourcegraph/issues/8883)
- A regression in repo-updater was fixed that lead to every repository's git clone being updated every time the list of repositories was synced from the code host. [#8501](https://github.com/sourcegraph/sourcegraph/issues/8501)

## 3.13.1

### Fixed

- To reduce the chance of users running into "502 Bad Gateway" errors an internal timeout has been increased from 60 seconds to 10 minutes so that long running requests are cut short by the proxy in front of `sourcegraph-frontend` and correctly reported as "504 Gateway Timeout". [#8606](https://github.com/sourcegraph/sourcegraph/pull/8606)
- Sourcegraph instances that are not connected to the internet will no longer display errors when users submit NPS survey responses (the responses will continue to be stored locally). Rather, an error will be printed to the frontend logs. [#8598](https://github.com/sourcegraph/sourcegraph/issues/8598)
- Showing `head>` in the search results if the first line of the file is shown [#8619](https://github.com/sourcegraph/sourcegraph/issues/8619)

## 3.13.0

### Added

- Experimental: Added new field `experimentalFeatures.customGitFetch` that allows defining custom git fetch commands for code hosts and repositories with special settings. [#8435](https://github.com/sourcegraph/sourcegraph/pull/8435)
- Experimental: the search query input now provides syntax highlighting, hover tooltips, and diagnostics on filters in search queries. Requires the global settings value `{ "experimentalFeatures": { "smartSearchField": true } }`.
- Added a setting `search.hideSuggestions`, which when set to `true`, will hide search suggestions in the search bar. [#8059](https://github.com/sourcegraph/sourcegraph/pull/8059)
- Experimental: A tool, [src-expose](https://docs.sourcegraph.com/admin/external_service/other#experimental-src-expose), can be used to import code from any code host.
- Experimental: Added new field `certificates` as in `{ "experimentalFeatures" { "tls.external": { "certificates": ["<CERT>"] } } }`. This allows you to add certificates to trust when communicating with a code host (via API or git+http). We expect this to be useful for adding internal certificate authorities/self-signed certificates. [#71](https://github.com/sourcegraph/sourcegraph/issues/71)
- Added a setting `auth.minPasswordLength`, which when set, causes a minimum password length to be enforced when users sign up or change passwords. [#7521](https://github.com/sourcegraph/sourcegraph/issues/7521)
- GitHub labels associated with code change campaigns are now displayed. [#8115](https://github.com/sourcegraph/sourcegraph/pull/8115)
- GitHub labels associated with campaigns are now displayed. [#8115](https://github.com/sourcegraph/sourcegraph/pull/8115)
- When creating a campaign, users can now specify the branch name that will be used on code host. This is also a breaking change for users of the GraphQL API since the `branch` attribute is now required in `CreateCampaignInput` when a `plan` is also specified. [#7646](https://github.com/sourcegraph/sourcegraph/issues/7646)
- Added an optional `content:` parameter for specifying a search pattern. This parameter overrides any other search patterns in a query. Useful for unambiguously specifying what to search for when search strings clash with other query syntax. [#6490](https://github.com/sourcegraph/sourcegraph/issues/6490)
- Interactive search mode, which helps users construct queries using UI elements, is now made available to users by default. A dropdown to the left of the search bar allows users to toggle between interactive and plain text modes. The option to use interactive search mode can be disabled by adding `{ "experimentalFeatures": { "splitSearchModes": false } }` in global settings. [#8461](https://github.com/sourcegraph/sourcegraph/pull/8461)
- Our [upgrade policy](https://docs.sourcegraph.com/#upgrading-sourcegraph) is now enforced by the `sourcegraph-frontend` on startup to prevent admins from mistakenly jumping too many versions. [#8157](https://github.com/sourcegraph/sourcegraph/pull/8157) [#7702](https://github.com/sourcegraph/sourcegraph/issues/7702)
- Repositories with bad object packs or bad objects are automatically repaired. We now detect suspect output of git commands to mark a repository for repair. [#6676](https://github.com/sourcegraph/sourcegraph/issues/6676)
- Hover tooltips for Scala and Perl files now have syntax highlighting. [#8456](https://github.com/sourcegraph/sourcegraph/pull/8456) [#8307](https://github.com/sourcegraph/sourcegraph/issues/8307)

### Changed

- `experimentalFeatures.splitSearchModes` was removed as a site configuration option. It should be set in global/org/user settings.
- Sourcegraph now waits for `90s` instead of `5s` for Redis to be available before quitting. This duration is configurable with the new `SRC_REDIS_WAIT_FOR` environment variable.
- Code intelligence usage statistics will be sent back via pings by default. Aggregated event counts can be disabled via the site admin flag `disableNonCriticalTelemetry`.
- The Sourcegraph Docker image optimized its use of Redis to make start-up significantly faster in certain scenarios (e.g when container restarts were frequent). ([#3300](https://github.com/sourcegraph/sourcegraph/issues/3300), [#2904](https://github.com/sourcegraph/sourcegraph/issues/2904))
- Upgrading Sourcegraph is officially supported for one minor version increment (e.g., 3.12 -> 3.13). Previously, upgrades from 2 minor versions previous were supported. Please reach out to support@sourcegraph.com if you would like assistance upgrading from a much older version of Sourcegraph.
- The GraphQL mutation `previewCampaignPlan` has been renamed to `createCampaignPlan`. This mutation is part of campaigns, which is still in beta and behind a feature flag and thus subject to possible breaking changes while we still work on it.
- The GraphQL mutation `previewCampaignPlan` has been renamed to `createCampaignPlan`. This mutation is part of the campaigns feature, which is still in beta and behind a feature flag and thus subject to possible breaking changes while we still work on it.
- The GraphQL field `CampaignPlan.changesets` has been deprecated and will be removed in 3.15. A new field called `CampaignPlan.changesetPlans` has been introduced to make the naming more consistent with the `Campaign.changesetPlans` field. Please use that instead. [#7966](https://github.com/sourcegraph/sourcegraph/pull/7966)
- Long lines (>2000 bytes) are no longer highlighted, in order to prevent performance issues in browser rendering. [#6489](https://github.com/sourcegraph/sourcegraph/issues/6489)
- No longer requires `read:org` permissions for GitHub OAuth if `allowOrgs` is not enabled in the site configuration. [#8163](https://github.com/sourcegraph/sourcegraph/issues/8163)
- [Documentation](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/configure/jaeger/README.md) in github.com/sourcegraph/deploy-sourcegraph for deploying Jaeger in Kubernetes clusters running Sourcegraph has been updated to use the [Jaeger Operator](https://www.jaegertracing.io/docs/1.16/operator/), the recommended standard way of deploying Jaeger in a Kubernetes cluster. We recommend existing customers that use Jaeger adopt this new method of deployment. Please reach out to support@sourcegraph.com if you'd like assistance updating.

### Fixed

- The syntax highlighter (syntect-server) no longer fails when run in environments without IPv6 support. [#8463](https://github.com/sourcegraph/sourcegraph/pull/8463)
- After adding/removing a gitserver replica the admin interface will correctly report that repositories that need to move replicas as cloning. [#7970](https://github.com/sourcegraph/sourcegraph/issues/7970)
- Show download button for images. [#7924](https://github.com/sourcegraph/sourcegraph/issues/7924)
- gitserver backoffs trying to re-clone repositories if they fail to clone. In the case of large monorepos that failed this lead to gitserver constantly cloning them and using many resources. [#7804](https://github.com/sourcegraph/sourcegraph/issues/7804)
- It is now possible to escape spaces using `\` in the search queries when using regexp. [#7604](https://github.com/sourcegraph/sourcegraph/issues/7604)
- Clicking filter chips containing whitespace is now correctly quoted in the web UI. [#6498](https://github.com/sourcegraph/sourcegraph/issues/6498)
- **Monitoring:** Fixed an issue with the **Frontend** -> **Search responses by status** panel which caused search response types to not be aggregated as expected. [#7627](https://github.com/sourcegraph/sourcegraph/issues/7627)
- **Monitoring:** Fixed an issue with the **Replacer**, **Repo Updater**, and **Searcher** dashboards would incorrectly report on a metric from the unrelated query-runner service. [#7531](https://github.com/sourcegraph/sourcegraph/issues/7531)
- Deterministic ordering of results from indexed search. Previously when refreshing a page with many results some results may come and go.
- Spread out periodic git reclones. Previously we would reclone all git repositories every 45 days. We now add in a jitter of 12 days to spread out the load for larger installations. [#8259](https://github.com/sourcegraph/sourcegraph/issues/8259)
- Fixed an issue with missing commit information in graphql search results. [#8343](https://github.com/sourcegraph/sourcegraph/pull/8343)

### Removed

- All repository fields related to `enabled` and `disabled` have been removed from the GraphQL API. These fields have been deprecated since 3.4. [#3971](https://github.com/sourcegraph/sourcegraph/pull/3971)
- The deprecated extension API `Hover.__backcompatContents` was removed.

## 3.12.10

This release backports the fixes released in `3.13.2` for customers still on `3.12`.

### Fixed

- The default timeout of indexed search has been increased. Previously indexed search would always return within 3s. This lead to broken behaviour on new instances which had yet to tune resource allocations. [#8720](https://github.com/sourcegraph/sourcegraph/pull/8720)
- Bitbucket Server older than 5.13 failed to sync since Sourcegraph 3.12. This was due to us querying for the `archived` label, but Bitbucket Server 5.13 does not support labels. [#8883](https://github.com/sourcegraph/sourcegraph/issues/8883)
- A regression in repo-updater was fixed that lead to every repository's git clone being updated every time the list of repositories was synced from the code host. [#8501](https://github.com/sourcegraph/sourcegraph/issues/8501)

## 3.12.9

This is `3.12.8` release with internal infrastructure fixes to publish the docker images.

## 3.12.8

### Fixed

- Extension API showInputBox and other Window methods now work on search results pages [#8519](https://github.com/sourcegraph/sourcegraph/issues/8519)
- Extension error notification styling is clearer [#8521](https://github.com/sourcegraph/sourcegraph/issues/8521)

## 3.12.7

### Fixed

- Campaigns now gracefully handle GitHub review dismissals when rendering the burndown chart.

## 3.12.6

### Changed

- When GitLab permissions are turned on using GitLab OAuth authentication, GitLab project visibility is fetched in batches, which is generally more efficient than fetching them individually. The `minBatchingThreshold` and `maxBatchRequests` fields of the `authorization.identityProvider` object in the GitLab repositories configuration control when such batch fetching is used. [#8171](https://github.com/sourcegraph/sourcegraph/pull/8171)

## 3.12.5

### Fixed

- Fixed an internal race condition in our Docker build process. The previous patch version 3.12.4 contained an lsif-server version that was newer than expected. The affected artifacts have since been removed from the Docker registry.

## 3.12.4

### Added

- New optional `apiURL` configuration option for Bitbucket Cloud code host connection [#8082](https://github.com/sourcegraph/sourcegraph/pull/8082)

## 3.12.3

### Fixed

- Fixed an issue in `sourcegraph/*` Docker images where data folders were either not created or had incorrect permissions - preventing the use of Docker volumes. [#7991](https://github.com/sourcegraph/sourcegraph/pull/7991)

## 3.12.2

### Added

- Experimental: The site configuration field `campaigns.readAccess.enabled` allows site-admins to give read-only access for code change campaigns to non-site-admins. This is a setting for the experimental feature campaigns and will only have an effect when campaigns are enabled under `experimentalFeatures`. [#8013](https://github.com/sourcegraph/sourcegraph/issues/8013)

### Fixed

- A regression in 3.12.0 which caused [find-leaked-credentials campaigns](https://docs.sourcegraph.com/user/campaigns#finding-leaked-credentials) to not return any results for private repositories. [#7914](https://github.com/sourcegraph/sourcegraph/issues/7914)
- Experimental: The site configuration field `campaigns.readAccess.enabled` allows site-admins to give read-only access for campaigns to non-site-admins. This is a setting for the experimental campaigns feature and will only have an effect when campaigns is enabled under `experimentalFeatures`. [#8013](https://github.com/sourcegraph/sourcegraph/issues/8013)

### Fixed

- A regression in 3.12.0 which caused find-leaked-credentials campaigns to not return any results for private repositories. [#7914](https://github.com/sourcegraph/sourcegraph/issues/7914)
- A regression in 3.12.0 which removed the horizontal bar between search result matches.
- Manual campaigns were wrongly displayed as being in draft mode. [#8009](https://github.com/sourcegraph/sourcegraph/issues/8009)
- Manual campaigns could be published and create the wrong changesets on code hosts, even though the campaign was never in draft mode (see line above). [#8012](https://github.com/sourcegraph/sourcegraph/pull/8012)
- A regression in 3.12.0 which caused manual campaigns to not properly update the UI after adding a changeset. [#8023](https://github.com/sourcegraph/sourcegraph/pull/8023)
- Minor improvements to manual campaign form fields. [#8033](https://github.com/sourcegraph/sourcegraph/pull/8033)

## 3.12.1

### Fixed

- The ephemeral `/site-config.json` escape-hatch config file has moved to `$HOME/site-config.json`, to support non-root container environments. [#7873](https://github.com/sourcegraph/sourcegraph/issues/7873)
- Fixed an issue where repository permissions would sometimes not be cached, due to improper Redis nil value handling. [#7912](https://github.com/sourcegraph/sourcegraph/issues/7912)

## 3.12.0

### Added

- Bitbucket Server repositories with the label `archived` can be excluded from search with `archived:no` [syntax](https://docs.sourcegraph.com/code_search/reference/queries). [#5494](https://github.com/sourcegraph/sourcegraph/issues/5494)
- Add button to download file in code view. [#5478](https://github.com/sourcegraph/sourcegraph/issues/5478)
- The new `allowOrgs` site config setting in GitHub `auth.providers` enables admins to restrict GitHub logins to members of specific GitHub organizations. [#4195](https://github.com/sourcegraph/sourcegraph/issues/4195)
- Support case field in repository search. [#7671](https://github.com/sourcegraph/sourcegraph/issues/7671)
- Skip LFS content when cloning git repositories. [#7322](https://github.com/sourcegraph/sourcegraph/issues/7322)
- Hover tooltips and _Find Reference_ results now display a badge to indicate when a result is search-based. These indicators can be disabled by adding `{ "experimentalFeatures": { "showBadgeAttachments": false } }` in global settings.
- Campaigns can now be created as drafts, which can be shared and updated without creating changesets (pull requests) on code hosts. When ready, a draft can then be published, either completely or changeset by changeset, to create changesets on the code host. [#7659](https://github.com/sourcegraph/sourcegraph/pull/7659)
- Experimental: feature flag `BitbucketServerFastPerm` can be enabled to speed up fetching ACL data from Bitbucket Server instances. This requires [Bitbucket Server Sourcegraph plugin](https://github.com/sourcegraph/bitbucket-server-plugin) to be installed.
- Experimental: A site configuration field `{ "experimentalFeatures" { "tls.external": { "insecureSkipVerify": true } } }` which allows you to configure SSL/TLS settings for Sourcegraph contacting your code hosts. Currently just supports turning off TLS/SSL verification. [#71](https://github.com/sourcegraph/sourcegraph/issues/71)
- Experimental: To search across multiple revisions of the same repository, list multiple branch names (or other revspecs) separated by `:` in your query, as in `repo:myrepo@branch1:branch2:branch2`. To search all branches, use `repo:myrepo@*refs/heads/`. Requires the site configuration value `{ "experimentalFeatures": { "searchMultipleRevisionsPerRepository": true } }`. Previously this was only supported for diff and commit searches.
- Experimental: interactive search mode, which helps users construct queries using UI elements. Requires the site configuration value `{ "experimentalFeatures": { "splitSearchModes": true } }`. The existing plain text search format is still available via the dropdown menu on the left of the search bar.
- A case sensitivity toggle now appears in the search bar.
- Add explicit repository permissions support with site configuration field `{ "permissions.userMapping" { "enabled": true, "bindID": "email" } }`.

### Changed

- The "Files" tab in the search results page has been renamed to "Filenames" for clarity.
- The search query builder now lives on its own page at `/search/query-builder`. The home search page has a link to it.
- User passwords when using builtin auth are limited to 256 characters. Existing passwords longer than 256 characters will continue to work.
- GraphQL API: Campaign.changesetCreationStatus has been renamed to Campaign.status to be aligned with CampaignPlan. [#7654](https://github.com/sourcegraph/sourcegraph/pull/7654)
- When using GitHub as an authentication provider, `read:org` scope is now required. This is used to support the new `allowOrgs` site config setting in the GitHub `auth.providers` configuration, which enables site admins to restrict GitHub logins to members of a specific GitHub organization. This for example allows having a Sourcegraph instance with GitHub sign in configured be exposed to the public internet without allowing everyone with a GitHub account access to your Sourcegraph instance.

### Fixed

- The experimental search pagination API no longer times out when large repositories are encountered. [#6384](https://github.com/sourcegraph/sourcegraph/issues/6384)
- We resolve relative symbolic links from the directory of the symlink, rather than the root of the repository. [#6034](https://github.com/sourcegraph/sourcegraph/issues/6034)
- Show errors on repository settings page when repo-updater is down. [#3593](https://github.com/sourcegraph/sourcegraph/issues/3593)
- Remove benign warning that verifying config took more than 10s when updating or saving an external service. [#7176](https://github.com/sourcegraph/sourcegraph/issues/7176)
- repohasfile search filter works again (regressed in 3.10). [#7380](https://github.com/sourcegraph/sourcegraph/issues/7380)
- Structural search can now run on very large repositories containing any number of files. [#7133](https://github.com/sourcegraph/sourcegraph/issues/7133)

### Removed

- The deprecated GraphQL mutation `setAllRepositoriesEnabled` has been removed. [#7478](https://github.com/sourcegraph/sourcegraph/pull/7478)
- The deprecated GraphQL mutation `deleteRepository` has been removed. [#7483](https://github.com/sourcegraph/sourcegraph/pull/7483)

## 3.11.4

### Fixed

- The `/.auth/saml/metadata` endpoint has been fixed. Previously it panicked if no encryption key was set.
- The version updating logic has been fixed for `sourcegraph/server`. Users running `sourcegraph/server:3.11.1` will need to manually modify their `docker run` command to use `sourcegraph/server:3.11.4` or higher. [#7442](https://github.com/sourcegraph/sourcegraph/issues/7442)

## 3.11.1

### Fixed

- The syncing process for newly created campaign changesets has been fixed again after they have erroneously been marked as deleted in the database. [#7522](https://github.com/sourcegraph/sourcegraph/pull/7522)
- The syncing process for newly created changesets (in campaigns) has been fixed again after they have erroneously been marked as deleted in the database. [#7522](https://github.com/sourcegraph/sourcegraph/pull/7522)

## 3.11.0

**Important:** If you use `SITE_CONFIG_FILE` or `CRITICAL_CONFIG_FILE`, please be sure to follow the steps in: [migration notes for Sourcegraph v3.11+](doc/admin/migration/3_11.md) after upgrading.

### Added

- Language statistics by commit are available via the API. [#6737](https://github.com/sourcegraph/sourcegraph/pull/6737)
- Added a new page that shows [language statistics for the results of a search query](https://docs.sourcegraph.com/user/search#statistics).
- Global settings can be configured from a local file using the environment variable `GLOBAL_SETTINGS_FILE`.
- High-level health metrics and dashboards have been added to Sourcegraph's monitoring (found under the **Site admin** -> **Monitoring** area). [#7216](https://github.com/sourcegraph/sourcegraph/pull/7216)
- Logging for GraphQL API requests not issued by Sourcegraph is now much more verbose, allowing for easier debugging of problematic queries and where they originate from. [#5706](https://github.com/sourcegraph/sourcegraph/issues/5706)
- A new campaign type finds and removes leaked NPM credentials. [#6893](https://github.com/sourcegraph/sourcegraph/pull/6893)
- Campaigns can now be retried to create failed changesets due to ephemeral errors (e.g. network problems when creating a pull request on GitHub). [#6718](https://github.com/sourcegraph/sourcegraph/issues/6718)
- The initial release of [structural code search](https://docs.sourcegraph.com/code_search/reference/structural).

### Changed

- `repohascommitafter:` search filter uses a more efficient git command to determine inclusion. [#6739](https://github.com/sourcegraph/sourcegraph/pull/6739)
- `NODE_NAME` can be specified instead of `HOSTNAME` for zoekt-indexserver. `HOSTNAME` was a confusing configuration to use in [Pure-Docker Sourcegraph deployments](https://github.com/sourcegraph/deploy-sourcegraph-docker). [#6846](https://github.com/sourcegraph/sourcegraph/issues/6846)
- The feedback toast now requests feedback every 60 days of usage (was previously only once on the 3rd day of use). [#7165](https://github.com/sourcegraph/sourcegraph/pull/7165)
- The lsif-server container now only has a dependency on Postgres, whereas before it also relied on Redis. [#6880](https://github.com/sourcegraph/sourcegraph/pull/6880)
- Renamed the GraphQL API `LanguageStatistics` fields to `name`, `totalBytes`, and `totalLines` (previously the field names started with an uppercase letter, which was inconsistent).
- Detecting a file's language uses a more accurate but slower algorithm. To revert to the old (faster and less accurate) algorithm, set the `USE_ENHANCED_LANGUAGE_DETECTION` env var to the string `false` (on the `sourcegraph/server` container, or if using the cluster deployment, on the `sourcegraph-frontend` pod).
- Diff and commit searches that make use of `before:` and `after:` filters to narrow their search area are now no longer subject to the 50-repository limit. This allows for creating saved searches on more than 50 repositories as before. [#7215](https://github.com/sourcegraph/sourcegraph/issues/7215)

### Fixed

- Changes to external service configurations are reflected much faster. [#6058](https://github.com/sourcegraph/sourcegraph/issues/6058)
- Deleting an external service will not show warnings for the non-existent service. [#5617](https://github.com/sourcegraph/sourcegraph/issues/5617)
- Suggested search filter chips are quoted if necessary. [#6498](https://github.com/sourcegraph/sourcegraph/issues/6498)
- Remove potential panic in gitserver if heavily loaded. [#6710](https://github.com/sourcegraph/sourcegraph/issues/6710)
- Multiple fixes to make the preview and creation of campaigns more robust and a smoother user experience. [#6682](https://github.com/sourcegraph/sourcegraph/pull/6682) [#6625](https://github.com/sourcegraph/sourcegraph/issues/6625) [#6658](https://github.com/sourcegraph/sourcegraph/issues/6658) [#7088](https://github.com/sourcegraph/sourcegraph/issues/7088) [#6766](https://github.com/sourcegraph/sourcegraph/issues/6766) [#6717](https://github.com/sourcegraph/sourcegraph/issues/6717) [#6659](https://github.com/sourcegraph/sourcegraph/issues/6659)
- Repositories referenced in campaigns that are removed in an external service configuration change won't lead to problems with the syncing process anymore. [#7015](https://github.com/sourcegraph/sourcegraph/pull/7015)
- The Searcher dashboard (and the `src_graphql_search_response` Prometheus metric) now properly account for search alerts instead of them being incorrectly added to the `timeout` category. [#7214](https://github.com/sourcegraph/sourcegraph/issues/7214)
- In the experimental search pagination API, the `cloning`, `missing`, and other repository fields now return a well-defined set of results. [#6000](https://github.com/sourcegraph/sourcegraph/issues/6000)

### Removed

- The management console has been removed. All critical configuration previously stored in the management console will be automatically migrated to your site configuration. For more information about this change, or if you use `SITE_CONFIG_FILE` / `CRITICAL_CONFIG_FILE`, please see the [migration notes for Sourcegraph v3.11+](doc/admin/migration/3_11.md).

## 3.10.4

### Fixed

- An issue where diff/commit searches that would run over more than 50 repositories would incorrectly display a timeout error instead of the correct error suggesting users scope their query to less repositories. [#7090](https://github.com/sourcegraph/sourcegraph/issues/7090)

## 3.10.3

### Fixed

- A critical regression in 3.10.2 which caused diff, commit, and repository searches to timeout. [#7090](https://github.com/sourcegraph/sourcegraph/issues/7090)
- A critical regression in 3.10.2 which caused "No results" to appear frequently on pages with search results. [#7095](https://github.com/sourcegraph/sourcegraph/pull/7095)
- An issue where the built-in Grafana Searcher dashboard would show duplicate success/error metrics. [#7078](https://github.com/sourcegraph/sourcegraph/pull/7078)

## 3.10.2

### Added

- Site admins can now use the built-in Grafana Searcher dashboard to observe how many search requests are successful, or resulting in errors or timeouts. [#6756](https://github.com/sourcegraph/sourcegraph/issues/6756)

### Fixed

- When searches timeout, a consistent UI with clear actions like a button to increase the timeout is now returned. [#6754](https://github.com/sourcegraph/sourcegraph/issues/6754)
- To reduce the chance of search timeouts in some cases, the default indexed search timeout has been raised from 1.5s to 3s. [#6754](https://github.com/sourcegraph/sourcegraph/issues/6754)
- We now correctly inform users of the limitations of diff/commit search. If a diff/commit search would run over more than 50 repositories, users will be shown an error suggesting they scope their search to less repositories using the `repo:` filter. Global diff/commit search support is being tracked in [#6826](https://github.com/sourcegraph/sourcegraph/issues/6826). [#5519](https://github.com/sourcegraph/sourcegraph/issues/5519)

## 3.10.1

### Added

- Syntax highlighting for Starlark (Bazel) files. [#6827](https://github.com/sourcegraph/sourcegraph/issues/6827)

### Fixed

- The experimental search pagination API no longer times out when large repositories are encountered. [#6384](https://github.com/sourcegraph/sourcegraph/issues/6384) [#6383](https://github.com/sourcegraph/sourcegraph/issues/6383)
- In single-container deployments, the builtin `postgres_exporter` now correctly respects externally configured databases. This previously caused PostgreSQL metrics to not show up in Grafana when an external DB was in use. [#6735](https://github.com/sourcegraph/sourcegraph/issues/6735)

## 3.10.0

### Added

- Indexed Search supports horizontally scaling. Instances with large number of repositories can update the `replica` field of the `indexed-search` StatefulSet. See [configure indexed-search replica count](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/docs/configure.md#configure-indexed-search-replica-count). [#5725](https://github.com/sourcegraph/sourcegraph/issues/5725)
- Bitbucket Cloud external service supports `exclude` config option. [#6035](https://github.com/sourcegraph/sourcegraph/issues/6035)
- `sourcegraph/server` Docker deployments now support the environment variable `IGNORE_PROCESS_DEATH`. If set to true the container will keep running, even if a subprocess has died. This is useful when manually fixing problems in the container which the container refuses to start. For example a bad database migration.
- Search input now offers filter type suggestions [#6105](https://github.com/sourcegraph/sourcegraph/pull/6105).
- The keyboard shortcut <kbd>Ctrl</kbd>+<kbd>Space</kbd> in the search input shows a list of available filter types.
- Sourcegraph Kubernetes cluster site admins can configure PostgreSQL by specifying `postgresql.conf` via ConfigMap. [sourcegraph/deploy-sourcegraph#447](https://github.com/sourcegraph/deploy-sourcegraph/pull/447)

### Changed

- **Required Kubernetes Migration:** The [Kubernetes deployment](https://github.com/sourcegraph/deploy-sourcegraph) manifest for indexed-search services has changed from a Normal Service to a Headless Service. This is to enable Sourcegraph to individually resolve indexed-search pods. Services are immutable, so please follow the [migration guide](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/docs/migrate.md#310).
- Fields of type `String` in our GraphQL API that contain [JSONC](https://komkom.github.io/) now have the custom scalar type `JSONCString`. [#6209](https://github.com/sourcegraph/sourcegraph/pull/6209)
- `ZOEKT_HOST` environment variable has been deprecated. Please use `INDEXED_SEARCH_SERVERS` instead. `ZOEKT_HOST` will be removed in 3.12.
- Directory names on the repository tree page are now shown in bold to improve readability.
- Added support for Bitbucket Server pull request activity to the [campaign](https://about.sourcegraph.com/product/code-change-management/) burndown chart. When used, this feature leads to more requests being sent to Bitbucket Server, since Sourcegraph needs to keep track of how a pull request's state changes over time. With [the instance scoped webhooks](https://docs.google.com/document/d/1I3Aq1WSUh42BP8KvKr6AlmuCfo8tXYtJu40WzdNT6go/edit) in our [Bitbucket Server plugin](https://github.com/sourcegraph/bitbucket-server-plugin/pull/10) as well as up-coming [heuristical syncing changes](#6389), this additional load will be significantly reduced in the future.
- Added support for Bitbucket Server pull request activity to the campaign burndown chart. When used, this feature leads to more requests being sent to Bitbucket Server, since Sourcegraph needs to keep track of how a pull request's state changes over time. With [the instance scoped webhooks](https://docs.google.com/document/d/1I3Aq1WSUh42BP8KvKr6AlmuCfo8tXYtJu40WzdNT6go/edit) in our [Bitbucket Server plugin](https://github.com/sourcegraph/bitbucket-server-plugin/pull/10) as well as up-coming [heuristical syncing changes](#6389), this additional load will be significantly reduced in the future.

### Fixed

- Support hyphens in Bitbucket Cloud team names. [#6154](https://github.com/sourcegraph/sourcegraph/issues/6154)
- Server will run `redis-check-aof --fix` on startup to fix corrupted AOF files. [#651](https://github.com/sourcegraph/sourcegraph/issues/651)
- Authorization provider configuration errors in external services will be shown as site alerts. [#6061](https://github.com/sourcegraph/sourcegraph/issues/6061)

### Removed

## 3.9.4

### Changed

- The experimental search pagination API's `PageInfo` object now returns a `String` instead of an `ID` for its `endCursor`, and likewise for the `after` search field. Experimental paginated search API users may need to update their usages to replace `ID` cursor types with `String` ones.

### Fixed

- The experimental search pagination API no longer omits a single repository worth of results at the end of the result set. [#6286](https://github.com/sourcegraph/sourcegraph/issues/6286)
- The experimental search pagination API no longer produces search cursors that can get "stuck". [#6287](https://github.com/sourcegraph/sourcegraph/issues/6287)
- In literal search mode, searching for quoted strings now works as expected. [#6255](https://github.com/sourcegraph/sourcegraph/issues/6255)
- In literal search mode, quoted field values now work as expected. [#6271](https://github.com/sourcegraph/sourcegraph/pull/6271)
- `type:path` search queries now correctly work in indexed search again. [#6220](https://github.com/sourcegraph/sourcegraph/issues/6220)

## 3.9.3

### Changed

- Sourcegraph is now built using Go 1.13.3 [#6200](https://github.com/sourcegraph/sourcegraph/pull/6200).

## 3.9.2

### Fixed

- URI-decode the username, password, and pathname when constructing Postgres connection paramers in lsif-server [#6174](https://github.com/sourcegraph/sourcegraph/pull/6174). Fixes a crashing lsif-server process for users with passwords containing special characters.

## 3.9.1

### Changed

- Reverted [#6094](https://github.com/sourcegraph/sourcegraph/pull/6094) because it introduced a minor security hole involving only Grafana.
  [#6075](https://github.com/sourcegraph/sourcegraph/issues/6075) will be fixed with a different approach.

## 3.9.0

### Added

- Our external service syncing model will stream in new repositories to Sourcegraph. Previously we could only add a repository to our database and clone it once we had synced all information from all external services (to detect deletions and renames). Now adding a repository to an external service configuration should be reflected much sooner, even on large instances. [#5145](https://github.com/sourcegraph/sourcegraph/issues/5145)
- There is now an easy way for site admins to view and export settings and configuration when reporting a bug. The page for doing so is at /site-admin/report-bug, linked to from the site admin side panel under "Report a bug".
- An experimental search pagination API to enable better programmatic consumption of search results is now available to try. For more details and known limitations see [the documentation](https://docs.sourcegraph.com/api/graphql/search).
- Search queries can now be interpreted literally.
  - There is now a dot-star icon in the search input bar to toggle the pattern type of a query between regexp and literal.
  - There is a new `search.defaultPatternType` setting to configure the default pattern type, regexp or literal, for searches.
  - There is a new `patternType:` search token which overrides the `search.defaultPatternType` setting, and the active state of the dot-star icon in determining the pattern type of the query.
  - Old URLs without a patternType URL parameter will be redirected to the same URL with
    patternType=regexp appended to preserve intended behavior.
- Added support for GitHub organization webhooks to enable faster updates of metadata used by [campaigns](https://about.sourcegraph.com/product/code-change-management/), such as pull requests or issue comments. See the [GitHub webhook documentation](https://docs.sourcegraph.com/admin/external_service/github#webhooks) for instructions on how to enable webhooks.
- Added support for GitHub organization webhooks to enable faster updates of changeset metadata used by campaigns. See the [GitHub webhook documentation](https://docs.sourcegraph.com/admin/external_service/github#webhooks) for instructions on how to enable webhooks.
- Added burndown chart to visualize progress of campaigns.
- Added ability to edit campaign titles and descriptions.

### Changed

- **Recommended Kubernetes Migration:** The [Kubernetes deployment](https://github.com/sourcegraph/deploy-sourcegraph) manifest for indexed-search pods has changed from a Deployment to a StatefulSet. This is to enable future work on horizontally scaling indexed search. To retain your existing indexes there is a [migration guide](https://github.com/sourcegraph/deploy-sourcegraph/blob/master/docs/migrate.md#39).
- Allow single trailing hyphen in usernames and org names [#5680](https://github.com/sourcegraph/sourcegraph/pull/5680)
- Indexed search won't spam the logs on startup if the frontend API is not yet available. [zoekt#30](https://github.com/sourcegraph/zoekt/pull/30), [#5866](https://github.com/sourcegraph/sourcegraph/pull/5866)
- Search query fields are now case insensitive. For example `repoHasFile:` will now be recognized, not just `repohasfile:`. [#5168](https://github.com/sourcegraph/sourcegraph/issues/5168)
- Search queries are now interpreted literally by default, rather than as regular expressions. [#5899](https://github.com/sourcegraph/sourcegraph/pull/5899)
- The `search` GraphQL API field now takes a two new optional parameters: `version` and `patternType`. `version` determines the search syntax version to use, and `patternType` determines the pattern type to use for the query. `version` defaults to "V1", which is regular expression searches by default, if not explicitly passed in. `patternType` overrides the pattern type determined by version.
- Saved searches have been updated to support the new patternType filter. All existing saved searches have been updated to append `patternType:regexp` to the end of queries to ensure deterministic results regardless of the patternType configurations on an instance. All new saved searches are required to have a `patternType:` field in the query.
- Allow text selection in search result headers (to allow for e.g. copying filenames)

### Fixed

- Web app: Fix paths with special characters (#6050)
- Fixed an issue that rendered the search filter `repohascommitafter` unusable in the presence of an empty repository. [#5149](https://github.com/sourcegraph/sourcegraph/issues/5149)
- An issue where `externalURL` not being configured in the management console could go unnoticed. [#3899](https://github.com/sourcegraph/sourcegraph/issues/3899)
- Listing branches and refs now falls back to a fast path if there are a large number of branches. Previously we would time out. [#4581](https://github.com/sourcegraph/sourcegraph/issues/4581)
- Sourcegraph will now ignore the ambiguous ref HEAD if a repository contains it. [#5291](https://github.com/sourcegraph/sourcegraph/issues/5291)

### Removed

## 3.8.2

### Fixed

- Sourcegraph cluster deployments now run a more stable syntax highlighting server which can self-recover from rarer failure cases such as getting stuck at high CPU usage when highlighting some specific files. [#5406](https://github.com/sourcegraph/sourcegraph/issues/5406) This will be ported to single-container deployments [at a later date](https://github.com/sourcegraph/sourcegraph/issues/5841).

## 3.8.1

### Added

- Add `nameTransformations` setting to GitLab external service to help transform repository name that shows up in the Sourcegraph UI.

## 3.8.0

### Added

- A toggle button for browser extension to quickly enable/disable the core functionality without actually enable/disable the entire extension in the browser extension manager.
- Tabs to easily toggle between the different search result types on the search results page.

### Changed

- A `hardTTL` setting was added to the [Bitbucket Server `authorization` config](https://docs.sourcegraph.com/admin/external_service/bitbucketserver#configuration). This setting specifies a duration after which a user's cached permissions must be updated before any user action is authorized. This contrasts with the already existing `ttl` setting which defines a duration after which a user's cached permissions will get updated in the background, but the previously cached (and now stale) permissions are used to authorize any user action occuring before the update concludes. If your previous `ttl` value is larger than the default of the new `hardTTL` setting (i.e. **3 days**), you must change the `ttl` to be smaller or, `hardTTL` to be larger.

### Fixed

### Removed

- The `statusIndicator` feature flag has been removed from the site configuration's `experimentalFeatures` section. The status indicator has been enabled by default since 3.6.0 and you can now safely remove the feature flag from your configuration.
- Public usage is now only available on Sourcegraph.com. Because many core features rely on persisted user settings, anonymous usage leads to a degraded experience for most users. As a result, for self-hosted private instances it is preferable for all users to have accounts. But on sourcegraph.com, users will continue to have to opt-in to accounts, despite the degraded UX.

## 3.7.2

### Added

- A [migration guide for Sourcegraph v3.7+](doc/admin/migration/3_7.md).

### Fixed

- Fixed an issue where some repositories with very long symbol names would fail to index after v3.7.
- We now retain one prior search index version after an upgrade, meaning upgrading AND downgrading from v3.6.2 <-> v3.7.2 is now 100% seamless and involves no downtime or negated search performance while repositories reindex. Please refer to the [v3.7+ migration guide](doc/admin/migration/3_7.md) for details.

## 3.7.1

### Fixed

- When re-indexing repositories, we now continue to serve from the old index in the meantime. Thus, you can upgrade to 3.7.1 without downtime.
- Indexed symbol search is now faster, as we've fixed a performance issue that occurred when many repositories without any symbols existed.
- Indexed symbol search now uses less disk space when upgrading directly to v3.7.1 as we properly remove old indexes.

## 3.7.0

### Added

- Indexed search now supports symbol queries. This feature will require re-indexing all repositories. This will increase the disk and memory usage of indexed search by roughly 10%. You can disable the feature with the configuration `search.index.symbols.enabled`. [#3534](https://github.com/sourcegraph/sourcegraph/issues/3534)
- Multi-line search now works for non-indexed search. [#4518](https://github.com/sourcegraph/sourcegraph/issues/4518)
- When using `SITE_CONFIG_FILE` and `EXTSVC_CONFIG_FILE`, you [may now also specify e.g. `SITE_CONFIG_ALLOW_EDITS=true`](https://docs.sourcegraph.com/admin/config/advanced_config_file) to allow edits to be made to the config in the application which will be overwritten on the next process restart. [#4912](https://github.com/sourcegraph/sourcegraph/issues/4912)

### Changed

- In the [GitHub external service config](https://docs.sourcegraph.com/admin/external_service/github#configuration) it's now possible to specify `orgs` without specifying `repositoryQuery` or `repos` too.
- Out-of-the-box TypeScript code intelligence is much better with an updated ctags version with a built-in TypeScript parser.
- Sourcegraph uses Git protocol version 2 for increased efficiency and performance when fetching data from compatible code hosts.
- Searches with `repohasfile:` are faster at finding repository matches. [#4833](https://github.com/sourcegraph/sourcegraph/issues/4833).
- Zoekt now runs with GOGC=50 by default, helping to reduce the memory consumption of Sourcegraph. [#3792](https://github.com/sourcegraph/sourcegraph/issues/3792)
- Upgraded the version of Go in use, which improves security for publicly accessible Sourcegraph instances.

### Fixed

- Disk cleanup in gitserver is now done in terms of percentages to fix [#5059](https://github.com/sourcegraph/sourcegraph/issues/5059).
- Search results now correctly show highlighting of matches with runes like 'İ' that lowercase to runes with a different number of bytes in UTF-8 [#4791](https://github.com/sourcegraph/sourcegraph/issues/4791).
- Fixed an issue where search would sometimes crash with a panic due to a nil pointer. [#5246](https://github.com/sourcegraph/sourcegraph/issues/5246)

### Removed

## 3.6.2

### Fixed

- Fixed Phabricator external services so they won't stop the syncing process for repositories when Phabricator doesn't return clone URLs. [#5101](https://github.com/sourcegraph/sourcegraph/pull/5101)

## 3.6.1

### Added

- New site config option `branding.brandName` configures the brand name to display in the Sourcegraph \<title\> element.
- `repositoryPathPattern` option added to the "Other" external service type for repository name customization.

## 3.6.0

### Added

- The `github.exclude` setting in [GitHub external service config](https://docs.sourcegraph.com/admin/external_service/github#configuration) additionally allows you to specify regular expressions with `{"pattern": "regex"}`.
- A new [`quicklinks` setting](https://docs.sourcegraph.com/user/personalization/quick_links) allows adding links to be displayed on the homepage and search page for all users (or users in an organization).
- Compatibility with the [Sourcegraph for Bitbucket Server](https://github.com/sourcegraph/bitbucket-server-plugin) plugin.
- Support for [Bitbucket Cloud](https://bitbucket.org) as an external service.

### Changed

- Updating or creating an external service will no longer block until the service is synced.
- The GraphQL fields `Repository.createdAt` and `Repository.updatedAt` are deprecated and will be removed in 3.8. Now `createdAt` is always the current time and updatedAt is always null.
- In the [GitHub external service config](https://docs.sourcegraph.com/admin/external_service/github#configuration) and [Bitbucket Server external service config](https://docs.sourcegraph.com/admin/external_service/bitbucket_server#permissions) `repositoryQuery` is now only required if `repos` is not set.
- Log messages from query-runner when saved searches fail now include the raw query as part of the message.
- The status indicator in the navigation bar is now enabled by default
- Usernames and org names can now contain the `.` character. [#4674](https://github.com/sourcegraph/sourcegraph/issues/4674)

### Fixed

- Commit searches now correctly highlight unicode characters, for example 加. [#4512](https://github.com/sourcegraph/sourcegraph/issues/4512)
- Symbol searches now show the number of symbol matches rather than the number of file matches found. [#4578](https://github.com/sourcegraph/sourcegraph/issues/4578)
- Symbol searches with truncated results now show a `+` on the results page to signal that some results have been omitted. [#4579](https://github.com/sourcegraph/sourcegraph/issues/4579)

## 3.5.4

### Fixed

- Fixed Phabricator external services so they won't stop the syncing process for repositories when Phabricator doesn't return clone URLs. [#5101](https://github.com/sourcegraph/sourcegraph/pull/5101)

## 3.5.2

### Changed

- Usernames and org names can now contain the `.` character. [#4674](https://github.com/sourcegraph/sourcegraph/issues/4674)

### Added

- Syntax highlighting requests that fail are now logged and traced. A new Prometheus metric `src_syntax_highlighting_requests` allows monitoring and alerting. [#4877](https://github.com/sourcegraph/sourcegraph/issues/4877).
- Sourcegraph's SAML authentication now supports RSA PKCS#1 v1.5. [#4869](https://github.com/sourcegraph/sourcegraph/pull/4869)

### Fixed

- Increased nginx proxy buffer size to fix issue where login failed when SAML AuthnRequest was too large. [#4849](https://github.com/sourcegraph/sourcegraph/pull/4849)
- A regression in 3.3.8 where `"corsOrigin": "*"` was improperly forbidden. [#4424](https://github.com/sourcegraph/sourcegraph/issues/4424)

## 3.5.1

### Added

- A new [`quicklinks` setting](https://docs.sourcegraph.com/user/personalization/quick_links) allows adding links to be displayed on the homepage and search page for all users (or users in an organization).
- Site admins can prevent the icon in the top-left corner of the screen from spinning on hovers by setting `"branding": { "disableSymbolSpin": true }` in their site configuration.

### Fixed

- Fix `repository.language` GraphQL field (previously returned empty for most repositories).

## 3.5.0

### Added

- Indexed search now supports matching consecutive literal newlines, with queries like e.g. `foo\nbar.*` to search over multiple lines. [#4138](https://github.com/sourcegraph/sourcegraph/issues/4138)
- The `orgs` setting in [GitHub external service config](https://docs.sourcegraph.com/admin/external_service/github) allows admins to select all repositories from the specified organizations to be synced.
- A new experimental search filter `repohascommitafter:"30 days ago"` allows users to exclude stale repositories that don't contain commits (to the branch being searched over) past a specified date from their search query.
- The `authorization` setting in the [Bitbucket Server external service config](https://docs.sourcegraph.com/admin/external_service/bitbucket_server#permissions) enables Sourcegraph to enforce the repository permissions defined in Bitbucket Server.
- A new, experimental status indicator in the navigation bar allows admins to quickly see whether the configured repositories are up to date or how many are currently being updated in the background. You can enable the status indicator with the following site configuration: `"experimentalFeatures": { "statusIndicator": "enabled" }`.
- A new search filter `repohasfile` allows users to filter results to just repositories containing a matching file. For example `ubuntu file:Dockerfile repohasfile:\.py$` would find Dockerfiles mentioning Ubuntu in repositories that contain Python files. [#4501](https://github.com/sourcegraph/sourcegraph/pull/4501)

### Changed

- The saved searches UI has changed. There is now a Saved searches page in the user and organizations settings area. A saved search appears in the settings area of the user or organization it is associated with.

### Removed

### Fixed

- Fixed repository search patterns which contain `.*`. Previously our optimizer would ignore `.*`, which in some cases would lead to our repository search excluding some repositories from the results.
- Fixed an issue where the Phabricator native integration would be broken on recent Phabricator versions. This fix depends on v1.2 of the [Phabricator extension](https://github.com/sourcegraph/phabricator-extension).
- Fixed an issue where the "Empty repository" banner would be shown on a repository page when starting to clone a repository.
- Prevent data inconsistency on cached archives due to restarts. [#4366](https://github.com/sourcegraph/sourcegraph/pull/4366)
- On the /extensions page, the UI is now less ambiguous when an extension has not been activated. [#4446](https://github.com/sourcegraph/sourcegraph/issues/4446)

## 3.4.5

### Fixed

- Fixed an issue where syntax highlighting taking too long would result in errors or wait long amounts of time without properly falling back to plaintext rendering after a few seconds. [#4267](https://github.com/sourcegraph/sourcegraph/issues/4267) [#4268](https://github.com/sourcegraph/sourcegraph/issues/4268) (this fix was intended to be in 3.4.3, but was in fact left out by accident)
- Fixed an issue with `sourcegraph/server` Docker deployments where syntax highlighting could produce `server closed idle connection` errors. [#4269](https://github.com/sourcegraph/sourcegraph/issues/4269) (this fix was intended to be in 3.4.3, but was in fact left out by accident)
- Fix `repository.language` GraphQL field (previously returned empty for most repositories).

## 3.4.4

### Fixed

- Fixed an out of bounds error in the GraphQL repository query. [#4426](https://github.com/sourcegraph/sourcegraph/issues/4426)

## 3.4.3

### Fixed

- Improved performance of the /site-admin/repositories page significantly (prevents timeouts). [#4063](https://github.com/sourcegraph/sourcegraph/issues/4063)
- Fixed an issue where Gitolite repositories would be inaccessible to non-admin users after upgrading to 3.3.0+ from an older version. [#4263](https://github.com/sourcegraph/sourcegraph/issues/4263)
- Repository names are now treated as case-sensitive, fixing an issue where users saw `pq: duplicate key value violates unique constraint \"repo_name_unique\"` [#4283](https://github.com/sourcegraph/sourcegraph/issues/4283)
- Repositories containing submodules not on Sourcegraph will now load without error [#2947](https://github.com/sourcegraph/sourcegraph/issues/2947)
- HTTP metrics in Prometheus/Grafana now distinguish between different types of GraphQL requests.

## 3.4.2

### Fixed

- Fixed incorrect wording in site-admin onboarding. [#4127](https://github.com/sourcegraph/sourcegraph/issues/4127)

## 3.4.1

### Added

- You may now specify `DISABLE_CONFIG_UPDATES=true` on the management console to prevent updates to the critical configuration. This is useful when loading critical config via a file using `CRITICAL_CONFIG_FILE` on the frontend.

### Changed

- When `EXTSVC_CONFIG_FILE` or `SITE_CONFIG_FILE` are specified, updates to external services and the site config are now prevented.
- Site admins will now see a warning if creating or updating an external service was successful but the process could not complete entirely due to an ephemeral error (such as GitHub API search queries running into timeouts and returning incomplete results).

### Removed

### Fixed

- Fixed an issue where `EXTSVC_CONFIG_FILE` being specified would incorrectly cause a panic.
- Fixed an issue where user/org/global settings from old Sourcegraph versions (2.x) could incorrectly be null, leading to various errors.
- Fixed an issue where an ephemeral infrastructure error (`tar/archive: invalid tar header`) would fail a search.

## 3.4.0

### Added

- When `repositoryPathPattern` is configured, paths from the full long name will redirect to the configured name. Extensions will function with the configured name. `repositoryPathPattern` allows administrators to configure "nice names". For example `sourcegraph.example.com/github.com/foo/bar` can configured to be `sourcegraph.example.com/gh/foo/bar` with `"repositoryPathPattern": "gh/{nameWithOwner}"`. (#462)
- Admins can now turn off site alerts for patch version release updates using the `alerts.showPatchUpdates` setting. Alerts will still be shown for major and minor version updates.
- The new `gitolite.exclude` setting in [Gitolite external service config](https://docs.sourcegraph.com/admin/external_service/gitolite#configuration) allows you to exclude specific repositories by their Gitolite name so that they won't be mirrored. Upon upgrading, previously "disabled" repositories will be automatically migrated to this exclusion list.
- The new `aws_codecommit.exclude` setting in [AWS CodeCommit external service config](https://docs.sourcegraph.com/admin/external_service/aws_codecommit#configuration) allows you to exclude specific repositories by their AWS name or ID so that they won't be synced. Upon upgrading, previously "disabled" repositories will be automatically migrated to this exclusion list.
- Added a new, _required_ `aws_codecommit.gitCredentials` setting to the [AWS CodeCommit external service config](https://docs.sourcegraph.com/admin/external_service/aws_codecommit#configuration). These Git credentials are required to create long-lived authenticated clone URLs for AWS CodeCommit repositories. For more information about Git credentials, see the AWS CodeCommit documentation: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_ssh-keys.html#git-credentials-code-commit. For detailed instructions on how to create the credentials in IAM, see this page: https://docs.aws.amazon.com/codecommit/latest/userguide/setting-up-gc.html
- Added support for specifying a URL formatted `gitolite.host` setting in [Gitolite external service config](https://docs.sourcegraph.com/admin/external_service/gitolite#configuration) (e.g. `ssh://git@gitolite.example.org:2222/`), in addition to the already supported SCP like format (e.g `git@gitolite.example.org`)
- Added support for overriding critical, site, and external service configurations via files. Specify `CRITICAL_CONFIG_FILE=critical.json`, `SITE_CONFIG_FILE=site.json`, and/or `EXTSVC_CONFIG_FILE=extsvc.json` on the `frontend` container to do this.

### Changed

- Kinds of external services in use are now included in [server pings](https://docs.sourcegraph.com/admin/pings).
- Bitbucket Server: An actual Bitbucket icon is now used for the jump-to-bitbucket action on repository pages instead of the previously generic icon.
- Default config for GitHub, GitHub Enterprise, GitLab, Bitbucket Server, and AWS Code Commit external services has been revised to make it easier for first time admins.

### Removed

- Fields related to Repository enablement have been deprecated. Mutations are now NOOPs, and for repositories returned the value is always true for Enabled. The enabled field and mutations will be removed in 3.6. Mutations: `setRepositoryEnabled`, `setAllRepositoriesEnabled`, `updateAllMirrorRepositories`, `deleteRepository`. Query parameters: `repositories.enabled`, `repositories.disabled`. Field: `Repository.enabled`.
- Global saved searches are now deprecated. Any existing global saved searches have been assigned to the Sourcegraph instance's first site admin's user account.
- The `search.savedQueries` configuration option is now deprecated. Existing entries remain in user and org settings for backward compatibility, but are unused as saved searches are now stored in the database.

### Fixed

- Fixed a bug where submitting a saved query without selecting the location would fail for non-site admins (#3628).
- Fixed settings editors only having a few pixels height.
- Fixed a bug where browser extension and code review integration usage stats were not being captured on the site-admin Usage Stats page.
- Fixed an issue where in some rare cases PostgreSQL starting up slowly could incorrectly trigger a panic in the `frontend` service.
- Fixed an issue where the management console password would incorrectly reset to a new secure one after a user account was created.
- Fixed a bug where gitserver would leak file descriptors when performing common operations.
- Substantially improved the performance of updating Bitbucket Server external service configurations on instances with thousands of repositories, going from e.g. several minutes to about a minute for ~20k repositories (#4037).
- Fully resolved the search performance regression in v3.2.0, restoring performance of search back to the same levels it was before changes made in v3.2.0.
- Fix a bug where using a repo search filter with the prefix `github.com` only searched for repos whose name starts with `github.com`, even though no `^` was specified in the search filter. (#4103)
- Fixed an issue where files that fail syntax highlighting would incorrectly render an error instead of gracefully falling back to their plaintext form.

## 3.3.9

### Added

- Syntax highlighting requests that fail are now logged and traced. A new Prometheus metric `src_syntax_highlighting_requests` allows monitoring and alerting. [#4877](https://github.com/sourcegraph/sourcegraph/issues/4877).

## 3.3.8

### Fixed

- Fully resolved the search performance regression in v3.2.0, restoring performance of search back to the same levels it was before changes made in v3.2.0.
- Fixed an issue where files that fail syntax highlighting would incorrectly render an error instead of gracefully falling back to their plaintext form.
- Fixed an issue introduced in v3.3 where Sourcegraph would under specific circumstances incorrectly have to re-clone and re-index repositories from Bitbucket Server and AWS CodeCommit.

## 3.3.7

### Added

- The `bitbucketserver.exclude` setting in [Bitbucket Server external service config](https://docs.sourcegraph.com/admin/external_service/bitbucketserver#configuration) additionally allows you to exclude repositories matched by a regular expression (so that they won't be synced).

### Changed

### Removed

### Fixed

- Fixed a major indexed search performance regression that occurred in v3.2.0. (#3685)
- Fixed an issue where Sourcegraph would fail to update repositories on some instances (`pq: duplicate key value violates unique constraint "repo_external_service_unique_idx"`) (#3680)
- Fixed an issue where Sourcegraph would not exclude unavailable Bitbucket Server repositories. (#3772)

## 3.3.6

## Changed

- All 24 language extensions are enabled by default.

## 3.3.5

## Changed

- Indexed search is now enabled by default for new Docker deployments. (#3540)

### Removed

- Removed smart-casing behavior from search.

### Fixed

- Removes corrupted archives in the searcher cache and tries to populate the cache again instead of returning an error.
- Fixed a bug where search scopes would not get merged, and only the lowest-level list of search scopes would appear.
- Fixed an issue where repo-updater was slower in performing its work which could sometimes cause other performance issues. https://github.com/sourcegraph/sourcegraph/pull/3633

## 3.3.4

### Fixed

- Fixed bundling of the Phabricator integration assets in the Sourcegraph docker image.

## 3.3.3

### Fixed

- Fixed bug that prevented "Find references" action from being completed in the activation checklist.

## 3.3.2

### Fixed

- Fixed an issue where the default `bitbucketserver.repositoryQuery` would not be created on migration from older Sourcegraph versions. https://github.com/sourcegraph/sourcegraph/issues/3591
- Fixed an issue where Sourcegraph would add deleted repositories to the external service configuration. https://github.com/sourcegraph/sourcegraph/issues/3588
- Fixed an issue where a repo-updater migration would hit code host rate limits. https://github.com/sourcegraph/sourcegraph/issues/3582
- The required `bitbucketserver.username` field of a [Bitbucket Server external service configuration](https://docs.sourcegraph.com/admin/external_service/bitbucketserver#configuration), if unset or empty, is automatically migrated to match the user part of the `url` (if defined). https://github.com/sourcegraph/sourcegraph/issues/3592
- Fixed a panic that would occur in indexed search / the frontend when a search error ocurred. https://github.com/sourcegraph/sourcegraph/issues/3579
- Fixed an issue where the repo-updater service could become deadlocked while performing a migration. https://github.com/sourcegraph/sourcegraph/issues/3590

## 3.3.1

### Fixed

- Fixed a bug that prevented external service configurations specifying client certificates from working (#3523)

## 3.3.0

### Added

- In search queries, treat `foo(` as `foo\(` and `bar[` as `bar\[` rather than failing with an error message.
- Enterprise admins can now customize the appearance of the homepage and search icon.
- A new settings property `notices` allows showing custom informational messages on the homepage and at the top of each page. The `motd` property is deprecated and its value is automatically migrated to the new `notices` property.
- The new `gitlab.exclude` setting in [GitLab external service config](https://docs.sourcegraph.com/admin/external_service/gitlab#configuration) allows you to exclude specific repositories matched by `gitlab.projectQuery` and `gitlab.projects` (so that they won't be synced). Upon upgrading, previously "disabled" repositories will be automatically migrated to this exclusion list.
- The new `gitlab.projects` setting in [GitLab external service config](https://docs.sourcegraph.com/admin/external_service/gitlab#configuration) allows you to select specific repositories to be synced.
- The new `bitbucketserver.exclude` setting in [Bitbucket Server external service config](https://docs.sourcegraph.com/admin/external_service/bitbucketserver#configuration) allows you to exclude specific repositories matched by `bitbucketserver.repositoryQuery` and `bitbucketserver.repos` (so that they won't be synced). Upon upgrading, previously "disabled" repositories will be automatically migrated to this exclusion list.
- The new `bitbucketserver.repos` setting in [Bitbucket Server external service config](https://docs.sourcegraph.com/admin/external_service/bitbucketserver#configuration) allows you to select specific repositories to be synced.
- The new required `bitbucketserver.repositoryQuery` setting in [Bitbucket Server external service configuration](https://docs.sourcegraph.com/admin/external_service/bitbucketserver#configuration) allows you to use Bitbucket API repository search queries to select repos to be synced. Existing configurations will be migrate to have it set to `["?visibility=public", "?visibility=private"]` which is equivalent to the previous implicit behaviour that this setting supersedes.
- "Quick configure" buttons for common actions have been added to the config editor for all external services.
- "Quick configure" buttons for common actions have been added to the management console.
- Site-admins now receive an alert every day for the seven days before their license key expires.
- The user menu (in global nav) now lists the user's organizations.
- All users on an instance now see a non-dismissable alert when when there's no license key in use and the limit of free user accounts is exceeded.
- All users will see a dismissible warning about limited search performance and accuracy on when using the sourcegraph/server Docker image with more than 100 repositories enabled.

### Changed

- Indexed searches that time out more consistently report a timeout instead of erroneously saying "No results."
- The symbols sidebar now only shows symbols defined in the current file or directory.
- The dynamic filters on search results pages will now display `lang:` instead of `file:` filters for language/file-extension filter suggestions.
- The default `github.repositoryQuery` of a [GitHub external service configuration](https://docs.sourcegraph.com/admin/external_service/github#configuration) has been changed to `["none"]`. Existing configurations that had this field unset will be migrated to have the previous default explicitly set (`["affiliated", "public"]`).
- The default `gitlab.projectQuery` of a [GitLab external service configuration](https://docs.sourcegraph.com/admin/external_service/gitlab#configuration) has been changed to `["none"]`. Existing configurations that had this field unset will be migrated to have the previous default explicitly set (`["?membership=true"]`).
- The default value of `maxReposToSearch` is now unlimited (was 500).
- The default `github.repositoryQuery` of a [GitHub external service configuration](https://docs.sourcegraph.com/admin/external_service/github#configuration) has been changed to `["none"]` and is now a required field. Existing configurations that had this field unset will be migrated to have the previous default explicitly set (`["affiliated", "public"]`).
- The default `gitlab.projectQuery` of a [GitLab external service configuration](https://docs.sourcegraph.com/admin/external_service/gitlab#configuration) has been changed to `["none"]` and is now a required field. Existing configurations that had this field unset will be migrated to have the previous default explicitly set (`["?membership=true"]`).
- The `bitbucketserver.username` field of a [Bitbucket Server external service configuration](https://docs.sourcegraph.com/admin/external_service/bitbucketserver#configuration) is now **required**. This field is necessary to authenticate with the Bitbucket Server API with either `password` or `token`.
- The settings and account pages for users and organizations are now combined into a single tab.

### Removed

- Removed the option to show saved searches on the Sourcegraph homepage.

### Fixed

- Fixed an issue where the site-admin repositories page `Cloning`, `Not Cloned`, `Needs Index` tabs were very slow on instances with thousands of repositories.
- Fixed an issue where failing to syntax highlight a single file would take down the entire syntax highlighting service.

## 3.2.6

### Fixed

- Fully resolved the search performance regression in v3.2.0, restoring performance of search back to the same levels it was before changes made in v3.2.0.

## 3.2.5

### Fixed

- Fixed a major indexed search performance regression that occurred in v3.2.0. (#3685)

## 3.2.4

### Fixed

- Fixed bundling of the Phabricator integration assets in the Sourcegraph docker image.

## 3.2.3

### Fixed

- Fixed https://github.com/sourcegraph/sourcegraph/issues/3336.
- Clearer error message when a repository sync fails due to the inability to clone a repository.
- Rewrite '@' character in Gitolite repository names to '-', which permits them to be viewable in the UI.

## 3.2.2

### Changed

- When using an external Zoekt instance (specified via the `ZOEKT_HOST` environment variable), sourcegraph/server no longer spins up a redundant internal Zoekt instance.

## 3.2.1

### Fixed

- Jaeger tracing, once enabled, can now be configured via standard [environment variables](https://github.com/jaegertracing/jaeger-client-go/blob/v2.14.0/README.md#environment-variables).
- Fixed an issue where some search and zoekt errors would not be logged.

## 3.2.0

### Added

- Sourcegraph can now automatically use the system's theme.
  To enable, open the user menu in the top right and make sure the theme dropdown is set to "System".
  This is currently supported on macOS Mojave with Safari Technology Preview 68 and later.
- The `github.exclude` setting was added to the [GitHub external service config](https://docs.sourcegraph.com/admin/external_service/github#configuration) to allow excluding repositories yielded by `github.repos` or `github.repositoryQuery` from being synced.

### Changed

- Symbols search is much faster now. After the initial indexing, you can expect code intelligence to be nearly instant no matter the size of your repository.
- Massively reduced the number of code host API requests Sourcegraph performs, which caused rate limiting issues such as slow search result loading to appear.
- The [`corsOrigin`](https://docs.sourcegraph.com/admin/config/site_config) site config property is no longer needed for integration with GitHub, GitLab, etc., via the [Sourcegraph browser extension](https://docs.sourcegraph.com/integration/browser_extension). Only the [Phabricator extension](https://github.com/sourcegraph/phabricator-extension) requires it.

### Fixed

- Fixed a bug where adding a search scope that adds a `repogroup` filter would cause invalid queries if `repogroup:sample` was already part of the query.
- An issue where errors during displaying search results would not be displayed.

### Removed

- The `"updateScheduler2"` experiment is now the default and it's no longer possible to configure.

## 3.1.2

### Added

- The `search.contextLines` setting was added to allow configuration of the number of lines of context to be displayed around search results.

### Changed

- Massively reduced the number of code host API requests Sourcegraph performs, which caused rate limiting issues such as slow search result loading to appear.
- Improved logging in various situations where Sourcegraph would potentially hit code host API rate limits.

### Fixed

- Fixed an issue where search results loading slowly would display a `Cannot read property "lastChild" of undefined` error.

## 3.1.1

### Added

- Query builder toggle (open/closed) state is now retained.

### Fixed

- Fixed an issue where single-term values entered into the "Exact match" field in the query builder were not getting wrapped in quotes.

## 3.1.0

### Added

- Added Docker-specific help text when running the Sourcegraph docker image in an environment with an sufficient open file descriptor limit.
- Added syntax highlighting for Kotlin and Dart.
- Added a management console environment variable to disable HTTPS, see [the docs](doc/admin/management_console.md#can-i-disable-https-on-the-management-console) for more information.
- Added `auth.disableUsernameChanges` to critical configuration to prevent users from changing their usernames.
- Site admins can query a user by email address or username from the GraphQL API.
- Added a search query builder to the main search page. Click "Use search query builder" to open the query builder, which is a form with separate inputs for commonly used search keywords.

### Changed

- File match search results now show full repository name if there are results from mirrors on different code hosts (e.g. github.com/sourcegraph/sourcegraph and gitlab.com/sourcegraph/sourcegraph)
- Search queries now use "smart case" by default. Searches are case insensitive unless you use uppercase letters. To explicitly set the case, you can still use the `case` field (e.g. `case:yes`, `case:no`). To explicitly set smart case, use `case:auto`.

### Fixed

- Fixed an issue where the management console would improperly regenerate the TLS cert/key unless `CUSTOM_TLS=true` was set. See the documentation for [how to use your own TLS certificate with the management console](doc/admin/management_console.md#how-can-i-use-my-own-tls-certificates-with-the-management-console).

## 3.0.1

### Added

- Symbol search now supports Elixir, Haskell, Kotlin, Scala, and Swift

### Changed

- Significantly optimized how file search suggestions are provided when using indexed search (cluster deployments).
- Both the `sourcegraph/server` image and the [Kubernetes deployment](https://github.com/sourcegraph/deploy-sourcegraph) manifests ship with Postgres `11.1`. For maximum compatibility, however, the minimum supported version remains `9.6`. The upgrade procedure is mostly automated for existing deployments. Please refer to [this page](https://docs.sourcegraph.com/admin/postgres) for detailed instructions.

### Removed

- The deprecated `auth.disableAccessTokens` site config property was removed. Use `auth.accessTokens` instead.
- The `disableBrowserExtension` site config property was removed. [Configure nginx](https://docs.sourcegraph.com/admin/nginx) instead to block clients (if needed).

## 3.0.0

See the changelog entries for 3.0.0 beta releases and our [3.0](doc/admin/migration/3_0.md) upgrade guide if you are upgrading from 2.x.

## 3.0.0-beta.4

### Added

- Basic code intelligence for the top 10 programming languages works out of the box without any configuration. [TypeScript/JavaScript](https://sourcegraph.com/extensions/sourcegraph/typescript), [Python](https://sourcegraph.com/extensions/sourcegraph/python), [Java](https://sourcegraph.com/extensions/sourcegraph/java), [Go](https://sourcegraph.com/extensions/sourcegraph/go), [C/C++](https://sourcegraph.com/extensions/sourcegraph/cpp), [Ruby](https://sourcegraph.com/extensions/sourcegraph/ruby), [PHP](https://sourcegraph.com/extensions/sourcegraph/php), [C#](https://sourcegraph.com/extensions/sourcegraph/csharp), [Shell](https://sourcegraph.com/extensions/sourcegraph/shell), and [Scala](https://sourcegraph.com/extensions/sourcegraph/scala) are enabled by default, and you can find more in the [extension registry](https://sourcegraph.com/extensions?query=category%3A"Programming+languages").

## 3.0.0-beta.3

- Fixed an issue where the site admin is redirected to the start page instead of being redirected to the repositories overview page after deleting a repo.

## 3.0.0-beta

### Added

- Repositories can now be queried by a git clone URL through the GraphQL API.
- A new Explore area is linked from the top navigation bar (when the `localStorage.explore=true;location.reload()` feature flag is enabled).
- Authentication via GitHub is now supported. To enable, add an item to the `auth.providers` list with `type: "github"`. By default, GitHub identities must be linked to an existing Sourcegraph user account. To enable new account creation via GitHub, use the `allowSignup` option in the `GitHubConnection` config.
- Authentication via GitLab is now supported. To enable, add an item to the `auth.providers` list with `type: "gitlab"`.
- GitHub repository permissions are supported if authentication via GitHub is enabled. See the
  documentation for the `authorization` field of the `GitHubConnection` configuration.
- The repository settings mirroring page now shows when a repo is next scheduled for an update (requires experiment `"updateScheduler2": "enabled"`).
- Configured repositories are periodically scheduled for updates using a new algorithm. You can disable the new algorithm with the following site configuration: `"experimentalFeatures": { "updateScheduler2": "disabled" }`. If you do so, please file a public issue to describe why you needed to disable it.
- When using HTTP header authentication, [`stripUsernameHeaderPrefix`](https://docs.sourcegraph.com/admin/auth/#username-header-prefixes) field lets an admin specify a prefix to strip from the HTTP auth header when converting the header value to a username.
- Sourcegraph extensions whose package.json contains `"wip": true` are considered [work-in-progress extensions](https://docs.sourcegraph.com/extensions/authoring/publishing#wip-extensions) and are indicated as such to avoid users accidentally using them.
- Information about user survey submissions and a chart showing weekly active users is now displayed on the site admin Overview page.
- A new GraphQL API field `UserEmail.isPrimary` was added that indicates whether an email is the user's primary email.
- The filters bar in the search results page can now display filters from extensions.
- Extensions' `activate` functions now receive a `sourcegraph.ExtensionContext` parameter (i.e., `export function activate(ctx: sourcegraph.ExtensionContext): void { ... }`) to support deactivation and running multiple extensions in the same process.
- Users can now request an Enterprise trial license from the site init page.
- When searching, a filter button `case:yes` will now appear when relevant. This helps discovery and makes it easier to use our case-sensitive search syntax.
- Extensions can now report progress in the UI through the `withProgress()` extension API.
- When calling `editor.setDecorations()`, extensions must now provide an instance of `TextDocumentDecorationType` as first argument. This helps gracefully displaying decorations from several extensions.

### Changed

- The Postgres database backing Sourcegraph has been upgraded from 9.4 to 11.1. Existing Sourcegraph users must conduct an [upgrade procedure](https://docs.sourcegraph.com/admin/postgres_upgrade)
- Code host configuration has moved out of the site config JSON into the "External services" area of the site admin web UI. Sourcegraph instances will automatically perform a one time migration of existing data in the site config JSON. After the migration these keys can be safely deleted from the site config JSON: `awsCodeCommit`, `bitbucketServer`, `github`, `gitlab`, `gitolite`, and `phabricator`.
- Site and user usage statistics are now visible to all users. Previously only site admins (and users, for their own usage statistics) could view this information. The information consists of aggregate counts of actions such as searches, page views, etc.
- The Git blame information shown at the end of a line is now provided by the [Git extras extension](https://sourcegraph.com/extensions/sourcegraph/git-extras). You must add that extension to continue using this feature.
- The `appURL` site configuration option was renamed to `externalURL`.
- The repository and directory pages now show all entries together instead of showing files and (sub)directories separately.
- Extensions no longer can specify titles (in the `title` property in the `package.json` extension manifest). Their extension ID (such as `alice/myextension`) is used.

### Fixed

- Fixed an issue where the site admin License page showed a count of current users, rather than the max number of users over the life of the license.
- Fixed number formatting issues on site admin Overview and Survey Response pages.
- Fixed resolving of git clone URLs with `git+` prefix through the GraphQL API
- Fixed an issue where the graphql Repositories endpoint would order by a field which was not indexed. Times on Sourcegraph.com went from 10s to 200ms.
- Fixed an issue where whitespace was not handled properly in environment variable lists (`SYMBOLS_URL`, `SEARCHER_URL`).
- Fixed an issue where clicking inside the repository popover or clicking "Show more" would dismiss the popover.

### Removed

- The `siteID` site configuration option was removed because it is no longer needed. If you previously specified this in site configuration, a new, random site ID will be generated upon server startup. You can safely remove the existing `siteID` value from your site configuration after upgrading.
- The **Info** panel was removed. The information it presented can be viewed in the hover.
- The top-level `repos.list` site configuration was removed in favour of each code-host's equivalent options,
  now configured via the new _External Services UI_ available at `/site-admin/external-services`. Equivalent options in code hosts configuration:
  - GitHub via [`github.repos`](https://docs.sourcegraph.com/admin/site_config/all#repos-array)
  - Gitlab via [`gitlab.projectQuery`](https://docs.sourcegraph.com/admin/site_config/all#projectquery-array)
  - Phabricator via [`phabricator.repos`](https://docs.sourcegraph.com/admin/site_config/all#phabricator-array)
  - [Other external services](https://docs.sourcegraph.com/admin/repo/add_from_other_external_services)
- Removed the `httpStrictTransportSecurity` site configuration option. Use [nginx configuration](https://docs.sourcegraph.com/admin/nginx) for this instead.
- Removed the `tls.letsencrypt` site configuration option. Use [nginx configuration](https://docs.sourcegraph.com/admin/nginx) for this instead.
- Removed the `tls.cert` and `tls.key` site configuration options. Use [nginx configuration](https://docs.sourcegraph.com/admin/nginx) for this instead.
- Removed the `httpToHttpsRedirect` and `experimentalFeatures.canonicalURLRedireect` site configuration options. Use [nginx configuration](https://docs.sourcegraph.com/admin/nginx) for these instead.
- Sourcegraph no longer requires access to `/var/run/docker.sock`.

## 2.13.6

### Added

- The `/-/editor` endpoint now accepts a `hostname_patterns` URL parameter, which specifies a JSON
  object mapping from hostname to repository name pattern. This serves as a hint to Sourcegraph when
  resolving git clone URLs to repository names. The name pattern is the same style as is used in
  code host configurations. The default value is `{hostname}/{path}`.

## 2.13.5

### Fixed

- Fixed another issue where Sourcegraph would try to fetch more than the allowed number of repositories from AWS CodeCommit.

## 2.13.4

### Changed

- The default for `experimentalFeatures.canonicalURLRedirect` in site config was changed back to `disabled` (to avoid [#807](https://github.com/sourcegraph/sourcegraph/issues/807)).

## 2.13.3

### Fixed

- Fixed an issue that would cause the frontend health check endpoint `/healthz` to not respond. This only impacts Kubernetes deployments.
- Fixed a CORS policy issue that caused requests to be rejected when they come from origins not in our [manifest.json](https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/browser/src/extension/manifest.spec.json#L72) (i.e. requested via optional permissions by the user).
- Fixed an issue that prevented `repositoryQuery` from working correctly on GitHub enterprise instances.

## 2.13.2

### Fixed

- Fixed an issue where Sourcegraph would try to fetch more than the allowed number of repositories from AWS CodeCommit.

## 2.13.1

### Changed

- The timeout when running `git ls-remote` to determine if a remote url is cloneable has been increased from 5s to 30s.
- Git commands now use [version 2 of the Git wire protocol](https://opensource.googleblog.com/2018/05/introducing-git-protocol-version-2.html), which should speed up certain operations (e.g. `git ls-remote`, `git fetch`) when communicating with a v2 enabled server.

## 2.13.0

### Added

- A new site config option `search.index.enabled` allows toggling on indexed search.
- Search now uses [Sourcegraph extensions](https://docs.sourcegraph.com/extensions) that register `queryTransformer`s.
- GitLab repository permissions are now supported. To enable this, you will need to set the `authz`
  field in the `GitLabConnection` configuration object and ensure that the access token set in the
  `token` field has both `sudo` and `api` scope.

### Changed

- When the `DEPLOY_TYPE` environment variable is incorrectly specified, Sourcegraph now shuts down and logs an error message.
- The `experimentalFeatures.canonicalURLRedirect` site config property now defaults to `enabled`. Set it to `disabled` to disable redirection to the `appURL` from other hosts.
- Updating `maxReposToSearch` site config no longer requires a server restart to take effect.
- The update check page no longer shows an error if you are using an insiders build. Insiders builds will now notify site administrators that updates are available 40 days after the release date of the installed build.
- The `github.repositoryQuery` site config property now accepts arbitrary GitHub repository searches.

### Fixed

- The user account sidebar "Password" link (to the change-password form) is now shown correctly.
- Fixed an issue where GitHub rate limits were underutilized if the remaining
  rate limit dropped below 150.
- Fixed an issue where GraphQL field `elapsedMilliseconds` returned invalid value on empty searches
- Editor extensions now properly search the selection as a literal string, instead of incorrectly using regexp.
- Fixed a bug where editing and deleting global saved searches was not possible.
- In index search, if the search regex produces multiline matches, search results are still processed per line and highlighted correctly.
- Go-To-GitHub and Go-To-GitLab buttons now link to the right branch, line and commit range.
- Go-to-GitHub button links to default branch when no rev is given.
- The close button in the panel header stays located on the top.
- The Phabricator icon is now displayed correctly.
- The view mode button in the BlobPage now shows the correct view mode to switch to.

### Removed

- The experimental feature flag to disable the new repo update scheduler has been removed.
- The `experimentalFeatures.configVars` feature flag was removed.
- The `experimentalFeatures.multipleAuthProviders` feature flag was removed because the feature is now always enabled.
- The following deprecated auth provider configuration properties were removed: `auth.provider`, `auth.saml`, `auth.openIDConnect`, `auth.userIdentityHTTPHeader`, and `auth.allowSignup`. Use `auth.providers` for all auth provider configuration. (If you were still using the deprecated properties and had no `auth.providers` set, all access to your instance will be rejected until you manually set `auth.providers`.)
- The deprecated site configuration properties `search.scopes` and `settings` were removed. Define search scopes and settings in global settings in the site admin area instead of in site configuration.
- The `pendingContents` property has been removed from our GraphQL schema.
- The **Explore** page was replaced with a **Repositories** search link in the top navigation bar.

## 2.12.3

### Fixed

- Fixed an error that prevented users without emails from submitting satisfaction surveys.

## 2.12.2

### Fixed

- Fixed an issue where private GitHub Enterprise repositories were not fetched.

## 2.12.1

### Fixed

- We use GitHub's REST API to query affliated repositories. This API has wider support on older GitHub enterprise versions.
- Fixed an issue that prevented users without email addresses from signing in (https://github.com/sourcegraph/sourcegraph/issues/426).

## 2.12.0

### Changed

- Reduced the size of in-memory data structured used for storing search results. This should reduce the backend memory usage of large result sets.
- Code intelligence is now provided by [Sourcegraph extensions](https://docs.sourcegraph.com/extensions). The extension for each language in the site configuration `langservers` property is automatically enabled.
- Support for multiple authentication providers is now enabled by default. To disable it, set the `experimentalFeatures.multipleAuthProviders` site config option to `"disabled"`. This only applies to Sourcegraph Enterprise.
- When using the `http-header` auth provider, valid auth cookies (from other auth providers that are currently configured or were previously configured) are now respected and will be used for authentication. These auth cookies also take precedence over the `http-header` auth. Previously, the `http-header` auth took precedence.
- Bitbucket Server username configuration is now used to clone repositories if the Bitbucket Server API does not set a username.
- Code discussions: On Sourcegraph.com / when `discussions.abuseProtection` is enabled in the site config, rate limits to thread creation, comment creation, and @mentions are now applied.

### Added

- Search syntax for filtering archived repositories. `archived:no` will exclude archived repositories from search results, `archived:only` will search over archived repositories only. This applies for GitHub and GitLab repositories.
- A Bitbucket Server option to exclude personal repositories in the event that you decide to give an admin-level Bitbucket access token to Sourcegraph and do not want to create a bot account. See https://docs.sourcegraph.com/integration/bitbucket_server#excluding-personal-repositories for more information.
- Site admins can now see when users of their Sourcegraph instance last used it via a code host integration (e.g. Sourcegraph browser extensions). Visit the site admin Analytics page (e.g. https://sourcegraph.example.com/site-admin/analytics) to view this information.
- A new site config option `extensions.allowRemoteExtensions` lets you explicitly specify the remote extensions (from, e.g., Sourcegraph.com) that are allowed.
- Pings now include a total count of user accounts.

### Fixed

- Files with the gitattribute `export-ignore` are no longer excluded for language analysis and search.
- "Discard changes?" confirmation popup doesn't pop up every single time you try to navigate to a new page after editting something in the site settings page anymore.
- Fixed an issue where Git repository URLs would sometimes be logged, potentially containing e.g. basic auth tokens.
- Fixed date formatting on the site admin Analytics page.
- File names of binary and large files are included in search results.

### Removed

- The deprecated environment variables `SRC_SESSION_STORE_REDIS` and `REDIS_MASTER_ENDPOINT` are no longer used to configure alternative redis endpoints. For more information, see "[using external services with Sourcegraph](https://docs.sourcegraph.com/admin/external_services)".

## 2.11.1

### Added

- A new site config option `git.cloneURLToRepositoryName` specifies manual mapping from Git clone URLs to Sourcegraph repository names. This is useful, for example, for Git submodules that have local clone URLs.

### Fixed

- Slack notifications for saved searches have been fixed.

## 2.11.0

### Changed

### Added

- Support for ACME "tls-alpn-01" challenges to obtain LetsEncrypt certificates. Previously Sourcegraph only supported ACME "http-01" challenges which required port 80 to be accessible.
- gitserver periodically removes stale lock files that git can leave behind.
- Commits with empty trees no longer return 404.
- Clients (browser/editor extensions) can now query configuration details from the `ClientConfiguration` GraphQL API.
- The config field `auth.accessTokens.allow` allows or restricts use of access tokens. It can be set to one of three values: "all-users-create" (the default), "none" (all access tokens are disabled), and "site-admin-create" (access tokens are enabled, but only site admins can create new access tokens). The field `auth.disableAccessTokens` is now deprecated in favor of this new field.
- A webhook endpoint now exists to trigger repository updates. For example, `curl -XPOST -H 'Authorization: token $ACCESS_TOKEN' $SOURCEGRAPH_ORIGIN/.api/repos/$REPO_URI/-/refresh`.
- Git submodules entries in the file tree now link to the submodule repository.

### Fixed

- An issue / edge case where the Code Intelligence management admin page would incorrectly show language servers as `Running` when they had been removed from Docker.
- Log level is respected in lsp-proxy logs.
- Fixed an error where text searches could be routed to a faulty search worker.
- Gitolite integration should correctly detect names which Gitolite would consider to be patterns, and not treat them as repositories.
- repo-updater backs off fetches on a repo that's failing to fetch.
- Attempts to add a repo with an empty string for the name are checked for and ignored.
- Fixed an issue where non-site-admin authenticated users could modify global settings (not site configuration), other organizations' settings, and other users' settings.
- Search results are rendered more eagerly, resulting in fewer blank file previews
- An issue where automatic code intelligence would fail to connect to the underlying `lsp` network, leading to `dial tcp: lookup lang on 0.0.0.0:53: no such host` errors.
- More useful error messages from lsp-proxy when a language server can't get a requested revision of a repository.
- Creation of a new user with the same name as an existing organization (and vice versa) is prevented.

### Removed

## 2.10.5

### Fixed

- Slack notifications for saved searches have been fixed.

## 2.10.4

### Fixed

- Fixed an issue that caused the frontend to return a HTTP 500 and log an error message like:
  ```
  lvl=eror msg="ui HTTP handler error response" method=GET status_code=500 error="Post http://127.0.0.1:3182/repo-lookup: context canceled"
  ```

## 2.10.3

### Fixed

- The SAML AuthnRequest signature when using HTTP redirect binding is now computed using a URL query string with correct ordering of parameters. Previously, the ordering was incorrect and caused errors when the IdP was configured to check the signature in the AuthnRequest.

## 2.10.2

### Fixed

- SAML IdP-initiated login previously failed with the IdP set a RelayState value. This now works.

## 2.10.1

### Changed

- Most `experimentalFeatures` in the site configuration now respond to configuration changes live, without requiring a server restart. As usual, you will be prompted for a restart after saving your configuration changes if one is required.
- Gravatar image avatars are no longer displayed for committers.

## 2.10.0

### Changed

- In the file tree, if a directory that contains only a single directory is expanded, its child directory is now expanded automatically.

### Fixed

- Fixed an issue where `sourcegraph/server` would not start code intelligence containers properly when the `sourcegraph/server` container was shut down non-gracefully.
- Fixed an issue where the file tree would return an error when navigating between repositories.

## 2.9.4

### Changed

- Repo-updater has a new and improved scheduler for periodic repo fetches. If you have problems with it, you can revert to the old behavior by adding `"experimentalFeatures": { "updateScheduler": "disabled" }` to your `config.json`.
- A once-off migration will run changing the layout of cloned repos on disk. This should only affect installations created January 2018 or before. There should be no user visible changes.
- Experimental feature flag "updateScheduler" enables a smarter and less spammy algorithm for automatic repository updates.
- It is no longer possible to disable code intelligence by unsetting the LSP_PROXY environment variable. Instead, code intelligence can be disabled per language on the site admin page (e.g. https://sourcegraph.example.com/site-admin/code-intelligence).
- Bitbucket API requests made by Sourcegraph are now under a self-enforced API rate limit (since Bitbucket Server does not have a concept of rate limiting yet). This will reduce any chance of Sourcegraph slowing down or causing trouble for Bitbucket Server instances connected to it. The limits are: 7,200 total requests/hr, with a bucket size / maximum burst size of 500 requests.
- Global, org, and user settings are now validated against the schema, so invalid settings will be shown in the settings editor with a red squiggly line.
- The `http-header` auth provider now supports being used with other auth providers (still only when `experimentalFeatures.multipleAuthProviders` is `true`).
- Periodic fetches of Gitolite-hosted repositories are now handled internally by repo-updater.

### Added

- The `log.sentry.dsn` field in the site config makes Sourcegraph log application errors to a Sentry instance.
- Two new repository page hotkeys were added: <kbd>r</kbd> to open the repositories menu and <kbd>v</kbd> to open the revision selector.
- Repositories are periodically (~45 days) recloned from the codehost. The codehost can be relied on to give an efficient packing. This is an alternative to running a memory and CPU intensive git gc and git prune.
- The `auth.sessionExpiry` field sets the session expiration age in seconds (defaults to 90 days).

### Fixed

- Fixed a bug in the API console that caused it to display as a blank page in some cases.
- Fixed cases where GitHub rate limit wasn't being respected.
- Fixed a bug where scrolling in references, history, etc. file panels was not possible in Firefox.
- Fixed cases where gitserver directory structure migration could fail/crash.
- Fixed "Generate access token" link on user settings page. Previously, this link would 404.
- Fixed a bug where the search query was not updated in the search bar when searching from the homepage.
- Fixed a possible crash in github-proxy.
- Fixed a bug where file matching for diff search was case sensitive by default.

### Removed

- `SOURCEGRAPH_CONFIG` environment variable has been removed. Site configuration is always read from and written to disk. You can configure the location by providing `SOURCEGRAPH_CONFIG_FILE`. The default path is `/etc/sourcegraph/config.json`.

## 2.9.3

### Changed

- The search results page will merge duplicated lines of context.
- The following deprecated site configuration properties have been removed: `github[].preemptivelyClone`, `gitOriginMap`, `phabricatorURL`, `githubPersonalAccessToken`, `githubEnterpriseURL`, `githubEnterpriseCert`, and `githubEnterpriseAccessToken`.
- The `settings` field in the site config file is deprecated and will not be supported in a future release. Site admins should move those settings (if any) to global settings (in the site admin UI). Global settings are preferred to site config file settings because the former can be applied without needing to restart/redeploy the Sourcegraph server or cluster.

### Fixed

- Fixed a goroutine leak which occurs when search requests are canceled.
- Console output should have fewer spurious line breaks.
- Fixed an issue where it was not possible to override the `StrictHostKeyChecking` SSH option in the SSH configuration.
- Cross-repository code intelligence indexing for non-Go languages is now working again (originally broken in 2.9.2).

## 2.9.1

### Fixed

- Fixed an issue where saving an organization's configuration would hang indefinitely.

## 2.9.0

### Changed

- Hover tooltips were rewritten to fix a couple of issues and are now much more robust, received a new design and show more information.
- The `max:` search flag was renamed to `count:` in 2.8.8, but for backward compatibility `max:` has been added back as a deprecated alias for `count:`.
- Drastically improved the performance / load time of the Code Intelligence site admin page.

### Added

- The site admin code intelligence page now displays an error or reason whenever language servers are unable to be managed from the UI or Sourcegraph API.
- The ability to directly specify the root import path of a repository via `.sourcegraph/config.json` in the repo root, instead of relying on the heuristics of the Go language server to detect it.

### Fixed

- Configuring Bitbucket Server now correctly suppresses the the toast message "Configure repositories and code hosts to add to Sourcegraph."
- A bug where canonical import path comments would not be detected by the Go language server's heuristics under `cmd/` folders.
- Fixed an issue where a repository would only be refreshed on demand by certain user actions (such as a page reload) and would otherwise not be updated when expected.
- If a code host returned a repository-not-found or unauthorized error (to `repo-updater`) for a repository that previously was known to Sourcegraph, then in some cases a misleading "Empty repository" screen was shown. Now the repository is displayed as though it still existed, using cached data; site admins must explicitly delete repositories on Sourcegraph after they have been deleted on the code host.
- Improved handling of GitHub API rate limit exhaustion cases. Cached repository metadata and Git data will be used to provide full functionality during this time, and log messages are more informative. Previously, in some cases, repositories would become inaccessible.
- Fixed an issue where indexed search would sometimes not indicate that there were more results to show for a given file.
- Fixed an issue where the code intelligence admin page would never finish loading language servers.

## 2.9.0-pre0

### Changed

- Search scopes have been consolidated into the "Filters" bar on the search results page.
- Usernames and organization names of up to 255 characters are allowed. Previously the max length was 38.

### Fixed

- The target commit ID of a Git tag object (i.e., not lightweight Git tag refs) is now dereferenced correctly. Previously the tag object's OID was given.
- Fixed an issue where AWS Code Commit would hit the rate limit.
- Fixed an issue where dismissing the search suggestions dropdown did not unfocus previously highlighted suggestions.
- Fixed an issue where search suggestions would appear twice.
- Indexed searches now return partial results if they timeout.
- Git repositories with files whose paths contain `.git` path components are now usable (via indexed and non-indexed search and code intelligence). These corrupt repositories are rare and generally were created by converting some other VCS repository to Git (the Git CLI will forbid creation of such paths).
- Various diff search performance improvements and bug fixes.
- New Phabricator extension versions would used cached stylesheets instead of the upgraded version.
- Fixed an issue where hovers would show an error for Rust and C/C++ files.

### Added

- The `sourcegraph/server` container now emits the most recent log message when redis terminates to make it easier to debug why redis stopped.
- Organization invites (which allow users to invite other users to join organizations) are significantly improved. A new accept-invitation page was added.
- The new help popover allows users to easily file issues in the Sourcegraph public issue tracker and view documentation.
- An issue where Java files would be highlighted incorrectly if they contained JavaDoc blocks with an uneven number of opening/closing `*`s.

### Removed

- The `secretKey` site configuration value is no longer needed. It was only used for generating tokens for inviting a user to an organization. The invitation is now stored in the database associated with the recipient, so a secret token is no longer needed.
- The `experimentalFeatures.searchTimeoutParameter` site configuration value has been removed. It defaulted to `enabled` in 2.8 and it is no longer possible to disable.

### Added

- Syntax highlighting for:
  - TOML files (including Go `Gopkg.lock` and Rust `Cargo.lock` files).
  - Rust files.
  - GraphQL files.
  - Protobuf files.
  - `.editorconfig` files.

## 2.8.9

### Changed

- The "invite user" site admin page was moved to a sub-page of the users page (`/site-admin/users/new`).
- It is now possible for a site admin to create a new user without providing an email address.

### Fixed

- Checks for whether a repo is cloned will no longer exhaust open file pools over time.

### Added

- The Phabricator extension shows code intelligence status and supports enabling / disabling code intelligence for files.

## 2.8.8

### Changed

- Queries for repositories (in the explore, site admin repositories, and repository header dropdown) are matched on case-insensitive substrings, not using fuzzy matching logic.
- HTTP Authorization headers with an unrecognized scheme are ignored; they no longer cause the HTTP request to be rejected with HTTP 401 Unauthorized and an "Invalid Authorization header." error.
- Renamed the `max` search flag to `count`. Searches that specify `count:` will fetch at least that number of results, or the full result set.
- Bumped `lsp-proxy`'s `initialize` timeout to 3 minutes for every language.
- Search results are now sorted by repository and file name.
- More easily accessible "Show more" button at the top of the search results page.
- Results from user satisfaction surveys are now always hosted locally and visible to admins. The `"experimentalFeatures": { "hostSurveysLocally" }` config option has been deprecated.
- If the OpenID Connect authentication provider reports that a user's email address is not verified, the authentication attempt will fail.

### Fixed

- Fixed an issue where the search results page would not update its title.
- The session cookie name is now `sgs` (not `sg-session`) so that Sourcegraph 2.7 and Sourcegraph 2.8 can be run side-by-side temporarily during a rolling update without clearing each other's session cookies.
- Fixed the default hostnames of the C# and R language servers
- Fixed an issue where deleting an organization prevented the creation of organizations with the name of the deleted organization.
- Non-UTF8 encoded files (e.g. ISO-8859-1/Latin1, UTF16, etc) are now displayed as text properly rather than being detected as binary files.
- Improved error message when lsp-proxy's initalize timeout occurs
- Fixed compatibility issues and added [instructions for using Microsoft ADFS 2.1 and 3.0 for SAML authentication](https://docs.sourcegraph.com/admin/auth/saml_with_microsoft_adfs).
- Fixed an issue where external accounts associated with deleted user accounts would still be returned by the GraphQL API. This caused the site admin external accounts page to fail to render in some cases.
- Significantly reduced the number of code host requests for non github.com or gitlab.com repositories.

### Added

- The repository revisions popover now shows the target commit's last-committed/authored date for branches and tags.
- Setting the env var `INSECURE_SAML_LOG_TRACES=1` on the server (or the `sourcegraph-frontend` pod in Kubernetes) causes all SAML requests and responses to be logged, which helps with debugging SAML.
- Site admins can now view user satisfaction surveys grouped by user, in addition to chronological order, and aggregate summary values (including the average score and the net promoter score over the last 30 days) are now displayed.
- The site admin overview page displays the site ID, the primary admin email, and premium feature usage information.
- Added Haskell as an experimental language server on the code intelligence admin page.

## 2.8.0

### Changed

- `gitMaxConcurrentClones` now also limits the concurrency of updates to repos in addition to the initial clone.
- In the GraphQL API, `site.users` has been renamed to `users`, `site.orgs` has been renamed to `organizations`, and `site.repositories` has been renamed to `repositories`.
- An authentication provider must be set in site configuration (see [authentication provider documentation](https://docs.sourcegraph.com/admin/auth)). Previously the server defaulted to builtin auth if none was set.
- If a process dies inside the Sourcegraph container the whole container will shut down. We suggest operators configure a [Docker Restart Policy](https://docs.docker.com/config/containers/start-containers-automatically/#restart-policy-details) or a [Kubernetes Restart Policy](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#restart-policy). Previously the container would operate in a degraded mode if a process died.
- Changes to the `auth.public` site config are applied immediately in `sourcegraph/server` (no restart needed).
- The new search timeout behavior is now enabled by default. Set `"experimentalFeatures": {"searchTimeoutParameter": "disabled"}` in site config to disable it.
- Search includes files up to 1MB (previous limit was 512KB for unindexed search and 128KB for indexed search).
- Usernames and email addresses reported by OpenID Connect and SAML auth providers are now trusted, and users will sign into existing Sourcegraph accounts that match on the auth provider's reported username or email.
- The repository sidebar file tree is much, much faster on massive repositories (200,000+ files)
- The SAML authentication provider was significantly improved. Users who were signed in using SAML previously will need to reauthenticate via SAML next time they visit Sourcegraph.
- The SAML `serviceProviderCertificate` and `serviceProviderPrivateKey` site config properties are now optional.

### Fixed

- Fixed an issue where Index Search status page failed to render.
- User data on the site admin Analytics page is now paginated, filterable by a user's recent activity, and searchable.
- The link to the root of a repository in the repository header now preserves the revision you're currently viewing.
- When using the `http-header` auth provider, signin/signup/signout links are now hidden.
- Repository paths beginning with `go/` are no longer reservered by Sourcegraph.
- Interpret `X-Forwarded-Proto` HTTP header when `httpToHttpsRedirect` is set to `load-balanced`.
- Deleting a user account no longer prevents the creation of a new user account with the same username and/or association with authentication provider account (SAML/OpenID/etc.)
- It is now possible for a user to verify an email address that was previously associated with now-deleted user account.
- Diff searches over empty repositories no longer fail (this was not an issue for Sourcegraph cluster deployments).
- Stray `tmp_pack_*` files from interrupted fetches should now go away.
- When multiple `repo:` tokens match the same repo, process @revspec requirements from all of them, not just the first one in the search.

### Removed

- The `ssoUserHeader` site config property (deprecated since January 2018) has been removed. The functionality was moved to the `http-header` authentication provider.
- The experiment flag `showMissingReposEnabled`, which defaulted to enabled, has been removed so it is no longer possible to disable this feature.
- Event-level telemetry has been completely removed from self-hosted Sourcegraph instances. As a result, the `disableTelemetry` site configuration option has been deprecated. The new site-admin Pings page clarifies the only high-level telemetry being sent to Sourcegraph.com.
- The deprecated `adminUsernames` site config property (deprecated since January 2018) has been removed because it is no longer necessary. Site admins can designate other users as site admins in the site admin area, and the first user to sign into a new instance always becomes a site admin (even when using an external authentication provider).

### Added

- The new repository contributors page (linked from the repository homepage) displays the top Git commit authors in a repository, with filtering options.
- Custom language servers in the site config may now specify a `metadata` property containing things like homepage/docs/issues URLs for the language server project, as well as whether or not the language server should be considered experimental (not ready for prime-time). This `metadata` will be displayed in the UI to better communicate the status of a language server project.
- Access tokens now have scopes (which define the set of operations they permit). All access tokens still provide full control of all resources associated with the user account (the `user:all` scope, which is now explicitly displayed).
- The new access token scope `site-admin:sudo` allows the holder to perform any action as any other user. Only site admins may create this token.
- Links to Sourcegraph's changelog have been added to the site admin Updates page and update alert.
- If the site configuration is invalid or uses deprecated properties, a global alert will be shown to all site admins.
- There is now a code intelligence status indicator when viewing files. It contains information about the capabailities of the language server that is providing code intelligence for the file.
- Java code intelligence can now be enabled for repositories that aren't automatically supported using a
  `javaconfig.json` file. For Gradle plugins, this file can be generated using
  the [Javaconfig Gradle plugin](https://docs.sourcegraph.com/extensions/language_servers/java#gradle-execution).
- The new `auth.providers` site config is an array of authentication provider objects. Currently only 1 auth provider is supported. The singular `auth.provider` is deprecated.
- Users authenticated with OpenID Connect are now able to sign out of Sourcegraph (if the provider supports token revocation or the end-session endpoint).
- Users can now specify the number of days, weeks, and months of site activity to query through the GraphQL API.
- Added 14 new experimental language servers on the code intelligence admin page.
- Added `httpStrictTransportSecurity` site configuration option to customize the Strict-Transport-Security HTTP header. It defaults to `max-age=31536000` (one year).
- Added `nameIDFormat` in the `saml` auth provider to set the SAML NameID format. The default changed from transient to persistent.
- (This feature has been removed.) Experimental env var expansion in site config JSON: set `SOURCEGRAPH_EXPAND_CONFIG_VARS=1` to replace `${var}` or `$var` (based on environment variables) in any string value in site config JSON (except for JSON object property names).
- The new (optional) SAML `serviceProviderIssuer` site config property (in an `auth.providers` array entry with `{"type":"saml", ...}`) allows customizing the SAML Service Provider issuer name.
- The site admin area now has an "Auth" section that shows the enabled authentication provider(s) and users' external accounts.

## 2.7.6

### Fixed

- If a user's account is deleted, session cookies for that user are no longer considered valid.

## 2.7.5

### Changed

- When deploying Sourcegraph to Kubernetes, RBAC is now used by default. Most Kubernetes clusters require it. See the Kubernetes installation instructions for more information (including disabling if needed).
- Increased git ssh connection timeout to 30s from 7s.
- The Phabricator integration no longer requires staging areas, but using them is still recommended because it improves performance.

### Fixed

- Fixed an issue where language servers that were not enabled would display the "Restart" button in the Code Intelligence management panel.
- Fixed an issue where the "Update" button in the Code Intelligence management panel would be displayed inconsistently.
- Fixed an issue where toggling a dynamic search scope would not also remove `@rev` (if specified)
- Fixed an issue where where modes that can only be determined by the full filename (not just the file extension) of a path weren't supported (Dockerfiles are the first example of this).
- Fixed an issue where the GraphiQL console failed when variables are specified.
- Indexed search no longer maintains its own git clones. For Kubernetes cluster deployments, this significantly reduces disk size requirements for the indexed-search pod.
- Fixed an issue where language server Docker containers would not be automatically restarted if they crashed (`sourcegraph/server` only).
- Fixed an issue where if the first user on a site authenticated via SSO, the site would remain stuck in uninitialized mode.

### Added

- More detailed progress information is displayed on pages that are waiting for repositories to clone.
- Admins can now see charts with daily, weekly, and monthly unique user counts by visiting the site-admin Analytics page.
- Admins can now host and see results from Sourcegraph user satisfaction surveys locally by setting the `"experimentalFeatures": { "hostSurveysLocally": "enabled"}` site config option. This feature will be enabled for all instances once stable.
- Access tokens are now supported for all authentication providers (including OpenID Connect and SAML, which were previously not supported).
- The new `motd` setting (in global, organization, and user settings) displays specified messages at the top of all pages.
- Site admins may now view all access tokens site-wide (for all users) and revoke tokens from the new access tokens page in the site admin area.

## 2.7.0

### Changed

- Missing repositories no longer appear as search results. Instead, a count of repositories that were not found is displayed above the search results. Hovering over the count will reveal the names of the missing repositories.
- "Show more" on the search results page will now reveal results that have already been fetched (if such results exist) without needing to do a new query.
- The bottom panel (on a file) now shows more tabs, including docstrings, multiple definitions, references (as before), external references grouped by repository, implementations (if supported by the language server), and file history.
- The repository sidebar file tree is much faster on massive repositories (200,000+ files)

### Fixed

- Searches no longer block if the index is unavailable (e.g. after the index pod restarts). Instead, it respects the normal search timeout and reports the situation to the user if the index is not yet available.
- Repository results are no longer returned for filters that are not supported (e.g. if `file:` is part of the search query)
- Fixed an issue where file tree elements may be scrolled out of view on page load.
- Fixed an issue that caused "Could not ensure repository updated" log messages when trying to update a large number of repositories from gitolite.
- When using an HTTP authentication proxy (`"auth.provider": "http-header"`), usernames are now properly normalized (special characters including `.` replaced with `-`). This fixes an issue preventing users from signing in if their username contained these special characters.
- Fixed an issue where the site-admin Updates page would incorrectly report that update checking was turned off when `telemetryDisabled` was set, even as it continued to report new updates.
- `repo:` filters that match multiple repositories and contain a revision specifier now correctly return partial results even if some of the matching repositories don't have a matching revision.
- Removed hardcoded list of supported languages for code intelligence. Any language can work now and support is determined from the server response.
- Fixed an issue where modifying `config.json` on disk would not correctly mark the server as needing a restart.
- Fixed an issue where certain diff searches (with very sparse matches in a repository's history) would incorrectly report no results found.
- Fixed an issue where the `langservers` field in the site-configuration didn't require both the `language` and `address` field to be specified for each entry

### Added

- Users (and site admins) may now create and manage access tokens to authenticate API clients. The site config `auth.disableAccessTokens` (renamed to `auth.accessTokens` in 2.11) disables this new feature. Access tokens are currently only supported when using the `builtin` and `http-header` authentication providers (not OpenID Connect or SAML).
- User and site admin management capabilities for user email addresses are improved.
- The user and organization management UI has been greatly improved. Site admins may now administer all organizations (even those they aren't a member of) and may edit profile info and configuration for all users.
- If SSO is enabled (via OpenID Connect or SAML) and the SSO system provides user avatar images and/or display names, those are now used by Sourcegraph.
- Enable new search timeout behavior by setting `"experimentalFeatures": { "searchTimeoutParameter": "enabled"}` in your site config.
  - Adds a new `timeout:` parameter to customize the timeout for searches. It defaults to 10s and may not be set higher than 1m.
  - The value of the `timeout:` parameter is a string that can be parsed by [time.Duration](https://golang.org/pkg/time/#ParseDuration) (e.g. "100ms", "2s").
  - When `timeout:` is not provided, search optimizes for retuning results as soon as possible and will include slower kinds of results (e.g. symbols) only if they are found quickly.
  - When `timeout:` is provided, all result kinds are given the full timeout to complete.
- A new user settings tokens page was added that allows users to obtain a token that they can use to authenticate to the Sourcegraph API.
- Code intelligence indexes are now built for all repositories in the background, regardless of whether or not they are visited directly by a user.
- Language servers are now automatically enabled when visiting a repository. For example, visiting a Go repository will now automatically download and run the relevant Docker container for Go code intelligence.
  - This change only affects when Sourcegraph is deployed using the `sourcegraph/server` Docker image (not using Kubernetes).
  - You will need to use the new `docker run` command at https://docs.sourcegraph.com/#quickstart in order for this feature to be enabled. Otherwise, you will receive errors in the log about `/var/run/docker.sock` and things will work just as they did before. See https://docs.sourcegraph.com/extensions/language_servers for more information.
- The site admin Analytics page will now display the number of "Code Intelligence" actions each user has made, including hovers, jump to definitions, and find references, on the Sourcegraph webapp or in a code host integration or extension.
- An experimental cross repository jump to definition which consults the OSS index on Sourcegraph.com. This is disabled by default; use `"experimentalFeatures": { "jumpToDefOSSIndex": "enabled" }` in your site configuration to enable it.
- Users can now view Git branches, tags, and commits, and compare Git branches and revisions on Sourcegraph. (The code host icon in the header takes you to the commit on the code host.)
- A new admin panel allows you to view and manage language servers. For Docker deployments, it allows you to enable/disable/update/restart language servers at the click of a button. For cluster deployments, it shows the current status of language servers.
- Users can now tweet their feedback about Sourcegraph when clicking on the feedback smiley located in the navbar and filling out a Twitter feedback form.
- A new button in the repository header toggles on/off the Git history panel for the current file.

## 2.6.8

### Bug fixes

- Searches of `type:repo` now work correctly with "Show more" and the `max` parameter.
- Fixes an issue where the server would crash if the DB was not available upon startup.

## 2.6.7

### Added

- The duration that the frontend waits for the PostgreSQL database to become available is now configurable with the `DB_STARTUP_TIMEOUT` env var (the value is any valid Go duration string).
- Dynamic search filters now suggest exclusions of Go test files, vendored files and node_modules files.

## 2.6.6

### Added

- Authentication to Bitbucket Server using username-password credentials is now supported (in the `bitbucketServer` site config `username`/`password` options), for servers running Bitbucket Server version 2.4 and older (which don't support personal access tokens).

## 2.6.5

### Added

- The externally accessible URL path `/healthz` performs a basic application health check, returning HTTP 200 on success and HTTP 500 on failure.

### Behavior changes

- Read-only forks on GitHub are no longer synced by default. If you want to add a readonly fork, navigate directly to the repository page on Sourcegraph to add it (e.g. https://sourcegraph.mycompany.internal/github.com/owner/repo). This prevents your repositories list from being cluttered with a large number of private forks of a private repository that you have access to. One notable example is https://github.com/EpicGames/UnrealEngine.
- SAML cookies now expire after 90 days. The previous behavior was every 1 hour, which was unintentionally low.

## 2.6.4

### Added

- Improve search timeout error messages
- Performance improvements for searching regular expressions that do not start with a literal.

## 2.6.3

### Bug fixes

- Symbol results are now only returned for searches that contain `type:symbol`

## 2.6.2

### Added

- More detailed logging to help diagnose errors with third-party authentication providers.
- Anchors (such as `#my-section`) in rendered Markdown files are now supported.
- Instrumentation section for admins. For each service we expose pprof, prometheus metrics and traces.

### Bug fixes

- Applies a 1s timeout to symbol search if invoked without specifying `type:` to not block plain text results. No change of behaviour if `type:symbol` is given explicitly.
- Only show line wrap toggle for code-view-rendered files.

## 2.6.1

### Bug fixes

- Fixes a bug where typing in the search query field would modify the expanded state of file search results.
- Fixes a bug where new logins via OpenID Connect would fail with the error `SSO error: ID Token verification failed`.

## 2.6.0

### Added

- Support for [Bitbucket Server](https://www.atlassian.com/software/bitbucket/server) as a codehost. Configure via the `bitbucketServer` site config field.
- Prometheus gauges for git clone queue depth (`src_gitserver_clone_queue`) and git ls-remote queue depth (`src_gitserver_lsremote_queue`).
- Slack notifications for saved searches may now be added for individual users (not just organizations).
- The new search filter `lang:` filters results by programming language (example: `foo lang:go` or `foo -lang:clojure`).
- Dynamic filters: filters generated from your search results to help refine your results.
- Search queries that consist only of `file:` now show files whose path matches the filters (instead of no results).
- Sourcegraph now automatically detects basic `$GOPATH` configurations found in `.envrc` files in the root of repositories.
- You can now configure the effective `$GOPATH`s of a repository by adding a `.sourcegraph/config.json` file to your repository with the contents `{"go": {"GOPATH": ["mygopath"]}}`.
- A new `"blacklistGoGet": ["mydomain.org,myseconddomain.com"]` offers users a quick escape hatch in the event that Sourcegraph is making unwanted `go get` or `git clone` requests to their website due to incorrectly-configured monorepos. Most users will never use this option.
- Search suggestions and results now include symbol results. The new filter `type:symbol` causes only symbol results to be shown.
  Additionally, symbols for a repository can be browsed in the new symbols sidebar.
- You can now expand and collapse all items on a search results page or selectively expand and collapse individual items.

### Configuration changes

- Reduced the `gitMaxConcurrentClones` site config option's default value from 100 to 5, to help prevent too many concurrent clones from causing issues on code hosts.
- Changes to some site configuration options are now automatically detected and no longer require a server restart. After hitting Save in the UI, you will be informed if a server restart is required, per usual.
- Saved search notifications are now only sent to the owner of a saved search (all of an organization's members for an organization-level saved search, or a single user for a user-level saved search). The `notifyUsers` and `notifyOrganizations` properties underneath `search.savedQueries` have been removed.
- Slack webhook URLs are now defined in user/organization JSON settings, not on the organization profile page. Previously defined organization Slack webhook URLs are automatically migrated to the organization's JSON settings.
- The "unlimited" value for `maxReposToSearch` is now `-1` instead of `0`, and `0` now means to use the default.
- `auth.provider` must be set (`builtin`, `openidconnect`, `saml`, `http-header`, etc.) to configure an authentication provider. Previously you could just set the detailed configuration property (`"auth.openIDConnect": {...}`, etc.) and it would implicitly enable that authentication provider.
- The `autoRepoAdd` site configuration property was removed. Site admins can add repositories via site configuration.

### Bug fixes

- Only cross reference index enabled repositories.
- Fixed an issue where search would return results with empty file contents for matches in submodules with indexing enabled. Searching over submodules is not supported yet, so these (empty) results have been removed.
- Fixed an issue where match highlighting would be incorrect on lines that contained multibyte characters.
- Fixed an issue where search suggestions would always link to master (and 404) even if the file only existed on a branch. Now suggestions always link to the revision that is being searched over.
- Fixed an issue where all file and repository links on the search results page (for all search results types) would always link to master branch, even if the results only existed in another branch. Now search results links always link to the revision that is being searched over.
- The first user to sign up for a (not-yet-initialized) server is made the site admin, even if they signed up using SSO. Previously if the first user signed up using SSO, they would not be a site admin and no site admin could be created.
- Fixed an issue where our code intelligence archive cache (in `lsp-proxy`) would not evict items from the disk. This would lead to disks running out of free space.

## 2.5.16, 2.5.17

- Version bump to keep deployment variants in sync.

## 2.5.15

### Bug fixes

- Fixed issue where a Sourcegraph cluster would incorrectly show "An update is available".
- Fixed Phabricator links to repositories
- Searches over a single repository are now less likely to immediately time out the first time they are searched.
- Fixed a bug where `auth.provider == "http-header"` would incorrectly require builtin authentication / block site access when `auth.public == "false"`.

### Phabricator Integration Changes

We now display a "View on Phabricator" link rather than a "View on other code host" link if you are using Phabricator and hosting on GitHub or another code host with a UI. Commit links also will point to Phabricator.

### Improvements to SAML authentication

You may now optionally provide the SAML Identity Provider metadata XML file contents directly, with the `auth.saml` `identityProviderMetadata` site configuration property. (Previously, you needed to specify the URL where that XML file was available; that is still possible and is more common.) The new option is useful for organizations whose SAML metadata is not web-accessible or while testing SAML metadata configuration changes.

## 2.5.13

### Improvements to builtin authentication

When using `auth.provider == "builtin"`, two new important changes mean that a Sourcegraph server will be locked down and only accessible to users who are invited by an admin user (previously, we advised users to place their own auth proxy in front of Sourcegraph servers).

1.  When `auth.provider == "builtin"` Sourcegraph will now by default require an admin to invite users instead of allowing anyone who can visit the site to sign up. Set `auth.allowSignup == true` to retain the old behavior of allowing anyone who can access the site to signup.
2.  When `auth.provider == "builtin"`, Sourcegraph will now respects a new `auth.public` site configuration option (default value: `false`). When `auth.public == false`, Sourcegraph will not allow anyone to access the site unless they have an account and are signed in.

## 2.4.3

### Added

- Code Intelligence support
- Custom links to code hosts with the `links:` config options in `repos.list`

### Changed

- Search by file path enabled by default

## 2.4.2

### Added

- Repository settings mirror/cloning diagnostics page

### Changed

- Repositories added from GitHub are no longer enabled by default. The site admin UI for enabling/disabling repositories is improved.

## 2.4.0

### Added

- Search files by name by including `type:path` in a search query
- Global alerts for configuration-needed and cloning-in-progress
- Better list interfaces for repositories, users, organizations, and threads
- Users can change their own password in settings
- Repository groups can now be specified in settings by site admins, organizations, and users. Then `repogroup:foo` in a search query will search over only those repositories specified for the `foo` repository group.

### Changed

- Log messages are much quieter by default

## 2.3.11

### Added

- Added site admin updates page and update checking
- Added site admin telemetry page

### Changed

- Enhanced site admin panel
- Changed repo- and SSO-related site config property names to be consistent, updated documentation

## 2.3.10

### Added

- Online site configuration editing and reloading

### Changed

- Site admins are now configured in the site admin area instead of in the `adminUsernames` config key or `ADMIN_USERNAMES` env var. Users specified in those deprecated configs will be designated as site admins in the database upon server startup until those configs are removed in a future release.

## 2.3.9

### Fixed

- An issue that prevented creation and deletion of saved queries

## 2.3.8

### Added

- Built-in authentication: you can now sign up without an SSO provider.
- Faster default branch code search via indexing.

### Fixed

- Many performance improvements to search.
- Much log spam has been eliminated.

### Changed

- We optionally read `SOURCEGRAPH_CONFIG` from `$DATA_DIR/config.json`.
- SSH key required to clone repositories from GitHub Enterprise when using a self-signed certificate.

## 0.3 - 13 December 2017

The last version without a CHANGELOG.
