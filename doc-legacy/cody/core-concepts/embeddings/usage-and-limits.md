# Usage and Limits

<p class="subtitle">Learn about all the usage polices and limits that apply while using embeddings.</p>

## Configure global policy match limit

A global policy refers to an embeddings policy without a pattern. By default, it is applied to up to 5000 repositories. The repositories matching the policy are first sorted by star count (descending) and id (descending) and then the first 5000 repositories are selected.

You can configure the limit by setting the `policyRepositoryMatchLimit` property in the embeddings configuration. A negative value disables the limit and all repositories are selected.

```json
{
  // [...]
  "embeddings": {
    // [...]
    "policyRepositoryMatchLimit": 5000
  }
}
```

## Limit the number of embeddings that can be generated

The number of embeddings that can be generated per repo is limited to `embeddings.maxCodeEmbeddingsPerRepo` for code embeddings (default 3.072.000) or `embeddings.maxTextEmbeddingsPerRepo` (default 512.000) for text embeddings.

Use the following site configuration to update the limits:

```json
{
  "embeddings": {
    "maxCodeEmbeddingsPerRepo": 3072000,
    "maxTextEmbeddingsPerRepo": 512000
  }
}
```
