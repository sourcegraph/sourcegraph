package azureopenai

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"golang.org/x/net/http2"

	"github.com/sourcegraph/sourcegraph/internal/completions/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// HTTP proxy value to be used for id token requests to Azure
// This value will only used when using an access token is not provided
// and it will only apply to requests made to the Azure authentication endpoint
// not other requests such as to the OpenAI API
var authProxyURL = os.Getenv("CODY_AZURE_OPENAI_IDENTITY_HTTP_PROXY")

// We want to reuse the client because when using the DefaultAzureCredential
// it will acquire a short lived token and reusing the client
// prevents acquiring a new token on every request.
// The client will refresh the token as needed.

var apiClient completionsClient

type completionsClient struct {
	mu          sync.RWMutex
	accessToken string
	endpoint    string
	client      *azopenai.Client
}

type CompletionsClient interface {
	GetCompletionsStream(ctx context.Context, body azopenai.CompletionsOptions, options *azopenai.GetCompletionsStreamOptions) (azopenai.GetCompletionsStreamResponse, error)
	GetCompletions(ctx context.Context, body azopenai.CompletionsOptions, options *azopenai.GetCompletionsOptions) (azopenai.GetCompletionsResponse, error)
	GetChatCompletions(ctx context.Context, body azopenai.ChatCompletionsOptions, options *azopenai.GetChatCompletionsOptions) (azopenai.GetChatCompletionsResponse, error)
	GetChatCompletionsStream(ctx context.Context, body azopenai.ChatCompletionsOptions, options *azopenai.GetChatCompletionsStreamOptions) (azopenai.GetChatCompletionsStreamResponse, error)
}

func GetAPIClient(endpoint, accessToken string) (CompletionsClient, error) {
	apiClient.mu.RLock()
	if apiClient.client != nil && apiClient.endpoint == endpoint && apiClient.accessToken == accessToken {
		apiClient.mu.RUnlock()
		return apiClient.client, nil
	}
	apiClient.mu.RUnlock()
	apiClient.mu.Lock()
	defer apiClient.mu.Unlock()

	// API Versions and docs https://learn.microsoft.com/en-us/azure/ai-services/openai/reference#completions
	clientOpts := &azopenai.ClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: apiVersionClient("2023-05-15"),
		},
	}
	var err error
	if accessToken != "" {
		credential := azcore.NewKeyCredential(accessToken)
		apiClient.client, err = azopenai.NewClientWithKeyCredential(endpoint, credential, clientOpts)
	} else {
		var opts *azidentity.DefaultAzureCredentialOptions
		opts, err = getCredentialOptions()
		if err != nil {
			return nil, err
		}
		credential, credErr := azidentity.NewDefaultAzureCredential(opts)
		if credErr != nil {
			return nil, credErr
		}
		apiClient.endpoint = endpoint

		apiClient.client, err = azopenai.NewClient(endpoint, credential, clientOpts)
	}
	return apiClient.client, err

}

func getCredentialOptions() (*azidentity.DefaultAzureCredentialOptions, error) {
	// if there is no proxy we don't need any options
	if authProxyURL == "" {
		return nil, nil
	}

	proxyUrl, err := url.Parse(authProxyURL)
	if err != nil {
		return nil, err
	}
	proxiedClient := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}
	return &azidentity.DefaultAzureCredentialOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: proxiedClient,
		},
	}, nil

}

type GetCompletionsAPIClientFunc func(accessToken, endpoint string) (CompletionsClient, error)

func NewClient(getClient GetCompletionsAPIClientFunc, accessToken, endpoint string) (types.CompletionsClient, error) {
	client, err := getClient(accessToken, endpoint)
	if err != nil {
		return nil, err
	}
	return &azureCompletionClient{
		client: client,
	}, nil
}

type azureCompletionClient struct {
	client CompletionsClient
}

func (c *azureCompletionClient) Complete(
	ctx context.Context,
	feature types.CompletionsFeature,
	requestParams types.CompletionRequestParameters,
) (*types.CompletionResponse, error) {

	switch feature {
	case types.CompletionsFeatureCode:
		return completeAutocomplete(ctx, c.client, requestParams)
	case types.CompletionsFeatureChat:
		return completeChat(ctx, c.client, requestParams)
	default:
		return nil, errors.New("invalid completions feature")
	}
}

