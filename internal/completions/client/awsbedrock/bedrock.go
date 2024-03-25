package awsbedrock

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func NewClient(cli httpcli.Doer, endpoint, accessToken string) types.CompletionsClient {
	return &awsBedrockAnthropicCompletionStreamClient{
		cli:         cli,
		accessToken: accessToken,
		endpoint:    endpoint,
	}
}

const (
	clientID = "sourcegraph/1.0"
)

type awsBedrockAnthropicCompletionStreamClient struct {
	cli         httpcli.Doer
	accessToken string
	endpoint    string
}

func (c *awsBedrockAnthropicCompletionStreamClient) Complete(
	ctx context.Context,
	feature types.CompletionsFeature,
	_ types.CompletionsVersion,
	requestParams types.CompletionRequestParameters,
) (*types.CompletionResponse, error) {
	resp, err := c.makeRequest(ctx, requestParams, false)
	if err != nil {
		return nil, errors.Wrap(err, "making request")
	}
	defer resp.Body.Close()

	var response bedrockAnthropicCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, errors.Wrap(err, "decoding response")
	}

	return &types.CompletionResponse{
		Completion: response.Completion,
		StopReason: response.StopReason,
	}, nil
}

func (a *awsBedrockAnthropicCompletionStreamClient) Stream(
	ctx context.Context,
	feature types.CompletionsFeature,
	_ types.CompletionsVersion,
	requestParams types.CompletionRequestParameters,
	sendEvent types.SendCompletionEvent,
) error {
	resp, err := a.makeRequest(ctx, requestParams, true)
	if err != nil {
		return errors.Wrap(err, "making request")
	}
	defer resp.Body.Close()

	// totalCompletion is the complete completion string, bedrock already uses
	// the new incremental Anthropic API, but our clients still expect a full
	// response in each event.
	var totalCompletion string
	dec := eventstream.NewDecoder()
	// Allocate a 1 MB buffer for decoding.
	buf := make([]byte, 0, 1024*1024)
	for {
		m, err := dec.Decode(resp.Body, buf)
		// Exit early on context cancellation.
		if ctx.Err() != nil && ctx.Err() == context.Canceled {
			return nil
		}

		// AWS's event stream decoder returns EOF once completed, so return.
		if err == io.EOF {
			return nil
		}

		// For any other error, return.
		if err != nil {
			return err
		}

		// Unmarshal the event payload from the stream.
		var p awsEventStreamPayload
		if err := json.Unmarshal(m.Payload, &p); err != nil {
			return errors.Wrap(err, "unmarshaling event payload")
		}

		data := p.Bytes

		// Gracefully skip over any data that isn't JSON-like. Anthropic's API sometimes sends
		// non-documented data over the stream, like timestamps.
		if !bytes.HasPrefix(data, []byte("{")) {
			continue
		}

		var event bedrockAnthropicCompletionResponse
		if err := json.Unmarshal(data, &event); err != nil {
			return errors.Errorf("failed to decode event payload: %w - body: %s", err, string(data))
		}

		// Collect the whole completion, AWS already uses the new Anthropic API
		// that sends partial completion results, but our clients still expect
		// a fill completion to be returned.
		totalCompletion += event.Completion

		err = sendEvent(types.CompletionResponse{
			Completion: totalCompletion,
			StopReason: event.StopReason,
		})
		if err != nil {
			return errors.Wrap(err, "sending event")
		}
	}
}

type awsEventStreamPayload struct {
	Bytes []byte `json:"bytes"`
}

