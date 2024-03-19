# Developing Sourcegraph

<style>
.markdown-body h2 {
  margin-top: 2em;
}
.markdown-body ul {
  list-style:none;
  padding-left: 1em;
}
.markdown-body ul li {
  margin: 0.5em 0;
}
.markdown-body ul li:before {
  content: '';
  display: inline-block;
  height: 1.2em;
  width: 1em;
  background-size: contain;
  background-repeat: no-repeat;
  background-image: url(../batch_changes/file-icon.svg);
  margin-right: 0.5em;
  margin-bottom: -0.29em;
}
body.theme-dark .markdown-body ul li:before {
  filter: invert(50%);
}
</style>

<p class="subtitle">Documentation for <b>developers contributing to the Sourcegraph code base</b></p>

<div class="cta-group">
<a class="btn btn-primary" href="setup/quickstart">★ Quickstart: develop Sourcegraph on your machine</a>
<a class="btn" href="https://github.com/sourcegraph/sourcegraph">GitHub repository</a>
<a class="btn" href="https://github.com/sourcegraph/sourcegraph/issues">Issue Tracker</a>
</div>

## Setup

<p class="subtitle">Learn how to develop Sourcegraph on your machine.</p>

<div class="getting-started">
  <a href="setup/quickstart" class="btn" alt="Run through the Quickstart guide">
   <span>★ Quickstart</span>
   <br>
   Run through the <b>step by step guide</b> and get your local environment ready.
  </a>

  <a href="../dev/how-to" class="btn" alt="How-to guides">
   <span>How-to guides</span>
   <br>
  <b>Context specific</b> guides: debugging live code
  </a>

  <a href="setup/troubleshooting" class="btn" alt="Troubleshooting">
   <span>Troubleshooting</span>
   <br>
  Help for the <b>most common</b> problems.
  </a>
</div>

## Background information

Clarification and discussion about key concepts, architecture, and development stack.

### Architecture

- [Overview](background-information/architecture/index.md)
- [Introducing a new service](background-information/architecture/introducing_a_new_service.md)

### Development

- [`sg` - the Sourcegraph developer tool](background-information/sg/index.md)
- [Using Bazel](background-information/bazel/index.md)
  - [Bazel and Go code](background-information/bazel/go.md)
  - [Bazel and client code](background-information/bazel/web.md)
  - [Bazel and container images](background-information/bazel/containers.md)
  - [Bazel FAQ](background-information/bazel/faq.md)
  - [Writing a server integration test](background-information/bazel/server_integration_tests.md)
- [Developing the web clients](background-information/web/index.md)
  - [Developing the web app](background-information/web/web_app.md)
  - [Developing the code host integrations](background-information/web/code_host_integrations.md)
  - [Working with GraphQL](background-information/web/graphql.md)
  - [Wildcard Component Library](background-information/web/wildcard.md)
  - [Styling UI](background-information/web/styling.md)
  - [Accessibility](background-information/web/accessibility.md)
  - [Temporary settings](background-information/web/temporary_settings.md)
  - [Build process](background-information/web/build.md)
