package qa

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/codygateway"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var errNotImplemented = errors.New("not implemented")

type OpenAIGatewayFeatureClient struct{}

func (o OpenAIGatewayFeatureClient) GetRequest(f codygateway.Feature, req *http.Request, stream bool) (*http.Request, error) {
	if f == codygateway.FeatureCodeCompletions {
		return nil, errNotImplemented
	}
	if f == codygateway.FeatureChatCompletions {
		body := fmt.Sprintf(`{
			"model":"gpt-4-turbo-preview",
			"messages":[{"role":"user","content":"You are Cody"},{"role":"assistant","content":"Ok, I am Cody"},{"role":"user","content":"What is your real name name though?"}],
			"n":1,
			"max_tokens":30,
			"temperature":0.2,
			"top_p":0.95,
			"stream":%t
			}`, stream)
		req.Method = "POST"
		req.URL.Path = "/v1/completions/openai"
		req.Body = io.NopCloser(strings.NewReader(body))
		return req, nil
	}
	return nil, errors.New("unknown feature: " + string(f))
}
