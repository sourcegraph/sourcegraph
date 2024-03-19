# How to guides

## Documentation

- [How to write great docs](https://documentation.divio.com/) (watch the video)
- [How to maintain the Tech Radar](maintain-tech-radar.md)

## New features

- [How to add support for a language](add_support_for_a_language.md)
- [Release Browser Extensions](releasing_browser_extensions.md)
- [How to implement the authentication process for an IDE extension](ide_auth_flow.md)

## Implementing Sourcegraph

- [Developing the product documentation](documentation_implementation.md)
- [Observability](../background-information/observability/index.md)
  - [How to add observability](add_observability.md)
  - [How to add logging](add_logging.md)
  - [How to find monitoring](find_monitoring.md)
  - [How to add monitoring](add_monitoring.md)
  - [How to enable continuous profiling in production](profiling_continuous.md)
- [GraphQL](../background-information/graphql_api.md)
  - [How to add a GraphQL query](../how-to/add_graphql_query.md)

## Local Environment

- [Set up local development with Zoekt and Sourcegraph](zoekt_local_dev.md)
- [How to debug live code](debug_live_code.md)
- [Ignoring editor config files in Git](ignoring_editor_config_files.md)

## Testing Sourcegraph & CI

- [How to run tests](testing.md)
   - See also [Testing Principles](../background-information/testing_principles.md) and [Continuous Integration](../background-information/ci/index.md)
- [Configure a test instance of Phabricator and Gitolite](configure_phabricator_gitolite.md)
- [How to test changes in dogfood](testing_in_dogfood.md)
- [How to receive a Slack notification if a specific CI step failed](receive_slack_notification_on_a_failed_ci_step.md)
- [How to allow a CI step to fail without breaking the build and still receive a notification](ci_soft_failure_and_still_notify.md)
- [Run a local Sourcegraph instance behind ngrok](sourcegraph_ngrok.md)
- [How to update the CI glossary](update_ci_glossary.md)
<!-- Commments break the rendering of list items, adding a blank line, so we put them here at the end -->
<!-- [Sync repositories from gitolite.sgdev.org](sync_repositories_from_gitolite_sgdev_org.md) -->
<!-- [Test a Phabricator and Gitolite instance](test_phabricator.md) -->


## Maintenance

- [How to update pnpm to a newer version](update_pnpm.md)


## Profiling

- [How to do one-off profiling for dogfood and production using pprof](profiling_one-off.md)

## Executors

- [How to deploy a new executor image](deploy_executor_image.md)
- [How to debug Firecracker executors](debug_executor.md)

## Access Control

- [Adding permissions](add_permissions.md)

## Wolfi

- [How to add and update Wolfi packages](wolfi/add_update_packages.md)
- [How to add and update Wolfi base images](wolfi/add_update_images.md)

## Cody Gateway

- [How to set up Cody Gateway locally](cody_gateway.md)

## Telemetry Gateway

- [How to set up Telemetry Gateway locally](telemetry_gateway.md)

## Windows support

Running Sourcegraph on Windows is not actively tested, but should be possible within the Windows Subsystem for Linux (WSL).
Sourcegraph currently relies on Unix specifics in several places, which makes it currently not possible to run Sourcegraph directly inside Windows without WSL.
We are happy to accept contributions here!
