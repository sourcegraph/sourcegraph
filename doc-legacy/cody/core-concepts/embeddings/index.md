# Embeddings

<p class="subtitle">Learn how you can use embeddings with Cody for better code understanding.</p>

## What are embeddings?

Embeddings are a semantic representation of text that allow you to create a search index over your codebase. Cody splits your codebase into searchable chunks and sends them to an external service specified in your site's configuration for embedding. The resulting embedding index is stored in a managed object storage service.

## Enable embeddings

By default, no embeddings are created. Embeddings are automatically enabled and configured when Cody is enabled. Admins must choose which code is sent to the third-party language model (LLM) for embedding (currently OpenAI). Once Sourcegraph provides first-party embeddings, they will be enabled for all repositories by default.

## Incremental embeddings

Incremental embeddings allow you to update the embeddings for a repository without re-embedding the entire repository. With incremental embeddings, outdated embeddings of deleted and modified files are removed, and new embeddings of modified and added files are added to the repository's embeddings. This speeds up updates, reduces data sent to the embedding provider, and saves costs.

Incremental embeddings are enabled by default, but you can disable them if needed by setting
the `incremental` property in the embeddings configuration to `false`.

```json
{
  // [...]
  "embeddings": {
    // [...]
    "incremental": false
  }
}
```

## Minimum time interval between automatically scheduled embeddings

You can adjust the minimum time between automatically scheduled embeddings. If you've configured a repository for automated embeddings, the repository will be scheduled for embedding with every new
commit.

By default, there is a 24-hour time interval that must pass between two embeddings. For example, if a repository is scheduled for embedding at 10:00 AM and a new commit happens at 11:00 AM, the next embedding will be scheduled earliest for 10:00 AM the next day.

You can configure the minimum time interval by setting the `minimumInterval` property in the embeddings configuration.

Supported time units are h (hours), m ( minutes), and s (seconds).

```json
{
  // [...]
  "embeddings": {
    // [...]
    "minimumInterval": "24h"
  }
}
```

## Third-party embeddings provider

Instead of [Sourcegraph Cody Gateway](./../cody-gateway.md), admins can also use a third-party embeddings provider like:

- OpenAI
- Azure OpenAI <span style="margin-left:0.25rem" class="badge badge-experimental">Experimental</span>

### OpenAI

To use embeddings with OpenAI:

- First, create your own key with OpenAI [here](https://beta.openai.com/account/api-keys)
- Next, go to **Site admin > Site configuration** (`/site-admin/configuration`) on your instance
- Finally set the following configuration:

```json
{
  // [...]
  "cody.enabled": true,
  "embeddings": {
    "provider": "openai",
    "accessToken": "<token>",
    "excludedFilePathPatterns": []
  }
}
```

### Azure OpenAI

<aside class="experimental">
<p>
<span style="margin-right:0.25rem;" class="badge badge-experimental">Experimental</span> Azure OpenAI support is in the experimental stage.
<br />
For any feedback, you can <a href="https://sourcegraph.com/contact">contact us</a> directly, file an <a href="https://github.com/sourcegraph/cody/issues">issue</a>, join our <a href="https://discord.com/servers/sourcegraph-969688426372825169">Discord</a>, or <a href="https://twitter.com/sourcegraphcody">tweet</a>.
</p>
</aside>

To use embeddings with Azure OpenAI:

- First, create a project in the Azure OpenAI portal
- Then, from the project overview, go to **Keys and Endpoint**
- From here get **one of the keys** and the **endpoint**
- Next, under **Model deployments** click **manage deployments**
- Make sure you deploy the models you want to use. For example, `text-embedding-ada-002`. Also, take note of the **deployment name**
- Finally, go to **Site admin > Site configuration** (`/site-admin/configuration`) on your instance and set:

```json
{
  "cody.enabled": true,
  "embeddings": {
    "provider": "azure-openai",
    "model": "<deployment name of the model>",
    "endpoint": "<endpoint>",
    "accessToken": "<See below>",
    "dimensions": 1536,
    "excludedFilePathPatterns": []
  }
}
```

For the access token, you can either:

- As of 5.2.4 the access token can be left empty and it will rely on Environmental, Workload Identity or Managed Identity credentials configured for the `frontend` service
- Set it to `<API_KEY>` if directly configuring the credentials using the API key specified in the Azure portal
- If Azure Authentication requests require a proxy the environment variable `CODY_AZURE_OPENAI_IDENTITY_HTTP_PROXY` can be specified to a proxy URL

<br>

> NOTE: Azure OpenAI is in experimental stage. It's not recommended to use in a production setting.

### Disable embeddings

Embeddings can be disabled, even with Cody enabled, by using the following site configuration:

```json
{
  "embeddings": { "enabled": false }
}
```
