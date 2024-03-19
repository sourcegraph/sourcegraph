package azureopenai

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/ai/azopenai"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/embed/client"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/embed/client/modeltransformations"
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
var apiClient embeddingsClient

type embeddingsClient struct {
	mu          sync.RWMutex
	accessToken string
	endpoint    string
	client      *azopenai.Client
}

type EmbeddingsClient interface {
	GetEmbeddings(ctx context.Context, body azopenai.EmbeddingsOptions, options *azopenai.GetEmbeddingsOptions) (azopenai.GetEmbeddingsResponse, error)
}

type GetEmbeddingsAPIClientFunc func(endpoint, accessToken string) (EmbeddingsClient, error)

func GetAPIClient(endpoint, accessToken string) (EmbeddingsClient, error) {
	apiClient.mu.RLock()
	if apiClient.client != nil && apiClient.endpoint == endpoint && apiClient.accessToken == accessToken {
		apiClient.mu.RUnlock()
		return apiClient.client, nil
	}
	apiClient.mu.RUnlock()
	apiClient.mu.Lock()
	defer apiClient.mu.Unlock()
	var err error
	if accessToken != "" {
		credential := azcore.NewKeyCredential(accessToken)
		apiClient.client, err = azopenai.NewClientWithKeyCredential(endpoint, credential, nil)
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
		apiClient.client, err = azopenai.NewClient(endpoint, credential, nil)
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

func NewClient(getClient GetEmbeddingsAPIClientFunc, config *conftypes.EmbeddingsConfig) (*azureOpenaiEmbeddingsClient, error) {
	client, err := getClient(config.Endpoint, config.AccessToken)
	if err != nil {
		return nil, err
	}
	return &azureOpenaiEmbeddingsClient{
		client:      client,
		dimensions:  config.Dimensions,
		accessToken: config.AccessToken,
		model:       config.Model,
		endpoint:    config.Endpoint,
	}, nil
}

type azureOpenaiEmbeddingsClient struct {
	client      EmbeddingsClient
	model       string
	dimensions  int
	endpoint    string
	accessToken string
}

func (c *azureOpenaiEmbeddingsClient) GetDimensions() (int, error) {
	if c.dimensions <= 0 {
		return 0, errors.New("invalid config for embeddings.dimensions, must be > 0")
	}
	return c.dimensions, nil
}

func (c *azureOpenaiEmbeddingsClient) GetModelIdentifier() string {
	return fmt.Sprintf("azure-openai/%s", c.model)
}

func (c *azureOpenaiEmbeddingsClient) GetQueryEmbedding(ctx context.Context, query string) (*client.EmbeddingsResults, error) {
	return c.getEmbeddings(ctx, []string{modeltransformations.ApplyToQuery(query, c.GetModelIdentifier())})
}

func (c *azureOpenaiEmbeddingsClient) GetDocumentEmbeddings(ctx context.Context, documents []string) (*client.EmbeddingsResults, error) {
	return c.getEmbeddings(ctx, modeltransformations.ApplyToDocuments(documents, c.GetModelIdentifier()))
}

// getEmbeddings tries to embed the given texts using the external service specified in the config.
func (c *azureOpenaiEmbeddingsClient) getEmbeddings(ctx context.Context, texts []string) (*client.EmbeddingsResults, error) {
	for _, text := range texts {
		if text == "" {
			// The Azure OpenAI API will return an error if any of the strings in texts is an empty string,
			// so fail fast to avoid making tons of retryable requests.
			return nil, errors.New("cannot generate embeddings for an empty string")
		}
	}

	// For now, we assume all Azure OpenAI models will benefit from stripping out newlines.
	augmentedTexts := make([]string, len(texts))
	// Replace newlines for certain (OpenAI) models, because they can negatively affect performance.
	for idx, text := range texts {
		augmentedTexts[idx] = strings.ReplaceAll(text, "\n", " ")
	}

	embeddings := make([]float32, 0, len(augmentedTexts)*c.dimensions)
	failed := make([]int, 0)
	for i, input := range augmentedTexts {
		// This is a difference to the OpenAI implementation: Azure OpenAI currently
		// only supports a single input at a time, so we will need to fire off a request
		// for each of the texts individually.
		resp, err := c.requestSingleEmbeddingWithRetryOnNull(ctx, input, 3)
		if err != nil {
			failed = append(failed, i)

			// reslice to provide zero value embedding for failed chunk
			embeddings = embeddings[:len(embeddings)+c.dimensions]
			continue
		}
		embeddings = append(embeddings, resp.Data[0].Embedding...)
	}

	return &client.EmbeddingsResults{Embeddings: embeddings, Failed: failed, Dimensions: c.dimensions}, nil
}

func (c *azureOpenaiEmbeddingsClient) requestSingleEmbeddingWithRetryOnNull(ctx context.Context, input string, retries int) (*azopenai.GetEmbeddingsResponse, error) {
	for range retries {
		response, err := c.client.GetEmbeddings(ctx, azopenai.EmbeddingsOptions{
			Input:          []string{input},
			DeploymentName: &c.model,
		}, nil)
		if err != nil {
			return nil, err
		}
		if len(response.Data) != 1 || len(response.Data[0].Embedding) == 0 {
			continue
		}
		return &response, nil
	}
	return nil, errors.Newf("null response for embedding after %d retries", retries)
}
