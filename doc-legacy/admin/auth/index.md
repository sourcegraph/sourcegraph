# User authentication

Sourcegraph supports the following ways for users to sign in:

- [Builtin password authentication](#builtin-password-authentication)
- [GitHub](#github)
- [GitLab](#gitlab)
- [Bitbucket Cloud](#bitbucket-cloud)
- [Gerrit](#gerrit) <span class="badge badge-beta">Beta</span>
- [SAML](saml/index.md)
- [OpenID Connect](#openid-connect)
  - [Google Workspace (Google accounts)](#google-workspace-google-accounts)
- [HTTP authentication proxies](#http-authentication-proxies)
  - [Username header prefixes](#username-header-prefixes)
- [IP Allow List](#ip-allow-list)
- [Username normalization](#username-normalization)
- [Troubleshooting](#troubleshooting)

The authentication providers are configured in the [`auth.providers`](../config/site_config.md#authentication-providers) site configuration option.

## Login form configuration

To configure the presentation of the login form, see the [login form configuration page](./login_form.md).
## Recommendations

If you are unsure which auth provider is right for you, we recommend applying the following rules in
order:

- If you have no external identity providers (i.e., not SSO) or are just trying to spin Sourcegraph
  up as quickly as possible to try, use [`builtin`](#builtin-password-authentication) authentication. You can
  always change the auth configuration later, and user identities from external providers will be
  linked automatically to existing Sourcegraph accounts using verified email addresses.
- If you are deploying Sourcegraph behind a HTTP authentication proxy service, use the
  [`http-header`](#http-authentication-proxies) provider type. The proxy service should handle
  authentication and session management and, in turn, set a HTTP header that indicates the user
  identity to Sourcegraph.
- If you are configuring Sourcegraph to index a GitHub or GitLab instance, we recommend using the
  OAuth provider for that code host. This applies even if the code host itself uses an external
  identity provider (e.g., SAML, OpenID Connect, LDAP, etc.). Sourcegraph will redirect to your code
  host on sign-in and the code host will perform the required sign-in flow before redirecting to
  Sourcegraph on success.
- If you are using an identity provider that supports SAML, use the [SAML auth provider](saml/index.md).
- If you are using an identity provider that supports OpenID Connect (including Google accounts),
  use the [OpenID Connect provider](#openid-connect).
- If you wish to use LDAP and cannot use the GitHub/GitLab OAuth provider as described above, or if
  you wish to use another authentication mechanism that is not yet supported, please [contact
  us](https://github.com/sourcegraph/sourcegraph/issues/new?template=feature_request.md) (we respond
  promptly).

Most users will use only one auth provider, but you can use multiple auth providers if desired to
enable sign-in via multiple services. Identities from different providers will be mapped to a
Sourcegraph user by comparing the user's verified email address to the email address from the
external identity provider.

> NOTE: If OAuth is the only sign-in method available on sign-out, a new OAuth sign-in will be attempted immediately upon a redirect to the sign-in page. If it is necessary to sign-out and have persistent access to the sign-in page, enable `builtin` sign-in in addition to your OAuth sign-in.

## Builtin password authentication

The [`builtin` auth provider](../config/site_config.md#builtin-password-authentication) manages user accounts internally in its own database. It supports user signup, login, and password reset (via email if configured, or else via a site admin).

Password reset links expire after 4 hours by default - this can be configured in site configuration with the [`auth.passwordResetLinkExpiry`](../config/site_config.md#auth-passwordResetLinkExpiry) field.

### Creating builtin authentication users

Users can be created with builtin password authentication in several ways:

- through the site admin page `/site-admin/users/new`
- through users [signing up](#how-to-control-user-sign-up)
- through the `createUser` mutation in the GraphQL API
- through [`src users create`](../../cli/references/users/create.md)

When [SMTP is enabled](../config/email.md), special behaviours apply to whether a builtin authentication user's email is marked as verified by default - refer to [email verification](../config/email.md#user-email-verification) for more details.

### How to control user sign-up

You can use the filter `allowSignup`, available in the builtin configuration, to control who can create an account in your Sourcegraph instance.

**allowSignup**

  If set to `true`, users will see a sign-up link under the login form and will have access to the sign-up page, where they can create their accounts without restriction.

  When not set, it will default to `false` -- in this case, users will see request account link. Unauthenticated users can submit request account forms and admins will get notification on the instance which they can approve or reject. Account request feature can be disabled by setting `auth.accessRequest: {enabled: false}`, and in this case new user accounts should be created by the site admin manually.

  If you choose to block sign-ups by using the `allowSignup` filter available in another auth provider (eg., [GitHub](#how-to-control-user-sign-up-and-sign-in-with-github-auth-provider) or [GitLab](#how-to-control-user-sign-up-and-sign-in-with-gitlab-auth-provider)), make sure this builtin filter is removed or set to `false`. Otherwise, users will have a way to bypass the restriction.

  During the initial setup, the builtin sign-up will be available for the first user so they can create an account and become admin.

  ```json
  {
    // ...,
    "auth.providers": [{ "type": "builtin", "allowSignup": true }]
  }
```
> NOTE: If Sourcegraph is running on a free license all users will be created as site admins. Learn more about license settings on our [pricing page](https://sourcegraph.com/pricing).



### Account lockout

<span class="badge badge-note">Sourcegraph 3.39+</span>

Account will be locked out for 30 minutes after 5 consecutive failed sign-in attempts within one hour for the builtin authentication provider. The threshold and duration of lockout and consecutive periods can be customized via `"auth.lockout"` in the site configuration:

```json
{
  // ...
  "auth.lockout": {
    // The number of seconds to be considered as a consecutive period
    "consecutivePeriod": 3600,
    // The threshold of failed sign-in attempts in a consecutive period
    "failedAttemptThreshold": 5,
    // The number of seconds for the lockout period
    "lockoutPeriod": 1800
  }
}
```

To enabled self-serve account unlock through emails, add the following lines to your site configuration:

```json
{
  // Validity expressed in minutes of the unlock account token
  "auth.unlockAccountLinkExpiry": 30,
  // Base64-encoded HMAC signing key to sign the JWT token for account unlock URLs
  "auth.unlockAccountLinkSigningKey": "your-signing-key",
}
```

The `ssh-keygen` command can be used to generate and encode the signing key, for example:

```bash
$ ssh-keygen -t ed25519 -a 128 -f auth.unlockAccountLinkSigningKey
$ base64 auth.unlockAccountLinkSigningKey | tr -d '\n'
LS0tLS1CRUdJTiBPUEVOU1NIIFBSSVZBVEUgS0VZLS0tLS0KYjNCbGJu...
```

Copy the result of the `base64` command as the value of the `"auth.unlockAccountLinkSigningKey"`.

## GitHub

[Create a GitHub OAuth
application](https://developer.github.com/apps/building-oauth-apps/creating-an-oauth-app/) (if using
GitHub Enterprise, create one on your instance, not GitHub.com). Set the following values, replacing
`sourcegraph.example.com` with the IP or hostname of your Sourcegraph instance:

- Homepage URL: `https://sourcegraph.example.com`
- Authorization callback URL: `https://sourcegraph.example.com/.auth/github/callback`

> NOTE: If you want to enable repository permissions, you should grant your OAuth app permission to
> your GitHub organization(s). You can do that either by creating the OAuth app under your GitHub
> organization (rather than your personal account) or by [following these
> instructions](https://help.github.com/articles/approving-oauth-apps-for-your-organization/).

Then add the following lines to your site configuration:

```json
{
  // ...
  "auth.providers": [
    {
      "type": "github",
      "url": "https://github.example.com",  // URL of your GitHub instance; can leave empty for github.com
      "displayName": "GitHub",
      "clientID": "replace-with-the-oauth-client-id",
      "clientSecret": "replace-with-the-oauth-client-secret",
      "allowSignup": false,  // CAUTION: Set to true to enable signup. If nothing is specified in `allowOrgs` or `allowOrgsMap`, any GitHub user can sign up.
      "allowOrgs": ["your-org-name"], // Restrict logins and sign-ups if enabled to members of these orgs.
      "allowOrgsMap": { "orgName": ["your-team-name"]} // Restrict logins and sign-ups if enabled to members of teams that belong to a given org.
    }
  ]
}
```

Replace the `clientID` and `clientSecret` values with the values from your GitHub OAuth app
configuration.

Leave the `url` field empty for GitHub.com.

Once you've configured GitHub as a sign-on provider, you may also want to [add GitHub repositories to Sourcegraph](../external_service/github.md#repository-syncing).


### How to control user sign-up and sign-in with GitHub auth provider

You can use the following filters to control how users will create accounts and sign in to your Sourcegraph instance via the GitHub auth provider.

**allowSignup**

Set `allowSignup` to `true` to enable anyone with a GitHub account to sign up without invitation (typically done only for GitHub Enterprise).

If set to `false` or not set, sign-up will be blocked. In this case, new users will only be able to sign in after an admin creates their account on Sourcegraph.
The new user email, during their account creation, should match one of their GitHub verified emails.

> WARNING: If `allowSignup` is set to `true`, anyone with internet access to both your Sourcegraph instance and your GitHub url are able to sign up and login to your instance. In particular, if url is set to `https://github.com`, this means that anyone with a Github account could log in to your Sourcegraph instance and search your indexed code. Make sure to also configure the `allowOrgs` field described below to limit sign-ups to your org, or limit public access to your Sourcegraph instance via IP restrictions / VPN. For assistance, contact support.


```json
  {
    "type": "github",
    ...
    "allowSignup": false
  }
```

**allowOrgs**

Restricts sign-ins to members of the listed organizations. If empty or unset, no restriction will be applied.

If combined with `"allowSignup": true`, only members of the allowed orgs can create their accounts in Sourcegraph via GitHub authentitcation.

When combined with `"allowSignup": false` or unset, an admin should first create the user account so that the user can sign in with GitHub if they belong to the allowed orgs.

  ```json
    {
      "type": "github",
      // ...
      "allowSignup": true,
      "allowOrgs": ["org1", "org2"]
    },
  ```

**allowOrgsMap**

  Restricts sign-ups and new logins to members of the listed teams or subteams mapped to the organizations they belong to. If empty or unset, no restrictions will be applied.

  When combined with `"allowSignup": true`, only members of the allowed teams can create their accounts in Sourcegraph via GitHub authentication.

  If set with `"allowSignup": false` or if `allowSignup` is unset, an admin should first create the new users accounts so that they can login with GitHub.

  In case both `allowOrgs` and `allowOrgsMap` filters are configured, org membership (`allowOrgs`) will be checked first. Only if the user doesn't belong to any of the listed organizations then team membership (`allowOrgsMap`) will be checked.

  Note that subteams inheritance is not supported — the name of child teams (subteams) should be informed so their members can be granted access to Sourcegraph.

  ```json
    {
       "type": "github",
      // ...
      "allowOrgsMap": {
        "org1": [
          "team1", "subteam1"
        ],
        "org2": [
          "subteam2"
        ]
      }
    }
  ```


## GitLab

[Create a GitLab OAuth application](https://docs.gitlab.com/ee/integration/oauth_provider.html). Set the following values, replacing `sourcegraph.example.com` with the IP or hostname of your
Sourcegraph instance:

- Authorization callback URL: `https://sourcegraph.example.com/.auth/gitlab/callback`
- Scopes: `read_user`, `read_api` (be sure to set `"apiScope": "read_api"` in the `auth.providers` config, as indicated below)

Then add the following lines to your site configuration:

```json
{
    // ...
    "auth.providers": [
      {
        "type": "gitlab",
        "displayName": "GitLab",
        "clientID": "replace-with-the-oauth-application-id",
        "clientSecret": "replace-with-the-oauth-secret",
        "url": "https://gitlab.example.com",
        "apiScope": "read_api", // If not set, it defaults to "api" and the OAuth application will have to be adjusted accordingly.
        "allowSignup": false, // If not set, it defaults to true allowing any GitLab user with access to your instance to sign up.
        "allowGroups": ["group", "group/subgroup", "group/subgroup/subgroup"], // Restrict logins and sign-ups to members of groups or subgroups based on the full-path provided.
      }
    ]
```

Replace the `clientID` and `clientSecret` values with the values from your GitLab OAuth app
configuration.

Once you've configured GitLab as a sign-on provider, you may also want to [add GitLab repositories to Sourcegraph](../external_service/gitlab.md#repository-syncing).

> NOTE: Administrators on the GitLab instance who then sign in to Sourcegraph will not have access to all of the repositories on Sourcegraph as well. Administrators will only have access to repositories on GitLab for which they are assigned the Reporter role and above.

### How to control user sign-up and sign-in with GitLab auth provider

You can use the following filters to control how users can create accounts and sign in to your Sourcegraph instance via the GitLab auth provider.

**allowSignup**

  Allows anyone with a GitLab account to create their accounts.

  When `false`, sign-up with GitLab will be blocked. In this case, new users can only sign in after an admin creates their account on Sourcegraph. The user account email should match their primary emails on GitLab (which are always verified).

  *If not set, unliked with GitHub, it defaults to `true`, allowing any GitLab user with access to your instance to sign up*.


  ```json
    {
      "type": "gitlab",
      // ...
      "allowSignup": false
    }
  ```

**allowGroups**

  Restricts new sign-ins to members of the listed groups or subgroups.

  Instead of informing the groups or subgroups names, use their full path that can be copied from the URL.

  Example: for a parent group, the full path can be simple as `group`. For nested groups it can look like `group/subgroup/subsubgroup`.

  When empty or unset, no restrictions will be applied.

  If combined with `"allowSignup": false`, an admin should first create the user account so that the user can sign in with GitLab.

  If combined with `"allowSignup": true` or with `allowSignup` unset, only members of  the allowed groups or subgroups can create their accounts in Sourcegraph via GitLab authentitcation.

> WARNING: Users will require a minimum access level of `Guest` in at least one of the specified groups in order to gain access to Sourcegraph. GitLab offers a lower user permission level, [Minimal Access](https://docs.gitlab.com/ee/user/permissions.html#users-with-minimal-access), for Premium and Ultimate tier customers, which is often used to configure SAML SSO on GitLab. Sourcegraph does not currently support the `Minimal Access` access level, and users with this access level will not be allowed to sign in. In these cases it is recommended to create a subgroup and add all users that require access to Sourcegraph to that subgroup with a minimum access level of `Guest`, and then add that subgroup to the `allowGroups` list.

  ```json
    {
      "type": "gitlab",
      // ...
      "allowSignup": true,
      "allowGroups": [
        "group/subgroup/subsubgroup"
      ]
    }
  ```

### How to set up GitLab auth provider for use with GitLab group SAML/SSO

GitLab groups can require SAML/SSO sign-in to have access to the group. The regular OAuth sign-in won't work in this case, as users will be redirected to the normal GitLab sign-in page, requesting a username/password. In this scenario, add a `ssoURL` to your GitLab auth provider configuration:

  ```json
    {
      "type": "gitlab",
      // ...
      "ssoURL": "https://gitlab.com/groups/your-group/-/saml/sso?token=xxxxxxxx"
      ]
    }
  ```

The `token` parameter can be found on the **Settings > SAML SSO** page on GitLab.

### Don't sync user permissions for internal repositories

If your organization has a lot of internal repositories that should be accessible to everyone on GitLab, you may want to [mark internal repositories as public](../external_service/gitlab.md#internal-repositories), and then configure your auth provider to not sync user permissions for internal repositories:

  ```json
    {
      "type": "gitlab",
      // ...
      "syncInternalRepoPermissions": false
    }
  ```

## Bitbucket Cloud

[Create a Bitbucket Cloud OAuth consumer](https://support.atlassian.com/bitbucket-cloud/docs/use-oauth-on-bitbucket-cloud/). Set the following values, replacing `sourcegraph.example.com` with the IP or hostname of your
Sourcegraph instance:

- Callback URL: `https://sourcegraph.example.com/.auth/bitbucketcloud/callback`
- Permissions:
  - `Account`: `Read`
  - `Repositories`: `Read` (more information in [repository permissions section](../permissions/index.md))

After the consumer is created, you will need the `Key` and the `Secret`, which can be found by expanding OAuth consumer in the list.
Then add the following lines to your [site configuration](../config/site_config.md):

```json
{
    // ...
    "auth.providers": [
      {
        "type": "bitbucketcloud",
        "displayName": "Bitbucket Cloud",
        "clientKey": "replace-with-the-oauth-consumer-key",
        "clientSecret": "replace-with-the-oauth-consumer-secret",
        "allowSignup": false // If not set, it defaults to true allowing any Bitbucket Cloud user with access to your instance to sign up.
      }
    ]
```
Replace the `clientKey` and `clientSecret` values with the values from your Bitbucket Cloud OAuth consumer.

## Gerrit
<span class="badge badge-beta">Beta</span>

To enable users to add Gerrit credentials and verify their access to repositories on Sourcegraph,
add the following lines to your [site configuration](../config/site_config.md):

```json
{
    // ...
    "auth.providers": [
      {
        "type": "gerrit",
        "displayName": "Gerrit",
        "url": "https://example.gerrit.com" // Must match the URL of the code host connection for which authorization is required
      }
    ]
```

Users can then add Gerrit credentials by visiting their **Settings** > **Account security**.

## OpenID Connect

The [`openidconnect` auth provider](../config/site_config.md#openid-connect-including-google-workspace) authenticates users via OpenID Connect, which is supported by many external services, including:

- [Google Workspace (Google accounts)](#google-workspace-google-accounts)
- [Okta](https://developer.okta.com/docs/api/resources/oidc.html)
- [Ping Identity](https://www.pingidentity.com/en/resources/client-library/articles/openid-connect.html)
- [Auth0](https://auth0.com/docs/protocols/oidc)
- [Salesforce Identity](https://developer.salesforce.com/page/Inside_OpenID_Connect_on_Force.com)
- [Microsoft Azure Active Directory](https://docs.microsoft.com/en-us/azure/active-directory/develop/active-directory-protocols-openid-connect-code)
- [Google Identity Platform](https://developers.google.com/identity/)
- Known issue: [OneLogin](https://www.onelogin.com/openid-connect) OpenID Connect is not supported (use SAML for OneLogin instead)

To configure Sourcegraph to authenticate users via OpenID Connect:

1. Create a new OpenID Connect client in the external service (such as one of those listed above).
    - **Redirect/callback URI:** `https://sourcegraph.example.com/.auth/callback` (replace `https://sourcegraph.example.com` with the value of the `externalURL` property in your config)
1. Provide the OpenID Connect client's issuer, client ID, and client secret in the Sourcegraph site configuration shown below.
1. (Optional) Require users to have a specific email domain name to authenticate (e.g., to limit users to only those from your organization).

Example [`openidconnect` auth provider](../config/site_config.md#openid-connect-including-google-workspace) configuration:

```json
{
  // ...
  "externalURL": "https://sourcegraph.example.com",
  "auth.providers": [
    {
      "type": "openidconnect",
      "issuer": "https://oidc.example.com",
      "clientID": "my-client-id",
      "configID":"my-config-id", //An arbitrary value that will be used to reference to this auth provider within the site config
      "clientSecret": "my-client-secret",
      "requireEmailDomain": "example.com"
    }
  ]
}
```

Sourcegraph supports the OpenID Connect Discovery standard for configuring the auth provider (using the document at, e.g., `https://oidc.example.com/.well-known/openid-configuration`).

See the [`openid` auth provider documentation](../config/site_config.md#openid-connect-including-google-workspace) for the full set of configuration options.

### How to control user sign-up with OpenID auth provider

**allowSignup**

  If true or not set, it allows new users to creating their Sourcegraph accounts via OpenID.
  When `false`, sign-up won't be available and a site admin should create new users accounts.

  ```json
    {
      "type": "openidconnect",
      // ...
      "allowSignup": false
    }
  ```

### Google Workspace (Google accounts)

Google's Workspace (formerly known as G Suite) supports OpenID Connect, which is the best way to enable Sourcegraph authentication using Google accounts. To set it up:

1. Create an **OAuth client ID** and client secret in the [Google API credentials console](https://console.developers.google.com/apis/credentials). [Google's interactive OpenID Connect documentation page](https://developers.google.com/identity/protocols/OpenIDConnect#getcredentials):
    - **Application type:** Web application
    - **Name:** Sourcegraph (or any other name your users will recognize)
    - **Authorized JavaScript origins:** (leave blank)
    - **Authorized redirect URIs:** `https://sourcegraph.example.com/.auth/callback` (replace `https://sourcegraph.example.com` with the value of the `externalURL` property in your config)
1. Use the **client ID** and **client secret** values in Sourcegraph site configuration (as shown in the example below).
1. Set your Google Workspace domain in `requireEmailDomain` to prevent users outside your organization from signing in.

Example [`openidconnect` auth provider](../config/site_config.md#openid-connect-including-google-workspace) configuration for Google Workspace:

```json
{
  // ...
  "externalURL": "https://sourcegraph.example.com",
  "auth.providers": [
    {
      "type": "openidconnect",
      "issuer": "https://accounts.google.com", // All Google Workspace domains use this issuer URI.
      "clientID": "my-client-id",
      "clientSecret": "my-client-secret",
      "requireEmailDomain": "example.com"
    }
  ]
}
```

## HTTP authentication proxies

You can wrap Sourcegraph in an authentication proxy that authenticates the user and passes the user's username or email (or both) to Sourcegraph via HTTP headers. The most popular such authentication proxy is [pusher/oauth2_proxy](https://github.com/pusher/oauth2_proxy). Another example is [Google Identity-Aware Proxy (IAP)](https://cloud.google.com/iap/). Both work well with Sourcegraph.

To use an authentication proxy to authenticate users to Sourcegraph, add the following lines to your site configuration:

```json
{
  // ...
  "auth.providers": [
    {
      "type": "http-header",
      "usernameHeader": "X-Forwarded-User",
      "emailHeader": "X-Forwarded-Email"
    }
  ]
}
```

Replace `X-Forwarded-User` with the name of the HTTP header added by the authentication proxy that contains the user's username.

Ensure that the HTTP proxy is not setting its own `Authorization` header on the request. Sourcegraph rejects requests with unrecognized `Authorization` headers and prints the error log `lvl=eror msg="Invalid Authorization header." err="unrecognized HTTP Authorization request header scheme (supported values: token, token-sudo)"`.

For pusher/oauth2_proxy, use the `-pass-basic-auth false` option to prevent it from sending the `Authorization` header.

### Username header prefixes

Some proxies add a prefix to the username header value. For example, Google IAP sets the `x-goog-authenticated-user-id` to a value like `accounts.google.com:alice` rather than just `alice`. If this is the case, use the `stripUsernameHeaderPrefix` field. If using Google IAP, for example, add the following lines to your site configuration:

```json
{
  // ...
  "auth.providers": [
    {
      "type": "http-header",
      "usernameHeader": "x-goog-authenticated-user-email",
      "stripUsernameHeaderPrefix": "accounts.google.com:"
    }
  ]
}
```

## IP Allowlist

In additional to identity authentication provider, you may wish to restrict access to your site to a list of IP addresses. This is only applicable to the UI and GraphQL API endpoints.

__IMPORTANT__: Before you start, you should know the subnet of your Sourcegraph deployment, this is needed to avoid breaking inter-service communication, e.g., kubernetes pod subnet.

In the event of lossing access to the UI, you can still access the UI from `localhost`. This depends on your deployment type. For kubernetes, you can use `kubectl port-forward`. For docker-compose deployment on a VM, you can use `ssh` tunnel.

### Case 1 - Only permit connection from IP addresses

- `auth.allowedIpAddress.userIpAddress` is the authorized user IP addresses. This is usually your organization VPN exit gateway IP addresses. 
- `auth.allowedIpAddress.trustedClientIpAddress` is the subnet of your Sourcegraph deployment. e.g, kubernetes pod subnet. You must configure this field correctly or risk lossing access to the instance UI.

```json
{
  "auth.allowedIpAddress": {
    "enabled": true,
    "userIpAddress": [
      "1.1.1.1",
      "100.100.100.0/25",
    ],
    "trustedClientIpAddress": [
      "10.244.0.0/16",
    ]
  },
}
```

### Case 2 - Only permit connection from upstream application load balancer and users from specific exit gateway

- `auth.allowedIpAddress.trustedClientIpAddress` is the subnet of your Sourcegraph deployment. e.g, kubernetes pod subnet. You must configure this field correctly or risk lossing access to the instance UI.
- `auth.allowedIpAddress.clientIpAddress` is the source ranges of the upstream application load balancer. This ensures only load balancer is permitted to open direct connection with your Sourcegraph instance, and you can trust the `x-forwarded-for` header produced by the application load balancer.
- `auth.allowedIpAddress.userIpAddress` is the authorized user IP addresses. This is usually your organization VPN exit gateway IP addresses. 
- `auth.allowedIpAddress.userIpRequestHeaders` is the header to infer user ip address. This depends on the upstream application load balancer, and you need to ensure this value is resilient to IP spoofing.

```json
{
  "auth.allowedIpAddress": {
    "enabled": true,
    "userIpAddress": [
      "1.1.1.1",
      "100.100.100.0/25",
    ],
    "trustedClientIpAddress": [
      "10.244.0.0/16",
    ],
    "clientIpAddress": [
      "130.211.0.0/22",
    ],
    // CF-Connecting-IP is reliable and free from spoofing if Cloudflare proxy is used
    "userIpRequestHeaders": ["X-Forwarded-For", "CF-Connecting-Ip"]
  },
}
```

## Linking a Sourcegraph account to an auth provider

In most cases, the link between a Sourcegraph account and an authentication provider account happens via email.

Consequently, you can only sign in via an auth provider if your email on Sourcegraph matches the one configured in the auth provider.

Let's say the email field in your Sourcegraph account was kept blank when a site admin created the account for you, but the username matches your username on GitHub or GitLab. Will this work? If you try to sign in to SG with GitHub or GitLab, it won't work, and you will see an error informing you that a verified email is missing.

Exceptions to this rule are [HTTP Proxies](#http-authentication-proxies), where there's an option to make the link via username only.
For [Bitbucket](../config/authorization_and_authentication.md#bitbucket-server-bitbucket-data-center-authorization), we don't support OAuth. Still, the match between the chosen auth provider used with Bitbucket and a user's Bitbucket account happens via username.

Using only a username to match a Sourcegraph account to an auth provider account is not recommended, as you can see [here](../external_service/gitlab.md#username), for example.
Usernames in Sourcegraph are mutable, so a malicious user could change a username, elevating their privileges.

## Linking accounts from multiple auth providers
Sourcegraph will automatically link accounts from multiple external auth providers, resulting in a single user account on Sourcegraph. That way a user can login with multiple auth methods and end up being logged in with the same Sourcegraph account. In general, to link accounts, the following condition needs to be met:

At the time of signing in with the new account, any of the email addresses configured on the user account on the auth provider must match any of the **verified** email addresses on the user account on the Sourcegraph side. If there is a match, the accounts are linked, [otherwise a new user account is created if auth provider is configured to support user sign ups](#how-to-control-user-sign-up).

## Username normalization

Usernames on Sourcegraph are normalized according to the following rules.

- Any characters not in `[a-zA-Z0-9-._]` are replaced with `-`
- Usernames with exactly one `@` character are interpreted as an email address, so the username will be extracted by truncating at the `@` character.
- Usernames with two or more `@` characters are not considered an email address, so the `@` will be treated as a non-standard character and be replaced with `-`
- Usernames with consecutive `-` or `.` characters are not allowed, so they are replaced with a single `-` or `.`
- Usernames that start with `.` or `-` are not allowed, starting periods and dashes are removed
- Usernames that end with `.` are not allowed, ending periods are removed

Usernames from authentication providers are normalized before being used in Sourcegraph. Usernames chosen by users are rejected if they do not meet these criteria.

For example, a user whose external username (according the authentication provider) is `alice_smith@example.com` would have the Sourcegraph username `alice-smith`.

If multiple accounts normalize into the same username, only the first user account is created. Other users won't be able to sign in. This is a rare occurrence; contact support if this is a blocker.

## [Troubleshooting](troubleshooting.md)