func (c *awsBedrockAnthropicCompletionStreamClient) makeRequest(ctx context.Context, requestParams types.CompletionRequestParameters, stream bool) (*http.Response, error) {
	defaultConfig, err := config.LoadDefaultConfig(ctx, awsConfigOptsForKeyConfig(c.endpoint, c.accessToken)...)
	if err != nil {
		return nil, errors.Wrap(err, "loading aws config")
	}

	creds, err := defaultConfig.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving aws credentials")
	}

	if requestParams.TopK == -1 {
		requestParams.TopK = 0
	}

	if requestParams.TopP == -1 {
		requestParams.TopP = 0
	}

	prompt, err := GetPrompt(requestParams.Messages)
	if err != nil {
		return nil, err
	}
	// Backcompat: Remove this code once enough clients are upgraded and we drop the
	// Prompt field on requestParams.
	if prompt == "" {
		prompt = requestParams.Prompt
	}

	if len(requestParams.StopSequences) == 0 {
		requestParams.StopSequences = []string{HUMAN_PROMPT}
	}

	if requestParams.MaxTokensToSample == 0 {
		requestParams.MaxTokensToSample = 300
	}

	payload := bedrockAnthropicCompletionsRequestParameters{
		StopSequences:     requestParams.StopSequences,
		Temperature:       requestParams.Temperature,
		MaxTokensToSample: requestParams.MaxTokensToSample,
		TopP:              requestParams.TopP,
		TopK:              requestParams.TopK,
		Prompt:            prompt,
		// Hard coded for now, so we don't accidentally get a newer API response
		// we don't support.
		AnthropicVersion: "bedrock-2023-05-31",
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, errors.Wrap(err, "marshalling request body")
	}
	apiURL, err := url.Parse(c.endpoint)
	if err != nil || apiURL.Scheme == "" {
		apiURL = &url.URL{
			Scheme: "https",
			Host:   fmt.Sprintf("bedrock-runtime.%s.amazonaws.com", defaultConfig.Region),
		}
	}

	if stream {
		apiURL.Path = fmt.Sprintf("/model/%s/invoke-with-response-stream", requestParams.Model)
	} else {
		apiURL.Path = fmt.Sprintf("/model/%s/invoke", requestParams.Model)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL.String(), bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	// Sign the request with AWS credentials.
	hash := sha256.Sum256(reqBody)
	if err := v4.NewSigner().SignHTTP(ctx, creds, req, hex.EncodeToString(hash[:]), "bedrock", defaultConfig.Region, time.Now()); err != nil {
		return nil, errors.Wrap(err, "signing request")
	}

	req.Header.Set("Cache-Control", "no-cache")
	if stream {
		req.Header.Set("Accept", "application/vnd.amazon.eventstream")
	} else {
		req.Header.Set("Accept", "application/json")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Client", clientID)
	req.Header.Set("X-Amzn-Bedrock-Accept", "*/*")
	// Don't store the prompt in the prompt history.
	req.Header.Set("X-Amzn-Bedrock-Save", "false")

	// Make the request.
	resp, err := c.cli.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "make request to bedrock")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewErrStatusNotOK("AWS Bedrock", resp)
	}

	return resp, nil
}

func awsConfigOptsForKeyConfig(endpoint string, accessToken string) []func(*config.LoadOptions) error {
	configOpts := []func(*config.LoadOptions) error{}
	if endpoint != "" {
		apiURL, err := url.Parse(endpoint)
		if err != nil || apiURL.Scheme == "" { // this is not a url assume it is a region
			configOpts = append(configOpts, config.WithRegion(endpoint))
		} else { // this is a url just use it directly
			configOpts = append(configOpts, config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
				func(service, region string, options ...interface{}) (aws.Endpoint, error) {
					return aws.Endpoint{URL: endpoint}, nil
				})))
		}
	}

	// We use the accessToken field to provide multiple values.
	// If it consists of two parts, separated by a `:`, the first part is
	// the aws access key, and the second is the aws secret key.
	// If there are three parts, the third part is the aws session token.
	// If no access token is given, we default to the AWS default credential provider
	// chain, which supports all basic known ways of connecting to AWS.
	if accessToken != "" {
		parts := strings.SplitN(accessToken, ":", 3)
		if len(parts) == 2 {
			configOpts = append(configOpts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(parts[0], parts[1], "")))
		} else if len(parts) == 3 {
			configOpts = append(configOpts, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(parts[0], parts[1], parts[2])))
		}
	}

	return configOpts
}

type bedrockAnthropicCompletionsRequestParameters struct {
	Prompt            string   `json:"prompt"`
	Temperature       float32  `json:"temperature,omitempty"`
	MaxTokensToSample int      `json:"max_tokens_to_sample"`
	StopSequences     []string `json:"stop_sequences,omitempty"`
	TopK              int      `json:"top_k,omitempty"`
	TopP              float32  `json:"top_p,omitempty"`
	AnthropicVersion  string   `json:"anthropic_version"`
}

type bedrockAnthropicCompletionResponse struct {
	Completion string `json:"completion"`
	StopReason string `json:"stop_reason"`
}
