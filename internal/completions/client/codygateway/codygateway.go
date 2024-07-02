package codygateway

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/codygateway"
	"github.com/sourcegraph/sourcegraph/internal/completions/client/anthropic"
	"github.com/sourcegraph/sourcegraph/internal/completions/client/fireworks"
	"github.com/sourcegraph/sourcegraph/internal/completions/client/google"
	"github.com/sourcegraph/sourcegraph/internal/completions/client/openai"
	"github.com/sourcegraph/sourcegraph/internal/completions/tokenusage"
	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// NewClient instantiates a completions provider backed by Sourcegraph's managed
// Cody Gateway service.
func NewClient(cli httpcli.Doer, endpoint, accessToken string, tokenizer tokenusage.Manager) (types.CompletionsClient, error) {
	gatewayURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	return &codyGatewayClient{
		upstream:    cli,
		gatewayURL:  gatewayURL,
		accessToken: accessToken,
		tokenizer:   tokenizer,
	}, nil
}

type codyGatewayClient struct {
	upstream    httpcli.Doer
	gatewayURL  *url.URL
	accessToken string
	tokenizer   tokenusage.Manager
}

func (c *codyGatewayClient) Stream(
	ctx context.Context, logger log.Logger, request types.CompletionRequest, responseMetadataCapture *types.ResponseMetadataCapture) error {
	cc, err := c.clientForParams(request.Feature, &request.Parameters)
	if err != nil {
		return err
	}

	err = cc.Stream(ctx, logger, request, responseMetadataCapture)
	return overwriteErrSource(err)
}

func (c *codyGatewayClient) Complete(ctx context.Context, logger log.Logger, request types.CompletionRequest) (*types.CompletionResponse, error) {
	cc, err := c.clientForParams(request.Feature, &request.Parameters)
	if err != nil {
		return nil, err
	}
	resp, err := cc.Complete(ctx, logger, request)
	return resp, overwriteErrSource(err)
}

// overwriteErrSource should be used on all errors returned by an underlying
// types.CompletionsClient to avoid confusing error messages.
func overwriteErrSource(err error) error {
	if err == nil {
		return nil
	}
	if statusErr, ok := types.IsErrStatusNotOK(err); ok {
		statusErr.Source = "Sourcegraph Cody Gateway"
	}
	return err
}

func (c *codyGatewayClient) clientForParams(feature types.CompletionsFeature, requestParams *types.CompletionRequestParameters) (types.CompletionsClient, error) {
	// Extract provider and model from the Cody Gateway model format and override
	// the request parameter's model.
	provider, model := getProviderFromGatewayModel(strings.ToLower(requestParams.Model))
	requestParams.Model = model

	// Based on the provider, instantiate the appropriate client backed by a
	// gatewayDoer that authenticates against the Gateway's API.
	switch provider {
	case string(conftypes.CompletionsProviderNameAnthropic):
		return anthropic.NewClient(gatewayDoer(c.upstream, feature, c.gatewayURL, c.accessToken, "/v1/completions/anthropic-messages"), "", "", true, c.tokenizer), nil
	case string(conftypes.CompletionsProviderNameOpenAI):
		return openai.NewClient(gatewayDoer(c.upstream, feature, c.gatewayURL, c.accessToken, "/v1/completions/openai"), "", "", c.tokenizer), nil
	case string(conftypes.CompletionsProviderNameFireworks):
		return fireworks.NewClient(gatewayDoer(c.upstream, feature, c.gatewayURL, c.accessToken, "/v1/completions/fireworks"), "", ""), nil
	case string(conftypes.CompletionsProviderNameGoogle):
		return google.NewClient(gatewayDoer(c.upstream, feature, c.gatewayURL, c.accessToken, "/v1/completions/google"), "", "", true)
	case "":
		return nil, errors.Newf("no provider provided in model %s - a model in the format '$PROVIDER/$MODEL_NAME' is expected", model)
	default:
		return nil, errors.Newf("no client known for upstream provider %s", provider)
	}
}

// getProviderFromGatewayModel extracts the model provider from Cody Gateway
// configuration's expected model naming format, "$PROVIDER/$MODEL_NAME".
// If a prefix isn't present, the whole value is assumed to be the model.
func getProviderFromGatewayModel(gatewayModel string) (provider string, model string) {
	parts := strings.SplitN(gatewayModel, "/", 2)
	if len(parts) < 2 {
		return "", parts[0] // assume it's the provider that's missing, not the model.
	}
	return parts[0], parts[1]
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (rt roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt(req)
}

// gatewayDoer redirects requests to Cody Gateway with all prerequisite headers.
func gatewayDoer(upstream httpcli.Doer, feature types.CompletionsFeature, gatewayURL *url.URL, accessToken, path string) httpcli.Doer {
	return httpcli.DoerFunc(func(req *http.Request) (*http.Response, error) {
		req.Host = gatewayURL.Host
		req.URL = gatewayURL
		req.URL.Path = path
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
		req.Header.Set(codygateway.FeatureHeaderName, string(feature))

		// HACK: Add actor transport directly. We tried adding the actor transport
		// in https://github.com/sourcegraph/sourcegraph/commit/6b058221ca87f5558759d92c0d72436cede70dc4
		// but it doesn't seem to work.
		resp, err := (&actor.HTTPTransport{
			RoundTripper: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
				return upstream.Do(r)
			}),
		}).RoundTrip(req)

		// If we get a response, record Cody Gateway's x-trace response header,
		// so that we can link up to an event on our end if needed.
		if resp != nil && resp.Header != nil {
			if span := trace.SpanFromContext(req.Context()); span.SpanContext().IsValid() {
				// Would be cool if we can make an OTEL trace link instead, but
				// adding a link after a span has started is not supported yet:
				// https://github.com/open-telemetry/opentelemetry-specification/issues/454
				span.SetAttributes(attribute.String("cody-gateway.x-trace", resp.Header.Get("X-Trace")))
				span.SetAttributes(attribute.String("cody-gateway.x-trace-span", resp.Header.Get("X-Trace-Span")))
			}
		}

		return resp, err
	})
}
