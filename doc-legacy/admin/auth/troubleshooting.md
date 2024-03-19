# Troubleshooting user authentication (SSO)

## Basic principles

As of 3.20, Sourcegraph supports 6 authentication methods as listed in our [User authentication (SSO)](https://docs.sourcegraph.com/admin/auth) page, details about how to configure those methods should refer to the relevant document pages.

Among these authentication methods, there are 4 that require Sourcegraph to send requests to the authentication provider, including GitHub OAuth, GitLab OAuth, OpenID Connect and SAML.

If a GitHub OAuth method is configured in Sourcegraph, users should be redirected to the GitHub instance (GitHub.com, GitHub Enterprise, etc.) to authorize the Sourcegraph OAuth application upon sign in. Once the user is authenticated on the GitHub instance and has authorized the application, the GitHub instance should redirect the user directly back to the Sourcegraph instance.

This redirection contains critical and confidential information that Sourcegraph consumes (as an OAuth consumer), and information may be stripped or expired if there is a third-party in the way.

Therefore, the **Callback URL/Single Sign On URL** is very important to get right on the authentication provider side. Here is an example of callback URL in a GitHub OAuth application settings:

![GitHub OAuth callback URL example](https://storage.googleapis.com/sourcegraph-assets/docs/images/auth/troubleshooting_callback_url_example.png)

For an OAuth application against GitHub.com (GitHub OAuth), the authentication flow should look like this (-> indicates a browser redirection):

```
   Sourcegraph (choose to sign in with GitHub.com)
-> GitHub.com (authorize the application)
-> Sourcegraph
```

### The authentication provider always redirects back to its direct consumer

GitHub OAuth, GitLab OAuth, OpenID Connect and SAML all require the admin to configure a **Callback URL/Single Sign On URL** on the authentication provider side, which means the redirection back to the Sourcegraph (the consumer) is critical to make the user authentication work (as explained in the last section).

In a complex authentication set up, there is a chain of authentication. For example, Sourcegraph syncs users from the GitHub instance via GitHub OAuth, and the GitHub instance in turn syncs users from a company central SSO (such as Okta) via SAML.

In such a setup, the Sourcegraph instance only talks to the GitHub instance, and is not aware of the existence of the central SSO. The central SSO, in turn, does not know that the Sourcegraph is syncing users from the central SSO (through GitHub).

Therefore, it makes no sense to have the central SSO redirect users directly back to Sourcegraph, which effectively bypasses the GitHub instance. The authentication provider should always redirect the user to its direct consumer. Thus, *the central SSO should only redirect users back to the GitHub instance, and the GitHub instance should redirect the user back to Sourcegraph with any information needed for the consumer.*

## Debugging playbook

### Enable oAuth log traces

Set the env var `INSECURE_OAUTH2_LOG_TRACES=1` to log all OAuth2 requests and responses on:

* [Docker Compose](../deploy/docker-compose/index.md) and [Kubernetes](../deploy/kubernetes/index.md): the `sourcegraph-frontend` deployment
* [Single-container](../deploy/docker-single-container/index.md): the `sourcegraph/server` container

### Make sure the client ID and client secret are actually correct

Invalid client ID and client secret pair does not prevent the user from completing the OAuth flow until a confusing error, which may mislead the user into thinking the client ID and client secret pair are correct.

### Make sure provider permissions are correct

If you are unable to use OAuth to login, perhaps after an upgrade, and receive the following error:

```
An error has occurred
The requested scope is invalid, unknown, or malformed.
```

This could be related to the scopes granted on your `clientID` and `clientSecret` on the `auth.providers` section in your site configuration.

For example, for the GitLab oAuth integratio, check the [GitLab scopes](https://gitlab.com/-/profile/applications) granted to ensure that you have the following configured:

* `api`
* `read_user`
* `read_api`

### Test in incognito mode

It is possible that a user's browser extension modifies or strips a HTTP header or cookie needed for the authentication. Ask the user to perform the test in incognito mode without any browser extensions. The incognito mode also helps ensure that the user is logged out from all involved parties.

### Preserve log in DevTools

To keep a record of redirection in a user's browser is very helpful to audit the auth flow. Here is how to do it in Chrome:

![Preserve log in Chrome](https://storage.googleapis.com/sourcegraph-assets/docs/images/auth/troubleshooting_preserve_log_in_chrome.png)

It's easier to spot problems if the user “clears logs” before running tests:

![Clear log in Chrome](https://storage.googleapis.com/sourcegraph-assets/docs/images/auth/troubleshooting_clear_log_in_chromejpg)

### Configure SAML in Okta

Things to note:

1. Okta has a weird UX that many options (e.g. SAML templates) are not available in the so-called **Developer Console** UI. Make sure to first change to the **Classic UI** on the top left.

    ![Okta classic UI](https://storage.googleapis.com/sourcegraph-assets/docs/images/auth/troubleshooting_okta_classic_ui.png)

2. The Okta docs must be opened directly from the application settings because of a special URL parameter `baseAdminUrl` used to display confidential information in the docs:

    ![Okta view instruction](https://storage.googleapis.com/sourcegraph-assets/docs/images/auth/troubleshooting_okta_view_instruction.png)
    ![Okta view instruction 2](https://storage.googleapis.com/sourcegraph-assets/docs/images/auth/troubleshooting_okta_view_instruction_2.png)

    Without the URL parameter `baseAdminUrl`, it displays a static warning:

    ![Okta static warning](https://storage.googleapis.com/sourcegraph-assets/docs/images/auth/troubleshooting_okta_static_warning.png)

### Things to collect for async analysis

- `"auth.providers"` portion of the Sourcegraph site configuration. Example:

    !["auth.providers" site config](https://storage.googleapis.com/sourcegraph-assets/docs/images/auth/troubleshooting_sourcegraph_auth_providers.png)

- A screenshot of the Sourcegraph OAuth application on the GitHub instance. Example:

    ![OAuth application settings](https://storage.googleapis.com/sourcegraph-assets/docs/images/auth/troubleshooting_oauth_application_settings.png)

- A screenshot of the GitHub authentication settings.
  - For GitHub Enterprise Server, it is in the management console's **Settings > Authentication**. Example:

        ![GitHub Enterprise Server SAML settings](https://storage.googleapis.com/sourcegraph-assets/docs/images/auth/troubleshooting_ghe_saml_settings.png)

  - For GitHub Enterprise Cloud, it is in the organization's **Settings > Organization security > SAML single sign-on**. Example:

        ![GitHub Enterprise Cloud SAML settings](https://storage.googleapis.com/sourcegraph-assets/docs/images/auth/troubleshooting_github_enterprise_cloud_saml_settings.png)

- Screenshots of the Okta application settings on both General and Sign On tabs, with all options from top to bottom.
- Full browser logs (https://toolbox.googleapps.com/apps/har_analyzer/)
