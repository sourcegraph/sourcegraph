package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/embed/client"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/embed/client/modeltransformations"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func NewClient(httpClient httpcli.Doer, config *conftypes.EmbeddingsConfig) *openaiEmbeddingsClient {
	return &openaiEmbeddingsClient{
		httpClient:  httpClient,
		dimensions:  config.Dimensions,
		accessToken: config.AccessToken,
		model:       config.Model,
		endpoint:    config.Endpoint,
	}
}

type openaiEmbeddingsClient struct {
	httpClient  httpcli.Doer
	model       string
	dimensions  int
	endpoint    string
	accessToken string
}

func (c *openaiEmbeddingsClient) GetDimensions() (int, error) {
	if c.dimensions <= 0 {
		return 0, errors.New("invalid config for embeddings.dimensions, must be > 0")
	}
	return c.dimensions, nil
}

func (c *openaiEmbeddingsClient) GetModelIdentifier() string {
	return fmt.Sprintf("openai/%s", c.model)
}

func (c *openaiEmbeddingsClient) GetQueryEmbedding(ctx context.Context, query string) (*client.EmbeddingsResults, error) {
	return c.getEmbeddings(ctx, []string{modeltransformations.ApplyToQuery(query, c.GetModelIdentifier())})
}

func (c *openaiEmbeddingsClient) GetDocumentEmbeddings(ctx context.Context, documents []string) (*client.EmbeddingsResults, error) {
	return c.getEmbeddings(ctx, modeltransformations.ApplyToDocuments(documents, c.GetModelIdentifier()))
}

func (c *openaiEmbeddingsClient) getEmbeddings(ctx context.Context, texts []string) (*client.EmbeddingsResults, error) {
	for _, text := range texts {
		if text == "" {
			// The OpenAI API will return an error if any of the strings in texts is an empty string,
			// so fail fast to avoid making tons of retryable requests.
			return nil, errors.New("cannot generate embeddings for an empty string")
		}
	}

	response, err := c.do(ctx, openaiEmbeddingAPIRequest{Model: c.model, Input: texts})
	if err != nil {
		return nil, err
	}

	if len(response.Data) == 0 {
		return nil, nil
	}

	// Ensure embedding responses are sorted in the original order.
	sort.Slice(response.Data, func(i, j int) bool {
		return response.Data[i].Index < response.Data[j].Index
	})

	dimensionality := len(response.Data[0].Embedding)
	embeddings := make([]float32, 0, len(response.Data)*dimensionality)
	failed := make([]int, 0)
	for _, embedding := range response.Data {
		if len(embedding.Embedding) != 0 {
			embeddings = append(embeddings, embedding.Embedding...)
		} else {
			// HACK(camdencheek): Nondeterministically, the OpenAI API will
			// occasionally send back a `null` for an embedding in the
			// response. Try it again a few times and hope for the best.
			resp, err := c.requestSingleEmbeddingWithRetryOnNull(ctx, texts[embedding.Index], 3)
			if err != nil {
				failed = append(failed, embedding.Index)

				// reslice to provide zero value embedding for failed chunk
				embeddings = embeddings[:len(embeddings)+dimensionality]
				continue
			}
			embeddings = append(embeddings, resp.Data[0].Embedding...)
		}
	}

	return &client.EmbeddingsResults{Embeddings: embeddings, Failed: failed, Dimensions: dimensionality}, nil
}

func (c *openaiEmbeddingsClient) requestSingleEmbeddingWithRetryOnNull(ctx context.Context, input string, retries int) (*openaiEmbeddingAPIResponse, error) {
	for i := 0; i < retries; i++ {
		response, err := c.do(ctx, openaiEmbeddingAPIRequest{Model: c.model, Input: []string{input}})
		if err != nil {
			return nil, err
		}
		if len(response.Data) != 1 || len(response.Data[0].Embedding) == 0 {
			continue
		}
		return response, nil
	}
	return nil, errors.Newf("null response for embedding after %d retries", retries)
}

func (c *openaiEmbeddingsClient) do(ctx context.Context, request openaiEmbeddingAPIRequest) (*openaiEmbeddingAPIResponse, error) {
	bodyBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, errors.Errorf("embeddings: %s %q: failed with status %d: %s", req.Method, req.URL.String(), resp.StatusCode, string(respBody))
	}

	var response openaiEmbeddingAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	return &response, nil
}

type openaiEmbeddingAPIRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openaiEmbeddingAPIResponse struct {
	Data []openaiEmbeddingAPIResponseData `json:"data"`
}

type openaiEmbeddingAPIResponseData struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}
