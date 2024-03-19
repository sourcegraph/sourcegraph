# Sourcegraph Cody Gateway

<p class="subtitle">Learn how Cody Gateway powers the default Sourcegraph provider for completions and embeddings to facilitate Cody features.</p>

<aside class="badge">
<p> <span style="margin-right:0.25rem;" class="badge badge-note">Sourcegraph 5.1+</span> Cody Gateway is supported for Sourcegraph Enterprise customers on version 5.1 or more. </p>
</aside>

Sourcegraph Cody Gateway powers the default `"provider": "sourcegraph"`, Cody completions and embeddings for Sourcegraph Enterprise users. It supports a variety of upstream LLM providers, such as [Anthropic](https://www.anthropic.com/) and [OpenAI](https://openai.com/), with rate limits, quotas, and model availability tied to your Sourcegraph Enterprise subscription.

Code snippets are sent to these third-party LLM providers when you use the Cody extension or enable embeddings. Make sure you review the [Cody usage and privacy notice](https://sourcegraph.com/terms/cody-notice).

## Using Cody Gateway in Sourcegraph Enterprise

To enable completions and embeddings provided by Cody Gateway on your Sourcegraph Enterprise instance, make sure your license key is set, and Cody is enabled in your [site configuration](../../admin/config/site_config.md):

```jsonc
{
  "licenseKey": "<...>",
  "cody.enabled": true,
}
```

After adding the license key, the default configuration and authentication will be automatically applied.

For more details about configuring Cody, read the following resources:

- [Enabling Cody for Sourcegraph Enterprise](./../overview/enable-cody-enterprise.md)
- [Code Graph Context: Embeddings](./code-graph.md#embeddings)

Cody Gateway is hosted at `cody-gateway.sourcegraph.com`. To use Cody Gateway, your Sourcegraph instance must be connected to the service in this domain.

> WARNING: Sourcegraph Cody Gateway access must be included in your Sourcegraph Enterprise subscription plan. You can verify it by checking it with your account manager. If you are a [Sourcegraph Cloud](../../cloud/index.md) user, Cody is enabled by default on your instance starting with Sourcegraph 5.1.

## Configuring custom models

To configure custom models for various Cody configurations (for example, `"completions"` and `"embeddings"`), specify the desired model with the upstream provider as a prefix to the name of the model. For example, to use the `claude-2` model from Anthropic, you would configure:

```json
{
  "completions": { "chatModel": "anthropic/claude-2" },
}
```

The currently supported upstream providers for models are:

- [`anthropic/`](https://www.anthropic.com/)
- [`openai/`](https://openai.com/)

For Sourcegraph Enterprise customers, model availability depends on your Sourcegraph Enterprise subscription.

> WARNING: When using OpenAI models for completions, only chat completions will work - code completions are currently unsupported.

## Rate limits and quotas

Rate limits, quotas, and model availability are tied to one of the following:

- your Sourcegraph Enterprise product subscription for Sourcegraph Enterprise instances
- your Sourcegraph.com account, for Cody Free and Pro

All successful requests to Cody Gateway will count toward your rate limits. Unsuccessful requests are not counted as usage.

Rate limits, quotas, and model availability are also configured per Cody feature - for example, you will have separate rate limits for Cody chat, Cody completions, and Cody embeddings.

In addition to the above, we may throttle concurrent requests to Cody Gateway per Sourcegraph Enterprise instance or Cody Free/Pro user to prevent excessive burst consumption.

>NOTE: You can reach out for more details about Sourcegraph Cody Gateway access available to you and how you can gain access to higher rate limits, quotas, and/or model options.

## Privacy and security

Sourcegraph Cody Gateway does not retain sensitive data (prompt test and source code included in requests, etc.) from any traffic received. Only rate limit consumption per Sourcegraph Enterprise subscription and some high-level diagnostic data (error codes from upstream, numeric/enum request parameters, etc) are tracked.

The code that powers Cody Gateway is also [source-available](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph$+f:cmd/cody-gateway+lang:go&patternType=lucky&sm=1&groupBy=path) for audit.

For more details about Cody Gateway security practices, please reach out to your account manager. You can also refer to the [Cody usage and privacy notice](https://sourcegraph.com/terms/cody-notice) for more privacy details about Cody in general.
