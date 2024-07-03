package google

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"cloud.google.com/go/auth/credentials"

	"cloud.google.com/go/auth/httptransport"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	Gemini          APIFamily = "gemini-public"
	VertexGemini    APIFamily = "gemini-vertex"
	VertexAnthropic APIFamily = "anthropic"
)

func NewClient(httpCli httpcli.Doer, endpoint, accessToken string, viaGateway bool) (types.CompletionsClient, error) {
	apiFamily, client, err := determineAPIFamilyAndClient(endpoint, accessToken)
	if err != nil {
		return nil, err
	}
	return &googleCompletionStreamClient{
		httpCli:     httpCli,
		gcpCli:      client,
		accessToken: accessToken,
		endpoint:    endpoint,
		viaGateway:  viaGateway,
		apiFamily:   apiFamily,
	}, nil

}

func determineAPIFamilyAndClient(endpoint, accessToken string) (APIFamily, *http.Client, error) {
	// e.g. https://generativelanguage.googleapis.com/v1/models
	if endpoint == "" || strings.HasPrefix(endpoint, "https://generativelanguage.googleapis.com") {
		// Default to Gemini API if the endpoint contains "generativelanguage"
		return Gemini, nil, nil
	}

	var apiFamily APIFamily
	if strings.Contains(endpoint, "anthropic") {
		apiFamily = VertexAnthropic
	} else {
		apiFamily = VertexGemini
	}

	client, err := createHTTPClient(accessToken)
	if err != nil {
		return "", nil, errors.Wrap(err, "createHTTPClient")
	}

	return apiFamily, client, nil
}

func createHTTPClient(accessToken string) (*http.Client, error) {
	serviceAccountInfo, err := base64.StdEncoding.DecodeString(accessToken)
	if err != nil {
		return nil, errors.Wrap(err, "DecodeString")
	}

	creds, err := credentials.DetectDefault(&credentials.DetectOptions{
		Scopes:          []string{"https://www.googleapis.com/auth/cloud-platform"},
		CredentialsJSON: serviceAccountInfo,
	})
	if err != nil {
		return nil, errors.Wrap(err, "DetectDefault")
	}

	client, err := httptransport.NewClient(&httptransport.Options{
		Credentials: creds,
	})
	if err != nil {
		return nil, errors.Wrap(err, "NewClient")
	}

	return client, nil
}

func (c *googleCompletionStreamClient) Complete(
	ctx context.Context,
	logger log.Logger,
	request types.CompletionRequest) (*types.CompletionResponse, error) {
	if c.apiFamily == VertexAnthropic {
		return c.handleAnthropicComplete(ctx, request)
	} else {
		return c.handleGeminiComplete(ctx, request)
	}
}

