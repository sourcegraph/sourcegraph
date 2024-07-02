# How to set up Cody Gateway locally

> WARNING: This is a development guide - to use Cody Gateway for Sourcegraph, refer to [Sourcegraph Cody Gateway](https://sourcegraph.com/docs/cody/core-concepts/cody-gateway).

This guide documents how to set up [Cody Gateway](https://handbook.sourcegraph.com/departments/engineering/teams/cody/cody-gateway/) locally for development.

To get started, Cody Gateway is included in the standard `dotcom` run set.
Since Cody Gateway currently depends on Sourcegraph.com, there's not much point running any other run set.

```sh
sg start dotcom
```

*However*, the locally running Cody Gateway is not used by default - `dev-private` credentials point towards a [shared development instance of Cody Gateway](https://handbook.sourcegraph.com/departments/engineering/teams/cody/cody-gateway/).
To use the locally running Cody Gateway, follow the steps in [use a locally running Cody Gateway](#use-a-locally-running-cody-gateway)

## Use a locally running Cody Gateway

To use this locally running Cody Gateway from your local Sourcegraph instance, configure Cody features to talk to your local Cody Gateway in site configuration, similar to what [customers do to enable Cody Enterprise](https://sourcegraph.com/docs/cody/overview/enable-cody-enterprise):

```json
{
  "completions": {
    "enabled": true,
    "provider": "sourcegraph",
    "endpoint": "http://localhost:9992",
    "chatModel": "anthropic/claude-3-sonnet-20240229",
    "completionModel": "fireworks/starcoder",
    // Create an Enterprise subscription and license key:
    // https://sourcegraph.test:3443/site-admin/dotcom/product/subscriptions
    // Under "Cody services", ensure access is enabled and get the access token
    // to use here.
    // Note that the license and tokens will only work locally.
    "accessToken": "..."
  }
}
```

Similar values can be [configured for embeddings](https://sourcegraph.com/docs/cody/core-concepts/embeddings) to use embeddings through your local Cody Gateway isntead.

Now, we need to make sure your local Cody Gateway instance can access upstream LLM services.
Add the following to your `sg.config.overwrite.yaml`:

```yaml
commands:
  cody-gateway:
    env:
      # Access token for accessing Anthropic:
      # https://start.1password.com/open/i?a=HEDEDSLHPBFGRBTKAKJWE23XX4&h=my.1password.com&i=athw572l6xqqvtnbbgadevgbqi&v=dnrhbauihkhjs5ag6vszsme45a
      CODY_GATEWAY_ANTHROPIC_ACCESS_TOKEN: "..."
      # Set other external API tokens as needed
      CODY_GATEWAY_FIREWORKS_ACCESS_TOKEN: "..."
      # Create a personal access token on https://sourcegraph.test:3443/user/settings/tokens
      # for your local site admin user. This allows your local Cody Gateway to
      # access user information in the Sourcegraph instance.
      #
      # IMPORTANT: The token needs to belong to a site admin, or have additional
      # roles and permissions. Your end user token will not be sufficient.
      CODY_GATEWAY_DOTCOM_ACCESS_TOKEN: "..."
      # For working with embeddings, set the following values - it's recommended to use the dev deployment.
      CODY_GATEWAY_SOURCEGRAPH_EMBEDDINGS_API_URL: 'https://embeddings.sgdev.org/v2/models/st-multi-qa-mpnet-base-dot-v1/infer' # Replace model name as needed
      # Available in https://team-sourcegraph.1password.com/vaults/dnrhbauihkhjs5ag6vszsme45a/allitems/nigajmdgojg3uwzd5237jfoygm
      CODY_GATEWAY_SOURCEGRAPH_EMBEDDINGS_API_TOKEN: "..."
```

For more configuration options, refer to the [configuration source code](https://github.com/sourcegraph/sourcegraph/blob/main/cmd/cody-gateway/shared/config.go#L60).

Then, restart `sg start dotcom` and try interacting with Cody!
If you'll be sending requests to Cody Gateway manually, you'll need to convert the Sourcegraph personal access token into a Cody Gateway access token. Run `sg cody-gateway gen-token sgp_<token>...` to generate a token that starts with `sgd_` and can be used in requests to CG as a bearer token.

## Observing Cody Gateway

You can get full tracing of Cody interactions, including Cody Gateway parts, with `sg start monitoring`'s Jaeger component.
Cody Gateway will also emit traces of its background jobs.

All event logging is output as standard logs in development under the `cody-gateway.events` log scope.

## Working with BigQuery for events logging

> NOTE: To make authentication work magically with the GCP, you need to install the [`gcloud`](https://cloud.google.com/sdk/docs/install-sdk) CLI and
run `gcloud auth login --project cody-gateway-dev` once.

To send events to BigQuery while developing Cody Gateway locally, add the following to your `sg.config.overwrite.yaml`:

```yaml
commands:
  cody-gateway:
    env:
      CODY_GATEWAY_BIGQUERY_PROJECT_ID: cody-gateway-dev
```

Then to view events statistics on the Enterprise subscriptions page, add the following section in the site configuration, and run the `sg start dotcom` stack:

```json
{
  "dotcom": {
    "codyGateway": {
      "bigQueryGoogleProjectID": "cody-gateway-dev",
      "bigQueryDataset": "cody_gateway",
      "bigQueryTable": "events"
    }
  }
}
```