func completeAutocomplete(
	ctx context.Context,
	client CompletionsClient,
	requestParams types.CompletionRequestParameters,
) (*types.CompletionResponse, error) {
	options, err := getCompletionsOptions(requestParams)
	if err != nil {
		return nil, err
	}
	response, err := client.GetCompletions(ctx, options, nil)
	if err != nil {
		return nil, toStatusCodeError(err)
	}

	// Text and FinishReason are documented as REQUIRED but checking just to be safe
	if !hasValidFirstCompletionsChoice(response.Choices) {
		return &types.CompletionResponse{}, nil
	}
	return &types.CompletionResponse{
		Completion: *response.Choices[0].Text,
		StopReason: string(*response.Choices[0].FinishReason),
	}, nil
}

func completeChat(
	ctx context.Context,
	client CompletionsClient,
	requestParams types.CompletionRequestParameters,
) (*types.CompletionResponse, error) {
	response, err := client.GetChatCompletions(ctx, getChatOptions(requestParams), nil)
	if err != nil {
		return nil, toStatusCodeError(err)
	}
	if !hasValidFirstChatChoice(response.Choices) {
		return &types.CompletionResponse{}, nil
	}
	return &types.CompletionResponse{
		Completion: *response.Choices[0].Delta.Content,
		StopReason: string(*response.Choices[0].FinishReason),
	}, nil
}

func (c *azureCompletionClient) Stream(
	ctx context.Context,
	feature types.CompletionsFeature,
	requestParams types.CompletionRequestParameters,
	sendEvent types.SendCompletionEvent,
) error {
	switch feature {
	case types.CompletionsFeatureCode:
		return streamAutocomplete(ctx, c.client, requestParams, sendEvent)
	case types.CompletionsFeatureChat:
		return streamChat(ctx, c.client, requestParams, sendEvent)
	default:
		return errors.New("invalid completions feature")
	}
}

func streamAutocomplete(
	ctx context.Context,
	client CompletionsClient,
	requestParams types.CompletionRequestParameters,
	sendEvent types.SendCompletionEvent,
) error {
	options, err := getCompletionsOptions(requestParams)
	if err != nil {
		return err
	}
	resp, err := client.GetCompletionsStream(ctx, options, nil)
	if err != nil {
		return toStatusCodeError(err)
	}
	defer resp.CompletionsStream.Close()

	// Azure sends incremental deltas for each message in a chat stream
	// build up the full message content over multiple responses
	var content string
	for {
		entry, err := resp.CompletionsStream.Read()
		// stream is done
		if errors.Is(err, io.EOF) {
			return nil
		}
		// some other error has occured
		if err != nil {
			return err
		}

		// hasValidFirstCompletionsChoice checks for a valid 1st choice which has text
		if hasValidFirstCompletionsChoice(entry.Choices) {
			content += *entry.Choices[0].Text
			finish := ""
			if entry.Choices[0].FinishReason != nil {
				finish = string(*entry.Choices[0].FinishReason)
			}
			ev := types.CompletionResponse{
				Completion: content,
				StopReason: finish,
			}
			err := sendEvent(ev)
			if err != nil {
				return err
			}
		}
	}
}

func streamChat(
	ctx context.Context,
	client CompletionsClient,
	requestParams types.CompletionRequestParameters,
	sendEvent types.SendCompletionEvent,
) error {

	resp, err := client.GetChatCompletionsStream(ctx, getChatOptions(requestParams), nil)
	if err != nil {
		return toStatusCodeError(err)
	}
	defer resp.ChatCompletionsStream.Close()

	// Azure sends incremental deltas for each message in a chat stream
	// build up the full message content over multiple responses
	var content string
	for {
		entry, err := resp.ChatCompletionsStream.Read()
		// stream is done
		if errors.Is(err, io.EOF) {
			return nil
		}
		// some other error has occurred
		if err != nil {
			return err
		}

		if hasValidFirstChatChoice(entry.Choices) {
			// hasValidFirstChatChoice checks that Delta.Content isn't null
			// it is marked as REQUIRED in docs despite being a pointer
			content += *entry.Choices[0].Delta.Content

			finish := ""
			// FinishReason is marked as REQUIRED but it's nil until the end
			if entry.Choices[0].FinishReason != nil {
				finish = string(*entry.Choices[0].FinishReason)
			}
			ev := types.CompletionResponse{
				Completion: content,
				StopReason: finish,
			}
			err := sendEvent(ev)
			if err != nil {
				return err
			}
		}
	}
}