- [Developing the GraphQL API](background-information/graphql_api.md)
- [Developing the SCIM API](background-information/scim_api.md)
- [Developing batch changes](background-information/batch_changes/index.md)
- [Developing code intelligence](background-information/codeintel/index.md)
- [Developing code insights](background-information/insights/index.md)
- [Developing code monitoring](background-information/codemonitoring/index.md)
- [Developing observability](background-information/observability/index.md)
- [Dependencies and generated code](background-information/dependencies_and_codegen.md)
- [Code reviews](background-information/pull_request_reviews.md)
- [Commit messages](background-information/commit_messages.md)
- [Exposing services](background-information/exposing-services.md)
- [Developing a store](background-information/basestore.md)
- [Developing a worker](background-information/workers.md)
- [Developing an out-of-band migration](background-information/oobmigrations.md)
- [Developing a background routine](background-information/backgroundroutine.md)
- [Testing](#testing)
  - [Testing principles and guidelines](background-information/testing_principles.md)
  - [Continuous integration](background-information/ci/index.md)
  - [How to write and run tests](how-to/testing.md)
    - [Testing Go code](background-information/languages/testing_go_code.md)
    - [Testing web code](background-information/testing_web_code.md)
- [Building p4-fusion](background-information/build_p4_fusion.md)
- [The `gitserver` API](background-information/gitserver-api.md)
- [Using gRPC alongside REST for internal APIs](background-information/gRPC_internal_api.md)

### Git

- [`git gc` and its modes of operations in Sourcegraph](background-information/git_gc.md)

### Languages

- [Tech stack](background-information/tech_stack.md)
- [Go](background-information/languages/go.md)
  - [Error handling in Go](background-information/languages/go_errors.md)
- [TypeScript](background-information/languages/typescript.md)
- [Bash](background-information/languages/bash.md)
- [Terraform](background-information/languages/terraform.md)

### SQL

- [Using PostgreSQL](background-information/postgresql.md)
- [Migrations overview](background-information/sql/migrations_overview.md)
- [Migrations](background-information/sql/migrations.md)
- High-performance guides
  - [Batch operations](background-information/sql/batch_operations.md)
  - [Materialized cache](background-information/sql/materialized_cache.md)

### Security

- [Security patterns](background-information/security_patterns.md)
- [Security policy](https://sourcegraph.com/security/)
- [How to disclose vulnerabilities](https://sourcegraph.com/handbook/engineering/security/reporting-vulnerabilities).
- [CSRF security model](security/csrf_security_model.md)
- [Sourcegraph secret formats](security/secret_formats.md)

### Testing

- [Continuous integration](background-information/ci/index.md)
- [Testing a pull request](background-information/testing_pr.md)
- [Testing principles](background-information/testing_principles.md)
- [Testing Go code](background-information/languages/testing_go_code.md)
- [Testing web code](background-information/testing_web_code.md)
- [Code host test plans](background-information/test-plans/code-hosts/index.md)

### Tools

- [Renovate dependency updates](background-information/renovate.md)
- [Honeycomb](background-information/honeycomb.md)
- [GoLand](background-information/goland.md)
- [Wolfi overview](background-information/wolfi/index.md)

### Other

- [Telemetry](background-information/telemetry/index.md)
- [Adding, changing and debugging pings](background-information/adding_ping_data.md)
- [DEPRECATED: Event level data usage pipeline](background-information/data-usage-pipeline.md)
- [DEPRECATED: Adding, changing and debugging user event data](background-information/adding_event_level_data.md)
- [Deploy Sourcegraph with Helm chart (BETA)](../../../admin/deploy/kubernetes/helm.md)
- [GitHub API oddities](background-information/github-api-oddities.md)

## How-to guides

Guides to help with troubleshooting, configuring test instances, debugging, and more.

### New features

- [How to add support for a language](how-to/add_support_for_a_language.md)
- [How to use feature flags](how-to/use_feature_flags.md)
- [How to add caching](how-to/add_caching.md)

### Observability

- [How to add observability](how-to/add_observability.md)
- [How to add logging](how-to/add_logging.md)
- [How to find monitoring](how-to/find_monitoring.md)
- [How to add monitoring](how-to/add_monitoring.md)
- [Set up local monitoring development](how-to/monitoring_local_dev.md)
- [Set up local OpenTelemetry development](how-to/opentelemetry_local_dev.md)

### Plans and licenses

- [How to generate a license key for testing and debugging](how-to/generate_license_key_for_testing.md)

### Documentation

- [Developing the product documentation](how-to/documentation_implementation.md)

### Executors

- [How to deploy a new executor image](how-to/deploy_executor_image.md)

### Testing

- [How to write and run tests](how-to/testing.md)
- [Run a local Sourcegraph instance behind ngrok](how-to/sourcegraph_ngrok.md)
- Testing against code hosts
  - [Configure a test instance of Phabricator and Gitolite](how-to/configure_phabricator_gitolite.md)
  - [Test a Phabricator and Gitolite instance](how-to/test_phabricator.md)
  - [Sync repositories from gitolite.sgdev.org](how-to/sync_repositories_from_gitolite_sgdev_org.md)

## Contributing

- [Open source FAQ](https://handbook.sourcegraph.com/company-info-and-process/community/faq/)
- [Code of conduct](https://handbook.sourcegraph.com/company-info-and-process/community/code_of_conduct/)
- [Project setup and CI checks for frontend issues](./contributing/frontend_contribution.md)
- [Accepting an external contribution guide](./contributing/accepting_contribution.md)
