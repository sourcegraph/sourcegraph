# Configuring SAML with Okta

## 1. Add a SAML application in Okta

1. Login to your Okta account.
2. On the left hand side, click on the “Applications” menu, and then select the “Applications” item.
3. Click on “Create App Integration”. Another screen should pop-up, listing sign-in methods. Choose “SAML 2.0”. Click "Next".
4. You should now see “Create SAML Integration” on this page, and you will be on “General Settings”. Specify a name for “App name” (example: “Sourcegraph”) and the [logo](https://sourcegraphstatic.com/sourcegraph-logo-asterisk-color-1024px.png). Click “Next”.
5. Now you should be on “Configure SAML”. On this page, you will need your Sourcegraph URL (Ex: `https://sourcegraph.example.com`). Follow along with the following instructions, replacing `<URL>` with your Sourcegraph URL:
    - In section A ("SAML Settings"), under "General":
      - For “Single sign on URL”, set the value to `<URL>`/.auth/saml/acs
        - Under this box, there should be a checkbox labeled “Use this for Recipient URL and Destination URL”. Check the box if it is not already selected.
      - For “Audience URI (SP Entity ID)”, set the value to `<URL>`/.auth/saml/metadata
      - For "Name ID format", choose "EmailAddress"
    - In the section titled “Attribute Statements (optional)”:
      - Set the following Name and Values, leaving the Name format to “Unspecified”
      - Email: user.email (This one is required)
      - Login: user.login (This one is optional)
      - displayName: user.firstName (This one is optional)
6. Click Next.
7. Now you should be on the “Feedback” step. Select the radio button for “I’m an Okta customer adding an internal app”, and provide feedback if you wish. Click "Finish".
8. You should now be on the Application page for Sourcegraph, where you can view the settings and configurations you have just set. You will want to grant users or groups sign-in access before moving on.
    - To grant access to your own user:
      - Go to the “Assignments” tab, where you should see a table of People and Groups. Click the “Assign” dropdown, and then “Assign to People”.
      - A new window should pop-up. Find your account, and click “Assign”, “Save and Go Back”, and then “Done”.
9. You have now finished configuring the settings in Okta. Before moving to step #2, make sure you have granted access to users/groups and copy the metadata URL to your clipboard:
   - Go into the “Sign On” tab
   - Scroll down to the section "SAML Signing Certificates"
   - Click the "Actions" dropdown in the active certificate and then "View IdP Metadata"
   - Copy the URL from the new browser tab that will open. It will have the following format:

   ```sh
       https://<your tenant>.okta.com/app/<unique identitfier>/sso/saml/metadata
   ```

## 2. Add the SAML auth provider to Sourcegraph site config

[Add a SAML auth provider](./index.md#add-a-saml-provider) with `identityProviderMetadataURL` set to the URL you copied from the "View IdP Metadata" link in the previous section. Here is an example of what your site configuration should look like:

```json
{
 // ...
 "externalURL": "https://sourcegraph.example.com",
 "auth.providers": [
   {
     "type": "saml",
     "configID": "okta",
     "identityProviderMetadataURL": "https://okta.example.com/app/8VglnckX0yyhdkp0bk00/sso/saml/metadata",
     "allowSignup": true 
   }
 ]
}
```

> NOTE: Optional, but recommended: [add automatic provisioning of users with SCIM](../../scim.md). 
