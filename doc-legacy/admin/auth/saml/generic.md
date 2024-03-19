# Configuring SAML

To configure Sourcegraph to use SAML authentication, you'll need to do 2 things:

1. Add application configuration to your identity provider (e.g., Auth0) describing Sourcegraph.
1. Add configuration to Sourcegraph describing your identity provider.

## 1. Add application configuration to your identity provider

Your identity provider should provide documentation on how to register a new SAML application. Here are links to docs for common identity providers:

* [Auth0](https://auth0.com/docs/protocols/saml/saml-idp-generic)
* [Ping Identity](https://learning.getpostman.com/docs/postman-enterprise/sso/saml-ping/)
* [Salesforce Identity](https://help.salesforce.com/articleView?id=identity_provider_enable.htm)
* We have vendor-specific instructions for [Okta](okta.md), [Azure AD](azure_ad.md), and [Microsoft ADFS](microsoft_adfs.md)

If you do not see your identity provider in the list above or otherwise have trouble with SAML configuration, please reach out to [support@sourcegraph.com](mailto:support@sourcegraph.com?subject=SAML%20help&body=I%20am%20trying%20to%20configure%20Sourcegraph%20with%20SAML%20authentication%20with%20%3Cfill%20in%20your%20auth%20provider%3E%2C%20but%20am%20running%20into%20issues%3A%20%3Cplease%20describe%3E).


Ensure the following values are set for the application configuration in the identity provider. (Note: the exact names and labels may vary slightly for different identity providers)

- **Assertion Consumer Service URL, Recipient URL, Destination URL, Single sign-on URL:** `https://sourcegraph.example.com/.auth/saml/acs` (substituting the `externalURL` from your [site configuration](../../config/site_config.md))
- **Service Provider (issuer, entity ID, audience URI, metadata URL):** `https://sourcegraph.example.com/.auth/saml/metadata` (substituting the `externalURL` from your [site configuration](../../config/site_config.md)). Some identity providers require you to input these metadata values manually, instead of fetching everything from one URL. In that case, navigate to `https://sourcegraph.example.com/.auth/saml/metadata` and transcribe the values in the XML to the identity provider configuration.
- **Attribute statements (claims):** Sourcegraph *requires* that an attribute `email` be set with the value of the user's verified email address. This is used to uniquely identify users to Sourcegraph. Other attributes such as `login` and `displayName` are optional.
  - `email` (required): the user's email
  - `login` (optional): the user's username
  - `displayName` (optional): the full name of the user
- **Name ID**: `email`

## 2. Add a SAML auth provider to Sourcegraph site configuration

[Add a SAML auth provider](./index.md#add-a-saml-provider).
