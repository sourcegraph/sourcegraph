# Administration

Site administrators are the admins responsible for deploying, managing, and configuring Sourcegraph for regular users. They have [special privileges](privileges.md) on the Sourcegraph instance.

## [Install Sourcegraph](install/index.md)

- [Install Sourcegraph with Docker](install/docker/index.md)
- [Install Sourcegraph with Docker Compose](install/docker-compose/index.md)
- [Install Sourcegraph with Kubernetes](install/kubernetes/index.md)
- [Install Sourcegraph on a cluster](install/index.md)
- [Set up a managed instance](install/managed.md)
- [Back up or migrate to a new Sourcegraph instance](install/migrate-backup.md)

## Management, deployment, and configuration

- [Configuration](config/index.md)
- [Adding Git repositories](repo/add.md) (from a code host or clone URL)
- [HTTP and HTTPS/SSL configuration](http_https_configuration.md)
  - [Adding SSL (HTTPS) to Sourcegraph with a self-signed certificate](ssl_https_self_signed_cert_nginx.md)
- [Monorepo](monorepo.md)
- [Repository webhooks](repo/webhooks.md)
- [User authentication](auth/index.md)
- [Upgrading Sourcegraph](updates.md)
- [Setting the URL for your instance](url.md)
- [Observability](observability.md)
- [Repository permissions](repo/permissions.md)
- [PostgreSQL configuration](postgres-conf.md)
- [Upgrading PostgreSQL](postgres.md)
- [Using external services (PostgreSQL, Redis, S3/GCS)](external_services/index.md)
- [User data deletion](user_data_deletion.md)
- [Validation](validation.md) **Experimental**

## Features

- [Code intelligence and language servers](../code_intelligence/index.md)
- [Sourcegraph extensions and extension registry](extensions/index.md)
- [Search](search.md)
- [Federation](federation/index.md)
- [Pings](pings.md)
- [Usage statistics](usage_statistics.md)
- [User feedback surveys](user_surveys.md)
- [Beta and prototype features](beta_and_prototype_features.md)

## Integrations

- [GitHub and GitHub Enterprise](../integration/github.md)
- [GitLab](../integration/gitlab.md)
- [Bitbucket Server](../integration/bitbucket_server.md)
- [AWS CodeCommit](../integration/aws_codecommit.md)
- [Phabricator](../integration/phabricator.md)
- [All integrations](../integration/index.md)

## Migration guides

- [From OpenGrok to Sourcegraph](migration/opengrok.md)
- [Migrating to Sourcegraph 3.0.1+](migration/3_0.md)
- [Migrating to Sourcegraph 3.7.2+](migration/3_7.md)
- [Pricing and subscriptions](subscriptions/index.md)
- [FAQ](faq.md)
