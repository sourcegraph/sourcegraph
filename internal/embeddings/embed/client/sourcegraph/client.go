package sourcegraph

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/codygateway"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/embed/client"
	"github.com/sourcegraph/sourcegraph/internal/embeddings/embed/client/modeltransformations"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func NewClient(client httpcli.Doer, config *conftypes.EmbeddingsConfig) *sourcegraphEmbeddingsClient {
	return &sourcegraphEmbeddingsClient{
		httpClient:  client,
		model:       config.Model,
		dimensions:  config.Dimensions,
		endpoint:    config.Endpoint,
		accessToken: config.AccessToken,
	}
}

type sourcegraphEmbeddingsClient struct {
	httpClient  httpcli.Doer
	model       string
	dimensions  int
	endpoint    string
	accessToken string
}

func (c *sourcegraphEmbeddingsClient) GetDimensions() (int, error) {
	// TODO: Later, we should ideally ask the gateway for the dimensionality of the model
	// so we don't have to hard-code defaults for all the models and can roll out new models
	// to older instances, too.
	if c.dimensions <= 0 {
		return 0, errors.New("invalid config for embeddings.dimensions, must be > 0")
	}

	return c.dimensions, nil
}

func (c *sourcegraphEmbeddingsClient) GetModelIdentifier() string {
	// Special-case the default model, since it already includes the provider name.
	// This ensures we can safely migrate customers from the OpenAI provider to
	// Cody Gateway.
	if strings.EqualFold(c.model, "openai/text-embedding-ada-002") {
		return "openai/text-embedding-ada-002"
	}
	return fmt.Sprintf("sourcegraph/%s", c.model)
}

func (c *sourcegraphEmbeddingsClient) GetQueryEmbedding(ctx context.Context, query string) (*client.EmbeddingsResults, error) {
	return c.getEmbeddings(ctx, []string{modeltransformations.ApplyToQuery(query, c.GetModelIdentifier())})
}

func (c *sourcegraphEmbeddingsClient) GetDocumentEmbeddings(ctx context.Context, documents []string) (*client.EmbeddingsResults, error) {
	return c.getEmbeddings(ctx, modeltransformations.ApplyToDocuments(documents, c.GetModelIdentifier()))
}

func (c *sourcegraphEmbeddingsClient) getEmbeddings(ctx context.Context, texts []string) (*client.EmbeddingsResults, error) {
	request := codygateway.EmbeddingsRequest{Model: c.model, Input: texts}
	response, err := c.do(ctx, request)
	if err != nil {
		return nil, err
	}

	if len(response.Embeddings) == 0 {
		return nil, nil
	}

	// Ensure embedding responses are sorted in the original order.
	sort.Slice(response.Embeddings, func(i, j int) bool {
		return response.Embeddings[i].Index < response.Embeddings[j].Index
	})

	dimensionality := response.ModelDimensions
	embeddings := make([]float32, 0, len(response.Embeddings)*dimensionality)
	failed := make([]int, 0)
	for _, embedding := range response.Embeddings {
		if len(embedding.Data) > 0 {
			embeddings = append(embeddings, embedding.Data...)
		} else {
			resp, err := c.requestSingleEmbeddingWithRetryOnNull(ctx, c.model, texts[embedding.Index], 3)
			if err != nil {
				failed = append(failed, embedding.Index)

				// reslice to provide zero value embedding for failed chunk
				embeddings = embeddings[:len(embeddings)+dimensionality]
				continue
			}
			embeddings = append(embeddings, resp...)
		}
	}

	return &client.EmbeddingsResults{Embeddings: embeddings, Failed: failed, Dimensions: response.ModelDimensions}, nil
}

func (c *sourcegraphEmbeddingsClient) do(ctx context.Context, request codygateway.EmbeddingsRequest) (*codygateway.EmbeddingsResponse, error) {
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
	if len(request.Input) > 1 {
		req.Header.Set("X-Cody-Embed-Batch-Size", strconv.Itoa(len(request.Input)))
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			retryAfterHeader := resp.Header.Get("retry-after")
			if retryAfterHeader != "" {
				// There are two valid formats for retry-after headers: seconds
				// until retry in int, or a RFC1123 date string.
				// First, see if it is denoted in seconds.
				s, err := strconv.Atoi(retryAfterHeader)
				// If denoted in seconds, only retry if we will get access within
				// the next retryAfterMaxSleepDuration seconds.
				if err == nil {
					return nil, client.NewRateLimitExceededError(time.Now().Add(time.Duration(s) * time.Second))
				}

				// If we weren't able to parse as seconds, try to parse as RFC1123.
				after, err := time.Parse(time.RFC1123, retryAfterHeader)
				if err == nil {
					return nil, client.NewRateLimitExceededError(after)
				}
				// We don't know how to parse this header, so let's just return a generic error.
			}
		}
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, errors.Errorf("embeddings: %s %q: failed with status %d: %s", req.Method, req.URL.String(), resp.StatusCode, string(respBody))
	}

	var response codygateway.EmbeddingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *sourcegraphEmbeddingsClient) requestSingleEmbeddingWithRetryOnNull(ctx context.Context, model string, input string, retries int) (resp []float32, err error) {
	for i := 0; i < retries; i++ {
		resp, err := c.do(ctx, codygateway.EmbeddingsRequest{
			Model: model,
			Input: []string{input},
		})
		if err != nil {
			return nil, err
		}
		if len(resp.Embeddings) != 1 || len(resp.Embeddings[0].Data) != c.dimensions {
			continue // retry
		}
		return resp.Embeddings[0].Data, err
	}
	return nil, errors.Newf("null response for embedding after %d retries", retries)
}
