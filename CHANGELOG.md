<!--
###################################### READ ME ###########################################
### This changelog should always be read on `main` branch. Its contents on version     ###
### branches do not necessarily reflect the changes that have gone into that branch.   ###
### To update the changelog add your changes to the appropriate section under the      ###
### "Unreleased" heading.                                                              ###
##########################################################################################
-->

**Note**: this is no longer used! Please see
https://www.notion.so/sourcegraph/Writing-a-changelog-entry-dd997f411d524caabf0d8d38a24a878c
for instructions on how to write changelog entries in your PR description.

# Changelog

All notable changes to Sourcegraph are documented in this file.

<!-- START CHANGELOG -->

## Unreleased

### Added

- Basic support for rendering Jupyter notebooks in the Sourcegraph web app. ([#59685](https://github.com/sourcegraph/sourcegraph/pull/59685))
- Mermaid diagrams in Markdown are now rendered in the Sourcegraph web app ([#62678](https://github.com/sourcegraph/sourcegraph/pull/62678))
- A feature flag for Cody, `completions.smartContextWindow` is added and set to "enabled" by default. It allows clients to adjust the context window based on the name of the chat model. When smartContextWindow is enabled, the `completions.chatModelMaxTokens` value is ignored. ([#62802](https://github.com/sourcegraph/sourcegraph/pull/62802))
- Code Insights: When facing the "incomplete datapoints" warning, you can now use GraphQL to discover which repositories had problems. The schemas for `TimeoutDatapointAlert` and `GenericIncompleteDatapointAlert` now contain an additional `repositories` field. ([#62756](https://github.com/sourcegraph/sourcegraph/pull/62756)).
- Users will now be presented with a modal that reminds them to connect any external code host accounts that's required for permissions. Without these accounts connected, users may be unable to view repositories that they otherwise have access to. [#62983](https://github.com/sourcegraph/sourcegraph/pull/62983)
- Added support for Google as an LLM provider for Cody, with the following models available through Cody Gateway: Gemini Pro (`gemini-pro-latest`), Gemini 1.5 Flash (`gemini-1.5-flash-latest`), and Gemini 1.5 Pro (`gemini-1.5-pro-latest`). [#63053](https://github.com/sourcegraph/sourcegraph/pull/63053)
- Added syntax highlighting for the Magik programming language. [#62919](https://github.com/sourcegraph/sourcegraph/pull/62919)
- Added syntax highlighting for the Hack programming language. [#62770](https://github.com/sourcegraph/sourcegraph/pull/62770)

### Changed

- The default and recommended chat model for Anthropic and Cody Gateway configurations is now `claude-3-sonnet-20240229`. [#62757](https://github.com/sourcegraph/sourcegraph/pull/62757)
- The default and recommended autocomplete model for Cody Gateway configurations is now `fireworks/starcoder`. [#62757](https://github.com/sourcegraph/sourcegraph/pull/62757)
- Code Insights: Language Stats Insights performance improved by another 70-90%. It's now able to handle repositories above 40 GB. [#62946](https://github.com/sourcegraph/sourcegraph/pull/62946)

### Fixed

- Pressing the numpad `Enter` key will now cycle through in-file search results [#62665](https://github.com/sourcegraph/sourcegraph/pull/62665)
- Providing an access token via the [`SRC_ACCESS_TOKEN`](https://sourcegraph.com/docs/cli/how-tos/creating_an_access_token) environment variable is now mandatory for uploading SCIP indexes using [src-cli](https://sourcegraph.com/docs/cli). [#62573](https://github.com/sourcegraph/sourcegraph/pull/62573)
- Fixed several conditions that could cause a repository being incorrectly marked as modified during code host syncing. For these cases, unnecessary git fetches were triggered. [#62837](https://github.com/sourcegraph/sourcegraph/pull/62837)
- Fixed usernames getting random suffixes added during SCIM provisioning. [#63122](https://github.com/sourcegraph/sourcegraph/pull/63122)
- Removed experimental pattern type "lucky" from the list of supported default pattern types. [#63486](https://github.com/sourcegraph/sourcegraph/pull/63486)

## 5.4.2198

### Added

### Changed

### Fixed

- Fix a bug that caused Bedrock Provisioned Throughput model names to fail [#62695](https://github.com/sourcegraph/sourcegraph/pull/62695) ~~[#62642](https://github.com/sourcegraph/sourcegraph/pull/62642)~~
- Fixed an issue where usernames were too eagerly suffixed with a random ID to prevent duplicates which could cause issues with Bitbucket Server permissions syncing where exact username matches are required. [#62747](https://github.com/sourcegraph/sourcegraph/pull/62747)
- Fixed an issue where code graph configuration policies would miscategorize lightweight tags as branches. [#62671](https://github.com/sourcegraph/sourcegraph/pull/62671)
- Fixed a bug in batch changes scheduling that could cause excessive CPU usage. [#62597](https://github.com/sourcegraph/sourcegraph/pull/62597)
- Cody ignore: update version constraint error message [#62654](https://github.com/sourcegraph/sourcegraph/pull/62654)

## 5.4.0

### Added

- Added rate and latency instrumentation for git / package repository syncing operations. These are visible in the gitserver dashboards (VCS "Clone/Fetch/IsCloneable" Metrics). [#61708](https://github.com/sourcegraph/sourcegraph/pull/61708)
- Added syntax highlighting for the [Pkl](https://pkl-lang.org/) configuration language. [#61478](https://github.com/sourcegraph/sourcegraph/pull/61478)
- New `rev:at.time()` search filter that allows you to search a branch at a point in time. [#61513](https://github.com/sourcegraph/sourcegraph/pull/61513)
- "cody.contextFilters" field to the site config. Admins can set include and exclude rules to define which repositories Cody can use as context in its requests to third-party LLMs. [#61101](https://github.com/sourcegraph/sourcegraph/pull/61101), [#61641](https://github.com/sourcegraph/sourcegraph/pull/61641)
- Added whether "cody.contextFilters" field is configured in the site config to pings. [#62080](https://github.com/sourcegraph/sourcegraph/pull/62080)
- Added exhaustive logging for all gRPC requests sent to gitserver. This feature is off by default, and can be enabled by setting the `SRC_GITSERVER_EXHAUSTIVE_LOGGING_ENABLED` environment variable to `true`. [#61270](https://github.com/sourcegraph/sourcegraph/pull/61270)
- Added an `internal` option for the `repositoryQuery` field for GitHub Enterprise code host connections. This will mirror all internal repositories on the code host. [#61924](https://github.com/sourcegraph/sourcegraph/pull/61924)

### Changed

- Improved syntax highlighting for Dart. [#58480](https://github.com/sourcegraph/sourcegraph/pull/58480)
- The default noop Event type in the honey package has been replaced with a new type that aggregates fields in memory for testing and logging purposes. [#61854](https://github.com/sourcegraph/sourcegraph/pull/61854)
- Improved the performance of Language Stats Insights by 50-70% by increasing the concurrent requests from the frontend to the gitserver from 1 to 4. You can override the concurrency with the `GET_INVENTORY_GIT_SERVER_CONCURRENCY` environment variable. [#62011](https://github.com/sourcegraph/sourcegraph/pull/62011)
- Raised the backend timeout for Language Stats Insights from 3 minutes to 5 minutes. You can override this with the `GET_INVENTORY_TIMEOUT` environment variable. [#62011](https://github.com/sourcegraph/sourcegraph/pull/62011)
- Code insights drilldown behavior has been changed from a diff search to a point-in-time search with the new `rev:at.time()`. [#61953](https://github.com/sourcegraph/sourcegraph/pull/61953)
- The `FirstEverCommit` gitserver client method has been changed to use a new bespoke gRPC endpoint instead of the legacy `exec` endpoint. [#62173](https://github.com/sourcegraph/sourcegraph/pull/62173)
- The `GetBehindAhead` gitserver client method has been changed to use a new bespoke gRPC endpoint instead of the legacy `exec` endpoint. [#62217](https://github.com/sourcegraph/sourcegraph/pull/62217)
- All uses of the `DiffSymbols` gitserver client method have been replaced with the new `ChangedFiles` method. As such, the `DiffSymbos` method has been removed [#62355](https://github.com/sourcegraph/sourcegraph/pull/62355)

### Fixed

- Updated the Docker-in-Docker image to 26.0.0 to resolve several vulnerabilities. [#61735](https://github.com/sourcegraph/sourcegraph/pull/61735)
- The GetCommit() RPC in the gitserver service now uses the correct protobuf type that allows for non-utf8 byte sequences in commit messages, author names, and author emails. [#61940](https://github.com/sourcegraph/sourcegraph/pull/61940)
- The ArchiveReader() RPC in the gitserver service now uses the correct protobuf type that allows for non-utf8 byte sequences in file paths. [#61970](https://github.com/sourcegraph/sourcegraph/pull/61970)
- Pinned code intel popovers and popovers opened via the keyboard are properly shown again. [#61966](https://github.com/sourcegraph/sourcegraph/pull/61966)
- Syntax highlighting works correctly for JSX files. [#62027](https://github.com/sourcegraph/sourcegraph/pull/62027)
- Changesets with a skipped CI check now have their CI status correctly displayed in the Batch Changes UI. [#62204](https://github.com/sourcegraph/sourcegraph/pull/62204)
- Fixed the Sourcegraph login page auto-redirecting to the single auth provider when request access is enabled. [#62376](https://github.com/sourcegraph/sourcegraph/pull/62376)
- Fixed bug for Python classes with decorators and C# classes with attributes that prevented symbol search (`type:symbol`) and the symbol sidebar from detecting them properly: [#62345](https://github.com/sourcegraph/sourcegraph/pull/62345)

## 5.3.12303

### Added

- Indexed-search now supports draining a replica of indexes to support zero-downtime reduction in cluster size. [#62005](https://github.com/sourcegraph/sourcegraph/pull/62005)

### Fixed

- Rust binaries are now built in release mode to avoid unnecessary debug checks. [#61740](https://github.com/sourcegraph/sourcegraph/pull/61740)
- Fixed how scip-ctags reports errors to avoid failing search indexing on non-fatal errors. [#61712](https://github.com/sourcegraph/sourcegraph/pull/61712)
- Fixed a bug in Enterprise Cody context for queries containing only stopwords. [#61848](https://github.com/sourcegraph/sourcegraph/pull/61848), [#62026](https://github.com/sourcegraph/sourcegraph/pull/62026)
- Fixed the instance dropdown on the zoekt grafana dashboard. [#61836](https://github.com/sourcegraph/sourcegraph/pull/61836)

## 5.3.11625

### Changed

- Notices configured in the site config now allow for specifying a style or color. [#61338](https://github.com/sourcegraph/sourcegraph/pull/61338)
- Reduce spamming error logs when canceling symbols indexing or canceling a syntax highlighting request. [#61880](https://github.com/sourcegraph/sourcegraph/pull/61880), [#61719](https://github.com/sourcegraph/sourcegraph/pull/61719), [#61732](https://github.com/sourcegraph/sourcegraph/pull/61732)

### Fixed

- Fixed a bug where the `src batch preview` command could fail due to an incorrect file-not-found error. [#61984](https://github.com/sourcegraph/sourcegraph/pull/61984)
- Fixed a bug where the Roles page in the Site Admin view was inaccessible. [#61738](https://github.com/sourcegraph/sourcegraph/pull/61738)
- Fixed a panic in Cody Attribution in sourcegraph-frontend when reporting an error. [#60439](https://github.com/sourcegraph/sourcegraph/issues/60439)

## 5.3.9104

### Added

- GitHub app installations can now be refreshed from the Batch Changes Site Admin page. [#60125](https://github.com/sourcegraph/sourcegraph/pull/60125)
- The SAML auth provider configuration now supports a `usernameAttributeNames` field that can be used to specify a list of SAML attribute that should be used as the username. [#60603](https://github.com/sourcegraph/sourcegraph/pull/60603)
- Added the GraphQL query `User.evaluateFeatureFlag` to show if a feature flag is enabled or disabled for a user. [#60828](https://github.com/sourcegraph/sourcegraph/pull/60828)
- Search Jobs now supports diff, commit and path searches. Before, only file searches were supported. [#60883](https://github.com/sourcegraph/sourcegraph/pull/60883)
- Auth providers now support a `noSignIn` option that, when set to true, will hide the auth provider from the sign in page, but still allow users to connect the external account from their Account Security page for permissions syncing. [#60722](https://github.com/sourcegraph/sourcegraph/pull/60722)
- Added a "Commits" button to the folders in repos that shows commits for the items in that folder. [#60909](https://github.com/sourcegraph/sourcegraph/pull/60909)
- The frontend Grafana dashboard has a new Prometheus metric that tracks the rate of requests that Sourcegraph issues to external services. [#61348](https://github.com/sourcegraph/sourcegraph/pull/61348)
- Added support for the `gitURLType` setting for Gerrit, Sourcegraph now supports cloning from Gerrit via SSH. Note: Not on Cloud yet, like for all code hosts. [#61537](https://github.com/sourcegraph/sourcegraph/pull/61537)
- Support for OpenAI chat models for enterprise customers. [#61539](https://github.com/sourcegraph/sourcegraph/pull/61539)
- Added support for explicitly enumerating repositories to sync from Bitbucket Cloud. Previously, Sourcegraph would automatically sync all repositories from a Bitbucket Cloud workspace. [#61536](https://github.com/sourcegraph/sourcegraph/pull/61536)

### Changed

- GitHub apps installation records will only be deleted from the database if the GitHub App has been uninstalled or if the GitHub app has been deleted. [#60460](https://github.com/sourcegraph/sourcegraph/pull/60460)
- The Anthropic provider for Cody has been updated to use the messages API which includes support for Claude 3 models. This is applicable to both BYOK and Cody Gateway users. The messages API does not support model identifiers which only set a major model version such as: `claude-2`, `claude-instant-v1` and `claude-instant-1`. Default values have been updated to `claude-2.0` and `claude-instant-1.2`, any legacy models identifiers in the site config will be set to the corresponding default previously mentioned. [#60953](https://github.com/sourcegraph/sourcegraph/pull/60953) [#61324](https://github.com/sourcegraph/sourcegraph/pull/61324)
- The AWS Bedrock provider for Cody has been updated to use Anthropic's Messages API, bringing support for Claude 3 models. [#61347](https://github.com/sourcegraph/sourcegraph/pull/61347)
- Improved the ranking of branches for repositories with a lot of branches. Branches in the branch selector are now always sorted by recency, with HEAD at the top. [#60323](https://github.com/sourcegraph/sourcegraph/pull/60323)
- Notices configured in the site config now allow for specifying a style or color. [#61338](https://github.com/sourcegraph/sourcegraph/pull/61338)

### Fixed

- Code Monitors now properly ignores monitors associated with soft-deleted users, which previously would have led to an error on the overview page. [#60405](https://github.com/sourcegraph/sourcegraph/pull/60405)
- Fixed a bug where clicking "Exclude Repo" on Azure DevOps or Gerrit repositories would not work. [#60509](https://github.com/sourcegraph/sourcegraph/pull/60509)
- Links in codeintel popovers respect the revision from the URL. [#60545](https://github.com/sourcegraph/sourcegraph/pull/60545)
- Fixed an issue where repositories with a name ending in `.git` failed to clone. [#60627](https://github.com/sourcegraph/sourcegraph/pull/60627)
- Fixed an issue where Sourcegraph could lose track of repositories on gitserver, leaving behind unnecessary data and inconsistent clone status in the UI. [#60627](https://github.com/sourcegraph/sourcegraph/pull/60627)
- The "Commits" button in repository and folder pages links to commits in the current revision instead of in the default branch. [#61408](https://github.com/sourcegraph/sourcegraph/pull/61408)
- Fixed an issue where code insights queries would fail if there are more than 65535 restricted repositories and regular expressions. [#61580](https://github.com/sourcegraph/sourcegraph/pull/61580)
- The "Commits" button in repository and folder pages uses Perforce language and links to `/-/changelists` for Perforce depots when the experimental feature `perforceChangelistMapping` is enabled. [#61408](https://github.com/sourcegraph/sourcegraph/pull/61408)
- Selecting "View blame prior to this change" on a file that was moved will now correctly navigate to the old file location at the specified commit. [#61577](https://github.com/sourcegraph/sourcegraph/pull/61577)
- Git blame performance on large files with a large number of commits has been drastically improved. [#61577](https://github.com/sourcegraph/sourcegraph/pull/61577)
- Code Insights now properly ignores search API alerts, which previously would have led to a code insight error. [#61431](https://github.com/sourcegraph/sourcegraph/pull/61431)

## 5.3.3

### Added

### Changed

- Changed the Azure OpenAI Cody provider to use the stable 2023-05-15 api version, due to the retirement of previous preview api versions. [61005](https://github.com/sourcegraph/sourcegraph/pull/61005)

### Fixed

- Fixed an issue in our build process that broke tooltips and validation in the settings editors. [#60808](https://github.com/sourcegraph/sourcegraph/pull/60808)

- Fixes a bug where the reference panel would not show any definitions or references for Protocol Buffers (and other languages where the name contained a space). [#60987](https://github.com/sourcegraph/sourcegraph/pull/60987)

- Fixed a bug where permission syncs could be scheduled for repositories or users even when a sync is already scheduled or in progress, leading to significant delays in the permissions sync system as a whole. [#61024](https://github.com/sourcegraph/sourcegraph/pull/61024)

- Fixed a bug in gitserver where it was possible to use expired Github App authorization tokens when syncing a large repository. Now, gitserver will use the latest tokens for each step of the syncing process (and refresh them if necessary). [#61179](https://github.com/sourcegraph/sourcegraph/pull/61179/)

### Removed

## 5.3.2

### Fixed

- A bug in search that could trigger a panic
- An unintentional change to the search results when using the LineMatch API which would include surrounding lines with no matches
- Autoupgrade only looks for open db connections from the Sourcegraph application services, and disregards other applications connected to the postgres instance. [#60771](https://github.com/sourcegraph/sourcegraph/pull/60771)
- Fixes a bug where hovers would not show up in C++ headers with the `.hxx` extension. [#60662](https://github.com/sourcegraph/sourcegraph/pull/60662)

## 5.3.1

### Fixed

- Updated container images to fix CVE-2023-4408, CVE-2023-50387, CVE-2023-50868, CVE-2023-5517, CVE-2023-5679, CVE-2023-6516

## 5.3.0

### Added

- The search bar now supports keyword search by default, which ANDs terms together instead of searching literally as before. The behavior can be disabled through a toggle on the search results page. [#58815](https://github.com/sourcegraph/sourcegraph/issues/58815)
- The `has.topic` filter now supports filtering by Gitlab topics. [#57649](https://github.com/sourcegraph/sourcegraph/pull/57649)
- Batch Changes now allows changesets to be exported in CSV and JSON format. [#56721](https://github.com/sourcegraph/sourcegraph/pull/56721)
- Supports custom ChatCompletion models in Cody clients for dotcom users. [#58158](https://github.com/sourcegraph/sourcegraph/pull/58158)
- Topics synced from GitHub and GitLab are now displayed for repository matches in the search results and on the repository tree page. [#58927](https://github.com/sourcegraph/sourcegraph/pull/58927)
- Added a new column "Repository metadata JSON" to the CSV export of repository search results, which includes the JSON encoded object of metadata key-value pairs. [#59334](https://github.com/sourcegraph/sourcegraph/pull/59334)
- Expiry to access tokens. Users can now select a maximum timespan for which a token is valid. Tokens will automatically lose access after this period. Default timeframes and an override to disable access tokens without expiration can be configured in the `auth.accessTokens` section of the site configuration. [#59565](https://github.com/sourcegraph/sourcegraph/pull/59565)
- Gerrit code host connections now support an 'exclude' field that prevents repos in this list from being synced. [#59739](https://github.com/sourcegraph/sourcegraph/pull/59739)
- Limit the number of active access tokens for a user. By default users are able to have 25 active access tokens. This limit can be configured using the `maxTokensPerUser` setting in the `auth.accessTokens` section of the site configuration. [#59731](https://github.com/sourcegraph/sourcegraph/pull/59731)
- Add experimental support for .cody/ignore when retrieving remote context. To enable it, set `experimentalFeatures.codyContextIgnore: true` in the site configuration. [#59836](https://github.com/sourcegraph/sourcegraph/pull/59836), [#59907](https://github.com/sourcegraph/sourcegraph/pull/59907)
- Site admin, link to the Cody Analytics service. [#60371](https://github.com/sourcegraph/sourcegraph/pull/60371)
- Added a reimagined filter panel to the search result page, facilitating a workflow centered around iterative refinement.
- Search results were treated to a design refresh, improving information density of results. [#59834](https://github.com/sourcegraph/sourcegraph/pull/59834)
- Added a preview pane to file search results so you can view the full file without navigating away from the search results. [#58311](https://github.com/sourcegraph/sourcegraph/pull/58311)

### Changed

- `cody.restrictUsersFeatureFlag` has been deprecated and replaced by role based access control instead. Until the old configuration value is removed from your site config, it will be respected just as before but with a warning displayed at the top of Sourcegraph. Once removed, the old feature flag will not be respected and instead the Cody access will be managed via role based access controls, see [the docs](https://docs.sourcegraph.com/cody/overview/enable-cody-enterprise#enable-cody-only-for-some-users) for more information. [#58831](https://github.com/sourcegraph/sourcegraph/pull/58831)
- The setting `experimentalFeatures.searchQueryInput` now refers to the new query input as `v2` (not `experimental`). <!-- NOTE: If v2 becomes the default before this release is cut, then update this entry to mention that instead of adding a separate entry. -->
- Search-based code intel doesn't include the currently selected search context anymore. It was possible to get into a situation where search-based code intel wouldn't find any information due to being restricted by the current search context. [#58010](https://github.com/sourcegraph/sourcegraph/pull/58010)
- The last commit which changed a file/directory is now shown in the files panel on the repo and file pages. To avoid duplicating information and confusion, the commits panel was removed. [58328](https://github.com/sourcegraph/sourcegraph/pull/58328)
- Clicking on a search result now opens the blob view at the same commit as the search result. Before, blob views were opened at the tip of the default branch, which sometimes caused inconsistencies in line numbers if the index was out of date. [#58381](https://github.com/sourcegraph/sourcegraph/pull/58381)
- The `exclude` configuration for code host configuration has been updated to allow chaining multiple conditions together and filtering GitHub repositories based on their size or number of GitHub stars. [#58377](https://github.com/sourcegraph/sourcegraph/pull/58377) and [#58405](https://github.com/sourcegraph/sourcegraph/pull/58405)
  - Multiple attributes on _a single_ `exclude` entry now have to be all true for a repository to be excluded. Example: `{"exclude": [{"name": "github.com/example/example"}, {"id": "my-id"}]}` will _only_ exclude repositories that have the name _and_ the id mentioned.
  - GitHub code host connections can exclude by size and stars now: `{"exclude": [{"name": "github.com/example/example"}, {"stars": "< 100", "size": ">= 1GB"}]}`
  - For `size` and `stars` the supported operators are `<`, `>`, `<=`, `>=`.
  - For `size` the supported units are `B`, `b`, `kB`, `KB`, `kiB`, `KiB`, `MiB`, `MB`, `GiB`, `GB`. No decimals points are supported.
- Structural Search is now disabled by default. To enable it, set `experimentalFeatures.structuralSearch: "enabled"` in the site configuration. [#57584](https://github.com/sourcegraph/sourcegraph/pull/57584)
- Search Jobs switches the format of downloaded results from CSV to JSON. [#59619](https://github.com/sourcegraph/sourcegraph/pull/59619)
- [Search Jobs](https://docs.sourcegraph.com/code_search/how-to/search-jobs) is now in beta and enabled by default. It can be disabled in the site configuration by setting `experimentalFeatures.searchJobs: false`.
- The search input on the search homepage is now automatically focused when the page loads.
- gRPC is now the only method for our internal APIs, and can not be disabled. All of corresponding the REST implementations have been removed. The vast majority of customers upgrading to 5.3 don't need to take any action - the change should be invisible. However, if you have restrictions on Sourcegraph’s internal (service to service) traffic, some firewall or security configurations may be necessary. You can downgrade to Sourcegraph 5.2 and disable gRPC while you troubleshoot / reach out to our customer support team. See [https://sourcegraph.com/docs/admin/updates/grpc](https://sourcegraph.com/docs/admin/updates/grpc) for more details. [#59093](https://github.com/sourcegraph/sourcegraph/pull/59093)
- The default `count:` for search has been increased to 10000, significantly increasing the number of searches that are exhaustive by default. [#60114](https://github.com/sourcegraph/sourcegraph/pull/60114)

### Fixed

- Site configuration edit history no longer breaks when the user that made the edit is deleted. [#57656](https://github.com/sourcegraph/sourcegraph/pull/57656)
- Drilling down into an insights query no longer mangles `content:` fields in your query. [#57679](https://github.com/sourcegraph/sourcegraph/pull/57679)
- The blame column now shows correct blame information when a hunk starts in a folded code section. [#58042](https://github.com/sourcegraph/sourcegraph/pull/58042)
- Fixed a bug where typing in the GraphQL editor in the Site Admin API console could cause the cursor to jump to the start of the editor. [#57862](https://github.com/sourcegraph/sourcegraph/pull/57862)
- The blame column no longer ignores whitespace-only changes by default. [#58134](https://github.com/sourcegraph/sourcegraph/pull/58134)
- Long lines now wrap correctly in the diff view. [#58138](https://github.com/sourcegraph/sourcegraph/pull/58138)
- Fixed an issue in the search input where pressing Enter after selecting a suggestion would sometimes insert another suggestions instead of submitting the query. [#58186](https://github.com/sourcegraph/sourcegraph/pull/58186)
- Fixed an issue where having sub-repo permissions enabled could cause repositories with a large number of files in directories to become unviewable. [#59420](https://github.com/sourcegraph/sourcegraph/pull/59420)
- On the search context, code monitoring, code insights, saved searches or notebook pages, when selecting a repository or file suggestion in the query input with Enter the suggestion is now properly appended to the query instead of navigating away to the corresponding repository or file page. [#59941](https://github.com/sourcegraph/sourcegraph/pull/59941)
- Perforce email matching for user permissions is now case insensitive, matching Perforce behavior. [#60252](https://github.com/sourcegraph/sourcegraph/pull/60252)
- A bug with syncing GitHub App installations that caused installations to be truncated after the first 30 orgs. [#60383](https://github.com/sourcegraph/sourcegraph/pull/60383)
- Various significant performance optimizations in the search and code navigation user flows.

### Removed

- The experimental GraphQL query `User.invitableCollaborators`.
- The following experimental settings in site-configuration are now deprecated and will not be read anymore: `maxReorderQueueSize`, `maxQueueMatchCount`, `maxReorderDurationMS`. [#57468](https://github.com/sourcegraph/sourcegraph/pull/57468)
- The feature-flag `search-ranking`, which allowed to disable the improved ranking introduced in 5.1, is now deprecated and will not be read anymore. [#57468](https://github.com/sourcegraph/sourcegraph/pull/57468)
- The GitHub Proxy service is no longer required and has been removed from deployment options. [#55290](https://github.com/sourcegraph/sourcegraph/issues/55290)
- The VSCode search extension "Sourcegraph for VS Code" has been sunset and removed from Sourcegraph
  repository. [#58023](https://github.com/sourcegraph/sourcegraph/pull/58023)
- The `rateLimit` configuration for Perforce code host connections has been removed to avoid confusion, it was unused. [#58188](https://github.com/sourcegraph/sourcegraph/pull/58188)
- The feature flag `search-ranking` is now completely removed. [#58156](https://github.com/sourcegraph/sourcegraph/pull/58156)
- The notepad UI, notebook creation feature. [#58217](https://github.com/sourcegraph/sourcegraph/pull/58217)
- The experimental `indexRepositoryName` option for the rust packages code host connection has been removed. [#59176](https://github.com/sourcegraph/sourcegraph/pull/59176)
- The column "Repository metadata" in the CSV export of repository search results is now deprecated and will be removed in a future release. Use "Repository metadata JSON" instead [#59334](https://github.com/sourcegraph/sourcegraph/pull/59334)
- Remote embeddings as context source for Cody has been removed. [#59493](https://github.com/sourcegraph/sourcegraph/pull/59493)

## 5.2.7

### Fixed

- The reference panel correctly shows definition and reference information instead of a "Could not find token" error for MATLAB. [#59636](https://github.com/sourcegraph/sourcegraph/pull/59636)
- The auto-index configuration page correctly shows any auto-inference errors instead of a nil pointer exception. [#59756](https://github.com/sourcegraph/sourcegraph/pull/59756)

## 5.2.6

### Added

- Implement adding automatic retry support for idempotent gRPC methods [#59404](https://github.com/sourcegraph/sourcegraph/pull/59404)

### Fixed

- Fix executors auth header `installSrc` [#59391](https://github.com/sourcegraph/sourcegraph/pull/59391)
- Avoid constantly rerunning failed embeddings jobs. [#58980](https://github.com/sourcegraph/sourcegraph/pull/58980)

## 5.2.5

### Added

- Added Cody providers data to pings. [#58848](https://github.com/sourcegraph/sourcegraph/pull/58848)
- Added the ability to proxy authentication requests when using the Azure OpenAI Cody provider. [#58862](https://github.com/sourcegraph/sourcegraph/pull/58862)

### Fixed

- Fixed an issue where updating a generic git code host would cause it to become unrestricted if permissions user mapping is enabled. [#58772](https://github.com/sourcegraph/sourcegraph/pull/58772)
- Fail embeddings jobs immediately if the rate limit is exceeded. [#58869](https://github.com/sourcegraph/sourcegraph/pull/58869)

### Changed

- Improved the admin page for search indexing. [#58866](https://github.com/sourcegraph/sourcegraph/pull/58866)

## 5.2.4

### Added

- Added the ability to use Workload Identity, Managed Identity and Environmental credentials when using the Azure OpenAI completions and embeddings providers [#58289](https://github.com/sourcegraph/sourcegraph/pull/58289)
- Added support for cloning via SSH from Azure DevOps. [#58655](https://github.com/sourcegraph/sourcegraph/pull/58655)

### Fixed

- Fixed two issues in Zoekt that could cause out of memory errors during search indexing. [sourcegraph/zoekt#686](https://github.com/sourcegraph/zoekt/pull/686), [sourcegraph/zoekt#689](https://github.com/sourcegraph/zoekt/pull/689)
- Fixed performance issue with embeddings job scheduling. (#58651)[https://github.com/sourcegraph/sourcegraph/pull/58651]

## 5.2.3

### Added

- Added configurable GraphQL query cost limitations to prevent unintended resource exhaustion. Default values are now provided and enforced, replacing the previously unlimited behaviour. For more information, please refer to: [GraphQL Cost Limits Documentation](https://docs.sourcegraph.com/api/graphql#cost-limits). See details at [#58346](https://github.com/sourcegraph/sourcegraph/pull/58346).
- Sourcegraph now supports connecting to Bitbucket Cloud using Workspace Access Tokens. [#58465](https://github.com/sourcegraph/sourcegraph/pull/58465).

### Fixed

- Defining file filters for embeddings jobs no longer causes all files to be skipped if `MaxFileSizeBytes` isn't defined. [#58262](https://github.com/sourcegraph/sourcegraph/pull/58262)

## 5.2.2

### Added

- Added a new authorization configuration options to GitLab code host connections: "markInternalReposAsPublic". Setting "markInternalReposAsPublic" to true is useful for organizations that have a large amount of internal repositories that everyone on the instance should be able to access, removing the need to have permissions to access these repositories. Additionally, when configuring a GitLab auth provider, you can specify "syncInternalRepoPermissions": false, which will remove the need to sync permissions for these internal repositories. [#57858](https://github.com/sourcegraph/sourcegraph/pull/57858)
- Experimental support for OpenAI powered autocomplete has been added. [#57872](https://github.com/sourcegraph/sourcegraph/pull/57872)

### Fixed

- Updated the endpoint used by the AWS Bedrock Claude provider. [#58028](https://github.com/sourcegraph/sourcegraph/pull/58028)

## 5.2.1

### Added

- Added two new authorization configuration options to GitHub code host connections: "markInternalReposAsPublic" and "syncInternalRepoPermissions". Setting "markInternalReposAsPublic" to true is useful for organizations that have a large amount of internal repositories that everyone on the instance should be able to access, removing the need to have permissions to access these repositories. Setting "syncInternalRepoPermissions" to true adds an additional step to user permission syncs that explicitly checks for internal repositories. However, this could lead to longer user permission sync times. [#56677](https://github.com/sourcegraph/sourcegraph/pull/56677)
- Fixed an issue with Code Monitors that could cause users to be notified multiple times for the same commit [#57546](https://github.com/sourcegraph/sourcegraph/pull/57546)
- Fixed an issue with Code Monitors that could prevent a new code monitor from being created if it targeted multiple repos [#57546](https://github.com/sourcegraph/sourcegraph/pull/57546)
- Sourcegraph instances will now emit a limited set of [telemetry events](https://docs.sourcegraph.com/admin/telemetry) in the background by default ([#57605](https://github.com/sourcegraph/sourcegraph/pull/57605)). Enablement will be based on the following conditions:
  - Customers with a license key created after October 3, 2023, or do not have a valid license key configured, will export all telemetry events recorded in the new system.
  - Customers with a license key created before October 3, 2023 will export only Cody-related events recorded in the new system, as covered by the [Cody Usage and Privacy Notice](https://about.sourcegraph.com/terms/cody-notice).
  - If you have a previous agreement regarding telemetry sharing, you account representative will reach out with more details.

### Fixed

- Fixed a user's Permissions page being inaccessible if the user has had no permission syncs with an external account connected. [#57372](https://github.com/sourcegraph/sourcegraph/pull/57372)
- Fixed a bug where site admins could not view a user's permissions if they didn't have access to all of the repositories the user has. Admins still won't be able to see repositories they don't have access to, but they will now be able to view the rest of the user's repository permissions. [#57375](https://github.com/sourcegraph/sourcegraph/pull/57375)
- Fixed a bug where gitserver statistics would not be properly decoded / reported when using REST (i.e. `experimentalFeatures.enableGRPC = false` in site configuration). [#57318](https://github.com/sourcegraph/sourcegraph/pull/57318)
- Updated the `curl` and `libcurl` dependencies to `8.4.0-r0` to fix [CVE-2023-38545](https://curl.se/docs/CVE-2023-38545.html). [#57533](https://github.com/sourcegraph/sourcegraph/pull/57533)
- Fixed a bug where commit signing failed when creating a changeset if `batchChanges.enforceFork` is set to true. [#57520](https://github.com/sourcegraph/sourcegraph/pull/57520)
- Fixed a regression in ranking of Go struct and interface in search results. [zoekt#655](https://github.com/sourcegraph/zoekt/pull/655)

## 5.2.0

### Added

- Experimental support for AWS Bedrock Claude for the completions provider has been added. [#56321](https://github.com/sourcegraph/sourcegraph/pull/56321)
- Recorded command logs can now be viewed for Git operations performed by Sourcegraph. This provides auditing and debugging capabilities. [#54997](https://github.com/sourcegraph/sourcegraph/issues/54997)
- Disk usage metrics for gitservers are now displayed on the site admin Git Servers page, showing free/total disk space. This helps site admins monitor storage capacity on GitServers. [#55958](https://github.com/sourcegraph/sourcegraph/issues/55958)
- Overhauled Admin Onboarding UI for enhanced user experience, introducing a license key modal with validation, automated navigation to Site Configuration Page, an interactive onboarding checklist button, and direct documentation links for SMTP and user authentication setup. [56366](https://github.com/sourcegraph/sourcegraph/pull/56366)
- New experimental feature "Search Jobs". Search Jobs allows you to run search queries across your organization's codebase (all repositories, branches, and revisions) at scale. It enhances the existing Sourcegraph's search capabilities, enabling you to run searches without query timeouts or incomplete results. Please refer to the [documentation](https://docs.sourcegraph.com/code_search/how-to/search-jobs) for more information.

### Changed

- OpenTelemetry Collector has been upgraded to v0.81, and OpenTelemetry packages have been upgraded to v1.16. [#54969](https://github.com/sourcegraph/sourcegraph/pull/54969), [#54999](https://github.com/sourcegraph/sourcegraph/pull/54999)
- Bitbucket Cloud code host connections no longer automatically syncs the repository of the username used. The appropriate workspace name will have to be added to the `teams` list if repositories for that account need to be synced. [#55095](https://github.com/sourcegraph/sourcegraph/pull/55095)
- Newly created access tokens are now hidden by default in the Sourcegraph UI. To view a token, click "show" button next to the token. [#56481](https://github.com/sourcegraph/sourcegraph/pull/56481)
- The GitHub proxy service has been removed and is no longer required. You can safely remove it from your deployment. [#55290](https://github.com/sourcegraph/sourcegraph/issues/55290)
- On startup, Zoekt indexserver will now delete the `<DATA_DIR>/.indexserver.tmp` directory to remove leftover repository clones, possibly causing a brief delay. Due to a bug, this directory wasn't previously cleaned up and could cause unnecessary disk usage. [zoekt#646](https://github.com/sourcegraph/zoekt/pull/646).
- gRPC is now used by default for all internal (service to service) communication. This change should be invisible to most customers. However, if you're running in an environment that places restrictions on Sourcegraph's internal traffic, some prior configuration might be required. See the ["Sourcegraph 5.2 gRPC Configuration Guide"](https://docs.sourcegraph.com/admin/updates/grpc) for more information. [#56738](https://github.com/sourcegraph/sourcegraph/pull/56738)

### Fixed

- Language detection for code highlighting now uses `go-enry` for all files by default, which fixes highlighting for MATLAB files. [#56559](https://github.com/sourcegraph/sourcegraph/pull/56559)

### Removed

- indexed-search has removed the deprecated environment variable ZOEKT_ENABLE_LAZY_DOC_SECTIONS [zoekt#620](https://github.com/sourcegraph/zoekt/pull/620)
- The federation feature that could redirect users from their own Sourcegraph instance to public repositories on Sourcegraph.com has been removed. It allowed users to open a repository URL on their own Sourcegraph instance and, if the repository wasn't found on that instance, the user would be redirect to the repository on Sourcegraph.com, where it was possibly found. The feature has been broken for over a year though and we don't know that it was used. If you want to use it, please open a feature-request issue and tag the `@sourcegraph/source` team. [#55161](https://github.com/sourcegraph/sourcegraph/pull/55161)
- The `applySearchQuerySuggestionOnEnter` experimental feature flag in user settings was removed, and this behavior is now always enabled. Previously, this behavior was on by default, but it was possible to disable it.
- The feature-flag `search-hybrid`, which allowed to disable the performance improvements for unindexed search in 4.3, is now deprecated and will not be read anymore. [#56470](https://github.com/sourcegraph/sourcegraph/pull/56470)

## 5.1.9

### Added

- Enable "Test connection" for Perforce code hosts. The "Test connection" button in the code host page UI now works for Perforce code hosts. [#56697](https://github.com/sourcegraph/sourcegraph/pull/56697)

### Changed

- User access to Perforce depots is sometimes denied unintentionally when using `"authorization"/"subRepoPermissions": true` in the code host config and the protects file contains exclusionary entries with the Host field filled out. Ignoring those rules (that use anything other than the wildcard (`*`) in the Host field) is now toggle-able by adding `"authorization"/"ignoreRulesWithHost"` to the code host config and setting the value to `true`. [#56450](https://github.com/sourcegraph/sourcegraph/pull/56450)

### Fixed

- Fixed an issue where the "gitLabProjectVisibilityExperimental" feature flag would not be respected by the permissions syncer. This meant that users on Sourcegraph that have signed in with GitLab would not see GitLab internal repositories that should be accessible to everyone on the GitLab instance, even though the feature flag was enabled [#56492](https://github.com/sourcegraph/sourcegraph/pull/56492)
- Fixed a bug when syncing repository lists from GitHub that could lead to 404 errors showing up when running into GitHub rate limits [#56478](https://github.com/sourcegraph/sourcegraph/pull/56478)

## 5.1.8

### Added

- Added experimental autocomplete support for Azure OpenAI [#56063](https://github.com/sourcegraph/sourcegraph/pull/56063)

### Changed

- Improved stability of gRPC connections [#56314](https://github.com/sourcegraph/sourcegraph/pull/56314), [#56302](https://github.com/sourcegraph/sourcegraph/pull/56302), [#56298](https://github.com/sourcegraph/sourcegraph/pull/56298), [#56217](https://github.com/sourcegraph/sourcegraph/pull/56217)

## 5.1.7

### Changed

- Pressing `Mod-f` will always select the input value in the file view search [#55546](https://github.com/sourcegraph/sourcegraph/pull/55546)
- Caddy has been updated to version 2.7.3 resolving a number of vulnerabilities. [#55606](https://github.com/sourcegraph/sourcegraph/pull/55606)
- The commit message defined in a batch spec will now be passed to `git commit` on stdin using `--file=-` instead of being included inline with `git commit -m` to improve how the message is interpreted by `git` in certain edge cases, such as when the commit message begins with a dash, and to prevent extra quotes being added to the message. This may mean that previous escaping strategies will behave differently.

### Fixed

- Fixed a bug in the `deploy-sourcegraph-helm` deployment of Sourcegraph, for sufficiantly large scip indexes uploads will fail when the precise-code-intel worker attempts to write to `/tmp` and doesn't have a volume mounted for this purpose. See [kubernetes release notes](./admin/updates/kubernetes.md#v516-➔-v517) for more details [#342](https://github.com/sourcegraph/deploy-sourcegraph-helm/pull/343)

## 5.1.6

### Added

- New Prometheus metrics have been added to track the response / request sizes of gRPC calls. [#55381](https://github.com/sourcegraph/sourcegraph/pull/55381)
- A new embeddings site configuration setting `excludeChunkOnError` allows embedding jobs to complete job execution despite chunks of code or text that fail. When enabled the chunks are skipped after failed retries but the index can continue being populated. When disabled the entire job fails and the index is not saved. This setting is enabled by default. Embedding job statistics now capture `code_chunks_excluded` and `text_chunks_excluded` for successfully completed jobs. Total excluded chunks and file names for excluded chunks are logged as warnings. [#55180](https://github.com/sourcegraph/sourcegraph/pull/55180)
- Experimental support for Azure OpenAI for the completions and embeddings provider has been added. [#55178](https://github.com/sourcegraph/sourcegraph/pull/55178)
- Added a feature flag for alternate GitLab project visibility resolution. This may solve some weird cases with not being able to see GitLab internal projects. [#54426](https://github.com/sourcegraph/sourcegraph/pull/54426)
  - To use this feature flag, create a Boolean feature flag named "gitLabProjectVisibilityExperimental" and set the value to True.
- It is now possible to add annotations to pods spawned by jobs created by the Kubernetes executor. [#55361](https://github.com/sourcegraph/sourcegraph/pull/55361)

### Changed

- Updated all packages in container images to latest versions
- Updated Docker-in-Docker image from 23.0.1 to 23.0.6
- The gRPC implementation for the Symbol service's `LocalCodeIntel` endpoint has been changed to stream its results. [#55242](https://github.com/sourcegraph/sourcegraph/pull/55242)
- When using OpenAI or Azure OpenAI for Cody completions, code completions will be disabled - chat will continue to work. This is because we currently don't support code completions with OpenAI. [#55624](https://github.com/sourcegraph/sourcegraph/pull/55624)

### Fixed

- Fixed a bug where user account requests could not be approved even though the license would permit user creation otherwise. [#55482](https://github.com/sourcegraph/sourcegraph/pull/55482)
- Fixed a bug where the background scheduler for embedding jobs based on policies would not schedule jobs for private repositories. [#55698](https://github.com/sourcegraph/sourcegraph/pull/55698)
- Fixed a source of inconsistency in precise code navigation, affecting implementations and prototypes especially. [#54410](https://github.com/sourcegraph/sourcegraph/pull/54410)

### Removed

## 5.1.5

### Known Issues

- Standard and multi-version upgrades are not currently working from Sourcegraph versions 5.0.X to 5.1.5. As a temporary workaround, please upgrade 5.0.X to 5.1.0, then 5.1.0 to 5.1.5.

### Fixed

- Fixed an embeddings job scheduler bug where if we cannot resolve one of the repositories or its default branch then all repositories submitted will not have their respective embeddings job enqueued. Embeddings job scheduler will now continue to schedule jobs for subsequent repositories in the submitted repositories set. [#54701](https://github.com/sourcegraph/sourcegraph/pull/54701)
- Creation of GitHub Apps will now respect system certificate authorities when specifying certificates for the tls.external site configuration. [#55084](https://github.com/sourcegraph/sourcegraph/pull/55084)
- Passing multi-line Coursier credentials in JVM packages configuration should now work correctly. [#55113](https://github.com/sourcegraph/sourcegraph/pull/55113)
- SCIP indexes are now ingested in a streaming fashion, eliminating out-of-memory errors in most cases, even when uploading very large indexes (1GB+ uncompressed). [#53828](https://github.com/sourcegraph/sourcegraph/pull/53828)
- Moved the license checks to worker service. We make sure to run only 1 instance of license checks this way. [54854](https://github.com/sourcegraph/sourcegraph/pull/54854)
- Updated base images to resolve issues in curl, OpenSSL, and OpenSSL. [55310](https://github.com/sourcegraph/sourcegraph/pull/55310)
- The default message size limit for gRPC clients has been raised from 4MB to 90MB. [#55209](https://github.com/sourcegraph/sourcegraph/pull/55209)
- The message printing feature for the custom gRPC internal error interceptor now supports logging all internal error types, instead of just non-utf 8 errors. [#55130](https://github.com/sourcegraph/sourcegraph/pull/55130)
- Fixed an issue where GitHub Apps could not be set up using Firefox. [#55305](https://github.com/sourcegraph/sourcegraph/pull/55305)
- Fixed nil panic on certain GraphQL fields when listing users. [#55322](https://github.com/sourcegraph/sourcegraph/pull/55322)

### Changed

- The "Files" tab of the fuzzy finder now allows you to navigate directly to a line number by appending `:NUMBER`. For example, the fuzzy query `main.ts:100` opens line 100 in the file `main.ts`. [#55064](https://github.com/sourcegraph/sourcegraph/pull/55064)
- The gRPC implementation for the Symbol service's `LocalCodeIntel` endpoint has been changed to stream its results. [#55242](https://github.com/sourcegraph/sourcegraph/pull/55242)
- GitLab auth providers now support an `ssoURL` option that facilitates scenarios where a GitLab group requires SAML/SSO. [#54957](https://github.com/sourcegraph/sourcegraph/pull/54957)

### Added

### Removed

## 5.1.4

### Fixed

- A bug where we would temporarily use much more memory than needed during embeddings fetching. [#54972](https://github.com/sourcegraph/sourcegraph/pull/54972)

### Changed

- The UI for license keys now displays more information about license validity. [#54990](https://github.com/sourcegraph/sourcegraph/pull/54990)
- Sourcegraph now supports more than one auth provider per URL. [#54289](https://github.com/sourcegraph/sourcegraph/pull/54289)
- Site-admins can now list, view and edit all code monitors. [#54981](https://github.com/sourcegraph/sourcegraph/pull/54981)

## 5.1.3

### Changed

- Cody source code (for the VS Code extension, CLI, and client shared libraries) has been moved to the [sourcegraph/cody repository](https://github.com/sourcegraph/cody).
- `golang.org/x/net/trace` instrumentation, previously available under `/debug/requests` and `/debug/events`, has been removed entirely from core Sourcegraph services. It remains available for Zoekt. [#53795](https://github.com/sourcegraph/sourcegraph/pull/53795)

### Fixed

- Fixed an embeddings job scheduler bug where if we cannot resolve one of the repositories or its default branch then all repositories submitted will not have their respective embeddings job enqueued. Embeddings job scheduler will now continue to schedule jobs for subsequent repositories in the submitted repositories set. [#54701](https://github.com/sourcegraph/sourcegraph/pull/54701)

## 5.1.2

### Fixed

- Fixes a crash when uploading indexes with malformed source ranges (this was a bug in scip-go). [#54304](https://github.com/sourcegraph/sourcegraph/pull/54304)
- Fixed validation of Bitbucket Cloud configuration in site-admin create/update form. [#54494](https://github.com/sourcegraph/sourcegraph/pull/54494)
- Fixed race condition with grpc `server.send` message. [#54500](https://github.com/sourcegraph/sourcegraph/pull/54500)
- Fixed a configuration initialization issue that broke the outbound request in the site admin page. [#54745](https://github.com/sourcegraph/sourcegraph/pull/54745)
- Fixed Postgres DSN construction edge-case. [#54858](https://github.com/sourcegraph/sourcegraph/pull/54858)

## 5.1.1

### Fixed

- Fixed the default behaviour when the explicit permissions API is enabled. Repositories are no longer marked as unrestricted by default. [#54419](https://github.com/sourcegraph/sourcegraph/pull/54419)

## 5.1.0

> **Note**: As of 5.1.0, the limited OSS subset of Sourcegraph has been removed, and code search OSS code has been relicensed going forward. See https://github.com/sourcegraph/sourcegraph/issues/53528#issuecomment-1594967818 for more information (blog post coming soon).

> **Note**: As of 5.1.0, the `rsa-sha` signature algorithm is no longer supported when connecting to code hosts over SSH. If you encounter the error `sign_and_send_pubkey: no mutual signature supported` when syncing repositories, see [Repository authentication](https://docs.sourcegraph.com/admin/repo/auth#error-sign_and_send_pubkey-no-mutual-signature-supported) for more information and steps to resolve the issue.

### [Known issues](KNOWN-ISSUES.md)

- There is an issue with Sourcegraph instances configured to use explicit permissions using permissions.userMapping in Site configuration, where repository permissions are not enforced. Customers using the explicit permissions API are advised to upgrade to v5.1.1 directly.
- There is an issue with creating and updating existing Bitbucket.org (Cloud) code host connections due to problem with JSON schema validation which prevents the JSON editor from loading and surfaces as an error in the UI.

### Added

- Executors natively support Kubernetes environments. [#49236](https://github.com/sourcegraph/sourcegraph/pull/49236)
- Documentation for GitHub fine-grained access tokens. [#50274](https://github.com/sourcegraph/sourcegraph/pull/50274)
- Code Insight dashboards retain size and order of the cards. [#50301](https://github.com/sourcegraph/sourcegraph/pull/50301)
- The LLM completions endpoint is now exposed through a GraphQL query in addition to the streaming endpoint [#50455](https://github.com/sourcegraph/sourcegraph/pull/50455)
- Permissions center statistics pane is added. Stats include numbers of queued jobs, users/repos with failed jobs, no permissions, and outdated permissions. [#50535](https://github.com/sourcegraph/sourcegraph/pull/50535)
- SCIM user provisioning support for Deactivate/Reactivation of users. [#50533](https://github.com/sourcegraph/sourcegraph/pull/50533)
- Login form can now be configured with ordering and limit of auth providers. [See docs](https://docs.sourcegraph.com/admin/auth/login_form). [#50586](https://github.com/sourcegraph/sourcegraph/pull/50586), [50284](https://github.com/sourcegraph/sourcegraph/pull/50284) and [#50705](https://github.com/sourcegraph/sourcegraph/pull/50705)
- OOM reaper events affecting `p4-fusion` jobs on `gitserver` are better detected and handled. Error (non-zero) exit status is used, and the resource (CPU, memory) usage of the job process is appended to the job output so that admins can infer possible OOM activity and take steps to address it. [#51284](https://github.com/sourcegraph/sourcegraph/pull/51284)
- When creating a new batch change, spaces are automatically replaced with dashes in the name field. [#50825](https://github.com/sourcegraph/sourcegraph/pull/50825) and [51071](https://github.com/sourcegraph/sourcegraph/pull/51071)
- Support for custom HTML injection behind an environment variable (`ENABLE_INJECT_HTML`). This allows users to enable or disable HTML customization as needed, which is now disabled by default. [#51400](https://github.com/sourcegraph/sourcegraph/pull/51400)
- Added the ability to block auto-indexing scheduling and inference via the `codeintel_autoindexing_exceptions` Postgres table. [#51578](https://github.com/sourcegraph/sourcegraph/pull/51578)
- When an admin has configured rollout windows for Batch Changes changesets, the configuration details are now visible to all users on the Batch Changes settings page. [#50479](https://github.com/sourcegraph/sourcegraph/pull/50479)
- Added support for regular expressions in`exclude` repositories for GitLab code host connections. [#51862](https://github.com/sourcegraph/sourcegraph/pull/51862)
- Branches created by Batch Changes will now be automatically deleted on the code host upon merging or closing a changeset if the new `batchChanges.autoDeleteBranch` site setting is enabled. [#52055](https://github.com/sourcegraph/sourcegraph/pull/52055)
- Repository metadata now generally available for everyone [#50567](https://github.com/sourcegraph/sourcegraph/pull/50567), [#50607](https://github.com/sourcegraph/sourcegraph/pull/50607), [#50857](https://github.com/sourcegraph/sourcegraph/pull/50857), [#50908](https://github.com/sourcegraph/sourcegraph/pull/50908), [#972](https://github.com/sourcegraph/src-cli/pull/972), [#51031](https://github.com/sourcegraph/sourcegraph/pull/51031), [#977](https://github.com/sourcegraph/src-cli/pull/977), [#50821](https://github.com/sourcegraph/sourcegraph/pull/50821), [#51258](https://github.com/sourcegraph/sourcegraph/pull/51258), [#52078](https://github.com/sourcegraph/sourcegraph/pull/52078), [#51985](https://github.com/sourcegraph/sourcegraph/pull/51985), [#52150](https://github.com/sourcegraph/sourcegraph/pull/52150), [#52249](https://github.com/sourcegraph/sourcegraph/pull/52249), [#51982](https://github.com/sourcegraph/sourcegraph/pull/51982), [#51248](https://github.com/sourcegraph/sourcegraph/pull/51248), [#51921](https://github.com/sourcegraph/sourcegraph/pull/51921), [#52301](https://github.com/sourcegraph/sourcegraph/pull/52301)
- Batch Changes for Gerrit Code Hosts [#52647](https://github.com/sourcegraph/sourcegraph/pull/52647).
- Batch Changes now supports per-batch-change control for pushing to a fork of the upstream repository when the property `changesetTemplate.fork` is specified in the batch spec. [#51572](https://github.com/sourcegraph/sourcegraph/pull/51572)
- Executors can now be configured to process multiple queues. [#52016](https://github.com/sourcegraph/sourcegraph/pull/52016)
- Added `isCodyEnabled` as a new GraphQL field to `Site`. [#52941](https://github.com/sourcegraph/sourcegraph/pull/52941)
- Enabled improved search ranking by default. This feature can be disabled through the `search-ranking` feature flag.[#53031](https://github.com/sourcegraph/sourcegraph/pull/53031)
- Added token callback route for Cody in VS Code and VS Code insiders. [#53313](https://github.com/sourcegraph/sourcegraph/pull/53313)
- Latest repository clone/sync output is surfaced in the "Mirroring and cloning" page (`{REPO}/-/settings/mirror`). Added primarily to enable easier debugging of issues with Perforce depots, it can also be useful for other code hosts. [#51598](https://github.com/sourcegraph/sourcegraph/pull/51598)
- New `file:has.contributor(...)` predicate for filtering files based on contributors. [#53206](https://github.com/sourcegraph/sourcegraph/pull/53206)
- Added multi-repo scope selector for Cody on the web supporting unified context generation API which uses combination of embeddings search and keyword search as fallback for context generation. [53046](https://github.com/sourcegraph/sourcegraph/pull/53046)
- Batch Changes can now sign commits for changesets published on GitHub code hosts via GitHub Apps. [#52333](https://github.com/sourcegraph/sourcegraph/pull/52333)
- Added history of changes to the site configuration page. Site admins can now see information about changes made to the site configuration, by whom and when. [#49842](https://github.com/sourcegraph/sourcegraph/pull/49842)
- For Perforce depots, users will now see the changelist ID (CL) instead of Git commit SHAs when visiting a depot or the view changelists page [#51195](https://github.com/sourcegraph/sourcegraph/pull/51195)
- Visiting a specific CL will now use the CL ID in the URL instead of the commit SHA. Other areas affected by this change are browsing files at a specific CL, viewing a specific file changed as part of a specific CL. To enable this behaviour, site admins should set `"perforceChangelistMapping": "enabled"` under experimentalFeatures in the site configuration. Note that currently we process only one perforce depot at a time to map the commit SHAs to their CL IDs in the backend. In a subsequent release we will add support to process multiple depots in parallel. Other areas where currently commit SHAs are used will be updated in future releases. [#53253](https://github.com/sourcegraph/sourcegraph/pull/53253) [#53608](https://github.com/sourcegraph/sourcegraph/pull/53608) [#54051](https://github.com/sourcegraph/sourcegraph/pull/54051)
- Added autoupgrading to automatically perform multi-version upgrades, without manual `migrator` invocations, through the `frontend` deployment. Please see the [documentation](https://docs.sourcegraph.com/admin/updates/automatic) for details. [#52242](https://github.com/sourcegraph/sourcegraph/pull/52242) [#53196](https://github.com/sourcegraph/sourcegraph/pull/53196)

### Changed

- Access tokens now begin with the prefix `sgp_` to make them identifiable as secrets. You can also prepend `sgp_` to previously generated access tokens, although they will continue to work as-is without that prefix.
- The commit message defined in a batch spec will now be quoted when git is invoked, i.e. `git commit -m "commit message"`, to improve how the message is interpreted by the shell in certain edge cases, such as when the commit message begins with a dash. This may mean that previous escaping strategies will behave differently.
- 429 errors from external services Sourcegraph talks to are only retried automatically if the Retry-After header doesn't indicate that a retry would be useless. The time grace period can be configured using `SRC_HTTP_CLI_EXTERNAL_RETRY_AFTER_MAX_DURATION` and `SRC_HTTP_CLI_INTERNAL_RETRY_AFTER_MAX_DURATION`. [#51743](https://github.com/sourcegraph/sourcegraph/pull/51743)
- Security Events NO LONGER write to database by default - instead, they will be written in the [audit log format](https://docs.sourcegraph.com/admin/audit_log) to console. There is a new site config setting `log.securityEventLogs` that can be used to configure security event logs to write to database if the old behaviour is desired. This new default will significantly improve performance for large instances. In addition, the old environment variable `SRC_DISABLE_LOG_PRIVATE_REPO_ACCESS` no longer does anything. [#51686](https://github.com/sourcegraph/sourcegraph/pull/51686)
- Audit Logs & Security Events are written with the same severity level as `SRC_LOG_LEVEL`. This prevents a misconfiguration
  issue when `log.AuditLogs.SeverityLevel` was set below the overall instance log level. `log.AuditLogs.SeverityLevel` has
  been marked as deprecated and will be removed in a future release [#52566](https://github.com/sourcegraph/sourcegraph/pull/52566)
- Update minimum supported Redis version to 6.2 [#52248](https://github.com/sourcegraph/sourcegraph/pull/52248)
- The batch spec properties [`transformChanges`](https://docs.sourcegraph.com/batch_changes/references/batch_spec_yaml_reference#transformchanges) and [`workspaces`](https://docs.sourcegraph.com/batch_changes/references/batch_spec_yaml_reference#workspaces) are now generally available.
- Cody feature flags have been simplified [#52919](https://github.com/sourcegraph/sourcegraph/pull/52919) See [the docs page for complete setup details](https://docs.sourcegraph.com/cody/explanations/enabling_cody_enterprise)
  - `cody.enabled` in site-config now controls whether Cody is on/off, default `false`.
  - When `cody.enabled` is set and no specific configuration for `completions` and `embeddings` are given, Cody will by default talk to the `sourcegraph` provider, Sourcegraphs Cody Gateway which allows access to chat completions and embeddings.
  - Enabling Cody now requires `cody.enabled` set to `true` and `completions` to be set.
  - `cody.restrictUsersFeatureFlag` replaces `experimentalFeatures.CodyRestrictUsersFeatureFlag` in site-config, default `false`.
  - `completions.enabled` has been deprecated, replaced by `cody.enabled`.
  - The feature flags for Cody in web features have been removed and the single source of truth is now `cody.enabled`.
  - The embeddings configuration now requires a `provider` field to be set.
  - Ping data now reflects whether `cody.enabled` and `completions` are set.
- If a Sourcegraph request is traced, its trace ID and span ID are now set to the `X-Trace` and `X-Trace-Span` response headers respectively. The trace URL (if a template is configured in `observability.tracing.urlTemplate`) is now set to `X-Trace-URL` - previously, the URL was set to `X-Trace`. [#53259](https://github.com/sourcegraph/sourcegraph/pull/53259)
- For users using the single-container server image with the default built-in database, the database must be reindexed. This process can take up to a few hours on systems with large datasets. See [Migrating to Sourcegraph 5.1.x](https://docs.sourcegraph.com/admin/migration/5_1) for full details. [#53256](https://github.com/sourcegraph/sourcegraph/pull/53256)
- [Sourcegraph Own](https://docs.sourcegraph.com/own) is now available as a beta enterprise feature. `search-ownership` feature flag is removed and doesn't need to be used.
- Update Jaeger to 1.45.0, and Opentelemetry-Collector to 0.75.0 [#54000](https://github.com/sourcegraph/sourcegraph/pull/54000)
- Switched container OS to Wolfi for hardened containers [#47182](https://github.com/sourcegraph/sourcegraph/pull/47182), [#47368](https://github.com/sourcegraph/sourcegraph/pull/47368)
- Batches changes now supports for CODEOWNERS for Github. Pull requests requiring CODEOWNERS approval, will no longer show as approved unless explicitly approved by a CODEOWNER. https://github.com/sourcegraph/sourcegraph/pull/53601
- The insecure `rsa-sha` signature algorithm is no longer supported when connecting to code hosts over SSH. See the [Repository authentication](https://docs.sourcegraph.com/admin/repo/auth#error-sign_and_send_pubkey-no-mutual-signature-supported) page for further details.

### Fixed

- GitHub `repositoryQuery` searches now respect date ranges and use API requests more efficiently. #[49969](https://github.com/sourcegraph/sourcegraph/pull/49969)
- Fixed an issue where search based references were not displayed in the references panel. [#50157](https://github.com/sourcegraph/sourcegraph/pull/50157)
- Symbol suggestions only insert `type:symbol` filters when necessary. [#50183](https://github.com/sourcegraph/sourcegraph/pull/50183)
- Removed an incorrect beta label on the Search Context creation page [#51188](https://github.com/sourcegraph/sourcegraph/pull/51188)
- Multi-version upgrades to version `5.0.2` in a fully airgapped environment will not work without the command `--skip-drift-check`. [#51164](https://github.com/sourcegraph/sourcegraph/pull/51164)
- Could not set "permissions.syncOldestUsers" or "permissions.syncOldestRepos" to zero. [#51255](https://github.com/sourcegraph/sourcegraph/pull/51255)
- GitLab code host connections will disable repo-centric repository permission syncs when the authentication provider is set as "oauth". This prevents repo-centric permission sync from getting incorrect data. [#51452](https://github.com/sourcegraph/sourcegraph/pull/51452)
- Code intelligence background jobs did not correctly use an internal context, causing SCIP data to sometimes be prematurely deleted. [#51591](https://github.com/sourcegraph/sourcegraph/pull/51591)
- Slow request logs now have the correct trace and span IDs attached if a trace is present on the request. [#51826](https://github.com/sourcegraph/sourcegraph/pull/51826)
- The braindot menu on the blob view no longer fetches data eagerly to prevent performance issues for larger monorepo users. [#53039](https://github.com/sourcegraph/sourcegraph/pull/53039)
- Fixed an issue where commenting out redacted site-config secrets would re-add the secrets. [#53152](https://github.com/sourcegraph/sourcegraph/pull/53152)
- Fixed an issue where SCIP packages would sometimes not be written to the database, breaking cross-repository jump to definition. [#53763](https://github.com/sourcegraph/sourcegraph/pull/53763)
- Fixed an issue when adding a new user external account was not scheduling a new permission sync for the user. [#54144](https://github.com/sourcegraph/sourcegraph/pull/54144)
- Adding a new user account now correctly schedules a permission sync for the user. [#54258](https://github.com/sourcegraph/sourcegraph/pull/54258)
- Users/repos without an existing sync job in the permission_sync_jobs table are now scheduled properly. [#54278](https://github.com/sourcegraph/sourcegraph/pull/54278)

### Removed

- User tags are removed in favor of the newer feature flags functionality. [#49318](https://github.com/sourcegraph/sourcegraph/pull/49318)
- Previously deprecated site config `experimentalFeatures.bitbucketServerFastPerm` has been removed. [#50707](https://github.com/sourcegraph/sourcegraph/pull/50707)
- Unused site-config field `api.rateLimit` has been removed. [#51087](https://github.com/sourcegraph/sourcegraph/pull/51087)
- Legacy (table-based) blob viewer. [#50915](https://github.com/sourcegraph/sourcegraph/pull/50915)

## 5.0.6

### Fixed

- SAML assertions to get user display name are now compared case insensitively and we do not always return an error. [#52992](https://github.com/sourcegraph/sourcegraph/pull/52992)
- Fixed an issue where `type:diff` search would not work when sub-repo permissions are enabled. [#53210](https://github.com/sourcegraph/sourcegraph/pull/53210)

## 5.0.5

### Added

- Organization members can now administer batch changes created by other members in their organization's namespace if the setting `orgs.allMembersBatchChangesAdmin` is enabled for that organization. [#50724](https://github.com/sourcegraph/sourcegraph/pull/50724)
- Allow instance public access mode based on `auth.public` site config and `allow-anonymous-usage` license tag [#52440](https://github.com/sourcegraph/sourcegraph/pull/52440)
- The endpoint configuration field for completions is now supported by the OpenAI provider [#52530](https://github.com/sourcegraph/sourcegraph/pull/52530)

### Fixed

- MAU calculation in product analytics and pings use the same condition and UTC at all times. [#52306](https://github.com/sourcegraph/sourcegraph/pull/52306) [#52579](https://github.com/sourcegraph/sourcegraph/pull/52579) [#52581](https://github.com/sourcegraph/sourcegraph/pull/52581)
- Bitbucket native integration: fix code-intel popovers on the pull request pages. [#52609](https://github.com/sourcegraph/sourcegraph/pull/52609)
- `id` column of `user_repo_permissions` table was switched to `bigint` to avoid `int` overflow. [#52299](https://github.com/sourcegraph/sourcegraph/pull/52299)
- In some circumstances filenames containing `..` either could not be read or would return a diff when viewed. We now always correctly read those files. [#52605](https://github.com/sourcegraph/sourcegraph/pull/52605)
- Syntax highlighting for several languages including Python, Java, C++, Ruby, TypeScript, and JavaScript is now working again when using the single Docker container deployment option. Other deployment options were not affected.

## 5.0.4

### Fixed

- Git blame lookups of repositories synced through `src serve-git` or code hosts using a custom `repositoryPathPattern` will now use the correct URL when streaming git blame is enabled. [#51525](https://github.com/sourcegraph/sourcegraph/pull/51525)
- Code Insights scoped to a static list of repository names would fail to resolve repositories with permissions enabled, resulting in insights that would not process. [#51657](https://github.com/sourcegraph/sourcegraph/pull/51657)
- Batches: Resolved an issue with GitHub webhooks where CI check updates fail due to the removal of a field from the GitHub webhook payload. [#52035](https://github.com/sourcegraph/sourcegraph/pull/52035)

## 5.0.3

### Added

- Cody aggregated pings. [#50835](https://github.com/sourcegraph/sourcegraph/pull/50835)

### Fixed

- Bitbucket Server adding an error log if there is no account match for the user. #[51030](https://github.com/sourcegraph/sourcegraph/pull/51030)
- Editing search context with special characters such as `/` resulted in http 404 error. [#51196](https://github.com/sourcegraph/sourcegraph/pull/51196)
- Significantly improved performance and reduced memory usage of the embeeddings service. [#50953](https://github.com/sourcegraph/sourcegraph/pull/50953), [#51372](https://github.com/sourcegraph/sourcegraph/pull/51372)
- Fixed an issue where a Code Insights query with structural search type received 0 search results for the latest commit of any matching repo. [#51076](https://github.com/sourcegraph/sourcegraph/pull/51076)

## 5.0.2

### Added

- An experimental site config setting to restrict cody to users by the cody-experimental feature flag [#50668](https://github.com/sourcegraph/sourcegraph/pull/50668)

### Changed

- Use the Alpine 3.17 releases of cURL and Git

### Fixed

- For Cody, explicitly detect some cases where context is needed to avoid failed responses. [#50541](https://github.com/sourcegraph/sourcegraph/pull/50541)
- Code Insights that are run over zero repositories will finish processing and show `"No data to display"`. #[50561](https://github.com/sourcegraph/sourcegraph/pull/50561)
- DNS timeouts on calls to host.docker.internal from every html page load for docker-compose air-gapped instances. No more DNS lookups in jscontext.go anymore. #[50638](https://github.com/sourcegraph/sourcegraph/pull/50638)
- Improved the speed of the embedding index by significantly decreasing the calls to Gitserver. [#50410](https://github.com/sourcegraph/sourcegraph/pull/50410)

### Removed

-

## 5.0.1

### Added

- The ability to exclude certain file path patterns from embeddings.
- Added a modal to show warnings and errors when exporting search results. [#50348](https://github.com/sourcegraph/sourcegraph/pull/50348)

### Changed

### Fixed

- Fixed CVE-2023-0464 in container images
- Fixed CVE-2023-24532 in container images
- Fixed an issue where Slack code monitoring notifications failed when the message was too long. [#50083](https://github.com/sourcegraph/sourcegraph/pull/50083)
- Fixed an edge case issue with usage statistics calculations that cross over month and year boundaries.
- Fixed the "Last incremental sync" value in user/repo permissions from displaying a wrong date if no sync had been completed yet.
- Fixed an issue that caused search context creation to fail with error "you must provide a first or last value to properly paginate" when defining the repositories and revisions with a JSON configuration.
- Fixed an issue where the incorrect actor was provided when searching an embeddings index.
- Fixed multiple requests downloading the embeddings index concurrently on an empty cache leading to an out-of-memory error.
- Fixed the encoding of embeddings indexes which caused out-of-memory errors for large indexes when uploading them from the worker service.
- Fixed git blame decorations styles
- CODEOWNERS rules with consecutive slashes (`//`) will no longer fail ownership searches
- Granting pending permissions to users when experimentalFeatures.unifiedPermissions is turned ON [#50059](https://github.com/sourcegraph/sourcegraph/pull/50059)
- The unified permissions out of band migration reported as unfinished if there were users with no permissions [#50147](https://github.com/sourcegraph/sourcegraph/pull/50147)
- Filenames with special characters are correctly handled in Cody's embedding service [#50023](https://github.com/sourcegraph/sourcegraph/pull/50023)
- Structural search correctly cleans up when done preventing a goroutine leak [#50034](https://github.com/sourcegraph/sourcegraph/pull/50034)
- Fetch search based definitions in the reference panel if no precise definitions were found [#50179](https://github.com/sourcegraph/sourcegraph/pull/50179)

### Removed

## 5.0.0

### Added

- The environment variable `TELEMETRY_HTTP_PROXY` can be set on the `sourcegraph-frontend` service, to use an HTTP proxy for telemetry and update check requests. [#47466](https://github.com/sourcegraph/sourcegraph/pull/47466)
- Kubernetes Deployments: Introduced a new Kubernetes deployment option ([deploy-sourcegraph-k8s](https://github.com/sourcegraph/deploy-sourcegraph-k8s)) to deploy Sourcegraph with Kustomize. [#46755](https://github.com/sourcegraph/sourcegraph/issues/46755)
- Kubernetes Deployments: The new Kustomize deployment ([deploy-sourcegraph-k8s](https://github.com/sourcegraph/deploy-sourcegraph-k8s)) introduces a new base cluster that runs all Sourcegraph services as non-root users with limited privileges and eliminates the need to create RBAC resources. [#4213](https://github.com/sourcegraph/deploy-sourcegraph/pull/4213)
- Added the `other.exclude` setting to [Other external service config](https://docs.sourcegraph.com/admin/code_hosts/other#configuration). It can be configured to exclude mirroring of repositories matching a pattern similar to other external services. This is useful when you want to exclude repositories discovered via `src serve-git`. [#48168](https://github.com/sourcegraph/sourcegraph/pull/48168)
- The **Site admin > Updates** page displays the upgrade readiness information about schema drift and out-of-band migrations. [#48046](https://github.com/sourcegraph/sourcegraph/pull/48046)
- Pings now contain ownership search and file-view activity counts. [#47062](https://github.com/sourcegraph/sourcegraph/47062)
- Greatly improves keyboard handling and accessibility of the files and symbol tree on the repository pages. [#12916](https://github.com/sourcegraph/sourcegraph/issues/12916)
- The file tree on the repository page now automatically expands into single-child directories. [#47117](https://github.com/sourcegraph/sourcegraph/pull/47117)
- When encountering GitHub rate limits, Sourcegraph will now wait the recommended amount of time and retry the request. This prevents sync jobs from failing prematurely due to external rate limits. [#48423](https://github.com/sourcegraph/sourcegraph/pull/48423)
- Added a dashboard with information about user and repository background permissions sync jobs. [#46317](https://github.com/sourcegraph/sourcegraph/issues/46317)
- When encountering GitHub or GitLab rate limits, Sourcegraph will now wait the recommended amount of time and retry the request. This prevents sync jobs from failing prematurely due to external rate limits. [#48423](https://github.com/sourcegraph/sourcegraph/pull/48423), [#48616](https://github.com/sourcegraph/sourcegraph/pull/48616)
- Switching between code editor, files and symbols trees using keyboard shortcuts (currently under the experimental feature flag: `blob-page-switch-areas-shortcuts`). [#46829](https://github.com/sourcegraph/sourcegraph/pull/46829).
- Added "SCIM" badges for SCIM-controlled users on the User admin page. [#48727](https://github.com/sourcegraph/sourcegraph/pull/48727)
- Added Azure DevOps Services as a Tier 1 Code Host, including: repository syncing, permissions syncing, and Batch Changes support. [#46265](https://github.com/sourcegraph/sourcegraph/issues/46265)
- Added feature to disable some fields on user profiles for SCIM-controlled users. [#48816](https://github.com/sourcegraph/sourcegraph/pull/48816)
- Native support for ingesting and searching GitHub topics with `repo:has.topic()` [#48875](https://github.com/sourcegraph/sourcegraph/pull/48875)
- [Role-based Access Control](https://docs.sourcegraph.com/admin/access_control) is now available as an enterprise feature (in Beta). It is currently only supported for Batch Changes functionality. [#43276](https://github.com/sourcegraph/sourcegraph/issues/43276)
- Site admins can now [restrict creation of batch changes to certain users](https://docs.sourcegraph.com/admin/access_control/batch_changes) by tailoring their roles and the permissions granted to those roles. [#34491](https://github.com/sourcegraph/sourcegraph/issues/34491)
- Site admins can now [configure outgoing webhooks](https://docs.sourcegraph.com/admin/config/webhooks/outgoing) for Batch Changes to inform external tools of events related to Sourcegraph batch changes and their changesets. [#38278](https://github.com/sourcegraph/sourcegraph/issues/38278)
- [Sourcegraph Own](https://docs.sourcegraph.com/own) is now available as an experimental enterprise feature. Enable the `search-ownership` feature flag to use it.
- Gitserver supports a new `COURSIER_CACHE_DIR` env var to configure the cache location for coursier JVM package repos.
- Pings now emit a histogram of repository sizes cloned by Sourcegraph [48211](https://github.com/sourcegraph/sourcegraph/pull/48211).
- The search input has been redesigned to greatly improve usability. New contextual suggestions help users learn the Sourcegraph query language as they search. Suggestions have been unified across contexts and filters, and the history mode has been integrated into the input. Improved and expanded keyboard shortcuts also make navigation much easier. This functionality is in beta, and can be disabled in the user menu.

### Changed

- Experimental GraphQL query, `permissionsSyncJobs` is substituted with new non-experimental query which provides full information about permissions sync jobs stored in the database. [#47933](https://github.com/sourcegraph/sourcegraph/pull/47933)
- Renders `readme.txt` files in the repository page. [#47944](https://github.com/sourcegraph/sourcegraph/pull/47944)
- Renders GitHub pull request references in all places where a commit message is referenced. [#48183](https://github.com/sourcegraph/sourcegraph/pull/48183)
- CodeMirror blob view (default) uses selection-driven code navigation. [#48066](https://github.com/sourcegraph/sourcegraph/pull/48066)
- Older Code Insights data points will now be automatically archived as configured by the site configuration setting `insights.maximumSampleSize`, set to 30 by default. All points can be exported. This behaviour can be disabled using the experimental setting `insightsDataRetention`. [#48259](https://github.com/sourcegraph/sourcegraph/pull/48259)
- The admin debug GraphQL endpoint for Code Insights will now include the series metadata in the response. [#49473](https://github.com/sourcegraph/sourcegraph/pull/49473)
- Usage telemetry has been streamlined; there are no longer two categories (critical and non-critical), and telemetry will be streamlined and reviewed/reduced further in upcoming releases. The site admin flag `disableNonCriticalTelemetry` currently still remains but has no effect.

### Fixed

- The symbols service `CACHE_DIR` and `MAX_TOTAL_PATHS_LENGTH` were renamed to have a `SYMBOLS_` prefix in the last version of Sourcegraph; this version fixes a bug where the old names without the `SYMBOLS_` prefix were not respected correctly. Both names now work.
- Fixed issues with propagating tracing configuration throughout the application. [#47428](https://github.com/sourcegraph/sourcegraph/pull/47428)
- Enable `auto gc` on fetch when `SRC_ENABLE_GC_AUTO` is set to `true`. [#47852](https://github.com/sourcegraph/sourcegraph/pull/47852)
- Fixes syntax highlighting and line number issues in the code preview rendered inside the references panel. [#48107](https://github.com/sourcegraph/sourcegraph/pull/48107)
- The ordering of code host sync error messages in the notifications menu will now be persistent. Previously the order was not guaranteed on a refresh of the status messages, which would make the code host sync error messages jump positions, giving a false sense of change to the site admins. [#48722](https://github.com/sourcegraph/sourcegraph/pull/48722)
- Fixed Detect & Track Code Insights running over all repositories when during creation a search was used to specify the repositories for the insight. [#49633](https://github.com/sourcegraph/sourcegraph/pull/49633)

### Removed

- The LSIF upload endpoint is no longer supported and has been replaced by a diagnostic error page. src-cli v4.5+ will translate all local LSIF files to SCIP prior to upload. [#47547](https://github.com/sourcegraph/sourcegraph/pull/47547)
- The experimental setting `authz.syncJobsRecordsLimit` has been removed. [#47933](https://github.com/sourcegraph/sourcegraph/pull/47933)
- Storing permissions sync jobs statuses in Redis has been removed as now all permissions sync related data is stored in a database. [#47933](https://github.com/sourcegraph/sourcegraph/pull/47933)
- The key `shared_steps` has been removed from auto-indexing configuration descriptions. If you have a custom JSON auto-indexing configuration set for a repository that defines this key, you should inline the content into each index job's `steps` array. [#48770](https://github.com/sourcegraph/sourcegraph/pull/48770)

## 4.5.1

### Changed

- Updated git to version 2.39.2 to address [reported security vulnerabilities](https://github.blog/2023-02-14-git-security-vulnerabilities-announced-3/) [#47892](https://github.com/sourcegraph/sourcegraph/pull/47892/files)
- Updated curl to 7.88.1 to address [reported security vulnerabilities](https://curl.se/docs/CVE-2022-42915.html) [#48144](https://github.com/sourcegraph/sourcegraph/pull/48144)

## 4.5.0

### Added

- Endpoint environment variables (`SEARCHER_URL`, `SYMBOLS_URL`, `INDEXED_SEARCH_SERVERS`, `SRC_GIT_SERVERS`) now can be set to replica count values in Kubernetes, Kustomize, Helm and Docker Compose environments. This avoids the need to use service discovery or generating the respective list of addresses in those environments. [#45862](https://github.com/sourcegraph/sourcegraph/pull/45862)
- The default author and email for changesets will now be pulled from user account details when possible. [#46385](https://github.com/sourcegraph/sourcegraph/pull/46385)
- Code Insights has a new display option: "Max number of series points to display". This setting controls the number of data points you see per series on an insight. [#46653](https://github.com/sourcegraph/sourcegraph/pull/46653)
- Added out-of-band migration that will migrate all existing data from LSIF to SCIP (see additional [migration documentation](https://docs.sourcegraph.com/admin/how-to/lsif_scip_migration)). [#45106](https://github.com/sourcegraph/sourcegraph/pull/45106)
- Code Insights has a new search-powered repositories field that allows you to select repositories with Sourcegraph search syntax. [#45687](https://github.com/sourcegraph/sourcegraph/pull/45687)
- You can now export all data for a Code Insight from the card menu or the standalone page. [#46795](https://github.com/sourcegraph/sourcegraph/pull/46795), [#46694](https://github.com/sourcegraph/sourcegraph/pull/46694)
- Added Gerrit as an officially supported code host with permissions syncing. [#46763](https://github.com/sourcegraph/sourcegraph/pull/46763)
- Markdown files now support `<picture>` and `<video>` elements in the rendered view. [#47074](https://github.com/sourcegraph/sourcegraph/pull/47074)
- Batch Changes: Log outputs from execution steps are now paginated in the web interface. [#46335](https://github.com/sourcegraph/sourcegraph/pull/46335)
- Monitoring: the searcher dashboard now contains more detailed request metrics as well as information on interactions with the local cache (via gitserver). [#47654](https://github.com/sourcegraph/sourcegraph/pull/47654)
- Renders GitHub pull request references in the commit list. [#47593](https://github.com/sourcegraph/sourcegraph/pull/47593)
- Added a new background permissions syncer & scheduler which is backed by database, unlike the old one that was based on an in-memory processing queue. The new system is enabled by default, but can be disabled. Revert to the in-memory processing queue by setting the feature flag `database-permission-sync-worker` to `false`. [#47783](https://github.com/sourcegraph/sourcegraph/pull/47783)
- Zoekt introduces a new opt-in feature, "shard merging". Shard merging consolidates small index files into larger ones, which reduces Zoekt-webserver's memory footprint [documentation](https://docs.sourcegraph.com/code_search/explanations/search_details#shard-merging)
- Blob viewer is now backed by the CodeMirror editor. Previous table-based blob viewer can be re-enabled by setting `experimentalFeatures.enableCodeMirrorFileView` to `false`. [#47563](https://github.com/sourcegraph/sourcegraph/pull/47563)
- Code folding support for the CodeMirror blob viewer. [#47266](https://github.com/sourcegraph/sourcegraph/pull/47266)
- CodeMirror blob keyboard navigation as experimental feature. Can be enabled in settings by setting `experimentalFeatures.codeNavigation` to `selection-driven`. [#44698](https://github.com/sourcegraph/sourcegraph/pull/44698)

### Changed

- Archived and deleted changesets are no longer counted towards the completion percentage shown in the Batch Changes UI. [#46831](https://github.com/sourcegraph/sourcegraph/pull/46831)
- Code Insights has a new UI for the "Add or remove insights" view, which now allows you to search code insights by series label in addition to insight title. [#46538](https://github.com/sourcegraph/sourcegraph/pull/46538)
- When SMTP is configured, users created by site admins via the "Create user" page will no longer have their email verified by default - users must verify their emails by using the "Set password" link they get sent, or have their emails verified by a site admin via the "Emails" tab in user settings or the `setUserEmailVerified` mutation. The `createUser` mutation retains the old behaviour of automatically marking emails as verified. To learn more, refer to the [SMTP and email delivery](https://docs.sourcegraph.com/admin/config/email) documentation. [#46187](https://github.com/sourcegraph/sourcegraph/pull/46187)
- Connection checks for code host connections have been changed to talk to code host APIs directly via HTTP instead of doing DNS lookup and TCP dial. That makes them more resistant in environments where proxies are used. [#46918](https://github.com/sourcegraph/sourcegraph/pull/46918)
- Expiration of licenses is now handled differently. When a license is expired promotion to site-admin is disabled, license-specific features are disabled (exceptions being SSO & permission syncing), grace period has been replaced with a 7-day-before-expiration warning. [#47251](https://github.com/sourcegraph/sourcegraph/pull/47251)
- Searcher will now timeout searches in 2 hours instead of 10 minutes. This timeout was raised for batch use cases (such as code insights) searching old revisions in very large repositories. This limit can be tuned with the environment variable `PROCESSING_TIMEOUT`. [#47469](https://github.com/sourcegraph/sourcegraph/pull/47469)
- Zoekt now bypasses the regex engine for queries that are common in the context of search-based code intelligence, such as `\bLITERAL\b case:yes`. This can lead to a significant speed-up for "Find references" and "Find implementations" if precise code intelligence is not available. [zoekt#526](https://github.com/sourcegraph/zoekt/pull/526)
- The Sourcegraph Free license has undergone a number of changes. Please contact support@sourcegraph.com with any questions or concerns. [#46504](https://github.com/sourcegraph/sourcegraph/pull/46504)
  - The Free license allows for only a single private repository on the instance.
  - The Free license does not support SSO of any kind.
  - The Free license does not offer mirroring of code host user permissions.
- Expired Sourcegraph licenses no longer allow continued use of the product. [#47251](https://github.com/sourcegraph/sourcegraph/pull/47251)
  - Licensed features are disabled once a license expires.
  - Users can no longer be promoted to Site Admins once a license expires.

### Fixed

- Resolved issue which would prevent Batch Changes from being able to update changesets on forks of repositories on Bitbucket Server created prior to version 4.2. [#47397](https://github.com/sourcegraph/sourcegraph/pull/47397)
- Fixed a bug where changesets created on forks of repositories in a personal user's namespace on GitHub could not be updated after creation. [#47397](https://github.com/sourcegraph/sourcegraph/pull/47397)
- Fixed a bug where saving default Sort & Limit filters in Code Insights did not persist [#46653](https://github.com/sourcegraph/sourcegraph/pull/46653)
- Restored the old syntax for `repo:contains` filters that was previously removed in version 4.0.0. For now, both the old and new syntaxes are supported to allow for smooth upgrades. Users are encouraged to switch to the new syntax, since the old one may still be removed in a future version.
- Fixed a bug where removing an auth provider would render a user's Account Security page inaccessible if they still had an external account associated with the removed auth provider. [#47092](https://github.com/sourcegraph/sourcegraph/pull/47092)
- Fixed a bug where the `repo:has.description()` parameter now correctly shows description of a repository synced from a Bitbucket server code host connection, while previously it used to show the repository name instead [#46752](https://github.com/sourcegraph/sourcegraph/pull/46752)
- Fixed a bug where permissions syncs consumed more rate limit tokens than required. This should lead to speed-ups in permission syncs, as well as other possible cases where a process runs in repo-updater. [#47374](https://github.com/sourcegraph/sourcegraph/pull/47374)
- Fixes UI bug where folders with single child were appearing as child folders themselves. [#46628](https://github.com/sourcegraph/sourcegraph/pull/46628)
- Performance issue with the Outbound requests page. [#47544](https://github.com/sourcegraph/sourcegraph/pull/47544)

### Removed

- The Code insights "run over all repositories" mode has been replaced with search-powered repositories filed syntax. [#45687](https://github.com/sourcegraph/sourcegraph/pull/45687)
- The settings `search.repositoryGroups`, `codeInsightsGqlApi`, `codeInsightsAllRepos`, `experimentalFeatures.copyQueryButton`,, `experimentalFeatures.showRepogroupHomepage`, `experimentalFeatures.showOnboardingTour`, `experimentalFeatures.showSearchContextManagement` and `codeIntelligence.autoIndexRepositoryGroups` have been removed as they were deprecated and unsued. [#47481](https://github.com/sourcegraph/sourcegraph/pull/47481)
- The site config `enableLegacyExtensions` setting was removed. It is no longer possible to enable legacy Sourcegraph extension API functionality in this version.

## 4.4.2

### Changed

- Expiration of licenses is now handled differently. When a license is expired promotion to site-admin is disabled, license-specific features are disabled (exceptions being SSO & permission syncing), grace period has been replaced with a 7-day-before-expiration warning. [#47251](https://github.com/sourcegraph/sourcegraph/pull/47251)

## 4.4.1

### Changed

- Connection checks for code host connections have been changed to talk to code host APIs directly via HTTP instead of doing DNS lookup and TCP dial. That makes them more resistant in environments where proxies are used. [#46918](https://github.com/sourcegraph/sourcegraph/pull/46918)
- The search query input overflow behavior on search home page has been fixed. [#46922](https://github.com/sourcegraph/sourcegraph/pull/46922)

## 4.4.0

### Added

- Added a button "Reindex now" to the index status page. Admins can now force an immediate reindex of a repository. [#45533](https://github.com/sourcegraph/sourcegraph/pull/45533)
- Added an option "Unlock user" to the actions dropdown on the Site Admin Users page. Admins can unlock user accounts that wer locked after too many sign-in attempts. [#45650](https://github.com/sourcegraph/sourcegraph/pull/45650)
- Templates for certain emails sent by Sourcegraph are now configurable via `email.templates` in site configuration. [#45671](https://github.com/sourcegraph/sourcegraph/pull/45671), [#46085](https://github.com/sourcegraph/sourcegraph/pull/46085)
- Keyboard navigation for search results is now enabled by default. Use Arrow Up/Down keys to navigate between search results, Arrow Left/Right to collapse and expand file matches, Enter to open the search result in the current tab, Ctrl/Cmd+Enter to open the result in a separate tab, / to refocus the search input, and Ctrl/Cmd+Arrow Down to jump from the search input to the first result. Arrow Left/Down/Up/Right in previous examples can be substituted with h/j/k/l for Vim-style bindings. Keyboard navigation can be disabled by creating the `search-results-keyboard-navigation` feature flag and setting it to false. [#45890](https://github.com/sourcegraph/sourcegraph/pull/45890)
- Added support for receiving GitLab webhook `push` events. [#45856](https://github.com/sourcegraph/sourcegraph/pull/45856)
- Added support for receiving Bitbucket Server / Datacenter webhook `push` events. [#45909](https://github.com/sourcegraph/sourcegraph/pull/45909)
- Monitoring: Indexed-Search's dashboard now has new graphs for search request durations and "in-flight" search request workloads [#45966](https://github.com/sourcegraph/sourcegraph/pull/45966)
- The GraphQL API now supports listing single-file commit history across renames (with `GitCommit.ancestors(follow: true, path: "<some-path>")`). [#45882](https://github.com/sourcegraph/sourcegraph/pull/45882)
- Added support for receiving Bitbucket Cloud webhook `push` events. [#45960](https://github.com/sourcegraph/sourcegraph/pull/45960)
- Added a way to test code host connection from the `Manage code hosts` page. [#45972](https://github.com/sourcegraph/sourcegraph/pull/45972)
- Updates to the site configuration from the site admin panel will now also record the user id of the author in the database in the `critical_and_site_config.author_user_id` column. [#46150](https://github.com/sourcegraph/sourcegraph/pull/46150)
- When setting and resetting passwords, if the user's primary email address is not yet verified, using the password reset link sent via email will now also verify the email address. [#46307](https://github.com/sourcegraph/sourcegraph/pull/46307)
- Added new code host details and updated edit code host pages in site admin area. [#46327](https://github.com/sourcegraph/sourcegraph/pull/46327)
- If the experimental setting `insightsDataRetention` is enabled, the number of Code Insights data points that can be viewed will be limited by the site configuration setting `insights.maximumSampleSize`, set to 30 by default. Older points beyond that number will be periodically archived. [#46206](https://github.com/sourcegraph/sourcegraph/pull/46206), [#46440](https://github.com/sourcegraph/sourcegraph/pull/46440)
- Bitbucket Cloud can now be added as an authentication provider on Sourcegraph. [#46309](https://github.com/sourcegraph/sourcegraph/pull/46309)
- Bitbucket Cloud code host connections now support permissions syncing. [#46312](https://github.com/sourcegraph/sourcegraph/pull/46312)
- Keep a log of corruption events that happen on repositories as they are detected. The Admin repositories page will now show when a repository has been detected as being corrupt and they'll also be able to see a history log of the corruption for that repository. [#46004](https://github.com/sourcegraph/sourcegraph/pull/46004)
- Added corrupted statistic as part of the global repositories statistics. [46412](https://github.com/sourcegraph/sourcegraph/pull/46412)
- Added a `Corrupted` status filter on the Admin repositories page, allowing Administrators to filter the list of repositories to only those that have been detected as corrupt. [#46415](https://github.com/sourcegraph/sourcegraph/pull/46415)
- Added “Background job dashboard” admin feature [#44901](https://github.com/sourcegraph/sourcegraph/pull/44901)

### Changed

- Code Insights no longer uses a custom index of commits to compress historical backfill and instead queries the repository log directly. This allows the compression algorithm to span any arbitrary time frame, and should improve the reliability of the compression in general. [#45644](https://github.com/sourcegraph/sourcegraph/pull/45644)
- GitHub code host configuration: The error message for non-existent organizations has been clarified to indicate that the organization is one that the user manually specified in their code host configuration. [#45918](https://github.com/sourcegraph/sourcegraph/pull/45918)
- Git blame view got a user-interface overhaul and now shows data in a more structured way with additional visual hints. [#44397](https://github.com/sourcegraph/sourcegraph/issues/44397)
- User emails marked as unverified will no longer receive code monitors and account update emails - unverified emails can be verified from the user settings page to continue receiving these emails. [#46184](https://github.com/sourcegraph/sourcegraph/pull/46184)
- Zoekt by default eagerly unmarshals the symbol index into memory. Previously we would unmarshal on every request for the purposes of symbol searches or ranking. This lead to pressure on the Go garbage collector. On sourcegraph.com we have noticed time spent in the garbage collector halved. In the unlikely event this leads to more OOMs in zoekt-webserver, you can disable by setting the environment variable `ZOEKT_ENABLE_LAZY_DOC_SECTIONS=t`. [zoekt#503](https://github.com/sourcegraph/zoekt/pull/503)
- Removes the right side action sidebar that is shown on the code view page and moves the icons into the top nav. [#46339](https://github.com/sourcegraph/sourcegraph/pull/46339)
- The `sourcegraph/prometheus` image no longer starts with `--web.enable-lifecycle --web.enable-admin-api` by default - these flags can be re-enabled by configuring `PROMETHEUS_ADDITIONAL_FLAGS` on the container. [#46393](https://github.com/sourcegraph/sourcegraph/pull/46393)
- The experimental setting `authz.syncJobsRecordsTTL` has been changed to `authz.syncJobsRecordsLimit` - records are no longer retained based on age, but based on this size cap. [#46676](https://github.com/sourcegraph/sourcegraph/pull/46676)
- Renders GitHub pull request references in git blame view. [#46409](https://github.com/sourcegraph/sourcegraph/pull/46409)

### Fixed

- Made search results export use the same results list as the search results page. [#45702](https://github.com/sourcegraph/sourcegraph/pull/45702)
- Code insights with more than 1 year of history will correctly show 12 data points instead of 11. [#45644](https://github.com/sourcegraph/sourcegraph/pull/45644)
- Hourly code insights will now behave correctly and will no longer truncate to midnight UTC on the calendar date the insight was created. [#45644](https://github.com/sourcegraph/sourcegraph/pull/45644)
- Code Insights: fixed an issue where filtering by a search context that included multiple repositories would exclude data. [#45574](https://github.com/sourcegraph/sourcegraph/pull/45574)
- Ignore null JSON objects returned from GitHub API when listing public repositories. [#45969](https://github.com/sourcegraph/sourcegraph/pull/45969)
- Fixed issue where emails that have never been verified before would be unable to receive resent verification emails. [#46185](https://github.com/sourcegraph/sourcegraph/pull/46185)
- Resolved issue preventing LSIF uploads larger than 2GiB (gzipped) from uploading successfully. [#46209](https://github.com/sourcegraph/sourcegraph/pull/46209)
- Local vars in Typescript are now detected as symbols which will positively impact ranking of search results. [go-ctags#10](https://github.com/sourcegraph/go-ctags/pull/10)
- Fix issue in Gitlab OAuth in which user group membership is set too wide - adds `min_access_level=10` to `/groups` request. [#46480](https://github.com/sourcegraph/sourcegraph/pull/46480)

### Removed

- The extension registry no longer supports browsing, creating, or updating legacy extensions. Existing extensions may still be enabled or disabled in user settings and may be listed via the API. (The extension API was deprecated in 2022-09 but is still available if the `enableLegacyExtensions` site config experimental features flag is enabled.)
- User and organization auto-defined search contexts have been permanently removed along with the `autoDefinedSearchContexts` GraphQL query. The only auto-defined context now is the `global` context. [#46083](https://github.com/sourcegraph/sourcegraph/pull/46083)
- The settings `experimentalFeatures.showSearchContext`, `experimentalFeatures.showSearchNotebook`, and `experimentalFeatures.codeMonitoring` have been removed and these features are now permanently enabled when available. [#46086](https://github.com/sourcegraph/sourcegraph/pull/46086)
- The legacy panels on the homepage (recent searches, etc) which were turned off by default but could still be re-enabled by setting `experimentalFeatures.showEnterpriseHomePanels` to true, are permanently removed now. [#45705](https://github.com/sourcegraph/sourcegraph/pull/45705)
- The `site { monitoringStatistics { alerts } }` GraphQL query has been deprecated and will no longer return any data. The query will be removed entirely in a future release. [#46299](https://github.com/sourcegraph/sourcegraph/pull/46299)
- The Monaco version of the search query input and the corresponding feature flag (`experimentalFeatures.editor`) have been permanently removed. [#46249](https://github.com/sourcegraph/sourcegraph/pull/46249)

## 4.3.1

### Changed

- A bug that broke the site-admin page when no repositories have been added to the Sourcegraph instance has been fixed. [#46123](https://github.com/sourcegraph/sourcegraph/pull/46123)

## 4.3.0

### Added

- A "copy path" button has been added to file content, path, and symbol search results on hover or focus, next to the file path. The button copies the relative path of the file in the repo, in the same way as the "copy path" button in the file and repo pages. [#42721](https://github.com/sourcegraph/sourcegraph/pull/42721)
- Unindexed search now use the index for files that have not changed between the unindexed commit and the indexed commit. The result is faster unindexed search in general. If you are noticing issues you can disable by setting the feature flag `search-hybrid` to false. [#37112](https://github.com/sourcegraph/sourcegraph/issues/37112)
- The number of commits listed in the History tab can now be customized for all users by site admins under Configuration -> Global Settings from the site admin page by using the config `history.defaultPageSize`. Individual users may also set `history.defaultPagesize` from their user settings page to override the value set under the Global Settings. [#44651](https://github.com/sourcegraph/sourcegraph/pull/44651)
- Batch Changes: Mounted files can be accessed via the UI on the executions page. [#43180](https://github.com/sourcegraph/sourcegraph/pull/43180)
- Added "Outbound request log" feature for site admins [#44286](https://github.com/sourcegraph/sourcegraph/pull/44286)
- Code Insights: the data series API now provides information about incomplete datapoints during processing
- Added a best-effort migration such that existing Code Insights will display zero results instead of missing points at the start and end of a graph. [#44928](https://github.com/sourcegraph/sourcegraph/pull/44928)
- More complete stack traces for Outbound request log [#45151](https://github.com/sourcegraph/sourcegraph/pull/45151)
- A new status message now reports how many repositories have already been indexed for search. [#45246](https://github.com/sourcegraph/sourcegraph/pull/45246)
- Search contexts can now be starred (favorited) in the search context management page. Starred search contexts will appear before other contexts in the context dropdown menu next to the search box. [#45230](https://github.com/sourcegraph/sourcegraph/pull/45230)
- Search contexts now let you set a context as your default. The default will be selected every time you open Sourcegraph and will appear near the top in the context dropdown menu next to the search box. [#45387](https://github.com/sourcegraph/sourcegraph/pull/45387)
- [search.largeFiles](https://docs.sourcegraph.com/admin/config/site_config#search-largeFiles) accepts an optional prefix `!` to negate a pattern. The order of the patterns within search.largeFiles is honored such that the last pattern matching overrides preceding patterns. For patterns that begin with a literal `!` prefix with a backslash, for example, `\!fileNameStartsWithExcl!.txt`. Previously indexed files that become excluded due to this change will remain in the index until the next reindex [#45318](https://github.com/sourcegraph/sourcegraph/pull/45318)
- [Webhooks](https://docs.sourcegraph.com/admin/config/webhooks/incoming) have been overhauled completely and can now be found under **Site admin > Repositories > Incoming webhooks**. Webhooks that were added via code host configuration are [deprecated](https://docs.sourcegraph.com/admin/config/webhooks/incoming#deprecation-notice) and will be removed in 5.1.0.
- Added support for receiving webhook `push` events from GitHub which will trigger Sourcegraph to fetch the latest commit rather than relying on polling.
- Added support for private container registries in Sourcegraph executors. [Using private registries](https://docs.sourcegraph.com/admin/deploy_executors#using-private-registries)

### Changed

- Batch Change: When one or more changesets are selected, we now display all bulk operations but disable the ones that aren't applicable to the changesets. [#44617](https://github.com/sourcegraph/sourcegraph/pull/44617)
- Gitserver's repository purge worker now runs on a regular interval instead of just on weekends, configurable by the `repoPurgeWorker` site configuration. [#44753](https://github.com/sourcegraph/sourcegraph/pull/44753)
- Editing the presentation metadata (title, line color, line label) or the default filters of a scoped Code Insight will no longer trigger insight recalculation. [#44769](https://github.com/sourcegraph/sourcegraph/pull/44769), [#44797](https://github.com/sourcegraph/sourcegraph/pull/44797)
- Indexed Search's `memory_map_areas_percentage_used` alert has been modified to alert earlier than it used to. It now issues a warning at 60% (previously 70%) and issues a critical alert at 80% (previously 90%).
- Saving a new view of a scoped Code Insight will no longer trigger insight recalculation. [#44679](https://github.com/sourcegraph/sourcegraph/pull/44679)

### Fixed

- The Code Insights commit indexer no longer errors when fetching commits from empty repositories when sub-repo permissions are enabled. [#44558](https://github.com/sourcegraph/sourcegraph/pull/44558)
- Unintended newline characters that could appear in diff view rendering have been fixed. [#44805](https://github.com/sourcegraph/sourcegraph/pull/44805)
- Signing out doesn't immediately log the user back in when there's only one OAuth provider enabled. It now redirects the user to the Sourcegraph login page. [#44803](https://github.com/sourcegraph/sourcegraph/pull/44803)
- An issue causing certain kinds of queries to behave inconsistently in Code Insights. [#44917](https://github.com/sourcegraph/sourcegraph/pull/44917)
- When the setting `batchChanges.enforceForks` is enabled, Batch Changes will now prefix the name of the fork repo it creates with the original repo's namespace name in order to prevent repo name collisions. [#43681](https://github.com/sourcegraph/sourcegraph/pull/43681), [#44458](https://github.com/sourcegraph/sourcegraph/pull/44458), [#44548](https://github.com/sourcegraph/sourcegraph/pull/44548), [#44924](https://github.com/sourcegraph/sourcegraph/pull/44924)
- Code Insights: fixed an issue where certain queries matching sequential whitespace characters would overcount. [#44969](https://github.com/sourcegraph/sourcegraph/pull/44969)
- GitHub fine-grained Personal Access Tokens can now clone repositories correctly, but are not yet officially supported. [#45137](https://github.com/sourcegraph/sourcegraph/pull/45137)
- Detect-and-track Code Insights will now return data for repositories without sub-repo permissions even when sub-repo permissions are enabled on the instance. [#45631](https://github.com/sourcegraph/sourcegraph/pull/45361)

### Removed

- Removed legacy GraphQL field `dirtyMetadata` on an insight series. `insightViewDebug` can be used as an alternative. [#44416](https://github.com/sourcegraph/sourcegraph/pull/44416)
- Removed `search.index.enabled` site configuration setting. Search indexing is now always enabled.
- Removed the experimental feature setting `showSearchContextManagement`. The search context management page is now available to all users with access to search contexts. [#45230](https://github.com/sourcegraph/sourcegraph/pull/45230)
- Removed the experimental feature setting `showComputeComponent`. Any notebooks that made use of the compute component will no longer render the block. The block will be deleted from the databse the next time a notebook that uses it is saved. [#45360](https://github.com/sourcegraph/sourcegraph/pull/45360)

## 4.2.1

- `minio` has been replaced with `blobstore`. Please see the update notes here: https://docs.sourcegraph.com/admin/how-to/blobstore_update_notes

## 4.2.0

### Added

- Creating access tokens is now tracked in the security events. [#43226](https://github.com/sourcegraph/sourcegraph/pull/43226)
- Added `codeIntelAutoIndexing.indexerMap` to site-config that allows users to update the indexers used when inferring precise code intelligence auto-indexing jobs (without having to overwrite the entire inference scripts). For example, `"codeIntelAutoIndexing.indexerMap": {"go": "my.registry/sourcegraph/lsif-go"}` will cause Go projects to use the specified container (in a alternative Docker registry). [#43199](https://github.com/sourcegraph/sourcegraph/pull/43199)
- Code Insights data points that do not contain any results will display zero instead of being omitted from the visualization. Only applies to insight data created after 4.2. [#43166](https://github.com/sourcegraph/sourcegraph/pull/43166)
- Sourcegraph ships with node-exporter, a Prometheus tool that provides hardware / OS metrics that helps Sourcegraph scale your deployment. See your deployment update for more information:
  - [Kubernetes](https://docs.sourcegraph.com/admin/updates/kubernetes)
  - [Docker Compose](https://docs.sourcegraph.com/admin/updates/docker_compose)
- A structural search diagnostic to warn users when a language filter is not set. [#43835](https://github.com/sourcegraph/sourcegraph/pull/43835)
- GitHub/GitLab OAuth success/fail attempts are now a part of the audit log. [#43886](https://github.com/sourcegraph/sourcegraph/pull/43886)
- When rendering a file which is backed by Git LFS, we show a page informing the file is LFS and linking to the file on the codehost. Previously we rendered the LFS pointer. [#43686](https://github.com/sourcegraph/sourcegraph/pull/43686)
- Batch changes run server-side now support secrets. [#27926](https://github.com/sourcegraph/sourcegraph/issues/27926)
- OIDC success/fail login attempts are now a part of the audit log. [#44467](https://github.com/sourcegraph/sourcegraph/pull/44467)
- A new experimental GraphQL query, `permissionsSyncJobs`, that lists the states of recently completed permissions sync jobs and the state of each provider. The TTL of entries retrained can be configured with `authz.syncJobsRecordsTTL`. [#44387](https://github.com/sourcegraph/sourcegraph/pull/44387), [#44258](https://github.com/sourcegraph/sourcegraph/pull/44258)
- The search input has a new search history button and allows cycling through recent searches via up/down arrow keys. [#44544](https://github.com/sourcegraph/sourcegraph/pull/44544)
- Repositories can now be ordered by size on the repo admin page. [#44360](https://github.com/sourcegraph/sourcegraph/pull/44360)
- The search bar contains a new Smart Search toggle. If a search returns no results, Smart Search attempts alternative queries based on a fixed set of rules, and shows their results (if there are any). Smart Search is enabled by default. It can be disabled by default with `"search.defaultMode": "precise"` in settings. [#44385](https://github.com/sourcegraph/sourcegraph/pull/44395)
- Repositories in the site-admin area can now be filtered, so that only indexed repositories are displayed [#45288](https://github.com/sourcegraph/sourcegraph/pull/45288)

### Changed

- Updated minimum required version of `git` to 2.38.1 in `gitserver` and `server` Docker image. This addresses: https://github.blog/2022-04-12-git-security-vulnerability-announced/ and https://lore.kernel.org/git/d1d460f6-e70f-b17f-73a5-e56d604dd9d5@github.com/. [#43615](https://github.com/sourcegraph/sourcegraph/pull/43615)
- When a `content:` filter is used in a query, only file contents will be searched (previously any of file contents, paths, or repos were searched). However, as before, if `type:` is also set, the `content:` filter will search for results of the specified `type:`. [#43442](https://github.com/sourcegraph/sourcegraph/pull/43442)
- Updated [p4-fusion](https://github.com/salesforce/p4-fusion) from `1.11` to `1.12`.

### Fixed

- Fixed a bug where path matches on files in the root directory of a repository were not highlighted. [#43275](https://github.com/sourcegraph/sourcegraph/pull/43275)
- Fixed a bug where a search query wouldn't be validated after the query type has changed. [#43849](https://github.com/sourcegraph/sourcegraph/pull/43849)
- Fixed an issue with insights where a single erroring insight would block access to all insights. This is a breaking change for users of the insights GraphQL api as the `InsightViewConnection.nodes` list may now contain `null`. [#44491](https://github.com/sourcegraph/sourcegraph/pull/44491)
- Fixed a bug where Open in Editor didn't work well with `"repositoryPathPattern" = "{nameWithOwner}"` [#43839](https://github.com/sourcegraph/sourcegraph/pull/44475)

### Removed

- Remove the older `log.gitserver.accessLogs` site config setting. The setting is succeeded by `log.auditLog.gitserverAccess`. [#43174](https://github.com/sourcegraph/sourcegraph/pull/43174)
- Remove `LOG_ALL_GRAPHQL_REQUESTS` env var. The setting is succeeded by `log.auditLog.graphQL`. [#43181](https://github.com/sourcegraph/sourcegraph/pull/43181)
- Removed support for setting `SRC_ENDPOINTS_CONSISTENT_HASH`. This was an environment variable to support the transition to a new consistent hashing scheme introduced in 3.31.0. [#43528](https://github.com/sourcegraph/sourcegraph/pull/43528)
- Removed legacy environment variable `ENABLE_CODE_INSIGHTS_SETTINGS_STORAGE` used in old versions of Code Insights to fall back to JSON settings based storage. All data was previously migrated in version 3.35 and this is no longer supported.

## 4.1.3

### Fixed

- Fixed a bug that caused the Phabricator native extension to not load the right CSS assets. [#43868](https://github.com/sourcegraph/sourcegraph/pull/43868)
- Fixed a bug that prevented search result exports to load. [#43344](https://github.com/sourcegraph/sourcegraph/pull/43344)

## 4.1.2

### Fixed

- Fix code navigation on OSS when CodeIntel is unavailable. [#43458](https://github.com/sourcegraph/sourcegraph/pull/43458)

### Removed

- Removed the onboarding checklist for new users that showed up in the top navigation bar, on user profiles, and in the site-admin overview page. After changes to the underlying user statistics system, the checklist caused severe performance issues for customers with large and heavily-used instances. [#43591](https://github.com/sourcegraph/sourcegraph/pull/43591)

## 4.1.1

### Fixed

- Fixed a bug with normalizing the `published` draft value for `changeset_specs`. [#43390](https://github.com/sourcegraph/sourcegraph/pull/43390)

## 4.1.0

### Added

- Outdated executors now show a warning from the admin page. [#40916](https://github.com/sourcegraph/sourcegraph/pull/40916)
- Added support for better Slack link previews for private instances. Link previews are currently feature-flagged, and site admins can turn them on by creating the `enable-link-previews` feature flag on the `/site-admin/feature-flags` page. [#41843](https://github.com/sourcegraph/sourcegraph/pull/41843)
- Added a new button in the repository settings, under "Mirroring", to delete a repository from disk and reclone it. [#42177](https://github.com/sourcegraph/sourcegraph/pull/42177)
- Batch changes run on the server can now be created within organisations. [#36536](https://github.com/sourcegraph/sourcegraph/issues/36536)
- GraphQL request logs are now compliant with the audit logging format. The old GraphQl logging based on `LOG_ALL_GRAPHQL_REQUESTS` env var is now deprecated and scheduled for removal. [#42550](https://github.com/sourcegraph/sourcegraph/pull/42550)
- Mounting files now works when running batch changes server side. [#31792](https://github.com/sourcegraph/sourcegraph/issues/31792)
- Added mini dashboard of total batch change metrics to the top of the batch changes list page. [#42046](https://github.com/sourcegraph/sourcegraph/pull/42046)
- Added repository sync counters to the code host details page. [#43039](https://github.com/sourcegraph/sourcegraph/pull/43039)

### Changed

- Git server access logs are now compliant with the audit logging format. Breaking change: The 'actor' field is now nested under 'audit' field. [#41865](https://github.com/sourcegraph/sourcegraph/pull/41865)
- All Perforce rules are now stored together in one column and evaluated on a "last rule takes precedence" basis. [#41785](https://github.com/sourcegraph/sourcegraph/pull/41785)
- Security events are now a part of the audit log. [#42653](https://github.com/sourcegraph/sourcegraph/pull/42653)
- "GC AUTO" is now the default garbage collection job. We disable sg maintenance, which had previously replace "GC AUTO", after repeated reports about repo corruption. [#42856](https://github.com/sourcegraph/sourcegraph/pull/42856)
- Security events (audit log) can now optionally omit the internal actor actions (internal traffic). [#42946](https://github.com/sourcegraph/sourcegraph/pull/42946)
- To use the optional `customGitFetch` feature, the `ENABLE_CUSTOM_GIT_FETCH` env var must be set on `gitserver`. [#42704](https://github.com/sourcegraph/sourcegraph/pull/42704)

### Fixed

- WIP changesets in Gitlab >= 14.0 are now prefixed with `Draft:` instead of `WIP:` to accomodate for the [breaking change in Gitlab 14.0](https://docs.gitlab.com/ee/update/removals.html#wip-merge-requests-renamed-draft-merge-requests). [#42024](https://github.com/sourcegraph/sourcegraph/pull/42024)
- When updating the site configuration, the provided Last ID is now used to prevent race conditions when simultaneous config updates occur. [#42691](https://github.com/sourcegraph/sourcegraph/pull/42691)
- When multiple auth providers of the same external service type is set up, there are now separate entries in the user's Account Security settings. [#42865](https://github.com/sourcegraph/sourcegraph/pull/42865)
- Fixed a bug with GitHub code hosts that did not label archived repos correctly when using the "public" repositoryQuery keyword. [#41461](https://github.com/sourcegraph/sourcegraph/pull/41461)
- Fixed a bug that would display the blank batch spec that a batch change is initialized with in the batch specs executions tab. [#42914](https://github.com/sourcegraph/sourcegraph/pull/42914)
- Fixed a bug that would cause menu dropdowns to not open appropriately. [#42779](https://github.com/sourcegraph/sourcegraph/pull/42779)

### Removed

-

## 4.0.1

### Fixed

- Fixed a panic that can be caused by some tracing configurations. [#42027](https://github.com/sourcegraph/sourcegraph/pull/42027)
- Fixed broken code navigation for Javascript. [#42055](https://github.com/sourcegraph/sourcegraph/pull/42055)
- Fixed issue with empty code navigation popovers. [#41958](https://github.com/sourcegraph/sourcegraph/pull/41958)

## 4.0.0

### Added

- A new look for Sourcegraph, previously in beta as "Simple UI", is now permanently enabled. [#41021](https://github.com/sourcegraph/sourcegraph/pull/41021)
- A new [multi-version upgrade](https://docs.sourcegraph.com/admin/updates#multi-version-upgrades) process now allows Sourcegraph instances to upgrade more than a single minor version. Instances at version 3.20 or later can now jump directly to 4.0. [#40628](https://github.com/sourcegraph/sourcegraph/pull/40628)
- Matching ranges in file paths are now highlighted for path results and content results. Matching paths in repository names are now highlighted for repository results. [#41296](https://github.com/sourcegraph/sourcegraph/pull/41296) [#41385](https://github.com/sourcegraph/sourcegraph/pull/41385) [#41470](https://github.com/sourcegraph/sourcegraph/pull/41470)
- Aggregations by repository, file, author, and capture group are now provided for search results. [#39643](https://github.com/sourcegraph/sourcegraph/issues/39643)
- Blob views and search results are now lazily syntax highlighted for better performance. [#39563](https://github.com/sourcegraph/sourcegraph/pull/39563) [#40263](https://github.com/sourcegraph/sourcegraph/pull/40263)
- File links in both the search results and the blob sidebar and now prefetched on hover or focus. [#40354](https://github.com/sourcegraph/sourcegraph/pull/40354) [#41420](https://github.com/sourcegraph/sourcegraph/pull/41420)
- Negation support for the search predicates `-repo:has.path()` and `-repo:has.content()`. [#40283](https://github.com/sourcegraph/sourcegraph/pull/40283)
- Experimental clientside OpenTelemetry can now be enabled with `"observability.client": { "openTelemetry": "/-/debug/otlp" }`, which sends OpenTelemetry to the new [bundled OpenTelemetry Collector](https://docs.sourcegraph.com/admin/observability/opentelemetry). [#37907](https://github.com/sourcegraph/sourcegraph/issues/37907)
- File diff stats are now characterized by 2 figures: lines added and lines removed. Previously, a 3rd figure for lines modified was also used. This is represented by the fields on the `DiffStat` type on the GraphQL API. [#40454](https://github.com/sourcegraph/sourcegraph/pull/40454)

### Changed

- [Sourcegraph with Kubernetes (without Helm)](https://docs.sourcegraph.com/admin/deploy/kubernetes): The `jaeger-agent` sidecar has been replaced by an [OpenTelemetry Collector](https://docs.sourcegraph.com/admin/observability/opentelemetry) DaemonSet + Deployment configuration. The bundled Jaeger instance is now disabled by default, instead of enabled. [#40456](https://github.com/sourcegraph/sourcegraph/issues/40456)
- [Sourcegraph with Docker Compose](https://docs.sourcegraph.com/admin/deploy/docker-compose): The `jaeger` service has been replaced by an [OpenTelemetry Collector](https://docs.sourcegraph.com/admin/observability/opentelemetry) service. The bundled Jaeger instance is now disabled by default, instead of enabled. [#40455](https://github.com/sourcegraph/sourcegraph/issues/40455)
- `"observability.tracing": { "type": "opentelemetry" }` is now the default tracer type. To revert to existing behaviour, set `"type": "jaeger"` instead. The legacy values `"type": "opentracing"` and `"type": "datadog"` have been removed. [#41242](https://github.com/sourcegraph/sourcegraph/pull/41242)
- `"observability.tracing": { "urlTemplate": "" }` is now the default, and if `"urlTemplate"` is left empty, no trace URLs are generated. To revert to existing behaviour, set `"urlTemplate": "{{ .ExternalURL }}/-/debug/jaeger/trace/{{ .TraceID }}"` instead. [#41242](https://github.com/sourcegraph/sourcegraph/pull/41242)
- Code host connection tokens are no longer supported as a fallback method for syncing changesets in Batch Changes. [#25394](https://github.com/sourcegraph/sourcegraph/issues/25394)
- **IMPORTANT:** `repo:contains(file:foo content:bar)` has been renamed to `repo:contains.file(path:foo content:bar)`. `repo:contains.file(foo)` has been renamed to `repo:contains.path(foo)`. `repo:contains()` **is no longer a valid predicate. Saved searches using** `repo:contains()` **will need to be updated to use the new syntax.** [#40389](https://github.com/sourcegraph/sourcegraph/pull/40389)

### Fixed

- Fixed support for bare repositories using the src-cli and other codehost type. This requires the latest version of src-cli. [#40863](https://github.com/sourcegraph/sourcegraph/pull/40863)
- The recommended [src-cli](https://github.com/sourcegraph/src-cli) version is now reported consistently. [#39468](https://github.com/sourcegraph/sourcegraph/issues/39468)
- A performance issue affecting structural search causing results to not stream. It is much faster now. [#40872](https://github.com/sourcegraph/sourcegraph/pull/40872)
- An issue where the saved search input box reports an invalid pattern type for `standard`, which is now valid. [#41068](https://github.com/sourcegraph/sourcegraph/pull/41068)
- Git will now respect system certificate authorities when specifying `certificates` for the `tls.external` site configuration. [#38128](https://github.com/sourcegraph/sourcegraph/issues/38128)
- Fixed a bug where setting `"observability.tracing": {}` would disable tracing, when the intended behaviour is to default to tracing with `"sampling": "selective"` enabled by default. [#41242](https://github.com/sourcegraph/sourcegraph/pull/41242)
- The performance, stability, and latency of search predicates like `repo:has.file()`, `repo:has.content()`, and `file:has.content()` have been dramatically improved. [#418](https://github.com/sourcegraph/zoekt/pull/418), [#40239](https://github.com/sourcegraph/sourcegraph/pull/40239), [#38988](https://github.com/sourcegraph/sourcegraph/pull/38988), [#39501](https://github.com/sourcegraph/sourcegraph/pull/39501)
- A search query issue where quoted patterns inside parenthesized expressions would be interpreted incorrectly. [#41455](https://github.com/sourcegraph/sourcegraph/pull/41455)

### Removed

- `CACHE_DIR` has been removed from the `sourcegraph-frontend` deployment. This required ephemeral storage which will no longer be needed. This variable (and corresponding filesystem mount) has been unused for many releases. [#38934](https://github.com/sourcegraph/sourcegraph/issues/38934)
- Quick links will no longer be shown on the homepage or search sidebar. The `quicklink` setting is now marked as deprecated. [#40750](https://github.com/sourcegraph/sourcegraph/pull/40750)
- Quick links will no longer be shown on the homepage or search sidebar if the "Simple UI" toggle is enabled and will be removed entirely in a future release. The `quicklink` setting is now marked as deprecated. [#40750](https://github.com/sourcegraph/sourcegraph/pull/40750)
- `file:contains()` has been removed from the list of valid predicates. `file:has.content()` and `file:contains.content()` remain, both of which work the same as `file:contains()` and are valid aliases of each other.
- The single-container `sourcegraph/server` deployment no longer bundles a Jaeger instance. [#41244](https://github.com/sourcegraph/sourcegraph/pull/41244)
- The following previously-deprecated fields have been removed from the Batch Changes GraphQL API: `GitBranchChangesetDescription.headRepository`, `BatchChange.initialApplier`, `BatchChange.specCreator`, `Changeset.publicationState`, `Changeset.reconcilerState`, `Changeset.externalState`.

## 3.43.2

### Fixed

- Fixed an issue causing context cancel error dumps when updating a code host config manually. [#40857](https://github.com/sourcegraph/sourcegraph/pull/41265)
- Fixed non-critical errors stopping the repo-syncing process for Bitbucket projectKeys. [#40897](https://github.com/sourcegraph/sourcegraph/pull/40582)
- Fixed an issue marking accounts as expired when the supplied Account ID list has no entries. [#40860](https://github.com/sourcegraph/sourcegraph/pull/40860)

## 3.43.1

### Fixed

- Fixed an infinite render loop on the batch changes detail page, causing the page to become unusable. [#40857](https://github.com/sourcegraph/sourcegraph/pull/40857)
- Unable to pick the correct GitLab OAuth for user authentication and repository permissions syncing when the instance configures more than one GitLab OAuth authentication providers. [#40897](https://github.com/sourcegraph/sourcegraph/pull/40897)

## 3.43.0

### Added

- Enforce 5-changeset limit for batch changes run server-side on an unlicensed instance. [#37834](https://github.com/sourcegraph/sourcegraph/issues/37834)
- Changesets that are not associated with any batch changes can have a retention period set using the site configuration `batchChanges.changesetsRetention`. [#36188](https://github.com/sourcegraph/sourcegraph/pull/36188)
- Added experimental support for exporting traces to an OpenTelemetry collector with `"observability.tracing": { "type": "opentelemetry" }` [#37984](https://github.com/sourcegraph/sourcegraph/pull/37984)
- Added `ROCKSKIP_MIN_REPO_SIZE_MB` to automatically use [Rockskip](https://docs.sourcegraph.com/code_intelligence/explanations/rockskip) for repositories over a certain size. [#38192](https://github.com/sourcegraph/sourcegraph/pull/38192)
- `"observability.tracing": { "urlTemplate": "..." }` can now be set to configure generated trace URLs (for example those generated via `&trace=1`). [#39765](https://github.com/sourcegraph/sourcegraph/pull/39765)

### Changed

- **IMPORTANT: Search queries with patterns surrounded by** `/.../` **will now be interpreted as regular expressions.** Existing search links or code monitors are unaffected. In the rare event where older links rely on the literal meaning of `/.../`, the string will be automatically quoted it in a `content` filter, preserving the original meaning. If you happen to use an existing older link and want `/.../` to work as a regular expression, add `patterntype:standard` to the query. New queries and code monitors will interpret `/.../` as regular expressions. [#38141](https://github.com/sourcegraph/sourcegraph/pull/38141).
- The password policy has been updated and is now part of the standard featureset configurable by site-admins. [#39213](https://github.com/sourcegraph/sourcegraph/pull/39213).
- Replaced the `ALLOW_DECRYPT_MIGRATION` envvar with `ALLOW_DECRYPTION`. See [updated documentation](https://docs.sourcegraph.com/admin/config/encryption). [#39984](https://github.com/sourcegraph/sourcegraph/pull/39984)
- Compute-powered insight now supports only one series custom colors for compute series bars [40038](https://github.com/sourcegraph/sourcegraph/pull/40038)

### Fixed

- Fix issue during code insight creation where selecting `"Run your insight over all your repositories"` reset the currently selected distance between data points. [#39261](https://github.com/sourcegraph/sourcegraph/pull/39261)
- Fix issue where symbols in the side panel did not have file level permission filtering applied correctly. [#39592](https://github.com/sourcegraph/sourcegraph/pull/39592)

### Removed

- The experimental dependencies search feature has been removed, including the `repo:deps(...)` search predicate and the site configuration options `codeIntelLockfileIndexing.enabled` and `experimentalFeatures.dependenciesSearch`. [#39742](https://github.com/sourcegraph/sourcegraph/pull/39742)

## 3.42.2

### Fixed

- Fix issue with capture group insights to fail immediately if they contain invalid queries. [#39842](https://github.com/sourcegraph/sourcegraph/pull/39842)
- Fix issue during conversion of just in time code insights to start backfilling data from the current time instead of the date the insight was created. [#39923](https://github.com/sourcegraph/sourcegraph/pull/39923)

## 3.42.1

### Fixed

- Reverted git version to avoid an issue with commit-graph that could cause repository corruptions [#39537](https://github.com/sourcegraph/sourcegraph/pull/39537)
- Fixed an issue with symbols where they were not respecting sub-repository permissions [#39592](https://github.com/sourcegraph/sourcegraph/pull/39592)

## 3.42.0

### Added

- Reattached changesets now display an action and factor into the stats when previewing batch changes. [#36359](https://github.com/sourcegraph/sourcegraph/issues/36359)
- New site configuration option `"permissions.syncUsersMaxConcurrency"` to control the maximum number of user-centric permissions syncing jobs could be spawned concurrently. [#37918](https://github.com/sourcegraph/sourcegraph/issues/37918)
- Added experimental support for exporting traces to an OpenTelemetry collector with `"observability.tracing": { "type": "opentelemetry" }` [#37984](https://github.com/sourcegraph/sourcegraph/pull/37984)
- Code Insights over some repos now get 12 historic data points in addition to a current daily value and future points that align with the defined interval. [#37756](https://github.com/sourcegraph/sourcegraph/pull/37756)
- A Kustomize overlay and Helm override file to apply envoy filter for networking error caused by service mesh. [#4150](https://github.com/sourcegraph/deploy-sourcegraph/pull/4150) & [#148](https://github.com/sourcegraph/deploy-sourcegraph-helm/pull/148)
- Resource Estimator: Ability to export the estimated results as override file for Helm and Docker Compose. [#18](https://github.com/sourcegraph/resource-estimator/pull/18)
- A toggle to enable/disable a beta simplified UI has been added to the user menu. This new UI is still actively in development and any changes visible with the toggle enabled may not be stable are subject to change. [#38763](https://github.com/sourcegraph/sourcegraph/pull/38763)
- Search query inputs are now backed by the CodeMirror library instead of Monaco. Monaco can be re-enabled by setting `experimentalFeatures.editor` to `"monaco"`. [38584](https://github.com/sourcegraph/sourcegraph/pull/38584)
- Better search-based code navigation for Python using tree-sitter [#38459](https://github.com/sourcegraph/sourcegraph/pull/38459)
- Gitserver endpoint access logs can now be enabled by adding `"log": { "gitserver.accessLogs": true }` to the site config. [#38798](https://github.com/sourcegraph/sourcegraph/pull/38798)
- Code Insights supports a new type of insight—compute-powered insight, currently under the experimental feature flag: `codeInsightsCompute` [#37857](https://github.com/sourcegraph/sourcegraph/issues/37857)
- Cache execution result when mounting files in a batch spec. [sourcegraph/src-cli#795](https://github.com/sourcegraph/src-cli/pull/795)
- Batch Changes changesets open on archived repositories will now move into a [Read-Only state](https://docs.sourcegraph.com/batch_changes/references/faq#why-is-my-changeset-read-only). [#26820](https://github.com/sourcegraph/sourcegraph/issues/26820)

### Changed

- Updated minimum required veresion of `git` to 2.35.2 in `gitserver` and `server` Docker image. This addresses [a few vulnerabilities announced by GitHub](https://github.blog/2022-04-12-git-security-vulnerability-announced/).
- Search: Pasting a query with line breaks into the main search query input will now replace them with spaces instead of removing them. [#37674](https://github.com/sourcegraph/sourcegraph/pull/37674)
- Rewrite resource estimator using the latest metrics [#37869](https://github.com/sourcegraph/sourcegraph/pull/37869)
- Selecting a line multiple times in the file view will only add a single browser history entry [#38204](https://github.com/sourcegraph/sourcegraph/pull/38204)
- The panels on the homepage (recent searches, etc) are now turned off by default. They can be re-enabled by setting `experimentalFeatures.showEnterpriseHomePanels` to true. [#38431](https://github.com/sourcegraph/sourcegraph/pull/38431)
- Log sampling is now enabled by default for Sourcegraph components that use the [new internal logging library](https://github.com/sourcegraph/log)—the first 100 identical log entries per second will always be output, but thereafter only every 100th identical message will be output. It can be configured for each service using the environment variables `SRC_LOG_SAMPLING_INITIAL` and `SRC_LOG_SAMPLING_THEREAFTER`, and if `SRC_LOG_SAMPLING_INITIAL` is set to `0` or `-1` the sampling will be disabled entirely. [#38451](https://github.com/sourcegraph/sourcegraph/pull/38451)
- Deprecated `experimentalFeatures.enableGitServerCommandExecFilter`. Setting this value has no effect on the code any longer and the code to guard against unknown commands is always enabled.
- Zoekt now runs with GOGC=25 by default, helping to reduce the memory consumption of Sourcegraph. Previously it ran with GOGC=50, but we noticed a regression when we switched to go 1.18 which contained significant changes to the go garbage collector. [#38708](https://github.com/sourcegraph/sourcegraph/issues/38708)
- Hide `Publish` action when working with imported changesets. [#37882](https://github.com/sourcegraph/sourcegraph/issues/37882)

### Fixed

- Fix an issue where updating the title or body of a Bitbucket Cloud pull request opened by a batch change could fail when the pull request was not on a fork of the target repository. [#37585](https://github.com/sourcegraph/sourcegraph/issues/37585)
- A bug where some complex `repo:` regexes only returned a subset of repository results. [#37925](https://github.com/sourcegraph/sourcegraph/pull/37925)
- Fix a bug when selecting all the changesets on the Preview Batch Change Page only selected the recently loaded changesets. [#38041](https://github.com/sourcegraph/sourcegraph/pull/38041)
- Fix a bug with bad code insights chart data points links. [#38102](https://github.com/sourcegraph/sourcegraph/pull/38102)
- Code Insights: the commit indexer no longer errors when fetching commits from empty repositories and marks them as successfully indexed. [#39081](https://github.com/sourcegraph/sourcegraph/pull/38091)
- The file view does not jump to the first selected line anymore when selecting multiple lines and the first selected line was out of view. [#38175](https://github.com/sourcegraph/sourcegraph/pull/38175)
- Fixed an issue where multiple activations of the back button are required to navigate back to a previously selected line in a file [#38193](https://github.com/sourcegraph/sourcegraph/pull/38193)
- Support timestamps with numeric timezone format from Gitlab's Webhook payload [#38250](https://github.com/sourcegraph/sourcegraph/pull/38250)
- Fix regression in 3.41 where search-based Code Insights could have their queries wrongly parsed into regex patterns when containing quotes or parentheses. [#38400](https://github.com/sourcegraph/sourcegraph/pull/38400)
- Fixed regression of mismatched `From` address when render emails. [#38589](https://github.com/sourcegraph/sourcegraph/pull/38589)
- Fixed a bug with GitHub code hosts using `"repositoryQuery":{"public"}` where it wasn't respecting exclude archived. [#38839](https://github.com/sourcegraph/sourcegraph/pull/38839)
- Fixed a bug with GitHub code hosts using `repositoryQuery` with custom queries, where it could potentially stall out searching for repos. [#38839](https://github.com/sourcegraph/sourcegraph/pull/38839)
- Fixed an issue in Code Insights were duplicate points were sometimes being returned when displaying series data. [#38903](https://github.com/sourcegraph/sourcegraph/pull/38903)
- Fix issue with Bitbucket Projects repository permissions sync regarding granting pending permissions. [#39013](https://github.com/sourcegraph/sourcegraph/pull/39013)
- Fix issue with Bitbucket Projects repository permissions sync when BindID is username. [#39035](https://github.com/sourcegraph/sourcegraph/pull/39035)
- Improve keyboard navigation for batch changes server-side execution flow. [#38601](https://github.com/sourcegraph/sourcegraph/pull/38601)
- Fixed a bug with the WorkspacePreview panel glitching when it's resized. [#36470](https://github.com/sourcegraph/sourcegraph/issues/36470)
- Handle special characters in search query when creating a batch change from search. [#38772](https://github.com/sourcegraph/sourcegraph/pull/38772)
- Fixed bug when parsing numeric timezone offset in Gitlab webhook payload. [#38250](https://github.com/sourcegraph/sourcegraph/pull/38250)
- Fixed setting unrestricted status on a repository when using the explicit permissions API. If the repository had never had explicit permissions before, previously this call would fail. [#39141](https://github.com/sourcegraph/sourcegraph/pull/39141)

### Removed

- The direct DataDog trace export integration has been removed. ([#37654](https://github.com/sourcegraph/sourcegraph/pull/37654))
- Removed the deprecated git exec forwarder. [#38092](https://github.com/sourcegraph/sourcegraph/pull/38092)
- Browser and IDE extensions banners. [#38715](https://github.com/sourcegraph/sourcegraph/pull/38715)

## 3.41.1

### Fixed

- Fix issue with Bitbucket Projects repository permissions sync when wrong repo IDs were used [#38637](https://github.com/sourcegraph/sourcegraph/pull/38637)
- Fix perforce permissions interpretation for rules where there is a wildcard in the depot name [#37648](https://github.com/sourcegraph/sourcegraph/pull/37648)

### Added

- Allow directory read access for sub repo permissions [#38487](https://github.com/sourcegraph/sourcegraph/pull/38487)

### Changed

- p4-fusion version is upgraded to 1.10 [#38272](https://github.com/sourcegraph/sourcegraph/pull/38272)

## 3.41.0

### Added

- Code Insights: Added toggle display of data series in line charts
- Code Insights: Added dashboard pills for the standalone insight page [#36341](https://github.com/sourcegraph/sourcegraph/pull/36341)
- Extensions: Added site config parameter `extensions.allowOnlySourcegraphAuthoredExtensions`. When enabled only extensions authored by Sourcegraph will be able to be viewed and installed. For more information check out the [docs](https://docs.sourcegraph.com/admin/extensions##allow-only-extensions-authored-by-sourcegraph). [#35054](https://github.com/sourcegraph/sourcegraph/pull/35054)
- Batch Changes Credentials can now be manually validated. [#35948](https://github.com/sourcegraph/sourcegraph/pull/35948)
- Zoekt-indexserver has a new debug landing page, `/debug`, which now exposes information about the queue, the list of indexed repositories, and the list of assigned repositories. Admins can reach the debug landing page by selecting Instrumentation > indexed-search-indexer from the site admin view. The debug page is linked at the top. [#346](https://github.com/sourcegraph/zoekt/pull/346)
- Extensions: Added `enableExtensionsDecorationsColumnView` user setting as [experimental feature](https://docs.sourcegraph.com/admin/beta_and_experimental_features#experimental-features). When enabled decorations of the extensions supporting column decorations (currently only git-extras extension does: [sourcegraph-git-extras/pull/276](https://github.com/sourcegraph/sourcegraph-git-extras/pull/276)) will be displayed in separate columns on the blob page. [#36007](https://github.com/sourcegraph/sourcegraph/pull/36007)
- SAML authentication provider has a new site configuration `allowGroups` that allows filtering users by group membership. [#36555](https://github.com/sourcegraph/sourcegraph/pull/36555)
- A new [templating](https://docs.sourcegraph.com/batch_changes/references/batch_spec_templating) variable, `batch_change_link` has been added for more control over where the "Created by Sourcegraph batch change ..." message appears in the published changeset description. [#491](https://github.com/sourcegraph/sourcegraph/pull/35319)
- Batch specs can now mount local files in the Docker container when using [Sourcegraph CLI](https://docs.sourcegraph.com/cli). [#31790](https://github.com/sourcegraph/sourcegraph/issues/31790)
- Code Monitoring: Notifications via Slack and generic webhooks are now enabled for everyone by default as a beta feature. [#37037](https://github.com/sourcegraph/sourcegraph/pull/37037)
- Code Insights: Sort and limit filters have been added to capture group insights. This gives users more control over which series are displayed. [#34611](https://github.com/sourcegraph/sourcegraph/pull/34611)
- [Running batch changes server-side](https://docs.sourcegraph.com/batch_changes/explanations/server_side) is now in beta! In addition to using src-cli to run batch changes locally, you can now run them server-side as well. This requires installing executors. While running server-side unlocks a new and improved UI experience, you can still use src-cli just like before.
- Code Monitoring: pings for new action types [#37288](https://github.com/sourcegraph/sourcegraph/pull/37288)
- Better search-based code navigation for Java using tree-sitter [#34875](https://github.com/sourcegraph/sourcegraph/pull/34875)

### Changed

- Code Insights: Added warnings about adding `context:` and `repo:` filters in search query.
- Batch Changes: The credentials of the last applying user will now be used to sync changesets when available. If unavailable, then the previous behaviour of using a site or code host configuration credential is retained. [#33413](https://github.com/sourcegraph/sourcegraph/issues/33413)
- Gitserver: we disable automatic git-gc for invocations of git-fetch to avoid corruption of repositories by competing git-gc processes. [#36274](https://github.com/sourcegraph/sourcegraph/pull/36274)
- Commit and diff search: The hard limit of 50 repositories has been removed, and long-running searches will continue running until the timeout is hit. [#36486](https://github.com/sourcegraph/sourcegraph/pull/36486)
- The Postgres DBs `frontend` and `codeintel-db` are now given 1 hour to begin accepting connections before Kubernetes restarts the containers. [#4136](https://github.com/sourcegraph/deploy-sourcegraph/pull/4136)
- The internal git command forwarder has been deprecated and will be removed in 3.42 [#37320](https://github.com/sourcegraph/sourcegraph/pull/37320)

### Fixed

- Unable to send emails through [Google SMTP relay](https://docs.sourcegraph.com/admin/config/email#configuring-sourcegraph-to-send-email-via-google-workspace-gmail) with mysterious error "EOF". [#35943](https://github.com/sourcegraph/sourcegraph/issues/35943)
- A common source of searcher evictions on kubernetes when running large structural searches. [#34828](https://github.com/sourcegraph/sourcegraph/issues/34828)
- An issue with permissions evaluation for saved searches
- An authorization check while Redis is down will now result in an internal server error, instead of clearing a valid session from the user's cookies. [#37016](https://github.com/sourcegraph/sourcegraph/issues/37016)

### Removed

-

## 3.40.2

### Fixed

- Fix issue with OAuth login using a Github code host by reverting gologin dependency update [#36685](https://github.com/sourcegraph/sourcegraph/pull/36685)
- Fix issue with single-container docker image where codeinsights-db was being incorrectly created [#36678](https://github.com/sourcegraph/sourcegraph/pull/36678)

## 3.40.1

### Fixed

- Support expiring OAuth tokens for GitLab which became the default in version 15.0. [#36003](https://github.com/sourcegraph/sourcegraph/pull/36003)
- Fix external service resolver erroring when webhooks not supported. [#35932](https://github.com/sourcegraph/sourcegraph/pull/35932)

## 3.40.0

### Added

- Code Insights: Added fuzzy search filter for dashboard select drop down
- Code Insights: You can share code insights through a shareable link. [#34965](https://github.com/sourcegraph/sourcegraph/pull/34965)
- Search: `path:` is now a valid filter. It is an alias for the existing `file:` filter. [#34947](https://github.com/sourcegraph/sourcegraph/pull/34947)
- Search: `-language` is a valid filter, but the web app displays it as invalid. The web app is fixed to reflect validity. [#34949](https://github.com/sourcegraph/sourcegraph/pull/34949)
- Search-based code intelligence now recognizes local variables in Python, Java, JavaScript, TypeScript, C/C++, C#, Go, and Ruby. [#33689](https://github.com/sourcegraph/sourcegraph/pull/33689)
- GraphQL API: Added support for async external service deletion. This should be used to delete an external service which cannot be deleted within 75 seconds timeout due to a large number of repos. Usage: add `async` boolean field to `deleteExternalService` mutation. Example: `mutation deleteExternalService(externalService: "id", async: true) { alwaysNil }`
- [search.largeFiles](https://docs.sourcegraph.com/admin/config/site_config#search-largeFiles) now supports recursive globs. For example, it is now possible to specify a pattern like `**/*.lock` to match a lock file anywhere in a repository. [#35411](https://github.com/sourcegraph/sourcegraph/pull/35411)
- Permissions: The `setRepositoryPermissionsUnrestricted` mutation was added, which allows explicitly marking a repo as available to all Sourcegraph users. [#35378](https://github.com/sourcegraph/sourcegraph/pull/35378)
- The `repo:deps(...)` predicate can now search through the [Python dependencies of your repositories](https://docs.sourcegraph.com/code_search/how-to/dependencies_search). [#32659](https://github.com/sourcegraph/sourcegraph/issues/32659)
- Batch Changes are now supported on [Bitbucket Cloud](https://bitbucket.org/). [#24199](https://github.com/sourcegraph/sourcegraph/issues/24199)
- Pings for server-side batch changes [#34308](https://github.com/sourcegraph/sourcegraph/pull/34308)
- Indexed search will detect when it is misconfigured and has multiple replicas writing to the same directory. [#35513](https://github.com/sourcegraph/sourcegraph/pull/35513)
- A new token creation callback feature that sends a token back to a trusted program automatically after the user has signed in [#35339](https://github.com/sourcegraph/sourcegraph/pull/35339)
- The Grafana dashboard now has a global container resource usage view to help site-admin quickly identify potential scaling issues. [#34808](https://github.com/sourcegraph/sourcegraph/pull/34808)

### Changed

- Sourcegraph's docker images are now based on Alpine Linux 3.14. [#34508](https://github.com/sourcegraph/sourcegraph/pull/34508)
- Sourcegraph is now built with Go 1.18. [#34899](https://github.com/sourcegraph/sourcegraph/pull/34899)
- Capture group Code Insights now use the Compute streaming endpoint. [#34905](https://github.com/sourcegraph/sourcegraph/pull/34905)
- Code Insights will now automatically generate queries with a default value of `fork:no` and `archived:no` if these fields are not specified by the user. This removes the need to manually add these fields to have consistent behavior from historical to non-historical results. [#30204](https://github.com/sourcegraph/sourcegraph/issues/30204)
- Search Code Insights now use the Search streaming endpoint. [#35286](https://github.com/sourcegraph/sourcegraph/pull/35286)
- Deployment: Nginx ingress controller updated to v1.2.0

### Fixed

- Code Insights: Fixed line chart data series hover effect. Now the active line will be rendered on top of the others.
- Code Insights: Fixed incorrect Line Chart size calculation in FireFox
- Unverified primary emails no longer breaks the Emails-page for users and Users-page for Site Admin. [#34312](https://github.com/sourcegraph/sourcegraph/pull/34312)
- Button to download raw file in blob page is now working correctly. [#34558](https://github.com/sourcegraph/sourcegraph/pull/34558)
- Searches containing `or` expressions are now optimized to evaluate natively on the backends that support it ([#34382](https://github.com/sourcegraph/sourcegraph/pull/34382)), and both commit and diff search have been updated to run optimized `and`, `or`, and `not` queries. [#34595](https://github.com/sourcegraph/sourcegraph/pull/34595)
- Carets in textareas in Firefox are now visible. [#34888](https://github.com/sourcegraph/sourcegraph/pull/34888)
- Changesets to GitHub code hosts could fail with a confusing, non actionable error message. [#35048](https://github.com/sourcegraph/sourcegraph/pull/35048)
- An issue causing search expressions to not work in conjunction with `type:symbol`. [#35126](https://github.com/sourcegraph/sourcegraph/pull/35126)
- A non-descriptive error message that would be returned when using `on.repository` if it is not a valid repository path [#35023](https://github.com/sourcegraph/sourcegraph/pull/35023)
- Reduced database load when viewing or previewing a batch change. [#35501](https://github.com/sourcegraph/sourcegraph/pull/35501)
- Fixed a bug where Capture Group Code Insights generated just in time only returned data for the latest repository in the list. [#35624](https://github.com/sourcegraph/sourcegraph/pull/35624)

### Removed

- The experimental API Docs feature released on our Cloud instance since 3.30.0 has been removed from the product entirely. This product functionality is being superseded by [doctree](https://github.com/sourcegraph/doctree). [#34798](https://github.com/sourcegraph/sourcegraph/pull/34798)

## 3.39.1

### Fixed

- Code Insights: Fixed bug that caused line rendering issues when series data is returned out of order by date.
- Code Insights: Fixed bug that caused before and after parameters to be switched when clicking in to the diff view from an insight.
- Fixed an issue with notebooks that caused the cursor to behave erratically in markdown blocks. [#34227](https://github.com/sourcegraph/sourcegraph/pull/34227)
- Batch Changes on docker compose installations were failing due to a missing environment variable [#813](https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/813).

## 3.39.0

### Added

- Added support for LSIF upload authentication against GitLab.com on Sourcegraph Cloud. [#33254](https://github.com/sourcegraph/sourcegraph/pull/33254)
- Add "getting started/quick start checklist for authenticated users" [#32882](https://github.com/sourcegraph/sourcegraph/pull/32882)
- A redesigned repository page is now available under the `new-repo-page` feature flag. [#33319](https://github.com/sourcegraph/sourcegraph/pull/33319)
- Pings now include notebooks usage metrics. [#30087](https://github.com/sourcegraph/sourcegraph/issues/30087)
- Notebooks are now enabled by default. [#33706](https://github.com/sourcegraph/sourcegraph/pull/33706)
- The Code Insights GraphQL API now accepts Search Contexts as a filter and will extract the expressions embedded the `repo` and `-repo` search query fields from the contexts to apply them as filters on the insight. [#33866](https://github.com/sourcegraph/sourcegraph/pull/33866)
- The Code Insights commit indexer can now index commits in smaller batches. Set the number of days per batch in the site setting `insights.commit.indexer.windowDuration`. A value of 0 (default) will disable batching. [#33666](https://github.com/sourcegraph/sourcegraph/pull/33666)
- Support account lockout after consecutive failed sign-in attempts for builtin authentication provider (i.e. username and password), new config options are added to the site configuration under `"auth.lockout"` to customize the threshold, length of lockout and consecutive periods. [#33999](https://github.com/sourcegraph/sourcegraph/pull/33999)
- pgsql-exporter for Code Insights has been added to docker-compose and Kubernetes deployments to gather database-level metrics. [#780](https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/780), [#4111](https://github.com/sourcegraph/deploy-sourcegraph/pull/4111)
- `repo:dependencies(...)` predicate can now search through the [Go dependencies of your repositories](https://docs.sourcegraph.com/code_search/how-to/dependencies_search). [#32658](https://github.com/sourcegraph/sourcegraph/issues/32658)
- Added a site config value `defaultRateLimit` to optionally configure a global default rate limit for external services.

### Changed

- Code Insights: Replaced native window confirmation dialog with branded modal. [#33637](https://github.com/sourcegraph/sourcegraph/pull/33637)
- Code Insights: Series data is now sorted by semantic version then alphabetically.
- Code Insights: Added locked insights overlays for frozen insights while in limited access mode. Restricted insight editing save change button for frozen insights. [#33062](https://github.com/sourcegraph/sourcegraph/pull/33062)
- Code Insights: A global dashboard will now be automatically created while in limited access mode to provide consistent visibility for unlocked insights. This dashboard cannot be deleted or modified while in limited access mode. [#32992](https://github.com/sourcegraph/sourcegraph/pull/32992)
- Update "getting started checklist for visitors" to a new design [TODO:]
- Update "getting started/quick start checklist for visitors" to a new design [#32882](https://github.com/sourcegraph/sourcegraph/pull/32882)
- Code Insights: Capture group values are now restricted to 100 characters. [#32828](https://github.com/sourcegraph/sourcegraph/pull/32828)
- Repositories for which gitserver's janitor job "sg maintenance" fails will eventually be re-cloned if "DisableAutoGitUpdates" is set to false (default) in site configuration. [#33432](https://github.com/sourcegraph/sourcegraph/pull/33432)
- The Code Insights database is now based on Postgres 12, removing the dependency on TimescaleDB. [#32697](https://github.com/sourcegraph/sourcegraph/pull/32697)

### Fixed

- Fixed create insight button being erroneously disabled.
- Fixed an issue where a `Warning: Sourcegraph cannot send emails!` banner would appear for all users instead of just site admins (introduced in v3.38).
- Fixed reading search pattern type from settings [#32989](https://github.com/sourcegraph/sourcegraph/issues/32989)
- Display a tooltip and truncate the title of a search result when content overflows [#32904](https://github.com/sourcegraph/sourcegraph/pull/32904)
- Search patterns containing `and` and `not` expressions are now optimized to evaluate natively on the Zoekt backend for indexed code content and symbol search wherever possible. These kinds of queries are now typically an order of magnitude faster. Previous cases where no results were returned for expensive search expressions should now work and return results quickly. [#33308](https://github.com/sourcegraph/sourcegraph/pull/33308)
- Fail to log extension activation event will no longer block extension from activating [#33300][https://github.com/sourcegraph/sourcegraph/pull/33300]
- Fixed out-ouf-memory events for gitserver's janitor job "sg maintenance". [#33353](https://github.com/sourcegraph/sourcegraph/issues/33353)
- Setting the publication state for changesets when previewing a batch spec now works correctly if all changesets are selected and there is more than one page of changesets. [#33619](https://github.com/sourcegraph/sourcegraph/issues/33619)

### Removed

-

## 3.38.1

### Fixed

- An issue introduced in 3.38 that caused alerts to not be delivered [#33398](https://github.com/sourcegraph/sourcegraph/pull/33398)

## 3.38.0

### Added

- Added new "Getting started onboarding tour" for not authenticated users on Sourcegraph.com instead of "Search onboarding tour" [#32263](https://github.com/sourcegraph/sourcegraph/pull/32263)
- Pings now include code host integration usage metrics [#31379](https://github.com/sourcegraph/sourcegraph/pull/31379)
- Added `PRECISE_CODE_INTEL_UPLOAD_AWS_USE_EC2_ROLE_CREDENTIALS` environment variable to enable EC2 metadata API authentication to an external S3 bucket storing precise code intelligence uploads. [#31820](https://github.com/sourcegraph/sourcegraph/pull/31820)
- LSIF upload pages now include a section listing the reasons and retention policies resulting in an upload being retained and not expired. [#30864](https://github.com/sourcegraph/sourcegraph/pull/30864)
- Timestamps in the history panel can now be formatted as absolute timestamps by using user setting `history.preferAbsoluteTimestamps`
- Timestamps in the history panel can now be formatted as absolute timestamps by using user setting `history.preferAbsoluteTimestamps` [#31837](https://github.com/sourcegraph/sourcegraph/pull/31837)
- Notebooks from private enterprise instances can now be embedded in external sites by enabling the `enable-embed-route` feature flag. [#31628](https://github.com/sourcegraph/sourcegraph/issues/31628)
- Pings now include IDE extensions usage metrics [#32000](https://github.com/sourcegraph/sourcegraph/pull/32000)
- New EventSource type: `IDEEXTENSION` for IDE extensions-related events [#32000](https://github.com/sourcegraph/sourcegraph/pull/32000)
- Code Monitoring now has a Logs tab enabled as a [beta feature](https://docs.sourcegraph.com/admin/beta_and_experimental_features). This lets you see recent runs of your code monitors and determine if any notifications were sent or if there were any errors during the run. [#32292](https://github.com/sourcegraph/sourcegraph/pull/32292)
- Code Monitoring creation and editing now supports syntax highlighting and autocomplete on the search box. [#32536](https://github.com/sourcegraph/sourcegraph/pull/32536)
- New `repo:dependencies(...)` predicate allows you to [search through the dependencies of your repositories](https://docs.sourcegraph.com/code_search/how-to/dependencies_search). This feature is currently in beta and only npm package repositories are supported with dependencies from `package-lock.json` and `yarn.lock` files. [#32405](https://github.com/sourcegraph/sourcegraph/issues/32405)
- Site config has a new _experimental_ feature called `gitServerPinnedRepos` that allows admins to pin specific repositories to particular gitserver instances. [#32831](https://github.com/sourcegraph/sourcegraph/pull/32831).
- Added [Rockskip](https://docs.sourcegraph.com/code_intelligence/explanations/rockskip), a scalable symbol service backend for a fast symbol sidebar and search-based code intelligence on monorepos.
- Code monitor email notifications can now optionally include the content of new search results. This is disabled by default but can be enabled by editing the code monitor's email action and toggling on "Include search results in sent message". [#32097](https://github.com/sourcegraph/sourcegraph/pull/32097)

### Changed

- Searching for the pattern `//` with regular expression search is now interpreted literally and will search for `//`. Previously, the `//` pattern was interpreted as our regular expression syntax `/<regexp>/` which would in turn be intrpreted as the empty string. Since searching for an empty string offers little practically utility, we now instead interpret `//` to search for its literal meaning in regular expression search. [#31520](https://github.com/sourcegraph/sourcegraph/pull/31520)
- Timestamps in the webapp will now display local time on hover instead of UTC time [#31672](https://github.com/sourcegraph/sourcegraph/pull/31672)
- Updated Postgres version from 12.6 to 12.7 [#31933](https://github.com/sourcegraph/sourcegraph/pull/31933)
- Code Insights will now periodically clean up data series that are not in use. There is a 1 hour grace period where the series can be reattached to a view, after which all of the time series data and metadata will be deleted. [#32094](https://github.com/sourcegraph/sourcegraph/pull/32094)
- Code Insights critical telemetry total count now only includes insights that are not frozen (limited by trial mode restrictions). [#32529](https://github.com/sourcegraph/sourcegraph/pull/32529)
- The Phabricator integration with Gitolite code hosts has been deprecated, the fields have been kept to not break existing systems, but the integration does not work anymore
- The SSH library used to push Batch Change branches to code hosts has been updated to prevent issues pushing to github.com or GitHub Enterprise releases after March 15, 2022. [#32641](https://github.com/sourcegraph/sourcegraph/issues/32641)
- Bumped the minimum supported version of Docker Compose from `1.22.0` to `1.29.0`. [#32631](https://github.com/sourcegraph/sourcegraph/pull/32631)
- [Code host API rate limit configuration](https://docs.sourcegraph.com/admin/repo/update_frequency#code-host-api-rate-limiting) no longer based on code host URLs but only takes effect on each individual external services. To enforce API rate limit, please add configuration to all external services that are intended to be rate limited. [#32768](https://github.com/sourcegraph/sourcegraph/pull/32768)

### Fixed

- Viewing or previewing a batch change is now more resilient when transient network or server errors occur. [#29859](https://github.com/sourcegraph/sourcegraph/issues/29859)
- Search: `select:file` and `select:file.directory` now properly deduplicates results. [#32469](https://github.com/sourcegraph/sourcegraph/pull/32469)
- Security: Patch container images against CVE 2022-0778 [#32679](https://github.com/sourcegraph/sourcegraph/issues/32679)
- When closing a batch change, draft changesets that will be closed are now also shown. [#32481](https://github.com/sourcegraph/sourcegraph/pull/32481)

### Removed

- The deprecated GraphQL field `SearchResults.resultCount` has been removed in favor of its replacement, `matchCount`. [#31573](https://github.com/sourcegraph/sourcegraph/pull/31573)
- The deprecated site-config field `UseJaeger` has been removed. Use `"observability.tracing": { "sampling": "all" }` instead [#31294](https://github.com/sourcegraph/sourcegraph/pull/31294/commits/6793220d6cf1200535a2610d79d2dd9e18c67dca)

## 3.37.0

### Added

- Code in search results is now selectable (e.g. for copying). Just clicking on the code continues to open the corresponding file as it did before. [#30033](https://github.com/sourcegraph/sourcegraph/pull/30033)
- Search Notebooks now support importing and exporting Markdown-formatted files. [#28586](https://github.com/sourcegraph/sourcegraph/issues/28586)
- Added standalone migrator service that can be used to run database migrations independently of an upgrade. For more detail see the [standalone migrator docs](https://docs.sourcegraph.com/admin/how-to/manual_database_migrations) and the [docker-compose](https://docs.sourcegraph.com/admin/install/docker-compose/operations#database-migrations) or [kubernetes](https://docs.sourcegraph.com/admin/install/kubernetes/update#database-migrations) upgrade docs.

### Changed

- Syntax highlighting for JSON now uses a distinct color for strings in object key positions. [#30105](https://github.com/sourcegraph/sourcegraph/pull/30105)
- GraphQL API: The order of events returned by `MonitorTriggerEventConnection` has been reversed so newer events are returned first. The `after` parameter has been modified accordingly to return events older the one specified, to allow for pagination. [31219](https://github.com/sourcegraph/sourcegraph/pull/31219)
- [Query based search contexts](https://docs.sourcegraph.com/code_search/how-to/search_contexts#beta-query-based-search-contexts) are now enabled by default as a [beta feature](https://docs.sourcegraph.com/admin/beta_and_experimental_features). [#30888](https://github.com/sourcegraph/sourcegraph/pull/30888)
- The symbols sidebar loads much faster on old commits (after processing it) when scoped to a subdirectory in a big repository. [#31300](https://github.com/sourcegraph/sourcegraph/pull/31300)

### Fixed

- Links generated by editor endpoint will render image preview correctly. [#30767](https://github.com/sourcegraph/sourcegraph/pull/30767)
- Fixed a race condition in the precise code intel upload expirer process that prematurely expired new uploads. [#30546](https://github.com/sourcegraph/sourcegraph/pull/30546)
- Pushing changesets from Batch Changes to code hosts with self-signed TLS certificates has been fixed. [#31010](https://github.com/sourcegraph/sourcegraph/issues/31010)
- Fixed LSIF uploads not being expired according to retention policies when the repository contained tags and branches with the same name but pointing to different commits. [#31108](https://github.com/sourcegraph/sourcegraph/pull/31108)
- Service discovery for the symbols service can transition from no endpoints to endpoints. Previously we always returned an error after the first empty state. [#31225](https://github.com/sourcegraph/sourcegraph/pull/31225)
- Fixed performance issue in LSIF upload processing, reducing the latency between uploading an LSIF index and accessing precise code intel in the UI. ([#30978](https://github.com/sourcegraph/sourcegraph/pull/30978), [#31143](https://github.com/sourcegraph/sourcegraph/pull/31143))
- Fixed symbols not appearing when no files changed between commits. [#31295](https://github.com/sourcegraph/sourcegraph/pull/31295)
- Fixed symbols not appearing when too many files changed between commits. [#31110](https://github.com/sourcegraph/sourcegraph/pull/31110)
- Fixed runaway disk usage in the `symbols` service. [#30647](https://github.com/sourcegraph/sourcegraph/pull/30647)

### Removed

- Removed `experimentalFeature.showCodeMonitoringTestEmailButton`. Test emails can still be sent by editing the code monitor and expanding the "Send email notification" section. [#29953](https://github.com/sourcegraph/sourcegraph/pull/29953)

## 3.36.3

### Fixed

- Fix Code Monitor permissions. For more detail see our [security advisory](https://github.com/sourcegraph/sourcegraph/security/advisories/GHSA-xqv2-x6f2-w3pf) [#30547](https://github.com/sourcegraph/sourcegraph/pull/30547)

## 3.36.2

### Removed

- The TOS consent screen which would appear for all users upon signing into Sourcegraph. We had some internal miscommunication on this onboarding flow and it didn’t turn out the way we intended, this effectively reverts that change. ![#30192](https://github.com/sourcegraph/sourcegraph/issues/30192)

## 3.36.1

### Fixed

- Fix broken 'src lsif upload' inside executor due to basic auth removal. [#30023](https://github.com/sourcegraph/sourcegraph/pull/30023)

## 3.36.0

### Added

- Search contexts can now be defined with a restricted search query as an alternative to a specific list of repositories and revisions. This feature is _beta_ and may change in the following releases. Allowed filters: `repo`, `rev`, `file`, `lang`, `case`, `fork`, `visibility`. `OR`, `AND` expressions are also allowed. To enable this feature to all users, set `experimentalFeatures.searchContextsQuery` to true in global settings. You'll then see a "Create context" button from the search results page and a "Query" input field in the search contexts form. If you want revisions specified in these query based search contexts to be indexed, set `experimentalFeatures.search.index.query.contexts` to true in site configuration. [#29327](https://github.com/sourcegraph/sourcegraph/pull/29327)
- More explicit Terms of Service and Privacy Policy consent has been added to Sourcegraph Server. [#28716](https://github.com/sourcegraph/sourcegraph/issues/28716)
- Batch changes will be created on forks of the upstream repository if the new `batchChanges.enforceForks` site setting is enabled. [#17879](https://github.com/sourcegraph/sourcegraph/issues/17879)
- Symbolic links are now searchable. Previously it was possible to navigate to symbolic links in the repository tree view, however the symbolic links were ignored during searches. [#29567](https://github.com/sourcegraph/sourcegraph/pull/29567), [#237](https://github.com/sourcegraph/zoekt/pull/237)
- Maximum number of references/definitions shown in panel can be adjusted in settings with `codeIntelligence.maxPanelResults`. If not set, a hardcoded limit of 500 was used. [#29629](https://github.com/sourcegraph/sourcegraph/29629)
- Search notebooks are now fully persistable. You can create notebooks through the WYSIWYG editor and share them via a unique URL. We support two visibility modes: private (only the creator can view the notebook) and public (everyone can view the notebook). This feature is _beta_ and may change in the following releases. [#27384](https://github.com/sourcegraph/sourcegraph/issues/27384)
- Code Insights that are run over all repositories now have data points with links that lead to the search page. [#29587](https://github.com/sourcegraph/sourcegraph/pull/29587)
- Code Insights creation UI query field now supports different syntax highlight modes based on `patterntype` filter. [#29733](https://github.com/sourcegraph/sourcegraph/pull/29733)
- Code Insights creation UI query field now has live-preview button that leads to the search page with predefined query value. [#29698](https://github.com/sourcegraph/sourcegraph/pull/29698)
- Code Insights creation UI detect and track patterns can now search across all repositories. [#29906](https://github.com/sourcegraph/sourcegraph/pull/29906)
- Pings now contain aggregated CTA metrics. [#29966](https://github.com/sourcegraph/sourcegraph/pull/29966)
- Pings now contain aggregated CTA metrics. [#29966](https://github.com/sourcegraph/sourcegraph/pull/29966) and [#31389](https://github.com/sourcegraph/sourcegraph/pull/31389)

### Changed

- Sourcegraph's API (streaming search, GraphQL, etc.) may now be used from any domain when using an access token for authentication, or with no authentication in the case of Sourcegraph.com. [#28775](https://github.com/sourcegraph/sourcegraph/pull/28775)
- The endpoint `/search/stream` will be retired in favor of `/.api/search/stream`. This requires no action unless you have developed custom code against `/search/stream`. We will support both endpoints for a short period of time before removing `/search/stream`. Please refer to the [documentation](https://docs.sourcegraph.com/api/stream_api) for more information.
- When displaying the content of symbolic links in the repository tree view, we will show the relative path to the link's target instead of the target's content. This behavior is consistent with how we display symbolic links in search results. [#29687](https://github.com/sourcegraph/sourcegraph/pull/29687)
- A new janitor job, "sg maintenance" was added to gitserver. The new job replaces "garbage collect" with the goal to optimize the performance of git operations for large repositories. You can choose to enable "garbage collect" again by setting the environment variables "SRC_ENABLE_GC_AUTO" to "true" and "SRC_ENABLE_SG_MAINTENANCE" to "false" for gitserver. Note that you must not enable both options at the same time. [#28224](https://github.com/sourcegraph/sourcegraph/pull/28224).
- Search results across repositories are now ordered by repository rank by default. By default the rank is the number of stars a repository has. An administrator can inflate the rank of a repository via `experimentalFeatures.ranking.repoScores`. If you notice increased latency in results, you can disable this feature by setting `experimentalFeatures.ranking.maxReorderQueueSize` to 0. [#29856](https://github.com/sourcegraph/sourcegraph/pull/29856)
- Search results within the same file are now ordered by relevance instead of line number. To order by line number, update the setting `experimentalFeatures.clientSearchResultRanking: "by-line-number"`. [#29046](https://github.com/sourcegraph/sourcegraph/pull/29046)
- Bumped the symbols processing timeout from 20 minutes to 2 hours and made it configurable. [#29891](https://github.com/sourcegraph/sourcegraph/pull/29891)

### Fixed

- Issue preventing searches from completing when certain patterns contain `@`. [#29489](https://github.com/sourcegraph/sourcegraph/pull/29489)
- The grafana dashboard for "successful search request duration" reports the time for streaming search which is used by the browser. Previously it reported the GraphQL time which the browser no longer uses. [#29625](https://github.com/sourcegraph/sourcegraph/pull/29625)
- A regression introduced in 3.35 causing Code Insights that are run over all repositories to not query against repositories that have permissions enabled. (Restricted repositories are and remain filtered based on user permissions when a user views a chart, not at query time.) This may cause global Insights to undercount for data points generated after upgrading to 3.35 and before upgrading to 3.36. [](https://github.com/sourcegraph/sourcegraph/pull/29725)
- Renaming repositories now removes the old indexes on Zoekt's disks. This did not affect search results, only wasted disk space. This was a regression introduced in Sourcegraph 3.33. [#29685](https://github.com/sourcegraph/sourcegraph/issues/29685)

### Removed

- Removed unused backend service from Kubernetes deployments. [#4050](https://github.com/sourcegraph/deploy-sourcegraph/pull/4050)

## 3.35.2

### Fixed

- Fix Code Monitor permissions. For more detail see our [security advisory](https://github.com/sourcegraph/sourcegraph/security/advisories/GHSA-xqv2-x6f2-w3pf) [#30547](https://github.com/sourcegraph/sourcegraph/pull/30547)

## 3.35.1

**⚠️ Due to issues related to Code Insights in the 3.35.0 release, users are advised to upgrade directly to 3.35.1.**

### Fixed

- Skipped migrations caused existing Code Insights to not appear. [#29395](https://github.com/sourcegraph/sourcegraph/pull/29395)
- Enterprise-only out-of-band migrations failed to execute due to missing enterprise configuration flag. [#29426](https://github.com/sourcegraph/sourcegraph/pull/29426)

## 3.35.0

**⚠️ Due to issues related to Code Insights on this release, users are advised to upgrade directly to 3.35.1.**

### Added

- Individual batch changes can publish multiple changesets to the same repository by specifying multiple target branches using the [`on.branches`](https://docs.sourcegraph.com/batch_changes/references/batch_spec_yaml_reference#on-repository) attribute. [#25228](https://github.com/sourcegraph/sourcegraph/issues/25228)
- Low resource overlay added. NOTE: this is designed for internal-use only. Customers can use the `minikube` overlay to achieve similar results.[#4012](https://github.com/sourcegraph/deploy-sourcegraph/pull/4012)
- Code Insights has a new insight `Detect and Track` which will generate unique time series from the matches of a pattern specified as a regular expression capture group. This is currently limited to insights scoped to specific repositories. [docs](https://docs.sourcegraph.com/code_insights/explanations/automatically_generated_data_series)
- Code Insights is persisted entirely in the `codeinsights-db` database. A migration will automatically be performed to move any defined insights and dashboards from your user, org, or global settings files.
- The GraphQL API for Code Insights has entered beta. [docs](https://docs.sourcegraph.com/code_insights/references/code_insights_graphql_api)
- The `SRC_GIT_SERVICE_MAX_EGRESS_BYTES_PER_SECOND` environment variable to control the egress throughput of gitserver's git service (e.g. used by zoekt-index-server to clone repos to index). Set to -1 for no limit. [#29197](https://github.com/sourcegraph/sourcegraph/pull/29197)
- Search suggestions via the GraphQL API were deprecated last release and are now no longer available. Suggestions now work only with the search streaming API. [#29283](https://github.com/sourcegraph/sourcegraph/pull/29283)
- Clicking on a token will now jump to its definition. [#28520](https://github.com/sourcegraph/sourcegraph/pull/28520)

### Changed

- The `ALLOW_DECRYPT_MIGRATION` environment variable is now read by the `worker` service, not the `frontend` service as in previous versions.
- External services will stop syncing if they exceed the user / site level limit for total number of repositories added. It will only continue syncing if the extra repositories are removed or the corresponding limit is increased, otherwise it will stop syncing for the very first repository each time the syncer attempts to sync the external service again. [#28674](https://github.com/sourcegraph/sourcegraph/pull/28674)
- Sourcegraph services now listen to SIGTERM signals. This allows smoother rollouts in kubernetes deployments. [#27958](https://github.com/sourcegraph/sourcegraph/pull/27958)
- The sourcegraph-frontend ingress now uses the networking.k8s.io/v1 api. This adds support for k8s v1.22 and later, and deprecates support for versions older than v1.18.x [#4029](https://github.com/sourcegraph/deploy-sourcegraph/pull/4029)
- Non-bare repositories found on gitserver will be removed by a janitor job. [#28895](https://github.com/sourcegraph/sourcegraph/pull/28895)
- The search bar is no longer auto-focused when navigating between files. This change means that the keyboard shortcut Cmd+LeftArrow (or Ctrl-LeftArrow) now goes back to the browser's previous page instead of moving the cursor position to the first position of the search bar. [#28943](https://github.com/sourcegraph/sourcegraph/pull/28943)
- Code Insights series over all repositories can now be edited
- Code Insights series over all repositories now support a custom time interval and will calculate with 12 points starting at the moment the series is created and working backwards.
- Minio service upgraded to RELEASE.2021-12-10T23-03-39Z. [#29188](https://github.com/sourcegraph/sourcegraph/pull/29188)
- Code insights creation UI form query field now supports suggestions and syntax highlighting. [#28130](https://github.com/sourcegraph/sourcegraph/pull/28130)
- Using `select:repo` in search queries will now stream results incrementally, greatly improving speed and reducing time-to-first-result. [#28920](https://github.com/sourcegraph/sourcegraph/pull/28920)
- The fuzzy file finder is now enabled by default and can be activated with the shortcut `Cmd+K` on macOS and `Ctrl+K` on Linux/Windows. Change the user setting `experimentalFeatures.fuzzyFinder` to `false` to disable this feature. [#29010](https://github.com/sourcegraph/sourcegraph/pull/29010)
- Search-based code intelligence and the symbol sidebar are much faster now that the symbols service incrementally processes files that changed. [#27932](https://github.com/sourcegraph/sourcegraph/pull/27932)

### Fixed

- Moving a changeset from draft state into published state was broken on GitLab code hosts. [#28239](https://github.com/sourcegraph/sourcegraph/pull/28239)
- The shortcuts for toggling the History Panel and Line Wrap were not working on Mac. [#28574](https://github.com/sourcegraph/sourcegraph/pull/28574)
- Suppresses docker-on-mac warning for Kubernetes, Docker Compose, and Pure Docker deployments. [#28405](https://github.com/sourcegraph/sourcegraph/pull/28821)
- Fixed an issue where certain regexp syntax for repository searches caused the entire search, including non-repository searches, to fail with a parse error (issue affects only version 3.34). [#28826](https://github.com/sourcegraph/sourcegraph/pull/28826)
- Modifying changesets on Bitbucket Server could previously fail if the local copy in Batch Changes was out of date. That has been fixed by retrying the operations in case of a 409 response. [#29100](https://github.com/sourcegraph/sourcegraph/pull/29100)

### Removed

- Settings files (user, org, global) as a persistence mechanism for Code Insights are now deprecated.
- Query-runner deployment has been removed. You can safely remove the `query-runner` service from your installation.

## 3.34.2

### Fixed

- A bug introduced in 3.34 and 3.34.1 that resulted in certain repositories being missed in search results. [#28624](https://github.com/sourcegraph/sourcegraph/pull/28624)

## 3.34.1

### Fixed

- Fixed Redis alerting for docker-compose deployments [#28099](https://github.com/sourcegraph/sourcegraph/issues/28099)

## 3.34.0

### Added

- Added documentation for merging site-config files. Available since 3.32 [#21220](https://github.com/sourcegraph/sourcegraph/issues/21220)
- Added site config variable `cloneProgressLog` to optionally enable logging of clone progress to temporary files for debugging. Disabled by default. [#26568](https://github.com/sourcegraph/sourcegraph/pull/26568)
- GNU's `wget` has been added to all `sourcegraph/*` Docker images that use `sourcegraph/alpine` as its base [#26823](https://github.com/sourcegraph/sourcegraph/pull/26823)
- Added the "no results page", a help page shown if a search doesn't return any results [#26154](https://github.com/sourcegraph/sourcegraph/pull/26154)
- Added monitoring page for Redis databases [#26967](https://github.com/sourcegraph/sourcegraph/issues/26967)
- The search indexer only polls repositories that have been marked as changed. This reduces a large source of load in installations with a large number of repositories. If you notice index staleness, you can try disabling by setting the environment variable `SRC_SEARCH_INDEXER_EFFICIENT_POLLING_DISABLED` on `sourcegraph-frontend`. [#27058](https://github.com/sourcegraph/sourcegraph/issues/27058)
- Pings include instance wide total counts of Code Insights grouped by presentation type, series type, and presentation-series type. [#27602](https://github.com/sourcegraph/sourcegraph/pull/27602)
- Added logging of incoming Batch Changes webhooks, which can be viewed by site admins. By default, sites without encryption will log webhooks for three days, while sites with encryption will not log webhooks without explicit configuration. [See the documentation for more details](https://docs.sourcegraph.com/admin/config/batch_changes#incoming-webhooks). [#26669](https://github.com/sourcegraph/sourcegraph/issues/26669)
- Added support for finding implementations of interfaces and methods. [#24854](https://github.com/sourcegraph/sourcegraph/pull/24854)

### Changed

- Removed liveness probes from Kubernetes Prometheus deployment [#2970](https://github.com/sourcegraph/deploy-sourcegraph/pull/2970)
- Batch Changes now requests the `workflow` scope on GitHub personal access tokens to allow batch changes to write to the `.github` directory in repositories. If you have already configured a GitHub PAT for use with Batch Changes, we suggest adding the scope to the others already granted. [#26606](https://github.com/sourcegraph/sourcegraph/issues/26606)
- Sourcegraph's Prometheus and Alertmanager dependency has been upgraded to v2.31.1 and v0.23.0 respectively. [#27336](https://github.com/sourcegraph/sourcegraph/pull/27336)
- The search UI's repositories count as well as the GraphQL API's `search().repositories` and `search().repositoriesCount` have changed semantics from the set of searchable repositories to the set of repositories with matches. In a future release, we'll introduce separate fields for the set of searchable repositories backed by a [scalable implementation](https://github.com/sourcegraph/sourcegraph/issues/27274). [#26995](https://github.com/sourcegraph/sourcegraph/issues/26995)

### Fixed

- An issue that causes the server to panic when performing a structural search via the GQL API for a query that also
  matches missing repos (affected versions 3.33.0 and 3.32.0)
  . [#26630](https://github.com/sourcegraph/sourcegraph/pull/26630)
- Improve detection for Docker running in non-linux
  environments. [#23477](https://github.com/sourcegraph/sourcegraph/issues/23477)
- Fixed the cache size calculation used for Kubernetes deployments. Previously, the calculated value was too high and would exceed the ephemeral storage request limit. #[26283](https://github.com/sourcegraph/sourcegraph/issues/26283)
- Fixed a regression that was introduced in 3.27 and broke SSH-based authentication for managing Batch Changes changesets on code hosts. SSH keys generated by Sourcegraph were not used for authentication and authenticating with the code host would fail if no SSH key with write-access had been added to `gitserver`. [#27491](https://github.com/sourcegraph/sourcegraph/pull/27491)
- Private repositories matching `-repo:` expressions are now excluded. This was a regression introduced in 3.33.0. [#27044](https://github.com/sourcegraph/sourcegraph/issues/27044)

### Removed

- All version contexts functionality (deprecated in 3.33) is now removed. [#26267](https://github.com/sourcegraph/sourcegraph/issues/26267)
- Query filter `repogroup` (deprecated in 3.33) is now removed. [#24277](https://github.com/sourcegraph/sourcegraph/issues/24277)
- Sourcegraph no longer uses CSRF security tokens/cookies to prevent CSRF attacks. Instead, Sourcegraph now relies solely on browser's CORS policies (which were already in place.) In practice, this is just as safe and leads to a simpler CSRF threat model which reduces security risks associated with our threat model complexity. [#7658](https://github.com/sourcegraph/sourcegraph/pull/7658)
- Notifications for saved searches (deprecated in v3.31.0) have been removed [#27912](https://github.com/sourcegraph/sourcegraph/pull/27912/files)

## 3.33.2

### Fixed

- Fixed: backported saved search and code monitor notification fixes from 3.34.0 [#28019](https://github.com/sourcegraph/sourcegraph/pull/28019)

## 3.33.1

### Fixed

- Private repositories matching `-repo:` expressions are now excluded. This was a regression introduced in 3.33.0. [#27044](https://github.com/sourcegraph/sourcegraph/issues/27044)
- Fixed a regression that was introduced in 3.27 and broke SSH-based authentication for managing Batch Changes changesets on code hosts. SSH keys generated by Sourcegraph were not used for authentication and authenticating with the code host would fail if no SSH key with write-access had been added to `gitserver`. [#27491](https://github.com/sourcegraph/sourcegraph/pull/27491)

## 3.33.0

### Added

- More rules have been added to the search query validation so that user get faster feedback on issues with their query. [#24747](https://github.com/sourcegraph/sourcegraph/pull/24747)
- Bloom filters have been added to the zoekt indexing backend to accelerate queries with code fragments matching `\w{4,}`. [zoekt#126](https://github.com/sourcegraph/zoekt/pull/126)
- For short search queries containing no filters but the name of a supported programming language we are now suggesting to run the query with a language filter. [#25792](https://github.com/sourcegraph/sourcegraph/pull/25792)
- The API scope used by GitLab OAuth can now optionally be configured in the provider. [#26152](https://github.com/sourcegraph/sourcegraph/pull/26152)
- Added Apex language support for syntax highlighting and search-based code intelligence. [#25268](https://github.com/sourcegraph/sourcegraph/pull/25268)

### Changed

- Search context management pages are now only available in the Sourcegraph enterprise version. Search context dropdown is disabled in the OSS version. [#25147](https://github.com/sourcegraph/sourcegraph/pull/25147)
- Search contexts GQL API is now only available in the Sourcegraph enterprise version. [#25281](https://github.com/sourcegraph/sourcegraph/pull/25281)
- When running a commit or diff query, the accepted values of `before` and `after` have changed from "whatever git accepts" to a [slightly more strict subset](https://docs.sourcegraph.com/code_search/reference/language#before) of that. [#25414](https://github.com/sourcegraph/sourcegraph/pull/25414)
- Repogroups and version contexts are deprecated in favor of search contexts. Read more about the deprecation and how to migrate to search contexts in the [blog post](https://about.sourcegraph.com/blog/introducing-search-contexts). [#25676](https://github.com/sourcegraph/sourcegraph/pull/25676)
- Search contexts are now enabled by default in the Sourcegraph enterprise version. [#25674](https://github.com/sourcegraph/sourcegraph/pull/25674)
- Code Insights background queries will now retry a maximum of 10 times (down from 100). [#26057](https://github.com/sourcegraph/sourcegraph/pull/26057)
- Our `sourcegraph/cadvisor` Docker image has been upgraded to cadvisor version `v0.42.0`. [#26126](https://github.com/sourcegraph/sourcegraph/pull/26126)
- Our `jaeger` version in the `sourcegraph/sourcegraph` Docker image has been upgraded to `1.24.0`. [#26215](https://github.com/sourcegraph/sourcegraph/pull/26215)

### Fixed

- A search regression in 3.32.0 which caused instances with search indexing _disabled_ (very rare) via `"search.index.enabled": false,` in their site config to crash with a panic. [#25321](https://github.com/sourcegraph/sourcegraph/pull/25321)
- An issue where the default `search.index.enabled` value on single-container Docker instances would incorrectly be computed as `false` in some situations. [#25321](https://github.com/sourcegraph/sourcegraph/pull/25321)
- StatefulSet service discovery in Kubernetes correctly constructs pod hostnames in the case where the ServiceName is different from the StatefulSet name. [#25146](https://github.com/sourcegraph/sourcegraph/pull/25146)
- An issue where clicking on a link in the 'Revisions' search sidebar section would result in an invalid query if the query didn't already contain a 'repo:' filter. [#25076](https://github.com/sourcegraph/sourcegraph/pull/25076)
- An issue where links to jump to Bitbucket Cloud wouldn't render in the UI. [#25533](https://github.com/sourcegraph/sourcegraph/pull/25533)
- Fixed some code insights pings being aggregated on `anonymous_user_id` instead of `user_id`. [#25926](https://github.com/sourcegraph/sourcegraph/pull/25926)
- Code insights running over all repositories using a commit search (`type:commit` or `type:diff`) would fail to deserialize and produce no results. [#25928](https://github.com/sourcegraph/sourcegraph/pull/25928)
- Fixed an issue where code insights queries could produce a panic on queued records that did not include a `record_time` [#25929](https://github.com/sourcegraph/sourcegraph/pull/25929)
- Fixed an issue where Batch Change changeset diffs would sometimes render incorrectly when previewed from the UI if they contained deleted empty lines. [#25866](https://github.com/sourcegraph/sourcegraph/pull/25866)
- An issue where `repo:contains.commit.after()` would fail on some malformed git repositories. [#25974](https://github.com/sourcegraph/sourcegraph/issues/25974)
- Fixed primary email bug where users with no primary email set would break the email setting page when trying to add a new email. [#25008](https://github.com/sourcegraph/sourcegraph/pull/25008)
- An issue where keywords like `and`, `or`, `not` would not be highlighted properly in the search bar due to the presence of quotes. [#26135](https://github.com/sourcegraph/sourcegraph/pull/26135)
- An issue where frequent search indexing operations led to incoming search queries timing out. When these timeouts happened in quick succession, `zoekt-webserver` processes would shut themselves down via their `watchdog` routine. This should now only happen when a given `zoekt-webserver` is under-provisioned on CPUs. [#25872](https://github.com/sourcegraph/sourcegraph/issues/25872)
- Since 3.28.0, Batch Changes webhooks would not update changesets opened in private repositories. This has been fixed. [#26380](https://github.com/sourcegraph/sourcegraph/issues/26380)
- Reconciling batch changes could stall when updating the state of a changeset that already existed. This has been fixed. [#26386](https://github.com/sourcegraph/sourcegraph/issues/26386)

### Removed

- Batch Changes changeset specs stored the raw JSON used when creating them, which is no longer used and is not exposed in the API. This column has been removed, thereby saving space in the Sourcegraph database. [#25453](https://github.com/sourcegraph/sourcegraph/issues/25453)
- The query builder page experimental feature, which was disabled in 3.21, is now removed. The setting `{ "experimentalFeatures": { "showQueryBuilder": true } }` now has no effect. [#26125](https://github.com/sourcegraph/sourcegraph/pull/26125)

## 3.32.1

### Fixed

- Fixed a regression that was introduced in 3.27 and broke SSH-based authentication for managing Batch Changes changesets on code hosts. SSH keys generated by Sourcegraph were not used for authentication and authenticating with the code host would fail if no SSH key with write-access had been added to `gitserver`. [#27491](https://github.com/sourcegraph/sourcegraph/pull/27491)

## 3.32.0

### Added

- The search sidebar shows a revisions section if all search results are from a single repository. This makes it easier to search in and switch between different revisions. [#23835](https://github.com/sourcegraph/sourcegraph/pull/23835)
- The various alerts overview panels in Grafana can now be clicked to go directly to the relevant panels and dashboards. [#24920](https://github.com/sourcegraph/sourcegraph/pull/24920)
- Added a `Documentation` tab to the Site Admin Maintenance panel that links to the official Sourcegraph documentation. [#24917](https://github.com/sourcegraph/sourcegraph/pull/24917)
- Code Insights that run over all repositories now generate a moving daily snapshot between time points. [#24804](https://github.com/sourcegraph/sourcegraph/pull/24804)
- The Code Insights GraphQL API now restricts the results to user, org, and globally scoped insights. Insights will be synced to the database with access associated to the user or org setting containing the insight definition. [#25017](https://github.com/sourcegraph/sourcegraph/pull/25017)
- The timeout for long-running Git commands can be customized via `gitLongCommandTimeout` in the site config. [#25080](https://github.com/sourcegraph/sourcegraph/pull/25080)

### Changed

- `allowGroupsPermissionsSync` in the GitHub authorization provider is now required to enable the experimental GitHub teams and organization permissions caching. [#24561](https://github.com/sourcegraph/sourcegraph/pull/24561)
- GitHub external code hosts now validate if a corresponding authorization provider is set, and emits a warning if not. [#24526](https://github.com/sourcegraph/sourcegraph/pull/24526)
- Sourcegraph is now built with Go 1.17. [#24566](https://github.com/sourcegraph/sourcegraph/pull/24566)
- Code Insights is now available only in the Sourcegraph enterprise. [#24741](https://github.com/sourcegraph/sourcegraph/pull/24741)
- Prometheus in Sourcegraph with Docker Compose now scrapes Postgres and Redis instances for metrics. [deploy-sourcegraph-docker#580](https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/580)
- Symbol suggestions now leverage optimizations for global searches. [#24943](https://github.com/sourcegraph/sourcegraph/pull/24943)

### Fixed

- Fixed a number of issues where repository permissions sync may fail for instances with very large numbers of repositories. [#24852](https://github.com/sourcegraph/sourcegraph/pull/24852), [#24972](https://github.com/sourcegraph/sourcegraph/pull/24972)
- Fixed excessive re-rendering of the whole web application on every keypress in the search query input. [#24844](https://github.com/sourcegraph/sourcegraph/pull/24844)
- Code Insights line chart now supports different timelines for each data series (lines). [#25005](https://github.com/sourcegraph/sourcegraph/pull/25005)
- Postgres exporter now exposes pg_stat_activity account to show the number of active DB connections. [#25086](https://github.com/sourcegraph/sourcegraph/pull/25086)

### Removed

- The `PRECISE_CODE_INTEL_DATA_TTL` environment variable is no longer read by the worker service. Instead, global and repository-specific data retention policies configurable in the UI by site-admins will control the length of time LSIF uploads are considered _fresh_. [#24793](https://github.com/sourcegraph/sourcegraph/pull/24793)
- The `repo.cloned` column was removed as it was deprecated in 3.26. [#25066](https://github.com/sourcegraph/sourcegraph/pull/25066)

## 3.31.2

### Fixed

- Fixed multiple CVEs for [libssl](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-3711) and [Python3](https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2021-29921). [#24700](https://github.com/sourcegraph/sourcegraph/pull/24700) [#24620](https://github.com/sourcegraph/sourcegraph/pull/24620) [#24695](https://github.com/sourcegraph/sourcegraph/pull/24695)

## 3.31.1

### Added

- The required authentication scopes required to enable caching behaviour for GitHub repository permissions can now be requested via `allowGroupsPermissionsSync` in GitHub `auth.providers`. [#24328](https://github.com/sourcegraph/sourcegraph/pull/24328)

### Changed

- Caching behaviour for GitHub repository permissions enabled via the `authorization.groupsCacheTTL` field in the code host config can now leverage additional caching of team and organization permissions for repository permissions syncing (on top of the caching for user permissions syncing introduced in 3.31). [#24328](https://github.com/sourcegraph/sourcegraph/pull/24328)

## 3.31.0

### Added

- Backend Code Insights GraphQL queries now support arguments `includeRepoRegex` and `excludeRepoRegex` to filter on repository names. [#23256](https://github.com/sourcegraph/sourcegraph/pull/23256)
- Code Insights background queries now process in a priority order backwards through time. This will allow insights to populate concurrently. [#23101](https://github.com/sourcegraph/sourcegraph/pull/23101)
- Operator documentation has been added to the Search Reference sidebar section. [#23116](https://github.com/sourcegraph/sourcegraph/pull/23116)
- Syntax highlighting support for the [Cue](https://cuelang.org) language.
- Reintroduced a revised version of the Search Types sidebar section. [#23170](https://github.com/sourcegraph/sourcegraph/pull/23170)
- Improved usability where filters followed by a space in the search query will warn users that the filter value is empty. [#23646](https://github.com/sourcegraph/sourcegraph/pull/23646)
- Perforce: [`git p4`'s `--use-client-spec` option](https://git-scm.com/docs/git-p4#Documentation/git-p4.txt---use-client-spec) can now be enabled by configuring the `p4.client` field. [#23833](https://github.com/sourcegraph/sourcegraph/pull/23833), [#23845](https://github.com/sourcegraph/sourcegraph/pull/23845)
- Code Insights will do a one-time reset of ephemeral insights specific database tables to clean up stale and invalid data. Insight data will regenerate automatically. [23791](https://github.com/sourcegraph/sourcegraph/pull/23791)
- Perforce: added basic support for Perforce permission table path wildcards. [#23755](https://github.com/sourcegraph/sourcegraph/pull/23755)
- Added autocompletion and search filtering of branch/tag/commit revisions to the repository compare page. [#23977](https://github.com/sourcegraph/sourcegraph/pull/23977)
- Batch Changes changesets can now be [set to published when previewing new or updated batch changes](https://docs.sourcegraph.com/batch_changes/how-tos/publishing_changesets#within-the-ui). [#22912](https://github.com/sourcegraph/sourcegraph/issues/22912)
- Added Python3 to server and gitserver images to enable git-p4 support. [#24204](https://github.com/sourcegraph/sourcegraph/pull/24204)
- Code Insights drill-down filters now allow filtering insights data on the dashboard page using repo: filters. [#23186](https://github.com/sourcegraph/sourcegraph/issues/23186)
- GitHub repository permissions can now leverage caching of team and organization permissions for user permissions syncing. Caching behaviour can be enabled via the `authorization.groupsCacheTTL` field in the code host config. This can significantly reduce the amount of time it takes to perform a full permissions sync due to reduced instances of being rate limited by the code host. [#23978](https://github.com/sourcegraph/sourcegraph/pull/23978)

### Changed

- Code Insights will now always backfill from the time the data series was created. [#23430](https://github.com/sourcegraph/sourcegraph/pull/23430)
- Code Insights queries will now extract repository name out of the GraphQL response instead of going to the database. [#23388](https://github.com/sourcegraph/sourcegraph/pull/23388)
- Code Insights backend has moved from the `repo-updater` service to the `worker` service. [#23050](https://github.com/sourcegraph/sourcegraph/pull/23050)
- Code Insights feature flag `DISABLE_CODE_INSIGHTS` environment variable has moved from the `repo-updater` service to the `worker` service. Any users of this flag will need to update their `worker` service configuration to continue using it. [#23050](https://github.com/sourcegraph/sourcegraph/pull/23050)
- Updated Docker-Compose Caddy Image to v2.0.0-alpine. [#468](https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/468)
- Code Insights historical samples will record using the timestamp of the commit that was searched. [#23520](https://github.com/sourcegraph/sourcegraph/pull/23520)
- Authorization checks are now handled using role based permissions instead of manually altering SQL statements. [23398](https://github.com/sourcegraph/sourcegraph/pull/23398)
- Docker Compose: the Jaeger container's `SAMPLING_STRATEGIES_FILE` now has a default value. If you are currently using a custom sampling strategies configuration, you may need to make sure your configuration is not overridden by the change when upgrading. [sourcegraph/deploy-sourcegraph#489](https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/489)
- Code Insights historical samples will record using the most recent commit to the start of the frame instead of the middle of the frame. [#23573](https://github.com/sourcegraph/sourcegraph/pull/23573)
- The copy icon displayed next to files and repositories will now copy the file or repository path. Previously, this action copied the URL to clipboard. [#23390](https://github.com/sourcegraph/sourcegraph/pull/23390)
- Sourcegraph's Prometheus dependency has been upgraded to v2.28.1. [23663](https://github.com/sourcegraph/sourcegraph/pull/23663)
- Sourcegraph's Alertmanager dependency has been upgraded to v0.22.2. [23663](https://github.com/sourcegraph/sourcegraph/pull/23714)
- Code Insights will now schedule sample recordings for the first of the next month after creation or a previous recording. [#23799](https://github.com/sourcegraph/sourcegraph/pull/23799)
- Code Insights now stores data in a new format. Data points will store complete vectors for all repositories even if the underlying Sourcegraph queries were compressed. [#23768](https://github.com/sourcegraph/sourcegraph/pull/23768)
- Code Insights rate limit values have been tuned for a more reasonable performance. [#23860](https://github.com/sourcegraph/sourcegraph/pull/23860)
- Code Insights will now generate historical data once per month on the first of the month, up to the configured `insights.historical.frames` number of frames. [#23768](https://github.com/sourcegraph/sourcegraph/pull/23768)
- Code Insights will now schedule recordings for the first of the next calendar month after an insight is created or recorded. [#23799](https://github.com/sourcegraph/sourcegraph/pull/23799)
- Code Insights will attempt to sync insight definitions from settings to the database once every 10 minutes. [23805](https://github.com/sourcegraph/sourcegraph/pull/23805)
- Code Insights exposes information about queries that are flagged `dirty` through the `insights` GraphQL query. [#23857](https://github.com/sourcegraph/sourcegraph/pull/23857/)
- Code Insights GraphQL query `insights` will now fetch 12 months of data instead of 6 if a specific time range is not provided. [#23786](https://github.com/sourcegraph/sourcegraph/pull/23786)
- Code Insights will now generate 12 months of historical data during a backfill instead of 6. [#23860](https://github.com/sourcegraph/sourcegraph/pull/23860)
- The `sourcegraph-frontend.Role` in Kubernetes deployments was updated to permit statefulsets access in the Kubernetes API. This is needed to better support stable service discovery for stateful sets during deployments, which isn't currently possible by using service endpoints. [#3670](https://github.com/sourcegraph/deploy-sourcegraph/pull/3670) [#23889](https://github.com/sourcegraph/sourcegraph/pull/23889)
- For Docker-Compose and Kubernetes users, the built-in main Postgres and codeintel databases have switched to an alpine Docker image. This requires re-indexing the entire database. This process can take up to a few hours on systems with large datasets. [#23697](https://github.com/sourcegraph/sourcegraph/pull/23697)
- Results are now streamed from searcher by default, improving memory usage and latency for large, unindexed searches. [#23754](https://github.com/sourcegraph/sourcegraph/pull/23754)
- [`deploy-sourcegraph` overlays](https://docs.sourcegraph.com/admin/install/kubernetes/configure#overlays) now use `resources:` instead of the [deprecated `bases:` field](https://kubectl.docs.kubernetes.io/references/kustomize/kustomization/bases/) for referencing Kustomize bases. [deploy-sourcegraph#3606](https://github.com/sourcegraph/deploy-sourcegraph/pull/3606)
- The `deploy-sourcegraph-docker` Pure Docker deployment scripts and configuration has been moved to the `./pure-docker` subdirectory. [deploy-sourcegraph-docker#454](https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/454)
- In Kubernetes deployments, setting the `SRC_GIT_SERVERS` environment variable explicitly is no longer needed. Addresses of the gitserver pods will be discovered automatically and in the same numerical order as with the static list. Unset the env var in your `frontend.Deployment.yaml` to make use of this feature. [#24094](https://github.com/sourcegraph/sourcegraph/pull/24094)
- The consistent hashing scheme used to distribute repositories across indexed-search replicas has changed to improve distribution and reduce load discrepancies. In the next upgrade, indexed-search pods will re-index the majority of repositories since the repo to replica assignments will change. This can take a few hours in large instances, but searches should succeed during that time since a replica will only delete a repo once it has been indexed in the new replica that owns it. You can monitor this process in the Zoekt Index Server Grafana dashboard—the "assigned" repos in "Total number of repos" will spike and then reduce until it becomes the same as "indexed". As a fail-safe, the old consistent hashing scheme can be enabled by setting the `SRC_ENDPOINTS_CONSISTENT_HASH` env var to `consistent(crc32ieee)` in the `sourcegraph-frontend` deployment. [#23921](https://github.com/sourcegraph/sourcegraph/pull/23921)
- In Kubernetes deployments an emptyDir (`/dev/shm`) is now mounted in the `pgsql` deployment to allow Postgres to access more than 64KB shared memory. This value should be configured to match the `shared_buffers` value in your Postgres configuration. [deploy-sourcegraph#3784](https://github.com/sourcegraph/deploy-sourcegraph/pull/3784/)

### Fixed

- The search reference will now show matching entries when using the filter input. [#23224](https://github.com/sourcegraph/sourcegraph/pull/23224)
- Graceful termination periods have been added to database deployments. [#3358](https://github.com/sourcegraph/deploy-sourcegraph/pull/3358) & [#477](https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/477)
- All commit search results for `and`-expressions are now highlighted. [#23336](https://github.com/sourcegraph/sourcegraph/pull/23336)
- Email notifiers in `observability.alerts` now correctly respect the `email.smtp.noVerifyTLS` site configuration field. [#23636](https://github.com/sourcegraph/sourcegraph/issues/23636)
- Alertmanager (Prometheus) now respects `SMTPServerConfig.noVerifyTLS` field. [#23636](https://github.com/sourcegraph/sourcegraph/issues/23636)
- Clicking on symbols in the left search pane now renders hover tooltips for indexed repositories. [#23664](https://github.com/sourcegraph/sourcegraph/pull/23664)
- Fixed a result streaming throttling issue that was causing significantly increased latency for some searches. [#23736](https://github.com/sourcegraph/sourcegraph/pull/23736)
- GitCredentials passwords stored in AWS CodeCommit configuration is now redacted. [#23832](https://github.com/sourcegraph/sourcegraph/pull/23832)
- Patched a vulnerability in `apk-tools`. [#23917](https://github.com/sourcegraph/sourcegraph/pull/23917)
- Line content was being duplicated in unindexed search payloads, causing memory instability for some dense search queries. [#23918](https://github.com/sourcegraph/sourcegraph/pull/23918)
- Updating draft merge requests on GitLab from batch changes no longer removes the draft status. [#23944](https://github.com/sourcegraph/sourcegraph/issues/23944)
- Report highlight matches instead of line matches in search results. [#21443](https://github.com/sourcegraph/sourcegraph/issues/21443)
- Force the `codeinsights-db` database to read from the `configMap` configuration file by explicitly setting the `POSTGRESQL_CONF_DIR` environment variable to the `configMap` mount path. [deploy-sourcegraph#3788](https://github.com/sourcegraph/deploy-sourcegraph/pull/3788)

### Removed

- The old batch repository syncer was removed and can no longer be activated by setting `ENABLE_STREAMING_REPOS_SYNCER=false`. [#22949](https://github.com/sourcegraph/sourcegraph/pull/22949)
- Email notifications for saved searches are now deprecated in favor of Code Monitoring. Email notifications can no longer be enabled for saved searches. Saved searches that already have notifications enabled will continue to work, but there is now a button users can click to migrate to code monitors. Notifications for saved searches will be removed entirely in the future. [#23275](https://github.com/sourcegraph/sourcegraph/pull/23275)
- The `sg_service` Postgres role and `sg_repo_access_policy` policy on the `repo` table have been removed due to performance concerns. [#23622](https://github.com/sourcegraph/sourcegraph/pull/23622)
- Deprecated site configuration field `email.smtp.disableTLS` has been removed. [#23639](https://github.com/sourcegraph/sourcegraph/pull/23639)
- Deprecated language servers have been removed from `deploy-sourcegraph`. [deploy-sourcegraph#3605](https://github.com/sourcegraph/deploy-sourcegraph/pull/3605)
- The experimental `codeInsightsAllRepos` feature flag has been removed. [#23850](https://github.com/sourcegraph/sourcegraph/pull/23850)

## 3.30.4

### Added

- Add a new environment variable `SRC_HTTP_CLI_EXTERNAL_TIMEOUT` to control the timeout for all external HTTP requests. [#23620](https://github.com/sourcegraph/sourcegraph/pull/23620)

### Changed

- Postgres has been upgraded to `12.8` in the single-server Sourcegraph image [#23999](https://github.com/sourcegraph/sourcegraph/pull/23999)

## 3.30.3

**⚠️ Users on 3.29.x are advised to upgrade directly to 3.30.3**. If you have already upgraded to 3.30.0, 3.30.1, or 3.30.2 please follow [this migration guide](https://docs.sourcegraph.com/admin/migration/3_30).

### Fixed

- Codeintel-db database images have been reverted back to debian due to corruption caused by glibc and alpine. [23324](https://github.com/sourcegraph/sourcegraph/pull/23324)

## 3.30.2

**⚠️ Users on 3.29.x are advised to upgrade directly to 3.30.3**. If you have already upgraded to 3.30.0, 3.30.1, or 3.30.2 please follow [this migration guide](https://docs.sourcegraph.com/admin/migration/3_30).

### Fixed

- Postgres database images have been reverted back to debian due to corruption caused by glibc and alpine. [23302](https://github.com/sourcegraph/sourcegraph/pull/23302)

## 3.30.1

**⚠️ Users on 3.29.x are advised to upgrade directly to 3.30.3**. If you have already upgraded to 3.30.0, 3.30.1, or 3.30.2 please follow [this migration guide](https://docs.sourcegraph.com/admin/migration/3_30).

### Fixed

- An issue where the UI would occasionally display `lsifStore.Ranges: ERROR: relation \"lsif_documentation_mappings\" does not exist (SQLSTATE 42P01)` [#23115](https://github.com/sourcegraph/sourcegraph/pull/23115)
- Fixed a vulnerability in our Postgres Alpine image related to libgcrypt [#23174](https://github.com/sourcegraph/sourcegraph/pull/23174)
- When syncing in streaming mode, repo-updater will now ensure a repo's transaction is committed before notifying gitserver to update that repo. [#23169](https://github.com/sourcegraph/sourcegraph/pull/23169)
- When encountering spurious errors during streaming syncing (like temporary 500s from codehosts), repo-updater will no longer delete all associated repos that weren't seen. Deletion will happen only if there were no errors or if the error was one of "Unauthorized", "Forbidden" or "Account Suspended". [#23171](https://github.com/sourcegraph/sourcegraph/pull/23171)
- External HTTP requests are now automatically retried when appropriate. [#23131](https://github.com/sourcegraph/sourcegraph/pull/23131)

## 3.30.0

**⚠️ Users on 3.29.x are advised to upgrade directly to 3.30.3**. If you have already upgraded to 3.30.0, 3.30.1, or 3.30.2 please follow [this migration guide](https://docs.sourcegraph.com/admin/migration/3_30).

### Added

- Added support for `select:file.directory` in search queries, which returns unique directory paths for results that satisfy the query. [#22449](https://github.com/sourcegraph/sourcegraph/pull/22449)
- An `sg_service` Postgres role has been introduced, as well as an `sg_repo_access_policy` policy on the `repo` table that restricts access to that role. The role that owns the `repo` table will continue to get unrestricted access. [#22303](https://github.com/sourcegraph/sourcegraph/pull/22303)
- Every service that connects to the database (i.e. Postgres) now has a "Database connections" monitoring section in its Grafana dashboard. [#22570](https://github.com/sourcegraph/sourcegraph/pull/22570)
- A new bulk operation to close many changesets at once has been added to Batch Changes. [#22547](https://github.com/sourcegraph/sourcegraph/pull/22547)
- Backend Code Insights will aggregate viewable repositories based on the authenticated user. [#22471](https://github.com/sourcegraph/sourcegraph/pull/22471)
- Added support for highlighting .frugal files as Thrift syntax.
- Added `file:contains.content(regexp)` predicate, which filters only to files that contain matches of the given pattern. [#22666](https://github.com/sourcegraph/sourcegraph/pull/22666)
- Repository syncing is now done in streaming mode by default. Customers with many repositories should notice code host updates much faster, with repo-updater consuming less memory. Using the previous batch mode can be done by setting the `ENABLE_STREAMING_REPOS_SYNCER` environment variable to `false` in `repo-updater`. That environment variable will be deleted in the next release. [#22756](https://github.com/sourcegraph/sourcegraph/pull/22756)
- Enabled the ability to query Batch Changes changesets, changesets stats, and file diff stats for an individual repository via the Sourcegraph GraphQL API. [#22744](https://github.com/sourcegraph/sourcegraph/pull/22744/)
- Added "Groovy" to the initial `lang:` filter suggestions in the search bar. [#22755](https://github.com/sourcegraph/sourcegraph/pull/22755)
- The `lang:` filter suggestions now show all supported, matching languages as the user types a language name. [#22765](https://github.com/sourcegraph/sourcegraph/pull/22765)
- Code Insights can now be grouped into dashboards. [#22215](https://github.com/sourcegraph/sourcegraph/issues/22215)
- Batch Changes changesets can now be [published from the Sourcegraph UI](https://docs.sourcegraph.com/batch_changes/how-tos/publishing_changesets#within-the-ui). [#18277](https://github.com/sourcegraph/sourcegraph/issues/18277)
- The repository page now has a new button to view batch change changesets created in that specific repository, with a badge indicating how many changesets are currently open. [#22804](https://github.com/sourcegraph/sourcegraph/pull/22804)
- Experimental: Search-based code insights can run over all repositories on the instance. To enable, use the feature flag `"experimentalFeatures": { "codeInsightsAllRepos": true }` and tick the checkbox in the insight creation/edit UI. [#22759](https://github.com/sourcegraph/sourcegraph/issues/22759)
- Search References is a new search sidebar section to simplify learning about the available search filters directly where they are used. [#21539](https://github.com/sourcegraph/sourcegraph/issues/21539)

### Changed

- Backend Code Insights only fills historical data frames that have changed to reduce the number of searches required. [#22298](https://github.com/sourcegraph/sourcegraph/pull/22298)
- Backend Code Insights displays data points for a fixed 6 months period in 2 week intervals, and will carry observations forward that are missing. [#22298](https://github.com/sourcegraph/sourcegraph/pull/22298)
- Backend Code Insights now aggregate over 26 weeks instead of 6 months. [#22527](https://github.com/sourcegraph/sourcegraph/pull/22527)
- Search queries now disallow specifying `rev:` without `repo:`. Note that to search across potentially multiple revisions, a query like `repo:.* rev:<revision>` remains valid. [#22705](https://github.com/sourcegraph/sourcegraph/pull/22705)
- The extensions status bar on diff pages has been redesigned and now shows information for both the base and head commits. [#22123](https://github.com/sourcegraph/sourcegraph/pull/22123/files)
- The `applyBatchChange` and `createBatchChange` mutations now accept an optional `publicationStates` argument to set the publication state of specific changesets within the batch change. [#22485](https://github.com/sourcegraph/sourcegraph/pull/22485) and [#22854](https://github.com/sourcegraph/sourcegraph/pull/22854)
- Search queries now return up to 80 suggested filters. Previously we returned up to 24. [#22863](https://github.com/sourcegraph/sourcegraph/pull/22863)
- GitHub code host connections can now include `repositoryQuery` entries that match more than 1000 repositories from the GitHub search API without requiring the previously documented work-around of splitting the query up with `created:` qualifiers, which is now done automatically. [#2562](https://github.com/sourcegraph/sourcegraph/issues/2562)

### Fixed

- The Batch Changes user and site credential encryption migrators added in Sourcegraph 3.28 could report zero progress when encryption was disabled, even though they had nothing to do. This has been fixed, and progress will now be correctly reported. [#22277](https://github.com/sourcegraph/sourcegraph/issues/22277)
- Listing Github Entreprise org repos now returns internal repos as well. [#22339](https://github.com/sourcegraph/sourcegraph/pull/22339)
- Jaeger works in Docker-compose deployments again. [#22691](https://github.com/sourcegraph/sourcegraph/pull/22691)
- A bug where the pattern `)` makes the browser unresponsive. [#22738](https://github.com/sourcegraph/sourcegraph/pull/22738)
- An issue where using `select:repo` in conjunction with `and` patterns did not yield expected repo results. [#22743](https://github.com/sourcegraph/sourcegraph/pull/22743)
- The `isLocked` and `isDisabled` fields of GitHub repositories are now fetched correctly from the GraphQL API of GitHub Enterprise instances. Users that rely on the `repos` config in GitHub code host connections should update so that locked and disabled repositories defined in that list are actually skipped. [#22788](https://github.com/sourcegraph/sourcegraph/pull/22788)
- Homepage no longer fails to load if there are invalid entries in user's search history. [#22857](https://github.com/sourcegraph/sourcegraph/pull/22857)
- An issue where regexp query highlighting in the search bar would render incorrectly on Firefox. [#23043](https://github.com/sourcegraph/sourcegraph/pull/23043)
- Code intelligence uploads and indexes are restricted to only site-admins. It was read-only for any user. [#22890](https://github.com/sourcegraph/sourcegraph/pull/22890)
- Daily usage statistics are restricted to only site-admins. It was read-only for any user. [#23026](https://github.com/sourcegraph/sourcegraph/pull/23026)
- Ephemeral storage requests now match their cache size requests for Kubernetes deployments. [#2953](https://github.com/sourcegraph/deploy-sourcegraph/pull/2953)

### Removed

- The experimental paginated search feature (the `stable:` keyword) has been removed, to be replaced with streaming search. [#22428](https://github.com/sourcegraph/sourcegraph/pull/22428)
- The experimental extensions view page has been removed. [#22565](https://github.com/sourcegraph/sourcegraph/pull/22565)
- A search query diagnostic that previously warned the user when quotes are interpreted literally has been removed. The literal meaning has been Sourcegraph's default search behavior for some time now. [#22892](https://github.com/sourcegraph/sourcegraph/pull/22892)
- Non-root overlays were removed for `deploy-sourcegraph` in favor of using `non-privileged`. [#3404](https://github.com/sourcegraph/deploy-sourcegraph/pull/3404)

### API docs (experimental)

API docs is a new experimental feature of Sourcegraph ([learn more](https://docs.sourcegraph.com/code_intelligence/apidocs)). It is enabled by default in Sourcegraph 3.30.0.

- API docs is enabled by default in Sourcegraph 3.30.0. It can be disabled by adding `"apiDocs": false` to the `experimentalFeatures` section of user settings.
- The API docs landing page now indicates what API docs are and provide more info.
- The API docs landing page now represents the code in the repository root, instead of an empty page.
- Pages now correctly indicate it is an experimental feature, and include a feedback widget.
- Subpages linked via the sidebar are now rendered much better, and have an expandable section.
- Symbols in documentation now have distinct icons for e.g. functions/vars/consts/etc.
- Symbols are now sorted in exported-first, alphabetical order.
- Repositories without LSIF documentation data now show a friendly error page indicating what languages are supported, how to set it up, etc.
- API docs can now distinguish between different types of symbols, tests, examples, benchmarks, etc. and whether symbols are public/private—to support filtering in the future.
- Only public/exported symbols are included by default for now.
- URL paths for Go packages are now friendlier, e.g. `/-/docs/cmd/frontend/auth` instead of `/-/docs/cmd-frontend-auth`.
- URLs are now formatted by the language indexer, in a way that makes sense for the language, e.g. `#Mocks.CreateUserAndSave` instead of `#ypeMocksCreateUserAndSave` for a Go method `CreateUserAndSave` on type `Mocks`.
- Go blank identifier assignments `var _ = ...` are no longer incorrectly included.
- Go symbols defined within functions, e.g. a `var` inside a `func` scope are no longer incorrectly included.
- `Functions`, `Variables`, and other top-level sections are no longer rendered empty if there are none in that section.
- A new test suite for LSIF indexers implementing the Sourcegraph documentation extension to LSIF [is available](https://github.com/sourcegraph/lsif-static-doc).
- We now emit the LSIF data needed to in the future support "Jump to API docs" from code views, "View code" from API docs, usage examples in API docs, and search indexing.
- Various UI style issues, color contrast issues, etc. have been fixed.
- Major improvements to the GraphQL APIs for API documentation.

## 3.29.0

### Added

- Code Insights queries can now run concurrently up to a limit set by the `insights.query.worker.concurrency` site config. [#21219](https://github.com/sourcegraph/sourcegraph/pull/21219)
- Code Insights workers now support a rate limit for query execution and historical data frame analysis using the `insights.query.worker.rateLimit` and `insights.historical.worker.rateLimit` site configurations. [#21533](https://github.com/sourcegraph/sourcegraph/pull/21533)
- The GraphQL `Site` `SettingsSubject` type now has an `allowSiteSettingsEdits` field to allow clients to determine whether the instance uses the `GLOBAL_SETTINGS_FILE` environment variable. [#21827](https://github.com/sourcegraph/sourcegraph/pull/21827)
- The Code Insights creation UI now remembers previously filled-in field values when returning to the form after having navigated away. [#21744](https://github.com/sourcegraph/sourcegraph/pull/21744)
- The Code Insights creation UI now shows autosuggestions for the repository field. [#21699](https://github.com/sourcegraph/sourcegraph/pull/21699)
- A new bulk operation to retry many changesets at once has been added to Batch Changes. [#21173](https://github.com/sourcegraph/sourcegraph/pull/21173)
- A `security_event_logs` database table has been added in support of upcoming security-related efforts. [#21949](https://github.com/sourcegraph/sourcegraph/pull/21949)
- Added featured Sourcegraph extensions query to the GraphQL API, as well as a section in the extension registry to display featured extensions. [#21665](https://github.com/sourcegraph/sourcegraph/pull/21665)
- The search page now has a `create insight` button to create search-based insight based on your search query [#21943](https://github.com/sourcegraph/sourcegraph/pull/21943)
- Added support for Terraform syntax highlighting. [#22040](https://github.com/sourcegraph/sourcegraph/pull/22040)
- A new bulk operation to merge many changesets at once has been added to Batch Changes. [#21959](https://github.com/sourcegraph/sourcegraph/pull/21959)
- Pings include aggregated usage for the Code Insights creation UI, organization visible insight count per insight type, and insight step size in days. [#21671](https://github.com/sourcegraph/sourcegraph/pull/21671)
- Search-based insight creation UI now supports `count:` filter in data series query input. [#22049](https://github.com/sourcegraph/sourcegraph/pull/22049)
- Code Insights background workers will now index commits in a new table `commit_index` for future optimization efforts. [#21994](https://github.com/sourcegraph/sourcegraph/pull/21994)
- The creation UI for search-based insights now supports the `count:` filter in the data series query input. [#22049](https://github.com/sourcegraph/sourcegraph/pull/22049)
- A new service, `worker`, has been introduced to run background jobs that were previously run in the frontend. See the [deployment documentation](https://docs.sourcegraph.com/admin/workers) for additional details. [#21768](https://github.com/sourcegraph/sourcegraph/pull/21768)

### Changed

- SSH public keys generated to access code hosts with batch changes now include a comment indicating they originated from Sourcegraph. [#20523](https://github.com/sourcegraph/sourcegraph/issues/20523)
- The copy query button is now permanently enabled and `experimentalFeatures.copyQueryButton` setting has been deprecated. [#21364](https://github.com/sourcegraph/sourcegraph/pull/21364)
- Search streaming is now permanently enabled and `experimentalFeatures.searchStreaming` setting has been deprecated. [#21522](https://github.com/sourcegraph/sourcegraph/pull/21522)
- Pings removes the collection of aggregate search filter usage counts and adds a smaller set of aggregate usage counts for query operators, predicates, and pattern counts. [#21320](https://github.com/sourcegraph/sourcegraph/pull/21320)
- Sourcegraph will now refuse to start if there are unfinished [out-of-band-migrations](https://docs.sourcegraph.com/admin/migrations) that are deprecated in the current version. See the [upgrade documentation](https://docs.sourcegraph.com/admin/updates) for changes to the upgrade process. [#20967](https://github.com/sourcegraph/sourcegraph/pull/20967)
- Code Insight pages now have new URLs [#21856](https://github.com/sourcegraph/sourcegraph/pull/21856)
- We are proud to bring you [an entirely new visual design for the Sourcegraph UI](https://about.sourcegraph.com/blog/introducing-sourcegraphs-new-ui/). We think you’ll find this new design improves your experience and sets the stage for some incredible features to come. Some of the highlights include:

  - **Refined search results:** The redesigned search bar provides more space for expressive queries, and the new results sidebar helps to discover search syntax without referencing documentation.
  - **Improved focus on code:** We’ve reduced non-essential UI elements to provide greater focus on the code itself, and positioned the most important items so they’re unobtrusive and located exactly where they are needed.
  - **Improved layouts:** We’ve improved pages like diff views to make them easier to use and to help find information quickly.
  - **New navigation:** A new global navigation provides immediate discoverability and access to current and future functionality.
  - **Promoting extensibility:** We've brought the extension registry back to the main navigation and improved its design and navigation.

  With bulk of the redesign complete, future releases will include more improvements and refinements.

### Fixed

- Stricter validation of structural search queries. The `type:` parameter is not supported for structural searches and returns an appropriate alert. [#21487](https://github.com/sourcegraph/sourcegraph/pull/21487)
- Batch changeset specs that are not attached to changesets will no longer prematurely expire before the batch specs that they are associated with. [#21678](https://github.com/sourcegraph/sourcegraph/pull/21678)
- The Y-axis of Code Insights line charts no longer start at a negative value. [#22018](https://github.com/sourcegraph/sourcegraph/pull/22018)
- Correctly handle field aliases in the query (like `r:` versus `repo:`) when used with `contains` predicates. [#22105](https://github.com/sourcegraph/sourcegraph/pull/22105)
- Running a code insight over a timeframe when the repository didn't yet exist doesn't break the entire insight anymore. [#21288](https://github.com/sourcegraph/sourcegraph/pull/21288)

### Removed

- The deprecated GraphQL `icon` field on CommitSearchResult and Repository was removed. [#21310](https://github.com/sourcegraph/sourcegraph/pull/21310)
- The undocumented `index` filter was removed from search type-ahead suggestions. [#18806](https://github.com/sourcegraph/sourcegraph/issues/18806)
- Code host connection tokens aren't used for creating changesets anymore when the user is site admin and no credential has been specified. [#16814](https://github.com/sourcegraph/sourcegraph/issues/16814)

## 3.28.0

### Added

- Added `select:commit.diff.added` and `select:commit.diff.removed` for `type:diff` search queries. These selectors return commit diffs only if a pattern matches in `added` (respespectively, `removed`) lines. [#20328](https://github.com/sourcegraph/sourcegraph/pull/20328)
- Additional language autocompletions for the `lang:` filter in the search bar. [#20535](https://github.com/sourcegraph/sourcegraph/pull/20535)
- Steps in batch specs can now have an `if:` attribute to enable conditional execution of different steps. [#20701](https://github.com/sourcegraph/sourcegraph/pull/20701)
- Extensions can now log messages through `sourcegraph.app.log` to aid debugging user issues. [#20474](https://github.com/sourcegraph/sourcegraph/pull/20474)
- Bulk comments on many changesets are now available in Batch Changes. [#20361](https://github.com/sourcegraph/sourcegraph/pull/20361)
- Batch specs are now viewable when previewing changesets. [#19534](https://github.com/sourcegraph/sourcegraph/issues/19534)
- Added a new UI for creating code insights. [#20212](https://github.com/sourcegraph/sourcegraph/issues/20212)

### Changed

- User and site credentials used in Batch Changes are now encrypted in the database if encryption is enabled with the `encryption.keys` config. [#19570](https://github.com/sourcegraph/sourcegraph/issues/19570)
- All Sourcegraph images within [deploy-sourcegraph](https://github.com/sourcegraph/deploy-sourcegraph) now specify the registry. Thanks! @k24dizzle [#2901](https://github.com/sourcegraph/deploy-sourcegraph/pull/2901).
- Default reviewers are now added to Bitbucket Server PRs opened by Batch Changes. [#20551](https://github.com/sourcegraph/sourcegraph/pull/20551)
- The default memory requirements for the `redis-*` containers have been raised by 1GB (to a new total of 7GB). This change allows Redis to properly run its key-eviction routines (when under memory pressure) without getting killed by the host machine. This affects both the docker-compose and Kubernetes deployments. [sourcegraph/deploy-sourcegraph-docker#373](https://github.com/sourcegraph/deploy-sourcegraph-docker/pull/373) and [sourcegraph/deploy-sourcegraph#2898](https://github.com/sourcegraph/deploy-sourcegraph/pull/2898)
- Only site admins can now list users on an instance. [#20619](https://github.com/sourcegraph/sourcegraph/pull/20619)
- Repository permissions can now be enabled for site admins via the `authz.enforceForSiteAdmins` setting. [#20674](https://github.com/sourcegraph/sourcegraph/pull/20674)
- Site admins can no longer view user added code host configuration. [#20851](https://github.com/sourcegraph/sourcegraph/pull/20851)
- Site admins cannot add access tokens for any user by default. [#20988](https://github.com/sourcegraph/sourcegraph/pull/20988)
- Our namespaced overlays now only scrape container metrics within that namespace. [#2969](https://github.com/sourcegraph/deploy-sourcegraph/pull/2969)
- The extension registry main page has a new visual design that better conveys the most useful information about extensions, and individual extension pages have better information architecture. [#20822](https://github.com/sourcegraph/sourcegraph/pull/20822)

### Fixed

- Search returned inconsistent result counts when a `count:` limit was not specified.
- Indexed search failed when the `master` branch needed indexing but was not the default. [#20260](https://github.com/sourcegraph/sourcegraph/pull/20260)
- `repo:contains(...)` built-in did not respect parameters that affect repo filtering (e.g., `repogroup`, `fork`). It now respects these. [#20339](https://github.com/sourcegraph/sourcegraph/pull/20339)
- An issue where duplicate results would render for certain `or`-expressions. [#20480](https://github.com/sourcegraph/sourcegraph/pull/20480)
- Issue where the search query bar suggests that some `lang` values are not valid. [#20534](https://github.com/sourcegraph/sourcegraph/pull/20534)
- Pull request event webhooks received from GitHub with unexpected actions no longer cause panics. [#20571](https://github.com/sourcegraph/sourcegraph/pull/20571)
- Repository search patterns like `^repo/(prefix-suffix|prefix)$` now correctly match both `repo/prefix-suffix` and `repo/prefix`. [#20389](https://github.com/sourcegraph/sourcegraph/issues/20389)
- Ephemeral storage requests and limits now match the default cache size to avoid Symbols pods being evicted. The symbols pod now requires 10GB of ephemeral space as a minimum to scheduled. [#2369](https://github.com/sourcegraph/deploy-sourcegraph/pull/2369)
- Minor query syntax highlighting bug for `repo:contains` predicate. [#21038](https://github.com/sourcegraph/sourcegraph/pull/21038)
- An issue causing diff and commit results with file filters to return invalid results. [#21039](https://github.com/sourcegraph/sourcegraph/pull/21039)
- All databases now have the Kubernetes Quality of Service class of 'Guaranteed' which should reduce the chance of them
  being evicted during NodePressure events. [#2900](https://github.com/sourcegraph/deploy-sourcegraph/pull/2900)
- An issue causing diff views to display without syntax highlighting [#21160](https://github.com/sourcegraph/sourcegraph/pull/21160)

### Removed

- The deprecated `SetRepositoryEnabled` mutation was removed. [#21044](https://github.com/sourcegraph/sourcegraph/pull/21044)

## 3.27.5

### Fixed

- Fix scp style VCS url parsing. [#20799](https://github.com/sourcegraph/sourcegraph/pull/20799)

## 3.27.4

### Fixed

- Fixed an issue related to Gitolite repos with `@` being prepended with a `?`. [#20297](https://github.com/sourcegraph/sourcegraph/pull/20297)
- Add missing return from handler when DisableAutoGitUpdates is true. [#20451](https://github.com/sourcegraph/sourcegraph/pull/20451)

## 3.27.3

### Fixed

- Pushing batch changes to Bitbucket Server code hosts over SSH was broken in 3.27.0, and has been fixed. [#20324](https://github.com/sourcegraph/sourcegraph/issues/20324)

## 3.27.2

### Fixed

- Fixed an issue with our release tooling that was preventing all images from being tagged with the correct version.
  All sourcegraph images have the proper release version now.

## 3.27.1

### Fixed

- Indexed search failed when the `master` branch needed indexing but was not the default. [#20260](https://github.com/sourcegraph/sourcegraph/pull/20260)
- Fixed a regression that caused "other" code hosts urls to not be built correctly which prevents code to be cloned / updated in 3.27.0. This change will provoke some cloning errors on repositories that are already sync'ed, until the next code host sync. [#20258](https://github.com/sourcegraph/sourcegraph/pull/20258)

## 3.27.0

### Added

- `count:` now supports "all" as value. Queries with `count:all` will return up to 999999 results. [#19756](https://github.com/sourcegraph/sourcegraph/pull/19756)
- Credentials for Batch Changes are now validated when adding them. [#19602](https://github.com/sourcegraph/sourcegraph/pull/19602)
- Batch Changes now ignore repositories that contain a `.batchignore` file. [#19877](https://github.com/sourcegraph/sourcegraph/pull/19877) and [src-cli#509](https://github.com/sourcegraph/src-cli/pull/509)
- Side-by-side diff for commit visualization. [#19553](https://github.com/sourcegraph/sourcegraph/pull/19553)
- The site configuration now supports defining batch change rollout windows, which can be used to slow or disable pushing changesets at particular times of day or days of the week. [#19796](https://github.com/sourcegraph/sourcegraph/pull/19796), [#19797](https://github.com/sourcegraph/sourcegraph/pull/19797), and [#19951](https://github.com/sourcegraph/sourcegraph/pull/19951).
- Search functionality via built-in `contains` predicate: `repo:contains(...)`, `repo:contains.file(...)`, `repo:contains.content(...)`, repo:contains.commit.after(...)`. [#18584](https://github.com/sourcegraph/sourcegraph/issues/18584)
- Database encryption, external service config & user auth data can now be encrypted in the database using the `encryption.keys` config. See [the docs](https://docs.sourcegraph.com/admin/encryption) for more info.
- Repositories that gitserver fails to clone or fetch are now gradually moved to the back of the background update queue instead of remaining at the front. [#20204](https://github.com/sourcegraph/sourcegraph/pull/20204)
- The new `disableAutoCodeHostSyncs` setting allows site admins to disable any periodic background syncing of configured code host connections. That includes syncing of repository metadata (i.e. not git updates, use `disableAutoGitUpdates` for that), permissions and batch changes changesets, but may include other data we'd sync from the code host API in the future.

### Changed

- Bumped the minimum supported version of Postgres from `9.6` to `12`. The upgrade procedure is mostly automated for existing deployments, but may require action if using the single-container deployment or an external database. See the [upgrade documentation](https://docs.sourcegraph.com/admin/updates) for your deployment type for detailed instructions.
- Changesets in batch changes will now be marked as archived instead of being detached when a new batch spec that doesn't include the changesets is applied. Once they're archived users can manually detach them in the UI. [#19527](https://github.com/sourcegraph/sourcegraph/pull/19527)
- The default replica count on `sourcegraph-frontend` and `precise-code-intel-worker` for Kubernetes has changed from `1` -> `2`.
- Changes to code monitor trigger search queries [#19680](https://github.com/sourcegraph/sourcegraph/pull/19680)
  - A `repo:` filter is now required. This is due to an existing limitations where only 50 repositories can be searched at a time, so using a `repo:` filter makes sure the right code is being searched. Any existing code monitor without `repo:` in the trigger query will continue to work (with the limitation that not all repositories will be searched) but will require a `repo:` filter to be added when making any changes to it.
  - A `patternType` filter is no longer required. `patternType:literal` will be added to a code monitor query if not specified.
  - Added a new checklist UI to make it more intuitive to create code monitor trigger queries.
- Deprecated the GraphQL `icon` field on `GenericSearchResultInterface`. It will be removed in a future release. [#20028](https://github.com/sourcegraph/sourcegraph/pull/20028/files)
- Creating changesets through Batch Changes as a site-admin without configured Batch Changes credentials has been deprecated. Please configure user or global credentials before Sourcegraph 3.29 to not experience any interruptions in changeset creation. [#20143](https://github.com/sourcegraph/sourcegraph/pull/20143)
- Deprecated the GraphQL `limitHit` field on `LineMatch`. It will be removed in a future release. [#20164](https://github.com/sourcegraph/sourcegraph/pull/20164)

### Fixed

- A regression caused by search onboarding tour logic to never focus input in the search bar on the homepage. Input now focuses on the homepage if the search tour isn't in effect. [#19678](https://github.com/sourcegraph/sourcegraph/pull/19678)
- New changes of a Perforce depot will now be reflected in `master` branch after the initial clone. [#19718](https://github.com/sourcegraph/sourcegraph/pull/19718)
- Gitolite and Other type code host connection configuration can be correctly displayed. [#19976](https://github.com/sourcegraph/sourcegraph/pull/19976)
- Fixed a regression that caused user and code host limits to be ignored. [#20089](https://github.com/sourcegraph/sourcegraph/pull/20089)
- A regression where incorrect query highlighting happens for certain quoted values. [#20110](https://github.com/sourcegraph/sourcegraph/pull/20110)
- We now respect the `disableAutoGitUpdates` setting when cloning or fetching repos on demand and during cleanup tasks that may re-clone old repos. [#20194](https://github.com/sourcegraph/sourcegraph/pull/20194)

## 3.26.3

### Fixed

- Setting `gitMaxCodehostRequestsPerSecond` to `0` now actually blocks all Git operations happening on the gitserver. [#19716](https://github.com/sourcegraph/sourcegraph/pull/19716)

## 3.26.2

### Fixed

- Our indexed search logic now correctly handles de-duplication of search results across multiple replicas. [#19743](https://github.com/sourcegraph/sourcegraph/pull/19743)

## 3.26.1

### Added

- Experimental: Sync permissions of Perforce depots through the Sourcegraph UI. To enable, use the feature flag `"experimentalFeatures": { "perforce": "enabled" }`. For more information, see [how to enable permissions for your Perforce depots](https://docs.sourcegraph.com/admin/repo/perforce). [#16705](https://github.com/sourcegraph/sourcegraph/issues/16705)
- Added support for user email headers in the HTTP auth proxy. See [HTTP Auth Proxy docs](https://docs.sourcegraph.com/admin/auth#http-authentication-proxies) for more information.
- Ignore locked and disabled GitHub Enterprise repositories. [#19500](https://github.com/sourcegraph/sourcegraph/pull/19500)
- Remote code host git operations (such as `clone` or `ls-remote`) can now be rate limited beyond concurrency (which was already possible with `gitMaxConcurrentClones`). Set `gitMaxCodehostRequestsPerSecond` in site config to control the maximum rate of these operations per git-server instance. [#19504](https://github.com/sourcegraph/sourcegraph/pull/19504)

### Changed

-

### Fixed

- Commit search returning duplicate commits. [#19460](https://github.com/sourcegraph/sourcegraph/pull/19460)
- Clicking the Code Monitoring tab tries to take users to a non-existent repo. [#19525](https://github.com/sourcegraph/sourcegraph/pull/19525)
- Diff and commit search not highlighting search terms correctly for some files. [#19543](https://github.com/sourcegraph/sourcegraph/pull/19543), [#19639](https://github.com/sourcegraph/sourcegraph/pull/19639)
- File actions weren't appearing on large window sizes in Firefox and Safari. [#19380](https://github.com/sourcegraph/sourcegraph/pull/19380)

### Removed

-

## 3.26.0

### Added

- Searches are streamed into Sourcegraph by default. [#19300](https://github.com/sourcegraph/sourcegraph/pull/19300)
  - This gives a faster time to first result.
  - Several heuristics around result limits have been improved. You should see more consistent result counts now.
  - Can be disabled with the setting `experimentalFeatures.streamingSearch`.
- Opsgenie API keys can now be added via an environment variable. [#18662](https://github.com/sourcegraph/sourcegraph/pull/18662)
- It's now possible to control where code insights are displayed through the boolean settings `insights.displayLocation.homepage`, `insights.displayLocation.insightsPage` and `insights.displayLocation.directory`. [#18979](https://github.com/sourcegraph/sourcegraph/pull/18979)
- Users can now create changesets in batch changes on repositories that are cloned using SSH. [#16888](https://github.com/sourcegraph/sourcegraph/issues/16888)
- Syntax highlighting for Elixir, Elm, REG, Julia, Move, Nix, Puppet, VimL, Coq. [#19282](https://github.com/sourcegraph/sourcegraph/pull/19282)
- `BUILD.in` files are now highlighted as Bazel/Starlark build files. Thanks to @jjwon0 [#19282](https://github.com/sourcegraph/sourcegraph/pull/19282)
- `*.pyst` and `*.pyst-include` are now highlighted as Python files. Thanks to @jjwon0 [#19282](https://github.com/sourcegraph/sourcegraph/pull/19282)
- The code monitoring feature flag is now enabled by default. [#19295](https://github.com/sourcegraph/sourcegraph/pull/19295)
- New query field `select` enables returning only results of the desired type. See [documentation](https://docs.sourcegraph.com/code_search/reference/language#select) for details. [#19236](https://github.com/sourcegraph/sourcegraph/pull/19236)
- Syntax highlighting for Elixer, Elm, REG, Julia, Move, Nix, Puppet, VimL thanks to @rvantonder
- `BUILD.in` files are now highlighted as Bazel/Starlark build files. Thanks to @jjwon0
- `*.pyst` and `*.pyst-include` are now highlighted as Python files. Thanks to @jjwon0
- Added a `search.defaultCaseSensitive` setting to configure whether query patterns should be treated case sensitivitely by default.

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
- The `repo.cloned` column is deprecated in favour of `gitserver_repos.clone_status`. It will be removed in a subsequent release.
- Precision class indicators have been improved for code intelligence results in both the hover overlay as well as the definition and references locations panel. [#18843](https://github.com/sourcegraph/sourcegraph/pull/18843)
- Pings now contain added, aggregated campaigns usage data: aggregate counts of unique monthly users and Weekly campaign and changesets counts for campaign cohorts created in the last 12 months. [#18604](https://github.com/sourcegraph/sourcegraph/pull/18604)

### Fixed

- Auto complete suggestions for repositories and files containing spaces will now be automatically escaped when accepting the suggestion. [#18635](https://github.com/sourcegraph/sourcegraph/issues/18635)
- An issue causing repository results containing spaces to not be clickable in some cases. [#18668](https://github.com/sourcegraph/sourcegraph/pull/18668)
- Closing a batch change now correctly closes the entailed changesets, when requested by the user. [#18957](https://github.com/sourcegraph/sourcegraph/pull/18957)
- TypesScript highlighting bug. [#15930](https://github.com/sourcegraph/sourcegraph/issues/15930)
- The number of shards is now reported accurately in Site Admin > Repository Status > Settings > Indexing. [#19265](https://github.com/sourcegraph/sourcegraph/pull/19265)

### Removed

- Removed the deprecated GraphQL fields `SearchResults.repositoriesSearched` and `SearchResults.indexedRepositoriesSearched`.
- Removed the deprecated search field `max`
- Removed the `experimentalFeatures.showBadgeAttachments` setting

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
- Campaigns users creating Personal Access Tokens on GitHub are now asked to request the `user:email` scope in addition to the [previous scopes](https://docs.sourcegraph.com/@3.24/admin/code_hosts/github#github-api-token-and-access). This will be used in a future Sourcegraph release to display more fine-grained information on the progress of pull requests. [#17555](https://github.com/sourcegraph/sourcegraph/issues/17555)

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
- The Sourcegraph CLI can now serve local repositories for Sourcegraph to clone. This was previously in a command called `src-expose`. See [serving local repositories](https://docs.sourcegraph.com/admin/code_hosts/src_serve_git) in our documentation to find out more. [#12363](https://github.com/sourcegraph/sourcegraph/issues/12363)
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
- The "Add repositories" page (/site-admin/external-services/new) now displays a dismissible notification explaining how and why we access code host data. [#11789](https://github.com/sourcegraph/sourcegraph/pull/11789).
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
- `gitolite.exclude` setting in [Gitolite external service config](https://docs.sourcegraph.com/admin/code_hosts/gitolite#configuration) now supports a regular expression via the `pattern` field. This is consistent with how we exclude in other external services. Additionally this is a replacement for the deprecated `blacklist` configuration. [#11403](https://github.com/sourcegraph/sourcegraph/pull/11403)
- Notifications about Sourcegraph being out of date will now be shown to site admins and users (depending on how out-of-date it is).
- Alerts are now configured using `observability.alerts` in the site configuration, instead of via the Grafana web UI. This does not yet support all Grafana notification channel types, and is not yet supported on `sourcegraph/server` ([#11473](https://github.com/sourcegraph/sourcegraph/issues/11473)). For more details, please refer to the [Sourcegraph alerting guide](https://docs.sourcegraph.com/admin/observability/alerting).
- Experimental basic support for detecting if your Sourcegraph instance is over or under-provisioned has been added through a set of dashboards and warning-level alerts based on container utilization.
- Query [operators](https://docs.sourcegraph.com/code_search/reference/queries#boolean-operators) `and` and `or` are now enabled by default in all search modes for searching file content. [#11521](https://github.com/sourcegraph/sourcegraph/pull/11521)

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
- When more than 200+ repository revisions in a search are unindexed (very rare), the remaining repositories are reported as missing instead of Sourcegraph issuing e.g. several thousand unindexed search requests which causes system slowness and ultimately times out—ensuring searches are still fast even if there are indexing issues on a deployment of Sourcegraph. This does not apply if `index:no` is present in the query.

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
- After completing the Sourcegraph user feedback survey, a button may appear for tweeting this feedback at [@sourcegraph](https://twitter.com/sourcegraph). [#9728](https://github.com/sourcegraph/sourcegraph/pull/9728)
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
- Experimental: A tool, [src-expose](https://docs.sourcegraph.com/admin/code_hosts/other#experimental-src-expose), can be used to import code from any code host.
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

- Fixed an issue in `sourcegraph/*` Docker images where data folders were either not created or had incorrect permissions—preventing the use of Docker volumes. [#7991](https://github.com/sourcegraph/sourcegraph/pull/7991)

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

**Important:** If you use `SITE_CONFIG_FILE` or `CRITICAL_CONFIG_FILE`, please be sure to follow the steps in: [migration notes for Sourcegraph v3.11+](https://docs.sourcegraph.com/admin/migration/3_11.md) after upgrading.

### Added

- Language statistics by commit are available via the API. [#6737](https://github.com/sourcegraph/sourcegraph/pull/6737)
- Added a new page that shows [language statistics for the results of a search query](https://docs.sourcegraph.com/user/search#statistics).
- Global settings can be configured from a local file using the environment variable `GLOBAL_SETTINGS_FILE`.
- High-level health metrics and dashboards have been added to Sourcegraph's monitoring (found under the **Site admin** -> **Monitoring** area). [#7216](https://github.com/sourcegraph/sourcegraph/pull/7216)
- Logging for GraphQL API requests not issued by Sourcegraph is now much more verbose, allowing for easier debugging of problematic queries and where they originate from. [#5706](https://github.com/sourcegraph/sourcegraph/issues/5706)
- A new campaign type finds and removes leaked npm credentials. [#6893](https://github.com/sourcegraph/sourcegraph/pull/6893)
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

- The management console has been removed. All critical configuration previously stored in the management console will be automatically migrated to your site configuration. For more information about this change, or if you use `SITE_CONFIG_FILE` / `CRITICAL_CONFIG_FILE`, please see the [migration notes for Sourcegraph v3.11+](https://docs.sourcegraph.com/admin/migration/3_11.md).

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
- Added support for GitHub organization webhooks to enable faster updates of metadata used by [campaigns](https://about.sourcegraph.com/product/code-change-management/), such as pull requests or issue comments. See the [GitHub webhook documentation](https://docs.sourcegraph.com/admin/code_hosts/github#webhooks) for instructions on how to enable webhooks.
- Added support for GitHub organization webhooks to enable faster updates of changeset metadata used by campaigns. See the [GitHub webhook documentation](https://docs.sourcegraph.com/admin/code_hosts/github#webhooks) for instructions on how to enable webhooks.
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

- A `hardTTL` setting was added to the [Bitbucket Server `authorization` config](https://docs.sourcegraph.com/admin/code_hosts/bitbucketserver#configuration). This setting specifies a duration after which a user's cached permissions must be updated before any user action is authorized. This contrasts with the already existing `ttl` setting which defines a duration after which a user's cached permissions will get updated in the background, but the previously cached (and now stale) permissions are used to authorize any user action occuring before the update concludes. If your previous `ttl` value is larger than the default of the new `hardTTL` setting (i.e. **3 days**), you must change the `ttl` to be smaller or, `hardTTL` to be larger.

### Fixed

### Removed

- The `statusIndicator` feature flag has been removed from the site configuration's `experimentalFeatures` section. The status indicator has been enabled by default since 3.6.0 and you can now safely remove the feature flag from your configuration.
- Public usage is now only available on Sourcegraph.com. Because many core features rely on persisted user settings, anonymous usage leads to a degraded experience for most users. As a result, for self-hosted private instances it is preferable for all users to have accounts. But on sourcegraph.com, users will continue to have to opt-in to accounts, despite the degraded UX.

## 3.7.2

### Added

- A [migration guide for Sourcegraph v3.7+](https://docs.sourcegraph.com/admin/migration/3_7.md).

### Fixed

- Fixed an issue where some repositories with very long symbol names would fail to index after v3.7.
- We now retain one prior search index version after an upgrade, meaning upgrading AND downgrading from v3.6.2 <-> v3.7.2 is now 100% seamless and involves no downtime or negated search performance while repositories reindex. Please refer to the [v3.7+ migration guide](https://docs.sourcegraph.com/admin/migration/3_7.md) for details.

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

- In the [GitHub external service config](https://docs.sourcegraph.com/admin/code_hosts/github#configuration) it's now possible to specify `orgs` without specifying `repositoryQuery` or `repos` too.
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

- The `github.exclude` setting in [GitHub external service config](https://docs.sourcegraph.com/admin/code_hosts/github#configuration) additionally allows you to specify regular expressions with `{"pattern": "regex"}`.
- A new [`quicklinks` setting](https://docs.sourcegraph.com/user/personalization/quick_links) allows adding links to be displayed on the homepage and search page for all users (or users in an organization).
- Compatibility with the [Sourcegraph for Bitbucket Server](https://github.com/sourcegraph/bitbucket-server-plugin) plugin.
- Support for [Bitbucket Cloud](https://bitbucket.org) as an external service.

### Changed

- Updating or creating an external service will no longer block until the service is synced.
- The GraphQL fields `Repository.createdAt` and `Repository.updatedAt` are deprecated and will be removed in 3.8. Now `createdAt` is always the current time and updatedAt is always null.
- In the [GitHub external service config](https://docs.sourcegraph.com/admin/code_hosts/github#configuration) and [Bitbucket Server external service config](https://docs.sourcegraph.com/admin/code_hosts/bitbucket_server#permissions) `repositoryQuery` is now only required if `repos` is not set.
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
- The `orgs` setting in [GitHub external service config](https://docs.sourcegraph.com/admin/code_hosts/github) allows admins to select all repositories from the specified organizations to be synced.
- A new experimental search filter `repohascommitafter:"30 days ago"` allows users to exclude stale repositories that don't contain commits (to the branch being searched over) past a specified date from their search query.
- The `authorization` setting in the [Bitbucket Server external service config](https://docs.sourcegraph.com/admin/code_hosts/bitbucket_server#permissions) enables Sourcegraph to enforce the repository permissions defined in Bitbucket Server.
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
- The new `gitolite.exclude` setting in [Gitolite external service config](https://docs.sourcegraph.com/admin/code_hosts/gitolite#configuration) allows you to exclude specific repositories by their Gitolite name so that they won't be mirrored. Upon upgrading, previously "disabled" repositories will be automatically migrated to this exclusion list.
- The new `aws_codecommit.exclude` setting in [AWS CodeCommit external service config](https://docs.sourcegraph.com/admin/code_hosts/aws_codecommit#configuration) allows you to exclude specific repositories by their AWS name or ID so that they won't be synced. Upon upgrading, previously "disabled" repositories will be automatically migrated to this exclusion list.
- Added a new, _required_ `aws_codecommit.gitCredentials` setting to the [AWS CodeCommit external service config](https://docs.sourcegraph.com/admin/code_hosts/aws_codecommit#configuration). These Git credentials are required to create long-lived authenticated clone URLs for AWS CodeCommit repositories. For more information about Git credentials, see the AWS CodeCommit documentation: https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_ssh-keys.html#git-credentials-code-commit. For detailed instructions on how to create the credentials in IAM, see this page: https://docs.aws.amazon.com/codecommit/latest/userguide/setting-up-gc.html
- Added support for specifying a URL formatted `gitolite.host` setting in [Gitolite external service config](https://docs.sourcegraph.com/admin/code_hosts/gitolite#configuration) (e.g. `ssh://git@gitolite.example.org:2222/`), in addition to the already supported SCP like format (e.g `git@gitolite.example.org`)
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

- The `bitbucketserver.exclude` setting in [Bitbucket Server external service config](https://docs.sourcegraph.com/admin/code_hosts/bitbucketserver#configuration) additionally allows you to exclude repositories matched by a regular expression (so that they won't be synced).

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
- The required `bitbucketserver.username` field of a [Bitbucket Server external service configuration](https://docs.sourcegraph.com/admin/code_hosts/bitbucketserver#configuration), if unset or empty, is automatically migrated to match the user part of the `url` (if defined). https://github.com/sourcegraph/sourcegraph/issues/3592
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
- The new `gitlab.exclude` setting in [GitLab external service config](https://docs.sourcegraph.com/admin/code_hosts/gitlab#configuration) allows you to exclude specific repositories matched by `gitlab.projectQuery` and `gitlab.projects` (so that they won't be synced). Upon upgrading, previously "disabled" repositories will be automatically migrated to this exclusion list.
- The new `gitlab.projects` setting in [GitLab external service config](https://docs.sourcegraph.com/admin/code_hosts/gitlab#configuration) allows you to select specific repositories to be synced.
- The new `bitbucketserver.exclude` setting in [Bitbucket Server external service config](https://docs.sourcegraph.com/admin/code_hosts/bitbucketserver#configuration) allows you to exclude specific repositories matched by `bitbucketserver.repositoryQuery` and `bitbucketserver.repos` (so that they won't be synced). Upon upgrading, previously "disabled" repositories will be automatically migrated to this exclusion list.
- The new `bitbucketserver.repos` setting in [Bitbucket Server external service config](https://docs.sourcegraph.com/admin/code_hosts/bitbucketserver#configuration) allows you to select specific repositories to be synced.
- The new required `bitbucketserver.repositoryQuery` setting in [Bitbucket Server external service configuration](https://docs.sourcegraph.com/admin/code_hosts/bitbucketserver#configuration) allows you to use Bitbucket API repository search queries to select repos to be synced. Existing configurations will be migrate to have it set to `["?visibility=public", "?visibility=private"]` which is equivalent to the previous implicit behaviour that this setting supersedes.
- "Quick configure" buttons for common actions have been added to the config editor for all external services.
- "Quick configure" buttons for common actions have been added to the management console.
- Site-admins now receive an alert every day for the seven days before their license key expires.
- The user menu (in global nav) now lists the user's organizations.
- All users on an instance now see a non-dismissible alert when when there's no license key in use and the limit of free user accounts is exceeded.
- All users will see a dismissible warning about limited search performance and accuracy on when using the sourcegraph/server Docker image with more than 100 repositories enabled.

### Changed

- Indexed searches that time out more consistently report a timeout instead of erroneously saying "No results."
- The symbols sidebar now only shows symbols defined in the current file or directory.
- The dynamic filters on search results pages will now display `lang:` instead of `file:` filters for language/file-extension filter suggestions.
- The default `github.repositoryQuery` of a [GitHub external service configuration](https://docs.sourcegraph.com/admin/code_hosts/github#configuration) has been changed to `["none"]`. Existing configurations that had this field unset will be migrated to have the previous default explicitly set (`["affiliated", "public"]`).
- The default `gitlab.projectQuery` of a [GitLab external service configuration](https://docs.sourcegraph.com/admin/code_hosts/gitlab#configuration) has been changed to `["none"]`. Existing configurations that had this field unset will be migrated to have the previous default explicitly set (`["?membership=true"]`).
- The default value of `maxReposToSearch` is now unlimited (was 500).
- The default `github.repositoryQuery` of a [GitHub external service configuration](https://docs.sourcegraph.com/admin/code_hosts/github#configuration) has been changed to `["none"]` and is now a required field. Existing configurations that had this field unset will be migrated to have the previous default explicitly set (`["affiliated", "public"]`).
- The default `gitlab.projectQuery` of a [GitLab external service configuration](https://docs.sourcegraph.com/admin/code_hosts/gitlab#configuration) has been changed to `["none"]` and is now a required field. Existing configurations that had this field unset will be migrated to have the previous default explicitly set (`["?membership=true"]`).
- The `bitbucketserver.username` field of a [Bitbucket Server external service configuration](https://docs.sourcegraph.com/admin/code_hosts/bitbucketserver#configuration) is now **required**. This field is necessary to authenticate with the Bitbucket Server API with either `password` or `token`.
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
- The `github.exclude` setting was added to the [GitHub external service config](https://docs.sourcegraph.com/admin/code_hosts/github#configuration) to allow excluding repositories yielded by `github.repos` or `github.repositoryQuery` from being synced.

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
- Added a management console environment variable to disable HTTPS, see [the docs](https://docs.sourcegraph.com/admin/management_console.md#can-i-disable-https-on-the-management-console) for more information.
- Added `auth.disableUsernameChanges` to critical configuration to prevent users from changing their usernames.
- Site admins can query a user by email address or username from the GraphQL API.
- Added a search query builder to the main search page. Click "Use search query builder" to open the query builder, which is a form with separate inputs for commonly used search keywords.

### Changed

- File match search results now show full repository name if there are results from mirrors on different code hosts (e.g. github.com/sourcegraph/sourcegraph and gitlab.com/sourcegraph/sourcegraph)
- Search queries now use "smart case" by default. Searches are case insensitive unless you use uppercase letters. To explicitly set the case, you can still use the `case` field (e.g. `case:yes`, `case:no`). To explicitly set smart case, use `case:auto`.

### Fixed

- Fixed an issue where the management console would improperly regenerate the TLS cert/key unless `CUSTOM_TLS=true` was set. See the documentation for [how to use your own TLS certificate with the management console](https://docs.sourcegraph.com/admin/management_console.md#how-can-i-use-my-own-tls-certificates-with-the-management-console).

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

See the changelog entries for 3.0.0 beta releases and our [3.0](https://docs.sourcegraph.com/admin/migration/3_0.md) upgrade guide if you are upgrading from 2.x.

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
- Admins can now see charts with daily, weekly, and monthly unique user counts by visiting the site-admin Analytics page.
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
  - You will need to use the new `docker run` command at https://docs.sourcegraph.com/#quick-install in order for this feature to be enabled. Otherwise, you will receive errors in the log about `/var/run/docker.sock` and things will work just as they did before. See https://docs.sourcegraph.com/extensions/language_servers for more information.
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
