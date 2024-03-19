# SCIM

<aside class="beta">
<p>
<span class="badge badge-beta">Beta</span> This feature is in beta while we're testing it with more IdPs. Our implementation complies with the SCIM 2.0 specification, and passes the validator for Okta and Azure AD. But implementations might differ on the side of IdPs and validators don't give a 100% coverage, so we can't guarantee that our solution works with all IdPs in every case.
</p>

<p><b>We're very much looking for input and feedback on this feature.</b> You can either <a href="https://sourcegraph.com/contact">contact us directly</a>, <a href="https://github.com/sourcegraph/sourcegraph">file an issue</a>, or <a href="https://twitter.com/sourcegraph">tweet at us</a>.</p>
</aside>

SCIM (System for Cross-domain Identity Management) is a standard for provisioning and deprovisioning users and groups in an organization. IdPs (identity providers) like Okta, OneLogin, and Azure Active Directory support provisioning users through SCIM.

Sourcegraph supports SCIM 2.0 for provisioning and de-provisioning _users_.

> NOTE: While our implementation of SCIM 2.0 is compliant with the specification, we’ve only tested it against two IdPs: Okta and Azure Active Directory. We can't guarantee it works with every IdP if the provider doesn't fully comply with the specification.

## How to use

To use SCIM, you must have an existing IdP configured as an auth provider on your Sourcegraph instance. For authenticating SCIM requests, we currently support Bearer token authentication. We have a guide for Okta setup [below](#setting-up-okta-as-the-idp).

To configure:

1. Generate a random alphanumeric bearer token of maximum 255 characters.
   To do this in a terminal, run:
   
   ```
   openssl rand -base64 342 | tr -dc 'a-zA-Z0-9' | cut -c -255
   ```

   (This command generates a random base64 string with more characters than required (342 characters) and then filters out non-alphanumeric characters. Finally, it trims the output to 255 characters. The reason we generate a longer string is to account for the fact that the base64 encoding has non-alphanumeric characters, which are removed by the tr command.)

2. Add the following line to your [site configuration](config/site_config.md):

   ```
   "scim.authToken": "{your token}"
   ```
3. If you use Microsoft Azure AD, add the following setting to your site config:

   ```
   "scim.identityProvider": "Azure AD"
   ```
4. Set up your IdP to use our SCIM API. The API is at

   ```
   https://sourcegraph.company.com/.api/scim/v2
   ```

## Configuring SCIM for Okta

To set up user provisioning in [Okta](https://help.okta.com/en-us/Content/Topics/Apps/Apps_App_Integration_Wizard_SCIM.htm), you must first set up a new app integration of the "SAML 2.0" type, then configure it to use SCIM. Here are the steps to do this:

1. Follow our [SAML guide](auth/saml/okta.md) to set up a new app integration with SAML, then open the integration you just created.
    - If you already have the integration, just open your existing app integration.
1. Go to the "General" tab and click "Edit" in the "App Settings" section.
1. Set "Provisioning" to "SCIM". This creates a new tab called "Provisioning".
1. Go to the "Provisioning" tab, and click "Edit"
1. Set "SCIM connector base URL" to `{yourSourcegraphUrl}/.api/scim/v2`
1. Set "Unique identifier field for users" to `userName`
1. Check the first three items in `Supported provisioning actions`: "Import New Users and Profile Updates", "Push New Users", and "Push Profile Updates".
1. Set "Authentication mode" to "HTTP Header"
1. Under "HTTP Header", paste the same alphanumeric bearer token you used in your site config.
1. Click "Test Connection Configuration" (first four items should be green—the user-related ones), then "Save".
1. Switch to "Provisioning" → "To App" and click "Edit". Enable "Create Users", "Update User Attributes" and "Deactivate Users".

> NOTE: You can also use our [SAML](auth/saml/okta.md) and [OpenID Connect](auth.md#openid-connect) integrations with Okta.

## Features and limitations

### User attributes

The User endpoint only synchronizes attributes needed to create a Sourcegraph account.

We sync the following attributes:

- preferred username
- name
- email addresses

### REST methods

We support REST API calls for:

- Creating users (POST)
- Updating users (PATCH)
- Replacing users (PUT)
- Deleting users (DELETE)
- Listing users (GET)
- Getting users (GET)

### Feature support

We support the following SCIM 2.0 features:

- ✅ Updating users (PATCH)
- ✅ Pagination for listing users
- ✅ Filtering for listing users

### Limitations

- ❌ Bulk operations – need to add users one by one
- ❌ Sorting – when listing users
- ❌ Entity tags (ETags)
- ❌ Multi-tenancy – you can only have 1 SCIM client configured at a time.
- ❌ Tests with many IdPs – we’ve only validated the endpoint with Okta and Azure AD.