func (c *googleCompletionStreamClient) handleAnthropicComplete(
	ctx context.Context,
	request types.CompletionRequest) (*types.CompletionResponse, error) {
	requestParams := request.Parameters
	resp, err := c.makeAnthopicRequest(ctx, requestParams, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response anthropicNonStreamingResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	if len(response.Content) == 0 {
		// Empty response.
		return &types.CompletionResponse{}, nil
	}

	if len(response.Content[0].Text) == 0 {
		// Empty response.
		return &types.CompletionResponse{}, nil
	}

	// NOTE: Candidates can be used to get multiple completions when CandidateCount is set,
	// which is not currently supported by Cody. For now, we only return the first completion.
	return &types.CompletionResponse{
		Completion: response.Content[0].Text,
	}, nil
}

func (c *googleCompletionStreamClient) handleGeminiComplete(
	ctx context.Context,
	request types.CompletionRequest) (*types.CompletionResponse, error) {
	requestParams := request.Parameters
	resp, err := c.makeGeminiRequest(ctx, requestParams, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response googleResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	if len(response.Candidates) == 0 {
		// Empty response.
		return &types.CompletionResponse{}, nil
	}

	if len(response.Candidates[0].Content.Parts) == 0 {
		// Empty response.
		return &types.CompletionResponse{}, nil
	}

	// NOTE: Candidates can be used to get multiple completions when CandidateCount is set,
	// which is not currently supported by Cody. For now, we only return the first completion.
	return &types.CompletionResponse{
		Completion: response.Candidates[0].Content.Parts[0].Text,
	}, nil
}

func (c *googleCompletionStreamClient) Stream(
	ctx context.Context,
	logger log.Logger,
	request types.CompletionRequest,
	sendEvent types.SendCompletionEvent) error {
	if c.apiFamily == VertexAnthropic {
		return c.handleVertexAnthropicStream(ctx, request, sendEvent)
	} else {
		return c.handleGeminiStream(ctx, request, sendEvent)
	}
}

func (c *googleCompletionStreamClient) handleGeminiStream(
	ctx context.Context,
	request types.CompletionRequest,
	sendEvent types.SendCompletionEvent,
) error {
	builder := types.NewCompletionResponseBuilder(request.Version)

	resp, err := c.makeGeminiRequest(ctx, request.Parameters, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	dec := NewDecoder(resp.Body)

	for dec.Scan() {
		if ctx.Err() != nil && ctx.Err() == context.Canceled {
			return nil
		}

		data := dec.Data()
		// Gracefully skip over any data that isn't JSON-like.
		if !bytes.HasPrefix(data, []byte("{")) {
			continue
		}

		var event googleResponse
		if err := json.Unmarshal(data, &event); err != nil {
			return errors.Errorf("failed to decode event payload: %w - body: %s", err, string(data))
		}

		if len(event.Candidates) > 0 && len(event.Candidates[0].Content.Parts) > 0 {
			err = sendEvent(builder.NextMessage(event.Candidates[0].Content.Parts[0].Text, nil))
			if err != nil {
				return err
			}

			if event.Candidates[0].FinishReason != "" {
				err = sendEvent(builder.Stop(event.Candidates[0].FinishReason))
				if err != nil {
					return err
				}
			}
		}
	}
	if dec.Err() != nil {
		return dec.Err()
	}

	return nil
}

func (c *googleCompletionStreamClient) handleVertexAnthropicStream(
	ctx context.Context,
	request types.CompletionRequest,
	sendEvent types.SendCompletionEvent,
) error {
	builder := types.NewCompletionResponseBuilder(request.Version)

	var resp *http.Response
	var err error

	defer (func() {
		if resp != nil {
			resp.Body.Close()
		}
	})()
	resp, err = c.makeAnthopicRequest(ctx, request.Parameters, true)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(resp.Body)
	var (
		event             []byte
		emptyMessageCount uint
		sentEvent         bool
	)
	for {
		rawLine, readErr := reader.ReadBytes('\n')
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return readErr
		}

		noSpaceLine := bytes.TrimSpace(rawLine)
		if len(noSpaceLine) == 0 {
			continue
		}

		if bytes.HasPrefix(noSpaceLine, []byte("event:")) {
			event = bytes.TrimSpace(bytes.TrimPrefix(noSpaceLine, []byte("event:")))
			continue
		}

		if bytes.HasPrefix(noSpaceLine, []byte("data:")) {
			data := bytes.TrimPrefix(noSpaceLine, []byte("data:"))
			eventType := string(event)

			switch eventType {
			case "message_start":
				// Handle message_start event
				var d anthropicStreamingResponse
				if err := json.Unmarshal(data, &d); err != nil {
					return err
				}
				// Process message_start event if needed
				continue

			case "content_block_delta":
				// Handle content_block_delta event
				var d anthropicStreamingResponse
				if err := json.Unmarshal(data, &d); err != nil {
					return err
				}
				sentEvent = true
				err = sendEvent(builder.NextMessage(d.Delta.Text, nil))
				if err != nil {
					return err
				}

				continue
			case "message_delta":
				// Handle message_delta event
				var d anthropicStreamingResponseTextBucket
				if err := json.Unmarshal(data, &d); err != nil {
					return err
				}
				// Process message_delta event if needed
				continue

			case "message_stop":
				// Handle message_stop event
				// Process message_stop event if needed
				// TODO: No stop word message here?

				continue

			default:
				// Handle other events if needed
				continue
			}
		}

		emptyMessageCount++
		if emptyMessageCount > 100 { // Adjust the limit as needed
			return errors.New("too many empty stream messages")
		}
	}

	if !sentEvent {
		return errors.New("stream closed with no events")
	}

	return nil

}

// makeRequest formats the request and calls the chat/completions endpoint for code_completion requests
func (c *googleCompletionStreamClient) makeGeminiRequest(ctx context.Context, requestParams types.CompletionRequestParameters, stream bool) (*http.Response, error) {
	apiURL := c.getAPIURL(requestParams, stream)
	endpointURL := apiURL.String()

	// Ensure TopK and TopP are non-negative
	requestParams.TopK = max(0, requestParams.TopK)
	requestParams.TopP = max(0, requestParams.TopP)
	// Generate the prompt
	prompt, err := getGeminiPrompt(requestParams.Messages)
	if err != nil {
		return nil, err
	}
	payload := googleRequest{
		Model:    requestParams.Model,
		Contents: prompt,
		GenerationConfig: googleGenerationConfig{
			Temperature:     requestParams.Temperature,
			TopP:            requestParams.TopP,
			TopK:            requestParams.TopK,
			MaxOutputTokens: requestParams.MaxTokensToSample,
			StopSequences:   requestParams.StopSequences,
		},
	}
	if c.viaGateway {
		endpointURL = c.endpoint
		// Add the Stream value to the payload if this is a Cody Gateway request,
		// as it is used for internal routing but not part of the Google API shape.
		payload.Stream = stream
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	// Vertex AI API requires an Authorization header with the access token.
	// Ref: https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/gemini#sample-requests
	if !c.viaGateway && !isDefaultAPIEndpoint(apiURL) {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewErrStatusNotOK("Google", resp)
	}

	return resp, nil
}

// makeRequest formats the request and calls the chat/completions endpoint for code_completion requests
func (c *googleCompletionStreamClient) makeAnthopicRequest(ctx context.Context, requestParams types.CompletionRequestParameters, stream bool) (*http.Response, error) {
	// Generate the prompt
	prompt, systemPrompt, err := getAnthropicPrompt(requestParams.Messages)
	if err != nil {
		return nil, err
	}

	payload := anthropicRequest{
		Messages:         prompt,
		MaxTokens:        requestParams.MaxTokensToSample,
		Stream:           stream,
		AnthropicVersion: "vertex-2023-10-16",
		System:           systemPrompt,
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	apiURL := c.getAPIURL(requestParams, stream)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL.String(), bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.gcpCli.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewErrStatusNotOK("Google", resp)
	}

	return resp, nil
}

// In the latest API Docs, the model name and API key must be used with the default API endpoint URL.
// Ref: https://ai.google.dev/gemini-api/docs/get-started/tutorial?lang=rest#gemini_and_content_based_apis
func (c *googleCompletionStreamClient) getAPIURL(requestParams types.CompletionRequestParameters, stream bool) *url.URL {
	apiURL, err := url.Parse(c.endpoint)
	if err != nil {
		apiURL = &url.URL{
			Scheme: "https",
			Host:   defaultAPIHost,
			Path:   defaultAPIPath,
		}
	}

	apiURL.Path = path.Join(apiURL.Path, requestParams.Model) + ":" + getgRPCMethod(stream, c.apiFamily)

	// We need to append the API key to the default API endpoint URL.
	if isDefaultAPIEndpoint(apiURL) {
		query := apiURL.Query()
		query.Set("key", c.accessToken)
		if stream {
			query.Set("alt", "sse")
		}
		apiURL.RawQuery = query.Encode()
	}

	return apiURL
}

// getgRPCMethod returns the gRPC method name based on the stream flag.
func getgRPCMethod(stream bool, apiFamily APIFamily) string {
	if apiFamily == VertexAnthropic {
		return "streamRawPredict"
	}
	if stream {
		return "streamGenerateContent"
	}
	return "generateContent"
}

// isDefaultAPIEndpoint checks if the given API endpoint URL is the default API endpoint.
// The default API endpoint is determined by the defaultAPIHost constant.
func isDefaultAPIEndpoint(endpoint *url.URL) bool {
	return endpoint.Host == defaultAPIHost
}
