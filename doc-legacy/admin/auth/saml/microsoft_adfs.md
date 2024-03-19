# Configuring SAML with Microsoft Active Directory Federation Services (ADFS)

This document applies to the following versions of [Microsoft Active Directory Federation Services (ADFS)](https://docs.microsoft.com/en-us/windows-server/identity/active-directory-federation-services):

- ADFS 2.1 (Windows Server 2012)
- ADFS 3.0 (Windows Server 2012 R2)
- ADFS 4.0 (Windows Server 2016)

These instructions guide you through configuring Sourcegraph as a relying party (RP) of ADFS, which enables users to authenticate to Sourcegraph using their Active Directory credentials.

## Pre-requisites

* Active Directory instance where all users have email and username attributes.
* An instance of ADFS running on Windows Server, joined to your Active Directory domain.
* Sourcegraph should be [configured to use HTTPS](../../http_https_configuration.md#nginx-ssl-https-configuration).
* Ensure that `externalURL` in [site config](../../config/site_config.md) meets the following
  criteria:
  * It is the URL used by end users (no trailing slash).
  * It is HTTPS.

## 1. Add Sourcegraph as a relying party (RP) to ADFS

These steps should be completed on the Windows Server instance with ADFS installed.

1.  Open **Server Manager**.
1.  In the upper right corner, click **Tools > AD FS Management**. This should open the AD FS
    Management tool in a new window.
1.  In the sidebar of the AD FS Management tool, right-click **Relying Party Trusts > Add Relying
    Party Trust...** This should open the **Add Relying Party Trust Wizard**.
1.  Proceed through the Add Relying Party Trust Wizard as follows:
    - Welcome (Page 1): Leave as is and click **Start**.
    - Select Data Source (Page 2)
      - Select "Import data about the relying party published online or on a local network" and set
        the Federation metadata address: `https://sourcegraph.example.com/.auth/saml/metadata`.<br>
        If this step fails, see [Troubleshooting: Add Relying Party Trust fails on Select Data
        Source](#add-relying-party-trust-fails-on-select-data-source-page).
    - Specify Display Name (Page 3)
      - Enter "Sourcegraph" as the display name (any value will do) and click **Next**.
    - Choose Issuance Authorization Rules (Page 4)
      - Ensure **Permit everyone** is selected (the default) and click **Next**.
    - Ready to Add Trust (Page 5)
      - Click **Next**.
    - Finish (Page 6)
      - Click **Close**.

If the last step did NOT open the **Edit Claim Issuance Policy** or **Edit Claim Rules** window,
right-click the item ("Sourcegraph" or whatever you set as the display name) in the Relying Party
Trusts list and click **Edit Claim Issuance Policy**. In the "Issuance Transform Rules" tab of this
window, add the 2 following rules:

#### Claim Rule 1: Send User Info

Click **Add Rule...** and proceed through the "Add Transform Claim Rule Wizard" as follows:

- Choose Rule Type (Page 1)
  - Claim rule template: `Send LDAP Attributes as Claims`
  - Click **Next**
- Configure Claim Rule (Page 2)
  - Claim rule name: `Send User Info` (any value is OK)
  - Attribute store: `Active Directory`
  - Mapping of LDAP attributes to outgoing claim types:<br>
    `E-Mail-Addresses` -> `E-Mail Address`<br>
    `Display-Name` -> `Common Name`<br>
    `SAM-Account-Name` -> `Name` (optional, username will be derived from email if omitted)<br>
  - Click **Finish**.

#### Claim Rule 2: Email to NameID

Click **Add Rule...** and proceed through the "Add Transform Claim Rule Wizard" as follows:

- Choose Rule Type (Page 1)
  - Claim rule template: `Transform an Incoming Claim`
  - Click **Next**.
- Configure Claim Rule (Page 2):
  - Claim rule name: `Email to NameID`
  - Incoming claim type: `E-Mail Address`
  - Outgoing claim type: `Name ID`
  - Outgoing name ID format: `Persistent identifier`
  - Select **Pass through all claim values**.
  - Click **Finish**.

Click **OK** to apply the new claim rules and close the window.

## 2. Add the SAML auth provider to Sourcegraph site config

[Add a SAML auth provider](./index.md#add-a-saml-provider) that points to your ADFS server's SAML metadata URL. This
typically contains the path `/federationmetadata/2007-06/federationmetadata.xml`. Example:

```
{
  // ...
  "externalURL": "https://sourcegraph.example.com",
  "auth.providers": [
    {
      "type": "saml",
      "configID": "ms_adfs"
      "identityProviderMetadataURL": "https://adfs.example.com/federationmetadata/2007-06/federationmetadata.xml"
    }
  ]
}
```

## Authenticate to Sourcegraph using ADFS

All configuration is now complete. Let's test that it works.

1.  Visit `https://sourcegraph.example.com`. (If you were already signed in, sign out of
    Sourcegraph before doing so.)
1.  Sign into Sourcegraph using ADFS. If ADFS is the only `auth.provider` entry, you should be
    automatically redirected to the sign-in page. Otherwise, click on the SAML sign-in button.
1.  After signing into ADFS, you should be redirected back to Sourcegraph and signed in.

## Troubleshooting

When troubleshooting, we recommend setting the env var `INSECURE_SAML_LOG_TRACES=1` on the
`sourcegraph/server` Docker container (or the `sourcegraph-frontend` pod if Sourcegraph is deployed
to a Kubernetes cluster). This logs all SAML requests and responses.

### Add Relying Party Trust fails on Select Data Source page

This section covers troubleshooting options if the "Import data about the relying party published
online or a local network" option fails on the **Select Data Source** page of the **Add Relying
Party Trust Wizard**.

First, check that the Federation metadata address value (which should look like
`https://sourcegraph.example.com/.auth/saml/metadata`) is accessible by navigating to it in your web
browser. If this fails, then something is likely misconfigured in Sourcegraph. Check that you have
at most 1 SAML auth provider configured (`auth.providers` in [site
config](../../config/site_config.md)) or contact support for further guidance.

If the endpoint works in your browser, it downloads a `metadata` XML file. This indicates the
endpoint is working, but is inaccessible from ADFS (likely due to a firewall issue or ADFS not
respecting Sourcegraph's TLS certificate due to it being self-signed or from an unrecognized
Certificate Authority). If this is the case, you have a few options:

* Select "Import data about the relying party from a file" on the "Select Data Source" page and
  upload the `metadata` XML file manually.
* Select "Enter data about the relying party manually" on the "Select Data Source" page. Do the
  following on the subsequent pages of the wizard:
  * Specify Display Name: Sourcegraph
  * Configure Certificate: Leave as is
  * Configure URL: Check "Enable support for the SAML 2.0 WebSSO protocol". Set the "Relying party
    SAML 2.0 SSO service URL" to: https://sourcegraph.example.com/.auth/saml/acs
  * Configure Identifiers: Add https://sourcegraph.example.com/.auth/saml/metadata as a relying
    party trust identifier.<br>
    **IMPORTANT:** ensure your `externalURL` is set to exactly the root URL of the relying party
    trust identifier, (including the URL scheme)
  * For the remaining pages in the wizard, follow the normal steps.
* Fix the connectivity issue from ADFS to Sourcegraph by adding the appropriate firewall rule or by
  authorizing Sourcegraph's TLS certificate on the Windows Server host running ADFS.

### Error prefetching SAML service provider metadata

If you notice `Error prefetching SAML service provider metadata` errors in the Sourcegraph logs,
this indicates that Sourcegraph cannot fetch the URL specified in the `identityProviderMetadataURL`
field of the ADFS SAML auth provider config. Navigate to this URL in your web browser. If it errors,
check the URL for typos, or there might be an issue with the accessibility of ADFS.

If it succeeds, it should download a `federationmetadata.xml` file. This indicates that ADFS is
accessible from your browser, but not from the container running Sourcegraph (probably due to a
firewall rule or due to Sourcegraph's host not respecting the TLS certificate of ADFS). You have two
options:

* Open the `federationmetadata.xml` file, transform it into a JSON string (using a tool like
  https://json-escape-text.now.sh), and set it in the `identityProviderMetadata` field of the
  `auth.provider` SAML config. You can then delete the `identityProviderMetadataURL` field.
* Fix the connectivity issue from Sourcegraph to ADFS by adding the appropriate firewall rule, or
  authorizing the ADFS TLS certificate on the container running Sourcegraph.

### Error on ADFS login page

This section covers troubleshooting tips if the following is true:

* You have completed the requisite SAML config on both ADFS and Sourcegraph without errors.
* On sign-in, users arrive at an ADFS error page that says something like "An error occurred".

1. On Windows Server, open **Event Viewer**.
1. In the lefthand sidebar, click **Application and Services Logs** > AD FS > Admin.
1. Find the event corresponding to the failed login. This should have Level "Error".

The error log message will indicate something about the root cause of the error. A common error
message is `The requested relying party trust '<URL>' is unspecified or unsupported`. If this is the
error, double-check the relying party identifiers of the Relying Party Trust entry in ADFS:

* Open **AD FS Management**.
* In the sidebar, click **Relying Party Trusts**.
* Right-click the row corresponding to Sourcegraph, click "Properties".
* In the properties editor popup, click the "Identifiers" tab. Examine the list of relying party
  identifiers to ensure there is an entry that matches the relying party trust URL from the Event
  Viewer error.

### Other troubleshooting

See [SAML troubleshooting](../saml.md#saml-troubleshooting) for more tips.
