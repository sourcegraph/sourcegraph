# Gerrit
<span class="badge badge-beta">Beta</span>

A Gerrit instance can be connected to Sourcegraph as follows:

1. [Configure Gerrit as a code host connection](#configure-gerrit-as-a-code-host-connection)
1. (Optional) [Add Gerrit as an authentication provider](#add-gerrit-as-an-authentication-provider)
1. (Optional) [Have users authenticate their Sourcegraph accounts using their Gerrit HTTP credentials](#have-users-authenticate-their-sourcegraph-accounts-using-their-gerrit-http-credentials)

## Configure Gerrit as a code host connection

1. In the **Site Admin** settings area, select **Manage code hosts** from the options on the left and select the **Add code host** option.
![The Manage code hosts section in the Site Admin settings area.](https://storage.googleapis.com/sourcegraph-assets/docs/images/administration/config/external-services/gerrit/gerrit-manage-code-hosts.png)
2. On the following screen, select **Gerrit** as the code host of choice.
![Gerrit as a code host option in the list](https://storage.googleapis.com/sourcegraph-assets/docs/images/administration/config/external-services/gerrit/gerrit-select.png)
3. Next you will have to provide a [configuration](#configuration) for the Gerrit code host connection. Here is an example configuration:
```json
{
  "url": "https://gerrit.example.com/", // Be sure to add a trailing slash
  "username": "<admin username>",
  "password": "<admin password>",
  "projects": [ // If not set, all projects on the Gerrit instance will be mirrored
    "docs",
    "kubernetes/kubernetes"
  ],
  "authorization": {} // Marks all repositories as private. Users will be required to present valid Gerrit HTTP credentials in order to view repositories
}
```
4. The provided `username` and `password` must be the HTTP credentials of an admin account on Gerrit. See [the Gerrit HTTP documentation](https://gerrit-documentation.storage.googleapis.com/Documentation/2.14.2/user-upload.html#http) for details on how to generate HTTP credentials.
5. Select **Add Repositories** to create the connection. Sourcegraph will start mirroring the specified projects.

If you added the `"authorization": {}` option to the configuration, and this is the first Gerrit code host connection you have created for this Gerrit instance, you might see a warning like this:
![Warning indicating that an authentication provider is required for a code host connection](https://storage.googleapis.com/sourcegraph-assets/docs/images/administration/config/external-services/gerrit/gerrit-auth-warning.png)

Simply follow the steps in the next section to configure a Gerrit authentication provider.

## Add Gerrit as an authentication provider

If the `"authorization": {}` option has been set on a Gerrit code host connection, a Gerrit authentication provider will be required so that authroized users are able to search for and browse the code mirrored by that code host connection.

1. In the **Site Admin** settings area, select [**Site configuration**](../config/site_config.md) from the options on the left.
2. Add a Gerrit configuration to the list of `"auth.providers"`.
![Add a Gerrit configuration to the list of configured authentication providers](https://storage.googleapis.com/sourcegraph-assets/docs/images/administration/config/external-services/gerrit/gerrit-auth.png)
3. Here is an example configuration:
```json
{
  "type": "gerrit",
  "url": "https://gerrit.example.com/" // This must match the URL of the code host connection. Be sure to add a trailing slash
}
```
4. Save the configuration. If there was a warning at the top of the page, it should now disappear.

Users should now be able to authenticate their Sourcegraph accounts using their Gerrit HTTP credentials.

## Have users authenticate their Sourcegraph accounts using their Gerrit HTTP credentials

After [configuring Gerrit as a code host connection](#configure-gerrit-as-code-host-connection) and [adding Gerrit as an authentication provider](#add-gerrit-as-an-authnetication-provider), users will be able to authenticate their Sourcegraph accounts using their Gerrit HTTP credentials:

As a user:

1. Visit your user settings page and select **Account security** from the options on the left.
![A user's Account security page](https://storage.googleapis.com/sourcegraph-assets/docs/images/administration/config/external-services/gerrit/gerrit-account-security.png)
1. Gerrit should appear in the list of accounts you are able to connect. If it does not appear, try refreshing the page.
1. Select the **Add** option next to Gerrit.
1. Provide your Gerrit username and HTTP password. If you are unsure of how to generate an HTTP password, see [the Gerrit HTTP documentation](https://gerrit-documentation.storage.googleapis.com/Documentation/2.14.2/user-upload.html#http).
1. Once your Gerrit credentials are verified, you'll be able to view your private Gerrit projects within Sourcegraph! If you cannot immediately see any projects you should have access to, try giving it some time, as it can take a while for your Gerrit permissions to reflect on Sourcegraph if there is a high volume of users on the system.

## Configuration

Gerrit connections support the following configuration options, which are specified in the JSON editor in the site admin "Manage code hosts" area.

<div markdown-func=jsonschemadoc jsonschemadoc:path="admin/external_service/gerrit.schema.json">[View page on docs.sourcegraph.com](https://docs.sourcegraph.com/admin/external_service/gerrit) to see rendered content.</div>
