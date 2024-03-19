# Configuring SAML with Azure Active Directory (Azure AD)

## 1. Add an unlisted (non-gallery) application to your Azure AD organization

1. In Azure AD, create an unlisted (non-gallery) application [following the official documentation](https://docs.microsoft.com/en-us/azure/active-directory/manage-apps/add-non-gallery-app).
1. Once the application is created, follow [these instructions to enable SAML SSO](https://docs.microsoft.com/en-us/azure/active-directory/manage-apps/configure-single-sign-on-non-gallery-applications). Use these configuration values (replacing "sourcegraph.example.com" with your Sourcegraph instance URL):
  * **Identifier (Entity ID):** `https://sourcegraph.example.com/.auth/saml/metadata`
  * **Reply URL (Assertion Consumer Service URL):** `https://sourcegraph.example.com/.auth/saml/acs`
  * **Sign-on URL, Relay State, and Logout URL** can be left empty.
  * **User Attributes & Claims:** Add the following attributes.<br>
    `emailaddress`: user.mail (required)<br>
    `name`: user.userprincipalname (optional)<br>
    `login`: user.userprincipalname (optional)<br>
  * **Name ID**: `email`
  * You can leave the other configuration values set to their defaults.
1. Record the value of the "App Federation Metadata Url". You'll need this in the next section.

## 2. Add the SAML auth provider to Sourcegraph site config

[Add a SAML auth provider](./index.md#add-a-saml-provider) with `identityProviderMetadataURL` set to the "App Federation Metadata Url" you recorded in the previous section. Here is an example of what your site configuration should look like:

```json
{
 // ...
 "externalURL": "https://sourcegraph.example.com",
 "auth.providers": [
   {
     "type": "saml",
     "configID": "azure",
     "identityProviderMetadataURL": "https://login.microsoftonline.com/7d2a00ed-73e8-4920-bbfa-ef68effe2d1e/federationmetadata/2007-06/federationmetadata.xml?appid=eff20ae4-145b-4bd3-ff3f-21edab43fe99"
   }
 ]
}
```

> NOTE: Optional, but recommended: [add automatic provisioning of users with SCIM](../../scim.md). 
