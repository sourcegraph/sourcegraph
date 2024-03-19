# Configuring SAML with JumpCloud

> NOTE: Please substitute `https://sourcegraph.example.com` with the actual value of `externalURL` in your [site configuration](../../config/site_config.md)).

## 1. Configure SAML 2.0 application on JumpCloud

Configure a new **SAML 2.0** application with the following settings:

- **Display Label**: Recommend `Sourcegraph`, but could be anything you prefer.
- **IdP Entity ID**: Recommend `JumpCloud`, but could be anything you prefer.
- **SP Entity ID**: `Sourcegraph`
- **ACS URL**: `https://sourcegraph.example.com/.auth/saml/acs`
- **SP Certificate**: (remain unset)
- **SAMLSubject NameID**: `email`
- **SAMLSubject NameID Format**: `urn:oasis:names:tc:SAML:2.0:nameid-format:persistent`
- **Signature Algorithm**: `RSA-SHA256`
- **Sign Assertion**: `true` (checked)
- **Default RelayState**: (remain unset)
- **IdP-Initiated URL**: (remain unset)
- **Declare Redirect Endpoint**: `false` (unchecked)
- **IdP URL**: Recommend `https://sso.jumpcloud.com/saml2/sourcegraph`, but could be anything you prefer.
- **Attributes**: (remain unset)

Once the application is created, look for a tiny link called **export metadata** on the bottom-right of the page. Click on the link and save the metadata file which will be used later.

## 2. Configure SAML authentication provider in Sourcegraph

[Add a SAML auth provider](./index.md#add-a-saml-provider) with `configID` set to the **SP Entity ID**, and `identityProviderMetadata` set to the content of the metadata you saved in the previous section. Here is an example of what your site configuration should look like:

```json
{
 // ...
 "externalURL": "https://sourcegraph.example.com",
 "auth.providers": [
    {
      "type": "saml",
      // This value must match the "SP Entity ID" of your JumpCloud application.
      "configID": "Sourcegraph",
      "serviceProviderIssuer": "Sourcegraph",
      // You can escape the metadata to a JSON string using a tool like https://json-escape-text.now.sh.
      // Please be noted it is an online tool and could leak or record your confidential information.
      "identityProviderMetadata": "<?xml version=\"1.0\" encoding=\"UTF-8\"?><md:EntityDescriptor xmlns:md=\"urn:oasis:names:tc:SAML:2.0:metadata\" entityID=\"JumpCloud\"><md:IDPSSODescriptor WantAuthnRequestsSigned=\"false\" protocolSupportEnumeration=\"urn:oasis:names:tc:SAML:2.0:protocol\"><md:KeyDescriptor use=\"signing\"><ds:KeyInfo xmlns:ds=\"http://www.w3.org/2000/09/xmldsig#\"><ds:X509Data><ds:X509Certificate>..."
    }
 ]
}
```
