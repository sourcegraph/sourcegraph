# Azure DevOps

Site admins can sync Git repositories hosted on [Azure DevOps](https://dev.azure.com) with Sourcegraph so that users can search and navigate the repositories.

To connect Azure DevOps to Sourcegraph, create a personal access token from your user settings page by following the below steps:

1. Navigate to the `Personal Access Tokens` page from the user settings.

![Visit the Personal Access Tokens page](https://storage.googleapis.com/sourcegraph-assets/docs/images/admin/config/azure-devops-personal-access-token-step-1.png)

2. Click on `New Token`.

![Click on New Token](https://storage.googleapis.com/sourcegraph-assets/docs/images/admin/config/azure-devops-personal-access-token-step-2.png)

3. Under the `Organization` menu, select `All accessible organizations` to allow access to all organizations. This is required to be able to perform connection checks from the code host page and to sync repositories from multiple organizations. Alternatively, site admins may also create a unique user that has access to only the selective organizations that they would like to sync with Sourcegraph. However the token being created **must** have access to `All accessible organizations` as shown below.

![Select All accessible organizations](https://storage.googleapis.com/sourcegraph-assets/docs/images/admin/config/azure-devops-personal-access-token-step-3.png)

4. Select the following scopes:

   - Code (Read)
   - Project and Team (Read)
   - User Profile (Read)

Next, configure the code host connection by following the next steps:

1. Go to **Site admin > Manage code hosts > Add repositories**.
1. Select **Azure DevOps**.
1. Provide a [configuration](#configuration) for the Azure DevOps code host connection. Here is an example configuration:

   ```json
   {
     "url": "https://dev.azure.com/",
     "username": "<admin username>",
     "token": "<admin token>",
     "projects": ["org1/project1"],
     "orgs": ["org2"]
   }
   ```

1. Select **Add repositories**.

## Repository syncing

Currently, all repositories belonging to the configured organizations/projects will be synced.

In addition, you may exclude one or more repositories by setting the [`exclude`](azuredevops.md#configuration) field in the code host connection.

### HTTPS cloning

Sourcegraph clones repositories from Azure DevOps via HTTP(S), using the [`username`](azuredevops.md#configuration) and [`token`](azuredevops.md#configuration) required fields you provide in the configuration.

## Configuration

Azure DevOps connections support the following configuration options, which are specified in the JSON editor in the site admin "Manage code hosts" area.

<div markdown-func=jsonschemadoc jsonschemadoc:path="admin/external_service/azuredevops.schema.json">[View page on docs.sourcegraph.com](https://docs.sourcegraph.com/admin/external_service/azuredevops) to see rendered content.</div>

## Webhooks

Please consult [this page](../config/webhooks/incoming.md) in order to configure webhooks.

## Permissions syncing

[User-level permissions](../permissions/syncing.md#permission-syncing) syncing is supported for Azure DevOps code host connections. Here is the list of prerequisites:

1. Configure Azure DevOps as an OAuth provider by consulting [this page](../config/authorization_and_authentication.md#azure-devops-services)
2. Next verify that users can now sign up / login to your Sourcegraph instance with your Azure DevOps OAuth provider
3. Set the following in your Azure DevOps code host connection:

   ```json
   {
     // ...
     "enforcePermissions": true
   }
   ```

4. For each Azure DevOps organization that is part of the `orgs` or `projects` list in the code host configuration, enable **Third-party application access via OAuth** from **Organization settings > Security > Policies**

![Enable Third-party application access via OAuth](https://storage.googleapis.com/sourcegraph-assets/docs/images/admin/config/azure-devops-organization-enable-oauth-access.png)

> NOTE: We do not support preemptive permissions syncing at this point. Once a user signs up / logins to Sourcegraph with their Azure DevOps account, Sourcegraph uses the authenticated `access_token` to calculate permissions by listing the organizations and projects that the user has access to. As a result, immediately after signing up user level permissions may not be 100% up to date. Users are advised to wait for an initial permissions sync to complete, whose status they may check from the `Permissions` tab under their account settings page. Alternatively they may also trigger a permissions sync for their account from the same page.

Since permissions are already enforced by setting `enforcePermission` in the code host configuration, even though user permissions may not have synced completely, users will not have access to any repositories that they cannot access on Azure DevOps. As the user permissions sync progresses and eventually completes, they will be able to access the complete set of repositories on Sourcegraph that they can already access on Azure DevOps.

## Rate limits

When Sourcegraph hits a rate limit imposed by Azure DevOps, Sourcegraph waits the appropriate amount of time specified by the code host before retrying the request. You can read more about how Azure DevOps imposes rate limits [here](https://learn.microsoft.com/en-us/azure/devops/integrate/concepts/rate-limits).
