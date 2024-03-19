# Executor secrets

<style type="text/css">
  img.executor-diagram {
    display: block;
    margin: 1em auto;
    max-width: 700px;
    margin-bottom: 0.5em;
  }
</style>

Executor secrets can be used to define additional values to be used in Sourcegraph executors.

Secret values are currently only available in server-side batch changes. Use [`step.env`](../../batch_changes/references/batch_spec_yaml_reference.md#steps-env) to reference configured secrets in executions.

## How secrets work

Executor secrets are defined per-feature. If you want to define a secret for server-side batch changes, create a secret for that namespace (examples of namespaces are "Code Graph" and "Batch Changes"). Secrets are [encrypted](../config/encryption.md) if encryption is on, and always redacted in log outputs.

There are two types of secrets: 

- **Global secrets** 

  These secrets are defined by an admin in the site-admin interface and will be usable by every user on the Sourcegraph instance.

- **Namespaced secrets**
  
  These secrets are set either in org or user settings and are only usable by the user or org members in their respective namespaces. If a namespaced secret has the same name as a global secret, the namespaced secret is preferred.

Examples:

- Global secret `GITHUB_TOKEN`

  Can be used by every server-side batch change

- User 1 secret `GITHUB_TOKEN`

  Can be used by batch changes created by user 1 in their own namespace, overwrites the global secret

- Org 1 secret `GITHUB_TOKEN`

  Can be used by batch changes created by any org member of org 1 in the org namespace, overwrites the global secret

## Creating a new secret

To create a global secret, go to **Site-admin** > **Executors/Secrets** and click **Add secret**.
To create a user secret, go to your user profile from the navbar > **Settings** >  **Executor secrets** and click **Add secret**.
To create an org secret, go to the org profile from the navbar > **Executor secrets** and click **Add secret**.

Then, fill in a name for the secret. This will be the name of the environment variable it will be accessible as.
Next, fill in the secret value and hit **Add secret**.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/batch_changes/create_executor_secret.png" class="lead-screenshot">

## Rotating a secret

To rotate a secret or to update the secret value, go to **Executor secrets** (see [Creating a new secret](#creating-a-new-secret)). Next to the secret you want to update or rotate click on **Update**. Fill in the new value and hit **Update secret**.

> Note: When updating secrets server-side batch changes execution caches that reference the secret will be invalidated.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/batch_changes/update_executor_secret.png" class="lead-screenshot">

## Removing a secret

To remove a secret, go to **Executor secrets** (see [Creating a new secret](#creating-a-new-secret)). Next to the secret you want to delete click on **Remove**.

> Note: When removing secrets server-side batch changes execution caches that reference the secret will be invalidated.

<img src="https://storage.googleapis.com/sourcegraph-assets/docs/images/batch_changes/remove_executor_secret.png" class="lead-screenshot">
