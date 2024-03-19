# Configuring Sourcegraph

This page documents how to configure a Sourcegraph instance. For deployment configuration, please refer to the [relevant deployment docs for your deployment type](../deploy/index.md#deployment-types).

- [Site configuration](site_config.md)
- [Global and user settings](settings.md)
- [Code host configuration](../external_service/index.md) (GitHub, GitLab, and the [Nginx HTTP server](../http_https_configuration.md).)
- [Search configuration](../search.md)
- [Configuring Authorization and Authentication](./authorization_and_authentication.md)
- [Batch Changes configuration](batch_changes.md)

## Common tasks

- [Add Git repositories from your code host](../repo/add.md)
- [Add user authentication providers (SSO)](../auth/index.md)
- [Configure search scopes](../../code_search/how-to/snippets.md)
- [Integrate with Phabricator](../../integration/phabricator.md)
- [Add organizations](../organizations.md)
- [Add teams](../teams/index.md) <span class="badge badge-experimental">Experimental</span>
- [Set up HTTPS](../http_https_configuration.md)
- [Use a custom domain](../url.md)
- [Configure email sending / SMTP server](email.md)
- [Update Sourcegraph](../updates/index.md)
- [Using external services (PostgreSQL, Redis, S3/GCS)](../external_services/index.md)
- [PostgreSQL Config](./postgres-conf.md)
- [Disabling user invitations](./user_invitations.md)
- [Configuring webhooks](./webhooks/index.md)
- [Configuring rate limits](../external_service/rate_limits.md)
- [Configuring command recording](../repo/recording.md)

## Advanced tasks

- [Loading configuration via the file system](advanced_config_file.md)
- [Restore postgres database from snapshot](restore/index.md)
- [Enabling database encryption for sensitive data](encryption.md)