// hasValidChatChoice checks to ensure there is a choice and the first one contains non-nil values
func hasValidFirstChatChoice(choices []azopenai.ChatChoice) bool {
	return len(choices) > 0 &&
		choices[0].Delta != nil &&
		choices[0].Delta.Content != nil
}

// hasValidChatChoice checks to ensure there is a choice and the first one contains non-nil values
func hasValidFirstCompletionsChoice(choices []azopenai.Choice) bool {
	return len(choices) > 0 &&
		choices[0].Text != nil
}

func getChatMessages(messages []types.Message) []azopenai.ChatRequestMessageClassification {
	azureMessages := make([]azopenai.ChatRequestMessageClassification, len(messages))
	for i, m := range messages {
		message := m.Text
		switch m.Speaker {
		case types.HUMAN_MESSAGE_SPEAKER:
			azureMessages[i] = &azopenai.ChatRequestUserMessage{Content: azopenai.NewChatRequestUserMessageContent(message)}
		case types.ASISSTANT_MESSAGE_SPEAKER:
			azureMessages[i] = &azopenai.ChatRequestAssistantMessage{Content: &message}
		}

	}
	return azureMessages
}

func getChatOptions(requestParams types.CompletionRequestParameters) azopenai.ChatCompletionsOptions {
	if requestParams.TopK < 0 {
		requestParams.TopK = 0
	}
	if requestParams.TopP < 0 {
		requestParams.TopP = 0
	}
	return azopenai.ChatCompletionsOptions{
		Messages:       getChatMessages(requestParams.Messages),
		Temperature:    &requestParams.Temperature,
		TopP:           &requestParams.TopP,
		N:              intToInt32Ptr(1),
		Stop:           requestParams.StopSequences,
		MaxTokens:      intToInt32Ptr(requestParams.MaxTokensToSample),
		DeploymentName: &requestParams.Model,
	}
}

func getCompletionsOptions(requestParams types.CompletionRequestParameters) (azopenai.CompletionsOptions, error) {
	if requestParams.TopK < 0 {
		requestParams.TopK = 0
	}
	if requestParams.TopP < 0 {
		requestParams.TopP = 0
	}
	prompt, err := getPrompt(requestParams.Messages)
	if err != nil {
		return azopenai.CompletionsOptions{}, err
	}
	return azopenai.CompletionsOptions{
		Prompt:         []string{prompt},
		Temperature:    &requestParams.Temperature,
		TopP:           &requestParams.TopP,
		N:              intToInt32Ptr(1),
		Stop:           requestParams.StopSequences,
		MaxTokens:      intToInt32Ptr(requestParams.MaxTokensToSample),
		DeploymentName: &requestParams.Model,
	}, nil
}

func getPrompt(messages []types.Message) (string, error) {
	if len(messages) != 1 {
		return "", errors.New("Expected to receive exactly one message with the prompt")
	}

	return messages[0].Text, nil
}

func intToInt32Ptr(i int) *int32 {
	v := int32(i)
	return &v
}

// toStatusCodeError converts Azure SDK ResponseError to a ErrStatusNotOK error
// when the status code is not OK.  This allows the request handler to return the
// appropriate status code to the calling client which is especially important for rate limits.
func toStatusCodeError(err error) error {
	var responseError *azcore.ResponseError
	if errors.As(err, &responseError) {
		if responseError.StatusCode != http.StatusOK {
			return types.NewErrStatusNotOK("AzureOpenAI", responseError.RawResponse)
		}
	}
	return err
}

type apiVersionRoundTripper struct {
	rt         http.RoundTripper
	apiVersion string
}

func (rt *apiVersionRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := req.Clone(req.Context())
	values := newReq.URL.Query()
	values.Set("api-version", rt.apiVersion)
	newReq.URL.RawQuery = values.Encode()
	return rt.rt.RoundTrip(newReq)
}

func apiVersionClient(apiVersion string) *http.Client {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	azureClientDefaultTransport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion:    tls.VersionTLS12,
			Renegotiation: tls.RenegotiateFreelyAsClient,
		},
	}

	if http2Transport, err := http2.ConfigureTransports(azureClientDefaultTransport); err == nil {
		http2Transport.ReadIdleTimeout = 10 * time.Second
		http2Transport.PingTimeout = 5 * time.Second
	}

	return &http.Client{
		Transport: &apiVersionRoundTripper{
			rt:         azureClientDefaultTransport,
			apiVersion: apiVersion,
		},
	}
}
