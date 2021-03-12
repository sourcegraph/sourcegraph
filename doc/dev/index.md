# Developing Sourcegraph

This documentation is for developers contributing to the Sourcegraph code base.

Sourcegraph development is open source at:

- [GitHub repository](https://github.com/sourcegraph/sourcegraph)
- [Issue tracker](https://github.com/sourcegraph/sourcegraph/issues)

## [Getting started](getting-started/index.md)

A hands-on introduction for setting up your local development environment.

## Guides & background information

- [Tech stack](background-information/tech_stack.md)
- [Security Patterns](background-information/security_patterns.md)

### [Architecture](architecture/index.md)

- [Overview](background-information/architecture/index.md)
- [Introducing a new service](background-information/architecture/introducing_a_new_service.md)

### Local development

- [How to debug live code](how-to/debug_live_code.md)
- [Set up local development with Zoekt and Sourcegraph](how-to/zoekt_local_dev.md)
- [Ignoring editor config files in Git](how-to/ignoring_editor_config_files.md)
- [Use `golangci-lint`](how-to/use-golangci-lint.md)

### Development

- [How to add support for a language](how-to/add_support_for_a_language.md)
- [Developing the web clients](background-information/web/index.md)
  - [Developing the web app](background-information/web/web_app.md)
  - [Developing the code host integrations](background-information/web/code_host_integrations.md)
  - [Styling UI](background-information/web/styling.md)
- [Developing the GraphQL API](background-information/graphql_api.md)
- [Developing batch changes](background-information/batch_changes/index.md)
- [Developing code intelligence](background-information/codeintel/index.md)
- [Developing code monitoring](background-information/codemonitoring/index.md)
- [Developing observability](background-information/observability/index.md)
- [Developing Sourcegraph extensions](background-information/sourcegraph_extensions.md)
- [Dependencies and generated code](background-information/dependencies_and_codegen.md)
- [Code reviews](background-information/code_reviews.md)
- [Commit messages](background-information/commit_messages.md)
- [Exposing services](background-information/exposing-services.md)
- [Developing a store](background-information/basestore.md)
- [Developing a worker](background-information/workers.md)
- [Developing an out-of-band migration](background-information/oobmigrations.md)
- [Developing a background routine](background-information/backgroundroutine.md)
- [Developing the product documentation](how-to/documentation_implementation.md)
- [Observability](background-information/observability/index.md)
  - [How to find monitoring](how-to/find_monitoring.md)
  - [How to add monitoring](how-to/add_monitoring.md)
  - [Set up local Sourcegraph monitoring development](how-to/monitoring_local_dev.md)

### [Languages](languages/index.md)

- [Go](background-information/languages/go.md)
  - [Testing Go code](background-information/languages/testing_go_code.md).
- [TypeScript](background-information/languages/typescript.md)
- [Bash](background-information/languages/bash.md)
- [Terraform](background-information/languages/terraform.md)

#### [Extended guides](languages/extended_guide/index.md)

- [Terraform Extended Guide](background-information/languages/extended_guide/terraform.md)

### Testing

- [Continuous Integration](background-information/continuous_integration.md)
- [How to run tests](background-information/testing.md)
- [Testing Principles](background-information/testing_principles.md)
- [Testing web code](background-information/testing_web_code.md)
- [Configure a test instance of Phabricator and Gitolite](how-to/configure_phabricator_gitolite.md)
- [Test a Phabricator and Gitolite instance](how-to/test_phabricator.md)
- [Adding or changing Buildkite secrets](how-to/adding_buildkite_secrets.md)

### Tools

- [Renovate dependency updates](background-information/renovate.md)
- [Honeycomb](background-information/honeycomb.md)
- [Using PostgreSQL](background-information/postgresql.md)

### Other

- [Telemetry](background-information/telemetry.md)
- [Adding, changing and debugging pings](background-information/adding_ping_data.md)

### [Troubleshooting](troubleshooting_local_development.md)

- [Problems with node_modules or Javascript packages](how-to/troubleshooting_local_development.md#problems-with-nodemodules-or-javascript-packages)
- [dial tcp 127.0.0.1:3090: connect: connection refused](how-to/troubleshooting_local_development.md#dial-tcp-1270013090-connect-connection-refused)
- [Database migration failures](how-to/troubleshooting_local_development.md#database-migration-failures)
- [Internal Server Error](how-to/troubleshooting_local_development.md#internal-server-error)
- [Increase maximum available file descriptors.](how-to/troubleshooting_local_development.md#increase-maximum-available-file-descriptors)
- [Caddy 2 certificate problems](how-to/troubleshooting_local_development.md#caddy-2-certificate-problems)
- [Running out of disk space](how-to/troubleshooting_local_development.md#running-out-of-disk-space)
- [Certificate expiry](how-to/troubleshooting_local_development.md#certificate-expiry)
- [CPU/RAM/bandwidth/battery usage](how-to/troubleshooting_local_development.md#cpurambandwidthbattery-usage)

## Guidelines

- [Code reviews](https://about.sourcegraph.com/handbook/engineering/code_reviews)
- [Open source FAQ](https://about.sourcegraph.com/community/faq)
- [Code of conduct](https://about.sourcegraph.com/community/code_of_conduct)
