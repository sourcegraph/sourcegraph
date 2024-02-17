package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/httpcli"
)

// schemeExecutorToken is the special type of token to communicate with the executor endpoints.
const schemeExecutorToken = "token-executor"

// schemeJobToken is the special type of token to communicate with the job endpoints.
const schemeJobToken = "Bearer"

// BaseClient is an abstract HTTP API-backed data access layer. Instances of this
// struct should not be used directly, but should be used compositionally by other
// stores that implement logic specific to a domain.
//
// The following is a minimal example of decorating the base client, making the
// actual logic of the decorated client extremely lean:
//
//	type SprocketClient struct {
//	    *httpcli.BaseClient
//
//	    baseURL *url.URL
//	}
//
//	func (c *SprocketClient) Fabricate(ctx context.Context(), spec SprocketSpec) (Sprocket, error) {
//	    url := c.baseURL.ResolveReference(&url.URL{Path: "/new"})
//
//	    req, err := httpcli.NewJSONRequest("POST", url.String(), spec)
//	    if err != nil {
//	        return Sprocket{}, err
//	    }
//
//	    var s Sprocket
//	    err := c.client.DoAndDecode(ctx, req, &s)
//	    return s, err
//	}
type BaseClient struct {
	httpClient httpcli.Doer
	options    BaseClientOptions
	baseURL    *url.URL
	logger     log.Logger
}

type BaseClientOptions struct {
	// ExecutorName name of the executor host.
	ExecutorName string

	// UserAgent specifies the user agent string to supply on requests.
	UserAgent string

	// EndpointOptions configures the endpoint the BaseClient will call for requests.
	EndpointOptions EndpointOptions
}

type EndpointOptions struct {
	// URL is the target request URL.
	URL string

	// PathPrefix is the prefix of the path to be called by the BaseClient.
	PathPrefix string

	// Token is the authorization token to include with all requests (via Authorization header).
	Token string
}

// NewBaseClient creates a new BaseClient with the given transport.
func NewBaseClient(logger log.Logger, options BaseClientOptions) (*BaseClient, error) {
	// Parse the base url upfront to save on overhead.
	baseURL, err := url.Parse(options.EndpointOptions.URL)
	if err != nil {
		return nil, err
	}
	cli, err := httpcli.NewInternalClientFactory("executor").Doer()
	if err != nil {
		return nil, err
	}
	return &BaseClient{
		httpClient: cli,
		options:    options,
		baseURL:    baseURL,
		logger:     logger,
	}, nil
}

// Do performs the given HTTP request and returns the body. If there is no content
// to be read due to a 204 response, then a false-valued flag is returned.
func (c *BaseClient) Do(ctx context.Context, req *http.Request) (hasContent bool, _ io.ReadCloser, err error) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.options.UserAgent)
	req = req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent {
			return false, nil, nil
		}

		if content, err := io.ReadAll(resp.Body); err != nil {
			c.logger.Error("Failed to read response body", log.Error(err))
		} else {
			c.logger.Error(
				"apiclient got unexpected status code",
				log.Int("code", resp.StatusCode),
				log.String("body", string(content)),
			)
		}

		return false, nil, &UnexpectedStatusCodeErr{StatusCode: resp.StatusCode}
	}

	return true, resp.Body, nil
}

type UnexpectedStatusCodeErr struct {
	StatusCode int
}

func (e *UnexpectedStatusCodeErr) Error() string {
	return fmt.Sprintf("unexpected status code %d", e.StatusCode)
}

// DoAndDecode performs the given HTTP request and unmarshals the response body into the
// given interface pointer. If the response body was empty due to a 204 response, then a
// false-valued flag is returned.
func (c *BaseClient) DoAndDecode(ctx context.Context, req *http.Request, payload any) (decoded bool, _ error) {
	hasContent, body, err := c.Do(ctx, req)
	if err == nil && hasContent {
		defer body.Close()
		return true, json.NewDecoder(body).Decode(&payload)
	}

	return false, err
}

// DoAndDrop performs the given HTTP request and ignores the response body.
func (c *BaseClient) DoAndDrop(ctx context.Context, req *http.Request) error {
	hasContent, body, err := c.Do(ctx, req)
	if hasContent {
		defer body.Close()
	}

	return err
}

// NewRequest creates a new http.Request where only the Authorization HTTP header is set.
func (c *BaseClient) NewRequest(jobId int, token, method, path string, payload io.Reader) (*http.Request, error) {
	u := c.newRelativeURL(path)

	r, err := http.NewRequest(method, u.String(), payload)
	if err != nil {
		return nil, err
	}

	c.addHeaders(jobId, token, r)
	return r, nil
}

// NewJSONRequest creates a new http.Request where the Content-Type is set to 'application/json' and the Authorization
// HTTP header is set.
func (c *BaseClient) NewJSONRequest(method, path string, payload any) (*http.Request, error) {
	u := c.newRelativeURL(path)

	r, err := newJSONRequest(method, u, payload)
	if err != nil {
		return nil, err
	}

	r.Header.Add("Authorization", fmt.Sprintf("%s %s", schemeExecutorToken, c.options.EndpointOptions.Token))
	return r, nil
}

// NewJSONJobRequest creates a new http.Request where the Content-Type is set to 'application/json' and the Authorization
// HTTP header is set.
func (c *BaseClient) NewJSONJobRequest(jobId int, method, path string, token string, payload any) (*http.Request, error) {
	u := c.newRelativeURL(path)

	r, err := newJSONRequest(method, u, payload)
	if err != nil {
		return nil, err
	}

	c.addHeaders(jobId, token, r)
	return r, nil
}

// newRelativeURL builds the relative URL on the provided base URL and adds any additional paths.
func (c *BaseClient) newRelativeURL(endpointPath string) *url.URL {
	// Create a shallow clone
	u := *c.baseURL
	u.Path = path.Join(u.Path, c.options.EndpointOptions.PathPrefix, endpointPath)
	return &u
}

// newJSONRequest creates an HTTP request with the given payload serialized as JSON. This
// will also ensure that the proper content type header (which is necessary, not pedantic).
func newJSONRequest(method string, url *url.URL, payload any) (*http.Request, error) {
	contents, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, url.String(), bytes.NewReader(contents))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (c *BaseClient) addHeaders(jobId int, token string, r *http.Request) {
	// If there is no token set, we may be talking with a version of Sourcegraph that is behind.
	if len(token) > 0 {
		r.Header.Add("Authorization", fmt.Sprintf("%s %s", schemeJobToken, token))
	} else {
		r.Header.Add("Authorization", fmt.Sprintf("%s %s", schemeExecutorToken, c.options.EndpointOptions.Token))
	}
	r.Header.Add("X-Sourcegraph-Job-ID", strconv.Itoa(jobId))
	r.Header.Add("X-Sourcegraph-Executor-Name", c.options.ExecutorName)
}
